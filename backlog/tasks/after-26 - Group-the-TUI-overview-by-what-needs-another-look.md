---
id: AFTER-26
title: Group the TUI overview by what needs another look
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 19:51'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-25
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/README.md
  - docs/behavior-and-evidence.md
priority: high
type: enhancement
ordinal: 26000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The landing list shows evidence in launch order, and only the selected row shows its state. Reopened pins, differing observations, reported failures, history from earlier snapshots and equal controls all look alike, so the reviewer must open each row to learn what needs attention. With no evidence, the landing screen is two lines and hides the change behind `d`. The product's contact sheet (docs/README.md "The contact sheet"; docs/behavior-and-evidence.md section 10) puts "needs another look" first.

Scope, per docs/TUI-DESIGN.md "Overview" and "Next line": groups in the design's order with counts and collapsible headers; report lines that tell same-named reports apart; the CHANGES line with potential oracles; the no-evidence variant that lists every path; and the Next line rules. Grouping and the Next line derive only from typed engine fields, through tested mappings. Until AFTER-30 adds `a`, the accept rule points to `after review --accept`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Overview groups rows as NEEDS ANOTHER LOOK, PINNED EXPECTATIONS, AGREES, REPORTED, EARLIER SNAPSHOTS (collapsed by default) and OTHER, in that order, with counts and without empty groups, and Enter on a header collapses or expands it.
- [x] #2 On the payment loop after the regression rerun, the 12h case and the reopened pin appear under NEEDS ANOTHER LOOK, the 30s control under AGREES, and the first run's rows under EARLIER SNAPSHOTS, labelled with the candidate they ran on.
- [x] #3 Each imported report has a line with its short ID, binding, import time and outcome counts, followed by the producer string as data, so two reports with identical test names are distinguishable; reported passes are never styled as observed results.
- [x] #4 The CHANGES line counts paths, hunks and potential oracles (from rawdiff, with every hunk unclassified) and lists potential-oracle paths; with no evidence, Overview states NOT CHECKED and lists every inventory path, including excluded and unknown ones.
- [x] #5 The Next line follows the design's rule table, a table-driven test covers every rule, and golden views cover the raw review, test report and payment loop overviews.
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
1. Extend the existing typed Entry/badge and rawdiff adapters with report provenance, overview groups/counts/collapse, and the full no-evidence inventory. 2. Add the design Next-line priority mapping, preserving transient Activity notices and navigation/action safety. Use the current design and real CLI spelling after pin PIN --accept (task description uses the superseded after review --accept). 3. Cover every mapping rule, raw/report/payment golden views, hostile content, and real PTYs at 80x24/120x40 in color and NO_COLOR; update TUI docs. 4. Independently verify the typed-state/history and navigation criteria, run relevant Task checks, commit implementation, fast-forward main, finalize task metadata, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented typed Overview groups, collapse/navigation, report provenance, rawdiff counts and complete raw inventory, and Next priorities. Independent verifier passed grouping/history/provenance/navigation criteria and identified one missing Otherwise blank test; parent added it and reran affected checks. Updated obsolete CLI PTY screen expectations and Docker proof navigation. Initial full CLI/Go checks timed out on the obsolete Stored records only expectation; focused diagnosis identified it, and final complete gates pass. Docker proof first exposed outdated detail/navigation and active-run labels, then a batched printable-key test issue; all corrected without weakening assertions. Final real proof passed in 202.67s, with active-run help latency 16.33ms. PTY excerpt: NEEDS ANOTHER LOOK 2 / REOPENED / DIFFERENT 12h provider requests 1 → 2 / AGREES 1 / EQUAL 30s provider requests 1 → 1 / EARLIER SNAPSHOTS 2; expanded history verified the original candidate label. Raw PTY excerpt: NOT CHECKED — no evidence was loaded / CHANGES 1 path · 1 captured hunk, all unclassified · 0 potential oracles. Real terminal size/color matrices covered 80x24 and 120x40 with and without NO_COLOR. Passed mise exec -- task test:cli, task check:go (build/all race tests/vet/gofmt), task test:terminal, task test:views, task test (141 documentation links and 45 backlog tasks), task format:check, task check:staged, git diff --check; affected LSP diagnostics clean. Passed task tui:proof with explicitly configured Docker binary/host after operator-authorized Colima start. Synthetic golden views remain explicitly synthetic; Docker proof supplies the real payment evidence. The accept hint uses the actual after pin PIN --accept command. Implementation d1b93dc fast-forwarded into main; no private evidence or credentials committed. Cleanup pending for the session-owned after-26-overview worktree; unrelated pre-existing worktrees preserved.

Cleanup completed: verified the creation receipt against Worktrunk, both delegated runs complete, and the exact Herdr workspace had only its idle shell. Worktrunk removed after-26-overview and its branch; the matching Herdr workspace disappeared. No owned worktree remains and no blocker or resumable implementation step remains. Colima was left running after the operator-authorized proof. Final task metadata is committed separately on main; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Grouped Overview by review priority with collapsible history, distinct reported provenance, complete raw inventory and typed Next guidance. Verified by independent acceptance checks, golden views, real size/color PTYs, full Go/CLI suites and the Docker-backed payment rerun proof. Implementation d1b93dc integrated into main; owned worktree/branch/workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->
