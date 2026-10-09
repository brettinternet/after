# Frozen paired HTTP-service execution

`internal/runner` runs one bounded, versioned `http-service` scenario kind; the synthetic payment experiment is its built-in instance. It executes captured snapshots only after the operator approves an exact plan. Help, capture, import, inspection, comparison, previews and ordinary tests do not execute project code. See [sandbox policy](SANDBOX.md) for container limits.

## Prepare and authorize

`Prepare(store, pair, repetitions, limits)` is a convenience wrapper for the built-in payment definition. `PrepareDefinition(store, pair, raw, source)` accepts only an explicitly selected, strictly decoded v1 JSON definition. The bounded schema fixes a digest-pinned service image and Linux platform, optional build argv, required start argv, sanitized environment, fd3 readiness ABI, fixed epoch, fake upstreams, ordered cases/requests, supported channels, repetitions and time/output limits. Unknown fields, shell entrypoints/command strings, unpinned images and out-of-range data fail before a plan is built.

The CLI's `after run BASE CANDIDATE --definition FILE` reads exactly the operator-selected file; it never discovers a replacement from either snapshot. Its exact source bytes and digest are frozen independently of the pair. When the explicitly selected path is inside the project and differs between base and candidate, the preview labels it a changed oracle; neither snapshot chooses or authorizes a replacement. Saved plans reconstruct the stored definition and do not reread a definition path. Plan preparation writes frozen input/scenario records but never contacts Docker, builds a helper or executes project code. `Preview()` returns a private JSON copy and its digest. Any changed definition, snapshot, request, repetition count, limit, compiler recipe or sandbox template requires new consent. Candidate suites are never run.

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

The preview binds definition bytes/digest, service and compiler image/platforms, direct argv, fd3 readiness, compiler recipe, generated-file slots, observer topology, network policy, inputs and all time/output/transfer limits. Read and inspect the complete plan before approving. A digest supplied by repository data is not permission.

## Readiness ABI and isolation

Every service uses `fd3-http-url-v1`: write exactly one newline-terminated `http://127.0.0.1:PORT` URL to fd 3, then close it. Credentials, paths, queries, fragments, non-loopback hosts, reserved observer/proxy ports, overflow and readiness timeout fail closed. Optional build and required start commands are argv arrays executed directly, never shell strings. V1 supports only `linux/amd64` and `linux/arm64`; the separately provisioned service image must support the frozen platform. No emulation, image pull, dependency download or host fallback is added.

After exact consent and acquiring the process-wide execution gate, AFTER compiles its own static launcher once in the pinned Go image using embedded source and fixed offline compiler argv. It does not compile captured project code in this preparation container. Binary stdout is bounded to 8 MiB; compiler diagnostics use the definition's output limit; the definition separately budgets preparation time (90 seconds in the supplied definitions). Successful exit, complete output and exact-owner cleanup are required. The complete unredacted launcher, digest, preparation result and diagnostics are retained. The compiler runs once per approved request; no executable cache is used.

The resulting executable and the generated non-executable service configuration fill only fixed trusted slots: `/input/after/launcher` (`0555`, at most 8 MiB) and `/input/after/service.json` (`0444`, at most 64 KiB). Captured regular files stay read-only under `/input`; definitions cannot choose generated paths or executable modes. The app image comes from the frozen definition. Its optional build argv and start argv execute directly inside that app container with a rebuilt environment and scratch-only writes. The app reports readiness on fd 3 and remains alive through observation.

Each sample has a separately owned app and observer container. The Go observer is compiled from shipped AFTER source, reads the frozen definition, owns the controlled clock, drives the exact declared requests and records each declared fake upstream independently. It joins only the app's offline network namespace—not its PID, IPC, filesystem, input or scratch namespaces. The app reaches observer services through the bounded loopback proxy, which permits only request routes declared by the definition. Candidate source, tests, drivers, policies and printed counters never become observer code. No mounts, published ports, downloads or repository-selected shell commands are allowed. App stdout/stderr are diagnostics, not evidence; observer output comes from its Docker attachment.

## Receipt, output and failures

One process-wide cancellable gate permits one experiment at a time; an observation has at most two active workloads. Each container inherits sandbox CPU, memory, PID, scratch, time and bounded diagnostic output. Preparation binary stdout has its separate 8 MiB ceiling; its diagnostic stderr and every runtime diagnostic channel have explicit limits. The observer accepts at most 128 recorded calls, 4,096-byte bodies and bounded HTTP requests; overflow fails the observation.

Receipts bind request ID, original pair, frozen definition/source/digest, scenario, environment/toolchain/dependency identities, authorization plan and artifact references. Plans include preparation recipe, image/platform, templates, generated slots, container argv/input archives/policies, requests and limits. Review bindings remain template/recipe-derived. Receipts retain the preparation result, diagnostics and validated launcher artifact/digest; each sample retains the actual materialized app-plan identity and separate observation/diagnostic channels. Driver identity covers launcher and observer source. Fake-upstream counts come from the independent ordered log, never app output. Opening readable cards reads stored evidence; it does not rebuild or rerun.

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

`task runner:proof` authorizes only the built-in synthetic payment captures/lifecycle probes; `task comparison:proof` verifies its stored witnesses. Both use the separately provisioned Go image and never pull it. They check real observations across two repetitions: identical responses, 12-hour provider counts 1/2, 30-second counts 1/1, candidate-test/driver edits and forged stdout/scratch output. Lifecycle probes cover namespace isolation, startup failure, output overflow, ready descendants at deadline/cancel, cleanup and an unrelated sentinel container. Premature app exit with a serving descendant and refused upstream connections must remain incomplete; app-generated 502 responses remain observable. Proxy transport failures abort rather than invent responses.

`task http-service:proof` runs the no-third-party Python standard-library fixture on its separately provisioned digest-pinned Python image, then compares its independent response and fake-upstream witnesses. `task test:poc` requires this proof along with both existing production-code mutation controls; missing image/capabilities fail instead of skip or pull.

`task check:go` runs race tests for consent, immutable previews, late selection, unstable repetitions, failure states, redaction, cleanup refusal and cancellable concurrency. Existing live validation is macOS with a Colima Linux aarch64 daemon; Linux workstation, Docker Desktop and amd64 are not independently certified.

The daemon/CLI, image, host kernel and shipped observer are trusted. This is not a malware VM. Candidate traffic can disrupt or impersonate its own app endpoint; payloads are untrusted. The app cannot rewrite observer memory, files or output through shared networking. Port conflicts, flooding and malformed responses fail closed. Results cover finite sequential traffic, not later background effects, durable billing, real payments or all inputs. The launcher/proxy is app-side infrastructure, not a trusted source of provider counts. The complete raw diff remains independently usable.
