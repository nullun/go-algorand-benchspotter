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

// Added by go-algorand-benchspotter (benchmarks/data/transactions); a candidate for upstream.

package transactions

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/protocol"
)

// BenchmarkTxnIDs times the two hashes that name transactions: the
// transaction ID of a signed payment (SignedTxn.ID, which encodes the
// transaction and hashes it), and the group ID of a 16-transaction group
// (each member's ID with Group cleared, then the hash of the TxGroup that
// lists them, as clients compute before signing).
func BenchmarkTxnIDs(b *testing.B) {
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	b.Run("signed-txn-id", func(b *testing.B) {
		stxn := SignedTxn{Txn: benchspotterPayment(proto)}
		stxn.Sig[0] = 0x42
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = stxn.ID()
		}
	})

	b.Run("group-id", func(b *testing.B) {
		const groupSize = 16
		require.LessOrEqual(b, groupSize, proto.MaxTxGroupSize)
		txns := make([]Transaction, groupSize)
		for i := range txns {
			if i%2 == 0 {
				txns[i] = benchspotterPayment(proto)
			} else {
				txns[i] = benchspotterAppCall(proto)
			}
			txns[i].Sender[0] = byte(i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			group := TxGroup{TxGroupHashes: make([]crypto.Digest, groupSize)}
			for j := range txns {
				group.TxGroupHashes[j] = crypto.Digest(txns[j].ID())
			}
			_ = crypto.HashObj(group)
		}
	})
}
