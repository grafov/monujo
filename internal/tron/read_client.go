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

package tron

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
)

// ReadClient queries public TronGrid HTTP APIs (watch-only, no keys).
type ReadClient struct {
	cfg    *config.Config
	client *http.Client
}

// NewReadClient builds a read-only HTTP client.
func NewReadClient(cfg *config.Config) *ReadClient {
	return &ReadClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type accountResponse struct {
	Data []struct {
		Balance int64               `json:"balance"`
		TRC20   []map[string]string `json:"trc20"`
	} `json:"data"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// AccountBalances holds native TRX (sun) and USDT minimal units for one address.
type AccountBalances struct {
	Address     string
	Sun         int64
	USDTMinimal string // empty if no USDT balance row
}

// GetBalances returns TRX balance in sun and USDT balance in token minimal units.
func (r *ReadClient) GetBalances(ctx context.Context, addr string) (*AccountBalances, error) {
	if err := ValidateBase58Address(addr); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/v1/accounts/%s", r.cfg.HTTPBaseURL, url.PathEscape(addr))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	r.addAPIKey(req)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trongrid accounts: HTTP %d: %s", resp.StatusCode, truncate(string(body)))
	}
	var ar accountResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("decode accounts: %w", err)
	}
	if !ar.Success && len(ar.Data) == 0 {
		return nil, fmt.Errorf("trongrid: %s", ar.Error)
	}
	out := &AccountBalances{Address: addr}
	if len(ar.Data) == 0 {
		return out, nil
	}
	a := ar.Data[0]
	out.Sun = a.Balance
	want := USDTContractForNetwork(r.cfg.Network)
	for _, m := range a.TRC20 {
		for contract, bal := range m {
			if contract == want {
				out.USDTMinimal = strings.TrimSpace(bal)
				break
			}
		}
	}
	return out, nil
}

type trc20TxResponse struct {
	Data []struct {
		TransactionID  string `json:"transaction_id"`
		BlockTimestamp int64  `json:"block_timestamp"`
		From           string `json:"from"`
		To             string `json:"to"`
		Value          string `json:"value"`
		Type           string `json:"type"`
		TokenInfo      struct {
			Symbol   string `json:"symbol"`
			Address  string `json:"address"`
			Decimals int    `json:"decimals"`
		} `json:"token_info"`
	} `json:"data"`
	Meta struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"meta"`
	Success bool `json:"success"`
}

// ListUSDTTransfers returns recent USDT TRC-20 transfers for addr (in/out).
func (r *ReadClient) ListUSDTTransfers(ctx context.Context, addr string, limit int) ([]domain.TRC20TransferRow, error) {
	if err := ValidateBase58Address(addr); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	contract := USDTContractForNetwork(r.cfg.Network)
	u, err := url.Parse(fmt.Sprintf("%s/v1/accounts/%s/transactions/trc20", r.cfg.HTTPBaseURL, url.PathEscape(addr)))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("only_confirmed", "true")
	q.Set("limit", strconv.Itoa(limit))
	q.Set("contract_address", contract)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	r.addAPIKey(req)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trongrid trc20 txs: HTTP %d: %s", resp.StatusCode, truncate(string(body)))
	}
	var tr trc20TxResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode trc20 txs: %w", err)
	}
	rows := make([]domain.TRC20TransferRow, 0, len(tr.Data))
	for _, t := range tr.Data {
		ts := time.UnixMilli(t.BlockTimestamp)
		symbol := t.TokenInfo.Symbol
		if symbol == "" {
			symbol = "USDT"
		}
		ca := t.TokenInfo.Address
		if ca == "" {
			ca = contract
		}
		rows = append(rows, domain.TRC20TransferRow{
			TxID:           t.TransactionID,
			BlockTimestamp: ts,
			From:           t.From,
			To:             t.To,
			ValueMinimal:   t.Value,
			TokenSymbol:    symbol,
			Contract:       ca,
			Type:           t.Type,
		})
	}
	return rows, nil
}

func (r *ReadClient) addAPIKey(req *http.Request) {
	if r.cfg.TronGridAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", r.cfg.TronGridAPIKey)
	}
}

const errorBodyTruncateLen = 200

func truncate(s string) string {
	if len(s) <= errorBodyTruncateLen {
		return s
	}
	return s[:errorBodyTruncateLen] + "..."
}
