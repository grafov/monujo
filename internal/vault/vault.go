// Package vault packs and unpacks a whole monujo keystore directory as a
// single passphrase-encrypted file. The outer container is an age file
// (RFC 9580-style, scrypt passphrase recipient via filippo.io/age); the
// plaintext payload is an uncompressed tar with a flat list of keystore
// files (no directories). An optional ASCII armor layer (PEM-like) is
// available for transports that mangle binary data.
//
// The package intentionally uses the same cryptographic primitive
// (password-based scrypt KDF + AEAD) that the age CLI would use with
// `age -p`, so a vault file can, in principle, also be opened with the
// upstream age tool. monujo does not exec any external binary.
//
// Passphrase hygiene: age only accepts a passphrase as a Go string,
// which cannot be zeroed. Callers should keep the []byte form of the
// passphrase for the minimum time possible and wipe it with
// keystore.ZeroPassphrase after Pack/Unpack returns. The short-lived
// string copy inside this package is unavoidable.
package vault

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// PackOptions controls how [Pack] writes the vault file.
type PackOptions struct {
	// Armor wraps the age ciphertext in ASCII armor (PEM-like) so the
	// result is 7-bit clean and safe to paste into email or git.
	Armor bool
}

// PackReport summarizes what [Pack] did. It is returned even on
// success; callers typically print it to stderr for the user.
type PackReport struct {
	// Files lists the basenames (no directory) that ended up in the tar.
	Files []string
	// SkippedNonAllowlisted lists entries of srcDir that did not pass
	// the vault allowlist and were silently ignored.
	SkippedNonAllowlisted []string
	// Bytes is the plaintext tar size in bytes (pre-encryption).
	Bytes int64
}

// UnpackOptions controls how [Unpack] writes files to destDir.
type UnpackOptions struct {
	// Overwrite allows replacing files that already exist in destDir.
	// When false, any conflict aborts the whole operation before a
	// single file is written.
	Overwrite bool
	// DryRun performs all validations (decryption, tar parsing,
	// allowlist, overwrite check) but writes nothing to disk.
	DryRun bool
}

// UnpackReport summarizes what [Unpack] did (or would have done in
// DryRun mode).
type UnpackReport struct {
	// Written is the list of basenames that were (or would be) written.
	Written []string
	// Conflicts is the list of basenames that already existed in
	// destDir; non-empty only when Overwrite is false and the caller
	// received ErrOverwriteRequired.
	Conflicts []string
	// Skipped is the list of tar entries that did not pass the vault
	// allowlist or were not regular files (directories, symlinks,
	// device nodes, etc.).
	Skipped []string
}

// ErrOverwriteRequired is returned by [Unpack] when the destination
// directory already contains one or more files that would be replaced
// and UnpackOptions.Overwrite is false. No files on disk are modified
// when this error is returned.
var ErrOverwriteRequired = errors.New("vault unpack: destination has conflicts; pass overwrite to replace")

// allowedName matches monujo keystore filenames that are safe to pack:
// <safeName>.json, <safeName>.scrypt.bak, and <safeName>.scrypt.bak.<digits>.
// safeName itself is [A-Za-z0-9_-]+ as enforced by keystore.safeName.
var allowedName = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.json|\.scrypt\.bak(?:\.\d+)?)$`)

// isAllowlisted reports whether name (a basename, no directory) is a
// monujo keystore artifact that Pack should include.
func isAllowlisted(name string) bool {
	return allowedName.MatchString(name)
}

// Pack reads every allowlisted keystore file from srcDir (non-recursive),
// bundles them into an uncompressed tar, encrypts the tar with age
// using a scrypt passphrase recipient, and writes the result to dst.
// When opts.Armor is true the output is ASCII-armored.
//
// srcDir must contain at least one allowlisted file; an empty directory
// is treated as a user error so the caller does not produce a useless
// "vault with no wallets inside" file.
//
// passphrase is used only for the duration of the call; the caller
// retains ownership of the slice and must wipe it afterwards (see
// keystore.ZeroPassphrase). It must not be empty.
func Pack(dst io.Writer, srcDir string, passphrase []byte, opts PackOptions) (PackReport, error) {
	var rep PackReport
	if len(passphrase) == 0 {
		return rep, errors.New("vault pack: empty passphrase")
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return rep, fmt.Errorf("vault pack: read dir: %w", err)
	}

	type pending struct {
		name string
		body []byte
		mode int64
	}
	var files []pending
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			rep.SkippedNonAllowlisted = append(rep.SkippedNonAllowlisted, name)
			continue
		}
		if !isAllowlisted(name) {
			rep.SkippedNonAllowlisted = append(rep.SkippedNonAllowlisted, name)
			continue
		}
		path := filepath.Join(srcDir, name)
		body, err := os.ReadFile(path) // #nosec G304 -- path is built from a validated allowlisted basename inside srcDir.
		if err != nil {
			return rep, fmt.Errorf("vault pack: read %s: %w", name, err)
		}
		files = append(files, pending{name: name, body: body, mode: 0o600})
	}
	if len(files) == 0 {
		return rep, errors.New("vault pack: nothing to pack (no *.json / *.scrypt.bak files in keystore directory)")
	}

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, f := range files {
		hdr := &tar.Header{
			Name:     f.name,
			Mode:     f.mode,
			Size:     int64(len(f.body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return rep, fmt.Errorf("vault pack: tar header %s: %w", f.name, err)
		}
		if _, err := tw.Write(f.body); err != nil {
			return rep, fmt.Errorf("vault pack: tar write %s: %w", f.name, err)
		}
		rep.Files = append(rep.Files, f.name)
	}
	if err := tw.Close(); err != nil {
		return rep, fmt.Errorf("vault pack: tar close: %w", err)
	}
	rep.Bytes = int64(tarBuf.Len())

	// Short-lived string copy of the passphrase: age requires it.
	// Short-lived string copy of the passphrase: age only accepts it as
	// a Go string, which is immutable and cannot be zeroed. The caller
	// wipes the []byte form; this copy is released when recipient goes
	// out of scope.
	recipient, err := age.NewScryptRecipient(string(passphrase))
	if err != nil {
		return rep, fmt.Errorf("vault pack: scrypt recipient: %w", err)
	}

	out := dst
	var armorCloser io.WriteCloser
	if opts.Armor {
		armorCloser = armor.NewWriter(dst)
		out = armorCloser
	}

	ageW, err := age.Encrypt(out, recipient)
	if err != nil {
		if armorCloser != nil {
			_ = armorCloser.Close()
		}
		return rep, fmt.Errorf("vault pack: age encrypt: %w", err)
	}
	if _, err := io.Copy(ageW, &tarBuf); err != nil {
		_ = ageW.Close()
		if armorCloser != nil {
			_ = armorCloser.Close()
		}
		return rep, fmt.Errorf("vault pack: age write: %w", err)
	}
	if err := ageW.Close(); err != nil {
		if armorCloser != nil {
			_ = armorCloser.Close()
		}
		return rep, fmt.Errorf("vault pack: age close: %w", err)
	}
	if armorCloser != nil {
		if err := armorCloser.Close(); err != nil {
			return rep, fmt.Errorf("vault pack: armor close: %w", err)
		}
	}
	return rep, nil
}

// Unpack decrypts src (age, optionally ASCII-armored — auto-detected),
// parses the embedded tar and writes every allowlisted entry to
// destDir with mode 0600 and an atomic tmpfile+rename sequence.
//
// When opts.Overwrite is false any pre-existing file in destDir that
// would be replaced is collected into UnpackReport.Conflicts and the
// function returns [ErrOverwriteRequired] without writing a single
// file. When opts.DryRun is true no files are written regardless of
// Overwrite; the report still enumerates what would have been written.
//
// passphrase is used only for the duration of the call; the caller
// retains ownership and must wipe it afterwards.
func Unpack(src io.Reader, destDir string, passphrase []byte, opts UnpackOptions) (UnpackReport, error) {
	var rep UnpackReport
	if len(passphrase) == 0 {
		return rep, errors.New("vault unpack: empty passphrase")
	}
	if strings.TrimSpace(destDir) == "" {
		return rep, errors.New("vault unpack: empty destination directory")
	}

	br := bufio.NewReader(src)
	head, err := br.Peek(len(armor.Header))
	if err != nil && !errors.Is(err, io.EOF) {
		return rep, fmt.Errorf("vault unpack: peek: %w", err)
	}
	var in io.Reader = br
	if strings.HasPrefix(string(head), armor.Header) {
		in = armor.NewReader(br)
	}

	identity, err := age.NewScryptIdentity(string(passphrase))
	if err != nil {
		return rep, fmt.Errorf("vault unpack: scrypt identity: %w", err)
	}

	plain, err := age.Decrypt(in, identity)
	if err != nil {
		return rep, fmt.Errorf("vault unpack: age decrypt: %w", err)
	}

	type entry struct {
		name string
		body []byte
	}
	var entries []entry

	// maxEntryBytes caps a single tar entry so a maliciously crafted
	// vault cannot OOM the process. Real monujo keystore files are
	// ~1 KiB; 1 MiB is ~1000x headroom.
	const maxEntryBytes = 1 << 20

	tr := tar.NewReader(plain)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return rep, fmt.Errorf("vault unpack: tar next: %w", err)
		}
		name := hdr.Name
		if hdr.Typeflag != tar.TypeReg {
			rep.Skipped = append(rep.Skipped, name)
			_, _ = io.Copy(io.Discard, io.LimitReader(tr, maxEntryBytes))
			continue
		}
		if name == "" || name != filepath.Base(name) || strings.Contains(name, "/") || strings.Contains(name, `\`) || name == "." || name == ".." {
			return rep, fmt.Errorf("vault unpack: invalid entry name %q", name)
		}
		if !isAllowlisted(name) {
			rep.Skipped = append(rep.Skipped, name)
			_, _ = io.Copy(io.Discard, io.LimitReader(tr, maxEntryBytes))
			continue
		}
		if hdr.Size > maxEntryBytes {
			return rep, fmt.Errorf("vault unpack: entry %q too large (%d bytes > %d)", name, hdr.Size, maxEntryBytes)
		}
		body, err := io.ReadAll(io.LimitReader(tr, maxEntryBytes+1))
		if err != nil {
			return rep, fmt.Errorf("vault unpack: read %s: %w", name, err)
		}
		if len(body) > maxEntryBytes {
			return rep, fmt.Errorf("vault unpack: entry %q exceeds %d bytes", name, maxEntryBytes)
		}
		entries = append(entries, entry{name: name, body: body})
	}
	if len(entries) == 0 {
		return rep, errors.New("vault unpack: empty vault (no allowlisted entries)")
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return rep, fmt.Errorf("vault unpack: mkdir dest: %w", err)
	}

	for _, e := range entries {
		target := filepath.Join(destDir, e.name)
		if _, err := os.Stat(target); err == nil {
			rep.Conflicts = append(rep.Conflicts, e.name)
		} else if !os.IsNotExist(err) {
			return rep, fmt.Errorf("vault unpack: stat %s: %w", e.name, err)
		}
		rep.Written = append(rep.Written, e.name)
	}
	if len(rep.Conflicts) > 0 && !opts.Overwrite {
		rep.Written = nil
		return rep, ErrOverwriteRequired
	}

	if opts.DryRun {
		return rep, nil
	}

	stageDir, err := os.MkdirTemp(destDir, ".monujo-vault-stage-*")
	if err != nil {
		return rep, fmt.Errorf("vault unpack: create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	for _, e := range entries {
		staged := filepath.Join(stageDir, e.name)
		if err := writeFileAtomic(staged, e.body, 0o600); err != nil {
			return rep, fmt.Errorf("vault unpack: stage %s: %w", e.name, err)
		}
	}
	backupDir, err := os.MkdirTemp(destDir, ".monujo-vault-backup-*")
	if err != nil {
		return rep, fmt.Errorf("vault unpack: create backup dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(backupDir) }()

	type appliedEntry struct {
		name       string
		hadBackup  bool
		backupPath string
	}
	var applied []appliedEntry
	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			a := applied[i]
			target := filepath.Join(destDir, a.name)
			_ = os.Remove(target)
			if a.hadBackup {
				_ = os.Rename(a.backupPath, target)
			}
		}
	}

	for _, e := range entries {
		target := filepath.Join(destDir, e.name)
		staged := filepath.Join(stageDir, e.name)
		appliedOne := appliedEntry{name: e.name}
		if opts.Overwrite {
			if _, err := os.Stat(target); err == nil {
				appliedOne.hadBackup = true
				appliedOne.backupPath = filepath.Join(backupDir, e.name)
				if err := os.Rename(target, appliedOne.backupPath); err != nil {
					rollback()
					return rep, fmt.Errorf("vault unpack: backup %s: %w", e.name, err)
				}
			} else if !os.IsNotExist(err) {
				rollback()
				return rep, fmt.Errorf("vault unpack: stat %s: %w", e.name, err)
			}
		} else if _, err := os.Stat(target); err == nil {
			rollback()
			return rep, ErrOverwriteRequired
		} else if !os.IsNotExist(err) {
			rollback()
			return rep, fmt.Errorf("vault unpack: stat %s: %w", e.name, err)
		}
		if err := os.Rename(staged, target); err != nil {
			if appliedOne.hadBackup {
				_ = os.Rename(appliedOne.backupPath, target)
			}
			rollback()
			return rep, fmt.Errorf("vault unpack: write %s: %w", e.name, err)
		}
		applied = append(applied, appliedOne)
	}
	return rep, nil
}

// writeFileAtomic mirrors the helper used in internal/keystore: write to
// a tmpfile in the same directory, fsync-friendly close, then rename.
// Kept private here to avoid a circular dependency.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".monujo-vault-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}
