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

// Added by go-algorand-benchspotter (benchmarks/ledger); a candidate for upstream.

package ledger

import (
	"testing"

	"github.com/algorand/go-deadlock"
	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/ledger/ledgercore"
	ledgertesting "github.com/algorand/go-algorand/ledger/testing"
	"github.com/algorand/go-algorand/protocol"
)

const (
	benchTxTailRounds      = 1000
	benchTxTailTxnsPerRnd  = 16
	benchTxTailLookback    = basics.Round(100)
	benchTxTailTxValidity  = basics.Round(10)
	benchTxTailLeaseExpiry = basics.Round(32)
)

// benchTxTailTxn returns the deterministic transaction placed at position
// intra of round rnd in the benchmark tail.
func benchTxTailTxn(rnd basics.Round, intra int) transactions.Transaction {
	return transactions.Transaction{
		Header: transactions.Header{
			Note: []byte{byte(rnd % 256), byte(rnd / 256), byte(intra), 1},
		},
	}
}

// benchTxTailFixture builds a txTail fed with benchTxTailRounds rounds, each
// carrying benchTxTailTxnsPerRnd transactions and one lease, committed up to
// benchTxTailLookback rounds behind the tip, the same way TestTxTailCheckdup
// does. It returns the tail and the round after the last one it holds.
func benchTxTailFixture(b *testing.B) (*txTail, basics.Round) {
	accts := ledgertesting.RandomAccounts(10, false)
	ledger := makeMockLedgerForTracker(b, true, 1, protocol.ConsensusCurrentVersion, []map[basics.Address]basics.AccountData{accts})
	defer ledger.Close()

	tail := &txTail{}
	require.NoError(b, tail.loadFromDisk(ledger, 0))

	for rnd := basics.Round(1); rnd <= benchTxTailRounds; rnd++ {
		blk := bookkeeping.Block{
			BlockHeader: bookkeeping.BlockHeader{
				Round: rnd,
				UpgradeState: bookkeeping.UpgradeState{
					CurrentProtocol: protocol.ConsensusCurrentVersion,
				},
			},
			Payset: make(transactions.Payset, benchTxTailTxnsPerRnd),
		}

		txids := make(map[transactions.Txid]ledgercore.IncludedTransactions, benchTxTailTxnsPerRnd)
		for intra := 0; intra < benchTxTailTxnsPerRnd; intra++ {
			txn := benchTxTailTxn(rnd, intra)
			blk.Payset[intra].Txn = txn
			txids[txn.ID()] = ledgercore.IncludedTransactions{LastValid: rnd + benchTxTailTxValidity, Intra: uint64(intra)}
		}
		txleases := make(map[ledgercore.Txlease]basics.Round, 1)
		txleases[ledgercore.Txlease{
			Sender: basics.Address(crypto.Hash([]byte{byte(rnd % 256), byte(rnd / 256), 2})),
			Lease:  crypto.Hash([]byte{byte(rnd % 256), byte(rnd / 256), 3}),
		}] = rnd + benchTxTailLeaseExpiry

		delta := ledgercore.MakeStateDelta(&blk.BlockHeader, 0, benchTxTailTxnsPerRnd, 0)
		delta.Txids = txids
		delta.Txleases = txleases
		tail.newBlock(blk, delta)
		tail.committedUpTo(rnd.SubSaturate(benchTxTailLookback))
	}
	return tail, basics.Round(benchTxTailRounds + 1)
}

// BenchmarkTxTailCheckDup measures txTail.checkDup, the duplicate transaction
// check every incoming transaction passes through via the pool. "hit" looks up
// a transaction that is in the tail and "miss" one that is not; neither
// carries a lease. Deadlock detection is switched off, as release builds of
// algod run, so the tail's RWMutex costs what it costs in production.
func BenchmarkTxTailCheckDup(b *testing.B) {
	deadlockDisable := deadlock.Opts.Disable
	deadlock.Opts.Disable = true
	defer func() { deadlock.Opts.Disable = deadlockDisable }()

	tail, current := benchTxTailFixture(b)
	proto := config.Consensus[protocol.ConsensusCurrentVersion]

	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		rnd := current - 5
		txn := benchTxTailTxn(rnd, 7)
		txid := txn.ID()
		lastValid := rnd + benchTxTailTxValidity
		var txInLedger *ledgercore.TransactionInLedgerError
		require.ErrorAs(b, tail.checkDup(proto, current, rnd, lastValid, txid, ledgercore.Txlease{}), &txInLedger)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := tail.checkDup(proto, current, rnd, lastValid, txid, ledgercore.Txlease{}); err == nil {
				b.Fatal("expected a duplicate")
			}
		}
	})

	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		txn := transactions.Transaction{
			Header: transactions.Header{
				Note: []byte("benchspotter: not in the tail"),
			},
		}
		txid := txn.ID()
		firstValid := current - 1
		lastValid := current + benchTxTailTxValidity
		require.NoError(b, tail.checkDup(proto, current, firstValid, lastValid, txid, ledgercore.Txlease{}))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := tail.checkDup(proto, current, firstValid, lastValid, txid, ledgercore.Txlease{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
