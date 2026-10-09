---
id: AFTER-50
title: Run declarative command scenarios that compare exit status and output streams
status: Done
assignee: []
created_date: '2026-10-08 23:14'
updated_date: '2026-10-09 09:51'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-49
documentation:
  - docs/EXTENSIONS.md
  - docs/RUNNER.md
  - docs/SANDBOX.md
  - docs/COMPARISON.md
priority: medium
type: feature
ordinal: 50000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
CLIs, code generators and library test harnesses are observable at the process boundary: argv, stdin and a fixed environment go in; exit status, stdout and stderr come out. docs/behavior-and-evidence.md lists this as the useful comparison for CLI and library changes, and it applies to any language whose program runs in a pinned image. A `command` scenario kind reuses the definition, freezing and consent machinery from AFTER-49 and needs no observer container: the runner records streams from the Docker attachment instead of trusting program-reported output. Output file trees and interactive TTY programs are out of scope here.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An explicit operator selection is recorded before implementation; this follow-up is not selected automatically from the POC queue.
- [x] #2 A versioned command definition declares a pinned image, build argv, and cases (argv, stdin bytes, fixed environment, optional input files), repetitions and limits; it follows the same strict decoding, explicit selection, digest freezing, consent and changed-oracle rules as http-service definitions.
- [x] #3 Each case, side and repetition runs in fresh offline sandbox state; exit status, stdout and stderr are recorded at the container boundary with completeness and truncation flags; timeouts, output overflow, signals and build failures produce incomplete receipts, never equality.
- [x] #4 Comparison is exact for exit status and text streams and structural for streams the definition declares as JSON, with exact witnesses, all repetitions retained (disagreement is unstable) and the rules digest bound; missing, truncated or redacted streams are incomparable.
- [x] #5 A synthetic CLI fixture shows a changed case and an unchanged control end to end through capture, consent, run, compare, pin and reopen; tests cover hostile terminal output, binary stdout and nondeterministic output reported as unstable.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks and the opt-in Docker proofs pass; record exact commands, actual results or objective external blockers in task notes.
- [x] #2 Update RUNNER, COMPARISON, IMPLEMENTATION and EXTENSIONS docs with the new scope and limits; independent verification for the execution boundary.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; keep private artifacts out of Git.
- [x] #4 Record extension seams in task notes: what needed code rather than data, and what differed from the existing instance; these feed the schema step in docs/EXTENSIONS.md.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing bounded definition, frozen plan and consent machinery with command cases (direct argv, stdin, fixed environment and optional input files); preserve HTTP behavior and offline isolation.
2. Run each command sample in fresh sandbox state, collecting separate bounded container-boundary streams and exit status; retain incomplete failures and all repetitions. Extend exact/JSON comparison and existing CLI/TUI/pin projections without a plugin framework.
3. Add a synthetic command fixture and focused regression checks for parsing, consent, incomplete execution, binary/hostile output and instability; add an opt-in end-to-end Docker proof and update required scope/limits docs.
4. Run relevant Task checks and one independent execution-boundary acceptance verification; fix only concrete scoped defects and rerun affected checks.
5. Refresh claim and main, integrate tested implementation, commit final authoritative metadata and clean up the receipt-verified worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator explicitly selected the next available item among AFTER-48–50 and authorized worktree implementation, commits, integration into main and cleanup. AFTER-48 and AFTER-49 are Done; selected dependency-ready AFTER-50. Unrelated existing worktrees are preserved.

Implementation completed uncommitted in owned after-50-command worktree (workflow 983b0ad5, executor 5bae2839). Implementer reports task command:check, command:proof, test:poc (including both mutants), cli:help:update, format, format:go, check and git diff --check passed. Independent verifier 052083c8 is now re-deriving acceptance and execution-boundary evidence; no final acceptance or integration claimed yet. Approved conservative v1 semantics: explicit base64 stdin/files, exact binary-safe text comparison or declared structural JSON, additive collision-safe input files, build output suppressed to avoid contaminating target streams, direct exec preserving container exit status, and statuses 125–255 treated as incomplete. Task flags not forwarded by test:cli were corrected with a dedicated help-update target. Primary remains at 46458f4 with only this task metadata modified.

Independent verifier 052083c8 passed AC1–5 with no concrete scoped defect. It ran mise exec -- task check:go, task test, task command:check, and (with explicit trusted local Docker CLI/socket) task command:proof, task test:poc and task http-service:proof. All passed; POC required proofs ran without skips and both negative-control mutants were killed. Owner-labeled containers/images were absent after proofs. Parent gopls diagnostics clean for command definition, runner, comparator and sandbox; task check:staged passed. Implementation 02a7886 fast-forward integrated into unchanged main, preserving primary task state. No code changes after verification. Live evidence is Colima Linux arm64 only; no native Linux/Desktop/amd64 certification, universal behavior claim or daemon/power-loss cleanup guarantee.

Extension seams: command-specific native code captures Docker-boundary stdin/stdout/stderr and inspected exit status, prepares the trusted direct-exec/build launcher, validates stream receipts and compares exact bytes or declared structural JSON. Definition, consent, immutable storage, repetitions, evidence axes and pin invalidation reuse existing machinery. Unlike HTTP, command scenarios need no observer container or readiness protocol. Inputs and binary witnesses are bounded base64; output trees, TTYs, downloads and shell entrypoints remain unsupported. Build diagnostics are suppressed; reserved exit statuses remain incomplete. No external blocker remains; final metadata commit and receipt-verified cleanup are next.

Post-integration task command:check passed on main. Cleanup verified original session receipt, live checkout/branch, completed children and exact Herdr workspace w2T with only an idle zsh; wt remove after-50-command --foreground --format=json removed checkout and branch and its post-remove hook closed the matching workspace. Filesystem, branch and workspace absence confirmed; unrelated pre-existing worktrees preserved. Final task state is committed separately on main; no push and no next task started.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered AFTER-50 on main in 02a7886: frozen declarative command scenarios, fresh offline execution with container-boundary exit/stdout/stderr, exact byte/structural JSON comparison, conservative incomplete outcomes, and synthetic pin/reopen proof. Independent verification passed all criteria; task check, command:check, command:proof, test:poc (both mutants) and HTTP regression proof passed. Scope/limits and extension seams documented. Owned worktree, branch and workspace removed; no blocker remains.
<!-- SECTION:FINAL_SUMMARY:END -->
