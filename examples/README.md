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

Needs Go and jq on `PATH`. Workspaces stay in `$TMPDIR/after-example-*` for
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

`examples/cli/05-payment-experiment.sh VARIANT` sends `POST /payments`, then
retries the same `Idempotency-Key` after 12 hours and after 30 seconds:

| Variant               | Change                         | HTTP responses | Provider charges (12h / 30s) | Exit |
| --------------------- | ------------------------------ | -------------- | ---------------------------- | ---- |
| `retention` (default) | 24h → 5 min retention          | identical      | 1→2 / 1→1                    | 4    |
| `refactor`            | `24 * 60 * 60` → `86400`       | identical      | 1→1 / 1→1                    | 0    |
| `no-dedup`            | idempotency disabled           | identical      | 1→2 / 1→2                    | 4    |
| `fake-log`            | app logs a false request count | identical      | 1→1 / 1→1                    | 0    |

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
| `j`/`k`, Enter, Esc | Move, open, back                                    |
| `/`, `n`/`N`        | Search, next/previous match                         |
| `]`/`[`, `}`/`{`    | Next/previous file, hunk (Diff)                     |
| `c`, `u`            | Capture after an edit, review the new capture       |
| `p`, `r`, `a`       | Pin, rerun, accept (each confirms its exact target) |
| `?`, `q`            | Help, quit                                          |

`AFTER_EXAMPLES_PREPARE_ONLY=1` prepares the state without opening the TUI.

## 🐳 Docker

The payment examples run code in the offline sandbox. Provision the image once
([SANDBOX.md](../docs/SANDBOX.md)), then:

```sh
export AFTER_DOCKER_BINARY="$(command -v docker)"
export AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
```

`after run` prints the exact plan and runs nothing until you type `yes`.

## Recording the GIFs

`task demo:gifs` re-records [demos/](demos) with [VHS](https://github.com/charmbracelet/vhs),
using the same fixtures as the scripts. It needs `ttyd` and the Docker setup above.
