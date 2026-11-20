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
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/transfer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// GRPCWriteClient signs and broadcasts transactions via Tron gRPC.
type GRPCWriteClient struct {
	cfg *config.Config
	gc  *gclient.GrpcClient
}

// NewGRPCWriteClientFromConfig connects to a Tron full node over gRPC.
func NewGRPCWriteClientFromConfig(cfg *config.Config) (*GRPCWriteClient, error) {
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
	return &GRPCWriteClient{cfg: cfg, gc: gc}, nil
}

// Close releases the gRPC connection.
func (w *GRPCWriteClient) Close() error {
	if w.gc != nil && w.gc.Conn != nil {
		return w.gc.Conn.Close()
	}
	return nil
}

func callerCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// SendTRX transfers native TRX (amount in sun).
func (w *GRPCWriteClient) SendTRX(ctx context.Context, fromAddr, toAddr string, amountSun int64, priv *ecdsa.PrivateKey) (string, error) {
	tx, err := w.CreateTRXTransfer(ctx, fromAddr, toAddr, amountSun)
	if err != nil {
		return "", err
	}
	if err := VerifyBuiltTransfer(tx, TransferSpec{
		Network: w.cfg.Network,
		Kind:    transfer.KindTRX,
		From:    fromAddr,
		To:      toAddr,
		Amount:  big.NewInt(amountSun),
	}); err != nil {
		return "", err
	}
	return w.signAndBroadcast(ctx, tx, priv)
}

// SendUSDT transfers USDT on the configured network using the canonical USDT (or test) contract.
func (w *GRPCWriteClient) SendUSDT(ctx context.Context, fromAddr, toAddr string, amount *big.Int, priv *ecdsa.PrivateKey) (string, error) {
	contract := USDTContractForNetwork(w.cfg.Network)
	tx, err := w.CreateTRC20Transfer(ctx, fromAddr, toAddr, contract, amount)
	if err != nil {
		return "", err
	}
	if err := VerifyBuiltTransfer(tx, TransferSpec{
		Network:     w.cfg.Network,
		Kind:        transfer.KindTRC20USDT,
		From:        fromAddr,
		To:          toAddr,
		Amount:      amount,
		FeeLimitSun: w.cfg.DefaultFeeLimitSun,
	}); err != nil {
		return "", err
	}
	return w.signAndBroadcast(ctx, tx, priv)
}

// CreateTRXTransfer builds an unsigned native TRX transfer via gRPC.
func (w *GRPCWriteClient) CreateTRXTransfer(ctx context.Context, from, to string, amountSun int64) (*core.Transaction, error) {
	txe, err := w.gc.TransferCtx(callerCtx(ctx), from, to, amountSun)
	if err != nil {
		return nil, err
	}
	tx := txe.GetTransaction()
	if tx == nil {
		return nil, fmt.Errorf("empty transaction")
	}
	if ret := txe.GetResult(); ret != nil && ret.GetCode() != 0 {
		return nil, fmt.Errorf("create trx: %s", string(ret.GetMessage()))
	}
	return tx, nil
}

// CreateTRC20Transfer builds an unsigned TRC-20 transfer via gRPC.
func (w *GRPCWriteClient) CreateTRC20Transfer(ctx context.Context, from, to, contract string, amount *big.Int) (*core.Transaction, error) {
	txe, err := w.gc.TRC20SendCtx(callerCtx(ctx), from, to, contract, amount, w.cfg.DefaultFeeLimitSun)
	if err != nil {
		return nil, err
	}
	tx := txe.GetTransaction()
	if tx == nil {
		return nil, fmt.Errorf("empty transaction")
	}
	if ret := txe.GetResult(); ret != nil && ret.GetCode() != 0 {
		return nil, fmt.Errorf("create trc20: %s", string(ret.GetMessage()))
	}
	return tx, nil
}

// BroadcastTransaction submits a signed transaction via gRPC.
func (w *GRPCWriteClient) BroadcastTransaction(ctx context.Context, tx *core.Transaction) error {
	if tx == nil {
		return fmt.Errorf("empty transaction")
	}
	ret, err := w.gc.BroadcastCtx(callerCtx(ctx), tx)
	if err != nil {
		return err
	}
	if ret == nil {
		return fmt.Errorf("broadcast: nil response")
	}
	if ret.GetCode() != 0 {
		return fmt.Errorf("broadcast: %s", string(ret.GetMessage()))
	}
	return nil
}

// GetTransactionSignWeight queries multisig sign weight via gRPC.
func (w *GRPCWriteClient) GetTransactionSignWeight(ctx context.Context, tx *core.Transaction) (*api.TransactionSignWeight, error) {
	if tx == nil {
		return nil, fmt.Errorf("empty transaction")
	}
	return w.gc.GetTransactionSignWeightCtx(callerCtx(ctx), tx)
}

func (w *GRPCWriteClient) signAndBroadcast(ctx context.Context, tx *core.Transaction, priv *ecdsa.PrivateKey) (string, error) {
	if tx == nil {
		return "", fmt.Errorf("empty transaction")
	}
	signed, err := transaction.SignTransactionECDSA(tx, priv)
	if err != nil {
		return "", err
	}
	txid := transactionIDHex(signed)
	ret, err := w.gc.BroadcastCtx(callerCtx(ctx), signed)
	if err != nil {
		return "", err
	}
	if ret == nil {
		return "", fmt.Errorf("broadcast: nil response")
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
