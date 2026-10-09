---
id: AFTER-52
title: 'Show command witness values in readable run, compare and card output'
status: Done
assignee: []
created_date: '2026-10-09 14:42'
updated_date: '2026-10-09 15:54'
labels:
  - follow-up
  - extensibility
dependencies:
  - AFTER-51
documentation:
  - docs/COMPARISON.md
  - docs/RUNNER.md
priority: medium
type: enhancement
ordinal: 52000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
For command scenarios, readable `after run`, `after compare` and the `after inspect` / TUI card show only per-channel EQUAL/DIFFERENT and "N stored witnesses". The actual difference (for example exit 2 -> 0, a removed `/error` key, changed stderr bytes) is only reachable through `--json`, which is how examples/cli/07-command-scenario.sh has to show it. Payment runs already summarize provider counts readably. Reviewers need to see what changed without jq, while untrusted program output stays safe: text witnesses are stored as base64 bytes so terminal controls never reach the terminal raw.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Readable run/compare output and the command comparison card show, per differing case and channel, the exit status before/after and bounded JSON-path changes, with repetition instability called out
- [x] #2 Differing text stream witnesses render as sanitized, bounded text (or an explicit binary/invalid-UTF-8 marker) and never emit raw terminal control bytes
- [x] #3 Equal cases stay one line; output for many cases/changes is bounded with a pointer to the full JSON or card
- [x] #4 Existing CLI/TUI goldens cover a changed exit status, a JSON stdout change and a hostile text stream
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Focused checks (and opt-in Docker proofs where affected) pass; record exact commands and outcomes in task notes
- [x] #2 Update affected docs/help and record limitations; independent verification for trust-boundary changes
- [x] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private artifacts committed
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse terminal sanitization and add shared bounded command witness presentation for readable CLI and receipt/comparison cards; retain exact stored JSON and payment/HTTP behavior.
2. Cover changed exit status, JSON paths, hostile text, instability and display limits through existing CLI/browser golden and regression helpers; document limits.
3. Run focused Task checks and independent trust-boundary verification, then commit implementation, fast-forward main, finalize authoritative task state and remove the verified session-owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in after-52-witnesses: shared bounded command witness renderer for readable CLI and cards, hostile/binary text handling, JSON-path/exit values, repetition instability, equal control lines, full-details pointers and goldens/docs. Executor reports task test:cli and task test:terminal passed. Initial 30-minute child timeout interrupted task check:go; resumed same child and full task check:go passed build, race tests, vet, U1000 and gofmt. Parent LSP diagnostics clean for CLI renderer, browser cards and commandview formatter. No runner/execution changes, so Docker proofs not rerun. Independent trust-boundary verification now running; integration pending.

Independent verifier PASS; no item-scoped defects. Executed go test ./internal/commandview ./internal/cli -run "TestReadableOutputGoldens|TestReadableCommandWitnessOverflowPointsToJSON|TestDeclaredJSONBase64PathIsNotDecodedAsText|TestTextWitnessSanitizesControlsAndMarksBinaryBytes|TestEqualCaseIsOneLineAndManyChangesAreBounded"; go test ./internal/browser -run "^TestGoldenViews$"; git diff --check: all passed. Payment/HTTP goldens unchanged. Implementation c700da5 committed after task check:staged passed formatting and secret scan, then fast-forward integrated into main. Summary bound is 24 lines at 76 columns; exact stored detail remains available. No blocker or next implementation step; task complete pending metadata commit and owned-worktree cleanup.

Post-integration task test passed on main: all Go race tests, 149 documentation links, and 57-task backlog validation. First parent invocation hit its 120-second tool deadline; rerun with adequate deadline passed. Verified session creation receipt against Worktrunk listing; all delegated work inactive and no panes/workspace referenced the owned checkout. Worktrunk foreground removal deleted after-52-witnesses worktree and branch; post-remove hook succeeded and workspace absence verified. Pre-existing unrelated worktrees retained. Claim released; no remaining blocker or resumable work for this item.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Delivered readable command witness values in run/compare and inspect/TUI cards, including exit changes, JSON paths, safe text/binary markers and repetition instability. Equal cases remain compact; bounded summaries link full evidence. Integrated c700da5 into main. CLI/terminal suites, full Go gate, independent trust-boundary verification and staged formatting/secret checks passed. Docker not rerun because execution is unchanged.
<!-- SECTION:FINAL_SUMMARY:END -->
