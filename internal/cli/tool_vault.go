// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/vault"
)

// Passphrase-reading seams for tests. Production code uses the helpers
// defined in hot.go / internal/keystore; tests override these with
// deterministic in-memory readers.
var (
	readPassTwiceFn  = readPassTwice
	readPassphraseFn = keystore.ReadPassphrase
	promptConfirmFn  = promptConfirmStdin
)

// promptConfirmStdin reads one line from os.Stdin and returns true if
// the user answered affirmatively. Factored so tests can stub TTY
// confirmation without touching real os.Stdin.
func promptConfirmStdin() (bool, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	return isAffirmative(strings.TrimSpace(strings.ToLower(line))), nil
}

func newToolVaultCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "vault",
		Short: "Pack/unpack the keystore directory as a single passphrase-encrypted archive",
		Long: `Bundle every wallet file in the keystore directory into a single
age-encrypted archive (scrypt passphrase recipient), suitable for backup
or transfer between machines. The inverse command decrypts and restores
the files atomically.

The vault passphrase is a SEPARATE secret from the per-wallet keystore
passphrases. Losing the vault passphrase makes the archive unreadable;
losing a keystore passphrase makes that individual wallet unrecoverable
regardless of the vault.`,
	}
	c.AddCommand(newToolVaultPackCmd())
	c.AddCommand(newToolVaultUnpackCmd())
	return c
}

func newToolVaultPackCmd() *cobra.Command {
	var (
		outPath   string
		armorFlag bool
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Encrypt the whole keystore directory into a single age archive",
		Long: `Read every *.json and *.scrypt.bak[.<ts>] file from --keystore-dir
(no recursion, no other files), bundle them into an uncompressed tar,
and encrypt the tar with a scrypt passphrase recipient via age.

Use --out - to write to stdout. By default the command refuses to
overwrite an existing --out file. Pass --overwrite to replace it;
on a TTY you will be asked for [y/N] confirmation first. Replacement
is atomic (temp file + rename) so a crashed pack never corrupts the
previous archive.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			return runVaultPack(cmd.ErrOrStderr(), cfg.KeystoreDir, outPath, armorFlag, overwrite, stdinIsTerminal())
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output archive path (use - for stdout)")
	cmd.Flags().BoolVar(&armorFlag, "armor", false, "Wrap the output in ASCII armor (PEM-like, 7-bit clean)")
	cmd.Flags().BoolVar(&overwrite, "overwrite",
		false, "DANGEROUS: replace an existing --out file (atomic temp+rename)")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func newToolVaultUnpackCmd() *cobra.Command {
	var (
		inPath    string
		outDir    string
		overwrite bool
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "unpack",
		Short: "Decrypt a vault archive and restore keystore files into a directory",
		Long: `Decrypt the vault archive (age, ASCII armor auto-detected), validate
every tar entry against the vault allowlist (no path traversal,
no directories, monujo keystore filenames only) and write the files
atomically into --out.

By default, if any target file already exists in --out, the operation
aborts before touching the disk. Pass --overwrite to replace existing
files; on a TTY you will be asked for [y/N] confirmation first.
--dry-run runs everything except the final writes.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			dest := outDir
			if strings.TrimSpace(dest) == "" {
				dest = cfg.KeystoreDir
			}
			return runVaultUnpack(cmd.ErrOrStderr(), inPath, dest, overwrite, dryRun, stdinIsTerminal())
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Input archive path (use - for stdin)")
	cmd.Flags().StringVar(&outDir, "out", "", "Destination directory (default: --keystore-dir)")
	cmd.Flags().BoolVar(&overwrite, "overwrite",
		false, "DANGEROUS: replace existing files in the destination directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the archive without writing any files")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}

// runVaultPack executes the pack flow. It is extracted from the cobra
// RunE so tests can exercise it with a plain io.Writer and without
// spawning a subprocess. isTerminal controls whether the --overwrite
// path requires an interactive [y/N] confirmation.
func runVaultPack(w io.Writer, keystoreDir, outPath string, armorFlag, overwrite, isTerminal bool) error {
	if strings.TrimSpace(keystoreDir) == "" {
		return errors.New("vault pack: empty keystore directory")
	}
	if strings.TrimSpace(outPath) == "" {
		return errors.New("vault pack: --out is required")
	}

	var (
		dst         io.Writer
		closeFn     func() error
		finalizeFn  func() error // called after a successful Pack to atomically publish the result
		cleanupFn   func()       // called on any error path after dst is opened
		writingTo   string
		replacedOld bool
	)
	switch {
	case outPath == "-":
		dst = os.Stdout
		closeFn = func() error { return nil }
		finalizeFn = func() error { return nil }
		cleanupFn = func() {}
		writingTo = "stdout"
	default:
		// Distinguish create-new vs replace-existing to keep the
		// happy path cheap and to honor --overwrite semantics.
		_, statErr := os.Stat(outPath)
		existed := statErr == nil
		switch {
		case statErr != nil && !os.IsNotExist(statErr):
			return fmt.Errorf("vault pack: stat --out: %w", statErr)
		case existed && !overwrite:
			return fmt.Errorf("vault pack: %q already exists; pass --overwrite to replace it", outPath)
		case existed && overwrite:
			if isTerminal {
				fmt.Fprintf(w, "DANGER: --overwrite will replace the existing archive at %s.\n", outPath)
				fmt.Fprintf(w, "Any earlier backup at that path will be LOST forever.\n")
				fmt.Fprintf(w, "Overwrite? [y/N]: ")
				ok, err := promptConfirmFn()
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("aborted")
				}
			}
			tmpFile, err := os.CreateTemp(filepath.Dir(outPath), ".monujo-vault-*.tmp")
			if err != nil {
				return fmt.Errorf("vault pack: create temp: %w", err)
			}
			if err := tmpFile.Chmod(0o600); err != nil {
				_ = tmpFile.Close()
				_ = os.Remove(tmpFile.Name())
				return fmt.Errorf("vault pack: chmod temp: %w", err)
			}
			tmpPath := tmpFile.Name()
			dst = tmpFile
			closeFn = tmpFile.Close
			finalizeFn = func() error { return os.Rename(tmpPath, outPath) }
			cleanupFn = func() { _ = os.Remove(tmpPath) }
			replacedOld = true
		default:
			// Target does not exist: O_EXCL direct create, no temp
			// file needed.
			f, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- outPath is a user-supplied archive destination by design.
			if err != nil {
				return fmt.Errorf("vault pack: open --out: %w", err)
			}
			dst = f
			closeFn = f.Close
			finalizeFn = func() error { return nil }
			cleanupFn = func() { _ = os.Remove(outPath) }
		}
		writingTo = outPath
	}

	pass, err := readPassTwiceFn("New vault passphrase: ")
	if err != nil {
		_ = closeFn()
		cleanupFn()
		return err
	}
	defer keystore.ZeroPassphrase(pass)

	report, err := vault.Pack(dst, keystoreDir, pass, vault.PackOptions{Armor: armorFlag})
	if cerr := closeFn(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		cleanupFn()
		return err
	}
	if ferr := finalizeFn(); ferr != nil {
		cleanupFn()
		return fmt.Errorf("vault pack: finalize: %w", ferr)
	}

	fmt.Fprintf(w, "Packed %d file(s), %d plaintext bytes, to %s.\n",
		len(report.Files), report.Bytes, writingTo)
	if replacedOld {
		fmt.Fprintf(w, "(Replaced previous archive at %s.)\n", outPath)
	}
	for _, name := range report.Files {
		fmt.Fprintf(w, "  + %s\n", name)
	}
	if len(report.SkippedNonAllowlisted) > 0 {
		fmt.Fprintf(w, "Skipped (not keystore files):\n")
		for _, name := range report.SkippedNonAllowlisted {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}
	return nil
}

// runVaultUnpack executes the unpack flow with an explicit isTerminal
// flag so tests can simulate non-TTY callers.
func runVaultUnpack(w io.Writer, inPath, destDir string, overwrite, dryRun, isTerminal bool) error {
	if strings.TrimSpace(inPath) == "" {
		return errors.New("vault unpack: --in is required")
	}
	if strings.TrimSpace(destDir) == "" {
		return errors.New("vault unpack: empty destination directory")
	}

	if overwrite && isTerminal && !dryRun {
		fmt.Fprintf(w, "DANGER: --overwrite will replace existing files in %s.\n", destDir)
		fmt.Fprintf(w, "Any wallet currently at a conflicting path will be LOST forever.\n")
		fmt.Fprintf(w, "Overwrite? [y/N]: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if !isAffirmative(line) {
			return errors.New("aborted")
		}
	}

	var src io.Reader
	var closeFn func() error
	if inPath == "-" {
		src = os.Stdin
		closeFn = func() error { return nil }
	} else {
		f, err := os.Open(inPath) // #nosec G304 -- inPath is a user-supplied archive source by design.
		if err != nil {
			return fmt.Errorf("vault unpack: open --in: %w", err)
		}
		src = f
		closeFn = f.Close
	}
	defer func() { _ = closeFn() }()

	pass, err := readPassphraseFn("Vault passphrase: ")
	if err != nil {
		return err
	}
	defer keystore.ZeroPassphrase(pass)

	report, err := vault.Unpack(src, destDir, pass, vault.UnpackOptions{
		Overwrite: overwrite,
		DryRun:    dryRun,
	})
	if err != nil {
		if errors.Is(err, vault.ErrOverwriteRequired) {
			fmt.Fprintf(w, "Refusing to overwrite existing files in %s:\n", destDir)
			for _, name := range report.Conflicts {
				fmt.Fprintf(w, "  ! %s\n", name)
			}
			fmt.Fprintf(w, "Pass --overwrite to replace them (DANGEROUS, irreversible).\n")
		}
		return err
	}

	action := "Unpacked"
	if dryRun {
		action = "Would unpack"
	}
	fmt.Fprintf(w, "%s %d file(s) into %s.\n", action, len(report.Written), destDir)
	for _, name := range report.Written {
		fmt.Fprintf(w, "  + %s\n", name)
	}
	if len(report.Skipped) > 0 {
		fmt.Fprintf(w, "Skipped (not allowlisted):\n")
		for _, name := range report.Skipped {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}
	return nil
}
