package app

import (
	"context"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HotService", func() {
	It("creates wallet and then loads it from keystore", func() {
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkShasta, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		svc := NewHotService(cfg)
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		Expect(svc.CreateWallet("w1", pass, false, false)).To(Succeed())

		pk, err := keystore.LoadPrivateKey(dir, "w1", pass)
		Expect(err).NotTo(HaveOccurred())
		defer pk.Wipe()

		Expect(pk.Address).NotTo(BeEmpty())
	})

	It("delegates account activation to SendTRX path and surfaces errors", func() {
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		svc := NewHotService(cfg)
		err = svc.ActivateAccount(context.Background(), "missing", "TInvalid", "1", []byte("x"))
		Expect(err).To(HaveOccurred())
	})

	It("PrintBalanceByName errors for unknown wallet without asking for passphrase", func() {
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		svc := NewHotService(cfg)
		err = svc.PrintBalanceByName(context.Background(), "no-such-wallet")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})
})
