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

package tron_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

var _ = Describe("write transport factory", func() {
	It("returns HTTPWriteClient for REST", func() {
		cfg := &config.Config{
			Network:         config.NetworkMainnet,
			HTTPBaseURL:     "https://api.trongrid.io",
			WriteTransport:  config.WriteTransportREST,
			GRPCAddress:     "grpc.trongrid.io:50051",
		}
		w, err := tron.NewTransactionWriterFromConfig(cfg)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = w.Close() })
		_, ok := w.(*tron.HTTPWriteClient)
		Expect(ok).To(BeTrue())
	})

	It("warns for plaintext TronGrid gRPC and stays silent on REST", func() {
		var buf bytes.Buffer
		tron.WarnTronGridPlaintextGRPC(&buf, &config.Config{
			WriteTransport: config.WriteTransportGRPC,
			GRPCInsecure:   true,
			GRPCAddress:    "grpc.trongrid.io:50051",
		})
		Expect(buf.String()).To(ContainSubstring("plaintext"))

		buf.Reset()
		tron.WarnTronGridPlaintextGRPC(&buf, &config.Config{
			WriteTransport: config.WriteTransportREST,
			GRPCInsecure:   true,
			GRPCAddress:    "grpc.trongrid.io:50051",
		})
		Expect(buf.String()).To(BeEmpty())
	})
})
