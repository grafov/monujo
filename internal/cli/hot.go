package cli

import (
	"context"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/spf13/cobra"
)

func newHotCmd() *cobra.Command {
	h := &cobra.Command{
		Use:   "hot",
		Short: "Hot wallet operations (encrypted keystore, online signing)",
	}
	h.AddCommand(newHotImportCmd())
	h.AddCommand(newHotCreateCmd())
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
			pass, err := readPassTwice("New keystore passphrase: ")
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
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new wallet locally (random key, encrypted keystore)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pass, err := readPassTwice("New keystore passphrase: ")
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
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHotActivateCmd() *cobra.Command {
	var fromName, to, amount string
	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Send TRX to a new address to activate it on-chain (first incoming transfer)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
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
	return cmd
}

func newHotBalanceCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show TRX and USDT balance for a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
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
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHotSendTRXCmd() *cobra.Command {
	var name, to, amount string
	cmd := &cobra.Command{
		Use:   "send-trx",
		Short: "Send TRX from a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
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
	return cmd
}

func newHotSendUSDTCmd() *cobra.Command {
	var name, to, amount string
	cmd := &cobra.Command{
		Use:   "send-usdt",
		Short: "Send USDT (TRC-20) from a named hot wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
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
	return cmd
}
