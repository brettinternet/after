# Headless CLI

This page documents current behavior. The broader CLI target, including commands
that are not implemented yet, is specified in [CLI-DESIGN.md](CLI-DESIGN.md).

`after` is the CLI entry point over AFTER's capture, private store, Go test report, raw-diff, frozen runner, comparison and pin APIs. It has no model, account, GitHub, or editor dependency. `--help` and `--version` are side-effect free. Data commands print concise readable text by default; pass `--json` for the unchanged version-1 `schema_version` / `kind` / `data` envelope. `export` always prints JSON. Diagnostics use stderr. `after review` captures and opens or resumes the [captured evidence browser](TUI.md) on a terminal, without execution on open. Explicit IDs and pairs remain read-only and do not consult saved session state. JSON strings escape terminal control characters. Consumers must still sanitize untrusted values when rendering them.

See [packaging, the repeatable demo and recovery](DEMO.md) for native distributions and a prepared-checkout walkthrough. Build with `mise exec -- task build`, then run commands from any directory with a selected project:

```sh
./bin/after capture --project "/work/payment" --include-untracked "fixtures/new case.json"
./bin/after inspect BASE_ID CANDIDATE_ID --project "/work/payment"
go test -json ./... | ./bin/after import --project "/work/payment"
./bin/after import "go test output.jsonl" --producer "go1.27.1 on linux/amd64" --snapshot CANDIDATE_ID --project "/work/payment"
./bin/after inspect REPORT_ARTIFACT_ID --project "/work/payment"
./bin/after compare RECEIPT_ID --project "/work/payment"
./bin/after export COMPARISON_ID --project "/work/payment"
```

## Bare commands and stored defaults

`after` and `after status` show a read-only summary of the newest stored capture,
its pair, any stored run/comparison, applicable pin heads, and reports bound to the
candidate. They do not capture, import, run, compare, or create `.after/`; a
checkout with no capture suggests `after capture`. Outside Git, bare `after` prints
short help, while `after status` reports the checkout requirement. The JSON form
`after status --json` returns these same facts with full IDs. A saved review on a
different pair is named in a `Saved review` row; its resume (`after review`) and
start-over (`after review --new`) suggestions take precedence over pin/run actions.

`after log [-n N]` lists the newest 20 stored capture events, run receipts,
imported reports, and pin revision events; `-n` accepts 1–10000. Rows are newest
first. Its JSON rows retain full record and snapshot IDs, and a shortened readable
list reports the total and suggests a larger `-n`. Corrupt records or reached store
bounds fail visibly rather than returning a falsely complete history.

## Diff

`after diff` streams the newest capture's patch; `after diff BASE CANDIDATE` accepts
any two stored snapshot IDs or unique prefixes. A shared captured patch is used
when available; otherwise the bounded pure-Go computed diff uses only captured
source blobs. A computed diff is a display fallback, not Git's captured patch.

Stdout contains only patch lines. Stderr names the full snapshot pair and patch
origin, lists every unknown, excluded, unsupported, or otherwise limited inventory
path, and reports capture/computation limits. Untrusted patch bytes pass through
`internal/terminal`: controls, format characters and invalid UTF-8 are made visible;
tabs remain tabs in a pipe and expand on a terminal. Terminal output uses the Diff
view's fixed colors unless `NO_COLOR` is set or `TERM=dumb`; pipes are never colored.
When sanitizing changes a byte, stderr warns that exact bytes are available with
`--raw`. Both safe and raw output stream the entire selected patch without a
line-count cutoff; storage and computed-diff byte bounds still apply.

`after diff --raw [BASE CANDIDATE]` writes the selected patch bytes exactly, with no
sanitizing, color, or summary on stdout. It is refused with exit 2 when stdout is a
terminal; redirect to a file or pipe. For an ordinary captured pair these bytes are
the stored patch and `git apply` can reproduce the captured candidate. A computed
pair's raw bytes are exactly its generated computed diff, which is not claimed to
be the captured Git patch. `--raw` and `--stat` cannot be combined. `--stat`
prints the same readable capture summary rows as `after capture`, while stderr
still reports pair, origin, uncovered inventory and limits. There is no pager.

```sh
after diff > change.patch
after diff --raw > exact.patch
git apply --check exact.patch
after diff BASE CANDIDATE --stat
```

Bare `after inspect` summarizes the newest capture record's pair and says which
capture it resolved. Bare `after compare` compares the newest stored run receipt
for that pair; comparison reads stored observations and may persist a comparison,
but never captures or executes project code. Bare `after export` always emits JSON
for the newest stored comparison associated with that capture pair. These results
include a `using` object with the full resolved capture, snapshot, receipt, and/or
comparison IDs. If a required record is missing, the diagnostic names the missing
record and the command that creates it.

Bare `after pin` lists computed pin heads, including forks, without choosing one.
`after pin --expectation TEXT` may omit the receipt to use the newest run of the
newest capture pair; this resolves a receipt, never a pin revision. Every decision
still requires an explicit pin revision ID. Pin history corruption or the 512-head /
16 MiB lookup limit is reported as unavailable rather than as a partial head list.

Every readable result ends with a `Next` block containing at most three available,
syntax-ready commands. `after run` without arguments prepares the newest capture;
when status identifies a rerun as the next action, its suggestion is the bare
`after run` command. Explicit `--project` and `--config` values are retained in
suggestions and shell-quoted when needed. A safe suggestion can be run as written;
it does not authorize execution.

## Grammar, help and diagnostics

Snapshot pairs use `BASE CANDIDATE` in `inspect`, `review`, and `run`. `--base`
is only a Git ref on `capture`; `--target` is optional and defaults to `HEAD`.
Pin inspection and mutations all use `pin`: `pin PIN`, `pin PIN --accept`,
`pin PIN --attach RECEIPT`, and `pin PIN --select SNAPSHOT [--mode MODE]`.
Creation uses `pin RECEIPT --expectation TEXT`; scope defaults to
`finite_example`, and reason is optional.

`after --help` and `after help` list only commands available in this build. Run
`after COMMAND --help` for that command's usage and options. Global options are
`--project`, `--config`, and `--json`; Docker and run-limit options appear only on
`run`, and inspection paging/raw-diff options appear only on `inspect` and
`export`. Printed run/inspection defaults come from the same configuration
package as runtime defaults. Unknown commands and flags suggest a unique closest
match within two edits. Removed forms such as `inspect CANDIDATE --base BASE` and
`review PIN --accept` exit 2 with the replacement syntax. Diagnostics sanitize
input and give a corrective action; missing arguments include a runnable example.

Flags may appear before or after positional arguments. Capture defaults to HEAD versus the working tree; `--staged` selects HEAD versus the index. `--base REF` selects a merge-base capture, and `--target REF` optionally chooses its target (default `HEAD`). `--base` is a Git ref on `capture`; snapshot pairs everywhere else are positional `BASE CANDIDATE` IDs. `--include-untracked` accepts repeated exact paths only. Capture and import create private `.after/` storage; inspection/export open it read-only. Import accepts `FILE`, `-`, or omitted input when stdin is piped; omitted terminal input exits 2 without reading and shows the file and pipe forms. `--producer` is optional: when omitted, no producer claim is stored. A supplied producer and `--captured-at RFC3339` are caller claims, not authenticated provenance. Without `--snapshot`, import captures the working tree using the `after capture` policy, writes a capture event, and binds the report to its candidate snapshot. Untracked files stay excluded; readable output warns that tests may have used excluded files. A binding is a caller claim, not proof the report's tests ran on that capture; if the default capture fails, import reports the capture reason and suggests `--snapshot ID`. The ordinary diff and all excluded/unsupported inventory entries remain available without imported or observed evidence. `inspect BASE_ID CANDIDATE_ID` returns bounded inventory pages and a base64 raw-patch page; `--diff-offset`, `--diff-size`, and `--inventory-offset`/`--inventory-limit` page the data. Imported report cards use `--card-offset`/`--card-limit`.

Each successful capture also writes an immutable capture event with its time, mode,
snapshot IDs and selected untracked paths. Recapturing unchanged content leaves
snapshot IDs unchanged but records a new event. Readable snapshot inspection shows
recorded capture times; legacy snapshots without an event say the time is unavailable.
History lookup limits are shown when reached. No file modification time is used.
The existing explicit `--json` snapshot and capture response shapes remain unchanged.

## Short IDs and pin heads

Every stored-ID argument accepts a unique prefix of at least four hex characters,
with or without `sha256:`, in any case. For example, `after inspect A750186B` and
`after inspect sha256:a750186b` select the same record when unique. JSON retains
full IDs. Capture's `--base`/`--target` are Git references, not stored IDs.

Resolution searches only kinds valid for that argument: run and positional
inspect/review pairs search snapshots; compare and pin creation search receipts;
`pin --attach` searches receipts, and `pin PIN` plus pin decisions search pin
revisions. Inspect/export search snapshots, receipts, comparisons, reports,
artifacts and stored execution plans. Trailing review IDs search comparisons,
receipts, reports and pins.
Missing prefixes exit 2 naming the searched kinds; ambiguity exits 2 with at most
ten short IDs, kinds and sanitized one-line summaries. Use more characters to
disambiguate. A full, syntactically valid `sha256:` digest that has no stored
record opens an `UNAVAILABLE` Card with the exact ID instead of guessing a type.
Other no-match diagnostics tell you to verify the ID or create the record with
an available capture, run, import or pin command.
Lookup fails explicitly on unsafe storage or exceeded scan/read limits; it never
chooses from a partial namespace (10,000 directory entries, 32 MiB of matching
object data per lookup).

Pin heads are computed from immutable histories, including every fork, without a
stored latest pointer. `pin OLD_REVISION` opens exactly that revision; readable
output names its newer descendant heads without selecting them. Head
lookup is bounded to 512 revisions and 16 MiB of pin records. When unavailable,
readable output says so while retaining the requested revision. JSON is unchanged.

`--approve` is deliberately **not** a prefix: it requires the full lowercase
`sha256:` digest with all 64 hex characters, copied from the exact preview.

## Checkout root and private storage

Project commands resolve the checkout root by checking for a `.git` directory or worktree `.git` file in the selected directory and its parents. The default selected directory is the invocation directory; `--project`, `AFTER_PROJECT`, or the YAML `project` setting can select another path inside a checkout. Resolution is filesystem-only and runs no Git command. It is shared by capture, import, inspect, compare, export, run, pin and review. The capture API itself still requires its caller to pass the resolved repository root explicitly.

Outside a checkout, project commands exit 2 with exactly `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR` and create no `.after/` directory or lock file. `help`, `version` and `config` do not require a checkout; read-only project commands never create storage. Writable store opens create `.after/.gitignore` with `*` using the store's durable, no-overwrite publication path, including when an older store has no ignore file yet. This ignores only private store contents and does not edit the checkout's `.gitignore` or `.git/info/exclude`.

Capture failures retain exit 1 and use fixed allowlisted reason/fix text, for example `after: capture failed: unmerged index is unsupported — resolve the index conflicts, then retry capture`. Repository content and absolute project paths are never interpolated into these diagnostics.

Stored artifacts (including observer response/effect channels referenced by receipts) can also be inspected or exported by content ID. `--artifact-offset` and `--artifact-size` return exact base64 byte pages, up to 65536 bytes, with `next`, `total`, and `more`. A complete valid JSON artifact fitting one page also has a `document` field preserving numeric precision. Artifact content alone does not establish its producer or evidence state; inspect its referring receipt for provenance.

## Review launch and resume

`after review` requires a terminal on stdin and stderr. With no saved review it captures the working tree, then opens the pair. `--staged`, `--base REF [--target REF]`, and repeated `--include-untracked PATH` use the same safe Git capture policy as `after capture`. Untracked files remain excluded unless selected. A capture that finds no changed or unknown paths reports that nothing is open. `--import-file FILE [--producer TEXT]` configures the TUI's explicit `i` action to import that file into the selected review; producer provenance is optional, and omission makes no producer claim.

A private, atomic `.after/session.json` stores the active snapshot pair, comparison mode, original baseline, selected pin revision IDs and capture flags; it contains no source bytes and is not evidence. A legacy original-base session safely infers its baseline from the saved pair. A later bare `after review` opens that exact pair and mode immediately and captures in the background. A differing capture stays pending until `u`; the TUI offers original-base (default) or last-inspected comparison and requires a reason. `c` reuses the saved flags. To move on to an unrelated change, run `after review --new` (optionally with `--staged`, `--base REF`, or other capture flags). It captures afresh, replaces only the saved UI session, and names the replaced pair on stderr. Old evidence and pins remain in the store and are still discovered. Changed capture flags also replace the saved review. Next blocks offer plain `after review` for the newest capture or saved pair, and explicit pairs for older captures; plain review always resumes an existing session rather than silently switching to the newest pair. Explicit `after review ID` or `after review BASE CANDIDATE [EVIDENCE ...]` opens stored records without reading or changing the saved review; an explicit pin revision stays on that revision. Terminal rendering and the saved-review message use stderr. Stdout is empty unless `--json`, which prints the versioned session object after the terminal is restored. Without a terminal, bare `after review` exits 2 and points to `after status --json`; explicit stored pairs can be inspected with `after inspect BASE CANDIDATE --json`.

Evidence discovery loads pin heads (including forks), the selected pair's newest runs and comparisons, and reports bound to the candidate. It loads at most 32 records, prioritizing pins that need another look, and the browser reports the omitted count. `after log` provides the separate bounded history view.

## Persistent expectations

`pin RECEIPT_ID --expectation TEXT [--scope finite_example|human_intent] [--reason TEXT]`
creates an immutable pin revision; `--scope` defaults to `finite_example`, and a
receipt that cannot support it suggests `--scope human_intent`. `pin PIN_ID` opens a
revision read-only using the TUI's shared Card renderer, followed by full IDs. Its
readable view names the selected `original base` or `last inspected` pair from the
latest immutable pin history event.
Receipt, comparison and report inspection use the same ordered Card parts and
bounded safe wrapping; `--json` retains the original record envelope. Decisions
use exactly one of `--select SNAPSHOT_ID [--mode original_base|last_inspected]`,
`--attach RECEIPT_ID`, or `--accept`; `--mode` defaults to `original_base`. `--reason` is optional; its command-line default is
stored verbatim in history. Each decision returns a new revision ID; use that ID
for the next operation. Selection reopens changed bindings without predicting
results. Receipt attachment never accepts the pin; human acceptance is a separate
action. No pin action executes code or grants run permission. See [the review workflow](REVIEW.md) for examples, exact
reuse rules, broader-intent limits, historical revisions and rerun authorization.

## Execution authorization

Bare `after run` prepares the payment-specific frozen plan for the newest stored capture; an explicit `BASE CANDIDATE` pair still works. Preparation does not execute project code. The readable preview names the resolved capture, displays consent rows decoded from the exact plan bytes and its byte size, stores the immutable plan in `.after/` with mode 0600, and prints its full authorization digest and exact command. Pass `--json` when a script must parse the digest or resolved IDs.

```sh
./bin/after run --project "/work/payment"
# Review the readable preview, then authorize only those stored bytes:
./bin/after run --approve sha256:<full-64-hex-digest> --project "/work/payment"
# The explicit external-file flow remains available:
./bin/after run BASE_ID CANDIDATE_ID --plan-out "/work/payment/.after/approved-preview.json"
./bin/after run --plan-file "/work/payment/.after/approved-preview.json" \
  --approve sha256:... --project "/work/payment"
```

The non-TTY preview returns status `authorization_required` and exit 3; it never reads stdin for consent. A plan is stored immutably in `.after/` (0600, never overwritten), and `after inspect PLAN` shows the shared strict consent summary followed by the indented, sanitized plan; `--json` includes base64 of the exact stored bytes. On a terminal, AFTER shows the summary and size on stderr and requires typing `yes`; any other answer runs nothing. If strict summary decoding fails, that failure is shown while consent still binds the exact preview digest. `interactive: false` disables the prompt but grants no authority. `--approve` requires the full digest: `after run --approve DIGEST` loads the matching stored plan, and `--plan-file FILE --approve DIGEST` continues to work. `--plan-out FILE` still creates an additional private (0600) file without overwriting an existing one. Both flows reconstruct against current immutable snapshots and compare exact bytes before execution. Any changed or edited plan is rejected. Each plan has a new random request ID; re-preparing is not equivalent to approving an old preview.

Only after exact approval may the runner contact the explicitly selected local Docker endpoint. Set both an absolute trusted CLI and a local Unix socket, for example:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
./bin/after run --plan-file "/work/payment/.after/approved-preview.json" \
  --approve sha256:... --project "/work/payment"
```

No Docker context, image pull, host execution, build, or project command is used by help, config display, capture, import, inspect, export, comparison, pin, review, preview, or setup diagnostics. Provision the pinned image separately as documented in [SANDBOX.md](SANDBOX.md). Missing endpoints or isolation produce an actionable setup problem or incomplete operational result; they never select a host fallback. Approved runs show elapsed time on terminal stderr after one second, with no percentage.

## Configuration

AFTER reads configuration only when a command needs it. The explicit `--config FILE` or `AFTER_CONFIG` path must exist; otherwise the optional discovered file is `${XDG_CONFIG_HOME}/after/config.yaml`, falling back to `~/.config/after/config.yaml`. Relative config/project paths are resolved from the invocation working directory. The discovered file may be absent. Configuration loading parses data only; it never runs repository code. `after config` reports effective values and each winning source (`flag`, `env`, `file`, or `default`), hides project/config paths and Docker endpoint values, and lists Docker setup problems with their configuration fixes.

Precedence is explicit flags > `AFTER_*` environment > YAML > defaults. Empty/whitespace strings and YAML `null` are unset and allow a lower layer to win. Explicit booleans and integers are values: `false` is not a default, and zero is not silently discarded. `diff_bytes: 0` is supported and suppresses patch bytes while retaining the change inventory; `repetitions: 0` is invalid.

| YAML key        | Environment           | Flag              | Type and default           | Bounds/meaning                                                         |
| --------------- | --------------------- | ----------------- | -------------------------- | ---------------------------------------------------------------------- |
| `project`       | `AFTER_PROJECT`       | `--project`       | path; invocation directory | selected checkout path; nearest `.git` ancestor is used; output hidden |
| `repetitions`   | `AFTER_REPETITIONS`   | `--repetitions`   | integer; `1`               | 1–5 paired repetitions                                                 |
| `run_seconds`   | `AFTER_RUN_SECONDS`   | `--run-seconds`   | integer; `180`             | 1–300 seconds per sandbox plan                                         |
| `output_bytes`  | `AFTER_OUTPUT_BYTES`  | `--output-bytes`  | integer; `65536`           | 1–1048576 bytes per container                                          |
| `interactive`   | `AFTER_INTERACTIVE`   | `--interactive`   | boolean; `true`            | permits a TTY prompt only; never authorizes execution                  |
| `raw_diff`      | `AFTER_RAW_DIFF`      | `--raw-diff`      | boolean; `true`            | include patch bytes in inspection output                               |
| `diff_bytes`    | `AFTER_DIFF_BYTES`    | `--diff-bytes`    | integer; `65536`           | 0–65536 bytes per raw-diff page                                        |
| `docker_binary` | `AFTER_DOCKER_BINARY` | `--docker-binary` | string; unset              | must be an absolute trusted CLI path when set                          |
| `docker_host`   | `AFTER_DOCKER_HOST`   | `--docker-host`   | string; unset              | must be a local `unix:///` socket when set                             |

Docker binary and host must be configured together. There is deliberately no consent/authorization setting: permission is an interactive action or the specific `--approve` digest. Runtime values remain hidden in the configuration table; explicitly labeled Docker CLI/socket suggestions are candidates, not selected endpoints. Setup probes only inspect configuration and filesystem metadata—they never run Docker or contact an endpoint. YAML is limited to 64 KiB, one mapping document, regular non-symlink files, scalar values, and the listed keys. Duplicate/unknown keys, anchors/aliases, malformed documents, invalid types/ranges, and missing explicit files fail with a setting/source diagnostic.

## Output and exit status

Readable output uses a leading result sentence and aligned rows or lists. On terminals, the AFTER theme styles output unless `NO_COLOR` is set or `TERM=dumb`; pipes never contain color sequences. Untrusted paths, report text, expectations, producers and errors pass through the terminal sanitizer. Rows clip only on a terminal, with full IDs retained in the `IDs` section of inspect results. Receipts, comparisons, pins and imported reports use the TUI's shared evidence Cards and ordered artifact sections; an unsupported shape retains its raw bytes with a limitation. General artifact inspection uses the bounded text/hex content viewer; use `after inspect ID --json` for exact base64 pages and raw patch pages.

Use `--json` on a command whose result a script consumes:

```sh
./bin/after capture --project /work/payment --json
./bin/after inspect BASE_ID CANDIDATE_ID --project /work/payment --json
./bin/after import report.jsonl --project /work/payment --json
./bin/after run BASE_ID CANDIDATE_ID --project /work/payment --json
./bin/after export COMPARISON_ID --project /work/payment # JSON is always emitted
```

The envelope is unchanged: one JSON object with `schema_version: 1`, a `kind`, and `data`, capped at 16 MiB. Raw patch bytes are base64; report and inventory results are paged. Imported test passes/failures retain producer `importer`, kind `reported`, unknown applicability, `not_run` execution, and `not_compared` state. They are never runner observations. A capture or import that remains active for one second on a terminal reports elapsed time on stderr; pipes receive no progress notice.

| Status | Meaning                                                                               |
| ------ | ------------------------------------------------------------------------------------- |
| `0`    | command completed; a complete comparison was equal                                    |
| `1`    | operational failure or incomplete/incomparable evidence                               |
| `2`    | invalid command arguments, configuration, IDs, or input data                          |
| `3`    | execution was declined, lacks exact authorization, or its digest mismatched           |
| `4`    | comparison finding (`different` or `unstable`), not an automatic regression judgement |

`run`, `compare`, `inspect`, and `export` distinguish findings from operational failures; JSON evidence remains available on finding/incomplete outcomes. Help/version are human-readable and do not inspect a project. `mise exec -- task test:cli` runs focused CLI/config/native-entry tests. `mise exec -- task cli:proof` runs the real subprocess payment proof and requires the separately provisioned pinned image plus explicit `AFTER_DOCKER_BINARY` and `AFTER_DOCKER_HOST`.
