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
