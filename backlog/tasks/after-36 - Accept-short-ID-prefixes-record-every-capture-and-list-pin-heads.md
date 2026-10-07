---
id: AFTER-36
title: 'Accept short ID prefixes, record every capture, and list pin heads'
status: Done
assignee: []
created_date: '2026-10-06 21:14'
updated_date: '2026-10-07 05:55'
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
- [x] #1 Every ID argument accepts a unique prefix of at least 4 hex characters, with or without `sha256:`, in any case, considering only valid record kinds; full IDs keep working, and fewer than 4 characters is an error that says so.
- [x] #2 No match exits 2 naming the kinds searched and `after log`; an ambiguous prefix exits 2 listing up to 10 matches as short ID, kind and a sanitized one-line description, with tests for ties across kinds and hostile descriptions.
- [x] #3 Each successful capture writes one immutable capture record; recapturing an unchanged tree yields the same snapshot IDs and a new capture record; failed captures write none; the record is in docs/SCHEMA.md with a validation test.
- [x] #4 Pin heads are the revisions no other revision extends, computed on each call; a fork returns each head, and readable output for an explicit older revision opens exactly that revision and names its newer heads.
- [x] #5 `--approve` rejects prefixes with an error that shows the full-digest form; docs/CLI.md documents prefixes and capture records.
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
1. Reuse delivered capture records and verify their existing tests. Add bounded store ID listing and one kind-scoped CLI prefix resolver, keeping authorization digests exact. 2. Resolve every current CLI ID position and flag, with safe bounded ambiguity descriptions and actionable no-match errors. 3. Compute pin heads from immutable history prefixes on every call; retain exact requested revisions and show descendant heads only in readable output. 4. Add focused resolver/history/command tests and extend actual 80-column PTY and pipe checks with both NO_COLOR modes; update CLI and review docs. 5. Run relevant Task targets and one independent acceptance pass focused on kind scoping, unsafe descriptions, exact consent and forks; fix scoped defects, commit, integrate, finalize and clean only the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Operator approved moving the immutable capture-record slice (AC3) into AFTER-35 to satisfy snapshot inspection capture times. AFTER-35 is implementing records in capture.Capture with unchanged snapshot IDs; this task must reuse that implementation rather than duplicate it. Prefix resolution and pin heads remain here. Recheck delivered evidence before checking AC3.

Capture-record AC3 delivered on main in 71251bc through operator-approved AFTER-35 scope transfer. capture.Capture writes immutable evidence.Capture records; recapture/failure/validation tests and schema docs added. Independent focused verification confirmed new events with stable snapshot IDs and unchanged existing JSON. Parent fixed reference validation during bounded history lookup and added missing/mismatched reference regression tests; full task check:go and test passed after correction. Remaining scope: ID prefixes, ambiguity handling, pin heads and approval-prefix rules; do not recreate capture records.

Implementation complete in owned after-36-ids worktree (base 71e0163). Added bounded kind-scoped prefix resolution, safe ten-row ambiguity errors, exact full-digest approval guidance, immutable-history pin heads and readable older-revision notices without JSON shape changes. Reused capture records delivered by AFTER-35. Focused test:cli and review/store race tests passed; task check:go and task test passed (full race suite, build, vet, Go format, 141 doc links and 45-task integrity). Existing tests expecting old no-match diagnostics were updated to the required exit-2/kind/log guidance; read-only absent-store checks still prove no files are created. LSP clean for resolver, head computation and listing. Independent verifier running as the sole general review pass; initial workflow invocation failed before launch due to passing script text as a path, captured partial diff and retried through supported workflow-block protocol (run 054c4f6b-d044-4a27-8da0-59bd857f485e). No protocol fallback.

Terminal evidence passed: mise exec -- go test ./internal/cli -run "^(TestCLIProcessHelper|TestProjectCommandProcessModes|TestBrowserPTY)$" -count=1 -v. The 80-column PTY/OS-pipe matrix uses uppercase SHA256 prefixes for import binding, inspect (snapshot/pair/receipt/comparison/report/artifact), compare, export, run preview, pin and review (including --select and --receipt), with NO_COLOR unset/set. Excerpts: Snapshot d229648f · merge base; Pin 7683e1a2 · [PINNED] · unknown; after: invalid input: --approve requires the full digest: --approve sha256:<64 lowercase hex characters>; after: no snapshot or receipt or comparison or report or artifact matches 0000... — after log lists recent records. TUI PTY used prefix candidate/base/evidence IDs and passed browse/import/capture/resize/terminal-restoration checks. Test log retained temporarily at /tmp/after-36-terminal-checks.log. No Docker execution is introduced or authorized by prefix resolution; runner approval still uses exact saved preview bytes. Documented limitation: after log itself remains AFTER-37; head scan is bounded to 512 revisions/16 MiB and reports unavailable without hiding the requested revision.

Delivered dc9a5f7 (Resolve short IDs and derive pin heads), fast-forwarded to main while preserving the primary claim. Independent fresh-context verifier da15e495-5f33-4934-8d48-7186e2b30dd2 passed AC1–AC5 with no defects: mise exec -- task test:cli; mise exec -- go test -race ./internal/review ./internal/store ./internal/capture ./internal/evidence; mise exec -- task check:go. The child manual artifact write hit FileNotFoundError; harness-retained output and transcript contain the completed verdict, command evidence and per-AC assessment, recovered by parent without another general review. Parent additionally passed mise exec -- task format:check, git diff --check and mise exec -- task check:staged (format/secrets) before implementation commit. Session creation receipt, clean merged branch, completed child, exact checkout-linked workspace and shell-only pane were verified. Worktrunk removed the owned worktree and branch; its post-remove hook removed the associated workspace, verified absent. Older unrelated worktrees were not adopted or altered. No remaining blocker or implementation step; no push. Final task state is committed separately from primary.

Final integrated-main gate passed: mise exec -- task check (formatting, full race suite, doc links/backlog integrity, native build, vet, Go formatting and Gitleaks history scan). Final metadata staged check also required before its separate commit.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented short IDs, safe kind-scoped ambiguity/no-match errors and computed pin heads in dc9a5f7 on main. Exact full-digest consent and existing JSON remain unchanged; capture records reused and reverified. Full Go checks, real 80-column PTY/pipe coverage and one independent verification pass succeeded. Owned worktree/branch/workspace removed; claim released; no push.
<!-- SECTION:FINAL_SUMMARY:END -->
