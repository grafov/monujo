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
	"bytes"
	"io"

	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("multisig cosign --addr resolution", func() {
	It("rejects an external wallet name before passphrase", func() {
		dir := GinkgoT().TempDir()
		const addr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
		_, err := keystore.StoreExternal(dir, "extcos", addr, nil)
		Expect(err).NotTo(HaveOccurred())

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"multisig", "cosign",
			"--in", "/dev/null",
			"--out", "-",
			"--addr", "extcos",
		})

		err = root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("external"))
	})
})
