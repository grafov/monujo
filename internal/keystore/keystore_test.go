package keystore

import (
	"encoding/json"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("keystore basic operations", func() {
	It("stores and loads a private key roundtrip", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		addr, err := Store(dir, "t1", hexKey, pass, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		pk, err := LoadPrivateKey(dir, "t1", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		Expect(pk.Address).To(Equal(addr))
	})

	It("validates wallet names before writing", func() {
		Expect(safeName("a b")).To(Equal("a_b"))
		Expect(ValidateWalletName("a_b-1")).To(Succeed())
		Expect(ValidateWalletName("a b")).To(HaveOccurred())
		Expect(ValidateWalletName("   ")).To(HaveOccurred())
	})

	It("generates and stores wallet key material", func() {
		dir := GinkgoT().TempDir()
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		addr, privHex, err := GenerateAndStore(dir, "gen1", pass, []string{"shasta"})
		Expect(err).NotTo(HaveOccurred())
		Expect(privHex).To(HaveLen(64))

		pk, err := LoadPrivateKey(dir, "gen1", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		Expect(pk.Address).To(Equal(addr))
		gotHex := hex.EncodeToString(crypto.FromECDSA(pk.Key))
		Expect(gotHex).To(Equal(privHex))
	})

	It("lists wallets for empty and missing directories", func() {
		dir := GinkgoT().TempDir()
		got, err := ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeEmpty())

		missing := filepath.Join(dir, "nope")
		got, err = ListWallets(missing)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeEmpty())
	})

	It("lists stored wallets and ignores non-wallet files", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		addr, err := Store(dir, "alpha", hexKey, pass, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		_, err = Store(dir, "beta", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		list, err := ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(2))
		Expect(list[0].Name).To(Equal("alpha"))
		Expect(list[0].Address).To(Equal(addr))
		Expect(list[0].External).To(BeFalse())
		Expect(list[1].Name).To(Equal("beta"))

		err = os.WriteFile(filepath.Join(dir, "junk.txt"), []byte("x"), 0o600)
		Expect(err).NotTo(HaveOccurred())

		list2, err := ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(list2).To(HaveLen(2))
	})

	It("detects whether network confirmation is required", func() {
		Expect(NetworkOperationNeedsConfirmation(nil, "mainnet")).To(BeFalse())
		Expect(NetworkOperationNeedsConfirmation([]string{}, "mainnet")).To(BeFalse())
		Expect(NetworkOperationNeedsConfirmation([]string{"shasta"}, "mainnet")).To(BeTrue())
		Expect(NetworkOperationNeedsConfirmation([]string{"mainnet", "shasta"}, "mainnet")).To(BeFalse())
	})

	It("reads networks and ensures newly used network is recorded", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		_, err = Store(dir, "w1", hexKey, pass, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		nets, err := ReadWalletNetworks(dir, "w1")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet"}))

		Expect(EnsureNetworkRecorded(dir, "w1", "shasta")).To(Succeed())

		nets, err = ReadWalletNetworks(dir, "w1")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet", "shasta"}))

		Expect(EnsureNetworkRecorded(dir, "w1", "shasta")).To(Succeed())

		pk, err := LoadPrivateKey(dir, "w1", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()
	})

	It("reports wallet existence", func() {
		dir := GinkgoT().TempDir()

		missing := filepath.Join(dir, "does-not-exist")
		got, err := WalletExists(missing, "anything")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeFalse())

		got, err = WalletExists(dir, "absent")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeFalse())

		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())
		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		_, err = Store(dir, "present", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		got, err = WalletExists(dir, "present")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTrue())

		// Legacy lookups still pass through safeName so old files remain readable.
		_, err = Store(dir, "a_b", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		got, err = WalletExists(dir, "a b")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTrue())
	})

	It("rejects unsupported KDF parameters before decrypting", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		_, err = Store(dir, "evil", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		path := filepath.Join(dir, "evil.json")
		body, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())

		var ew EncryptedWalletFile
		Expect(json.Unmarshal(body, &ew)).To(Succeed())
		ew.Argon2Memory = 1 << 30
		body, err = json.MarshalIndent(&ew, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path, body, 0o600)).To(Succeed())

		_, err = LoadPrivateKey(dir, "evil", pass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported argon2id parameters"))
	})

	It("loads legacy file without networks and can append network metadata", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer ZeroPassphrase(pass)

		_, err = Store(dir, "legacy", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		nets, err := ReadWalletNetworks(dir, "legacy")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(BeEmpty())

		pk, err := LoadPrivateKey(dir, "legacy", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		Expect(EnsureNetworkRecorded(dir, "legacy", "mainnet")).To(Succeed())

		nets, err = ReadWalletNetworks(dir, "legacy")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet"}))
	})
})

var _ = Describe("external address-only keystore entries", func() {
	const validAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

	It("stores, lists with External=true, and rejects LoadPrivateKey", func() {
		dir := GinkgoT().TempDir()
		gotAddr, err := StoreExternal(dir, "watch1", validAddr, []string{"shasta"})
		Expect(err).NotTo(HaveOccurred())
		Expect(gotAddr).To(Equal(validAddr))

		list, err := ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(1))
		Expect(list[0].Name).To(Equal("watch1"))
		Expect(list[0].Address).To(Equal(validAddr))
		Expect(list[0].External).To(BeTrue())
		Expect(list[0].Networks).To(Equal([]string{"shasta"}))

		pk, err := LoadPrivateKey(dir, "watch1", []byte("any-pass"))
		Expect(err).To(HaveOccurred())
		Expect(pk).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("external"))
	})

	It("ReadWalletNetworks works without KDF fields", func() {
		dir := GinkgoT().TempDir()
		_, err := StoreExternal(dir, "ext", validAddr, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		nets, err := ReadWalletNetworks(dir, "ext")
		Expect(err).NotTo(HaveOccurred())
		Expect(nets).To(Equal([]string{"mainnet"}))
	})

	It("EnsureNetworkRecorded refuses external wallets", func() {
		dir := GinkgoT().TempDir()
		_, err := StoreExternal(dir, "ext", validAddr, nil)
		Expect(err).NotTo(HaveOccurred())

		err = EnsureNetworkRecorded(dir, "ext", "shasta")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("external"))
	})

	It("rejects invalid Tron address", func() {
		dir := GinkgoT().TempDir()
		_, err := StoreExternal(dir, "bad", "not-an-address", nil)
		Expect(err).To(HaveOccurred())
	})
})
