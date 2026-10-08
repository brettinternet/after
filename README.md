# AFTER

See what your code does differently, not just how it reads differently.

A change shortens a payment idempotency window from 24 hours to 5 minutes. The diff is one line, every HTTP response is identical, and the tests pass. AFTER runs both versions in an offline sandbox and counts what the fake payment provider actually received:

```text
$ after compare
Using the newest run of base 564e6c29 → candidate b7668b46
Comparison 9977cced · different · complete
  12h responses          [EQUAL]
  12h provider requests  [DIFFERENT] · 1 → 2
  30s responses          [EQUAL]
  30s provider requests  [EQUAL] · count unchanged
```

A retry after 12 hours now charges the customer twice. Pin "one charge per retry" as your expectation, and AFTER reopens the pin when later evidence changes.

**Status:** proof of concept. It runs one frozen payment experiment, not arbitrary applications, and has not been tested with human reviewers.

## Try it

```sh
mise trust && mise install && mise exec -- task init
mise exec -- task build
mise exec -- task demo:inspect                      # no Docker
mise exec -- examples/cli/01-working-tree.sh        # capture and inspect a change
mise exec -- examples/cli/05-payment-experiment.sh  # the run above (Docker)
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
- Contributing: [AGENTS.md](AGENTS.md), [handoff](docs/HANDOFF.md), [implementation contract](docs/IMPLEMENTATION.md), [backlog](backlog/tasks)

## Checks

```sh
mise exec -- task check:staged  # before each commit
mise exec -- task check:go      # build, race tests, vet, gofmt
mise exec -- task test          # Go tests, links, backlog
mise exec -- task docs:check    # presentation checks in Chromium
mise exec -- task check         # everything
```
