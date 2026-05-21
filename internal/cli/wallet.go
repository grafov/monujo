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

// rejectAddressLikeWalletName returns an error if name would later be
// misclassified as a Tron base58 address by looksLikeTronAddress (used by
// --from/--to/--addr resolution). Empty input is left to cobra MarkFlagRequired.
func rejectAddressLikeWalletName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("wallet name must not be empty")
	}
	if err := keystore.ValidateWalletName(trimmed); err != nil {
		return err
	}
	if looksLikeTronAddress(trimmed) {
		return fmt.Errorf("wallet name %q looks like a Tron base58 address; choose a non-address name to avoid --addr/--from/--to ambiguity", trimmed)
	}
	return nil
}

// newWalletCmd registers the `wallet` group: lifecycle operations on the
// local encrypted keystore (no network access, no online RPCs). The
// network-bound counterparts live under `hot` and `offline`.
func newWalletCmd() *cobra.Command {
	w := &cobra.Command{
		Use:   "wallet",
		Short: "Local encrypted keystore management (no network)",
		Long: `Manage local keystore files: list, create new encrypted wallets, import hex
private keys, and add external Tron addresses as named aliases (no private key).
None of these subcommands talk to the Tron network; the same files are consumed
both by the online "hot" commands and by the air-gapped "offline sign" command.`,
	}
	w.AddCommand(newWalletListCmd())
	w.AddCommand(newWalletCreateCmd())
	w.AddCommand(newWalletImportCmd())
	w.AddCommand(newWalletImportExternalCmd())
	w.AddCommand(newWalletCheckCmd())
	return w
}

func newWalletCheckCmd() *cobra.Command {
	var addr string
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
			res, err := resolveWalletFrom(cfg.KeystoreDir, addr, "--addr")
			if err != nil {
				return err
			}
			if !res.KnownInKeystore {
				return fmt.Errorf("address %q not found in keystore %q; wallet check needs a local wallet (see wallet list), or pass --addr as a wallet name", res.Address, cfg.KeystoreDir)
			}
			if res.External {
				return fmt.Errorf("wallet %q is external (address-only); wallet check requires a local encrypted keystore file", res.Name)
			}
			name := res.Name
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
	cmd.Flags().StringVar(&addr, "addr", "", "Keystore wallet name OR Tron base58 address listed in wallet list")
	_ = cmd.MarkFlagRequired("addr")
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
	var allowExport bool
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
			if err := rejectAddressLikeWalletName(name); err != nil {
				return err
			}
			if err := confirmWalletOverwrite(cfg, name, overwrite); err != nil {
				return err
			}
			if exportPriv && !allowExport {
				return fmt.Errorf("--export-private-key is unsafe; pass --allow-private-key-export to acknowledge the risk")
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
	cmd.Flags().BoolVar(&allowExport, "allow-private-key-export", false, "Acknowledge that --export-private-key exposes spend authority")
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
			if err := rejectAddressLikeWalletName(name); err != nil {
				return err
			}
			key := strings.TrimSpace(keyHex)
			if key == "" {
				key = strings.TrimSpace(os.Getenv("MONUJO_IMPORT_KEY"))
				if key != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: MONUJO_IMPORT_KEY can leak through process environments; use only in controlled scripts.")
				}
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --key-hex is visible in process listings and shell history; prefer an interactive air-gapped import.")
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
	cmd.Flags().StringVar(&keyHex, "key-hex", "", "64-character hex private key (unsafe: visible in process listings; optional if MONUJO_IMPORT_KEY is set)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false,
		"DANGEROUS: replace existing wallet file; the previous private key is permanently lost")
	registerGenerateFlags(cmd, &gen)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newWalletImportExternalCmd() *cobra.Command {
	var name string
	var tronAddr string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import-external",
		Short: "Import a Tron address as a named external wallet (no private key)",
		Long: `Store a Tron base58 address under a keystore file name so you can refer to it
by name in commands that accept a wallet name or address (e.g. hot balance --addr,
hot send-* --to, offline build --from/--to). The file contains no private key
material; signing commands (hot send-* --from, offline sign, multisig cosign,
wallet check) require a local encrypted wallet instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := rejectAddressLikeWalletName(name); err != nil {
				return err
			}
			if err := confirmWalletOverwrite(cfg, name, overwrite); err != nil {
				return err
			}
			svc := app.NewHotService(cfg)
			return svc.ImportExternalWallet(name, tronAddr)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Wallet name (file name in keystore)")
	cmd.Flags().StringVar(&tronAddr, "addr", "", "Tron base58 address to store as an external alias")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false,
		"DANGEROUS: replace existing wallet file; if the previous file held a private key it is permanently lost")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}

var errNeedKeyOrEnv = errNeedKeyType{}

type errNeedKeyType struct{}

func (errNeedKeyType) Error() string {
	return "provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key"
}
