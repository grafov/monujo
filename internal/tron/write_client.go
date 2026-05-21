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
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"math/big"
	"time"

	gclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// WriteClient signs and broadcasts transactions via Tron gRPC (hot wallet).
type WriteClient struct {
	cfg *config.Config
	gc  *gclient.GrpcClient
}

// NewWriteClientFromConfig connects to Tron full node gRPC and returns a ready client.
func NewWriteClientFromConfig(cfg *config.Config) (*WriteClient, error) {
	gc := gclient.NewGrpcClientWithTimeout(cfg.GRPCAddress, 60*time.Second)
	creds := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	if cfg.GRPCInsecure {
		creds = insecure.NewCredentials()
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}
	if err := gc.Start(opts...); err != nil {
		return nil, err
	}
	if cfg.TronGridAPIKey != "" {
		if err := gc.SetAPIKey(cfg.TronGridAPIKey); err != nil {
			_ = gc.Conn.Close()
			return nil, err
		}
	}
	return &WriteClient{cfg: cfg, gc: gc}, nil
}

// Close releases the gRPC connection.
func (w *WriteClient) Close() error {
	if w.gc != nil && w.gc.Conn != nil {
		return w.gc.Conn.Close()
	}
	return nil
}

// SendTRX transfers native TRX (amount in sun).
func (w *WriteClient) SendTRX(ctx context.Context, fromAddr, toAddr string, amountSun int64, priv *ecdsa.PrivateKey) (string, error) {
	if err := w.bindContext(ctx); err != nil {
		return "", err
	}
	txe, err := w.gc.Transfer(fromAddr, toAddr, amountSun)
	if err != nil {
		return "", err
	}
	return w.signAndBroadcast(txe.GetTransaction(), priv)
}

// SendUSDT transfers USDT on the configured network using the canonical USDT (or test) contract.
func (w *WriteClient) SendUSDT(ctx context.Context, fromAddr, toAddr string, amount *big.Int, priv *ecdsa.PrivateKey) (string, error) {
	if err := w.bindContext(ctx); err != nil {
		return "", err
	}
	contract := USDTContractForNetwork(w.cfg.Network)
	txe, err := w.gc.TRC20Send(fromAddr, toAddr, contract, amount, w.cfg.DefaultFeeLimitSun)
	if err != nil {
		return "", err
	}
	return w.signAndBroadcast(txe.GetTransaction(), priv)
}

func (w *WriteClient) bindContext(ctx context.Context) error {
	if ctx == nil {
		return w.gc.SetContext(context.Background())
	}
	return w.gc.SetContext(ctx)
}

func (w *WriteClient) signAndBroadcast(tx *core.Transaction, priv *ecdsa.PrivateKey) (string, error) {
	if tx == nil {
		return "", fmt.Errorf("empty transaction")
	}
	signed, err := transaction.SignTransactionECDSA(tx, priv)
	if err != nil {
		return "", err
	}
	txid := transactionIDHex(signed)
	ret, err := w.gc.Broadcast(signed)
	if err != nil {
		return "", err
	}
	if ret.GetCode() != 0 {
		return "", fmt.Errorf("broadcast: %s", string(ret.GetMessage()))
	}
	return txid, nil
}

func transactionIDHex(tx *core.Transaction) string {
	raw, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return common.BytesToHexString(sum[:])
}
