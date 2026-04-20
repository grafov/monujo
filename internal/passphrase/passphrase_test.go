package passphrase

import (
	"bytes"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/grafov/monujo/internal/keystore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("passphrase generation", func() {
	It("uses defaults and meets minimum entropy", func() {
		p, bits, err := Generate(Options{Words: DefaultWords, Separator: DefaultSeparator, Dict: DefaultDict})
		Expect(err).NotTo(HaveOccurred())
		defer keystore.ZeroPassphrase(p)

		parts := strings.Split(string(p), DefaultSeparator)
		Expect(parts).To(HaveLen(DefaultWords))
		Expect(bits).To(BeNumerically(">=", MinEntropyBits))
	})

	It("respects word count and separator", func() {
		p, _, err := Generate(Options{Words: 5, Separator: "-", Dict: DictEnglish})
		Expect(err).NotTo(HaveOccurred())
		defer keystore.ZeroPassphrase(p)

		parts := strings.Split(string(p), "-")
		Expect(parts).To(HaveLen(5))
	})

	It("capitalizes first rune in each word when requested", func() {
		for _, id := range []DictID{DictEnglish, DictEsperanto, DictRussian} {
			p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: id, Capitalize: true})
			Expect(err).NotTo(HaveOccurred(), string(id))

			parts := strings.Split(string(p), " ")
			for _, w := range parts {
				r, _ := utf8.DecodeRuneInString(w)
				Expect(unicode.IsUpper(r)).To(BeTrue(), string(id)+": "+w)
			}
			keystore.ZeroPassphrase(p)
		}
	})

	It("appends a digits group when requested", func() {
		p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: DictEnglish, Digits: 5})
		Expect(err).NotTo(HaveOccurred())
		defer keystore.ZeroPassphrase(p)

		parts := strings.Split(string(p), " ")
		Expect(parts).To(HaveLen(5))
		digits := parts[len(parts)-1]
		Expect(digits).To(HaveLen(5))
		Expect(regexp.MustCompile(`^[0-9]+$`).MatchString(digits)).To(BeTrue())
	})

	It("computes entropy according to dictionary size and digit count", func() {
		cases := []struct {
			words  int
			digits int
			dict   DictID
		}{
			{7, 0, DictEnglish},
			{6, 3, DictEnglishShort},
			{8, 0, DictEsperanto},
			{5, 2, DictRussian},
		}

		for _, tc := range cases {
			sz, err := DictSize(tc.dict)
			Expect(err).NotTo(HaveOccurred(), string(tc.dict))

			want := math.Log2(float64(sz))*float64(tc.words) + math.Log2(10)*float64(tc.digits)
			p, got, err := Generate(Options{Words: tc.words, Separator: " ", Dict: tc.dict, Digits: tc.digits})
			Expect(err).NotTo(HaveOccurred())
			keystore.ZeroPassphrase(p)
			Expect(math.Abs(got - want)).To(BeNumerically("<=", 1e-6))
		}
	})

	It("returns output already normalized for all dictionaries", func() {
		for _, id := range KnownDicts() {
			p, _, err := Generate(Options{Words: 4, Separator: " ", Dict: id})
			Expect(err).NotTo(HaveOccurred(), string(id))

			norm := keystore.NormalizePassphrase(p)
			Expect(bytes.Equal(p, norm)).To(BeTrue(), string(id))

			keystore.ZeroPassphrase(norm)
			keystore.ZeroPassphrase(p)
		}
	})

	It("only emits words from the selected dictionary", func() {
		d, err := loadDict(DictEnglish)
		Expect(err).NotTo(HaveOccurred())

		index := make(map[string]struct{}, len(d.words))
		for _, w := range d.words {
			index[w] = struct{}{}
		}

		p, _, err := Generate(Options{Words: 10, Separator: " ", Dict: DictEnglish})
		Expect(err).NotTo(HaveOccurred())
		defer keystore.ZeroPassphrase(p)

		for _, w := range strings.Split(string(p), " ") {
			_, ok := index[w]
			Expect(ok).To(BeTrue(), w)
		}
	})

	It("rejects invalid generation options", func() {
		_, _, err := Generate(Options{Words: 0, Separator: " ", Dict: DictEnglish})
		Expect(err).To(HaveOccurred())

		_, _, err = Generate(Options{Words: 3, Separator: "", Dict: DictEnglish})
		Expect(err).To(HaveOccurred())

		_, _, err = Generate(Options{Words: 3, Separator: " ", Dict: "bogus"})
		Expect(err).To(HaveOccurred())

		_, _, err = Generate(Options{Words: 3, Separator: " ", Dict: DictEnglish, Digits: -1})
		Expect(err).To(HaveOccurred())
	})

	It("returns fresh allocations on consecutive calls", func() {
		p1, _, err := Generate(Options{Words: 5, Separator: " ", Dict: DictEnglish})
		Expect(err).NotTo(HaveOccurred())
		p2, _, err := Generate(Options{Words: 5, Separator: " ", Dict: DictEnglish})
		Expect(err).NotTo(HaveOccurred())

		if len(p1) > 0 && len(p2) > 0 {
			Expect(&p1[0] == &p2[0]).To(BeFalse())
		}

		keystore.ZeroPassphrase(p1)
		keystore.ZeroPassphrase(p2)
	})

	It("keeps sampling distribution reasonably uniform", func() {
		const draws = 20000
		counts := make(map[string]int)
		for i := 0; i < draws; i++ {
			p, _, err := Generate(Options{Words: 1, Separator: " ", Dict: DictEnglishShort})
			Expect(err).NotTo(HaveOccurred())
			counts[string(p)]++
			keystore.ZeroPassphrase(p)
		}

		sz, err := DictSize(DictEnglishShort)
		Expect(err).NotTo(HaveOccurred())

		expected := float64(draws) / float64(sz)
		var maxCount int
		for _, c := range counts {
			if c > maxCount {
				maxCount = c
			}
		}
		Expect(float64(maxCount)).To(BeNumerically("<=", expected*5))

		coverage := float64(len(counts)) / float64(sz)
		Expect(coverage).To(BeNumerically(">=", 0.8))
	})
})

var _ = Describe("dictionary metadata", func() {
	It("parses known dictionary IDs and rejects unknown ones", func() {
		for _, id := range KnownDicts() {
			got, err := ParseDict(string(id))
			Expect(err).NotTo(HaveOccurred(), string(id))
			Expect(got).To(Equal(id))
		}

		_, err := ParseDict("klingon")
		Expect(err).To(HaveOccurred())
	})

	It("returns expected dictionary sizes", func() {
		cases := map[DictID]int{
			DictEnglish:      7776,
			DictEnglishShort: 1296,
			DictEsperantoX:   14851,
			DictEsperanto:    14851,
			DictRussian:      15213,
		}

		for id, want := range cases {
			got, err := DictSize(id)
			Expect(err).NotTo(HaveOccurred(), string(id))
			Expect(got).To(Equal(want), string(id))
		}
	})

	It("keeps hyphenated English words well-formed", func() {
		d, err := loadDict(DictEnglish)
		Expect(err).NotTo(HaveOccurred())

		for _, w := range d.words {
			if strings.ContainsRune(w, '-') {
				Expect(strings.HasPrefix(w, "-")).To(BeFalse(), w)
				Expect(strings.HasSuffix(w, "-")).To(BeFalse(), w)
				Expect(strings.Contains(w, "--")).To(BeFalse(), w)
			}
		}
	})

	It("keeps Russian dictionary within expected rune set", func() {
		d, err := loadDict(DictRussian)
		Expect(err).NotTo(HaveOccurred())

		pattern := regexp.MustCompile(`^[а-яёй]{3,10}$`)
		for _, w := range d.words {
			Expect(pattern.MatchString(w)).To(BeTrue(), w)
		}
	})

	It("keeps Esperanto dictionary within allowed alphabet", func() {
		d, err := loadDict(DictEsperanto)
		Expect(err).NotTo(HaveOccurred())

		allowed := map[rune]struct{}{}
		for _, r := range "abcdefghijklmnopqrstuvwxyzĉĝĥĵŝŭ" {
			allowed[r] = struct{}{}
		}

		for _, w := range d.words {
			for _, r := range w {
				_, ok := allowed[r]
				Expect(ok).To(BeTrue(), w)
			}
		}
	})
})

var _ = Describe("input method warnings", func() {
	It("prints warnings for Esperanto and Russian dictionaries only", func() {
		var buf bytes.Buffer

		PrintInputMethodWarning(&buf, DictEsperanto)
		out := buf.String()
		for _, want := range []string{"Esperanto diacritics", "ĉ ĝ ĥ ĵ ŝ ŭ", "esperanto-x"} {
			Expect(out).To(ContainSubstring(want))
		}

		buf.Reset()
		PrintInputMethodWarning(&buf, DictRussian)
		out = buf.String()
		for _, want := range []string{"Cyrillic", "ё (U+0451)", "й (U+0439)", "ещё", "еще", "Latin lookalikes"} {
			Expect(out).To(ContainSubstring(want))
		}

		for _, id := range []DictID{DictEnglish, DictEnglishShort, DictEsperantoX} {
			buf.Reset()
			PrintInputMethodWarning(&buf, id)
			Expect(buf.Len()).To(Equal(0), string(id))
		}
	})
})
