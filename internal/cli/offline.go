package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/transfer"
	"github.com/spf13/cobra"
)

func newOfflineCmd() *cobra.Command {
	o := &cobra.Command{
		Use:   "offline",
		Short: "Air-gapped wallet operations: build (online) → sign (offline) → broadcast (online)",
		Long: `Three-step offline signing workflow for Tron transfers.

Run "build" online to fetch a fresh ref-block and assemble an unsigned
transaction file. Carry the file (e.g. on a USB stick) to an offline machine
that holds the encrypted keystore, run "sign" there, and bring the signed
file back online to "broadcast".

Tron transactions expire ~60 seconds after build, so the offline signing
session needs to fit within that window or the broadcast will be rejected
with TAPOS expired and you will need to rebuild.`,
	}
	o.AddCommand(newOfflineBuildCmd())
	o.AddCommand(newOfflineSignCmd())
	o.AddCommand(newOfflineBroadcastCmd())
	return o
}

// resolveHMACPassphrase returns the HMAC passphrase resolved in priority
// order: --hmac-passphrase > $MONUJO_HMAC (or the user-supplied env name) >
// nothing. The returned slice must be wiped by the caller.
func resolveHMACPassphrase(direct, envName string) []byte {
	if direct != "" {
		return []byte(direct)
	}
	name := strings.TrimSpace(envName)
	if name == "" {
		return nil
	}
	val := os.Getenv(name)
	if val == "" {
		return nil
	}
	return []byte(val)
}

func newOfflineBuildCmd() *cobra.Command {
	var (
		fromAddr string
		fromName string
		to       string
		amount   string
		kind     string
		note     string
		out      string
		hmacPass string
		hmacEnv  string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Online: build an unsigned transfer file (no keystore needed)",
		Long: `Connect to the Tron node, fetch a fresh ref-block and assemble an
unsigned transfer transaction. The output file can then be carried to an
offline machine for signing.

This command does NOT need access to the keystore: pass the cold wallet
public address with --from-address. If a keystore happens to be present on
this online machine you may use --from-name <wallet> as a convenience to
look up the address (no decryption is performed).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pass := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(pass)

			svc := app.NewOfflineService(cfg)
			_, err = svc.Build(context.Background(), app.BuildOptions{
				Kind:           kind,
				FromAddress:    fromAddr,
				FromName:       fromName,
				To:             to,
				AmountTRX:      amount,
				AmountUSDT:     amount,
				Note:           note,
				OutPath:        out,
				HMACPassphrase: pass,
				IsTerminal:     stdinIsTerminal(),
			})
			_ = yes
			return err
		},
	}
	cmd.Flags().StringVar(&fromAddr, "from-address", "", "Sender Tron base58 address (cold wallet); required if --from-name is not set")
	cmd.Flags().StringVar(&fromName, "from-name", "", "Optional: resolve sender address via keystore wallet name (no decryption)")
	cmd.Flags().StringVar(&to, "to", "", "Recipient Tron base58 address")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount: TRX decimal for --kind=trx, USDT decimal for --kind=usdt")
	cmd.Flags().StringVar(&kind, "kind", "trx", "Transfer kind: trx or usdt")
	cmd.Flags().StringVar(&note, "note", "", "Optional human-readable note carried alongside the unsigned tx")
	cmd.Flags().StringVar(&out, "out", "", "Path for the unsigned envelope JSON file (use - for stdout)")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection (or use --hmac-passphrase-env)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable (e.g. MONUJO_HMAC)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Reserved for future build-time confirmations (currently unused)")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("amount")
	_ = cmd.MarkFlagRequired("out")
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		k := strings.ToLower(strings.TrimSpace(kind))
		switch k {
		case "trx":
			kind = transfer.KindTRX
		case "usdt", "trc20", "trc20_usdt":
			kind = transfer.KindTRC20USDT
		default:
			return fmt.Errorf("unknown --kind %q (use trx or usdt)", kind)
		}
		if fromAddr == "" && fromName == "" {
			return fmt.Errorf("either --from-address or --from-name must be provided")
		}
		return nil
	}
	return cmd
}

func newOfflineSignCmd() *cobra.Command {
	var (
		in       string
		out      string
		name     string
		hmacPass string
		hmacEnv  string
		yes      bool
		force    bool
	)
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Offline: sign an unsigned transfer file using the local keystore",
		Long: `Read an unsigned transfer envelope, verify its HMAC (if the file has
one), recompute the human-readable fields from the embedded raw_data,
print them for visual inspection and ask for confirmation before signing
with the keystore wallet referenced by --name.

The wallet's address must match the unsigned payload's "from" field;
otherwise this command refuses to load the key. This protects against
accidentally signing with a different cold wallet on the offline machine.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)

			pass, err := keystore.ReadPassphrase("Keystore passphrase: ")
			if err != nil {
				return err
			}
			defer keystore.ZeroPassphrase(pass)

			svc := app.NewOfflineService(cfg)
			_, err = svc.Sign(app.SignOptions{
				InPath:         in,
				OutPath:        out,
				WalletName:     name,
				WalletPass:     pass,
				HMACPassphrase: hmac,
				Yes:            yes,
				IsTerminal:     stdinIsTerminal(),
				Force:          force,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "Path to the unsigned envelope JSON file")
	cmd.Flags().StringVar(&out, "out", "", "Path for the signed envelope JSON file (use - for stdout)")
	cmd.Flags().StringVar(&name, "name", "", "Keystore wallet name (its address must match the unsigned payload's from)")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt (for scripts)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip wallet network mismatch check")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.MarkFlagRequired("out")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newOfflineBroadcastCmd() *cobra.Command {
	var (
		in       string
		hmacPass string
		hmacEnv  string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "broadcast",
		Short: "Online: broadcast a previously signed transfer file (no keystore needed)",
		Long: `Read a signed transfer envelope, verify its HMAC (if any), recompute
the human-readable fields from raw_data, ask for confirmation, then send
to the configured Tron node via gRPC.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)

			svc := app.NewOfflineService(cfg)
			_, err = svc.Broadcast(context.Background(), app.BroadcastOptions{
				InPath:         in,
				HMACPassphrase: hmac,
				Yes:            yes,
				IsTerminal:     stdinIsTerminal(),
			})
			return err
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "Path to the signed envelope JSON file")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt (for scripts)")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}
