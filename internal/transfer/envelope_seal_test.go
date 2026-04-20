package transfer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSealOpenRoundtripNoHMAC(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	blob, err := SealPayload(KindUnsignedTransfer, payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, fp, err := OpenEnvelope(blob, KindUnsignedTransfer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: %q vs %q", got, payload)
	}
	if len(fp.SHA256Hex) != 64 {
		t.Fatalf("fingerprint hex length: %d", len(fp.SHA256Hex))
	}
}

func TestSealOpenRoundtripWithHMAC(t *testing.T) {
	key, err := DeriveHMACKey([]byte("a strong passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"k":"v"}`)
	blob, err := SealPayload(KindSignedTransfer, payload, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := OpenEnvelope(blob, KindSignedTransfer, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}
}

func TestOpenWrongKind(t *testing.T) {
	blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenEnvelope(blob, KindSignedTransfer, nil); err == nil {
		t.Fatal("expected error on kind mismatch")
	}
}

func TestOpenTamperedPayloadWithHMAC(t *testing.T) {
	key, err := DeriveHMACKey([]byte("p1"))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := SealPayload(KindUnsignedTransfer, []byte(`{"a":1}`), key)
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatal(err)
	}
	tampered := []byte(strings.Replace(string(blob), env.PayloadB64,
		base64.StdEncoding.EncodeToString([]byte(`{"a":2}`)), 1))
	_, _, err = OpenEnvelope(tampered, KindUnsignedTransfer, key)
	if !errors.Is(err, ErrHMACMismatch) {
		t.Fatalf("want ErrHMACMismatch, got %v", err)
	}
}

func TestOpenWithWrongPassphrase(t *testing.T) {
	good, err := DeriveHMACKey([]byte("right"))
	if err != nil {
		t.Fatal(err)
	}
	bad, err := DeriveHMACKey([]byte("wrong"))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), good)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = OpenEnvelope(blob, KindUnsignedTransfer, bad)
	if !errors.Is(err, ErrHMACMismatch) {
		t.Fatalf("want ErrHMACMismatch, got %v", err)
	}
}

func TestOpenHMACPresentButNoKey(t *testing.T) {
	key, err := DeriveHMACKey([]byte("k"))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), key)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = OpenEnvelope(blob, KindUnsignedTransfer, nil)
	if !errors.Is(err, ErrHMACRequired) {
		t.Fatalf("want ErrHMACRequired, got %v", err)
	}
}

func TestOpenBadEnvelope(t *testing.T) {
	if _, _, err := OpenEnvelope([]byte("not json"), KindUnsignedTransfer, nil); err == nil {
		t.Fatal("expected json parse error")
	}
	bad := []byte(`{"monujo":"other","kind":"unsigned_transfer","version":1,"payload_b64":"YQ=="}`)
	if _, _, err := OpenEnvelope(bad, KindUnsignedTransfer, nil); err == nil {
		t.Fatal("expected magic mismatch error")
	}
}

func TestDeriveHMACKeyEmpty(t *testing.T) {
	if _, err := DeriveHMACKey(nil); err == nil {
		t.Fatal("expected error on empty passphrase")
	}
}

func TestDeriveHMACKeyNormalizedAcrossForms(t *testing.T) {
	nfd, err := DeriveHMACKey([]byte("cafe\u0301"))
	if err != nil {
		t.Fatalf("nfd: %v", err)
	}
	nfc, err := DeriveHMACKey([]byte("caf\u00e9"))
	if err != nil {
		t.Fatalf("nfc: %v", err)
	}
	if !bytes.Equal(nfd, nfc) {
		t.Fatalf("NFD and NFC forms of same passphrase must yield the same HMAC key: nfd=%x nfc=%x", nfd, nfc)
	}

	lig, err := DeriveHMACKey([]byte("\uFB01ne"))
	if err != nil {
		t.Fatalf("ligature: %v", err)
	}
	plain, err := DeriveHMACKey([]byte("fine"))
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	if !bytes.Equal(lig, plain) {
		t.Fatalf("ligature and ASCII forms must yield the same HMAC key (NFKC compat): lig=%x plain=%x", lig, plain)
	}
}

func TestSealOpenPassphraseNormalizedAcrossForms(t *testing.T) {
	sealKey, err := DeriveHMACKey([]byte("cafe\u0301-offline"))
	if err != nil {
		t.Fatal(err)
	}
	openKey, err := DeriveHMACKey([]byte("caf\u00e9-offline"))
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("hello")
	env, err := SealPayload(KindUnsignedTransfer, payload, sealKey)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := OpenEnvelope(env, KindUnsignedTransfer, openKey)
	if err != nil {
		t.Fatalf("OpenEnvelope with NFC passphrase must unseal NFD-sealed envelope: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: want %q, got %q", payload, got)
	}
}

