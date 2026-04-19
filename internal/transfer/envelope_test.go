package transfer

import "testing"

	func TestMarshalUnsignedPayloadRoundtrip(t *testing.T) {
	u := UnsignedTransferPayload{
		Chain:      ChainTronMainnet,
		Kind:       KindTRC20USDT,
		From:       "TFrom",
		To:         "TTo",
		Amount:     "1.0",
		RawDataHex: "0a02abcd",
		TxIDHex:    "0x" + "00000000000000000000000000000000000000000000000000000000000000ab",
	}
	b, err := MarshalUnsignedPayload(u)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalUnsignedPayload(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.From != u.From || got.To != u.To || got.RawDataHex != u.RawDataHex {
		t.Fatalf("mismatch: %+v vs %+v", got, u)
	}
	if got.Version != EnvelopeVersion {
		t.Fatalf("version: %d", got.Version)
	}
}

func TestMarshalUnsignedPayloadMissingFields(t *testing.T) {
	if _, err := MarshalUnsignedPayload(UnsignedTransferPayload{Kind: KindTRX}); err == nil {
		t.Fatal("expected error on missing required fields")
	}
}

func TestUnmarshalUnsignedPayloadBadVersion(t *testing.T) {
	if _, err := UnmarshalUnsignedPayload([]byte(`{"version":99}`)); err == nil {
		t.Fatal("expected error on bad version")
	}
}

func TestMarshalSignedPayloadRoundtrip(t *testing.T) {
	s := SignedTransferPayload{
		Chain:        ChainTronMainnet,
		Kind:         KindTRX,
		From:         "TFrom",
		To:           "TTo",
		Amount:       "1.5",
		RawDataHex:   "0a02abcd",
		SignatureHex: "0x" + "00",
		TxIDHex:      "0xabcd",
	}
	b, err := MarshalSignedPayload(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalSignedPayload(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.SignatureHex != s.SignatureHex || got.RawDataHex != s.RawDataHex {
		t.Fatalf("mismatch: %+v vs %+v", got, s)
	}
}

func TestMarshalSignedPayloadMissingSignature(t *testing.T) {
	if _, err := MarshalSignedPayload(SignedTransferPayload{
		Kind: KindTRX, From: "a", To: "b", Amount: "1", RawDataHex: "0a",
	}); err == nil {
		t.Fatal("expected error on missing signature")
	}
}
