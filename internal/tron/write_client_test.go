package tron_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

const (
	// Well-known mainnet base58 addresses; only used in-process by bufconn
	// fakes, never sent over the network.
	testFromAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
	testToAddr   = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

	// trc20TransferSelector is the keccak256 4-byte selector for
	// transfer(address,uint256), as used by gotron-sdk's TRC20Send.
	trc20TransferSelector = "a9059cbb"
)

// recordedCalls captures gRPC calls observed by the in-process WalletServer
// fake so that black-box specs can assert on them without leaking impl details.
type recordedCalls struct {
	mu              sync.Mutex
	createTxCalls   []*core.TransferContract
	triggerCalls    []*core.TriggerSmartContract
	broadcastCalls  []*core.Transaction
	signWeightCalls []*core.Transaction
	feeLimitsSeen   []int64
}

func (r *recordedCalls) recordCreate(c *core.TransferContract) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createTxCalls = append(r.createTxCalls, c)
}

func (r *recordedCalls) recordTrigger(c *core.TriggerSmartContract) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.triggerCalls = append(r.triggerCalls, c)
}

func (r *recordedCalls) recordBroadcast(tx *core.Transaction, fee int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastCalls = append(r.broadcastCalls, tx)
	r.feeLimitsSeen = append(r.feeLimitsSeen, fee)
}

func (r *recordedCalls) recordSignWeight(tx *core.Transaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.signWeightCalls = append(r.signWeightCalls, tx)
}

// mockWallet is a configurable api.WalletServer fake. Each test installs only
// the handlers it needs by setting the *Func fields; the rest fall through to
// the embedded UnimplementedWalletServer (which returns codes.Unimplemented).
type mockWallet struct {
	api.UnimplementedWalletServer

	calls *recordedCalls

	createTx2      func(context.Context, *core.TransferContract) (*api.TransactionExtention, error)
	trigger        func(context.Context, *core.TriggerSmartContract) (*api.TransactionExtention, error)
	broadcast      func(context.Context, *core.Transaction) (*api.Return, error)
	getSignWeight  func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error)
}

func (m *mockWallet) CreateTransaction2(ctx context.Context, in *core.TransferContract) (*api.TransactionExtention, error) {
	m.calls.recordCreate(in)
	if m.createTx2 == nil {
		return nil, errors.New("mockWallet: CreateTransaction2 not configured")
	}
	return m.createTx2(ctx, in)
}

func (m *mockWallet) TriggerContract(ctx context.Context, in *core.TriggerSmartContract) (*api.TransactionExtention, error) {
	m.calls.recordTrigger(in)
	if m.trigger == nil {
		return nil, errors.New("mockWallet: TriggerContract not configured")
	}
	return m.trigger(ctx, in)
}

func (m *mockWallet) BroadcastTransaction(ctx context.Context, in *core.Transaction) (*api.Return, error) {
	var fee int64
	if in.GetRawData() != nil {
		fee = in.GetRawData().GetFeeLimit()
	}
	m.calls.recordBroadcast(in, fee)
	if m.broadcast == nil {
		return nil, errors.New("mockWallet: BroadcastTransaction not configured")
	}
	return m.broadcast(ctx, in)
}

func (m *mockWallet) GetTransactionSignWeight(ctx context.Context, in *core.Transaction) (*api.TransactionSignWeight, error) {
	m.calls.recordSignWeight(in)
	if m.getSignWeight == nil {
		return nil, errors.New("mockWallet: GetTransactionSignWeight not configured")
	}
	return m.getSignWeight(ctx, in)
}

// startBufconn spins up an in-process gRPC server bound to a bufconn listener
// and returns a dialed *grpc.ClientConn ready for [tron.NewWriteClientWithConn].
func startBufconn(mock *mockWallet) *grpc.ClientConn {
	GinkgoHelper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	api.RegisterWalletServer(srv, mock)
	go func() {
		defer GinkgoRecover()
		_ = srv.Serve(lis)
	}()
	DeferCleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = conn.Close() })
	return conn
}

func okExtention(raw *core.TransactionRaw) *api.TransactionExtention {
	return &api.TransactionExtention{
		Transaction: &core.Transaction{RawData: raw},
		Result:      &api.Return{Result: true, Code: api.Return_SUCCESS},
	}
}

func okBroadcast() *api.Return {
	return &api.Return{Result: true, Code: api.Return_SUCCESS}
}

func mustGenerateKey() *ecdsa.PrivateKey {
	GinkgoHelper()
	k, err := crypto.GenerateKey()
	Expect(err).NotTo(HaveOccurred())
	return k
}

// tronAddrBytesToBase58 converts a 21-byte TRON address payload (0x41 prefix
// + 20 bytes) into its base58check representation, mirroring what
// gotron-sdk does internally. We rely on it to assert that the contract
// address bytes that reach the fake match our expected mainnet/shasta string.
func tronAddrBytesToBase58(b []byte) string {
	GinkgoHelper()
	Expect(b).To(HaveLen(21), "TRON address payload must be 21 bytes (0x41 prefix + 20)")
	return common.EncodeCheck(b)
}

func mustParseBigInt(s string) *big.Int {
	GinkgoHelper()
	v, ok := new(big.Int).SetString(s, 10)
	Expect(ok).To(BeTrue(), "parse %q as decimal big.Int", s)
	return v
}

var _ = Describe("WriteClient", func() {
	var (
		calls *recordedCalls
		mock  *mockWallet
		conn  *grpc.ClientConn
		cfg   *config.Config
		wc    *tron.WriteClient
		priv  *ecdsa.PrivateKey
	)

	BeforeEach(func() {
		calls = &recordedCalls{}
		mock = &mockWallet{calls: calls}
		conn = startBufconn(mock)
		cfg = &config.Config{
			Network:            config.NetworkMainnet,
			HTTPBaseURL:        "",
			GRPCAddress:        "bufconn",
			TronGridAPIKey:     "",
			Output:             config.OutputTable,
			KeystoreDir:        "",
			DefaultFeeLimitSun: 100_000_000,
		}
		wc = tron.NewWriteClientWithConn(cfg, conn)
		priv = mustGenerateKey()
		DeferCleanup(func() { _ = wc.Close() })
	})

	Describe("SendTRX", func() {
		Context("when the upstream accepts the transfer", func() {
			BeforeEach(func() {
				mock.createTx2 = func(_ context.Context, _ *core.TransferContract) (*api.TransactionExtention, error) {
					raw := &core.TransactionRaw{
						RefBlockBytes: []byte{0xab, 0xcd},
						Expiration:    time.Now().Add(time.Minute).UnixMilli(),
						Timestamp:     time.Now().UnixMilli(),
						Data:          []byte("trx-transfer"),
					}
					return okExtention(raw), nil
				}
				mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
					return okBroadcast(), nil
				}
			})

			It("forwards the from/to/amount to CreateTransaction2", func() {
				_, err := wc.SendTRX(context.Background(), testFromAddr, testToAddr, 1_500_000, priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(calls.createTxCalls).To(HaveLen(1))
				got := calls.createTxCalls[0]
				Expect(got.GetAmount()).To(Equal(int64(1_500_000)))
				Expect(got.GetOwnerAddress()).NotTo(BeEmpty())
				Expect(got.GetToAddress()).NotTo(BeEmpty())
				Expect(got.GetOwnerAddress()).NotTo(Equal(got.GetToAddress()))
			})

			It("returns a 0x-prefixed 64-hex-char txid derived from RawData", func() {
				txid, err := wc.SendTRX(context.Background(), testFromAddr, testToAddr, 1, priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
			})

			It("eventually broadcasts the signed transaction", func() {
				_, err := wc.SendTRX(context.Background(), testFromAddr, testToAddr, 1, priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(calls.broadcastCalls).To(HaveLen(1))
				Expect(calls.broadcastCalls[0].GetSignature()).To(HaveLen(1),
					"signed transaction must carry exactly one ECDSA signature")
			})
		})

		Context("when CreateTransaction2 returns a gRPC error", func() {
			BeforeEach(func() {
				mock.createTx2 = func(context.Context, *core.TransferContract) (*api.TransactionExtention, error) {
					return nil, errors.New("upstream-create-failed")
				}
			})

			It("propagates the error and never broadcasts", func() {
				_, err := wc.SendTRX(context.Background(), testFromAddr, testToAddr, 1, priv)
				Expect(err).To(HaveOccurred())
				Expect(calls.broadcastCalls).To(BeEmpty())
			})
		})

		Context("when broadcast reports a non-success code", func() {
			BeforeEach(func() {
				mock.createTx2 = func(context.Context, *core.TransferContract) (*api.TransactionExtention, error) {
					return okExtention(&core.TransactionRaw{Data: []byte("x")}), nil
				}
				mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
					return &api.Return{
						Result:  false,
						Code:    api.Return_CONTRACT_VALIDATE_ERROR,
						Message: []byte("invalid signature"),
					}, nil
				}
			})

			It("surfaces the broadcast error message", func() {
				_, err := wc.SendTRX(context.Background(), testFromAddr, testToAddr, 1, priv)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid signature"))
			})
		})

		Context("when the from address is not a valid base58 address", func() {
			It("returns an error before any RPC is made", func() {
				_, err := wc.SendTRX(context.Background(), "not-base58", testToAddr, 1, priv)
				Expect(err).To(HaveOccurred())
				Expect(calls.createTxCalls).To(BeEmpty())
				Expect(calls.broadcastCalls).To(BeEmpty())
			})
		})

		Context("when the parent context is already cancelled", func() {
			var cancelled context.Context
			BeforeEach(func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				cancelled = ctx

				mock.createTx2 = func(context.Context, *core.TransferContract) (*api.TransactionExtention, error) {
					return okExtention(&core.TransactionRaw{Data: []byte("x")}), nil
				}
				mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
					return okBroadcast(), nil
				}
			})

			It("returns quickly with a context error", func() {
				done := make(chan error, 1)
				go func() {
					_, err := wc.SendTRX(cancelled, testFromAddr, testToAddr, 1, priv)
					done <- err
				}()
				Eventually(done, "2s").Should(Receive(HaveOccurred()))
			})
		})
	})

	Describe("SendUSDT", func() {
		Context("when the upstream accepts the transfer", func() {
			BeforeEach(func() {
				mock.trigger = func(_ context.Context, _ *core.TriggerSmartContract) (*api.TransactionExtention, error) {
					raw := &core.TransactionRaw{
						RefBlockBytes: []byte{0xff, 0xee},
						Expiration:    time.Now().Add(time.Minute).UnixMilli(),
						Timestamp:     time.Now().UnixMilli(),
						Data:          []byte("trc20-transfer"),
					}
					return okExtention(raw), nil
				}
				mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
					return okBroadcast(), nil
				}
			})

			It("targets the mainnet USDT contract by default", func() {
				_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("12345000"), priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(calls.triggerCalls).To(HaveLen(1))
				Expect(tronAddrBytesToBase58(calls.triggerCalls[0].GetContractAddress())).To(Equal(tron.MainnetUSDTContract))
			})

			It("packs the transfer(address,uint256) selector and 32-byte amount", func() {
				_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("65535"), priv)
				Expect(err).NotTo(HaveOccurred())

				data := calls.triggerCalls[0].GetData()
				Expect(data).To(HaveLen(4 + 32 + 32),
					"selector(4) + addr(32) + amount(32)")
				Expect(hex.EncodeToString(data[:4])).To(Equal(trc20TransferSelector))
				Expect(hex.EncodeToString(data[len(data)-32:])).To(Equal(
					"000000000000000000000000000000000000000000000000000000000000ffff"))
			})

			It("propagates DefaultFeeLimitSun into the broadcast transaction", func() {
				_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("1"), priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(calls.feeLimitsSeen).To(ConsistOf(int64(100_000_000)))
			})

			It("returns a 0x-prefixed 64-hex-char txid", func() {
				txid, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("1"), priv)
				Expect(err).NotTo(HaveOccurred())
				Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
			})

			Context("on shasta", func() {
				BeforeEach(func() {
					cfg.Network = config.NetworkShasta
					wc = tron.NewWriteClientWithConn(cfg, conn)
				})

				It("targets the shasta test USDT contract", func() {
					_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("1"), priv)
					Expect(err).NotTo(HaveOccurred())
					Expect(tronAddrBytesToBase58(calls.triggerCalls[0].GetContractAddress())).To(Equal(tron.ShastaTestUSDTContract))
				})
			})
		})

		Context("when broadcast reports failure", func() {
			BeforeEach(func() {
				mock.trigger = func(context.Context, *core.TriggerSmartContract) (*api.TransactionExtention, error) {
					return okExtention(&core.TransactionRaw{Data: []byte("x")}), nil
				}
				mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
					return &api.Return{
						Result:  false,
						Code:    api.Return_BANDWITH_ERROR,
						Message: []byte("not enough bandwidth"),
					}, nil
				}
			})

			It("surfaces the upstream message", func() {
				_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("1"), priv)
				Expect(err).To(HaveOccurred())
				Expect(strings.ToLower(err.Error())).To(ContainSubstring("not enough bandwidth"))
			})
		})

		Context("when TriggerContract reports a non-zero result code", func() {
			BeforeEach(func() {
				mock.trigger = func(context.Context, *core.TriggerSmartContract) (*api.TransactionExtention, error) {
					return &api.TransactionExtention{
						Transaction: &core.Transaction{RawData: &core.TransactionRaw{}},
						Result: &api.Return{
							Result:  false,
							Code:    api.Return_CONTRACT_VALIDATE_ERROR,
							Message: []byte("token contract not found"),
						},
					}, nil
				}
			})

			It("returns the contract validation error and never broadcasts", func() {
				_, err := wc.SendUSDT(context.Background(), testFromAddr, testToAddr, mustParseBigInt("1"), priv)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("token contract not found"))
				Expect(calls.broadcastCalls).To(BeEmpty())
			})
		})
	})

	Describe("Close", func() {
		It("releases the gRPC connection without error", func() {
			Expect(wc.Close()).To(Succeed())
		})
	})
})
