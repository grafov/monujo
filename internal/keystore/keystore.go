// Package keystore stores Tron secp256k1 private keys encrypted at rest.
//
// The current v2 format uses Argon2id (RFC 9106) as the password-based key
// derivation function and AES-256-GCM for encryption. Older v1 files produced
// by earlier monujo releases used scrypt; they are still readable and are
// transparently re-encrypted to v2 on first successful unlock (see
// [LoadPrivateKey]).
//
// Parameter choices for v2 follow the OWASP Password Storage Cheat Sheet
// "server" profile; salt length satisfies NIST SP 800-132 §5.1. monujo is not
// a FIPS 140-3 / ISO/IEC 19790 certified cryptographic module: Argon2id is
// not on the FIPS-approved list. See docs/06-keystore.md for details.
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
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/scrypt"
)

const (
	fileVersion             = 2
	fileVersionLegacyScrypt = 1
	keyBytes                = 32

	// Argon2id parameters for v2. These correspond to the OWASP
	// Password Storage Cheat Sheet "server" profile: t=3, m=64 MiB,
	// p=4, keyLen=32. A desktop unlock finishes in ~150-400 ms on
	// typical hardware.
	argon2Time    uint32 = 3
	argon2Memory  uint32 = 64 * 1024
	argon2Threads uint8  = 4
	argon2KeyLen  uint32 = 32

	// saltLen is 16 bytes (128 bits) which satisfies NIST SP 800-132
	// §5.1 and matches OWASP guidance for Argon2id. v1 files used 32
	// bytes; we keep reading them via SaltHex and only apply 16 when
	// writing v2.
	saltLen  = 16
	nonceLen = 12

	kdfScrypt   = "scrypt"
	kdfArgon2id = "argon2id"

	cipherAES256GCM = "aes-256-gcm"

	// legacyBackupSuffix is appended to the safe name when we back up
	// a v1 scrypt file during auto-migration. If a file with this
	// name already exists, a unix-ms timestamp is appended on top.
	legacyBackupSuffix = ".scrypt.bak"
)

func validateTronBase58Address(s string) error {
	s = strings.TrimSpace(s)
	if _, err := address.Base58ToAddress(s); err != nil {
		return fmt.Errorf("invalid Tron address: %w", err)
	}
	return nil
}

// EncryptedWalletFile is the on-disk JSON format for monujo keystore
// files. The same struct represents both v1 (scrypt, legacy) and v2
// (argon2id, current) files; which set of KDF fields is populated
// depends on [EncryptedWalletFile.KDF]:
//
//   - KDF == "scrypt"   -> N, R, P are set; argon2_* are zero.
//   - KDF == "argon2id" -> Argon2Time, Argon2Memory, Argon2Threads,
//     Argon2KeyLen are set; N, R, P are zero.
//
// All unrelated fields use omitempty so that round-tripping a v1 file
// during metadata updates (e.g. [EnsureNetworkRecorded]) does not
// introduce spurious zero-valued argon2_* fields.
type EncryptedWalletFile struct {
	Version  int      `json:"version"`
	External bool     `json:"external,omitempty"`
	Address  string   `json:"address"`
	Networks []string `json:"networks,omitempty"`
	KDF      string   `json:"kdf"`
	SaltHex  string   `json:"salt_hex"`

	// Legacy scrypt parameters (kdf == "scrypt").
	N int `json:"n,omitempty"`
	R int `json:"r,omitempty"`
	P int `json:"p,omitempty"`

	// Argon2id parameters (kdf == "argon2id").
	Argon2Time    uint32 `json:"argon2_time,omitempty"`
	Argon2Memory  uint32 `json:"argon2_memory,omitempty"`
	Argon2Threads uint8  `json:"argon2_threads,omitempty"`
	Argon2KeyLen  uint32 `json:"argon2_key_len,omitempty"`

	Cipher   string `json:"cipher"`
	NonceHex string `json:"nonce_hex"`
	DataHex  string `json:"data_hex"`
}

// WalletListItem is public metadata for one keystore file (no secret material).
type WalletListItem struct {
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Networks []string `json:"networks,omitempty"`
	External bool     `json:"external"`
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

// ListWallets returns one entry per valid wallet JSON in keystoreDir, sorted by name.
// A missing directory yields an empty list. Unreadable or invalid JSON files are skipped.
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
		addr := strings.TrimSpace(ew.Address)
		if addr == "" {
			continue
		}
		stem := name[:len(name)-len(".json")]
		if ew.External {
			if ew.Version != fileVersion {
				continue
			}
			if err := validateTronBase58Address(addr); err != nil {
				continue
			}
			out = append(out, WalletListItem{
				Name:     stem,
				Address:  addr,
				Networks: normalizeNetworkList(ew.Networks),
				External: true,
			})
			continue
		}
		if !isSupportedVersion(ew.Version) {
			continue
		}
		if strings.TrimSpace(ew.KDF) == "" {
			continue
		}
		out = append(out, WalletListItem{
			Name:     stem,
			Address:  addr,
			Networks: normalizeNetworkList(ew.Networks),
			External: false,
		})
	}
	slices.SortFunc(out, func(a, b WalletListItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// WalletExists reports whether a wallet file with the given logical name exists
// in keystoreDir. A missing keystore directory is not an error and returns
// (false, nil). The check is performed through os.OpenRoot so it cannot escape
// keystoreDir even if safeName is ever weakened.
func WalletExists(keystoreDir, name string) (bool, error) {
	root, err := os.OpenRoot(keystoreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = root.Close() }()
	_, err = root.Stat(safeName(name) + ".json")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// readWalletFile reads a wallet file by logical name from keystoreDir using os.Root,
// so that the resulting open is constrained to keystoreDir and cannot escape via
// symlinks or path traversal even if safeName is ever weakened.
func readWalletFile(keystoreDir, name string) ([]byte, error) {
	root, err := os.OpenRoot(keystoreDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(safeName(name) + ".json")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// ReadWalletNetworks returns normalized network ids from the wallet JSON without decrypting the key.
// Both v1 (legacy scrypt) and v2 (argon2id) files are accepted.
func ReadWalletNetworks(keystoreDir, name string) ([]string, error) {
	body, err := readWalletFile(keystoreDir, name)
	if err != nil {
		return nil, err
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		return nil, err
	}
	if ew.External {
		if ew.Version != fileVersion {
			return nil, fmt.Errorf("unsupported keystore version %d", ew.Version)
		}
		return normalizeNetworkList(ew.Networks), nil
	}
	if !isSupportedVersion(ew.Version) {
		return nil, fmt.Errorf("unsupported keystore version %d", ew.Version)
	}
	return normalizeNetworkList(ew.Networks), nil
}

// EnsureNetworkRecorded appends network to the wallet file metadata if missing
// (no re-encryption). Works on both v1 and v2 files; metadata updates do not
// trigger v1->v2 migration by themselves — only a successful [LoadPrivateKey]
// does, because migration requires the plaintext key.
func EnsureNetworkRecorded(keystoreDir, name, network string) error {
	net := NormalizeNetworkID(network)
	if net == "" {
		return fmt.Errorf("empty network")
	}
	body, err := readWalletFile(keystoreDir, name)
	if err != nil {
		return err
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		return err
	}
	if ew.External {
		return fmt.Errorf("wallet %q is external (address-only); networks cannot be updated this way", name)
	}
	if !isSupportedVersion(ew.Version) {
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
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	return writeFileAtomic(path, out, 0o600)
}

// isSupportedVersion reports whether the keystore file version can be read by this build.
func isSupportedVersion(v int) bool {
	return v == fileVersion || v == fileVersionLegacyScrypt
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
	if err := os.Rename(tmpPath, path); err != nil {
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
	plain := crypto.FromECDSA(priv)
	defer zeroBytes(plain)

	ew, err := buildV2Envelope(addr, plain, passphrase, networks)
	if err != nil {
		return "", err
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

// StoreExternal writes an address-only wallet record (no private key) under
// safeName(name).json. Such entries appear in [ListWallets] with External
// true and cannot be used with [LoadPrivateKey]. It returns the normalized
// base58 address written to the file.
func StoreExternal(keystoreDir, name, tronAddress string, networks []string) (string, error) {
	addr := strings.TrimSpace(tronAddress)
	if err := validateTronBase58Address(addr); err != nil {
		return "", err
	}
	if err := os.MkdirAll(keystoreDir, 0o700); err != nil {
		return "", err
	}
	ef := EncryptedWalletFile{
		Version:  fileVersion,
		External: true,
		Address:  addr,
		Networks: normalizeNetworkList(networks),
	}
	body, err := json.MarshalIndent(&ef, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(keystoreDir, safeName(name)+".json")
	if err := writeFileAtomic(path, body, 0o600); err != nil {
		return "", err
	}
	return addr, nil
}

// buildV2Envelope encrypts plain with argon2id+AES-256-GCM and returns the
// v2 on-disk envelope. plain is not zeroed by this function; callers own the
// buffer's lifetime.
func buildV2Envelope(addr string, plain, passphrase []byte, networks []string) (EncryptedWalletFile, error) {
	var zero EncryptedWalletFile
	if len(plain) != keyBytes {
		return zero, fmt.Errorf("invalid plaintext key length %d", len(plain))
	}
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return zero, err
	}
	normPass := NormalizePassphrase(passphrase)
	defer zeroBytes(normPass)
	dk := argon2.IDKey(normPass, salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	defer zeroBytes(dk)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return zero, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return zero, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return zero, err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)

	return EncryptedWalletFile{
		Version:       fileVersion,
		Address:       addr,
		Networks:      normalizeNetworkList(networks),
		KDF:           kdfArgon2id,
		SaltHex:       hex.EncodeToString(salt),
		Argon2Time:    argon2Time,
		Argon2Memory:  argon2Memory,
		Argon2Threads: argon2Threads,
		Argon2KeyLen:  argon2KeyLen,
		Cipher:        cipherAES256GCM,
		NonceHex:      hex.EncodeToString(nonce),
		DataHex:       hex.EncodeToString(ciphertext),
	}, nil
}

// migrationWarnWriter is the stream used for one-off human notices when a
// v1 scrypt file is silently migrated to v2 argon2id on the fly. Tests
// redirect it to a buffer to avoid polluting test output.
var migrationWarnWriter io.Writer = os.Stderr

// LoadPrivateKey decrypts the named wallet and returns an ECDSA private key
// (caller must wipe when done).
//
// The function reads both v1 (scrypt, legacy) and v2 (argon2id) files.
// After a successful v1 decryption it opportunistically re-encrypts the file
// in v2 format and writes the original aside as <name>.scrypt.bak (with a
// unix-ms suffix on collision). A migration failure is logged to stderr but
// does not fail the current operation: the caller receives its PrivKey and a
// later load will attempt the migration again.
func LoadPrivateKey(keystoreDir, name string, passphrase []byte) (*PrivKey, error) {
	body, err := readWalletFile(keystoreDir, name)
	if err != nil {
		return nil, err
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		return nil, err
	}
	if ew.External {
		return nil, fmt.Errorf("wallet %q is external (address-only) and has no private key", name)
	}
	if !isSupportedVersion(ew.Version) {
		return nil, fmt.Errorf("unsupported keystore version %d", ew.Version)
	}
	if err := validateVersionKDF(ew.Version, ew.KDF); err != nil {
		return nil, err
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
	if ew.Cipher != cipherAES256GCM {
		return nil, fmt.Errorf("unsupported cipher %q", ew.Cipher)
	}

	plain, err := decryptWalletData(ew, salt, nonce, data, passphrase)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plain)

	priv, err := crypto.ToECDSA(plain)
	if err != nil {
		return nil, err
	}
	got := address.PubkeyToAddress(priv.PublicKey).String()
	if got != ew.Address {
		zeroPriv(priv)
		return nil, fmt.Errorf("address mismatch in keystore")
	}

	if ew.Version == fileVersionLegacyScrypt {
		if mErr := migrateV1ToV2(keystoreDir, name, ew, plain, passphrase); mErr != nil {
			fmt.Fprintf(migrationWarnWriter,
				"warning: failed to migrate wallet %q to argon2id: %v\n", name, mErr)
		}
	}

	return &PrivKey{Key: priv, Address: ew.Address}, nil
}

// validateVersionKDF rejects cross-version KDF combinations. v1 files must
// use scrypt; v2 files must use argon2id. Any other combination is almost
// certainly a malformed or hand-edited file.
func validateVersionKDF(version int, kdf string) error {
	switch version {
	case fileVersionLegacyScrypt:
		if kdf != kdfScrypt {
			return fmt.Errorf("keystore v1 requires kdf %q, got %q", kdfScrypt, kdf)
		}
	case fileVersion:
		if kdf != kdfArgon2id {
			return fmt.Errorf("keystore v2 requires kdf %q, got %q", kdfArgon2id, kdf)
		}
	default:
		return fmt.Errorf("unsupported keystore version %d", version)
	}
	return nil
}

// decryptWalletData runs the correct KDF (scrypt for v1, argon2id for v2) and
// performs AES-256-GCM open. The returned plaintext must be zeroed by the caller.
func decryptWalletData(ew EncryptedWalletFile, salt, nonce, data, passphrase []byte) ([]byte, error) {
	normPass := NormalizePassphrase(passphrase)
	defer zeroBytes(normPass)

	var dk []byte
	switch ew.KDF {
	case kdfScrypt:
		var err error
		dk, err = scrypt.Key(normPass, salt, ew.N, ew.R, ew.P, 32)
		if err != nil {
			return nil, err
		}
	case kdfArgon2id:
		if ew.Argon2KeyLen == 0 || ew.Argon2Time == 0 || ew.Argon2Memory == 0 || ew.Argon2Threads == 0 {
			return nil, fmt.Errorf("keystore v2: missing argon2id parameters")
		}
		dk = argon2.IDKey(normPass, salt, ew.Argon2Time, ew.Argon2Memory, ew.Argon2Threads, ew.Argon2KeyLen)
	default:
		return nil, fmt.Errorf("unsupported kdf %q", ew.KDF)
	}
	defer zeroBytes(dk)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, errors.New("decryption failed (wrong passphrase or corrupt file)")
	}
	if len(plain) != keyBytes {
		zeroBytes(plain)
		return nil, fmt.Errorf("invalid decrypted key length")
	}
	return plain, nil
}

// migrateV1ToV2 rewrites a successfully-decrypted v1 scrypt file as v2 argon2id
// in place. It is invoked only after the plaintext key has been recovered and
// the on-disk address verified, so failure here never loses key material.
//
// Steps, in order:
//  1. Rename the original file to <safeName>.scrypt.bak (suffixed with a
//     unix-ms timestamp if a backup already exists).
//  2. Encrypt the plaintext with argon2id+AES-256-GCM and write a v2 JSON
//     atomically (temp file + rename) under the original path.
//
// On success an informational line is written to [migrationWarnWriter].
func migrateV1ToV2(keystoreDir, name string, old EncryptedWalletFile, plain, passphrase []byte) error {
	stem := safeName(name)
	origPath := filepath.Join(keystoreDir, stem+".json")
	backupPath := filepath.Join(keystoreDir, stem+legacyBackupSuffix)
	if _, err := os.Stat(backupPath); err == nil {
		backupPath = fmt.Sprintf("%s.%d", backupPath, time.Now().UnixMilli())
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat backup path: %w", err)
	}

	ew, err := buildV2Envelope(old.Address, plain, passphrase, old.Networks)
	if err != nil {
		return fmt.Errorf("build v2 envelope: %w", err)
	}
	body, err := json.MarshalIndent(&ew, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal v2 envelope: %w", err)
	}

	if err := os.Rename(origPath, backupPath); err != nil {
		return fmt.Errorf("backup v1 file: %w", err)
	}
	if err := writeFileAtomic(origPath, body, 0o600); err != nil {
		// Best-effort: try to restore original from backup so we don't
		// leave the wallet missing. If restore fails too, the user
		// still has the .bak file we just wrote.
		_ = os.Rename(backupPath, origPath)
		return fmt.Errorf("write v2 file: %w", err)
	}

	fmt.Fprintf(migrationWarnWriter,
		"note: migrated wallet %q from scrypt to argon2id; legacy backup saved as %s\n",
		name, filepath.Base(backupPath))
	return nil
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
