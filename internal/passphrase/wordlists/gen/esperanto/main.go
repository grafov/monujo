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

// Regenerates esperanto.txt and esperanto_x.txt from upstream Wikipedia 2023
// frequency lemmas plus the current esperanto.txt (Tekstaro merge).
//
// Run from repository root:
//
//	go run ./internal/passphrase/wordlists/gen/esperanto
package main

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/grafov/monujo/internal/keystore"
	"golang.org/x/text/unicode/norm"
)

const minRuneLen = 3
const maxRuneLen = 12

func main() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("runtime.Caller failed")
	}
	wordlistsDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	upPath, err := safeJoin(wordlistsDir, filepath.Join("upstream", "esperanto_wikipedia_2023.txt"))
	if err != nil {
		log.Fatal(err)
	}
	existingPath, err := safeJoin(wordlistsDir, "esperanto.txt")
	if err != nil {
		log.Fatal(err)
	}
	outDiac, err := safeJoin(wordlistsDir, "esperanto.txt")
	if err != nil {
		log.Fatal(err)
	}
	outX, err := safeJoin(wordlistsDir, "esperanto_x.txt")
	if err != nil {
		log.Fatal(err)
	}

	rawWords, err := loadLines(upPath)
	if err != nil {
		log.Fatal(err)
	}
	existing, err := loadLines(existingPath)
	if err != nil {
		log.Fatal(err)
	}
	rawWords = append(rawWords, existing...)

	candidates := make(map[string]struct{})
	for _, line := range rawWords {
		w := normalizeCandidate(line)
		if w == "" {
			continue
		}
		candidates[w] = struct{}{}
	}

	// Resolve x-system collisions: keep lexicographically smallest diacritic lemma per x-form.
	byX := make(map[string]string)
	for w := range candidates {
		xw := toXSystem(w)
		if utf8.RuneCountInString(xw) > maxRuneLen {
			continue
		}
		prev, ok := byX[xw]
		if !ok || w < prev {
			byX[xw] = w
		}
	}

	final := make([]string, 0, len(byX))
	for _, w := range byX {
		final = append(final, w)
	}
	sort.Strings(final)

	if err := writeLines(outDiac, final); err != nil {
		log.Fatal(err)
	}
	xList := make([]string, len(final))
	for i, w := range final {
		xList[i] = toXSystem(w)
	}
	if err := writeLines(outX, xList); err != nil {
		log.Fatal(err)
	}

	n := len(final)
	fmt.Fprintf(os.Stderr, "esperanto wordlists: %d words, log2(n)=%.6f bits/word\n", n, math.Log2(float64(n)))
}

// safeJoin returns filepath.Join(root, rel) only if the result stays under root.
func safeJoin(root, rel string) (string, error) {
	root = filepath.Clean(root)
	joined := filepath.Clean(filepath.Join(root, rel))
	r, err := filepath.Rel(root, joined)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", rel, root)
	}
	return joined, nil
}

func loadLines(path string) ([]string, error) {
	// #nosec G304 -- path is always produced by [safeJoin] under wordlistsDir.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, raw := range bytes.Split(data, []byte{'\n'}) {
		line := string(bytes.TrimSpace(raw))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func normalizeCandidate(in string) string {
	w := strings.ToLower(strings.TrimSpace(in))
	if !utf8.ValidString(w) {
		return ""
	}
	w = string(norm.NFC.Bytes([]byte(w)))
	if !utf8.ValidString(w) {
		return ""
	}
	n := utf8.RuneCountInString(w)
	if n < minRuneLen || n > maxRuneLen {
		return ""
	}
	for _, r := range w {
		if !isEsperantoLetter(r) {
			return ""
		}
	}
	nfk := keystore.NormalizePassphrase([]byte(w))
	if !bytes.Equal(nfk, []byte(w)) {
		keystore.ZeroPassphrase(nfk)
		return ""
	}
	keystore.ZeroPassphrase(nfk)
	return w
}

func isEsperantoLetter(r rune) bool {
	switch r {
	case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z':
		return true
	case 'ĉ', 'ĝ', 'ĥ', 'ĵ', 'ŝ', 'ŭ':
		return true
	default:
		return false
	}
}

func toXSystem(w string) string {
	var b strings.Builder
	b.Grow(len(w) + 8)
	for _, r := range w {
		switch r {
		case 'ĉ':
			b.WriteString("cx")
		case 'ĝ':
			b.WriteString("gx")
		case 'ĥ':
			b.WriteString("hx")
		case 'ĵ':
			b.WriteString("jx")
		case 'ŝ':
			b.WriteString("sx")
		case 'ŭ':
			b.WriteString("ux")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeLines(path string, lines []string) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	// #nosec G306 -- public wordlist committed to the repo, not secret material.
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
