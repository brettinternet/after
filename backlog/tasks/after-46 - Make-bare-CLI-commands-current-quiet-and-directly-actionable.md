---
id: AFTER-46
title: 'Make bare CLI commands current, quiet, and directly actionable'
status: Done
assignee:
  - '@pi'
created_date: '2026-10-08 19:51'
updated_date: '2026-10-08 20:57'
labels:
  - poc
  - cli
  - ux
milestone: m-1
dependencies: []
priority: high
type: enhancement
ordinal: 46000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A bare-command UX audit (every command with no arguments in a fresh checkout with one modified and one untracked file) found gaps that undercut "UX above all": `after diff`/`status`/`inspect` silently show a stale stored capture after edits; the first suggestion is `after capture` rather than the main `after review` flow; `after run` discards its real preparation error; missing-run diagnostics and several Next/stderr lines print full 71-character IDs where bare commands or short IDs work; empty stdin imports store a 0/0/0 report and exit 0; readable output repeats each Limit row per snapshot, shows an Index row for working-tree captures, duplicates the inspect Captured row, and counts excluded untracked paths as changed; `capture` says "resume the saved review" with none saved; `diff --stat` reprints the capture summary instead of per-file counts; `config` misaligns long keys and offers Docker values only as copy-paste. Item 1 (staleness) resolves per an oracle consultation recorded in notes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 After editing a captured working tree, bare `after diff`, `after status` and bare `after inspect` no longer present the old capture as current without saying so (behavior per recorded oracle decision), and no project code runs
- [x] #2 With no capture, `status`, `log` and `pin` suggest `after review` first; `pin` with a capture but no pins does not suggest `after capture`
- [x] #3 `after run` preparation failures report the specific reason (for example, the checkout is not the supported payment fixture) with a corrective action
- [x] #4 Missing-run diagnostics suggest bare `after run` for the newest capture; readable Next commands and `diff` stderr use short IDs where a unique prefix resolves, while JSON keeps full IDs
- [x] #5 `after import` with empty input stores nothing and exits 2 with a corrective action
- [x] #6 Readable capture/status/inspect output prints each limit once, omits the Index row for working-tree captures, shows Captured once, and distinguishes changed from excluded paths
- [x] #7 `after capture` only says "resume the saved review" when one is saved
- [x] #8 `after diff --stat` prints per-file added/removed line counts and a total, like `git diff --stat`
- [x] #9 `after config` aligns all keys, and Docker setup offers a runnable way to apply detected values
- [x] #10 docs/CLI.md (and CLI-DESIGN.md where it changes) describe the new behavior; focused tests cover the critical behaviors
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Oracle decision for AC #1 (staleness), consulted 2026-10-08: choose live, non-persisting `after diff` (HEAD vs working tree by default; reuse --staged/--base/--target/--include-untracked; `--stored` for the newest stored capture; explicit BASE CANDIDATE unchanged; never inherits prior capture/review flags; never falls back to a stored patch on live-read failure: exit 1, empty stdout, suggest `--stored`). `status`/bare `after`/bare `inspect` stay stored summaries but add a JSON `freshness` object (matches/changed/unknown/not_checked + scope/reason) and a readable freshness line; `--stored` skips the check. Merge-base captures are not_checked (immutable_comparison); incomplete captures are unknown. Read commands create no .after/, snapshots, capture events or session updates. Prerequisite: capture speed (measured 41s/34s for a 344-file repo, spawn-dominated): split the consistent scan from persistence and batch `git cat-file --batch-check/--batch` and patch-construction `hash-object --stdin-paths` within the hardened runner and 16 MiB output ceiling.

Progress 2026-10-08: implemented on branch after-46-cli-ux (.worktrees/after-46-cli-ux), commit f815772. Capture prerequisite: blobs now read via one cat-file --batch-check plus output-bounded cat-file --batch chunks (budget charged before payload reads); makeDiff hashes distinct payloads with one hash-object --stdin-paths. Measured bare after diff --stat on the 344-file AFTER repo: 1.34s (was 41s/34s capture). Added capture.ReadLive (live, unstored diff) and capture.Unchanged (content-hash freshness, no timestamps). Checks: go vet ./...; mise exec -- task test (go test -race ./..., docs and backlog checks) passes; task format:check passes; task check:staged passes. Regression caught and fixed before commit: buffering the stored patch exceeded TestDiffLargeCapturedPatchStreamsWithinMemoryBudget (81 MB > 64 MiB), so stored patches stream through bounded pages again. Manual bare-command walkthrough in a temp repo confirmed each AC. An independent reviewer pass on capture batching/freshness is in progress.

Review 2026-10-08: independent reviewer found one defect: rawdiff.View.index returned at MaxHunks, so diff --stat dropped later files and attributed their lines to the last indexed file (reproduced: 100001-hunk a.txt plus z.txt gave 1 file, a.txt 200004; git gives 2 files, 200002 and 2). Fixed in e21a4b5: hunk bookkeeping stops at the cap but file sections stay indexed; live diff now reports completeness/index limits (omitting routine capture caveats). Extended TestBoundsLargePatchAndPartialIndex. Repro now matches git diff --stat and prints the hunk-cap limit. mise exec -- task test (go test -race ./..., docs, backlog) and task check:staged pass. Fast-forwarded main to e21a4b5.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Bare after diff now prints the checkout's current change without storing it (capture flags supported; --stored and BASE CANDIDATE print stored patches; a failed read prints nothing and suggests --stored). status, bare after and bare inspect label the stored capture and report checkout freshness by content hash (JSON freshness: matches/changed/unknown/not_checked; --stored skips; merge-base is not_checked; unreadable checkout exits 1). Capture now batches cat-file and hash-object, cutting a 344-file diff from ~41s to ~1.3s. Also: review-first suggestions, specific after run preparation errors, short IDs in readable suggestions/diagnostics, empty import exits 2 storing nothing, deduplicated limits and accurate changed/excluded counts, git-style diff --stat, aligned config with a runnable Docker export line. Docs: CLI.md, CLI-DESIGN.md, RAW-DIFF.md, CAPTURE.md. Commits f815772, e21a4b5 on main (not pushed).
<!-- SECTION:FINAL_SUMMARY:END -->
