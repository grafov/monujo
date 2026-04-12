// Package domain defines chain-agnostic value types and parsing helpers.
package domain

import (
	"fmt"
	"math/big"
	"strings"
)

// TRXDecimals is sun precision for TRX (1 TRX = 1e6 sun).
const TRXDecimals = 6

// USDTDecimals is TRC-20 USDT decimals on Tron (mainnet and common test tokens).
const USDTDecimals = 6

// ParseTRXDecimal parses a decimal TRX amount string into sun (int64).
// Example: "1" -> 1_000_000 sun. No scientific notation.
func ParseTRXDecimal(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	units, err := parseDecimalToMinimalUnits(s, TRXDecimals)
	if err != nil {
		return 0, fmt.Errorf("trx amount: %w", err)
	}
	if !units.IsInt64() {
		return 0, fmt.Errorf("trx amount overflows int64")
	}
	return units.Int64(), nil
}

// ParseUSDTDecimal parses a decimal USDT amount into minimal units (*big.Int, 6 decimals).
func ParseUSDTDecimal(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty amount")
	}
	return parseDecimalToMinimalUnits(s, USDTDecimals)
}

// parseDecimalToMinimalUnits converts "12.34" with fracLen fractional digits to integer minimal units.
func parseDecimalToMinimalUnits(s string, fracLen int) (*big.Int, error) {
	if strings.HasPrefix(s, "-") {
		return nil, fmt.Errorf("negative amount not allowed")
	}
	parts := strings.Split(s, ".")
	switch len(parts) {
	case 1:
		whole := strings.TrimSpace(parts[0])
		if whole == "" {
			return nil, fmt.Errorf("invalid number")
		}
		for _, c := range whole {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid character in amount")
			}
		}
		w := new(big.Int)
		if _, ok := w.SetString(whole, 10); !ok {
			return nil, fmt.Errorf("invalid integer part")
		}
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(fracLen)), nil)
		return new(big.Int).Mul(w, scale), nil
	case 2:
		whole := strings.TrimSpace(parts[0])
		frac := strings.TrimSpace(parts[1])
		if whole == "" {
			whole = "0"
		}
		for _, c := range whole {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid character in amount")
			}
		}
		for _, c := range frac {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid character in fraction")
			}
		}
		if len(frac) > fracLen {
			return nil, fmt.Errorf("too many fractional digits (max %d)", fracLen)
		}
		frac += strings.Repeat("0", fracLen-len(frac))
		w := new(big.Int)
		if _, ok := w.SetString(whole+frac, 10); !ok {
			return nil, fmt.Errorf("invalid amount")
		}
		return w, nil
	default:
		return nil, fmt.Errorf("multiple decimal points")
	}
}

// FormatMinimalUSDT renders minimal USDT units as decimal string.
func FormatMinimalUSDT(u *big.Int) string {
	if u == nil {
		return "0"
	}
	return formatMinimalUnits(u, USDTDecimals)
}

// FormatSunAsTRX renders sun as TRX decimal string.
func FormatSunAsTRX(sun int64) string {
	return formatMinimalUnits(big.NewInt(sun), TRXDecimals)
}

func formatMinimalUnits(u *big.Int, fracLen int) string {
	if u.Sign() < 0 {
		return "0"
	}
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(fracLen)), nil)
	whole := new(big.Int).Div(u, denom)
	frac := new(big.Int).Mod(u, denom)
	if frac.Sign() == 0 {
		return whole.String()
	}
	fs := fmt.Sprintf("%0*d", fracLen, frac)
	fs = strings.TrimRight(fs, "0")
	return whole.String() + "." + fs
}
