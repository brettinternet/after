---
id: AFTER-29
title: Summarize the exact execution plan on the consent screen
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
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
- [ ] #1 `r` opens a modal consent screen whose Summary shows the snapshots, runs (sides × cases × repetitions, and concurrency), build and app argv, app environment, image, topology, mounts, Docker policy and limits, copied verbatim from the decoded preview, listing distinct values when experiments disagree.
- [ ] #2 The summary is decoded strictly, rejecting unknown fields, from the same bytes whose digest `y` approves; a test proves that a preview which fails decoding opens Exact plan with "summary unavailable" and that `y` still approves exactly those bytes.
- [ ] #3 Exact plan uses the content viewer, and no byte-page wording remains.
- [ ] #4 Only `y`, `n`, Esc, Tab, scrolling, `b`, `?`, `q` and Ctrl-C act on the consent screen; search is unavailable, quitting denies, and paste never approves; the existing invalidation rules (pair change, new preview, snapshot switch, active run or action) still block approval, with tests.
- [ ] #5 Hostile values in argv or plan strings render as escaped data and cannot forge summary labels, and golden views cover the consent screen at 120×40 and 80×24.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
