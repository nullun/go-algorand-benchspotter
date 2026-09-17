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

// Added by go-algorand-benchspotter (benchmarks/network/vpack); a candidate for upstream.

package vpack

import (
	"math/rand"
	"testing"

	"github.com/algorand/go-algorand/agreement"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
)

// benchspotterVoteMsgp returns the msgp encoding of one fixed, fully populated
// unauthenticatedVote of the shape the network layer sees on the wire: a
// period-0 cert vote in a mainnet-sized round with a two-level ephemeral key.
func benchspotterVoteMsgp() []byte {
	rng := rand.New(rand.NewSource(7))
	fill := func(dst []byte) {
		for i := range dst {
			dst[i] = byte(rng.Intn(256))
		}
	}

	var v agreement.UnauthenticatedVote
	fill(v.R.Sender[:])
	v.R.Round = basics.Round(48_000_000)
	v.R.Period = 0
	v.R.Step = 2 // cert
	v.R.Proposal.OriginalPeriod = 0
	v.R.Proposal.OriginalProposer = v.R.Sender
	fill(v.R.Proposal.BlockDigest[:])
	fill(v.R.Proposal.EncodingDigest[:])
	fill(v.Cred.Proof[:])
	fill(v.Sig.Sig[:])
	fill(v.Sig.PK[:])
	fill(v.Sig.PK2[:])
	fill(v.Sig.PK1Sig[:])
	fill(v.Sig.PK2Sig[:])
	// PKSigOld is always empty on the wire.

	return protocol.EncodeMsgp(&v)
}

// BenchmarkVpackVote measures the stateless vpack codec the network layer
// applies to every agreement vote (network/msgCompressor.go vpackCompressVote
// and the matching decompression on receipt).
func BenchmarkVpackVote(b *testing.B) {
	msgpBuf := benchspotterVoteMsgp()

	enc := NewStatelessEncoder()
	compressed, err := enc.CompressVote(nil, msgpBuf)
	if err != nil {
		b.Fatal(err)
	}
	dec := NewStatelessDecoder()
	roundTrip, err := dec.DecompressVote(nil, compressed)
	if err != nil {
		b.Fatal(err)
	}
	if string(roundTrip) != string(msgpBuf) {
		b.Fatalf("vpack round trip mismatch: %d bytes in, %d bytes out", len(msgpBuf), len(roundTrip))
	}

	b.Run("compress", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(msgpBuf)))
		dst := make([]byte, 0, MaxCompressedVoteSize)
		for i := 0; i < b.N; i++ {
			dst, err = enc.CompressVote(dst[:0], msgpBuf)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("decompress", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(compressed)))
		dst := make([]byte, 0, MaxMsgpackVoteSize)
		for i := 0; i < b.N; i++ {
			dst, err = dec.DecompressVote(dst[:0], compressed)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
