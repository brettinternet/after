# Frozen paired execution

`internal/runner` implements AFTER-8's payment-specific paired runner. The
[headless CLI](CLI.md) previews plans, obtains operator confirmation, executes
only the exact authorized plan, and persists a comparison. Help, capture, import,
inspection, comparison, preview, and ordinary tests do not execute the runner.

## Boundary and authorization

`Prepare(store, pair, repetitions, limits)` loads validated immutable captures,
requires the payment ABI (`go.mod`, `app/main.go`), and freezes both snapshots,
a random request ID, two scenarios, driver/observer/rules identities, sandbox
plans and 1–5 repetitions. `Preview()` returns a private JSON copy and its digest.
`Executor.Run` requires that exact approved digest before any Docker command.
The caller must obtain operator consent; a repository-supplied digest is not
permission. Changing the request, snapshots, repetitions, limits or frozen code
requires new approval. Plan preparation writes frozen scenario/input records but
executes nothing. No baseline reuse or candidate-owned suite execution occurs.

The built-in experiment uses the [synthetic payment fixture](../internal/paymentfixture/README.md):
epoch 1735689600, key `synthetic-key-a`, POST body
`{"amount_cents":1200,"currency":"USD"}`, then the same request at 43200 or
30 seconds. Each side/case/repetition starts a new application with empty state.
The clock advances deterministically, not by waiting for simulated hours.

Each sample uses two containers under the [sandbox policy](SANDBOX.md):

- The application container builds the captured `./app` with `go build -trimpath
-o /work/app ./app`. A shipped launcher starts `/work/app` with clock/provider
  URLs `http://127.0.0.1:18081` and `http://127.0.0.1:18082`. The payment ABI writes
  its listening loopback URL to descriptor 3 and serves `POST /payments`.
- The frozen observer is compiled separately from embedded AFTER source. It
  joins only the application's **offline network namespace**, not its PID, IPC,
  filesystem, input or scratch namespaces. It owns the clock and append-only
  fake-provider log and issues HTTP requests through a bounded loopback proxy.
- No candidate source, tests, driver, mask or printed counter is compiled into
  the observer. App stdout/stderr are diagnostics only. Observer output comes
  from its own Docker attachment; malformed/missing channels fail closed.

All builds run inside the approved isolation. There are no mounts, published
ports, downloads, shell commands selected by repository data or host execution
fallbacks. The pinned image supplies the toolchain and standard library.
Readiness polling is bounded real time, separate from the experiment clock.
The application remains a service until owned cleanup; its intentional shutdown
is recorded with exit code -1, not presented as a naturally successful exit.

## Results and bounds

One process-wide, cancellable gate permits one experiment at a time. Each has
at most two active workloads; preparation containers never run. The existing
per-container CPU, memory, PID, scratch, time and combined stdout/stderr limits
apply to both. Provider logs are capped at 128 calls, bodies at 4096 bytes and
HTTP requests at two seconds. A provider overflow fails the entire observation.

Each immutable receipt includes its request ID, original snapshot pair, frozen
scenario bindings, environment/toolchain/dependency identities, authorization
plan identity, timestamps and artifact references. The persisted execution plan
records both container argv/input archives/policies and the nested build/app argv.
Dependency identity conservatively covers the whole captured footprint; the
image manifest binds the toolchain, and each executed sample retains its actual
derived image ID. The driver identity covers both launcher and observer source. Sample artifacts record
side, case, repetition, timestamps, status, actual derived image IDs, cleanup and
truncation flags. Each observation retains exact response status/body and
provider-received timestamp/method/path/key/body. Counts are log lengths, never
app diagnostics. The raw inventory still includes candidate oracle edits.

Every requested sample is retained, including failed or unstarted samples.
Denied permission, cancellation, deadline, startup/build failure, output limit,
missing/incompatible channels and cleanup failures yield incomplete receipts,
never behavioral equality or a fabricated baseline. Late output stays attached
to the submitted pair/request, not a newly selected UI snapshot. Matching
repetitions remain `not_compared`; differing complete repetitions are `unstable`.
[Exact finite comparisons](COMPARISON.md) validate these stored bindings and
recompute semantic repetition stability. The versioned shipped comparison policy
is retained as a rules artifact; any policy change requires a new authorized run.

Diagnostics use fixed failure codes plus bounded separate artifacts. The store
redacts explicitly configured literals; redacted/truncated data cannot yield a
complete receipt. This is not automatic secret discovery: do not capture private
inputs without an appropriate store redaction policy. No host credentials or
ambient Docker configuration enter execution.

Cancellation joins attachment goroutines and cleans both exact owned container
names and their input images. Failed cleanup prevents later samples in the same
request from executing. Daemon loss, host crash or forced agent termination can
interrupt cleanup: retain resource identities and reconcile them using the
[sandbox recovery procedure](SANDBOX.md) before further execution. A crash may
leave immutable sample artifacts without a final receipt; automatic cross-session
resumption/reconciliation is not implemented. Storage failure must not be
represented as durable completion.

## Reproduce

With the separately provisioned pinned image and explicit local Docker endpoint:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
mise exec -- task runner:proof
```

This authorizes only synthetic generated Git captures and lifecycle probes. No
pull is performed. The proof persists real observations through the private
store, checks two repetitions of 12h counts 1/2 and 30s counts 1/1 with identical
responses, and edits candidate tests/driver plus forged stdout/scratch output.
Separate lifecycle probes check namespace isolation, startup failure, combined
stdout/stderr overflow, readiness-confirmed descendant deadline/cancellation,
cleanup and preservation of an unrelated sentinel container. Live regressions
also require premature app exit with a serving descendant and refused upstream
connections to remain incomplete, while genuine app-generated 502 responses
remain observable. Proxy transport failures abort the request rather than
manufacturing a response.

`mise exec -- task check:go` runs race-enabled unit tests for consent, immutable
previews, barrier-controlled late selection, repetition instability, all failure
states, redaction, cleanup refusal and cancellable concurrency. Ordinary tests
skip live Docker. Validated on macOS with a Colima Linux aarch64 daemon; native
Linux workstation, Docker Desktop and amd64 are not independently certified.

## Limits

The Docker daemon/CLI, image, host kernel and AFTER's shipped observer are trusted.
This is not a malware-analysis VM. Candidate traffic can disrupt or impersonate
its own application endpoint, and all observed payloads remain untrusted data.
The app cannot rewrite the observer's memory/files/output through the shared
network. Provider/clock port conflicts, flooding or malformed responses fail
rather than grant authority. Observations describe finite sequential traffic,
not background effects after the window, durable billing, real payments or all
possible inputs. The launcher/proxy is app-side infrastructure, not a trusted
source of provider counts. The complete raw diff remains independently usable.
