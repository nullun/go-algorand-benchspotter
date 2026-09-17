#!/usr/bin/env bash
#
# protocol/encodebench_test.go holds BenchmarkCodecEncoder, which encodes nil
# and segfaults in protocol.Encode, failing the protocol package under
# go test -bench. Repaired it would time the encoding of an empty struct,
# which every generated msgp_gen_test.go benchmark already does for a real
# type. Remove it. The same change is a commit on the bench/fix-broken-benchmarks
# branch of the go-algorand clone. This edits the throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
FILE="$CHECKOUT/protocol/encodebench_test.go"

if [ ! -f "$FILE" ]; then
  echo "skip: $FILE does not exist at this ref"
  exit 0
fi
if ! grep -q 'func BenchmarkCodecEncoder' "$FILE"; then
  echo "skip: $FILE does not hold BenchmarkCodecEncoder"
  exit 0
fi
rm "$FILE"
echo "removed $FILE"
