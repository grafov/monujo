package cli

import (
	"encoding/json"
	"fmt"
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
	}Добавь команду check, которая по адресу во "--from" (логика подстановки как в других командах -- определяем это имя или адрес) потребует ввод парольной фразы и сообщит подходит ли она к адресу или нет.
	
	Обновить документацию, объяснив, что команда check позволяет проверить валидность пары адрес-пароль, без совершения операций в сети. Для сверки с бекапом например, не перепуталось ли чего.
	
	   адресом.
	   - Для `--from` в `hot send-*`/`hot activate`/`offline sign`
	   - Для `--from` в `hot send-*`/`hot activate`/`offline sign`/`wallet check`
		 программа ищет соответствие через `keystore.ListWallets` (по
	Here is what was implemented.
	
	Command monujo wallet check
	internal/cli/wallet.go: newWalletCheckCmd — --from via resolveWalletFrom; if the value is a base58 address but no local wallet exists, clear error; readUnlockPassphrase → keystore.LoadPrivateKey → pk.Wipe(); table line Passphrase matches keystore wallet "…" -> T… or JSON {"ok":true,"wallet":"…","address":"…"} when --output json.
	Passphrase input (tests + pipes)
	w.AddCommand(newWalletListCmd())
	w.AddCommand(newWalletCreateCmd())
	w.AddCommand(newWalletImportCmd())
	w.AddCommand(newWalletCheckCmd())
	return w
}

func newWalletCheckCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verify the keystore passphrase unlocks the wallet (no network)",
		Long: `Decrypt the named local wallet once and confirm the passphrase matches
the Tron address stored in the keystore file. No blockchain RPC is used.

Use this to validate backups (name, address, and passphrase notes) before
traveling or after restoring files. Unlike "wallet list", this command
actually loads the private key briefly in memory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			name, addr, ok, err := resolveWalletFrom(cfg.KeystoreDir, from)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("address %q not found in keystore %q; wallet check needs a local wallet (see wallet list), or pass --from as a wallet name", addr, cfg.KeystoreDir)
			}
			pass, err := readUnlockPassphrase(cmd, "Keystore passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			pk, err := keystore.LoadPrivateKey(cfg.KeystoreDir, name, pass)
			if err != nil {
				return err
			}
			defer pk.Wipe()
			if cfg.Output == config.OutputJSON {
				out := struct {
					OK      bool   `json:"ok"`
					Wallet  string `json:"wallet"`
					Address string `json:"address"`
				}{OK: true, Wallet: name, Address: pk.Address}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(&out)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Passphrase matches keystore wallet %q -> %s\n", name, pk.Address)
			return err
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Keystore wallet name OR Tron base58 address listed in wallet list")
	_ = cmd.MarkFlagRequired("from")
	return cmd
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
