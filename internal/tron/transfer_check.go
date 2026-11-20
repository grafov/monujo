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
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/transfer"
)

// TransferSpec is the transfer the caller asked a node to build.
// Amount is sun for [transfer.KindTRX] and token minimal units for
// [transfer.KindTRC20USDT]. FeeLimitSun is enforced only for TRC-20:
// a native TRX transfer has no contract fee limit.
type TransferSpec struct {
	Network     config.Network
	Kind        string
	From        string
	To          string
	Amount      *big.Int
	FeeLimitSun int64
}

// PlainTransferIssues lists every way s differs from a plain monujo
// transfer on net. A plain transfer has no attached TRX (call_value),
// no attached TRC-10 (call_token_value or token_id), no transaction
// memo, and — when it is a TRC-20 call — the canonical USDT contract
// for net. PermissionID is intentionally not an issue: a multisig
// transaction legitimately targets a non-zero active permission.
// An empty slice means s is a plain transfer.
func (s RawDataSummary) PlainTransferIssues(net config.Network) []string {
	var issues []string
	if s.CallValueSun != 0 {
		issues = append(issues, fmt.Sprintf("call_value is %d sun", s.CallValueSun))
	}
	if s.CallTokenValue != 0 {
		issues = append(issues, fmt.Sprintf("call_token_value is %d", s.CallTokenValue))
	}
	if s.TokenID != 0 {
		issues = append(issues, fmt.Sprintf("token_id is %d", s.TokenID))
	}
	if len(s.Memo) > 0 {
		issues = append(issues, fmt.Sprintf("transaction memo is set (%d bytes)", len(s.Memo)))
	}
	if s.Kind == transfer.KindTRC20USDT {
		want := USDTContractForNetwork(net)
		if s.Contract != want {
			issues = append(issues, fmt.Sprintf("contract %s is not the canonical USDT contract %s", s.Contract, want))
		}
	}
	return issues
}

// CheckPlainTransfer returns an error when s is not a plain transfer on
// net. The error text joins every issue from [RawDataSummary.PlainTransferIssues]
// so the signer can refuse before showing a confirmation or asking for a
// passphrase. A nil error means every attached-value field is clear and,
// for a TRC-20 transfer, the contract is the canonical USDT token.
func (s RawDataSummary) CheckPlainTransfer(net config.Network) error {
	issues := s.PlainTransferIssues(net)
	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf("plain transfer rejected: %s", strings.Join(issues, "; "))
}

// VerifyBuiltTransfer decodes the transaction a node returned and refuses
// it unless it is exactly the plain transfer described by spec. Callers
// invoke it before signing or sealing raw_data, so the node cannot swap
// the recipient, amount, contract or fee limit, and cannot attach hidden
// TRX, TRC-10 or a memo.
//
// The check order is: the transaction exists and has one contract; it is
// a plain transfer on spec.Network; permission_id is 0; kind, from, to
// and amount match spec; a TRC-20 transfer also matches the canonical
// USDT contract and spec.FeeLimitSun.
func VerifyBuiltTransfer(tx *core.Transaction, spec TransferSpec) error {
	if tx == nil || tx.GetRawData() == nil {
		return errors.New("node returned a transaction that does not match the request: empty transaction")
	}
	raw := tx.GetRawData()
	if n := len(raw.GetContract()); n != 1 {
		return fmt.Errorf("node returned a transaction that does not match the request: contracts=%d want 1", n)
	}
	summary, err := summarizeRaw(raw)
	if err != nil {
		return fmt.Errorf("node returned a transaction that does not match the request: %w", err)
	}
	if err := summary.CheckPlainTransfer(spec.Network); err != nil {
		return fmt.Errorf("node returned a transaction that does not match the request: %w", err)
	}
	if summary.PermissionID != 0 {
		return fmt.Errorf("node returned a transaction that does not match the request: permission_id=%d want 0", summary.PermissionID)
	}
	if summary.Kind != spec.Kind {
		return fmt.Errorf("node returned a transaction that does not match the request: kind=%s want %s", summary.Kind, spec.Kind)
	}
	if summary.From != spec.From {
		return fmt.Errorf("node returned a transaction that does not match the request: from=%s want %s", summary.From, spec.From)
	}
	if summary.To != spec.To {
		return fmt.Errorf("node returned a transaction that does not match the request: to=%s want %s", summary.To, spec.To)
	}
	if spec.Amount == nil || summary.AmountWei == nil || summary.AmountWei.Cmp(spec.Amount) != 0 {
		return fmt.Errorf("node returned a transaction that does not match the request: amount=%s want %s", amountText(summary.AmountWei), amountText(spec.Amount))
	}
	if spec.Kind == transfer.KindTRC20USDT {
		want := USDTContractForNetwork(spec.Network)
		if summary.Contract != want {
			return fmt.Errorf("node returned a transaction that does not match the request: contract=%s want %s", summary.Contract, want)
		}
		if summary.FeeLimitSun != spec.FeeLimitSun {
			return fmt.Errorf("node returned a transaction that does not match the request: fee_limit=%d want %d", summary.FeeLimitSun, spec.FeeLimitSun)
		}
	}
	return nil
}

func amountText(v *big.Int) string {
	if v == nil {
		return "<nil>"
	}
	return v.String()
}
