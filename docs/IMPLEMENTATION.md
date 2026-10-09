# Implementation contract

This is the current scope and acceptance contract for AFTER's local CLI/TUI proof of concept. The [Backlog](../backlog/tasks) owns task status; [behavior and evidence](behavior-and-evidence.md) explains the product rationale.

## What the POC proves

A reviewer can capture a Git candidate, inspect its complete change inventory and an imported report without running project code, authorize a frozen HTTP-service experiment on two isolated versions, compare responses and independently observed fake-upstream calls, pin an expectation, see it reopen when its basis changes, and explicitly rerun it. The built-in payment definition demonstrates identical responses, 1→2 provider calls at twelve hours, and 1→1 at thirty seconds; the pinned Python standard-library fixture demonstrates the same observer boundary on another language/image. Both are finite synthetic cases, not universal behavior.

A test name, expected-output file, or seeded screenshot is not an observation. Technical completion requires the M1/M2 acceptance criteria and the [adversarial gate](POC-GATE.md). Human benefit is separate; no agent can certify it.

## Scope and defaults

| Area                    | Decision                                                                                                                                                                                                                                                                   |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Product                 | Go engine, CLI, and TUI; no JavaScript runtime in the executable. No model, account, editor, GitHub, or browser-app dependency.                                                                                                                                            |
| Platforms               | macOS and Linux inspection. Execution requires Docker Engine or a compatible Docker CLI on a local Linux daemon; no Windows execution promise.                                                                                                                             |
| Git                     | Trusted `/usr/bin/git`; explicit argv, NUL-safe paths, no external diff/textconv/filter execution. Never change the user's index or working tree; never fetch.                                                                                                             |
| Snapshots               | HEAD vs working tree by default, HEAD vs index with `--staged`, or explicit merge-base comparison. Non-ignored untracked paths are excluded unless selected exactly. Ignored files are not selectable.                                                                     |
| Unsupported Git content | Unmerged indexes, submodules, LFS-only content, missing objects, sparse/partial captures, and non-regular files are inventoried or rejected; capture does not claim completeness.                                                                                          |
| Report adapter          | Stock `go test -json` and the operator-selected AFTER-48 follow-up's `junit-xml-v1`. Both report status/output only, never test inputs or effects. See [GO-REPORTS.md](GO-REPORTS.md) and [JUNIT-REPORTS.md](JUNIT-REPORTS.md).                                            |
| Scenario                | Built-in payment instance or explicitly operator-selected v1 `http-service` JSON: pinned image/platform, direct build/start argv, fd3 readiness, bounded controlled-clock cases, fake upstreams, channels and limits. The shipped Go observer owns the fake endpoints.     |
| Storage                 | Versioned JSON records and content-addressed artifacts in private, ignored `.after/`; no database or plugin system ([proposed later design](EXTENSIONS.md)). See [SCHEMA.md](SCHEMA.md) and [STORAGE.md](STORAGE.md).                                                      |
| Invalidation            | Conservatively bind the whole project footprint: fixture, driver, observer, rules, toolchain, dependencies, and environment. Incomplete bindings remain unknown or stale.                                                                                                  |
| Execution               | Explicit digest-bound authorization. Builds and dependency preparation count as execution. No host-process fallback.                                                                                                                                                       |
| Sandbox                 | Disposable Docker isolation, non-root workloads, read-only inputs, no host home/socket or ambient credentials, no external network, and resource/time/output limits. AFTER-6 established and attack-tested the topology and pinned image; see [SANDBOX.md](SANDBOX.md).    |
| TUI                     | Bubble Tea v1.3.10, selected by the [terminal gate](TERMINAL.md); bounded safe-text viewport, background jobs, and PTY restoration tests. The TUI is a review client and test instrument, not a terminal-only commitment; capture/comparison stay separate from rendering. |

Do not add universal adapters, a semantic dependency graph, GitHub sync, a browser application, production replay, arbitrary URL execution, autonomous repair, or another agent runtime. Boundary generation and reduction are the separately gated M3 tasks AFTER-19/20.

## Records and trust

`internal/evidence` owns the version-1 records; [SCHEMA.md](SCHEMA.md) defines fields and validation. Snapshots bind captured source and limitations; scenarios freeze inputs, driver, observer, and rules separately from expectations; receipts bind execution and artifacts; comparisons retain exact witnesses; pins keep human expectations and append-only decisions. Capture uses consistent double reads but cannot claim an atomic filesystem snapshot.

Producer/kind, applicability, execution, comparison, and human decision are independent. Imports are `reported` with unknown applicability; digests are not signatures. Only a completed runner creates observations, and completion does not mean behavior passed. Immutable storage rejects concurrent writers and newer unsupported schemas. Redaction, truncation, missing channels, and incomplete capture fail closed. Current observations/source stay out of Git by default; demo inputs are synthetic.

## Commands and consent

The CLI's current grammar is in [CLI.md](CLI.md); its broader redesign target is [CLI-DESIGN.md](CLI-DESIGN.md). Typical reads:

```sh
after capture --project /work/payment
go test -json ./... | after import --project /work/payment
after inspect BASE CANDIDATE --project /work/payment
after review BASE CANDIDATE --project /work/payment
```

`capture`, import, inspection, comparison, help, configuration, and run preview do not execute repository code. A run must show and authorize the exact plan; non-TTY calls do not read stdin for consent. A saved plan is reconstructed and checked byte-for-byte before execution. Preview binds definition bytes/digest, preparation recipe and budgets, image/platform, argv, mounts, network policy, inputs, limits, and snapshots. Configuration cannot authorize execution.

The TUI offers an evidence list, inspector, raw diff, and explicit actions: Enter opens evidence; `d` shows the diff and unknown inventory; `p` pins an expectation; `r` requests a preview and exact consent. Snapshot acceptance is explicit; selection never runs an item. It sanitizes terminal controls, handles Unicode/tabs/long or empty lines/resizes, and runs Git/capture/execution jobs off the UI loop with snapshot/request IDs.

## Acceptance checks

`task test:poc` is the opt-in Docker/PTY gate. It authorizes synthetic payment and Python HTTP-service runs, rejects skips, requires named proofs, and kills two production-code mutants with their expected assertions. Its [ten design checks](POC-GATE.md) cover independent effects, frozen oracles, invalidation, late/failed/unstable results, missing evidence, no models, hostile content, and finite scope. It also tests interrupted writes, workload-crash cleanup, bounded output, descendant/container cancellation, environment contamination, permission denial, dirty/index/untracked Git behavior, and Unicode paths. It does not prove cleanup after host-supervisor SIGKILL, daemon failure, or power loss. Docker, its CLI, pinned image, host kernel, and AFTER are trusted.

`task terminal:bench` measures generated medium (10 files/20,000 changed lines) and large (50 files/100,000 changed lines) diffs. The cache is warm from capture, not a cold-disk benchmark. Warm raw view ≤1s, cached inventory ≤2s, document preparation ≤1s/64 MiB, and input/render ≤100ms/256 KiB are evaluation budgets, not guarantees. Actual gate results and measurements are in [POC-GATE.md](POC-GATE.md).

## Milestones and human boundary

M1 tasks AFTER-1–11 establish records, capture, import, isolation, the real fixture/runner, comparison, CLI, and review pins. M2 includes the terminal/browser and gate work AFTER-12–17, TUI redesign tasks AFTER-21–33, and CLI redesign tasks AFTER-34–46 (with AFTER-24 in the TUI sequence). Backlog dependencies, not ID order, determine readiness.

AFTER-17 provides a reproducible evaluation kit. AFTER-18 is a human study and requires explicit authorization, 12–16 consenting unfamiliar engineers, a facilitator, a second scorer, and a 75-minute session plan. AFTER-19/20 explore bounded generation and reduction only after a separate go decision. The proposed ~30% re-review-time reduction is a target, not a measured result. See [EVALUATION.md](EVALUATION.md).
