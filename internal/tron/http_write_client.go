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
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/grafov/monujo/internal/config"
	"google.golang.org/protobuf/proto"
)

// HTTPWriteClient builds and broadcasts via Tron Wallet HTTP API (HTTPS).
type HTTPWriteClient struct {
	cfg    *config.Config
	client *http.Client
}

// NewHTTPWriteClientFromConfig returns an HTTPS write client.
func NewHTTPWriteClientFromConfig(cfg *config.Config) *HTTPWriteClient {
	return &HTTPWriteClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Close is a no-op for HTTP (no persistent connection).
func (h *HTTPWriteClient) Close() error {
	return nil
}

// SendTRX builds, signs locally, and broadcasts a TRX transfer.
func (h *HTTPWriteClient) SendTRX(ctx context.Context, fromAddr, toAddr string, amountSun int64, priv *ecdsa.PrivateKey) (string, error) {
	tx, err := h.CreateTRXTransfer(ctx, fromAddr, toAddr, amountSun)
	if err != nil {
		return "", err
	}
	return h.signAndBroadcast(ctx, tx, priv)
}

// SendUSDT builds, signs locally, and broadcasts a USDT transfer.
func (h *HTTPWriteClient) SendUSDT(ctx context.Context, fromAddr, toAddr string, amount *big.Int, priv *ecdsa.PrivateKey) (string, error) {
	contract := USDTContractForNetwork(h.cfg.Network)
	tx, err := h.CreateTRC20Transfer(ctx, fromAddr, toAddr, contract, amount)
	if err != nil {
		return "", err
	}
	return h.signAndBroadcast(ctx, tx, priv)
}

// CreateTRXTransfer calls POST /wallet/createtransaction.
func (h *HTTPWriteClient) CreateTRXTransfer(ctx context.Context, from, to string, amountSun int64) (*core.Transaction, error) {
	body := map[string]any{
		"owner_address": from,
		"to_address":    to,
		"amount":        amountSun,
		"visible":       true,
	}
	respBody, err := h.postWallet(ctx, "/wallet/createtransaction", body)
	if err != nil {
		return nil, err
	}
	return parseUnsignedTransaction(respBody)
}

// CreateTRC20Transfer calls POST /wallet/triggersmartcontract.
func (h *HTTPWriteClient) CreateTRC20Transfer(ctx context.Context, from, to, contract string, amount *big.Int) (*core.Transaction, error) {
	toAddr, err := address.Base58ToAddress(to)
	if err != nil {
		return nil, err
	}
	ab := common.LeftPadBytes(amount.Bytes(), 32)
	toAddrBytes := toAddr.Bytes()
	if len(toAddrBytes) == 21 {
		toAddrBytes = toAddrBytes[1:]
	}
	toAddrPadded := common.LeftPadBytes(toAddrBytes, 32)
	dataHex := trc20TransferMethodSignature + common.Bytes2Hex(toAddrPadded) + common.Bytes2Hex(ab)
	body := map[string]any{
		"owner_address":    from,
		"contract_address": contract,
		"data":             strings.TrimPrefix(dataHex, "0x"),
		"fee_limit":        h.cfg.DefaultFeeLimitSun,
		"visible":          true,
	}
	respBody, err := h.postWallet(ctx, "/wallet/triggersmartcontract", body)
	if err != nil {
		return nil, err
	}
	return parseTriggerSmartContractTransaction(respBody)
}

// BroadcastTransaction calls POST /wallet/broadcasthex with a signed tx.
func (h *HTTPWriteClient) BroadcastTransaction(ctx context.Context, tx *core.Transaction) error {
	if tx == nil {
		return fmt.Errorf("empty transaction")
	}
	b, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("marshal transaction: %w", err)
	}
	body := map[string]string{
		"transaction": hex.EncodeToString(b),
	}
	respBody, err := h.postWallet(ctx, "/wallet/broadcasthex", body)
	if err != nil {
		return err
	}
	return parseBroadcastResult(respBody)
}

// GetTransactionSignWeight calls POST /wallet/getsignweight.
func (h *HTTPWriteClient) GetTransactionSignWeight(ctx context.Context, tx *core.Transaction) (*api.TransactionSignWeight, error) {
	if tx == nil {
		return nil, fmt.Errorf("empty transaction")
	}
	payload, err := transactionToSignWeightJSON(tx)
	if err != nil {
		return nil, err
	}
	respBody, err := h.postWallet(ctx, "/wallet/getsignweight", payload)
	if err != nil {
		return nil, err
	}
	return parseSignWeightResponse(respBody)
}

func (h *HTTPWriteClient) signAndBroadcast(ctx context.Context, tx *core.Transaction, priv *ecdsa.PrivateKey) (string, error) {
	signed, err := transaction.SignTransactionECDSA(tx, priv)
	if err != nil {
		return "", err
	}
	txid := transactionIDHex(signed)
	if err := h.BroadcastTransaction(ctx, signed); err != nil {
		return "", err
	}
	return txid, nil
}

func (h *HTTPWriteClient) postWallet(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := h.cfg.HTTPBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.TronGridAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", h.cfg.TronGridAPIKey)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, truncate(string(respBody)))
	}
	if errMsg := parseHTTPErrorField(respBody); errMsg != "" {
		return nil, fmt.Errorf("%s: %s", path, errMsg)
	}
	return respBody, nil
}

func parseHTTPErrorField(body []byte) string {
	var er struct {
		Error string `json:"Error"`
	}
	if err := json.Unmarshal(body, &er); err == nil && er.Error != "" {
		return er.Error
	}
	return ""
}

func parseUnsignedTransaction(body []byte) (*core.Transaction, error) {
	rawHex, err := extractRawDataHex(body)
	if err != nil {
		return nil, err
	}
	return transactionFromRawDataHex(rawHex)
}

func parseTriggerSmartContractTransaction(body []byte) (*core.Transaction, error) {
	var wrap struct {
		Result struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"result"`
		Transaction json.RawMessage `json:"transaction"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("parse trigger response: %w", err)
	}
	if wrap.Result.Code != 0 {
		msg := wrap.Result.Message
		if msg == "" {
			msg = fmt.Sprintf("error code %d", wrap.Result.Code)
		}
		return nil, fmt.Errorf("trigger contract: %s", msg)
	}
	if len(wrap.Transaction) == 0 {
		return parseUnsignedTransaction(body)
	}
	rawHex, err := extractRawDataHex(wrap.Transaction)
	if err != nil {
		return nil, err
	}
	return transactionFromRawDataHex(rawHex)
}

func extractRawDataHex(body []byte) (string, error) {
	var top struct {
		RawDataHex  string `json:"raw_data_hex"`
		Transaction *struct {
			RawDataHex string `json:"raw_data_hex"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &top); err != nil {
		return "", fmt.Errorf("parse transaction json: %w", err)
	}
	if top.RawDataHex != "" {
		return top.RawDataHex, nil
	}
	if top.Transaction != nil && top.Transaction.RawDataHex != "" {
		return top.Transaction.RawDataHex, nil
	}
	return "", fmt.Errorf("response missing raw_data_hex")
}

func transactionFromRawDataHex(rawHex string) (*core.Transaction, error) {
	raw, err := decodeRawDataHex(rawHex)
	if err != nil {
		return nil, err
	}
	return &core.Transaction{RawData: raw}, nil
}

func parseBroadcastResult(body []byte) error {
	var br struct {
		Result  bool   `json:"result"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &br); err != nil {
		return fmt.Errorf("parse broadcast response: %w", err)
	}
	if br.Result {
		return nil
	}
	msg := br.Message
	if msg == "" {
		msg = br.Code
	}
	if msg == "" {
		msg = "broadcast failed"
	}
	return fmt.Errorf("broadcast: %s", msg)
}

func transactionToSignWeightJSON(tx *core.Transaction) (map[string]any, error) {
	rawDataHex, err := rawDataToHex(tx.GetRawData())
	if err != nil {
		return nil, err
	}
	sigs := make([]string, 0, len(tx.GetSignature()))
	for _, sig := range tx.GetSignature() {
		sigs = append(sigs, hex.EncodeToString(sig))
	}
	return map[string]any{
		"raw_data_hex": strings.TrimPrefix(rawDataHex, "0x"),
		"signature":    sigs,
		"visible":      true,
	}, nil
}

func parseSignWeightResponse(body []byte) (*api.TransactionSignWeight, error) {
	var js struct {
		Permission struct {
			ID        int32  `json:"id"`
			Threshold int64  `json:"threshold"`
			Type      string `json:"type"`
			Name      string `json:"permission_name"`
		} `json:"permission"`
		CurrentWeight int64           `json:"current_weight"`
		ApprovedList  json.RawMessage `json:"approved_list"`
		Result        struct {
			Code string `json:"code"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &js); err != nil {
		return nil, fmt.Errorf("parse sign weight: %w", err)
	}
	perm := &core.Permission{
		Id:             js.Permission.ID,
		Threshold:      js.Permission.Threshold,
		PermissionName: js.Permission.Name,
	}
	approved, err := decodeApprovedList(js.ApprovedList)
	if err != nil {
		return nil, err
	}
	out := &api.TransactionSignWeight{
		Permission:    perm,
		CurrentWeight: js.CurrentWeight,
		ApprovedList:  approved,
	}
	if js.Result.Code != "" {
		out.Result = &api.TransactionSignWeight_Result{
			Code: signWeightResultCode(js.Result.Code),
		}
	}
	return out, nil
}

func signWeightResultCode(s string) api.TransactionSignWeight_ResultResponseCode {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ENOUGH_PERMISSION":
		return api.TransactionSignWeight_Result_ENOUGH_PERMISSION
	case "NOT_ENOUGH_PERMISSION":
		return api.TransactionSignWeight_Result_NOT_ENOUGH_PERMISSION
	default:
		return api.TransactionSignWeight_Result_OTHER_ERROR
	}
}

func decodeApprovedList(raw json.RawMessage) ([][]byte, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		out := make([][]byte, 0, len(asStrings))
		for _, s := range asStrings {
			b, err := decodeTronAddressPayload(s)
			if err != nil {
				return nil, err
			}
			out = append(out, b)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported approved_list encoding")
}

func decodeTronAddressPayload(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "T") {
		a, err := address.Base58ToAddress(s)
		if err != nil {
			return nil, err
		}
		return a.Bytes(), nil
	}
	b, err := hex.DecodeString(trimHex0x(s))
	if err != nil {
		return nil, err
	}
	return b, nil
}

const trc20TransferMethodSignature = "a9059cbb"
