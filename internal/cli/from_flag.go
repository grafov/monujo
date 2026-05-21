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

package cli

import (
	"fmt"
	"strings"

	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
)

// WalletResolution is the result of resolving a wallet name or base58 address
// against the keystore directory (plaintext metadata only).
type WalletResolution struct {
	Name            string
	Address         string
	KnownInKeystore bool
	External        bool
}

// looksLikeTronAddress reports whether s passes Tron base58 address
// validation. The CLI uses this to disambiguate sender/subject flags
// (--from or --addr) between a wallet name and a raw base58 address:
// anything that parses as an address is treated as one, anything else
// as a wallet name.
func looksLikeTronAddress(s string) bool {
	return tron.ValidateBase58Address(strings.TrimSpace(s)) == nil
}

func looksLikeInvalidTronAddress(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "T") && len(s) == 34
}

// resolveWalletFrom interprets the CLI value for --from or --addr (either
// a keystore wallet name or a Tron base58 address) and returns resolution
// details. flag is the flag name for user-facing errors (e.g. "--from", "--addr").
//
// Behavior:
//   - Empty input yields an error.
//   - Input that parses as a base58 address is looked up by Address in the
//     keystore. On hit, KnownInKeystore is true and External reflects the entry.
//     On miss, only Address is set (same as raw input) and KnownInKeystore is false.
//   - Anything else is treated as a wallet name and looked up by Name in
//     the keystore. On hit, KnownInKeystore is true. On miss, error.
//
// The keystore is accessed via keystore.ListWallets, which reads only the
// plaintext metadata of wallet files and never attempts to decrypt private keys.
func resolveWalletFrom(keystoreDir, from, flag string) (WalletResolution, error) {
	in := strings.TrimSpace(from)
	if in == "" {
		return WalletResolution{}, fmt.Errorf("%s must be provided (Tron base58 address or keystore wallet name)", flag)
	}
	if looksLikeTronAddress(in) {
		items, listErr := keystore.ListWallets(keystoreDir)
		if listErr != nil {
			return WalletResolution{}, fmt.Errorf("resolve %s address: %w", flag, listErr)
		}
		for _, w := range items {
			if w.Address == in {
				return WalletResolution{
					Name:            w.Name,
					Address:         in,
					KnownInKeystore: true,
					External:        w.External,
				}, nil
			}
		}
		return WalletResolution{Address: in}, nil
	}
	items, listErr := keystore.ListWallets(keystoreDir)
	if listErr != nil {
		return WalletResolution{}, fmt.Errorf("resolve %s wallet name: %w", flag, listErr)
	}
	for _, w := range items {
		if w.Name == in {
			return WalletResolution{
				Name:            in,
				Address:         w.Address,
				KnownInKeystore: true,
				External:        w.External,
			}, nil
		}
	}
	return WalletResolution{}, fmt.Errorf("wallet %q not found in keystore %q", in, keystoreDir)
}

// resolveRecipient interprets the --to CLI flag value as either a Tron
// base58 address or a keystore wallet name, and returns the
// corresponding address.
//
// Unlike resolveWalletFrom, a base58 address is accepted verbatim
// without a keystore check: --to is just a recipient address, so the
// receiver does not need to be a local wallet. A wallet name is
// resolved via keystore.ListWallets (plaintext address metadata, no
// decryption); a name that is not in the keystore produces an error.
func resolveRecipient(keystoreDir, value string) (string, error) {
	in := strings.TrimSpace(value)
	if in == "" {
		return "", fmt.Errorf("--to must be provided (Tron base58 address or keystore wallet name)")
	}
	if looksLikeTronAddress(in) {
		return in, nil
	}
	if looksLikeInvalidTronAddress(in) {
		return "", tron.ValidateBase58Address(in)
	}
	items, err := keystore.ListWallets(keystoreDir)
	if err != nil {
		return "", fmt.Errorf("resolve --to wallet name: %w", err)
	}
	for _, w := range items {
		if w.Name == in {
			return w.Address, nil
		}
	}
	return "", fmt.Errorf("wallet %q not found in keystore %q", in, keystoreDir)
}
