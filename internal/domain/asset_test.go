package domain

import (
	"math/big"
	"testing"
)

func TestParseUSDTDecimal(t *testing.T) {
	v, err := ParseUSDTDecimal("1.5")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "1500000" {
		t.Fatalf("got %s", v.String())
	}
}

func TestParseTRXDecimal(t *testing.T) {
	sun, err := ParseTRXDecimal("2.5")
	if err != nil {
		t.Fatal(err)
	}
	if sun != 2500000 {
		t.Fatalf("got %d", sun)
	}
}

func TestFormatMinimalUSDT(t *testing.T) {
	s := FormatMinimalUSDT(big.NewInt(1500000))
	if s != "1.5" {
		t.Fatalf("got %q", s)
	}
}
