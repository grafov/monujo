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
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/transfer"
	"github.com/grafov/monujo/internal/tron"
)

func mustEncodeAny(msg proto.Message) *anypb.Any {
	GinkgoHelper()
	b, err := proto.Marshal(msg)
	Expect(err).NotTo(HaveOccurred())
	return &anypb.Any{
		TypeUrl: "type.googleapis.com/protocol." + string(proto.MessageName(msg).Name()),
		Value:   b,
	}
}

func mustDecodeBase58(addr string) []byte {
	GinkgoHelper()
	b, err := common.DecodeCheck(addr)
	Expect(err).NotTo(HaveOccurred())
	return b
}

var _ = Describe("Offline", func() {
	var (
		calls *recordedCalls
		mock  *mockWallet
		conn  *grpc.ClientConn
		cfg   *config.Config
		wc    *tron.GRPCWriteClient
	)

	BeforeEach(func() {
		calls = &recordedCalls{}
		mock = &mockWallet{calls: calls}
		conn = startBufconn(mock)
		cfg = &config.Config{
			Network:            config.NetworkMainnet,
			GRPCAddress:        "bufconn",
			Output:             config.OutputTable,
			DefaultFeeLimitSun: 100_000_000,
		}
		wc = tron.NewWriteClientWithConn(cfg, conn)
		DeferCleanup(func() { _ = wc.Close() })
	})

	Describe("BuildUnsignedTRX", func() {
		var ownerBytes []byte
		BeforeEach(func() {
			ownerBytes = mustDecodeBase58(testFromAddr)
			toBytes := mustDecodeBase58(testToAddr)
			mock.createTx2 = func(_ context.Context, in *core.TransferContract) (*api.TransactionExtention, error) {
				raw := &core.TransactionRaw{
					RefBlockBytes: []byte{0xab, 0xcd},
					Expiration:    time.Now().Add(time.Minute).UnixMilli(),
					Timestamp:     time.Now().UnixMilli(),
					Contract: []*core.Transaction_Contract{
						{
							Type: core.Transaction_Contract_TransferContract,
							Parameter: mustEncodeAny(&core.TransferContract{
								OwnerAddress: in.GetOwnerAddress(),
								ToAddress:    in.GetToAddress(),
								Amount:       in.GetAmount(),
							}),
						},
					},
				}
				_ = ownerBytes
				_ = toBytes
				return okExtention(raw), nil
			}
		})

		It("returns a payload with a hex-encoded raw_data, txid, and human fields", func() {
			pl, err := tron.BuildUnsignedTRX(context.Background(), wc, config.NetworkMainnet, testFromAddr, testToAddr, 1_500_000, "lunch")
			Expect(err).NotTo(HaveOccurred())
			Expect(pl.Kind).To(Equal(transfer.KindTRX))
			Expect(pl.From).To(Equal(testFromAddr))
			Expect(pl.To).To(Equal(testToAddr))
			Expect(pl.AmountMinimal).To(Equal("1500000"))
			Expect(pl.Amount).To(Equal("1.5"))
			Expect(pl.Note).To(Equal("lunch"))
			Expect(pl.RawDataHex).To(MatchRegexp(`^0x[0-9a-f]+$`))
			Expect(pl.TxIDHex).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
			Expect(pl.Chain).To(Equal(transfer.ChainTronMainnet))
		})

		It("rejects invalid base58 from-address before any RPC", func() {
			_, err := tron.BuildUnsignedTRX(context.Background(), wc, config.NetworkMainnet, "not-base58", testToAddr, 1, "")
			Expect(err).To(HaveOccurred())
			Expect(calls.createTxCalls).To(BeEmpty())
		})

		It("rejects non-positive amount", func() {
			_, err := tron.BuildUnsignedTRX(context.Background(), wc, config.NetworkMainnet, testFromAddr, testToAddr, 0, "")
			Expect(err).To(HaveOccurred())
			_, err = tron.BuildUnsignedTRX(context.Background(), wc, config.NetworkMainnet, testFromAddr, testToAddr, -1, "")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("BuildUnsignedUSDT", func() {
		BeforeEach(func() {
			mock.trigger = func(_ context.Context, in *core.TriggerSmartContract) (*api.TransactionExtention, error) {
				raw := &core.TransactionRaw{
					RefBlockBytes: []byte{0xff, 0xee},
					Expiration:    time.Now().Add(time.Minute).UnixMilli(),
					Timestamp:     time.Now().UnixMilli(),
					FeeLimit:      100_000_000,
					Contract: []*core.Transaction_Contract{
						{
							Type: core.Transaction_Contract_TriggerSmartContract,
							Parameter: mustEncodeAny(&core.TriggerSmartContract{
								OwnerAddress:    in.GetOwnerAddress(),
								ContractAddress: in.GetContractAddress(),
								Data:            in.GetData(),
							}),
						},
					},
				}
				return okExtention(raw), nil
			}
		})

		It("populates Contract and FeeLimitSun from raw_data", func() {
			pl, err := tron.BuildUnsignedUSDT(context.Background(), wc, config.NetworkMainnet, 100_000_000, testFromAddr, testToAddr, mustParseBigInt("12345000"), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(pl.Kind).To(Equal(transfer.KindTRC20USDT))
			Expect(pl.Contract).To(Equal(tron.MainnetUSDTContract))
			Expect(pl.AmountMinimal).To(Equal("12345000"))
			Expect(pl.Amount).To(ContainSubstring("USDT"))

			summary, err := tron.DecodeRawDataSummary(pl.RawDataHex)
			Expect(err).NotTo(HaveOccurred())
			Expect(pl.FeeLimitSun).To(Equal(summary.FeeLimitSun))
			Expect(pl.FeeLimitSun).To(BeNumerically(">", 0))
		})
	})

	Describe("SignRawData and DecodeRawDataSummary", func() {
		It("round-trips: signing the raw data produces a valid 65-byte sig and a stable txid", func() {
			raw := &core.TransactionRaw{
				RefBlockBytes: []byte{0x12, 0x34},
				Expiration:    time.Now().Add(time.Minute).UnixMilli(),
				Timestamp:     time.Now().UnixMilli(),
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TransferContract,
						Parameter: mustEncodeAny(&core.TransferContract{
							OwnerAddress: mustDecodeBase58(testFromAddr),
							ToAddress:    mustDecodeBase58(testToAddr),
							Amount:       42,
						}),
					},
				},
			}
			rawBytes, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())
			rawHex := "0x" + hex.EncodeToString(rawBytes)

			priv, err := crypto.GenerateKey()
			Expect(err).NotTo(HaveOccurred())

			sig, txid, err := tron.SignRawData(rawHex, priv)
			Expect(err).NotTo(HaveOccurred())
			Expect(sig).To(MatchRegexp(`^0x[0-9a-f]{130}$`))
			Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))

			summary, err := tron.DecodeRawDataSummary(rawHex)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Kind).To(Equal(transfer.KindTRX))
			Expect(summary.From).To(Equal(testFromAddr))
			Expect(summary.To).To(Equal(testToAddr))
			Expect(summary.AmountWei.Int64()).To(Equal(int64(42)))
		})

		It("decodes a TRC-20 transfer call back to base58 to and amount", func() {
			toBytes := mustDecodeBase58(testToAddr)
			data := append([]byte{0xa9, 0x05, 0x9c, 0xbb}, make([]byte, 64)...)
			copy(data[4+12:], toBytes[1:])
			data[len(data)-1] = 0xff

			raw := &core.TransactionRaw{
				Expiration: time.Now().Add(time.Minute).UnixMilli(),
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TriggerSmartContract,
						Parameter: mustEncodeAny(&core.TriggerSmartContract{
							OwnerAddress:    mustDecodeBase58(testFromAddr),
							ContractAddress: mustDecodeBase58(tron.MainnetUSDTContract),
							Data:            data,
						}),
					},
				},
			}
			rawBytes, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())
			rawHex := hex.EncodeToString(rawBytes)

			summary, err := tron.DecodeRawDataSummary(rawHex)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Kind).To(Equal(transfer.KindTRC20USDT))
			Expect(summary.From).To(Equal(testFromAddr))
			Expect(summary.To).To(Equal(testToAddr))
			Expect(summary.Contract).To(Equal(tron.MainnetUSDTContract))
			Expect(summary.AmountWei.Int64()).To(Equal(int64(0xff)))
			Expect(summary.CallValueSun).To(Equal(int64(0)))
			Expect(summary.Memo).To(BeNil())
		})

		It("records attached value, memo and a non-canonical token label", func() {
			toBytes := mustDecodeBase58(testToAddr)
			data := append([]byte{0xa9, 0x05, 0x9c, 0xbb}, make([]byte, 64)...)
			copy(data[4+12:], toBytes[1:])
			data[len(data)-1] = 0x02

			raw := &core.TransactionRaw{
				Expiration: time.Now().Add(time.Minute).UnixMilli(),
				Data:       []byte("note"),
				Contract: []*core.Transaction_Contract{
					{
						Type:         core.Transaction_Contract_TriggerSmartContract,
						PermissionId: 3,
						Parameter: mustEncodeAny(&core.TriggerSmartContract{
							OwnerAddress:    mustDecodeBase58(testFromAddr),
							ContractAddress: mustDecodeBase58(testToAddr),
							Data:            data,
							CallValue:       5_000_000,
							CallTokenValue:  7,
							TokenId:         1002000,
						}),
					},
				},
			}
			rawBytes, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())

			summary, err := tron.DecodeRawDataSummary(hex.EncodeToString(rawBytes))
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Kind).To(Equal(transfer.KindTRC20USDT))
			Expect(summary.Contract).To(Equal(testToAddr))
			Expect(summary.AmountStr).To(Equal("2 (contract " + testToAddr + ")"))
			Expect(summary.AmountStr).NotTo(HaveSuffix("USDT"))
			Expect(summary.CallValueSun).To(Equal(int64(5_000_000)))
			Expect(summary.CallTokenValue).To(Equal(int64(7)))
			Expect(summary.TokenID).To(Equal(int64(1002000)))
			Expect(summary.PermissionID).To(Equal(int32(3)))
			Expect(summary.Memo).To(Equal([]byte("note")))
		})

		It("rejects unsupported contract types", func() {
			raw := &core.TransactionRaw{
				Contract: []*core.Transaction_Contract{
					{Type: core.Transaction_Contract_AccountCreateContract},
				},
			}
			rawBytes, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())
			_, err = tron.DecodeRawDataSummary(hex.EncodeToString(rawBytes))
			Expect(err).To(HaveOccurred())
			Expect(strings.ToLower(err.Error())).To(ContainSubstring("unsupported"))
		})
	})

	Describe("BroadcastSigned", func() {
		It("rebuilds tx from raw_data + signature and forwards to gRPC", func() {
			raw := &core.TransactionRaw{
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TransferContract,
						Parameter: mustEncodeAny(&core.TransferContract{
							OwnerAddress: mustDecodeBase58(testFromAddr),
							ToAddress:    mustDecodeBase58(testToAddr),
							Amount:       1,
						}),
					},
				},
			}
			rawBytes, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())
			rawHex := "0x" + hex.EncodeToString(rawBytes)

			priv, err := crypto.GenerateKey()
			Expect(err).NotTo(HaveOccurred())
			sig, _, err := tron.SignRawData(rawHex, priv)
			Expect(err).NotTo(HaveOccurred())

			mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
				return okBroadcast(), nil
			}

			txid, err := tron.BroadcastSigned(context.Background(), wc, rawHex, sig)
			Expect(err).NotTo(HaveOccurred())
			Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
			Expect(calls.broadcastCalls).To(HaveLen(1))
			Expect(calls.broadcastCalls[0].GetSignature()).To(HaveLen(1))
		})

		It("rejects malformed signature length", func() {
			raw := &core.TransactionRaw{}
			b, err := proto.Marshal(raw)
			Expect(err).NotTo(HaveOccurred())
			_, err = tron.BroadcastSigned(context.Background(), wc, hex.EncodeToString(b), "0x"+strings.Repeat("ab", 64))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("BuildUnsigned rejects a substituted node transaction", func() {
		It("rejects a TRX transfer whose recipient was swapped", func() {
			tx := plainTRXTx(testFromAddr, testFromAddr, 1_500_000)
			_, err := tron.BuildUnsignedTRX(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, testFromAddr, testToAddr, 1_500_000, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("to="))
		})

		It("rejects a TRX transfer whose amount was swapped", func() {
			tx := plainTRXTx(testFromAddr, testToAddr, 2)
			_, err := tron.BuildUnsignedTRX(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, testFromAddr, testToAddr, 1, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("amount="))
		})

		It("rejects a TRX transfer whose owner was swapped", func() {
			tx := plainTRXTx(testToAddr, testToAddr, 1)
			_, err := tron.BuildUnsignedTRX(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, testFromAddr, testToAddr, 1, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("from="))
		})

		It("rejects a TRX transfer that carries a memo", func() {
			tx := plainTRXTx(testFromAddr, testToAddr, 1)
			tx.RawData.Data = []byte("memo")
			_, err := tron.BuildUnsignedTRX(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, testFromAddr, testToAddr, 1, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("memo"))
		})

		It("rejects a transaction with two contracts", func() {
			tx := plainTRXTx(testFromAddr, testToAddr, 1)
			tx.RawData.Contract = append(tx.RawData.Contract, tx.RawData.Contract[0])
			_, err := tron.BuildUnsignedTRX(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, testFromAddr, testToAddr, 1, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("contracts=2"))
		})

		It("rejects a USDT transfer with attached call_value", func() {
			tx := plainUSDTTx(testFromAddr, testToAddr, tron.MainnetUSDTContract, 255, 1_000_000, 100_000_000)
			_, err := tron.BuildUnsignedUSDT(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, 100_000_000, testFromAddr, testToAddr, big.NewInt(255), "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("call_value"))
		})

		It("rejects a USDT transfer aimed at a different contract", func() {
			tx := plainUSDTTx(testFromAddr, testToAddr, testToAddr, 255, 0, 100_000_000)
			_, err := tron.BuildUnsignedUSDT(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, 100_000_000, testFromAddr, testToAddr, big.NewInt(255), "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("canonical USDT"))
		})

		It("rejects a USDT transfer whose fee_limit was raised", func() {
			tx := plainUSDTTx(testFromAddr, testToAddr, tron.MainnetUSDTContract, 255, 0, 999)
			_, err := tron.BuildUnsignedUSDT(context.Background(), &fixedWriter{tx: tx}, config.NetworkMainnet, 100_000_000, testFromAddr, testToAddr, big.NewInt(255), "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fee_limit="))
		})
	})
})

type fixedWriter struct {
	tx *core.Transaction
}

func (f *fixedWriter) Close() error { return nil }

func (f *fixedWriter) SendTRX(context.Context, string, string, int64, *ecdsa.PrivateKey) (string, error) {
	return "", errors.New("unused")
}

func (f *fixedWriter) SendUSDT(context.Context, string, string, *big.Int, *ecdsa.PrivateKey) (string, error) {
	return "", errors.New("unused")
}

func (f *fixedWriter) CreateTRXTransfer(context.Context, string, string, int64) (*core.Transaction, error) {
	return f.tx, nil
}

func (f *fixedWriter) CreateTRC20Transfer(context.Context, string, string, string, *big.Int) (*core.Transaction, error) {
	return f.tx, nil
}

func (f *fixedWriter) BroadcastTransaction(context.Context, *core.Transaction) error {
	return errors.New("unused")
}

func (f *fixedWriter) GetTransactionSignWeight(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
	return nil, errors.New("unused")
}

func plainTRXTx(from, to string, amount int64) *core.Transaction {
	return &core.Transaction{RawData: &core.TransactionRaw{
		Expiration: time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:  time.Now().UnixMilli(),
		Contract: []*core.Transaction_Contract{{
			Type: core.Transaction_Contract_TransferContract,
			Parameter: mustEncodeAny(&core.TransferContract{
				OwnerAddress: mustDecodeBase58(from),
				ToAddress:    mustDecodeBase58(to),
				Amount:       amount,
			}),
		}},
	}}
}

func plainUSDTTx(from, to, contract string, amount byte, callValue, fee int64) *core.Transaction {
	toBytes := mustDecodeBase58(to)
	data := append([]byte{0xa9, 0x05, 0x9c, 0xbb}, make([]byte, 64)...)
	copy(data[16:36], toBytes[1:])
	data[len(data)-1] = amount
	return &core.Transaction{RawData: &core.TransactionRaw{
		Expiration: time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:  time.Now().UnixMilli(),
		FeeLimit:   fee,
		Contract: []*core.Transaction_Contract{{
			Type: core.Transaction_Contract_TriggerSmartContract,
			Parameter: mustEncodeAny(&core.TriggerSmartContract{
				OwnerAddress:    mustDecodeBase58(from),
				ContractAddress: mustDecodeBase58(contract),
				Data:            data,
				CallValue:       callValue,
			}),
		}},
	}}
}
