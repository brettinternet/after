# Command-line design

This page records the interaction decisions for AFTER's CLI. [CLI.md](CLI.md) and `after COMMAND --help` are the runtime reference; a target described here is not available until it appears in runtime help.

The CLI is a local interface to the Go evidence engine, not a general application runner. It has no model, account, GitHub or editor dependency. The product invariants in [AGENTS.md](../AGENTS.md) and [IMPLEMENTATION.md](IMPLEMENTATION.md) apply.

## Principles

1. Make useful, read-only choices when arguments are omitted, and name each resolved choice.
2. Print readable results by default; keep the versioned JSON contract behind `--json`.
3. Accept short unique IDs for records, but never shorten an execution-approval digest.
4. Use `BASE CANDIDATE` for snapshot pairs. `--base` means a Git ref only on capture-style commands.
5. End readable results with up to three runnable next actions; errors say how to fix the problem.
6. Defaults never authorize execution, include untracked files, or choose a pin revision.

## Output

A readable result starts with what happened, then shows aligned rows or a list, then a `Next` block. Diagnostics go to stderr. On a terminal, use the shared TUI theme; disable color for pipes, `NO_COLOR` or `TERM=dumb`. Sanitize untrusted paths, report text, expectations, producers and errors before layout. Clip rows only on terminals. Keep full IDs in JSON and the inspect `IDs` section. Runtime examples and exit codes are in [CLI.md](CLI.md).

## IDs and defaults

Stored IDs accept a unique prefix of at least four hex characters, with or without `sha256:`, in any case. Resolve only record types valid at that argument position. Missing or ambiguous IDs exit 2; ambiguity lists up to ten sanitized matches. A full digest with no stored record opens an `UNAVAILABLE` Card in `inspect` rather than guessing its type. Lookup fails rather than using a partial namespace: at most 10,000 directory entries and 32 MiB of matching object data.

`--approve` is different: require the full lowercase `sha256:` digest and all 64 hex characters printed by the exact preview. It is never a prefix.

| Omitted value  | Resolution                                  |
| -------------- | ------------------------------------------- |
| Snapshot pair  | Newest capture record's base and candidate  |
| Run receipt    | Newest run for that pair                    |
| Comparison     | Newest comparison for that pair             |
| Import binding | Capture the working tree at import time     |
| Pin revision   | Never default; require an explicit revision |

Readable output names resolved IDs with short prefixes; JSON retains full IDs. Each capture creates an immutable event with its time, mode, pair and selected untracked paths. Re-capturing unchanged content reuses snapshot IDs but creates another event. No file mtime establishes capture time. Pin heads are derived from immutable histories, including forks; there is no mutable latest pointer. An explicit old revision opens that revision. Head lookup is bounded to 512 revisions and 16 MiB of pin records.

## status

Bare `after` and `after status` summarize the newest stored capture, not a new capture. They show its pair, stored run/comparison, relevant pin heads and reports, plus whether the checkout still matches. The check is read-only and compares content hashes in the recorded capture scope. `--stored` skips it. A saved review is separate UI state: status names it when it is on another pair, and plain `after review` resumes it rather than silently switching to the newest capture.

`Next` follows this precedence: no capture; saved review on another pair; changed checkout; pin needing another look; no run for this pair but prior runs exist; otherwise review/diff. A changed checkout points to the live `after diff`; when a review is saved, `after review --new` starts the current change. The complete state table and freshness values are in [CLI.md](CLI.md#bare-commands-and-stored-defaults).

## review

`after review` is the terminal entry point. With no saved session, capture and open the current change. With a session, reopen that exact pair/mode and capture in the background; keep a changed capture pending until the reviewer chooses how to compare it. `--new` is the explicit action that replaces the saved UI session. Stored evidence and pins remain in the store. Explicit IDs/pairs open stored records without updating the session. A fresh empty capture opens no TUI and exits 0; it may suggest a default-branch comparison or explicitly selected untracked paths, but never chooses one automatically. See [TUI-DESIGN.md#launch-and-resume](TUI-DESIGN.md#launch-and-resume) for the screen contract.

## capture and evidence

Capture HEAD versus the working tree by default, or the index/merge base when requested. Exclude untracked files unless named. Root discovery checks `.git` entries in the selected path's ancestors; it runs no Git command. Writers create private `.after/` only after checkout resolution and add `.after/.gitignore` containing `*`; read-only commands do not create the store. Capture errors use fixed reason/fix text, not repository content or absolute paths.

A `go test -json` import is reported evidence, not a runner observation. A producer and timestamp are caller claims; binding a report to a snapshot does not prove that tests ran on it. Comparison reads stored observations and creates a comparison record without executing code. Pins record a human expectation/decision separately from evidence; selecting or attaching a receipt is not acceptance.

## diff

Bare `after diff` reads HEAD versus the current working tree and stores nothing. Capture flags can select staged, merge-base or named untracked inputs. `--stored` prints the newest captured patch; `BASE CANDIDATE` selects any stored pair. If a live read fails, print no patch and do not fall back to stored data. Prefer a captured patch; otherwise a bounded pure-Go diff over captured source blobs is a display fallback, not the original Git patch.

Stdout is patch bytes only. Stderr names the compared states and patch origin, then lists uncovered inventory and limits. Safe output makes controls and invalid UTF-8 visible; `--raw` emits exact bytes only to a file or pipe. `--stat` reports per-file counts. Full command behavior is in [CLI.md#diff](CLI.md#diff).

## run

`after run` prepares the fixed offline payment experiment; it does not execute until explicit consent. Show the exact plan's consent summary, size and digest. A terminal operator types `yes`; a non-TTY preview exits 3 and never reads stdin. `--approve` must match the full digest. Plans are private and immutable; reconstruct from captured inputs and compare exact bytes before execution. A fresh plan has a random request ID. `--interactive false` is not consent.

Only after approval may the runner contact its explicitly configured local Docker CLI and Unix socket. There is no host fallback, context selection, image pull or generic application adapter. Details and setup constraints are in [CLI.md#run-and-exact-consent](CLI.md#run-and-exact-consent), [RUNNER.md](RUNNER.md) and [SANDBOX.md](SANDBOX.md).

## Help and completion

`after --help` lists implemented commands. `after COMMAND --help` shows that command's options and real defaults from the runtime configuration package. Keep Docker and run limits on `run`, and inspection paging flags on `inspect`/`export`. Unknown commands/flags suggest a close match within two edits; missing required input gives a runnable example.

`after completion [bash|zsh|fish]` generates shell scripts. Complete only IDs valid for the current argument, newest first, with sanitized descriptions where supported. Completion reads the store read-only and never captures, imports or runs project code. Bash Readline has no description API; Zsh and Fish display descriptions. See [CLI.md#shell-completion](CLI.md#shell-completion) for installation.

## Backlog map

These task names identify the CLI decisions' implementation scope; [Backlog.md](../backlog/tasks) remains authoritative.

| Task     | Name                                                                                     |
| -------- | ---------------------------------------------------------------------------------------- |
| AFTER-24 | Start and resume reviews with a plain `after review`                                     |
| AFTER-34 | Resolve the checkout root, keep `.after/` out of Git, and report specific capture errors |
| AFTER-35 | Print readable results by default and keep the versioned JSON behind `--json`            |
| AFTER-36 | Accept short ID prefixes, record every capture, and list pin heads                       |
| AFTER-37 | Make bare commands useful with status, log, resolved defaults and `Next` blocks          |
| AFTER-38 | Use one grammar, honest help, and errors that say how to fix them                        |
| AFTER-39 | Import go test JSON from a pipe, with an optional producer and binding at import         |
| AFTER-40 | Show a readable run consent, store plans privately, and guide Docker setup               |
| AFTER-41 | Print the captured change with after diff                                                |
| AFTER-42 | Complete commands, flags, and record IDs in the shell                                    |
| AFTER-43 | Start a new review with after review --new and show the saved review in status           |
| AFTER-44 | Point reviewers to their branch changes when the working tree matches HEAD               |
| AFTER-45 | Ask for a pin's expectation on a terminal                                                |
| AFTER-46 | Make bare CLI commands current, quiet, and directly actionable                           |

The POC stays deliberately narrow: no universal adapter, semantic dependency graph, GitHub synchronization, browser app, production replay, arbitrary URL execution, autonomous repair or second agent runtime. Models cannot manufacture observations, assign evidence badges or authorize execution. See [IMPLEMENTATION.md](IMPLEMENTATION.md) for scope and trust boundaries.
