package keystore

import (
	"encoding/hex"
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
	addr, err := Store(dir, "t1", hexKey, pass)
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
	addr, privHex, err := GenerateAndStore(dir, "gen1", pass)
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
