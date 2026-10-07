# Captured evidence review loop

`after review` captures and opens a change, then resumes that review on the next launch. It does not build code, import a report, run project code or contact Docker on open. Pin inspection and decisions use `after pin PIN`.
The responsive frame, contextual key map, review session, Activity view, and
AFTER-26 grouped Overview and AFTER-27 evidence Cards are implemented; later
redesign tasks are specified in [TUI-DESIGN.md](TUI-DESIGN.md).

```sh
after review                         # capture HEAD vs working tree; open or resume
 after review --staged               # capture HEAD vs index
 after review --base main            # capture merge base with main vs HEAD
 after review --include-untracked notes.txt
 after review --new                  # start and save a fresh review
 after review <candidate-id>         # open a stored snapshot's newest capture
 after review <base-id> <candidate-id> [<evidence-id> ...]
```

Opening an explicit snapshot, record ID or pair does not read or change the saved review. An explicit pin revision stays on exactly that immutable revision. By default, review discovers pin heads (forks appear independently), the pair's newest runs and comparisons, and reports bound to the candidate. At most 32 records load; pins needing another look come first, and Overview reports the number omitted. `after log` provides a separate bounded history view.

## Frame, keys and evidence

The frame header shows the sanitized project directory name, short base and
candidate snapshot IDs, and source words from the stored snapshot records:
`commit <short hash>`, `working tree`, `staged`, or `merge base <short hash>`.
A ready capture is shown as `new capture <id> · u`; active capture, import and run
work show elapsed time, updated once per second, and no percentage. The Overview,
Changes (with inventory count), Diff and Activity tabs exist after records load. The active tab is reverse video
in color mode and bracketed in `NO_COLOR` mode. A detail screen replaces tabs
with a breadcrumb and the selected row's typed badge.

Number keys `1`–`4` and Tab/Shift+Tab switch between Overview, Changes, Diff and
Activity. In a detail or preview, Tab/Shift+Tab moves between document sections. In Diff,
`]`/`[` move through indexed files and `}`/`{` move through indexed hunks. At fewer
than 12 rows the tab bar hides (number keys still work); below 7 rows the status
line hides and hints shrink to `? help · q quit`. Below 60 columns the project
and source words are omitted, tab labels shorten, and only high-priority hints
are shown. The viewport remains capped at 240×100.

One contextual key map drives dispatch, footer hints, and grouped `?` help.
Hints show only enabled actions and are clipped at the terminal width; help also
shows disabled actions with their reason. Color is optional: `NO_COLOR` (any
non-empty value) or `TERM=dumb` disables styling. The actions are:

| Key                                                | Action                                                                                      |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| Up/down or `j`/`k`, PgUp/PgDn, Home/End or `g`/`G` | Select a row or scroll the focused document                                                 |
| `1`–`4`, Tab/Shift+Tab                             | Switch top-level views; cycle detail sections within a document                             |
| Enter                                              | Inspect the selected record or inventory entry; expand/collapse Overview groups             |
| `d` / `2`                                          | Open Changes: the complete unclassified inventory                                           |
| `3`                                                | Open the captured raw Diff                                                                  |
| `]` / `[`                                          | Next / previous indexed file in Diff                                                        |
| `}` / `{`                                          | Next / previous indexed hunk in Diff                                                        |
| `b`                                                | Toggle exact-byte hex view; NUL in the first 8,000 bytes opens hex automatically            |
| Left/right or `h`/`l`                              | Pan through the first 4 KiB of each line; `[b]` marks longer lines                          |
| Esc                                                | Return from a detail, preview, or overlay; return to Overview from another top-level view   |
| `?`                                                | Open grouped help with contextual disabled reasons                                          |
| `c`                                                | Capture again with this review's saved flags (or HEAD versus working tree for explicit IDs) |
| `i`                                                | Import the file configured at launch, if any                                                |
| `u`                                                | Use a pending capture with the original review base; selection is explicit                  |
| `p`                                                | Pin the selected measured provider-request count as a finite expectation                    |
| `r`                                                | Prepare the exact frozen offline execution plan without executing                           |
| `y` / `n`                                          | In the preview only: approve this plan once / deny without execution                        |
| `s` / `4`                                          | Open Activity and inspect the saved review, pair and loaded evidence                        |
| `x`                                                | Request cancellation of an active owned job                                                 |
| `q` / Ctrl-C                                       | During a run, `q` asks for `y` confirmation (`n` continues); Ctrl-C cancels and quits now   |

`s` opens Activity and records the session references without adding evidence
rows. Activity records capture/import/run lifecycle, plan decisions, selection
changes and failures; `Enter` opens full IDs and the full sanitized error. The
log keeps at most 64 events and states how many older events were dropped. A
successful `i` import is bound to the candidate selected at keypress and its
reported cards load immediately when fewer than 32 evidence IDs are already
loaded. At that limit the immutable report remains stored and Activity retains
its ID. Repeated `s` presses with unchanged session references grow neither
Activity nor the evidence list.

`a` no longer accepts a capture. Pin acceptance is not yet available in this TUI;
help points to `after pin PIN --accept` for the explicit CLI decision.

## Changes and Diff

Changes lists every path in the rawdiff inventory, including excluded and
unsupported paths. It groups potential-oracle paths first, then other known
changes, then unknown paths; order within each group follows the sorted inventory.
Rows use `A`, `D`, `M`, or `?`, and show `oracle`, `binary`, mode transitions,
recorded limitations verbatim, and added/deleted line counts from this pair's
captured patch. At 110 columns the selected path has a preview pane. Enter opens
four sections: Diff, Base source, Candidate source, and Inventory record. A side
that has no captured bytes states whether it is absent because the path was added
or deleted, or unavailable because capture did not retain it.

Diff displays every captured patch line after a safe old/new line-number gutter;
the stored patch bytes remain unchanged and are available in exact-byte view.
Its sticky header names the current file, its position, typed flags, and the
`captured patch` origin. Binary, mode-only, added, and deleted files receive
trusted summary dividers while their raw patch lines remain visible. Added lines
are green, deleted lines red, and `@@` lines cyan when color is enabled; `+` and
`-` remain visible and meaningful with `NO_COLOR`. Captured file and hunk jumps use
the bounded `rawdiff` index; computed-diff navigation uses its own display offsets
and does not create hunk IDs or change captured counts.

For pairs without a shared captured patch, AFTER-32 computes a deterministic,
bounded unified presentation from both stored source manifests. The sticky header,
Overview CHANGES line and Changes counts label it exactly
`computed from captured sources — not Git's patch`. Unknown and over-bound paths
keep explicit limitations, and both captured sources remain available. This is a
display fallback, not a Git patch. At a 1×1 terminal, `q` still quits even though
the frame cannot show a useful hint.

Each evidence row has a bracketed badge derived only from typed engine fields,
a plain-text name and summary, and a trailing kind/freshness column at widths
of 80 or more. Selection is marked by `>` and bold; color is optional.
Narrow rows clip explicitly; Enter retains the complete record and scope.

Each evidence row has a bracketed badge derived only from typed engine fields,
a plain-text name and summary, and a trailing kind/freshness column at widths
of 80 or more. Selection is marked by `>` and bold; color is optional.
`NO_COLOR` (any non-empty value) or `TERM=dumb` disables styling.
Narrow rows clip explicitly; Enter retains the complete record and scope.

The first matching badge wins:

| Row                          | Precedence                                                                                                    |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Observation                  | UNAVAILABLE → STALE → UNKNOWN → FAILED → CANCELLED → INCOMPLETE → UNSTABLE → DIFFERENT → EQUAL → NOT COMPARED |
| Report                       | UNAVAILABLE → STALE → REPORTED                                                                                |
| Pin                          | UNAVAILABLE → REOPENED → STALE → UNKNOWN → ACCEPTED → PINNED                                                  |
| Inventory or absent evidence | NOT CHECKED                                                                                                   |

UNAVAILABLE means a missing, corrupt or unsupported stored ID. INCOMPLETE
includes incomparable comparisons. EQUAL requires current, complete evidence;
a receipt opened without a comparison is NOT COMPARED. Payment cases use their
own paired and repetition witnesses: the 12h case may be DIFFERENT while the
30s control is EQUAL. A differing repetition makes only its case UNSTABLE.
Incomplete/incomparable comparisons never become equal case rows.

## Overview triage and Next line

Overview groups evidence in the order **NEEDS ANOTHER LOOK**, **PINNED EXPECTATIONS**,
**AGREES**, **REPORTED**, **EARLIER SNAPSHOTS**, and **OTHER**. Empty groups are
omitted and each header counts its evidence rows. Enter collapses or expands a
group; Earlier Snapshots starts collapsed. Reopened pins and stale/unknown pins
come before unavailable IDs and current failed, cancelled, incomplete, unstable,
or different observations. Current equal observations are under Agrees. Reports
stay under Reported, and observations bound to another pair remain under Earlier
Snapshots with `ran on <candidate-id>` so their values cannot look current.

Each report has a line before its cards containing the short report ID, caller-
selected snapshot binding (or `unbound`), import time, and pass/fail/skip counts.
The caller-supplied producer string is clipped as data at the end of the line;
identical test names from separate reports therefore remain distinguishable.
Reported passes use the reported badge/style, never the observed-equal style.

The Overview `CHANGES` line counts rawdiff paths and captured hunks (all
unclassified), and potential-oracle paths appear directly below it. For computed
source diffs, captured hunk counts remain separate from computed added/deleted
lines. With evidence loaded, `2 shows all` leads to the full Changes inventory.
With no evidence, Overview says `NOT CHECKED` and lists every inventory path,
including excluded and unsupported entries; Enter opens a listed path's captured
detail without implying that anything ran.

At 110 columns or more, Overview asynchronously previews the selected row's
Card beside the list; a preview result is discarded if the selection changes
before it arrives. Narrower terminals keep the one-pane list, and Enter opens
that same Card in the full-screen detail. Card parts keep their trusted typed
titles in ordered divider rows, while bounded sanitized wrapping preserves the
complete prose. Receipt and comparison details retain an explicit Receipt Card
alongside the payment-case Card; full receipt, scenario, witness, sample,
artifact, pin-history and ID sections remain reachable. An unexpected stored
shape is shown as full raw content with a limitation, never guessed into a
conclusion. Reports and unavailable IDs use the same Card-first section format.

The Next line uses only typed record state: `Loading stored records — nothing
runs on open`; `Couldn't load this pair — check the IDs with after inspect`;
`Nothing has run. y runs this exact plan once · n denies`; the active-run line;
a pending-capture notice; reopened-pin and current-result guidance; and, when no
evidence is loaded, `Not checked — read the change, or c captures again after
editing`. Otherwise it is blank. A transient action result replaces the line only
until the next keypress. A current pin's available command is
`after pin PIN --accept`; report names, output, and producer prose never become
badges, group labels, or Next-line text.

Rows read, for example, `12h same-key retry · provider requests 1 → 2 · responses same`.
When repetitions disagree, every count is listed (`1 → 2,1`).
Reports show the package with `(package)` or the test name, with
`reported · pass/fail/skip` in the trailing column. These are reported outcomes,
not AFTER observations: applicability remains unknown unless the binding is to
another selected snapshot, which is stale. Receipts for another pair are stale,
never rebound. Pin decisions remain separate from evidence freshness and execution.
A job completion is not an observation or acceptance.

Receipt sections expose producer, both snapshot bindings, timestamps, execution
and environment metadata, frozen scenario/input, exact comparison witnesses,
limits, every repetition's before/after observation and diagnostics. Channel
names such as `base/43200/0/observation` identify side, delay and repetition.
Observation JSON contains exact responses and independently recorded provider
calls. Report sections show caller provenance and unavailable inputs/effects;
they do not reconstruct observations from test names. No screen asserts more
than the recorded finite inputs and supported channels.

Documents render one captured line per row behind a trusted numbered gutter;
payload newlines cannot create rows or chrome. Tabs expand at four-column stops,
and controls, bidi formats, invalid UTF-8, and leading combining marks are made
safe by the shared terminal renderer. Long lines show a `[b]` marker after the
first 4 KiB; horizontal pan never splits a grapheme.

A NUL in the first 8,000 stored bytes opens exact hex view. Press `b` to switch
between text and hex for any document. Hex offsets make CR, BOM, trailing
whitespace, and every stored byte inspectable. Text documents index at most
250,000 lines; an explicit limitation directs the reviewer to hex, which still
reaches the full blob. Stored bytes never change. Only typed AFTER observation,
sample, comparison-detail, and execution-plan JSON is indented; captured source,
diagnostics, and imported test output remain verbatim. Source sections read
immutable captured blobs, never live paths. Use [headless export](CLI.md) for
machine-readable/raw artifact pages.

## Background actions and limits

To enable `i`, supply `--import-file /path/to/report.jsonl --producer 'caller Go version'`.
The file is opened only on that key action, through the same bounded regular-file
reader as the CLI, bound to the launch candidate (not a subsequently selected one).
Import completion adds a result row with its persisted ID. Capture completion
notifies without replacing the selected pair or cursor; press `u` to use it.
Actions are unavailable until the initial immutable records finish loading.
One capture and one run can progress together; one short pin/selection/preview
operation runs at a time. They share a lazy, process-owned store writer whose
individual publications are serialized. A run never blocks the UI event loop.
Active/completed/cancellation status is visible; there is no invented percentage.
Whole-document reads and indexing run off the UI loop. Late document/request
results cannot replace a newer view, and discarding a UI completion never deletes
a stored receipt.

## Pin, edit, reopen, rerun

Select a complete current payment observation and press `p`. The pin preserves
that case's measured candidate provider-request count, with explicit finite scope;
it is not acceptance of the whole change. Twelve-hour and thirty-second rows show
all recorded repetition counts. Imported, incomplete, historical or unstable
candidate counts cannot become this finite-count pin.

Edit the checkout externally, then press `c`. Only `u` uses the captured
candidate, retaining the original base. Pin revisions reopen conservatively on whole-project basis changes;
the inspector gives the exact reason and **missing current evidence** state.
Prior observations, including the control, remain inspectable under their original
snapshot IDs. Their values are history, not predictions for the new candidate.
The original base stays selected even if HEAD has moved. When the selected pair
has no shared captured patch, the computed display is based on those stored
sources, not a patch from another pair or a new observation.

Press `r` to open a modal consent screen. Its **Summary** is strictly decoded from
the same retained preview bytes whose digest approval uses. It shows the snapshot
pair; sides × cases × repetitions and concurrency; build/app argv and environment;
and every distinct nested image, topology, mount, Docker policy, and limit value.
Values are JSON-escaped data copied from the decoded preview; only counts are
derived. Unknown nested fields or malformed previews show **Exact plan** with
“summary unavailable”; the preview bytes and digest are unchanged. Tab/Shift+Tab
switch between Summary and Exact plan. Exact plan uses the content viewer and
`b` opens its exact-byte hex view.

Consent is modal: only `y`, `n`, Esc, Tab/Shift+Tab, scrolling, `b`, `?`, `q`, and
Ctrl-C act; search and other review actions are unavailable. `y` approves the
retained bytes once, `n` and Esc deny, and quitting denies. Paste never authorizes.
The selected pair and existing idle/action and invalidation checks still gate
approval: a new preview or accepted snapshot invalidates old consent. Execution
reconstructs the exact preview before checking its digest. Docker settings must be
supplied explicitly as for the [headless run command](CLI.md); configuration never
grants consent, and there is no host fallback. `x` cancels an active run and retains
its incomplete receipt. Success adds the real comparison without accepting behavior.
Late results stay at their originating pair and cannot replace the selected result.

Pins are immutable revisions, shared with the [headless review API](REVIEW.md).
Use `s` for full references. `.after/session.json` stores only the selected pair,
original-base mode, and safe capture flags; it is atomically replaced with mode
0600 after a pending capture is selected and again on quit. Invalid state is
reported and replaced at the next save. It is UI state, never evidence. A resumed
review opens its saved pair first and captures in the background; a differing
candidate waits as pending until `u`, without moving the selected pair. `c` uses
the saved capture options. Different capture flags or `--new` start a new review.

Terminal rendering and the saved-review message use stderr. Stdout stays empty
unless `--json` prints the versioned session JSON after terminal restoration.
Without a terminal, bare `after review` exits 2 and points to `after status --json`; explicit stored pairs can be inspected with `after inspect BASE CANDIDATE --json`. There is no automatic file watcher. Keep the private state and evidence references private.

The viewport is capped at 240 columns and 100 rows; extremely narrow terminals
clip explicitly. Storage/capture/report bounds still apply. Artifact reads verify
the whole bounded blob before indexing a document. An unavailable artifact
produces a limitation, never an empty successful result. The browser is not an authenticated
producer verifier: content-addressed records bind bytes, not producer honesty.

## Verification

- `mise exec -- task test:views`: deterministic consent Summary and Changes/Diff
  goldens at 120×40 and 80×24, alongside the existing Overview views through 40×12.
- `mise exec -- task test:terminal`: real Git captures, full inventory, immutable
  source, exact-byte hex round trips, badge precedence, case-specific outcomes, safe theme SGR, stale documents, Changes/Diff navigation and summaries, responsive cancellation and persisted denied-run receipts, plus terminal foundation tests. The 100,250-line captured patch is measured against the terminal per-event budgets.
- `mise exec -- task test:terminal` also drives the browser through real PTYs at
  80×24 and 120×40 with color and `NO_COLOR`, checking Changes, Diff, terminal
  restoration, and safe styling.
- `mise exec -- task test:cli`: actual CLI capture/import followed by PTY browsing,
  inspector/diff/help, background capture/import, 32×8 resize and termios,
  alternate-screen and cursor restoration. Consent also runs in real PTYs at 120×40
  and 80×24, with and without `NO_COLOR`, checking Summary, Exact plan, quit-denies,
  styling, and terminal restoration. No payload terminal controls escape.
- `mise exec -- task tui:proof` with explicit `AFTER_DOCKER_BINARY` and
  `AFTER_DOCKER_HOST`: real offline payment execution and PTY-driven inspect,
  raw diff, one-request pin, restart, retention edit, capture acceptance, reopening,
  denial, authorized one-versus-two witness and unchanged control, cancellation,
  and reopened history after restart. No fabricated observations or accounts.
- `mise exec -- task cli:proof` with the separately provisioned Docker settings:
  real paired synthetic payment execution, then native CLI PTY browsing of its
  observed receipt and all eight observations. Twelve-hour provider calls are
  one versus two with identical responses; thirty-second control is one versus
  one. This target explicitly authorizes only the logged synthetic test plans.
