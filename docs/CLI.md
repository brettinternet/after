# Headless CLI

This page documents current behavior; the planned redesign (AFTER-34
to AFTER-45) is specified in [CLI-DESIGN.md](CLI-DESIGN.md).

`after` is the headless entry point over AFTER's capture, private store, Go test report, raw-diff, frozen runner, and comparison APIs. It has no model, account, GitHub, or editor dependency. `--help` and `--version` are side-effect free. Headless commands emit one versioned JSON object on stdout; diagnostics use stderr. `review --tui <candidate-id> --base <base-id>` instead opens the [captured evidence browser](TUI.md) on a terminal, without execution on open. JSON strings escape terminal control characters. Consumers must still sanitize untrusted values when rendering them.

See [packaging, the repeatable demo and recovery](DEMO.md) for native distributions and a prepared-checkout walkthrough. Build with `mise exec -- task build`, then run commands from any directory with a selected project:

```sh
./bin/after capture --project "/work/payment" --include-untracked "fixtures/new case.json"
./bin/after inspect CANDIDATE_ID --base BASE_ID --project "/work/payment"
./bin/after import "go test output.jsonl" --producer "go1.27.1 on linux/amd64" --snapshot CANDIDATE_ID --project "/work/payment"
./bin/after inspect REPORT_ARTIFACT_ID --project "/work/payment"
./bin/after compare RECEIPT_ID --project "/work/payment"
./bin/after export COMPARISON_ID --project "/work/payment"
```

Flags may appear before or after positional arguments. Capture defaults to HEAD versus the working tree; `--staged` selects HEAD versus the index. Explicit `--base REF --target REF` selects a merge-base capture. `--include-untracked` accepts repeated exact paths only. Capture and import create private `.after/` storage; inspection/export open it read-only. Import requires caller-supplied `--producer` provenance and accepts optional `--captured-at RFC3339`; these claims are retained but not authenticated. `--snapshot` is an optional validated content digest, not proof the report ran on that capture. The ordinary diff and all excluded/unsupported inventory entries remain available without imported or observed evidence. `inspect CANDIDATE_ID --base BASE_ID` returns bounded inventory pages and a base64 raw-patch page; `--diff-offset`, `--diff-size`, and `--inventory-offset`/`--inventory-limit` page the data. Imported report cards use `--card-offset`/`--card-limit`.

## Checkout root and private storage

Project commands resolve the checkout root by checking for a `.git` directory or worktree `.git` file in the selected directory and its parents. The default selected directory is the invocation directory; `--project`, `AFTER_PROJECT`, or the YAML `project` setting can select another path inside a checkout. Resolution is filesystem-only and runs no Git command. It is shared by capture, import, inspect, compare, export, run, pin and review. The capture API itself still requires its caller to pass the resolved repository root explicitly.

Outside a checkout, project commands exit 2 with exactly `after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR` and create no `.after/` directory or lock file. `help`, `version` and `config` do not require a checkout; read-only project commands never create storage. Writable store opens create `.after/.gitignore` with `*` using the store's durable, no-overwrite publication path, including when an older store has no ignore file yet. This ignores only private store contents and does not edit the checkout's `.gitignore` or `.git/info/exclude`.

Capture failures retain exit 1 and use fixed allowlisted reason/fix text, for example `after: capture failed: unmerged index is unsupported — resolve the index conflicts, then retry capture`. Repository content and absolute project paths are never interpolated into these diagnostics.

Stored artifacts (including observer response/effect channels referenced by receipts) can also be inspected or exported by content ID. `--artifact-offset` and `--artifact-size` return exact base64 byte pages, up to 65536 bytes, with `next`, `total`, and `more`. A complete valid JSON artifact fitting one page also has a `document` field preserving numeric precision. Artifact content alone does not establish its producer or evidence state; inspect its referring receipt for provenance.

## Persistent expectations

`pin RECEIPT_ID --expectation TEXT --scope finite_example|human_intent --reason TEXT`
creates an immutable pin revision. `review PIN_REVISION_ID` opens it read-only.
Mutations use exactly one of `--select SNAPSHOT_ID --mode original_base|last_inspected`,
`--receipt RECEIPT_ID`, or `--accept`, always with `--reason TEXT`. Each returns a
new revision ID; use that ID for the next operation. Selection reopens changed
bindings without predicting results. Receipt attachment never accepts the pin;
human acceptance is a separate action. No pin/review action executes code or
grants run permission. See [the review workflow](REVIEW.md) for examples, exact
reuse rules, broader-intent limits, historical revisions and rerun authorization.

## Execution authorization

The first run command prepares the payment-specific frozen plan; it does not execute it before authorization. In a non-TTY invocation, save the exact plan and read its digest from the JSON response:

```sh
./bin/after run BASE_ID CANDIDATE_ID --project "/work/payment" \
  --plan-out "/work/payment/.after/approved-preview.json"
# Review the plan and digest, then authorize only that exact plan:
./bin/after run --plan-file "/work/payment/.after/approved-preview.json" \
  --approve sha256:... --project "/work/payment"
```

The preview-only invocation returns status `authorization_required` and exit 3. It never prompts on non-TTY stdin. With an interactive terminal, `run BASE_ID CANDIDATE_ID` prints the exact plan to stderr and requires typing `yes`; any other input denies execution. `interactive: false` disables this prompt but grants no authority. `--approve` is accepted only with `--plan-file` and must equal the exact digest. The plan file is private (0600), created without overwrite, and is reconstructed against current immutable snapshots before execution. Any changed or edited plan is rejected. Each plan has a new random request ID; re-preparing is not equivalent to approving an old preview.

Only after exact approval may the runner contact the explicitly selected local Docker endpoint. Set both an absolute trusted CLI and a local Unix socket, for example:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
./bin/after run --plan-file "/work/payment/.after/approved-preview.json" \
  --approve sha256:... --project "/work/payment"
```

No Docker context, image pull, host execution, build, or project command is used by help, config display, capture, import, inspect, export, comparison, pin, review, or preview. Provision the pinned image separately as documented in [SANDBOX.md](SANDBOX.md). Missing endpoints or isolation produce an incomplete operational result; they never select a host fallback.

## Configuration

AFTER reads configuration only when a command needs it. The explicit `--config FILE` or `AFTER_CONFIG` path must exist; otherwise the optional discovered file is `${XDG_CONFIG_HOME}/after/config.yaml`, falling back to `~/.config/after/config.yaml`. Relative config/project paths are resolved from the invocation working directory. The discovered file may be absent. Configuration loading parses data only; it never runs repository code. `after config` reports effective values and each winning source (`flag`, `env`, `file`, or `default`); project/config paths and Docker endpoint values are hidden.

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

Docker binary and host must be configured together. There is deliberately no consent/authorization setting: permission is an interactive action or the specific `--approve` digest. Runtime values remain hidden in config output. YAML is limited to 64 KiB, one mapping document, regular non-symlink files, scalar values, and the listed keys. Duplicate/unknown keys, anchors/aliases, malformed documents, invalid types/ranges, and missing explicit files fail with a setting/source diagnostic.

## Output and exit status

Every command result is a single JSON object with `schema_version: 1`, a `kind`, and `data`, capped at 16 MiB. Raw patch bytes are base64; report and inventory results are paged. Imported test passes/failures retain producer `importer`, kind `reported`, unknown applicability, `not_run` execution, and `not_compared` state. They are never runner observations.

| Status | Meaning                                                                               |
| ------ | ------------------------------------------------------------------------------------- |
| `0`    | command completed; a complete comparison was equal                                    |
| `1`    | operational failure or incomplete/incomparable evidence                               |
| `2`    | invalid command arguments, configuration, IDs, or input data                          |
| `3`    | execution was declined, lacks exact authorization, or its digest mismatched           |
| `4`    | comparison finding (`different` or `unstable`), not an automatic regression judgement |

`run`, `compare`, `inspect`, and `export` distinguish findings from operational failures; JSON evidence remains available on finding/incomplete outcomes. Help/version are human-readable and do not inspect a project. `mise exec -- task test:cli` runs focused CLI/config/native-entry tests. `mise exec -- task cli:proof` runs the real subprocess payment proof and requires the separately provisioned pinned image plus explicit `AFTER_DOCKER_BINARY` and `AFTER_DOCKER_HOST`.
