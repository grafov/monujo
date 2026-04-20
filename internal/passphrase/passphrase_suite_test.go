package passphrase

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPassphrase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/passphrase suite")
}
