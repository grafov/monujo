//go:build integration

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
	"context"
	"encoding/hex"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// Opt-in integration suite for [package tron]. It is built only when the
// `integration` tag is supplied AND MONUJO_TRON_E2E=1 is set in the
// environment, so a default `make test` never reaches the network.
//
// Required environment for write specs:
//
//	MONUJO_TRON_E2E=1                      gate flag (also gates read specs)
//	MONUJO_TRON_TEST_KEY=<hex-priv>        funded shasta sender key (no 0x)
//	MONUJO_TRON_TEST_ADDR_TO=<base58>      destination address
//
// Optional:
//
//	MONUJO_TRON_TEST_ADDR_READ=<base58>    address to query in read specs
//	                                        (defaults to MONUJO_TRON_TEST_ADDR_TO)
//	MONUJO_TRONGRID_HTTP=<url>             override HTTP endpoint
//	MONUJO_TRON_GRPC=<host:port>           override gRPC endpoint
//
// Run with:  make test-integration

var _ = Describe("integration: tron", Ordered, func() {
	var (
		cfg      *config.Config
		readAddr string
		toAddr   string
		keyHex   string
	)

	BeforeAll(func() {
		if os.Getenv("MONUJO_TRON_E2E") != "1" {
			Skip("MONUJO_TRON_E2E!=1 — skipping integration suite")
		}
		net, err := config.ParseNetwork("shasta")
		Expect(err).NotTo(HaveOccurred())
		cfg, err = config.Build(net, config.OutputJSON,
			os.Getenv("MONUJO_TRONGRID_HTTP"),
			os.Getenv("MONUJO_TRON_GRPC"),
			config.TronGridAPIKeyFromEnv(),
			"",
		)
		Expect(err).NotTo(HaveOccurred())
		if os.Getenv("MONUJO_TRON_WRITE_TRANSPORT") == "grpc" {
			cfg.WriteTransport = config.WriteTransportGRPC
			cfg.GRPCInsecure = true
		}

		toAddr = strings.TrimSpace(os.Getenv("MONUJO_TRON_TEST_ADDR_TO"))
		readAddr = strings.TrimSpace(os.Getenv("MONUJO_TRON_TEST_ADDR_READ"))
		if readAddr == "" {
			readAddr = toAddr
		}
		keyHex = strings.TrimSpace(os.Getenv("MONUJO_TRON_TEST_KEY"))
	})

	Describe("ReadClient against Shasta", func() {
		BeforeEach(func() {
			if readAddr == "" {
				Skip("MONUJO_TRON_TEST_ADDR_READ / MONUJO_TRON_TEST_ADDR_TO not set")
			}
			Expect(tron.ValidateBase58Address(readAddr)).To(Succeed())
		})

		It("fetches balances for a known address", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			rc := tron.NewReadClient(cfg)
			out, err := rc.GetBalances(ctx, readAddr)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(BeNil())
			Expect(out.Address).To(Equal(readAddr))
		})

		It("fetches recent USDT transfers for a known address", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			rc := tron.NewReadClient(cfg)
			rows, err := rc.ListUSDTTransfers(ctx, readAddr, 5)
			Expect(err).NotTo(HaveOccurred())
			Expect(rows).NotTo(BeNil())
		})
	})

	Describe("GRPCWriteClient against Shasta", func() {
		BeforeEach(func() {
			if keyHex == "" || toAddr == "" {
				Skip("MONUJO_TRON_TEST_KEY / MONUJO_TRON_TEST_ADDR_TO not set")
			}
			Expect(tron.ValidateBase58Address(toAddr)).To(Succeed())
		})

		It("broadcasts a minimal SendTRX transaction and observes the txid", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			keyBytes, err := hex.DecodeString(keyHex)
			Expect(err).NotTo(HaveOccurred(), "MONUJO_TRON_TEST_KEY must be hex without 0x")
			priv, err := crypto.ToECDSA(keyBytes)
			Expect(err).NotTo(HaveOccurred())

			fromAddrBytes := crypto.PubkeyToAddress(priv.PublicKey).Bytes()
			Expect(fromAddrBytes).NotTo(BeEmpty())

			wc, err := tron.NewTransactionWriterFromConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = wc.Close() })

			fromAddr := strings.TrimSpace(os.Getenv("MONUJO_TRON_TEST_ADDR_FROM"))
			if fromAddr == "" {
				Skip("MONUJO_TRON_TEST_ADDR_FROM not set; required to derive sender base58")
			}
			Expect(tron.ValidateBase58Address(fromAddr)).To(Succeed())

			txid, err := wc.SendTRX(ctx, fromAddr, toAddr, 1, priv)
			Expect(err).NotTo(HaveOccurred())
			Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
		})
	})
})
