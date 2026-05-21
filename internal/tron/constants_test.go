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
