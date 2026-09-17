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

// Added by go-algorand-benchspotter (benchmarks/ledger/ledgercore); a candidate for upstream.

package ledgercore

import (
	"testing"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
)

const (
	benchAccountDeltasHint    = 4000 // the block's expected transaction count, as eval passes it
	benchAccountDeltasEntries = 4000 // distinct accounts a full block touches
	benchAccountDeltasRepeats = 1000 // extra upserts that overwrite an existing account
)

// benchAccountDeltasAddrs returns n deterministic distinct addresses.
func benchAccountDeltasAddrs(n int) []basics.Address {
	addrs := make([]basics.Address, n)
	for i := range addrs {
		addrs[i] = basics.Address(crypto.Hash([]byte{byte(i % 256), byte((i / 256) % 256), byte(i / 65536), 0xad}))
	}
	return addrs
}

func benchAccountDeltasData(i int) AccountData {
	return AccountData{
		AccountBaseData: AccountBaseData{
			Status:      basics.Offline,
			MicroAlgos:  basics.MicroAlgos{Raw: 1_000_000 + uint64(i)},
			RewardsBase: uint64(i % 7),
			TotalAssets: uint64(i % 5),
		},
	}
}

// benchAccountDeltasFilled returns an AccountDeltas holding the first n of
// addrs, allocated with the given hint.
func benchAccountDeltasFilled(hint int, addrs []basics.Address, n int) AccountDeltas {
	ad := MakeAccountDeltas(hint)
	for i := 0; i < n; i++ {
		ad.Upsert(addrs[i], benchAccountDeltasData(i))
	}
	return ad
}

// BenchmarkAccountDeltasOps measures the AccountDeltas operations the block
// evaluator performs while building a round's StateDelta.
//
//   - "upsert" fills a fresh AccountDeltas with benchAccountDeltasEntries
//     accounts and then overwrites benchAccountDeltasRepeats of them, the way
//     transactions touching the same account within a block do.
//   - "merge" merges a full set of account and resource deltas into a
//     StateDelta that already holds half of the accounts (a transaction
//     group's deltas landing on the block's), on a fresh copy each iteration.
//   - "optimize" runs StateDelta.OptimizeAllocatedMemory over a delta whose
//     block filled a quarter of its hint, so both the slice and the cache map
//     are reallocated, restoring the oversized allocation between iterations.
func BenchmarkAccountDeltasOps(b *testing.B) {
	addrs := benchAccountDeltasAddrs(benchAccountDeltasEntries)
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	b.Run("upsert", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ad := MakeAccountDeltas(benchAccountDeltasHint)
			for k := 0; k < benchAccountDeltasEntries; k++ {
				ad.Upsert(addrs[k], benchAccountDeltasData(k))
			}
			for k := 0; k < benchAccountDeltasRepeats; k++ {
				ad.Upsert(addrs[k*3%benchAccountDeltasEntries], benchAccountDeltasData(k+1))
			}
			if ad.Len() != benchAccountDeltasEntries {
				b.Fatalf("unexpected length %d", ad.Len())
			}
		}
	})

	b.Run("merge", func(b *testing.B) {
		b.ReportAllocs()
		other := benchAccountDeltasFilled(benchAccountDeltasHint, addrs, benchAccountDeltasEntries)
		for k := 0; k < benchAccountDeltasEntries; k += 4 {
			holding := &basics.AssetHolding{Amount: uint64(k)}
			other.UpsertAssetResource(addrs[k], basics.AssetIndex(k%97+1), AssetParamsDelta{}, AssetHoldingDelta{Holding: holding})
			state := &basics.AppLocalState{Schema: basics.StateSchema{NumUint: 1}}
			other.UpsertAppResource(addrs[k], basics.AppIndex(k%89+1), AppParamsDelta{}, AppLocalStateDelta{LocalState: state})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			base := benchAccountDeltasFilled(benchAccountDeltasHint, addrs, benchAccountDeltasEntries/2)
			b.StartTimer()

			base.MergeAccounts(other)
			if base.Len() != benchAccountDeltasEntries {
				b.Fatalf("unexpected length %d", base.Len())
			}
		}
	})

	b.Run("optimize", func(b *testing.B) {
		b.ReportAllocs()
		filled := benchAccountDeltasEntries / 4
		sd := MakeStateDelta(nil, 0, benchAccountDeltasHint, 0)
		for k := 0; k < filled; k++ {
			sd.Accts.Upsert(addrs[k], benchAccountDeltasData(k))
		}
		// keep the oversized allocations so each iteration can start from them
		oversized := sd.Accts.Accts
		oversizedCache := sd.Accts.acctsCache

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			sd.Accts.Accts = oversized
			sd.Accts.acctsCache = oversizedCache
			b.StartTimer()

			sd.OptimizeAllocatedMemory(proto.MaxBalLookback)
			if cap(sd.Accts.Accts) != filled {
				b.Fatalf("expected the accounts slice to be trimmed to %d, got cap %d", filled, cap(sd.Accts.Accts))
			}
		}
	})
}
