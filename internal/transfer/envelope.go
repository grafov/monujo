// Package transfer defines versioned JSON payloads for future offline signing workflows.
package transfer

import (
	"encoding/json"
	"fmt"
)

// EnvelopeVersion is the current on-wire format version for unsigned/signed transfer files.
const EnvelopeVersion = 1

// ChainID identifies a target network for a transfer request.
type ChainID string

const (
	// ChainTronMainnet is TRON mainnet.
	ChainTronMainnet ChainID = "tron:mainnet"
	// ChainTronShasta is TRON Shasta testnet.
	ChainTronShasta ChainID = "tron:shasta"
)

// UnsignedTransferRequest is a portable description of an intended transfer (for future offline signing).
// Fields are intentionally minimal and stable across releases.
type UnsignedTransferRequest struct {
	Version   int       `json:"version"`
	Chain     ChainID   `json:"chain"`
	Kind      string    `json:"kind"` // "trx" | "trc20_usdt"
	From      string    `json:"from"`
	To        string    `json:"to"`
	Amount    string    `json:"amount"` // decimal string in TRX or USDT depending on Kind
	Contract  string    `json:"contract,omitempty"`
	CreatedAt int64     `json:"created_at_unix,omitempty"`
	Note      string    `json:"note,omitempty"`
}

// SignedTransferResponse wraps a signed broadcast-ready payload (for a future offline signer).
// RawTransactionHex is chain-specific; for Tron it may be protobuf hex of core.Transaction.
type SignedTransferResponse struct {
	Version          int    `json:"version"`
	Chain            ChainID `json:"chain"`
	UnsignedID       string `json:"unsigned_id,omitempty"`
	RawTransactionHex string `json:"raw_transaction_hex,omitempty"`
	TxID             string `json:"tx_id,omitempty"`
}

// MarshalUnsigned serializes an unsigned request to JSON with validation.
func MarshalUnsigned(u UnsignedTransferRequest) ([]byte, error) {
	if u.Version == 0 {
		u.Version = EnvelopeVersion
	}
	if u.Version != EnvelopeVersion {
		return nil, fmt.Errorf("unsupported envelope version %d", u.Version)
	}
	if u.Kind == "" || u.From == "" || u.To == "" || u.Amount == "" {
		return nil, fmt.Errorf("unsigned transfer: missing required fields")
	}
	return json.MarshalIndent(u, "", "  ")
}

// UnmarshalUnsigned parses JSON into UnsignedTransferRequest.
func UnmarshalUnsigned(b []byte) (UnsignedTransferRequest, error) {
	var u UnsignedTransferRequest
	if err := json.Unmarshal(b, &u); err != nil {
		return u, err
	}
	if u.Version != EnvelopeVersion {
		return u, fmt.Errorf("unsupported envelope version %d", u.Version)
	}
	return u, nil
}
