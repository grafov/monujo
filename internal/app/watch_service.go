// Package app wires configuration to domain services for CLI commands.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
	"github.com/grafov/monujo/internal/tron"
)

// WatchService exposes watch-only operations.
type WatchService struct {
	cfg *config.Config
	rc  *tron.ReadClient
}

// NewWatchService constructs a watch-only service.
func NewWatchService(cfg *config.Config) *WatchService {
	return &WatchService{
		cfg: cfg,
		rc:  tron.NewReadClient(cfg),
	}
}

// BalanceOutput is printed for watch balance.
type BalanceOutput struct {
	Address     string `json:"address"`
	TRXSun      int64  `json:"trx_sun"`
	TRX         string `json:"trx"`
	USDTMinimal string `json:"usdt_minimal_units"`
	USDT        string `json:"usdt"`
}

// PrintBalance fetches and prints TRX/USDT balances for addr.
func (w *WatchService) PrintBalance(ctx context.Context, addr string) error {
	b, err := w.rc.GetBalances(ctx, addr)
	if err != nil {
		return err
	}
	usdtMin := b.USDTMinimal
	if usdtMin == "" {
		usdtMin = "0"
	}
	u := new(big.Int)
	if _, ok := u.SetString(usdtMin, 10); !ok {
		u = big.NewInt(0)
	}
	out := BalanceOutput{
		Address:     b.Address,
		TRXSun:      b.Sun,
		TRX:         domain.FormatSunAsTRX(b.Sun),
		USDTMinimal: usdtMin,
		USDT:        domain.FormatMinimalUSDT(u),
	}
	switch w.cfg.Output {
	case config.OutputJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	default:
		fmt.Printf("Address: %s\n", out.Address)
		fmt.Printf("TRX:     %s (%d sun)\n", out.TRX, out.TRXSun)
		fmt.Printf("USDT:    %s (minimal units %s)\n", out.USDT, out.USDTMinimal)
		return nil
	}
}

// TxsOutput is one row for JSON listing.
type TxsOutput struct {
	TxID           string `json:"tx_id"`
	Time           string `json:"time"`
	From           string `json:"from"`
	To             string `json:"to"`
	ValueMinimal   string `json:"value_minimal"`
	Token          string `json:"token"`
	Contract       string `json:"contract"`
	Type           string `json:"type"`
}

// PrintTransfers lists USDT transfers for addr.
func (w *WatchService) PrintTransfers(ctx context.Context, addr string, limit int) error {
	rows, err := w.rc.ListUSDTTransfers(ctx, addr, limit)
	if err != nil {
		return err
	}
	if w.cfg.Output == config.OutputJSON {
		out := make([]TxsOutput, 0, len(rows))
		for _, r := range rows {
			out = append(out, TxsOutput{
				TxID:         r.TxID,
				Time:         r.BlockTimestamp.Format(time.RFC3339),
				From:         r.From,
				To:           r.To,
				ValueMinimal: r.ValueMinimal,
				Token:        r.TokenSymbol,
				Contract:     r.Contract,
				Type:         r.Type,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, r := range rows {
		fmt.Printf("%s  %s  %s -> %s  %s %s  [%s]\n",
			r.BlockTimestamp.Format(time.RFC3339),
			r.TxID,
			r.From,
			r.To,
			r.ValueMinimal,
			r.TokenSymbol,
			r.Type,
		)
	}
	return nil
}
