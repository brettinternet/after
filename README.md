# AFTER

**See what the code will do differently.** A local-first, example-first review tool: inspect concrete before/after behavior, pin an expectation, and reopen it when its evidence changes.

**Status:** proposal, reproducible documentation tooling and implementation backlog. The Go engine and TUI are not implemented yet. The presentation is a simulation.

- [Agent handoff](docs/HANDOFF.md)
- [Implementation contract and POC gates](docs/IMPLEMENTATION.md)
- [Product proposal](docs/README.md) · [behavior/evidence design](docs/behavior-and-evidence.md)
- [Interactive concept](docs/presentation.html) · [PDF](docs/presentation.pdf)
- [Backlog tasks](backlog/tasks)

## Development setup

Install [mise](https://mise.jdx.dev/), then:

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task test
mise exec -- backlog task AFTER-1 --plain
```

The pinned tools are Go, Bun, Prettier, Gitleaks, Lefthook, Task, Worktrunk and Backlog.md. Bun and Playwright support documentation checks only; the planned AFTER binary does not need them at runtime.

The tooling is adapted from the sibling `project` template: mise, Task targets, staged formatting/secret hooks, EditorConfig and blocking Worktrunk setup. Web/server tasks, Docker Compose services, Hum, Varlock and copied environment state were deliberately omitted: this project currently has no development service stack or secrets to configure. Future fixture isolation is a separate, tested execution boundary, not the template's database stack.

## Checks

```sh
mise exec -- task check:staged          # after staging intended files
mise exec -- task test                  # local links and backlog dependency/quality checks
mise exec -- task check                 # all current tooling/documentation checks
mise exec -- task docs:browser:install  # once, when Chromium is absent
mise exec -- task docs:check            # real browser checks; ignored artifacts/
mise exec -- task docs:render           # also regenerate PDF and preview
```

CI runs formatting, backlog/link checks, secret scanning and Chromium interaction/layout/print checks. AFTER-1 adds Go checks; AFTER-15/16 add the real sandbox POC gate. Current green CI does not establish an engine that has not been built.
