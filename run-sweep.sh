#!/usr/bin/env bash
#
# run-sweep.sh - benchspotter historical benchmark sweep over go-algorand stable
# releases, each built with the Go toolchain named in its go.mod.
#
# Usage:
#   ./run-sweep.sh                 # all stable tags that carry a toolchain directive, oldest-first
#   ./run-sweep.sh TAG [TAG...]    # only these tags (in the order given); used for smoke tests
#
# Environment overrides:
#   CHECKOUT      go-algorand clone to use (default ./checkout/go-algorand, created on demand)
#   REMOTE        clone source (default https://github.com/algorand/go-algorand.git)
#   COUNT         -count passed to go test per session (default 4)
#   RUNS          sessions recorded per tag, each a fresh process (default 3). Sessions share
#                 the tag as name and are tagged run1..runN, so `trend --tag run1` gives one
#                 clean series and `compare bench --session <id> ...` shows repeatability.
#   TOOLCHAIN     "pinned" (default) sets GOTOOLCHAIN=<go.mod toolchain directive> per tag,
#                 "auto" leaves GOTOOLCHAIN untouched. NOTE: with a local Go newer than the
#                 directive, "auto" builds every tag with the local Go: auto only ever steps
#                 UP to a newer toolchain, never down. Only "pinned" gives per-release toolchains.
#   BENCHES       space-separated benchmark regexps (default: the set in lib/benches.txt)
#   RUNNER        label recorded as a session tag (default: runner:$(hostname -s))
#   SKIP_IF_DONE  1 (default) drops tags that already have a release session; 0 benchmarks
#                 them again. Applies to an explicit tag list too.
#   MAX_TAGS      benchmark at most this many tags and leave the rest (default 0, no limit).
#                 With SKIP_IF_DONE=1 a backlog clears over as many runs as it takes, oldest
#                 first, which is how a sweep fits under a CI job time limit.
#
# The sweep runs `git clean -xfdq` and `git checkout --detach` inside $CHECKOUT,
# which it owns. This repo is only written to under $BENCHSPOTTER_PATH.
#
# One full sweep is roughly 21-24 minutes per tag with RUNS=3, COUNT=4.
set -u -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
. lib/common.sh

COUNT="${COUNT:-4}"
RUNS="${RUNS:-3}"
RUNNER="${RUNNER:-runner:$(hostname -s)}"
SKIP_IF_DONE="${SKIP_IF_DONE:-1}"
MAX_TAGS="${MAX_TAGS:-0}"
# One name per line, trailing "# ..." comments dropped.
BENCHES="${BENCHES:-$(sed 's/#.*//' lib/benches.txt | awk 'NF {print $1}' | tr '\n' ' ')}"

bs_preflight || exit 1
bs_clone || { echo "clone/fetch failed" >&2; exit 1; }

LOGDIR="${LOGDIR:-$BENCHSPOTTER_PATH/sweep-logs/$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$LOGDIR"
SUMMARY="$LOGDIR/summary.tsv"
cp "$0" "$LOGDIR/run-sweep.sh"
printf 'tag\tcommit_date\ttoolchain\trun\tsession\tseconds\tstatus\n' > "$SUMMARY"

# ---- tag list --------------------------------------------------------------

if [ $# -gt 0 ]; then
  TAGS=("$@")
else
  TAGS=()
  # Oldest first. The site orders points by commit time (a session tag), so
  # this is only for benchspotter's own trend view, which orders by record time.
  while IFS= read -r t; do
    if git -C "$CHECKOUT" show "$t:go.mod" 2>/dev/null | grep -q '^toolchain '; then
      TAGS+=("$t")
    fi
  done < <(git -C "$CHECKOUT" for-each-ref --sort=creatordate --format='%(refname:short)' refs/tags | grep -- '-stable$')
fi

# A sweep is ~22 minutes a tag, so re-measuring a tag that already has a release
# session is the expensive mistake here, not skipping one. The nightly job makes
# the same check against its own commit; see SKIP_IF_DONE in run-nightly.sh.
if [ "$SKIP_IF_DONE" = 1 ] && [ ${#TAGS[@]} -gt 0 ]; then
  DONE="$(benchspotter session ls -f json 2>/dev/null \
    | jq -r '[.[] | select(.tags | index("release")) | .name] | unique[]?')"
  KEEP=()
  for t in "${TAGS[@]}"; do
    if grep -qxF "$t" <<<"$DONE"; then
      echo "already benchmarked, skipping: $t"
    else
      KEEP+=("$t")
    fi
  done
  # bash 3.2 (macOS /bin/bash) treats "${KEEP[@]}" on an empty array as unset.
  TAGS=(${KEEP[@]+"${KEEP[@]}"})
fi

if [ ${#TAGS[@]} -eq 0 ]; then
  echo "nothing to benchmark"
  exit 0
fi

# Oldest first, so a batched backlog records sessions in tag order for
# benchspotter's own trend view; the site sorts by commit time regardless.
if [ "$MAX_TAGS" -gt 0 ] && [ ${#TAGS[@]} -gt "$MAX_TAGS" ]; then
  echo "${#TAGS[@]} tags to do, taking the oldest $MAX_TAGS this run"
  TAGS=("${TAGS[@]:0:$MAX_TAGS}")
fi

BENCH_ARGS=()
# (/|$) also selects literal sub-benchmarks, which benchspotter lists as Parent/sub.
for b in $BENCHES; do BENCH_ARGS+=(--bench "^${b}(/|\$)"); done

# ---- cleanup ---------------------------------------------------------------

CHILD=""
SKIPPED=()
finish() {
  trap - EXIT INT TERM
  [ -n "$CHILD" ] && kill "$CHILD" 2>/dev/null
  echo
  echo "logs and summary: $LOGDIR"
  if [ ${#SKIPPED[@]} -gt 0 ]; then
    echo "skipped tags (${#SKIPPED[@]}): ${SKIPPED[*]}"
  fi
}
trap finish EXIT
trap 'exit 130' INT TERM

# ---- sweep -----------------------------------------------------------------

echo "storage:   $BENCHSPOTTER_PATH"
echo "checkout:  $CHECKOUT"
echo "logs:      $LOGDIR"
echo "count:     $COUNT"
echo "runs:      $RUNS"
echo "toolchain: $TOOLCHAIN"
echo "tags (${#TAGS[@]}): ${TAGS[*]}"
echo

for tag in "${TAGS[@]}"; do
  start=$SECONDS
  run=''; id='-'
  BS_GOVERSION='?'; BS_COMMIT='?'; BS_COMMIT_DATE='?'
  fail() {
    echo "  FAILED ($1) after $((SECONDS-start))s; see $LOGDIR/$tag.*.log"
    SKIPPED+=("$tag${run:+/run$run}")
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$tag" "$BS_COMMIT_DATE" "$BS_GOVERSION" "$run" "$id" "$((SECONDS-start))" "skipped:$1" >> "$SUMMARY"
  }

  echo "=== $tag"
  if ! bs_prepare "$tag" "$LOGDIR" "$tag"; then fail prepare; continue; fi
  echo "  $BS_COMMIT ($BS_COMMIT_DATE), toolchain $BS_GOVERSION (go.mod directive: ${BS_DIRECTIVE:-none}), prepared in $((SECONDS-start))s"

  # Each run is a separate benchspotter session and a fresh go test process per
  # package. The go build cache makes runs after the first cheap to start.
  for run in $(seq 1 "$RUNS"); do
    rstart=$SECONDS
    id='-'
    # benchspotter resolves package directories relative to its own working
    # directory, so it has to run inside the checkout. BENCHSPOTTER_PATH is
    # exported, so the results still land in this repo's store.
    ( cd "$CHECKOUT" && benchspotter bench --no-prompt --profile bench "${BENCH_ARGS[@]}" \
        --count "$COUNT" --name "$tag" --print-id ) \
        >"$LOGDIR/$tag.run$run.id" 2>"$LOGDIR/$tag.run$run.bench.log" &
    CHILD=$!
    wait "$CHILD"; rc=$?
    CHILD=""
    id="$(tr -d '[:space:]' <"$LOGDIR/$tag.run$run.id")"
    partial=""
    if [ $rc -ne 0 ] || [ -z "$id" ]; then
      id="$(bs_last_session_id "$tag")"
      if [ -z "$id" ]; then id='-'; fail "run$run bench rc=$rc"; continue; fi
      partial="; PARTIAL, a package failed, see sweep log"
      benchspotter session tag "$id" partial
      echo "  run $run: partial session $id kept"
    fi

    bs_session_meta "$id" "$tag" \
      "$tag ($BS_COMMIT) built with $BS_GOVERSION, run $run/$RUNS, count $COUNT$partial" \
      release "run$run" "$BS_GOVERSION" "$RUNNER"

    if [ -n "$partial" ]; then fail "run$run bench rc=$rc"; continue; fi
    echo "  run $run: session $id done in $((SECONDS-rstart))s"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$tag" "$BS_COMMIT_DATE" "$BS_GOVERSION" "$run" "$id" "$((SECONDS-rstart))" ok >> "$SUMMARY"
  done
  run=''
  echo "  $tag finished in $((SECONDS-start))s"
done

echo
echo "=== summary"
column -t -s $'\t' "$SUMMARY"
[ ${#SKIPPED[@]} -eq 0 ]
