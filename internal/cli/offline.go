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
		from      string
		to        string
		amount    string
		kind      string
		note      string
		out       string
		hmacPass  string
		hmacEnv   string
		yes       bool
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Online: build an unsigned transfer file (no keystore needed)",
		Long: `Connect to the Tron node, fetch a fresh ref-block and assemble an
unsigned transfer transaction. The output file can then be carried to an
offline machine for signing.

This command does NOT strictly need access to the keystore. The --from
flag accepts either a Tron base58 address (T..., 34 chars) or, if a
keystore happens to be present on this online machine, a wallet name
from it (no decryption is performed). The interpretation is automatic:
anything that parses as a base58 address is treated as one, anything
else as a name.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			resFrom, err := resolveWalletFrom(cfg.KeystoreDir, from, "--from")
			if err != nil {
				return err
			}
			addr := resFrom.Address
			toAddr, err := resolveRecipient(cfg.KeystoreDir, to)
			if err != nil {
				return err
			}
			pass := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(pass)
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

			opts := app.BuildOptions{
				Kind:           kind,
				FromAddress:    addr,
				To:             toAddr,
				AmountTRX:      amount,
				AmountUSDT:     amount,
				Note:           note,
				OutPath:        out,
				Overwrite:      overwrite,
				HMACPassphrase: pass,
				IsTerminal:     stdinIsTerminal(),
			}

			svc := app.NewOfflineService(cfg)
			_, err = svc.Build(context.Background(), opts)
			_ = yes
			return err
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Sender: Tron base58 address (cold wallet) OR keystore wallet name")
	cmd.Flags().StringVar(&to, "to", "", "Recipient: Tron base58 address OR keystore wallet name (resolved to its address)")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount: TRX decimal for --kind=trx, USDT decimal for --kind=usdt")
	cmd.Flags().StringVar(&kind, "kind", "trx", "Transfer kind: trx or usdt")
	cmd.Flags().StringVar(&note, "note", "", "Optional human-readable note carried alongside the unsigned tx")
	cmd.Flags().StringVar(&out, "out", "", "Path for the unsigned envelope JSON file (use - for stdout)")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable (e.g. MONUJO_HMAC)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Reserved for future build-time confirmations (currently unused)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace --out if it already exists")
	_ = cmd.MarkFlagRequired("from")
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
		if strings.TrimSpace(from) == "" {
			return fmt.Errorf("--from must be provided (Tron base58 address or keystore wallet name)")
		}
		return nil
	}
	return cmd
}

func newOfflineSignCmd() *cobra.Command {
	var (
		in        string
		out       string
		addr      string
		hmacPass  string
		hmacEnv   string
		yes       bool
		force     bool
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Offline: sign an unsigned transfer file using the local keystore",
		Long: `Read an unsigned transfer envelope, verify its HMAC (if the file has
one), recompute the human-readable fields from the embedded raw_data,
print them for visual inspection and ask for confirmation before signing
with the keystore wallet referenced by --addr.

The --addr flag accepts either a keystore wallet name or a Tron base58
address. Address input is resolved against the local keystore (via the
plaintext address metadata) into the corresponding wallet name; an
address that is not present in the keystore is rejected because offline
sign needs the private key.

The wallet's address must match the unsigned payload's "from" field;
otherwise this command refuses to load the key. This protects against
accidentally signing with a different cold wallet on the offline machine.`,
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
				return fmt.Errorf("address %q not found in keystore %q; offline sign needs the private key, pass the wallet name instead", res.Address, cfg.KeystoreDir)
			}
			if res.External {
				return fmt.Errorf("wallet %q is external (address-only); offline sign needs a local encrypted keystore file", res.Name)
			}
			name := res.Name
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

			svc := app.NewOfflineService(cfg)
			_, err = svc.Sign(app.SignOptions{
				InPath:         in,
				OutPath:        out,
				WalletName:     name,
				ReadWalletPass: func() ([]byte, error) { return keystore.ReadPassphrase("Keystore passphrase: ") },
				Overwrite:      overwrite,
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
	cmd.Flags().StringVar(&addr, "addr", "", "Keystore wallet name OR Tron base58 address (must map to a local wallet)")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt (for scripts)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip wallet network mismatch check")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace --out if it already exists")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.MarkFlagRequired("out")
	_ = cmd.MarkFlagRequired("addr")
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
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

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
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for HMAC integrity protection (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt (for scripts)")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}
