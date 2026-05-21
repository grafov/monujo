// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
