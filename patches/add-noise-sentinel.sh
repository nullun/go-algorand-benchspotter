#!/usr/bin/env bash
#
# Drop a benchmark into the checkout that does not depend on go-algorand at all.
# It hashes a fixed buffer and runs a fixed integer loop, so its night-to-night
# movement is the machine's, not upstream's. Every other series can be read
# against it: a step that the sentinel took too is the runner, not go-algorand.
#
# The package is test-only and imports nothing outside the standard library, so
# it compiles in a second at any ref. It is selected like everything else, via
# lib/benches.txt, and reported as NoiseSentinelHash and NoiseSentinelLoop.
#
# This edits the throwaway checkout only. Upstream go-algorand needs no change.
set -eu

: "${CHECKOUT:?}"
DIR="$CHECKOUT/benchsentinel"
mkdir -p "$DIR"

cat > "$DIR/sentinel_test.go" <<'EOF'
// Package benchsentinel is added to the checkout by go-algorand-benchspotter
// (patches/add-noise-sentinel.sh). It is not part of go-algorand.
package benchsentinel

import (
	"crypto/sha256"
	"testing"
)

var buf = func() []byte {
	b := make([]byte, 64<<10)
	var x uint32 = 0x9e3779b9
	for i := range b {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		b[i] = byte(x)
	}
	return b
}()

var sink uint64

// BenchmarkNoiseSentinelHash is memory-bandwidth and SHA-256 throughput: 64 KiB
// per op through the standard library, which uses the hardware instructions
// on every platform this repo runs on.
func BenchmarkNoiseSentinelHash(b *testing.B) {
	b.SetBytes(int64(len(buf)))
	for i := 0; i < b.N; i++ {
		s := sha256.Sum256(buf)
		sink += uint64(s[0])
	}
}

// BenchmarkNoiseSentinelLoop is a dependent integer chain with no memory
// traffic: clock frequency and scheduler pre-emption, nothing else.
func BenchmarkNoiseSentinelLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		x := uint64(i) | 1
		for k := 0; k < 4096; k++ {
			x = x*6364136223846793005 + 1442695040888963407
			x ^= x >> 29
		}
		sink += x
	}
}
EOF

echo "wrote $DIR/sentinel_test.go"
