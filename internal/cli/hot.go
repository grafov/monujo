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
balance lookup, sending TRX and TRC-20 USDT.

These commands talk to the Tron network. To create or import a wallet
file in the first place, use "monujo wallet" instead. For air-gapped
signing where the keystore lives on an offline machine, see
"monujo offline".

Use --addr with "hot balance" for the wallet name or address to query.
Use --from and --to with send-trx and send-usdt: --from accepts either a
keystore wallet name or a Tron base58 address that maps to a local wallet.
Balance can be queried for a raw address (no keystore needed); send
operations require a local wallet file for the private key.`,
	}
	h.AddCommand(newHotBalanceCmd())
	h.AddCommand(newHotSendTRXCmd())
	h.AddCommand(newHotSendUSDTCmd())
	return h
}

// resolveHotSender resolves --from for hot send-trx/send-usdt into the
// keystore wallet name required by HotService. Address-form input is
// accepted only when it maps to a local wallet, because sending needs the
// private key.
func resolveHotSender(keystoreDir, from string) (string, error) {
	res, err := resolveWalletFrom(keystoreDir, from, "--from")
	if err != nil {
		return "", err
	}
	if !res.KnownInKeystore {
		return "", fmt.Errorf("address %q not found in keystore %q; sending requires a local private key, pass the wallet name instead", res.Address, keystoreDir)
	}
	if res.External {
		return "", fmt.Errorf("wallet %q is external (address-only) and has no private key; use a local wallet for --from", res.Name)
	}
	return res.Name, nil
}

func newHotBalanceCmd() *cobra.Command {
	var addr string
	var force bool
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show TRX and USDT balance for a wallet (no passphrase needed)",
		Long: `Show TRX and USDT balance for the address referenced by --addr.

--addr accepts either a keystore wallet name (address is read from the
plaintext metadata of the keystore file, no decryption is performed) or
a Tron base58 address directly. In the latter case no keystore file is
required at all; the balance lookup is a read-only network operation.

If you need to verify keystore integrity (i.e. that the encrypted key
actually decrypts to the recorded address) without touching the
network, use "wallet check --addr ..." instead of a signing command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			res, err := resolveWalletFrom(cfg.KeystoreDir, addr, "--addr")
			if err != nil {
				return err
			}
			svc := app.NewHotService(cfg)
			if res.KnownInKeystore {
				if err := confirmWalletNetworkForOperation(cfg, res.Name, force); err != nil {
					return err
				}
				if res.External {
					return svc.PrintBalanceByAddress(context.Background(), res.Address)
				}
				return svc.PrintBalanceByName(context.Background(), res.Name)
			}
			return svc.PrintBalanceByAddress(context.Background(), res.Address)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "Keystore wallet name OR Tron base58 address")
	cmd.Flags().BoolVar(&force, "force", false, "Skip interactive confirmation when operation network differs from networks recorded in the wallet file")
	_ = cmd.MarkFlagRequired("addr")
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
