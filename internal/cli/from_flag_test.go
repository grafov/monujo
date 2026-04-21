package cli

import (
	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// stashWallet creates a real keystore file in dir under the given name
// and returns its Tron base58 address. Uses app.HotService so wallets
// look exactly the way production code writes them.
func stashWallet(dir, name string) string {
	cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
	Expect(err).NotTo(HaveOccurred())
	pass := []byte("stash-passphrase-16+chars!!")
	defer keystore.ZeroPassphrase(pass)
	svc := app.NewHotService(cfg)
	Expect(svc.CreateWallet(name, pass, false, false)).To(Succeed())
	items, err := keystore.ListWallets(dir)
	Expect(err).NotTo(HaveOccurred())
	for _, w := range items {
		if w.Name == name {
			return w.Address
		}
	}
	Fail("wallet not found after create: " + name)
	return ""
}

var _ = Describe("resolveWalletFrom", func() {
	It("rejects an empty --from", func() {
		dir := GinkgoT().TempDir()
		_, _, ok, err := resolveWalletFrom(dir, "")
		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring("--from must be provided"))
	})

	It("errors on a wallet name that is not present in the keystore", func() {
		dir := GinkgoT().TempDir()
		_, _, ok, err := resolveWalletFrom(dir, "nonexistent")
		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring(`wallet "nonexistent" not found`))
	})

	It("resolves a wallet name to its stored address", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "alice")

		name, got, ok, err := resolveWalletFrom(dir, "alice")
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(name).To(Equal("alice"))
		Expect(got).To(Equal(addr))
	})

	It("resolves a base58 address that matches a local wallet back to its name", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "bob")

		name, got, ok, err := resolveWalletFrom(dir, addr)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(name).To(Equal("bob"))
		Expect(got).To(Equal(addr))
	})

	It("accepts a valid base58 address absent from the keystore with ok=false", func() {
		dir := GinkgoT().TempDir()
		const foreign = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

		name, got, ok, err := resolveWalletFrom(dir, foreign)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
		Expect(name).To(BeEmpty())
		Expect(got).To(Equal(foreign))
	})

	It("treats gibberish neither base58 nor matching a name as a name lookup error", func() {
		dir := GinkgoT().TempDir()
		_, _, ok, err := resolveWalletFrom(dir, "not-an-addr-and-not-a-name")
		Expect(err).To(HaveOccurred())
		Expect(ok).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("trims surrounding whitespace before classification", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "carol")

		name, got, ok, err := resolveWalletFrom(dir, "   carol   ")
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(name).To(Equal("carol"))
		Expect(got).To(Equal(addr))
	})
})

var _ = Describe("resolveRecipient", func() {
	It("rejects an empty --to value", func() {
		dir := GinkgoT().TempDir()
		_, err := resolveRecipient(dir, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--to must be provided"))
	})

	It("passes a valid base58 address through without touching the keystore", func() {
		// No keystore file exists in dir, yet a raw address must be
		// accepted verbatim (recipient is arbitrary).
		dir := GinkgoT().TempDir()
		const foreign = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

		got, err := resolveRecipient(dir, foreign)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(foreign))
	})

	It("resolves a local wallet name to its address", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "dave")

		got, err := resolveRecipient(dir, "dave")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(addr))
	})

	It("errors when the name is not in the keystore", func() {
		dir := GinkgoT().TempDir()

		_, err := resolveRecipient(dir, "ghost")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "ghost" not found`))
	})
})
