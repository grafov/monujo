package keystore

import (
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

	It("sanitizes wallet names", func() {
		Expect(safeName("a b")).To(Equal("a_b"))
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
