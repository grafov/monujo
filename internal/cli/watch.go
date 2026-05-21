package cli

import (
	"context"
	"fmt"

	"github.com/grafov/monujo/internal/app"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	w := &cobra.Command{
		Use:   "watch",
		Short: "Watch-only queries (public address, no private keys)",
	}
	w.AddCommand(newWatchBalanceCmd())
	w.AddCommand(newWatchTxsCmd())
	w.AddCommand(newWatchPermissionsCmd())
	return w
}

func newWatchPermissionsCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: "Show owner/witness/active permissions for a Tron account",
		Long: `Read the owner, witness and active permissions for a Tron account
via the TronGrid HTTP API. Each permission is printed with its id, name,
threshold, allowed contract types (operations bitmap decoded by name),
and weighted keys. Keys whose address matches a local keystore wallet are
annotated with "[local: <wallet-name>]" so you can tell which signatures
you can contribute yourself.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			svc := app.NewWatchService(cfg)
			return svc.PrintPermissions(context.Background(), addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "Tron base58 address")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}

func newWatchBalanceCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show TRX and USDT balance for a Tron address",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			svc := app.NewWatchService(cfg)
			return svc.PrintBalance(context.Background(), addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "Tron base58 address")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}

func newWatchTxsCmd() *cobra.Command {
	var addr string
	var limit int
	cmd := &cobra.Command{
		Use:   "txs",
		Short: "List recent USDT (TRC-20) transfers for an address",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			svc := app.NewWatchService(cfg)
			return svc.PrintTransfers(context.Background(), addr, limit)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 200 {
				return fmt.Errorf("--limit must be between 1 and 200")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "Tron base58 address")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max transactions to fetch (1-200)")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}
