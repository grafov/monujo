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

var _ = Describe("confirmWalletOverwrite", func() {
	It("passes when wallet does not exist", func() {
		dir := GinkgoT().TempDir()
		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		Expect(confirmWalletOverwrite(cfg, "absent", false)).To(Succeed())
		Expect(confirmWalletOverwrite(cfg, "absent", true)).To(Succeed())
	})

	It("errors when wallet exists and --overwrite is not set", func() {
		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		_, err = keystore.Store(dir, "present", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		err = confirmWalletOverwrite(cfg, "present", false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("already exists"))
		Expect(err.Error()).To(ContainSubstring("--overwrite"))
	})

	It("allows overwrite without prompt when stdin is not a terminal", func() {
		if term.IsTerminal(int(os.Stdin.Fd())) { //nolint:gosec // G115
			Skip("stdin is a terminal; would require interactive input")
		}

		dir := GinkgoT().TempDir()
		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
		pass := []byte("test-passphrase-32-chars-long!!")
		defer keystore.ZeroPassphrase(pass)

		_, err = keystore.Store(dir, "present", hexKey, pass, nil)
		Expect(err).NotTo(HaveOccurred())

		cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
		Expect(err).NotTo(HaveOccurred())

		Expect(confirmWalletOverwrite(cfg, "present", true)).To(Succeed())
	})
})
