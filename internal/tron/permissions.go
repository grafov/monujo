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
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
)

// PermissionType names the three kinds of permission slots a Tron account
// can have. The values match [core.Permission_PermissionType] ordinals
// (Owner=0, Witness=1, Active=2) so callers can compare directly.
type PermissionType int

// Known permission types.
const (
	PermissionTypeOwner   PermissionType = 0
	PermissionTypeWitness PermissionType = 1
	PermissionTypeActive  PermissionType = 2
)

// String returns a short lowercase name for a [PermissionType].
func (t PermissionType) String() string {
	switch t {
	case PermissionTypeOwner:
		return "owner"
	case PermissionTypeWitness:
		return "witness"
	case PermissionTypeActive:
		return "active"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

// PermissionKey is one weighted signer in a Tron permission. The address
// is always rendered in base58check form, regardless of how TronGrid
// returned it (HTTP API returns hex-41 addresses).
type PermissionKey struct {
	AddressBase58 string `json:"address"`
	Weight        int64  `json:"weight"`
}

// AccountPermission is one permission row (owner/witness/active) from a
// Tron account. Operations are kept as the raw 32-byte hex bitmap and as
// the decoded list of contract type names so higher layers can either
// display the names or diff bitmaps.
type AccountPermission struct {
	Type           PermissionType  `json:"type"`
	ID             int32           `json:"id"`
	PermissionName string          `json:"permission_name"`
	Threshold      int64           `json:"threshold"`
	ParentID       int32           `json:"parent_id,omitempty"`
	OperationsHex  string          `json:"operations_hex,omitempty"`
	Operations     []string        `json:"operations,omitempty"`
	Keys           []PermissionKey `json:"keys,omitempty"`
}

// AccountPermissions aggregates owner, witness and active permissions for
// a single address. Witness is nil for non-SR accounts. ActivePermissions
// is ordered the way TronGrid returned it (typically by id ascending).
type AccountPermissions struct {
	Address           string               `json:"address"`
	Owner             *AccountPermission   `json:"owner_permission,omitempty"`
	Witness           *AccountPermission   `json:"witness_permission,omitempty"`
	ActivePermissions []*AccountPermission `json:"active_permission,omitempty"`
}

type jsonPermissionKey struct {
	Address string `json:"address"`
	Weight  int64  `json:"weight"`
}

type jsonPermission struct {
	Type           int                 `json:"type"`
	ID             int32               `json:"id"`
	PermissionName string              `json:"permission_name"`
	Threshold      int64               `json:"threshold"`
	ParentID       int32               `json:"parent_id"`
	Operations     string              `json:"operations"`
	Keys           []jsonPermissionKey `json:"keys"`
}

type permissionsAccountResponse struct {
	Data []struct {
		Address           string            `json:"address"`
		OwnerPermission   *jsonPermission   `json:"owner_permission"`
		WitnessPermission *jsonPermission   `json:"witness_permission"`
		ActivePermissions []*jsonPermission `json:"active_permission"`
	} `json:"data"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// GetPermissions fetches the permission structure for addr via the
// TronGrid /v1/accounts HTTP endpoint. The returned object always has
// Address set; Owner/Witness/ActivePermissions are nil when the network
// has not recorded them (unfunded account or non-SR account).
func (r *ReadClient) GetPermissions(ctx context.Context, addr string) (*AccountPermissions, error) {
	if err := ValidateBase58Address(addr); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/v1/accounts/%s", r.cfg.HTTPBaseURL, url.PathEscape(addr))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	r.addAPIKey(req)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("trongrid permissions: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return parsePermissionsResponse(addr, body)
}

func parsePermissionsResponse(addr string, body []byte) (*AccountPermissions, error) {
	var ar permissionsAccountResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("decode accounts: %w", err)
	}
	if !ar.Success && len(ar.Data) == 0 {
		return nil, fmt.Errorf("trongrid: %s", ar.Error)
	}
	out := &AccountPermissions{Address: addr}
	if len(ar.Data) == 0 {
		return out, nil
	}
	row := ar.Data[0]
	if row.OwnerPermission != nil {
		p, err := convertPermission(row.OwnerPermission, PermissionTypeOwner)
		if err != nil {
			return nil, fmt.Errorf("owner_permission: %w", err)
		}
		out.Owner = p
	}
	if row.WitnessPermission != nil {
		p, err := convertPermission(row.WitnessPermission, PermissionTypeWitness)
		if err != nil {
			return nil, fmt.Errorf("witness_permission: %w", err)
		}
		out.Witness = p
	}
	for i, ap := range row.ActivePermissions {
		p, err := convertPermission(ap, PermissionTypeActive)
		if err != nil {
			return nil, fmt.Errorf("active_permission[%d]: %w", i, err)
		}
		out.ActivePermissions = append(out.ActivePermissions, p)
	}
	return out, nil
}

func convertPermission(src *jsonPermission, fallbackType PermissionType) (*AccountPermission, error) {
	out := &AccountPermission{
		ID:             src.ID,
		PermissionName: src.PermissionName,
		Threshold:      src.Threshold,
		ParentID:       src.ParentID,
		OperationsHex:  strings.ToLower(src.Operations),
	}
	if src.Type == 0 && fallbackType != PermissionTypeOwner {
		out.Type = fallbackType
	} else {
		out.Type = PermissionType(src.Type)
	}
	if src.Operations != "" {
		ops, err := DecodePermissionOperations(src.Operations)
		if err != nil {
			return nil, err
		}
		out.Operations = ops
	}
	for i, k := range src.Keys {
		b58, err := hex41ToBase58(k.Address)
		if err != nil {
			return nil, fmt.Errorf("keys[%d]: %w", i, err)
		}
		out.Keys = append(out.Keys, PermissionKey{AddressBase58: b58, Weight: k.Weight})
	}
	return out, nil
}

// hex41ToBase58 converts a 21-byte hex TRON address (with the 0x41 mainnet
// prefix) into base58check. An input that already looks like a base58
// address is returned unchanged so this function can be fed either
// representation defensively.
func hex41ToBase58(s string) (string, error) {
	clean := strings.TrimSpace(s)
	if clean == "" {
		return "", fmt.Errorf("empty address")
	}
	if strings.HasPrefix(clean, "T") && len(clean) == 34 {
		if err := ValidateBase58Address(clean); err == nil {
			return clean, nil
		}
	}
	clean = strings.TrimPrefix(clean, "0x")
	b, err := hex.DecodeString(clean)
	if err != nil {
		return "", fmt.Errorf("decode address hex: %w", err)
	}
	if len(b) != 21 {
		return "", fmt.Errorf("address length %d (want 21)", len(b))
	}
	if b[0] != 0x41 {
		return "", fmt.Errorf("address prefix 0x%02x (want 0x41)", b[0])
	}
	return common.EncodeCheck(b), nil
}

// DecodePermissionOperations expands a 32-byte hex bitmap (as stored in
// [core.Permission] `operations`) into a sorted list of contract-type
// names derived from [core.Transaction_Contract_ContractType_name]. Each
// bit corresponds to one contract type (LSB of byte 0 = type 0 =
// AccountCreateContract; LSB of byte 1 = type 8; etc.).
//
// Bits that do not map to a known contract type are rendered as
// "UnknownContract(N)" so the output still fully describes the permission.
func DecodePermissionOperations(opsHex string) ([]string, error) {
	clean := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(opsHex)), "0x")
	if clean == "" {
		return nil, nil
	}
	b, err := hex.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("decode operations: %w", err)
	}
	var names []string
	for byteIdx, byt := range b {
		if byt == 0 {
			continue
		}
		for bit := 0; bit < 8; bit++ {
			if byt&(1<<bit) == 0 {
				continue
			}
			ctype := int32(byteIdx*8 + bit)
			if name, ok := core.Transaction_Contract_ContractType_name[ctype]; ok {
				names = append(names, name)
			} else {
				names = append(names, fmt.Sprintf("UnknownContract(%d)", ctype))
			}
		}
	}
	sort.Strings(names)
	return names, nil
}
