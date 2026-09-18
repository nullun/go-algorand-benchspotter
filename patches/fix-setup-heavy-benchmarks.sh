#!/usr/bin/env bash
#
# Two upstream benchmarks whose setup, not their timed loop, is most of what
# they cost, and whose numbers depend on b.N. Both edits are the shape upstream
# already uses elsewhere (BenchmarkBlockEvaluatorDiskAppCalls pins b.N the
# same way), so each is a copy and a PR away.
#
# ledger/evalbench_test.go, benchmarkBlockEvaluator: about 6 s of setup
# (50k signed payments, an in-memory ledger) before the ~0.5 s/op timed
# loop. Go runs the function once with b.N=1, sees 0.5 s, and runs it again
# with a larger b.N, so the setup runs twice and the first, cold iteration is
# the one measured when -benchtime is short. Pinning b.N to at least 2 runs
# the setup once and reports the warm iteration.
#
# data/transactions/verify/txn_test.go, BenchmarkPaysetGroups: generates and
# signs b.N transactions in setup, then verifies them once. Both the setup
# cost and the ns/op (the execpool amortises differently over 2000 and 40000
# txns) follow b.N, which follows -benchtime and the speed of the machine.
# Generate a fixed 2000 txns and verify them as many times as b.N needs, with
# a fresh cache per pass so nothing is short-circuited.
#
# This edits the throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
cd "$CHECKOUT"

EVAL=ledger/evalbench_test.go
VERIFY=data/transactions/verify/txn_test.go

if [ -f "$EVAL" ] && grep -qF 'func benchmarkBlockEvaluator(b *testing.B' "$EVAL"; then
  if grep -qF 'patched by benchspotter: setup once' "$EVAL"; then
    echo "skip: $EVAL already patched"
  else
    perl -0pi -e 's{(func benchmarkBlockEvaluator\(b \*testing\.B[^\n]*\{\n)}{$1\t// patched by benchspotter: setup once, and the timed iteration is the warm one.\n\tif b.N < 2 {\n\t\tb.N = 2 //nolint:staticcheck // intentionally setting b.N\n\t}\n}' "$EVAL"
    echo "patched $EVAL"
  fi
else
  echo "skip: $EVAL absent or differs"
fi

if [ -f "$VERIFY" ] && grep -qF '_, signedTxn, secrets, addrs := generateTestObjects(b.N, 20, 0, 50)' "$VERIFY"; then
  perl -0pi -e 's{\tif b\.N < 2000 \{\n\t\tb\.N = 2000 //nolint:staticcheck // intentionally setting b\.N\n\t\}\n\t_, signedTxn, secrets, addrs := generateTestObjects\(b\.N, 20, 0, 50\)\n}{\t// patched by benchspotter: a fixed 2000 txns, verified as many times as b.N needs.\n\tconst paysetGroupsTxns = 2000\n\tif b.N < paysetGroupsTxns {\n\t\tb.N = paysetGroupsTxns //nolint:staticcheck // intentionally setting b.N\n\t}\n\t_, signedTxn, secrets, addrs := generateTestObjects(paysetGroupsTxns, 20, 0, 50)\n}' "$VERIFY"
  perl -0pi -e 's{\tcache := MakeVerifiedTransactionCache\(50000\)\n\n\tb\.ResetTimer\(\)\n\terr := PaysetGroups\(context\.Background\(\), txnGroups, blkHdr, verificationPool, cache, nil\)\n\trequire\.NoError\(b, err\)\n\tb\.StopTimer\(\)\n}{\tb.ResetTimer()\n\tfor done := 0; done < b.N; done += paysetGroupsTxns {\n\t\tcache := MakeVerifiedTransactionCache(50000)\n\t\terr := PaysetGroups(context.Background(), txnGroups, blkHdr, verificationPool, cache, nil)\n\t\trequire.NoError(b, err)\n\t}\n\tb.StopTimer()\n}' "$VERIFY"
  grep -qF 'for done := 0; done < b.N; done += paysetGroupsTxns' "$VERIFY" || { echo "PaysetGroups patch did not apply" >&2; exit 1; }
  echo "patched $VERIFY"
else
  echo "skip: $VERIFY absent, already patched or differs"
fi

gofmt -l $( [ -f "$EVAL" ] && echo "$EVAL" ) $( [ -f "$VERIFY" ] && echo "$VERIFY" ) | grep . && { echo "gofmt disagrees with the patch" >&2; exit 1; } || true
