package tron_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

var _ = Describe("USDTContractForNetwork", func() {
	DescribeTable("returns the canonical USDT contract per network",
		func(net config.Network, want string) {
			Expect(tron.USDTContractForNetwork(net)).To(Equal(want))
		},
		Entry("mainnet returns canonical Tether contract",
			config.NetworkMainnet, tron.MainnetUSDTContract),
		Entry("shasta returns the test USDT-like contract",
			config.NetworkShasta, tron.ShastaTestUSDTContract),
		Entry("unknown network falls back to mainnet (default branch)",
			config.Network("nile"), tron.MainnetUSDTContract),
		Entry("empty network falls back to mainnet (default branch)",
			config.Network(""), tron.MainnetUSDTContract),
	)

	It("exposes mainnet contract that survives base58 validation", func() {
		Expect(tron.ValidateBase58Address(tron.MainnetUSDTContract)).To(Succeed())
	})

	It("exposes shasta contract that survives base58 validation", func() {
		Expect(tron.ValidateBase58Address(tron.ShastaTestUSDTContract)).To(Succeed())
	})
})
