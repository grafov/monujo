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
	"bytes"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/passphrase"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// readPassTwice prompts twice for a new keystore passphrase and returns the
// confirmed bytes. The returned slice is owned by the caller and must be
// wiped via [keystore.ZeroPassphrase].
func readPassTwice(prompt string) ([]byte, error) {
	p1, err := keystore.ReadPassphrase(prompt)
	if err != nil {
		return nil, err
	}
	p2, err := keystore.ReadPassphrase("Confirm passphrase: ")
	if err != nil {
		keystore.ZeroPassphrase(p1)
		return nil, err
	}
	if string(p1) != string(p2) {
		keystore.ZeroPassphrase(p1)
		keystore.ZeroPassphrase(p2)
		return nil, errPassMismatch
	}
	keystore.ZeroPassphrase(p2)
	return p1, nil
}

var errPassMismatch = errPassMismatchType{}

type errPassMismatchType struct{}

func (errPassMismatchType) Error() string { return "passphrases do not match" }

// readUnlockPassphrase prompts on stderr and reads the keystore passphrase.
// On a TTY it delegates to [keystore.ReadPassphrase] (no echo). Otherwise it
// reads from cmd.InOrStdin() so cobra tests with SetIn and shell pipes work.
func readUnlockPassphrase(cmd *cobra.Command, prompt string) ([]byte, error) {
	rawFd := os.Stdin.Fd()
	if rawFd <= uintptr(math.MaxInt) && term.IsTerminal(int(rawFd)) {
		return keystore.ReadPassphrase(prompt)
	}
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSuffix(data, []byte("\n"))
	data = bytes.TrimSuffix(data, []byte("\r"))
	return data, nil
}

// generateFlags bundles the --generate-* passphrase options shared by
// `wallet create` and `wallet import`. When Enabled is false every other
// field is ignored and the caller falls back to interactive passphrase
// entry.
type generateFlags struct {
	Enabled     bool
	Words       int
	Dict        string
	Separator   string
	Capitalize  bool
	Digits      int
	AllowNonTTY bool
}

func registerGenerateFlags(cmd *cobra.Command, g *generateFlags) {
	cmd.Flags().BoolVar(&g.Enabled, "generate-passphrase", false,
		"Generate a random passphrase instead of prompting (prints it to stderr once)")
	cmd.Flags().IntVar(&g.Words, "generate-words", passphrase.DefaultWords,
		"Words in generated passphrase (>=4)")
	cmd.Flags().StringVar(&g.Dict, "generate-dict", string(passphrase.DefaultDict),
		"Wordlist for generated passphrase: english|english-short|esperanto-x|esperanto|russian")
	cmd.Flags().StringVar(&g.Separator, "generate-separator", passphrase.DefaultSeparator,
		"Separator between generated words")
	cmd.Flags().BoolVar(&g.Capitalize, "generate-capitalize", false,
		"Capitalize the first letter of each generated word")
	cmd.Flags().IntVar(&g.Digits, "generate-digits", 0,
		"Append N random decimal digits to the generated passphrase")
	cmd.Flags().BoolVar(&g.AllowNonTTY, "generate-allow-non-tty", false,
		"Allow generating the passphrase when stderr is not a terminal (UNSAFE)")
}

// obtainNewKeystorePassphrase returns the passphrase for a *new* keystore
// file. If gen.Enabled is true a random passphrase is generated, printed
// to w once, and returned; otherwise the user is prompted twice as
// before. In both cases the returned []byte is owned by the caller and
// must be wiped with keystore.ZeroPassphrase.
func obtainNewKeystorePassphrase(w io.Writer, gen generateFlags, prompt string) ([]byte, error) {
	if !gen.Enabled {
		return readPassTwice(prompt)
	}
	if gen.Words < 4 {
		return nil, fmt.Errorf("--generate-words must be >= 4 (got %d)", gen.Words)
	}
	if gen.Digits < 0 {
		return nil, fmt.Errorf("--generate-digits must be >= 0 (got %d)", gen.Digits)
	}
	id, err := passphrase.ParseDict(gen.Dict)
	if err != nil {
		return nil, err
	}
	if !isTerminalWriter(w) && !gen.AllowNonTTY {
		return nil, fmt.Errorf("refusing to print generated passphrase to non-TTY stderr; pass --generate-allow-non-tty to override (unsafe)")
	}
	phrase, bits, err := passphrase.Generate(passphrase.Options{
		Words:      gen.Words,
		Separator:  gen.Separator,
		Dict:       id,
		Capitalize: gen.Capitalize,
		Digits:     gen.Digits,
	})
	if err != nil {
		return nil, err
	}
	passphrase.PrintInputMethodWarning(w, id)
	fmt.Fprintln(w, "Generated passphrase (shown once, stored only as the encryption key of this keystore):")
	if _, werr := w.Write(phrase); werr != nil {
		keystore.ZeroPassphrase(phrase)
		return nil, werr
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Estimated entropy: %.1f bits (%d words from %q).\n", bits, gen.Words, id)
	if bits < passphrase.MinEntropyBits {
		fmt.Fprintf(w, "warning: estimated entropy %.1f bits is below the recommended %.0f bits.\n",
			bits, passphrase.MinEntropyBits)
	}
	fmt.Fprintln(w, "Save it in a password manager NOW; if you lose it the keystore is unrecoverable.")
	return phrase, nil
}
