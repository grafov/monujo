package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("resolveHotSender", func() {
	It("returns the wallet name when --from is a keystore name", func() {
		dir := GinkgoT().TempDir()
		stashWallet(dir, "sender")

		name, err := resolveHotSender(dir, "sender")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("sender"))
	})

	It("resolves a base58 address to its wallet name when present in keystore", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "sender")

		name, err := resolveHotSender(dir, addr)
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("sender"))
	})

	It("rejects a base58 address that does not map to a local wallet", func() {
		dir := GinkgoT().TempDir()
		const foreign = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

		_, err := resolveHotSender(dir, foreign)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found in keystore"))
		Expect(err.Error()).To(ContainSubstring("pass the wallet name"))
	})

	It("surfaces the resolve error for a missing wallet name", func() {
		dir := GinkgoT().TempDir()

		_, err := resolveHotSender(dir, "ghost")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "ghost" not found`))
	})

	It("rejects an empty --from value", func() {
		dir := GinkgoT().TempDir()

		_, err := resolveHotSender(dir, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--from must be provided"))
	})
})
