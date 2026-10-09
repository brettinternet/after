---
id: AFTER-60
title: Show HTTP witness changes and refresh demo GIFs
status: Done
assignee:
  - pi
created_date: '2026-10-09 21:14'
updated_date: '2026-10-09 22:39'
labels: []
dependencies: []
ordinal: 60000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Operator requested readable hello → HELLO evidence in Example 09 instead of jq, and fresh GIF recordings including the corrected oracle alignment.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Readable HTTP comparisons show bounded before/after witness values; Example 09 needs no JSON witness workaround.
- [x] #2 task demo:gifs succeeds and refreshed GIFs are committed.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Implementation is integrated into main and focused CLI regression tests pass.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Extend the existing explicit HTTP comparison renderer with capped per-repetition JSON path and before/after values, retaining full JSON as the overflow escape. 2. Remove Example 09 witness jq workaround and verify the real Docker example plus focused renderer regressions. 3. Run task demo:gifs, inspect the oracle frame, run staged checks, commit and integrate.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verified: task test:cli passed, including terminal safety and 24-detail overflow regression; reran successfully after rebasing onto updated main. Example 09 executed in a PTY against the provisioned Docker sandbox: run and compare each printed hello → HELLO for both repetitions. task demo:gifs regenerated payment, oracle and review GIFs; visually checked oracle indentation and payment observation. task docs:check passed in Chromium; task check:staged passed. Integrated implementation into main.

CI repair: restored the missing completion gate. Integration is present in main; task test:cli passed again during CI repair.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added bounded per-repetition HTTP witness paths and before/after values, removed Example 09 JSON workaround, and regenerated all demo GIFs. Verified with CLI tests, actual Docker example, Chromium presentation check and staged checks.
<!-- SECTION:FINAL_SUMMARY:END -->
