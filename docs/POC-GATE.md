# Adversarial POC gate

`task test:poc` explicitly authorizes synthetic offline Docker experiments, hostile workloads, resource probes, and real terminal sessions. It is not an inspection-only check. Set `AFTER_DOCKER_BINARY` to a trusted absolute Docker CLI and `AFTER_DOCKER_HOST` to its local Unix socket; provision the [pinned image and isolation capabilities](SANDBOX.md) separately. The gate never pulls an image or falls back to host execution.

## What the gate runs

The gate runs the Go suite serially without cached results, enables every Docker/CLI/PTY proof, rejects any skipped test, and requires named proofs. Temporary Go source overlays then break snapshot freshness binding and drop repeated requests in the protected observer. Each mutant must fail its designated test with the expected assertion; compile errors or unrelated failures do not count. The checkout and assertions are unchanged. A mutant that survives fails the gate.

Linux ARM64 CI provisions both pinned images and runs `task test:poc -- --package PACKAGE` for every required proof package in separate jobs. Each slice rejects skips, requires its own anchors, and runs its applicable negative control; the review and runner slices own the two mutants. Demo and study cases A/B/C run in separate jobs too. Each proof job has a 15-minute ceiling (13 minutes for execution); failures cancel sibling slices, and a newer push cancels the old workflow. Logs upload even on failure. Ordinary `task test` does not authorize Docker and is not a substitute.

## Executable design checks

These named tests run through the gate along with the full suite. Synthetic fault injection is not a substitute for the real isolated proofs.

| Check                                 | Tests and required evidence                                                                                                                                                                                                                                      |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Same response, changed effect         | `internal/compare/TestComparisonProof`, `internal/runner/TestRunnerProof`: real identical HTTP responses, twelve-hour provider calls 1→2, thirty-second control 1→1.                                                                                             |
| Implementation and oracle both change | `internal/runner/TestRunnerProof`, `internal/rawdiff/TestCapturedModesInventoryAndFrozenContext`: frozen observer stays independent; changed oracle remains inventory.                                                                                           |
| Changed basis                         | `internal/review/TestInvalidationMatrix`: fixture, driver, observer, runtime, dependencies, environment, argv, rules, mask, scenario/test name, code/harmless edit, missing footprint, and binding negative control.                                             |
| Late result                           | `internal/runner/TestLateResultAndInstability`, `internal/browser/TestLoopConsentAndSnapshotBarrier`: barriers retain old snapshot/request binding; acceptance is explicit.                                                                                      |
| Failed execution                      | `internal/runner/TestFailuresAndRedaction`, `TestRunnerProof`, `internal/sandbox/TestObservedProof`: injected faults, broken build, live child, refused upstream, HTTP 502, start/output/timeout/cancel.                                                         |
| Unstable repeats                      | `internal/runner/TestLateResultAndInstability`, `internal/compare/TestRepetitionsAndExactPayloads`, `internal/browser/TestStatesAndLatePersistedRun`: retain all repetitions; no cherry-picked equality.                                                         |
| Missing evidence                      | `internal/rawdiff/TestMissingArtifactsRedactionAndInvalidPair`, `internal/gotestreport/TestMalformedAndUnsupportedKeepUnrelatedCards`, `internal/browser/TestEngineBrowserAndCapturedPages`: explicit unavailable state and usable raw/inventory view.           |
| No models                             | `cmd/after/TestPaymentCLIProof`, `internal/cli/TestReviewLoopPTYProof`: real payment capture/run/compare/pin/reopen/consented rerun without a model or account.                                                                                                  |
| Command scenarios                     | `internal/cli/TestCommandCLIProof`: changed/control direct-argv cases pass through capture, exact consent, run, separate-channel compare, pin and reopen.                                                                                                        |
| Hostile content                       | `internal/capture/TestHostileGitConfigurationAndEnvironment`, `TestNonRegularAndParentSymlink`, `internal/store/TestConfinement`, `TestRedactionAndFalseCompleteness`, `internal/sandbox/TestInputPaths`, `TestDockerProof`, plus terminal/import/browser tests. |
| Finite scope                          | `internal/compare/TestPaymentAndDeterminism`, `internal/browser/TestEngineBrowserAndCapturedPages`, CLI/PTY proofs: exact inputs, finite channels, sequential synthetic limits.                                                                                  |

`internal/paymentfixture/TestPaymentProof` also mutates application deduplication and printed counts: disabling deduplication changes actual provider traffic, while forged app output does not. The independent observer mutation separately proves the runner catches dropped repeated requests.

## Performance reproduction

`task terminal:bench` creates fresh Git repos with medium (10 files, 20,000 changed lines) and large (50 files, 100,000 changed lines) inputs. It records capture, first and warm raw view, cached inventory, document preparation, allocations, input/resize rendering, and quit latency; it asserts all expected paths and unclassified hunks. The OS cache is warm from capture, not a cold-disk benchmark. `BenchmarkCapturedViewport` separately measures the large warm viewport.

| Author measurement      |      Medium |        Large |
| ----------------------- | ----------: | -----------: |
| Paths / changed lines   | 10 / 20,000 | 50 / 100,000 |
| Raw bytes               |   1,501,270 |    7,506,350 |
| First raw open          |    1.480 ms |     7.099 ms |
| Warm raw open           |    1.468 ms |     5.791 ms |
| Cached inventory        |      585 ns |     1.869 µs |
| Document preparation    |    1.281 ms |     4.779 ms |
| Preparation allocations | 2,168,112 B | 11,613,488 B |
| Max input/render        |      171 µs |       129 µs |
| Max resize/render       |      263 µs |       365 µs |

Budgets are evaluation targets, not portable guarantees: warm raw view ≤1s, cached inventory ≤2s, document preparation ≤1s/64 MiB, navigation/resize/quit ≤100ms and ≤256 KiB per event. `internal/cli/TestReviewLoopPTYProof` measured 15.31 ms from PTY input to help rendering while a fixture ran; this was not the generated large diff running in a container. Allocation counters are not whole-system or Docker peak RSS.

## Trust limits

The Docker daemon, CLI, pinned image, host kernel, and AFTER executable are trusted. Finite adversarial probes are not proof of general sandbox safety. The workload-crash test kills a container workload with a live descendant and requires cleanup of its owned container while an unrelated sentinel survives. Storage tests cover interrupted writes and writer recovery.

A host-supervisor SIGKILL, daemon failure, or power loss can prevent deferred cleanup. The gate does not claim orphan recovery after those events. Cancellation and ordinary workload failure are covered. Inspect exact owned resource identities before manual cleanup; never prune by shared label.

Imported provenance is not a signature. Repository text cannot grant execution or observed/current status. Redaction and missing/truncated channels fail closed. Synthetic sequential payment observations are not production billing, concurrency, universal behavior, human acceptance, or usability evidence.

## Verification record

Author run: `task test:poc` exited 0 on macOS arm64 (Apple M5 Max, 18 logical CPUs, 64 GiB) with a local Linux/cgroup-v2 Docker daemon. All required proofs passed without skips. The command proof captured and changed one direct-argv case while keeping its control equal, then compared all channels and pinned/reopened the result. The snapshot-binding mutant failed `TestInvalidationMatrix` with `bad reopening`; the observer mutant failed `TestRunnerProof` with `got 1 calls want 2`. The final gate success line followed both expected failures. No owned sandbox containers or derived images remained. `task check` also passed. These are author results; no remote CI result is claimed.

An independent check found no item-scoped defect in gate guards, CI wiring, or performance coverage. It passed `task test:terminal`, `task terminal:bench`, and `git diff --check`; the large warm viewport measured 29,727 ns/op, 16,185 B/op, and 285 allocations/op. It did not rerun Docker or independently certify the full gate log.

The author measurements are one run. All generated inventory and hunk counts were complete. The PTY and performance figures are not cross-machine guarantees.
