# Headless CLI

`CLI.md` documents the commands shipped in `after`; [CLI-DESIGN.md](CLI-DESIGN.md) records their interaction contract. Use `after --help` and `after COMMAND --help` for the installed binary's syntax.

`after` is a Go CLI over local capture, private storage, Go test JSON/JUnit XML report import, diff, the frozen payment and explicitly selected HTTP-service/command runners, comparison and pins. It has no model, account, GitHub or editor dependency. Help, version and configuration display do not run repository code. Data commands print readable text by default; `--json` requests the version-1 `schema_version` / `kind` / `data` envelope, while `export` always emits JSON. Diagnostics go to stderr. See [DEMO.md](DEMO.md) for packaging and a prepared-checkout walkthrough.

Build with `task build`. These commands show the usual capture-to-review path:

```sh
./bin/after capture --project /work/payment
./bin/after inspect BASE CANDIDATE --project /work/payment
./bin/after review --project /work/payment
./bin/after diff --project /work/payment
```

A real capture in a throwaway Git checkout with one edited file and one untracked file looked like this (IDs shortened):

```text
$ after capture
Captured candidate 128f4386 (working tree) against base 89005345 (commit)
  Base         89005345 · 1 path · complete · 0 excluded · 0 unsupported
  Candidate    128f4386 · 1 path · complete · 1 excluded · 0 unsupported
  Limit        two matching reads; not an atomic filesystem snapshot
  Limit        diff includes captured regular files only; inspect excluded and unsupported inventory
Next
  after review
    open a review of this change
  after diff --stored
    print this captured patch
```

## Commands

Commands are `capture`, `review`, `diff`, `log`, `inspect`, `import`, `run`, `compare`, `pin`, `export`, `status`, `config` and `completion`; `help` and `version` are also available. Bare defaults and command-specific options are below and in `after --help`.

Global options are `--project DIR`, `--config FILE` and `--json`. Flags may come before or after positional arguments. Snapshot pairs are always `BASE CANDIDATE`. `--base` on `capture`, `diff` or `review` is a Git ref for a merge-base capture, with `--target` defaulting to `HEAD`; it is not a snapshot ID. `--include-untracked` selects exact, non-ignored paths and may repeat.

`help`, `version` and `config` work outside Git. Bare `after` prints short help outside a checkout; project commands exit 2 with `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR`. Unknown commands and flags suggest a close match within two edits. Missing arguments show a runnable example. Removed forms exit 2 with the replacement syntax.

## Checkout and private storage

By default, project selection starts at the invocation directory. `--project`, `AFTER_PROJECT` or YAML `project` can select a directory inside a checkout. The CLI finds the nearest parent containing a `.git` directory or worktree `.git` file by filesystem checks only; it runs no Git command to find the root. The capture API still receives an explicit repository root. Relative project and config paths resolve from the invocation directory.

Read-only commands do not create `.after/`. Writers open `.after/` at the checkout root, with directory mode `0700` and private files mode `0600`. A writable open creates `.after/.gitignore` containing `*`, including for an older store without that file. It does not edit the user's `.gitignore` or `.git/info/exclude`. The store admits one writer; a second writer fails rather than merging concurrent changes. See [STORAGE.md](STORAGE.md).

Capture failures exit 1 with fixed, allowlisted reason and fix text, never repository content or the absolute project path. For example:

```text
after: capture failed: unmerged index is unsupported — resolve the index conflicts, then retry capture
```

## Bare commands and stored defaults

Bare `after` and `after status` summarize the newest capture event, its pair, the newest run/comparison for that pair, applicable pin heads and reports bound to the candidate. They do not capture, import, run, compare or create storage. A readable result ends with a `Next` block of up to three available commands. JSON keeps full IDs. Bare `inspect`, `compare` and `export` include a `using` object with the resolved capture, snapshot, receipt or comparison IDs.

The summary labels itself `Stored capture` and checks whether the checkout still matches it. `after status --json` returns the same facts with full IDs. `freshness` is `matches`, `changed`, `unknown` or `not_checked`. The check rereads the capture's scope with the hardened reads in `internal/capture` and compares content hashes, not timestamps: HEAD, index, tracked files, selected untracked paths and excluded untracked inventory. Incomplete captures and unreadable checkouts are `unknown`; a failed read still prints the stored summary but exits 1. Merge-base captures are `not_checked` (`immutable_comparison`); `--stored` skips the check (`stored_requested`).

| Status condition                              | Suggested next step                                                 |
| --------------------------------------------- | ------------------------------------------------------------------- |
| No capture                                    | `after review`                                                      |
| A saved review is on another pair             | Resume with `after review`, or start over with `after review --new` |
| Checkout changed                              | `after diff`, then `after review` (`--new` if a review is saved)    |
| A pin needs another look                      | `after review`; accept only when offered and supported              |
| No run for this pair, but an earlier pair ran | Bare `after run` prepares a rerun                                   |
| Otherwise                                     | `after review` and `after diff`                                     |

A saved review on another pair appears in a `Saved review` row. Bare `after inspect` summarizes the newest capture and performs the same freshness check; `--stored` skips it. Bare `after compare` compares the newest receipt for that pair and may persist a comparison, but never executes project code. Bare `after export` emits JSON for the newest comparison associated with the pair. Bare `after pin` lists computed heads without choosing one; it may suggest pinning an unpinned observation from the newest run. `after log` lists newest events first; readable output shortens IDs, reports the total and suggests a larger `-n` when more events exist, while JSON retains record and snapshot IDs. Corrupt records or reached store bounds fail instead of returning a partial history. Every readable result names resolved defaults and offers only commands that exist. Suggested IDs use the shortest unique prefix of at least eight hex characters; suggestions retain explicit project/config flags and shell-quote values.

## Capture, import and review

`capture` compares HEAD with the working tree by default; `--staged` compares HEAD with the index. `--base REF [--target REF]` captures a merge-base comparison. Untracked files are excluded unless selected explicitly. Capture stores immutable snapshots plus a capture event containing time, mode, snapshot IDs and selected untracked paths. Capturing unchanged content reuses snapshot IDs but creates another event. Capture times come from events, never file modification times. Empty captures are recorded too; output suggests a merge-base comparison for committed branch changes or `--include-untracked` for excluded files. See [CAPTURE.md](CAPTURE.md).

`import` accepts `FILE`, `-`, or omitted input when stdin is piped. Omitted terminal input exits 2 without reading. Input is bounded to 8 MiB. `--format auto` (default) recognizes leading XML markup or a JSON object; `--format junit` and `--format go-test-json` select explicitly. Unrecognized input fails with a format fix instead of guessing from filenames. Empty auto input or a stream with no Go test JSON events exits 2 and stores nothing; empty JUnit suites are stored as incomplete with no test outcomes. `--producer` and `--captured-at` are optional caller claims, not authenticated provenance. Without `--snapshot`, import captures the working tree using capture policy and binds the report to its candidate; untracked files remain excluded. A failed capture names its reason and suggests `--snapshot ID`. A binding does not prove where tests ran. When untracked files are excluded, readable output warns that tests may have used them. Imported outcomes are `reported`, producer `importer`, applicability unknown, execution `not_run`, comparison `not_compared`—not runner observations. `--offset` and `--limit` page report cards (defaults 0 and 128; limit 1–256). See [GO-REPORTS.md](GO-REPORTS.md) and [JUNIT-REPORTS.md](JUNIT-REPORTS.md).

`review` needs a terminal on stdin and stderr. With no saved session, it captures and opens the pair; with a session, it resumes that exact pair and mode while capturing in the background. A changed capture stays pending until `u`; choose the original baseline (default) or last-inspected comparison and give a reason. `c` recaptures with the saved flags. `--new` or changed capture flags captures afresh and replaces only the UI session, names the replaced pair on stderr, and keeps old evidence and pins. An explicit pin ID remains on that revision. Next blocks use bare `after review` for the newest or saved pair and explicit pairs for older captures. Explicit IDs/pairs open stored records without reading or changing the saved session; `review ID` and `review BASE CANDIDATE [EVIDENCE …]` accept up to 32 evidence IDs. The session `.after/session.json` is atomic private UI state: pair, mode, original baseline, pin revisions and capture flags; it contains no source bytes and is not evidence.

An empty fresh review exits 0 without opening the TUI. It says whether the worktree/index matches HEAD or a merge-base comparison is empty, and can suggest `--base` when the branch is ahead or `--include-untracked` for excluded files. Default-branch discovery checks `refs/remotes/origin/HEAD`, then local `main`, then `master`; when HEAD is ahead, guidance includes the commit count. It does not fetch or choose the comparison automatically. Up to three excluded paths are shown. Unknown or unsupported inventory remains reviewable. `--import-file FILE [--producer TEXT]` is read only after the TUI's explicit `i` import action. A fresh `--new` review opens even when its capture is empty. Evidence discovery loads up to 32 records, prioritizing pins needing review, and reports omissions. Legacy original-base sessions infer the baseline from the saved pair. TUI output and the saved-review message go to stderr; stdout stays empty except for `--json`, which prints the session after the terminal is restored. Without a terminal, bare `after review` exits 2 and points to `after status --json`; inspect stored pairs with `after inspect BASE CANDIDATE --json`. See [TUI.md](TUI.md) and [REVIEW.md](REVIEW.md).

## Diff

`after diff` prints the current checkout change (HEAD versus working tree) like `git diff HEAD`; capture flags select staged, merge-base or untracked inputs. It stores nothing, creates no `.after/`, and does not inherit flags from a prior capture or saved review. If a live read fails, it prints no patch, exits 1 and suggests `after diff --stored`; it never falls back to an older patch.

`after diff --stored` prints the newest stored capture's patch; `after diff BASE CANDIDATE` accepts any stored snapshot pair. Capture flags cannot be combined with either. AFTER uses a shared captured patch when available; otherwise a bounded pure-Go diff is computed from captured source blobs. The computed diff is a display fallback, not Git's captured patch.

Stdout contains only patch lines. Stderr names the comparison and origin, then lists uncovered, excluded, unsupported or unknown inventory and limits. `internal/terminal` makes controls, format characters and invalid UTF-8 visible; tabs remain tabs in pipes and expand on terminals. Terminal colors are disabled by `NO_COLOR` or `TERM=dumb`; pipes are never colored. If sanitization changes bytes, stderr points to `--raw`. Safe and raw modes stream the full selected patch without a line cutoff; stored/computed byte bounds still apply.

`--raw` writes exact bytes to stdout with no sanitization, color or summary, and is refused with exit 2 when stdout is a terminal. Redirect it to a file or pipe. For a current change or ordinary captured pair, the generated patch can be checked with `git apply --check`; computed diffs are their generated bytes, not a captured Git patch. `--stat` prints per-file changed-line counts and a total (`Bin` for binary files). `--raw` and `--stat` cannot be combined. There is no pager; use `after review` for an interactive diff. `diff` rejects `--json`.

```sh
after diff > change.patch
after diff --raw > exact.patch
git apply --check exact.patch
after diff --stored --stat
after diff BASE CANDIDATE --stat
```

## Short IDs and pin heads

Stored-ID arguments accept any unique prefix of at least four hex characters, with or without `sha256:`, in any case. For example, `after inspect A750186B` and `after inspect sha256:a750186b` resolve the same record when unique. Git refs and approval digests are not ordinary ID prefixes; `--approve` requires the full lowercase `sha256:` plus 64 hex characters.

Resolution checks only kinds valid at that position:

| Position                                             | Kinds                                                         |
| ---------------------------------------------------- | ------------------------------------------------------------- |
| `run`, pair positions in `inspect`, `review`, `diff` | snapshots                                                     |
| `compare`, pin creation, `pin --attach`              | receipts                                                      |
| bare `pin ID`                                        | receipt or pin revision                                       |
| pin decisions                                        | pin revisions; `--select` also takes a snapshot               |
| `inspect` / `export`                                 | snapshots, receipts, comparisons, reports, artifacts or plans |
| `review ID`                                          | snapshots, receipts, comparisons, reports or pins             |
| trailing `review` IDs                                | comparisons, receipts, reports or pins                        |
| `import --snapshot`                                  | snapshot                                                      |

A missing or ambiguous prefix exits 2; a missing-ID diagnostic names searched kinds and how to create a record. Ambiguity lists at most ten short IDs, kinds and sanitized one-line descriptions. Lookup fails rather than choosing from a partial namespace: at most 10,000 directory entries and 32 MiB of matching object data. A full valid digest with no record opens an `UNAVAILABLE` Card in `inspect`, preserving the exact ID.

Pin heads are calculated from immutable histories, including forks; there is no stored latest pointer. `pin OLD_REVISION` opens exactly that revision and names newer descendant heads without selecting one. Head lookup is bounded to 512 revisions and 16 MiB of pin records; when it exceeds those bounds, the pin remains viewable but heads are unavailable. `--approve` is never shortened in readable suggestions.

## Pins and expectations

Create a pin with `after pin [RECEIPT] --expectation TEXT [--scope finite_example|human_intent] [--reason TEXT]`. Without a receipt it uses the newest run of the newest capture pair. Scope defaults to `finite_example`; a receipt that cannot support it suggests `human_intent`. Expectations are limited to 4096 bytes.

On a terminal, `after pin RECEIPT` without `--expectation` shows the scope, basis receipt and snapshot pair, then numbered finite-case suggestions. Enter a number to choose one or type text verbatim. Empty line or EOF creates nothing. Without a terminal, it exits 2 without reading stdin and prints a runnable `--expectation` example. `--scope` and `--reason` override the defaults; reasons are stored verbatim.

`after pin PIN` opens that revision read-only and names its selected original-base or last-inspected pair. Make one explicit decision at a time: `--select SNAPSHOT [--mode original_base|last_inspected]`, `--attach RECEIPT`, or `--accept`. Mode defaults to `original_base`; reason is optional. Default reasons are `Pinned from the command line`, `Selected from the command line`, `Attached from the command line` or `Accepted from the command line`; supplied reasons are stored verbatim. Expectations and reasons are each bounded to 4096 bytes. Each decision creates a new revision; use its ID next. Attaching a receipt does not accept the pin. Pins never execute code or grant run permission. See [REVIEW.md](REVIEW.md) for reuse rules and broader-intent limits.

## Run and exact consent

`after run` prepares the built-in payment plan for the newest capture, or an explicitly selected version-1 `http-service` or `command` definition with `--definition FILE`; an explicit `BASE CANDIDATE` pair also works. Preparation does not execute project code. The readable preview shows the resolved snapshots, selected definition bytes/digest, exact consent summary, plan size and full authorization digest. A definition inside the project that differs between the captured snapshots is disclosed as a changed oracle; neither snapshot selects a replacement. A plan is stored immutably in `.after/` with mode `0600`; `inspect PLAN` shows its consent summary and sanitized plan, and JSON includes the exact bytes as base64.

HTTP-service scenarios require complete captures and a bounded source tree (255 files / 8 MiB); they retain the v1 fd3 readiness and observer contract. Command scenarios support direct argv only, a digest-pinned image/platform, optional build argv, 1–4 cases, 1–5 repetitions, bounded seconds/output/preparation, per-case argv, base64 `stdin_base64` bytes, a fixed `environment` array and optional additive `input_files` with base64 `content_base64`. Input paths cannot traverse, overlap one another, collide with snapshots or reserved runtime files. Command stdout/stderr may each be exact-byte `text` (including invalid UTF-8) or definition-declared structural `json`; invalid JSON is incomparable. Every case/side/repetition gets fresh offline sandbox state; build and target run in that same container, without shell, image pulls, TTY, output-tree comparison, host fallback or an observer container. Build output is discarded. Exit statuses 125–255 are incomplete (helper/build/exec failure or possible signal); intentional application exits in that range are unsupported. Preparation fails with an actionable reason before saving a plan if inputs exceed bounds. `--plan-out FILE` creates another private `0600` file without overwriting; `--plan-file FILE` reconstructs it. Both paths rebuild from immutable snapshots and compare exact bytes. Each plan has a random request ID, so preparing again does not authorize an earlier plan.

On a terminal, type `yes` to authorize exactly the displayed plan bytes. The consent summary is decoded from those bytes; if decoding fails, the error is shown but the digest still binds the exact plan. A non-TTY preview does not read stdin, reports `authorization_required` and exits 3. `--interactive false` disables the prompt but grants no authority. `--approve` accepts only the full digest from the exact preview; a mismatch exits 3. Approval is not a setting and cannot be a prefix.

Only after approval may the runner contact an explicitly configured local Docker endpoint. Set both an absolute trusted CLI and a local Unix socket:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \\
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \\
./bin/after run --plan-file .after/approved-preview.json \\
  --approve sha256:<full-digest> --project /work/payment
```

No help, config, capture, import, inspection, comparison, pin, review, preview or setup diagnostic runs project code, contacts Docker, builds the project on the host or pulls an image. There is no Docker-context or host-process fallback. Provision the pinned target and Go compiler images separately; see [SANDBOX.md](SANDBOX.md) and [RUNNER.md](RUNNER.md). A TTY run reports elapsed time on stderr after one second, without a percentage.

## Configuration

Configuration is loaded only when needed. Precedence is explicit flags > `AFTER_*` environment > YAML > defaults. Empty/whitespace strings and YAML `null` are unset; explicit `false` and zero remain values. An explicit `--config FILE` or `AFTER_CONFIG` must exist. Otherwise the optional file is `${XDG_CONFIG_HOME}/after/config.yaml`, falling back to `~/.config/after/config.yaml`. Paths are relative to the invocation directory.

| YAML key        | Environment           | Flag              | Default              | Bounds / meaning                                                              |
| --------------- | --------------------- | ----------------- | -------------------- | ----------------------------------------------------------------------------- |
| `project`       | `AFTER_PROJECT`       | `--project`       | invocation directory | selected path inside a checkout; printed path hidden                          |
| `repetitions`   | `AFTER_REPETITIONS`   | `--repetitions`   | 1                    | 1–5 paired repetitions                                                        |
| `run_seconds`   | `AFTER_RUN_SECONDS`   | `--run-seconds`   | 180                  | 1–300 seconds per sandbox plan                                                |
| `output_bytes`  | `AFTER_OUTPUT_BYTES`  | `--output-bytes`  | 65536                | 1–1048576 bytes per container                                                 |
| `interactive`   | `AFTER_INTERACTIVE`   | `--interactive`   | true                 | allow a terminal prompt only; never authorizes execution                      |
| `raw_diff`      | `AFTER_RAW_DIFF`      | `--raw-diff`      | true                 | include patch bytes in inspect/export                                         |
| `diff_bytes`    | `AFTER_DIFF_BYTES`    | `--diff-bytes`    | 65536                | 0–65536 bytes per raw-diff page; 0 suppresses patch bytes but keeps inventory |
| `docker_binary` | `AFTER_DOCKER_BINARY` | `--docker-binary` | unset                | absolute trusted CLI path                                                     |
| `docker_host`   | `AFTER_DOCKER_HOST`   | `--docker-host`   | unset                | local `unix:///` socket                                                       |

Docker CLI and socket must be configured together. There is no authorization setting. `after config` shows effective values and their source (`flag`, `env`, `file` or `default`), but hides project/config paths and Docker endpoint values. It only inspects configuration and filesystem metadata. If it finds a Docker CLI on `PATH` and exactly one existing local socket, it may offer a shell-quoted export line as a suggestion; it does not run or contact either.

YAML is limited to 64 KiB, one mapping document, regular non-symlink files, scalar values and the keys in the table. Duplicate/unknown keys, anchors/aliases, malformed documents, invalid types/ranges and missing explicit files fail with a setting/source diagnostic.

## Output, paging and exit status

On a terminal, readable output uses the AFTER theme unless `NO_COLOR` is set or `TERM=dumb`; pipes never contain color. Untrusted paths, report text, expectations, producers, patch bytes and errors pass through the sanitizer. Terminal rows may clip; full IDs remain in the `IDs` section of inspect results. JSON escapes control characters, but consumers must still sanitize untrusted values when rendering them.

`--json` emits one object with `schema_version: 1`, `kind` and `data`, capped at 16 MiB; existing capture and import shapes remain unchanged. Raw patch and artifact bytes are base64. Inventory, report cards and artifacts are paged. Inspect/export support `--diff-offset`, `--diff-size`, `--inventory-offset`, `--inventory-limit`, `--card-offset`, `--card-limit`, `--artifact-offset` and `--artifact-size`; page defaults are 0/65536 for byte offsets/sizes and 0/128 for lists, with list limits 1–256. Artifact pages are at most 65536 bytes and report `next`, `total` and `more`; complete valid JSON fitting one page also has a precision-preserving `document` field. Inspect Cards use the TUI's ordered evidence sections; unsupported shapes retain raw bytes and a limitation. General artifacts use a bounded text/hex viewer. Artifact bytes alone do not establish a producer; inspect the referring receipt for provenance. Imported passes/failures retain reported/importer/unknown-applicability/not-run/not-compared state.

| Exit | Meaning                                                                             |
| ---- | ----------------------------------------------------------------------------------- |
| 0    | Command completed; a complete comparison was equal                                  |
| 1    | Operational failure or incomplete/incomparable evidence                             |
| 2    | Invalid command, arguments, configuration, IDs or input                             |
| 3    | Run declined, lacks exact authorization or has a digest mismatch                    |
| 4    | Comparison finding: `different` or `unstable`; not an automatic regression judgment |

`run`, `compare`, `inspect` and `export` preserve JSON evidence on finding/incomplete outcomes. Capture/import that remain active for one second report elapsed time to terminal stderr; pipes receive no progress notice. `task test:cli` runs focused CLI/config/native-entry tests. `task command:check` verifies command definition/plan/comparison regressions without Docker; `task command:proof` runs the real capture/consent/run/compare/pin/reopen command fixture on the provisioned image. `task cli:proof` runs the real subprocess payment proof and requires the pinned image plus explicit Docker settings.

## Shell completion

`after completion [bash|zsh|fish]` prints a script for the named shell; with no argument it uses the basename of `$SHELL`. Unsupported shells exit 2 and list the supported names. Scripts complete commands, flags, `--scope` and `--mode` values, and stored IDs valid at that argument position. They offer the 50 newest records, newest first, as short IDs with sanitized one-line descriptions where supported. Bash Readline shows words only; Zsh and Fish show descriptions.

Legacy snapshots, plans and generic artifacts without event times follow timestamped records in stable short-ID order; file mtimes are not used. Completion opens `.after/` read-only, returns no ID candidates when no store exists, creates no storage and never captures, imports or runs code. Shell adapters pass words as arguments; descriptions are sanitized before crossing the shell interface.

Install Bash in `~/.bashrc`:

```sh
eval "$(after completion bash)"
```

Install Zsh before `compinit`:

```sh
mkdir -p ~/.zfunc
after completion zsh > ~/.zfunc/_after
fpath=(~/.zfunc $fpath)
autoload -Uz compinit && compinit
```

Install Fish:

```sh
mkdir -p ~/.config/fish/completions
after completion fish > ~/.config/fish/completions/after.fish
```

For capture and storage contracts see [CAPTURE.md](CAPTURE.md) and [STORAGE.md](STORAGE.md); for the versioned records see [SCHEMA.md](SCHEMA.md).
