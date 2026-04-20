package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/passphrase"
	"github.com/spf13/cobra"
)

func newHotCmd() *cobra.Command {
	h := &cobra.Command{
		Use:   "hot",
		Short: "Hot wallet operations (encrypted keystore, online signing)",
	}
	h.AddCommand(newHotImportCmd())
	h.AddCommand(newHotCreateCmd())
	h.AddCommand(newHotListCmd())
	h.AddCommand(newHotBalanceCmd())
	h.AddCommand(newHotActivateCmd())
	h.AddCommand(newHotSendTRXCmd())
	h.AddCommand(newHotSendUSDTCmd())
	return h
}

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

func newHotImportCmd() *cobra.Command {
	var name string
	var keyHex string
	var gen generateFlags
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a hex private key into the encrypted keystore",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			key := strings.TrimSpace(keyHex)
			if key == "" {
				key = strings.TrimSpace(os.Getenv("MONUJO_IMPORT_KEY"))
			}
			if key == "" {
				return errNeedKeyOrEnv
			}
			pass, err := obtainNewKeystorePassphrase(cmd.ErrOrStderr(), gen, "New keystore passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.ImportWallet(name, key, pass)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name (file name in keystore)")
	cmd.Flags().StringVar(&keyHex, "key-hex", "", "64-character hex private key (optional if MONUJO_IMPORT_KEY is set)")
	registerGenerateFlags(cmd, &gen)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

var errNeedKeyOrEnv = errNeedKeyType{}

type errNeedKeyType struct{}

func (errNeedKeyType) Error() string {
	return "provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key"
}

func newHotCreateCmd() *cobra.Command {
	var name string
	var exportPriv bool
	var gen generateFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new wallet locally (random key, encrypted keystore)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pass, err := obtainNewKeystorePassphrase(cmd.ErrOrStderr(), gen, "New keystore passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			includeJSON := exportPriv && cfg.Output == config.OutputJSON
			return svc.CreateWallet(name, pass, exportPriv, includeJSON)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name (file name in keystore)")
	cmd.Flags().BoolVar(&exportPriv, "export-private-key", false, "Print raw hex private key once (unsafe)")
	registerGenerateFlags(cmd, &gen)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHotListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally stored hot wallets (names and Tron addresses from keystore files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			svc := app.NewHotService(cfg)
			return svc.ListWallets()
		},
	}
}

func newHotActivateCmd() *cobra.Command {
	var fromName, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Send TRX to a new address to activate it on-chain (first incoming transfer)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, fromName, force); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase("Passphrase (funding wallet): ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.ActivateAccount(context.Background(), fromName, to, amount, pass)
		},
	}
	cmd.Flags().StringVar(&fromName, "from", "", "Funding wallet name (must have TRX on the selected network)")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron address to activate")
	cmd.Flags().StringVar(&amount, "amount", "", "TRX amount to send (decimal, e.g. 1)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}

func newHotBalanceCmd() *cobra.Command {
	var name string
	var force bool
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show TRX and USDT balance for a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase("Passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.PrintBalance(context.Background(), name, pass)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHotSendTRXCmd() *cobra.Command {
	var name, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "send-trx",
		Short: "Send TRX from a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase("Passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.SendTRX(context.Background(), name, to, amount, pass)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron address")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount in TRX (decimal, e.g. 1.5)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}

func newHotSendUSDTCmd() *cobra.Command {
	var name, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "send-usdt",
		Short: "Send USDT (TRC-20) from a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase("Passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.SendUSDT(context.Background(), name, to, amount, pass)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron address")
	cmd.Flags().StringVar(&amount, "amount", "", "USDT amount (decimal, e.g. 10.25)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}

// generateFlags bundles the --generate-* passphrase options shared by
// `hot create` and `hot import`. When Enabled is false every other field
// is ignored and the caller falls back to interactive passphrase entry.
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
