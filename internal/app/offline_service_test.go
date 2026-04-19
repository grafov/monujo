package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
	"github.com/grafov/monujo/internal/tron/trontest"
	"github.com/grafov/monujo/internal/transfer"
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

func startFakeServer(t *testing.T) (*fakeWallet, *grpc.ClientConn) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	fw := &fakeWallet{}
	api.RegisterWalletServer(srv, fw)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return fw, conn
}

// newOfflineForTest builds an OfflineService wired to a bufconn fake and
// to a temp keystore directory. The returned wallet name is "cold" with
// passphrase "pass1234"; its address replaces the from-address in build
// calls so that sign verification succeeds.
func newOfflineForTest(t *testing.T, fw *fakeWallet, conn *grpc.ClientConn) (svc *OfflineService, ksDir, walletName, walletAddr string, walletPass []byte) {
	t.Helper()
	ksDir = t.TempDir()
	cfg, err := config.Build(config.NetworkMainnet, config.OutputJSON, "", "bufconn", "", ksDir)
	if err != nil {
		t.Fatal(err)
	}

	walletName = "cold"
	walletPass = []byte("pass1234abcd!!!")
	addr, _, err := keystore.GenerateAndStore(ksDir, walletName, walletPass, []string{string(cfg.Network)})
	if err != nil {
		t.Fatal(err)
	}
	walletAddr = addr

	svc = NewOfflineService(cfg)
	svc.onlineWriter = func() (*tron.WriteClient, func(), error) {
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

func TestOffline_BuildSignBroadcast_TRX(t *testing.T) {
	fw, conn := startFakeServer(t)
	svc, _, name, addr, pass := newOfflineForTest(t, fw, conn)
	defer keystore.ZeroPassphrase(pass)

	dir := t.TempDir()
	unsigned := filepath.Join(dir, "u.json")
	signed := filepath.Join(dir, "s.json")

	if _, err := svc.Build(context.Background(), BuildOptions{
		Kind:        transfer.KindTRX,
		FromAddress: addr,
		To:          knownToAddr,
		AmountTRX:   "1.5",
		OutPath:     unsigned,
	}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, err := svc.Sign(SignOptions{
		InPath:     unsigned,
		OutPath:    signed,
		WalletName: name,
		WalletPass: pass,
		Yes:        true,
	}); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	res, err := svc.Broadcast(context.Background(), BroadcastOptions{
		InPath: signed,
		Yes:    true,
	})
	if err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	if res.TxID == "" || !strings.HasPrefix(res.TxID, "0x") {
		t.Fatalf("bad txid: %q", res.TxID)
	}
	if len(fw.broadcastCalls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(fw.broadcastCalls))
	}
	if len(fw.broadcastCalls[0].GetSignature()) != 1 {
		t.Fatal("signed tx must carry exactly one signature")
	}
}

func TestOffline_BuildSignBroadcast_WithHMAC(t *testing.T) {
	fw, conn := startFakeServer(t)
	svc, _, name, addr, pass := newOfflineForTest(t, fw, conn)
	defer keystore.ZeroPassphrase(pass)

	hmacPass := []byte("shared-hmac-passphrase")
	dir := t.TempDir()
	unsigned := filepath.Join(dir, "u.json")
	signed := filepath.Join(dir, "s.json")

	if _, err := svc.Build(context.Background(), BuildOptions{
		Kind:           transfer.KindTRX,
		FromAddress:    addr,
		To:             knownToAddr,
		AmountTRX:      "0.001",
		OutPath:        unsigned,
		HMACPassphrase: append([]byte(nil), hmacPass...),
	}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, err := svc.Sign(SignOptions{
		InPath:         unsigned,
		OutPath:        signed,
		WalletName:     name,
		WalletPass:     pass,
		HMACPassphrase: append([]byte(nil), hmacPass...),
		Yes:            true,
	}); err != nil {
		t.Fatalf("Sign with HMAC: %v", err)
	}

	if _, err := svc.Broadcast(context.Background(), BroadcastOptions{
		InPath:         signed,
		HMACPassphrase: append([]byte(nil), hmacPass...),
		Yes:            true,
	}); err != nil {
		t.Fatalf("Broadcast with HMAC: %v", err)
	}
}

func TestOffline_Sign_RejectsTamperedPayloadWithoutHMAC(t *testing.T) {
	fw, conn := startFakeServer(t)
	svc, _, name, addr, pass := newOfflineForTest(t, fw, conn)
	defer keystore.ZeroPassphrase(pass)

	dir := t.TempDir()
	unsigned := filepath.Join(dir, "u.json")

	if _, err := svc.Build(context.Background(), BuildOptions{
		Kind:        transfer.KindTRX,
		FromAddress: addr,
		To:          knownToAddr,
		AmountTRX:   "1.0",
		OutPath:     unsigned,
	}); err != nil {
		t.Fatal(err)
	}

	tamperPayload(t, unsigned, func(pl *transfer.UnsignedTransferPayload) {
		pl.To = otherToAddr
	})

	_, err := svc.Sign(SignOptions{
		InPath:     unsigned,
		OutPath:    filepath.Join(dir, "s.json"),
		WalletName: name,
		WalletPass: pass,
		Yes:        true,
	})
	if err == nil {
		t.Fatal("expected sign to refuse tampered payload (to-mismatch)")
	}
	if !strings.Contains(err.Error(), "to mismatch") {
		t.Fatalf("expected to-mismatch error, got: %v", err)
	}
}

func TestOffline_Sign_RejectsTamperedFileWithHMAC(t *testing.T) {
	fw, conn := startFakeServer(t)
	svc, _, name, addr, pass := newOfflineForTest(t, fw, conn)
	defer keystore.ZeroPassphrase(pass)

	hmacPass := []byte("hmac-pass")
	dir := t.TempDir()
	unsigned := filepath.Join(dir, "u.json")

	if _, err := svc.Build(context.Background(), BuildOptions{
		Kind:           transfer.KindTRX,
		FromAddress:    addr,
		To:             knownToAddr,
		AmountTRX:      "1.0",
		OutPath:        unsigned,
		HMACPassphrase: append([]byte(nil), hmacPass...),
	}); err != nil {
		t.Fatal(err)
	}

	tamperPayload(t, unsigned, func(pl *transfer.UnsignedTransferPayload) {
		pl.To = otherToAddr
	})

	_, err := svc.Sign(SignOptions{
		InPath:         unsigned,
		OutPath:        filepath.Join(dir, "s.json"),
		WalletName:     name,
		WalletPass:     pass,
		HMACPassphrase: append([]byte(nil), hmacPass...),
		Yes:            true,
	})
	if err == nil {
		t.Fatal("expected sign to refuse tampered HMAC envelope")
	}
	if !errors.Is(err, transfer.ErrHMACMismatch) {
		t.Fatalf("expected ErrHMACMismatch, got: %v", err)
	}
}

func TestOffline_Sign_RejectsWrongKeystoreAddress(t *testing.T) {
	fw, conn := startFakeServer(t)
	svc, ksDir, _, addr, pass := newOfflineForTest(t, fw, conn)
	defer keystore.ZeroPassphrase(pass)

	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	otherPass := []byte("otherpass1234")
	defer keystore.ZeroPassphrase(otherPass)
	otherHex := common.BytesToHexString(crypto.FromECDSA(priv))[2:]
	if _, err := keystore.Store(ksDir, "decoy", otherHex, otherPass, []string{"mainnet"}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	unsigned := filepath.Join(dir, "u.json")
	if _, err := svc.Build(context.Background(), BuildOptions{
		Kind:        transfer.KindTRX,
		FromAddress: addr,
		To:          knownToAddr,
		AmountTRX:   "1.0",
		OutPath:     unsigned,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sign(SignOptions{
		InPath:     unsigned,
		OutPath:    filepath.Join(dir, "s.json"),
		WalletName: "decoy",
		WalletPass: otherPass,
		Yes:        true,
	}); err == nil {
		t.Fatal("expected sign to reject mismatched keystore wallet address")
	}
}

// tamperPayload reads an envelope file, decodes its payload JSON, applies
// mutate, re-encodes the payload (without recomputing HMAC) and writes the
// envelope back. This simulates a flash-drive attacker who flips bytes in
// payload_b64 without knowing the HMAC passphrase.
func tamperPayload(t *testing.T, path string, mutate func(*transfer.UnsignedTransferPayload)) {
	t.Helper()
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var env transfer.Envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatal(err)
	}
	rawPayload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := transfer.UnmarshalUnsignedPayload(rawPayload)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&pl)
	body, err := transfer.MarshalUnsignedPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	env.PayloadB64 = base64.StdEncoding.EncodeToString(body)
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Well-known mainnet base58 addresses used by the fake server only.
const (
	knownToAddr = "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM"
	otherToAddr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
)

