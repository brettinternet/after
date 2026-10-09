# Captured evidence review loop

`after review` captures a Git comparison and opens the evidence browser; a later launch resumes the saved pair. Opening stored records never builds code, runs project code, imports a report, or contacts Docker. The browser is implemented as described in [TUI-DESIGN.md](TUI-DESIGN.md).

## Open a review

```sh
after review                              # capture HEAD vs working tree; open or resume
after review --staged                    # HEAD vs index
after review --base main                 # merge base with main vs HEAD
after review --include-untracked notes.txt
after review --new                        # replace the saved review with a fresh capture
after review <record-or-snapshot-id>      # open a record or a snapshot's newest capture
after review <base-id> <candidate-id> [<evidence-id> ...]
after review --import-file report.jsonl --producer 'go version go1.24'
```

Capture defaults to HEAD versus the working tree. Untracked files stay excluded unless selected by exact, repeatable `--include-untracked PATH` flags; ignored files are not selected. `--base REF` accepts a Git ref and `--target REF` defaults to `HEAD`. These modes follow the safe Git capture policy in [CAPTURE.md](CAPTURE.md). `--new` captures afresh, names the replaced pair on stderr, and replaces only the saved UI session—not stored snapshots or evidence. Explicit IDs open stored records without capturing or changing that session.

A capture in a throwaway checkout produced this real output (IDs abbreviated):

```text
$ after capture
Captured candidate 6c4d6f9f (working tree) against base 72c96d9b (commit)
  Base         72c96d9b · 1 path · complete · 0 excluded · 0 unsupported
  Candidate    6c4d6f9f · 1 path · complete · 0 excluded · 0 unsupported
  Limit        two matching reads; not an atomic filesystem snapshot
  Limit        diff includes captured regular files only; inspect excluded and unsupported inventory
Next
  after review
    open a review of this change
  after diff --stored
    print this captured patch
```

The browser discovers pin heads (including forks), newest runs and comparisons for the selected pair, and reports bound to its candidate. It loads at most 32 evidence IDs; pins needing another look come first and Overview reports omissions. Use `after log` for bounded history or `after pin PIN` for headless pin inspection and decisions. See [CLI.md](CLI.md#review) for the full launch and exit behavior. Without a terminal, bare `after review` exits 2 and points to `after status --json`; inspect a stored pair with `after inspect BASE CANDIDATE --json`.

## Layout and keys

The browser fills the terminal. The header shows the project, comparison mode, short snapshot IDs and source words from each snapshot (`commit <hash>`, `working tree`, `staged`, or `merge base <hash>`). Below it, tabs (Overview, Changes with the inventory count, Diff and Activity) sit on a rule that underlines the active tab. A detail replaces tabs with a breadcrumb and badge, then its section tabs with the section position and size on the right. The body is padded by two columns, and a footer rule and status line stay on the last rows. The status line names the next step (for example `Enter opens this path`) or the latest result; shortcuts live in `?` help instead of an always-on hint row. The viewport is capped at 240×100.

| Terminal size        | Layout                                                            |
| -------------------- | ----------------------------------------------------------------- |
| At least 110 columns | Overview and Changes show a 45% list and a Card or file preview.  |
| Under 60 columns     | Drop side padding and project/source words.                       |
| Under 40 columns     | Shorten tab labels.                                               |
| Under 12 rows        | Hide the tab bar and footer rule; number keys still switch views. |
| Under 7 rows         | Hide the status line; the footer shows `? help · q quit`.         |

Color is on unless `NO_COLOR` is non-empty or `TERM=dumb`; `xterm-256color`, `COLORTERM=truecolor` or `COLORTERM=24bit` also enables fixed 256-color backgrounds for Diff tints and the selection bar. Styling uses a closed set of fixed SGR codes and never carries meaning alone: the active tab is bold over a heavy rule in color and bracketed without it, badges keep their brackets, selection is an accent bar (`>` without color), and search matches use `⟦…⟧` without color. Narrow rows clip, but Enter opens the full record. At 1×1, `q` still quits even when the frame cannot show a useful hint.

| Key                                                   | Action                                                                                                                            |
| ----------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `↑`/`↓`, `j`/`k`, Ctrl-N/Ctrl-P                       | Move or scroll one row.                                                                                                           |
| PgDn/PgUp, Ctrl-D/Ctrl-U, Ctrl-F/Ctrl-B, Ctrl-V/Alt-v | Move or scroll one page.                                                                                                          |
| Home/End, `g`/`G`, Alt-`<`/Alt-`>`                    | Move to the start or end.                                                                                                         |
| `1`–`4`, Tab/Shift+Tab                                | Switch views; cycle detail/preview sections.                                                                                      |
| Enter, `l` in lists                                   | Open a record or inventory entry; expand/collapse an Overview group.                                                              |
| Esc, Ctrl-G, `h` in Changes/Activity                  | Go back.                                                                                                                          |
| `d` / `2`, `3`, `4`                                   | Changes, Diff, Activity.                                                                                                          |
| `]`/`[`, `}`/`{`                                      | Next/previous indexed file or hunk in Diff.                                                                                       |
| `/`, `n`/`N`                                          | Search current list or document; cycle matches.                                                                                   |
| `←`/`→`, `h`/`l` in documents, `b`                    | Pan; toggle exact-byte hex view.                                                                                                  |
| `c`, `i`, `u`, `p`, `r`, `a`                          | Capture, import configured report, use pending capture, pin, prepare run, accept pin. Confirmations apply to `u`, `p`, and `a`.   |
| Consent: `y` / `n`                                    | Run the displayed plan once / deny it.                                                                                            |
| `s`, `x`, `?`, `q`, Ctrl-C                            | Activity, cancel an owned job, help, quit. During a run, `q` asks for `y` confirmation (`n` continues); Ctrl-C cancels and quits. |

`?` lists every key by group, dims unavailable ones and says why. Text fields (search and reasons) accept Backspace/Ctrl-H, Ctrl-W to delete a word and Ctrl-U to clear. Bracketed paste is data and never triggers an action.

## Overview and evidence

Overview groups rows in this order: **NEEDS ANOTHER LOOK**, **PINNED EXPECTATIONS**, **AGREES**, **REPORTED**, **EARLIER SNAPSHOTS**, **OTHER**. Empty groups are hidden; Earlier Snapshots starts collapsed. Reopened pins, stale/unknown pins, unavailable IDs and current failed or incomplete observations lead the first group. Current complete equal observations are under Agrees. Reports remain separate from observations.

Badges come only from typed evidence fields; the first applicable rule wins:

| State                               | Meaning                                                                                               |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `UNAVAILABLE`                       | Stored ID is missing, corrupt or unsupported.                                                         |
| `REOPENED`, `STALE`, `UNKNOWN`      | Pin decision or evidence applicability needs review; stale evidence belongs to another pair or basis. |
| `FAILED`, `CANCELLED`, `INCOMPLETE` | Execution failed/cancelled, or evidence is incomplete/incomparable.                                   |
| `UNSTABLE`, `DIFFERENT`, `EQUAL`    | Case comparison result. `EQUAL` requires a complete, current result.                                  |
| `REPORTED`                          | Imported report, not an AFTER observation; pass/fail/skip appears separately.                         |
| `ACCEPTED`, `PINNED`                | Human pin decision, separate from evidence freshness.                                                 |
| `NOT COMPARED`, `NOT CHECKED`       | No comparison, or no evidence for the item.                                                           |

Payment case states use their own witnesses: the 12-hour case can be `DIFFERENT` while the 30-second control is `EQUAL`. A differing repetition makes that case `UNSTABLE`; incomplete comparisons never become equal. Report cards show `<package> (package)` or the test name and a separate `reported · pass/fail/skip` result. Reports do not reconstruct test inputs or side effects; see [GO-REPORTS.md](GO-REPORTS.md). Report names, output and producer strings are data, not badges or status text. A report's producer is caller-supplied, not authenticated.

With no evidence, Overview says `NOT CHECKED` and lists every inventory path, including excluded and unsupported entries. Its `CHANGES` line counts paths and captured hunks as separate quantities; potential-oracle paths appear below. Computed added/deleted lines never inflate captured-hunk counts.

## Changes and Diff

Changes keeps the full rawdiff inventory: potential-oracle paths first, then other known changes, then unknown paths, each in sorted inventory order. Rows use `A`, `D`, `M`, or `?` and show oracle/binary flags, mode changes, captured line counts and recorded limitations. Enter opens Diff, Base source, Candidate source and Inventory record. Missing source bytes state why; they are not replaced with live files.

Diff shows the captured patch unchanged behind an aligned old/new line-number gutter. Each file starts with a trusted bar naming the path, change kind (binary, mode-only, added, deleted or modified), potential-oracle flag, line counts and file position; the bar stays pinned while scrolling through that file. Git metadata lines (`diff --git`, `index`, `---`/`+++`, mode and binary headers) are muted but remain visible. `]`/`[` and `}`/`{` use the captured file/hunk index. Added and removed lines are tinted green and red; when a removed line is paired with the added line that replaced it, the changed words are emphasized. With 16 colors the tints are foreground colors; without color the `+`/`-` prefixes carry the meaning. The same styling applies to a changed path's Diff section. Highlighting is built in rather than delegated to an external pager such as `delta`, whose output would cross the safe-rendering boundary.

If the snapshots have no shared captured patch, the browser displays a deterministic unified diff from their stored source manifests. It is labeled `computed from captured sources — not Git's patch` in Diff, Overview and Changes. It is only a presentation: it creates no Git patch, hunk IDs or captured counts. Limits are:

| Bound                            |                    Limit |
| -------------------------------- | -----------------------: |
| Path length                      |                    4 KiB |
| Base + candidate source per path |                    1 MiB |
| Combined source lines per path   |                   20,000 |
| Total source bytes per pair      |                    8 MiB |
| Myers comparison steps           |                1,000,000 |
| Rendered computed patch          |                   16 MiB |
| Binary check                     | NUL in first 8,000 bytes |

Over-bound files show `too large to diff here — open both sources`; unknown paths retain their limitation. Computation and indexing run in the background. See [raw diff](RAW-DIFF.md) for captured-patch limits.

## Details, documents and search

Details use typed Cards. Payment cases show Card, Receipt Card, Witnesses, Artifacts, Receipt, Scenario, Plan and IDs. Receipts without case rows omit Witnesses; pins show History, Current result and Basis receipt; reports show Output and Report; changed paths show Diff, both sources and Inventory record. Unexpected stored shapes remain raw content with a limitation, not a guessed conclusion. Full IDs remain available in details and Activity.

Every document uses one trusted row per captured line. Multi-part documents separate parts with a blank line and a `── Part ──` heading. Payload newlines cannot create headings or chrome. Tabs expand to four-column stops; controls, bidi formats, invalid UTF-8 and leading combining marks are rendered safely. Long lines show `[b]` after the first 4 KiB; pan does not split graphemes. A NUL in the first 8,000 bytes opens hex view automatically. `b` toggles text/hex for any document; hex offsets expose every stored byte, including CR, BOM and trailing whitespace. Stored bytes do not change. Only typed observation, sample, comparison-detail and execution-plan JSON is indented; source, diagnostics and imported output stay verbatim.

Documents are limited to 16 MiB and 250,000 indexed text lines. Past the line limit, the text view shows a limitation; hex still reaches the full bounded blob. Artifact reads verify the bounded blob before indexing; an unavailable artifact shows a limitation, not an empty result. Search `/` scans sanitized displayed text on Overview, Changes, Activity, details and Diff; it is unavailable on consent. Queries are limited to 512 sanitized bytes and use case-insensitive matching unless they contain uppercase. Enter jumps to the next match, `n`/`N` wrap, and Esc clears search before leaving a detail. The status shows `match k of n` or `no matches`; hits use reverse video in color and `⟦…⟧` without color, and document navigation pans to off-screen hits. Scans run in cancellable background work; query/view changes cancel them and late results are discarded.

## Explicit actions

- `c` captures in the background using saved flags (or HEAD versus working tree for explicit IDs). A new candidate stays pending until `u`; the selected pair never changes by itself.
- `u` names the candidate, counts paths changed from the candidate under review, and asks for a reason. Choose **original base** (default) or **last inspected**; pins may reopen and earlier results become history. The original baseline remains available.
- `i` is enabled only when `--import-file` was supplied. The file opens only on the keypress through the CLI's bounded regular-file reader and binds to the candidate selected then. Optional `--producer TEXT` is a caller claim, not authenticated provenance. Loaded report Cards appear immediately if fewer than 32 IDs are loaded; otherwise the report stays stored and Activity shows its ID.
- `p` pins a complete current payment case's measured candidate request count as a finite example, not whole-change approval. Confirm the receipt, snapshot IDs and reason. A duplicate basis receipt plus expectation is refused and identifies the existing pin. Imported, incomplete, historical or unstable counts cannot be pinned.
- `a` accepts a pin only when its current complete result matches the selected pair. AFTER records the human decision; it does not accept the whole change.
- `r` prepares an offline execution plan; it does not run it. The modal Summary strictly decodes the same retained preview bytes whose digest is approved. It lists the pair, run counts/concurrency, argv, environment, images, topology, mounts, Docker policy and limits. Values are JSON-escaped plan data; only counts are derived. Unknown fields or malformed data show `Summary unavailable`; Exact plan remains available and its bytes/digest do not change.
- In consent, only `y`, `n`, Esc, Tab/Shift+Tab, scrolling, `b`, `?`, `q` and Ctrl-C act. `y` approves those bytes once; `n`, Esc, quit or Ctrl-C denies. Paste cannot approve. Builds are execution and run only as part of the approved plan. Execution requires explicit `AFTER_DOCKER_BINARY` and `AFTER_DOCKER_HOST`; configuration grants no consent and there is no host fallback. `x` requests cancellation; an incomplete receipt is kept. A successful run adds measured evidence but never accepts behavior.

Reasons for `p`, `u` and `a` are prefilled, editable and required; Ctrl-W deletes a word and Ctrl-U clears them. Sanitized text is capped at 4,096 bytes and recorded in pin history. Confirmation names the exact IDs and effect.

## Activity, session and background work

Activity records capture/import/run lifecycle, plan decisions, selection changes and failures. It holds up to 64 events and reports how many older events were dropped; Enter shows full IDs and the full sanitized error. `s` opens Activity with session references; it does not add evidence rows. Repeating `s` without changing references does not grow the list.

`.after/session.json` stores the active pair, comparison mode, original baseline, selected pin revision IDs and safe capture flags. It is atomically saved with mode 0600 after selection changes and on quit. It contains no source bytes and is UI state, not evidence. A resumed review opens its saved pair immediately and captures in the background; a different candidate waits for `u`. `--new` replaces only this session. Explicit-ID reviews do not read or change it. There is no file watcher.

Initial record loading, capture, import, run preparation, whole-document reads, computed diffs and search run off the UI event loop. A capture and run can progress together; short pin/selection/preview operations are serialized. The store serializes publications. Active work shows elapsed time, not an invented percentage. Results stay bound to their originating pair; discarding a stale screen update never deletes a stored receipt.

## Verification

- `task test:views`: deterministic consent and Changes/Diff views at 120×40 and 80×24, plus Overview down to 40×12.
- `task test:terminal`: capture inventory, immutable sources, hex, badges, outcomes, navigation, search, paste safety, cancellation and large-input budgets.
- `task test:cli`: CLI capture/import, PTY browsing, background actions, resizing and terminal restoration.
- `task tui:proof`, `task cli:proof` and `task test:poc` are Docker-gated synthetic execution proofs. They require explicit plan authorization; they are not ordinary documentation checks.
