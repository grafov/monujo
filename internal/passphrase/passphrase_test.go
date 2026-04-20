package passphrase

import (
	"bytes"
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/grafov/monujo/internal/keystore"
)

func TestGenerateDefaults(t *testing.T) {
	p, bits, err := Generate(Options{Words: DefaultWords, Separator: DefaultSeparator, Dict: DefaultDict})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer keystore.ZeroPassphrase(p)
	parts := strings.Split(string(p), DefaultSeparator)
	if len(parts) != DefaultWords {
		t.Fatalf("want %d words, got %d (%q)", DefaultWords, len(parts), p)
	}
	if bits < MinEntropyBits {
		t.Fatalf("default entropy below min: %.2f < %.2f", bits, MinEntropyBits)
	}
}

func TestGenerateWordCountAndSeparator(t *testing.T) {
	p, _, err := Generate(Options{Words: 5, Separator: "-", Dict: DictEnglish})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer keystore.ZeroPassphrase(p)
	parts := strings.Split(string(p), "-")
	if len(parts) != 5 {
		t.Fatalf("want 5 words, got %d (%q)", len(parts), p)
	}
}

func TestGenerateCapitalize(t *testing.T) {
	for _, id := range []DictID{DictEnglish, DictEsperanto, DictRussian} {
		p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: id, Capitalize: true})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		parts := strings.Split(string(p), " ")
		for _, w := range parts {
			r, _ := utf8.DecodeRuneInString(w)
			if !unicode.IsUpper(r) {
				t.Fatalf("%s: first rune of %q is not uppercase", id, w)
			}
		}
		keystore.ZeroPassphrase(p)
	}
}

func TestGenerateDigits(t *testing.T) {
	p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: DictEnglish, Digits: 5})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer keystore.ZeroPassphrase(p)
	parts := strings.Split(string(p), " ")
	if len(parts) != 5 {
		t.Fatalf("expected 4 words + 1 digit group, got %d parts (%q)", len(parts), p)
	}
	digits := parts[len(parts)-1]
	if len(digits) != 5 {
		t.Fatalf("digit group length: want 5, got %d", len(digits))
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(digits) {
		t.Fatalf("digit group contains non-digit: %q", digits)
	}
}

func TestGenerateEntropyMath(t *testing.T) {
	cases := []struct {
		words, digits int
		dict          DictID
	}{
		{7, 0, DictEnglish},
		{6, 3, DictEnglishShort},
		{8, 0, DictEsperanto},
		{5, 2, DictRussian},
	}
	for _, tc := range cases {
		sz, err := DictSize(tc.dict)
		if err != nil {
			t.Fatalf("DictSize(%s): %v", tc.dict, err)
		}
		want := math.Log2(float64(sz))*float64(tc.words) + math.Log2(10)*float64(tc.digits)
		p, got, err := Generate(Options{Words: tc.words, Separator: " ", Dict: tc.dict, Digits: tc.digits})
		if err != nil {
			t.Fatalf("Generate %+v: %v", tc, err)
		}
		keystore.ZeroPassphrase(p)
		if math.Abs(got-want) > 1e-6 {
			t.Fatalf("%+v: entropy want %.6f got %.6f (dictSize=%d)", tc, want, got, sz)
		}
	}
}

func TestGenerateNFKCIdempotent(t *testing.T) {
	for _, id := range KnownDicts() {
		p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: id})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		norm := keystore.NormalizePassphrase(p)
		if !bytes.Equal(p, norm) {
			t.Fatalf("%s: Generate output not NFKC-idempotent: %q vs %q", id, p, norm)
		}
		keystore.ZeroPassphrase(norm)
		keystore.ZeroPassphrase(p)
	}
}

func TestGenerateWordsInDict(t *testing.T) {
	d, err := loadDict(DictEnglish)
	if err != nil {
		t.Fatal(err)
	}
	index := make(map[string]struct{}, len(d.words))
	for _, w := range d.words {
		index[w] = struct{}{}
	}
	p, _, err := Generate(Options{Words: 10, Separator: " ", Dict: DictEnglish})
	if err != nil {
		t.Fatal(err)
	}
	defer keystore.ZeroPassphrase(p)
	for _, w := range strings.Split(string(p), " ") {
		if _, ok := index[w]; !ok {
			t.Fatalf("word %q not in dictionary", w)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	if _, _, err := Generate(Options{Words: 0, Separator: " ", Dict: DictEnglish}); err == nil {
		t.Error("expected error for words=0")
	}
	if _, _, err := Generate(Options{Words: 3, Separator: "", Dict: DictEnglish}); err == nil {
		t.Error("expected error for empty separator")
	}
	if _, _, err := Generate(Options{Words: 3, Separator: " ", Dict: "bogus"}); err == nil {
		t.Error("expected error for unknown dict")
	}
	if _, _, err := Generate(Options{Words: 3, Separator: " ", Dict: DictEnglish, Digits: -1}); err == nil {
		t.Error("expected error for negative digits")
	}
}

func TestParseDict(t *testing.T) {
	for _, id := range KnownDicts() {
		got, err := ParseDict(string(id))
		if err != nil {
			t.Errorf("ParseDict(%q): %v", id, err)
		}
		if got != id {
			t.Errorf("ParseDict(%q) = %q", id, got)
		}
	}
	if _, err := ParseDict("klingon"); err == nil {
		t.Error("expected error for unknown dict name")
	}
}

func TestDictSizesAndLoading(t *testing.T) {
	cases := map[DictID]int{
		DictEnglish:      7776,
		DictEnglishShort: 1296,
		DictEsperantoX:   4096,
		DictEsperanto:    4096,
		DictRussian:      4096,
	}
	for id, want := range cases {
		got, err := DictSize(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if got != want {
			t.Errorf("%s: want %d, got %d", id, want, got)
		}
	}
}

func TestDictValidationASCIIHyphen(t *testing.T) {
	d, err := loadDict(DictEnglish)
	if err != nil {
		t.Fatal(err)
	}
	foundHyphen := false
	for _, w := range d.words {
		if strings.ContainsRune(w, '-') {
			foundHyphen = true
			if strings.HasPrefix(w, "-") || strings.HasSuffix(w, "-") {
				t.Errorf("boundary hyphen in %q", w)
			}
			if strings.Contains(w, "--") {
				t.Errorf("adjacent hyphens in %q", w)
			}
		}
	}
	if !foundHyphen {
		t.Log("note: no hyphenated words found (this is not a failure, the EFF list currently has a few)")
	}
}

func TestRussianDictRuneSet(t *testing.T) {
	d, err := loadDict(DictRussian)
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^[а-яёй]{3,7}$`)
	for _, w := range d.words {
		if !pattern.MatchString(w) {
			t.Fatalf("word %q does not match russian regex", w)
		}
	}
}

func TestEsperantoDictRuneSet(t *testing.T) {
	d, err := loadDict(DictEsperanto)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[rune]struct{}{}
	for _, r := range "abcdefghijklmnopqrstuvwxyzĉĝĥĵŝŭ" {
		allowed[r] = struct{}{}
	}
	for _, w := range d.words {
		for _, r := range w {
			if _, ok := allowed[r]; !ok {
				t.Fatalf("word %q has disallowed rune %q", w, r)
			}
		}
	}
}

func TestPrintInputMethodWarning(t *testing.T) {
	var buf bytes.Buffer

	PrintInputMethodWarning(&buf, DictEsperanto)
	out := buf.String()
	for _, want := range []string{"Esperanto diacritics", "ĉ ĝ ĥ ĵ ŝ ŭ", "esperanto-x"} {
		if !strings.Contains(out, want) {
			t.Errorf("esperanto warning missing %q: %s", want, out)
		}
	}

	buf.Reset()
	PrintInputMethodWarning(&buf, DictRussian)
	out = buf.String()
	for _, want := range []string{"Cyrillic", "ё (U+0451)", "й (U+0439)", "ещё", "еще", "Latin lookalikes"} {
		if !strings.Contains(out, want) {
			t.Errorf("russian warning missing %q: %s", want, out)
		}
	}

	for _, id := range []DictID{DictEnglish, DictEnglishShort, DictEsperantoX} {
		buf.Reset()
		PrintInputMethodWarning(&buf, id)
		if buf.Len() != 0 {
			t.Errorf("%s: unexpected warning output: %q", id, buf.String())
		}
	}
}

func TestGenerateReturnsFreshAllocation(t *testing.T) {
	p1, _, err := Generate(Options{Words: 5, Separator: " ", Dict: DictEnglish})
	if err != nil {
		t.Fatal(err)
	}
	p2, _, err := Generate(Options{Words: 5, Separator: " ", Dict: DictEnglish})
	if err != nil {
		t.Fatal(err)
	}
	if len(p1) > 0 && len(p2) > 0 && &p1[0] == &p2[0] {
		t.Fatal("consecutive Generate calls returned aliasing slices")
	}
	keystore.ZeroPassphrase(p1)
	keystore.ZeroPassphrase(p2)
}

func TestGenerateUniformDistributionCheap(t *testing.T) {
	const draws = 20000
	counts := make(map[string]int)
	for i := 0; i < draws; i++ {
		p, _, err := Generate(Options{Words: 1, Separator: " ", Dict: DictEnglishShort})
		if err != nil {
			t.Fatal(err)
		}
		counts[string(p)]++
		keystore.ZeroPassphrase(p)
	}
	// Chi-square would be more rigorous; a coarse sanity check is enough
	// here: with 1296 buckets and 20000 draws the expected count is ~15.4
	// per bucket. We demand that no bucket exceeds 5x expected and that
	// we actually hit at least 80% of the buckets.
	sz, err := DictSize(DictEnglishShort)
	if err != nil {
		t.Fatal(err)
	}
	expected := float64(draws) / float64(sz)
	var maxCount int
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	if float64(maxCount) > expected*5 {
		t.Errorf("non-uniform sampling suspected: max count %d vs expected %.2f", maxCount, expected)
	}
	coverage := float64(len(counts)) / float64(sz)
	if coverage < 0.8 {
		t.Errorf("low dictionary coverage: %.2f (want >= 0.80)", coverage)
	}
}
