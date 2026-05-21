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
