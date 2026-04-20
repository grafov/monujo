package keystore

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NormalizePassphrase", func() {
	It("keeps ASCII input unchanged and returns a fresh allocation", func() {
		in := []byte("hello-world-123")
		out := NormalizePassphrase(in)
		Expect(bytes.Equal(in, out)).To(BeTrue())

		if len(in) > 0 && len(out) > 0 {
			Expect(&in[0] == &out[0]).To(BeFalse())
		}

		ZeroPassphrase(out)
		Expect(bytes.Equal(in, []byte("hello-world-123"))).To(BeTrue())
	})

	It("normalizes NFD to NFC", func() {
		nfd := []byte("cafe\u0301")
		nfc := []byte("caf\u00e9")
		got := NormalizePassphrase(nfd)
		Expect(bytes.Equal(got, nfc)).To(BeTrue())
	})

	It("normalizes ligatures", func() {
		lig := []byte("\uFB01ne")
		want := []byte("fine")
		got := NormalizePassphrase(lig)
		Expect(bytes.Equal(got, want)).To(BeTrue())
	})

	It("normalizes full-width forms", func() {
		full := []byte("\uFF50\uFF41\uFF53\uFF53")
		want := []byte("pass")
		got := NormalizePassphrase(full)
		Expect(bytes.Equal(got, want)).To(BeTrue())
	})

	It("returns an empty non-nil slice for empty input", func() {
		got := NormalizePassphrase(nil)
		Expect(got).NotTo(BeNil())
		Expect(got).To(HaveLen(0))
	})
})
