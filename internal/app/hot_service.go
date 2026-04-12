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
