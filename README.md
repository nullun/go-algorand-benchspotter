# go-algorand-benchspotter

Benchmark history for [algorand/go-algorand](https://github.com/algorand/go-algorand), recorded
with [benchspotter](https://github.com/MichaelMure/benchspotter) and kept here rather than in a
fork of go-algorand: the data grows on its own schedule, and none of it belongs in a branch that
has to stay rebaseable on upstream master.

go-algorand needs no changes for any of this. Both scripts clone it read-only into a throwaway
checkout they own, and the source edits a stable benchmark run needs are applied there (see
`patches/`).

## What is here

- `.benchspotter/` - the benchspotter store, and the point of the repo. `benchspotter trend`,
  `session ls` and `compare` work from a checkout of this repo with no flags.
- `run-nightly.sh` - one set of sessions against a single ref, by default `origin/master`.
- `run-sweep.sh` - the historical sweep over the stable tags that carry a go.mod toolchain
  directive, each built with the toolchain it names.
- `lib/benches.txt` - the benchmark set, with notes on what is deliberately excluded and why.
- `patches/` - edits applied to the checkout before benchmarking.
- `site/` - a static trend viewer published to GitHub Pages.

## Running it

Needs `benchspotter`, `jq`, `git`, `make`, a C toolchain (libsodium) and any recent Go. The
go.mod toolchain directive of the ref under test decides the Go that actually compiles it.

    ./run-nightly.sh                      # origin/master
    ./run-nightly.sh v5.0.1-stable        # any ref
    ./run-sweep.sh                        # every stable tag, oldest first, ~22 min each
    ./run-sweep.sh v5.0.0-stable v5.0.1-stable

Both take `RUNS`, `COUNT`, `BENCHES`, `TOOLCHAIN`, `CHECKOUT` and `REMOTE`; the header of each
script lists them. Results go to `.benchspotter` unless `BENCHSPOTTER_PATH` says otherwise.

    benchspotter trend --tag run1         # one clean series per release
    benchspotter trend --tag nightly
    benchspotter session ls

## The nightly job

`.github/workflows/nightly.yml` runs at 02:00 UTC, benchmarks whatever `origin/master` points
at, commits the sessions and triggers the site rebuild. It exits without benchmarking if that
commit already has a nightly session, so a quiet day costs nothing.

The runner matters more than anything else in this repo. Set the `BENCH_RUNNER` repository
variable to a self-hosted label on a machine you control and that is otherwise idle at 02:00.
On a GitHub-hosted runner the neighbours on the same host move the numbers by more than most
real regressions do: the `AppendMsgBlockHeader` +14% at v4.2.1 and the `processDecoded`
23us -> 9.7us at v3.27.0 that this archive found would both vanish into that noise. Sessions are
tagged `runner:<label>`, so filter a trend to one machine before reading anything into it.

A self-hosted runner also needs `jq` and a C toolchain; the workflow installs benchspotter itself.

## The site

`site/build.py` reads the store and writes `site/dist/{index.html,data.json}`; it uses the Python
standard library only and does not need the benchspotter binary.

    uv run site/build.py && open site/dist/index.html

`.github/workflows/pages.yml` publishes it. Enable Pages for the repo with "GitHub Actions" as
the source. The nightly job calls that workflow directly, because a push made with
`GITHUB_TOKEN` does not trigger other workflows.

## Reading the numbers

The archive in `.benchspotter` is 60 sessions: 20 `vX.Y.Z-stable` tags (v3.24.0 to v5.0.1,
2024-05 to 2026-08) x 3 runs, `-count=4`, all on one Apple M2 Pro on 2026-09-14. Run-to-run
spread inside a single tag is the yardstick for whether a step between tags means anything, and
in this archive it is not constant: the first six tags ran while the machine was in use and
`MerkleCommit` moves 7-25% between repeats of the same tag there, against 0.3-2.5% for the last
six, which ran overnight. Read nothing into a step smaller than the spread of its own tag.

Known artefacts, both worth checking before believing a jump:

- `data/transactions/verify/txn_test.go` has picked the ed25519 batch verifier on a coin toss in
  `init()` since v5.0.0 (commit a7a983998f). Pure-Go ed25519consensus is about 26us per
  `BenchmarkTxn` op with 6 allocs, libsodium cgo about 41us with 8. `patches/pin-ed25519-verifier.sh`
  pins it to the Go implementation, which is what algod has defaulted to since v4.4.1. The v3 and
  v4 sessions in the archive predate the patch and are mostly libsodium numbers, so they are not
  comparable to production and not comparable to each other tag by tag.
- `MerkleCommit` is recorded for `sha512_256` only from 2026-09-15 on. The benchmark's
  3 x 5 x 5 grid of hash x Item x Count produced 45 series that all moved together, and cost
  most of the run time of a session. The `sha256` and `sumhash` series stay in the archive and
  the site marks them as no longer recorded.
- `AppendMsgBlockHeader` steps +14% at v4.2.1 because msgp_gen was regenerated for a larger block
  header, not because anything got slower. `AppendMsgSignedTxn` is flat across the same commit,
  which is how you can tell.
