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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

// tronJSONTransaction is the canonical TRON HTTP/TronWeb envelope for a
// transaction. The canonical wire form is raw_data_hex (hex-encoded protobuf
// TransactionRaw); raw_data is kept as an opaque JSON blob so we do not
// alter whatever the coordinator produced, and signature is an array of hex
// strings (no 0x prefix).
type tronJSONTransaction struct {
	Visible    bool            `json:"visible,omitempty"`
	TxID       string          `json:"txID"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	RawDataHex string          `json:"raw_data_hex"`
	Signature  []string        `json:"signature,omitempty"`
}

// DecodeTronJSON parses body as a canonical Tron transaction JSON envelope
// (the same shape the TRON HTTP APIs and tronWeb.trx.multiSign use). It
// returns the hex-encoded raw_data, existing signatures in hex, and the
// SHA-256 of raw_data bytes as a lowercase hex txid. The incoming txID
// field (when present) must match the recomputed hash or the function
// returns an error so a tampered coordinator file cannot pass silently.
//
// Both raw_data_hex and each signature may carry a leading "0x" prefix in
// the input; the returned strings are normalized to lowercase hex without
// any prefix.
func DecodeTronJSON(body []byte) (rawDataHex string, sigsHex []string, txIDHex string, err error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil, "", errors.New("multisig: empty tron json")
	}
	var jtx tronJSONTransaction
	if err := json.Unmarshal(body, &jtx); err != nil {
		return "", nil, "", fmt.Errorf("multisig: parse tron json: %w", err)
	}
	if jtx.RawDataHex == "" {
		return "", nil, "", errors.New("multisig: tron json missing raw_data_hex")
	}
	rawHex := trimHex0x(jtx.RawDataHex)
	rawBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return "", nil, "", fmt.Errorf("multisig: decode raw_data_hex: %w", err)
	}
	if err := proto.Unmarshal(rawBytes, new(core.TransactionRaw)); err != nil {
		return "", nil, "", fmt.Errorf("multisig: unmarshal raw_data: %w", err)
	}
	sum := sha256.Sum256(rawBytes)
	computedID := hex.EncodeToString(sum[:])
	if jtx.TxID != "" {
		want := trimHex0x(jtx.TxID)
		if !bytes.EqualFold([]byte(want), []byte(computedID)) {
			return "", nil, "", fmt.Errorf("multisig: txID mismatch (json=%s computed=%s)", want, computedID)
		}
	}
	sigs := make([]string, 0, len(jtx.Signature))
	for i, s := range jtx.Signature {
		clean := trimHex0x(s)
		b, err := hex.DecodeString(clean)
		if err != nil {
			return "", nil, "", fmt.Errorf("multisig: decode signature[%d]: %w", i, err)
		}
		if len(b) != 65 {
			return "", nil, "", fmt.Errorf("multisig: signature[%d] length %d (want 65)", i, len(b))
		}
		sigs = append(sigs, hex.EncodeToString(b))
	}
	return rawHex, sigs, computedID, nil
}

// EncodeTronJSON serializes rawDataHex and sigsHex back into the canonical
// Tron transaction JSON envelope so the result can be fed to another
// cosigner, to tronWeb.trx.multiSign, or to /wallet/broadcasttransaction.
// The returned bytes are indented for human inspection; they round-trip
// through [DecodeTronJSON] unchanged modulo whitespace.
//
// Any 0x-prefix on the inputs is tolerated and stripped.
func EncodeTronJSON(rawDataHex string, sigsHex []string) ([]byte, error) {
	rawHex := trimHex0x(rawDataHex)
	rawBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return nil, fmt.Errorf("multisig: decode raw_data_hex: %w", err)
	}
	var raw core.TransactionRaw
	if err := proto.Unmarshal(rawBytes, &raw); err != nil {
		return nil, fmt.Errorf("multisig: unmarshal raw_data: %w", err)
	}
	sum := sha256.Sum256(rawBytes)
	out := tronJSONTransaction{
		TxID:       hex.EncodeToString(sum[:]),
		RawDataHex: rawHex,
	}
	for i, s := range sigsHex {
		clean := trimHex0x(s)
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("multisig: decode signature[%d]: %w", i, err)
		}
		if len(b) != 65 {
			return nil, fmt.Errorf("multisig: signature[%d] length %d (want 65)", i, len(b))
		}
		out.Signature = append(out.Signature, hex.EncodeToString(b))
	}
	return json.MarshalIndent(out, "", "  ")
}

// AppendMultisigSignature signs the hex-encoded raw_data with priv and
// returns the updated list of signatures (hex, no 0x prefix) and the
// recomputed txID. The previous signatures are preserved in order; the new
// one is appended, matching [transaction.SignTransactionECDSA].
//
// Returns [ErrAlreadySigned] if priv's address is already represented in
// prevSigsHex so callers can distinguish a duplicate from a real failure
// (and decide whether to override with --force).
func AppendMultisigSignature(rawDataHex string, prevSigsHex []string, priv *ecdsa.PrivateKey) (newSigsHex []string, txIDHex string, err error) {
	if priv == nil {
		return nil, "", errors.New("multisig: nil private key")
	}
	rawHex := trimHex0x(rawDataHex)
	rawBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return nil, "", fmt.Errorf("multisig: decode raw_data_hex: %w", err)
	}
	sum := sha256.Sum256(rawBytes)

	selfAddr := address.PubkeyToAddress(priv.PublicKey)
	for i, s := range prevSigsHex {
		clean := trimHex0x(s)
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, "", fmt.Errorf("multisig: decode prev signature[%d]: %w", i, err)
		}
		signer, err := recoverSigner(sum[:], b)
		if err != nil {
			return nil, "", fmt.Errorf("multisig: recover signer[%d]: %w", i, err)
		}
		if bytes.Equal(signer, selfAddr) {
			return nil, "", ErrAlreadySigned
		}
	}
	sig, err := crypto.Sign(sum[:], priv)
	if err != nil {
		return nil, "", fmt.Errorf("multisig: sign: %w", err)
	}
	out := make([]string, 0, len(prevSigsHex)+1)
	for _, s := range prevSigsHex {
		out = append(out, hex.EncodeToString(mustDecodeHex(trimHex0x(s))))
	}
	out = append(out, hex.EncodeToString(sig))
	return out, hex.EncodeToString(sum[:]), nil
}

// ErrAlreadySigned is returned by [AppendMultisigSignature] when the
// private key's address is already present in the existing signature list.
var ErrAlreadySigned = errors.New("multisig: this key already signed the transaction")

// GetMultisigSignWeight reassembles a [core.Transaction] from rawDataHex
// and sigsHex and calls the node's GetTransactionSignWeight RPC. The
// returned weight structure contains the permission (threshold, keys),
// the list of addresses whose signatures were accepted and the cumulative
// weight.
//
// No broadcast is performed; this is safe to use from "multisig info".
func GetMultisigSignWeight(ctx context.Context, w TransactionWriter, rawDataHex string, sigsHex []string) (*api.TransactionSignWeight, error) {
	if w == nil {
		return nil, errors.New("multisig: nil write client")
	}
	tx, err := assembleTransaction(rawDataHex, sigsHex)
	if err != nil {
		return nil, err
	}
	return w.GetTransactionSignWeight(ctx, tx)
}

// BroadcastMultisig queries the node for the current sign weight, refuses
// to broadcast if the accumulated weight has not reached the permission
// threshold, and then broadcasts the fully signed transaction. The returned
// weight is always populated (even on pre-check failure) so callers can
// render a helpful diagnostic.
func BroadcastMultisig(ctx context.Context, w TransactionWriter, rawDataHex string, sigsHex []string) (string, *api.TransactionSignWeight, error) {
	if w == nil {
		return "", nil, errors.New("multisig: nil write client")
	}
	tx, err := assembleTransaction(rawDataHex, sigsHex)
	if err != nil {
		return "", nil, err
	}
	weight, err := w.GetTransactionSignWeight(ctx, tx)
	if err != nil {
		return "", nil, fmt.Errorf("multisig: get sign weight: %w", err)
	}
	perm := weight.GetPermission()
	if perm == nil {
		return "", weight, errors.New("multisig: node returned no permission; broadcast refused")
	}
	threshold := perm.GetThreshold()
	current := weight.GetCurrentWeight()
	if current < threshold {
		return "", weight, fmt.Errorf("multisig: current weight %d < threshold %d (permission id=%d); broadcast refused",
			current, threshold, perm.GetId())
	}
	if err := w.BroadcastTransaction(ctx, tx); err != nil {
		return "", weight, err
	}
	txid, err := txIDFromRawData(tx.GetRawData())
	if err != nil {
		return "", weight, err
	}
	return txid, weight, nil
}

// assembleTransaction rebuilds a [core.Transaction] from hex-encoded
// raw_data and signatures. The resulting message is only ever used for
// read-only RPCs (GetTransactionSignWeight) or a final broadcast.
func assembleTransaction(rawDataHex string, sigsHex []string) (*core.Transaction, error) {
	rawHex := trimHex0x(rawDataHex)
	sigs := make([][]byte, 0, len(sigsHex))
	for i, s := range sigsHex {
		clean := trimHex0x(s)
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("multisig: decode signature[%d]: %w", i, err)
		}
		if len(b) != 65 {
			return nil, fmt.Errorf("multisig: signature[%d] length %d (want 65)", i, len(b))
		}
		sigs = append(sigs, b)
	}
	tx, err := transaction.FromRawDataHex(rawHex, sigs...)
	if err != nil {
		return nil, fmt.Errorf("multisig: assemble transaction: %w", err)
	}
	return tx, nil
}

// recoverSigner recovers the 21-byte TRON address (0x41 prefix + 20 bytes
// keccak-derived) from a SHA-256 hash and a 65-byte secp256k1 signature.
// This mirrors gotron-sdk's [keystore.RecoverPubkey] but works without a
// Windows build tag and never mutates the input signature slice.
func recoverSigner(hash, signature []byte) ([]byte, error) {
	if len(signature) != 65 {
		return nil, fmt.Errorf("signature length %d (want 65)", len(signature))
	}
	normalized := make([]byte, 65)
	copy(normalized, signature)
	if normalized[64] >= 27 {
		normalized[64] -= 27
	}
	pub, err := crypto.SigToPub(hash, normalized)
	if err != nil {
		return nil, err
	}
	addr := address.PubkeyToAddress(*pub)
	return addr, nil
}

// mustDecodeHex is used only inside [AppendMultisigSignature] after the
// same string has already been validated by [hex.DecodeString] in the same
// function, so an error here would indicate a caller-side race and should
// never occur in tested paths.
func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}
