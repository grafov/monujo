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

// Package app wires configuration to domain services for CLI commands.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
)

// WatchService exposes watch-only operations.
type WatchService struct {
	cfg *config.Config
	rc  *tron.ReadClient
}

// NewWatchService constructs a watch-only service.
func NewWatchService(cfg *config.Config) *WatchService {
	return &WatchService{
		cfg: cfg,
		rc:  tron.NewReadClient(cfg),
	}
}

// BalanceOutput is printed for watch balance.
type BalanceOutput struct {
	Address     string `json:"address"`
	TRXSun      int64  `json:"trx_sun"`
	TRX         string `json:"trx"`
	USDTMinimal string `json:"usdt_minimal_units"`
	USDT        string `json:"usdt"`
}

// PrintBalance fetches and prints TRX/USDT balances for addr.
func (w *WatchService) PrintBalance(ctx context.Context, addr string) error {
	b, err := w.rc.GetBalances(ctx, addr)
	if err != nil {
		return err
	}
	usdtMin := b.USDTMinimal
	if usdtMin == "" {
		usdtMin = "0"
	}
	u := new(big.Int)
	if _, ok := u.SetString(usdtMin, 10); !ok {
		u = big.NewInt(0)
	}
	out := BalanceOutput{
		Address:     b.Address,
		TRXSun:      b.Sun,
		TRX:         domain.FormatSunAsTRX(b.Sun),
		USDTMinimal: usdtMin,
		USDT:        domain.FormatMinimalUSDT(u),
	}
	switch w.cfg.Output {
	case config.OutputJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	default:
		fmt.Printf("Address: %s\n", out.Address)
		fmt.Printf("TRX:     %s (%d sun)\n", out.TRX, out.TRXSun)
		fmt.Printf("USDT:    %s (minimal units %s)\n", out.USDT, out.USDTMinimal)
		return nil
	}
}

// TxsOutput is one row for JSON listing.
type TxsOutput struct {
	TxID         string `json:"tx_id"`
	Time         string `json:"time"`
	From         string `json:"from"`
	To           string `json:"to"`
	ValueMinimal string `json:"value_minimal"`
	Token        string `json:"token"`
	Contract     string `json:"contract"`
	Type         string `json:"type"`
}

// PermissionsOutput is the JSON shape returned by `watch permissions`.
// LocalWalletForKey[address] carries the wallet name for any permission
// key that matches a local keystore wallet (empty otherwise).
type PermissionsOutput struct {
	Address           string              `json:"address"`
	Owner             *PermissionOutput   `json:"owner_permission,omitempty"`
	Witness           *PermissionOutput   `json:"witness_permission,omitempty"`
	ActivePermissions []*PermissionOutput `json:"active_permission,omitempty"`
	LocalWalletForKey map[string]string   `json:"local_wallet_for_key,omitempty"`
}

// PermissionOutput mirrors [tron.AccountPermission] in JSON-friendly form.
type PermissionOutput struct {
	Type           string          `json:"type"`
	ID             int32           `json:"id"`
	PermissionName string          `json:"permission_name"`
	Threshold      int64           `json:"threshold"`
	OperationsHex  string          `json:"operations_hex,omitempty"`
	Operations     []string        `json:"operations,omitempty"`
	Keys           []PermissionKey `json:"keys,omitempty"`
}

// PermissionKey is one signer entry inside a [PermissionOutput]. LocalWallet
// is non-empty when the address matches a keystore entry (local encrypted
// wallet or external address-only alias).
type PermissionKey struct {
	Address        string `json:"address"`
	Weight         int64  `json:"weight"`
	LocalWallet    string `json:"local_wallet,omitempty"`
	ExternalWallet bool   `json:"external_wallet,omitempty"`
}

// PrintPermissions fetches and prints the account permission structure
// for addr, highlighting permission keys that match a local wallet so the
// user can tell which signatures are available from this machine.
func (w *WatchService) PrintPermissions(ctx context.Context, addr string) error {
	perms, err := w.rc.GetPermissions(ctx, addr)
	if err != nil {
		return err
	}
	local, err := loadLocalAddressIndex(w.cfg.KeystoreDir)
	if err != nil {
		return err
	}
	out := permissionsToOutput(perms, local)
	if w.cfg.Output == config.OutputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	printPermissionsTable(out)
	return nil
}

func permissionsToOutput(p *tron.AccountPermissions, local map[string]keystoreAddrEntry) PermissionsOutput {
	out := PermissionsOutput{Address: p.Address}
	if p.Owner != nil {
		out.Owner = permissionToOutput(p.Owner, local, &out.LocalWalletForKey)
	}
	if p.Witness != nil {
		out.Witness = permissionToOutput(p.Witness, local, &out.LocalWalletForKey)
	}
	for _, ap := range p.ActivePermissions {
		out.ActivePermissions = append(out.ActivePermissions, permissionToOutput(ap, local, &out.LocalWalletForKey))
	}
	return out
}

func permissionToOutput(p *tron.AccountPermission, local map[string]keystoreAddrEntry, idxOut *map[string]string) *PermissionOutput {
	o := &PermissionOutput{
		Type:           p.Type.String(),
		ID:             p.ID,
		PermissionName: p.PermissionName,
		Threshold:      p.Threshold,
		OperationsHex:  p.OperationsHex,
		Operations:     p.Operations,
	}
	for _, k := range p.Keys {
		out := PermissionKey{Address: k.AddressBase58, Weight: k.Weight}
		if ref, ok := local[k.AddressBase58]; ok {
			out.LocalWallet = ref.Name
			out.ExternalWallet = ref.External
			if *idxOut == nil {
				*idxOut = map[string]string{}
			}
			(*idxOut)[k.AddressBase58] = ref.Name
		}
		o.Keys = append(o.Keys, out)
	}
	return o
}

// keystoreAddrEntry maps a Tron address to a keystore logical name and whether
// the entry is external (address-only, no private key).
type keystoreAddrEntry struct {
	Name     string
	External bool
}

func loadLocalAddressIndex(keystoreDir string) (map[string]keystoreAddrEntry, error) {
	items, err := keystore.ListWallets(keystoreDir)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]keystoreAddrEntry, len(items))
	for _, w := range items {
		idx[w.Address] = keystoreAddrEntry{Name: w.Name, External: w.External}
	}
	return idx, nil
}

func printPermissionsTable(p PermissionsOutput) {
	fmt.Printf("Address: %s\n", p.Address)
	if p.Owner != nil {
		printOnePermission(p.Owner)
	}
	if p.Witness != nil {
		printOnePermission(p.Witness)
	}
	for _, ap := range p.ActivePermissions {
		printOnePermission(ap)
	}
}

func printOnePermission(p *PermissionOutput) {
	fmt.Printf("\n[%s] id=%d name=%q threshold=%d\n", p.Type, p.ID, p.PermissionName, p.Threshold)
	if len(p.Operations) > 0 {
		fmt.Printf("  operations: %s\n", strings.Join(p.Operations, ", "))
	} else if p.OperationsHex != "" {
		fmt.Printf("  operations_hex: %s\n", p.OperationsHex)
	}
	for _, k := range p.Keys {
		tag := ""
		if k.LocalWallet != "" {
			if k.ExternalWallet {
				tag = fmt.Sprintf("  [external: %s]", k.LocalWallet)
			} else {
				tag = fmt.Sprintf("  [local: %s]", k.LocalWallet)
			}
		}
		fmt.Printf("  key: %s  weight=%d%s\n", k.Address, k.Weight, tag)
	}
}

// PrintTransfers lists USDT transfers for addr.
func (w *WatchService) PrintTransfers(ctx context.Context, addr string, limit int) error {
	rows, err := w.rc.ListUSDTTransfers(ctx, addr, limit)
	if err != nil {
		return err
	}
	if w.cfg.Output == config.OutputJSON {
		out := make([]TxsOutput, 0, len(rows))
		for _, r := range rows {
			out = append(out, TxsOutput{
				TxID:         r.TxID,
				Time:         r.BlockTimestamp.Format(time.RFC3339),
				From:         r.From,
				To:           r.To,
				ValueMinimal: r.ValueMinimal,
				Token:        r.TokenSymbol,
				Contract:     r.Contract,
				Type:         r.Type,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, r := range rows {
		fmt.Printf("%s  %s  %s -> %s  %s %s  [%s]\n",
			r.BlockTimestamp.Format(time.RFC3339),
			r.TxID,
			r.From,
			r.To,
			r.ValueMinimal,
			r.TokenSymbol,
			r.Type,
		)
	}
	return nil
}
