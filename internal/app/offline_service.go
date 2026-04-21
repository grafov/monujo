package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/domain"
	"github.com/grafov/monujo/internal/keystore"
	"github.com/grafov/monujo/internal/transfer"
	"github.com/grafov/monujo/internal/tron"
)

// OfflineService implements the air-gap workflow: build (online), sign
// (offline), broadcast (online). It is intentionally split from
// [HotService] so the offline machine can be installed without any online
// configuration and vice versa.
//
// Build/Broadcast never touch the keystore directory; only Sign does.
type OfflineService struct {
	cfg *config.Config
	ks  string

	// onlineWriter and broadcaster exist to make Build/Broadcast testable
	// without a real gRPC server. Production callers leave them nil and the
	// service falls back to tron.NewWriteClientFromConfig.
	onlineWriter func() (*tron.WriteClient, func(), error)

	// confirmIn is os.Stdin in production, overridable in tests.
	confirmIn io.Reader
	// confirmOut is os.Stderr in production (matches HotService output split:
	// human-facing prompts go to stderr, machine output to stdout).
	confirmOut io.Writer
	// stdoutOverride lets tests capture the JSON/table output stream.
	stdoutOverride io.Writer
	// terminalOverride lets tests force the "interactive" branch.
	terminalOverride *bool
}

// NewOfflineService creates an OfflineService bound to cfg. cfg.KeystoreDir
// is consulted only by the Sign step.
func NewOfflineService(cfg *config.Config) *OfflineService {
	return &OfflineService{cfg: cfg, ks: cfg.KeystoreDir}
}

func (o *OfflineService) writer() (*tron.WriteClient, func(), error) {
	if o.onlineWriter != nil {
		return o.onlineWriter()
	}
	wc, err := tron.NewWriteClientFromConfig(o.cfg)
	if err != nil {
		return nil, func() {}, err
	}
	return wc, func() { _ = wc.Close() }, nil
}

func (o *OfflineService) stdout() io.Writer {
	if o.stdoutOverride != nil {
		return o.stdoutOverride
	}
	return os.Stdout
}

func (o *OfflineService) stderr() io.Writer {
	if o.confirmOut != nil {
		return o.confirmOut
	}
	return os.Stderr
}

func (o *OfflineService) inReader() io.Reader {
	if o.confirmIn != nil {
		return o.confirmIn
	}
	return os.Stdin
}

func (o *OfflineService) isTerminal(fallback bool) bool {
	if o.terminalOverride != nil {
		return *o.terminalOverride
	}
	return fallback
}

// BuildOptions carries inputs for [OfflineService.Build].
//
// FromAddress is a Tron base58 address; name↔address resolution is done
// by the CLI layer via resolveWalletFrom so the service only deals with
// a validated address.
type BuildOptions struct {
	Kind           string // transfer.KindTRX or transfer.KindTRC20USDT
	FromAddress    string // Tron base58 address of the cold wallet
	To             string
	AmountTRX      string // for KindTRX
	AmountUSDT     string // for KindTRC20USDT
	Note           string
	OutPath        string // file to write the unsigned envelope to; "-" → stdout
	HMACPassphrase []byte // optional; if set, envelope is HMAC-protected
	IsTerminal     bool   // whether stdin is a tty (caller resolved this)
}

// resolveFromAddress validates opts.FromAddress. The CLI layer already
// maps the unified --from flag (name or base58) to a base58 address, so
// all that remains here is trimming and Tron-format validation.
func (o *OfflineService) resolveFromAddress(opts BuildOptions) (string, error) {
	addr := strings.TrimSpace(opts.FromAddress)
	if addr == "" {
		return "", errors.New("--from must be provided (Tron base58 address or keystore wallet name)")
	}
	if err := tron.ValidateBase58Address(addr); err != nil {
		return "", err
	}
	return addr, nil
}

// Build creates an unsigned transfer envelope and writes it to opts.OutPath.
// Network is taken from o.cfg.Network. Returns the seal fingerprint of the
// written file so the caller can show it to the human (or log it).
func (o *OfflineService) Build(ctx context.Context, opts BuildOptions) (transfer.FileFingerprint, error) {
	var zeroFP transfer.FileFingerprint
	from, err := o.resolveFromAddress(opts)
	if err != nil {
		return zeroFP, err
	}
	if err := tron.ValidateBase58Address(opts.To); err != nil {
		return zeroFP, fmt.Errorf("--to: %w", err)
	}

	wc, closer, err := o.writer()
	if err != nil {
		return zeroFP, fmt.Errorf("connect online node: %w", err)
	}
	defer closer()

	var pl transfer.UnsignedTransferPayload
	switch opts.Kind {
	case transfer.KindTRX:
		amountSun, perr := domain.ParseTRXDecimal(opts.AmountTRX)
		if perr != nil {
			return zeroFP, perr
		}
		pl, err = tron.BuildUnsignedTRX(ctx, wc, from, opts.To, amountSun, opts.Note)
	case transfer.KindTRC20USDT:
		amount, perr := domain.ParseUSDTDecimal(opts.AmountUSDT)
		if perr != nil {
			return zeroFP, perr
		}
		if amount.Sign() <= 0 {
			return zeroFP, errors.New("amount must be positive")
		}
		pl, err = tron.BuildUnsignedUSDT(ctx, wc, from, opts.To, amount, opts.Note)
	default:
		return zeroFP, fmt.Errorf("unsupported kind %q", opts.Kind)
	}
	if err != nil {
		return zeroFP, err
	}

	body, err := transfer.MarshalUnsignedPayload(pl)
	if err != nil {
		return zeroFP, err
	}
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zeroFP, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	blob, err := transfer.SealPayload(transfer.KindUnsignedTransfer, body, hmacKey)
	if err != nil {
		return zeroFP, err
	}
	fp, err := o.writeFile(opts.OutPath, blob)
	if err != nil {
		return zeroFP, err
	}

	if err := o.printBuildResult(pl, fp, opts.OutPath, len(hmacKey) > 0); err != nil {
		return fp, err
	}
	return fp, nil
}

// SignOptions carries inputs for [OfflineService.Sign].
type SignOptions struct {
	InPath         string
	OutPath        string
	WalletName     string
	WalletPass     []byte
	HMACPassphrase []byte
	Yes            bool
	IsTerminal     bool
	Force          bool // skip wallet network mismatch confirmation
}

// Sign reads an unsigned envelope, decodes the raw_data, verifies the
// declared payload fields against it, asks the human to confirm (unless
// Yes), then signs with the keystore and writes the signed envelope.
//
// The keystore's wallet address must match the unsigned payload's From
// field; otherwise Sign refuses to load the key (defends against silently
// signing with a different cold wallet on the offline machine).
func (o *OfflineService) Sign(opts SignOptions) (transfer.FileFingerprint, error) {
	var zeroFP transfer.FileFingerprint

	blob, err := os.ReadFile(opts.InPath)
	if err != nil {
		return zeroFP, fmt.Errorf("read --in: %w", err)
	}
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zeroFP, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	body, fp, err := transfer.OpenEnvelope(blob, transfer.KindUnsignedTransfer, hmacKey)
	if err != nil {
		return fp, err
	}
	pl, err := transfer.UnmarshalUnsignedPayload(body)
	if err != nil {
		return fp, err
	}

	summary, err := tron.DecodeRawDataSummary(pl.RawDataHex)
	if err != nil {
		return fp, fmt.Errorf("decode raw_data: %w", err)
	}
	if err := verifyPayloadAgainstSummary(pl, summary); err != nil {
		return fp, err
	}
	if pl.Chain != chainForCfg(o.cfg) {
		return fp, fmt.Errorf("envelope chain %q does not match current --network %q",
			pl.Chain, chainForCfg(o.cfg))
	}

	o.printConfirm("about to SIGN unsigned transfer", pl, fp, opts.InPath, hmacKey != nil)
	if !opts.Yes {
		if err := o.requireConfirm(); err != nil {
			return fp, err
		}
	}

	recorded, err := keystore.ReadWalletNetworks(o.ks, opts.WalletName)
	if err != nil {
		return fp, err
	}
	netForKS := networkFromChain(pl.Chain)
	if !opts.Force && keystore.NetworkOperationNeedsConfirmation(recorded, netForKS) {
		return fp, fmt.Errorf("wallet %q networks %v do not include %q; pass --force to proceed",
			opts.WalletName, recorded, netForKS)
	}

	pk, err := keystore.LoadPrivateKey(o.ks, opts.WalletName, opts.WalletPass)
	if err != nil {
		return fp, err
	}
	defer pk.Wipe()
	if pk.Address != pl.From {
		return fp, fmt.Errorf("keystore wallet %q address %s does not match unsigned payload from %s",
			opts.WalletName, pk.Address, pl.From)
	}

	sigHex, txid, err := tron.SignRawData(pl.RawDataHex, pk.Key)
	if err != nil {
		return fp, err
	}
	if txid != pl.TxIDHex {
		return fp, fmt.Errorf("computed txid %s differs from payload tx_id_hex %s (corrupted file?)", txid, pl.TxIDHex)
	}

	signed := transfer.SignedTransferPayload{
		Version:      transfer.EnvelopeVersion,
		Chain:        pl.Chain,
		Kind:         pl.Kind,
		From:         pl.From,
		To:           pl.To,
		Amount:       pl.Amount,
		Contract:     pl.Contract,
		FeeLimitSun:  pl.FeeLimitSun,
		Expiration:   pl.Expiration,
		TxIDHex:      pl.TxIDHex,
		RawDataHex:   pl.RawDataHex,
		SignatureHex: sigHex,
	}
	signedBody, err := transfer.MarshalSignedPayload(signed)
	if err != nil {
		return fp, err
	}
	signedBlob, err := transfer.SealPayload(transfer.KindSignedTransfer, signedBody, hmacKey)
	if err != nil {
		return fp, err
	}
	outFP, err := o.writeFile(opts.OutPath, signedBlob)
	if err != nil {
		return outFP, err
	}

	if err := keystore.EnsureNetworkRecorded(o.ks, opts.WalletName, netForKS); err != nil {
		return outFP, fmt.Errorf("signed ok but could not update wallet networks metadata: %w", err)
	}

	if err := o.printSignResult(signed, outFP, opts.OutPath); err != nil {
		return outFP, err
	}
	return outFP, nil
}

// BroadcastOptions carries inputs for [OfflineService.Broadcast].
type BroadcastOptions struct {
	InPath         string
	HMACPassphrase []byte
	Yes            bool
	IsTerminal     bool
}

// BroadcastResult is what Broadcast returns/prints.
type BroadcastResult struct {
	TxID    string `json:"tx_id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`
	Amount  string `json:"amount"`
	Network string `json:"network"`
}

// Broadcast reads a signed envelope, verifies fields, asks for confirmation
// (unless Yes), and forwards to the gRPC node.
func (o *OfflineService) Broadcast(ctx context.Context, opts BroadcastOptions) (BroadcastResult, error) {
	var zero BroadcastResult
	blob, err := os.ReadFile(opts.InPath)
	if err != nil {
		return zero, fmt.Errorf("read --in: %w", err)
	}
	hmacKey, err := optionalHMACKey(opts.HMACPassphrase)
	if err != nil {
		return zero, err
	}
	defer keystore.ZeroPassphrase(hmacKey)

	body, fp, err := transfer.OpenEnvelope(blob, transfer.KindSignedTransfer, hmacKey)
	if err != nil {
		return zero, err
	}
	signed, err := transfer.UnmarshalSignedPayload(body)
	if err != nil {
		return zero, err
	}

	summary, err := tron.DecodeRawDataSummary(signed.RawDataHex)
	if err != nil {
		return zero, fmt.Errorf("decode raw_data: %w", err)
	}
	if err := verifySignedAgainstSummary(signed, summary); err != nil {
		return zero, err
	}
	if signed.Chain != chainForCfg(o.cfg) {
		return zero, fmt.Errorf("envelope chain %q does not match current --network %q",
			signed.Chain, chainForCfg(o.cfg))
	}

	o.printSignedConfirm("about to BROADCAST signed transfer", signed, fp, opts.InPath, hmacKey != nil)
	if !opts.Yes {
		if err := o.requireConfirm(); err != nil {
			return zero, err
		}
	}

	wc, closer, err := o.writer()
	if err != nil {
		return zero, err
	}
	defer closer()

	txid, err := tron.BroadcastSigned(ctx, wc, signed.RawDataHex, signed.SignatureHex)
	if err != nil {
		return zero, err
	}
	res := BroadcastResult{
		TxID:    txid,
		From:    signed.From,
		To:      signed.To,
		Kind:    signed.Kind,
		Amount:  signed.Amount,
		Network: networkFromChain(signed.Chain),
	}
	if err := o.printBroadcastResult(res); err != nil {
		return res, err
	}
	return res, nil
}

// optionalHMACKey returns nil for empty passphrase and a derived key
// otherwise. The returned slice is owned by the caller and should be wiped.
func optionalHMACKey(pass []byte) ([]byte, error) {
	if len(pass) == 0 {
		return nil, nil
	}
	return transfer.DeriveHMACKey(pass)
}

// chainForCfg mirrors tron.chainForNetwork at the app layer (we re-derive
// it here to avoid taking a dep on an unexported tron helper).
func chainForCfg(cfg *config.Config) transfer.ChainID {
	switch cfg.Network {
	case config.NetworkShasta:
		return transfer.ChainTronShasta
	default:
		return transfer.ChainTronMainnet
	}
}

// networkFromChain converts a transfer.ChainID back to a keystore network
// id ("mainnet"/"shasta") for EnsureNetworkRecorded calls.
func networkFromChain(c transfer.ChainID) string {
	switch c {
	case transfer.ChainTronShasta:
		return string(config.NetworkShasta)
	default:
		return string(config.NetworkMainnet)
	}
}

// verifyPayloadAgainstSummary refuses to proceed if the human-readable
// fields declared in the payload disagree with what is actually inside
// raw_data. This catches a tampered file even when no HMAC is in use.
func verifyPayloadAgainstSummary(p transfer.UnsignedTransferPayload, s tron.RawDataSummary) error {
	if p.Kind != s.Kind {
		return fmt.Errorf("kind mismatch: payload=%q raw_data=%q", p.Kind, s.Kind)
	}
	if p.From != s.From {
		return fmt.Errorf("from mismatch: payload=%q raw_data=%q", p.From, s.From)
	}
	if p.To != s.To {
		return fmt.Errorf("to mismatch: payload=%q raw_data=%q", p.To, s.To)
	}
	wantWei, ok := new(big.Int).SetString(p.AmountMinimal, 10)
	if !ok {
		return fmt.Errorf("payload amount_minimal not a decimal int: %q", p.AmountMinimal)
	}
	if s.AmountWei == nil || wantWei.Cmp(s.AmountWei) != 0 {
		return fmt.Errorf("amount mismatch: payload=%s raw_data=%v", wantWei, s.AmountWei)
	}
	if p.Kind == transfer.KindTRC20USDT && p.Contract != s.Contract {
		return fmt.Errorf("contract mismatch: payload=%q raw_data=%q", p.Contract, s.Contract)
	}
	if p.Expiration != s.ExpirationMs {
		return fmt.Errorf("expiration mismatch: payload=%d raw_data=%d", p.Expiration, s.ExpirationMs)
	}
	return nil
}

// verifySignedAgainstSummary is verifyPayloadAgainstSummary for the signed
// payload (which carries fewer fields than the unsigned one).
func verifySignedAgainstSummary(p transfer.SignedTransferPayload, s tron.RawDataSummary) error {
	if p.Kind != s.Kind {
		return fmt.Errorf("kind mismatch: payload=%q raw_data=%q", p.Kind, s.Kind)
	}
	if p.From != s.From {
		return fmt.Errorf("from mismatch: payload=%q raw_data=%q", p.From, s.From)
	}
	if p.To != s.To {
		return fmt.Errorf("to mismatch: payload=%q raw_data=%q", p.To, s.To)
	}
	if p.Kind == transfer.KindTRC20USDT && p.Contract != s.Contract {
		return fmt.Errorf("contract mismatch: payload=%q raw_data=%q", p.Contract, s.Contract)
	}
	if p.Expiration != s.ExpirationMs {
		return fmt.Errorf("expiration mismatch: payload=%d raw_data=%d", p.Expiration, s.ExpirationMs)
	}
	return nil
}

// writeFile writes blob to path (or stdout when path == "-") and returns
// the SHA-256 fingerprint of the bytes that hit disk.
func (o *OfflineService) writeFile(path string, blob []byte) (transfer.FileFingerprint, error) {
	fp := transfer.FileFingerprint{}
	if path == "" {
		return fp, errors.New("output path must not be empty (use \"-\" for stdout)")
	}
	if path == "-" {
		_, err := o.stdout().Write(blob)
		if err != nil {
			return fp, err
		}
		_, _ = o.stdout().Write([]byte("\n"))
	} else {
		if err := os.WriteFile(path, blob, 0o600); err != nil {
			return fp, fmt.Errorf("write %s: %w", path, err)
		}
	}
	// Recompute fingerprint from the bytes we serialized so the value
	// shown to the human matches what's on disk byte-for-byte.
	body := blob
	_, fp2, err := transfer.OpenEnvelope(body, detectKind(body), nil)
	if err == nil {
		return fp2, nil
	}
	// Fallback: ignore HMAC errors (we just want the fingerprint).
	if errors.Is(err, transfer.ErrHMACRequired) {
		return fp2, nil
	}
	return fp, err
}

// detectKind peeks at an envelope blob to choose the right kind for
// fingerprinting via OpenEnvelope (we don't care about the payload here).
func detectKind(blob []byte) string {
	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(blob, &probe); err == nil && probe.Kind != "" {
		return probe.Kind
	}
	return transfer.KindUnsignedTransfer
}

// printConfirm prints the to-be-signed unsigned payload summary to stderr
// before asking for confirmation.
func (o *OfflineService) printConfirm(action string, p transfer.UnsignedTransferPayload, fp transfer.FileFingerprint, srcPath string, hmacChecked bool) {
	w := o.stderr()
	fmt.Fprintf(w, "\n=== %s ===\n", action)
	fmt.Fprintf(w, "  source file:   %s\n", srcPath)
	fmt.Fprintf(w, "  file SHA-256:  %s\n", fp.SHA256Hex)
	fmt.Fprintf(w, "  hmac checked:  %v\n", hmacChecked)
	fmt.Fprintf(w, "  network:       %s\n", p.Chain)
	fmt.Fprintf(w, "  kind:          %s\n", p.Kind)
	fmt.Fprintf(w, "  from:          %s\n", p.From)
	fmt.Fprintf(w, "  to:            %s\n", p.To)
	fmt.Fprintf(w, "  amount:        %s (minimal: %s)\n", p.Amount, p.AmountMinimal)
	if p.Contract != "" {
		fmt.Fprintf(w, "  contract:      %s\n", p.Contract)
	}
	if p.FeeLimitSun > 0 {
		fmt.Fprintf(w, "  fee_limit_sun: %d\n", p.FeeLimitSun)
	}
	fmt.Fprintf(w, "  expiration:    %s (unix_ms=%d)\n", tron.FormatExpirationISO(p.Expiration), p.Expiration)
	fmt.Fprintf(w, "  tx_id:         %s\n", p.TxIDHex)
	if p.Note != "" {
		fmt.Fprintf(w, "  note:          %s\n", p.Note)
	}
}

func (o *OfflineService) printSignedConfirm(action string, p transfer.SignedTransferPayload, fp transfer.FileFingerprint, srcPath string, hmacChecked bool) {
	w := o.stderr()
	fmt.Fprintf(w, "\n=== %s ===\n", action)
	fmt.Fprintf(w, "  source file:   %s\n", srcPath)
	fmt.Fprintf(w, "  file SHA-256:  %s\n", fp.SHA256Hex)
	fmt.Fprintf(w, "  hmac checked:  %v\n", hmacChecked)
	fmt.Fprintf(w, "  network:       %s\n", p.Chain)
	fmt.Fprintf(w, "  kind:          %s\n", p.Kind)
	fmt.Fprintf(w, "  from:          %s\n", p.From)
	fmt.Fprintf(w, "  to:            %s\n", p.To)
	fmt.Fprintf(w, "  amount:        %s\n", p.Amount)
	if p.Contract != "" {
		fmt.Fprintf(w, "  contract:      %s\n", p.Contract)
	}
	if p.FeeLimitSun > 0 {
		fmt.Fprintf(w, "  fee_limit_sun: %d\n", p.FeeLimitSun)
	}
	fmt.Fprintf(w, "  expiration:    %s (unix_ms=%d)\n", tron.FormatExpirationISO(p.Expiration), p.Expiration)
	fmt.Fprintf(w, "  tx_id:         %s\n", p.TxIDHex)
}

// requireConfirm blocks until the user types y/yes on stdin; returns an
// error if stdin is not a terminal (caller can pre-set --yes for scripts).
func (o *OfflineService) requireConfirm() error {
	if !o.isTerminal(true) {
		return errors.New("aborted: refused to act without --yes on a non-interactive stdin")
	}
	fmt.Fprint(o.stderr(), "Continue? [y/N]: ")
	line, err := bufio.NewReader(o.inReader()).ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "y" || line == "yes" {
		return nil
	}
	return errors.New("aborted by user")
}

// buildJSONOutput is the typed JSON output for the build phase. Using a
// concrete type (instead of map[string]any) lets json.Encode validate at
// compile time and keeps the linter happy.
type buildJSONOutput struct {
	Phase         string           `json:"phase"`
	OutPath       string           `json:"out_path"`
	FileSHA256    string           `json:"file_sha256"`
	HMACEnabled   bool             `json:"hmac_enabled"`
	Chain         transfer.ChainID `json:"chain"`
	Kind          string           `json:"kind"`
	From          string           `json:"from"`
	To            string           `json:"to"`
	Amount        string           `json:"amount"`
	AmountMinimal string           `json:"amount_minimal"`
	Contract      string           `json:"contract,omitempty"`
	FeeLimitSun   int64            `json:"fee_limit_sun,omitempty"`
	ExpirationMs  int64            `json:"expiration_ms"`
	TxID          string           `json:"tx_id"`
}

type signJSONOutput struct {
	Phase      string           `json:"phase"`
	OutPath    string           `json:"out_path"`
	FileSHA256 string           `json:"file_sha256"`
	Chain      transfer.ChainID `json:"chain"`
	TxID       string           `json:"tx_id"`
}

// printBuildResult prints a JSON or table summary to stdout (machine output).
func (o *OfflineService) printBuildResult(p transfer.UnsignedTransferPayload, fp transfer.FileFingerprint, outPath string, hmacOn bool) error {
	if o.cfg.Output == config.OutputJSON {
		return json.NewEncoder(o.stdout()).Encode(buildJSONOutput{
			Phase:         "build",
			OutPath:       outPath,
			FileSHA256:    fp.SHA256Hex,
			HMACEnabled:   hmacOn,
			Chain:         p.Chain,
			Kind:          p.Kind,
			From:          p.From,
			To:            p.To,
			Amount:        p.Amount,
			AmountMinimal: p.AmountMinimal,
			Contract:      p.Contract,
			FeeLimitSun:   p.FeeLimitSun,
			ExpirationMs:  p.Expiration,
			TxID:          p.TxIDHex,
		})
	}
	fmt.Fprintf(o.stdout(), "Unsigned transfer written to %s\n", outPath)
	fmt.Fprintf(o.stdout(), "  file SHA-256: %s\n", fp.SHA256Hex)
	fmt.Fprintf(o.stdout(), "  hmac:         %v\n", hmacOn)
	fmt.Fprintf(o.stdout(), "  tx_id:        %s\n", p.TxIDHex)
	return nil
}

func (o *OfflineService) printSignResult(p transfer.SignedTransferPayload, fp transfer.FileFingerprint, outPath string) error {
	if o.cfg.Output == config.OutputJSON {
		return json.NewEncoder(o.stdout()).Encode(signJSONOutput{
			Phase:      "sign",
			OutPath:    outPath,
			FileSHA256: fp.SHA256Hex,
			Chain:      p.Chain,
			TxID:       p.TxIDHex,
		})
	}
	fmt.Fprintf(o.stdout(), "Signed transfer written to %s\n", outPath)
	fmt.Fprintf(o.stdout(), "  file SHA-256: %s\n", fp.SHA256Hex)
	fmt.Fprintf(o.stdout(), "  tx_id:        %s\n", p.TxIDHex)
	return nil
}

func (o *OfflineService) printBroadcastResult(r BroadcastResult) error {
	if o.cfg.Output == config.OutputJSON {
		return json.NewEncoder(o.stdout()).Encode(r)
	}
	fmt.Fprintf(o.stdout(), "Broadcast OK\nNetwork: %s\nTxID:    %s\nFrom:    %s\nTo:      %s\n%s: %s\n",
		r.Network, r.TxID, r.From, r.To, r.Kind, r.Amount)
	return nil
}
