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

// Added by go-algorand-benchspotter (benchmarks/network); a candidate for upstream.

package network

import (
	"math/rand"
	"testing"

	"github.com/algorand/go-algorand/protocol"
)

// BenchmarkMessageFilterCheck measures messageFilter.CheckIncomingMessage, the
// per-message duplicate check wsNetwork runs on incoming traffic, against a
// filter primed to the default incoming capacity (5 buckets of 512). "hit"
// looks up a message sitting in the oldest bucket, so every bucket is probed
// before it is found; "miss" looks up a message that was never added. Neither
// sub mutates the filter.
func BenchmarkMessageFilterCheck(b *testing.B) {
	const (
		bucketCount = 5   // config.Local IncomingMessageFilterBucketCount default
		bucketSize  = 512 // config.Local IncomingMessageFilterBucketSize default
		msgSize     = 300 // roughly one msgp-encoded agreement vote
	)

	rng := rand.New(rand.NewSource(11))
	newMsg := func() []byte {
		msg := make([]byte, msgSize)
		for i := range msg {
			msg[i] = byte(rng.Intn(256))
		}
		return msg
	}

	// Fill every bucket without letting the filter rotate the oldest one out:
	// the last add into a full bucket opens a fresh bucket, so stop one short.
	mf := makeMessageFilter(bucketCount, bucketSize)
	oldest := newMsg()
	mf.CheckIncomingMessage(protocol.AgreementVoteTag, oldest, true, false)
	for i := 1; i < bucketCount*bucketSize-1; i++ {
		mf.CheckIncomingMessage(protocol.AgreementVoteTag, newMsg(), true, false)
	}
	absent := newMsg()

	if !mf.CheckIncomingMessage(protocol.AgreementVoteTag, oldest, false, false) {
		b.Fatal("primed message is not in the filter")
	}
	if mf.CheckIncomingMessage(protocol.AgreementVoteTag, absent, false, false) {
		b.Fatal("unseen message is in the filter")
	}

	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if !mf.CheckIncomingMessage(protocol.AgreementVoteTag, oldest, false, false) {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if mf.CheckIncomingMessage(protocol.AgreementVoteTag, absent, false, false) {
				b.Fatal("expected miss")
			}
		}
	})
}
