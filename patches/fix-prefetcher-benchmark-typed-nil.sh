#!/usr/bin/env bash
#
# ledger/eval/prefetcher/prefetcher_test.go ends BenchmarkPrefetcherPayment and
# BenchmarkPrefetcherApps with require.NoError(b, k.Err). From v3.17.0 to
# v4.5.1 the Err field is a *GroupTaskError, so a nil one arrives at NoError
# as a non-nil error interface holding a nil pointer and every group fails the
# benchmark with "Received unexpected error: <nil>", which takes the whole
# package's session with it. v4.6.0 made the field an error and the benchmarks
# pass there unchanged.
#
# Replace the assertion with a plain nil check at refs where the field is still
# a pointer. Nothing to send upstream: master has the interface type.
set -eu

: "${CHECKOUT:?}"
cd "$CHECKOUT"

TEST=ledger/eval/prefetcher/prefetcher_test.go
IMPL=ledger/eval/prefetcher/prefetcher.go

if [ ! -f "$TEST" ] || [ ! -f "$IMPL" ]; then
  echo "skip: prefetcher package not present at this ref"
  exit 0
fi

if ! grep -qE '^\s+Err\s+\*GroupTaskError' "$IMPL"; then
  echo "skip: Err is not a pointer at this ref"
  exit 0
fi

if ! grep -qF 'require.NoError(b, k.Err)' "$TEST"; then
  echo "skip: $TEST differs"
  exit 0
fi

perl -0pi -e 's{^(\t+)require\.NoError\(b, k\.Err\)\n}{$1if k.Err != nil { // patched by benchspotter: a nil *GroupTaskError is a non-nil error\n$1\tb.Fatal(k.Err)\n$1}\n}mg' "$TEST"
echo "patched $TEST"

gofmt -l "$TEST" | grep . && { echo "gofmt disagrees with the patch" >&2; exit 1; } || true
