// Package keystore stores Tron secp256k1 private keys encrypted at rest (scrypt + AES-GCM).
package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"golang.org/x/crypto/scrypt"
)

const (
	fileVersion = 1
	keyBytes    = 32
	scryptN     = 32768
	scryptR     = 8
	scryptP     = 1
	saltLen     = 32
	nonceLen    = 12
)

// EncryptedWalletFile is the on-disk JSON format for monujo keystore v1.
type EncryptedWalletFile struct {
	Version  int    `json:"version"`
	Address  string `json:"address"`
	KDF      string `json:"kdf"`
	SaltHex  string `json:"salt_hex"`
	N        int    `json:"n"`
	R        int    `json:"r"`
	P        int    `json:"p"`
	Cipher   string `json:"cipher"`
	NonceHex string `json:"nonce_hex"`
	DataHex  string `json:"data_hex"`
}

// Store imports a hex-encoded secp256k1 private key, derives Tron address, and writes an encrypted file.
func Store(keystoreDir, name string, privHex string, passphrase []byte) (tronAddress string, err error) {
	priv, err := parsePrivateKeyHex(privHex)
	if err != nil {
		return "", err
	}
	defer zeroPriv(priv)
	return writeEncryptedWallet(keystoreDir, name, priv, passphrase)
}

// GenerateAndStore creates a new random secp256k1 key, encrypts it to the keystore, and returns
// the Tron address and a hex copy of the private key for optional one-time export. The caller must
// treat privHex as secret and avoid logging it.
func GenerateAndStore(keystoreDir, name string, passphrase []byte) (addr string, privHex string, err error) {
	priv, err := crypto.GenerateKey()
	if err != nil {
		return "", "", err
	}
	defer zeroPriv(priv)
	raw := crypto.FromECDSA(priv)
	privHex = hex.EncodeToString(raw)
	addr, err = writeEncryptedWallet(keystoreDir, name, priv, passphrase)
	return addr, privHex, err
}

func writeEncryptedWallet(keystoreDir, name string, priv *ecdsa.PrivateKey, passphrase []byte) (tronAddress string, err error) {
	if err := os.MkdirAll(keystoreDir, 0o700); err != nil {
		return "", err
	}
	addr := address.PubkeyToAddress(priv.PublicKey).String()

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	dk, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		return "", err
	}
	defer zeroBytes(dk)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	plain := crypto.FromECDSA(priv)
	out := gcm.Seal(nil, nonce, plain, nil)
	zeroBytes(plain)

	ew := EncryptedWalletFile{
		Version:  fileVersion,
		Address:  addr,
		KDF:      "scrypt",
		SaltHex:  hex.EncodeToString(salt),
		N:        scryptN,
		R:        scryptR,
		P:        scryptP,
		Cipher:   "aes-256-gcm",
		NonceHex: hex.EncodeToString(nonce),
		DataHex:  hex.EncodeToString(out),
	}
	body, err := json.MarshalIndent(ew, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	return addr, nil
}

// LoadPrivateKey decrypts the named wallet and returns an ECDSA private key (caller must wipe when done).
func LoadPrivateKey(keystoreDir, name string, passphrase []byte) (*PrivKey, error) {
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		return nil, err
	}
	if ew.Version != fileVersion {
		return nil, fmt.Errorf("unsupported keystore version %d", ew.Version)
	}
	salt, err := hex.DecodeString(ew.SaltHex)
	if err != nil {
		return nil, err
	}
	nonce, err := hex.DecodeString(ew.NonceHex)
	if err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(ew.DataHex)
	if err != nil {
		return nil, err
	}
	dk, err := scrypt.Key(passphrase, salt, ew.N, ew.R, ew.P, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dk)
	if err != nil {
		zeroBytes(dk)
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		zeroBytes(dk)
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		zeroBytes(dk)
		return nil, errors.New("decryption failed (wrong passphrase or corrupt file)")
	}
	zeroBytes(dk)
	if len(plain) != keyBytes {
		zeroBytes(plain)
		return nil, fmt.Errorf("invalid decrypted key length")
	}
	priv, err := crypto.ToECDSA(plain)
	zeroBytes(plain)
	if err != nil {
		return nil, err
	}
	got := address.PubkeyToAddress(priv.PublicKey).String()
	if got != ew.Address {
		zeroPriv(priv)
		return nil, fmt.Errorf("address mismatch in keystore")
	}
	return &PrivKey{Key: priv, Address: ew.Address}, nil
}

// PrivKey wraps a decrypted key with expected Tron address.
type PrivKey struct {
	Key     *ecdsa.PrivateKey
	Address string
}

// Wipe zeros the private key material.
func (p *PrivKey) Wipe() {
	if p == nil || p.Key == nil {
		return
	}
	zeroPriv(p.Key)
	p.Key = nil
}

func zeroPriv(k *ecdsa.PrivateKey) {
	if k == nil || k.D == nil {
		return
	}
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func parsePrivateKeyHex(s string) (*ecdsa.PrivateKey, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 64 {
		return nil, fmt.Errorf("private key must be 64 hex characters")
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode private key hex: %w", err)
	}
	if len(raw) != keyBytes {
		return nil, fmt.Errorf("invalid private key length")
	}
	// Reject trivially weak keys.
	if isAllZero(raw) {
		return nil, fmt.Errorf("refusing weak private key")
	}
	priv, err := crypto.ToECDSA(raw)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

func isAllZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
