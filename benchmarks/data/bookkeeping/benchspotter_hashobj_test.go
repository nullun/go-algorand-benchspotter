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

// Added by go-algorand-benchspotter (benchmarks/data/bookkeeping); a candidate for upstream.

package bookkeeping

import (
	"testing"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/protocol"
)

// BenchmarkHashObj times crypto.HashObj (encode, prefix with the domain
// separator, SHA-512/256) on the two objects hashed most often on a node:
// a populated payment transaction, whose hash is its transaction ID, and
// a populated block header, whose hash is the block hash.
func BenchmarkHashObj(b *testing.B) {
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	b.Run("txn", func(b *testing.B) {
		txn := benchspotterSignedPayment(proto).Txn
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = crypto.HashObj(txn)
		}
	})

	b.Run("block-header", func(b *testing.B) {
		hdr := benchspotterBlockHeader()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = crypto.HashObj(hdr)
		}
	})
}
