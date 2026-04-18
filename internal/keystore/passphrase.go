package keystore

import (
	"fmt"
	"math"
	"os"

	"golang.org/x/term"
)

// ZeroPassphrase overwrites passphrase bytes.
func ZeroPassphrase(p []byte) {
	for i := range p {
		p[i] = 0
	}
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
