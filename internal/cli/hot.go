package cli

import (
	"context"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/spf13/cobra"
)

// newHotCmd registers the `hot` group: online operations against the Tron
// network using a wallet from the local encrypted keystore. Lifecycle
// operations on keystore files (list/create/import) live under `wallet`.
func newHotCmd() *cobra.Command {
	h := &cobra.Command{
		Use:   "hot",
		Short: "Online wallet operations (encrypted keystore, online signing)",
		Long: `Online operations using a wallet from the local encrypted keystore:
balance lookup, account activation, sending TRX and TRC-20 USDT.

These commands talk to the Tron network. To create or import a wallet
file in the first place, use "monujo wallet" instead. For air-gapped
signing where the keystore lives on an offline machine, see
"monujo offline".`,
	}
	h.AddCommand(newHotBalanceCmd())
	h.AddCommand(newHotActivateCmd())
	h.AddCommand(newHotSendTRXCmd())
	h.AddCommand(newHotSendUSDTCmd())
	return h
}

func newHotActivateCmd() *cobra.Command {
	var fromName, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Send TRX to a new address to activate it on-chain (alias for `hot send-trx`)",
		Long: `Send TRX to a new address so it appears on-chain. In Tron an account
becomes visible to read APIs only after its first incoming transfer;
"activate" is the conventional name for that funding TRX.

This command is a UX alias for:

    monujo hot send-trx --from <funding> --to <new-address> --amount <trx>

It exists for readability of scripts and tutorials; the underlying
behavior, fees and confirmation prompts are identical to send-trx.`,
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
		Short: "Show TRX and USDT balance for a named wallet (no passphrase needed)",
		Long: `Show TRX and USDT balance for the address recorded in the keystore
file referenced by --from. The address is read from the plaintext
metadata of the keystore (no decryption is performed), so this command
does NOT prompt for the keystore passphrase.

If you need to verify keystore integrity (i.e. that the encrypted key
actually decrypts to the recorded address), perform any signing
operation that loads the key, e.g. "hot send-trx".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
				return err
			}
			svc := app.NewHotService(cfg)
			return svc.PrintBalanceByName(context.Background(), name)
		},
	}
	cmd.Flags().StringVar(&name, "from", "", "Wallet name in the keystore")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func newHotSendTRXCmd() *cobra.Command {
	var name, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "send-trx",
		Short: "Send TRX from a named wallet",
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
	cmd.Flags().StringVar(&name, "from", "", "Sender wallet name in the keystore")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron address")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount in TRX (decimal, e.g. 1.5)")
	_ = cmd.MarkFlagRequired("from")
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
		Short: "Send USDT (TRC-20) from a named wallet",
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
	cmd.Flags().StringVar(&name, "from", "", "Sender wallet name in the keystore")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron address")
	cmd.Flags().StringVar(&amount, "amount", "", "USDT amount (decimal, e.g. 10.25)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}
