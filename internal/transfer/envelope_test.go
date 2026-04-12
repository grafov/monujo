package transfer

import "testing"

func TestMarshalUnsignedRoundtrip(t *testing.T) {
	u := UnsignedTransferRequest{
		Version: EnvelopeVersion,
		Chain:   ChainTronMainnet,
		Kind:    "trc20_usdt",
		From:    "TFrom",
		To:      "TTo",
		Amount:  "1.0",
	}
	b, err := MarshalUnsigned(u)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := UnmarshalUnsigned(b)
	if err != nil {
		t.Fatal(err)
	}
	if u2.From != u.From || u2.To != u.To {
		t.Fatal("mismatch")
	}
}
