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
