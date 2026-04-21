package cli

import (
	"fmt"
	"strings"

	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/tron"
)

// looksLikeTronAddress reports whether s passes Tron base58 address
// validation. The CLI uses this to disambiguate the unified --from flag
// between a wallet name and a raw base58 address: anything that parses
// as an address is treated as one, anything else as a wallet name.
func looksLikeTronAddress(s string) bool {
	return tron.ValidateBase58Address(strings.TrimSpace(s)) == nil
}

// resolveWalletFrom interprets the shared --from flag value (either a
// keystore wallet name or a Tron base58 address) and returns both the
// wallet name and the base58 address it refers to.
//
// Behavior:
//   - Empty input yields an error.
//   - Input that parses as a base58 address is looked up by Address in the
//     keystore. On hit, returns (walletName, inputAddress, true, nil); on
//     miss, returns ("", inputAddress, false, nil) so callers that do not
//     need a keystore file (e.g. `hot balance`, `offline build`) can still
//     proceed with just the address.
//   - Anything else is treated as a wallet name and looked up by Name in
//     the keystore. On hit, returns (name, walletAddress, true, nil); on
//     miss, returns ("", "", false, error) because a name that is not on
//     disk is almost certainly a user typo rather than an intentional
//     bypass.
//
// The keystore is accessed via keystore.ListWallets, which reads only the
// plaintext metadata of the encrypted wallet files and never attempts to
// decrypt the private keys.
func resolveWalletFrom(keystoreDir, from string) (name, address string, ok bool, err error) {
	in := strings.TrimSpace(from)
	if in == "" {
		return "", "", false, fmt.Errorf("--from must be provided (Tron base58 address or keystore wallet name)")
	}
	if looksLikeTronAddress(in) {
		items, listErr := keystore.ListWallets(keystoreDir)
		if listErr != nil {
			return "", "", false, fmt.Errorf("resolve --from address: %w", listErr)
		}
		for _, w := range items {
			if w.Address == in {
				return w.Name, in, true, nil
			}
		}
		return "", in, false, nil
	}
	items, listErr := keystore.ListWallets(keystoreDir)
	if listErr != nil {
		return "", "", false, fmt.Errorf("resolve --from wallet name: %w", listErr)
	}
	for _, w := range items {
		if w.Name == in {
			return in, w.Address, true, nil
		}
	}
	return "", "", false, fmt.Errorf("wallet %q not found in keystore %q", in, keystoreDir)
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
