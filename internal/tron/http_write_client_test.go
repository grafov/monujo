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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

var _ = Describe("HTTPWriteClient", func() {
	var (
		ts  *httptest.Server
		cfg *config.Config
		hwc *tron.HTTPWriteClient
	)

	BeforeEach(func() {
		sampleRaw, err := proto.Marshal(&core.TransactionRaw{
			RefBlockBytes: []byte{0xab, 0xcd},
			Expiration:    time.Now().Add(time.Minute).UnixMilli(),
			Timestamp:     time.Now().UnixMilli(),
		})
		Expect(err).NotTo(HaveOccurred())
		sampleHex := hex.EncodeToString(sampleRaw)

		mux := http.NewServeMux()
		mux.HandleFunc("/wallet/createtransaction", func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Header.Get("TRON-PRO-API-KEY")).To(Equal("test-key"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"txID":         strings.Repeat("aa", 32),
				"raw_data_hex": sampleHex,
			})
		})
		mux.HandleFunc("/wallet/triggersmartcontract", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"result": true},
				"transaction": map[string]any{
					"txID":         strings.Repeat("bb", 32),
					"raw_data_hex": sampleHex,
				},
			})
		})
		mux.HandleFunc("/wallet/broadcasthex", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": true,
				"code":   "SUCCESS",
				"txid":   strings.Repeat("cc", 32),
			})
		})
		mux.HandleFunc("/wallet/getsignweight", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"current_weight": 2,
				"permission": map[string]any{
					"id":        2,
					"threshold": 2,
				},
				"approved_list": []string{},
				"result":        map[string]any{"code": "ENOUGH_PERMISSION"},
			})
		})
		ts = httptest.NewServer(mux)
		cfg = &config.Config{
			Network:            config.NetworkMainnet,
			HTTPBaseURL:        ts.URL,
			TronGridAPIKey:     "test-key",
			DefaultFeeLimitSun: 100_000_000,
		}
		hwc = tron.NewHTTPWriteClientFromConfig(cfg)
	})

	AfterEach(func() {
		ts.Close()
	})

	It("creates TRX transfer via REST", func() {
		tx, err := hwc.CreateTRXTransfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", 1_000_000)
		Expect(err).NotTo(HaveOccurred())
		Expect(tx.GetRawData()).NotTo(BeNil())
	})

	It("creates USDT transfer via REST", func() {
		amt := mustParseBigInt("1000")
		tx, err := hwc.CreateTRC20Transfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", tron.MainnetUSDTContract, amt)
		Expect(err).NotTo(HaveOccurred())
		Expect(tx.GetRawData()).NotTo(BeNil())
	})

	It("broadcasts signed transaction via broadcasthex", func() {
		tx, err := hwc.CreateTRXTransfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(hwc.BroadcastTransaction(context.Background(), tx)).To(Succeed())
	})

	It("queries sign weight via REST", func() {
		tx, err := hwc.CreateTRXTransfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", 1)
		Expect(err).NotTo(HaveOccurred())
		weight, err := hwc.GetTransactionSignWeight(context.Background(), tx)
		Expect(err).NotTo(HaveOccurred())
		Expect(weight.GetCurrentWeight()).To(Equal(int64(2)))
		Expect(weight.GetPermission().GetThreshold()).To(Equal(int64(2)))
	})
})
