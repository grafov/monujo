package cli

import (
	"os"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/spf13/cobra"
)

// newWalletCmd registers the `wallet` group: lifecycle operations on the
// local encrypted keystore (no network access, no online RPCs). The
// network-bound counterparts live under `hot` and `offline`.
func newWalletCmd() *cobra.Command {
	w := &cobra.Command{
		Use:   "wallet",
		Short: "Local encrypted keystore management (no network)",
		Long: `Manage local keystore files: list, create new wallets, import existing
private keys. None of these subcommands talk to the Tron network; the
same files are consumed both by the online "hot" commands and by the
air-gapped "offline sign" command.`,
	}
	w.AddCommand(newWalletListCmd())
	w.AddCommand(newWalletCreateCmd())
	w.AddCommand(newWalletImportCmd())
	return w
}

func newWalletListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally stored wallets (names and Tron addresses from keystore files)",
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

func newWalletCreateCmd() *cobra.Command {
	var name string
	var exportPriv bool
	var overwrite bool
	var gen generateFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new wallet locally (random key, encrypted keystore)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletOverwrite(cfg, name, overwrite); err != nil {
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
	cmd.Flags().BoolVar(&overwrite, "overwrite", false,
		"DANGEROUS: replace existing wallet file; the previous private key is permanently lost")
	registerGenerateFlags(cmd, &gen)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newWalletImportCmd() *cobra.Command {
	var name string
	var keyHex string
	var overwrite bool
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
			if err := confirmWalletOverwrite(cfg, name, overwrite); err != nil {
				return err
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
	cmd.Flags().BoolVar(&overwrite, "overwrite", false,
		"DANGEROUS: replace existing wallet file; the previous private key is permanently lost")
	registerGenerateFlags(cmd, &gen)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

var errNeedKeyOrEnv = errNeedKeyType{}

type errNeedKeyType struct{}

func (errNeedKeyType) Error() string {
	return "provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key"
}
