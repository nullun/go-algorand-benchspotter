#!/usr/bin/env bash
#
# lib/backfill-commit-times.sh - give every session that lacks one a
# commit:<utc timestamp> tag, looked up from the session's hash in the
# go-algorand checkout. Sessions recorded before 2026-09-17 have none; the
# scripts tag new sessions themselves (bs_session_meta in lib/common.sh).
# Safe to run again: a session that already has the tag is skipped.
set -u -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
. lib/common.sh
bs_preflight || exit 1
bs_clone || exit 1

n=0
while IFS=$'\t' read -r id commit; do
  t="$(bs_commit_time "$commit" 2>/dev/null)"
  if [ -z "$t" ]; then
    echo "no such commit in $CHECKOUT: $commit (session $id)" >&2
    continue
  fi
  benchspotter session tag "$id" "commit:$t" >/dev/null && n=$((n+1))
done < <(benchspotter session ls -f json \
  | jq -r '.[] | select(all(.tags[]; startswith("commit:") | not)) | [.id, .commit] | @tsv')
echo "tagged $n sessions"
