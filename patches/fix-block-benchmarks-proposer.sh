#!/usr/bin/env bash
#
# ledger/evalbench_test.go and ledger/fullblock_perf_test.go build blocks with
# GenerateBlock and never finish them, so the block has no proposer. Payouts
# have been part of ConsensusCurrentVersion since v40, and the validate path
# (Ledger.Validate, Ledger.AddBlock, Eval with validate=true) rejects such a
# block with "proposer missing when payouts enabled". That fails
# BenchmarkBlockEvaluatorRAMCrypto, DiskCrypto and all four BenchmarkBlockValidation*.
#
# Finish each block with a placeholder proposer, as ledger/ledger_perf_test.go
# already does. An unfunded proposer is ineligible, so the payout stays zero.
#
# ledger/ledger_perf_test.go itself (BenchmarkAppInt1 and the other full-block
# benchmarks) only finishes its blocks from v4.0.1 on. The v3 tags run them
# under ConsensusFuture, where payouts were already enabled, and add the
# unfinished block, so the same fix is applied there too; at later tags the
# file already has it and is left alone.
#
# fullblock_perf_test.go also prints progress with fmt.Printf. When Go runs a
# second round of iterations that lands inside the result line and benchspotter
# drops the result. go test merges the binary's stderr into stdout, so the
# progress goes through b.Logf, which is held back until after the result line.
#
# The same change is a commit on the bench/fix-broken-benchmarks branch of the
# go-algorand clone, for an upstream PR. This edits the throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
cd "$CHECKOUT"

EVAL=ledger/evalbench_test.go
FULL=ledger/fullblock_perf_test.go
PERF=ledger/ledger_perf_test.go
COMMITTEE='"github.com/algorand/go-algorand/data/committee"'

if [ ! -f "$EVAL" ] || [ ! -f "$FULL" ]; then
  echo "skip: benchmark files not present at this ref"
  exit 0
fi

if grep -qF 'MakeValidatedBlock(unfinishedBlock.UnfinishedBlock()' "$EVAL"; then
  # Two sites: the setup blocks ("vb :=", indented four tabs) and the timed
  # block ("validatedBlock :=", one tab). Same replacement, indentation kept.
  perl -0pi -e 's{^(\t+)(\w+) := ledgercore\.MakeValidatedBlock\(unfinishedBlock\.UnfinishedBlock\(\), unfinishedBlock\.UnfinishedDeltas\(\)\)\n}{$1blk := unfinishedBlock.FinishBlock(committee.Seed{0x01}, basics.Address{0x01}, false) // patched by benchspotter\n$1deltas := unfinishedBlock.UnfinishedDeltas()\n$1deltas.Hdr = &blk.BlockHeader\n$1$2 := ledgercore.MakeValidatedBlock(blk, deltas)\n}mg' "$EVAL"
  grep -qF "$COMMITTEE" "$EVAL" || perl -0pi -e 's{(\t"github.com/algorand/go-algorand/data/bookkeeping"\n)}{$1\t"github.com/algorand/go-algorand/data/committee"\n}' "$EVAL"
  echo "patched $EVAL"
else
  echo "skip: $EVAL already finishes its blocks or differs"
fi

if grep -qF 'bc.l0.AddBlock(vblk.UnfinishedBlock(), cert)' "$FULL"; then
  perl -0pi -e 's{\tbc\.blocks = append\(bc\.blocks, vblk\.UnfinishedBlock\(\)\)\n\n\terr = bc\.l0\.AddBlock\(vblk\.UnfinishedBlock\(\), cert\)\n}{\tblk := vblk.FinishBlock(committee.Seed{0x01}, basics.Address{0x01}, false) // patched by benchspotter\n\tbc.blocks = append(bc.blocks, blk)\n\n\terr = bc.l0.AddBlock(blk, cert)\n}' "$FULL"
  grep -qF "$COMMITTEE" "$FULL" || perl -0pi -e 's{(\t"github.com/algorand/go-algorand/data/bookkeeping"\n)}{$1\t"github.com/algorand/go-algorand/data/committee"\n}' "$FULL"
  echo "patched $FULL"
else
  echo "skip: $FULL already finishes its blocks or differs"
fi

if grep -qF $'\tfmt.Printf(' "$FULL"; then
  perl -pi -e 's{^(\t+)fmt\.Printf\(}{$1b.Logf(}' "$FULL"
  echo "progress output of $FULL sent through b.Logf"
fi

if [ -f "$PERF" ] && grep -qF 'l0.AddBlock(lvb.UnfinishedBlock(), cert)' "$PERF"; then
  # Finish the block once after GenerateBlock, then use it at the three sites
  # that took the unfinished block (the two AddBlock calls and the replay list).
  perl -0pi -e 's{^(\t+)lvb, err := eval\.GenerateBlock\(nil\)\n\1require\.NoError\(b, err\)\n}{$1lvb, err := eval.GenerateBlock(nil)\n$1require.NoError(b, err)\n\n$1fb := lvb.FinishBlock(committee.Seed{0x01}, basics.Address{0x01}, false) // patched by benchspotter\n}m' "$PERF"
  perl -pi -e 's{lvb\.UnfinishedBlock\(\)}{fb}g' "$PERF"
  grep -qF "$COMMITTEE" "$PERF" || perl -0pi -e 's{(\t"github.com/algorand/go-algorand/data/bookkeeping"\n)}{$1\t"github.com/algorand/go-algorand/data/committee"\n}' "$PERF"
  echo "patched $PERF"
else
  echo "skip: $PERF already finishes its blocks or differs"
fi

gofmt -l "$EVAL" "$FULL" "$PERF" | grep . && { echo "gofmt disagrees with the patch" >&2; exit 1; } || true
