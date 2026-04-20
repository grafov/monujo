// Package passphrase generates human-readable passphrases from embedded
// word lists using the OS cryptographic RNG.
//
// Design goals and invariants:
//
//   - The only source of randomness is [crypto/rand.Reader]. Word selection
//     uses [crypto/rand.Int] on a big.Int upper bound, which is free from
//     modulo bias.
//   - The generated passphrase is returned as a freshly allocated []byte
//     that is NFKC-normalized (via [keystore.NormalizePassphrase]), matching
//     exactly the bytes that will later be fed to Argon2id. Callers are
//     responsible for wiping the buffer with [keystore.ZeroPassphrase].
//   - The package deliberately avoids the Go string type for the final
//     secret because strings are immutable and cannot be zeroed.
//   - Word lists are embedded at build time and validated on first use.
//
// See docs/06-keystore.md §"Генератор парольных фраз" for threat-model
// discussion and recommendations on dictionary selection.
package passphrase

import (
	"bytes"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/grafov/monujo/internal/keystore"
)

//go:embed wordlists/*.txt
var wordlistsFS embed.FS

// DictID identifies a built-in dictionary via its public flag name.
type DictID string

const (
	DictEnglish      DictID = "english"
	DictEnglishShort DictID = "english-short"
	DictEsperantoX   DictID = "esperanto-x"
	DictEsperanto    DictID = "esperanto"
	DictRussian      DictID = "russian"
)

// Defaults used by the CLI when the user does not override them.
const (
	DefaultWords     = 7
	DefaultSeparator = " "
	DefaultDict      = DictEnglish
)

// MinEntropyBits is the advisory lower bound. A generated passphrase below
// this threshold still succeeds, but the caller (CLI) is expected to print
// a warning. The value follows NIST SP 800-63B §5.1.1.2 guidance for
// CSP-generated secrets and keeps a margin over the 64-bit hard floor.
const MinEntropyBits = 80.0

// Options controls [Generate].
type Options struct {
	// Words is the number of words to draw from the dictionary. Must be
	// >= 1. For realistic security the CLI enforces >= 4; this package
	// does not impose that policy.
	Words int

	// Separator is inserted between words and before the optional digits
	// group. Must be non-empty to keep words visually distinct; a single
	// space is the default.
	Separator string

	// Dict selects the embedded wordlist. Empty string means [DefaultDict].
	Dict DictID

	// Capitalize, when true, uppercases the first rune of every word. The
	// digits group (if any) is unaffected.
	Capitalize bool

	// Digits, when > 0, appends that many random decimal digits as a
	// final group separated from the last word by [Options.Separator].
	// When 0, no digits group is produced.
	Digits int
}

// errors returned by [Generate].
var (
	ErrInvalidWords     = errors.New("passphrase: words must be >= 1")
	ErrInvalidDigits    = errors.New("passphrase: digits must be >= 0")
	ErrEmptySeparator   = errors.New("passphrase: separator must be non-empty")
	ErrUnknownDict      = errors.New("passphrase: unknown dictionary")
	ErrDictUnavailable  = errors.New("passphrase: dictionary is unavailable or failed validation")
	ErrInternalBadIndex = errors.New("passphrase: internal: random index out of range")
)

// Generate produces a fresh passphrase according to opts.
//
// The returned slice is the caller's property: it is a freshly allocated
// []byte, NFKC-normalized, and must be wiped with
// [keystore.ZeroPassphrase] when the caller is done. The returned entropy
// value is measured in bits and is computed from the chosen dictionary
// size and parameters; it does not attempt to model human-perceived
// strength.
func Generate(opts Options) ([]byte, float64, error) {
	if opts.Words < 1 {
		return nil, 0, ErrInvalidWords
	}
	if opts.Digits < 0 {
		return nil, 0, ErrInvalidDigits
	}
	sep := opts.Separator
	if sep == "" {
		return nil, 0, ErrEmptySeparator
	}
	dictID := opts.Dict
	if dictID == "" {
		dictID = DefaultDict
	}
	d, err := loadDict(dictID)
	if err != nil {
		return nil, 0, err
	}

	var buf bytes.Buffer
	buf.Grow(opts.Words*8 + len(sep)*opts.Words + opts.Digits)

	for i := 0; i < opts.Words; i++ {
		if i > 0 {
			buf.WriteString(sep)
		}
		w, err := pickWord(d)
		if err != nil {
			zeroBuffer(&buf)
			return nil, 0, err
		}
		if opts.Capitalize {
			w = capitalizeFirst(w)
		}
		buf.WriteString(w)
	}

	if opts.Digits > 0 {
		buf.WriteString(sep)
		if err := appendDigits(&buf, opts.Digits); err != nil {
			zeroBuffer(&buf)
			return nil, 0, err
		}
	}

	// Copy to a fresh []byte, then NFKC-normalize. The temporary buffer
	// is zeroed before it goes back to GC so no plaintext lingers in
	// the bytes.Buffer's internal slice.
	raw := make([]byte, buf.Len())
	copy(raw, buf.Bytes())
	zeroBuffer(&buf)

	out := keystore.NormalizePassphrase(raw)
	keystore.ZeroPassphrase(raw)

	bits := entropyBits(opts, len(d.words))
	return out, bits, nil
}

// entropyBits computes the advisory entropy estimate: log2(N)*W for the
// word part, plus log2(10)*Digits for the digits group. Capitalization
// and separators contribute zero (they are deterministic given opts).
func entropyBits(opts Options, dictSize int) float64 {
	if dictSize <= 0 || opts.Words <= 0 {
		return 0
	}
	bits := math.Log2(float64(dictSize)) * float64(opts.Words)
	if opts.Digits > 0 {
		bits += math.Log2(10) * float64(opts.Digits)
	}
	return bits
}

func pickWord(d *dict) (string, error) {
	n := big.NewInt(int64(len(d.words)))
	idxBig, err := rand.Int(rand.Reader, n)
	if err != nil {
		return "", fmt.Errorf("passphrase: crypto/rand failure: %w", err)
	}
	idx := idxBig.Int64()
	if idx < 0 || idx >= int64(len(d.words)) {
		return "", ErrInternalBadIndex
	}
	return d.words[idx], nil
}

func appendDigits(w io.ByteWriter, n int) error {
	ten := big.NewInt(10)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return fmt.Errorf("passphrase: crypto/rand failure: %w", err)
		}
		d := v.Int64()
		if d < 0 || d > 9 {
			return ErrInternalBadIndex
		}
		digit := byte('0') + byte(d)
		if err := w.WriteByte(digit); err != nil {
			return err
		}
	}
	return nil
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	upper := unicode.ToUpper(r)
	if upper == r {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + utf8.UTFMax)
	b.WriteRune(upper)
	b.WriteString(s[size:])
	return b.String()
}

func zeroBuffer(b *bytes.Buffer) {
	raw := b.Bytes()
	for i := range raw {
		raw[i] = 0
	}
	b.Reset()
}

// ---- Dictionary loading & validation --------------------------------------

type dict struct {
	id    DictID
	words []string
}

type dictEntry struct {
	file      string
	asciiOnly bool
	// allowedRuneSet is nil for ASCII-only lists; for UTF-8 lists it is
	// the set of permitted runes (lowercase letters of the target script
	// plus any language-specific extras like ё/й).
	allowedRuneSet map[rune]struct{}
}

var dictRegistry = map[DictID]dictEntry{
	DictEnglish:      {file: "wordlists/english.txt", asciiOnly: true},
	DictEnglishShort: {file: "wordlists/english_short.txt", asciiOnly: true},
	DictEsperantoX:   {file: "wordlists/esperanto_x.txt", asciiOnly: true},
	DictEsperanto: {
		file: "wordlists/esperanto.txt",
		allowedRuneSet: runeSet(
			"abcdefghijklmnopqrstuvwxyzĉĝĥĵŝŭ",
		),
	},
	DictRussian: {
		file: "wordlists/russian.txt",
		allowedRuneSet: runeSet(
			"абвгдеёжзийклмнопрстуфхцчшщъыьэюя",
		),
	},
}

func runeSet(s string) map[rune]struct{} {
	m := make(map[rune]struct{}, utf8.RuneCountInString(s))
	for _, r := range s {
		m[r] = struct{}{}
	}
	return m
}

// KnownDicts returns the dictionary IDs in a stable, user-facing order.
func KnownDicts() []DictID {
	return []DictID{
		DictEnglish,
		DictEnglishShort,
		DictEsperantoX,
		DictEsperanto,
		DictRussian,
	}
}

var (
	dictCacheMu sync.Mutex
	dictCache   = map[DictID]*dict{}
)

func loadDict(id DictID) (*dict, error) {
	dictCacheMu.Lock()
	defer dictCacheMu.Unlock()
	if d, ok := dictCache[id]; ok {
		return d, nil
	}
	entry, ok := dictRegistry[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownDict, id)
	}
	data, err := wordlistsFS.ReadFile(entry.file)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDictUnavailable, entry.file, err)
	}
	words, err := parseWordlist(data, entry)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDictUnavailable, entry.file, err)
	}
	d := &dict{id: id, words: words}
	dictCache[id] = d
	return d, nil
}

const (
	minWordLen = 3
	maxWordLen = 12
	minDictLen = 1296
)

func parseWordlist(data []byte, entry dictEntry) ([]string, error) {
	// Reject BOM; parse LF-terminated lines. We do not trim trailing
	// whitespace on each line because embedded whitespace would be a
	// data-quality bug that should surface loudly.
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return nil, errors.New("unexpected UTF-8 BOM")
	}
	seen := make(map[string]struct{})
	var words []string
	for _, rawLine := range bytes.Split(data, []byte{'\n'}) {
		line := bytes.TrimRight(rawLine, "\r")
		if len(line) == 0 {
			continue
		}
		w := string(line)
		if _, dup := seen[w]; dup {
			return nil, fmt.Errorf("duplicate word %q", w)
		}
		runeLen := utf8.RuneCountInString(w)
		if runeLen < minWordLen || runeLen > maxWordLen {
			return nil, fmt.Errorf("word %q has length %d outside [%d,%d]", w, runeLen, minWordLen, maxWordLen)
		}
		if entry.asciiOnly {
			// ASCII lowercase with optional inner hyphens (e.g. EFF long
			// contains "drop-down", "felt-tip", "t-shirt", "yo-yo").
			// First and last rune must be a letter; no adjacent hyphens.
			for i := 0; i < len(w); i++ {
				c := w[i]
				switch {
				case c >= 'a' && c <= 'z':
					// ok
				case c == '-':
					if i == 0 || i == len(w)-1 {
						return nil, fmt.Errorf("word %q has hyphen at boundary", w)
					}
					if w[i-1] == '-' {
						return nil, fmt.Errorf("word %q has adjacent hyphens", w)
					}
				default:
					return nil, fmt.Errorf("word %q is not ASCII lowercase/hyphen", w)
				}
			}
		} else {
			if !utf8.ValidString(w) {
				return nil, fmt.Errorf("word %q is not valid UTF-8", w)
			}
			normalized := keystore.NormalizePassphrase([]byte(w))
			if !bytes.Equal(normalized, []byte(w)) {
				keystore.ZeroPassphrase(normalized)
				return nil, fmt.Errorf("word %q is not NFKC-idempotent", w)
			}
			keystore.ZeroPassphrase(normalized)
			for _, r := range w {
				if _, ok := entry.allowedRuneSet[r]; !ok {
					return nil, fmt.Errorf("word %q contains disallowed rune %q", w, r)
				}
			}
		}
		seen[w] = struct{}{}
		words = append(words, w)
	}
	if len(words) < minDictLen {
		return nil, fmt.Errorf("dictionary too small: %d words, need >= %d", len(words), minDictLen)
	}
	return words, nil
}

// PrintInputMethodWarning prints a stderr warning when the selected
// dictionary uses non-ASCII characters that require a specific input
// method. It is a no-op for ASCII-only dictionaries. The text is
// deliberately stable (the tests pin it) so CLI users can script around
// it if needed.
func PrintInputMethodWarning(w io.Writer, id DictID) {
	switch id {
	case DictEsperanto:
		fmt.Fprintln(w, `WARNING: the "esperanto" wordlist uses Esperanto diacritics (ĉ ĝ ĥ ĵ ŝ ŭ).`)
		fmt.Fprintln(w, "You MUST be able to type these characters on EVERY machine where this")
		fmt.Fprintln(w, "wallet will be unlocked. If you only have a plain QWERTY keyboard,")
		fmt.Fprintln(w, "choose --dict esperanto-x instead (x-system, ASCII-only).")
	case DictRussian:
		fmt.Fprintln(w, `WARNING: the "russian" wordlist uses Cyrillic (UTF-8), including the`)
		fmt.Fprintln(w, "letters ё (U+0451) and й (U+0439).")
		fmt.Fprintln(w, "  - You MUST have a Cyrillic input method (раскладка/IME) on EVERY")
		fmt.Fprintln(w, "    machine where this wallet will be unlocked.")
		fmt.Fprintln(w, "  - Pay attention to ё/е and й/и when typing: NFKC treats these as")
		fmt.Fprintln(w, `    DIFFERENT letters, so "ещё" and "еще" are distinct passphrases.`)
		fmt.Fprintln(w, "  - Do NOT mix Latin lookalikes (a/а, o/о, e/е) — a single wrong")
		fmt.Fprintln(w, "    codepoint invalidates the passphrase irrecoverably.")
	default:
		// no-op for ASCII dictionaries
	}
}

// ParseDict turns a CLI flag value into a [DictID], validating it against
// the registry. Returns [ErrUnknownDict] for unrecognized names.
func ParseDict(s string) (DictID, error) {
	id := DictID(s)
	if _, ok := dictRegistry[id]; !ok {
		return "", fmt.Errorf("%w: %q (known: %v)", ErrUnknownDict, s, KnownDicts())
	}
	return id, nil
}

// DictSize returns the word count for id. Intended for entropy tables in
// documentation and CLI "--help" hints. Returns 0 with an error for
// unknown or unloadable dictionaries.
func DictSize(id DictID) (int, error) {
	d, err := loadDict(id)
	if err != nil {
		return 0, err
	}
	return len(d.words), nil
}
