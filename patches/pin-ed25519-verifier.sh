#!/usr/bin/env bash
#
# data/transactions/verify/txn_test.go (since v5.0.0, commit a7a983998f) picks
# the ed25519 batch verifier on a coin toss in init():
#
#     useEd25519Consensus := crypto.RandUint64()%2 == 0
#
# true selects pure-Go ed25519consensus, false the libsodium cgo path. The two
# are about 26us and 41us per BenchmarkTxn op (6 vs 8 allocs), so an unpatched
# master run reports a 60% regression and recovery on alternate days at random.
#
# Pin it. algod itself has defaulted to the Go implementation since v4.4.1
# (EnableBatchVerification), so true is what production runs; set
# BENCH_ED25519_CONSENSUS=false to trend the libsodium path instead.
#
# This edits the throwaway checkout only. Upstream go-algorand needs no change.
set -eu

: "${CHECKOUT:?}"
FILE="$CHECKOUT/data/transactions/verify/txn_test.go"
WANT="${BENCH_ED25519_CONSENSUS:-true}"
PATTERN='useEd25519Consensus := crypto.RandUint64()%2 == 0'

if [ ! -f "$FILE" ]; then
  echo "skip: $FILE does not exist at this ref"
  exit 0
fi

if ! grep -qF "$PATTERN" "$FILE"; then
  if grep -q 'useEd25519Consensus := \(true\|false\)' "$FILE"; then
    echo "skip: already pinned"
  else
    echo "skip: coin toss not present at this ref"
  fi
  exit 0
fi

perl -pi -e "s/\QuseEd25519Consensus := crypto.RandUint64()%2 == 0\E/useEd25519Consensus := $WANT \/\/ pinned by benchspotter/" "$FILE"
grep -n 'useEd25519Consensus :=' "$FILE"
echo "pinned useEd25519Consensus = $WANT"
