---
id: AFTER-15
title: Enforce the adversarial evidence and execution acceptance gate
status: To Do
assignee: []
created_date: '2026-10-03 05:44'
labels:
  - poc
  - verification
  - security
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
- [ ] #1 A single task test:poc target executes all ten acceptance checks mapped in docs/IMPLEMENTATION.md against the real engine and TUI integration; CI runs them and fails on a deliberately broken binding or observer.
- [ ] #2 Fault-injection tests cover oracle edits, every invalidation input, out-of-order completion, build/start/driver failures, missing artifacts and unstable repetitions; none become false current equality or fabricated observations.
- [ ] #3 Hostile fixtures probe shell/argv injection, Git helper execution, path/symlink/archive-like traversal, terminal ESC/OSC payloads, forged badges, overlarge logs and secret redaction with no unintended command or data exposure.
- [ ] #4 Executable sandbox tests confirm no external network/host credential access, resource bounds and complete owned-process/container cleanup after timeout/cancel/crash; test skips cannot masquerade as a passing execution gate.
- [ ] #5 Deduplication mutation changes the independent provider observation while the response remains equal; an intentionally weakened observer causes the gate to fail.
- [ ] #6 Reproducible medium/large diff benchmarks record latency, responsiveness, memory, hardware and cold/warm state; failures and incomplete coverage are visible rather than hidden by dropped inventory.
- [ ] #7 Independent verification results and residual threat-model limits are recorded separately from author claims; no real-world safety or universal behavior claim is made.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [ ] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->
