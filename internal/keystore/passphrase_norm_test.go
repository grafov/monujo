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
