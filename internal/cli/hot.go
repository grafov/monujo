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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
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

// confirmHotSend prints a short preview and asks [y/N] before passphrase.
// With yes=true (scripts), confirmation is skipped. Non-TTY without --yes fails.
func confirmHotSend(cfg *config.Config, kind, fromName, toAddr, amount string, yes bool) error {
	fmt.Fprintf(os.Stderr, "\n=== hot %s ===\n", kind)
	fmt.Fprintf(os.Stderr, "  network: %s\n", cfg.Network)
	fmt.Fprintf(os.Stderr, "  from:    %s\n", fromName)
	fmt.Fprintf(os.Stderr, "  to:      %s\n", toAddr)
	fmt.Fprintf(os.Stderr, "  amount:  %s\n", amount)
	if yes {
		return nil
	}
	if !stdinIsTerminal() {
		return fmt.Errorf("aborted: refused to act without --yes on a non-interactive stdin")
	}
	fmt.Fprintf(os.Stderr, "Continue? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if isAffirmative(line) {
		return nil
	}
	return fmt.Errorf("aborted by user")
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
	var force, yes bool
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
			if err := confirmHotSend(cfg, "send-trx", name, toAddr, amount+" TRX", yes); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase(fmt.Sprintf("Passphrase for %s: ", name))
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
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive send confirmation prompt (for scripts)")
	return cmd
}

func newHotSendUSDTCmd() *cobra.Command {
	var from, to, amount string
	var force, yes bool
	cmd := &cobra.Command{
		Use:   "send-usdt",
		Short: "Send USDT (TRC-20); on Shasta this is test USDT, not Tether",
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
			if err := confirmHotSend(cfg, "send-usdt", name, toAddr, amount+" USDT", yes); err != nil {
				return err
			}
			pass, err := keystore.ReadPassphrase(fmt.Sprintf("Passphrase for %s: ", name))
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
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive send confirmation prompt (for scripts)")
	return cmd
}
