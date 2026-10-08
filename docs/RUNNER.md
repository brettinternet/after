# Frozen paired execution

`internal/runner` is the payment-specific paired runner. It executes captured snapshots only after the operator approves an exact plan; help, capture, import, inspection, comparison, previews and ordinary tests do not execute it. See [sandbox policy](SANDBOX.md) for container limits.

## Prepare and authorize

`Prepare(store, pair, repetitions, limits)` loads validated immutable captures, requires the payment ABI (`go.mod`, `app/main.go`), and freezes both snapshots, a random request ID, scenarios, driver/observer/rules identities, sandbox plans and 1–5 repetitions. It writes frozen input/scenario records but does not execute. `Preview()` returns a private JSON copy and its digest. Any changed snapshot, request, repetition count, limit or frozen code requires new consent. There is no baseline reuse or candidate-owned suite execution.

The CLI binds consent to exact preview bytes. A terminal shows the decoded summary and size, stores the immutable plan privately (0600), then asks for `yes`; non-TTY mode returns the full digest and exact `--approve` command. `--approve` reconstructs and checks the stored bytes; `--plan-file FILE --approve DIGEST` and `--plan-out FILE` are also supported. `after inspect PLAN` displays the strict consent summary and sanitized plan. A summary-decode problem is visible but does not alter the bytes covered by consent. Configuration cannot authorize a run. Docker settings are not probed or used until approval and explicit Docker configuration.

```sh
after run
# Inspect the exact plan and approve only its full digest:
after run --approve sha256:<full-64-hex-digest>
# Or save a plan to an explicit file:
after run BASE CANDIDATE --plan-out .after/approved-preview.json
after run --plan-file .after/approved-preview.json \
  --approve sha256:<full-64-hex-digest>
```

The preview binds image, commands, mounts, network policy, limits, inputs and snapshots. Read it before approving. The CLI accepts interactive confirmation or the exact full digest; a digest supplied by repository data is not permission. An active run reports elapsed time on terminal stderr after one second, without a percentage.

## Payment ABI and isolation

The built-in experiment sends the same request twice from fixed epoch `1735689600`, using key `synthetic-key-a` and body `{"amount_cents":1200,"currency":"USD"}`. The second request is at 43,200 seconds or 30 seconds. Each side/case/repetition starts a fresh app with empty state; the clock advances deterministically, not by waiting. The fake provider records received calls; it does not supply the app's deduplication logic.

Each sample runs two containers:

- The app container builds captured `./app` with `go build -trimpath -o /work/app ./app`. The trusted launcher starts it with clock/provider URLs `127.0.0.1:18081` and `127.0.0.1:18082`. The app reports its readiness URL on descriptor 3 and must serve `POST /payments`.
- A frozen observer, compiled from embedded AFTER source, owns the clock, request driver and append-only fake-provider log. It joins only the app's offline network namespace—not its PID, IPC, filesystem, input or scratch namespaces. The app reaches the observer through a bounded loopback proxy.

Candidate source, tests, driver, mask and printed counters are never compiled into the observer. App stdout/stderr are diagnostics, not evidence. Observer output comes from its Docker attachment; missing or malformed channels fail closed. Builds and app execution stay inside the approved isolation. There are no mounts, published ports, downloads, repository-selected shell commands or host fallback. The pinned image supplies Go and the standard library. Readiness uses a bounded wall-clock timeout separate from simulated time. The app intentionally remains a service until cleanup; its shutdown exit `-1` is not natural success.

## Receipt, output and failures

One process-wide cancellable gate permits one experiment at a time; a sample has at most two active workloads. Each container inherits sandbox CPU, memory, PID, scratch, time and combined stdout/stderr limits. The observer accepts at most 128 provider calls, 4,096-byte bodies and two-second HTTP requests; overflow fails the observation.

Receipts bind request ID, original pair, scenario, environment/toolchain/dependency identities, authorization plan and artifact references. Plans include container argv/input archives/policies and nested build/app argv. Dependency identity covers the captured footprint; the image manifest binds the toolchain and samples retain actual derived image IDs. Driver identity covers launcher and observer source. Sample artifacts record side/case/repetition, timestamps, status, actual image IDs, cleanup/truncation flags and separate observations/diagnostics. Provider counts are log lengths, never app-reported values. Opening readable cards reads stored evidence; it does not rebuild or rerun.

Denied consent, cancellation, deadlines, startup/build failures, output limits, missing channels, redaction or cleanup failures produce incomplete receipts, never equality or a fabricated baseline. All requested samples, including failed/unstarted ones, are retained. Late output stays bound to the submitted pair/request. Matching repetitions remain `not_compared`; disagreeing complete repetitions are `unstable`. [Comparison](COMPARISON.md) validates stored bindings and recomputes semantic stability.

The store redacts only explicitly configured literals; it does not discover secrets. Redacted/truncated data cannot yield a complete receipt. Do not capture private inputs without a suitable store redaction policy. Host credentials and ambient Docker configuration do not enter the workload.

Cancellation joins attachment goroutines and removes exact owned containers and input images. A cleanup failure blocks later samples in that request. Daemon loss, host crash or forced termination can interrupt cleanup; retain resource identities and follow [sandbox recovery](SANDBOX.md#residual-limits-and-failure-recovery). A crash can leave sample artifacts without a final receipt; automatic cross-session recovery is not implemented. Storage failure is not durable completion.

## Reproduce and limits

With the separately provisioned image and explicitly configured local daemon:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
task runner:proof
```

This opt-in proof authorizes generated synthetic captures/lifecycle probes only; it does not pull the image. It checks real observations across two repetitions: identical responses, 12-hour provider counts 1/2, 30-second counts 1/1, candidate-test/driver edits and forged stdout/scratch output. Lifecycle probes cover namespace isolation, startup failure, output overflow, ready descendants at deadline/cancel, cleanup and an unrelated sentinel container. Premature app exit with a serving descendant and refused upstream connections must remain incomplete; app-generated 502 responses remain observable. Proxy transport failures abort rather than invent responses. Ordinary checks skip live Docker.

`task check:go` runs race tests for consent, immutable previews, late selection, unstable repetitions, failure states, redaction, cleanup refusal and cancellable concurrency. Existing live validation is macOS with a Colima Linux aarch64 daemon; Linux workstation, Docker Desktop and amd64 are not independently certified.

The daemon/CLI, image, host kernel and shipped observer are trusted. This is not a malware VM. Candidate traffic can disrupt or impersonate its own app endpoint; payloads are untrusted. The app cannot rewrite observer memory, files or output through shared networking. Port conflicts, flooding and malformed responses fail closed. Results cover finite sequential traffic, not later background effects, durable billing, real payments or all inputs. The launcher/proxy is app-side infrastructure, not a trusted source of provider counts. The complete raw diff remains independently usable.
