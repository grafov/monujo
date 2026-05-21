// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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

func stashExternal(dir, name, tronAddr string) {
	_, err := keystore.StoreExternal(dir, name, tronAddr, []string{"mainnet"})
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("resolveWalletFrom", func() {
	const foreignAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

	It("rejects an empty value and names the --from flag", func() {
		dir := GinkgoT().TempDir()
		res, err := resolveWalletFrom(dir, "", "--from")
		Expect(err).To(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring("--from must be provided"))
	})

	It("rejects an empty value and names the --addr flag", func() {
		dir := GinkgoT().TempDir()
		res, err := resolveWalletFrom(dir, "", "--addr")
		Expect(err).To(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring("--addr must be provided"))
	})

	It("errors on a wallet name that is not present in the keystore", func() {
		dir := GinkgoT().TempDir()
		res, err := resolveWalletFrom(dir, "nonexistent", "--from")
		Expect(err).To(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring(`wallet "nonexistent" not found`))
	})

	It("resolves a wallet name to its stored address", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "alice")

		res, err := resolveWalletFrom(dir, "alice", "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeTrue())
		Expect(res.External).To(BeFalse())
		Expect(res.Name).To(Equal("alice"))
		Expect(res.Address).To(Equal(addr))
	})

	It("resolves a base58 address that matches a local wallet back to its name", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "bob")

		res, err := resolveWalletFrom(dir, addr, "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeTrue())
		Expect(res.External).To(BeFalse())
		Expect(res.Name).To(Equal("bob"))
		Expect(res.Address).To(Equal(addr))
	})

	It("accepts a valid base58 address absent from the keystore with KnownInKeystore=false", func() {
		dir := GinkgoT().TempDir()

		res, err := resolveWalletFrom(dir, foreignAddr, "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeFalse())
		Expect(res.External).To(BeFalse())
		Expect(res.Name).To(BeEmpty())
		Expect(res.Address).To(Equal(foreignAddr))
	})

	It("resolves an external wallet by name with External=true", func() {
		dir := GinkgoT().TempDir()
		stashExternal(dir, "custodian", foreignAddr)

		res, err := resolveWalletFrom(dir, "custodian", "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeTrue())
		Expect(res.External).To(BeTrue())
		Expect(res.Name).To(Equal("custodian"))
		Expect(res.Address).To(Equal(foreignAddr))
	})

	It("resolves an external wallet by base58 address with External=true", func() {
		dir := GinkgoT().TempDir()
		stashExternal(dir, "custodian", foreignAddr)

		res, err := resolveWalletFrom(dir, foreignAddr, "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeTrue())
		Expect(res.External).To(BeTrue())
		Expect(res.Name).To(Equal("custodian"))
		Expect(res.Address).To(Equal(foreignAddr))
	})

	It("treats gibberish neither base58 nor matching a name as a name lookup error", func() {
		dir := GinkgoT().TempDir()
		res, err := resolveWalletFrom(dir, "not-an-addr-and-not-a-name", "--from")
		Expect(err).To(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeFalse())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("trims surrounding whitespace before classification", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "carol")

		res, err := resolveWalletFrom(dir, "   carol   ", "--from")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.KnownInKeystore).To(BeTrue())
		Expect(res.Name).To(Equal("carol"))
		Expect(res.Address).To(Equal(addr))
	})
})

var _ = Describe("resolveRecipient", func() {
	const foreignAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

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

		got, err := resolveRecipient(dir, foreignAddr)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(foreignAddr))
	})

	It("resolves a local wallet name to its address", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "dave")

		got, err := resolveRecipient(dir, "dave")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(addr))
	})

	It("resolves an external wallet name to its address", func() {
		dir := GinkgoT().TempDir()
		stashExternal(dir, "payee", foreignAddr)

		got, err := resolveRecipient(dir, "payee")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(foreignAddr))
	})

	It("errors when the name is not in the keystore", func() {
		dir := GinkgoT().TempDir()

		_, err := resolveRecipient(dir, "ghost")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "ghost" not found`))
	})

	It("reports address-like typos as invalid Tron addresses", func() {
		dir := GinkgoT().TempDir()

		_, err := resolveRecipient(dir, "T000000000000000000000000000000000")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid Tron address"))
	})
})
