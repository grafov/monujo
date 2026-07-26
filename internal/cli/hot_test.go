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
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/monujo/internal/keystore"
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

	It("rejects an external wallet name for sending", func() {
		dir := GinkgoT().TempDir()
		const foreign = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
		_, err := keystore.StoreExternal(dir, "extpay", foreign, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = resolveHotSender(dir, "extpay")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("external"))
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

var _ = Describe("hot send confirmation", func() {
	BeforeEach(func() {
		networkFlag = "mainnet"
		outputFlag = "table"
		httpURLFlag = ""
		grpcURLFlag = ""
		grpcInsecureFlag = false
		writeTransportFlag = "rest"
		allowInsecureHTTPFlag = false
		apiKeyFlag = ""
		keystoreDirFlag = ""
	})

	It("rejects non-interactive hot send without --yes before passphrase", func() {
		dir := GinkgoT().TempDir()
		stashWallet(dir, "sender")
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"hot", "send-trx",
			"--from", "sender",
			"--to", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
			"--amount", "1",
		})
		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("without --yes"))
	})
})
