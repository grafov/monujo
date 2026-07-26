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

package tron

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const testRealUnsignedTxJSON = `{
  "visible": false,
  "txID": "ae80c7aa55e19c2da5e712d4be50e4c9422dd6bfc99039cc42d5deb76938c0e7",
  "raw_data": {
    "contract": [{"parameter": {"value": {"amount": 1000, "owner_address": "41eca9bc828a3005b9a3b909f2cc5c2a54794de05f", "to_address": "41495511a493d8c362be4267224e6d81013a6862ee"}, "type_url": "type.googleapis.com/protocol.TransferContract"}, "type": "TransferContract"}],
    "ref_block_bytes": "8acb",
    "ref_block_hash": "7c571ea3e7e9dbf3",
    "expiration": 1773182925000,
    "timestamp": 1773182865805
  },
  "raw_data_hex": "0a028acb22087c571ea3e7e9dbf340c8f1c0cfcd335a66080112620a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412310a1541eca9bc828a3005b9a3b909f2cc5c2a54794de05f121541495511a493d8c362be4267224e6d81013a6862ee18e807708da3bdcfcd33"
}`

const testRealSignedTxJSON = `{
  "ret": [{"contractRet": "SUCCESS"}],
  "signature": [
    "1788bf0281f67c42f5a09322523edee9969e808a863776f21ab6543de9dd42ca0e4955577879df8074086c620eb96709ba9a95d60a166de190a081e8d69801da00"
  ],
  "txID": "c70b20da8cb8d96a99c1aceb283412acb9dea313a9e467178431a3c6ae8b9f08",
  "raw_data": {
    "contract": [{"parameter": {"value": {"data": "a9059cbb00000000000000000000004150125bea243a640bf85348873b6c4f3c517aee7a0000000000000000000000000000000000000000000000000000000000000001", "owner_address": "41495511a493d8c362be4267224e6d81013a6862ee", "contract_address": "41eca9bc828a3005b9a3b909f2cc5c2a54794de05f"}, "type_url": "type.googleapis.com/protocol.TriggerSmartContract"}, "type": "TriggerSmartContract"}],
    "ref_block_bytes": "8ad9",
    "ref_block_hash": "eaef69105495eb15",
    "expiration": 1773182914544,
    "fee_limit": 100000000,
    "timestamp": 1773182854544
  },
  "raw_data_hex": "0a028ad92208eaef69105495eb1540f09fc0cfcd335aae01081f12a9010a31747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e54726967676572536d617274436f6e747261637412740a1541495511a493d8c362be4267224e6d81013a6862ee121541eca9bc828a3005b9a3b909f2cc5c2a54794de05f2244a9059cbb00000000000000000000004150125bea243a640bf85348873b6c4f3c517aee7a00000000000000000000000000000000000000000000000000000000000000017090cbbccfcd33900180c2d72f"
}`

var _ = Describe("transactionToHTTPJSON", func() {
	It("correctly serializes a TransferContract transaction to HTTP JSON format", func() {
		tx, err := transaction.FromJSON([]byte(testRealUnsignedTxJSON))
		Expect(err).NotTo(HaveOccurred())

		res, err := transactionToHTTPJSON(tx)
		Expect(err).NotTo(HaveOccurred())

		Expect(res["visible"]).To(BeTrue())
		Expect(res["txID"]).To(Equal("ae80c7aa55e19c2da5e712d4be50e4c9422dd6bfc99039cc42d5deb76938c0e7"))
		Expect(res["raw_data_hex"]).To(Equal("0a028acb22087c571ea3e7e9dbf340c8f1c0cfcd335a66080112620a2d747970652e676f6f676c65617069732e636f6d2f70726f746f636f6c2e5472616e73666572436f6e747261637412310a1541eca9bc828a3005b9a3b909f2cc5c2a54794de05f121541495511a493d8c362be4267224e6d81013a6862ee18e807708da3bdcfcd33"))
		Expect(res["signature"]).To(BeEmpty())

		rawData, ok := res["raw_data"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(rawData["ref_block_bytes"]).To(Equal("8acb"))
		Expect(rawData["ref_block_hash"]).To(Equal("7c571ea3e7e9dbf3"))
		Expect(rawData["expiration"]).To(Equal(int64(1773182925000)))
		Expect(rawData["timestamp"]).To(Equal(int64(1773182865805)))

		contracts, ok := rawData["contract"].([]map[string]any)
		Expect(ok).To(BeTrue())
		Expect(contracts).To(HaveLen(1))

		c := contracts[0]
		Expect(c["type"]).To(Equal("TransferContract"))

		param, ok := c["parameter"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(param["type_url"]).To(Equal("type.googleapis.com/protocol.TransferContract"))

		val, ok := param["value"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(val["amount"]).To(Equal(int64(1000)))
		Expect(val["owner_address"]).To(Equal("TXYZopYRdj2D9XRtbG411XZZ3kM5VkAeBf"))
		Expect(val["to_address"]).To(Equal("TGexEkipwwnskd2rDXWvptwLLhjCTcNeCi"))
	})

	It("correctly serializes a TriggerSmartContract transaction to HTTP JSON format", func() {
		tx, err := transaction.FromJSON([]byte(testRealSignedTxJSON))
		Expect(err).NotTo(HaveOccurred())

		res, err := transactionToHTTPJSON(tx)
		Expect(err).NotTo(HaveOccurred())

		Expect(res["visible"]).To(BeTrue())
		Expect(res["txID"]).To(Equal("c70b20da8cb8d96a99c1aceb283412acb9dea313a9e467178431a3c6ae8b9f08"))
		Expect(res["signature"]).To(ConsistOf("1788bf0281f67c42f5a09322523edee9969e808a863776f21ab6543de9dd42ca0e4955577879df8074086c620eb96709ba9a95d60a166de190a081e8d69801da00"))

		rawData, ok := res["raw_data"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(rawData["fee_limit"]).To(Equal(int64(100000000)))

		contracts, ok := rawData["contract"].([]map[string]any)
		Expect(ok).To(BeTrue())
		Expect(contracts).To(HaveLen(1))

		c := contracts[0]
		Expect(c["type"]).To(Equal("TriggerSmartContract"))

		param, ok := c["parameter"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(param["type_url"]).To(Equal("type.googleapis.com/protocol.TriggerSmartContract"))

		val, ok := param["value"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(val["owner_address"]).To(Equal("TGexEkipwwnskd2rDXWvptwLLhjCTcNeCi"))
		Expect(val["contract_address"]).To(Equal("TXYZopYRdj2D9XRtbG411XZZ3kM5VkAeBf"))
		Expect(val["data"]).To(Equal("a9059cbb00000000000000000000004150125bea243a640bf85348873b6c4f3c517aee7a0000000000000000000000000000000000000000000000000000000000000001"))
	})

	It("includes Permission_id when non-zero", func() {
		tc := &core.TransferContract{}
		tcBytes, err := proto.Marshal(tc)
		Expect(err).NotTo(HaveOccurred())

		tx := &core.Transaction{
			RawData: &core.TransactionRaw{
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_TransferContract,
						Parameter: &anypb.Any{
							TypeUrl: "type.googleapis.com/protocol.TransferContract",
							Value:   tcBytes,
						},
						PermissionId: 4,
					},
				},
			},
		}

		res, err := transactionToHTTPJSON(tx)
		Expect(err).NotTo(HaveOccurred())

		rawData := res["raw_data"].(map[string]any)
		contracts := rawData["contract"].([]map[string]any)
		Expect(contracts).To(HaveLen(1))
		Expect(contracts[0]["Permission_id"]).To(Equal(int32(4)))
	})

	It("returns an error for unsupported contract types", func() {
		vw := &core.VoteWitnessContract{}
		vwBytes, err := proto.Marshal(vw)
		Expect(err).NotTo(HaveOccurred())

		tx := &core.Transaction{
			RawData: &core.TransactionRaw{
				Contract: []*core.Transaction_Contract{
					{
						Type: core.Transaction_Contract_VoteWitnessContract,
						Parameter: &anypb.Any{
							TypeUrl: "type.googleapis.com/protocol.VoteWitnessContract",
							Value:   vwBytes,
						},
					},
				},
			},
		}

		_, err = transactionToHTTPJSON(tx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported contract type"))
	})
})
