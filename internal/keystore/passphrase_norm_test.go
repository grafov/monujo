package keystore

import (
	"bytes"
	"testing"
)

func TestNormalizePassphraseASCIIIdempotent(t *testing.T) {
	in := []byte("hello-world-123")
	out := NormalizePassphrase(in)
	if !bytes.Equal(in, out) {
		t.Fatalf("ASCII input must be unchanged: in=%q out=%q", in, out)
	}
	if len(in) > 0 && len(out) > 0 && &in[0] == &out[0] {
		t.Fatal("NormalizePassphrase must return a fresh allocation, not alias input")
	}

	ZeroPassphrase(out)
	if !bytes.Equal(in, []byte("hello-world-123")) {
		t.Fatalf("zeroing output must not affect original: in=%q", in)
	}
}

func TestNormalizePassphraseNFDtoNFC(t *testing.T) {
	nfd := []byte("cafe\u0301")
	nfc := []byte("caf\u00e9")
	got := NormalizePassphrase(nfd)
	if !bytes.Equal(got, nfc) {
		t.Fatalf("NFD→NFC: want %q (%x), got %q (%x)", nfc, nfc, got, got)
	}
}

func TestNormalizePassphraseLigature(t *testing.T) {
	lig := []byte("\uFB01ne")
	want := []byte("fine")
	got := NormalizePassphrase(lig)
	if !bytes.Equal(got, want) {
		t.Fatalf("ligature fi should decompose: want %q (%x), got %q (%x)", want, want, got, got)
	}
}

func TestNormalizePassphraseFullwidth(t *testing.T) {
	full := []byte("\uFF50\uFF41\uFF53\uFF53")
	want := []byte("pass")
	got := NormalizePassphrase(full)
	if !bytes.Equal(got, want) {
		t.Fatalf("full-width → half-width: want %q (%x), got %q (%x)", want, want, got, got)
	}
}

func TestNormalizePassphraseEmpty(t *testing.T) {
	got := NormalizePassphrase(nil)
	if got == nil {
		t.Fatal("must return non-nil slice (even if empty) to keep zeroing logic uniform")
	}
	if len(got) != 0 {
		t.Fatalf("empty input must produce empty output, got %q", got)
	}
}
