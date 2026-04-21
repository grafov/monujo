package cli

import (
	"bytes"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The offline CLI path does not itself decrypt anything, so these tests
// only cover --from resolution: the cobra command must refuse addresses
// that are not present in the local keystore *before* it asks for the
// keystore passphrase, and must keep accepting wallet names.

var _ = Describe("offline sign --from resolution", func() {
	It("rejects a base58 address that is not in the keystore", func() {
		dir := GinkgoT().TempDir()

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"offline", "sign",
			"--in", "/dev/null",
			"--out", "-",
			"--from", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found in keystore"))
		Expect(err.Error()).To(ContainSubstring("pass the wallet name"))
	})

	It("rejects a name that does not exist in the keystore", func() {
		dir := GinkgoT().TempDir()

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"offline", "sign",
			"--in", "/dev/null",
			"--out", "-",
			"--from", "missing",
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "missing" not found`))
	})
})

var _ = Describe("offline build --from resolution", func() {
	It("accepts a foreign base58 address (build does not need keystore)", func() {
		// This test only verifies that --from resolution does not reject a
		// foreign address up-front. The command still fails later because
		// there is no online Tron node in the test environment, but the
		// error must NOT be the --from "not found" one.
		dir := GinkgoT().TempDir()
		out := GinkgoT().TempDir() + "/u.json"

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"offline", "build",
			"--from", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
			"--to", "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
			"--amount", "0.1",
			"--kind", "trx",
			"--out", out,
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred(), "expected a downstream failure (no node)")
		Expect(err.Error()).NotTo(ContainSubstring("not found in keystore"))
		Expect(err.Error()).NotTo(ContainSubstring("--from must be provided"))
	})

	It("rejects a name that does not exist in the keystore", func() {
		dir := GinkgoT().TempDir()
		out := GinkgoT().TempDir() + "/u.json"

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"offline", "build",
			"--from", "ghost",
			"--to", "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
			"--amount", "0.1",
			"--kind", "trx",
			"--out", out,
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "ghost" not found`))
	})

	It("rejects --to when name is not in the keystore", func() {
		dir := GinkgoT().TempDir()
		out := GinkgoT().TempDir() + "/u.json"

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"offline", "build",
			"--from", "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
			"--to", "nonexistent",
			"--amount", "0.1",
			"--kind", "trx",
			"--out", out,
		})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`wallet "nonexistent" not found`))
	})
})
