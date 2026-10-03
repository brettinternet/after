---
id: AFTER-5
title: Expose the complete raw diff and changed-oracle inventory
status: To Do
assignee: []
created_date: '2026-10-03 05:40'
labels:
  - poc
  - git
  - review
milestone: m-0
dependencies:
  - AFTER-3
  - AFTER-4
documentation:
  - docs/IMPLEMENTATION.md
  - docs/behavior-and-evidence.md
priority: high
type: feature
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
An example-first interface must not hide anything it cannot explain. Scope: bounded raw diff/source-context access and inventory bookkeeping shared by CLI and TUI. Highlight fixture/test/expected-output edits without treating them as a newly approved oracle.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every changed path and available text hunk in each capture mode is accessible through the ordinary diff, including unclassified changes; unsupported/binary/oversized changes remain visible by path and count.
- [ ] #2 Unique hunk IDs make mapped/folded/unclassified totals reconcile without double-counting hunks referenced by multiple examples; zero mapped hunks is a valid first-use state.
- [ ] #3 Test, fixture, mask, golden and comparison-policy changes can be inspected beside production changes and labeled as potential oracle changes; user-selected frozen scenarios are not silently replaced.
- [ ] #4 Diff/context access uses captured content rather than the live working tree and disables external helpers; path traversal, weird filenames, missing context and large patches have regression tests.
- [ ] #5 Streaming/viewport-friendly access is bounded and preserves a usable raw path when import or richer evidence fails; partial rendering clearly exposes its limit.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->
