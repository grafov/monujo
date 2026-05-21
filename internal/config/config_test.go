package config

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("config", func() {
	It("parses network and output flags", func() {
		net, err := ParseNetwork(" SHASTA ")
		Expect(err).NotTo(HaveOccurred())
		Expect(net).To(Equal(NetworkShasta))

		out, err := ParseOutputFormat(" JSON ")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(OutputJSON))

		_, err = ParseNetwork("nile")
		Expect(err).To(HaveOccurred())
		_, err = ParseOutputFormat("yaml")
		Expect(err).To(HaveOccurred())
	})

	It("builds defaults and trims overrides", func() {
		cfg, err := Build(NetworkMainnet, OutputTable, "https://example.test///", "node.test:50051", " api-key ", "/tmp/ks")
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.HTTPBaseURL).To(Equal("https://example.test"))
		Expect(cfg.GRPCAddress).To(Equal("node.test:50051"))
		Expect(cfg.TronGridAPIKey).To(Equal("api-key"))
		Expect(cfg.KeystoreDir).To(Equal("/tmp/ks"))
		Expect(cfg.DefaultFeeLimitSun).To(BeNumerically(">", 0))
	})

	It("uses MONUJO_KEYSTORE_DIR when set", func() {
		dir := filepath.Join(GinkgoT().TempDir(), "ks")
		old, had := os.LookupEnv("MONUJO_KEYSTORE_DIR")
		Expect(os.Setenv("MONUJO_KEYSTORE_DIR", dir)).To(Succeed())
		DeferCleanup(func() {
			if had {
				_ = os.Setenv("MONUJO_KEYSTORE_DIR", old)
			} else {
				_ = os.Unsetenv("MONUJO_KEYSTORE_DIR")
			}
		})

		got, err := DefaultKeystoreDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(dir))
	})
})
