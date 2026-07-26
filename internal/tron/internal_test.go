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

package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

// White-box specs for unexported helpers of [package tron]. They register
// Describe blocks into Ginkgo's global tree and run via the single
// TestTron(t) entry point declared in tron_suite_test.go (black-box file
// in [package tron_test]). Both files compile into the same test binary,
// so the tron_test entry point picks up these specs automatically.

var _ = Describe("truncate", func() {
	It("returns the input untouched when shorter or equal to limit", func() {
		Expect(truncate("")).To(Equal(""))
		Expect(truncate(strings.Repeat("x", errorBodyTruncateLen))).To(Equal(strings.Repeat("x", errorBodyTruncateLen)))
	})

	It("appends an ellipsis when input is longer than limit", func() {
		long := strings.Repeat("X", errorBodyTruncateLen+1)
		out := truncate(long)
		Expect(out).To(HaveLen(errorBodyTruncateLen + 3))
		Expect(out).To(HaveSuffix("..."))
		Expect(out[:errorBodyTruncateLen]).To(Equal(strings.Repeat("X", errorBodyTruncateLen)))
	})
})

var _ = Describe("transactionIDHex", func() {
	makeTx := func(refBlock []byte, expiration int64, data []byte) *core.Transaction {
		return &core.Transaction{
			RawData: &core.TransactionRaw{
				RefBlockBytes: refBlock,
				Expiration:    expiration,
				Data:          data,
			},
		}
	}

	It("returns the 0x-prefixed lowercase sha256 hex of the marshalled raw data", func() {
		tx := makeTx([]byte{0x12, 0x34}, 1_700_000_000_000, []byte("monujo-test"))

		raw, err := proto.Marshal(tx.GetRawData())
		Expect(err).NotTo(HaveOccurred())
		want := sha256.Sum256(raw)
		wantHex := "0x" + hex.EncodeToString(want[:])

		got := transactionIDHex(tx)
		Expect(got).To(Equal(wantHex))
		Expect(got).To(HaveLen(66))
		Expect(got).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
	})

	It("is deterministic for equal inputs", func() {
		tx := makeTx([]byte{0xde, 0xad}, 42, []byte("same"))
		Expect(transactionIDHex(tx)).To(Equal(transactionIDHex(tx)))
	})

	It("produces different hashes for different inputs", func() {
		a := makeTx([]byte{0x01}, 10, []byte("a"))
		b := makeTx([]byte{0x01}, 10, []byte("b"))
		Expect(transactionIDHex(a)).NotTo(Equal(transactionIDHex(b)))
	})

	It("returns the sha256 of an empty marshalled message when raw data is nil", func() {
		tx := &core.Transaction{}
		raw, err := proto.Marshal(tx.GetRawData())
		Expect(err).NotTo(HaveOccurred())
		want := sha256.Sum256(raw)

		got := transactionIDHex(tx)
		Expect(got).To(Equal("0x" + hex.EncodeToString(want[:])))
		Expect(got).NotTo(BeEmpty(),
			"current behaviour: even nil raw data produces a fixed digest")
	})
})
