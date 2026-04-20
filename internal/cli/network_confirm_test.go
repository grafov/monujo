package cli

import (
	"encoding/hex"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/term"
)

var _ = Describe("confirmWalletNetworkForOperation", func() {
	It("passes when wallet network matches", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		_, err = keystore.Store(dir, "w", hexKey, pass, []string{"shasta"})
		Expect(err).NotTo(HaveOccurred())

		cfg, err := config.Build(config.NetworkShasta, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		Expect(confirmWalletNetworkForOperation(cfg, "w", false)).To(Succeed())
	})

	It("skips mismatch check when forced", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		_, err = keystore.Store(dir, "w", hexKey, pass, []string{"shasta"})
		Expect(err).NotTo(HaveOccurred())

		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		Expect(confirmWalletNetworkForOperation(cfg, "w", true)).To(Succeed())
	})

	It("errors on mismatch in non-interactive mode", func() {
		if term.IsTerminal(int(os.Stdin.Fd())) { //nolint:gosec // G115
			Skip("stdin is a terminal; would require interactive input")
		}

		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		_, err = keystore.Store(dir, "w", hexKey, pass, []string{"shasta"})
		Expect(err).NotTo(HaveOccurred())

		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		err = confirmWalletNetworkForOperation(cfg, "w", false)
		Expect(err).To(HaveOccurred())
	})
})
