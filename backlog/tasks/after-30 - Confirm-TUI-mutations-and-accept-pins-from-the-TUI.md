---
id: AFTER-30
title: Confirm TUI mutations and accept pins from the TUI
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-06 20:34'
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
- [ ] #1 `p`, `u` and `a` open a prompt naming the effect and the exact target IDs; Enter with a non-empty reason performs the mutation, Esc performs none, and selection, navigation and paste never mutate.
- [ ] #2 The reason field accepts typing and bracketed paste, Ctrl-U clears the prefilled default, control characters are escaped, 4,096 bytes are enforced, empty input is refused, and the stored pin history contains the entered reason.
- [ ] #3 `p` on a case that already has a pin with the same basis receipt and expectation creates no new pin and points to the existing one.
- [ ] #4 `a` on a pin with a current complete receipt for the selected pair records `accepted` through the engine and the row shows `[ACCEPTED]`; otherwise the hints omit `a`, pressing it explains why, and an engine rejection is shown rather than masked.
- [ ] #5 The `u` prompt names the new candidate, states that the original base is kept, counts the paths that differ from the candidate under review, and says that pins may reopen and earlier results become history.
- [ ] #6 PTY tests cover pin, duplicate refusal, snapshot use, acceptance and cancelling each prompt, and the Docker-gated `task tui:proof` is updated and run when Docker settings are available.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
