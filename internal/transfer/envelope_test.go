package transfer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("payload envelope marshaling", func() {
	It("round-trips unsigned payloads", func() {
		u := UnsignedTransferPayload{
			Chain:      ChainTronMainnet,
			Kind:       KindTRC20USDT,
			From:       "TFrom",
			To:         "TTo",
			Amount:     "1.0",
			RawDataHex: "0a02abcd",
			TxIDHex:    "0x" + "00000000000000000000000000000000000000000000000000000000000000ab",
		}

		b, err := MarshalUnsignedPayload(u)
		Expect(err).NotTo(HaveOccurred())

		got, err := UnmarshalUnsignedPayload(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.From).To(Equal(u.From))
		Expect(got.To).To(Equal(u.To))
		Expect(got.RawDataHex).To(Equal(u.RawDataHex))
		Expect(got.Version).To(Equal(EnvelopeVersion))
	})

	It("rejects unsigned payloads with missing required fields", func() {
		_, err := MarshalUnsignedPayload(UnsignedTransferPayload{Kind: KindTRX})
		Expect(err).To(HaveOccurred())
	})

	It("rejects unsigned payloads with bad envelope version", func() {
		_, err := UnmarshalUnsignedPayload([]byte(`{"version":99}`))
		Expect(err).To(HaveOccurred())
	})

	It("round-trips signed payloads", func() {
		s := SignedTransferPayload{
			Chain:        ChainTronMainnet,
			Kind:         KindTRX,
			From:         "TFrom",
			To:           "TTo",
			Amount:       "1.5",
			RawDataHex:   "0a02abcd",
			SignatureHex: "0x" + "00",
			TxIDHex:      "0xabcd",
		}

		b, err := MarshalSignedPayload(s)
		Expect(err).NotTo(HaveOccurred())

		got, err := UnmarshalSignedPayload(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.SignatureHex).To(Equal(s.SignatureHex))
		Expect(got.RawDataHex).To(Equal(s.RawDataHex))
	})

	It("rejects signed payloads without signature", func() {
		_, err := MarshalSignedPayload(SignedTransferPayload{
			Kind: KindTRX, From: "a", To: "b", Amount: "1", RawDataHex: "0a",
		})
		Expect(err).To(HaveOccurred())
	})
})
