package keystore

import (
	"fmt"
	"math"
	"os"

	"golang.org/x/term"
	"golang.org/x/text/unicode/norm"
)

// ZeroPassphrase overwrites passphrase bytes.
func ZeroPassphrase(p []byte) {
	for i := range p {
		p[i] = 0
	}
}

// NormalizePassphrase returns a fresh NFKC-normalized copy of p.
//
// The normalization is applied once, immediately before every password-based
// KDF invocation in monujo (Argon2id for keystore v2, scrypt for legacy v1,
// scrypt for the offline envelope HMAC). NFKC is chosen per
// NIST SP 800-63B §5.1.1.2 "Memorized Secrets" and the OWASP Password
// Storage Cheat Sheet: compatibility decomposition + canonical composition
// maximizes reproducibility between operating systems and input methods
// (macOS NFD, ligature input, full-width / half-width variants).
//
// The returned slice is always a new allocation, so it can be safely zeroed
// with [ZeroPassphrase] without affecting the caller's original buffer.
// The caller remains responsible for wiping the input p.
func NormalizePassphrase(p []byte) []byte {
	out := make([]byte, 0, len(p))
	return norm.NFKC.Append(out, p...)
}

// ReadPassphrase reads a passphrase from the terminal without echo (fallback to stdin if not a TTY).
func ReadPassphrase(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	rawFd := os.Stdin.Fd()
	if rawFd > uintptr(math.MaxInt) {
		return nil, fmt.Errorf("stdin fd out of int range")
	}
	fd := int(rawFd)
	if term.IsTerminal(fd) {
		line, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, err
		}
		return line, nil
	}
	var buf [4096]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}
