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

	"github.com/grafov/monujo/internal/tron"
)

var _ = Describe("ValidateBase58Address", func() {
	DescribeTable("accepts valid mainnet base58 addresses",
		func(addr string) {
			Expect(tron.ValidateBase58Address(addr)).To(Succeed())
		},
		Entry("USDT TRC-20 mainnet contract", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
		Entry("Binance hot wallet", "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"),
		Entry("Justin Sun account", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"),
	)

	DescribeTable("rejects malformed addresses",
		func(addr string) {
			Expect(tron.ValidateBase58Address(addr)).NotTo(Succeed())
		},
		Entry("empty string", ""),
		Entry("whitespace only", "   "),
		Entry("hex address without T prefix", "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"),
		Entry("contains base58-illegal char 0", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj60"),
		Entry("contains base58-illegal char O", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLjOt"),
		Entry("contains base58-illegal char I", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLjIt"),
		Entry("contains base58-illegal char l", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLjlt"),
		Entry("too short", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj"),
		Entry("too long", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6tAAAA"),
		Entry("flipped checksum byte", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6u"),
	)

	When("address fails validation", func() {
		It("wraps the underlying decode error", func() {
			err := tron.ValidateBase58Address("not-a-tron-address")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid Tron address"))
		})
	})
})
