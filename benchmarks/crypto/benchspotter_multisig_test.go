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

// benchMultisigFixture builds a fixed 3-of-5 multisig over a fixed message
// with deterministic ed25519 keys, signed by the first three keys.
func benchMultisigFixture(b *testing.B) (Hashable, Digest, MultisigSig) {
	const (
		version   = uint8(1)
		threshold = uint8(3)
		nkeys     = 5
	)
	msg := TestingHashable{data: []byte("benchspotter multisig txid")}

	secrets := make([]*SecretKey, nkeys)
	pks := make([]PublicKey, nkeys)
	for i := range secrets {
		var s Seed
		for j := range s {
			s[j] = byte(i*31 + j)
		}
		secrets[i] = GenerateSignatureSecrets(s)
		pks[i] = secrets[i].SignatureVerifier
	}

	addr, err := MultisigAddrGen(version, threshold, pks)
	if err != nil {
		b.Fatal(err)
	}

	sigs := make([]MultisigSig, threshold)
	for i := range sigs {
		sigs[i], err = MultisigSign(msg, addr, version, threshold, pks, *secrets[i])
		if err != nil {
			b.Fatal(err)
		}
	}
	msig, err := MultisigAssemble(sigs)
	if err != nil {
		b.Fatal(err)
	}
	return msg, addr, msig
}

// BenchmarkMultisigVerify measures checking an assembled 3-of-5 MultisigSig
// with MultisigVerify: the address check and three ed25519 verifications in a
// batch, as the transaction verifier does for every multisig-signed transaction.
func BenchmarkMultisigVerify(b *testing.B) {
	msg, addr, msig := benchMultisigFixture(b)
	if err := MultisigVerify(msg, addr, msig); err != nil {
		b.Fatal(err)
	}

	// MultisigVerify is MakeBatchVerifier + MultisigBatchPrep + Verify, the
	// same path the transaction verifier takes, so one series covers both.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := MultisigVerify(msg, addr, msig); err != nil {
			b.Fatal(err)
		}
	}
}
