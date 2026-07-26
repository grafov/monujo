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
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
)

// TransactionWriter builds, signs (locally), and broadcasts Tron transactions.
type TransactionWriter interface {
	Close() error
	SendTRX(ctx context.Context, fromAddr, toAddr string, amountSun int64, priv *ecdsa.PrivateKey) (string, error)
	SendUSDT(ctx context.Context, fromAddr, toAddr string, amount *big.Int, priv *ecdsa.PrivateKey) (string, error)
	CreateTRXTransfer(ctx context.Context, from, to string, amountSun int64) (*core.Transaction, error)
	CreateTRC20Transfer(ctx context.Context, from, to, contract string, amount *big.Int) (*core.Transaction, error)
	BroadcastTransaction(ctx context.Context, tx *core.Transaction) error
	GetTransactionSignWeight(ctx context.Context, tx *core.Transaction) (*api.TransactionSignWeight, error)
}

func transactionWriterIsNil(w TransactionWriter) bool {
	return w == nil || reflect.ValueOf(w).IsNil()
}

// NewTransactionWriterFromConfig opens the configured write transport.
func NewTransactionWriterFromConfig(cfg *config.Config) (TransactionWriter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	switch cfg.WriteTransport {
	case config.WriteTransportGRPC:
		return NewGRPCWriteClientFromConfig(cfg)
	case config.WriteTransportREST:
		return NewHTTPWriteClientFromConfig(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported write transport %q", cfg.WriteTransport)
	}
}

// WarnTronGridPlaintextGRPC writes a stderr warning when using plaintext gRPC
// against a known TronGrid host.
func WarnTronGridPlaintextGRPC(w io.Writer, cfg *config.Config) {
	if w == nil || cfg == nil {
		return
	}
	if cfg.WriteTransport != config.WriteTransportGRPC || !cfg.GRPCInsecure {
		return
	}
	if !config.IsTronGridGRPC(cfg.GRPCAddress) {
		return
	}
	_, _ = fmt.Fprintf(w, "warning: TronGrid gRPC at %s uses plaintext (not TLS); prefer default HTTPS REST or use --write-transport rest\n",
		cfg.GRPCAddress)
}
