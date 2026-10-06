---
id: AFTER-45
title: Ask for a pin's expectation on a terminal
status: To Do
assignee: []
created_date: '2026-10-06 21:41'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-37
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/REVIEW.md
priority: low
type: enhancement
ordinal: 45000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Creating a pin from the command line needs `--expectation TEXT`, so `after pin RECEIPT` alone can only fail. The TUI already suggests an expectation for each pinnable case, such as `At 43200s, expect 1 provider request(s) for the frozen two same-key requests; finite example only`.

Scope, per docs/CLI-DESIGN.md "pin": on a terminal, `after pin RECEIPT` without `--expectation` lists the TUI's suggested expectations for the receipt's pinnable cases, numbered, and reads one line. A number picks a suggestion, other text is the expectation, and an empty line cancels. The pin is created as with `--expectation`, with the default scope and reason. Without a terminal, the command exits 2 with a runnable example. The CLI and TUI share one suggestion function, and bare `after pin` suggests pinning from the newest run when it has an unpinned observation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 On a terminal, `after pin RECEIPT` prints the receipt's pinnable cases with the TUI's suggested expectations, numbered, and reads one line; a number pins that suggestion, other text becomes the expectation verbatim, and an empty line or end of input creates nothing.
- [ ] #2 Before reading, the prompt states the scope, basis receipt and pair; the created pin records the default scope and reason exactly as `--expectation` would; the CLI and TUI share one suggestion function.
- [ ] #3 Without a terminal, `after pin RECEIPT` exits 2 with a runnable `--expectation` example and reads nothing; a receipt with no suggestion asks for the text only.
- [ ] #4 Bare `after pin` offers `after pin <receipt>` in its Next block when the newest run has an observation without a pin; PTY tests cover choosing, typing, cancelling and no terminal, and docs/CLI.md documents the prompt.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
