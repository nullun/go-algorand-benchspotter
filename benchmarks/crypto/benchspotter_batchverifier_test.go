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
	"fmt"
	"testing"
)

// benchBatchFixture builds n deterministic ed25519 keypairs, one message each
// and a valid signature over it. Deterministic so that two refs verify the
// same bytes: ed25519 verification time does not depend on the message, but a
// benchmark that reruns the same work is one less thing to rule out when a
// point moves.
func benchBatchFixture(b *testing.B, n int) ([]SignatureVerifier, []Hashable, []Signature) {
	pks := make([]SignatureVerifier, n)
	msgs := make([]Hashable, n)
	sigs := make([]Signature, n)
	for i := range pks {
		var s Seed
		for j := range s {
			s[j] = byte(i*7 + j*3 + 1)
		}
		secrets := GenerateSignatureSecrets(s)
		msgs[i] = TestingHashable{data: []byte(fmt.Sprintf("benchspotter batch verify %d", i))}
		pks[i] = secrets.SignatureVerifier
		sigs[i] = secrets.Sign(msgs[i])
	}
	return pks, msgs, sigs
}

// BenchmarkBatchVerify measures the batch verifier a ref ships with, through
// the exported API only: one op enqueues `size` valid signatures into a fresh
// verifier and verifies them in one call. That is the shape agreement uses for
// a period's votes and the ledger for a block's transactions, so the sizes
// bracket what a node actually sees.
//
// Only MakeBatchVerifierWithHint, EnqueueSignature and Verify are used, all
// exported since v3.17.0, so this runs at every tag the sweep covers.
// Upstream's own BenchmarkBatchVerifierImpls compares the three
// implementations against each other, which needs unexported names and only
// exists from v4.4.1; this one answers the different question of what the
// default costs over time.
//
// The verifier is made inside the timed loop on purpose. Reusing one across
// iterations leaves the signatures of every previous iteration enqueued, so
// the batch grows with b.N and ns/op stops meaning anything; that was the bug
// in BenchmarkBatchVerifierImpls before v5.0.0 (patches/fix-batchverifier-impls-benchmark.sh).
func BenchmarkBatchVerify(b *testing.B) {
	for _, size := range []int{1, 64, 256} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			pks, msgs, sigs := benchBatchFixture(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bv := MakeBatchVerifierWithHint(size)
				for j := range pks {
					bv.EnqueueSignature(pks[j], msgs[j], sigs[j])
				}
				if err := bv.Verify(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
