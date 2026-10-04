---
id: AFTER-16
title: Package the POC and ship a repeatable real demo
status: Done
assignee: []
created_date: '2026-10-03 05:44'
updated_date: '2026-10-04 02:31'
labels:
  - poc
  - release
  - docs
  - reviewed
milestone: m-1
dependencies:
  - AFTER-15
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: task
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The POC must be reproducible by someone other than its implementer. Scope: distribution builds, honest quick start, owned demo setup/teardown and real-engine CI. Do not publish a release or push binaries unless the current operator explicitly authorizes it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Documented build/package tasks produce versioned macOS/Linux native binaries with checksums, pinned Go/module dependencies and no runtime Bun requirement; supported architecture and execution prerequisites are explicit.
- [x] #2 task demo (or one equally clear documented command) creates an owned disposable fixture workspace and walks capture, run consent, inspect, pin, edit, reopen and rerun using real receipts and an unchanged control.
- [x] #3 Demo setup never resets the user's repository, copies secrets, silently pulls an image, starts production services or deletes an unknown path; cleanup targets only verified owned demo resources and leaves unrelated resources intact.
- [x] #4 A fresh prepared checkout can run the no-Docker import/inspection walkthrough and, after explicit image provisioning, the full offline paired demo; missing prerequisites get actionable errors.
- [x] #5 CI builds/tests supported host platforms and runs the sandbox POC gate on Linux with its exact image/toolchain identities; docs include diagnostic and recovery steps for store errors and cleanup failures.
- [x] #6 Root README/HANDOFF and command help accurately distinguish working capabilities, untested broader behavior and deferred features; the simulated presentation is not relabeled as observed product evidence.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks pass and exact commands, actual results or objective external blockers are recorded in task notes.
- [x] #2 Update affected docs and limitations; do not claim unrun experiments or human validation.
- [x] #3 Commit implementation and final task state using the repository delivery workflow; keep sensitive/private artifacts out of Git.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add pinned-Go four-target versioned packaging and checksum verification. 2. Add a Go demo harness using the native CLI and allowlisted synthetic fixture files, private owned temporary setup, exact-plan consent, no-Docker import path, verified cleanup and receipt/control assertions. 3. Exercise package/demo tasks in CI; refresh quick start, help, prerequisites and recovery limits. 4. Run focused tests, real offline demo, release checks and one independent risk-focused review; integrate and commit on main.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented native four-target package/checksum task, owned Go demo with native CLI calls and exact consent, no-Docker reported-evidence walkthrough, receipt-backed pin/edit/reopen/rerun and explicit CI host matrix. Verification passed: task demo:inspect; task demo:proof (actual identical HTTP responses, 12h effects 1→2 then 1→1, 30s control 1→1 both runs, stale/missing reopening then current attached receipt); task package under GOAMD64=v3/GOARM64=v8.2 contamination; SHA256SUMS checks; task check; task test:poc (all proofs, no skips, both mutations killed). Tests verify canonical-temp ownership/inode/token cleanup, symlink preservation, consent denial, package metadata/baselines and all four checksums. One independent scoped review found inherited CPU baselines; fixed with explicit v1/v8.0 and executable regression coverage. Initial capture decoding and rerun-base mistakes corrected; rerun uses the pin-selected original base, not the new paired-diff base ID. A tool timeout interrupted an initial run; exact containers/images were reconciled using creation times, inputs, argv, paired network and ownership labels and removed; later complete run cleaned normally. First gate rejected package-without-tests skip; real package test added and full gate rerun passed. Local proof is macOS arm64 with Colima Linux arm64, Go 1.27.1 and the pinned image; CI is configured, not newly executed or claimed green. No release publication, push or human validation.

Delivery: implementation d4cd6f1 fast-forwarded into main after claim/state refresh. Staged formatting and secret checks passed before implementation commit. Complete logs retained privately in ignored artifacts/after-16/. Session-owned implementation worktree/branch and its exact Herdr workspace removed with Worktrunk after creation-receipt and idle-pane verification; failed demo directories moved to Trash, Docker resource inventory empty. Older pre-existing worktrees were not adopted or modified. No remaining task blocker; AFTER-17 is next but not started.

Review 2026: Found the demo's no-prompt proof mode was enabled by ambient AFTER_DEMO_PROOF=1, so a leftover export made task demo (or study) skip exact-digest consent. Replaced with an explicit --authorize-synthetic-plans flag passed only by demo:proof/study:proof; DEMO.md updated. Packaging, owned cleanup, CI matrix and docs otherwise matched criteria. task check:go PASS; go test -race ./internal/demo ./internal/packagebuild PASS; task demo:inspect PASS; task demo:proof PASS on Colima (12h 1->2 then 1->1, 30s 1->1, reopen stale/missing, rerun attached); manual run with AFTER_DEMO_PROOF=1 and stdin 'no' now declines before execution.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered versioned four-target native packages/checksums and a repeatable owned native-CLI payment demo, with no-Docker report inspection, exact per-run consent, real receipts, pin/edit/reopen/rerun, safe cleanup, CI matrix and honest recovery documentation. Verified task check, demo:inspect, demo:proof, package/checksums and full test:poc including both killed negative controls. Independent review CPU-baseline defect fixed and regression-tested. Implementation d4cd6f1 integrated on main; no push/publication or human-study claims.
<!-- SECTION:FINAL_SUMMARY:END -->
