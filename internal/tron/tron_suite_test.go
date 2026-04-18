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
//     (ValidateBase58Address, USDTContractForNetwork, ReadClient, WriteClient);
//   - white-box specs in [package tron] for unexported helpers
//     (truncate, transactionIDHex);
//   - opt-in integration specs guarded by a build tag and an env switch.
//
// Run with `go tool ginkgo ./internal/tron` or `make test`.
func TestTron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/tron suite")
}
