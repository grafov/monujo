package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/scrypt"

	"github.com/grafov/monujo/internal/keystore"
)

// Envelope kinds.
const (
	// KindUnsignedTransfer wraps an [UnsignedTransferPayload].
	KindUnsignedTransfer = "unsigned_transfer"
	// KindSignedTransfer wraps a [SignedTransferPayload].
	KindSignedTransfer = "signed_transfer"
	// KindMultisigTransfer wraps a [MultisigTransferPayload] (a partially or
	// fully signed multi-signature Tron transaction).
	KindMultisigTransfer = "multisig_transfer"
)

const (
	envelopeMagic = "monujo-offline"
	hmacAlgID     = "scrypt+hmac-sha256"

	// hmacScryptSaltStr is a fixed deterministic salt: the HMAC key is derived
	// from a user passphrase, not stored, so a fixed salt is acceptable here
	// (the threat model is integrity, not key recovery from the file). Using
	// a fixed salt keeps the file self-contained and lets the offline machine
	// re-derive the key without extra metadata.
	hmacScryptSaltStr = "monujo-offline-v1"

	hmacScryptN = 32768
	hmacScryptR = 8
	hmacScryptP = 1
	hmacKeyLen  = 32
)

// Envelope is the on-disk wrapper for unsigned and signed transfer payloads.
//
// The payload is base64-encoded so the HMAC covers exactly the bytes that
// live on disk regardless of JSON whitespace. When HMACHex is empty, no
// integrity check is performed; the caller must rely on visual confirmation
// of decoded fields instead.
type Envelope struct {
	Magic      string `json:"monujo"`
	Kind       string `json:"kind"`
	Version    int    `json:"version"`
	PayloadB64 string `json:"payload_b64"`
	HMACAlg    string `json:"hmac_alg,omitempty"`
	HMACHex    string `json:"hmac_hex,omitempty"`
}

// FileFingerprint is the SHA-256 of the on-disk envelope bytes, rendered as
// a hex string. It is shown to the human on the source machine and on the
// target machine so a mismatch (e.g. file replaced on a USB stick) can be
// caught even without an HMAC passphrase.
type FileFingerprint struct {
	SHA256Hex string
}

// DeriveHMACKey derives a 32-byte HMAC key from a user passphrase using
// scrypt with a fixed salt. The passphrase is Unicode NFKC-normalized via
// [keystore.NormalizePassphrase] before the KDF so that the same visual
// password produces the same key across operating systems and input methods
// (per NIST SP 800-63B §5.1.1.2 and the OWASP Password Storage Cheat Sheet).
//
// Returns an error only if the underlying scrypt call rejects parameters;
// for the constants used here it never does, but errors are still
// propagated for forward compatibility.
//
// The input passphrase is not zeroed by this function; callers should wipe
// it after use (see [keystore.ZeroPassphrase]).
func DeriveHMACKey(passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("hmac passphrase must not be empty")
	}
	normPass := keystore.NormalizePassphrase(passphrase)
	defer keystore.ZeroPassphrase(normPass)
	dk, err := scrypt.Key(normPass, []byte(hmacScryptSaltStr), hmacScryptN, hmacScryptR, hmacScryptP, hmacKeyLen)
	if err != nil {
		return nil, fmt.Errorf("derive hmac key: %w", err)
	}
	return dk, nil
}

// SealPayload wraps payload bytes into a JSON [Envelope] of the given kind.
// If hmacKey is non-nil and non-empty, the HMAC-SHA256 of the base64 payload
// (raw bytes, not the JSON quotes) is included so [OpenEnvelope] can verify it.
//
// The returned bytes are pretty-printed JSON suitable for direct disk write.
func SealPayload(kind string, payload []byte, hmacKey []byte) ([]byte, error) {
	if !validKind(kind) {
		return nil, fmt.Errorf("seal: unknown kind %q", kind)
	}
	if len(payload) == 0 {
		return nil, errors.New("seal: empty payload")
	}
	b64 := base64.StdEncoding.EncodeToString(payload)
	env := Envelope{
		Magic:      envelopeMagic,
		Kind:       kind,
		Version:    EnvelopeVersion,
		PayloadB64: b64,
	}
	if len(hmacKey) > 0 {
		mac := hmac.New(sha256.New, hmacKey)
		mac.Write([]byte(b64))
		env.HMACAlg = hmacAlgID
		env.HMACHex = hex.EncodeToString(mac.Sum(nil))
	}
	return json.MarshalIndent(env, "", "  ")
}

// OpenEnvelope parses blob as a JSON [Envelope], verifies it has the
// expected kind and version, and returns the decoded payload bytes plus a
// SHA-256 fingerprint of blob (always computed, regardless of HMAC).
//
// Behavior around HMAC:
//
//   - if env.HMACHex is empty and hmacKey is nil/empty → no check (returns ok);
//   - if env.HMACHex is non-empty and hmacKey is nil/empty → returns
//     [ErrHMACRequired] so the caller can re-prompt for a passphrase;
//   - if both are present → constant-time comparison; mismatch returns
//     [ErrHMACMismatch].
func OpenEnvelope(blob []byte, expectedKind string, hmacKey []byte) ([]byte, FileFingerprint, error) {
	sum := sha256.Sum256(blob)
	fp := FileFingerprint{SHA256Hex: hex.EncodeToString(sum[:])}

	var env Envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		return nil, fp, fmt.Errorf("open: parse envelope: %w", err)
	}
	if env.Magic != envelopeMagic {
		return nil, fp, fmt.Errorf("open: not a monujo offline envelope (magic=%q)", env.Magic)
	}
	if env.Version != EnvelopeVersion {
		return nil, fp, fmt.Errorf("open: unsupported envelope version %d", env.Version)
	}
	if !validKind(env.Kind) {
		return nil, fp, fmt.Errorf("open: unknown envelope kind %q", env.Kind)
	}
	if env.Kind != expectedKind {
		return nil, fp, fmt.Errorf("open: expected kind %q, got %q", expectedKind, env.Kind)
	}
	if env.PayloadB64 == "" {
		return nil, fp, errors.New("open: empty payload_b64")
	}

	if env.HMACHex != "" {
		if env.HMACAlg != hmacAlgID {
			return nil, fp, fmt.Errorf("open: unsupported hmac_alg %q", env.HMACAlg)
		}
		if len(hmacKey) == 0 {
			return nil, fp, ErrHMACRequired
		}
		want, err := hex.DecodeString(env.HMACHex)
		if err != nil {
			return nil, fp, fmt.Errorf("open: decode hmac_hex: %w", err)
		}
		mac := hmac.New(sha256.New, hmacKey)
		mac.Write([]byte(env.PayloadB64))
		if !hmac.Equal(want, mac.Sum(nil)) {
			return nil, fp, ErrHMACMismatch
		}
	}

	payload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	if err != nil {
		return nil, fp, fmt.Errorf("open: decode payload_b64: %w", err)
	}
	return payload, fp, nil
}

// ErrHMACRequired is returned by [OpenEnvelope] when the file contains an
// HMAC but the caller did not supply a key.
var ErrHMACRequired = errors.New("envelope has hmac but no passphrase was provided")

// ErrHMACMismatch is returned by [OpenEnvelope] on integrity check failure.
var ErrHMACMismatch = errors.New("envelope hmac mismatch (file tampered or wrong passphrase)")

func validKind(k string) bool {
	switch k {
	case KindUnsignedTransfer, KindSignedTransfer, KindMultisigTransfer:
		return true
	default:
		return false
	}
}
