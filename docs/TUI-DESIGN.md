# Review TUI design

**Status: target design. AFTER-21–24, AFTER-28, AFTER-29 and AFTER-32 are implemented as
documented in [TUI.md](TUI.md); AFTER-25–27, AFTER-30–31 and AFTER-33 remain target behavior.**
This is the design for backlog tasks AFTER-21 to AFTER-33 (milestone M2). The
command line, including how `after review` launches, is designed in
[CLI-DESIGN.md](CLI-DESIGN.md). The product
invariants in [AGENTS.md](../AGENTS.md), [IMPLEMENTATION.md](IMPLEMENTATION.md),
and [TERMINAL.md](TERMINAL.md) still apply. Where this document changes an earlier TUI
decision, it says so. Values in mockups are illustrative.

## Why

An audit ran `after review --tui` on the three `examples/tui` scenarios: raw
review, test reports, and the real Docker-backed payment loop. The engine is
honest, but the screen is hard to read and hard to act on.

| Finding                    | Seen today                                                                                                                                                            | Task   |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| Content is unreadable      | Every row is `data \| "…"`: Go-quoted and split every 32 bytes (`"diff --git a/shipping.go b/shipp"`, `"ing.go\n"`); tabs show as `\t`, nested JSON as `\\\"`         | 21     |
| Byte paging                | `[`/`]` move 4 KiB windows (`bytes 0..4096/18602`); one receipt takes five pages                                                                                      | 21     |
| No hierarchy               | One text style. Only the selected row has state: `STATE observed \| current \| completed \| equal \| complete \| report=none`                                         | 22     |
| Misleading per-case state  | After the regression rerun, the 30s control (1 → 1) says `different`, because every case row inherits the receipt-level outcome                                       | 22     |
| No context                 | The header is `AFTER review \| examples`: no project, snapshots, or pending capture                                                                                   | 23     |
| Static footer              | `Enter inspect \| d diff \| ? help \| Esc back \| q quit` on every screen; `p r a y n` exist only in help                                                             | 23     |
| Launch and resume friction | Opening needs two 71-character IDs. Resuming means rebuilding a command with up to 32 `--evidence` flags. The TUI draws on stdout, so its session JSON can't be piped | 24     |
| Pseudo-rows in evidence    | Job completions and `s` append "not checked" rows to the evidence list; pressing `s` twice adds two                                                                   | 25     |
| Imports need a restart     | An `i` import appears as a "background result" row; its report shows only after restarting with `--evidence`                                                          | 25     |
| Rows can't be scanned      | `"43200s: provider requests base=[1] candidate=[2]; finite recorded samples only"`; two reports both read `"example.com/cart / "`                                     | 22, 26 |
| Dead-end empty state       | With no evidence, the landing screen is two lines, and the change itself is hidden behind `d`                                                                         | 26     |
| Raw JSON as the UI         | Enter opens `json.MarshalIndent` dumps of receipts, pins, and inventory entries, then up to 20 sections named like `artifact: base/43200/0/observation`               | 27     |
| No diff viewer             | The patch is quoted text: no color, file jumps, line numbers, or summaries of binary or mode changes                                                                  | 28     |
| Unreadable consent         | Approval shows the 18 KiB exact plan as quoted, chunked JSON instead of a readable summary                                                                            | 29     |
| Unconfirmed mutations      | `p` twice creates two identical pins. `a` switches the reviewed snapshot (reopening pins) without confirmation. Reasons are canned                                    | 30     |
| The loop can't close       | Accepting a pin against its current result works only headless                                                                                                        | 30     |
| No search                  | No `/` in inventories, report lists, 100,000-line patches, or plans                                                                                                   | 31     |
| The diff disappears        | After `a`, the pair spans two captures, so the patch view only says "no shared captured patch". The reviewer loses the diff right after the first edit                | 32     |
| One meaning of "before"    | `a` always keeps the original base. The follow-up comparison (last inspected → latest) from the [proposal](README.md) exists only as headless `--mode last_inspected` | 33     |

## Principles

1. **Lead with the behavior change.** Write `12h same-key retry · provider requests 1 → 2`,
   and state the scope once per card, not on every row.
2. **Words carry meaning; color reinforces it.** Every state reads correctly with
   `NO_COLOR` or on a monochrome terminal.
3. **Trust lives in position and style, never in content.** AFTER alone draws badges,
   headers, tabs, and key hints, from typed engine fields. Repository text stays in
   data columns and content panes.
4. **Everything stays reachable:** the complete inventory (including excluded,
   unsupported, and unknown paths), every artifact, full IDs, and exact bytes.
5. **Always show the next useful action**, and show only keys that work on the
   current screen.
6. **Readable is not more trusted.** Rendering never changes evidence semantics. One
   tested mapping turns engine enums into words.

This follows the contact sheet and code microscope in the [proposal](README.md)
and the terminal experiment in [behavior-and-evidence.md](behavior-and-evidence.md)
§10.

## Code boundaries

| Concern                                                 | Home                                              |
| ------------------------------------------------------- | ------------------------------------------------- |
| Sanitizing, clipping, wrapping, styling, document index | `internal/terminal`                               |
| Per-case comparison outcome                             | `internal/compare` (engine helper, not view code) |
| Computed source diffs                                   | `internal/rawdiff`                                |
| Rows with typed fields, badge mapping, sections, layout | `internal/browser`                                |
| Flags, saved review, readable command output            | `internal/cli`                                    |

Do not import Bubbles or lipgloss. `internal/terminal` stays the only text and
viewport boundary.

## Frame

```text
AFTER · payment · base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
 1 Overview   2 Changes 1   3 Diff   4 Activity
 …body…
Pin reopened: no result for this candidate yet — r previews a rerun
↑↓ move  enter open  r rerun  2 changes  / search  ? help  q quit
```

- Rows, top to bottom: header, tab bar, body, next/status line, key hints. Body height
  is the terminal height minus 4. Style and position separate the rows; there are no
  horizontal rules.
- Header: `AFTER · <project> · base <id> (<source>) → candidate <id> (<source>)`.
  Source is `commit <git short hash>`, `working tree`, `staged`, or
  `merge base <git short hash>`, from the snapshot record. In follow-up mode (AFTER-33)
  the left side reads `last inspected <id>`. On the right, show `new capture <id> · u`
  while a capture is pending, and `running 1:12 · x` while a run is active.
- Tab bar: `1 Overview`, `2 Changes <count>`, `3 Diff`, `4 Activity`. Show tabs only
  for views that exist. The active tab is reverse video, and bracketed in no-color
  mode. Drill-in screens replace the tab bar with a breadcrumb, such as
  `Overview › 12h same-key retry`, followed by the row's badge.
- Height under 12: hide the tab bar (number keys still work). Height under 7: hide
  the next/status line, and shrink the hints to `? help · q quit`.
- Width 110 or more: Overview and Changes split into a list (about 45%) and a
  preview pane, separated by `│`. Narrower terminals use one pane, and Enter opens
  the preview full-screen.
- Width under 60: drop the project and source words from the header, shorten the
  tab labels (`1 Ov 2 Ch 3 Df 4 Ac`), and keep only the highest-priority hints.
- The 240×100 viewport cap remains.

## Visual vocabulary

### Badges

A badge is a bracketed upper-case word in a fixed 14-cell column, such as
`[DIFFERENT]`. Brackets appear in every mode. In color mode, the whole badge,
brackets included, takes one style, so the plain text `[WORD]` stays contiguous for
tests and screen readers. One tested function derives badges, from typed engine
fields only. The first matching rule wins.

| Badge          | Row types        | Rule                                                                      | Style     |
| -------------- | ---------------- | ------------------------------------------------------------------------- | --------- |
| `UNAVAILABLE`  | any              | The stored ID is missing, corrupt, or unsupported                         | problem   |
| `REOPENED`     | pin              | Decision is `reopened`                                                    | attention |
| `STALE`        | any              | Applicability is `stale` (another pair, or a changed basis)               | attention |
| `UNKNOWN`      | pin, observation | Applicability is `unknown`                                                | attention |
| `FAILED`       | observation      | Execution `failed`                                                        | problem   |
| `CANCELLED`    | observation      | Execution `cancelled`                                                     | problem   |
| `INCOMPLETE`   | observation      | Completeness `incomplete`, or comparison `incomparable`                   | problem   |
| `UNSTABLE`     | observation      | The case's comparison is `unstable`                                       | attention |
| `DIFFERENT`    | observation      | The case's comparison is `different`                                      | changed   |
| `EQUAL`        | observation      | The case's comparison is `equal`, complete and current                    | observed  |
| `NOT COMPARED` | observation      | No comparison record                                                      | muted     |
| `REPORTED`     | report           | An imported report card; the reported outcome goes in the trailing column | reported  |
| `ACCEPTED`     | pin              | Decision `accepted`, current                                              | decision  |
| `PINNED`       | pin              | Decision `pinned`, current                                                | decision  |
| `NOT CHECKED`  | none             | No evidence for the item or screen                                        | muted     |

Precedence by row type: observations use `UNAVAILABLE`, `STALE`, `UNKNOWN`,
`FAILED`, `CANCELLED`, `INCOMPLETE`, `UNSTABLE`, `DIFFERENT`, `EQUAL`, then
`NOT COMPARED`. Reports use `UNAVAILABLE`, `STALE`, then `REPORTED`. Pins use
`UNAVAILABLE`, `REOPENED`, `STALE`, `UNKNOWN`, `ACCEPTED`, then `PINNED`.

A payment case's outcome comes from that case's comparison witnesses, not from
the receipt-level aggregate. Any differing `repetition` witness for the case means
`unstable`; otherwise, any differing `paired` witness means `different`; otherwise
the case is `equal`. An incomplete or incomparable comparison stays `INCOMPLETE`.

The trailing column of a row says what kind of evidence it is and how fresh it is:
`observed · current`, `ran on ac148d6a` (history), `reported · fail`,
`no current result`, `current result attached`, or `accepted 19:58`.

### Styles

`internal/terminal` owns a small theme of fixed SGR sequences: ANSI 16-color
foregrounds plus bold, faint, and reverse. Terminal themes keep control of the
actual colors. Styling goes through one function that sanitizes and clips its input
before adding SGR, so untrusted text can never be styled raw. Color is on unless
`NO_COLOR` is non-empty or `TERM` is `dumb`. Tests force it off. This supersedes
AFTER-12's "no theme" decision for styling only.

| Style     | Look         | Used for                                                                           |
| --------- | ------------ | ---------------------------------------------------------------------------------- |
| observed  | green        | `EQUAL`                                                                            |
| changed   | bold magenta | `DIFFERENT`; changed values such as `1 → 2`; `changed` markers                     |
| attention | bold yellow  | `REOPENED`, `STALE`, `UNKNOWN`, `UNSTABLE`; the pending-capture notice             |
| problem   | bold red     | `FAILED`, `CANCELLED`, `INCOMPLETE`, `UNAVAILABLE`; reported `fail`; errors        |
| reported  | blue         | `REPORTED`; report lines                                                           |
| decision  | cyan         | `PINNED`, `ACCEPTED`; prompt titles                                                |
| muted     | faint        | `NOT CHECKED`, `NOT COMPARED`, history rows, gutters, trailing columns, key labels |
| strong    | bold         | header, group headers, the selected row's name, key names                          |

Outside the Diff view, green is reserved for a current, complete, equal
observation; reported passes and human acceptance are never green. The Diff view
uses green `+`, red `-`, cyan `@@`, and bold `diff --git` lines; `+` and `-` keep
their meaning without color. The selected row is marked by `>` in every mode, plus
bold in color mode.

### Formatting

- IDs: 8 hex characters in chrome and rows (`a750186b`). Full `sha256:` IDs appear in
  the **IDs** section of a detail, in Activity details, and in session output.
- Delays use the largest exact units: `43200s` → `12h`, `90s` → `1m30s`, `30s`.
- Times: local `15:04:05` for today, otherwise `Jan 2 15:04`. Durations: `110ms`,
  `32s`, `1m05s`. Exact RFC 3339 values stay in record sections.
- Counts: `1 → 2` when every repetition on a side agrees; otherwise list each
  repetition, as in `1,1 → 2,1`.
- Paths: truncate the middle and keep the file name (`internal/…/handlers/status.go`).

## Overview

Overview is the landing view: the contact sheet with "needs another look" first.
This is the payment loop after the regression rerun, before any decision:

```text
AFTER · payment · base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
 1 Overview   2 Changes 1   3 Diff   4 Activity
 NEEDS ANOTHER LOOK 2
 > [REOPENED]     pin · At 43200s, expect 1 provider request(s) for the frozen…  current result attached
   [DIFFERENT]    12h same-key retry   provider requests 1 → 2 · responses same  observed · current
 AGREES 1
   [EQUAL]        30s same-key retry   provider requests 1 → 1 · responses same  observed · current
 ▸ EARLIER SNAPSHOTS 2
 CHANGES 1 path · computed from captured sources · no potential oracles   2 shows all

Current result attached — compare it with the expectation; a accepts
↑↓ move  enter open  a accept  r rerun  2 changes  / search  ? help  q quit
```

Groups appear in this order, and empty groups are omitted. Headers show counts.
Enter on a header collapses or expands it.

| Group                                      | Rows                                                                                                                                                                   |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| NEEDS ANOTHER LOOK                         | `REOPENED` pins, then `STALE` or `UNKNOWN` pins, `UNAVAILABLE` IDs, then current observations that are `FAILED`, `CANCELLED`, `INCOMPLETE`, `UNSTABLE`, or `DIFFERENT` |
| PINNED EXPECTATIONS                        | `PINNED` and `ACCEPTED` pins                                                                                                                                           |
| AGREES                                     | Current `EQUAL` observations                                                                                                                                           |
| REPORTED — imported, not observed by AFTER | For each report, one line (below), then its cards. Stale reports keep `STALE`                                                                                          |
| EARLIER SNAPSHOTS                          | Observations for another pair (history). Starts collapsed                                                                                                              |
| OTHER                                      | `NOT COMPARED` observations                                                                                                                                            |

Within a group, keep engine order (12h before 30s; evidence in selection order).

Report cards read `example.com/cart (package)` or `TestFreeShippingThreshold`, using
the card's scope. Each report gets a line that starts with trusted fields (short
report ID, binding, import time, and outcome counts) and ends with the caller's
producer string as clipped data. That line tells two reports with identical names
apart:

```text
 REPORTED — imported, not observed by AFTER 4
   report 5d1c9e02 · bound to d166c6ac · imported 13:17 · 2 pass · go version go1.27.1 darwin/arm64; candidate suite
   [REPORTED]     example.com/cart (package)                                       reported · pass
   [REPORTED]     TestFreeShippingThreshold                                        reported · pass
   report 0b77e4a1 · bound to d166c6ac · imported 13:18 · 2 fail · go version go1.27.1 darwin/arm64; base tests on candidate code
   [REPORTED]     example.com/cart (package)                                       reported · fail
   [REPORTED]     TestFreeShippingThreshold                                        reported · fail
```

The **CHANGES** line counts paths, hunks, and potential oracles. It counts paths
because binary and mode-only changes have no hunks. With a shared captured patch,
it reads `7 paths · 4 hunks, all unclassified · 1 potential oracle`, using
`rawdiff.Count`. No example-to-code mapping exists yet, so every hunk is
unclassified. Potential-oracle paths are listed under the line, because a changed
oracle is a trust signal. With evidence loaded, the line ends with `2 shows all`,
pointing to the complete inventory. With no evidence, Overview lists every path
instead:

```text
AFTER · project · base c99cc0cd (commit 1b2c3d4) → candidate 199a8a2c (working tree)
 1 Overview   2 Changes 7   3 Diff   4 Activity
 NOT CHECKED — no evidence was loaded for this change
   Nothing was run or imported for candidate 199a8a2c. The change is readable below.
 CHANGES 7 paths · 4 hunks, all unclassified · 1 potential oracle
 > M  testdata/health.golden.json                                          +1 −1  oracle
   M  assets/logo.png                                                      binary
   M  build.sh                                                             mode 100644 → 100755
   D  handlers/health.go                                                   −3
   A  handlers/status.go                                                   +3
   D  legacy/xml.go                                                        −4
   ?  .env.local                                                           excluded: untracked; not selected

Not checked — read the change, or c captures again after editing
↑↓ move  enter open  2 changes  3 diff  c capture  / search  ? help  q quit
```

### Next line

The line shows the text below verbatim, in every view, with no label. "Selected"
means the selected row of the current view. The first matching rule wins. A
transient action result replaces the line until the next key press, and every
result is also logged in Activity.

| State                                            | Next line                                                                      |
| ------------------------------------------------ | ------------------------------------------------------------------------------ |
| Records loading                                  | `Loading stored records — nothing runs on open`                                |
| Load failed (the body shows the sanitized error) | `Couldn't load this pair — check the IDs with after inspect`                   |
| Consent screen                                   | `Nothing has run. y runs this exact plan once · n denies`                      |
| Run active                                       | `Running the approved plan · 1:12 · x cancels (the incomplete result is kept)` |
| Capture pending                                  | `New capture 9c01d2e4 — u reviews it`                                          |
| Selected pin reopened, no current result         | `Pin reopened: no result for this candidate yet — r previews a rerun`          |
| Selected pin has a current complete result       | `Current result attached — compare it with the expectation; a accepts`         |
| No evidence loaded                               | `Not checked — read the change, or c captures again after editing`             |
| Otherwise                                        | blank                                                                          |

Until AFTER-30 adds `a`, the accept rule says
`Current result attached — accept it with after pin PIN --accept`.

## Detail

Enter opens a drill-in detail. A detail is a list of sections, with **Card**
first. This is a payment observation:

```text
AFTER · payment · base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
 Overview › 12h same-key retry                                       [DIFFERENT] observed · current
 Card  Witnesses 2  Artifacts 6  Receipt  Scenario  Plan  IDs
                         base a750186b                      candidate 784eb013
   Provider requests     1                                  2                            changed
   Response 1            200 {"payment":"pay_1","status":…  200 same
   Response 2            200 {"payment":"pay_1","status":…  200 same

   Input     key synthetic-key-a · body {"amount_cents":1200,"currency":"USD"} · second request 12h later
   Witness   provider: added /1 = {"at":43200,"method":"POST",…}
   Samples   1 repetition per side — one sample cannot show instability
   Run       runner · 19:56:10–19:57:42 (1m32s) · complete · not redacted
   Scope     Sequential synthetic payment ABI only; two same-key requests at 12h and 30s; …  (3 limits)


tab section  ↑↓ scroll  b bytes  / search  p pin  esc back  ? help
```

A pin:

```text
 Overview › Pin                                                  [REOPENED] current result attached
 Card  History 3  Current result  Basis receipt  IDs
   Expectation   At 43200s, expect 1 provider request(s) for the frozen two same-key requests;
                 finite example only                      (your words; AFTER does not evaluate them)
   Scope         finite example · basis receipt 6c43983d on a750186b → ac148d6a
   Reviewing     a750186b → 784eb013 (original base)
   Reopened      <the engine's exact reason>
   Current       receipt 0f51d3b0 · 12h 1 → 2 [DIFFERENT] · 30s 1 → 1 [EQUAL]
   History
     19:43:05  pinned    Preserve this finite provider-request count; not whole-change approval
     19:51:12  selected  candidate 784eb013, original base · reopened
     19:57:44  attached  receipt 0f51d3b0 · stays reopened until you decide
```

An imported report card:

```text
 Overview › TestFreeShippingThreshold                     [REPORTED] fail · applicability unknown
 Card  Output  Report  IDs
   Package     example.com/cart
   Test        TestFreeShippingThreshold · attempt 1
   Outcome     fail, as reported by go test JSON. AFTER did not run or observe this test.
   Producer    go version go1.27.1 darwin/arm64; base tests on candidate code (caller-supplied)
   Bound to    candidate d166c6ac (the caller's choice; a digest is not a signature)
   Events      13:18:02.105–13:18:02.215 (110ms)
   Inputs      unavailable; test reports record neither inputs nor effects
   Output      === RUN   TestFreeShippingThreshold
               shipping_test.go:8: ShippingCents(5000) = 599, want 0
               --- FAIL: TestFreeShippingThreshold (0.00s)                   tab: full output
```

Sections by row type:

| Row                       | Sections                                                       |
| ------------------------- | -------------------------------------------------------------- |
| Payment case              | Card · Witnesses · Artifacts · Receipt · Scenario · Plan · IDs |
| Receipt without case rows | Card · Artifacts · Receipt · Scenario · Plan · IDs             |
| Pin                       | Card · History · Current result · Basis receipt · IDs          |
| Report card               | Card · Output · Report · IDs                                   |
| Changes path              | Diff · Base source · Candidate source · Inventory record       |
| Unavailable ID            | Card (the limitation) · IDs                                    |

Rules:

- Cards are deterministic templates over strictly decoded records: receipt,
  comparison report, `runner.Observation`, `runner.Sample`, scenario and frozen
  input, pin view, and report card. If a record doesn't decode or has an unexpected
  shape, show its raw content with a limitation instead of guessing. Record values
  always render as data.
- A section is an ordered list of parts. Each part has a trusted title built from
  typed fields, plus content. The viewer draws the title as a styled divider row
  (`── candidate · 12h · repetition 1 · observation · 314 B ──`) and the content as
  gutter rows, so content can never forge a divider.
- Witnesses holds one part per witness for the case (relation, channel, sides,
  outcome, and changes as `added /1 = …` lines), then the comparison record and the
  `comparison-rules` artifact.
- Artifacts holds the case's channels matching
  `^(base|candidate)/(43200|30)/[0-9]+/(observation|sample|candidate-diagnostics|observer-diagnostics)$`,
  ordered by side, repetition, and channel. `execution-plan` goes in Plan. Any other
  channel goes in an **Other artifacts** part under its escaped raw name.
- Every artifact referenced by the receipt, comparison, scenario, and pin history
  stays reachable from some detail.
- Long prose fields wrap inside the card with a bounded wrap helper in
  `internal/terminal`. It uses the same sanitizing and grapheme widths as `Line`.
- At widths of 110 or more, Overview shows the selected row's Card in the preview
  pane.

## Changes

```text
AFTER · project · base c99cc0cd (commit 1b2c3d4) → candidate 199a8a2c (working tree)
 1 Overview   2 Changes 7   3 Diff   4 Activity
 POTENTIAL ORACLES — tests, fixtures and golden files changed with the code 1
 > M  testdata/health.golden.json                                          +1 −1  oracle
 CHANGED 5
   M  assets/logo.png                                                      binary
   M  build.sh                                                             mode 100644 → 100755
   D  handlers/health.go                                                   −3
   A  handlers/status.go                                                   +3
   D  legacy/xml.go                                                        −4
 UNKNOWN — not fully captured 1
   ?  .env.local                                                           excluded: untracked; not selected
 7 paths · 4 hunks, all unclassified

Not checked — read the change, or c captures again after editing
↑↓ move  enter open  3 diff  / search  ? help  q quit
```

- Letters: `A` added, `D` deleted, `M` modified, `?` unknown. Paths keep inventory
  order (sorted) within each group.
- Flags are words: `oracle`, `binary`, `mode <old> → <new>`, and each recorded
  limitation verbatim (`excluded: untracked; not selected`).
- `+`/`−` counts come from the captured patch for the path, or from the computed
  diff (AFTER-32), which is labelled as computed.
- A rename shows as a deletion plus an addition, exactly as captured. Rename
  detection is out of scope.
- Enter opens the path's detail. A missing side is stated, never left blank.
- At widths of 110 or more, the preview pane shows the selected file's diff.

## Diff

```text
 1 Overview   2 Changes 7   3 Diff   4 Activity
 handlers/status.go · added · file 4 of 6 · captured patch                 ] next file  } next hunk
           │ diff --git a/handlers/status.go b/handlers/status.go
           │ new file mode 100644
           │ --- /dev/null
           │ +++ b/handlers/status.go
           │ @@ -0,0 +1,3 @@
         1 │+package handlers
         2 │+
         3 │+func Status() string { return "healthy" }
           │ diff --git a/legacy/xml.go b/legacy/xml.go
           │ deleted file mode 100644
```

- The complete captured patch appears unchanged after the gutter. The gutter shows
  old and new line numbers on hunk lines when the `@@` header parses, and stays blank
  otherwise.
- A sticky header names the file containing the top visible line, its position, its
  flags, and the diff's origin: `captured patch` or `computed from captured sources`.
- `]`/`[` move between files and `}`/`{` between hunks, using the rawdiff index.
  Opening Diff from a Changes path lands on that file.
- Binary patches, mode-only changes, additions, and deletions get a trusted summary
  divider (such as `binary`), and their raw lines stay visible in muted style.

### Computed diffs (AFTER-32)

A captured patch exists only for a commit base and the candidate captured with it
(`rawdiff.CapturedPair`). Every snapshot switch, and every follow-up comparison,
pairs snapshots from different captures, so today the reviewer loses the diff right
after the first edit. For such pairs, `internal/rawdiff` computes a unified diff
from the two stored file sets.

- Inventory order, Git-style headers, 3 lines of context, and the same viewer and
  navigation as a captured patch.
- Labelled `computed from captured sources — not Git's patch` in the sticky header,
  the Overview CHANGES line, and Changes counts. Computed hunks are never counted as
  captured hunks.
- Pure Go and deterministic, with no subprocess or new dependency. Per changed
  path, the base+candidate source pair is limited to 1 MiB and 20,000 combined
  lines; paths are capped at 4 KiB and the whole pair at 8 MiB of source bytes.
  The Myers edit walk
  stops at 1,000,000 diagonal/comparison steps. The rendered computed patch is
  capped at 16 MiB. A source blob read is already capped by storage at 16 MiB;
  binary classification checks the first 8,000 bytes of either side before text
  diffing. Paths over a computation/output bound show
  `too large to diff here — open both sources`; unknown paths retain their recorded
  limitation and have no computed hunk. Mode-only changes show
  `mode <old> → <new>`.
- Computation and document indexing run in the existing background `Load` job, not
  in the TUI event loop. Computed navigation offsets do not create hunk IDs or
  change captured `Hunks`/`Count` semantics.
- A fuzz/property test applies the computed unified diff to the base and checks the
  candidate bytes, while repeating the same inputs to verify deterministic output.

## Content viewer

Every document uses one viewer: detail parts, sources, the diff, the exact plan, and
Activity details.

- One captured line per row, behind a trusted gutter (`  12 │`). Payload newlines
  end a row; they never create chrome. Tabs expand to 4-column stops.
- Control and format characters render as visible `\uNNNN` escapes, and invalid
  UTF-8 as U+FFFD; this is existing `terminal.Line` behavior.
- Content with a NUL byte in its first 8,000 bytes opens as a hex dump
  (`00000000  89 50 4e 47 0d 0a 1a 0a  …  |.PNG....|`). `b` toggles the hex view
  for any document, so the exact bytes, including CR, BOM, and trailing whitespace,
  are always one key away.
- Stored JSON records and AFTER-written JSON artifacts (observation, sample,
  comparison details, execution plan) are indented with `json.Indent` for reading.
  Captured sources, diagnostics, and report output are shown verbatim. The hex view
  shows the exact stored bytes, and no stored artifact changes.
- Documents up to the `terminal.Document` limits (16 MiB, 250,000 lines) are
  indexed once, off the event loop, and scroll continuously. Past the line limit,
  show a limitation; the hex view still reaches every byte.
- Long lines clip with `…`. Horizontal pan reaches the first 4 KiB of a line; past
  that, the marker points to the hex view. Per-event work stays within the
  [TERMINAL.md](TERMINAL.md) budgets on the 100,000-line captured diff.
- Soft wrap is limited to bounded prose in cards. Documents use pan and hex instead.

## Consent

```text
AFTER · payment · base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
 Run this exact plan?   Summary   Exact plan 18.2 KiB
 Nothing has run. y runs exactly this plan once (plan 33d2a7af); n or esc denies.
   Snapshots     base a750186b → candidate 784eb013 (the selected pair)
   Runs          2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1
                 each run is an app container and an observer container
   Build         /usr/local/go/bin/go build -trimpath -o /work/app ./app
   App           /work/app http://127.0.0.1:18081 http://127.0.0.1:18082 · GOMAXPROCS=2
   Image         docker.io/library/golang@sha256:e0174e51…
   Topology      app network=none; observer joins app network only; separate PID, IPC, files…
   Mounts        no host mounts, volumes, sockets or published ports; only bounded /work and…
   Policy        --network=none --read-only --user=65534:65534 --cap-drop=ALL --security-op…
   Limits        180 seconds and 65536 output bytes per container
   Docker flags  21 per container; tab shows the exact plan

   y approve and run once        n deny        tab read the exact plan
```

- Decode the summary strictly (rejecting unknown fields) from **the exact preview
  bytes whose digest `y` approves**. Labels are trusted. Values are data copied
  verbatim from the decoded plan (`snapshots`, `repetitions`, `concurrency`,
  `build_argv`, `app_argv`, `app_environment`, `limits`, each container's `image`,
  `mounts`, and `docker_policy`, and each experiment's `Topology`), never
  paraphrased. Only
  counts are derived. If the experiments disagree on a copied value, show every
  distinct value. If decoding fails, open **Exact plan** with "summary
  unavailable", and `y` still approves exactly those bytes.
- **Exact plan** uses the content viewer; the old page-based consent flow is removed.
- Consent is modal. Only `y`, `n`, Esc, Tab, scrolling, `b`, `?`, `q`, and Ctrl-C
  work there. Search is unavailable, so `y` and `n` only ever approve or deny.
  Quitting denies.
- The approval conditions are unchanged: the selected pair, no active run or action,
  no invalidation by a new preview or snapshot switch, and paste never approves.

## Activity and session

```text
 1 Overview   2 Changes 1   3 Diff   4 Activity
 SESSION
   saved in .after/session.json after every change · after review resumes it
   pair       base a750186b → candidate 784eb013
   loaded     1a6270f3 receipt · 57c25a3c pin revision · 0f51d3b0 comparison
 ACTIVITY
 > 19:59:10  run cancelled     comparison c700c664 · incomplete result kept, not equality
   19:57:44  result attached   pin revision 57c25a3c · stays reopened until you decide
   19:57:42  run finished      comparison 0f51d3b0 · 12h 1 → 2, 30s 1 → 1
   19:56:10  run approved      plan 33d2a7af · 4 runs
   19:51:12  capture used      candidate 784eb013, original base · pin reopened
   19:50:58  capture finished  candidate 784eb013 ready
```

- Events cover capture, import, plan prepared, denied, or approved, run finished,
  failed, or cancelled, result attached, pin and selection changes, and errors.
  Enter shows full IDs and the full sanitized error. The log is bounded and says
  when older events were dropped.
- Job results and session references never become evidence rows. `s` opens the
  Session section.
- `i` binds the report to the candidate selected when `i` is pressed, and the event
  names it. A successful import adds the report to the loaded evidence (within the
  32-ID limit), so its rows appear without a restart. At the limit, the report stays
  stored and the event shows its ID.
- While capture, import, or a run is active, show elapsed time, ticking once per
  second only while active. Never show an invented percentage.
- `q` during a run asks for confirmation. Ctrl-C still quits at once, cancelling and
  joining owned work.

### Launch and resume

```sh
after review                 # capture HEAD vs the working tree, then open or resume
after review --staged        # capture HEAD vs the index instead
after review --base main     # capture the merge base with main vs HEAD
after review --new           # start a new review from a fresh capture
after review 784eb013        # open stored records, or a BASE CANDIDATE pair, without capturing
```

- `after review` captures exactly as `after capture` does, with the same flags: no
  project code runs, and untracked files stay excluded unless named. Recapturing an
  unchanged tree reuses the same snapshot IDs. Nothing else runs on open.
- The review is saved in `.after/session.json`: the pair, the comparison mode, and
  the capture flags. It is UI state, not evidence. It is written atomically with
  mode 0600 after every selection change and on quit, and read as untrusted input
  (a bounded regular file with valid IDs).
- With a saved review, `after review` opens its pair at once and captures in the
  background, as `c` does. If the fresh capture differs, it is pending (`u`); the
  selected pair never changes underneath the reviewer. Without a saved review,
  stderr shows elapsed time while capturing, then the fresh capture opens. A capture
  with no changes opens nothing; see [CLI-DESIGN.md](CLI-DESIGN.md#review).
- `c` captures again with the saved review's capture flags, or HEAD against the
  working tree when explicit IDs opened the review.
- `--new`, or different capture flags (for example, `--staged` after a
  working-tree review), starts a new saved review, and stderr names the review it
  replaced. An invalid session file is reported and replaced. No evidence is lost;
  it lives in the store.
- Explicit IDs open those records, or a `BASE CANDIDATE` pair, without capturing
  and without reading or changing the saved review.
- Evidence is discovered, not listed by hand: each pin's head revisions (forks show
  separately), the pair's newest runs and comparisons, and reports bound to the
  candidate. Within the 32-record limit, pins that need another look come first;
  Overview says how many older records `after log` lists. An explicit pin revision
  ID still opens exactly that revision. See [CLI-DESIGN.md](CLI-DESIGN.md#ids-and-defaults).
- On quit, stderr gets `Saved review a750186b → 784eb013 · after review resumes it`.
  `--json` also prints the session JSON to stdout. Without a terminal,
  `after review` exits 2 and points to `after status --json`.

## Prompts

```text
 ┌ Pin this expectation? ─────────────────────────────────────────────────────────────┐
 │ At 43200s, expect 1 provider request(s) for the frozen two same-key requests;      │
 │ finite example only                                                                │
 │ Basis   receipt 6c43983d · base a750186b → candidate ac148d6a                      │
 │ Reason  Preserve this finite provider-request count; not whole-change approval▏    │
 │ enter pin   esc cancel   the reason is stored in the pin's history                 │
 └────────────────────────────────────────────────────────────────────────────────────┘
```

- `p` (pin), `u` (use a new capture), and `a` (accept a pin) always confirm, stating
  the exact effect and target IDs. Nothing mutates on selection, navigation, or
  paste.
- Each prompt has a one-line reason field, prefilled with a default that Ctrl-U
  clears. The engine records the reason in the pin's history. The field accepts
  typing and bracketed paste (sanitized, at most 4,096 bytes) and refuses an empty
  reason.
- `p` refuses a case that already has a pin with the same basis receipt and
  expectation, and points to that pin.
- `a` is enabled only when the pin has a current complete receipt for the selected
  pair; the engine enforces this too. Otherwise the hints omit `a`, and pressing it
  explains why.
- `u` names the new candidate, counts the paths that differ from the candidate under
  review, and says that pins may reopen and earlier results become history. With
  AFTER-33 it also asks what "before" means: `original base <id>` (the default) or
  `last inspected <id>`, using the engine's existing `original_base` and
  `last_inspected` modes. The header shows the active mode. Changing the mode
  happens at the next `u`.

**Key change:** `a` moves from "accept snapshot" to "accept pin", matching
`pin --accept` and the `accepted` decision. AFTER-23 moves snapshot switching
to `u`.

## Key map

One table drives dispatch, footer hints, and the help overlay. Each entry has a
key, a label, a group, its contexts, an enabled check that returns a reason, and a
hint priority.

| Key                          | Context                              | Action                                                         |
| ---------------------------- | ------------------------------------ | -------------------------------------------------------------- |
| ↑/↓, `j`/`k`                 | lists, documents                     | Move or scroll                                                 |
| PgUp/PgDn, Home/End, `g`/`G` | lists, documents                     | Page; first or last                                            |
| Enter                        | lists; group headers                 | Open; collapse or expand                                       |
| Esc                          | everywhere                           | Back, close an overlay, or cancel a prompt (denies in consent) |
| `1`–`4`, Tab/Shift+Tab       | top level                            | Switch view (Tab cycles sections in detail and consent)        |
| ←/→, `h`/`l`                 | documents                            | Pan                                                            |
| `b`                          | documents                            | Toggle the exact bytes (hex)                                   |
| `/`, `n`/`N`                 | lists, documents (not consent)       | Search; next or previous match                                 |
| `]`/`[`, `}`/`{`             | Diff                                 | Next or previous file; next or previous hunk                   |
| `d`                          | top level                            | Changes (the complete inventory)                               |
| `c`                          | records loaded                       | Capture again in the background, with the review's flags       |
| `i`                          | `--import-file` given                | Import that report in the background                           |
| `u`                          | a new capture is pending             | Use the new capture (confirms)                                 |
| `p`                          | a pinnable current observation       | Pin its measured count (confirms)                              |
| `a`                          | a pin with a current complete result | Accept the pin (confirms)                                      |
| `r`                          | records loaded                       | Prepare a rerun preview; nothing runs                          |
| `y`, `n`                     | consent only                         | Approve once; deny                                             |
| `x`                          | jobs active                          | Cancel active jobs (names them)                                |
| `s`                          | outside consent                      | Activity › Session                                             |
| `?`                          | everywhere                           | Help overlay, grouped, including unavailable actions and why   |
| `q`, Ctrl-C                  | everywhere                           | Quit (`q` confirms during a run); quit now                     |

While a text field has focus (a prompt's reason or the search query), printable
keys and paste are text. Only Enter, Esc, Backspace, Ctrl-U (clear), and Ctrl-C
act.

## Copy

Use one vocabulary: base, candidate, capture, observed, reported, pin, expectation,
accept, history. Avoid `STATE`, `data`, `job`, `pass`, `safe`, and `verified`, and
don't call an observation a `regression`.

| Before                                                                                      | After                                                                                    |
| ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `STATE observed \| current \| completed \| equal \| complete \| report=none`                | `[EQUAL] … observed · current`                                                           |
| `"43200s: provider requests base=[1] candidate=[2]; finite recorded samples only"`          | `12h same-key retry · provider requests 1 → 2` (scope on the card)                       |
| `Stored records only; no new observation. c capture \| i import configured file`            | A blank next line; the hints show `c` and `i` when available                             |
| `Job finished; stored result retained; a accepts new snapshot`                              | `New capture 9c01d2e4 — u reviews it`                                                    |
| `Snapshot accepted; prior evidence is history, not a prediction; s session IDs for restart` | `Reviewing 9c01d2e4 · earlier results moved to history · pin reopened`                   |
| `No execution yet. The quoted plan is unreadable; y approves once; n denies`                | `Nothing has run. y runs this exact plan once · n denies`                                |
| `Authorized run active; x cancel; navigation and capture remain available`                  | `Running the approved plan · 0:42 · x cancels`                                           |
| `Action failed: select a complete current payment observation to pin`                       | The hints omit `p`; pressing it says `Can't pin: choose a current, complete observation` |
| `data \| "example.com/cart / "`                                                             | `example.com/cart (package)`                                                             |

Keep status messages under about 70 characters. Put errors after a trusted prefix,
sanitized, with the full text in Activity.

## Rendering untrusted content safely

These rules are non-negotiable. Each task keeps or extends their tests.

1. All untrusted text passes `internal/terminal` sanitizing before layout. This
   includes paths, test and channel names, outputs, producer strings, expectation
   and reason text, errors, and artifact bytes. Styling applies only after
   sanitizing and clipping.
2. Measure, clip, and wrap only with `internal/terminal`, which uses uniseg widths.
3. Untrusted text never occupies a badge, tab, group header, prompt title, or key
   hint. The header shows only the sanitized project name and AFTER's short IDs and
   source words.
4. Only the renderer creates rows. Every content row starts with a trusted gutter or
   sits in a fixed data column after trusted fields.
5. Badges, groups, and Next lines derive only from typed engine fields, through
   tested mappings. Never from names, output, expectation prose, or imported
   booleans.
6. With `NO_COLOR`, output contains no SGR. In color mode, tests assert that every
   escape sequence in a frame is a theme SGR (`ESC [ … m`).
7. The consent summary is decoded from the bytes `y` approves. Mutations confirm.
   Paste is data: it never triggers an action, and only enters text fields.
8. Everything stays reachable: the complete inventory, every artifact, full IDs, and
   exact bytes.

## Testing and verification

- Golden views in `internal/browser/testdata/views/` render fixed sizes (120×40,
  80×24, 40×12) in no-color mode from synthetic stores, without Docker. They are
  compared byte for byte, like `internal/compare/testdata/differences.json`. A
  test-only `-update` flag regenerates them; review the diff.
- Golden inputs must be deterministic: fix Git author and committer dates, use fixed
  timestamps in synthetic records, and inject the clock and time zone (UTC) into
  rendering.
- One color-mode test per view asserts that only theme SGR sequences appear.
- Keep the hostile-content tests, and assert that hostile strings appear only in data
  columns or content rows.
- `task test:terminal` (including the 100,000-line budget) and `task test:cli` PTY
  flows stay green with updated expectations. PTY transcripts must still contain no
  payload controls and must restore termios, the alternate screen, and the cursor.
- The Docker-gated `task tui:proof`, `task cli:proof`, and `task test:poc` (which
  includes the TUI PTY proof) assert on screen text and must be updated with the
  screens. Run them when Docker settings are available; otherwise record the blocker.
- Keep docs/TUI.md, docs/DEMO.md, examples/README.md, and the `examples/tui` hints in
  step with keys and screens.
- Check every task in a real terminal at 80×24 and 120×40, with and without
  `NO_COLOR`, and paste a tmux capture or PTY excerpt into the task notes.

## Not in this design

Mouse input, configurable themes, side-by-side diffs, syntax highlighting, rename
detection, grouping by divergence signature (there are only two payment cases),
automatic capture or file watching (a product rule), clipboard writes (OSC 52 is
deliberately blocked), soft-wrapped documents, and GitHub or browser surfaces.

## Delivery order

| Task     | Scope                                                       | Depends on | Priority |
| -------- | ----------------------------------------------------------- | ---------- | -------- |
| AFTER-21 | Content viewer: readable, safe documents                    | —          | High     |
| AFTER-22 | Theme, badges, per-case outcomes, golden views              | 21         | High     |
| AFTER-23 | Frame, key map, hints, help, `u`                            | 22         | High     |
| AFTER-24 | Plain `after review`: capture, resume, discovery            | 23, 36, 38 | High     |
| AFTER-25 | Activity and session view, timers, quit guard, live imports | 23, 24     | Medium   |
| AFTER-26 | Overview triage and Next line                               | 25         | High     |
| AFTER-27 | Evidence cards and wide preview, also in `after inspect`    | 26, 35     | High     |
| AFTER-28 | Changes list and Diff view                                  | 23         | High     |
| AFTER-29 | Readable consent summary                                    | 23         | High     |
| AFTER-30 | Confirmed mutations, reasons, pin acceptance                | 27         | Medium   |
| AFTER-31 | Search                                                      | 27, 28     | Medium   |
| AFTER-32 | Computed diffs for pairs without a shared patch             | 28         | High     |
| AFTER-33 | Comparison mode: original base or last inspected            | 30, 32     | Medium   |

The AFTER condition of the [human study](EVALUATION.md) (AFTER-18) uses this TUI.
The kit freezes the Git commit and executable checksum, so TUI changes after a
freeze need a refreeze and a new rehearsal. AFTER-18 therefore depends on the
high-priority tasks.
