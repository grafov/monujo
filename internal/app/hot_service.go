package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	addr, err := keystore.Store(h.ks, name, privHex, pass, []string{string(h.cfg.Network)})
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

// ImportExternalWallet stores a Tron address under name without any private key material.
func (h *HotService) ImportExternalWallet(name, tronAddress string) error {
	addr, err := keystore.StoreExternal(h.ks, name, tronAddress, []string{string(h.cfg.Network)})
	if err != nil {
		return err
	}
	if h.cfg.Output == config.OutputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"name":     name,
			"address":  addr,
			"path":     h.ks,
			"external": true,
		})
	}
	fmt.Printf("Imported external wallet %q -> %s\nKeystore: %s\n", name, addr, h.ks)
	return nil
}

// CreateWallet generates a new private key locally, encrypts it into the keystore, and prints the address.
// If exportPriv is true, prints the raw hex private key once (high risk); includePrivInJSON adds the key to JSON output.
func (h *HotService) CreateWallet(name string, pass []byte, exportPriv, includePrivInJSON bool) error {
	addr, privHex, err := keystore.GenerateAndStore(h.ks, name, pass, []string{string(h.cfg.Network)})
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
	fmt.Printf("%-20s\t%-35s\t%s\t%s\n", "NAME", "ADDRESS", "TYPE", "NETWORKS")
	for _, w := range items {
		nets := "-"
		if len(w.Networks) > 0 {
			nets = strings.Join(w.Networks, ",")
		}
		wtype := "local"
		if w.External {
			wtype = "extern"
		}
		fmt.Printf("%-20s\t%-35s\t%s\t%s\n", w.Name, w.Address, wtype, nets)
	}
	return nil
}

// PrintBalanceByName shows TRX/USDT for a named hot wallet without
// requiring the keystore passphrase. The address is read from the
// plaintext metadata of the encrypted wallet file (the same field that
// `wallet list` exposes), then forwarded to [WatchService.PrintBalance].
//
// Decrypting the key for a balance lookup would prove file integrity but
// would also force the user to type their passphrase for a read-only
// operation; that trade-off is not worth it. Any subsequent signing
// operation (e.g. SendTRX) will still verify integrity end-to-end via
// [keystore.LoadPrivateKey].
func (h *HotService) PrintBalanceByName(ctx context.Context, name string) error {
	items, err := keystore.ListWallets(h.ks)
	if err != nil {
		return err
	}
	for _, w := range items {
		if w.Name == name {
			ws := NewWatchService(h.cfg)
			return ws.PrintBalance(ctx, w.Address)
		}
	}
	return fmt.Errorf("wallet %q not found in keystore %q", name, h.ks)
}

// PrintBalanceByAddress is the keystore-free counterpart of
// [HotService.PrintBalanceByName] used when --addr is a raw Tron base58
// address rather than a keystore wallet name. It validates the address
// and delegates to [WatchService.PrintBalance].
//
// No keystore lookup is performed: balance is a read-only operation on
// the Tron network, so the caller does not need a local wallet file.
func (h *HotService) PrintBalanceByAddress(ctx context.Context, addr string) error {
	if err := tron.ValidateBase58Address(addr); err != nil {
		return err
	}
	ws := NewWatchService(h.cfg)
	return ws.PrintBalance(ctx, addr)
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
	if err := keystore.EnsureNetworkRecorded(h.ks, name, string(h.cfg.Network)); err != nil {
		return fmt.Errorf("broadcast ok (tx %s) but could not update wallet networks metadata: %w", txid, err)
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
	if err := keystore.EnsureNetworkRecorded(h.ks, name, string(h.cfg.Network)); err != nil {
		return fmt.Errorf("broadcast ok (tx %s) but could not update wallet networks metadata: %w", txid, err)
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
