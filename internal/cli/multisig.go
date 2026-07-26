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
	"context"
	"fmt"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/spf13/cobra"
)

func newMultisigCmd() *cobra.Command {
	m := &cobra.Command{
		Use:   "multisig",
		Short: "Cosign/broadcast only (no propose/setup): participate in Tron multisig",
		Long: `Cosign and broadcast only — monujo does NOT create/propose multisig
transactions or reconfigure account permissions. Setup those elsewhere
(TronWeb, TronLink, TronScan, or any other Tron tool).

This command group only:

  - appends a signature to an existing transaction (cosign, offline-safe),
  - inspects the current signature weight against the permission threshold
    (info, online),
  - and broadcasts a transaction whose cumulative weight has reached the
    threshold (broadcast, online).

Files can be exchanged in two formats:

  - canonical TRON JSON (txID, raw_data, raw_data_hex, signature[]), the
    same shape tronWeb.trx.multiSign and /wallet/broadcasttransaction use,
  - monujo sealed envelope (kind=multisig_transfer) with optional HMAC and
    a SHA-256 fingerprint for visual verification when carrying files
    between machines.

Use --format to force one or the other on input and output. The default
(auto) detects the input format by looking at the top-level JSON keys and
emits the output in the same format.`,
	}
	m.AddCommand(newMultisigCosignCmd())
	m.AddCommand(newMultisigBroadcastCmd())
	m.AddCommand(newMultisigInfoCmd())
	return m
}

func parseMultisigFormat(s string) (app.MultisigFormat, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "", "auto":
		return app.MultisigFormatAuto, nil
	case "tron-json", "tron_json", "tronjson", "json":
		return app.MultisigFormatTronJSON, nil
	case "monujo", "envelope", "sealed":
		return app.MultisigFormatMonujo, nil
	default:
		return "", fmt.Errorf("unknown --format %q (use auto|tron-json|monujo)", s)
	}
}

func newMultisigCosignCmd() *cobra.Command {
	var (
		in        string
		out       string
		addr      string
		format    string
		outFormat string
		hmacPass  string
		hmacEnv   string
		note      string
		yes       bool
		force     bool
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "cosign",
		Short: "Offline: append a signature to an existing multisig transaction",
		Long: `Read an incoming multisig transaction (Tron JSON or monujo sealed
envelope), verify that the declared human-readable fields agree with the
embedded raw_data, ask for visual confirmation, and append one signature
produced by the local keystore wallet selected with --addr.

The output file is written in the same format as the input by default
(controllable with --format). Refuses to add a second signature from the
same key unless --force is passed.`,
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
				return fmt.Errorf("address %q not found in keystore %q; multisig cosign needs the private key, pass the wallet name instead", res.Address, cfg.KeystoreDir)
			}
			if res.External {
				return fmt.Errorf("wallet %q is external (address-only); multisig cosign needs a local encrypted keystore file", res.Name)
			}
			name := res.Name
			fm, err := parseMultisigFormat(format)
			if err != nil {
				return err
			}
			fmOut, err := parseMultisigFormat(outFormat)
			if err != nil {
				return err
			}
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

			svc := app.NewMultisigService(cfg)
			_, err = svc.Cosign(app.CosignOptions{
				InPath:         in,
				OutPath:        out,
				Format:         fm,
				OutFormat:      fmOut,
				WalletName:     name,
				ReadWalletPass: func() ([]byte, error) { return keystore.ReadPassphrase("Keystore passphrase: ") },
				Overwrite:      overwrite,
				HMACPassphrase: hmac,
				Yes:            yes,
				IsTerminal:     stdinIsTerminal(),
				Force:          force,
				Note:           note,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "Path to the incoming multisig transaction file")
	cmd.Flags().StringVar(&out, "out", "", "Path for the updated transaction file (use - for stdout)")
	cmd.Flags().StringVar(&addr, "addr", "", "Keystore wallet name OR Tron base58 address (must map to a local wallet)")
	cmd.Flags().StringVar(&format, "format", "auto", "Input format (and default output): auto|tron-json|monujo")
	cmd.Flags().StringVar(&outFormat, "out-format", "auto", "Override output format: auto|tron-json|monujo (default: same as --format)")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for monujo-envelope HMAC integrity (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().StringVar(&note, "note", "", "Optional note carried with a monujo envelope output")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt (for scripts)")
	cmd.Flags().BoolVar(&force, "force", false, "Override safety refusals (already-signed, network-mismatch)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace --out if it already exists")
	_ = cmd.MarkFlagRequired("in")
	_ = cmd.MarkFlagRequired("out")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}

func newMultisigBroadcastCmd() *cobra.Command {
	var (
		in       string
		format   string
		hmacPass string
		hmacEnv  string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "broadcast",
		Short: "Online: broadcast a fully-signed multisig transaction",
		Long: `Read a multisig transaction (Tron JSON or monujo sealed envelope),
query the node for the current sign weight, and broadcast it only when
the accumulated weight has reached the permission threshold.

If the threshold has not been reached the broadcast is refused with a
human-readable diff so you know which cosigners are still missing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			fm, err := parseMultisigFormat(format)
			if err != nil {
				return err
			}
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

			svc := app.NewMultisigService(cfg)
			_, err = svc.BroadcastMultisig(context.Background(), app.BroadcastMultisigOptions{
				InPath:         in,
				Format:         fm,
				HMACPassphrase: hmac,
				Yes:            yes,
				IsTerminal:     stdinIsTerminal(),
			})
			return err
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "Path to the multisig transaction file")
	cmd.Flags().StringVar(&format, "format", "auto", "Input format: auto|tron-json|monujo")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for monujo-envelope HMAC integrity (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive confirmation prompt")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}

func newMultisigInfoCmd() *cobra.Command {
	var (
		in       string
		format   string
		hmacPass string
		hmacEnv  string
	)
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Online: show current sign weight, threshold and approved signers",
		Long: `Read a multisig transaction and call the node's GetTransactionSignWeight
to show the permission id, threshold, current cumulative weight, the list
of addresses whose signatures were accepted, and the list of keys from
the permission that have not signed yet.

No broadcast and no signing happen.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			fm, err := parseMultisigFormat(format)
			if err != nil {
				return err
			}
			hmac := resolveHMACPassphrase(hmacPass, hmacEnv)
			defer keystore.ZeroPassphrase(hmac)
			if hmacPass != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --hmac-passphrase is visible in process listings and shell history; prefer --hmac-passphrase-env in controlled scripts.")
			}

			svc := app.NewMultisigService(cfg)
			_, err = svc.Info(context.Background(), app.InfoOptions{
				InPath:         in,
				Format:         fm,
				HMACPassphrase: hmac,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "Path to the multisig transaction file")
	cmd.Flags().StringVar(&format, "format", "auto", "Input format: auto|tron-json|monujo")
	cmd.Flags().StringVar(&hmacPass, "hmac-passphrase", "", "Passphrase for monujo-envelope HMAC integrity (unsafe: visible in process listings)")
	cmd.Flags().StringVar(&hmacEnv, "hmac-passphrase-env", "", "Read HMAC passphrase from this environment variable")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}
