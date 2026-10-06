---
id: AFTER-43
title: Start a new review with after review --new and show the saved review in status
status: To Do
assignee: []
created_date: '2026-10-06 21:41'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-24
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/TUI-DESIGN.md
  - docs/CLI.md
  - docs/TUI.md
priority: medium
type: feature
ordinal: 43000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
AFTER-24 saves one review per checkout and always resumes it. When a reviewer moves on to an unrelated change with the same capture flags, `after review` reopens the old pair, and the only way out is deleting `.after/session.json` by hand. `after status` does not mention the saved review, and Next blocks written by AFTER-37 cannot suggest a plain `after review` if it did not exist yet.

Scope, per docs/CLI-DESIGN.md "status" and "review" and docs/TUI-DESIGN.md "Launch and resume": `after review --new` captures as usual, replaces the saved review with the fresh pair, and names the replaced review on stderr. When the saved review is on another pair, `after status` shows a `Saved review` row, and its Next block offers resuming and starting over. Next blocks suggest a plain `after review` for the newest capture's pair or the saved review's pair, and the explicit pair otherwise. The saved review is UI state, so no evidence is lost.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `after review --new` captures with the given flags, replaces the saved review with the fresh pair, names the replaced review on stderr, and opens the fresh pair; nothing is removed from the store, and pins from the old review are still discovered.
- [ ] #2 When the saved review is on another pair than the newest capture, `after status` shows a `Saved review` row naming it, its Next block offers `after review` and `after review --new`, and `after status --json` includes the saved pair.
- [ ] #3 Next blocks suggest a plain `after review` for the newest capture's pair or the saved review's pair, and `after review BASE CANDIDATE` for any other pair; tests run each suggested command.
- [ ] #4 PTY tests cover `--new` with and without a saved review, and docs/CLI.md and docs/TUI.md document starting over.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
