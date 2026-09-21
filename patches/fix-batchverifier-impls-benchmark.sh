#!/usr/bin/env bash
#
# crypto/batchverifier_bench_test.go, BenchmarkBatchVerifierImpls: before
# v5.0.0 each sub-benchmark made one BatchVerifier outside the timed loop and
# reused it for every iteration. A BatchVerifier accumulates what is enqueued
# until Verify, and nothing resets it here, so iteration i verified i*64
# signatures instead of 64. The reported ns/op therefore followed b.N, which
# follows -benchtime and the speed of the machine, and no two points of the
# series were measuring the same work.
#
# Upstream fixed it in v5.0.0 (a fresh verifier per iteration, passed as a
# constructor), which is why the series drops 98% at that tag: not a speedup,
# the benchmark starting to measure a 64-signature batch. This applies the same
# edit to the tags that carry the old shape, v4.4.1 through v4.7.4, so the
# points either side of v5.0.0 can be read as one series. The text of the
# function is identical across those tags.
#
# Already fixed at v5.0.0 and later, and the file does not exist before v4.4.1,
# so both are a no-op. This edits the throwaway checkout only; upstream needs
# no change.
set -eu

: "${CHECKOUT:?}"
cd "$CHECKOUT"

BENCH=crypto/batchverifier_bench_test.go

if [ ! -f "$BENCH" ]; then
  echo "skip: $BENCH absent at this ref"
  exit 0
fi
if ! grep -qF 'runImpl := func(b *testing.B, bv BatchVerifier,' "$BENCH"; then
  echo "skip: $BENCH already takes a constructor or differs"
  exit 0
fi

# The verifier moves inside the loop, so runImpl takes a constructor.
perl -0pi -e 's!\trunImpl := func\(b \*testing\.B, bv BatchVerifier,\n\t\tmsgs \[\]\[\]Hashable, pks \[\]\[\]SignatureVerifier, sigs \[\]\[\]Signature\) \{\n\t\tb\.ResetTimer\(\)\n\t\tfor i := 0; i < b\.N; i\+\+ \{\n\t\t\tbatchIdx := i % numBatches\n!\t// patched by benchspotter: a fresh verifier per iteration, as upstream does from v5.0.0.\n\trunImpl := func(b *testing.B, makeBV func() BatchVerifier,\n\t\tmsgs [][]Hashable, pks [][]SignatureVerifier, sigs [][]Signature) {\n\t\tb.ResetTimer()\n\t\tfor i := 0; i < b.N; i++ {\n\t\t\tbv := makeBV()\n\t\t\tbatchIdx := i % numBatches\n!' "$BENCH"

# The three call sites hand over the constructor instead of the verifier.
perl -0pi -e 's!\t\tbv := makeLibsodiumBatchVerifier\(batchSize\)\n\t\tbv\.\(\*cgoBatchVerifier\)\.useSingle = (true|false)\n\t\trunImpl\(b, bv, msgs, pks, sigs\)\n!\t\trunImpl(b, func() BatchVerifier {\n\t\t\tbv := makeLibsodiumBatchVerifier(batchSize)\n\t\t\tbv.(*cgoBatchVerifier).useSingle = $1\n\t\t\treturn bv\n\t\t}, msgs, pks, sigs)\n!g' "$BENCH"

perl -0pi -e 's!\t\tbv := makeEd25519ConsensusBatchVerifier\(batchSize\)\n\t\trunImpl\(b, bv, msgs, pks, sigs\)\n!\t\trunImpl(b, func() BatchVerifier { return makeEd25519ConsensusBatchVerifier(batchSize) }, msgs, pks, sigs)\n!' "$BENCH"

# Every call site has to have moved: a half-applied patch would not compile,
# and the compile gate would then drop the whole crypto package for this ref.
if grep -qF 'runImpl(b, bv, msgs, pks, sigs)' "$BENCH"; then
  echo "batchverifier patch applied only in part" >&2
  exit 1
fi
grep -qF 'bv := makeBV()' "$BENCH" || { echo "batchverifier patch did not apply" >&2; exit 1; }

gofmt -l "$BENCH" | grep . && { echo "gofmt disagrees with the patch" >&2; exit 1; } || true
echo "patched $BENCH"
