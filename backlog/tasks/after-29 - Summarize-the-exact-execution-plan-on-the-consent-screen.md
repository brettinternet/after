---
id: AFTER-29
title: Summarize the exact execution plan on the consent screen
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 02:41'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-23
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/SANDBOX.md
  - docs/RUNNER.md
priority: high
type: enhancement
ordinal: 29000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Before any execution, `r` shows the exact plan as 18 KiB of quoted 32-byte chunks and asks the reviewer to "Read all byte pages". Consent is a trust boundary: a reviewer who cannot read the plan cannot meaningfully approve it. The plan (internal/runner/plan.go, internal/sandbox/plan.go and observed.go) already contains AFTER-authored fields that answer the key questions: snapshots, argv, environment, image, Docker policy, mounts, topology and limits.

Scope, per docs/TUI-DESIGN.md "Consent": a modal consent screen with a Summary section decoded strictly from the exact preview bytes that `y` approves, values copied verbatim and only counts derived, plus an Exact plan section in the content viewer. Approval semantics are unchanged.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `r` opens a modal consent screen whose Summary shows the snapshots, runs (sides × cases × repetitions, and concurrency), build and app argv, app environment, image, topology, mounts, Docker policy and limits, copied verbatim from the decoded preview, listing distinct values when experiments disagree.
- [x] #2 The summary is decoded strictly, rejecting unknown fields, from the same bytes whose digest `y` approves; a test proves that a preview which fails decoding opens Exact plan with "summary unavailable" and that `y` still approves exactly those bytes.
- [x] #3 Exact plan uses the content viewer, and no byte-page wording remains.
- [x] #4 Only `y`, `n`, Esc, Tab, scrolling, `b`, `?`, `q` and Ctrl-C act on the consent screen; search is unavailable, quitting denies, and paste never approves; the existing invalidation rules (pair change, new preview, snapshot switch, active run or action) still block approval, with tests.
- [x] #5 Hostile values in argv or plan strings render as escaped data and cannot forge summary labels, and golden views cover the consent screen at 120×40 and 80×24.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the browser content viewer and contextual key map for modal Summary/Exact plan sections. Strictly decode the existing runner/sandbox preview shapes from the retained approval bytes, display escaped verbatim distinct values and derived run counts, and fall back to Exact plan on decode failure without changing authorization.
2. Preserve exact-byte approval and invalidation; restrict modal keys including help, deny on quit, and test pasted input, stale pairs/previews and active work.
3. Add focused summary/model tests, 120x40 and 80x24 golden views and real PTY color/NO_COLOR coverage; update TUI documentation and run relevant Task checks.
4. Independently verify the consent trust boundary once, fix concrete defects, commit and integrate on main, finalize authoritative task metadata and clean the receipt-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in receipt-owned after-29-consent; initial executor timeout recovered by resuming the same child after partial-diff inspection. Executor reports task test:views, test:terminal, test:cli, format:check and test passing; final focused tests rerun after worker/hex adjustments. Real consent PTYs pass 120x40 and 80x24 with color and NO_COLOR: Run this exact plan? / Summary / Runs 2x2x1=4 / Tab Exact plan / q denied / terminal restored. Initial PTY timeout traced to discarded same-frame unread text; helper corrected. Parent diff --check and consent.go LSP diagnostics clean. Independent consent-boundary verifier now running; not yet committed/integrated. Docker execution was not rerun because execution machinery is unchanged.

Delivered b617d25 on main by fast-forward after refreshing claim and primary state. Single independent verification found fixed-array JSON decoding could discard extra sides/cases; parent fixed with slices and exact 2x2 validation plus extra/missing side/case regression tests. All other criteria passed independent checks. Final parent checks passed: mise exec -- task format:go check:go test:views test:cli; mise exec -- task test; git diff --check; consent source/tests LSP clean. Post-integration mise exec -- task test:views test passed (full race suite, 140 links, 45 tasks). task check:staged and commit hooks passed with no leaks. No second general review was needed for the bounded decoder correction. Worktrunk removed the clean receipt-verified worktree and branch after confirming no active children and shell-only pane; Herdr workspace removal verified by workspace_not_found. Unrelated pre-existing worktrees preserved. No remaining blocker or resumable step; no push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Readable strict-decoded Summary and shared Exact plan viewer preserve exact-byte consent. Modal safety, hostile text, two golden sizes and four real PTYs pass. Fixed the independent verifier finding about silently discarded experiment dimensions. Implemented as b617d25 on main; owned worktree, branch and workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->
