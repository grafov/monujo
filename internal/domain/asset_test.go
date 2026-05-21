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

package domain

import (
	"math/big"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("asset parsing and formatting", func() {
	It("parses USDT decimal amounts", func() {
		v, err := ParseUSDTDecimal("1.5")
		Expect(err).NotTo(HaveOccurred())
		Expect(v.String()).To(Equal("1500000"))
	})

	It("parses TRX decimal amounts to sun", func() {
		sun, err := ParseTRXDecimal("2.5")
		Expect(err).NotTo(HaveOccurred())
		Expect(sun).To(Equal(int64(2500000)))
	})

	It("formats minimal USDT amounts", func() {
		s := FormatMinimalUSDT(big.NewInt(1500000))
		Expect(s).To(Equal("1.5"))
	})
})
