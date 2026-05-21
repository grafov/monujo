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
	"errors"
	"os"
	"path/filepath"

	"github.com/grafov/monujo/internal/vault"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const vaultTestPass = "correct horse battery staple"

// stubPass sets up deterministic passphrase readers for a single test.
// twicePass is returned by the "new passphrase" flow (pack), onePass by
// the "existing passphrase" flow (unpack). Reset the originals via the
// returned closer.
func stubPass(twicePass, onePass string) func() {
	origTwice := readPassTwiceFn
	origOnce := readPassphraseFn
	readPassTwiceFn = func(_ string) ([]byte, error) {
		return []byte(twicePass), nil
	}
	readPassphraseFn = func(_ string) ([]byte, error) {
		return []byte(onePass), nil
	}
	return func() {
		readPassTwiceFn = origTwice
		readPassphraseFn = origOnce
	}
}

var _ = Describe("runVaultPack and runVaultUnpack", func() {
	var (
		srcDir, dstDir string
		vaultPath      string
		restore        func()
	)

	BeforeEach(func() {
		srcDir = GinkgoT().TempDir()
		dstDir = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(srcDir, "a.json"), []byte(`{"v":2}`), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "b.scrypt.bak"), []byte("legacy"), 0o600)).To(Succeed())
		vaultPath = filepath.Join(GinkgoT().TempDir(), "vault.age")
		restore = stubPass(vaultTestPass, vaultTestPass)
	})

	AfterEach(func() {
		restore()
	})

	It("round-trips via pack then unpack", func() {
		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, false, false)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Packed 2 file"))

		info, err := os.Stat(vaultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))

		out.Reset()
		Expect(runVaultUnpack(&out, vaultPath, dstDir, false, false, false)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Unpacked 2 file"))

		for _, name := range []string{"a.json", "b.scrypt.bak"} {
			got, err := os.ReadFile(filepath.Join(dstDir, name))
			Expect(err).NotTo(HaveOccurred())
			orig, err := os.ReadFile(filepath.Join(srcDir, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(orig))
		}
	})

	It("pack refuses to overwrite an existing --out file by default", func() {
		Expect(os.WriteFile(vaultPath, []byte("existing"), 0o600)).To(Succeed())
		var out bytes.Buffer
		err := runVaultPack(&out, srcDir, vaultPath, false, false, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--overwrite"))
		body, rerr := os.ReadFile(vaultPath)
		Expect(rerr).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("existing")))
	})

	It("pack with --overwrite (non-TTY) replaces the previous archive atomically", func() {
		Expect(os.WriteFile(vaultPath, []byte("old-archive"), 0o600)).To(Succeed())

		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, true, false)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Replaced previous archive"))

		// The replacement is a valid vault we can unpack.
		Expect(runVaultUnpack(&bytes.Buffer{}, vaultPath, dstDir, false, false, false)).To(Succeed())
		got, err := os.ReadFile(filepath.Join(dstDir, "a.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]byte(`{"v":2}`)))

		// No stray temp files were left behind in the archive directory.
		entries, err := os.ReadDir(filepath.Dir(vaultPath))
		Expect(err).NotTo(HaveOccurred())
		for _, e := range entries {
			Expect(e.Name()).NotTo(HavePrefix(".monujo-vault-"))
		}
	})

	It("pack --overwrite keeps the previous archive when Pack itself fails", func() {
		Expect(os.WriteFile(vaultPath, []byte("old-archive-bytes"), 0o600)).To(Succeed())

		empty := GinkgoT().TempDir()
		var out bytes.Buffer
		err := runVaultPack(&out, empty, vaultPath, false, true, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nothing to pack"))

		body, rerr := os.ReadFile(vaultPath)
		Expect(rerr).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("old-archive-bytes")))

		entries, err := os.ReadDir(filepath.Dir(vaultPath))
		Expect(err).NotTo(HaveOccurred())
		for _, e := range entries {
			Expect(e.Name()).NotTo(HavePrefix(".monujo-vault-"))
		}
	})

	It("pack --overwrite on TTY aborts when the user answers no", func() {
		Expect(os.WriteFile(vaultPath, []byte("old-archive-bytes"), 0o600)).To(Succeed())

		origPrompt := promptConfirmFn
		promptConfirmFn = func() (bool, error) { return false, nil }
		defer func() { promptConfirmFn = origPrompt }()

		var out bytes.Buffer
		err := runVaultPack(&out, srcDir, vaultPath, false, true, true)
		Expect(err).To(MatchError("aborted"))

		body, rerr := os.ReadFile(vaultPath)
		Expect(rerr).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("old-archive-bytes")))
	})

	It("pack --overwrite on TTY proceeds when the user answers yes", func() {
		Expect(os.WriteFile(vaultPath, []byte("old-archive-bytes"), 0o600)).To(Succeed())

		origPrompt := promptConfirmFn
		promptConfirmFn = func() (bool, error) { return true, nil }
		defer func() { promptConfirmFn = origPrompt }()

		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, true, true)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("DANGER"))
		Expect(out.String()).To(ContainSubstring("Overwrite? [y/N]"))

		info, err := os.Stat(vaultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Size()).To(BeNumerically(">", int64(len("old-archive-bytes"))))
	})

	It("unpack with wrong passphrase returns an error and writes nothing", func() {
		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, false, false)).To(Succeed())

		restore()
		restore = stubPass(vaultTestPass, "totally wrong")

		out.Reset()
		Expect(runVaultUnpack(&out, vaultPath, dstDir, false, false, false)).To(HaveOccurred())
		_, statErr := os.Stat(filepath.Join(dstDir, "a.json"))
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("unpack refuses to overwrite existing files without --overwrite", func() {
		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, false, false)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(dstDir, "a.json"), []byte("local"), 0o600)).To(Succeed())

		out.Reset()
		err := runVaultUnpack(&out, vaultPath, dstDir, false, false, false)
		Expect(err).To(MatchError(vault.ErrOverwriteRequired))
		Expect(out.String()).To(ContainSubstring("Refusing to overwrite"))
		Expect(out.String()).To(ContainSubstring("a.json"))

		body, rerr := os.ReadFile(filepath.Join(dstDir, "a.json"))
		Expect(rerr).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("local")))
	})

	It("unpack --dry-run writes nothing", func() {
		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, vaultPath, false, false, false)).To(Succeed())

		out.Reset()
		Expect(runVaultUnpack(&out, vaultPath, dstDir, false, true, false)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would unpack"))

		_, statErr := os.Stat(filepath.Join(dstDir, "a.json"))
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("pack errors when --out is missing", func() {
		var out bytes.Buffer
		Expect(runVaultPack(&out, srcDir, "", false, false, false)).To(HaveOccurred())
	})

	It("unpack errors when --in is missing", func() {
		var out bytes.Buffer
		Expect(runVaultUnpack(&out, "", dstDir, false, false, false)).To(HaveOccurred())
	})

	It("pack propagates empty-source error and does not leave the output file", func() {
		empty := GinkgoT().TempDir()
		var out bytes.Buffer
		vp := filepath.Join(GinkgoT().TempDir(), "empty.age")
		err := runVaultPack(&out, empty, vp, false, false, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nothing to pack"))
		_, statErr := os.Stat(vp)
		Expect(errors.Is(statErr, os.ErrNotExist)).To(BeTrue())
	})
})
