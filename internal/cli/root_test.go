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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("loadConfig transport guards", func() {
	BeforeEach(func() {
		// Reset package-level flag state between Execute runs.
		networkFlag = "mainnet"
		outputFlag = "table"
		httpURLFlag = ""
		grpcURLFlag = ""
		grpcInsecureFlag = false
		writeTransportFlag = "rest"
		allowInsecureHTTPFlag = false
		apiKeyFlag = ""
		keystoreDirFlag = ""
	})

	It("rejects --grpc-url with default REST write transport", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--grpc-url", "127.0.0.1:50051",
			"wallet", "list",
		})
		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--write-transport grpc"))
	})

	It("rejects --grpc-insecure with default REST write transport", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--grpc-insecure",
			"wallet", "list",
		})
		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--write-transport grpc"))
	})

	It("allows --grpc-url with --write-transport grpc", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--write-transport", "grpc",
			"--grpc-url", "127.0.0.1:50051",
			"--grpc-insecure",
			"wallet", "list",
		})
		Expect(root.Execute()).To(Succeed())
	})

	It("rejects http:// HTTP base URL for REST without --allow-insecure-http", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--http-url", "http://127.0.0.1:8090",
			"wallet", "list",
		})
		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("https://"))
	})

	It("allows http:// with --allow-insecure-http and warns on stderr", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		errBuf := &bytes.Buffer{}
		root.SetErr(errBuf)
		// loadConfig writes warnings to os.Stderr, not cmd.Err.
		r, w, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		old := os.Stderr
		os.Stderr = w
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--http-url", "http://127.0.0.1:8090",
			"--allow-insecure-http",
			"wallet", "list",
		})
		execErr := root.Execute()
		_ = w.Close()
		os.Stderr = old
		var warnBuf bytes.Buffer
		_, _ = warnBuf.ReadFrom(r)
		_ = r.Close()
		Expect(execErr).NotTo(HaveOccurred())
		Expect(warnBuf.String()).To(ContainSubstring("insecure http://"))
	})

	It("rejects invalid --write-transport", func() {
		dir := GinkgoT().TempDir()
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{
			"--keystore-dir", dir,
			"--write-transport", "quic",
			"wallet", "list",
		})
		err := root.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("write transport"))
	})
})
