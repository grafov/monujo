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
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// fixture loads a JSON file from testdata. Tests that hit the in-process
// httptest server depend on these snapshots; callers should not mutate them.
func fixture(name string) []byte {
	GinkgoHelper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred(), "read fixture %s", name)
	return b
}

// safeBaseURL fails the spec early if the configured base URL points outside
// 127.0.0.1; it is a defensive guard against accidentally hitting real TronGrid.
func safeBaseURL(u string) {
	GinkgoHelper()
	parsed, err := url.Parse(u)
	Expect(err).NotTo(HaveOccurred())
	Expect(parsed.Hostname()).To(Or(Equal("127.0.0.1"), Equal("localhost")),
		"refusing to talk to non-loopback host %q", parsed.Host)
}

var _ = Describe("ReadClient", func() {
	var (
		ts      *httptest.Server
		mux     *http.ServeMux
		cfg     *config.Config
		rc      *tron.ReadClient
		hits    atomic.Int32
		lastReq atomic.Pointer[http.Request]
	)

	captureReq := func(w http.ResponseWriter, r *http.Request, body []byte, status int) {
		hits.Add(1)
		clone := r.Clone(context.Background())
		lastReq.Store(clone)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}

	BeforeEach(func() {
		hits.Store(0)
		lastReq.Store(nil)
		mux = http.NewServeMux()
		ts = httptest.NewServer(mux)
		DeferCleanup(ts.Close)
		safeBaseURL(ts.URL)
		cfg = &config.Config{
			Network:            config.NetworkMainnet,
			HTTPBaseURL:        ts.URL,
			GRPCAddress:        "",
			TronGridAPIKey:     "",
			Output:             config.OutputTable,
			KeystoreDir:        "",
			DefaultFeeLimitSun: 0,
		}
		rc = tron.NewReadClient(cfg)
	})

	Describe("GetBalances", func() {
		const validAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"

		Context("when the address is invalid", func() {
			It("returns an error without hitting the network", func() {
				out, err := rc.GetBalances(context.Background(), "not-a-tron-address")
				Expect(err).To(HaveOccurred())
				Expect(out).To(BeNil())
				Expect(hits.Load()).To(BeZero(), "no HTTP call should be made")
			})
		})

		Context("when the upstream returns an account with USDT", func() {
			BeforeEach(func() {
				body := fixture("accounts_with_usdt.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("decodes sun balance and the matching USDT row", func() {
				out, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Address).To(Equal(validAddr))
				Expect(out.Sun).To(Equal(int64(12345678)))
				Expect(out.USDTMinimal).To(Equal("1234500000"))
			})

			It("does not send the API key header when none is configured", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				Expect(req.Header.Get("TRON-PRO-API-KEY")).To(BeEmpty())
			})

			It("URL-escapes the address into the path", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				Expect(req.URL.Path).To(Equal("/v1/accounts/" + validAddr))
			})
		})

		Context("when an API key is configured", func() {
			BeforeEach(func() {
				cfg.TronGridAPIKey = "secret-api-key"
				rc = tron.NewReadClient(cfg)
				body := fixture("accounts_with_usdt.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("forwards it as a TRON-PRO-API-KEY header", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				Expect(req.Header.Get("TRON-PRO-API-KEY")).To(Equal("secret-api-key"))
			})
		})

		Context("when the account has no matching USDT row", func() {
			BeforeEach(func() {
				body := fixture("accounts_no_usdt.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("returns sun balance and an empty USDT field", func() {
				out, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Sun).To(Equal(int64(7000000)))
				Expect(out.USDTMinimal).To(BeEmpty())
			})
		})

		Context("when data is empty but success is true", func() {
			BeforeEach(func() {
				body := fixture("accounts_empty.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("returns zeroed balances without an error", func() {
				out, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Address).To(Equal(validAddr))
				Expect(out.Sun).To(BeZero())
				Expect(out.USDTMinimal).To(BeEmpty())
			})
		})

		Context("when upstream returns success=false with empty data", func() {
			BeforeEach(func() {
				body := fixture("accounts_error.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("surfaces the upstream error message", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("address quota exceeded"))
			})
		})

		Context("when upstream returns HTTP 500", func() {
			BeforeEach(func() {
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, []byte("internal error body"), http.StatusInternalServerError)
				})
			})

			It("returns an error mentioning the status code and body", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("HTTP 500"))
				Expect(err.Error()).To(ContainSubstring("internal error body"))
			})
		})

		Context("when upstream returns malformed JSON", func() {
			BeforeEach(func() {
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, []byte("{not json"), http.StatusOK)
				})
			})

			It("returns a decode error", func() {
				_, err := rc.GetBalances(context.Background(), validAddr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("decode accounts"))
			})
		})

		Context("when network is shasta", func() {
			BeforeEach(func() {
				cfg.Network = config.NetworkShasta
				rc = tron.NewReadClient(cfg)
				body := fixture("accounts_shasta_with_usdt.json")
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					captureReq(w, r, body, http.StatusOK)
				})
			})

			It("matches the shasta test USDT contract row", func() {
				out, err := rc.GetBalances(context.Background(), "TJRyWwFs9wTFGZg3JbrVriFbNfCug5tDeC")
				Expect(err).NotTo(HaveOccurred())
				Expect(out.USDTMinimal).To(Equal("777000"))
			})
		})

		Context("when the request context is already cancelled", func() {
			BeforeEach(func() {
				mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(50 * time.Millisecond)
					captureReq(w, r, fixture("accounts_with_usdt.json"), http.StatusOK)
				})
			})

			It("returns a context error before completing the request", func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := rc.GetBalances(ctx, validAddr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("context canceled"))
			})
		})
	})

	Describe("ListUSDTTransfers", func() {
		const validAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"

		registerHandler := func(body []byte, status int) {
			mux.HandleFunc("/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
				captureReq(w, r, body, status)
			})
		}

		Context("when address is invalid", func() {
			It("returns an error and never calls the network", func() {
				rows, err := rc.ListUSDTTransfers(context.Background(), "not-an-addr", 10)
				Expect(err).To(HaveOccurred())
				Expect(rows).To(BeNil())
				Expect(hits.Load()).To(BeZero())
			})
		})

		DescribeTable("clamps the limit query parameter",
			func(in int, want string) {
				registerHandler(fixture("trc20_transfers_empty.json"), http.StatusOK)
				_, err := rc.ListUSDTTransfers(context.Background(), validAddr, in)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				Expect(req.URL.Query().Get("limit")).To(Equal(want))
			},
			Entry("zero limit defaults to 20", 0, "20"),
			Entry("negative limit defaults to 20", -10, "20"),
			Entry("explicit small limit passes through", 5, "5"),
			Entry("over-limit value is clamped to 200", 5_000, "200"),
		)

		Context("when sending a request", func() {
			It("includes only_confirmed=true and the mainnet contract address", func() {
				registerHandler(fixture("trc20_transfers_empty.json"), http.StatusOK)
				_, err := rc.ListUSDTTransfers(context.Background(), validAddr, 7)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				q := req.URL.Query()
				Expect(q.Get("only_confirmed")).To(Equal("true"))
				Expect(q.Get("contract_address")).To(Equal(tron.MainnetUSDTContract))
				Expect(req.URL.Path).To(HavePrefix("/v1/accounts/"))
				Expect(req.URL.Path).To(HaveSuffix("/transactions/trc20"))
			})

			It("uses the shasta test contract when network is shasta", func() {
				cfg.Network = config.NetworkShasta
				rc = tron.NewReadClient(cfg)
				registerHandler(fixture("trc20_transfers_empty.json"), http.StatusOK)
				_, err := rc.ListUSDTTransfers(context.Background(), validAddr, 1)
				Expect(err).NotTo(HaveOccurred())
				req := lastReq.Load()
				Expect(req).NotTo(BeNil())
				Expect(req.URL.Query().Get("contract_address")).To(Equal(tron.ShastaTestUSDTContract))
			})
		})

		Context("when upstream returns transfers", func() {
			BeforeEach(func() {
				registerHandler(fixture("trc20_transfers.json"), http.StatusOK)
			})

			It("maps every field, applying defaults for empty symbol/contract", func() {
				rows, err := rc.ListUSDTTransfers(context.Background(), validAddr, 100)
				Expect(err).NotTo(HaveOccurred())
				Expect(rows).To(HaveLen(2))

				first := rows[0]
				Expect(first.TxID).To(HavePrefix("aaaa"))
				Expect(first.From).To(Equal("TFrom1111111111111111111111111111"))
				Expect(first.To).To(Equal("TTo000000000000000000000000000000"))
				Expect(first.ValueMinimal).To(Equal("1500000"))
				Expect(first.TokenSymbol).To(Equal("USDT"))
				Expect(first.Contract).To(Equal(tron.MainnetUSDTContract))
				Expect(first.Type).To(Equal("Transfer"))
				Expect(first.BlockTimestamp.UTC()).To(Equal(time.UnixMilli(1700000000000).UTC()))

				second := rows[1]
				Expect(second.TokenSymbol).To(Equal("USDT"),
					"empty symbol must default to USDT")
				Expect(second.Contract).To(Equal(tron.MainnetUSDTContract),
					"empty contract address must default to network USDT contract")
				Expect(second.ValueMinimal).To(Equal("42"))
			})
		})

		Context("when upstream returns an empty data array", func() {
			BeforeEach(func() {
				registerHandler(fixture("trc20_transfers_empty.json"), http.StatusOK)
			})

			It("returns an empty slice with no error", func() {
				rows, err := rc.ListUSDTTransfers(context.Background(), validAddr, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(rows).To(BeEmpty())
			})
		})

		Context("when upstream returns HTTP 4xx", func() {
			BeforeEach(func() {
				body := strings.Repeat("X", 500)
				registerHandler([]byte(body), http.StatusTooManyRequests)
			})

			It("includes the status code and a truncated body in the error", func() {
				_, err := rc.ListUSDTTransfers(context.Background(), validAddr, 0)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("HTTP 429"))
				Expect(err.Error()).To(HaveSuffix("..."), "long body must be truncated")
			})
		})

		Context("when upstream returns malformed JSON", func() {
			BeforeEach(func() {
				registerHandler([]byte("not-json"), http.StatusOK)
			})

			It("returns a decode error", func() {
				_, err := rc.ListUSDTTransfers(context.Background(), validAddr, 0)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("decode trc20 txs"))
			})
		})
	})
})
