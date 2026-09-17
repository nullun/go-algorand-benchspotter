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
	"github.com/algorand/go-algorand/crypto/merklesignature"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/protocol"
)

// benchspotterAddr returns a fixed, distinct, non-zero address for fixture
// building. Addresses only need to be valid byte arrays for WellFormed.
func benchspotterAddr(b byte) basics.Address {
	var addr basics.Address
	for i := range addr {
		addr[i] = b ^ byte(i*7)
	}
	return addr
}

// benchspotterHeader is a populated transaction header shared by every
// fixture below. GenesisID and GenesisHash are set as they would be on
// mainnet, and the note is a typical short memo.
func benchspotterHeader(proto config.ConsensusParams, sender byte) Header {
	return Header{
		Sender:      benchspotterAddr(sender),
		Fee:         basics.MicroAlgos{Raw: proto.MinTxnFee},
		FirstValid:  1_000_000,
		LastValid:   1_001_000,
		Note:        []byte("benchspotter fixture note, 40 bytes long"),
		GenesisID:   "mainnet-v1.0",
		GenesisHash: crypto.Digest{0xc0, 0x61, 0xc4, 0xd8, 0xfc, 0x1d, 0xbd, 0xde},
	}
}

// benchspotterPayment is a valid payment with a note.
func benchspotterPayment(proto config.ConsensusParams) Transaction {
	return Transaction{
		Type:   protocol.PaymentTx,
		Header: benchspotterHeader(proto, 0x11),
		PaymentTxnFields: PaymentTxnFields{
			Receiver: benchspotterAddr(0x22),
			Amount:   basics.MicroAlgos{Raw: 1_234_567},
		},
	}
}

// benchspotterAssetTransfer is a valid asset transfer.
func benchspotterAssetTransfer(proto config.ConsensusParams) Transaction {
	return Transaction{
		Type:   protocol.AssetTransferTx,
		Header: benchspotterHeader(proto, 0x33),
		AssetTransferTxnFields: AssetTransferTxnFields{
			XferAsset:     31566704,
			AssetAmount:   5_000_000,
			AssetReceiver: benchspotterAddr(0x44),
		},
	}
}

// benchspotterAssetConfig is a valid asset creation with every param set.
func benchspotterAssetConfig(proto config.ConsensusParams) Transaction {
	return Transaction{
		Type:   protocol.AssetConfigTx,
		Header: benchspotterHeader(proto, 0x55),
		AssetConfigTxnFields: AssetConfigTxnFields{
			AssetParams: basics.AssetParams{
				Total:         1_000_000_000_000,
				Decimals:      6,
				DefaultFrozen: false,
				UnitName:      "BENCH",
				AssetName:     "Benchspotter Token",
				URL:           "https://example.com/benchspotter",
				MetadataHash:  [32]byte{0x99, 0x88, 0x77},
				Manager:       benchspotterAddr(0x55),
				Reserve:       benchspotterAddr(0x56),
				Freeze:        benchspotterAddr(0x57),
				Clawback:      benchspotterAddr(0x58),
			},
		},
	}
}

// benchspotterAppCall is a valid call to an existing application with
// arguments, accounts, foreign apps, foreign assets and box references,
// as a typical DeFi interaction carries.
func benchspotterAppCall(proto config.ConsensusParams) Transaction {
	return Transaction{
		Type:   protocol.ApplicationCallTx,
		Header: benchspotterHeader(proto, 0x66),
		ApplicationCallTxnFields: ApplicationCallTxnFields{
			ApplicationID: 1002541853,
			OnCompletion:  NoOpOC,
			ApplicationArgs: [][]byte{
				[]byte("swap"),
				{0x00, 0x00, 0x00, 0x00, 0x00, 0x0f, 0x42, 0x40},
				[]byte("fixed-input"),
			},
			Accounts:      []basics.Address{benchspotterAddr(0x67), benchspotterAddr(0x68)},
			ForeignApps:   []basics.AppIndex{1002541853, 1002541854},
			ForeignAssets: []basics.AssetIndex{31566704, 312769},
			Boxes: []BoxRef{
				{Index: 0, Name: []byte("pool-state")},
				{Index: 1, Name: []byte("user-position-0000000001")},
			},
		},
	}
}

// benchspotterKeyreg is a valid go-online key registration.
func benchspotterKeyreg(proto config.ConsensusParams) Transaction {
	var votePK crypto.OneTimeSignatureVerifier
	var selPK crypto.VRFVerifier
	var spPK merklesignature.Commitment
	for i := range votePK {
		votePK[i] = byte(i + 1)
	}
	for i := range selPK {
		selPK[i] = byte(i + 2)
	}
	for i := range spPK {
		spPK[i] = byte(i + 3)
	}
	return Transaction{
		Type:   protocol.KeyRegistrationTx,
		Header: benchspotterHeader(proto, 0x77),
		KeyregTxnFields: KeyregTxnFields{
			VotePK:          votePK,
			SelectionPK:     selPK,
			StateProofPK:    spPK,
			VoteFirst:       1_000_000,
			VoteLast:        2_000_000,
			VoteKeyDilution: 1000,
		},
	}
}

// BenchmarkWellFormed times Transaction.WellFormed, the stateless
// validity check every transaction passes through on entry to the
// transaction pool and again during block evaluation, once per
// transaction type on a populated, valid transaction.
func BenchmarkWellFormed(b *testing.B) {
	proto := config.Consensus[protocol.ConsensusCurrentVersion]
	spec := SpecialAddresses{
		FeeSink:     benchspotterAddr(0xEE),
		RewardsPool: benchspotterAddr(0xFF),
	}

	run := func(name string, tx Transaction) {
		b.Run(name, func(b *testing.B) {
			require.NoError(b, tx.WellFormed(spec, proto))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := tx.WellFormed(spec, proto); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	run("pay", benchspotterPayment(proto))
	run("axfer", benchspotterAssetTransfer(proto))
	run("acfg", benchspotterAssetConfig(proto))
	run("appl", benchspotterAppCall(proto))
	run("keyreg", benchspotterKeyreg(proto))
}
