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
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
	"github.com/grafov/monujo/internal/transfer"
)

// trc20TransferSelectorBytes is the keccak256 4-byte selector for
// transfer(address,uint256) used by canonical TRC-20 transfers.
var trc20TransferSelectorBytes = []byte{0xa9, 0x05, 0x9c, 0xbb}

// chainForNetwork maps a config network to a transfer.ChainID for the
// air-gap envelope (so the offline machine can refuse to sign on a
// different chain).
func chainForNetwork(n config.Network) transfer.ChainID {
	switch n {
	case config.NetworkShasta:
		return transfer.ChainTronShasta
	default:
		return transfer.ChainTronMainnet
	}
}

// rawDataToHex marshals the given TransactionRaw and returns 0x-prefixed hex.
func rawDataToHex(raw *core.TransactionRaw) (string, error) {
	if raw == nil {
		return "", errors.New("offline: nil raw data")
	}
	b, err := proto.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("offline: marshal raw_data: %w", err)
	}
	return "0x" + hex.EncodeToString(b), nil
}

// txIDFromRawData computes the Tron txid (sha256 of marshaled raw_data)
// and returns it as a 0x-prefixed hex string, matching transactionIDHex.
func txIDFromRawData(raw *core.TransactionRaw) (string, error) {
	b, err := proto.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("offline: marshal raw_data for txid: %w", err)
	}
	sum := sha256.Sum256(b)
	return "0x" + hex.EncodeToString(sum[:]), nil
}

// trimHex0x returns s without the leading "0x" or "0X" prefix, if present.
func trimHex0x(s string) string {
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		return s[2:]
	}
	return s
}

// decodeRawDataHex parses a hex-encoded TransactionRaw (with or without 0x
// prefix) into a *core.TransactionRaw value.
func decodeRawDataHex(rawDataHex string) (*core.TransactionRaw, error) {
	b, err := hex.DecodeString(trimHex0x(rawDataHex))
	if err != nil {
		return nil, fmt.Errorf("offline: decode raw_data_hex: %w", err)
	}
	var raw core.TransactionRaw
	if err := proto.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("offline: unmarshal raw_data: %w", err)
	}
	return &raw, nil
}

// BuildUnsignedTRX prepares an unsigned TRX transfer payload using the live
// gRPC node (only the node's CreateTransaction2 call, no signing). The
// resulting payload can be carried to an offline machine for signing.
//
// The function validates from/to addresses locally before any RPC.
func BuildUnsignedTRX(ctx context.Context, w *WriteClient, from, to string, amountSun int64, note string) (transfer.UnsignedTransferPayload, error) {
	var zero transfer.UnsignedTransferPayload
	if w == nil {
		return zero, errors.New("offline: nil write client")
	}
	if err := ValidateBase58Address(from); err != nil {
		return zero, fmt.Errorf("offline: from: %w", err)
	}
	if err := ValidateBase58Address(to); err != nil {
		return zero, fmt.Errorf("offline: to: %w", err)
	}
	if amountSun <= 0 {
		return zero, errors.New("offline: trx amount must be positive")
	}
	if err := w.bindContext(ctx); err != nil {
		return zero, err
	}
	txe, err := w.gc.Transfer(from, to, amountSun)
	if err != nil {
		return zero, err
	}
	tx := txe.GetTransaction()
	if tx == nil {
		return zero, errors.New("offline: empty transaction extension")
	}
	raw := tx.GetRawData()
	rawHex, err := rawDataToHex(raw)
	if err != nil {
		return zero, err
	}
	txid, err := txIDFromRawData(raw)
	if err != nil {
		return zero, err
	}
	return transfer.UnsignedTransferPayload{
		Version:       transfer.EnvelopeVersion,
		Chain:         chainForNetwork(w.cfg.Network),
		Kind:          transfer.KindTRX,
		From:          from,
		To:            to,
		Amount:        domain.FormatSunAsTRX(amountSun),
		AmountMinimal: fmt.Sprintf("%d", amountSun),
		RawDataHex:    rawHex,
		Expiration:    raw.GetExpiration(),
		Timestamp:     raw.GetTimestamp(),
		TxIDHex:       txid,
		Note:          note,
	}, nil
}

// BuildUnsignedUSDT prepares an unsigned TRC-20 USDT transfer payload using
// the live gRPC node (TRC20Send), without signing. The contract address is
// taken from cfg.Network via [USDTContractForNetwork], matching what the
// online hot wallet would use.
func BuildUnsignedUSDT(ctx context.Context, w *WriteClient, from, to string, amount *big.Int, note string) (transfer.UnsignedTransferPayload, error) {
	var zero transfer.UnsignedTransferPayload
	if w == nil {
		return zero, errors.New("offline: nil write client")
	}
	if err := ValidateBase58Address(from); err != nil {
		return zero, fmt.Errorf("offline: from: %w", err)
	}
	if err := ValidateBase58Address(to); err != nil {
		return zero, fmt.Errorf("offline: to: %w", err)
	}
	if amount == nil || amount.Sign() <= 0 {
		return zero, errors.New("offline: usdt amount must be positive")
	}
	if err := w.bindContext(ctx); err != nil {
		return zero, err
	}
	contract := USDTContractForNetwork(w.cfg.Network)
	txe, err := w.gc.TRC20Send(from, to, contract, amount, w.cfg.DefaultFeeLimitSun)
	if err != nil {
		return zero, err
	}
	tx := txe.GetTransaction()
	if tx == nil {
		return zero, errors.New("offline: empty transaction extension")
	}
	raw := tx.GetRawData()
	rawHex, err := rawDataToHex(raw)
	if err != nil {
		return zero, err
	}
	txid, err := txIDFromRawData(raw)
	if err != nil {
		return zero, err
	}
	return transfer.UnsignedTransferPayload{
		Version:       transfer.EnvelopeVersion,
		Chain:         chainForNetwork(w.cfg.Network),
		Kind:          transfer.KindTRC20USDT,
		From:          from,
		To:            to,
		Amount:        domain.FormatMinimalUSDT(amount) + " USDT",
		AmountMinimal: amount.String(),
		Contract:      contract,
		FeeLimitSun:   w.cfg.DefaultFeeLimitSun,
		RawDataHex:    rawHex,
		Expiration:    raw.GetExpiration(),
		Timestamp:     raw.GetTimestamp(),
		TxIDHex:       txid,
		Note:          note,
	}, nil
}

// SignRawData signs a hex-encoded TransactionRaw with priv. Returns a
// 0x-prefixed signature hex (65 bytes: r||s||v) and the recomputed txid.
//
// This is fully offline: no network access, no gotron-sdk gRPC.
func SignRawData(rawDataHex string, priv *ecdsa.PrivateKey) (sigHex, txIDHex string, err error) {
	if priv == nil {
		return "", "", errors.New("offline: nil private key")
	}
	raw, err := decodeRawDataHex(rawDataHex)
	if err != nil {
		return "", "", err
	}
	rawBytes, err := proto.Marshal(raw)
	if err != nil {
		return "", "", fmt.Errorf("offline: marshal raw_data: %w", err)
	}
	sum := sha256.Sum256(rawBytes)
	sig, err := crypto.Sign(sum[:], priv)
	if err != nil {
		return "", "", fmt.Errorf("offline: sign: %w", err)
	}
	return "0x" + hex.EncodeToString(sig), "0x" + hex.EncodeToString(sum[:]), nil
}

// BroadcastSigned reassembles a core.Transaction from rawDataHex+signature
// and broadcasts it via the WriteClient's gRPC connection. Returns the txid
// (0x-prefixed) on success.
func BroadcastSigned(ctx context.Context, w *WriteClient, rawDataHex, sigHex string) (string, error) {
	if w == nil {
		return "", errors.New("offline: nil write client")
	}
	raw, err := decodeRawDataHex(rawDataHex)
	if err != nil {
		return "", err
	}
	sig, err := hex.DecodeString(trimHex0x(sigHex))
	if err != nil {
		return "", fmt.Errorf("offline: decode signature_hex: %w", err)
	}
	if len(sig) != 65 {
		return "", fmt.Errorf("offline: signature must be 65 bytes, got %d", len(sig))
	}
	if err := w.bindContext(ctx); err != nil {
		return "", err
	}
	tx := &core.Transaction{
		RawData:   raw,
		Signature: [][]byte{sig},
	}
	txid, err := txIDFromRawData(raw)
	if err != nil {
		return "", err
	}
	ret, err := w.gc.Broadcast(tx)
	if err != nil {
		return "", err
	}
	if ret.GetCode() != 0 {
		return "", fmt.Errorf("broadcast: %s", string(ret.GetMessage()))
	}
	return txid, nil
}

// RawDataSummary holds the human-readable fields decoded from a
// TransactionRaw's first contract. It is used by both signer and
// broadcaster to independently verify what the payload claims.
type RawDataSummary struct {
	Kind         string
	From         string
	To           string
	AmountStr    string
	AmountWei    *big.Int
	Contract     string
	FeeLimitSun  int64
	ExpirationMs int64
	TimestampMs  int64
}

// DecodeRawDataPermissionID parses rawDataHex and returns the Permission_id
// of the first contract. Non-decodable input or missing contract yields 0
// (Tron's default permission id). This is used by the multisig CLI so it
// can classify a transaction by its target permission without needing the
// higher-level raw_data summary.
func DecodeRawDataPermissionID(rawDataHex string) int32 {
	raw, err := decodeRawDataHex(rawDataHex)
	if err != nil {
		return 0
	}
	contracts := raw.GetContract()
	if len(contracts) == 0 {
		return 0
	}
	return contracts[0].GetPermissionId()
}

// DecodeRawDataSummary parses rawDataHex and extracts the kind/from/to/amount
// fields from the first contract. Supports TRX TransferContract and
// TRC-20 transfer(address,uint256) via TriggerSmartContract; other contract
// types are rejected so the offline signer never produces a payload it
// cannot show to the human.
func DecodeRawDataSummary(rawDataHex string) (RawDataSummary, error) {
	var out RawDataSummary
	raw, err := decodeRawDataHex(rawDataHex)
	if err != nil {
		return out, err
	}
	out.ExpirationMs = raw.GetExpiration()
	out.TimestampMs = raw.GetTimestamp()
	out.FeeLimitSun = raw.GetFeeLimit()

	contracts := raw.GetContract()
	if len(contracts) != 1 {
		return out, fmt.Errorf("offline: expected exactly one contract, got %d", len(contracts))
	}
	c := contracts[0]
	switch c.GetType() {
	case core.Transaction_Contract_TransferContract:
		var tc core.TransferContract
		if err := unmarshalAny(c.GetParameter(), &tc); err != nil {
			return out, fmt.Errorf("offline: decode TransferContract: %w", err)
		}
		out.Kind = transfer.KindTRX
		out.From = common.EncodeCheck(tc.GetOwnerAddress())
		out.To = common.EncodeCheck(tc.GetToAddress())
		out.AmountWei = big.NewInt(tc.GetAmount())
		out.AmountStr = domain.FormatSunAsTRX(tc.GetAmount())
	case core.Transaction_Contract_TriggerSmartContract:
		var ts core.TriggerSmartContract
		if err := unmarshalAny(c.GetParameter(), &ts); err != nil {
			return out, fmt.Errorf("offline: decode TriggerSmartContract: %w", err)
		}
		to, amount, err := decodeTRC20Transfer(ts.GetData())
		if err != nil {
			return out, err
		}
		out.Kind = transfer.KindTRC20USDT
		out.From = common.EncodeCheck(ts.GetOwnerAddress())
		out.Contract = common.EncodeCheck(ts.GetContractAddress())
		out.To = to
		out.AmountWei = amount
		out.AmountStr = domain.FormatMinimalUSDT(amount) + " USDT"
	default:
		return out, fmt.Errorf("offline: unsupported contract type %v (only TRX transfer and TRC-20 transfer are supported)", c.GetType())
	}
	return out, nil
}

// unmarshalAny decodes a protobuf Any into msg, ignoring the type URL (gotron
// stores raw protocol contracts there which are not registered in protobuf
// global registry on every build). This mirrors what gotron-sdk does.
func unmarshalAny(a *anypb.Any, msg proto.Message) error {
	if a == nil {
		return errors.New("nil any")
	}
	return proto.Unmarshal(a.GetValue(), msg)
}

// decodeTRC20Transfer parses an ABI-encoded transfer(address,uint256) call
// data buffer (selector + 32 bytes recipient + 32 bytes amount) and returns
// the recipient base58 address and the raw amount in token minimal units.
func decodeTRC20Transfer(data []byte) (to string, amount *big.Int, err error) {
	if len(data) != 4+32+32 {
		return "", nil, fmt.Errorf("offline: trc20 data length: %d (want 68)", len(data))
	}
	if !bytesEqual(data[:4], trc20TransferSelectorBytes) {
		return "", nil, fmt.Errorf("offline: trc20 selector mismatch (not transfer(address,uint256))")
	}
	addrSlot := data[4:36]
	if !allZero(addrSlot[:11]) {
		return "", nil, errors.New("offline: trc20 to-address upper bytes not zero")
	}
	tronAddr := append([]byte{0x41}, addrSlot[12:]...)
	to = common.EncodeCheck(tronAddr)
	amount = new(big.Int).SetBytes(data[36:68])
	return to, amount, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// FormatExpirationISO returns a stable, timezone-independent rendering of the
// expiration timestamp suitable for showing to the human. Centralized here so
// signer and broadcaster show the same string.
func FormatExpirationISO(unixMs int64) string {
	if unixMs <= 0 {
		return "(none)"
	}
	return time.UnixMilli(unixMs).UTC().Format(time.RFC3339)
}
