---
id: AFTER-35
title: Print readable results by default and keep the versioned JSON behind --json
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 05:38'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-22
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/TUI-DESIGN.md
  - docs/TERMINAL.md
  - docs/DEMO.md
  - docs/EVALUATION.md
priority: high
type: enhancement
ordinal: 35000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every command prints one line of versioned JSON: `after capture` prints 1.3 KB on one line, `inspect` returns the patch as base64, and no result says what happened in words (docs/CLI-DESIGN.md "Why"). Reviewers read these results; scripts can ask for JSON.

Scope, per docs/CLI-DESIGN.md "Output", "inspect" and the capture, compare, pin, import and run mockups: readable text on stdout by default, and `--json` prints exactly today's envelope. `export` always prints JSON. Each result is a sentence, then aligned rows or a list. Styling and badges come from the AFTER-22 theme on a terminal only. Untrusted text goes through `internal/terminal` and clips only on a terminal. IDs, times, counts and paths follow the TUI formatting rules. `inspect` follows the design's table; records whose Card arrives in AFTER-27 print their sentence and IDs, and plans arrive with AFTER-40. Next blocks are AFTER-37's. This supersedes the headless one-JSON-object rule, so this task moves every caller that parses output to `--json`: tests, `examples/`, `internal/demo` including the study kit, `internal/pocgate`, Taskfile proofs and docs. On a terminal, a capture or import still running after one second shows elapsed time on stderr.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Every command except `export` prints readable text by default, and `--json` prints today's versioned envelope unchanged; a test enumerates the commands so none is missed.
- [x] #2 Readable output follows the design's shape and formatting: a leading sentence, aligned rows, 8-character IDs, local times and counts; golden files in `internal/cli/testdata/` cover each command with a fixed clock, time zone and 80 columns, regenerated only with a test-only `-update` flag.
- [x] #3 On a terminal, styling uses only theme SGR sequences and badges; NO_COLOR, `TERM=dumb` or a pipe produces no escape sequences, rows clip only on a terminal, and hostile paths, test names, expectations, producers and errors are sanitized in every command's output.
- [x] #4 `after inspect` prints a pair's capture summary rows, a snapshot's source, capture times and path count, an artifact's kind, size and content through the content viewer rules, and a sentence plus an IDs section for receipts, comparisons, pin revisions and reports, with full IDs one per line and never clipped.
- [x] #5 Every caller that parses output passes `--json`; `task test`, `task examples`, `task demo:inspect` and `task study:check` pass, Docker-gated proofs are rerun when Docker settings are available, and docs/CLI.md documents readable output and `--json`.
- [x] #6 On a terminal, a capture or import still running after one second shows elapsed time on stderr with no percentage, and a pipe gets none.
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
1. Preserve the version-1 JSON encoder behind --json (export always JSON); add typed readable result rendering using existing terminal sanitizer/theme/content viewer, with independent stdout/stderr terminal detection and delayed capture/import elapsed notices.
2. Cover every current command/result with deterministic 80-column goldens, hostile-content and clipping/color tests, and real PTY plus pipe checks; retain existing grammar and defer Next blocks/cards/plans to their owning tasks.
3. Migrate all JSON-consuming tests, examples, demo/study and POC callers and update output documentation. Run focused checks plus task test, examples, demo:inspect, study:check and available Docker proofs.
4. Perform one independent acceptance verification focused on unchanged JSON, terminal safety and caller migration; fix concrete findings, commit checked implementation, fast-forward main, finalize task metadata, and remove the receipt-owned worktree/workspace.

5. Operator approved moving the immutable capture-record slice of AFTER-36 forward: persist capture time/mode/pair/index/selected untracked paths from capture.Capture without changing snapshot IDs or existing CLI JSON; show recorded times on snapshot inspection, honestly unavailable for legacy snapshots. Add schema/storage/capture tests and documentation, update AFTER-36 ownership notes, and rerun affected checks. This resolves independent verification failure for AC4.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation is uncommitted in owned after-35-readable worktree. Initial executor timed out after 30 minutes; parent inspected partial diff, resumed the same child, and it completed. Executor reports passing mise exec -- task test:cli, examples, demo:inspect, study:check, check:go, test:terminal and test (140 links, 45 tasks), plus git diff --check. Native PTY excerpts: Captured candidate 5bde4d00; Base 64c880ce · 4 paths · complete; pipe comparison: incomparable · incomplete. Browser and consent PTY restoration passed at 80x24/120x40 with and without NO_COLOR. Parent LSP diagnostics for render.go clean. Independent acceptance verification is running before integration. Docker CLI exists but explicit AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST settings are absent, so conditional Docker proofs are not claimed.

Delivered as 71251bc and fast-forwarded main. Independent verifier passed original AC1/2/3/5/6 and identified missing capture-time storage for AC4. Operator authorized moving AFTER-36 capture records forward. Focused independent verification confirmed AC4 and new event/stable snapshot/unchanged JSON tests, but found history scans bypassed reference validation. Parent fixed scans to use the validated loader and added adversarial canonical-hash fixtures for missing and mismatched snapshot references; both reject corrupt history. Parent then ran task format:go, check:go (full race tests/build/vet/format), test, and check:staged successfully. Post-integration task test passed on main (140 links, 45 tasks). LSP clean for renderer, capture, schema and store changes. Schema Markdown formatting failure in initial staged check was fixed with task format and rechecked. Executor and verifier also passed task test:cli, examples, demo:inspect, study:check; native PTY snapshot excerpt: Snapshot c0f7f663 · merge base; Captured 23:21:34 · merge_base · record 418b1124. Existing command PTY/pipe matrix covers 80 columns and NO_COLOR; deterministic goldens use test-only -update. Capture history is bounded and explicitly reports partial lookup; legacy snapshots honestly lack capture records. No Docker proof claim: explicit Docker settings absent. No remaining item blocker. Verified no active children and only an idle shell in exact receipt-owned workspace; Worktrunk removed after-35-readable worktree and branch and its hook closed the matching Herdr workspace, verified absent. Pre-existing unrelated worktrees retained without adoption. Final task metadata committed separately; no push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Readable CLI results with safe theme/content rendering, unchanged opt-in JSON, migrated callers and delayed progress. Added operator-approved immutable capture events for real inspection times. Full Go/race, examples, demo/study and PTY checks passed; independent findings corrected with regression tests. Integrated 71251bc on main; owned worktree, branch and workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->
