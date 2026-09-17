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

// Added by go-algorand-benchspotter (benchmarks/crypto/merklesignature); a candidate for upstream.

package merklesignature

import (
	"testing"
)

// BenchmarkMerkleSignatureVerify measures Verifier.VerifyBytes, the full
// per-participant state proof signature check: the vector commitment path of
// the ephemeral Falcon key followed by the Falcon-1024 signature itself.
func BenchmarkMerkleSignatureVerify(b *testing.B) {
	// Three ephemeral keys (rounds 0, 256, 512); Falcon keygen is ~30 ms each.
	secrets, err := New(0, 512, 256)
	if err != nil {
		b.Fatal(err)
	}
	const round = uint64(256)
	msg := []byte("benchspotter state proof message")
	sig, err := secrets.GetSigner(round).SignBytes(msg)
	if err != nil {
		b.Fatal(err)
	}
	verifier := secrets.GetVerifier()
	if err := verifier.VerifyBytes(round, msg, &sig); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := verifier.VerifyBytes(round, msg, &sig); err != nil {
			b.Fatal(err)
		}
	}
}
