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

package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/transfer"
	"github.com/grafov/monujo/internal/tron"
)

// MultisigFormat enumerates the two wire formats accepted and emitted by
// [MultisigService]. MultisigFormatAuto asks the service to detect the
// format from the file contents (a top-level "monujo" key means sealed
// envelope, anything else is assumed to be canonical Tron JSON).
type MultisigFormat string

// Known multisig wire formats.
const (
	MultisigFormatAuto     MultisigFormat = "auto"
	MultisigFormatTronJSON MultisigFormat = "tron-json"
	MultisigFormatMonujo   MultisigFormat = "monujo"
)

// MultisigService implements participation in Tron multi-signature
// transactions. It does NOT create or propose new multisig transactions;
// those are built by an external coordinator (TronWeb/TronLink/TronScan/
// java-tron CLI). The service only appends a signature, inspects the
// current sign weight, and broadcasts when the threshold is met.
type MultisigService struct {
	cfg *config.Config
	ks  string

	onlineWriter func() (tron.TransactionWriter, func(), error)

	confirmIn        io.Reader
	confirmOut       io.Writer
	stdoutOverride   io.Writer
	terminalOverride *bool
}

// NewMultisigService creates a MultisigService bound to cfg.
func NewMultisigService(cfg *config.Config) *MultisigService {
	return &MultisigService{cfg: cfg, ks: cfg.KeystoreDir}
}

func (m *MultisigService) writer() (tron.TransactionWriter, func(), error) {
	if m.onlineWriter != nil {
		return m.onlineWriter()
	}
	wc, err := tron.NewTransactionWriterFromConfig(m.cfg)
	if err != nil {
		return nil, func() {}, err
	}
	return wc, func() { _ = wc.Close() }, nil
}

func (m *MultisigService) stdout() io.Writer {
	if m.stdoutOverride != nil {
		return m.stdoutOverride
	}
	return os.Stdout
}

func (m *MultisigService) stderr() io.Writer {
	if m.confirmOut != nil {
		return m.confirmOut
	}
	return os.Stderr
}

func (m *MultisigService) inReader() io.Reader {
	if m.confirmIn != nil {
		return m.confirmIn
	}
	return os.Stdin
}

func (m *MultisigService) isTerminal(fallback bool) bool {
	if m.terminalOverride != nil {
		return *m.terminalOverride
	}
	return fallback
}

// MultisigInput is the parsed transaction as it is carried between
// cosigners. It is format-agnostic: the same structure is produced from
// either a Tron JSON blob or a monujo sealed envelope, and the same
// structure is emitted back on the way out.
type MultisigInput struct {
	Format        MultisigFormat
	RawDataHex    string
	SignaturesHex []string
	TxIDHex       string
	// Fingerprint is the SHA-256 of the on-disk file bytes (always
	// computed) used for visual sanity checks between machines.
	Fingerprint transfer.FileFingerprint
	// HMACChecked indicates whether the sealed envelope's HMAC was
	// verified against the user-supplied passphrase.
	HMACChecked bool
	// Envelope carries the monujo display fields when Format is
	// MultisigFormatMonujo; it is empty for a plain Tron JSON input.
	Envelope transfer.MultisigTransferPayload
}

// CosignOptions carries inputs for [MultisigService.Cosign].
type CosignOptions struct {
	InPath         string
	OutPath        string
	Format         MultisigFormat // auto|tron-json|monujo: input format; also default output
	OutFormat      MultisigFormat // optional override for output format only
	WalletName     string
	WalletPass     []byte
	ReadWalletPass func() ([]byte, error)
	Overwrite      bool
	HMACPassphrase []byte
	Yes            bool
	IsTerminal     bool
	Force          bool // override "already signed" refusal
	Note           string
}

// Cosign reads an incoming multisig transaction, verifies its
// human-readable fields against the raw_data, asks for confirmation,
// signs with the keystore wallet and writes the updated transaction in
// the requested (or same-as-input) format. The written file is ready to
// be forwarded to the next cosigner or to `multisig broadcast`.
func (m *MultisigService) Cosign(opts CosignOptions) (transfer.FileFingerprint, error) {
	var zero transfer.FileFingerprint
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zero, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	blob, err := os.ReadFile(opts.InPath)
	if err != nil {
		return zero, fmt.Errorf("read --in: %w", err)
	}
	in, err := m.readInput(blob, opts.Format, hmacKey)
	if err != nil {
		return zero, err
	}
	signerAddr, err := localWalletAddress(m.ks, opts.WalletName)
	if err != nil {
		return in.Fingerprint, err
	}

	summary, summaryErr := tron.DecodeRawDataSummary(in.RawDataHex)
	displayKind, displayFields, warnings, err := m.renderSummary(in, summary, summaryErr)
	if err != nil {
		return in.Fingerprint, err
	}

	m.printCosignConfirm(in, signerAddr, displayKind, displayFields, warnings, opts.InPath)
	if !opts.Yes {
		if err := m.requireConfirm(opts.IsTerminal); err != nil {
			return in.Fingerprint, err
		}
	}

	if in.Format == MultisigFormatMonujo && in.Envelope.Chain != "" {
		if in.Envelope.Chain != chainForCfg(m.cfg) {
			return in.Fingerprint, fmt.Errorf("envelope chain %q does not match current --network %q",
				in.Envelope.Chain, chainForCfg(m.cfg))
		}
	}

	recorded, err := keystore.ReadWalletNetworks(m.ks, opts.WalletName)
	if err != nil {
		return in.Fingerprint, err
	}
	netForKS := networkFromChain(chainForCfg(m.cfg))
	if !opts.Force && keystore.NetworkOperationNeedsConfirmation(recorded, netForKS) {
		return in.Fingerprint, fmt.Errorf("wallet %q networks %v do not include %q; pass --force to proceed",
			opts.WalletName, recorded, netForKS)
	}

	walletPass, cleanupPass, err := readSignPassphrase(opts.WalletPass, opts.ReadWalletPass)
	if err != nil {
		return in.Fingerprint, err
	}
	defer cleanupPass()

	pk, err := keystore.LoadPrivateKey(m.ks, opts.WalletName, walletPass)
	if err != nil {
		return in.Fingerprint, err
	}
	defer pk.Wipe()

	newSigs, txid, err := tron.AppendMultisigSignature(in.RawDataHex, in.SignaturesHex, pk.Key)
	if err != nil {
		if errors.Is(err, tron.ErrAlreadySigned) && !opts.Force {
			return in.Fingerprint, fmt.Errorf("wallet %q already signed this transaction; pass --force to add a duplicate signature anyway",
				opts.WalletName)
		}
		return in.Fingerprint, err
	}
	if txid != in.TxIDHex {
		return in.Fingerprint, fmt.Errorf("computed txid %s differs from input txid %s (corrupted file?)", txid, in.TxIDHex)
	}

	outFormat := opts.OutFormat
	if outFormat == "" || outFormat == MultisigFormatAuto {
		outFormat = opts.Format
	}
	if outFormat == "" || outFormat == MultisigFormatAuto {
		outFormat = in.Format
	}
	outBlob, err := m.renderOutput(outFormat, in, newSigs, opts.Note, hmacKey)
	if err != nil {
		return in.Fingerprint, err
	}
	outFP, err := m.writeFile(opts.OutPath, outBlob, opts.Overwrite)
	if err != nil {
		return outFP, err
	}

	if err := keystore.EnsureNetworkRecorded(m.ks, opts.WalletName, netForKS); err != nil {
		return outFP, fmt.Errorf("signed ok but could not update wallet networks metadata: %w", err)
	}
	if err := m.printCosignResult(outFormat, txid, len(newSigs), outFP, opts.OutPath); err != nil {
		return outFP, err
	}
	return outFP, nil
}

// InfoOptions carries inputs for [MultisigService.Info].
type InfoOptions struct {
	InPath         string
	Format         MultisigFormat
	HMACPassphrase []byte
}

// InfoResult is the machine-readable output of [MultisigService.Info].
type InfoResult struct {
	TxID            string   `json:"tx_id"`
	PermissionID    int32    `json:"permission_id"`
	PermissionName  string   `json:"permission_name"`
	Threshold       int64    `json:"threshold"`
	CurrentWeight   int64    `json:"current_weight"`
	ApprovedSigners []string `json:"approved_signers"`
	MissingSigners  []string `json:"missing_signers"`
	ResultCode      string   `json:"result_code,omitempty"`
	ResultMessage   string   `json:"result_message,omitempty"`
}

// Info inspects a multisig transaction and asks the node for the current
// sign weight and permission. No broadcast and no signing occur.
func (m *MultisigService) Info(ctx context.Context, opts InfoOptions) (InfoResult, error) {
	var zero InfoResult
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zero, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	blob, err := os.ReadFile(opts.InPath)
	if err != nil {
		return zero, fmt.Errorf("read --in: %w", err)
	}
	in, err := m.readInput(blob, opts.Format, hmacKey)
	if err != nil {
		return zero, err
	}

	wc, closer, err := m.writer()
	if err != nil {
		return zero, err
	}
	defer closer()

	weight, err := tron.GetMultisigSignWeight(ctx, wc, in.RawDataHex, in.SignaturesHex)
	if err != nil {
		return zero, err
	}
	res := buildInfoResult(in.TxIDHex, weight)
	if err := m.printInfoResult(res); err != nil {
		return res, err
	}
	return res, nil
}

// BroadcastMultisigOptions carries inputs for
// [MultisigService.BroadcastMultisig].
type BroadcastMultisigOptions struct {
	InPath         string
	Format         MultisigFormat
	HMACPassphrase []byte
	Yes            bool
	IsTerminal     bool
}

// MultisigBroadcastResult is the machine-readable result of a successful
// multisig broadcast.
type MultisigBroadcastResult struct {
	TxID          string   `json:"tx_id"`
	Network       string   `json:"network"`
	PermissionID  int32    `json:"permission_id"`
	Threshold     int64    `json:"threshold"`
	CurrentWeight int64    `json:"current_weight"`
	Signers       []string `json:"approved_signers"`
}

// BroadcastMultisig submits a fully signed multisig transaction. The node
// is queried for the current sign weight first; the broadcast is refused
// (with a helpful error) if the accumulated weight has not reached the
// permission threshold.
func (m *MultisigService) BroadcastMultisig(ctx context.Context, opts BroadcastMultisigOptions) (MultisigBroadcastResult, error) {
	var zero MultisigBroadcastResult
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zero, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	blob, err := os.ReadFile(opts.InPath)
	if err != nil {
		return zero, fmt.Errorf("read --in: %w", err)
	}
	in, err := m.readInput(blob, opts.Format, hmacKey)
	if err != nil {
		return zero, err
	}

	summary, summaryErr := tron.DecodeRawDataSummary(in.RawDataHex)
	displayKind, displayFields, warnings, err := m.renderSummary(in, summary, summaryErr)
	if err != nil {
		return zero, err
	}
	m.printBroadcastConfirm(in, displayKind, displayFields, warnings, opts.InPath)
	if !opts.Yes {
		if err := m.requireConfirm(opts.IsTerminal); err != nil {
			return zero, err
		}
	}

	wc, closer, err := m.writer()
	if err != nil {
		return zero, err
	}
	defer closer()

	txid, weight, err := tron.BroadcastMultisig(ctx, wc, in.RawDataHex, in.SignaturesHex)
	if err != nil {
		return zero, err
	}
	approved := base58ApprovedList(weight.GetApprovedList())
	res := MultisigBroadcastResult{
		TxID:          txid,
		Network:       networkFromChain(chainForCfg(m.cfg)),
		PermissionID:  weight.GetPermission().GetId(),
		Threshold:     weight.GetPermission().GetThreshold(),
		CurrentWeight: weight.GetCurrentWeight(),
		Signers:       approved,
	}
	if err := m.printMultisigBroadcastResult(res); err != nil {
		return res, err
	}
	return res, nil
}

// readInput parses blob as either a Tron JSON envelope or a monujo sealed
// envelope, honoring the explicit format or auto-detecting. The returned
// [MultisigInput] always has Fingerprint populated and SignaturesHex
// normalised to lowercase hex with no 0x prefix.
func (m *MultisigService) readInput(blob []byte, format MultisigFormat, hmacKey []byte) (MultisigInput, error) {
	var zero MultisigInput
	detected := detectMultisigFormat(blob)
	if format == "" || format == MultisigFormatAuto {
		format = detected
	}
	switch format {
	case MultisigFormatMonujo:
		body, fp, err := transfer.OpenEnvelope(blob, transfer.KindMultisigTransfer, hmacKey)
		if err != nil {
			return zero, err
		}
		pl, err := transfer.UnmarshalMultisigPayload(body)
		if err != nil {
			return zero, fmt.Errorf("decode multisig payload: %w", err)
		}
		return MultisigInput{
			Format:        MultisigFormatMonujo,
			RawDataHex:    pl.RawDataHex,
			SignaturesHex: pl.SignaturesHex,
			TxIDHex:       strings.TrimPrefix(strings.ToLower(pl.TxIDHex), "0x"),
			Fingerprint:   fp,
			HMACChecked:   hmacKey != nil,
			Envelope:      pl,
		}, nil
	case MultisigFormatTronJSON:
		rawHex, sigs, txid, err := tron.DecodeTronJSON(blob)
		if err != nil {
			return zero, err
		}
		return MultisigInput{
			Format:        MultisigFormatTronJSON,
			RawDataHex:    rawHex,
			SignaturesHex: sigs,
			TxIDHex:       txid,
			Fingerprint:   plainFingerprint(blob),
		}, nil
	default:
		return zero, fmt.Errorf("unknown multisig format %q", format)
	}
}

// plainFingerprint computes the SHA-256 of raw bytes. Tron JSON files are
// not monujo envelopes but a fingerprint is still useful for visual
// sanity checks between cosigner machines.
func plainFingerprint(blob []byte) transfer.FileFingerprint {
	sum := sha256.Sum256(blob)
	return transfer.FileFingerprint{SHA256Hex: hex.EncodeToString(sum[:])}
}

// renderOutput serializes an updated transaction in the requested format.
// For MultisigFormatMonujo the monujo envelope is rebuilt around the
// original display fields (which may be empty for kind=unknown_contract);
// for MultisigFormatTronJSON the raw_data_hex and signatures are wrapped
// in the canonical TRON HTTP envelope.
func (m *MultisigService) renderOutput(format MultisigFormat, in MultisigInput, sigs []string, note string, hmacKey []byte) ([]byte, error) {
	switch format {
	case MultisigFormatTronJSON:
		return tron.EncodeTronJSON(in.RawDataHex, sigs)
	case MultisigFormatMonujo:
		pl := in.Envelope
		if pl.Chain == "" {
			pl = buildMonujoFromTronJSON(in, m.cfg)
		}
		pl.SignaturesHex = sigs
		pl.Version = transfer.EnvelopeVersion
		if note != "" {
			pl.Note = note
		}
		body, err := transfer.MarshalMultisigPayload(pl)
		if err != nil {
			return nil, err
		}
		return transfer.SealPayload(transfer.KindMultisigTransfer, body, hmacKey)
	}
	return nil, fmt.Errorf("unknown multisig format %q", format)
}

// buildMonujoFromTronJSON synthesizes a [transfer.MultisigTransferPayload]
// from a Tron JSON input (which does not carry monujo display fields).
// TRX and TRC-20 transfers are populated fully; other contract types get
// KindUnknownContract and leave to/amount/contract empty.
func buildMonujoFromTronJSON(in MultisigInput, cfg *config.Config) transfer.MultisigTransferPayload {
	pl := transfer.MultisigTransferPayload{
		Version:    transfer.EnvelopeVersion,
		Chain:      chainForCfg(cfg),
		RawDataHex: in.RawDataHex,
		TxIDHex:    in.TxIDHex,
	}
	summary, err := tron.DecodeRawDataSummary(in.RawDataHex)
	if err != nil {
		pl.Kind = transfer.KindUnknownContract
		return pl
	}
	pl.From = summary.From
	pl.To = summary.To
	pl.Kind = summary.Kind
	pl.Contract = summary.Contract
	pl.FeeLimitSun = summary.FeeLimitSun
	pl.Expiration = summary.ExpirationMs
	pl.Timestamp = summary.TimestampMs
	pl.Amount = summary.AmountStr
	if summary.AmountWei != nil {
		pl.AmountMinimal = summary.AmountWei.String()
	}
	pl.PermissionID = tron.DecodeRawDataPermissionID(in.RawDataHex)
	return pl
}

func localWalletAddress(keystoreDir, name string) (string, error) {
	items, err := keystore.ListWallets(keystoreDir)
	if err != nil {
		return "", err
	}
	for _, w := range items {
		if w.Name != name {
			continue
		}
		if w.External {
			return "", fmt.Errorf("wallet %q is external (address-only); multisig cosign needs a local encrypted keystore file", name)
		}
		return w.Address, nil
	}
	return "", fmt.Errorf("wallet %q not found in keystore %q", name, keystoreDir)
}

func permissionIDForInput(in MultisigInput) int32 {
	if in.Envelope.PermissionID != 0 {
		return in.Envelope.PermissionID
	}
	return tron.DecodeRawDataPermissionID(in.RawDataHex)
}

// renderSummary pretty-prints the relevant fields of the incoming
// transaction. When the summary decoder succeeded and the kind is TRX or
// TRC-20, display fields are extracted from raw_data and strict-cross-
// checked against any monujo envelope fields. When decoding failed (e.g.
// an exchange contract) the function returns a warning and no display
// values so the caller can show a generic "forward-as-is" prompt.
func (m *MultisigService) renderSummary(in MultisigInput, s tron.RawDataSummary, sErr error) (kind string, fields map[string]string, warnings []string, err error) {
	fields = map[string]string{}
	if sErr != nil {
		warnings = append(warnings, fmt.Sprintf("raw_data contract not decoded: %v (forwarding signatures anyway)", sErr))
		if in.Format == MultisigFormatMonujo && in.Envelope.Kind == transfer.KindUnknownContract {
			return in.Envelope.Kind, fields, warnings, nil
		}
		return transfer.KindUnknownContract, fields, warnings, nil
	}
	kind = s.Kind
	fields["from"] = s.From
	fields["to"] = s.To
	fields["amount"] = s.AmountStr
	if s.AmountWei != nil {
		fields["amount_minimal"] = s.AmountWei.String()
	}
	if s.Contract != "" {
		fields["contract"] = s.Contract
	}
	if s.FeeLimitSun > 0 {
		fields["fee_limit_sun"] = fmt.Sprintf("%d", s.FeeLimitSun)
	}
	fields["expiration"] = tron.FormatExpirationISO(s.ExpirationMs)
	if in.Format == MultisigFormatMonujo && in.Envelope.Chain != "" {
		if err := verifyMultisigEnvelopeAgainstSummary(in.Envelope, s); err != nil {
			return kind, fields, warnings, err
		}
	}
	return kind, fields, warnings, nil
}

// verifyMultisigEnvelopeAgainstSummary checks that a monujo-format input
// agrees with the raw_data on the authoritative fields. For
// KindUnknownContract we only check that the decoded summary also could
// not map to TRX/USDT (which would be a contradiction).
func verifyMultisigEnvelopeAgainstSummary(p transfer.MultisigTransferPayload, s tron.RawDataSummary) error {
	if p.Kind == transfer.KindUnknownContract {
		return nil
	}
	if p.Kind != s.Kind {
		return fmt.Errorf("kind mismatch: payload=%q raw_data=%q", p.Kind, s.Kind)
	}
	if p.From != s.From {
		return fmt.Errorf("from mismatch: payload=%q raw_data=%q", p.From, s.From)
	}
	if p.To != s.To {
		return fmt.Errorf("to mismatch: payload=%q raw_data=%q", p.To, s.To)
	}
	if p.AmountMinimal != "" {
		want, ok := new(big.Int).SetString(p.AmountMinimal, 10)
		if !ok {
			return fmt.Errorf("payload amount_minimal not a decimal int: %q", p.AmountMinimal)
		}
		if s.AmountWei == nil || want.Cmp(s.AmountWei) != 0 {
			return fmt.Errorf("amount mismatch: payload=%s raw_data=%v", want, s.AmountWei)
		}
	}
	if p.Kind == transfer.KindTRC20USDT && p.Contract != s.Contract {
		return fmt.Errorf("contract mismatch: payload=%q raw_data=%q", p.Contract, s.Contract)
	}
	if p.Expiration != 0 && p.Expiration != s.ExpirationMs {
		return fmt.Errorf("expiration mismatch: payload=%d raw_data=%d", p.Expiration, s.ExpirationMs)
	}
	return nil
}

// detectMultisigFormat inspects the top-level JSON shape to choose
// between a sealed monujo envelope and a canonical Tron JSON tx.
func detectMultisigFormat(blob []byte) MultisigFormat {
	var probe struct {
		Monujo string `json:"monujo"`
		Kind   string `json:"kind"`
	}
	if err := json.Unmarshal(blob, &probe); err == nil && probe.Monujo != "" {
		return MultisigFormatMonujo
	}
	return MultisigFormatTronJSON
}

// buildInfoResult renders a [*api.TransactionSignWeight] as a typed
// [InfoResult] ready for JSON emission. MissingSigners is computed as the
// set of permission keys that did not sign yet (base58 addresses).
func buildInfoResult(txidHex string, weight *api.TransactionSignWeight) InfoResult {
	out := InfoResult{TxID: txidHex}
	perm := weight.GetPermission()
	if perm != nil {
		out.PermissionID = perm.GetId()
		out.PermissionName = perm.GetPermissionName()
		out.Threshold = perm.GetThreshold()
	}
	out.CurrentWeight = weight.GetCurrentWeight()
	out.ApprovedSigners = base58ApprovedList(weight.GetApprovedList())
	if perm != nil {
		permKeys := map[string]bool{}
		for _, k := range perm.GetKeys() {
			permKeys[common.EncodeCheck(k.GetAddress())] = true
		}
		for _, approved := range out.ApprovedSigners {
			delete(permKeys, approved)
		}
		for addr := range permKeys {
			out.MissingSigners = append(out.MissingSigners, addr)
		}
	}
	if res := weight.GetResult(); res != nil {
		out.ResultCode = res.GetCode().String()
		out.ResultMessage = res.GetMessage()
	}
	return out
}

// base58ApprovedList converts a list of raw Tron address payloads (0x41
// prefix + 20 bytes) into base58check strings.
func base58ApprovedList(raw [][]byte) []string {
	out := make([]string, 0, len(raw))
	for _, b := range raw {
		if len(b) == 0 {
			continue
		}
		out = append(out, common.EncodeCheck(b))
	}
	return out
}

// writeFile writes blob to path ("-" → stdout) and returns the fingerprint
// of the bytes that hit disk.
func (m *MultisigService) writeFile(path string, blob []byte, overwrite bool) (transfer.FileFingerprint, error) {
	fp := transfer.FileFingerprint{}
	if path == "" {
		return fp, errors.New("output path must not be empty (use \"-\" for stdout)")
	}
	if path == "-" {
		if _, err := m.stdout().Write(blob); err != nil {
			return fp, err
		}
		_, _ = m.stdout().Write([]byte("\n"))
	} else {
		if err := writeOutputFile(path, blob, overwrite); err != nil {
			return fp, fmt.Errorf("write %s: %w", path, err)
		}
	}
	return plainFingerprint(blob), nil
}

// printCosignConfirm prints the to-be-signed summary to stderr.
func (m *MultisigService) printCosignConfirm(in MultisigInput, signerAddr string, kind string, fields map[string]string, warnings []string, srcPath string) {
	w := m.stderr()
	fmt.Fprintf(w, "\n=== about to COSIGN multisig transfer ===\n")
	fmt.Fprintf(w, "  source file:   %s\n", srcPath)
	fmt.Fprintf(w, "  format:        %s\n", in.Format)
	fmt.Fprintf(w, "  file SHA-256:  %s\n", in.Fingerprint.SHA256Hex)
	fmt.Fprintf(w, "  hmac checked:  %v\n", in.HMACChecked)
	fmt.Fprintf(w, "  tx_id:         %s\n", in.TxIDHex)
	fmt.Fprintf(w, "  permission_id: %d\n", permissionIDForInput(in))
	fmt.Fprintf(w, "  signer:        %s\n", signerAddr)
	fmt.Fprintf(w, "  existing sigs: %d\n", len(in.SignaturesHex))
	fmt.Fprintf(w, "  kind:          %s\n", kind)
	for _, k := range []string{"from", "to", "amount", "amount_minimal", "contract", "fee_limit_sun", "expiration"} {
		if v, ok := fields[k]; ok && v != "" {
			fmt.Fprintf(w, "  %-14s %s\n", k+":", v)
		}
	}
	for _, warn := range warnings {
		fmt.Fprintf(w, "  WARNING:       %s\n", warn)
	}
}

func (m *MultisigService) printBroadcastConfirm(in MultisigInput, kind string, fields map[string]string, warnings []string, srcPath string) {
	w := m.stderr()
	fmt.Fprintf(w, "\n=== about to BROADCAST multisig transfer ===\n")
	fmt.Fprintf(w, "  source file:   %s\n", srcPath)
	fmt.Fprintf(w, "  format:        %s\n", in.Format)
	fmt.Fprintf(w, "  file SHA-256:  %s\n", in.Fingerprint.SHA256Hex)
	fmt.Fprintf(w, "  hmac checked:  %v\n", in.HMACChecked)
	fmt.Fprintf(w, "  tx_id:         %s\n", in.TxIDHex)
	fmt.Fprintf(w, "  signatures:    %d\n", len(in.SignaturesHex))
	fmt.Fprintf(w, "  kind:          %s\n", kind)
	for _, k := range []string{"from", "to", "amount", "amount_minimal", "contract", "fee_limit_sun", "expiration"} {
		if v, ok := fields[k]; ok && v != "" {
			fmt.Fprintf(w, "  %-14s %s\n", k+":", v)
		}
	}
	for _, warn := range warnings {
		fmt.Fprintf(w, "  WARNING:       %s\n", warn)
	}
}

// requireConfirm mirrors OfflineService.requireConfirm; kept private so
// the two services print the same prompt without an accidental drift.
func (m *MultisigService) requireConfirm(isTerminal bool) error {
	if !m.isTerminal(isTerminal) {
		return errors.New("aborted: refused to act without --yes on a non-interactive stdin")
	}
	fmt.Fprint(m.stderr(), "Continue? [y/N]: ")
	line, err := bufio.NewReader(m.inReader()).ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "y" || line == "yes" {
		return nil
	}
	return errors.New("aborted by user")
}

type cosignJSONOutput struct {
	Phase      string `json:"phase"`
	OutPath    string `json:"out_path"`
	FileSHA256 string `json:"file_sha256"`
	Format     string `json:"format"`
	Signatures int    `json:"signatures"`
	TxID       string `json:"tx_id"`
}

func (m *MultisigService) printCosignResult(format MultisigFormat, txid string, nSigs int, fp transfer.FileFingerprint, outPath string) error {
	if m.cfg.Output == config.OutputJSON {
		return json.NewEncoder(m.stdout()).Encode(cosignJSONOutput{
			Phase:      "cosign",
			OutPath:    outPath,
			FileSHA256: fp.SHA256Hex,
			Format:     string(format),
			Signatures: nSigs,
			TxID:       txid,
		})
	}
	fmt.Fprintf(m.stdout(), "Multisig cosign written to %s\n", outPath)
	fmt.Fprintf(m.stdout(), "  format:       %s\n", format)
	fmt.Fprintf(m.stdout(), "  signatures:   %d\n", nSigs)
	fmt.Fprintf(m.stdout(), "  file SHA-256: %s\n", fp.SHA256Hex)
	fmt.Fprintf(m.stdout(), "  tx_id:        %s\n", txid)
	return nil
}

func (m *MultisigService) printInfoResult(r InfoResult) error {
	if m.cfg.Output == config.OutputJSON {
		return json.NewEncoder(m.stdout()).Encode(r)
	}
	fmt.Fprintf(m.stdout(), "Multisig transaction %s\n", r.TxID)
	fmt.Fprintf(m.stdout(), "  permission:    id=%d name=%q threshold=%d\n", r.PermissionID, r.PermissionName, r.Threshold)
	fmt.Fprintf(m.stdout(), "  current weight: %d\n", r.CurrentWeight)
	fmt.Fprintf(m.stdout(), "  approved:       %d\n", len(r.ApprovedSigners))
	for _, s := range r.ApprovedSigners {
		fmt.Fprintf(m.stdout(), "    + %s\n", s)
	}
	if len(r.MissingSigners) > 0 {
		fmt.Fprintf(m.stdout(), "  missing:        %d\n", len(r.MissingSigners))
		for _, s := range r.MissingSigners {
			fmt.Fprintf(m.stdout(), "    - %s\n", s)
		}
	}
	if r.ResultMessage != "" {
		fmt.Fprintf(m.stdout(), "  node result:    %s %s\n", r.ResultCode, r.ResultMessage)
	}
	return nil
}

func (m *MultisigService) printMultisigBroadcastResult(r MultisigBroadcastResult) error {
	if m.cfg.Output == config.OutputJSON {
		return json.NewEncoder(m.stdout()).Encode(r)
	}
	fmt.Fprintf(m.stdout(), "Multisig broadcast OK\nNetwork: %s\nTxID:    %s\n", r.Network, r.TxID)
	fmt.Fprintf(m.stdout(), "Permission id=%d threshold=%d current_weight=%d\n", r.PermissionID, r.Threshold, r.CurrentWeight)
	for _, s := range r.Signers {
		fmt.Fprintf(m.stdout(), "  signer: %s\n", s)
	}
	return nil
}
