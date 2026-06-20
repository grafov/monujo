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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
)

func transactionToHTTPJSON(tx *core.Transaction, visible bool) (map[string]any, error) {
	if tx == nil || tx.GetRawData() == nil {
		return nil, errors.New("nil transaction or raw_data")
	}

	raw := tx.GetRawData()

	rawBytes, err := proto.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal raw_data: %w", err)
	}

	h := sha256.Sum256(rawBytes)
	txID := hex.EncodeToString(h[:])

	rawDataHex, err := transaction.ToRawDataHex(tx)
	if err != nil {
		return nil, fmt.Errorf("get raw_data hex: %w", err)
	}

	contractsJSON := make([]map[string]any, 0, len(raw.GetContract()))
	for _, c := range raw.GetContract() {
		if c == nil {
			continue
		}

		var valMap map[string]any
		var typeStr string

		switch c.GetType() {
		case core.Transaction_Contract_TransferContract:
			typeStr = "TransferContract"
			var tc core.TransferContract
			if err := proto.Unmarshal(c.GetParameter().GetValue(), &tc); err != nil {
				return nil, fmt.Errorf("unmarshal TransferContract: %w", err)
			}
			valMap = map[string]any{
				"owner_address": encodeAddress(tc.GetOwnerAddress(), visible),
				"to_address":    encodeAddress(tc.GetToAddress(), visible),
				"amount":        tc.GetAmount(),
			}

		case core.Transaction_Contract_TriggerSmartContract:
			typeStr = "TriggerSmartContract"
			var ts core.TriggerSmartContract
			if err := proto.Unmarshal(c.GetParameter().GetValue(), &ts); err != nil {
				return nil, fmt.Errorf("unmarshal TriggerSmartContract: %w", err)
			}
			valMap = map[string]any{
				"owner_address":    encodeAddress(ts.GetOwnerAddress(), visible),
				"contract_address": encodeAddress(ts.GetContractAddress(), visible),
				"data":             hex.EncodeToString(ts.GetData()),
			}
			if ts.GetCallValue() != 0 {
				valMap["call_value"] = ts.GetCallValue()
			}

		default:
			return nil, fmt.Errorf("unsupported contract type %s for REST sign weight (use --write-transport grpc)", c.GetType().String())
		}

		paramJSON := map[string]any{
			"value":    valMap,
			"type_url": c.GetParameter().GetTypeUrl(),
		}

		contractJSON := map[string]any{
			"type":      typeStr,
			"parameter": paramJSON,
		}

		if c.GetPermissionId() != 0 {
			contractJSON["Permission_id"] = c.GetPermissionId()
		}

		contractsJSON = append(contractsJSON, contractJSON)
	}

	rawDataJSON := map[string]any{
		"contract":        contractsJSON,
		"ref_block_bytes": hex.EncodeToString(raw.GetRefBlockBytes()),
		"ref_block_hash":  hex.EncodeToString(raw.GetRefBlockHash()),
		"expiration":      raw.GetExpiration(),
		"timestamp":       raw.GetTimestamp(),
	}

	if raw.GetFeeLimit() != 0 {
		rawDataJSON["fee_limit"] = raw.GetFeeLimit()
	}

	if len(raw.GetData()) > 0 {
		rawDataJSON["data"] = hex.EncodeToString(raw.GetData())
	}

	sigs := make([]string, 0, len(tx.GetSignature()))
	for _, sig := range tx.GetSignature() {
		sigs = append(sigs, hex.EncodeToString(sig))
	}

	return map[string]any{
		"visible":      visible,
		"txID":         txID,
		"raw_data":     rawDataJSON,
		"raw_data_hex": rawDataHex,
		"signature":    sigs,
	}, nil
}

func encodeAddress(addr []byte, visible bool) string {
	if visible {
		return common.EncodeCheck(addr)
	}
	return hex.EncodeToString(addr)
}
