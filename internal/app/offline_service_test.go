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
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/transfer"
	"github.com/grafov/monujo/internal/tron"
	"github.com/grafov/monujo/internal/tron/trontest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// fakeWallet is a minimal in-process api.WalletServer used by the e2e
// tests. It accepts CreateTransaction2 / TriggerContract / Broadcast and
// records every broadcast it sees so the test can assert on it.
type fakeWallet struct {
	api.UnimplementedWalletServer

	mu             sync.Mutex
	createCalls    int
	triggerCalls   int
	broadcastCalls []*core.Transaction
	failBroadcast  bool
}

func (f *fakeWallet) CreateTransaction2(_ context.Context, in *core.TransferContract) (*api.TransactionExtention, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.createCalls++
	raw := &core.TransactionRaw{
		RefBlockBytes: []byte{0xab, 0xcd},
		Expiration:    time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:     time.Now().UnixMilli(),
		Contract: []*core.Transaction_Contract{
			{
				Type:      core.Transaction_Contract_TransferContract,
				Parameter: mustAny(&core.TransferContract{OwnerAddress: in.GetOwnerAddress(), ToAddress: in.GetToAddress(), Amount: in.GetAmount()}),
			},
		},
	}
	return &api.TransactionExtention{
		Transaction: &core.Transaction{RawData: raw},
		Result:      &api.Return{Result: true, Code: api.Return_SUCCESS},
	}, nil
}

func (f *fakeWallet) BroadcastTransaction(_ context.Context, in *core.Transaction) (*api.Return, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.broadcastCalls = append(f.broadcastCalls, in)
	if f.failBroadcast {
		return &api.Return{Result: false, Code: api.Return_CONTRACT_VALIDATE_ERROR, Message: []byte("rejected")}, nil
	}
	return &api.Return{Result: true, Code: api.Return_SUCCESS}, nil
}

func mustAny(msg proto.Message) *anypb.Any {
	b, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return &anypb.Any{TypeUrl: "type.googleapis.com/protocol." + string(proto.MessageName(msg).Name()), Value: b}
}

func startFakeServer() (*fakeWallet, *grpc.ClientConn) {
	GinkgoHelper()

	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	fw := &fakeWallet{}
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

// newOfflineForTest builds an OfflineService wired to a bufconn fake and
// to a temp keystore directory. The returned wallet name is "cold" with
// passphrase "pass1234"; its address replaces the from-address in build
// calls so that sign verification succeeds.
func newOfflineForTest(fw *fakeWallet, conn *grpc.ClientConn) (svc *OfflineService, ksDir, walletName, walletAddr string, walletPass []byte) {
	GinkgoHelper()

	ksDir = GinkgoT().TempDir()
	cfg, err := config.Build(config.NetworkMainnet, config.OutputJSON, "", "bufconn", "", ksDir)
	Expect(err).NotTo(HaveOccurred())

	walletName = "cold"
	walletPass = []byte("pass1234abcd!!!")
	addr, _, err := keystore.GenerateAndStore(ksDir, walletName, walletPass, []string{string(cfg.Network)})
	Expect(err).NotTo(HaveOccurred())
	walletAddr = addr

	svc = NewOfflineService(cfg)
	svc.onlineWriter = func() (tron.TransactionWriter, func(), error) {
		return trontest.NewWriteClientWithConn(cfg, conn), func() {}, nil
	}
	svc.confirmIn = strings.NewReader("y\n")
	svc.confirmOut = io.Discard
	svc.stdoutOverride = io.Discard
	tt := true
	svc.terminalOverride = &tt
	_ = fw
	return svc, ksDir, walletName, walletAddr, walletPass
}

var _ = Describe("OfflineService", func() {
	It("builds, signs and broadcasts TRX transfer", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		signed := filepath.Join(dir, "s.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.5",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    signed,
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).NotTo(HaveOccurred())

		res, err := svc.Broadcast(context.Background(), BroadcastOptions{
			InPath: signed,
			Yes:    true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.TxID).To(HavePrefix("0x"))
		Expect(res.TxID).NotTo(BeEmpty())
		Expect(fw.broadcastCalls).To(HaveLen(1))
		Expect(fw.broadcastCalls[0].GetSignature()).To(HaveLen(1))
	})

	It("builds, signs and broadcasts envelope protected with HMAC", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		hmacPass := []byte("shared-hmac-passphrase")
		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		signed := filepath.Join(dir, "s.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:           transfer.KindTRX,
			FromAddress:    addr,
			To:             knownToAddr,
			AmountTRX:      "0.001",
			OutPath:        unsigned,
			HMACPassphrase: append([]byte(nil), hmacPass...),
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Sign(SignOptions{
			InPath:         unsigned,
			OutPath:        signed,
			WalletName:     name,
			WalletPass:     pass,
			HMACPassphrase: append([]byte(nil), hmacPass...),
			Yes:            true,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Broadcast(context.Background(), BroadcastOptions{
			InPath:         signed,
			HMACPassphrase: append([]byte(nil), hmacPass...),
			Yes:            true,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects tampered payload without HMAC", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.0",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		tamperPayload(unsigned, func(pl *transfer.UnsignedTransferPayload) {
			pl.To = otherToAddr
		})

		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    filepath.Join(dir, "s.json"),
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("to mismatch"))
	})

	It("rejects fee_limit mismatch for USDT payloads", func() {
		summary := tron.RawDataSummary{
			Kind:         transfer.KindTRC20USDT,
			From:         knownToAddr,
			To:           otherToAddr,
			AmountStr:    "1 USDT",
			AmountWei:    big.NewInt(1000000),
			Contract:     tron.MainnetUSDTContract,
			FeeLimitSun:  100_000_000,
			ExpirationMs: 1,
		}
		pl := transfer.UnsignedTransferPayload{
			Kind:          transfer.KindTRC20USDT,
			From:          summary.From,
			To:            summary.To,
			Amount:        summary.AmountStr,
			AmountMinimal: "1000000",
			Contract:      summary.Contract,
			FeeLimitSun:   42,
			Expiration:    summary.ExpirationMs,
		}
		err := verifyPayloadAgainstSummary(pl, summary)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fee_limit mismatch"))
	})

	It("rejects tampered signed amount before broadcast", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		signed := filepath.Join(dir, "s.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.0",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    signed,
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).NotTo(HaveOccurred())

		tamperSignedPayload(signed, func(pl *transfer.SignedTransferPayload) {
			pl.Amount = "999"
			pl.AmountMinimal = "999000000"
		})

		_, err = svc.Broadcast(context.Background(), BroadcastOptions{
			InPath: signed,
			Yes:    true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("amount mismatch"))
		Expect(fw.broadcastCalls).To(BeEmpty())
	})

	It("does not read the keystore passphrase before interactive confirmation", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		signed := filepath.Join(dir, "s.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.0",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		svc.confirmIn = strings.NewReader("n\n")
		reads := 0
		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    signed,
			WalletName: name,
			ReadWalletPass: func() ([]byte, error) {
				reads++
				return append([]byte(nil), pass...), nil
			},
			IsTerminal: true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("aborted by user"))
		Expect(reads).To(Equal(0))
	})

	It("refuses non-interactive sign without --yes before reading passphrase", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.0",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		notTTY := false
		svc.terminalOverride = &notTTY
		reads := 0
		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    filepath.Join(dir, "s.json"),
			WalletName: name,
			ReadWalletPass: func() ([]byte, error) {
				reads++
				return append([]byte(nil), pass...), nil
			},
			IsTerminal: false,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("without --yes"))
		Expect(reads).To(Equal(0))
	})

	It("rejects tampered envelope when HMAC is used", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		hmacPass := []byte("hmac-pass")
		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")

		_, err := svc.Build(context.Background(), BuildOptions{
			Kind:           transfer.KindTRX,
			FromAddress:    addr,
			To:             knownToAddr,
			AmountTRX:      "1.0",
			OutPath:        unsigned,
			HMACPassphrase: append([]byte(nil), hmacPass...),
		})
		Expect(err).NotTo(HaveOccurred())

		tamperPayload(unsigned, func(pl *transfer.UnsignedTransferPayload) {
			pl.To = otherToAddr
		})

		_, err = svc.Sign(SignOptions{
			InPath:         unsigned,
			OutPath:        filepath.Join(dir, "s.json"),
			WalletName:     name,
			WalletPass:     pass,
			HMACPassphrase: append([]byte(nil), hmacPass...),
			Yes:            true,
		})
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, transfer.ErrHMACMismatch)).To(BeTrue())
	})

	It("rejects signing when keystore wallet address does not match payload", func() {
		fw, conn := startFakeServer()
		svc, ksDir, _, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		priv, err := crypto.GenerateKey()
		Expect(err).NotTo(HaveOccurred())

		otherPass := []byte("otherpass1234")
		defer keystore.ZeroPassphrase(otherPass)
		otherHex := common.BytesToHexString(crypto.FromECDSA(priv))[2:]
		_, err = keystore.Store(ksDir, "decoy", otherHex, otherPass, []string{"mainnet"})
		Expect(err).NotTo(HaveOccurred())

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		_, err = svc.Build(context.Background(), BuildOptions{
			Kind:        transfer.KindTRX,
			FromAddress: addr,
			To:          knownToAddr,
			AmountTRX:   "1.0",
			OutPath:     unsigned,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    filepath.Join(dir, "s.json"),
			WalletName: "decoy",
			WalletPass: otherPass,
			Yes:        true,
		})
		Expect(err).To(HaveOccurred())
	})

	It("rejects signing a USDT envelope that also moves TRX", func() {
		fw, conn := startFakeServer()
		svc, _, name, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		unsigned := filepath.Join(dir, "u.json")
		writePlainMismatchEnvelope(unsigned, addr, tron.MainnetUSDTContract, 1_000_000, false)

		_, err := svc.Sign(SignOptions{
			InPath:     unsigned,
			OutPath:    filepath.Join(dir, "s.json"),
			WalletName: name,
			WalletPass: pass,
			Yes:        true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("call_value"))
	})

	It("rejects broadcasting a signed envelope aimed at a non-canonical contract", func() {
		fw, conn := startFakeServer()
		svc, _, _, addr, pass := newOfflineForTest(fw, conn)
		defer keystore.ZeroPassphrase(pass)

		dir := GinkgoT().TempDir()
		signed := filepath.Join(dir, "s.json")
		writePlainMismatchEnvelope(signed, addr, knownToAddr, 0, true)

		_, err := svc.Broadcast(context.Background(), BroadcastOptions{
			InPath: signed,
			Yes:    true,
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("canonical USDT"))
		Expect(fw.broadcastCalls).To(BeEmpty())
	})
})

// writePlainMismatchEnvelope seals a TRC-20 transfer whose display fields
// match raw_data but which is not a plain USDT transfer: callValue is
// attached TRX, or contract is not the canonical USDT token. signed
// selects the signed envelope used by broadcast.
func writePlainMismatchEnvelope(path, from, contract string, callValue int64, signed bool) {
	GinkgoHelper()

	toBytes, err := common.DecodeCheck(knownToAddr)
	Expect(err).NotTo(HaveOccurred())
	fromBytes, err := common.DecodeCheck(from)
	Expect(err).NotTo(HaveOccurred())
	contractBytes, err := common.DecodeCheck(contract)
	Expect(err).NotTo(HaveOccurred())

	data := append([]byte{0xa9, 0x05, 0x9c, 0xbb}, make([]byte, 64)...)
	copy(data[16:36], toBytes[1:])
	data[len(data)-1] = 1
	raw := &core.TransactionRaw{
		Expiration: time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:  time.Now().UnixMilli(),
		FeeLimit:   100_000_000,
		Contract: []*core.Transaction_Contract{{
			Type: core.Transaction_Contract_TriggerSmartContract,
			Parameter: mustAny(&core.TriggerSmartContract{
				OwnerAddress:    fromBytes,
				ContractAddress: contractBytes,
				Data:            data,
				CallValue:       callValue,
			}),
		}},
	}
	rawBytes, err := proto.Marshal(raw)
	Expect(err).NotTo(HaveOccurred())
	rawHex := hex.EncodeToString(rawBytes)
	summary, err := tron.DecodeRawDataSummary(rawHex)
	Expect(err).NotTo(HaveOccurred())

	if signed {
		body, err := transfer.MarshalSignedPayload(transfer.SignedTransferPayload{
			Version:       transfer.EnvelopeVersion,
			Chain:         transfer.ChainTronMainnet,
			Kind:          summary.Kind,
			From:          summary.From,
			To:            summary.To,
			Amount:        summary.AmountStr,
			AmountMinimal: summary.AmountWei.String(),
			Contract:      summary.Contract,
			FeeLimitSun:   summary.FeeLimitSun,
			Expiration:    summary.ExpirationMs,
			TxIDHex:       "0x" + strings.Repeat("ab", 32),
			RawDataHex:    rawHex,
			SignatureHex:  "0x" + strings.Repeat("11", 65),
		})
		Expect(err).NotTo(HaveOccurred())
		blob, err := transfer.SealPayload(transfer.KindSignedTransfer, body, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path, blob, 0o600)).To(Succeed())
		return
	}

	body, err := transfer.MarshalUnsignedPayload(transfer.UnsignedTransferPayload{
		Version:       transfer.EnvelopeVersion,
		Chain:         transfer.ChainTronMainnet,
		Kind:          summary.Kind,
		From:          summary.From,
		To:            summary.To,
		Amount:        summary.AmountStr,
		AmountMinimal: summary.AmountWei.String(),
		Contract:      summary.Contract,
		FeeLimitSun:   summary.FeeLimitSun,
		Expiration:    summary.ExpirationMs,
		Timestamp:     summary.TimestampMs,
		TxIDHex:       "0x" + strings.Repeat("ab", 32),
		RawDataHex:    rawHex,
	})
	Expect(err).NotTo(HaveOccurred())
	blob, err := transfer.SealPayload(transfer.KindUnsignedTransfer, body, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(path, blob, 0o600)).To(Succeed())
}

// tamperPayload reads an envelope file, decodes its payload JSON, applies
// mutate, re-encodes the payload (without recomputing HMAC) and writes the
// envelope back. This simulates a flash-drive attacker who flips bytes in
// payload_b64 without knowing the HMAC passphrase.
func tamperPayload(path string, mutate func(*transfer.UnsignedTransferPayload)) {
	GinkgoHelper()

	blob, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	var env transfer.Envelope
	Expect(json.Unmarshal(blob, &env)).To(Succeed())

	rawPayload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	Expect(err).NotTo(HaveOccurred())

	pl, err := transfer.UnmarshalUnsignedPayload(rawPayload)
	Expect(err).NotTo(HaveOccurred())

	mutate(&pl)
	body, err := transfer.MarshalUnsignedPayload(pl)
	Expect(err).NotTo(HaveOccurred())

	env.PayloadB64 = base64.StdEncoding.EncodeToString(body)
	out, err := json.MarshalIndent(env, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(path, out, 0o600)).To(Succeed())
}

func tamperSignedPayload(path string, mutate func(*transfer.SignedTransferPayload)) {
	GinkgoHelper()

	blob, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	var env transfer.Envelope
	Expect(json.Unmarshal(blob, &env)).To(Succeed())

	rawPayload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	Expect(err).NotTo(HaveOccurred())

	pl, err := transfer.UnmarshalSignedPayload(rawPayload)
	Expect(err).NotTo(HaveOccurred())

	mutate(&pl)
	body, err := transfer.MarshalSignedPayload(pl)
	Expect(err).NotTo(HaveOccurred())

	env.PayloadB64 = base64.StdEncoding.EncodeToString(body)
	out, err := json.MarshalIndent(env, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(path, out, 0o600)).To(Succeed())
}

// Well-known mainnet base58 addresses used by the fake server only.
const (
	knownToAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
	otherToAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
)
