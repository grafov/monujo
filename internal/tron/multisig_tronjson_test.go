package tron_test

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/monujo/internal/tron"
)

var _ = Describe("DecodeTronJSON / EncodeTronJSON", func() {
	It("round-trips raw_data_hex and signatures from a canonical Tron JSON envelope", func() {
		k := mustGenerateKey()
		rawHex := buildMultisigRawDataHex(2, 123)
		sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k)
		Expect(err).NotTo(HaveOccurred())

		body, err := tron.EncodeTronJSON(rawHex, sigs)
		Expect(err).NotTo(HaveOccurred())

		var probe map[string]any
		Expect(json.Unmarshal(body, &probe)).To(Succeed())
		Expect(probe).To(HaveKey("txID"))
		Expect(probe).To(HaveKey("raw_data_hex"))
		Expect(probe).To(HaveKey("signature"))

		gotRaw, gotSigs, gotID, err := tron.DecodeTronJSON(body)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotRaw).To(Equal(rawHex))
		Expect(gotSigs).To(Equal(sigs))
		Expect(gotID).To(HaveLen(64))
	})

	It("tolerates leading 0x on raw_data_hex and signatures", func() {
		rawHex := buildMultisigRawDataHex(0, 1)
		k := mustGenerateKey()
		sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k)
		Expect(err).NotTo(HaveOccurred())

		envelope := map[string]any{
			"raw_data_hex": "0x" + rawHex,
			"signature":    []string{"0x" + sigs[0]},
		}
		body, err := json.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

		gotRaw, gotSigs, gotID, err := tron.DecodeTronJSON(body)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotRaw).To(Equal(rawHex))
		Expect(gotSigs).To(ConsistOf(sigs[0]))
		Expect(gotID).To(HaveLen(64))
	})

	It("rejects a tampered txID that does not match sha256(raw_data)", func() {
		rawHex := buildMultisigRawDataHex(2, 5)
		envelope := map[string]any{
			"txID":         strings.Repeat("aa", 32),
			"raw_data_hex": rawHex,
		}
		body, err := json.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())

		_, _, _, err = tron.DecodeTronJSON(body)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("txID mismatch"))
	})

	It("rejects signatures with wrong length", func() {
		rawHex := buildMultisigRawDataHex(0, 1)
		envelope := map[string]any{
			"raw_data_hex": rawHex,
			"signature":    []string{hex.EncodeToString(make([]byte, 10))},
		}
		body, err := json.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())
		_, _, _, err = tron.DecodeTronJSON(body)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("signature[0] length"))
	})

	It("rejects empty input", func() {
		_, _, _, err := tron.DecodeTronJSON([]byte("   "))
		Expect(err).To(HaveOccurred())
	})
})
