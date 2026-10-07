# Examples

Runnable scenarios, from capturing a diff to catching a duplicate payment charge.
Each script builds a small throwaway Git project, runs the real `bin/after`, and
explains what it shows. Read a script top to bottom to learn the commands it runs.

```sh
mise exec -- task build
mise exec -- examples/cli/01-working-tree.sh
```

`mise exec --` puts the pinned Go and jq on `PATH`. Workspaces are kept under
`$TMPDIR/after-example-*` so you can explore them afterwards; move them to Trash
when done. `mise exec -- task examples` runs every example that needs no Docker
as a smoke test.

## CLI

| Script                                                   | Scenario                                                                    | Shows                                                                              |
| -------------------------------------------------------- | --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| [01-working-tree](cli/01-working-tree.sh)                | Raise a rate limit, forget the docs, leave a scratch file                   | `capture`, inventory, raw patch, untracked files excluded until you name them      |
| [02-staged-and-branches](cli/02-staged-and-branches.sh)  | Commit a fix but not a debug edit; review a feature branch after main moved | `--staged`, `--base main --target feature` (merge-base, like a PR)                 |
| [03-moved-oracle](cli/03-moved-oracle.sh)                | Discount changes and its test is edited to agree; the suite stays green     | `import` of `go test -json`, potential-oracle flag, a frozen-oracle run that fails |
| [04-config](cli/04-config.sh)                            | Team YAML defaults overridden by environment and flags                      | `config` provenance, fail-fast validation, no consent setting                      |
| [05-payment-experiment](cli/05-payment-experiment.sh) 🐳 | Four payment-service changes, run before and after in the offline sandbox   | `run` preview/approve, provider-request counts, `compare` witnesses, exit codes    |
| [06-pin-reopen-rerun](cli/06-pin-reopen-rerun.sh) 🐳     | Reviewer pins "one charge per retry"; the fix reopens it; rerun; accept     | `pin --select/--attach/--accept`, immutable revision history                       |

`05-payment-experiment.sh` takes a variant. Each runs the same frozen requests:
`POST /payments`, then a retry with the same `Idempotency-Key` after 12 hours and after 30 seconds.

| Variant               | Change                                     | HTTP responses | Provider requests (12h / 30s) | `after` exit |
| --------------------- | ------------------------------------------ | -------------- | ----------------------------- | ------------ |
| `retention` (default) | 24h → 5 min retention                      | identical      | 1→2 / 1→1                     | 4            |
| `refactor`            | `24 * 60 * 60` → `86400`                   | identical      | 1→1 / 1→1                     | 0            |
| `no-dedup`            | idempotency disabled                       | identical      | 1→2 / 1→2                     | 4            |
| `fake-log`            | app prints a false request count to stdout | identical      | 1→1 / 1→1                     | 0            |

The responses never change, so response snapshots and a reviewer reading the diff
can both miss the duplicate charge. `fake-log` shows that AFTER counts what the fake
provider received and ignores what the app reports about itself.

## TUI

Each script prepares state, prints a short key guide, then opens `after review BASE CANDIDATE`
after you press Enter. Set `AFTER_EXAMPLES_PREPARE_ONLY=1` to prepare without opening.
Use `1`/`2`/`3` or Tab/Shift+Tab for Overview/Changes/Diff; the frame identifies
the project and snapshot sources. After an external edit, `c` captures and `u`
explicitly uses the new candidate with the original base. Contextual footer hints
and grouped `?` help explain which actions are available. `a` does not switch
snapshots.

| Script                                                     | Scenario                                                                           |
| ---------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| [01-raw-review](tui/01-raw-review.sh)                      | Rename, deletion, binary asset, mode change, edited golden file, uncaptured secret |
| [02-test-reports](tui/02-test-reports.sh)                  | Shipping threshold moves; green candidate suite beside a failing frozen-oracle run |
| [03-payment-review-loop](tui/03-payment-review-loop.sh) 🐳 | Pin known-good behavior, apply a regression, watch the pin reopen, rerun, catch it |

## 🐳 Docker examples

These execute the captured code, so they need the offline sandbox. Provision the
pinned image once ([SANDBOX.md](../docs/SANDBOX.md)), then set both variables:

```sh
export AFTER_DOCKER_BINARY="$(command -v docker)"
export AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
```

Every run first saves an exact plan and asks you to type `yes` on the terminal.
The script then runs `after run --plan-file ... --approve <digest>`, the same
non-interactive flow you would use in CI. Each run takes one to two minutes.
Payment data is synthetic; the sandbox has no network.
