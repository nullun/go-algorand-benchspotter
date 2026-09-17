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

// Added by go-algorand-benchspotter (benchmarks/crypto); a candidate for upstream.

package crypto

import (
	"testing"
)

// BenchmarkVrfProve measures VrfPrivkey.Prove, the ECVRF proof a node
// computes for every sortition attempt (proposal and vote eligibility).
func BenchmarkVrfProve(b *testing.B) {
	var seed [32]byte
	for i := range seed {
		seed[i] = byte(i*13 + 1)
	}
	_, sk := VrfKeygenFromSeed(seed)
	msg := TestingHashable{data: []byte("benchspotter vrf sortition seed and selector")}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := sk.Prove(msg); !ok {
			b.Fatal("VRF prove failed")
		}
	}
}
