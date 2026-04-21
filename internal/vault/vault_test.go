package vault_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"

	"github.com/grafov/monujo/internal/vault"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testPass = "correct horse battery staple"

func writeFile(dir, name string, body []byte) {
	GinkgoHelper()
	path := filepath.Join(dir, name)
	Expect(os.WriteFile(path, body, 0o600)).To(Succeed())
}

var _ = Describe("Pack and Unpack", func() {
	var srcDir, dstDir string

	BeforeEach(func() {
		srcDir = GinkgoT().TempDir()
		dstDir = GinkgoT().TempDir()
	})

	It("round-trips allowlisted keystore files byte-for-byte with 0600 mode", func() {
		writeFile(srcDir, "a.json", []byte(`{"version":2,"name":"a"}`))
		writeFile(srcDir, "b.scrypt.bak", []byte("legacy-b"))
		writeFile(srcDir, "c.scrypt.bak.123", []byte("legacy-c-ts"))

		var buf bytes.Buffer
		preport, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(preport.Files).To(ConsistOf("a.json", "b.scrypt.bak", "c.scrypt.bak.123"))
		Expect(preport.Bytes).To(BeNumerically(">", 0))

		ureport, err := vault.Unpack(&buf, dstDir, []byte(testPass), vault.UnpackOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ureport.Written).To(ConsistOf("a.json", "b.scrypt.bak", "c.scrypt.bak.123"))
		Expect(ureport.Conflicts).To(BeEmpty())

		for _, name := range []string{"a.json", "b.scrypt.bak", "c.scrypt.bak.123"} {
			orig, err := os.ReadFile(filepath.Join(srcDir, name))
			Expect(err).NotTo(HaveOccurred())
			got, err := os.ReadFile(filepath.Join(dstDir, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(orig))
			info, err := os.Stat(filepath.Join(dstDir, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		}
	})

	It("errors on wrong passphrase", func() {
		writeFile(srcDir, "a.json", []byte("data"))
		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = vault.Unpack(&buf, dstDir, []byte("wrong passphrase"), vault.UnpackOptions{})
		Expect(err).To(HaveOccurred())
	})

	It("errors on ciphertext tampering", func() {
		writeFile(srcDir, "a.json", []byte("data"))
		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())

		b := buf.Bytes()
		b[len(b)-1] ^= 0xFF
		_, err = vault.Unpack(bytes.NewReader(b), dstDir, []byte(testPass), vault.UnpackOptions{})
		Expect(err).To(HaveOccurred())
	})

	It("refuses to overwrite existing files by default and leaves them intact", func() {
		writeFile(srcDir, "a.json", []byte("new"))
		writeFile(dstDir, "a.json", []byte("old-local"))

		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())

		rep, err := vault.Unpack(&buf, dstDir, []byte(testPass), vault.UnpackOptions{})
		Expect(err).To(MatchError(vault.ErrOverwriteRequired))
		Expect(rep.Conflicts).To(ConsistOf("a.json"))

		// Existing file untouched.
		body, err := os.ReadFile(filepath.Join(dstDir, "a.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("old-local")))
	})

	It("overwrites when Overwrite=true", func() {
		writeFile(srcDir, "a.json", []byte("new"))
		writeFile(dstDir, "a.json", []byte("old-local"))

		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = vault.Unpack(&buf, dstDir, []byte(testPass), vault.UnpackOptions{Overwrite: true})
		Expect(err).NotTo(HaveOccurred())

		body, err := os.ReadFile(filepath.Join(dstDir, "a.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(body).To(Equal([]byte("new")))
	})

	It("ignores non-allowlisted entries during pack", func() {
		writeFile(srcDir, "a.json", []byte("keep"))
		writeFile(srcDir, ".monujo-keystore-abcd.tmp", []byte("tmp"))
		writeFile(srcDir, "README.txt", []byte("readme"))
		writeFile(srcDir, ".hidden", []byte("hidden"))
		Expect(os.Mkdir(filepath.Join(srcDir, "subdir"), 0o700)).To(Succeed())

		var buf bytes.Buffer
		rep, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(rep.Files).To(ConsistOf("a.json"))
		Expect(rep.SkippedNonAllowlisted).To(ContainElements(".monujo-keystore-abcd.tmp", "README.txt", ".hidden", "subdir"))
	})

	It("rejects path traversal entries during unpack", func() {
		// Build a malicious tar manually, encrypt it with age ourselves.
		var plain bytes.Buffer
		tw := tar.NewWriter(&plain)
		Expect(tw.WriteHeader(&tar.Header{Name: "../escape.json", Mode: 0o600, Size: 4, Typeflag: tar.TypeReg})).To(Succeed())
		_, err := tw.Write([]byte("data"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tw.Close()).To(Succeed())

		var enc bytes.Buffer
		r, err := age.NewScryptRecipient(testPass)
		Expect(err).NotTo(HaveOccurred())
		w, err := age.Encrypt(&enc, r)
		Expect(err).NotTo(HaveOccurred())
		_, err = w.Write(plain.Bytes())
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Close()).To(Succeed())

		_, err = vault.Unpack(&enc, dstDir, []byte(testPass), vault.UnpackOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid entry name"))
	})

	It("round-trips with armor", func() {
		writeFile(srcDir, "a.json", []byte("data"))
		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{Armor: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.HasPrefix(buf.String(), "-----BEGIN AGE ENCRYPTED FILE-----")).To(BeTrue())

		_, err = vault.Unpack(&buf, dstDir, []byte(testPass), vault.UnpackOptions{})
		Expect(err).NotTo(HaveOccurred())
		got, err := os.ReadFile(filepath.Join(dstDir, "a.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]byte("data")))
	})

	It("errors on empty source directory", func() {
		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nothing to pack"))
	})

	It("DryRun writes nothing but reports would-be written entries", func() {
		writeFile(srcDir, "a.json", []byte("data"))
		var buf bytes.Buffer
		_, err := vault.Pack(&buf, srcDir, []byte(testPass), vault.PackOptions{})
		Expect(err).NotTo(HaveOccurred())

		rep, err := vault.Unpack(&buf, dstDir, []byte(testPass), vault.UnpackOptions{DryRun: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(rep.Written).To(ConsistOf("a.json"))

		_, err = os.Stat(filepath.Join(dstDir, "a.json"))
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
})
