// Copyright (C) 2019-2026 Algorand Foundation Ltd.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

// Added by go-algorand-benchspotter (benchmarks/ledger/apply); a candidate for upstream.

package apply

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/crypto/merklesignature"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
)

// benchApplyAddr returns a deterministic address for the benchmark fixtures.
func benchApplyAddr(tag byte) basics.Address {
	return basics.Address(crypto.Hash([]byte{0xa9, 0x91, tag}))
}

// BenchmarkApply measures the ledger/apply entry points the block evaluator
// calls for each transaction type, against the package's in-memory mock
// Balances. Every operation leaves the mock in the state it found it, so no
// reset is needed between iterations:
//
//   - payment is left out: the mock's Move is a no-op, so apply.Payment on
//     it would measure only the call overhead.
//   - "asset-transfer" sends a non-zero amount of an asset to the sender
//     itself, so the holding is taken out of and put back into one account.
//   - "keyreg" registers the same participation keys and goes online again.
//   - "app-call" is a NoOp call to an existing app whose approval program the
//     mock evaluator accepts.
func BenchmarkApply(b *testing.B) {
	sender := benchApplyAddr(1)
	receiver := benchApplyAddr(2)
	spec := transactions.SpecialAddresses{FeeSink: benchApplyAddr(3), RewardsPool: benchApplyAddr(4)}
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	b.Run("asset-transfer", func(b *testing.B) {
		b.ReportAllocs()
		const asset = basics.AssetIndex(7)
		balances := makeMockBalancesWithAccounts(protocol.ConsensusCurrentVersion, map[basics.Address]basics.AccountData{
			sender: {
				MicroAlgos: basics.MicroAlgos{Raw: 10_000_000},
				Assets:     map[basics.AssetIndex]basics.AssetHolding{asset: {Amount: 1_000_000}},
			},
			receiver: {
				MicroAlgos: basics.MicroAlgos{Raw: 10_000_000},
				Assets:     map[basics.AssetIndex]basics.AssetHolding{asset: {Amount: 0}},
			},
		})
		tx := transactions.Transaction{
			Type: protocol.AssetTransferTx,
			Header: transactions.Header{
				Sender:     sender,
				Fee:        basics.MicroAlgos{Raw: proto.MinTxnFee},
				FirstValid: 100,
				LastValid:  1000,
			},
			AssetTransferTxnFields: transactions.AssetTransferTxnFields{
				XferAsset:     asset,
				AssetAmount:   1_000,
				AssetReceiver: sender,
			},
		}
		var ad transactions.ApplyData
		require.NoError(b, AssetTransfer(tx.AssetTransferTxnFields, tx.Header, balances, spec, &ad))
		require.Equal(b, uint64(1_000_000), balances.b[sender].Assets[asset].Amount)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ad = transactions.ApplyData{}
			if err := AssetTransfer(tx.AssetTransferTxnFields, tx.Header, balances, spec, &ad); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("keyreg", func(b *testing.B) {
		b.ReportAllocs()
		balances := makeMockBalancesWithAccounts(protocol.ConsensusCurrentVersion, map[basics.Address]basics.AccountData{
			sender: {MicroAlgos: basics.MicroAlgos{Raw: 10_000_000}},
		})
		var votePK crypto.OneTimeSignatureVerifier
		var selectionPK crypto.VRFVerifier
		var stateProofPK merklesignature.Commitment
		for i := range votePK {
			votePK[i] = byte(i + 1)
			selectionPK[i] = byte(i + 2)
			stateProofPK[i] = byte(i + 3)
		}
		tx := transactions.Transaction{
			Type: protocol.KeyRegistrationTx,
			Header: transactions.Header{
				Sender:     sender,
				Fee:        basics.MicroAlgos{Raw: proto.MinTxnFee},
				FirstValid: 100,
				LastValid:  1000,
			},
			KeyregTxnFields: transactions.KeyregTxnFields{
				VotePK:          votePK,
				SelectionPK:     selectionPK,
				StateProofPK:    stateProofPK,
				VoteFirst:       100,
				VoteLast:        3000,
				VoteKeyDilution: 10_000,
			},
		}
		round := basics.Round(500)
		var ad transactions.ApplyData
		require.NoError(b, Keyreg(tx.KeyregTxnFields, tx.Header, balances, spec, &ad, round))
		require.Equal(b, basics.Online, balances.b[sender].Status)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ad = transactions.ApplyData{}
			if err := Keyreg(tx.KeyregTxnFields, tx.Header, balances, spec, &ad, round); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("app-call", func(b *testing.B) {
		b.ReportAllocs()
		const appIdx = basics.AppIndex(11)
		creator := benchApplyAddr(5)
		balances := newTestBalancesPass()
		balances.SetProto(protocol.ConsensusCurrentVersion)
		balances.appCreators = map[basics.AppIndex]basics.Address{appIdx: creator}
		balances.balances = map[basics.Address]basics.AccountData{
			creator: {
				MicroAlgos: basics.MicroAlgos{Raw: 10_000_000},
				AppParams: map[basics.AppIndex]basics.AppParams{appIdx: {
					ApprovalProgram:   []byte{0x0a, 0x81, 0x01},
					ClearStateProgram: []byte{0x0a, 0x81, 0x01},
					StateSchemas: basics.StateSchemas{
						GlobalStateSchema: basics.StateSchema{NumUint: 1},
						LocalStateSchema:  basics.StateSchema{NumUint: 1},
					},
				}},
			},
			sender: {MicroAlgos: basics.MicroAlgos{Raw: 10_000_000}},
		}
		params := balances.ConsensusParams()
		ep := makeTestEp(&params)
		tx := transactions.Transaction{
			Type: protocol.ApplicationCallTx,
			Header: transactions.Header{
				Sender:     sender,
				Fee:        basics.MicroAlgos{Raw: proto.MinTxnFee},
				FirstValid: 100,
				LastValid:  1000,
			},
			ApplicationCallTxnFields: transactions.ApplicationCallTxnFields{
				ApplicationID: appIdx,
				OnCompletion:  transactions.NoOpOC,
			},
		}
		var ad transactions.ApplyData
		require.NoError(b, ApplicationCall(tx.ApplicationCallTxnFields, tx.Header, balances, &ad, 0, ep, 1))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ad = transactions.ApplyData{}
			if err := ApplicationCall(tx.ApplicationCallTxnFields, tx.Header, balances, &ad, 0, ep, 1); err != nil {
				b.Fatal(err)
			}
		}
	})
}
