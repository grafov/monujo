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
		Expect(cfg.WriteTransport).To(Equal(WriteTransportREST))
		Expect(cfg.DefaultFeeLimitSun).To(BeNumerically(">", 0))
	})

	It("parses write transport", func() {
		wt, err := ParseWriteTransport("grpc")
		Expect(err).NotTo(HaveOccurred())
		Expect(wt).To(Equal(WriteTransportGRPC))

		wt, err = ParseWriteTransport("")
		Expect(err).NotTo(HaveOccurred())
		Expect(wt).To(Equal(WriteTransportREST))

		_, err = ParseWriteTransport("quic")
		Expect(err).To(HaveOccurred())
	})

	It("validates REST HTTP base URL scheme", func() {
		Expect(IsHTTPSBaseURL("https://api.trongrid.io")).To(BeTrue())
		Expect(IsHTTPSBaseURL("http://127.0.0.1:8090")).To(BeFalse())
		Expect(ValidateRESTHTTPBaseURL("https://api.trongrid.io", false)).To(Succeed())
		Expect(ValidateRESTHTTPBaseURL("http://127.0.0.1:8090", false)).NotTo(Succeed())
		Expect(ValidateRESTHTTPBaseURL("http://127.0.0.1:8090", true)).To(Succeed())
	})

	It("detects TronGrid gRPC hosts", func() {
		Expect(IsTronGridGRPC(defaultGRPCMainnet)).To(BeTrue())
		Expect(IsTronGridGRPC("node.example:50051")).To(BeFalse())
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
