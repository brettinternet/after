---
id: AFTER-30
title: Confirm TUI mutations and accept pins from the TUI
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-07 23:54'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-27
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/REVIEW.md
priority: medium
type: feature
ordinal: 30000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Mutations happen on a single key press with canned reasons. `p` twice creates two identical pins, and switching snapshots (`u` since AFTER-23) reopens pins without confirmation. The loop also cannot close inside the TUI: accepting a pin against its current complete result works only headless (`after review --accept`), so reviewers must leave the TUI and assemble IDs to record a decision.

Scope, per docs/TUI-DESIGN.md "Prompts": confirmation prompts for `p`, `u` and a new `a` (accept pin) that state the exact effect and target IDs, with a one-line reason field (prefilled default that Ctrl-U clears, typing and bracketed paste, sanitized, at most 4,096 bytes, empty refused); duplicate-pin refusal; and `a` enabled only when the engine would accept. Engine rules are unchanged: acceptance requires a current complete receipt, and the engine enforces that independently of the UI.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `p`, `u` and `a` open a prompt naming the effect and the exact target IDs; Enter with a non-empty reason performs the mutation, Esc performs none, and selection, navigation and paste never mutate.
- [x] #2 The reason field accepts typing and bracketed paste, Ctrl-U clears the prefilled default, control characters are escaped, 4,096 bytes are enforced, empty input is refused, and the stored pin history contains the entered reason.
- [x] #3 `p` on a case that already has a pin with the same basis receipt and expectation creates no new pin and points to the existing one.
- [x] #4 `a` on a pin with a current complete receipt for the selected pair records `accepted` through the engine and the row shows `[ACCEPTED]`; otherwise the hints omit `a`, pressing it explains why, and an engine rejection is shown rather than masked.
- [x] #5 The `u` prompt names the new candidate, states that the original base is kept, counts the paths that differ from the candidate under review, and says that pins may reopen and earlier results become history.
- [x] #6 PTY tests cover pin, duplicate refusal, snapshot use, acceptance and cancelling each prompt, and the Docker-gated `task tui:proof` is updated and run when Docker settings are available.
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
1. Extend the existing browser action/key loop with frozen-target confirmation prompts for pin, candidate selection and acceptance; reuse review engine validation and record sanitized bounded reasons. 2. Refuse duplicate receipt/expectation pins and derive acceptance availability from current complete evidence. 3. Add focused model/action tests and PTY coverage including cancellation, paste, dimensions/color and update the Docker proof and docs. 4. Run applicable Task checks and available Docker proof, perform one independent acceptance verification, fix concrete defects, commit implementation, integrate main, finalize task metadata and clean the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is present in owned after-30-confirm worktree; no integration yet. Passed mise exec -- task test:tui-mutations, test:terminal, test:cli, format:check; parent additionally passed task check:go (build, all race tests, vet, gofmt) and prompt.go LSP diagnostics. Executor hit an initial 30-minute timeout and resumed; four Docker proof attempts exposed PTY synchronization/status assertions, not unavailable Docker. Latest snapshot-cancel assertion now checks unchanged pair and pin history rather than a status superseded by pending capture. Independent verifier is running the authorized real Docker proof and acceptance review; AC/DoD remain unchecked pending evidence.

Delivered implementation commit 24f3d9d by fast-forward to main. Independent verifier found no concrete product defect in AC1-5 and passed focused race tests; its Docker run failed on clipped historical-row text. Parent recovered the oversized log and corrected that assertion, accepted-badge assertion order, and obsolete reopening wording. Final authorized task tui:proof PASS (296.63s), including real pin/duplicate/capture/cancel/accept prompts at 80x24 and 120x40, color and NO_COLOR, persisted reasons, real 1->2 witness and 1->1 control, cancellation/restart and terminal restoration. PTY excerpt: Pin this finite expectation?; Matching pin already exists; Use this captured candidate?; original base is kept; 1 paths differ; Pins may reopen; earlier results become history; ACCEPTED. Active-run help latency 15.92225ms. Final task test PASS (all race tests, 141 links, 45-task integrity); task check:go and check:staged PASS. Docker binary and host were explicitly supplied for the local offline daemon; no host fallback, credentials or private receipts committed. No remaining blocker; no further task authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Confirmed p/u/a mutations with frozen target IDs and bounded escaped reasons, refused duplicate pins, and added engine-backed acceptance. Verified focused tests, full Go checks, documentation/backlog checks and real Docker PTY proof. Implementation 24f3d9d integrated into main; task metadata finalized separately. No push.
<!-- SECTION:FINAL_SUMMARY:END -->
