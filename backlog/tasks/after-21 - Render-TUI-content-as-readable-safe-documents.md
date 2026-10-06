---
id: AFTER-21
title: 'Render TUI content as readable, safe documents'
status: Done
assignee: []
created_date: '2026-10-06 20:29'
updated_date: '2026-10-06 22:12'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies: []
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
  - docs/TERMINAL.md
priority: high
type: enhancement
ordinal: 21000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every TUI content page is unreadable today. `pageLines` in internal/browser/model.go Go-quotes the payload and splits it every 32 bytes (`"diff --git a/shipping.go b/shipp"`, then `"ing.go\n"`), tabs show as `\t`, nested JSON as `\\\"`, and `[`/`]` step through 4 KiB byte windows (one receipt takes five pages). The quoting was a safety measure: it stops payload newlines and control sequences from forging trusted rows. `internal/terminal` (`Document`, `Line`) already gives that guarantee per line, so content can be shown as real lines without weakening it.

Scope: one shared content viewer for every document the TUI shows today (inspector sections, captured sources, the raw patch, the exact plan), as specified in docs/TUI-DESIGN.md "Content viewer" and "Rendering untrusted content safely". AFTER-written JSON artifacts are the observation, sample, comparison details and execution plan; captured sources, diagnostics and report output stay verbatim. If indenting would exceed the document limits, show the stored bytes verbatim. Stored blobs are at most 16 MiB (`store.MaxBlobBytes`), so only the 250,000-line limit can be exceeded; the hex view can compute rows from byte offsets without an index.

Out of scope: styling and badges (AFTER-22), cards (AFTER-27), diff-specific old/new line numbers and file navigation (AFTER-28), search (AFTER-31). Keep the existing screens and other keys.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Inspector sections, captured sources, the raw patch and the exact plan render one captured line per row behind a trusted line-number gutter; no row is Go-quoted or split mid-line, and tabs expand to four-column stops.
- [x] #2 Scrolling (j/k, PgUp/PgDn, Home/End, g/G) moves continuously through the whole document; `[`/`]` byte paging and all byte-page wording are gone, and the 100,000-line captured-diff budget test in `task test:terminal` still passes.
- [x] #3 Payload newlines, ESC/CSI/OSC sequences, CR, bidi controls and leading combining marks cannot create, overwrite or restyle a row or the gutter; the existing hostile-content tests run against the new viewer.
- [x] #4 Content with a NUL byte in its first 8,000 bytes opens as a hex dump, and `b` toggles an exact hex view for any document, so CR, BOM and trailing whitespace are visible and every stored byte is reachable.
- [x] #5 Stored JSON records and AFTER-written JSON artifacts display indented; captured sources, diagnostics and report output display verbatim; stored bytes are never modified.
- [x] #6 Documents over the `terminal.Document` line limit show an explicit limitation while the hex view still reaches every byte, and lines longer than 4 KiB show a clip marker pointing to the hex view.
- [x] #7 docs/TUI.md documents the viewer keys, and the examples/tui hints no longer mention byte pages.
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
1. Replace browser byte pages with a whole-document viewer indexed off the event loop using terminal.Document, trusted line gutters, bounded horizontal pan and exact offset-based hex rows.
2. Preserve raw bytes; indent only typed AFTER JSON sections with fallback at document limits. Detect binary NULs and expose explicit line/long-line limitations.
3. Update navigation, consent/help/docs/examples and focused hostile-content, byte-roundtrip, large-document and real PTY tests at both required sizes/color modes.
4. Run focused Task checks and independent acceptance verification for the rendering trust boundary; fix concrete findings, commit implementation, fast-forward main, finalize task metadata and clean the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented and fast-forwarded to main as 3a15130 after rebasing over the intervening CLI design/backlog commit. Shared off-loop indexed documents retain exact raw hex bytes, trusted gutters, safe pan, typed JSON formatting and explicit limits. The bounded JSON indenter caps output before allocation growth; malformed or over-budget artifacts fall back verbatim.

Validation: mise exec -- task format:go, test:terminal, test:cli, check:go and test all passed. After rebase, test:terminal, test:cli and test passed again (140 links, 45 tasks). task examples passed, including corrected TUI hints. task check:staged passed before implementation commit after task format fixed docs/TUI.md formatting. LSP diagnostics clean for terminal/text.go, browser/data.go and browser/model.go. Initial large-document preparation exceeded budget; ASCII indexing optimization resolved it. Final 100250-line captured-diff test: 55.8ms prepare, 11.6MB allocation, 1.01ms max input/view, 0.85ms max resize/view on darwin/arm64; OS cache warm from capture.

Independent verifier exercised terminal tests and TestBrowserDocumentPTY at 80x24 and 120x40, NO_COLOR set and unset. All safety/navigation/hex/limit checks passed. Its sole concrete defect was an obsolete example hint for removed bracket keys; corrected to continuous scroll and b hex, then task examples executed the corrected hint. Synthetic real PTY excerpt: 1 │ package main; 8 │ STATE observed | forged; 13 │ last tail. Forged STATE text remains behind the trusted gutter; terminal restoration checked.

Docker binary/host settings were absent; no Docker execution or full Docker POC claim is made. Task changes concern document presentation, with exact-plan consent covered by existing browser tests. No credentials or private evidence committed. Owned implementation worktree and branch removed via Worktrunk; associated Herdr workspace verified gone. Pre-existing unrelated worktrees were not adopted or removed. No remaining blocker or resumable step for this task; no push performed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Replaced quoted byte pages with readable, continuously scrollable safe documents and exact hex access. Added bounded typed JSON formatting, explicit text limits, hostile-content tests and four real PTY configurations. Independent verification defect in example hints fixed; focused/full Go checks, examples and staged checks passed. Delivered on main in 3a15130; owned worktree, branch and workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
