package cli

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/passphrase"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newToolCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tool",
		Short: "Miscellaneous helpers (passphrase generator, ...)",
	}
	c.AddCommand(newToolGenpassCmd())
	c.AddCommand(newToolVaultCmd())
	return c
}

func newToolGenpassCmd() *cobra.Command {
	var (
		words       int
		separator   string
		dict        string
		capitalize  bool
		digits      int
		allowNonTTY bool
	)
	cmd := &cobra.Command{
		Use:   "genpass",
		Short: "Generate a random passphrase using crypto/rand and an embedded wordlist",
		Long: `Generate a random passphrase suitable for the local keystore.

The passphrase is built from an embedded wordlist using crypto/rand with a
uniform, bias-free selection. The result is printed to standard error
exactly once; it is never written to stdout or to a file. The passphrase
is NFKC-normalized so that the bytes you see are the bytes that will go
into Argon2id later.

Dictionaries:
  english         EFF Long Wordlist (7776 words, ASCII, default).
  english-short   EFF Short Wordlist (1296 words, ASCII).
  esperanto-x     Esperanto x-system (cx/gx/hx/jx/sx/ux, ASCII).
  esperanto       Esperanto with diacritics (ĉĝĥĵŝŭ, UTF-8).
  russian         Russian Cyrillic including ё and й (UTF-8).

WARNING: the passphrase is shown ONCE. If you lose it, the wallet is
unrecoverable by design — see docs/06-keystore.md
§"Восстановление доступа при утере пароля".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenpass(cmd.ErrOrStderr(), genpassArgs{
				words:       words,
				separator:   separator,
				dict:        dict,
				capitalize:  capitalize,
				digits:      digits,
				allowNonTTY: allowNonTTY,
			})
		},
	}
	cmd.Flags().IntVar(&words, "words", passphrase.DefaultWords, "number of words (>= 4 recommended)")
	cmd.Flags().StringVar(&separator, "separator", passphrase.DefaultSeparator, "separator between words (non-empty)")
	cmd.Flags().StringVar(&dict, "dict", string(passphrase.DefaultDict), "wordlist: english|english-short|esperanto-x|esperanto|russian")
	cmd.Flags().BoolVar(&capitalize, "capitalize", false, "capitalize the first letter of each word")
	cmd.Flags().IntVar(&digits, "digits", 0, "append N random decimal digits as a final group (default 0)")
	cmd.Flags().BoolVar(&allowNonTTY, "allow-non-tty", false, "allow printing the passphrase when stderr is not a terminal (UNSAFE)")
	return cmd
}

type genpassArgs struct {
	words       int
	separator   string
	dict        string
	capitalize  bool
	digits      int
	allowNonTTY bool
}

func runGenpass(w io.Writer, a genpassArgs) error {
	if a.words < 4 {
		return fmt.Errorf("--words must be >= 4 (got %d); shorter passphrases are too weak", a.words)
	}
	if a.digits < 0 {
		return fmt.Errorf("--digits must be >= 0 (got %d)", a.digits)
	}
	id, err := passphrase.ParseDict(a.dict)
	if err != nil {
		return err
	}
	if !isTerminalWriter(w) && !a.allowNonTTY {
		return fmt.Errorf("refusing to print passphrase to non-TTY stderr; pass --allow-non-tty to override (unsafe)")
	}

	phrase, bits, err := passphrase.Generate(passphrase.Options{
		Words:      a.words,
		Separator:  a.separator,
		Dict:       id,
		Capitalize: a.capitalize,
		Digits:     a.digits,
	})
	if err != nil {
		return err
	}
	defer keystore.ZeroPassphrase(phrase)

	passphrase.PrintInputMethodWarning(w, id)
	// Not a bug: passphrase specially enclosed by two empty strings for better visibility
	// on the screen.
	fmt.Fprint(w, "Generated passphrase (shown once, not stored):\n\n")
	if _, err := w.Write(phrase); err != nil {
		return err
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "\nEstimated entropy: %.1f bits (%d words from %q).\n", bits, a.words, id)
	if bits < passphrase.MinEntropyBits {
		fmt.Fprintf(w, "warning: estimated entropy %.1f bits is below the recommended %.0f bits.\n",
			bits, passphrase.MinEntropyBits)
	}
	fmt.Fprintln(w, "Save it in a password manager now; loss of the passphrase is NOT recoverable.")
	return nil
}

// isTerminalWriter reports whether w is a TTY-backed *os.File. Non-file
// writers (e.g. bytes.Buffer in tests) return false; callers must opt in
// with --allow-non-tty to print there.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	if fd > uintptr(math.MaxInt) {
		return false
	}
	return term.IsTerminal(int(fd))
}
