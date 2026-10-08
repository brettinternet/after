---
id: AFTER-39
title: >-
  Import go test JSON from a pipe, with an optional producer and binding at
  import
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-08 03:12'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-36
  - AFTER-38
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/GO-REPORTS.md
  - docs/SCHEMA.md
priority: medium
type: enhancement
ordinal: 39000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`after import` refuses a report without `--producer`, cannot read a pipe, and binds the report to nothing unless the caller copies a 71-character `--snapshot` ID. The common flow is `go test -json ./... | after import`, right after the tests ran on the working tree.

Scope, per docs/CLI-DESIGN.md "import": input from `FILE`, `-`, or a piped stdin, and with no file and a terminal on stdin, exit 2 showing both forms. `--producer` becomes optional: an omitted producer stores no claim, and output shows `not stated` with a hint. Binding defaults to a capture of the working tree taken at import, under the `after capture` policy, writing a capture record and saying when untracked files were excluded. A failed capture fails the import with its reason and suggests `--snapshot`. `--snapshot ID` and `--captured-at` keep working. Readable output says the binding and producer are caller claims and that AFTER did not run the tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `go test -json ./... | after import`, `after import FILE` and `after import -` import identical reports for identical bytes; with no file and a terminal on stdin, import exits 2 showing the pipe and file forms and reads nothing.
- [x] #2 Without `--producer`, the report stores no producer claim and readable output shows `not stated` with the `--producer` hint; docs/GO-REPORTS.md and docs/SCHEMA.md document the optional field, and existing reports with a producer still validate.
- [x] #3 Without `--snapshot`, import captures the working tree under the `after capture` policy, writes a capture record, binds the report to that candidate, and names it; when untracked files were excluded, output says the tests may have used them.
- [x] #4 A failed binding capture fails the import with the capture package's reason, suggests `--snapshot`, and stores no report.
- [x] #5 Readable output states that the binding and producer are caller claims and that AFTER did not run or observe the tests; hostile producer and test-name tests cover readable output.
- [x] #6 `after review --import-file FILE` no longer requires `--producer`, and the TUI's `i` import records no producer claim when it is omitted.
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
1. Extend bounded report import to FILE, dash, and piped stdin with no terminal reads for missing input; make producer optional without inventing provenance. 2. Reuse capture.Capture working-tree policy and capture history when no snapshot is supplied; fail before persisting a report if capture fails. 3. Update readable trust warnings, review/TUI import validation, schema/help/docs, and focused regression tests. 4. Independently verify acceptance and real 80-column terminal/pipe behavior with and without NO_COLOR; run relevant Task checks, commit implementation, fast-forward main, finalize task metadata, and remove the verified owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in owned after-39-import worktree; initial executor timed out, changes were inspected and same executor resumed successfully. No commits yet. Executor reports task test:cli, check:go, test, format:check passing; final focused CLI tests, vet and gofmt passed. Real 80-column PTY/pipe import modes with and without NO_COLOR and producer-free TUI import passed. Independent verifier is now checking acceptance and provenance/input failure risks before integration.

Independent verifier passed all six criteria and the single scoped review found no concrete defects. Independently passed: mise exec -- task test:cli; mise exec -- task check:go (build, race tests, vet, format); mise exec -- task test (including 141 local links and backlog integrity); mise exec -- task format:check. Real terminal evidence: mise exec -- go test ./internal/cli -run "^(TestProjectCommandProcessModes|TestBrowserPTY)$" -count=1 -v and NO_COLOR=1 mise exec -- go test ./internal/cli -run "^TestBrowserPTY$" -count=1 -v passed. Import process tests use 80-column PTY and pipe with NO_COLOR unset/set; review rejects piped nonterminal invocation under both settings and live TUI import passes under both. Output excerpts: Producer not stated; add --producer TEXT to record a caller claim; Binding ... caller claim; Capture working tree captured at import; Untracked 1 path excluded; tests may have used them; AFTER did not run or observe these tests. Review PTY verifies persisted metadata and successful import rather than retaining a visible transcript; readable wording is covered separately. Implementation a0e9946 passed task check:staged and was fast-forwarded to main without conflicts. Session-owned creation receipt and clean checkout verified before Worktrunk foreground removal; after-39-import branch/worktree removed and matching Herdr workspace disappearance confirmed. Older unrelated worktrees left untouched. No blocker or resumable implementation remains; no push authorized.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented FILE/dash/piped import, optional producer with no invented claim, working-tree capture binding/history and explicit provenance/untracked warnings. Producer-free review/TUI import works. All six acceptance criteria independently verified with race/build/vet/format checks and actual PTY/pipe tests. Delivered a0e9946 on main; owned worktree cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->
