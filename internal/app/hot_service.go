package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
)

// HotService performs online signing operations with encrypted hot wallets.
type HotService struct {
	cfg *config.Config
	ks  string
}

// NewHotService creates a hot wallet service using cfg.KeystoreDir.
func NewHotService(cfg *config.Config) *HotService {
	return &HotService{cfg: cfg, ks: cfg.KeystoreDir}
}

// SendResult is returned after broadcast.
type SendResult struct {
	TxID    string `json:"tx_id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`
	Amount  string `json:"amount"`
	Network string `json:"network"`
}

func (h *HotService) printSend(out SendResult) error {
	if h.cfg.Output == config.OutputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Printf("Broadcast OK\nNetwork: %s\nTxID:    %s\nFrom:    %s\nTo:      %s\n%s: %s\n",
		out.Network, out.TxID, out.From, out.To, out.Kind, out.Amount)
	return nil
}

// ImportWallet encrypts a hex private key into the keystore under name.
func (h *HotService) ImportWallet(name, privHex string, pass []byte) error {
	addr, err := keystore.Store(h.ks, name, privHex, pass)
	if err != nil {
		return err
	}
	if h.cfg.Output == config.OutputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"name":    name,
			"address": addr,
			"path":    h.ks,
		})
	}
	fmt.Printf("Imported wallet %q -> %s\nKeystore: %s\n", name, addr, h.ks)
	return nil
}

// CreateWallet generates a new private key locally, encrypts it into the keystore, and prints the address.
// If exportPriv is true, prints the raw hex private key once (high risk); includePrivInJSON adds the key to JSON output.
func (h *HotService) CreateWallet(name string, pass []byte, exportPriv, includePrivInJSON bool) error {
	addr, privHex, err := keystore.GenerateAndStore(h.ks, name, pass)
	if err != nil {
		return err
	}
	if h.cfg.Output == config.OutputJSON {
		m := map[string]string{
			"name":    name,
			"address": addr,
			"path":    h.ks,
		}
		if includePrivInJSON {
			m["private_key_hex"] = privHex
		}
		return json.NewEncoder(os.Stdout).Encode(m)
	}
	fmt.Printf("Created wallet %q -> %s\nKeystore: %s\n", name, addr, h.ks)
	if exportPriv {
		fmt.Fprintf(os.Stderr, "WARNING: printing private key is unsafe; anyone with this hex can spend funds.\n")
		fmt.Printf("Private key (hex): %s\n", privHex)
	}
	return nil
}

// ListWallets prints metadata for each encrypted wallet file in the keystore (no decryption).
func (h *HotService) ListWallets() error {
	items, err := keystore.ListWallets(h.ks)
	if err != nil {
		return err
	}
	if h.cfg.Output == config.OutputJSON {
		out := struct {
			KeystoreDir string                    `json:"keystore_dir"`
			Wallets     []keystore.WalletListItem `json:"wallets"`
		}{KeystoreDir: h.ks, Wallets: items}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Printf("Keystore: %s\n", h.ks)
	if len(items) == 0 {
		fmt.Println("No wallets.")
		return nil
	}
	fmt.Printf("%s\t%s\n", "NAME", "ADDRESS")
	for _, w := range items {
		fmt.Printf("%s\t%s\n", w.Name, w.Address)
	}
	return nil
}

// ActivateAccount sends the first TRX to a new address so the account exists on-chain (same as send-trx).
func (h *HotService) ActivateAccount(ctx context.Context, fromName, to string, amountTRX string, pass []byte) error {
	return h.SendTRX(ctx, fromName, to, amountTRX, pass)
}

// PrintBalance shows TRX/USDT for a named hot wallet (address from key).
func (h *HotService) PrintBalance(ctx context.Context, name string, pass []byte) error {
	pk, err := keystore.LoadPrivateKey(h.ks, name, pass)
	if err != nil {
		return err
	}
	defer pk.Wipe()
	w := NewWatchService(h.cfg)
	return w.PrintBalance(ctx, pk.Address)
}

// SendTRX sends TRX from named wallet.
func (h *HotService) SendTRX(ctx context.Context, name, to string, amountTRX string, pass []byte) error {
	amountSun, err := domain.ParseTRXDecimal(amountTRX)
	if err != nil {
		return err
	}
	if err := tron.ValidateBase58Address(to); err != nil {
		return err
	}
	pk, err := keystore.LoadPrivateKey(h.ks, name, pass)
	if err != nil {
		return err
	}
	defer pk.Wipe()
	wc, err := tron.NewWriteClientFromConfig(h.cfg)
	if err != nil {
		return err
	}
	defer wc.Close()
	txid, err := wc.SendTRX(ctx, pk.Address, to, amountSun, pk.Key)
	if err != nil {
		return err
	}
	return h.printSend(SendResult{
		TxID:    txid,
		From:    pk.Address,
		To:      to,
		Kind:    "TRX",
		Amount:  domain.FormatSunAsTRX(amountSun),
		Network: string(h.cfg.Network),
	})
}

// SendUSDT sends USDT from named wallet.
func (h *HotService) SendUSDT(ctx context.Context, name, to, amount string, pass []byte) error {
	amt, err := domain.ParseUSDTDecimal(amount)
	if err != nil {
		return err
	}
	if err := tron.ValidateBase58Address(to); err != nil {
		return err
	}
	pk, err := keystore.LoadPrivateKey(h.ks, name, pass)
	if err != nil {
		return err
	}
	defer pk.Wipe()
	if amt.Sign() <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	wc, err := tron.NewWriteClientFromConfig(h.cfg)
	if err != nil {
		return err
	}
	defer wc.Close()
	txid, err := wc.SendUSDT(ctx, pk.Address, to, amt, pk.Key)
	if err != nil {
		return err
	}
	return h.printSend(SendResult{
		TxID:    txid,
		From:    pk.Address,
		To:      to,
		Kind:    "USDT",
		Amount:  amount + " USDT",
		Network: string(h.cfg.Network),
	})
}
