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

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/committee"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
)

// benchspotterAddr returns a fixed, distinct, non-zero address.
func benchspotterAddr(b byte) basics.Address {
	var addr basics.Address
	for i := range addr {
		addr[i] = b ^ byte(i*7)
	}
	return addr
}

func benchspotterDigest(b byte) (d crypto.Digest) {
	for i := range d {
		d[i] = b + byte(i)
	}
	return d
}

func benchspotterHeader(proto config.ConsensusParams, sender byte) transactions.Header {
	return transactions.Header{
		Sender:      benchspotterAddr(sender),
		Fee:         basics.MicroAlgos{Raw: proto.MinTxnFee},
		FirstValid:  1_000_000,
		LastValid:   1_001_000,
		Note:        []byte("benchspotter fixture note, 40 bytes long"),
		GenesisID:   "mainnet-v1.0",
		GenesisHash: benchspotterDigest(0xc0),
	}
}

// benchspotterSignedPayment is a signed payment with a note, as it
// travels over the network and sits in a block.
func benchspotterSignedPayment(proto config.ConsensusParams) transactions.SignedTxn {
	stxn := transactions.SignedTxn{
		Txn: transactions.Transaction{
			Type:   protocol.PaymentTx,
			Header: benchspotterHeader(proto, 0x11),
			PaymentTxnFields: transactions.PaymentTxnFields{
				Receiver: benchspotterAddr(0x22),
				Amount:   basics.MicroAlgos{Raw: 1_234_567},
			},
		},
	}
	for i := range stxn.Sig {
		stxn.Sig[i] = byte(i * 3)
	}
	return stxn
}

// benchspotterSignedAppCall is a signed call to an existing application
// with arguments, accounts, foreign apps, foreign assets and box
// references, as a typical DeFi interaction carries.
func benchspotterSignedAppCall(proto config.ConsensusParams) transactions.SignedTxn {
	stxn := transactions.SignedTxn{
		Txn: transactions.Transaction{
			Type:   protocol.ApplicationCallTx,
			Header: benchspotterHeader(proto, 0x66),
			ApplicationCallTxnFields: transactions.ApplicationCallTxnFields{
				ApplicationID: 1002541853,
				OnCompletion:  transactions.NoOpOC,
				ApplicationArgs: [][]byte{
					[]byte("swap"),
					{0x00, 0x00, 0x00, 0x00, 0x00, 0x0f, 0x42, 0x40},
					[]byte("fixed-input"),
				},
				Accounts:      []basics.Address{benchspotterAddr(0x67), benchspotterAddr(0x68)},
				ForeignApps:   []basics.AppIndex{1002541853, 1002541854},
				ForeignAssets: []basics.AssetIndex{31566704, 312769},
				Boxes: []transactions.BoxRef{
					{Index: 0, Name: []byte("pool-state")},
					{Index: 1, Name: []byte("user-position-0000000001")},
				},
			},
		},
	}
	for i := range stxn.Sig {
		stxn.Sig[i] = byte(i * 5)
	}
	return stxn
}

// benchspotterBlockHeader is a block header with every field a mainnet
// block carries populated, including payouts, rewards, upgrade state,
// state proof tracking and a few participation updates.
func benchspotterBlockHeader() BlockHeader {
	var seed committee.Seed
	for i := range seed {
		seed[i] = byte(0x5e + i)
	}
	var branch512 crypto.Sha512Digest
	for i := range branch512 {
		branch512[i] = byte(0xb5 + i)
	}
	return BlockHeader{
		Round:     50_000_000,
		Branch:    BlockHash(benchspotterDigest(0xb0)),
		Branch512: branch512,
		Seed:      seed,
		TxnCommitments: TxnCommitments{
			NativeSha512_256Commitment: benchspotterDigest(0x01),
			Sha256Commitment:           benchspotterDigest(0x02),
			Sha512Commitment:           branch512,
		},
		TimeStamp:      1_750_000_000,
		GenesisID:      "mainnet-v1.0",
		GenesisHash:    benchspotterDigest(0xc0),
		Proposer:       benchspotterAddr(0x70),
		FeesCollected:  basics.MicroAlgos{Raw: 123_000},
		Bonus:          basics.MicroAlgos{Raw: 10_000_000},
		ProposerPayout: basics.MicroAlgos{Raw: 10_061_500},
		RewardsState: RewardsState{
			FeeSink:                   benchspotterAddr(0xEE),
			RewardsPool:               benchspotterAddr(0xFF),
			RewardsLevel:              218288,
			RewardsRate:               0,
			RewardsResidue:            5_000_000,
			RewardsRecalculationRound: 50_500_000,
		},
		UpgradeState: UpgradeState{
			CurrentProtocol:        protocol.ConsensusCurrentVersion,
			NextProtocol:           protocol.ConsensusFuture,
			NextProtocolApprovals:  9_000,
			NextProtocolVoteBefore: 50_005_000,
			NextProtocolSwitchOn:   50_145_000,
		},
		UpgradeVote: UpgradeVote{
			UpgradePropose: protocol.ConsensusFuture,
			UpgradeDelay:   140_000,
			UpgradeApprove: true,
		},
		TxnCounter: 3_000_000_000,
		StateProofTracking: map[protocol.StateProofType]StateProofTrackingData{
			protocol.StateProofBasic: {
				StateProofVotersCommitment:  crypto.GenericDigest(benchspotterDigest(0xa0).ToSlice()),
				StateProofOnlineTotalWeight: basics.MicroAlgos{Raw: 1_500_000_000_000_000},
				StateProofNextRound:         50_000_256,
			},
		},
		ParticipationUpdates: ParticipationUpdates{
			ExpiredParticipationAccounts: []basics.Address{benchspotterAddr(0x80), benchspotterAddr(0x81)},
			AbsentParticipationAccounts:  []basics.Address{benchspotterAddr(0x82)},
		},
	}
}

// BenchmarkCodecRealistic times protocol.Encode and protocol.Decode on
// populated values: a signed payment with a note, a signed application
// call with arguments and references, and a full block header. The
// generated msgp benchmarks only encode zero values, so they miss the
// cost of the fields that real traffic carries.
func BenchmarkCodecRealistic(b *testing.B) {
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	pay := benchspotterSignedPayment(proto)
	appl := benchspotterSignedAppCall(proto)
	hdr := benchspotterBlockHeader()

	payEnc := protocol.Encode(&pay)
	applEnc := protocol.Encode(&appl)
	hdrEnc := protocol.Encode(&hdr)

	b.Run("encode-signed-pay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = protocol.Encode(&pay)
		}
	})
	b.Run("decode-signed-pay", func(b *testing.B) {
		var out transactions.SignedTxn
		require.NoError(b, protocol.Decode(payEnc, &out))
		require.Equal(b, pay, out)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out = transactions.SignedTxn{}
			if err := protocol.Decode(payEnc, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("encode-signed-appl", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = protocol.Encode(&appl)
		}
	})
	b.Run("decode-signed-appl", func(b *testing.B) {
		var out transactions.SignedTxn
		require.NoError(b, protocol.Decode(applEnc, &out))
		require.Equal(b, appl, out)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out = transactions.SignedTxn{}
			if err := protocol.Decode(applEnc, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("encode-block-header", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = protocol.Encode(&hdr)
		}
	})
	b.Run("decode-block-header", func(b *testing.B) {
		var out BlockHeader
		require.NoError(b, protocol.Decode(hdrEnc, &out))
		require.Equal(b, hdr, out)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			out = BlockHeader{}
			if err := protocol.Decode(hdrEnc, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
}
