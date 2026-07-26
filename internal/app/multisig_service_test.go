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

package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
	"github.com/grafov/monujo/internal/tron/trontest"
	"github.com/grafov/monujo/internal/transfer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// multisigFakeWallet is a minimal [api.WalletServer] answering
// GetTransactionSignWeight/BroadcastTransaction so MultisigService can be
// exercised end-to-end without hitting a real Tron node.
type multisigFakeWallet struct {
	api.UnimplementedWalletServer

	mu             sync.Mutex
	broadcastCalls []*core.Transaction
	weightResp     func(*core.Transaction) *api.TransactionSignWeight
}

func (f *multisigFakeWallet) GetTransactionSignWeight(_ context.Context, in *core.Transaction) (*api.TransactionSignWeight, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.weightResp != nil {
		return f.weightResp(in), nil
	}
	return &api.TransactionSignWeight{}, nil
}

func (f *multisigFakeWallet) BroadcastTransaction(_ context.Context, in *core.Transaction) (*api.Return, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcastCalls = append(f.broadcastCalls, in)
	return &api.Return{Result: true, Code: api.Return_SUCCESS}, nil
}

func startMultisigFake() (*multisigFakeWallet, *grpc.ClientConn) {
	GinkgoHelper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	fw := &multisigFakeWallet{}
	api.RegisterWalletServer(srv, fw)
	go func() { _ = srv.Serve(lis) }()
	DeferCleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
	)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { _ = conn.Close() })
	return fw, conn
}

// multisigTestRawDataHex crafts a TRX TransferContract raw_data hex for
// the multisig service tests. The from/to addresses and amount are the
// inputs the test wants to see round-trip all the way through.
func multisigTestRawDataHex(fromBase58, toBase58 string, amountSun int64, permissionID int32) string {
	GinkgoHelper()
	fromBytes := mustDecodeB58(fromBase58)
	toBytes := mustDecodeB58(toBase58)
	param := &core.TransferContract{
		OwnerAddress: fromBytes,
		ToAddress:    toBytes,
		Amount:       amountSun,
	}
	pbytes, err := proto.Marshal(param)
	Expect(err).NotTo(HaveOccurred())
	raw := &core.TransactionRaw{
		RefBlockBytes: []byte{0x12, 0x34},
		Expiration:    time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:     time.Now().UnixMilli(),
		Contract: []*core.Transaction_Contract{
			{
				Type:         core.Transaction_Contract_TransferContract,
				Parameter:    &anypb.Any{TypeUrl: "type.googleapis.com/protocol.TransferContract", Value: pbytes},
				PermissionId: permissionID,
			},
		},
	}
	b, err := proto.Marshal(raw)
	Expect(err).NotTo(HaveOccurred())
	return hex.EncodeToString(b)
}

func mustDecodeB58(a string) []byte {
	GinkgoHelper()
	addr, err := address.Base58ToAddress(a)
	Expect(err).NotTo(HaveOccurred())
	return addr.Bytes()
}

// newMultisigForTest builds a MultisigService bound to the in-process
// bufconn fake and to a temp keystore containing a single wallet whose
// private key the test can sign with. The returned address is used as
// the "from" of the multisig transaction.
func newMultisigForTest(conn *grpc.ClientConn) (svc *MultisigService, ksDir, walletName, walletAddr string, walletPass []byte, cfg *config.Config) {
	GinkgoHelper()
	ksDir = GinkgoT().TempDir()
	cfg, err := config.Build(config.NetworkMainnet, config.OutputJSON, "", "bufconn", "", ksDir)
	Expect(err).NotTo(HaveOccurred())
	walletName = "cosigner"
	walletPass = []byte("pass1234abcd!!!")
	addr, _, err := keystore.GenerateAndStore(ksDir, walletName, walletPass, []string{string(cfg.Network)})
	Expect(err).NotTo(HaveOccurred())
	walletAddr = addr

	svc = NewMultisigService(cfg)
	svc.onlineWriter = func() (tron.TransactionWriter, func(), error) {
		return trontest.NewWriteClientWithConn(cfg, conn), func() {}, nil
	}
	svc.confirmIn = strings.NewReader("y\n")
	svc.confirmOut = io.Discard
	svc.stdoutOverride = io.Discard
	tt := true
	svc.terminalOverride = &tt
	return svc, ksDir, walletName, walletAddr, walletPass, cfg
}

var _ = Describe("MultisigService", func() {
	const toAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"

	It("cosigns a Tron JSON input and writes it back in Tron JSON", func() {
		_, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 1_500_000, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		inPath := filepath.Join(dir, "in.json")
		outPath := filepath.Join(dir, "out.json")
		Expect(os.WriteFile(inPath, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{
			InPath:     inPath,
			OutPath:    outPath,
			Format:     MultisigFormatAuto,
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).NotTo(HaveOccurred())

		outBytes, err := os.ReadFile(outPath)
		Expect(err).NotTo(HaveOccurred())

		gotRaw, gotSigs, _, err := tron.DecodeTronJSON(outBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotRaw).To(Equal(rawHex))
		Expect(gotSigs).To(HaveLen(1))
	})

	It("converts Tron JSON input to a sealed monujo envelope when --format=monujo", func() {
		_, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 42_000, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		inPath := filepath.Join(dir, "in.json")
		outPath := filepath.Join(dir, "out.json")
		Expect(os.WriteFile(inPath, body, 0o600)).To(Succeed())

		hmacPass := []byte("shared-hmac")
		_, err = svc.Cosign(CosignOptions{
			InPath:         inPath,
			OutPath:        outPath,
			Format:         MultisigFormatAuto,
			OutFormat:      MultisigFormatMonujo,
			WalletName:     name,
			WalletPass:     pass,
			HMACPassphrase: append([]byte(nil), hmacPass...),
			Yes:            true,
		})
		Expect(err).NotTo(HaveOccurred())

		outBytes, err := os.ReadFile(outPath)
		Expect(err).NotTo(HaveOccurred())
		var probe map[string]any
		Expect(json.Unmarshal(outBytes, &probe)).To(Succeed())
		Expect(probe).To(HaveKey("monujo"))

		hmacKey, err := transfer.DeriveHMACKey(append([]byte(nil), hmacPass...))
		Expect(err).NotTo(HaveOccurred())
		inner, _, err := transfer.OpenEnvelope(outBytes, transfer.KindMultisigTransfer, hmacKey)
		Expect(err).NotTo(HaveOccurred(), "raw=%s", string(outBytes))
		pl, err := transfer.UnmarshalMultisigPayload(inner)
		Expect(err).NotTo(HaveOccurred())
		Expect(pl.SignaturesHex).To(HaveLen(1))
		Expect(pl.RawDataHex).To(Equal(rawHex))
		Expect(pl.Kind).To(Equal(transfer.KindTRX))
		Expect(pl.From).To(Equal(from))
		Expect(pl.To).To(Equal(toAddr))
	})

	It("rejects monujo multisig envelopes whose display amount disagrees with raw_data", func() {
		_, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 1000, 2)
		pl := transfer.MultisigTransferPayload{
			Version:       transfer.EnvelopeVersion,
			Chain:         transfer.ChainTronMainnet,
			Kind:          transfer.KindTRX,
			PermissionID:  2,
			From:          from,
			To:            toAddr,
			Amount:        "999",
			AmountMinimal: "999000000",
			RawDataHex:    rawHex,
			TxIDHex:       "0xdeadbeef",
			Expiration:    time.Now().Add(time.Minute).UnixMilli(),
		}
		payload, err := transfer.MarshalMultisigPayload(pl)
		Expect(err).NotTo(HaveOccurred())
		body, err := transfer.SealPayload(transfer.KindMultisigTransfer, payload, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		inPath := filepath.Join(dir, "in.json")
		outPath := filepath.Join(dir, "out.json")
		Expect(os.WriteFile(inPath, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{
			InPath:     inPath,
			OutPath:    outPath,
			Format:     MultisigFormatMonujo,
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("amount mismatch"))
	})

	It("refuses to sign twice with the same key (ErrAlreadySigned, no --force)", func() {
		_, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 1000, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		inPath := filepath.Join(dir, "in.json")
		outPath := filepath.Join(dir, "out.json")
		Expect(os.WriteFile(inPath, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{InPath: inPath, OutPath: outPath, WalletName: name, WalletPass: pass, Yes: true})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Cosign(CosignOptions{InPath: outPath, OutPath: outPath, WalletName: name, WalletPass: pass, Yes: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("already signed"))
	})

	It("broadcasts only when weight reaches threshold, surfaces node weight in result", func() {
		fw, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 100, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		cosigned := filepath.Join(dir, "cosigned.json")
		Expect(os.WriteFile(cosigned, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{
			InPath: cosigned, OutPath: cosigned, WalletName: name, WalletPass: pass, Yes: true, Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())

		ownerBytes := mustDecodeB58(from)
		fw.weightResp = func(tx *core.Transaction) *api.TransactionSignWeight {
			_ = tx
			return &api.TransactionSignWeight{
				Permission: &core.Permission{
					Id:        2,
					Threshold: 1,
					Keys: []*core.Key{
						{Address: ownerBytes, Weight: 1},
					},
				},
				ApprovedList:  [][]byte{ownerBytes},
				CurrentWeight: 1,
			}
		}

		res, err := svc.BroadcastMultisig(context.Background(), BroadcastMultisigOptions{
			InPath: cosigned,
			Yes:    true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.TxID).To(HavePrefix("0x"))
		Expect(res.CurrentWeight).To(Equal(int64(1)))
		Expect(res.Threshold).To(Equal(int64(1)))
		Expect(res.Signers).To(ConsistOf(from))
		Expect(fw.broadcastCalls).To(HaveLen(1))
	})

	It("refuses broadcast below threshold and does not call BroadcastTransaction", func() {
		fw, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 100, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		cosigned := filepath.Join(dir, "cosigned.json")
		Expect(os.WriteFile(cosigned, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{
			InPath: cosigned, OutPath: cosigned, WalletName: name, WalletPass: pass, Yes: true, Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())

		ownerBytes := mustDecodeB58(from)
		fw.weightResp = func(_ *core.Transaction) *api.TransactionSignWeight {
			return &api.TransactionSignWeight{
				Permission: &core.Permission{
					Id:        2,
					Threshold: 2,
					Keys: []*core.Key{
						{Address: ownerBytes, Weight: 1},
						{Address: bytes.Repeat([]byte{0x41}, 21), Weight: 1},
					},
				},
				ApprovedList:  [][]byte{ownerBytes},
				CurrentWeight: 1,
			}
		}

		_, err = svc.BroadcastMultisig(context.Background(), BroadcastMultisigOptions{
			InPath: cosigned,
			Yes:    true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("threshold"))
		Expect(fw.broadcastCalls).To(BeEmpty())
	})

	It("Info surfaces current_weight, threshold and missing signers", func() {
		fw, conn := startMultisigFake()
		svc, _, name, from, pass, _ := newMultisigForTest(conn)
		defer keystore.ZeroPassphrase(pass)

		rawHex := multisigTestRawDataHex(from, toAddr, 100, 2)
		body, err := tron.EncodeTronJSON(rawHex, nil)
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		cosigned := filepath.Join(dir, "cosigned.json")
		Expect(os.WriteFile(cosigned, body, 0o600)).To(Succeed())

		_, err = svc.Cosign(CosignOptions{
			InPath: cosigned, OutPath: cosigned, WalletName: name, WalletPass: pass, Yes: true, Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())

		ownerBytes := mustDecodeB58(from)
		other := bytes.Repeat([]byte{0x41, 0x11}, 0)
		other = append(other, 0x41)
		other = append(other, bytes.Repeat([]byte{0x22}, 20)...)

		fw.weightResp = func(_ *core.Transaction) *api.TransactionSignWeight {
			return &api.TransactionSignWeight{
				Permission: &core.Permission{
					Id:             2,
					Threshold:      2,
					PermissionName: "active0",
					Keys: []*core.Key{
						{Address: ownerBytes, Weight: 1},
						{Address: other, Weight: 1},
					},
				},
				ApprovedList:  [][]byte{ownerBytes},
				CurrentWeight: 1,
			}
		}

		res, err := svc.Info(context.Background(), InfoOptions{InPath: cosigned})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.PermissionID).To(Equal(int32(2)))
		Expect(res.PermissionName).To(Equal("active0"))
		Expect(res.Threshold).To(Equal(int64(2)))
		Expect(res.CurrentWeight).To(Equal(int64(1)))
		Expect(res.ApprovedSigners).To(ConsistOf(from))
		Expect(res.MissingSigners).To(HaveLen(1))
	})
})
