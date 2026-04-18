package keystore

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestStoreLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)
	addr, err := Store(dir, "t1", hexKey, pass, []string{"mainnet"})
	if err != nil {
		t.Fatal(err)
	}
	pk, err := LoadPrivateKey(dir, "t1", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()
	if pk.Address != addr {
		t.Fatalf("address mismatch %s vs %s", pk.Address, addr)
	}
}

func TestSafeName(t *testing.T) {
	if safeName("a b") != "a_b" {
		t.Fatal(safeName("a b"))
	}
}

func TestGenerateAndStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)
	addr, privHex, err := GenerateAndStore(dir, "gen1", pass, []string{"shasta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(privHex) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(privHex))
	}
	pk, err := LoadPrivateKey(dir, "gen1", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()
	if pk.Address != addr {
		t.Fatalf("address mismatch %s vs %s", pk.Address, addr)
	}
	gotHex := hex.EncodeToString(crypto.FromECDSA(pk.Key))
	if gotHex != privHex {
		t.Fatal("exported hex does not match loaded key")
	}
}

func TestListWalletsEmptyOrMissing(t *testing.T) {
	dir := t.TempDir()
	got, err := ListWallets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	missing := filepath.Join(dir, "nope")
	got, err = ListWallets(missing)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("missing dir: expected empty, got %v", got)
	}
}

func TestListWalletsAfterStore(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)
	addr, err := Store(dir, "alpha", hexKey, pass, []string{"mainnet"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Store(dir, "beta", hexKey, pass, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ListWallets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 wallets, got %d", len(list))
	}
	if list[0].Name != "alpha" || list[0].Address != addr {
		t.Fatalf("first: %+v addr %s", list[0], addr)
	}
	if list[1].Name != "beta" {
		t.Fatalf("second name: %s", list[1].Name)
	}
	_ = os.WriteFile(filepath.Join(dir, "junk.txt"), []byte("x"), 0o600)
	list2, err := ListWallets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list2) != 2 {
		t.Fatalf("junk should be ignored, got %d entries", len(list2))
	}
}

func TestNetworkOperationNeedsConfirmation(t *testing.T) {
	if NetworkOperationNeedsConfirmation(nil, "mainnet") {
		t.Fatal("nil recorded should not need confirm")
	}
	if NetworkOperationNeedsConfirmation([]string{}, "mainnet") {
		t.Fatal("empty recorded should not need confirm")
	}
	if !NetworkOperationNeedsConfirmation([]string{"shasta"}, "mainnet") {
		t.Fatal("different network should need confirm")
	}
	if NetworkOperationNeedsConfirmation([]string{"mainnet", "shasta"}, "mainnet") {
		t.Fatal("matching network should not need confirm")
	}
}

func TestReadWalletNetworksEnsureRecorded(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)
	if _, err := Store(dir, "w1", hexKey, pass, []string{"mainnet"}); err != nil {
		t.Fatal(err)
	}
	nets, err := ReadWalletNetworks(dir, "w1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0] != "mainnet" {
		t.Fatalf("networks: %v", nets)
	}
	if err := EnsureNetworkRecorded(dir, "w1", "shasta"); err != nil {
		t.Fatal(err)
	}
	nets, err = ReadWalletNetworks(dir, "w1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 2 || nets[0] != "mainnet" || nets[1] != "shasta" {
		t.Fatalf("after ensure: %v", nets)
	}
	if err := EnsureNetworkRecorded(dir, "w1", "shasta"); err != nil {
		t.Fatal(err)
	}
	pk, err := LoadPrivateKey(dir, "w1", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()
}

func TestLegacyFileWithoutNetworksStillLoads(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer ZeroPassphrase(pass)
	if _, err := Store(dir, "legacy", hexKey, pass, nil); err != nil {
		t.Fatal(err)
	}
	nets, err := ReadWalletNetworks(dir, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 0 {
		t.Fatalf("expected no networks, got %v", nets)
	}
	pk, err := LoadPrivateKey(dir, "legacy", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()
	if err := EnsureNetworkRecorded(dir, "legacy", "mainnet"); err != nil {
		t.Fatal(err)
	}
	nets, err = ReadWalletNetworks(dir, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0] != "mainnet" {
		t.Fatalf("expected mainnet: %v", nets)
	}
}
