---
id: AFTER-36
title: 'Accept short ID prefixes, record every capture, and list pin heads'
status: To Do
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 05:38'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies:
  - AFTER-34
documentation:
  - docs/CLI-DESIGN.md
  - docs/CLI.md
  - docs/SCHEMA.md
  - docs/STORAGE.md
  - docs/REVIEW.md
priority: high
type: feature
ordinal: 36000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
IDs are 71 characters, prefixes are rejected with `invalid stable ID`, and nothing lists captures or runs, so reviewers copy IDs out of earlier JSON. The defaults in AFTER-37 and the plain `after review` in AFTER-24 need the newest capture, but a capture leaves no record of when or how it ran: recapturing an unchanged tree returns the same snapshots and writes nothing new. Defaults that read pins need each pin's newest revisions.

Scope, per docs/CLI-DESIGN.md "IDs and defaults": prefix resolution for every ID argument (at least 4 hex characters, optional `sha256:`, any case, only the record kinds valid for that argument) with the design's no-match and ambiguity errors. An immutable capture record (capture time, mode, base, candidate, index snapshot and selected untracked paths), written by `capture.Capture` itself so every capture path records it and documented in docs/SCHEMA.md. Pin head computation from the immutable records, with no stored pointer. Snapshot IDs do not change, and `--approve` still requires the full digest.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every ID argument accepts a unique prefix of at least 4 hex characters, with or without `sha256:`, in any case, considering only valid record kinds; full IDs keep working, and fewer than 4 characters is an error that says so.
- [ ] #2 No match exits 2 naming the kinds searched and `after log`; an ambiguous prefix exits 2 listing up to 10 matches as short ID, kind and a sanitized one-line description, with tests for ties across kinds and hostile descriptions.
- [x] #3 Each successful capture writes one immutable capture record; recapturing an unchanged tree yields the same snapshot IDs and a new capture record; failed captures write none; the record is in docs/SCHEMA.md with a validation test.
- [ ] #4 Pin heads are the revisions no other revision extends, computed on each call; a fork returns each head, and readable output for an explicit older revision opens exactly that revision and names its newer heads.
- [ ] #5 `--approve` rejects prefixes with an error that shows the full-digest form; docs/CLI.md documents prefixes and capture records.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Focused tests and relevant Task targets pass; record exact commands and outcomes in task notes.
- [ ] #2 Update affected docs/help and record limitations; independent verification for cross-cutting or trust-boundary changes.
- [ ] #3 Run each changed command in a real terminal at 80 columns and in a pipe, with and without NO_COLOR; record output excerpts in task notes.
- [ ] #4 Commit implementation and final task state using the repository delivery workflow; no credentials or private receipts committed.
<!-- DOD:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator approved moving the immutable capture-record slice (AC3) into AFTER-35 to satisfy snapshot inspection capture times. AFTER-35 is implementing records in capture.Capture with unchanged snapshot IDs; this task must reuse that implementation rather than duplicate it. Prefix resolution and pin heads remain here. Recheck delivered evidence before checking AC3.

Capture-record AC3 delivered on main in 71251bc through operator-approved AFTER-35 scope transfer. capture.Capture writes immutable evidence.Capture records; recapture/failure/validation tests and schema docs added. Independent focused verification confirmed new events with stable snapshot IDs and unchanged existing JSON. Parent fixed reference validation during bounded history lookup and added missing/mismatched reference regression tests; full task check:go and test passed after correction. Remaining scope: ID prefixes, ambiguity handling, pin heads and approval-prefix rules; do not recreate capture records.
<!-- SECTION:NOTES:END -->
