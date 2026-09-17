# benchspotter store

This is benchspotter's default store for this repo, so `benchspotter trend`, `session ls` and
`compare` work here with no flags or environment variables.

- `sessions/` - one directory per session, with `meta.json` (name, commit, tags, benchmark list,
  machine) and `results.bench` (raw `go test -bench` output).
- `sweep-logs/<timestamp>/` - logs from `run-sweep.sh`: `summary.tsv` (tag, commit date,
  toolchain, run, session id, seconds, status), the `run-sweep.sh` used, and per-tag build and
  bench logs.
- `nightly-logs/<timestamp>/` - the same, from `run-nightly.sh`.

Session tags carry what benchspotter cannot record itself. Its `go_version` field is
`runtime.Version()` of the benchspotter binary and is identical everywhere, so the toolchain that
built the code under test is a tag (`go1.21.10`), alongside the kind (`release`, `nightly`, or
`range` for a commit between two nightlies measured by the range workflow), `run1`..`runN`, the
machine (`runner:...`), the commit time (`commit:2026-09-15T19:19:14Z`, UTC, what the site orders
by) and `partial` for a session where one package failed.

Everything in this store was measured by GitHub Actions. The 2026-09-14 Apple M2 Pro sweep over
20 stable tags lives on the `archive/macbook-sweep` branch and is frozen there: different
hardware, so its numbers do not belong on the same axis as these.
