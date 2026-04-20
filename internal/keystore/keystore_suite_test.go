package keystore

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKeystore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/keystore suite")
}
