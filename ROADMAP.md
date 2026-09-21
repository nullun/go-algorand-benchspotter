# Roadmap

What this repo should measure next and what it should do better, from a survey of every
`func Benchmark` in go-algorand at master (8cd5eb5f6, 2026-09-15). The tree has 777 of them in
46 packages; this repo tracked six. Numbers below are from an Apple M2 Pro unless stated and
are there for scale, not for comparison with the hosted-runner series.

Checked items are done. The order within a section is priority.

## 1. Repo improvements

- [x] **Noise sentinel.** A benchmark that does not depend on go-algorand at all (a fixed hash
  loop over a static buffer), dropped into the throwaway checkout by `patches/`. Its movement
  night to night is pure machine noise, so every other series can be read against it, and a
  hosted-runner point that moved with the sentinel can be discounted.
- [x] **Step detection in CI.** Compare the newest point's median per series against the previous
  few points on the same runner, write a table to the job summary, and open an issue when a step
  exceeds the point's own spread. `site/build.py` already computes the medians.
- [x] **Toolchain rules on the chart.** A go.mod toolchain bump moves every series at once. The
  Go version is already a session tag, so the site can draw a rule at each change.
- [x] **Compare links.** The tooltip links to the GitHub compare view between the previous
  point's commit and this one, which is the list of PRs that could have caused a step.
- [x] **Range dispatch.** Master often merges several PRs between two hourly checks and each
  master point is one commit. A
  workflow that takes two refs and benchmarks every first-parent commit between them fills the
  gap when a step appears. Sessions are tagged `range` so the site can toggle them.
- [x] **Profiles on demand.** A dispatch input that adds a cpu or mem profile to a single
  master run. Profiles are too large to record every time.
- [x] **Wider set, one package at a time.** benchspotter aborts a session when any package's
  `go test` fails, so a new package should be smoke-run at `-count 1` on a dispatch before it
  joins the master set.
- [x] **Noise yardstick.** The band and the step check take the spread of every repeat of every
  run, since inside-process spread is two to four times the between-process spread on the hosted
  runner. Three runs of count four stay: the runs guard against a noisy neighbour mid-session,
  the count is where the noise averages out.
- [x] **Hourly, when there is something new.** Both workflows run hourly behind a cheap check job
  that compares upstream with the store; master gets one point per merge. `MAX_TAGS` is 1.
- [x] **Benchmarks of our own.** `benchmarks/` carries benchmarks go-algorand does not have,
  copied in and compile-gated per package before each run (section 3).
- [x] **Commit order.** Sessions carry `commit:<utc timestamp>` and the site and step check
  sort on it, with a toggle back to record order. An old tag measured late lands where it
  belongs; the sweep no longer has to run oldest first for the display's sake.
- [x] **Consensus version rules.** Sessions carry `consensus:vNN`, read from
  `protocol/consensus.go` at the ref, backfilled for the store, and the chart draws a rule at
  each change alongside the toolchain rules.
- [ ] **Per-benchmark benchtime.** Several upstream benchmarks build `b.N` fixtures in untimed
  setup, so their wall clock balloons at default benchtime. benchspotter has no benchtime flag;
  `GOFLAGS=-benchtime=Nx` applies to the whole session, so this needs either a benchspotter
  change or one session per pinned benchmark.
- [ ] **Self-hosted runner.** Still the largest single improvement. Everything else widens the
  signal; this narrows the noise.

Not possible with benchspotter as it is: selecting one cell of a grid. It discovers sub-benchmarks
by reading the test source, so a `b.Run` whose name is built at run time cannot be selected and
the parent has to be taken whole. A pattern that matches nothing is silently ignored, which is
what lets one `lib/benches.txt` serve every tag.

## 2. Existing benchmarks

### Added to `lib/benches.txt`

Flat or few-sub, deterministic, no grid over sizes, under a few seconds at default benchtime.
See the file for the list; the notes there say what each one measures.

Where an upstream benchmark only exists at recent tags, or asks a question that needs unexported
names, a benchmark of this repo's own in `benchmarks/` can cover the same ground over the whole
sweep through the exported API. `BenchmarkBatchVerify` is the first of those: upstream's
`BenchmarkBatchVerifierImpls` compares the three implementations and starts at v4.4.1, while this
one measures whichever verifier a ref makes by default and runs back to v3.17.0. It already shows
something the upstream one cannot: a 64-signature batch costs 1.16 ms at v3.17.0 and 2.60 ms at
v4.7.4 on an M2 Pro, which is `useSingleVerifierDefault` arriving with the interface at v3.24.0.

### Existing but deliberately not tracked

Grids over sizes or counts whose cells move together, in the pattern of the retired
`MerkleCommit`: `BenchmarkMerkleVerify1M` and `BenchmarkMerkleProve1M` (3 families, 1M-leaf tree
and 1M precomputed proofs per family in setup), `BenchmarkHashes` (9 algorithms x 6 sizes),
`BenchmarkUintMath`/`UintCmp`/`ByteMath`/`ByteLogic`/`ByteCompare`/`Base64Decode`/`JsonRef`
(opcode x width tables), `BenchmarkBn254`/`Bls12381` (random points, minutes), `BenchmarkTxnTypes`
(13 consensus versions x 8 cases), `BenchmarkTxHandlerDecodeMsg` (5 group sizes),
`BenchmarkDigestCaches` (thread counts), `BenchmarkTransactionPoolPending` (5 pool sizes, rebuilt
per sub), `BenchmarkBatchVerifierBig*` (288 cells).

Setup-dominated, disk-bound or timing-dependent: `BenchmarkLargeMerkleTrieRebuild` and
`BenchmarkLargeCatchpointDataWriting` (6M accounts x b.N), `BenchmarkBalancesChanges` (rewrites
b.N from wall clock, sleeps), `BenchmarkWriteCatchpointStagingBalances`,
`BenchmarkRestoringFromCatchpointFile`, `BenchmarkBoxDatabaseRead` (seeds from the clock),
`BenchmarkLookupKeyByPrefix`, `BenchmarkLedgerStartup`, `BenchmarkExpiredOnlineCirculation`,
`BenchmarkTxTailBlockHeaderCache` (2M blocks regardless of b.N), every `data/account` benchmark
(sqlite on disk, O(b.N) key generation), `BenchmarkSQLWrites`/`SQLErasableWrites`/`SQLQueryAPIs`
(disk speed, leave files in /tmp and cwd), `BenchmarkTxHandlerIncDeDup` (sleeps by design),
`BenchmarkTxHandlerProcessIncomingTxn*` (NumCPU consumers), `BenchmarkTransactionPoolSteadyState`
(commits real blocks), `BenchmarkGetUnverifiedTransactionGroups50` (fixed inner loop, ns/op
meaningless), `BenchmarkVariableTransactionMessageBlockSizes` (measures a sleep),
`BenchmarkServiceFetchBlocks` (54 s at one iteration, 10 ms poll loop, asserts on round equality,
leaves .db files), `BenchmarkTxSync` (b.N/30 inner loop, zero at small b.N),
`BenchmarkCryptoVerifierBundleVertification` (732 ms/op), `BenchmarkNetworkPerformance`,
`BenchmarkAssembleBlock`, `BenchmarkBuildVerify` (62 s of falcon keygen before the first timed
iteration), `BenchmarkMerkleSignatureSchemeGenerate` (52 s/op), `BenchmarkRandomizedEncode`/
`Decode` (global RNG seed, high variance), `BenchmarkFindMultiMulCutoff` (random points inside
the timed loop), `BenchmarkNodeLeafImplementation` (dead code, map-order noise),
`BenchmarkEcdsa`/`BenchmarkEd25519Verifyx1` (O(b.N) keygen in setup),
`BenchmarkWebsocketNetworkBasic` (localhost sockets, `t.Errorf` on a stall).

Generated `msgp_gen_test.go` benchmarks everywhere: they encode zero-valued structs and the
Unmarshal ones ignore the required-field error, so they measure an empty-struct fast path and
cannot see a codec regression on real traffic. Cheap and stable, but low signal.

Name collisions: `BenchmarkSign` exists in both `crypto` and `crypto/secp256k1`. The site keys
series by package and name since 2026-09-17, so selecting it would now give two series; it is
still left out because `BenchmarkSignVerify` covers the ed25519 side.

### Broken at master, each a one-line fix

Each of these panics or fails and takes its whole package's test binary down, which is the
session-aborting failure mode. The checked ones are fixed twice over: as scripts in `patches/`
that repair the throwaway checkout, and as commits on the `bench/fix-broken-benchmarks` branch
of the go-algorand clone at `~/GitHub/algorand/go-algorand` (seven: one per item, plus one that
moves `benchmarkBlockValidationMix`'s progress output from `fmt.Printf` to `b.Logf`, since it
landed inside the result line whenever Go ran a second round of iterations and parsers dropped
the result). The patches skip themselves once the upstream code has the fix. The weights fix is
on the branch only, since nothing here would trend it.

- [x] **The txHandler backlog family** (`BenchmarkHandleTxns`, `HandleTxnGroups`, `HandleMsig*`,
  `HandleBLW*`, `HandleLsigTxnGroups`). `runHandlerBenchmarkWithBacklog` emulates `Start()`
  without starting the rate limiter, and `handler.Stop()` then calls a nil cancel func in
  `redCongestionManager.Stop` (util/rateLimit.go:462). Fix: `handler.erl.Start()` after
  data/txHandler_test.go:1738, or return from `Stop` when `ctxCancel` is nil.
- [x] **`BenchmarkBlockEvaluatorRAMCrypto`, `DiskCrypto`, and all four `BenchmarkBlockValidation*`.**
  They validate blocks built from an unfinished block with no proposer, and payouts have been in
  `ConsensusCurrentVersion` since v40. The pattern that works is at ledger/ledger_perf_test.go:302.
  Fix: `FinishBlock(committee.Seed{0x01}, <address>, false)` at ledger/evalbench_test.go:528 and
  :565 and at ledger/fullblock_perf_test.go:148.
- [x] **`BenchmarkCryptoVerifierVoteVertification`** (agreement/cryptoVerifier_test.go:285) calls
  `Verified(AgreementVoteTag)`, which panics since votes moved to `VerifiedVotes()`, and behind
  that builds the vote for round 300 against a fixture at round 1. Fixed, it is a 35 us benchmark
  of `unauthenticatedVote.verify`, the dominant CPU cost under vote load.
- [x] **`BenchmarkCodecEncoder`** (protocol/encodebench_test.go:31) encodes a nil and segfaults.
  Removed, on the branch and by a patch here: repaired it would encode an empty struct, which
  every generated msgp benchmark already does for a real type.
- [x] **`BenchmarkVerifyWeights` and `BenchmarkNumReveals`** (crypto/stateproof/weights_test.go:199,
  :220) fail on "too many reveals in state proof" unconditionally: they ask for a 1.1
  signed-to-proven ratio, which needs more reveals than `MaxReveals` allows, and have done since
  they were added with the cap. Fixed on the branch with a ratio of 2 (about 0.5 us/op each). Not
  patched or trended here: the arithmetic they time is a handful of big-integer operations and
  the state proof cost lives in falcon and the merkle proofs.
- [x] **`BenchmarkTransactionPoolRecompute`** (data/pools/transactionPool_test.go:1023) calls
  `FailNow` unless `MaxTxnBytesPerBlock` is 5 MB, allocates b.N pools of 75k txns and sleeps a
  second. Rewritten, on the branch and by a patch here: the pool is filled until a transaction no
  longer fits the block (28,649 payments at 5 MB), then `recomputeBlockEvaluator` is timed with
  nothing committed between iterations, so each op is the re-evaluation of one full block's
  worth of pending transactions: about 0.74 s/op, 26 us per txn, reported as `ns/txn` too.
  Tracked.

- [x] **`BenchmarkBatchVerifierImpls` before v5.0.0** (crypto/batchverifier_bench_test.go:63 at
  v4.7.4). Not broken, wrong: one `BatchVerifier` is made outside the timed loop and reused, and
  nothing enqueued is ever dropped before `Verify`, so iteration i verifies i*64 signatures and
  ns/op grows with b.N. Upstream fixed it at v5.0.0 by passing a constructor;
  `patches/fix-batchverifier-impls-benchmark.sh` applies the same edit to v4.4.1 through v4.7.4,
  where the text of the function is identical. Nothing for the branch: master already has the fix.
  Verified at v4.7.4 by running the patched benchmark at 100ms and 1s, which now agree within 1%.

The exclusions this repo recorded for `BenchmarkBlockEvaluatorRAM*` and `BenchmarkPrefetcherPayment`
were partly stale: the `NoCrypto` variants call `Eval` with validation off so the proposer check
never runs, and the prefetcher benchmarks never build a block. Both are tracked now. The
prefetcher pair does fail at the tags from v3.17.0 to v4.5.1, where the group's `Err` is a
`*GroupTaskError` and `require.NoError` sees a nil pointer as a non-nil error;
`patches/fix-prefetcher-benchmark-typed-nil.sh` swaps in a nil check there.

## 3. New benchmarks to write upstream

Ordered by production heat divided by effort. All deterministic and CPU-bound unless noted.

Checked items exist in this repo under `benchmarks/<package path>/benchspotter_<topic>_test.go`,
written to upstream standards (licence header, in-package, gofmt, `go vet`) and copied into the
checkout by `patches/add-benchmarks.sh`, which compiles each package it touches and drops the
files where they do not build, so old tags skip them. They are tracked in `lib/benches.txt` with
the note `(benchmarks/)`. The plan is to let them run, and upstream the ones that prove useful,
which a benchmark does by moving when a real change lands in its area. Notes on what was found
while writing them:

- `apply.Payment` is left out of `BenchmarkApply`: the mock `Balances` does not move balances, so
  it would measure call overhead. The other three apply calls are covered.
- `BenchmarkTxTailCheckDup` disables go-deadlock detection for its duration, as evalbench does;
  with it on, the mutex stack capture is 70x the work being measured. Release algod runs without it.
- `BenchmarkStateOps` and `BenchmarkBoxOps` include the package's mock ledger cost (it clones the
  state map per call). Deterministic, so the trend holds, but the absolute figures are not algod's.
- The `data/transactions` and `data/bookkeeping` files cannot import `txntest` (import cycle), so
  their fixtures are struct literals, duplicated between the two packages.
- `BenchmarkAssemble` and `Disassemble` are `package logic_test`, as evalBench_test.go is, because
  `txntest` imports `logic`.

### An hour or less each

- [x] **Falcon verify and sign.** `FalconVerifier.VerifyBytes` (crypto/falconWrapper.go:109) runs
  once per revealed participant per state proof; nothing benchmarks it. One signer in setup.
- [x] **`merklesignature.Verifier.VerifyBytes`** (crypto/merklesignature/merkleSignatureScheme.go:266),
  the full per-participant state proof check. `New(0, 512, 256)` gives a 60 ms setup.
- [x] **VRF prove** (`VrfPrivkey.proveBytes`, crypto/vrf.go:99). Verify is covered, prove is not,
  and every eligible account runs it each agreement step. Write it with b.N-independent setup.
- [x] **`MultisigVerify` and `MultisigBatchPrep`** (crypto/multisig.go:231, :243). No multisig
  benchmark exists. Subs for a 3-of-5 direct and batched.
- [x] **`txTail.checkDup`** (ledger/txtail.go:352). Every incoming transaction hits it through the
  pool. Populate 1000 rounds with the loop at ledger/txtail_test.go:425; time a hit and a miss.
- [x] **`Transaction.WellFormed`** (data/transactions/transaction.go:369). Per txn in verification
  and in the pool. Subs per transaction type.
- [x] **`SignedTxn.ID` and `TxGroup` hashing** (data/transactions/signedtxn.go:64, transaction.go:176).
  `Txn.ID` is covered by `BenchmarkEncoding/ID`; the group hash is not.
- [x] **`HashObj` on a real `Transaction` and `BlockHeader`.** `BenchmarkHash` hashes 32 raw bytes
  and misses the msgp `ToBeHashed` cost. Lives in data/transactions to avoid the import cycle.
- [x] **Realistic msgp round trips** with populated pay, appl-with-boxes, `Block` and
  `unauthenticatedVote` fixtures, so the codec line means something.
- [x] **`makeCompactResourceDeltas`** as a `resource-deltas` sub next to the existing
  `account-deltas` at ledger/acctupdates_test.go:1285.
- [x] **`AccountDeltas.Upsert`, `MergeAccounts`, `OptimizeAllocatedMemory`**
  (ledger/ledgercore/statedelta.go:495, :445, :573). Only the `make()` sizing is benchmarked.
- [x] **vpack vote compression and decompression** (network/msgCompressor.go:69,
  network/vpack/vpack.go:233). Runs on every vote sent and received; the package has fuzzers and
  no benchmark. Encode one vote to msgp once, loop compress and decompress with a reused buffer.
- [x] **`unauthenticatedVote.verify`** (agreement/vote.go:105) synchronously, without the
  execpool, on the 100-account fixture.
- [x] **`messageFilter.CheckIncomingMessage`** (network/messageFilter.go:48) at capacity, hit and
  miss. The existing benchmark covers only the hash.
- [x] **`logic.AssembleString` and `Disassemble`** (data/transactions/logic/assembler.go:2993, :3472).
  Every AVM benchmark assembles a 2000-op program in setup, so assembler regressions inflate
  their wall clock without showing in ns/op.
- [x] **`LogicSigSanityCheck`** (data/transactions/verify/txn.go:413) via `PrepareGroupContext` on
  the TinyMan group.
- [x] **`apply.Payment`, `AssetTransfer`, `Keyreg`, `ApplicationCall`** (ledger/apply). The package
  has zero benchmarks and `mockBalances_test.go` already provides the fixture.

### Half a day to a day each

- [x] **Stateful AVM opcodes.** The largest hole in the tree. No benchmark touches
  `app_global_*`, `app_local_*`, `asset_holding_get`, `asset_params_get`, `acct_params_get`,
  `block`, `online_stake`, or any box op (data/transactions/logic/box.go). The `MakeTestApp`
  harness in evalStateful_test.go gives the fixture; shape it like `benchmarkOperation`
  (eval_test.go:4002) through `EvalApp`.
- [x] **Inner transactions.** `itxn_begin/field/submit/next` have zero benchmarks and dominate
  modern app call cost. Subs for inner pay, axfer and appl.
- [ ] **Tracker stack lookups.** `accountUpdates.lookupLatest`, `lookupResource`, `lookupKv`
  (ledger/acctupdates.go:1013, :1237, :358) are the per-txn read path and the REST accounts
  path. Only the sqlite layer is benchmarked, so delta-depth regressions are invisible. Use
  `makeMockLedgerForTracker` with 5k accounts and 320 rounds of deltas.
- [ ] **`roundCowBase` resource lookups and `getKey`** (ledger/eval/eval.go:341, :367, :431), the
  AVM's view of state, with a fake `LedgerForCowBase`.
- [ ] **Pool `Remember` for groups.** `recomputeBlockEvaluator` is covered by the rewritten
  `BenchmarkTransactionPoolRecompute` (section 2); the group path of `Remember` still is not.
- [ ] **`endOfBlock`, knock-offline list generation, `proposerPayout`, `autoHeartbeat`**
  (ledger/eval/eval.go:1446, :1740, :1710, :638). Once per block under payouts, unbenchmarked.
  Needs the proposer-aware `endBlock` helper (ledger/simple_test.go:181).
- [ ] **`TopOnlineAccounts` and `onlineCirculation`** (ledger/acctonline.go:802, :533). Run every
  round for agreement; only the cache layer is benchmarked.
- [ ] **State proof verify and prover at realistic size.** `BenchmarkBuildVerify` has a 62 s setup
  from 2000 sequential falcon keygens. Cut to 32 participants sharing one signer, or check a
  serialized proof into testdata; setup drops under a second.
- [ ] **`merkletrie.Trie.Commit` in isolation** (crypto/merkletrie/trie.go:224). Commit is hit
  once per 1000 ops inside `BenchmarkAdd`, so catchpoint commit regressions are invisible. A
  bounded `accountsUpdateBalances` at 50k accounts is the same story.
- [ ] **`msgBroadcaster.preparePeerData` and `innerBroadcast`** (network/wsNetwork.go:1480, :1515)
  over stub `wsPeer`s with drained send channels, subs by peer count. No sockets.
- [ ] **`voteAggregator.filterVote` and `proposalTracker.handle`** (agreement/voteAggregator.go:196,
  proposalTracker.go:147) with a pre-populated round. router_gc_test.go:162 shows the fixture.
- [ ] **catchup `processBlockBytes`** (catchup/universalFetcher.go:100) with 0, 1000 and 25000 txn
  blocks, and the relay-side `topicBlockBytes` (rpcs/blockService.go:453).
- [ ] **execpool backlog enqueue** (util/execpool/backlog.go:106) with capped parallelism. Every
  txn and vote crosses it.
- [ ] **A small `Ledger.Validate` benchmark**: one 1000-txn block in memory with a finished
  proposer, as a better trending target than the 50k-txn ones.

### Not worth attempting in isolation

`catchpointCatchupAccessor.ProcessStagingBalances` and `BuildMerkleTrie` need hundreds of MB of
fixture and are sqlite-bound. `catchup.Service.fetchAndWrite` needs the whole catchup stack,
which is why `BenchmarkServiceFetchBlocks` became a load test. `rpcs/txSyncer.syncFromClient` is
a network round trip by construction; its benchmarkable half is `BenchmarkTransactionFilteringPerformance`.
