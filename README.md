# AFTER

**See what the code will do differently.** A local-first, example-first review tool: inspect concrete before/after behavior, pin an expectation, and reopen it when its evidence changes.

**Status:** native Go help/version CLI and versioned evidence contracts. Capture, import, execution, comparison and TUI are not implemented yet. The presentation is a simulation.

- [Agent handoff](docs/HANDOFF.md)
- [Implementation contract and POC gates](docs/IMPLEMENTATION.md)
- [Version-1 record schema and trust limits](docs/SCHEMA.md)
- [Product proposal](docs/README.md) · [behavior/evidence design](docs/behavior-and-evidence.md)
- [Interactive concept](docs/presentation.html) · [PDF](docs/presentation.pdf)
- [Backlog tasks](backlog/tasks)

## Development setup

Install [mise](https://mise.jdx.dev/), then:

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task build
./bin/after --help
./bin/after --version
mise exec -- task test
```

The pinned tools are Go, Bun, Prettier, Gitleaks, Lefthook, Task, Worktrunk and Backlog.md. Bun and Playwright support documentation checks only; the AFTER binary does not need them at runtime. Help/version work outside a repository without Docker, network access or model credentials. No arguments prints help. Unknown commands, flags and extra arguments exit 2; help/version exit 0; output failures exit 1. The CLI never prompts or executes project code.

The tooling is adapted from the sibling `project` template: mise, Task targets, staged formatting/secret hooks, EditorConfig and blocking Worktrunk setup. Web/server tasks, Docker Compose services, Hum, Varlock and copied environment state were deliberately omitted: this project currently has no development service stack or secrets to configure. Future fixture isolation is a separate, tested execution boundary, not the template's database stack.

## Checks

```sh
mise exec -- task check:staged          # after staging intended files
mise exec -- task build                 # bin/after native executable
mise exec -- task test:go               # Go tests with race detection
mise exec -- task check:go              # build, race tests, vet and gofmt check
mise exec -- task format:go             # format Go sources
mise exec -- task test                  # Go tests, local links and backlog checks
mise exec -- task check                 # all Go/tooling/documentation checks
mise exec -- task docs:browser:install  # once, when Chromium is absent
mise exec -- task docs:check            # real browser checks; ignored artifacts/
mise exec -- task docs:render           # also regenerate PDF and preview
```

CI runs Go build/race tests/vet/format checks and CLI smoke commands on macOS and Linux, plus formatting, backlog/link checks, secret scanning and Chromium presentation checks. Race tests require a C compiler and target macOS/Linux amd64/arm64; the shipped binary is standard-library-only. AFTER-15/16 add the real sandbox POC gate. Green foundation checks do not establish the proposed review engine.
