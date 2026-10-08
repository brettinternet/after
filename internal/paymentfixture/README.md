# Synthetic payment-retention experiment

This is an executable fixture, not preloaded evidence or production billing.
`testdata/payment/app` is a standard-library Go HTTP application with in-memory
idempotency storage. `testdata/payment/driver` is the frozen HTTP driver,
controlled clock and independently recording fake provider. The application runs
as a separate child process; the driver never reads its printed request count.

## Reproduce

First provision the pinned image separately as described in the
[sandbox instructions](../../docs/SANDBOX.md). No image is pulled by this command.
With a trusted local Linux Docker daemon and explicitly chosen CLI/socket:

```sh
AFTER_DOCKER_BINARY=/usr/bin/docker \
AFTER_DOCKER_HOST=unix:///var/run/docker.sock \
task fixtures:payment
```

On macOS, substitute the absolute installed Docker CLI path and the Colima Unix
socket reported by `docker context inspect colima`. The command authorizes only
these synthetic plans and logs every frozen plan/digest before executing it.
Missing images, unsupported isolation or failed execution fail the check; there
is no host fallback. No credentials, real payments, external network services,
network dependency installation or sleeps for simulated hours are required.
Only ephemeral loopback HTTP servers inside `--network=none` are used.

The test generates a disposable Git repository with real commits, captures its
revisions through AFTER's capture/store APIs, reads only the captured blobs, and
runs four offline sandbox plans. No committed repository SHA is hardcoded:

- Base: 24-hour retention.
- Candidate: 5-minute retention, with all other source bytes unchanged.
- Mutation: base retention with app deduplication disabled.
- Mutation: base retention with the app-printed count changed from `1` to `999`.

The driver/provider source is byte-identical across all four. The recorder
appends each received request without consulting any expected count or doing
its own deduplication. Assertions outside the sandbox check the actual records.
The counts below are **expected checks**, not data fed into the observer.

Ordinary `task test` checks snapshot generation and frozen inputs
without contacting Docker, building the fixture, or executing a captured app.
The opt-in command is required for behavioral evidence. Each sandbox has a
180-second total limit and 64 KiB combined output limit; it builds offline inside
the existing non-root, read-only, bounded-memory/PID/CPU policy. Each app case
has a 15-second process deadline and two-second HTTP request timeouts. Processes,
containers and input-only images are cleaned on normal completion/failure.

## Concrete inputs and supported observations

Each case starts at synthetic `2025-01-01T00:00:00Z` with a new app process,
empty key map, independent empty provider log, and new clock/provider servers.
Every action sends:

```http
POST /payments
Idempotency-Key: synthetic-key-a
Content-Type: application/json

{"amount_cents":1200,"currency":"USD"}
```

The different-key control changes only the second key to `synthetic-key-b`.
The fake provider always responds `200 {"payment":"synthetic-accepted"}`; the
app returns that body both for new and cached requests. This stable provider
response deliberately allows identical HTTP outputs despite changed effects.
It does not suppress or combine provider requests.

| Case                                 | Action offsets from initial time (seconds) | Base calls | Candidate calls |
| ------------------------------------ | ------------------------------------------ | ---------- | --------------- |
| Same key after twelve hours          | 0, 43200                                   | 1          | 2               |
| Thirty-second same-key control       | 0, 30                                      | 1          | 1               |
| Expired-key control                  | 0, 90000                                   | 2          | 2               |
| Just before five minutes             | 0, 299                                     | 1          | 1               |
| Exactly five minutes                 | 0, 300                                     | 1          | 2               |
| Just before twenty-four hours        | 0, 86399                                   | 1          | 2               |
| Exactly twenty-four hours            | 0, 86400                                   | 2          | 2               |
| Different-key control                | 0, 30                                      | 2          | 2               |
| Cached retry does not refresh expiry | 0, 240, 360                                | 1          | 2               |

The clock lives in the driver; the app fetches it for each payment and uses that
single value for both insertion and expiry in its own key map. A key expires
when `now >= created + retention`; a cached retry does not refresh creation time.
The driver advances the clock only between completed requests. The provider log
uses that same clock. There is no wall-clock TTL store or separate expiry clock.

Disabling deduplication produces a provider call for **every** action, including
two in the thirty-second control and three in the no-sliding-expiry case.
Changing the app-printed count leaves all independently captured observations
identical to the base. App stdout is discarded, never parsed as evidence.

The observed channels are HTTP response status/body and provider-received
method, path, key, raw body and controlled time. Counts are derived from the
length of that traffic log. The proof checks exact received inputs/times and
identical response status/body across revisions, not just a summarized counter.

## Limits and next integration

All data is synthetic. These are sequential bounded experiments, not concurrent
billing, durable transactions, real provider behavior, retries under network
failure, production charge counts or proof over all keys/inputs. No database,
filesystem-effect channel, money movement or production credentials exist.
The fixture deliberately omits production validation and idempotency conflicts
for different payloads sharing a key.

Separate processes establish an independent recording path for this known
synthetic app, **not a security boundary against a malicious candidate**. The app
and observer still share container UID, scratch and network namespace; an
adversarial app could attack the observer or write misleading stderr. Such output
is not authenticated evidence. The [AFTER-8 paired runner](../../docs/RUNNER.md) now protects the frozen observer
in a separate container and persists bound receipts for this payment ABI; it does
not turn this original single-container fixture proof into a general runner. This fixture does not create product receipts, freshness badges,
comparisons or human acceptance decisions. Normal cancellation uses sandbox
cleanup; abrupt host/daemon loss has the recovery limits documented there.

Validated on macOS with a Colima Linux aarch64 daemon using the pinned Go image.
Native Linux workstation, Docker Desktop and amd64 execution are not separately
certified by this run.
