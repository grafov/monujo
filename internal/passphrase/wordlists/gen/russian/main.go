// Regenerates russian.txt from upstream Wiktionary-mirrored RNC frequency
// list (Lyashevskaya & Sharov 2009).
//
// Run from repository root:
//
//	go run ./internal/passphrase/wordlists/gen/russian
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
const maxRuneLen = 10

func main() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("runtime.Caller failed")
	}
	wordlistsDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	upPath, err := safeJoin(wordlistsDir, filepath.Join("upstream", "russian_rnc.txt"))
	if err != nil {
		log.Fatal(err)
	}
	outPath, err := safeJoin(wordlistsDir, "russian.txt")
	if err != nil {
		log.Fatal(err)
	}

	rawWords, err := loadLines(upPath)
	if err != nil {
		log.Fatal(err)
	}

	candidates := make(map[string]struct{})
	for _, line := range rawWords {
		w := normalizeCandidate(line)
		if w == "" {
			continue
		}
		candidates[w] = struct{}{}
	}

	final := make([]string, 0, len(candidates))
	for w := range candidates {
		final = append(final, w)
	}
	sort.Strings(final)

	if err := writeLines(outPath, final); err != nil {
		log.Fatal(err)
	}

	n := len(final)
	fmt.Fprintf(os.Stderr, "russian wordlist: %d words, log2(n)=%.6f bits/word\n", n, math.Log2(float64(n)))
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
		if !isRussianLetter(r) {
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

func isRussianLetter(r rune) bool {
	if r >= 'а' && r <= 'я' {
		return true
	}
	if r == 'ё' {
		return true
	}
	return false
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
