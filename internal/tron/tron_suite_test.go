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

package tron_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestTron is the Ginkgo entry point for the internal/tron suite.
//
// Tests are organised as Ginkgo specs to keep behavioural intent close to the
// public API of the package. The suite is split into:
//
//   - black-box specs in [package tron_test] for the public surface
//     (ValidateBase58Address, USDTContractForNetwork, ReadClient, GRPCWriteClient);
//   - white-box specs in [package tron] for unexported helpers
//     (truncate, transactionIDHex);
//   - opt-in integration specs guarded by a build tag and an env switch.
//
// Run with `go tool ginkgo ./internal/tron` or `make test`.
func TestTron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/tron suite")
}
