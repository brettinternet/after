---
id: AFTER-17
title: Prepare the counterbalanced review-loop evaluation kit
status: Done
assignee: []
created_date: '2026-10-03 05:44'
updated_date: '2026-10-04 02:31'
labels:
  - poc
  - evaluation
  - docs
  - reviewed
milestone: m-1
dependencies:
  - AFTER-16
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: medium
type: docs
ordinal: 17000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A terminal demo cannot prove the product hypothesis. Scope: a runnable evaluation kit and scoring protocol, not recruitment or invented user measurements. Keep answer keys grounded in independently captured effects; use this to prevent optimizing only for screen polish.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A versioned study kit provides equivalent raw-diff, strong guided-tour and AFTER conditions over unfamiliar changes plus controlled follow-up edits; ordering is randomized/counterbalanced with reproducible assignment.
- [x] #2 Cases include the consequential payment side effect, misleading changed tests, absent evidence, stale observations and harmless edits that trigger unnecessary reopening; independent answer keys identify concrete consequences and limitations.
- [x] #3 Instructions and recording sheets measure correct explanations, missed defects, re-review time, stale-evidence trust errors, useful-example availability, setup effort and unnecessary reruns without collecting credentials or private source.
- [x] #4 Proposed 12–16 participants and roughly 30 percent re-review-time improvement are clearly targets, with defect-detection and false-freshness guardrails, not claimed results; small-sample interpretation limits are explicit.
- [x] #5 An automated kit consistency/smoke check proves fixtures and answer-key evidence still match the shipped engine; a dry run is labeled an author rehearsal, not user research.
- [x] #6 The handoff states that M1/M2 technical completion does not establish usability or market value and identifies the exact human recruitment/scheduling needed for AFTER-18.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [x] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the owned native-CLI demo harness to generate three synthetic initial/follow-up study cases, immutable review IDs, raw diffs and independently observed facilitator keys. 2. Add reproducible counterbalanced assignments, equivalent condition instructions, blank recording sheets and bounded scoring/human handoff. 3. Add focused consistency tests and an explicit Docker-backed author-rehearsal task; run actual proofs and one scoped independent review. 4. Record evidence, finalize and commit on main as requested.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented and committed directly on main as explicitly requested: 3f7d47c. Kit v1 reuses the owned native demo harness for A repair, B harmless comment, C weakened oracle; separate missing/initial/follow-up stores prevent future-result leakage, and facilitator keys come from separately executed protected-observer receipts. Candidate tests are captured source, not falsely claimed passing reports. Seeded six-permutation assignment tests cover 6/12/16/18 slots and document incomplete case-period balance. Protocol, strong tour, anonymous CSV sheets, scoring and precise human handoff are in docs/EVALUATION.md. Verification passed: mise exec -- task study:check; task study:assign -- --assign-seed cohort-01 --participants 12; task study:proof (six actual offline runs, all initial 12h counts 1→2, A follow-up 1→1, B/C 1→2, all 30s controls 1→1, equal HTTP responses and all pins stale/missing after follow-up); task check; task check:staged. Local proof used macOS arm64/Colima and pinned toolchain image; CI is configured, not claimed newly run. Initial proof exposed CopyFS private-mode mismatch, fixed and covered by native copied-store inspection. One independent scoped review found wrong documented TUI pin syntax; fixed to candidate/base/evidence and exercised both initial current and follow-up stale commands in real PTYs with clean quit. LSP diagnostics clean. Logs remain in ignored artifacts/after-17/. Successful proof cleaned its owned workspace; the first failed owned workspace was moved to Trash after PTY verification. No new worktrees; pre-existing worktrees untouched. No push, human observations or usability claims. Next step requires explicit AFTER-18 recruitment/scheduling authorization, facilitator plus second scorer, 12–16 consenting unfamiliar engineers, privacy/deletion ownership and 75-minute sessions; not started.

Review 2026: No defects found. Checked assignment counterbalancing (period/condition and case/condition balance at multiples of 6; documented case/period gap at 12/16), case setup/follow-up edits against key counts (A repair 1->1; B comment, C weakened oracle 1->2), isolated missing/initial/follow-up/facilitator stores and docs. Shares AFTER-16's consent fix: study:proof now passes --authorize-synthetic-plans instead of ambient env. task study:check (go test -race ./internal/demo) PASS; study:proof not rerun (same execute path verified by demo:proof).
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered versioned counterbalanced raw/tour/AFTER study kit, isolated synthetic checkpoints, real observer-backed keys, anonymous scoring sheets and explicit human-study handoff. Verified six real runs, focused assignment/capture tests, actual current/stale TUI PTYs and full task check. Independent review command defect fixed. Implementation 3f7d47c on main; author rehearsal only, no user-research results or push.
<!-- SECTION:FINAL_SUMMARY:END -->
