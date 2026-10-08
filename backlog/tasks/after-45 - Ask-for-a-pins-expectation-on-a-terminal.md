---
id: AFTER-45
title: Ask for a pin's expectation on a terminal
status: Done
assignee: []
created_date: '2026-10-06 21:41'
updated_date: '2026-10-08 14:53'
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
- [x] #1 On a terminal, `after pin RECEIPT` prints the receipt's pinnable cases with the TUI's suggested expectations, numbered, and reads one line; a number pins that suggestion, other text becomes the expectation verbatim, and an empty line or end of input creates nothing.
- [x] #2 Before reading, the prompt states the scope, basis receipt and pair; the created pin records the default scope and reason exactly as `--expectation` would; the CLI and TUI share one suggestion function.
- [x] #3 Without a terminal, `after pin RECEIPT` exits 2 with a runnable `--expectation` example and reads nothing; a receipt with no suggestion asks for the text only.
- [x] #4 Bare `after pin` offers `after pin <receipt>` in its Next block when the newest run has an observation without a pin; PTY tests cover choosing, typing, cancelling and no terminal, and docs/CLI.md documents the prompt.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [x] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [x] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [x] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the browser finite-case suggestion logic in the CLI; resolve receipt-or-pin IDs without changing pin inspection. Add a bounded, terminal-only one-line prompt with explicit basis/scope and unchanged creation defaults. 2. Offer an unpinned newest-run suggestion from bare pin; update help/docs. 3. Verify persistence, cancellation/EOF, no-read pipe behavior and 80-column PTY behavior with/without NO_COLOR; run focused Task checks and one independent review. 4. Commit implementation, fast-forward main, finalize task metadata and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented bounded terminal-only expectation input, shared browser/CLI finite-case wording, receipt-or-pin resolution, and unpinned newest-run Next guidance. Existing review.Create preserves scope/reason/history semantics. Empty line, EOF (including partial EOF), and oversized input create no pins; --interactive=false and pipes never read stdin. Help and docs updated. Verification: mise exec -- go test ./internal/cli -run "^TestPin(PromptInputBoundaries|ExpectationPTY)$" -v passed, exercising actual 80-column subprocess PTYs and pipes with NO_COLOR both unset/set; choices 1/2, verbatim spaced text, blank/EOF cancellation, text-only incomplete receipt and stored defaults checked. Excerpts: "Scope: finite_example", "Basis receipt:", "Base:", "Candidate:", "1. At 43200s, expect 1 provider request(s)", "Number or expectation (empty line cancels):"; pipe exits 2 with "--expectation" example; bare pin Next offers after pin RECEIPT until both suggestions are pinned. task check:go passed (build, all race tests, vet, gofmt); task test passed (all race tests, 142 links, 45 tasks); task check:staged passed (format and secrets); LSP diagnostics clean. Initial test:cli attempt timed out at 120 seconds; full CLI race suite subsequently passed in ~347 seconds. First check:go found only the changed help golden; explicitly regenerated TestCommandHelpGoldens/pin and reran successfully. Independent verifier b2143448-a75d-4530-9f4a-3cd59a6aa3b8 reported PASS with no concrete defects; focused pin/review and browser checks passed. Its sandbox blocked primary task access and explicit artifact writing, but managed output was persisted; parent confirmed authoritative task and ran all Task checks. Synthetic test artifacts validate input/persistence only, not execution; no Docker proof needed for this non-executing change.

Delivered implementation e3da3fa (Prompt for pin expectations on terminals), fast-forwarded into primary main after rechecking the authoritative claim and preserving its metadata changes. No push. Verified session creation receipt, clean checkout and completed verifier before Worktrunk foreground removal; branch deleted and exact Herdr workspace w25 disappeared after its post-remove hook. No checkout/workspace from this task remains. Pre-existing unrelated worktrees were left untouched. No remaining blocker or resumable implementation step.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
AFTER-45 complete: terminal pin creation now offers shared finite-case suggestions or verbatim text, shows scope/receipt/pair, and safely cancels on blank/EOF. Pipes require an explicit expectation without reading input; bare pin guides unpinned newest-run observations. Verified actual 80-column PTYs and pipes with/without NO_COLOR, persistence/defaults/bounds, full race suite/build/vet/format, links/backlog, staged secret checks, and independent acceptance verification. Implementation e3da3fa merged to main; task metadata finalized on main; owned worktree, branch and Herdr workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->
