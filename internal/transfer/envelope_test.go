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
			Chain:         ChainTronMainnet,
			Kind:          KindTRX,
			From:          "TFrom",
			To:            "TTo",
			Amount:        "1.5",
			AmountMinimal: "1500000",
			RawDataHex:    "0a02abcd",
			SignatureHex:  "0x" + "00",
			TxIDHex:       "0xabcd",
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
			Kind: KindTRX, From: "a", To: "b", Amount: "1", AmountMinimal: "1000000", RawDataHex: "0a",
		})
		Expect(err).To(HaveOccurred())
	})

	It("round-trips multisig transfer payloads (trc20)", func() {
		m := MultisigTransferPayload{
			Chain:         ChainTronMainnet,
			Kind:          KindTRC20USDT,
			PermissionID:  2,
			From:          "TFrom",
			To:            "TTo",
			Amount:        "10.0 USDT",
			AmountMinimal: "10000000",
			Contract:      "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			RawDataHex:    "0a02abcd",
			TxIDHex:       "0x00",
			SignaturesHex: []string{"0xdead", "0xbeef"},
		}

		b, err := MarshalMultisigPayload(m)
		Expect(err).NotTo(HaveOccurred())

		got, err := UnmarshalMultisigPayload(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.SignaturesHex).To(Equal(m.SignaturesHex))
		Expect(got.PermissionID).To(Equal(int32(2)))
		Expect(got.Version).To(Equal(EnvelopeVersion))
	})

	It("accepts multisig payloads with unknown contract kind (no display fields)", func() {
		m := MultisigTransferPayload{
			Chain:        ChainTronMainnet,
			Kind:         KindUnknownContract,
			PermissionID: 2,
			From:         "TFrom",
			RawDataHex:   "0a02abcd",
			TxIDHex:      "0x00",
		}

		b, err := MarshalMultisigPayload(m)
		Expect(err).NotTo(HaveOccurred())

		got, err := UnmarshalMultisigPayload(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Kind).To(Equal(KindUnknownContract))
		Expect(got.To).To(BeEmpty())
	})

	It("rejects multisig payload with trx kind but missing to/amount", func() {
		_, err := MarshalMultisigPayload(MultisigTransferPayload{
			Kind:       KindTRX,
			From:       "TFrom",
			RawDataHex: "0a02abcd",
			TxIDHex:    "0x00",
		})
		Expect(err).To(HaveOccurred())
	})

	It("rejects multisig payload with trc20 kind but missing contract", func() {
		_, err := MarshalMultisigPayload(MultisigTransferPayload{
			Kind:          KindTRC20USDT,
			From:          "TFrom",
			To:            "TTo",
			Amount:        "1 USDT",
			AmountMinimal: "1000000",
			RawDataHex:    "0a02abcd",
			TxIDHex:       "0x00",
		})
		Expect(err).To(HaveOccurred())
	})

	It("rejects multisig payload with unknown kind value", func() {
		_, err := MarshalMultisigPayload(MultisigTransferPayload{
			Kind:       "garbage",
			From:       "TFrom",
			RawDataHex: "0a02abcd",
			TxIDHex:    "0x00",
		})
		Expect(err).To(HaveOccurred())
	})

	It("rejects multisig payload with bad envelope version", func() {
		_, err := UnmarshalMultisigPayload([]byte(`{"version":99}`))
		Expect(err).To(HaveOccurred())
	})
})
