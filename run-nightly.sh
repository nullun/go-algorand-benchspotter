#!/usr/bin/env bash
#
# run-nightly.sh - one benchmark session set against a single go-algorand ref,
# by default whatever origin/master points at right now.
#
# Usage:
#   ./run-nightly.sh              # origin/master
#   ./run-nightly.sh <ref>        # any branch, tag or commit
#
# Environment overrides:
#   CHECKOUT      go-algorand clone to use (default ./checkout/go-algorand, created on demand)
#   REMOTE        clone source (default https://github.com/algorand/go-algorand.git)
#   RUNS          sessions recorded, each a fresh process (default 3)
#   COUNT         -count passed to go test per session (default 4)
#   TOOLCHAIN     "pinned" (default) uses the go.mod toolchain directive, "auto" the local Go
#   BENCHES       space-separated benchmark regexps (default: the set in lib/benches.txt)
#   RUNNER        label recorded as a session tag (default: runner:$(hostname -s))
#   KIND          session kind tag (default nightly). The range workflow passes "range" so
#                 its per-commit points can be told apart from the nightly series.
#   PROFILES      extra benchspotter profiles to record alongside bench, space-separated
#                 (cpu mem block mutex). Off by default: a profile is megabytes per session.
#   SKIP_IF_DONE  1 (default) exits 0 without benchmarking if this commit already has a
#                 $KIND session; 0 benchmarks it again
#
# Results land in $BENCHSPOTTER_PATH (default .benchspotter in this repo). The
# go-algorand checkout is destroyed and rebuilt per run; this repo is not touched.
set -u -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
. lib/common.sh

REF="${1:-origin/master}"
RUNS="${RUNS:-3}"
COUNT="${COUNT:-4}"
RUNNER="${RUNNER:-runner:$(hostname -s)}"
KIND="${KIND:-nightly}"
PROFILES="${PROFILES:-}"
SKIP_IF_DONE="${SKIP_IF_DONE:-1}"
# One name per line, trailing "# ..." comments dropped.
BENCHES="${BENCHES:-$(sed 's/#.*//' lib/benches.txt | awk 'NF {print $1}' | tr '\n' ' ')}"

bs_preflight || exit 1
bs_clone || { echo "clone/fetch failed" >&2; exit 1; }

LOGDIR="${LOGDIR:-$BENCHSPOTTER_PATH/nightly-logs/$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$LOGDIR"

FULLCOMMIT="$(git -C "$CHECKOUT" rev-parse "$REF")" || exit 1
if [ "$SKIP_IF_DONE" = 1 ] && benchspotter session ls -f json 2>/dev/null \
     | jq -e --arg c "$FULLCOMMIT" --arg k "$KIND" 'any(.[]; .commit == $c and (.tags | index($k)))' >/dev/null; then
  echo "$REF is $FULLCOMMIT, already benchmarked; nothing to do"
  exit 0
fi

echo "storage: $BENCHSPOTTER_PATH"
echo "logs:    $LOGDIR"
echo "ref:     $REF ($FULLCOMMIT)"

NAME="master-$(git -C "$CHECKOUT" log -1 --format=%cs "$FULLCOMMIT")"
if ! bs_prepare "$FULLCOMMIT" "$LOGDIR" "$NAME"; then
  echo "prepare failed for $REF; see $LOGDIR/$NAME.*.log" >&2
  exit 1
fi
echo "toolchain: $BS_GOVERSION (go.mod directive: ${BS_DIRECTIVE:-none})"

BENCH_ARGS=()
# (/|$) also selects literal sub-benchmarks, which benchspotter lists as Parent/sub.
for b in $BENCHES; do BENCH_ARGS+=(--bench "^${b}(/|\$)"); done
PROFILE_ARGS=(--profile bench)
for p in $PROFILES; do PROFILE_ARGS+=(--profile "$p"); done

rc_all=0
for run in $(seq 1 "$RUNS"); do
  start=$SECONDS
  # benchspotter resolves package directories relative to its own working
  # directory, so it has to run inside the checkout. BENCHSPOTTER_PATH is
  # exported, so the results still land in this repo's store.
  ( cd "$CHECKOUT" && benchspotter bench --no-prompt "${PROFILE_ARGS[@]}" "${BENCH_ARGS[@]}" \
      --count "$COUNT" --name "$NAME" --print-id ) \
      >"$LOGDIR/$NAME.run$run.id" 2>"$LOGDIR/$NAME.run$run.bench.log"
  rc=$?
  id="$(tr -d '[:space:]' <"$LOGDIR/$NAME.run$run.id")"
  partial=""
  if [ $rc -ne 0 ] || [ -z "$id" ]; then
    id="$(bs_last_session_id "$NAME")"
    if [ -z "$id" ]; then
      echo "  run $run: failed (rc=$rc), no session kept; see $LOGDIR/$NAME.run$run.bench.log" >&2
      rc_all=1
      continue
    fi
    partial="; PARTIAL, a package failed, see nightly log"
    benchspotter session tag "$id" partial
    rc_all=1
  fi
  bs_session_meta "$id" "$NAME" \
    "$REF ($BS_COMMIT, $BS_COMMIT_DATE) built with $BS_GOVERSION, run $run/$RUNS, count $COUNT$partial" \
    "$KIND" "run$run" "$BS_GOVERSION" "$RUNNER"
  echo "  run $run: session $id done in $((SECONDS-start))s${partial:+ (partial)}"
done

git -C "$CHECKOUT" checkout --quiet --detach "$FULLCOMMIT" 2>/dev/null
exit $rc_all
