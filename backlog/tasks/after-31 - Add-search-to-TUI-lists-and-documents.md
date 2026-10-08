---
id: AFTER-31
title: Add search to TUI lists and documents
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-08 00:44'
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
- [x] #1 `/` opens a query line in lists and documents; Enter jumps to the first match after the cursor, `n`/`N` cycle forward and backward with wraparound, Esc clears, and the status line shows `match k of n` or `no matches`.
- [x] #2 Matching is a plain substring over the displayed sanitized text, case-insensitive unless the query contains an upper-case letter, and highlighting cannot be altered by content or change the row structure.
- [x] #3 Search in the 100,000-line captured diff runs off the event loop, keeps input within the docs/TERMINAL.md per-event budgets, and is cancelled when the query changes or the view closes.
- [x] #4 The query accepts typed and pasted text (sanitized and bounded), paste never triggers an action, and `/` does nothing on the consent screen, where `n` still denies.
- [x] #5 Tests cover list and document search, wraparound, case rules, hostile queries and content, and cancellation, and docs/TUI.md documents search.
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
1. Extend the existing safe Document, theme, browser key map and owned background-worker patterns with bounded smart-case substring search; keep consent and mutation prompts isolated. 2. Search immutable sanitized list/document text off the event loop, cancel obsolete work, highlight without changing row geometry, and navigate with wraparound. 3. Add focused safety, navigation, cancellation, 100,000-line budget and real PTY tests at both required sizes/color modes; update TUI help/docs. 4. Run relevant Task checks and one independent acceptance review, correct scoped defects, commit in the worktree, fast-forward main, finalize task metadata and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation checkpoint: worktree after-31-search contains search implementation/tests/docs; executor run 5f381d10-7114-4712-9068-e37e22a0c9c2 reached its 30-minute deadline during validation. Parent inspected preserved changes and git diff --check passed; resumed same child context as 384bf898-9882-4f4c-9f94-67973fbc5344 to finish validation. No implementation commit or merge yet; independent verification remains pending.

Delivered implementation commit 545c3ed and fast-forwarded main. One independent verifier pass (588cae89) passed terminal safety, consent, cancellation, budgets and real PTY matrix, and found offscreen long-line document hits. Fixed by retaining display-column offsets and panning to the first substring; TestDocumentSearchRevealsOffscreenMatch passes for source/Diff, wide Unicode/tabs, color/NO_COLOR and wraparound. No second general review. Six card goldens changed only for the new / search footer. Final checks passed: mise exec -- task test:terminal; mise exec -- task check:go (build, full race tests, vet, gofmt); mise exec -- task test (full race tests, 141 local links, 45-task graph); mise exec -- task check:staged (formatting/secrets); git diff --check. Focused regression command: mise exec -- go test -race ./internal/browser -run ^TestDocumentSearchRevealsOffscreenMatch$. LSP diagnostics clean on changed search/render files. Initial validation timeout and stale goldens were resolved. Final 100000-line search input+view: 1.880042ms, 75504 allocated bytes, cancelled request rejected after view change; below 100ms/256KiB per-event budgets. Real PTY TestResponsiveFramePTY passed 80x24 and 120x40 with color and NO_COLOR. Excerpts: > M app/⟦config.go⟧ +1 · −1; match 1 of 1; Diff: const deduplicate = true; match 1 of 2. Search is bounded to displayed indexed text (existing line/index limits), query capped at 512 bytes; consent remains unsearchable and n denies. No Docker execution proof needed for this browse-only change. Owned after-31-search worktree and branch removed via Worktrunk after receipt, clean-state, completed-child and shell-pane verification; pre-existing worktrees were not adopted or touched.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added safe smart-case search to TUI lists/documents, cancellable background scans, wraparound navigation, match highlighting/status and long-line hit panning. Verified full Go/race/vet checks, terminal budgets, consent/paste safety and real PTYs at both required sizes/color modes. Independent finding corrected with regression coverage. Implementation 545c3ed integrated into main; final task state committed separately; owned worktree/branch cleaned up. No remaining blocker or next resumable step for this item.
<!-- SECTION:FINAL_SUMMARY:END -->
