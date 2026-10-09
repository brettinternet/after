# Review TUI design

This document records the layout, state vocabulary and safety rules for AFTER's terminal review UI. The interface is implemented; [TUI.md](TUI.md) is the user guide. Screen values below are illustrative, not captured output.

**Status:** AFTER-21–33 behavior is implemented. The task table at the end records the design sequence, not current backlog status.

## Intent and boundaries

The TUI turns a snapshot pair and its immutable evidence into a reviewable list. It keeps the complete raw inventory and exact bytes available when richer records are missing or limited. A changed test, fixture or golden file is a potential-oracle flag, not proof that behavior is preserved. The TUI never derives evidence from repository text or makes a human decision for the reviewer.

- Meaning must survive `NO_COLOR` and monochrome terminals.
- Trust comes from fixed position and style, not payload. Only AFTER draws badges, headings, tabs and key hints from typed values.
- Readable output does not strengthen evidence; UI labels map tested engine fields.
- Captured source, raw diff, unknown paths, full IDs and artifacts remain reachable.
- Every action hint reflects the current screen and state.

| Concern                                                | Package             |
| ------------------------------------------------------ | ------------------- |
| Safe text, clipping, wrapping, styling, document index | `internal/terminal` |
| Case-specific comparison outcome                       | `internal/compare`  |
| Captured and computed diffs                            | `internal/rawdiff`  |
| Browser rows, badges, cards, sections and layout       | `internal/browser`  |
| Flags, saved review and readable commands              | `internal/cli`      |

`internal/terminal` is the only rendering and viewport boundary. Do not add Bubbles or lipgloss.

## Frame

```text
   AFTER   payment   original base a750186b (commit 3f2a1c9)  →  candidate 784eb013 (working tree)
  [Overview]   Changes 1   Diff   Activity
  ━━━━━━━━━━──────────────────────────────────────────────────────────────────────────

    ▾ NEEDS ANOTHER LOOK  1
  > [DIFFERENT]   12h same-key retry · provider requests 1 → 2 · responses same

    CHANGES  1 path · 1 captured hunk · 0 potential oracles

  ────────────────────────────────────────────────────────────────────────────────────
  Enter opens the full record                                                   ? help
```

- The frame fills the terminal: header, tabs (or breadcrumb and section tabs) over a rule, a blank spacer, the padded body, then a footer rule and status line pinned to the last rows. The header uses the sanitized project directory, short IDs and source values from the snapshots: `commit <hash>`, `working tree`, `staged` or `merge base <hash>`. Follow-up mode names the left side `last inspected`.
- The footer shows the status or Next line, or else a gentle idle hint for the selected row (`Enter opens this path`, `] and [ jump between files`), with `? help` on the right. Shortcuts are not listed in the frame; the footer names editing keys only while a text field has focus.
- A pending capture shows `new capture <id> · u`; active capture/import/run shows elapsed time, updated once per second, never a percentage.
- Under 12 rows, hide tabs and the rules (number keys still work). Under 7, hide the status line and show `? help · q quit`. The viewport is capped at 240×100.
- At 110 columns, Overview and Changes split into a 45% list and preview; narrower screens use one pane and Enter opens details. Under 60 columns, drop the side padding and project/source words; under 40, shorten tabs.

## Badges and formatting

Badges occupy a fixed 14-cell column. The first applicable rule wins; all are derived from typed evidence fields, never names, summaries, report output or expectation text.

| Priority | Badge                 | Applies to            | Condition                                  | Style     |
| -------: | --------------------- | --------------------- | ------------------------------------------ | --------- |
|        1 | `UNAVAILABLE`         | Any                   | ID missing, corrupt or unsupported         | Problem   |
|        2 | `REOPENED`            | Pin                   | Decision is reopened                       | Attention |
|        3 | `STALE`               | Any                   | Applicability is stale                     | Attention |
|        4 | `UNKNOWN`             | Pin, observation      | Applicability is unknown                   | Attention |
|        5 | `FAILED`, `CANCELLED` | Observation           | Execution failed or cancelled              | Problem   |
|        6 | `INCOMPLETE`          | Observation           | Incomplete evidence or incomparable result | Problem   |
|        7 | `UNSTABLE`            | Observation           | Case comparison is unstable                | Attention |
|        8 | `DIFFERENT`           | Observation           | Case comparison differs                    | Changed   |
|        9 | `EQUAL`               | Observation           | Complete current case compares equal       | Observed  |
|       10 | `NOT COMPARED`        | Observation           | No comparison record                       | Muted     |
|       11 | `REPORTED`            | Report                | Imported report not stale/unavailable      | Reported  |
|       12 | `ACCEPTED`, `PINNED`  | Pin                   | Current accepted or pinned decision        | Decision  |
|       13 | `NOT CHECKED`         | Inventory/no evidence | No evidence for the item                   | Muted     |

Payment outcomes use each case's witnesses, not the receipt aggregate: a differing repetition makes that case `UNSTABLE`; otherwise a differing paired witness makes it `DIFFERENT`; otherwise a complete comparable case is `EQUAL`. Incomplete or incomparable cases cannot be equal. The trailing column (shown at widths of 80 or more) identifies evidence and freshness: `observed · current`, `ran on <id>`, `reported · fail`, `no current result`, `current result attached` or `accepted <time>`.

Color is on unless `NO_COLOR` is non-empty or `TERM=dumb`. `internal/terminal` uses fixed SGR sequences; 256-color terminals (`TERM` containing `256color`, or `COLORTERM=truecolor`/`24bit`) also get fixed backgrounds for Diff tints and the selection bar. Chrome styles (brand, rules, accent, Diff tints) never carry evidence meaning. Green evidence styling is reserved for current complete equality; reported passes and human acceptance are not green, and green/red line counts and Diff tints describe text, not results. Selection is an accent bar with a background in color and `>` without it; badges retain brackets in every mode. Group headings are bold with a muted count; secondary text is faint.

### Formatting

- IDs: 8 hex characters in rows/chrome; full `sha256:` IDs in details, Activity and session JSON.
- Durations use largest exact units: `43200s` → `12h`, `90s` → `1m30s`, `30s`.
- Times: local `15:04:05` today, otherwise `Jan 2 15:04`. Durations: `110ms`, `32s`, `1m05s`; record sections keep RFC 3339.
- Counts show `1 → 2` when repetitions agree, or each value (`1,1 → 2,1`).
- Long paths truncate in the middle and retain the file name.

## Overview

Groups appear in order; empty groups disappear and each heading counts evidence rows. Enter toggles a group. Earlier Snapshots starts collapsed.

| Group               | Rows                                                                                                                           |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| NEEDS ANOTHER LOOK  | Reopened pins; stale/unknown pins; unavailable IDs; current failed, cancelled, incomplete, unstable or different observations. |
| PINNED EXPECTATIONS | Current pinned and accepted pins.                                                                                              |
| AGREES              | Current equal observations.                                                                                                    |
| REPORTED            | Imported reports, visibly separate from AFTER observations.                                                                    |
| EARLIER SNAPSHOTS   | Observations for another pair, with `ran on <candidate-id>`.                                                                   |
| OTHER               | Not-compared observations.                                                                                                     |

Within NEEDS ANOTHER LOOK, reopened pins come first, then stale/unknown pins, unavailable IDs, and current problem results. Keep engine order within other groups (12h before 30s; other evidence in selection order). Reported passes never use the observed-equal style.

Each report has a provenance line before its cards: short report ID, caller-chosen binding or `unbound`, import time, pass/fail/skip counts and clipped producer string. This distinguishes reports with identical test names. The producer is data, not an authenticated claim.

The `CHANGES` line counts inventory paths and captured hunks separately; binary and mode-only paths may have no hunks. Captured hunks are unclassified. Potential-oracle paths appear below. With evidence loaded, `2 shows all` opens the full inventory. With no evidence, Overview says `NOT CHECKED` and lists all paths—including excluded and unsupported entries—without implying anything ran. For computed diffs, display computed line counts separately from captured hunk counts. Report cards show `<package> (package)` or a test name and `reported · pass/fail/skip`; reports provide no test inputs or side effects.

### Next line

The line derives only from typed state; a transient action result replaces it until the next key. Reports and caller text never populate it.

| State                             | Text                                                                           |
| --------------------------------- | ------------------------------------------------------------------------------ |
| Loading                           | `Loading stored records — nothing runs on open`                                |
| Pair load failed                  | `Couldn't load this pair — check the IDs with after inspect`                   |
| Consent                           | `Nothing has run. y runs this exact plan once · n denies`                      |
| Run active                        | `Running the approved plan · 1:12 · x cancels (the incomplete result is kept)` |
| Capture pending                   | `New capture 9c01d2e4 — u reviews it`                                          |
| Pin reopened, no current result   | `Pin reopened: no result for this candidate yet — r previews a rerun`          |
| Pin has a current complete result | `Current complete result attached — a accepts this pin`                        |
| No evidence                       | `Not checked — read the change, or c captures again after editing`             |
| Otherwise                         | blank                                                                          |

Keep transient status messages short; Activity holds the full sanitized error.

At 110 columns, Overview previews the selected row's Card; discard a preview if selection changed before it arrived. Enter opens that Card full-screen on narrow screens. Typed card titles are trusted dividers, separate from wrapped data.

## Detail

Enter opens a sectioned detail with Card first. The same structure is used for records, report cards and inventory paths.

| Row                       | Sections                                                                      |
| ------------------------- | ----------------------------------------------------------------------------- |
| Payment case              | Card · Receipt Card · Witnesses · Artifacts · Receipt · Scenario · Plan · IDs |
| Receipt without case rows | Card · Artifacts · Receipt · Scenario · Plan · IDs                            |
| Pin                       | Card · History · Current result · Basis receipt · IDs                         |
| Report                    | Card · Output · Report · IDs                                                  |
| Changed path              | Diff · Base source · Candidate source · Inventory record                      |
| Unavailable ID            | Card with limitation · IDs                                                    |

Cards are templates over strictly decoded records. An unknown shape stays raw with a limitation; it is never guessed into a conclusion. Each section is ordered trusted titles plus content rows, so payload cannot forge headings. Long prose uses bounded safe wrapping. For a payment case, Witnesses includes its exact witnesses, comparison record and `comparison-rules` artifact. Artifacts lists matching channels by side/repetition/channel:

```text
^(base|candidate)/(43200|30)/[0-9]+/(observation|sample|candidate-diagnostics|observer-diagnostics)$
```

`execution-plan` appears under Plan; other channels remain reachable under Other artifacts. Every artifact referenced by a receipt, comparison, scenario or pin history stays reachable.

## Changes

```text
POTENTIAL ORACLES 1
 > M testdata/health.golden.json  +1 −1  oracle
CHANGED 2
   M assets/logo.png             binary
   D handlers/old.go             −3
UNKNOWN 1
   ? .env.local                  excluded: untracked; not selected
```

This is an illustrative layout. The inventory groups potential-oracle paths, other known changes and unknown paths; each group follows sorted inventory order. `A`, `D`, `M`, `?` mean added, deleted, modified and unknown. Flags show `oracle`, `binary`, mode transitions and recorded limitations verbatim. Line counts come from the captured or explicitly labeled computed diff. A rename appears as deletion plus addition; rename detection is out of scope. Enter opens Diff, Base source, Candidate source and Inventory record. Missing source bytes say whether the path was added/deleted or capture did not retain them.

## Diff

Diff shows every captured patch line after a safe, aligned old/new line-number gutter; display rows map one-to-one to patch lines plus one trusted bar per file. The bar names the path, its change kinds (binary, mode-only, added, deleted, modified or computed-diff limits), potential-oracle flag, `+`/`−` counts and `k/n` position, and is repeated as a sticky header while scrolling within the file. The tab strip shows the origin and size. `]`/`[` navigate indexed files; `}`/`{` navigate indexed hunks. Opening Diff from a Changes path selects that file. Git metadata lines are faint; `@@` lines are cyan. Added and removed lines get green and red tints. Removed lines are paired positionally with the added lines that follow them in the same run, and the differing middle after the shared prefix and suffix (widened to whole words and graphemes) is emphasized; lines sharing too little are tinted without emphasis, and lines over 4 KiB are not compared. Captured patch bytes are unchanged, and a changed path's Diff section uses the same line styles.

Highlighting is built in. An external pager such as `delta` would reintroduce foreign escape sequences and process execution into the safe-rendering boundary, so AFTER does not invoke one. Language-aware token coloring is not implemented.

### Computed diffs (AFTER-32)

A raw patch is shared only by its captured pair. For other snapshot pairs, `internal/rawdiff` computes a deterministic unified presentation from both immutable source manifests. It uses inventory order, Git-style headers and three context lines. The sticky header, Overview and Changes label it `computed from captured sources — not Git's patch`.

| Bound                            |                                   Limit |
| -------------------------------- | --------------------------------------: |
| Path length                      |                                   4 KiB |
| Base + candidate source per path |                                   1 MiB |
| Combined lines per path          |                                  20,000 |
| Source bytes across the pair     |                                   8 MiB |
| Myers work                       |                         1,000,000 steps |
| Rendered output                  |                                  16 MiB |
| Binary classification            | NUL in first 8,000 bytes of either side |

Over-bound paths show `too large to diff here — open both sources`; unknown paths keep their recorded limitation and have no computed hunk. Mode-only changes show the mode transition. Computation runs during background load. Display offsets are for navigation only; computed hunks do not get IDs or change captured `Hunks`/`Count`. Tests apply computed patches back to the base and check candidate bytes and deterministic output.

## Content viewer

One viewer serves details, source, Diff, plan and Activity documents.

- Renderer-created rows use a trusted numbered gutter. Payload newlines cannot create chrome; tabs expand at four-column stops.
- Terminal controls and bidi formats become visible escapes, invalid UTF-8 becomes U+FFFD, and leading combining marks get a dotted circle. See [TERMINAL.md](TERMINAL.md).
- Documents are bounded to 16 MiB and 250,000 indexed text lines. At the line limit, show a limitation; hex still reaches the complete stored blob.
- NUL in the first 8,000 bytes opens hex. `b` toggles text/hex for every document; offset rows expose exact bytes, including CR, BOM and trailing whitespace. Stored bytes never change.
- Only typed observation, sample, comparison-detail and execution-plan JSON is indented. Sources, diagnostics and report output remain verbatim.
- Pan reaches the first 4 KiB of each line and marks a longer line with `[b]`; it does not split graphemes. Search is a smart-case substring over sanitized displayed text, reports `match k of n` or `no matches`, highlights hits (reverse video or `⟦…⟧`), and pans to off-screen document matches. Queries are capped at 512 sanitized bytes; consent is not searchable. Soft wrap is limited to bounded Card prose.
- Whole-document reads and indexing run off the event loop. Hex reads the full bounded blob; artifact reads verify the bounded blob before indexing and show a limitation on failure.

## Consent

```text
Run this exact plan?   Summary   Exact plan 18.2 KiB
Nothing has run. y runs this exact plan once; n or Esc denies.
  Snapshots     base a750186b → candidate 784eb013
  Runs          2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1
  Build         /usr/local/go/bin/go build -trimpath -o /work/app ./app
  Image         docker.io/library/golang@sha256:e0174e51…
  Topology      app network=none; observer joins app network only
  Mounts        no host mounts, volumes, sockets or published ports
  Policy        --network=none --read-only --user=65534:65534 --cap-drop=ALL
  Limits        180 seconds and 65536 output bytes per container
```

Illustrative only. Strictly decode the Summary from the exact preview bytes whose digest `y` approves. It shows snapshot pair; sides, cases, repetitions and concurrency; build/app argv and environment; and every distinct preparation, image, container snapshot/input/argv/environment, topology, mount, Docker policy and limit. Copy JSON-escaped values from the plan; derive only counts. Build commands are execution and run only after consent. Reject unknown nested fields for the summary. On malformed or unsupported data, show `Summary unavailable`; Exact plan and the approved bytes remain unchanged.

Consent is modal: only `y`, `n`, Esc, Tab/Shift+Tab, scrolling, `b`, `?`, `q` and Ctrl-C act. `y` approves once; `n`, Esc and quit deny. Search is disabled and paste never authorizes. Existing selected-pair, idle-action and invalidation checks still gate approval; a new preview or snapshot selection invalidates prior consent. Execution reconstructs the preview before checking its digest. Docker configuration never grants consent; there is no host fallback. Cancellation retains an incomplete receipt; completion does not accept behavior.

## Activity and session

Activity records capture/import/run lifecycle, plan decisions, selections and failures. `s` opens Activity and shows session references without adding evidence rows. The event log holds 64 events and reports dropped older entries; Enter shows full IDs and sanitized error details. Work indicators show elapsed time, never percentage. A capture and run may progress together; short pin/selection/preview work is serialized. Late results remain bound to their original pair.

### Launch and resume

```sh
after review                              # capture HEAD vs working tree, then open/resume
after review --staged                    # HEAD vs index
after review --base main                 # merge base with main vs HEAD
after review --include-untracked notes.txt
after review --new                       # fresh capture and replace saved session
after review BASE CANDIDATE [EVIDENCE...]
```

`after review` uses safe capture semantics: no project code runs, and untracked files remain excluded unless explicitly selected. Unchanged captures reuse snapshot IDs. With no saved session, an empty implicit capture exits 0 without opening the TUI. A saved review opens its exact pair and captures in the background; a different candidate waits for `u`. There is no automatic watcher. `c` reuses saved capture flags; for explicit IDs it uses HEAD versus working tree.

`.after/session.json` is private UI state, not evidence: it stores pair, comparison mode, baseline, selected pin revision IDs and capture flags, but no source bytes. Writes are atomic and mode 0600 after a selection change and on quit. A legacy original-base session can infer the baseline from its saved pair. Invalid state is reported and replaced at the next save. `--new` or different capture flags starts a new session; evidence and pins stay stored. Explicit IDs open without reading or changing the saved session. `--json` writes session JSON to stdout only after terminal restoration; rendering and save notices use stderr. Without a terminal, bare `after review` exits 2 and points to `after status --json`; inspect explicit stored pairs with `after inspect BASE CANDIDATE --json`.

Discovery loads pin heads (forks separately), newest pair runs/comparisons and reports bound to the candidate. At most 32 evidence IDs load; pins needing another look come first. Overview reports omissions; `after log` lists bounded history. An explicit pin revision opens exactly that revision.

## Mutation prompts and key map

`p`, `u` and `a` always confirm exact IDs and effect; navigation, selection and paste never mutate state. Prompts prefill a reason. The reviewer may edit or clear it with Ctrl-U; sanitized, bracketed-pasted text is data, capped at 4,096 bytes, recorded in pin history, and cannot be empty. `p` refuses an existing pin with the same basis receipt and expectation. `u` counts changed paths, warns that pins may reopen and history remains, and offers original-base (default) or last-inspected mode. `a` is available only for a pin with a current complete result on the selected pair; the engine enforces this too. See [REVIEW.md](REVIEW.md) for immutable pin revisions.

| Key                                                                         | Context                      | Action                                                                        |
| --------------------------------------------------------------------------- | ---------------------------- | ----------------------------------------------------------------------------- |
| `↑`/`↓`, `j`/`k`, Ctrl-N/P                                                  | Lists/documents              | Move one row.                                                                 |
| PgUp/PgDn, Ctrl-D/U, Ctrl-F/B, Ctrl-V/Alt-v, Home/End, `g`/`G`, Alt-`<`/`>` | Lists/documents              | Page, first/last.                                                             |
| Enter (`l` in lists), Esc (Ctrl-G; `h` in Changes/Activity)                 | Lists/overlays               | Open or expand; return or cancel (deny in consent).                           |
| `1`–`4`, Tab/Shift+Tab                                                      | Views/details                | Switch top-level view or detail section.                                      |
| `←`/`→`, `h`/`l`, `b`                                                       | Documents                    | Pan; toggle hex.                                                              |
| `/`, `n`/`N`                                                                | Lists/documents, not consent | Search, next/previous match.                                                  |
| `]`/`[`, `}`/`{`                                                            | Diff                         | Next/previous file and hunk.                                                  |
| `d`, `c`, `i`, `u`, `p`, `a`, `r`                                           | Available review state       | Changes, capture, import, select candidate, pin, accept pin, prepare preview. |
| `y`, `n`                                                                    | Consent only                 | Approve once; deny.                                                           |
| `s`, `x`, `?`, `q`, Ctrl-C                                                  | Session/jobs/any view        | Activity; cancel; help; quit. During a run `q` confirms, Ctrl-C quits now.    |

The single key map drives dispatch and grouped help. `?` lists every binding by group in an aligned key column, merges aliases of the same action, and dims unavailable ones with their reason. Emacs aliases in consent only move or leave. While search or a reason field is focused, printable keys and paste are text; only Enter, Esc, Backspace/Ctrl-H, Ctrl-W, Ctrl-U and Ctrl-C act.

## Safe rendering and verification

1. Sanitize untrusted paths, names, outputs, producer strings, reasons, errors and bytes before layout; apply styling only after sanitizing and clipping.
2. Measure, clip and wrap only in `internal/terminal` using grapheme widths. The renderer alone creates rows; content cannot forge badges, dividers, tabs, prompts, hints or group headers.
3. Badges, groups and Next lines come only from tested typed-field mappings. A report is reported evidence, never an observation. A changed snapshot does not create a result; completion is not acceptance.
4. With `NO_COLOR`, emit no SGR. In color mode, allow only theme SGR. Consent must summarize the approved bytes; mutations require confirmation; paste is data.
5. Keep the inventory, every artifact, full IDs and exact bytes reachable.

`internal/browser/testdata/views/` holds deterministic 120×40, 80×24 and 40×12 golden views from synthetic stores; no Docker is needed. The test-only `-update` flag regenerates them. Tests inject timestamps/time zones, check theme sequences and hostile-content placement, and exercise PTY restoration and search at 80×24/120×40 with and without `NO_COLOR`. `task test:terminal` includes the 100,000-line search/input budget; `task test:views`, `task test:terminal` and `task test:cli` are local checks. `task tui:proof`, `task cli:proof` and `task test:poc` are Docker-gated synthetic execution proofs (the POC gate includes the TUI PTY proof); run them only with explicit plan authorization. Keep docs/TUI.md, docs/DEMO.md, examples/README.md and the `examples/tui` hints in step with screen changes. [AFTER-18's human study](EVALUATION.md) freezes the commit and binary checksum; changing the TUI after a freeze requires a refreeze and rehearsal.

Not in this design: mouse input, configurable themes, side-by-side diffs, language-aware syntax highlighting, external diff pagers, rename detection, divergence-signature grouping, automatic capture/file watching, clipboard writes, soft-wrapped documents, GitHub or browser UI.

## Delivery order

The original dependency order is retained for traceability; backlog status is authoritative.

| Task     | Scope                                               | Depends on | Priority |
| -------- | --------------------------------------------------- | ---------- | -------- |
| AFTER-21 | Safe content viewer                                 | —          | High     |
| AFTER-22 | Theme, badges, case outcomes, goldens               | 21         | High     |
| AFTER-23 | Frame, key map, hints, help, `u`                    | 22         | High     |
| AFTER-24 | Plain `after review`, capture, resume, discovery    | 23, 36, 38 | High     |
| AFTER-25 | Activity, session, timers, quit guard, live imports | 23, 24     | Medium   |
| AFTER-26 | Overview triage and Next line                       | 25         | High     |
| AFTER-27 | Cards and wide preview                              | 26, 35     | High     |
| AFTER-28 | Changes and Diff                                    | 23         | High     |
| AFTER-29 | Readable consent                                    | 23         | High     |
| AFTER-30 | Confirmed mutations, reasons, pin acceptance        | 27         | Medium   |
| AFTER-31 | Search                                              | 27, 28     | Medium   |
| AFTER-32 | Computed diffs                                      | 28         | High     |
| AFTER-33 | Original-base/last-inspected comparison             | 30, 32     | Medium   |

AFTER-18 uses this TUI for the study condition and depends on high-priority work. The study is a human activity, not an autonomous acceptance check.
