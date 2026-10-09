---
id: AFTER-47
title: 'Make the review TUI full-screen, progressive and keyboard-friendly'
status: Done
assignee:
  - '@pi'
created_date: '2026-10-08 23:01'
updated_date: '2026-10-09 00:22'
labels:
  - poc
  - tui
  - ux
milestone: m-1
dependencies: []
documentation:
  - docs/TUI-DESIGN.md
  - docs/TUI.md
priority: medium
type: enhancement
ordinal: 47000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The review TUI shows a dense always-on key-hint row, floats its footer under short content, and gives every line equal weight, so reviewers cannot tell where to look next. The operator asked for a full-screen, sleek and minimal layout that uses whitespace, rules, contrast and colour to emphasize the right details, keeps shortcuts in ? help, adds vim/emacs navigation, and delta-style diff highlighting built into AFTER (no external delta: external diff programs and foreign escape sequences violate the safe-rendering boundary). The VHS demo tapes and GIFs should show the new UI.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 The frame fills the terminal: header, tab strip with rule, padded body and a footer pinned to the last rows; key hints are no longer always shown, the footer shows the next action or status plus '? help', and ? lists every key grouped with unavailable reasons
- [x] #2 Overview, Changes, details, consent, prompts, Activity and help use spacing, rules and colour from the closed theme vocabulary to emphasize state and next actions; meaning survives NO_COLOR
- [x] #3 Diff shows trusted file header bars, tinted added/removed lines with changed-word emphasis on 256-colour terminals, muted patch metadata and an aligned gutter; every captured patch line stays visible and diff rows align with their gutter line numbers
- [x] #4 Lists and documents accept vim and emacs navigation (j/k, Ctrl-N/P, Ctrl-D/U, Ctrl-F/B, Ctrl-V/Alt-V, g/G, Alt-</>, l/h open/back in lists, Ctrl-G back) and text fields accept Ctrl-W/Ctrl-H editing
- [x] #5 Golden views, browser and PTY tests pass with the new layout; docs/TUI.md, docs/TUI-DESIGN.md, docs/TERMINAL.md and examples reflect it; the review VHS tape and GIF are regenerated
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. internal/terminal: extend the closed Style set (accent, rule, key, selection bar, diff line/emphasis backgrounds with a 256-colour capability flag) and a selection helper that re-applies the row background after trusted resets.
2. Frame (model.go): header with project + pair, tab strip with active underline rule, 2-column margins, body padded to fill height, footer rule + next/status line with right-aligned '? help'; remove the always-on hint row; help becomes the full key reference.
3. Views: Overview groups with spacing/colour headings and an empty-state hero; Changes coloured letters/counts and a titled preview; document screens get a section strip instead of meta rows and a subtle gutter for Cards; consent/prompt/activity spacing.
4. Diff: fix the off-by-one between the patch document and DiffRows (no heading divider on the patch document), file header bars for every file, muted git metadata, tinted +/- rows and positional pairing with changed-span emphasis.
5. Keys: vim/emacs aliases in keyMap; l/h open/back in lists; Ctrl-W/Ctrl-H in search and reason fields.
6. Regenerate goldens, update browser/CLI tests for the new strings, docs and examples; update examples/demos/review.tape and regenerate review.gif; run task check.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Integrated as 861c476 (fast-forward after rebasing onto 99e89a4). Verification: go vet ./...; gofmt clean; go test ./... and task check:go (race) pass except TestCompletionScriptsInBashZshAndFish, which also fails on unmodified main because the mise fish shim has no version (environmental). Golden views/cards regenerated and reviewed; browser, frame/document PTY and CLI loop/new-review PTY tests updated to the new strings (tests now match visible text after stripping the closed theme SGR set). TestCapturedBrowserDiffEventBudget: alloc/event 131 KiB (budget 256 KiB) after making ChangedSpan allocation-free and capping it at 4 KiB lines. Manual tmux checks at 100x30/110x32/120x40 in xterm-256color; review.tape extended with Diff, ] and ? and review.gif regenerated with VHS 0.11.0 (frames inspected). Decisions: badges keep brackets in every mode (matches CLI output); emphasis backgrounds 22/52 with 28/124 emphasis; no external delta and no language-aware token colouring; documents show section position k/n in the strip; help merges aliases of the same action; canSearch now evaluates the screen help was opened from.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
The review TUI now fills the terminal: header, tab strip on a rule with an active underline, padded body, and a pinned footer with the next step or status and '? help' (shortcuts moved to a grouped, aligned ? reference). Diff gets trusted per-file bars (path, kinds, oracle, counts, k/n; sticky), muted Git metadata, aligned gutter rows that map 1:1 to patch lines, green/red tints and changed-word emphasis on 256-colour terminals; changed-path Diff sections share the styling. Lists, Changes preview, Activity and help use the extended closed theme with NO_COLOR fallbacks. Vim/Emacs navigation and Ctrl-W/Ctrl-H editing were added. Verified with go test/race suites (only the pre-existing fish-shim failure), regenerated goldens, PTY tests, tmux inspection and a regenerated review.gif.
<!-- SECTION:FINAL_SUMMARY:END -->
