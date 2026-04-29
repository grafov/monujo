package cli

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/grafov/monujo/internal/app"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("monujo wallet group", func() {
	const addressLikeName = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

	It("wallet create rejects --name that looks like a Tron address before passphrase", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "create", "--name", addressLikeName})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("looks like a Tron base58 address"))
	})

	It("wallet import rejects --name that looks like a Tron address before passphrase", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())
		keyHex := hex.EncodeToString(crypto.FromECDSA(priv))

		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "import", "--name", addressLikeName, "--key-hex", keyHex})

		err = root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("looks like a Tron base58 address"))
	})

	It("wallet import-external rejects --name that looks like a Tron address before writing keystore", func() {
		dir := GinkgoT().TempDir()
		const extAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"

		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "import-external", "--name", addressLikeName, "--addr", extAddr})

		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("looks like a Tron base58 address"))

		items, err := keystore.ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(BeEmpty())
	})

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
		Expect(items[0].External).To(BeFalse())
	})

	It("imports an external wallet via HotService and lists it as external", func() {
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())
		svc := app.NewHotService(cfg)
		const addr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
		Expect(svc.ImportExternalWallet("cust", addr)).To(Succeed())

		items, err := keystore.ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(1))
		Expect(items[0].Name).To(Equal("cust"))
		Expect(items[0].Address).To(Equal(addr))
		Expect(items[0].External).To(BeTrue())
	})

	It("wallet import-external via root creates a file without passphrase", func() {
		dir := GinkgoT().TempDir()
		const addr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "import-external", "--name", "ext1", "--addr", addr})

		Expect(root.Execute()).To(Succeed())
		items, err := keystore.ListWallets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(1))
		Expect(items[0].External).To(BeTrue())
	})

	It("wallet check refuses an external wallet before passphrase", func() {
		dir := GinkgoT().TempDir()
		const addr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
		_, err := keystore.StoreExternal(dir, "onlyaddr", addr, nil)
		Expect(err).NotTo(HaveOccurred())

		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetIn(strings.NewReader("should-not-be-read\n"))
		root.SetArgs([]string{"--keystore-dir", dir, "wallet", "check", "--addr", "onlyaddr"})

		err = root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("external"))
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
		Expect(found).To(ConsistOf("list", "create", "import", "import-external", "check"))
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
