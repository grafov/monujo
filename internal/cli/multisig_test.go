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
