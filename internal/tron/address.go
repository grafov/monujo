package tron

import (
	"fmt"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

// ValidateBase58Address returns nil if s is a valid Tron base58 address.
func ValidateBase58Address(s string) error {
	if _, err := address.Base58ToAddress(s); err != nil {
		return fmt.Errorf("invalid Tron address: %w", err)
	}
	return nil
}
