// Package transfer defines versioned JSON payloads for the offline-signing
// workflow (build → sign → broadcast across an air-gap).
//
// Two payload kinds live here:
//
//   - [UnsignedTransferPayload] is produced online by `monujo offline build`
//     and carries everything the offline signer needs to recompute,
//     human-verify and sign the Tron transaction (RawDataHex), plus
//     human-readable fields for visual confirmation.
//   - [SignedTransferPayload] is produced offline by `monujo offline sign`
//     and adds the ECDSA signature so the file can be broadcast online.
//
// Both payloads are wrapped in an [Envelope] (see envelope_seal.go) before
// they touch disk: the envelope holds the base64 payload and an optional
// HMAC, so tampering can be detected end-to-end.
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

// Kind values for transfer payloads.
const (
	// KindTRX is a native TRX transfer.
	KindTRX = "trx"
	// KindTRC20USDT is a TRC-20 USDT transfer (transfer(address,uint256)).
	KindTRC20USDT = "trc20_usdt"
)

// UnsignedTransferPayload is the offline-signer input: enough to recompute,
// verify and sign a Tron transaction without any network access.
//
// All "display" fields (From, To, Amount, Contract, Expiration) are repeated
// outside RawDataHex so the offline signer can show them to the human before
// asking for the keystore passphrase. Authoritative bytes used for signing
// are RawDataHex; the signer MUST recompute the human-readable fields from
// it (see [tron.DecodeRawDataSummary]) and refuse to sign on mismatch.
type UnsignedTransferPayload struct {
	Version       int     `json:"version"`
	Chain         ChainID `json:"chain"`
	Kind          string  `json:"kind"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	Amount        string  `json:"amount"`
	AmountMinimal string  `json:"amount_minimal"`
	Contract      string  `json:"contract,omitempty"`
	FeeLimitSun   int64   `json:"fee_limit_sun,omitempty"`
	RawDataHex    string  `json:"raw_data_hex"`
	RefBlockNum   int64   `json:"ref_block_num,omitempty"`
	Expiration    int64   `json:"expiration_unix_ms"`
	Timestamp     int64   `json:"timestamp_unix_ms"`
	TxIDHex       string  `json:"tx_id_hex"`
	CreatedAtUnix int64   `json:"created_at_unix,omitempty"`
	Note          string  `json:"note,omitempty"`
}

// SignedTransferPayload is the broadcaster input: signed RawData plus a copy
// of the human-verifiable fields. The broadcaster MUST recompute fields from
// RawDataHex and refuse to broadcast on mismatch.
type SignedTransferPayload struct {
	Version      int     `json:"version"`
	Chain        ChainID `json:"chain"`
	Kind         string  `json:"kind"`
	From         string  `json:"from"`
	To           string  `json:"to"`
	Amount       string  `json:"amount"`
	Contract     string  `json:"contract,omitempty"`
	FeeLimitSun  int64   `json:"fee_limit_sun,omitempty"`
	Expiration   int64   `json:"expiration_unix_ms"`
	TxIDHex      string  `json:"tx_id_hex"`
	RawDataHex   string  `json:"raw_data_hex"`
	SignatureHex string  `json:"signature_hex"`
}

// MarshalUnsignedPayload serializes an unsigned payload to canonical JSON
// (compact, fixed key order via the struct definition) and validates required
// fields. The output is intended to be wrapped in an [Envelope] before it
// touches disk, so the HMAC covers exactly these bytes.
func MarshalUnsignedPayload(u UnsignedTransferPayload) ([]byte, error) {
	if u.Version == 0 {
		u.Version = EnvelopeVersion
	}
	if u.Version != EnvelopeVersion {
		return nil, fmt.Errorf("unsupported payload version %d", u.Version)
	}
	if u.Kind == "" || u.From == "" || u.To == "" || u.Amount == "" || u.RawDataHex == "" {
		return nil, fmt.Errorf("unsigned transfer: missing required fields")
	}
	return json.Marshal(u)
}

// UnmarshalUnsignedPayload parses canonical JSON into UnsignedTransferPayload
// and verifies the version.
func UnmarshalUnsignedPayload(b []byte) (UnsignedTransferPayload, error) {
	var u UnsignedTransferPayload
	if err := json.Unmarshal(b, &u); err != nil {
		return u, err
	}
	if u.Version != EnvelopeVersion {
		return u, fmt.Errorf("unsupported payload version %d", u.Version)
	}
	return u, nil
}

// MarshalSignedPayload serializes a signed payload to canonical JSON.
func MarshalSignedPayload(s SignedTransferPayload) ([]byte, error) {
	if s.Version == 0 {
		s.Version = EnvelopeVersion
	}
	if s.Version != EnvelopeVersion {
		return nil, fmt.Errorf("unsupported payload version %d", s.Version)
	}
	switch {
	case s.Kind == "", s.From == "", s.To == "", s.Amount == "":
		return nil, fmt.Errorf("signed transfer: missing required fields")
	case s.RawDataHex == "", s.SignatureHex == "":
		return nil, fmt.Errorf("signed transfer: missing required fields")
	}
	return json.Marshal(s)
}

// UnmarshalSignedPayload parses canonical JSON into SignedTransferPayload.
func UnmarshalSignedPayload(b []byte) (SignedTransferPayload, error) {
	var s SignedTransferPayload
	if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.Version != EnvelopeVersion {
		return s, fmt.Errorf("unsupported payload version %d", s.Version)
	}
	return s, nil
}
