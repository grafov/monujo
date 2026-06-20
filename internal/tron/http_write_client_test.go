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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

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

	It("packs TRC20 transfer data with selector and 32-byte padded address", func() {
		var capturedData string
		ts.Close()
		sampleRaw, err := proto.Marshal(&core.TransactionRaw{
			RefBlockBytes: []byte{0xab, 0xcd},
			Expiration:    time.Now().Add(time.Minute).UnixMilli(),
			Timestamp:     time.Now().UnixMilli(),
		})
		Expect(err).NotTo(HaveOccurred())
		sampleHex := hex.EncodeToString(sampleRaw)

		mux := http.NewServeMux()
		mux.HandleFunc("/wallet/triggersmartcontract", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			var req map[string]any
			Expect(json.Unmarshal(body, &req)).To(Succeed())
			capturedData, _ = req["data"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"result": true},
				"transaction": map[string]any{
					"txID":         strings.Repeat("bb", 32),
					"raw_data_hex": sampleHex,
				},
			})
		})
		ts = httptest.NewServer(mux)
		cfg.HTTPBaseURL = ts.URL
		hwc = tron.NewHTTPWriteClientFromConfig(cfg)

		const toAddr = "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1"
		amt := mustParseBigInt("65535")
		_, err = hwc.CreateTRC20Transfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", toAddr, tron.MainnetUSDTContract, amt)
		Expect(err).NotTo(HaveOccurred())

		toBytes, err := address.Base58ToAddress(toAddr)
		Expect(err).NotTo(HaveOccurred())
		addrPayload := toBytes.Bytes()[1:]
		addrPadded := common.LeftPadBytes(addrPayload, 32)
		amountPadded := common.LeftPadBytes(amt.Bytes(), 32)
		expected := "a9059cbb" + common.Bytes2Hex(addrPadded) + common.Bytes2Hex(amountPadded)
		Expect(capturedData).To(Equal(expected))
	})

	It("returns error when trigger contract reports code without message", func() {
		ts.Close()
		mux := http.NewServeMux()
		mux.HandleFunc("/wallet/triggersmartcontract", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"code": 1, "message": ""},
			})
		})
		ts = httptest.NewServer(mux)
		cfg.HTTPBaseURL = ts.URL
		hwc = tron.NewHTTPWriteClientFromConfig(cfg)

		_, err := hwc.CreateTRC20Transfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", tron.MainnetUSDTContract, mustParseBigInt("1"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error code 1"))
	})

	It("broadcasts signed transaction via broadcasthex", func() {
		tx, err := hwc.CreateTRXTransfer(context.Background(), "TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g", "TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1", 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(hwc.BroadcastTransaction(context.Background(), tx)).To(Succeed())
	})

	It("queries sign weight via REST and asserts request shape", func() {
		fromBytes, err := address.Base58ToAddress("TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g")
		Expect(err).NotTo(HaveOccurred())
		toBytes, err := address.Base58ToAddress("TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1")
		Expect(err).NotTo(HaveOccurred())

		param := &core.TransferContract{
			OwnerAddress: fromBytes.Bytes(),
			ToAddress:    toBytes.Bytes(),
			Amount:       1,
		}
		anyParam, err := proto.Marshal(param)
		Expect(err).NotTo(HaveOccurred())

		tx := &core.Transaction{
			RawData: &core.TransactionRaw{
				RefBlockBytes: []byte{0x12, 0x34},
				RefBlockHash:  []byte{0x56, 0x78},
				Expiration:    time.Now().Add(time.Minute).UnixMilli(),
				Timestamp:     time.Now().UnixMilli(),
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TransferContract,
						Parameter: &anypb.Any{
							TypeUrl: "type.googleapis.com/protocol.TransferContract",
							Value:   anyParam,
						},
					},
				},
			},
		}

		var capturedReq map[string]any
		ts.Close()

		mux := http.NewServeMux()
		mux.HandleFunc("/wallet/getsignweight", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(body, &capturedReq)).To(Succeed())

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
		cfg.HTTPBaseURL = ts.URL
		hwc = tron.NewHTTPWriteClientFromConfig(cfg)

		weight, err := hwc.GetTransactionSignWeight(context.Background(), tx)
		Expect(err).NotTo(HaveOccurred())
		Expect(weight.GetCurrentWeight()).To(Equal(int64(2)))

		Expect(capturedReq["visible"]).To(BeTrue())
		Expect(capturedReq["raw_data_hex"]).NotTo(BeEmpty())
		Expect(capturedReq["signature"]).To(BeEmpty())

		rawData, ok := capturedReq["raw_data"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(rawData["ref_block_bytes"]).NotTo(BeEmpty())
		Expect(rawData["ref_block_hash"]).NotTo(BeEmpty())

		contracts, ok := rawData["contract"].([]any)
		Expect(ok).To(BeTrue())
		Expect(contracts).To(HaveLen(1))

		c, ok := contracts[0].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(c["type"]).To(Equal("TransferContract"))

		paramJSON, ok := c["parameter"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(paramJSON["type_url"]).To(Equal("type.googleapis.com/protocol.TransferContract"))

		val, ok := paramJSON["value"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(val["amount"]).To(BeEquivalentTo(1))
		Expect(val["owner_address"]).To(Equal("TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g"))
		Expect(val["to_address"]).To(Equal("TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1"))
	})

	It("queries sign weight for a multisig signed transaction with Permission_id", func() {
		fromBytes, err := address.Base58ToAddress("TZ4UXDV5ZhNW7fb2AMSbgfAEZ7hWsnYS2g")
		Expect(err).NotTo(HaveOccurred())
		toBytes, err := address.Base58ToAddress("TPswDDCAWhJAZGdHPidFg5nEf8TkNToDX1")
		Expect(err).NotTo(HaveOccurred())

		param := &core.TransferContract{
			OwnerAddress: fromBytes.Bytes(),
			ToAddress:    toBytes.Bytes(),
			Amount:       1,
		}
		anyParam, err := proto.Marshal(param)
		Expect(err).NotTo(HaveOccurred())

		tx := &core.Transaction{
			RawData: &core.TransactionRaw{
				RefBlockBytes: []byte{0x12, 0x34},
				RefBlockHash:  []byte{0x56, 0x78},
				Expiration:    time.Now().Add(time.Minute).UnixMilli(),
				Timestamp:     time.Now().UnixMilli(),
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TransferContract,
						Parameter: &anypb.Any{
							TypeUrl: "type.googleapis.com/protocol.TransferContract",
							Value:   anyParam,
						},
						PermissionId: 2,
					},
				},
			},
			Signature: [][]byte{make([]byte, 65)},
		}

		var capturedReq map[string]any
		ts.Close()

		mux := http.NewServeMux()
		mux.HandleFunc("/wallet/getsignweight", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(json.Unmarshal(body, &capturedReq)).To(Succeed())

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
		cfg.HTTPBaseURL = ts.URL
		hwc = tron.NewHTTPWriteClientFromConfig(cfg)

		_, err = hwc.GetTransactionSignWeight(context.Background(), tx)
		Expect(err).NotTo(HaveOccurred())

		Expect(capturedReq["signature"]).To(HaveLen(1))

		rawData := capturedReq["raw_data"].(map[string]any)
		contracts := rawData["contract"].([]any)
		Expect(contracts).To(HaveLen(1))

		c := contracts[0].(map[string]any)
		Expect(c["Permission_id"]).To(BeEquivalentTo(2))
	})
})
