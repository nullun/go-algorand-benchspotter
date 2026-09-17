#!/usr/bin/env bash
#
# data/txHandler_test.go: two benchmark harnesses emulate TxHandler.Start()
# by hand and leave the ElasticRateLimiter unstarted, then call handler.Stop(),
# which stops the rate limiter, whose congestion manager cancels a context it
# never created: nil pointer dereference in redCongestionManager.Stop
# (util/rateLimit.go). It takes the whole data test binary down, and with it
# BenchmarkHandleTxns, HandleTxnGroups, HandleMsig*, HandleBLW* and
# HandleLsigTxnGroups.
#
# Start the rate limiter where the harness starts the stream verifier. The
# same change is a commit on the bench/fix-broken-benchmarks branch of the
# go-algorand clone, for an upstream PR. This edits the throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
FILE="$CHECKOUT/data/txHandler_test.go"

if [ ! -f "$FILE" ]; then
  echo "skip: $FILE does not exist at this ref"
  exit 0
fi
if ! grep -q 'erl  *\*util.ElasticRateLimiter' "$CHECKOUT/data/txHandler.go"; then
  echo "skip: no rate limiter on TxHandler at this ref"
  exit 0
fi
if grep -qF 'handler.erl.Start() // patched by benchspotter' "$FILE"; then
  echo "skip: already patched"
  exit 0
fi

# Both harnesses have the same two lines back to back: start the stream
# verifier, blank line, make the result channel.
perl -0pi -e 's{(\thandler\.streamVerifier\.Start\(handler\.ctx\)\n)(\n\ttestResultChan := make\(chan \*txBacklogMsg, 10\))}{$1\tif handler.erl != nil {\n\t\thandler.erl.Start() // patched by benchspotter\n\t}\n$2}g' "$FILE"
n="$(grep -c 'handler.erl.Start() // patched by benchspotter' "$FILE")"
echo "patched $n harness(es) in $FILE"
[ "$n" -gt 0 ] || { echo "skip: harness shape differs at this ref"; exit 0; }
