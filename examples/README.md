# Examples

Each script builds a throwaway Git repo, runs the real `bin/after`, and pauses
before each step. It ends with what a diff alone would have missed:

![examples/cli/03-moved-oracle.sh](demos/oracle.gif)

```sh
task build
examples/cli/03-moved-oracle.sh                        # Enter for the next step
AFTER_EXAMPLES_STEP=0 examples/cli/03-moved-oracle.sh  # no pauses
```

```text
   A diff shows  two edited files and a green test run
   AFTER shows   the test edit is a potential oracle, and the original tests fail: behavior changed
```

Needs Go and jq on `PATH`; example 08 also needs the mise-pinned Bun. Workspaces stay in `$TMPDIR/after-example-*` for
exploring. `task examples` smoke-tests everything that needs no Docker.

## CLI

| Script                                                   | Change                                                      | AFTER shows                                                        |
| -------------------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------ |
| [01-working-tree](cli/01-working-tree.sh)                | Rate limit doubles; docs forgotten; scratch file untracked  | Every path, including the one it left out; when a capture is stale |
| [02-staged-and-branches](cli/02-staged-and-branches.sh)  | Fix staged next to a debug edit; feature branch behind main | Only the staged change; the branch from its merge base             |
| [03-moved-oracle](cli/03-moved-oracle.sh)                | Discount 10% → 15%, test edited to agree                    | Edited test flagged; original tests fail on the new code           |
| [04-config](cli/04-config.sh)                            | YAML defaults, environment overrides, an invalid value      | Each value and the layer it came from; exit 2 on bad input         |
| [05-payment-experiment](cli/05-payment-experiment.sh) 🐳 | Payment idempotency change (four variants below)            | Same HTTP responses, different provider charges                    |
| [06-pin-reopen-rerun](cli/06-pin-reopen-rerun.sh) 🐳     | Reviewer pins "one charge per retry"; the author fixes it   | The pin reopens on the edit until a rerun is attached and accepted |
| [07-command-scenario](cli/07-command-scenario.sh) 🐳     | A quote CLI is "tidied": struct output, shorter JSON decode | Valid quote equal as JSON; a misspelled field now exits 0 with $0  |
| [08-junit-report](cli/08-junit-report.sh)                | Shipping threshold moves; Bun writes a JUnit report         | One reported failure and pass, never an observation                |
| [09-http-service](cli/09-http-service.sh) 🐳             | An echo proxy starts uppercasing request bodies             | Equal HTTP responses; the upstream received a changed body         |

`examples/cli/05-payment-experiment.sh VARIANT` sends `POST /payments`, then
retries the same `Idempotency-Key` after 12 hours and after 30 seconds:

| Variant               | Change                         | HTTP responses | Provider charges (12h / 30s) | Exit |
| --------------------- | ------------------------------ | -------------- | ---------------------------- | ---- |
| `retention` (default) | 24h → 5 min retention          | identical      | 1→2 / 1→1                    | 4    |
| `refactor`            | `24 * 60 * 60` → `86400`       | identical      | 1→1 / 1→1                    | 0    |
| `no-dedup`            | idempotency disabled           | identical      | 1→2 / 1→2                    | 4    |
| `fake-log`            | app logs a false request count | identical      | 1→1 / 1→1                    | 0    |

Example [08-junit-report](cli/08-junit-report.sh) runs Bun's built-in test runner
on a synthetic JavaScript shipping change and imports its freshly generated JUnit
XML. One unchanged test fails and one passes. `inspect` shows reported-only cards
alongside `diff`; import success does not mean test success or observed behavior.
Run with `mise exec -- examples/cli/08-junit-report.sh`. No packages are downloaded;
Bun is example tooling, not an AFTER runtime dependency. The unmodified report
imports complete.

Example [09-http-service](cli/09-http-service.sh) 🐳 selects a non-payment Python
stdlib echo proxy definition with `--definition`. Uppercasing a request body leaves
the HTTP responses equal, but the independent observer records a changed lowercase
payload and an equal uppercase payload. Each case runs twice per side. It requires
a matching arm64 host and the separately provisioned pinned Python service **and**
Go observer images in [SANDBOX.md](../docs/SANDBOX.md); it never pulls or installs.
After the Docker exports below, run `mise exec -- examples/cli/09-http-service.sh`
and review the exact consent prompt. These two cases do not establish general
correctness. Workspaces and live reports remain private in the temporary directory.

## TUI

![after review on a messy change](demos/review.gif)

| Script                                                     | Change                                                                       |
| ---------------------------------------------------------- | ---------------------------------------------------------------------------- |
| [01-raw-review](tui/01-raw-review.sh)                      | Rename, deletion, binary, mode change, edited golden file, untracked secret  |
| [02-test-reports](tui/02-test-reports.sh)                  | Free-shipping threshold moves; green suite next to failing original tests    |
| [03-payment-review-loop](tui/03-payment-review-loop.sh) 🐳 | Pin known-good behavior, introduce a regression, watch the pin reopen, rerun |

| Keys                | Action                                              |
| ------------------- | --------------------------------------------------- |
| `1`–`4`, Tab        | Overview, Changes, Diff, Activity                   |
| `j`/`k`, Enter, Esc | Move, open, back (also `l`/`h` and Emacs keys)      |
| `/`, `n`/`N`        | Search, next/previous match                         |
| `]`/`[`, `}`/`{`    | Next/previous file, hunk (Diff)                     |
| `c`, `u`            | Capture after an edit, review the new capture       |
| `p`, `r`, `a`       | Pin, rerun, accept (each confirms its exact target) |
| `?`, `q`            | Help, quit                                          |

`AFTER_EXAMPLES_PREPARE_ONLY=1` prepares the state without opening the TUI.

## 🐳 Docker

The payment, command and HTTP-service examples run code in the offline sandbox. Example 07
selects its own [command definition](../docs/RUNNER.md#direct-argv-command-scenarios)
with `after run BASE CANDIDATE --definition FILE`. Provision the image once
([SANDBOX.md](../docs/SANDBOX.md)), then:

```sh
export AFTER_DOCKER_BINARY="$(command -v docker)"
export AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
```

`after run` prints the exact plan and runs nothing until you type `yes`.

## Recording the GIFs

`task demo:gifs` re-records [demos/](demos) with [VHS](https://github.com/charmbracelet/vhs),
using the same fixtures as the scripts. It needs `ttyd` and the Docker setup above.
