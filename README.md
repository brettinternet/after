# AFTER

See what your code does differently, not just how it reads differently.

![after run: identical responses, but the 12h retry charges twice](examples/demos/payment.gif)

A change shortens a payment idempotency window from 24 hours to 5 minutes. The diff is one line, every HTTP response is identical, and the tests pass. AFTER runs both versions in an offline sandbox and counts what the fake payment provider actually received: a retry after 12 hours now charges the customer twice. Pin "one charge per retry" as your expectation, and AFTER reopens the pin when later evidence changes.

## Try it

Commands assume the tools are on `PATH`, through mise shell activation or another
installation method. `mise install` installs the pinned versions in `mise.toml`.

```sh
mise trust && mise install && task init
task build
task demo:inspect                      # no Docker
examples/cli/01-working-tree.sh        # capture and inspect a change
examples/cli/05-payment-experiment.sh  # the run above (Docker)
```

Reading a change never runs project code:

```text
$ after capture
$ after inspect
…
  Inventory    2 paths · 2 shown
  M  limits/limits.go
  ?  notes.txt · excluded: untracked; not selected
```

Only `after run` executes anything, and only after you approve the exact plan it prints.

## Docs

- [Examples](examples/README.md): CLI and TUI walkthroughs
- [Demo and packaging](docs/DEMO.md)
- [CLI](docs/CLI.md) · [TUI](docs/TUI.md) · [Record schema](docs/SCHEMA.md)
- [Evaluation kit](docs/EVALUATION.md)
- [Proposal](docs/README.md) · [Interactive concept](docs/presentation.html) ([PDF](docs/presentation.pdf))
- [Extension design](docs/EXTENSIONS.md) (proposal)
- Contributing: [AGENTS.md](AGENTS.md), [handoff](docs/HANDOFF.md), [implementation contract](docs/IMPLEMENTATION.md), [backlog](backlog/tasks)

## Checks

```sh
task check:staged  # before each commit
task check:go      # build, race tests, vet, unused code, gofmt
task test          # Go tests, links, backlog
task docs:check    # presentation checks in Chromium
task check         # everything
```
