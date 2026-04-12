package app

import (
	"context"
	"testing"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
)

func TestHotService_CreateWalletThenLoad(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Build(config.NetworkShasta, config.OutputTable, "", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewHotService(cfg)
	pass := []byte("test-passphrase-32-chars-long!!")
	defer keystore.ZeroPassphrase(pass)
	if err := svc.CreateWallet("w1", pass, false, false); err != nil {
		t.Fatal(err)
	}
	pk, err := keystore.LoadPrivateKey(dir, "w1", pass)
	if err != nil {
		t.Fatal(err)
	}
	defer pk.Wipe()
	if pk.Address == "" {
		t.Fatal("empty address")
	}
}

func TestHotService_ActivateAccountDelegatesToSendTRX(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewHotService(cfg)
	err = svc.ActivateAccount(context.Background(), "missing", "TInvalid", "1", []byte("x"))
	if err == nil {
		t.Fatal("expected error for missing wallet / invalid address")
	}
}
