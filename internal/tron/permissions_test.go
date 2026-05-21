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
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fbsobreira/gotron-sdk/pkg/common"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// base58ToHex41 returns the 0x41-prefixed hex representation of a TRON
// base58check address so a test can build a TronGrid-shaped permissions
// fixture without having the node compute the conversion for it.
func base58ToHex41(a string) string {
	GinkgoHelper()
	b, err := common.DecodeCheck(a)
	Expect(err).NotTo(HaveOccurred())
	return hex.EncodeToString(b)
}

var _ = Describe("ReadClient.GetPermissions", func() {
	const addr = "TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9"
	const key1 = "TPL66VK2gCXNCD7EJg9pgJRfqcRazjhUZY"
	const key2 = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

	var (
		ts  *httptest.Server
		mux *http.ServeMux
		cfg *config.Config
		rc  *tron.ReadClient
	)

	BeforeEach(func() {
		mux = http.NewServeMux()
		ts = httptest.NewServer(mux)
		DeferCleanup(ts.Close)
		cfg = &config.Config{
			Network:     config.NetworkMainnet,
			HTTPBaseURL: ts.URL,
			Output:      config.OutputTable,
		}
		rc = tron.NewReadClient(cfg)
	})

	It("parses owner, witness and active permissions, converting hex-41 keys to base58", func() {
		body := fmt.Sprintf(`{
			"data": [{
				"address": "%s",
				"owner_permission": {
					"type": 0, "id": 0, "permission_name": "owner", "threshold": 2,
					"keys": [
						{"address": "%s", "weight": 1},
						{"address": "%s", "weight": 1}
					]
				},
				"witness_permission": {
					"type": 1, "id": 1, "permission_name": "witness", "threshold": 1,
					"keys": [{"address": "%s", "weight": 1}]
				},
				"active_permission": [{
					"type": 2, "id": 2, "permission_name": "active0", "threshold": 2,
					"operations": "7fff1fc00033e01c00000000000000000000000000000000000000000000000000",
					"keys": [
						{"address": "%s", "weight": 1},
						{"address": "%s", "weight": 1}
					]
				}]
			}],
			"success": true
		}`, addr, base58ToHex41(key1), base58ToHex41(key2), base58ToHex41(key1), base58ToHex41(key1), base58ToHex41(key2))

		mux.HandleFunc("/v1/accounts/"+addr, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		})

		out, err := rc.GetPermissions(context.Background(), addr)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Address).To(Equal(addr))
		Expect(out.Owner).NotTo(BeNil())
		Expect(out.Owner.Threshold).To(Equal(int64(2)))
		Expect(out.Owner.Type).To(Equal(tron.PermissionTypeOwner))
		Expect(out.Owner.Keys).To(HaveLen(2))
		Expect(out.Owner.Keys[0].AddressBase58).To(Equal(key1))
		Expect(out.Owner.Keys[1].AddressBase58).To(Equal(key2))

		Expect(out.Witness).NotTo(BeNil())
		Expect(out.Witness.Type).To(Equal(tron.PermissionTypeWitness))

		Expect(out.ActivePermissions).To(HaveLen(1))
		active := out.ActivePermissions[0]
		Expect(active.Type).To(Equal(tron.PermissionTypeActive))
		Expect(active.Operations).NotTo(BeEmpty(),
			"operations bitmap must decode to at least one contract name")
	})

	It("returns an address-only shell for unfunded accounts (empty data)", func() {
		mux.HandleFunc("/v1/accounts/"+addr, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":[],"success":true}`))
		})
		out, err := rc.GetPermissions(context.Background(), addr)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Address).To(Equal(addr))
		Expect(out.Owner).To(BeNil())
		Expect(out.Witness).To(BeNil())
		Expect(out.ActivePermissions).To(BeEmpty())
	})

	It("surfaces TronGrid HTTP errors verbatim", func() {
		mux.HandleFunc("/v1/accounts/"+addr, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream"}`))
		})
		_, err := rc.GetPermissions(context.Background(), addr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("HTTP 502"))
	})

	It("rejects a non-base58 address before any HTTP call", func() {
		_, err := rc.GetPermissions(context.Background(), "not-base58")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("DecodePermissionOperations", func() {
	It("returns nothing for the zero bitmap or empty string", func() {
		for _, s := range []string{"", strings.Repeat("00", 32)} {
			out, err := tron.DecodePermissionOperations(s)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(BeEmpty())
		}
	})

	It("decodes bit 0 of byte 0 to AccountCreateContract", func() {
		hexStr := "01" + strings.Repeat("00", 31)
		out, err := tron.DecodePermissionOperations(hexStr)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ConsistOf("AccountCreateContract"))
	})

	It("decodes mixed bits across two bytes", func() {
		hexStr := "01" + "01" + strings.Repeat("00", 30)
		out, err := tron.DecodePermissionOperations(hexStr)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(HaveLen(2))
	})

	It("rejects non-hex input", func() {
		_, err := tron.DecodePermissionOperations("zz")
		Expect(err).To(HaveOccurred())
	})
})
