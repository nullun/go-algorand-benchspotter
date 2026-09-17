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

// Added by go-algorand-benchspotter (benchmarks/agreement); a candidate for upstream.

package agreement

import (
	"testing"

	"github.com/algorand/go-algorand/crypto"
)

// BenchmarkVoteVerify measures unauthenticatedVote.verify called directly on
// the 100-account fixture: the sortition credential check, the ephemeral
// one-time signature verification and the ledger lookups a node performs for
// every vote it relays, without the execpool that cryptoVerifier wraps it in.
func BenchmarkVoteVerify(b *testing.B) {
	ledger, addresses, selections, votings := readOnlyFixture100()
	round := ledger.NextRound()
	per := period(0)
	stp := soft

	senderIdx := findSender(ledger, round, per, stp, addresses, selections)
	if senderIdx < 0 {
		b.Fatal("no account in the fixture is selected for this round")
	}

	proposal := proposalValue{
		OriginalPeriod:   per,
		OriginalProposer: addresses[senderIdx],
		BlockDigest:      crypto.Hash([]byte("benchspotter vote verify block")),
	}
	uv := makeUnauthenticatedVote(ledger, addresses[senderIdx], selections[senderIdx], votings[senderIdx], round, per, stp, proposal)
	if _, err := uv.verify(ledger); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := uv.verify(ledger); err != nil {
			b.Fatal(err)
		}
	}
}
