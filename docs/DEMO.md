# Build, package and reproduce

AFTER is a bounded local proof of concept, not a general application runner.
The [CLI](CLI.md) and [TUI](TUI.md) work without Bun, models, accounts or GitHub.
The presentation remains an invented simulation; this walkthrough produces real
receipts only after separately authorized execution. No human benefit study has run.

## Fresh checkout and native packages

Install mise, then from a fresh checkout:

```sh
mise trust
mise install
mise exec -- task init
mise exec -- task build
./bin/after --help
mise exec -- task package
```

Go is pinned in `mise.toml`; `go.mod`/`go.sum` pin modules. Preparation may download
trusted tools/modules. It is separate from offline project execution. Packaging
uses the local Go toolchain, read-only module resolution, `CGO_ENABLED=0` and
trimmed paths. It creates a new ignored `dist/0.1.0-dev.COMMIT[-suffix]/` directory
(the actual version uses `.dirty` for modified checkouts), containing four raw
executables named `after_VERSION_OS_ARCH`, `SHA256SUMS` and `BUILD.txt`.
No archive extractor, installer, signing, upload or release publication runs.

Supported inspection targets: macOS and Linux, amd64 and arm64. `/usr/bin/git`
is required for capture and demo setup; an actual terminal is required for the
TUI. The binary has Go module dependencies but no runtime Go, Bun or C-library
requirement. Development race tests require a C compiler. Cross-compilation is
not cross-platform execution proof: CI is configured to run native checks on all
four host targets and the Docker gate on Linux amd64. Local macOS/Colima results
are not a claim that a new CI run has passed.

From the printed package directory, verify before installing the matching file:

```sh
shasum -a 256 -c SHA256SUMS
# Linux also supports: sha256sum -c SHA256SUMS
```

Checksums detect corruption, not publisher authenticity. Unsigned development
binaries may require the platform's explicit local execution approval.

## Without Docker

```sh
mise exec -- task demo:inspect
# Retain private JSON steps and captured state for exploration:
mise exec -- task demo:inspect -- --keep
```

This creates a private temporary Git repository from exactly four allowlisted
synthetic payment fixture files. It commits the 24-hour base, edits retention to
five minutes, captures the dirty candidate and inspects the raw diff. It imports
the checked-in synthetic Go report, explicitly labeled **reported / unknown /
not_run**. It does not run those tests, contact Docker, infer HTTP inputs or invent
payment effects. The fixture files and report are the only source data copied;
no caller project, ignored files, credentials or production services are used.

Each native CLI response is written to `step-NN.json` outside the fixture project.
Use the capture step's base/candidate IDs with `after inspect` or
`after review --tui CANDIDATE --base BASE --project PRINTED_WORKSPACE/payment`.
The no-Docker route has no observed receipt to pin as a finite example. In the
TUI, `1`/`2`/`3` and Tab/Shift+Tab switch Overview, Changes, and Diff; the frame
shows short snapshot IDs and sources. After an external edit, `c` captures in the
background and `u` explicitly uses the new candidate while retaining the original
base. `?` lists context-enabled keys and explains unavailable actions. `a` does
not switch snapshots.

## Real offline payment loop

Execution requires an explicitly selected trusted absolute Docker CLI, a local
Unix socket, a Linux daemon with cgroup v2/resource controls, and the pinned image.
macOS requires a prepared Linux VM, such as Colima (at least 2 GiB memory).
See [sandbox provisioning and limits](SANDBOX.md). Provisioning is an explicit
network action, never performed by the demo:

```sh
docker pull docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190
```

Example for a prepared Linux machine:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
mise exec -- task demo -- --keep
```

On macOS substitute the installed CLI's absolute path and the local socket from
`docker context inspect colima`. No context or credential configuration is
inherited by the actual sandbox. The demo never silently pulls a missing image.

The command walks capture/import/inspect, displays an exact execution preview,
and asks you to type its digest. Read the snapshots, image, argv, isolation and
limits before consenting. EOF or a different digest stops before execution.
Allow several minutes: every observation builds and runs real processes offline.

1. The initial receipt must record identical HTTP responses, **one → two** actual
   provider requests at twelve hours, and the unchanged **one → one** 30s control.
2. A finite expectation is pinned with an explicit synthetic-demo reason.
3. The demo restores 24-hour retention with a small comment edit and captures it.
4. Selecting that candidate must reopen the pin as stale with no current result.
5. A second preview requires separate exact consent. The rerun uses the pin's
   original base, records **one → one** in both cases, and attaches its new receipt.
6. Attachment does **not** accept the expectation or establish universal behavior.
   Human acceptance remains a separate [review action](REVIEW.md).

For an explicitly authorized automated synthetic check, `task demo:proof` passes
`--authorize-synthetic-plans`, then logs and authorizes both exact generated plans
without prompts. No environment variable skips consent. This is the demo's
CI proof mode, not consent configuration for arbitrary project execution.
`task test:poc` additionally runs the full adversarial Docker/PTY gate and mutation
controls. Neither command starts production services or uses a host fallback.

## Ownership, failure diagnostics and recovery

A successful demo removes only the temporary directory it created in the canonical
OS temporary directory, after checking its original inode, random ownership token
and prefix. Symlink targets are not followed during deletion. Docker resources are
removed by the runner using exact random names and ownership labels. Unrelated
repositories/resources are untouched. `--keep`, consent denial and operational
errors retain the printed workspace and JSON steps for diagnosis. There is no
cross-session automatic deletion command: inspect the exact retained directory
and `.owner` before manually moving it to Trash. Do not delete paths by glob,
shared label or guessed name. Retained `.after` data is private; never commit it.

- **Missing binary/tools:** run `task init` then `task build`. Demo setup requires
  `/usr/bin/git`; missing fixture data means run the task from a complete checkout.
- **Docker endpoint/image/isolation failure:** inspect the incomplete run JSON and
  sandbox diagnostics. Set both Docker variables, provision the exact image,
  verify the local daemon/cgroup controls, then start a new demo. Do not substitute
  host execution or weaken policy. Operational failure is not behavioral equality.
- **Store lock/permissions/schema/hash errors:** stop competing writers, check
  ownership and private permissions, and preserve the store. Never remove a live
  lock, edit hashes/manifests or downgrade a newer schema. Use a compatible binary;
  for this disposable demo start a fresh workspace instead of repairing evidence.
- **Interrupted execution/cleanup failure:** stop further execution and follow
  [sandbox recovery](SANDBOX.md#residual-limits-and-failure-recovery). Match exact
  names, labels, creation time and inputs to the interrupted invocation before
  removing anything. Never globally prune. Ordinary CLI SIGINT/SIGTERM cancels the
  runner context; forced process death, power loss or daemon failure can still
  leave resources. A killed client does not prove daemon cleanup completed.
- **Package/checksum failure:** keep the failed directory for diagnosis; rerun
  packaging to a new directory. Never distribute a partial or mismatched set.

The runner observes only sequential synthetic payment HTTP responses and fake
provider requests, with a finite window. Arbitrary application adapters, remote
PR synchronization, browser behavior and human validation remain deferred.
