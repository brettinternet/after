---
id: AFTER-21
title: 'Render TUI content as readable, safe documents'
status: To Do
assignee: []
created_date: '2026-10-06 20:29'
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
- [ ] #1 Inspector sections, captured sources, the raw patch and the exact plan render one captured line per row behind a trusted line-number gutter; no row is Go-quoted or split mid-line, and tabs expand to four-column stops.
- [ ] #2 Scrolling (j/k, PgUp/PgDn, Home/End, g/G) moves continuously through the whole document; `[`/`]` byte paging and all byte-page wording are gone, and the 100,000-line captured-diff budget test in `task test:terminal` still passes.
- [ ] #3 Payload newlines, ESC/CSI/OSC sequences, CR, bidi controls and leading combining marks cannot create, overwrite or restyle a row or the gutter; the existing hostile-content tests run against the new viewer.
- [ ] #4 Content with a NUL byte in its first 8,000 bytes opens as a hex dump, and `b` toggles an exact hex view for any document, so CR, BOM and trailing whitespace are visible and every stored byte is reachable.
- [ ] #5 Stored JSON records and AFTER-written JSON artifacts display indented; captured sources, diagnostics and report output display verbatim; stored bytes are never modified.
- [ ] #6 Documents over the `terminal.Document` line limit show an explicit limitation while the hex view still reaches every byte, and lines longer than 4 KiB show a clip marker pointing to the hex view.
- [ ] #7 docs/TUI.md documents the viewer keys, and the examples/tui hints no longer mention byte pages.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Checked in a real terminal at 80×24 and 120×40, with and without NO_COLOR; a capture or PTY excerpt is recorded in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
