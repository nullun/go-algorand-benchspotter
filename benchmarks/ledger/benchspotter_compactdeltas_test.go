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

// Added by go-algorand-benchspotter (benchmarks/ledger); a candidate for upstream.

package ledger

import (
	"testing"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/ledger/ledgercore"
)

const (
	benchCompactResourceRounds = 320 // one MaxBalLookback window of rounds
	benchCompactResourceWindow = 32  // accounts touched per round
	benchCompactResourceStride = 16  // accounts the window slides by each round
)

// benchCompactResourceDeltas builds benchCompactResourceRounds rounds of state
// deltas. Each round touches a window of benchCompactResourceWindow accounts
// that overlaps the previous round by half, and gives every account in the
// window an account change, one asset holding and one app local state, so the
// compaction sees both fresh entries and repeated changes it has to fold
// together and sizes its maps from the account count as it does in production.
func benchCompactResourceDeltas() []ledgercore.StateDelta {
	numAddrs := benchCompactResourceWindow + benchCompactResourceStride*(benchCompactResourceRounds-1)
	addrs := make([]basics.Address, numAddrs)
	for i := range addrs {
		addrs[i] = basics.Address(crypto.Hash([]byte{byte(i % 256), byte((i / 256) % 256), byte(i / 65536), 0xbe}))
	}

	stateDeltas := make([]ledgercore.StateDelta, benchCompactResourceRounds)
	for rnd := range stateDeltas {
		start := rnd * benchCompactResourceStride
		for k := start; k < start+benchCompactResourceWindow; k++ {
			addr := addrs[k]
			stateDeltas[rnd].Accts.Upsert(addr, ledgercore.AccountData{
				AccountBaseData: ledgercore.AccountBaseData{MicroAlgos: basics.MicroAlgos{Raw: uint64(rnd*1000 + k)}},
			})
			aidx := basics.AssetIndex(k%97 + 1)
			holding := &basics.AssetHolding{Amount: uint64(rnd*1000 + k)}
			stateDeltas[rnd].Accts.UpsertAssetResource(addr, aidx, ledgercore.AssetParamsDelta{}, ledgercore.AssetHoldingDelta{Holding: holding})

			appIdx := basics.AppIndex(k%89 + 1)
			state := &basics.AppLocalState{
				Schema:   basics.StateSchema{NumUint: 1},
				KeyValue: basics.TealKeyValue{"counter": basics.TealValue{Type: basics.TealUintType, Uint: uint64(rnd)}},
			}
			stateDeltas[rnd].Accts.UpsertAppResource(addr, appIdx, ledgercore.AppParamsDelta{}, ledgercore.AppLocalStateDelta{LocalState: state})
		}
	}
	return stateDeltas
}

// BenchmarkCompactResourceDeltas measures makeCompactResourceDeltas, the step
// of the account tracker's commit that folds the asset and app resource
// changes of a range of rounds into one set of database writes. The base
// caches are empty, so every resource takes the not-yet-loaded path, as in
// the upstream account-deltas benchmark.
func BenchmarkCompactResourceDeltas(b *testing.B) {
	b.ReportAllocs()

	stateDeltas := benchCompactResourceDeltas()
	var baseAccounts lruAccounts
	var baseResources lruResources
	baseAccounts.init(nil, 100, 80)
	baseResources.init(nil, 100, 80)

	out := makeCompactResourceDeltas(stateDeltas, 1, true, baseAccounts, baseResources)
	if out.len() == 0 || len(out.misses) != out.len() {
		b.Fatalf("unexpected compaction: %d entries, %d misses", out.len(), len(out.misses))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out = makeCompactResourceDeltas(stateDeltas, 1, true, baseAccounts, baseResources)
	}
	_ = out
}
