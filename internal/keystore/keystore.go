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
	"slices"
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
	Version  int      `json:"version"`
	Address  string   `json:"address"`
	Networks []string `json:"networks,omitempty"`
	KDF      string   `json:"kdf"`
	SaltHex  string   `json:"salt_hex"`
	N        int      `json:"n"`
	R        int      `json:"r"`
	P        int      `json:"p"`
	Cipher   string   `json:"cipher"`
	NonceHex string   `json:"nonce_hex"`
	DataHex  string   `json:"data_hex"`
}

// WalletListItem is public metadata for one keystore file (no secret material).
type WalletListItem struct {
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Networks []string `json:"networks,omitempty"`
}

// NormalizeNetworkID returns lowercase Tron network ids used in keystore (mainnet, shasta).
func NormalizeNetworkID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	switch s {
	case "mainnet":
		return "mainnet"
	case "shasta":
		return "shasta"
	default:
		return s
	}
}

func normalizeNetworkList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		n := NormalizeNetworkID(s)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// NetworkOperationNeedsConfirmation reports whether the wallet file lists networks
// and the operation network is not among them.
func NetworkOperationNeedsConfirmation(recorded []string, operationNetwork string) bool {
	if len(recorded) == 0 {
		return false
	}
	op := NormalizeNetworkID(operationNetwork)
	for _, r := range recorded {
		if NormalizeNetworkID(r) == op {
			return false
		}
	}
	return true
}

// ListWallets returns one entry per valid v1 wallet JSON in keystoreDir, sorted by name.
// A missing directory yields an empty list. Unreadable or non-v1 JSON files are skipped.
func ListWallets(keystoreDir string) ([]WalletListItem, error) {
	entries, err := os.ReadDir(keystoreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []WalletListItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		path := filepath.Join(keystoreDir, name)
		path = filepath.Clean(path)
		base := filepath.Clean(keystoreDir)
		rel, err := filepath.Rel(base, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var ew EncryptedWalletFile
		if err := json.Unmarshal(body, &ew); err != nil {
			continue
		}
		if ew.Version != fileVersion || strings.TrimSpace(ew.Address) == "" {
			continue
		}
		stem := name[:len(name)-len(".json")]
		out = append(out, WalletListItem{
			Name:     stem,
			Address:  ew.Address,
			Networks: normalizeNetworkList(ew.Networks),
		})
	}
	slices.SortFunc(out, func(a, b WalletListItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// ReadWalletNetworks returns normalized network ids from the wallet JSON without decrypting the key.
func ReadWalletNetworks(keystoreDir, name string) ([]string, error) {
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	body, err := os.ReadFile(path) //nolint:gosec // G304 -- path under keystoreDir+safeName
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
	return normalizeNetworkList(ew.Networks), nil
}

// EnsureNetworkRecorded appends network to the wallet file metadata if missing (no re-encryption).
func EnsureNetworkRecorded(keystoreDir, name, network string) error {
	net := NormalizeNetworkID(network)
	if net == "" {
		return fmt.Errorf("empty network")
	}
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	body, err := os.ReadFile(path) //nolint:gosec // G304 -- path under keystoreDir+safeName
	if err != nil {
		return err
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		return err
	}
	if ew.Version != fileVersion {
		return fmt.Errorf("unsupported keystore version %d", ew.Version)
	}
	for _, x := range normalizeNetworkList(ew.Networks) {
		if x == net {
			return nil
		}
	}
	ew.Networks = normalizeNetworkList(append(slices.Clone(ew.Networks), net))
	out, err := json.MarshalIndent(&ew, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, out, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".monujo-keystore-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil { //nolint:gosec // G703 -- tmp from CreateTemp in same dir
		return err
	}
	ok = true
	return nil
}

// Store imports a hex-encoded secp256k1 private key, derives Tron address, and writes an encrypted file.
// networks lists Tron networks (e.g. mainnet, shasta) recorded for this wallet; may be nil.
func Store(keystoreDir, name string, privHex string, passphrase []byte, networks []string) (tronAddress string, err error) {
	priv, err := parsePrivateKeyHex(privHex)
	if err != nil {
		return "", err
	}
	defer zeroPriv(priv)
	return writeEncryptedWallet(keystoreDir, name, priv, passphrase, networks)
}

// GenerateAndStore creates a new random secp256k1 key, encrypts it to the keystore, and returns
// the Tron address and a hex copy of the private key for optional one-time export. The caller must
// treat privHex as secret and avoid logging it.
func GenerateAndStore(keystoreDir, name string, passphrase []byte, networks []string) (addr string, privHex string, err error) {
	priv, err := crypto.GenerateKey()
	if err != nil {
		return "", "", err
	}
	defer zeroPriv(priv)
	raw := crypto.FromECDSA(priv)
	privHex = hex.EncodeToString(raw)
	addr, err = writeEncryptedWallet(keystoreDir, name, priv, passphrase, networks)
	return addr, privHex, err
}

func writeEncryptedWallet(keystoreDir, name string, priv *ecdsa.PrivateKey, passphrase []byte, networks []string) (tronAddress string, err error) {
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

	nets := normalizeNetworkList(networks)
	ew := EncryptedWalletFile{
		Version:  fileVersion,
		Address:  addr,
		Networks: nets,
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
	body, err := os.ReadFile(path) //nolint:gosec // G304 -- path under keystoreDir+safeName
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
