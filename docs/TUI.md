# Captured evidence review loop

`after review --tui` opens the stored engine records, not a demo or a live checkout.
It does not run Git, import a report, build code or contact Docker on open.
The existing headless `review <pin-revision-id>` remains unchanged.
This page documents current behavior; the remaining redesign (AFTER-23–33) is
specified in [TUI-DESIGN.md](TUI-DESIGN.md).

```sh
after capture --project /path/to/project
# Use the returned candidate/base snapshot IDs. Evidence IDs are optional.
after review --tui <candidate-id> --base <base-id> \
  --project /path/to/project --evidence <comparison-or-receipt-or-report-or-pin-revision-id>
```

Repeat `--evidence` for up to 32 stored IDs. There is no automatic discovery or
implicit selection of the newest result. Start without evidence to review the
complete captured inventory and patch with an explicit **not checked** screen.
Unknown/corrupt evidence IDs do not hide the raw inventory.

## Keys and evidence

| Key                                         | Action                                                                                                                  |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Up/down or `j`/`k`, PgUp/PgDn, Home/End     | Select a record or scroll the focused document continuously                                                             |
| Enter                                       | Inspect the selected record or inventory entry                                                                          |
| `d`                                         | Open the complete unclassified inventory, including unknown, excluded, unsupported, binary and potential-oracle entries |
| Tab in inventory                            | Open the captured raw patch                                                                                             |
| Tab / Shift+Tab in inspector, patch or plan | Next/previous section                                                                                                   |
| `g` / `G`                                   | Move to the start/end of the complete text or hex document                                                              |
| `b`                                         | Toggle exact-byte hex view for any document; NUL in the first 8,000 bytes opens hex automatically                       |
| Left/right or `h`/`l`                       | Pan text by grapheme-safe columns through the first 4 KiB of each line; `[b]` marks longer lines                        |
| Escape                                      | Return to list, preserving selection                                                                                    |
| `?`                                         | Scrollable help                                                                                                         |
| `c`                                         | Explicitly capture HEAD versus working tree in the background; untracked files stay excluded                            |
| `i`                                         | Explicitly import the file configured at launch, if any                                                                 |
| `p`                                         | Pin the selected measured candidate provider-request count as a finite expectation (list only)                          |
| `a`                                         | Accept the pending captured candidate, retaining the original review base; not behavior acceptance                      |
| `r`                                         | Prepare and display the exact frozen offline execution plan, without executing                                          |
| `y` / `n`                                   | In the preview only: approve this plan once / deny without execution                                                    |
| `s`                                         | Inspect selected snapshot IDs, immutable pin revisions and retained result IDs for restart                              |
| `x`                                         | Request cancellation of the active job                                                                                  |
| `q` / Ctrl-C                                | Quit, cancel and join owned work, restore terminal                                                                      |

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
notifies without replacing the selected pair or cursor; press `a` to accept it.
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

Edit the checkout externally, then press `c`. Only `a` accepts the captured
candidate. Pin revisions reopen conservatively on whole-project basis changes;
the inspector gives the exact reason and **missing current evidence** state.
Prior observations, including the control, remain inspectable under their original
snapshot IDs. Their values are history, not predictions for the new candidate.
The original base stays selected even if HEAD has moved. When the selected pair
has no shared captured patch, the patch page says so; complete stored inventory
and both captured sources remain available, not a patch for the wrong pair.

Press `r`, continuously inspect the complete exact plan (images, inputs, argv,
mounts, offline network and resource limits), then `y` to approve it once or
`n`/Escape to deny. `b` switches to exact hex when needed. Paste cannot authorize
execution. A new preview or accepted snapshot invalidates old consent.
Execution reconstructs the exact preview before checking its digest. Docker settings
must be supplied explicitly as for the [headless run command](CLI.md); configuration
never grants consent, and there is no host fallback. `x` cancels the run and retains
its incomplete receipt. Success adds the real comparison without accepting behavior.
Late results stay at their originating pair and cannot replace the selected result.

Pins are immutable revisions, shared with the [headless review API](REVIEW.md).
Use `s` for full references; on normal quit, session JSON is printed after terminal
restoration. Restart with its pair and `--evidence <latest-pin-revision-id>` (and any
comparison/history IDs desired). There is no implicit newest-pin selection,
automatic refresh or filesystem watcher. Keep these references private.

The viewport is capped at 240 columns and 100 rows; extremely narrow terminals
clip explicitly. Storage/capture/report bounds still apply. Artifact reads verify
the whole bounded blob before indexing a document. An unavailable artifact
produces a limitation, never an empty successful result. The browser is not an authenticated
producer verifier: content-addressed records bind bytes, not producer honesty.

## Verification

- `mise exec -- task test:terminal`: real Git captures, full inventory, immutable
  source, exact-byte hex round trips, badge precedence, case-specific outcomes, safe theme SGR, stale documents, responsive
  cancellation and persisted denied-run receipts, plus terminal foundation tests.
- `mise exec -- task test:views`: six deterministic no-color golden views. Regenerate only with `mise exec -- task test:views -- -update` and review the diff.
- `mise exec -- task test:cli`: actual CLI capture/import followed by PTY browsing,
  inspector/diff/help, background capture/import, 32×8 resize and termios,
  alternate-screen and cursor restoration. No payload terminal controls escape.
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
