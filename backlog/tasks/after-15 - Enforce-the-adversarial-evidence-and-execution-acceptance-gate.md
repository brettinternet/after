---
id: AFTER-15
title: Enforce the adversarial evidence and execution acceptance gate
status: Done
assignee: []
created_date: '2026-10-03 05:44'
updated_date: '2026-10-04 02:24'
labels:
  - poc
  - verification
  - security
  - reviewed
milestone: m-1
dependencies:
  - AFTER-14
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: task
ordinal: 15000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A robust POC must actively falsify its evidence model. Scope: a cross-cutting negative-test gate and performance harness, extending existing focused tests rather than a second test framework. Re-read section 10 of behavior-and-evidence.md and verify all ten checks by executable evidence, not a checklist alone.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A single task test:poc target executes all ten acceptance checks mapped in docs/IMPLEMENTATION.md against the real engine and TUI integration; CI runs them and fails on a deliberately broken binding or observer.
- [x] #2 Fault-injection tests cover oracle edits, every invalidation input, out-of-order completion, build/start/driver failures, missing artifacts and unstable repetitions; none become false current equality or fabricated observations.
- [x] #3 Hostile fixtures probe shell/argv injection, Git helper execution, path/symlink/archive-like traversal, terminal ESC/OSC payloads, forged badges, overlarge logs and secret redaction with no unintended command or data exposure.
- [x] #4 Executable sandbox tests confirm no external network/host credential access, resource bounds and complete owned-process/container cleanup after timeout/cancel/crash; test skips cannot masquerade as a passing execution gate.
- [x] #5 Deduplication mutation changes the independent provider observation while the response remains equal; an intentionally weakened observer causes the gate to fail.
- [x] #6 Reproducible medium/large diff benchmarks record latency, responsiveness, memory, hardware and cold/warm state; failures and incomplete coverage are visible rather than hidden by dropped inventory.
- [x] #7 Independent verification results and residual threat-model limits are recorded separately from author claims; no real-world safety or universal behavior claim is made.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [x] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the existing Go adversarial, Docker, CLI and PTY proofs in one serial test:poc target; require named acceptance tests and reject skips. Wire the authorized gate into Linux CI with separate pinned-image provisioning. 2. Add isolated Go-overlay negative controls for broken review binding and a weakened independent observer, plus missing workload-crash and real build-failure probes. 3. Extend captured-diff measurements to medium/large complete inventories, cold/warm raw reads, cached list and input responsiveness during an active fixture; record hardware/cache/memory and visible budgets. 4. Run the complete gate and project checks, obtain one independent criteria-based verification, document actual results and threat limits, integrate and commit on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented test:poc orchestration, no-skip/required-proof guards, actual production-code overlay negative controls, Linux CI gate, real workload-crash/literal-argv/build-failure probes, and medium/large complete-inventory timings plus active-fixture PTY response timing. task check passed in the implementation worktree (build, race tests, vet, formatting, local links, backlog integrity, secrets); LSP diagnostics clean. Initial full gate was interrupted by the shell tool 1200s deadline after real CLI/PTY proofs passed (active-fixture help render 15.75ms); NOT a gate pass. Inspected and removed its single exact session-owned staging container; no derived images remained. Full serial gate restarted as a tracked background shell job with log/exit artifact. Fresh-context independent verifier is running read-only. Residual limit explicitly documented: workload-crash cleanup is tested; host supervisor SIGKILL/daemon death can bypass deferred cleanup and does not have automatic orphan recovery.

Independent verification (fresh-context verifier, one bounded pass): no concrete item-scoped defect found in gate fail-closed behavior, CI wiring or performance harness. Independently ran task test:terminal, task terminal:bench and git diff --check: passed; complete 10/10 and 50/50 paths/hunks, Apple M5 Max darwin/arm64, warm viewport 29,727 ns/op, 16,185 B/op, 285 allocs/op. Verifier explicitly left full Docker/mutation acceptance unverified pending the parent-owned gate; this is separate from author execution claims. Host supervisor SIGKILL/daemon/power-loss cleanup remains a documented residual limit.

Final author evidence: task test:poc completed exit 0; all required real Docker, protected observer, CLI and PTY proofs passed with zero skips. Snapshot-binding overlay failed the expected bad-reopening assertion; actual embedded observer overlay failed got 1 calls want 2; gate accepted only those expected negative-control failures. Workload SIGKILL with live descendant, literal shell metacharacter argv, real build failure, offline/credential/resource/timeout/cancel probes all passed. No owned sandbox containers or input images remained. On Apple M5 Max (18 CPUs, 64GiB, darwin/arm64), medium/large warm raw open 1.468/5.791ms, preparation 1.281/4.779ms and 2.17/11.61MB allocated; active-fixture PTY navigation 15.31ms. Full measurements and cache/threat limits are in docs/POC-GATE.md. task check passed; implementation staged checks passed. Commit aa9ecad fast-forwarded to main; task test passed again after integration. CI workflow is configured but remote CI was not run because no push was requested. Full local gate log/exit preserved in ignored artifacts/poc. Independent verification was one pass, no concrete findings; its narrower executable coverage is separately recorded above. Session-owned after-15-gate checkout and branch removed after receipt, inactive-process and pane verification; exact Herdr workspace removal confirmed. Pre-existing worktrees were neither adopted nor modified. No remaining task blocker; no next task started.

Review 2026: Reviewed pocgate skip/failure/required-proof accounting, overlay mutation controls (sites unchanged by this review) and CI poc job (bash pipefail propagates tee'd failures). No findings; full test:poc not rerun in this review.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered aa9ecad on main: one no-skip adversarial POC gate, Linux CI wiring, binding/observer mutation controls, workload-crash/build/argv probes, medium/large benchmarks and explicit limits. Verified with task test:poc, task check, integrated task test and independent terminal/benchmark checks. Host supervisor SIGKILL/daemon failure remains an explicit manual-cleanup limit; no production-safety or human-study claim. Final task state committed separately; no push.
<!-- SECTION:FINAL_SUMMARY:END -->
