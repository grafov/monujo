package cli

import (
	"context"
	"fmt"

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
"monujo offline".

The --from flag accepts either a keystore wallet name or a Tron base58
address. Balance can be served from a raw address (no keystore needed);
send operations still require a local wallet file for the private key,
so passing an address that is not present in the keystore is rejected.`,
	}
	h.AddCommand(newHotBalanceCmd())
	h.AddCommand(newHotActivateCmd())
	h.AddCommand(newHotSendTRXCmd())
	h.AddCommand(newHotSendUSDTCmd())
	return h
}

// resolveHotSender resolves --from for hot send-like commands
// (send-trx/send-usdt/activate) into the keystore wallet name required by
// HotService. Address-form input is accepted only when it maps to a local
// wallet, because sending needs the private key.
func resolveHotSender(keystoreDir, from string) (string, error) {
	name, addr, ok, err := resolveWalletFrom(keystoreDir, from)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("address %q not found in keystore %q; sending requires a local private key, pass the wallet name instead", addr, keystoreDir)
	}
	return name, nil
}

func newHotActivateCmd() *cobra.Command {
	var from, to, amount string
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
			name, err := resolveHotSender(cfg.KeystoreDir, from)
			if err != nil {
				return err
			}
			toAddr, err := resolveRecipient(cfg.KeystoreDir, to)
			if err != nil {
				return err
			}
			if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase("Passphrase (funding wallet): ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)
			svc := app.NewHotService(cfg)
			return svc.ActivateAccount(context.Background(), name, toAddr, amount, pass)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Funding wallet: keystore name OR Tron base58 address that maps to a local wallet")
	cmd.Flags().StringVar(&to, "to", "", "Recipient: Tron base58 address OR keystore wallet name (resolved to its address)")
	cmd.Flags().StringVar(&amount, "amount", "", "TRX amount to send (decimal, e.g. 1)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}

func newHotBalanceCmd() *cobra.Command {
	var from string
	var force bool
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show TRX and USDT balance for a wallet (no passphrase needed)",
		Long: `Show TRX and USDT balance for the address referenced by --from.

--from accepts either a keystore wallet name (address is read from the
plaintext metadata of the keystore file, no decryption is performed) or
a Tron base58 address directly. In the latter case no keystore file is
required at all; the balance lookup is a read-only network operation.

If you need to verify keystore integrity (i.e. that the encrypted key
actually decrypts to the recorded address) without touching the
network, use "wallet check --from ..." instead of a signing command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			name, addr, ok, err := resolveWalletFrom(cfg.KeystoreDir, from)
			if err != nil {
				return err
			}
			svc := app.NewHotService(cfg)
			if ok {
				if err := confirmWalletNetworkForOperation(cfg, name, force); err != nil {
					return err
				}
				return svc.PrintBalanceByName(context.Background(), name)
			}
			return svc.PrintBalanceByAddress(context.Background(), addr)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Keystore wallet name OR Tron base58 address")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func newHotSendTRXCmd() *cobra.Command {
	var from, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "send-trx",
		Short: "Send TRX from a wallet (keystore name or base58 address)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			name, err := resolveHotSender(cfg.KeystoreDir, from)
			if err != nil {
				return err
			}
			toAddr, err := resolveRecipient(cfg.KeystoreDir, to)
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
			return svc.SendTRX(context.Background(), name, toAddr, amount, pass)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Sender: keystore wallet name OR Tron base58 address that maps to a local wallet")
	cmd.Flags().StringVar(&to, "to", "", "Recipient: Tron base58 address OR keystore wallet name (resolved to its address)")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount in TRX (decimal, e.g. 1.5)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}

func newHotSendUSDTCmd() *cobra.Command {
	var from, to, amount string
	var force bool
	cmd := &cobra.Command{
		Use:   "send-usdt",
		Short: "Send USDT (TRC-20) from a wallet (keystore name or base58 address)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			name, err := resolveHotSender(cfg.KeystoreDir, from)
			if err != nil {
				return err
			}
			toAddr, err := resolveRecipient(cfg.KeystoreDir, to)
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
			return svc.SendUSDT(context.Background(), name, toAddr, amount, pass)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Sender: keystore wallet name OR Tron base58 address that maps to a local wallet")
	cmd.Flags().StringVar(&to, "to", "", "Recipient: Tron base58 address OR keystore wallet name (resolved to its address)")
	cmd.Flags().StringVar(&amount, "amount", "", "USDT amount (decimal, e.g. 10.25)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	return cmd
}
