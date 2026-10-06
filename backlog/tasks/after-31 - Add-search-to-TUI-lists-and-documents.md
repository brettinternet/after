---
id: AFTER-31
title: Add search to TUI lists and documents
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies:
  - AFTER-27
  - AFTER-28
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/TERMINAL.md
priority: medium
type: feature
ordinal: 31000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Nothing can be found except by scrolling: not a path in a large inventory, a test in a report, a string in a 100,000-line patch, nor a flag in a plan.

Scope, per docs/TUI-DESIGN.md "Key map": `/` opens a one-line query, `n`/`N` move between matches, and matches are highlighted in lists (Overview, Changes, Activity) and documents (detail parts, sources, Diff). Matching is a plain substring over the displayed sanitized text, case-insensitive unless the query contains an upper-case letter. Searching large documents runs off the event loop and is cancellable. Search is unavailable on the consent screen, where `n` keeps meaning deny.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `/` opens a query line in lists and documents; Enter jumps to the first match after the cursor, `n`/`N` cycle forward and backward with wraparound, Esc clears, and the status line shows `match k of n` or `no matches`.
- [ ] #2 Matching is a plain substring over the displayed sanitized text, case-insensitive unless the query contains an upper-case letter, and highlighting cannot be altered by content or change the row structure.
- [ ] #3 Search in the 100,000-line captured diff runs off the event loop, keeps input within the docs/TERMINAL.md per-event budgets, and is cancelled when the query changes or the view closes.
- [ ] #4 The query accepts typed and pasted text (sanitized and bounded), paste never triggers an action, and `/` does nothing on the consent screen, where `n` still denies.
- [ ] #5 Tests cover list and document search, wraparound, case rules, hostile queries and content, and cancellation, and docs/TUI.md documents search.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
