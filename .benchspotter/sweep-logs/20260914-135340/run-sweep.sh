#!/usr/bin/env bash
#
# run-sweep.sh - benchspotter historical benchmark sweep over go-algorand
# stable releases, each built with the Go toolchain named in its go.mod.
#
# Usage:
#   ./run-sweep.sh                 # all stable tags that carry a toolchain directive, oldest-first
#   ./run-sweep.sh TAG [TAG...]    # only these tags (in the order given); used for smoke tests
#
# Environment overrides:
#   BENCHSPOTTER_PATH   result storage; MUST be outside this worktree (default below)
#   COUNT               -count passed to go test per session (default 4)
#   RUNS                benchspotter sessions recorded per tag, each a fresh process
#                       (default 3). Sessions share the tag as name and are tagged
#                       run1..runN, so `trend --tag run1` gives one clean series and
#                       `compare bench --session <id> ...` shows repeatability.
#   TOOLCHAIN           "pinned" (default) sets GOTOOLCHAIN=<go.mod toolchain directive> per tag,
#                       "auto" leaves GOTOOLCHAIN untouched. NOTE: with a local Go newer than the
#                       directive, "auto" builds every tag with the local Go: auto only ever steps
#                       UP to a newer toolchain, never down. Only "pinned" gives per-release toolchains.
#   BENCHES             space-separated benchmark regexps (default: the seven below)
#
# The worktree is restored to its starting ref on exit, including on Ctrl-C.
# The worktree must be clean at start because `git clean -xfdq` runs per tag.

set -u -o pipefail

WORKTREE="${SWEEP_WORKTREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
cd "$WORKTREE"

export BENCHSPOTTER_PATH="${BENCHSPOTTER_PATH:-$HOME/GitHub/algorand/go-algorand-benchspotter-results}"
COUNT="${COUNT:-4}"
RUNS="${RUNS:-3}"
TOOLCHAIN="${TOOLCHAIN:-pinned}"
BENCHES="${BENCHES:-BenchmarkTxn BenchmarkPaysetGroups BenchmarkCompactDeltas BenchmarkTxHandlerProcessing BenchmarkMerkleCommit BenchmarkAppendMsgSignedTxn BenchmarkAppendMsgBlockHeader}"
# Every benchmark above exists at all 20 tags and was run successfully at both
# v3.24.0-stable (go1.21.10) and v5.0.1-stable (go1.25.3) on 2026-09-14.
#
# Deliberately absent, all verified to fail and thereby abort the rest of the
# benchspotter session for that tag:
#   BenchmarkHandleTxnGroups      panics at every tag: runHandlerBenchmarkWithBacklog
#                                 never starts the ElasticRateLimiter, and
#                                 handler.Stop() then calls a nil ctxCancel.
#   BenchmarkBlockValidationMix   "proposer missing when payouts enabled" at both
#                                 ends: builds blocks under ConsensusFuture.
#   BenchmarkBlockEvaluatorRAM*   same error from v4.0.x on, once payouts became
#                                 part of ConsensusCurrentVersion.
#   BenchmarkPay / BenchmarkAppInt1  same error at v3.24.0 (ConsensusFuture).
#   BenchmarkPrefetcherPayment    fails at v3.24.0.
# Any block-building ledger benchmark needs an upstream fix (set a proposer)
# before it can be trended across this window.

# ---- preflight -------------------------------------------------------------

command -v benchspotter >/dev/null || { echo "benchspotter not on PATH" >&2; exit 1; }

mkdir -p "$BENCHSPOTTER_PATH"
BENCHSPOTTER_PATH="$(cd "$BENCHSPOTTER_PATH" && pwd)"
case "$BENCHSPOTTER_PATH" in
  "$WORKTREE"|"$WORKTREE"/*)
    echo "BENCHSPOTTER_PATH ($BENCHSPOTTER_PATH) is inside the worktree; git clean would delete it" >&2
    exit 1 ;;
esac

if [ -n "$(git status --porcelain)" ]; then
  echo "worktree is not clean; commit or remove changes first (git clean -xfdq runs per tag)" >&2
  exit 1
fi

START_REF="$(git symbolic-ref --quiet --short HEAD || git rev-parse HEAD)"
LOGDIR="${SWEEP_LOGDIR:-$BENCHSPOTTER_PATH/sweep-logs/$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$LOGDIR"
SUMMARY="$LOGDIR/summary.tsv"

# This file lives in the worktree and `git clean` will delete it mid-run, so
# re-exec from a copy in the log directory.
if [ -z "${SWEEP_WORKTREE:-}" ]; then
  cp "${BASH_SOURCE[0]}" "$LOGDIR/run-sweep.sh"
  SWEEP_WORKTREE="$WORKTREE" SWEEP_LOGDIR="$LOGDIR" exec bash "$LOGDIR/run-sweep.sh" "$@"
fi

printf 'tag\tcommit_date\ttoolchain\trun\tsession\tseconds\tstatus\n' > "$SUMMARY"

# ---- tag list --------------------------------------------------------------

if [ $# -gt 0 ]; then
  TAGS=("$@")
else
  TAGS=()
  # Oldest-first is mandatory: benchspotter orders the trend view by session
  # creation time, not by commit date.
  while IFS= read -r t; do
    if git show "$t:go.mod" 2>/dev/null | grep -q '^toolchain '; then
      TAGS+=("$t")
    fi
  done < <(git for-each-ref --sort=creatordate --format='%(refname:short)' refs/tags | grep -- '-stable$')
fi

BENCH_ARGS=()
# (/|$) also selects literal sub-benchmarks, which benchspotter lists as Parent/sub.
for b in $BENCHES; do BENCH_ARGS+=(--bench "^${b}(/|\$)"); done

# ---- cleanup ---------------------------------------------------------------

CHILD=""
SKIPPED=()
restore() {
  trap - EXIT INT TERM
  [ -n "$CHILD" ] && kill "$CHILD" 2>/dev/null
  echo
  echo "restoring worktree to $START_REF"
  git clean -xfdq
  git checkout --quiet "$START_REF"
  echo "logs and summary: $LOGDIR"
  if [ ${#SKIPPED[@]} -gt 0 ]; then
    echo "skipped tags (${#SKIPPED[@]}): ${SKIPPED[*]}"
  fi
}
trap restore EXIT
trap 'exit 130' INT TERM

# ---- sweep -----------------------------------------------------------------

echo "storage:   $BENCHSPOTTER_PATH"
echo "logs:      $LOGDIR"
echo "count:     $COUNT"
echo "runs:      $RUNS"
echo "toolchain: $TOOLCHAIN"
echo "tags (${#TAGS[@]}): ${TAGS[*]}"
echo

for tag in "${TAGS[@]}"; do
  start=$SECONDS
  cdate="$(git log -1 --format=%cs "$tag" 2>/dev/null || echo '?')"
  gov='?'; id='-'; run=''
  fail() {
    echo "  FAILED ($1) after $((SECONDS-start))s; see $LOGDIR/$tag.*.log"
    SKIPPED+=("$tag${run:+/run$run}")
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$tag" "$cdate" "$gov" "$run" "$id" "$((SECONDS-start))" "skipped:$1" >> "$SUMMARY"
  }

  echo "=== $tag ($cdate)"
  if ! git checkout --detach --quiet "$tag"; then fail checkout; continue; fi
  git clean -xfdq

  directive="$(awk '/^toolchain /{print $2}' go.mod)"
  if [ "$TOOLCHAIN" = pinned ] && [ -n "$directive" ]; then
    export GOTOOLCHAIN="$directive"
  else
    unset GOTOOLCHAIN
  fi

  gov="$(go version 2>"$LOGDIR/$tag.goversion.log" | awk '{print $3}')"
  if [ -z "$gov" ]; then fail go-version; continue; fi
  echo "  toolchain: $gov (go.mod directive: ${directive:-none})"

  # crypto/libs is gitignored, so git clean removed it; the crypto package needs it.
  os="$(./scripts/ostype.sh)"; arch="$(./scripts/archtype.sh)"
  if ! make "crypto/libs/$os/$arch/lib/libsodium.a" >"$LOGDIR/$tag.libsodium.log" 2>&1; then
    fail libsodium; continue
  fi
  echo "  libsodium built ($((SECONDS-start))s elapsed)"
  commit="$(git rev-parse --short HEAD)"

  # Each run is a separate benchspotter session and a fresh go test process
  # per package. The go build cache makes runs after the first cheap to start.
  for run in $(seq 1 "$RUNS"); do
    rstart=$SECONDS
    id='-'
    benchspotter bench --no-prompt --profile bench "${BENCH_ARGS[@]}" \
        --count "$COUNT" --name "$tag" --print-id \
        >"$LOGDIR/$tag.run$run.id" 2>"$LOGDIR/$tag.run$run.bench.log" &
    CHILD=$!
    wait "$CHILD"; rc=$?
    CHILD=""
    id="$(tr -d '[:space:]' <"$LOGDIR/$tag.run$run.id")"
    partial=""
    if [ $rc -ne 0 ] || [ -z "$id" ]; then
      # benchspotter keeps a partial session when one package fails, but prints
      # no id. Find it by name so the surviving benchmarks stay in the trend.
      id="$(benchspotter session ls -f json 2>/dev/null \
            | jq -r --arg n "$tag" '[.[] | select(.name==$n)] | sort_by(.time) | last | .id // empty')"
      if [ -z "$id" ]; then id='-'; fail "run$run bench rc=$rc"; continue; fi
      partial="; PARTIAL, a package failed, see sweep log"
      benchspotter session tag "$id" partial
      echo "  run $run: partial session $id kept"
    fi

    # benchspotter's own GoVersion field is runtime.Version() of benchspotter
    # itself, identical for every session, so record the real toolchain as a tag.
    # Upstream benchspotter also records the wrong commit from a git worktree
    # (fixed in the local build), so the note carries the real commit too.
    benchspotter session tag "$id" "$gov"
    benchspotter session tag "$id" release
    benchspotter session tag "$id" "run$run"
    benchspotter session note "$id" "$tag ($commit) built with $gov, run $run/$RUNS, count $COUNT$partial"

    if [ -n "$partial" ]; then
      fail "run$run bench rc=$rc"; continue
    fi
    echo "  run $run: session $id done in $((SECONDS-rstart))s"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$tag" "$cdate" "$gov" "$run" "$id" "$((SECONDS-rstart))" ok >> "$SUMMARY"
  done
  run=''
  echo "  $tag finished in $((SECONDS-start))s"
done

echo
echo "=== summary"
column -t -s $'\t' "$SUMMARY"
[ ${#SKIPPED[@]} -eq 0 ]
