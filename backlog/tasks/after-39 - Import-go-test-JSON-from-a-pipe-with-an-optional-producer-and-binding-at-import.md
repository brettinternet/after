---
id: AFTER-39
title: >-
  Import go test JSON from a pipe, with an optional producer and binding at
  import
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
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
- [ ] #1 `go test -json ./... | after import`, `after import FILE` and `after import -` import identical reports for identical bytes; with no file and a terminal on stdin, import exits 2 showing the pipe and file forms and reads nothing.
- [ ] #2 Without `--producer`, the report stores no producer claim and readable output shows `not stated` with the `--producer` hint; docs/GO-REPORTS.md and docs/SCHEMA.md document the optional field, and existing reports with a producer still validate.
- [ ] #3 Without `--snapshot`, import captures the working tree under the `after capture` policy, writes a capture record, binds the report to that candidate, and names it; when untracked files were excluded, output says the tests may have used them.
- [ ] #4 A failed binding capture fails the import with the capture package's reason, suggests `--snapshot`, and stores no report.
- [ ] #5 Readable output states that the binding and producer are caller claims and that AFTER did not run or observe the tests; hostile producer and test-name tests cover readable output.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
