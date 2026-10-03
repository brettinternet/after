# Exact finite comparisons

`internal/compare.Run(store, receiptID)` implements AFTER-9. It reads validated
immutable receipts and snapshots, never executes code, and persists a comparison
record plus a bounded `comparison-details-v1` artifact. The [headless CLI](CLI.md)
exposes `compare`, `inspect`, and `export`; none executes project code. The store
must be writable to publish the result and idempotently reconstruct
frozen input/scenario records. Inspection alone must not call the runner executor.

## Supported policy and compatibility

The shipped `runner.ComparisonRules` JSON is the only accepted policy. Its exact
bytes are retained as a `comparison-rules` artifact and hashed into the scenario
and receipt. A policy edit changes that digest, the scenario and authorization
plan. Both old and new artifacts can be inspected or passed to `compare.JSON`
for a precise policy diff. Arbitrary masks, field dropping and normalization
rules are **not supported**, including candidate-owned rules. Unknown/older
policies are incomparable, never silently upgraded. Earlier AFTER-8 receipts
with its placeholder rules require a newly authorized run for comparison.

Before comparing, the engine reconstructs the shipped plan from the captured
sources, recorded limits, repetition count and original request ID. Exact plan
bytes, authorization, scenario/input/driver/observer/rules bindings, toolchain,
argv and each source-bound environment/dependency digest must match. Environment
hashes include the source archive: they are not simply required to be identical
across different versions, nor ignored. No Docker call or project build occurs
in this verification. Missing sources, unknown plans, incomplete capture or
incompatible environments cannot prove equality.

Every expected side/case/repetition must have one complete metadata and
observation artifact with matching receipt/request/pair/time bindings. Each sample's
app and observer execution-plan identities must match its reconstructed side/case.
Required observation fields must be present and non-null with the correct type;
explicit empty bodies/keys and zero timestamps remain valid values. Duplicate,
new, missing, malformed or unsupported channels fail closed. Report imports,
failed execution, redaction and truncation remain incomparable. Store corruption
or unavailable referenced data returns an error; the raw inventory/diff remains
a separate API, not dependent on comparison success.

## Exact witnesses

The detail artifact links its receipt and both source inventories, retains every
receipt artifact reference, and contains paired and within-side repetition
witnesses. Each witness identifies both sample metadata and observation blobs.
All samples are inspectable, including those associated with failed comparisons.
The receipt leads to the snapshot file inventory and captured raw diff.

- Responses: ordered HTTP status and body. Valid JSON bodies compare structurally;
  other bodies compare as exact text. JSON and text are distinct representations.
- Provider: ordered timestamp, method, fixed observer destination
  `127.0.0.1:18082`, recorded path, idempotency key and body. Count is the independent
  log length, not an app-reported number. Identical responses do not conceal a
  changed count, destination path, operation, payload or ordering.
- JSON: sorted object-key traversal; arrays retain order; null differs from
  missing; exact decimal values without floating-point conversion; Unicode code
  points are not normalized. `1`, `1.0` and `1e0` compare equal, but large adjacent
  integers remain distinct. Original numeric tokens survive in witness values.
  RFC 6901 pointers escape `~` and `/`; absent values are omitted, unlike `null`.
  Duplicate keys, invalid UTF-8 and unpaired UTF-16 escapes are rejected.

Paired differences produce `different`, not a regression judgement. Disagreeing
repetitions on either side take precedence as `unstable`; all paired and
repetition witnesses remain available. Repetition equality is recomputed under
the frozen semantic policy, rather than treating the runner's byte-level
instability hint as a semantic difference (e.g. JSON key order alone).
Missing/incompatible evidence takes precedence over conclusive outcomes. A new
redaction policy hiding comparison details makes the persisted record and detail
summary incomparable. Never treat a partial witness list as complete equality.

## Bounds and finite scope

JSON inputs and the stored report are limited to 1 MiB; nesting to 64 levels;
nodes to 32768; number tokens to 1024 bytes and exponents to signed 32-bit values;
witnesses to 2048 changes per comparison. Decimal comparison does not allocate
exponent-sized numbers. Samples retain the runner's 1–5 repetitions, two cases,
128 provider-call and 4096-byte body limits. Limit failures are explicit, not
silent truncation. Oversized detail publication returns an error without a
conclusive record; the receipt remains inspectable.

These are two sequential synthetic same-key requests at 12h and 30s, not universal
safety, causation or performance claims. The provider channel does not record
query strings, other headers, other destinations or effects after its observation
window. Recorded paths and the known observer endpoint are not a general network
trace. Receipt hashes are bindings, not producer authentication. No model,
account, network service or API key is involved. Terminal clients must escape all
untrusted strings; this package returns data, not terminal-ready text.

## Verification

`mise exec -- task check:go` covers deterministic golden/property tests, exact
numbers/Unicode, missing/new channels, failures, malicious rules, repetitions,
redaction and stored artifact/inventory links. `mise exec -- task comparison:fuzz`
runs the bounded JSON determinism/reflexivity fuzz target.

With the pinned sandbox image already provisioned and explicit local Docker
binary/endpoint, `mise exec -- task comparison:proof` authorizes only synthetic
payment execution. It stores real observations, compares two repetitions, and
requires identical responses, 12h counts 1 versus 2, 30s counts 1 versus 1, and
16 paired/repetition channel witnesses. It uses the same `AFTER_DOCKER_BINARY`
and `AFTER_DOCKER_HOST` setup as [the runner proof](RUNNER.md). Ordinary tests do
not execute Docker or project code. The live proof was run on macOS with a Colima
Linux aarch64 daemon; it is not certification of other host/daemon platforms.
