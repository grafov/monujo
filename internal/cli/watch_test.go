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

package cli

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("watch CLI", func() {
	It("rejects tx limits outside the documented range before network access", func() {
		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{
			"watch", "txs",
			"--addr", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
			"--limit", "500",
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--limit must be between 1 and 200"))
	})
})
