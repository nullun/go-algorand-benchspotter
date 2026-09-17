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

// benchFalconFixture builds one Falcon-1024 signer from a fixed seed and one
// signature over a fixed message, outside the timed loop.
func benchFalconFixture(b *testing.B) (FalconSigner, []byte, FalconSignature) {
	var seed FalconSeed
	for i := range seed {
		seed[i] = byte(i*7 + 3)
	}
	signer, err := GenerateFalconSigner(seed)
	if err != nil {
		b.Fatal(err)
	}
	msg := []byte("Neque porro quisquam est qui dolorem ipsum quia dolor sit amet")
	sig, err := signer.SignBytes(msg)
	if err != nil {
		b.Fatal(err)
	}
	return signer, msg, sig
}

// BenchmarkFalconVerify measures FalconVerifier.VerifyBytes, the Falcon-1024
// check performed for every ephemeral key in a state proof.
func BenchmarkFalconVerify(b *testing.B) {
	signer, msg, sig := benchFalconFixture(b)
	verifier := signer.GetVerifyingKey()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := verifier.VerifyBytes(msg, sig); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFalconSign measures FalconSigner.SignBytes, the Falcon-1024
// compressed signature a participant produces for a state proof message.
func BenchmarkFalconSign(b *testing.B) {
	signer, msg, _ := benchFalconFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := signer.SignBytes(msg); err != nil {
			b.Fatal(err)
		}
	}
}
