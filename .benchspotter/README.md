# benchspotter store

This is benchspotter's default store for this repo, so `benchspotter trend`, `session ls` and
`compare` work here with no flags or environment variables.

- `sessions/` - one directory per session, with `meta.json` (name, commit, tags, benchmark list,
  machine) and `results.bench` (raw `go test -bench` output).
- `sweep-logs/<timestamp>/` - logs from `run-sweep.sh`. `20260914-165937` is the full 20-tag
  sweep and holds `summary.tsv` (tag, commit date, toolchain, run, session id, seconds, status),
  the `run-sweep.sh` used, `overnight.log`, and per-tag build and bench logs. The earlier
  directories are the shorter debugging runs that preceded it.
- `nightly-logs/<timestamp>/` - the same, from `run-nightly.sh`.

Session tags carry what benchspotter cannot record itself. Its `go_version` field is
`runtime.Version()` of the benchspotter binary and is identical everywhere, so the toolchain that
built the code under test is a tag (`go1.21.10`), alongside `release` or `nightly`, `run1`..`runN`,
the machine (`runner:...`) and `partial` for a session where one package failed. The 60 archived
sweep sessions were tagged `runner:Steves-MacBook-Pro` after the fact, since they predate the tag
and all ran on that machine.
