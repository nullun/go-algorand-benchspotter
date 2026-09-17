#!/usr/bin/env bash
#
# data/pools/transactionPool_test.go: BenchmarkTransactionPoolRecompute calls
# b.FailNow unless MaxTxnBytesPerBlock is 5 MB, builds b.N pools of 75k
# transactions before the timer starts, sleeps a second and writes a CPU
# profile when CPUPROFILE is set. It cannot run under go test -bench with the
# default block size or a normal benchtime.
#
# Replace it with a fixed 5000-transaction pool that times
# recomputeBlockEvaluator, the work OnNewBlock does after every block. The
# new benchmark lives in its own file; the old one is cut out of the test
# file along with the imports only it used. The same change is a commit on the
# bench/fix-broken-benchmarks branch of the go-algorand clone. This edits the
# throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
TEST="$CHECKOUT/data/pools/transactionPool_test.go"
NEW="$CHECKOUT/data/pools/transactionPool_bench_test.go"

if [ ! -f "$TEST" ]; then
  echo "skip: $TEST does not exist at this ref"
  exit 0
fi
if [ -f "$NEW" ]; then
  echo "skip: $NEW already exists, upstream has the rewrite"
  exit 0
fi
if ! grep -q '^// BenchmarkTransactionPoolRecompute attempts to build' "$TEST" \
   || ! grep -q '^func BenchmarkTransactionPoolSteadyState' "$TEST"; then
  echo "skip: old benchmark not found in the expected shape"
  exit 0
fi

# Cut from the old benchmark's comment up to the next function.
perl -0pi -e 's{^// BenchmarkTransactionPoolRecompute attempts to build.*?(?=^func BenchmarkTransactionPoolSteadyState)}{}ms' "$TEST"
# Imports only the old benchmark used. Each is removed only if nothing else
# in the file refers to it, so this stays correct if the file changes.
for imp in os runtime/pprof runtime; do
  pkg="${imp##*/}"
  if ! grep -q "\\b$pkg\\." <(grep -v '^import\|^\t"' "$TEST"); then
    perl -pi -e "s{^\\t\"\\Q$imp\\E\"\\n}{}" "$TEST"
  fi
done

cat > "$NEW" <<'GOEOF'
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

package pools

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/ledger/ledgercore"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/protocol"
)

// BenchmarkTransactionPoolRecompute times recomputeBlockEvaluator, which
// OnNewBlock runs after every block: a fresh block evaluator is started and
// every pending transaction group is applied to it again. The pool holds a
// fixed number of payment transactions and nothing is committed between
// iterations, so the pool is the same size on every one and the cost measured
// is the re-evaluation of the pending set.
func BenchmarkTransactionPoolRecompute(b *testing.B) {
	const numAccounts = 100
	const numTxns = 5000

	secrets, addresses := generateAccounts(numAccounts)
	l := mockLedger(b, initAccFixed(addresses, 1<<50), protocol.ConsensusCurrentVersion)
	pool := MakeTransactionPool(l, config.GetDefaultLocal(), logging.Base(), nil)

	for i := 0; i < numTxns; i++ {
		tx := transactions.Transaction{
			Type: protocol.PaymentTx,
			Header: transactions.Header{
				Sender:      addresses[i%numAccounts],
				Fee:         basics.MicroAlgos{Raw: 20000 + proto.MinTxnFee},
				FirstValid:  0,
				LastValid:   basics.Round(proto.MaxTxnLife),
				GenesisHash: l.GenesisHash(),
			},
			PaymentTxnFields: transactions.PaymentTxnFields{
				Receiver: addresses[(i+1)%numAccounts],
				// The amount makes each transaction, and so each txid, distinct.
				Amount: basics.MicroAlgos{Raw: proto.MinBalance + uint64(i)},
			},
		}
		require.NoError(b, pool.rememberOne(tx.Sign(secrets[i%numAccounts])))
	}
	require.Equal(b, numTxns, pool.PendingCount())

	committed := map[transactions.Txid]ledgercore.IncludedTransactions{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.recomputeBlockEvaluator(committed, 0)
	}
	b.StopTimer()
	require.Equal(b, numTxns, pool.PendingCount())
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/numTxns, "ns/txn")
}
GOEOF

gofmt -l "$TEST" "$NEW" | grep . && { echo "gofmt disagrees with the patch" >&2; exit 1; } || true
echo "replaced BenchmarkTransactionPoolRecompute; new file $NEW"
