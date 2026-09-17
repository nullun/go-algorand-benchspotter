#!/usr/bin/env bash
#
# lib/backfill-tags.sh - give every session that lacks them the tags that are
# looked up from the session's hash in the go-algorand checkout: commit:<utc
# timestamp> and consensus:<vNN>. Sessions recorded before 2026-09-17 have
# neither; the scripts tag new sessions themselves (bs_session_meta in
# lib/common.sh). Safe to run again: a tag a session already has is skipped.
set -u -o pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
. lib/common.sh
bs_preflight || exit 1
bs_clone || exit 1

n=0
while IFS=$'\t' read -r id commit tags; do
  case "$tags" in *commit:*) ;; *)
    t="$(bs_commit_time "$commit" 2>/dev/null)"
    if [ -z "$t" ]; then echo "no such commit in $CHECKOUT: $commit (session $id)" >&2; continue; fi
    benchspotter session tag "$id" "commit:$t" >/dev/null && n=$((n+1)) ;;
  esac
  case "$tags" in *consensus:*) ;; *)
    v="$(bs_consensus_version "$commit")"
    if [ -z "$v" ]; then echo "no ConsensusCurrentVersion at $commit (session $id)" >&2; continue; fi
    benchspotter session tag "$id" "consensus:$v" >/dev/null && n=$((n+1)) ;;
  esac
done < <(benchspotter session ls -f json | jq -r '.[] | [.id, .commit, (.tags | join(","))] | @tsv')
echo "added $n tags"
