package keystore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"golang.org/x/crypto/scrypt"
)

// Legacy scrypt parameters used only by tests to synthesize pre-migration v1
// wallet files. Production code no longer writes scrypt; it only reads the
// n/r/p values that happen to be stored in legacy files.
const (
	scryptN = 32768
	scryptR = 8
	scryptP = 1
)

// writeV1ScryptWallet produces a v1 scrypt+AES-256-GCM file exactly as
// pre-migration monujo releases did. Used by migration tests.
func writeV1ScryptWallet(t *testing.T, dir, name string, privHex string, passphrase []byte, networks []string) string {
	t.Helper()
	raw, err := hex.DecodeString(strings.TrimPrefix(privHex, "0x"))
	if err != nil {
		t.Fatalf("decode priv hex: %v", err)
	}
	priv, err := crypto.ToECDSA(raw)
	if err != nil {
		t.Fatalf("to ecdsa: %v", err)
	}
	addr := address.PubkeyToAddress(priv.PublicKey).String()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		t.Fatalf("salt: %v", err)
	}
	dk, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		t.Fatalf("scrypt: %v", err)
	}
	block, err := aes.NewCipher(dk)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("nonce: %v", err)
	}
	ciphertext := gcm.Seal(nil, nonce, raw, nil)

	ew := EncryptedWalletFile{
		Version:  fileVersionLegacyScrypt,
		Address:  addr,
		Networks: normalizeNetworkList(networks),
		KDF:      kdfScrypt,
		SaltHex:  hex.EncodeToString(salt),
		N:        scryptN,
		R:        scryptR,
		P:        scryptP,
		Cipher:   cipherAES256GCM,
		NonceHex: hex.EncodeToString(nonce),
		DataHex:  hex.EncodeToString(ciphertext),
	}
	body, err := json.MarshalIndent(ew, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, safeName(name)+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return addr
}

func readWalletJSON(t *testing.T, dir, name string) EncryptedWalletFile {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, safeName(name)+".json"))
	if err != nil {
		t.Fatalf("read wallet: %v", err)
	}
	var ew EncryptedWalletFile
	if err := json.Unmarshal(body, &ew); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return ew
}

func TestStoreWritesV2Argon2id(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)

	if _, err := Store(dir, "new1", hexKey, pass, []string{"mainnet"}); err != nil {
		t.Fatal(err)
	}
	ew := readWalletJSON(t, dir, "new1")
	if ew.Version != fileVersion {
		t.Fatalf("version: want %d, got %d", fileVersion, ew.Version)
	}
	if ew.KDF != kdfArgon2id {
		t.Fatalf("kdf: want %q, got %q", kdfArgon2id, ew.KDF)
	}
	if ew.Argon2Time == 0 || ew.Argon2Memory == 0 || ew.Argon2Threads == 0 || ew.Argon2KeyLen == 0 {
		t.Fatalf("argon2 params empty: %+v", ew)
	}
	if ew.N != 0 || ew.R != 0 || ew.P != 0 {
		t.Fatalf("scrypt legacy params should be zero: %+v", ew)
	}
	if ew.Cipher != cipherAES256GCM {
		t.Fatalf("cipher: %q", ew.Cipher)
	}
	salt, err := hex.DecodeString(ew.SaltHex)
	if err != nil {
		t.Fatalf("salt hex: %v", err)
	}
	if len(salt) != saltLen {
		t.Fatalf("salt len: want %d, got %d", saltLen, len(salt))
	}
}

func TestLoadV1ScryptAutoMigrates(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)

	wantAddr := writeV1ScryptWallet(t, dir, "legacy", hexKey, pass, []string{"mainnet"})

	var warn bytes.Buffer
	prev := migrationWarnWriter
	migrationWarnWriter = &warn
	defer func() { migrationWarnWriter = prev }()

	pk, err := LoadPrivateKey(dir, "legacy", pass)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer pk.Wipe()
	if pk.Address != wantAddr {
		t.Fatalf("address: want %s, got %s", wantAddr, pk.Address)
	}
	if !strings.Contains(warn.String(), "migrated wallet") {
		t.Fatalf("expected migration note in stderr, got %q", warn.String())
	}

	ew := readWalletJSON(t, dir, "legacy")
	if ew.Version != fileVersion {
		t.Fatalf("after migrate: version want %d, got %d", fileVersion, ew.Version)
	}
	if ew.KDF != kdfArgon2id {
		t.Fatalf("after migrate: kdf want %q, got %q", kdfArgon2id, ew.KDF)
	}
	if ew.N != 0 || ew.R != 0 || ew.P != 0 {
		t.Fatalf("after migrate: scrypt params must be zero: %+v", ew)
	}

	backupPath := filepath.Join(dir, safeName("legacy")+legacyBackupSuffix)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup at %s: %v", backupPath, err)
	}
	bkp, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var bkpEw EncryptedWalletFile
	if err := json.Unmarshal(bkp, &bkpEw); err != nil {
		t.Fatalf("unmarshal backup: %v", err)
	}
	if bkpEw.Version != fileVersionLegacyScrypt || bkpEw.KDF != kdfScrypt {
		t.Fatalf("backup not v1/scrypt: %+v", bkpEw)
	}

	pk2, err := LoadPrivateKey(dir, "legacy", pass)
	if err != nil {
		t.Fatalf("reload after migrate: %v", err)
	}
	defer pk2.Wipe()
	if pk2.Address != wantAddr {
		t.Fatal("reload address mismatch")
	}
}

func TestLoadV1ScryptAutoMigratesPreservesNetworks(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("pw")
	defer ZeroPassphrase(pass)

	writeV1ScryptWallet(t, dir, "w", hexKey, pass, []string{"shasta", "mainnet"})
	var warn bytes.Buffer
	prev := migrationWarnWriter
	migrationWarnWriter = &warn
	defer func() { migrationWarnWriter = prev }()

	pk, err := LoadPrivateKey(dir, "w", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()

	nets, err := ReadWalletNetworks(dir, "w")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 2 || nets[0] != "mainnet" || nets[1] != "shasta" {
		t.Fatalf("networks after migrate: %v", nets)
	}
}

func TestLoadV1ScryptAutoMigratesBackupCollision(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("pw")
	defer ZeroPassphrase(pass)

	writeV1ScryptWallet(t, dir, "w", hexKey, pass, nil)
	preexisting := filepath.Join(dir, safeName("w")+legacyBackupSuffix)
	if err := os.WriteFile(preexisting, []byte("PLACEHOLDER"), 0o600); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	prev := migrationWarnWriter
	migrationWarnWriter = &warn
	defer func() { migrationWarnWriter = prev }()

	pk, err := LoadPrivateKey(dir, "w", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()

	body, err := os.ReadFile(preexisting)
	if err != nil {
		t.Fatalf("read original backup: %v", err)
	}
	if string(body) != "PLACEHOLDER" {
		t.Fatalf("original backup was overwritten: %q", string(body))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), safeName("w")+legacyBackupSuffix+".") {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("want exactly one timestamped backup, got %d; entries: %v", matched, entries)
	}
}

func TestLoadV1ScryptWrongPasswordDoesNotMigrate(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	correct := []byte("right")
	defer ZeroPassphrase(correct)
	writeV1ScryptWallet(t, dir, "w", hexKey, correct, nil)

	var warn bytes.Buffer
	prev := migrationWarnWriter
	migrationWarnWriter = &warn
	defer func() { migrationWarnWriter = prev }()

	wrong := []byte("bad")
	defer ZeroPassphrase(wrong)
	if _, err := LoadPrivateKey(dir, "w", wrong); err == nil {
		t.Fatal("expected error with wrong password")
	}

	ew := readWalletJSON(t, dir, "w")
	if ew.Version != fileVersionLegacyScrypt || ew.KDF != kdfScrypt {
		t.Fatalf("file should still be v1/scrypt: %+v", ew)
	}
	backupPath := filepath.Join(dir, safeName("w")+legacyBackupSuffix)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup must not exist after failed decrypt: err=%v", err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected migration warning on failure: %q", warn.String())
	}
}

func TestLoadRejectsInconsistent(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		version int
		kdf     string
	}{
		{"v2-scrypt", fileVersion, kdfScrypt},
		{"v1-argon", fileVersionLegacyScrypt, kdfArgon2id},
	}
	pass := []byte("pw")
	defer ZeroPassphrase(pass)
	for _, tc := range cases {
		ew := EncryptedWalletFile{
			Version:       tc.version,
			Address:       "TPlaceholder",
			KDF:           tc.kdf,
			SaltHex:       hex.EncodeToString(make([]byte, 16)),
			N:             scryptN,
			R:             scryptR,
			P:             scryptP,
			Argon2Time:    argon2Time,
			Argon2Memory:  argon2Memory,
			Argon2Threads: argon2Threads,
			Argon2KeyLen:  argon2KeyLen,
			Cipher:        cipherAES256GCM,
			NonceHex:      hex.EncodeToString(make([]byte, 12)),
			DataHex:       hex.EncodeToString(make([]byte, 48)),
		}
		body, err := json.MarshalIndent(&ew, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, safeName(tc.name)+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = LoadPrivateKey(dir, tc.name, pass)
		if err == nil {
			t.Fatalf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestEnsureNetworkRecordedWorksOnV1AndV2(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("pw")
	defer ZeroPassphrase(pass)

	writeV1ScryptWallet(t, dir, "v1", hexKey, pass, nil)
	if err := EnsureNetworkRecorded(dir, "v1", "mainnet"); err != nil {
		t.Fatalf("v1 ensure: %v", err)
	}
	nets, err := ReadWalletNetworks(dir, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0] != "mainnet" {
		t.Fatalf("v1 networks: %v", nets)
	}
	v1file := readWalletJSON(t, dir, "v1")
	if v1file.Version != fileVersionLegacyScrypt || v1file.KDF != kdfScrypt {
		t.Fatalf("v1 should stay v1 after metadata update: %+v", v1file)
	}
	if _, err := Store(dir, "v2", hexKey, pass, nil); err != nil {
		t.Fatal(err)
	}
	if err := EnsureNetworkRecorded(dir, "v2", "shasta"); err != nil {
		t.Fatalf("v2 ensure: %v", err)
	}
	v2file := readWalletJSON(t, dir, "v2")
	if v2file.Version != fileVersion || v2file.KDF != kdfArgon2id {
		t.Fatalf("v2 envelope changed: %+v", v2file)
	}
}
