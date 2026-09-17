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
- `lib/benches.txt` - the benchmark set, 45 names with a note on what each measures.
- `lib/check-steps.py` - compares the newest point with the ones before it; the nightly runs it.
- `patches/` - edits applied to the checkout before benchmarking, and the noise sentinel.
- `site/` - a static trend viewer published to GitHub Pages.
- `ROADMAP.md` - the survey of every benchmark upstream: what is tracked, what was left out
  and why, what is broken at master, and what is worth writing next.

## Running it

Needs `benchspotter`, `jq`, `git`, `make`, a C toolchain (libsodium) and any recent Go. The
go.mod toolchain directive of the ref under test decides the Go that actually compiles it.

    ./run-nightly.sh                      # origin/master
    ./run-nightly.sh v5.0.1-stable        # any ref
    ./run-sweep.sh                        # every stable tag, oldest first, ~22 min each
    ./run-sweep.sh v5.0.0-stable v5.0.1-stable

Both take `RUNS`, `COUNT`, `BENCHES`, `TOOLCHAIN`, `CHECKOUT`, `REMOTE` and `SKIP_IF_DONE`, and
the nightly also `KIND` and `PROFILES`; the header of each script lists them. `SKIP_IF_DONE=1` is the default and drops work that is already
in the store: a commit with a `nightly` session, a tag with a `release` session. Pass
`SKIP_IF_DONE=0` to measure something a second time. Results go to `.benchspotter` unless
`BENCHSPOTTER_PATH` says otherwise.

    benchspotter trend --tag run1         # one clean series per release
    benchspotter trend --tag nightly
    benchspotter session ls

## The scheduled jobs

`.github/workflows/nightly.yml` runs at 02:00 UTC, benchmarks whatever `origin/master` points
at, commits the sessions and triggers the site rebuild. It exits without benchmarking if that
commit already has a nightly session, so a quiet day costs nothing. After the run it compares
the new point with the seven nightlies before it in commit order on the same runner
(`lib/check-steps.py`),
writes the result to the job summary and opens an issue when a series stepped by more than 5%
and more than twice its run-to-run spread. A dispatch can add cpu or mem profiles to the run
with the `profiles` input; they are megabytes per session, so only for a run to drill into.

`.github/workflows/range.yml` is for the morning after a step: given the last good and the
first bad nightly commit, it benchmarks every first-parent commit between them, tagged `range`,
so the step can be pinned to one merge. It refuses ranges over `max_commits` (20).

`.github/workflows/releases.yml` runs at 10:00 UTC and does the same for every `*-stable` tag
that has no `release` session yet, which is how a new upstream release reaches the site. Most
days it finds nothing and stops after the checkout. It takes at most `MAX_TAGS` (6) tags in one
run, oldest first: a GitHub-hosted job is killed at 6 hours and nothing is committed until the
last tag finishes, so a backlog is better cleared over several nights than lost in one long
job. The 21 tags that `main` starts with take about four nights.

Both share the `benchmark-runner` concurrency group, so they queue behind each other rather than
fighting over one machine, and both get their toolchain from `.github/actions/bench-setup`.

Each job passes the sha it just pushed to `pages.yml`. Without that the site would build from
`github.sha`, which is fixed before the results commit exists, and every build would be one
night behind.

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

Series are keyed by package and name, since a bare name can exist in two packages, and the
select groups them by package. The line above the chart names the package, links to it upstream
and repeats the note from `lib/benches.txt`, so the comment there is what the site shows.

Points are in commit order by default, so a tag measured late lands where it belongs in history
rather than at the end; "order by run date" gives record order, which is the order to read the
noise sentinel in. Each session carries its commit time as a tag (`lib/backfill-commit-times.sh`
added it to the sessions that predate this), and the step check uses the same order.

The chart draws a dashed rule where the Go toolchain that built the code changed, since every
series tends to step there at once. Clicking a point opens the upstream compare view between
it and the previous point, which is the list of PRs that could have moved it. The noise
sentinel toggle overlays the sentinel's relative movement on the current series (see below).

`.github/workflows/pages.yml` publishes it. Enable Pages for the repo with "GitHub Actions" as
the source, and give Actions write permission to contents (Settings > Actions > General) or the
results commit cannot be pushed. The benchmark jobs call the pages workflow directly, because a
push made with `GITHUB_TOKEN` does not trigger other workflows.

## The M2 Pro archive

`main` carries only what GitHub Actions measured, starting 2026-09-16. The sweep that started
this repo - 20 `vX.Y.Z-stable` tags (v3.24.0 to v5.0.1, 2024-05 to 2026-08) x 3 runs,
`-count=4`, one Apple M2 Pro, 2026-09-14 - is frozen on the `archive/macbook-sweep` branch.

    git switch archive/macbook-sweep
    benchspotter trend --tag run1
    uv run site/build.py && open site/dist/index.html

It is a separate branch rather than 60 more sessions in one store because the two were measured
on different hardware. Same benchmarks, same commits, different ns/op: a line drawn from the
laptop series into the hosted series steps at the handover, and the step is the machine. Neither
set is wrong, they are just not one series. The archive is what says which changes upstream made
over two years; `main` is what will say what changes from here.

## Reading the numbers

Run-to-run spread inside a single point is the yardstick for whether a step between points means
anything, and it is not constant: in the M2 Pro archive the first six tags ran while the machine
was in use and `MerkleCommit` moves 7-25% between repeats of the same tag there, against 0.3-2.5%
for the six that ran overnight. A shared GitHub runner is noisier again. Read nothing into a step
smaller than the spread of its own point, which the site draws as the band around the line.

`NoiseSentinelHash` and `NoiseSentinelLoop` are not go-algorand code. `patches/add-noise-sentinel.sh`
adds them to the checkout: a sha256 over a fixed buffer and a dependent integer loop, the same
work at every ref. When they move, the machine did, and a step every other series took with
them is the runner rather than upstream. `lib/check-steps.py` says so in its report, and the
site can overlay them on any series.

Known artefacts, all worth checking before believing a jump:

- `data/transactions/verify/txn_test.go` has picked the ed25519 batch verifier on a coin toss in
  `init()` since v5.0.0 (commit a7a983998f). Pure-Go ed25519consensus is about 26us per
  `BenchmarkTxn` op with 6 allocs, libsodium cgo about 41us with 8. `patches/pin-ed25519-verifier.sh`
  pins it to the Go implementation, which is what algod has defaulted to since v4.4.1. The v3 and
  v4 sessions in the M2 Pro archive predate the patch and are mostly libsodium numbers, so they
  are not comparable to production and not comparable to each other tag by tag.
- The set grew from 6 names to 45 on 2026-09-17, so most series start there. Names that do
  not exist at a tag are skipped, and `BenchmarkPay` and `BenchmarkAppInt1` fail on the missing
  proposer at tags before v5, so a release sweep of an old tag would abort; the old tags are
  already measured.
- `MerkleCommit` is not recorded from 2026-09-15 on. Its 3 x 5 x 5 grid of hash family x Item x
  Count produced 45 of the 52 series in the archive, they all moved together, and they were most
  of the run time of a session. The archive branch still has them and its site marks them as no
  longer recorded; `lib/benches.txt` says how to bring them back.
- `AppendMsgBlockHeader` steps +14% at v4.2.1 in the archive because msgp_gen was regenerated for
  a larger block header, not because anything got slower. `AppendMsgSignedTxn` is flat across the
  same commit, which is how you can tell.
