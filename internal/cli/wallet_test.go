package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("monujo wallet group", func() {
	It("creates a wallet via app.HotService and lists it back", func() {
		// Drive the same service entry points the `wallet create` and
		// `wallet list` cobra commands use, against a fresh temp keystore.
		// This avoids the cobra/TTY plumbing while still validating that
		// the lifecycle path works end-to-end.
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		svc := app.NewHotService(cfg)
		Expect(svc.CreateWallet("w1", pass, false, false)).To(Succeed())

		path := filepath.Join(dir, "w1.json")
		_, err = os.Stat(path)
		Expect(err).NotTo(HaveOccurred(), "wallet file should exist after create")

		items, err := keystore.ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(1))
		Expect(items[0].Name).To(Equal("w1"))
		Expect(items[0].Address).NotTo(BeEmpty())
	})

	It("registers wallet subcommands under root", func() {
		root := newRootCmd()
		var found []string
		for _, c := range root.Commands() {
			if c.Name() == "wallet" {
				for _, sub := range c.Commands() {
					found = append(found, sub.Name())
				}
			}
		}
		Expect(found).To(ConsistOf("list", "create", "import", "check"))
	})

	It("wallet check succeeds with correct passphrase (by wallet name)", func() {
		dir := GinkgoT().TempDir()
		stashWallet(dir, "chk")

		root := newRootCmd()
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(io.Discard)
		root.SetIn(strings.NewReader("stash-passphrase-16+chars!!"))
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "check", "--addr", "chk"})

		Expect(root.Execute()).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Passphrase matches"))
		Expect(stdout.String()).To(ContainSubstring("chk"))
	})

	It("wallet check succeeds when --addr is the base58 address", func() {
		dir := GinkgoT().TempDir()
		addr := stashWallet(dir, "byaddr")

		root := newRootCmd()
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(io.Discard)
		root.SetIn(strings.NewReader("stash-passphrase-16+chars!!"))
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "check", "--addr", addr})

		Expect(root.Execute()).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(addr))
	})

	It("wallet check fails on wrong passphrase", func() {
		dir := GinkgoT().TempDir()
		stashWallet(dir, "badpass")

		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetIn(strings.NewReader("wrong-passphrase-not-the-real-one!"))
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "check", "--addr", "badpass"})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("decryption failed"))
	})

	It("wallet import without --key-hex and without env returns a clear error", func() {
		// Ensure the env var is empty for this spec so we exercise the
		// "missing key" path deterministically.
		prev, hadPrev := os.LookupEnv("MONUJO_IMPORT_KEY")
		Expect(os.Unsetenv("MONUJO_IMPORT_KEY")).To(Succeed())
		DeferCleanup(func() {
			if hadPrev {
				_ = os.Setenv("MONUJO_IMPORT_KEY", prev)
			}
		})

		dir := GinkgoT().TempDir()

		root := newRootCmd()
		root.SetOut(io.Discard)
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "import", "--name", "w1"})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("MONUJO_IMPORT_KEY"))
	})
})
