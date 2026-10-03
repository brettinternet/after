# Adversarial POC gate

`mise exec -- task test:poc` explicitly authorizes **synthetic** offline Docker experiments, including builds, hostile workloads, resource exhaustion probes and real terminal sessions. It is not an inspection-only command. Set `AFTER_DOCKER_BINARY` to a trusted absolute Docker CLI and `AFTER_DOCKER_HOST` to its local Unix socket. Separately provision the [pinned image and required isolation capabilities](SANDBOX.md). The gate never pulls an image or falls back to host execution.

The gate serially runs the existing Go suite without cached test results, enables every Docker/CLI/PTY proof, rejects **any skipped test**, and requires named proofs. It then uses temporary Go source overlays to break snapshot freshness binding and discard repeated requests in the protected observer. Each mutant must fail its designated test with the expected assertion, not merely fail compilation. Production source and assertions are not rewritten. The gate fails if a mutation survives. Linux CI separately provisions the pinned image, runs this same command and uploads its log, even on failure. Local ordinary `task test` still does not authorize Docker; it is not a substitute for this gate.

## Executable design-check map

All named tests below run through `test:poc`, along with the rest of the suite. Synthetic unit fault injection is distinguished from real isolated execution.

| Check                              | Executable evidence                                                                                                                                                                                                                          |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. Same response, changed effect   | `compare/TestComparisonProof`, `runner/TestRunnerProof`: real isolated HTTP responses and independently received 1→2 requests; 30s control remains 1→1                                                                                       |
| 2. Implementation and oracle edits | `runner/TestRunnerProof`: candidate driver/test panic cannot replace the frozen observer; `rawdiff/TestCapturedModesInventoryAndFrozenContext`: changed oracle remains inventory                                                             |
| 3. Changed basis                   | `review/TestInvalidationMatrix`: fixture, driver, observer, runtime, dependencies, environment, argv, rules/mask, renamed scenario/test, code/harmless edit, missing footprint; binding negative control                                     |
| 4. Late result                     | `runner/TestLateResultAndInstability`, `browser/TestLoopConsentAndSnapshotBarrier`: deterministic barriers, immutable old pair/request binding and explicit snapshot acceptance                                                              |
| 5. Failed execution                | `runner/TestFailuresAndRedaction`: injected faults; `TestRunnerProof`: real broken build, exited parent with live child, refused upstream, legitimate HTTP 502; `sandbox/TestObservedProof`: start/output/timeout/cancel                     |
| 6. Unstable repeats                | `runner/TestLateResultAndInstability`, `compare/TestRepetitionsAndExactPayloads`, `browser/TestStatesAndLatePersistedRun`: all repetitions retained, no cherry-picked equality                                                               |
| 7. Missing evidence                | `rawdiff/TestMissingArtifactsRedactionAndInvalidPair`, `gotestreport/TestMalformedAndUnsupportedKeepUnrelatedCards`, `browser/TestEngineBrowserAndCapturedPages`: usable inventory/raw escape, explicit unavailable state                    |
| 8. No models                       | `cmd/after/TestPaymentCLIProof`, `cli/TestReviewLoopPTYProof`: actual capture, execution, comparison, pin, restart, reopen and consented rerun, with no model integration or account                                                         |
| 9. Hostile content                 | `capture/TestHostileGitConfigurationAndEnvironment`, `TestNonRegularAndParentSymlink`; `store/TestConfinement`, `TestRedactionAndFalseCompleteness`; `sandbox/TestInputPaths`, `TestDockerProof`; terminal/import/browser hostile-data tests |
| 10. Finite scope                   | `compare/TestPaymentAndDeterminism`, `browser/TestEngineBrowserAndCapturedPages`, real CLI/PTY proofs: concrete inputs and channels, visible sequential synthetic limits                                                                     |

`paymentfixture/TestPaymentProof` also runs the real disabled-deduplication and forged printed-count mutations: actual provider traffic changes when deduplication is disabled, not when an app prints a different count. The separate protected-observer negative control drops repeated requests and must be caught by `runner/TestRunnerProof`.

## Performance reproduction

`mise exec -- task terminal:bench` generates a fresh Git repository with medium (10 files, 20,000 changed lines) and large (50 files, 100,000 changed lines) fixed inputs. It records capture, first application raw-view open, warm raw-view open, cached inventory access, document preparation, allocated bytes, per-event allocation, maximum navigation/resize rendering and quit latency. It asserts every expected path and unclassified hunk is retained. Capture limitations are printed, not removed to imply perfect coverage. The OS file cache is **warm from capture**, not a claimed cold disk benchmark; no privileged cache purge is attempted. `BenchmarkCapturedViewport` measures the large warm viewport separately.

Warm raw open ≤1s, cached inventory access ≤2s, document preparation ≤1s/64MiB, navigation/resize/quit ≤100ms and ≤256KiB allocated per event are evaluation budgets, not portable guarantees. `cli/TestReviewLoopPTYProof` separately records actual PTY input-to-help rendering while an authorized fixture is active, with a 1s scheduling-inclusive budget. This is the small payment fixture, not a claim that the generated large diff was simultaneously running in a container. Memory numbers are Go allocation counters, not whole-system or Docker peak RSS.

## Threat-model limits

The Docker daemon, CLI, pinned image, host kernel and AFTER executable are trusted. The probes exercise finite adversarial inputs, not a proof of real-world sandbox safety. The workload-crash test kills the container's workload with a descendant alive; timeout/cancel/crash must return only after owned-container removal. An unrelated sentinel must survive. Storage tests separately cover interrupted writes and writer recovery.

A **host supervisor SIGKILL, daemon failure or host power loss** can prevent deferred cleanup. This gate does not claim automatic orphan recovery after those events. Do not infer such recovery from the workload-crash test. Preserve exact owned resource identities and inspect ownership before manual cleanup; never prune by a shared label. Cancellation and normal workload failure, unlike uncatchable host death, are covered by executable cleanup checks.

Imported provenance is not a signature; repository text cannot grant execution or observed/current status. Redaction and missing/truncated channels fail closed. Finite sequential synthetic payment observations are not production billing, concurrency, universal behavior, user benefit or human acceptance evidence. No human study was run.

## Verification record

Author execution: `mise exec -- task test:poc` completed with exit 0 on an Apple M5 Max, 18 logical CPUs, 64GiB RAM, macOS arm64 with a local Linux/cgroup-v2 Docker daemon. All required proofs ran without skips. The snapshot-binding mutant failed `TestInvalidationMatrix` with `bad reopening`; the protected-observer mutant failed `TestRunnerProof` with `got 1 calls want 2`. The final gate success line followed both failures. No owned sandbox containers or derived images remained. `mise exec -- task check` also passed (race tests, build/vet/format, local links, backlog integrity and secrets). CI is configured; no remote CI run is claimed before pushing.

| Author measurement          |      Medium |        Large |
| --------------------------- | ----------: | -----------: |
| Paths / changed lines       | 10 / 20,000 | 50 / 100,000 |
| Raw bytes                   |   1,501,270 |    7,506,350 |
| First application raw open  |     1.480ms |      7.099ms |
| Warm raw open               |     1.468ms |      5.791ms |
| Cached inventory access     |       585ns |      1.869µs |
| Document preparation        |     1.281ms |      4.779ms |
| Preparation allocated bytes |   2,168,112 |   11,613,488 |
| Maximum input + render      |       171µs |        129µs |
| Maximum resize + render     |       263µs |        365µs |

Actual active-fixture PTY input-to-help render: **15.31ms**. All generated inventory/hunk counts were complete. These are one-run measurements with the cache and memory limits described above.

Independent verification: a separate fresh-context verifier found no concrete item-scoped defect in gate guards, CI wiring or performance coverage. It independently passed `task test:terminal`, `task terminal:bench` and `git diff --check`, measuring the large warm viewport at 29,727ns/op, 16,185B/op and 285 allocations/op. It did **not** rerun Docker or certify the author's full gate log; those results above remain author execution evidence. The full report and limitations are summarized separately in AFTER-15.

An earlier author invocation was killed by its shell tool's 20-minute timeout; it was **not** a pass. Its one identified owned staging container was inspected and manually removed before the complete successful run. This is direct evidence of the host-supervisor interruption limitation, not automatic recovery.

The gate's final success line is emitted only after both real proofs and mutation controls complete; a partial or interrupted log is not a pass.
