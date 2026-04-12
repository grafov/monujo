package cli

import (
	"context"

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
	return w
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
	}
	cmd.Flags().StringVar(&addr, "addr", "", "Tron base58 address")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max transactions to fetch (1-200)")
	_ = cmd.MarkFlagRequired("addr")
	return cmd
}
