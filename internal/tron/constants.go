// Package tron implements Tron-specific clients (read via HTTP, write via gRPC).
package tron

import (
	"github.com/grafov/monujo/internal/config"
)

// MainnetUSDTContract is mainnet USDT (TRC-20) contract — canonical Tether on
// TRON mainnet.
const MainnetUSDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

// ShastaTestUSDTContract is a commonly used USDT-like token on Shasta for
// integration tests (not official Tether mainnet).
const ShastaTestUSDTContract = "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"

// USDTContractForNetwork returns the USDT (or test USDT) contract for the
// configured network.
func USDTContractForNetwork(n config.Network) string {
	switch n {
	case config.NetworkShasta:
		return ShastaTestUSDTContract
	default:
		return MainnetUSDTContract
	}
}
