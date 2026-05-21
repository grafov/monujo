package app

import (
	"github.com/grafov/monujo/internal/tron"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WatchService helpers", func() {
	It("marks permission keys that match local keystore entries", func() {
		perms := &tron.AccountPermissions{
			Address: "TAccount",
			Owner: &tron.AccountPermission{
				Type:           tron.PermissionTypeOwner,
				ID:             0,
				PermissionName: "owner",
				Threshold:      1,
				Keys: []tron.PermissionKey{
					{AddressBase58: "TLocal", Weight: 1},
					{AddressBase58: "TRemote", Weight: 1},
				},
			},
		}

		out := permissionsToOutput(perms, map[string]keystoreAddrEntry{
			"TLocal": {Name: "cold", External: false},
		})

		Expect(out.Owner.Keys).To(HaveLen(2))
		Expect(out.Owner.Keys[0].LocalWallet).To(Equal("cold"))
		Expect(out.LocalWalletForKey).To(HaveKeyWithValue("TLocal", "cold"))
		Expect(out.Owner.Keys[1].LocalWallet).To(BeEmpty())
	})
})
