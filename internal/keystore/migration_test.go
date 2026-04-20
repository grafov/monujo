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

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
func writeV1ScryptWallet(dir, name, privHex string, passphrase []byte, networks []string) string {
	GinkgoHelper()

	raw, err := hex.DecodeString(strings.TrimPrefix(privHex, "0x"))
	Expect(err).NotTo(HaveOccurred())

	priv, err := crypto.ToECDSA(raw)
	Expect(err).NotTo(HaveOccurred())
	addr := address.PubkeyToAddress(priv.PublicKey).String()

	Expect(os.MkdirAll(dir, 0o700)).To(Succeed())

	salt := make([]byte, 32)
	_, err = io.ReadFull(rand.Reader, salt)
	Expect(err).NotTo(HaveOccurred())

	dk, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, 32)
	Expect(err).NotTo(HaveOccurred())

	block, err := aes.NewCipher(dk)
	Expect(err).NotTo(HaveOccurred())

	gcm, err := cipher.NewGCM(block)
	Expect(err).NotTo(HaveOccurred())

	nonce := make([]byte, nonceLen)
	_, err = io.ReadFull(rand.Reader, nonce)
	Expect(err).NotTo(HaveOccurred())

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
	Expect(err).NotTo(HaveOccurred())

	path := filepath.Join(dir, safeName(name)+".json")
	Expect(os.WriteFile(path, body, 0o600)).To(Succeed())
	return addr
}

func readWalletJSON(dir, name string) EncryptedWalletFile {
	GinkgoHelper()

	body, err := os.ReadFile(filepath.Join(dir, safeName(name)+".json"))
	Expect(err).NotTo(HaveOccurred())

	var ew EncryptedWalletFile
	Expect(json.Unmarshal(body, &ew)).To(Succeed())
	return ew
}

var _ = Describe("keystore migration and compatibility", func() {
	It("stores new files as v2 argon2id", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		_, err = Store(dir, "new1", hexKey, pass, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		ew := readWalletJSON(dir, "new1")
		Expect(ew.Version).To(Equal(fileVersion))
		Expect(ew.KDF).To(Equal(kdfArgon2id))
		Expect(ew.Argon2Time).To(BeNumerically(">", 0))
		Expect(ew.Argon2Memory).To(BeNumerically(">", 0))
		Expect(ew.Argon2Threads).To(BeNumerically(">", 0))
		Expect(ew.Argon2KeyLen).To(BeNumerically(">", 0))
		Expect(ew.N).To(Equal(0))
		Expect(ew.R).To(Equal(0))
		Expect(ew.P).To(Equal(0))
		Expect(ew.Cipher).To(Equal(cipherAES256GCM))

		salt, err := hex.DecodeString(ew.SaltHex)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(salt)).To(Equal(saltLen))
	})

	It("auto-migrates v1 scrypt wallet on successful load", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		wantAddr := writeV1ScryptWallet(dir, "legacy", hexKey, pass, []string{"mainnet"})

		var warn bytes.Buffer
		prev := migrationWarnWriter
		migrationWarnWriter = &warn
		DeferCleanup(func() { migrationWarnWriter = prev })

		pk, err := LoadPrivateKey(dir, "legacy", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()
		Expect(pk.Address).To(Equal(wantAddr))
		Expect(warn.String()).To(ContainSubstring("migrated wallet"))

		ew := readWalletJSON(dir, "legacy")
		Expect(ew.Version).To(Equal(fileVersion))
		Expect(ew.KDF).To(Equal(kdfArgon2id))
		Expect(ew.N).To(Equal(0))
		Expect(ew.R).To(Equal(0))
		Expect(ew.P).To(Equal(0))

		backupPath := filepath.Join(dir, safeName("legacy")+legacyBackupSuffix)
		_, err = os.Stat(backupPath)
		Expect(err).NotTo(HaveOccurred())

		bkp, err := os.ReadFile(backupPath)
		Expect(err).NotTo(HaveOccurred())

		var bkpEw EncryptedWalletFile
		Expect(json.Unmarshal(bkp, &bkpEw)).To(Succeed())
		Expect(bkpEw.Version).To(Equal(fileVersionLegacyScrypt))
		Expect(bkpEw.KDF).To(Equal(kdfScrypt))

		pk2, err := LoadPrivateKey(dir, "legacy", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk2.Wipe()
		Expect(pk2.Address).To(Equal(wantAddr))
	})

	It("preserves network metadata during auto-migration", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("pw")
		defer ZeroPassphrase(pass)

		writeV1ScryptWallet(dir, "w", hexKey, pass, []string{"shasta", "mainnet"})

		var warn bytes.Buffer
		prev := migrationWarnWriter
		migrationWarnWriter = &warn
		DeferCleanup(func() { migrationWarnWriter = prev })

		pk, err := LoadPrivateKey(dir, "w", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		nets, err := ReadWalletNetworks(dir, "w")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet", "shasta"}))
	})

	It("creates timestamped backup if default backup path already exists", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("pw")
		defer ZeroPassphrase(pass)

		writeV1ScryptWallet(dir, "w", hexKey, pass, nil)
		preexisting := filepath.Join(dir, safeName("w")+legacyBackupSuffix)
		Expect(os.WriteFile(preexisting, []byte("PLACEHOLDER"), 0o600)).To(Succeed())

		var warn bytes.Buffer
		prev := migrationWarnWriter
		migrationWarnWriter = &warn
		DeferCleanup(func() { migrationWarnWriter = prev })

		pk, err := LoadPrivateKey(dir, "w", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		body, err := os.ReadFile(preexisting)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("PLACEHOLDER"))

		entries, err := os.ReadDir(dir)
		Expect(err).NotTo(HaveOccurred())
		matched := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), safeName("w")+legacyBackupSuffix+".") {
				matched++
			}
		}
		Expect(matched).To(Equal(1))
	})

	It("does not migrate v1 wallet when password is wrong", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		correct := []byte("right")
		defer ZeroPassphrase(correct)
		writeV1ScryptWallet(dir, "w", hexKey, correct, nil)

		var warn bytes.Buffer
		prev := migrationWarnWriter
		migrationWarnWriter = &warn
		DeferCleanup(func() { migrationWarnWriter = prev })

		wrong := []byte("bad")
		defer ZeroPassphrase(wrong)
		_, err = LoadPrivateKey(dir, "w", wrong)
		Expect(err).To(HaveOccurred())

		ew := readWalletJSON(dir, "w")
		Expect(ew.Version).To(Equal(fileVersionLegacyScrypt))
		Expect(ew.KDF).To(Equal(kdfScrypt))

		backupPath := filepath.Join(dir, safeName("w")+legacyBackupSuffix)
		_, err = os.Stat(backupPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
		Expect(warn.Len()).To(Equal(0))
	})

	It("rejects files with inconsistent version and KDF combinations", func() {
		dir := GinkgoT().TempDir()
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
			Expect(err).NotTo(HaveOccurred())

			path := filepath.Join(dir, safeName(tc.name)+".json")
			Expect(os.WriteFile(path, body, 0o600)).To(Succeed())

			_, err = LoadPrivateKey(dir, tc.name, pass)
			Expect(err).To(HaveOccurred(), tc.name)
		}
	})

	It("loads wallet across NFC and NFD equivalent passphrases", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		nfd := []byte("cafe\u0301-pass-1")
		defer ZeroPassphrase(nfd)

		addr, err := Store(dir, "u1", hexKey, nfd, nil)
		Expect(err).NotTo(HaveOccurred())

		nfc := []byte("caf\u00e9-pass-1")
		defer ZeroPassphrase(nfc)

		pk, err := LoadPrivateKey(dir, "u1", nfc)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()
		Expect(pk.Address).To(Equal(addr))
	})

	It("loads wallet across ligature compatibility forms", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		withLig := []byte("\uFB01le-secret")
		defer ZeroPassphrase(withLig)

		_, err = Store(dir, "u2", hexKey, withLig, nil)
		Expect(err).NotTo(HaveOccurred())

		ascii := []byte("file-secret")
		defer ZeroPassphrase(ascii)
		pk, err := LoadPrivateKey(dir, "u2", ascii)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()
	})

	It("rejects wrong non-ASCII passphrase", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		good := []byte("пароль")
		defer ZeroPassphrase(good)

		_, err = Store(dir, "u3", hexKey, good, nil)
		Expect(err).NotTo(HaveOccurred())

		bad := []byte("пароль1")
		defer ZeroPassphrase(bad)
		_, err = LoadPrivateKey(dir, "u3", bad)
		Expect(err).To(HaveOccurred())
	})

	It("updates networks on both v1 and v2 wallets without rewriting version semantics", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("pw")
		defer ZeroPassphrase(pass)

		writeV1ScryptWallet(dir, "v1", hexKey, pass, nil)
		Expect(EnsureNetworkRecorded(dir, "v1", "mainnet")).To(Succeed())

		nets, err := ReadWalletNetworks(dir, "v1")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet"}))

		v1file := readWalletJSON(dir, "v1")
		Expect(v1file.Version).To(Equal(fileVersionLegacyScrypt))
		Expect(v1file.KDF).To(Equal(kdfScrypt))

		_, err = Store(dir, "v2", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(EnsureNetworkRecorded(dir, "v2", "shasta")).To(Succeed())

		v2file := readWalletJSON(dir, "v2")
		Expect(v2file.Version).To(Equal(fileVersion))
		Expect(v2file.KDF).To(Equal(kdfArgon2id))
	})
})
