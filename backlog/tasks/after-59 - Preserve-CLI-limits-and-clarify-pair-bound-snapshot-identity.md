---
id: AFTER-59
title: Preserve CLI limits and clarify pair-bound snapshot identity
status: Done
assignee:
  - '@pi'
created_date: '2026-10-09 20:54'
updated_date: '2026-10-09 21:12'
labels: []
dependencies: []
ordinal: 59000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Terminal clipping hides evidence qualifications. Example 06 makes unchanged base commits look changed because snapshot records include the paired diff. Operator chose identity clarification, not a schema migration.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Terminal Limit rows retain their full sanitized text at narrow widths; pipes remain unwrapped.
- [x] #2 Readable capture exposes the base Git commit and example 06 explains why base snapshot IDs can change.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
Print sanitized Limit rows without clipping, letting the terminal wrap naturally and preserving pipe output. Add base commit to capture summaries without changing stored records. Explain pair-bound identity in capture docs and example 06. Extend renderer regression coverage and run Go/staged checks.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verified task test:cli, task examples, task check:go and task test. Narrow-terminal renderer regression retains more than 4 KiB of Unicode and hostile limit text safely. Example smoke output shows the full base commit. Rebased onto concurrent main changes and reran all Go checks; Docker example 06 not rerun (narration-only change).
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Limits now wrap naturally instead of clipping. Capture shows the base commit; docs and example 06 explain pair-bound snapshot IDs. Verified Go race tests/build/vet/staticcheck, examples, links, backlog and staged checks; integrated into main.
<!-- SECTION:FINAL_SUMMARY:END -->
