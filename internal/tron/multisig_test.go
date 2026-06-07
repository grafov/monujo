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
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// buildMultisigRawDataHex constructs a raw_data payload representing a
// TRX TransferContract with the given permission id. The returned hex is
// suitable for [tron.AppendMultisigSignature] and round-trips through
// [tron.DecodeTronJSON].
func buildMultisigRawDataHex(permissionID int32, amountSun int64) string {
	GinkgoHelper()
	from := mustDecodeBase58(testFromAddr)
	to := mustDecodeBase58(testToAddr)
	param := &core.TransferContract{
		OwnerAddress: from,
		ToAddress:    to,
		Amount:       amountSun,
	}
	anyParam, err := proto.Marshal(param)
	Expect(err).NotTo(HaveOccurred())
	raw := &core.TransactionRaw{
		RefBlockBytes: []byte{0x12, 0x34},
		Expiration:    time.Now().Add(time.Minute).UnixMilli(),
		Timestamp:     time.Now().UnixMilli(),
		Contract: []*core.Transaction_Contract{
			{
				Type: core.Transaction_Contract_TransferContract,
				Parameter: &anypb.Any{
					TypeUrl: "type.googleapis.com/protocol.TransferContract",
					Value:   anyParam,
				},
				PermissionId: permissionID,
			},
		},
	}
	b, err := proto.Marshal(raw)
	Expect(err).NotTo(HaveOccurred())
	return hex.EncodeToString(b)
}

var _ = Describe("Multisig participation", func() {
	var (
		calls *recordedCalls
		mock  *mockWallet
		conn  *grpc.ClientConn
		cfg   *config.Config
		wc    *tron.WriteClient
	)

	BeforeEach(func() {
		calls = &recordedCalls{}
		mock = &mockWallet{calls: calls}
		conn = startBufconn(mock)
		cfg = &config.Config{Network: config.NetworkMainnet, GRPCAddress: "bufconn"}
		wc = tron.NewWriteClientWithConn(cfg, conn)
		DeferCleanup(func() { _ = wc.Close() })
	})

	Describe("AppendMultisigSignature", func() {
		It("accumulates signatures from different keys in order", func() {
			rawHex := buildMultisigRawDataHex(2, 1_000_000)
			k1 := mustGenerateKey()
			k2 := mustGenerateKey()

			sigs, txid1, err := tron.AppendMultisigSignature(rawHex, nil, k1)
			Expect(err).NotTo(HaveOccurred())
			Expect(sigs).To(HaveLen(1))
			Expect(txid1).To(MatchRegexp(`^[0-9a-f]{64}$`))

			sigs, txid2, err := tron.AppendMultisigSignature(rawHex, sigs, k2)
			Expect(err).NotTo(HaveOccurred())
			Expect(sigs).To(HaveLen(2))
			Expect(txid2).To(Equal(txid1), "txid must not depend on the signature list")
		})

		It("refuses to double-sign with the same key (ErrAlreadySigned)", func() {
			rawHex := buildMultisigRawDataHex(2, 42)
			k := mustGenerateKey()

			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = tron.AppendMultisigSignature(rawHex, sigs, k)
			Expect(errors.Is(err, tron.ErrAlreadySigned)).To(BeTrue())
		})

		It("produces different txids for different permission ids (regression)", func() {
			k := mustGenerateKey()
			_, idZero, err := tron.AppendMultisigSignature(buildMultisigRawDataHex(0, 10), nil, k)
			Expect(err).NotTo(HaveOccurred())
			_, idTwo, err := tron.AppendMultisigSignature(buildMultisigRawDataHex(2, 10), nil, k)
			Expect(err).NotTo(HaveOccurred())
			Expect(idZero).NotTo(Equal(idTwo))
		})
	})

	Describe("BroadcastMultisig", func() {
		var rawHex string
		var k1, k2 *ecdsa.PrivateKey

		BeforeEach(func() {
			rawHex = buildMultisigRawDataHex(2, 500)
			k1 = mustGenerateKey()
			k2 = mustGenerateKey()
		})

		It("refuses broadcast below threshold and does not call Broadcast", func() {
			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k1)
			Expect(err).NotTo(HaveOccurred())

			mock.getSignWeight = func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
				return &api.TransactionSignWeight{
					Permission: &core.Permission{
						Id:        2,
						Threshold: 2,
						Keys: []*core.Key{
							{Address: pkToTronBytes(k1), Weight: 1},
							{Address: pkToTronBytes(k2), Weight: 1},
						},
					},
					ApprovedList:  [][]byte{pkToTronBytes(k1)},
					CurrentWeight: 1,
				}, nil
			}

			_, _, err = tron.BroadcastMultisig(context.Background(), wc, rawHex, sigs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("threshold"))
			Expect(calls.broadcastCalls).To(BeEmpty())
			Expect(calls.signWeightCalls).To(HaveLen(1))
		})

		It("broadcasts when weight meets threshold", func() {
			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k1)
			Expect(err).NotTo(HaveOccurred())
			sigs, _, err = tron.AppendMultisigSignature(rawHex, sigs, k2)
			Expect(err).NotTo(HaveOccurred())

			mock.getSignWeight = func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
				return &api.TransactionSignWeight{
					Permission: &core.Permission{
						Id:        2,
						Threshold: 2,
						Keys: []*core.Key{
							{Address: pkToTronBytes(k1), Weight: 1},
							{Address: pkToTronBytes(k2), Weight: 1},
						},
					},
					ApprovedList:  [][]byte{pkToTronBytes(k1), pkToTronBytes(k2)},
					CurrentWeight: 2,
				}, nil
			}
			mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
				return okBroadcast(), nil
			}

			txid, weight, err := tron.BroadcastMultisig(context.Background(), wc, rawHex, sigs)
			Expect(err).NotTo(HaveOccurred())
			Expect(weight.GetCurrentWeight()).To(Equal(int64(2)))
			Expect(txid).To(MatchRegexp(`^0x[0-9a-f]{64}$`))
			Expect(calls.broadcastCalls).To(HaveLen(1))
			Expect(calls.broadcastCalls[0].GetSignature()).To(HaveLen(2),
				"broadcast must carry all accumulated signatures")
		})

		It("wraps broadcast errors with multisig prefix", func() {
			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k1)
			Expect(err).NotTo(HaveOccurred())
			sigs, _, err = tron.AppendMultisigSignature(rawHex, sigs, k2)
			Expect(err).NotTo(HaveOccurred())

			mock.getSignWeight = func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
				return &api.TransactionSignWeight{
					Permission:    &core.Permission{Id: 2, Threshold: 2},
					CurrentWeight: 2,
				}, nil
			}
			mock.broadcast = func(context.Context, *core.Transaction) (*api.Return, error) {
				return &api.Return{
					Result:  false,
					Code:    api.Return_CONTRACT_VALIDATE_ERROR,
					Message: []byte("invalid signature"),
				}, nil
			}

			_, _, err = tron.BroadcastMultisig(context.Background(), wc, rawHex, sigs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("multisig: broadcast:"))
			Expect(err.Error()).To(ContainSubstring("invalid signature"))
		})

		It("propagates node errors and never broadcasts on GetTransactionSignWeight failure", func() {
			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k1)
			Expect(err).NotTo(HaveOccurred())
			mock.getSignWeight = func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
				return nil, errors.New("rpc boom")
			}
			_, _, err = tron.BroadcastMultisig(context.Background(), wc, rawHex, sigs)
			Expect(err).To(HaveOccurred())
			Expect(calls.broadcastCalls).To(BeEmpty())
		})
	})

	Describe("GetMultisigSignWeight", func() {
		It("returns the node's sign-weight response verbatim", func() {
			rawHex := buildMultisigRawDataHex(2, 1)
			k := mustGenerateKey()
			sigs, _, err := tron.AppendMultisigSignature(rawHex, nil, k)
			Expect(err).NotTo(HaveOccurred())

			mock.getSignWeight = func(context.Context, *core.Transaction) (*api.TransactionSignWeight, error) {
				return &api.TransactionSignWeight{
					Permission:    &core.Permission{Id: 2, Threshold: 2},
					CurrentWeight: 1,
				}, nil
			}

			w, err := tron.GetMultisigSignWeight(context.Background(), wc, rawHex, sigs)
			Expect(err).NotTo(HaveOccurred())
			Expect(w.GetCurrentWeight()).To(Equal(int64(1)))
			Expect(w.GetPermission().GetThreshold()).To(Equal(int64(2)))
		})
	})
})

// pkToTronBytes returns the 21-byte Tron address payload (0x41 prefix)
// derived from a test-only private key. It mirrors what a real node
// returns in approved_list and permission keys.
func pkToTronBytes(k *ecdsa.PrivateKey) []byte {
	_ = crypto.Keccak256
	return address.PubkeyToAddress(k.PublicKey).Bytes()
}
