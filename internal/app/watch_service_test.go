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
