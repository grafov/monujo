package cli

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"golang.org/x/term"
)

func TestConfirmWalletNetworkForOperation_matchingNetwork(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer keystore.ZeroPassphrase(pass)
	if _, err := keystore.Store(dir, "w", hexKey, pass, []string{"shasta"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Build(config.NetworkShasta, config.OutputTable, "", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := confirmWalletNetworkForOperation(cfg, "w", false); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmWalletNetworkForOperation_forceSkipsMismatch(t *testing.T) {
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer keystore.ZeroPassphrase(pass)
	if _, err := keystore.Store(dir, "w", hexKey, pass, []string{"shasta"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := confirmWalletNetworkForOperation(cfg, "w", true); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmWalletNetworkForOperation_mismatchWithoutForceNonTTY(t *testing.T) {
	if term.IsTerminal(int(os.Stdin.Fd())) { //nolint:gosec // G115
		t.Skip("stdin is a terminal; would require interactive input")
	}
	dir := t.TempDir()
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(priv))
	pass := []byte("test-passphrase-32-chars-long!!")
	defer keystore.ZeroPassphrase(pass)
	if _, err := keystore.Store(dir, "w", hexKey, pass, []string{"shasta"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Build(config.NetworkMainnet, config.OutputTable, "", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	err = confirmWalletNetworkForOperation(cfg, "w", false)
	if err == nil {
		t.Fatal("expected error when network mismatches and not a TTY")
	}
}
