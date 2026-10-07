# Command-line design

**Status: staged target design.** AFTER-34 to AFTER-36 and AFTER-38 are implemented;
AFTER-37 and the remaining command redesign tasks are still future work. This is
the design for backlog tasks AFTER-34 to AFTER-45, plus the `after review` launch
in AFTER-24 (milestone M2). [CLI.md](CLI.md) documents current behavior; do not
copy future commands from this target into runtime help before they exist.
The CLI shares vocabulary, badges, styles, and formatting with the
[review TUI design](TUI-DESIGN.md). The product invariants in
[AGENTS.md](../AGENTS.md) and [IMPLEMENTATION.md](IMPLEMENTATION.md) still apply.
Where this document changes an earlier CLI decision, it says so. Values in mockups
are illustrative.

## Why

An audit ran every command with no arguments in a fresh Git checkout with one
modified and one untracked file, then with typical arguments.

| Finding                        | Seen today                                                                                                                                                                                                                | Task   |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| Most commands fail bare        | `inspect`, `review`, `compare`, `import`, `pin`, and `export` exit 2 with `unexpected or missing command arguments`, without naming what is missing. `run` needs two IDs or a plan file                                   | 37     |
| JSON is the interface          | `after capture` prints 1.3 KB of JSON on one line. `inspect` returns the patch as base64. No result says what to do next                                                                                                  | 35     |
| IDs are 71 characters          | Prefixes are rejected (`invalid stable ID`), and nothing lists captures or runs, so IDs are copied out of earlier JSON                                                                                                    | 36, 37 |
| Inconsistent grammar           | `--base` is a snapshot ID in `inspect` and `review` but a Git ref in `capture`. Pairs are `CANDIDATE --base BASE` in `inspect` and `review`, but `BASE CANDIDATE` in `run`. Pin changes live under `review`               | 38     |
| Help is noisy and wrong        | Every command lists all ten global flags, including Docker flags on `capture`. Defaults are misreported: `--repetitions (default: 0)` is 1, `--run-seconds (default: 0)` is 180, `--interactive (default: false)` is true | 38     |
| Errors don't help              | `after captrue` prints `unknown command; use --help`. `after capture --stagd` prints `invalid command arguments or flags`                                                                                                 | 38     |
| Subdirectories fail and litter | From a subdirectory, or outside Git, `capture` prints `capture failed; repository was not changed` and leaves `.after/writer.lock` behind. Specific capture errors, such as an unmerged index, are hidden                 | 34     |
| `.after/` isn't ignored        | After the first capture, `git status` shows `?? .after/`, so `git add -A` would commit captured private content                                                                                                           | 34     |
| Import needs flags             | A report file is refused without `--producer`, and a pipe can't be read                                                                                                                                                   | 39     |
| Running is laborious           | An interactive `run` dumps the whole plan JSON to stderr before asking. A non-interactive run needs `--plan-out`, then `--plan-file` with `--approve`. Missing Docker settings get no guidance                            | 40     |
| Reviewing takes four steps     | Capture, copy two IDs, run `review --tui CANDIDATE --base BASE`, then rebuild that command with `--evidence` flags to resume                                                                                              | 24     |
| Committed work reviews nothing | On a feature branch whose changes are committed, `after capture` records an empty change, and nothing suggests `--base main`                                                                                              | 44     |
| No diff in the shell           | The patch is only available as base64 inside JSON                                                                                                                                                                         | 41     |
| No completion                  | Commands, flags, and IDs must be typed in full                                                                                                                                                                            | 42     |

## Principles

1. **Every command does the most likely thing with no arguments**, and says what it
   chose.
2. **Readable by default; JSON on request.** `--json` prints today's versioned
   object.
3. **Short IDs in and out.** Type a prefix, read 8 characters. JSON keeps full IDs.
4. **One grammar.** Pairs are always `BASE CANDIDATE`, and a flag means the same
   thing on every command.
5. **Every result names the next command, and every error says how to fix it.**
6. **Defaults never cross a safety line.** Nothing executes without exact consent,
   untracked files stay excluded, pin revisions are never guessed, and every
   resolved default is shown.

## Commands

| Command                 | With no arguments                                              | With arguments                                                                                         |
| ----------------------- | -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `after`, `after status` | Summarize this checkout's review and suggest the next command  | —                                                                                                      |
| `after review`          | Capture the working tree, then open or resume the review       | `--new`, `--staged`, `--base REF [--target REF]`, `--include-untracked PATH`; stored `ID…` or a pair   |
| `after capture`         | Capture HEAD versus the working tree (unchanged)               | `--staged`, `--base REF [--target REF]`, `--include-untracked PATH`                                    |
| `after diff`            | Print the newest capture's patch                               | `BASE CANDIDATE`, `--stat`, `--raw`                                                                    |
| `after log`             | List the 20 newest captures, runs, reports, and pin events     | `-n N`                                                                                                 |
| `after inspect`         | Summarize the newest capture                                   | `ID` (any record or plan), `BASE CANDIDATE`                                                            |
| `after import`          | Read `go test -json` from a pipe                               | `FILE`, `--producer TEXT`, `--snapshot ID`                                                             |
| `after run`             | Prepare a plan for the newest capture, then ask before running | `BASE CANDIDATE`, `--approve DIGEST`                                                                   |
| `after compare`         | Compare the newest run of the newest capture                   | `RECEIPT`                                                                                              |
| `after pin`             | List pins                                                      | `[RECEIPT] --expectation TEXT`; `PIN`; `PIN --accept`, `PIN --attach RECEIPT`, `PIN --select SNAPSHOT` |
| `after export`          | Print the newest comparison as JSON                            | `ID`                                                                                                   |
| `after config`          | Show settings, their sources, and setup problems               | —                                                                                                      |
| `after completion`      | Print the completion script for `$SHELL`                       | `bash`, `zsh`, `fish`                                                                                  |

Outside a Git checkout, bare `after` prints short help. `help`, `version`,
`--help`, and `--version` keep working everywhere. "Newest capture" always means
the most recent capture record, however it was made.

## Project and storage

- The project is the Git top-level that contains the current directory, or that
  contains `--project DIR`. Every command works from any subdirectory. Finding it
  only checks for `.git` in that directory and its parents, and runs nothing. The
  capture package still receives an explicit root and does no discovery itself.
- Outside a Git work tree, commands exit 2 with
  `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR`
  and create nothing.
- Read-only commands never create `.after/`. Writers create it only at the
  top-level, after Git resolution succeeds.
- A new `.after/` gets a `.gitignore` containing `*`, so Git ignores the store
  without AFTER editing the user's ignore files. An existing store gets one at its
  next writable open.
- Capture failures state the capture package's reason, such as
  `unmerged index is unsupported` or `shallow repositories are unsupported`. These
  are fixed strings, never repository text.
- Output never prints the project's absolute path or the values `after config`
  hides. Repository paths are relative to the top-level. The only absolute paths
  printed are suggested Docker settings for the user to copy.

## Output

- Commands print readable text to stdout. `--json` prints exactly today's versioned
  object (`schema_version`, `kind`, `data`); fields are only ever added. `export`
  always prints JSON. Diagnostics go to stderr, and exit codes are unchanged.
- **This supersedes** "headless commands emit one versioned JSON object on stdout".
  Scripts, tests, the demo, the study kit, and examples pass `--json`.
- Every result has one shape: a sentence saying what happened, aligned rows or a
  list, then a **Next** block with at most three runnable commands.
- On a terminal, output uses the TUI theme and badges (`[DIFFERENT]`, `[REOPENED]`).
  `NO_COLOR`, `TERM=dumb`, or a pipe turns styling off; the words stay.
- On a terminal, work still running after one second (a capture, an import, or a
  run) shows elapsed time on stderr, with no percentage.
- Untrusted text (paths, test names, expectations, producers, errors) is sanitized
  by `internal/terminal` and placed after trusted labels. On a terminal, rows clip to
  the width. In a pipe, nothing is clipped.
- IDs, times, counts, delays, and paths follow the
  [TUI formatting rules](TUI-DESIGN.md#formatting). Full IDs appear in `--json` and
  in the IDs section of `after inspect ID`.
- A suggested command includes a repository path only when the path is printable,
  single-quoted when it contains anything outside `A–Z a–z 0–9 . _ / -`.
- Readable output never prints raw artifact bytes. Use `after inspect ID --json`
  pages, or `after diff --raw` into a file.

## IDs and defaults

- An ID argument accepts any unique prefix of at least 4 hex characters, with or
  without `sha256:`, in any case. Resolution only considers the record kinds that
  are valid for that argument. Full IDs keep working.
- No match exits 2: `after: no capture or run matches 3f2a — after log lists recent records`.
  An ambiguous prefix exits 2 and lists up to 10 matches:

    ```text
    after: 3f2a matches 2 records — use more characters:
      3f2a1c90  snapshot  working tree · captured 19:50:58
      3f2a77e1  receipt   run a750186b → 784eb013 · 19:57:42
    ```

- `--approve` takes the full digest only. Consent binds exact bytes, and the
  preview prints the full digest in the exact command to run.
- Every capture writes an immutable **capture record**: capture time, mode, base,
  candidate, index snapshot, and the selected untracked paths. This includes
  `after capture`, `after review`, the TUI's `c`, and import binding. Snapshot IDs
  are unchanged: recapturing an unchanged tree yields the same snapshots and only a
  new capture record. [SCHEMA.md](SCHEMA.md) documents the record.
- Omitted arguments resolve as follows, and readable output names each one (for
  example, `Using the newest capture: base a750186b → candidate 784eb013`):

    | Argument       | Default                                       |
    | -------------- | --------------------------------------------- |
    | Pair           | The newest capture's base and candidate       |
    | Receipt        | The newest run receipt for that pair          |
    | Comparison     | The newest comparison for that pair           |
    | Report binding | A capture of the working tree taken at import |
    | Pin revision   | Never defaulted                               |

- Pins are listed by **head revision**: a revision that no other revision extends.
  Heads are computed from the immutable records on each invocation, with no stored
  pointer. A fork shows each head. An explicit older revision ID still opens exactly
  that revision, and readable output names its newer heads.

## status

```text
$ after
payment · base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
  Captured   19:50:58 · 1 path changed · untracked files excluded
  Behavior   run 1a6270f3 · 12h [DIFFERENT] 1 → 2 · 30s [EQUAL] 1 → 1
  Pins       1 needs another look · [REOPENED] At 43200s, expect 1 provider request(s)…
  Reports    none for this candidate

Next  after review                  compare the reopened pin with its current result
      after pin 57c25a3c --accept   accept it against its current result
```

Status reads stored records only; it never captures. When the saved review is on
another pair, a `Saved review` row names it. The first matching rule picks the
Next block:

| State                                                    | Next                                                              |
| -------------------------------------------------------- | ----------------------------------------------------------------- |
| No capture                                               | `after review` — capture this checkout and open it                |
| The saved review is on another pair                      | `after review` — resume it, `after review --new` — start over     |
| A pin needs another look                                 | `after review`, plus `after pin <id> --accept` when it can accept |
| The pair has no run, but earlier pairs in the project do | `after run` — prepare a rerun (asks before running)               |
| Otherwise                                                | `after review`, `after diff`                                      |

`after status --json` returns the same facts for scripts.

## review

`after review` is the one command a reviewer needs. Without a saved review, it
captures exactly as `after capture` does, with the same flags, then opens the TUI.
With a saved review, it opens that pair at once and captures in the background; a
changed capture waits as pending (`u`). `--new` starts a new saved review from the
fresh capture instead. Explicit IDs open stored records without capturing. The TUI
design's [Launch and resume](TUI-DESIGN.md#launch-and-resume) specifies sessions,
evidence discovery, and the quit message. Without a terminal, `after review` exits
2 and points to `after status --json`.

When the fresh capture has no changes and there is no saved review, `after review`
opens nothing, exits 0, and suggests capture flags that would find a change:

```text
$ after review
Nothing to review: the working tree matches HEAD (commit 9b1e2d4).
This branch is 3 commits ahead of main.

Next  after review --base main   review the branch's commits
```

It suggests `--base` with the default branch when HEAD is ahead of it, and
`--include-untracked` when untracked files were excluded. The default branch is the
remote default (`origin/HEAD`), else a local `main`, else `master`. Nothing is
chosen automatically.

## capture

```text
$ after capture
Captured candidate 784eb013 (working tree) against base a750186b (commit 3f2a1c9)
  M  app/config.go                                                  +1 −1
  ?  notes.txt                                                      excluded: untracked; not selected
1 path changed · 1 hunk · no potential oracles
Untracked files aren't captured. Include one with: after capture --include-untracked notes.txt

Next  after review   open this change
      after diff     print the captured patch
```

An unchanged tree says `Unchanged since the capture at 19:50:58 (same snapshots)`.
A capture with no changes is still recorded, and its Next block makes the same
suggestions as [`after review`](#review).
Path rows use the [Changes](TUI-DESIGN.md#changes) letters, counts, and flags.
`after inspect` with no arguments prints the same summary for the newest capture.

## log

```text
$ after log
19:57:44     pin      57c25a3c  attached run 1a6270f3 · [REOPENED] At 43200s, expect 1 provider…
19:57:42     run      1a6270f3  a750186b → 784eb013 · 12h [DIFFERENT] 1 → 2 · 30s [EQUAL] 1 → 1
19:50:58     capture  784eb013  working tree against a750186b (commit 3f2a1c9) · 1 path
13:17:02     report   5d1c9e02  go test · 2 pass · bound to 784eb013
Oct 2 19:40  capture  ac148d6a  working tree against a750186b (commit 3f2a1c9) · 2 paths
20 newest of 31 · after log -n 50 shows more
```

Captures show their candidate snapshot ID, because that is the ID reviewers see
everywhere else. `after review 784eb013` opens that candidate with the base from its
newest capture record. Pin events show the revision they created.

## inspect

`after inspect` prints one record with the TUI's renderers:

| Record                                       | Readable output                                                                                    |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| A pair (`BASE CANDIDATE`, or no arguments)   | The capture summary rows, as `after capture` prints them                                           |
| Snapshot                                     | Its source, capture times, and path count                                                          |
| Receipt, comparison, pin revision, or report | The [Detail](TUI-DESIGN.md#detail) Card section                                                    |
| Plan                                         | The [consent summary](TUI-DESIGN.md#consent), then the indented plan                               |
| Artifact                                     | Its kind and size, then its content as the [content viewer](TUI-DESIGN.md#content-viewer) shows it |

Every record ends with an IDs section: one full ID per line, never clipped. Until
AFTER-27 lands, records without a renderer print their sentence and IDs, and point
to `--json`. Plans arrive with AFTER-40.

## compare

```text
$ after compare
Using the newest run of base a750186b → candidate 784eb013
Compared run 1a6270f3 · comparison 0f51d3b0
  12h same-key retry   [DIFFERENT]  provider requests 1 → 2 · responses same
  30s same-key retry   [EQUAL]      provider requests 1 → 1 · responses same
An exact comparison of the recorded samples, not a general guarantee.

Next  after review   look at the difference
```

Exit status 4 still reports a finding.

## pin

```text
$ after pin
2 pins in payment
  [REOPENED]  57c25a3c  At 43200s, expect 1 provider request(s) for…  current result attached
  [PINNED]    b80e2c17  At 30s, expect 1 provider request(s) for the…  pinned 19:43:05

Next  after review                  compare the reopened pin with its current result
      after pin 57c25a3c --accept   accept it against its current result
```

- `after pin RECEIPT --expectation TEXT` creates a pin; without `RECEIPT`, it pins
  the newest run of the newest capture. `--scope` defaults to `finite_example`; a
  receipt that can't support it gets an error suggesting `--scope human_intent`.
- On a terminal, `after pin RECEIPT` without `--expectation` asks for it. It lists
  the TUI's suggested expectation for each pinnable case, numbered, and reads one
  line: a number picks a suggestion, other text is the expectation, and an empty
  line cancels. Without a terminal, it exits 2 with a runnable example.
- `after pin PIN` prints the pin's card. `--accept`, `--attach RECEIPT`, and
  `--select SNAPSHOT [--mode original_base|last_inspected]` record decisions.
  `--mode` defaults to `original_base`.
- `--reason` is optional everywhere. Defaults say where the action came from, such
  as `Accepted from the command line`. History stores the reason verbatim.

## import

```text
$ go test -json ./... | after import
Imported report 5d1c9e02 · 2 pass · bound to candidate 784eb013 (working tree, captured now)
  Producer   not stated — add --producer TEXT to record where the tests ran
  Status     reported by go test; AFTER did not run or observe these tests

Next  after review   see the report beside the change
```

- Input is `FILE`, `-`, or a pipe on stdin. With no file and a terminal on stdin,
  import exits 2 and shows both forms.
- `--producer` is optional. Without it, the report stores no producer claim, and
  output shows `not stated`.
- Binding defaults to a capture of the working tree taken at import, which is
  usually the tree the tests just ran on. `--snapshot ID` binds another capture.
  If the capture fails, import fails with its reason and suggests `--snapshot`. A
  binding is still a caller claim, not proof of where the tests ran. When the
  capture excludes untracked files, the output says so, because the tests may have
  used them.

## run

On a terminal:

```text
$ after run
Using the newest capture: base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
Nothing has run. Plan 33d2a7af would run:
  Runs          2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1
  Build         /usr/local/go/bin/go build -trimpath -o /work/app ./app
  Image         docker.io/library/golang@sha256:e0174e51…
  Topology      app network=none; observer joins app network only; separate PID, IPC, files…
  …
  Exact plan    after inspect 33d2a7af (18.2 KiB)
Type yes to run exactly this plan once:
```

Without a terminal:

```text
$ after run
Using the newest capture: base a750186b (commit 3f2a1c9) → candidate 784eb013 (working tree)
Prepared plan 33d2a7af. Nothing has run. Approve exactly this plan with:
  after run --approve sha256:33d2a7af…   (the full digest)
```

- The summary rows are the [consent summary](TUI-DESIGN.md#consent), decoded from
  the exact plan bytes by the same function. Typing `yes` remains the interactive
  consent; this replaces printing the whole plan JSON.
- Plans are stored privately in `.after/` (mode 0600, never overwritten).
  `--approve DIGEST` finds the plan by its full digest, then reconstructs and
  checks it exactly as `--plan-file` does today. `--plan-out` and `--plan-file`
  keep working. The non-interactive preview still exits 3.
- During a run, stderr shows elapsed time on a terminal, with no percentage.
- Missing Docker settings are named with the configuration lines to add. A detected
  Docker CLI on `PATH` and an existing socket file may be suggested, but AFTER never
  contacts Docker and never picks an endpoint itself.

## diff

```text
$ after diff
diff --git a/app/config.go b/app/config.go
--- a/app/config.go
+++ b/app/config.go
@@ -1,5 +1,5 @@
 package main

-const retentionSeconds int64 = 24 * 60 * 60
+const retentionSeconds int64 = 5 * 60
 const deduplicate = true
 const printedCount = "1"
```

- With no arguments, the pair is the newest capture. `after diff BASE CANDIDATE`
  takes any two snapshots. Pairs without a shared captured patch use the
  [computed diff](TUI-DESIGN.md#computed-diffs-after-32).
- Stdout carries only the patch. Stderr names the pair and the diff's origin
  (`captured patch` or `computed from captured sources`), then lists the paths the
  patch doesn't cover: excluded, unsupported, and unknown paths.
- Lines are sanitized, and colored on a terminal like the TUI's Diff view. Tabs
  are kept in a pipe. When sanitizing changes any byte, stderr says so. `--raw`
  writes the exact bytes and is refused when stdout is a terminal.
- `--stat` prints the capture summary rows instead of the patch.
- There is no pager. `after review` is the interactive viewer.

## config

`after config` lists every setting with its value and source, as today; configured
paths and endpoints stay hidden. Below the table, it lists setup problems with the
exact fix, such as the Docker settings `after run` needs.

## Errors and help

- Errors read `after: <what is wrong> — <how to fix it>`, quoting user input
  sanitized. Exit codes are unchanged.
- A missing argument with no default is named, with a runnable example.
- An unknown command or flag suggests the closest match within two edits:
  `after: unknown command "captrue" — did you mean capture?` and
  `after: unknown flag --stagd — did you mean --staged?`.
- Help starts with one sentence, then usage examples, then only the command's own
  options with their real defaults, read from the configuration defaults. Global
  options (`--project`, `--config`, `--json`) follow in a short group. Docker and
  run limits appear only on `run`.

```text
$ after capture --help
Capture this checkout's change. Nothing in the project runs, and untracked files
are excluded unless you name them.

Usage
  after capture                            HEAD against the working tree
  after capture --staged                   HEAD against the index
  after capture --base main                the merge base with main against HEAD
  after capture --include-untracked PATH   also capture one untracked file

Options
  --staged                   capture the index instead of the working tree
  --base REF                 capture the merge base with REF
  --target REF               the other side of a --base capture (default: HEAD)
  --include-untracked PATH   include a non-ignored untracked file (repeatable)

Global
  --project DIR   the checkout to use (default: the one containing this directory)
  --config FILE   configuration file (default: ~/.config/after/config.yaml, if present)
  --json          print the versioned JSON result
```

Top-level help leads with the everyday commands:

```text
$ after --help
See what a change does differently before you accept it. Nothing in your project
runs unless you approve an exact plan.

Everyday
  after              what to do next in this checkout
  after review       capture your change and review it
  after diff         print the captured patch
  after log          recent captures, runs, reports, and pin events

Evidence
  after run          prepare a run of both sides, then ask before running it
  after import       read go test -json from a pipe
  after compare      compare a run's results
  after pin          list pins, or pin an expectation
  after inspect      show any record
  after export       print a comparison as JSON

Setup
  after capture      capture without opening the review
  after config       settings and setup problems
  after completion   print a shell completion script

Global options: --project DIR, --config FILE, --json
Run after COMMAND --help for a command's options.
```

### Grammar changes

| Today                                                     | Planned                                                         |
| --------------------------------------------------------- | --------------------------------------------------------------- |
| `inspect CANDIDATE --base BASE`                           | `inspect BASE CANDIDATE`                                        |
| `review --tui CANDIDATE --base BASE [--evidence ID…]`     | `review BASE CANDIDATE [EVIDENCE_ID…]` (needs a terminal)       |
| `review PIN`                                              | `pin PIN`                                                       |
| `review PIN --select ID --mode MODE --reason TEXT`        | `pin PIN --select ID [--mode MODE] [--reason TEXT]`             |
| `review PIN --receipt ID --reason TEXT`                   | `pin PIN --attach ID [--reason TEXT]`                           |
| `review PIN --accept --reason TEXT`                       | `pin PIN --accept [--reason TEXT]`                              |
| `pin RECEIPT --expectation TEXT --scope SCOPE --reason R` | `pin [RECEIPT] --expectation TEXT [--scope SCOPE] [--reason R]` |
| `capture --base REF --target REF`                         | `capture --base REF [--target REF]`                             |
| `import FILE --producer TEXT`                             | `import [FILE] [--producer TEXT]`                               |
| `run --plan-file FILE --approve DIGEST`                   | `run --approve DIGEST`                                          |
| `review --tui … --import-file FILE --producer TEXT`       | `review … --import-file FILE [--producer TEXT]`                 |
| `--base`: a snapshot ID or a Git ref                      | `--base`: a Git ref only                                        |

AFTER-38 makes these changes, except the `import` row and the optional `--producer`
(AFTER-39), and the `run` row (AFTER-40). Removed forms fail with an error that
shows the new form. `review`, `inspect`, and `run` accept snapshot pairs as
positional `BASE CANDIDATE` IDs; `review` accepts trailing evidence IDs. Bare
capture/resume selection and record-ID review remain later work.

## completion

`after completion [bash|zsh|fish]` prints a script; without an argument, it uses
`$SHELL`. Scripts complete commands, flags, flag values (`--scope`, `--mode`), and
ID arguments. ID candidates are the 50 newest matching records, shown as short IDs
with a sanitized one-line description. Completion reads the store read-only and
never captures, imports, or runs anything.

## Safety rules

These rules are non-negotiable. Each task keeps or extends their tests.

1. No default executes anything. `after run` prepares a plan and asks on a
   terminal, or exits 3 without one. Consent is still typing `yes` or the exact
   `--approve` digest.
2. Captures, including implicit ones in `after review` and `after import`, follow
   the `after capture` policy: no project code runs, and untracked files are
   excluded unless named.
3. Every resolved default is named in readable output, and its full ID is in the
   JSON.
4. Pin revisions are never defaulted, and an explicit revision always opens exactly
   that revision.
5. Untrusted text is sanitized before layout. Styling is applied only on a terminal,
   only through the theme, and never with `NO_COLOR`.
6. No new Docker or network contact. Setup checks only read configuration and
   stat files.

## Testing and verification

- Golden readable outputs for every command live in `internal/cli/testdata/`. They
  use synthetic stores, a fixed clock and time zone, no color, and 80 columns, and
  regenerate only with a test-only `-update` flag.
- One color test per command asserts that every escape sequence is a theme SGR.
- JSON tests keep the versioned schema, using `--json`.
- Hostile-content tests cover paths, test names, expectations, producers, and
  errors in readable output and in suggested commands.
- Update every caller in the same task: `examples/`, `internal/demo` (including the
  study kit), `internal/pocgate`, Taskfile proofs, and docs (CLI.md, REVIEW.md,
  DEMO.md, EVALUATION.md, examples/README.md).
- Run each changed command in a real terminal and in a pipe, and paste the output
  into the task notes.

## Delivery order

| Task     | Scope                                                     | Depends on | Priority |
| -------- | --------------------------------------------------------- | ---------- | -------- |
| AFTER-34 | Checkout root, ignored `.after/`, specific capture errors | —          | High     |
| AFTER-35 | Readable output by default, `--json`                      | 22         | High     |
| AFTER-36 | Short ID prefixes, capture records, pin heads             | 34         | High     |
| AFTER-38 | One grammar, help, and errors                             | 35         | High     |
| AFTER-37 | Bare commands: status, log, defaults, Next blocks         | 36, 38     | High     |
| AFTER-24 | Plain `after review`: capture, resume, discovery          | 23, 36, 38 | High     |
| AFTER-39 | Import from a pipe, optional producer, binding at import  | 36, 38     | Medium   |
| AFTER-40 | Readable `run` consent, stored plans, Docker guidance     | 29, 37     | Medium   |
| AFTER-41 | `after diff`                                              | 32, 37     | Medium   |
| AFTER-42 | Shell completion                                          | 36, 38     | Low      |
| AFTER-43 | `after review --new`, the saved review in status          | 24, 37     | Medium   |
| AFTER-44 | Guidance when the working tree matches HEAD               | 24, 37     | Medium   |
| AFTER-45 | Pin expectation prompt on a terminal                      | 37         | Low      |

AFTER-35 prints results without Next blocks. AFTER-37 adds them, suggesting only
commands that exist, and AFTER-40, AFTER-41, and AFTER-43 add their own commands.
AFTER-27 also prints its evidence cards from `after inspect ID`, so it depends on
AFTER-35. The [human study](EVALUATION.md) (AFTER-18) freezes the binary, so it
waits for the high-priority CLI tasks as well as the TUI tasks.
