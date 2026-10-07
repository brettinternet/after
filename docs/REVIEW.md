# Persistent expectations and conservative reopening

`internal/review` and the `pin` command separate human intent, immutable
receipts, applicability, and explicit snapshot selection. `review BASE CANDIDATE`
opens the terminal browser. They never
execute project code. The [runner](RUNNER.md) remains the only execution path,
with its exact-plan authorization and offline isolation unchanged.

## Pin and inspect

Bare `after pin` lists only computed head revisions, including every fork; it does
not select a revision or choose one by timestamp. Use an explicit `PIN_REVISION_ID`
for inspection or any decision. `after pin --expectation TEXT` can default its
receipt to the newest run of the newest capture pair, but that receipt default
never implies a pin-revision default. When no capture/run exists, the CLI names the
missing record and the command needed to create it.

```sh
./bin/after pin RECEIPT_ID --project /work/payment \
  --expectation "Twelve-hour retries must not duplicate provider calls" \
  --reason "Preserve this concrete case"
./bin/after pin PIN_REVISION_ID --project /work/payment
```

Every mutation returns a new immutable `data.pin.id`. Use that ID for the next
operation; an older ID deliberately opens historical state, never an implicit
"latest" selection. Revisions contain the entire bounded chronological history;
no mutable latest-pointer, file watcher, or title-based association exists.
Branches from an old revision are explicit historical forks, not overwrites.
`review.Heads` derives revisions that no other stored revision extends, retaining
all forks. It compares the exact original basis and history events, not titles or
timestamps. Readable inspection of an older revision lists its newer descendant
heads without switching the requested revision; JSON remains unchanged. Each
lookup scans at most 512 revisions and 16 MiB of pin records. Corruption or a
reached bound is reported as unavailable, never as a complete head list.
All CLI record IDs also accept [unique prefixes](CLI.md#short-ids-and-pin-heads).
The first event binds the original scenario, receipt and snapshot pair. Neither
later selection nor rerun can change the expectation or that original basis.

`--scope` defaults to `finite_example`, which requires a complete observed runner
receipt; the error suggests `--scope human_intent` when that receipt is incomplete.
`human_intent` records a broader requirement and may start from an incomplete runner receipt.
Neither scope makes the prose an executable assertion or proves a universal
property. Inspect the receipt's artifact IDs for actual outputs/effects and use
`compare RECEIPT_ID` for deterministic witnesses. A human decision does not alter
the comparison result; an accepted expectation is not an automatically passing
test. Imported test names/statuses cannot serve as a bound runner basis.

## Select, rerun, then decide

```sh
./bin/after pin PIN_REVISION_ID --project /work/payment \
  --select NEW_CANDIDATE_ID --mode original_base --reason "Inspect new capture"
# Use the returned revision ID, and the target snapshot pair from its last event.
./bin/after run BASE_ID NEW_CANDIDATE_ID --project /work/payment \
  --plan-out /work/payment/.after/rerun-preview.json
# Read the preview, then explicitly authorize its exact digest:
./bin/after run --project /work/payment \
  --plan-file /work/payment/.after/rerun-preview.json --approve sha256:...
./bin/after pin REOPENED_REVISION_ID --project /work/payment \
  --attach NEW_RECEIPT_ID --reason "Attach authorized rerun"
./bin/after pin ATTACHED_REVISION_ID --project /work/payment \
  --accept --reason "Explicitly reviewed this finite result"
```

Only one action is allowed per `pin` command. `--mode` defaults to
`original_base`; `--reason` is optional, and its command-line default is recorded
verbatim in history. Selection accepts a captured snapshot for inspection, **not its
behavior**. Changed selections reopen the pin, clear the current receipt and
return `missing_current_result: true`; no predicted before/after values appear.
Old receipts remain in history at their original IDs. An authorized rerun creates
a new receipt; attachment checks its exact selected bindings but never accepts
the pin, and attaching a different receipt to an accepted pin reopens it.
Acceptance is a separate human action requiring a current complete
receipt. Failures and incomplete receipts may be attached but cannot support
acceptance. No command evaluates expectation prose as a test oracle.

`original_base` always keeps the pin's original base. `last_inspected` compares
the previous selected candidate to the new candidate. Both retain the original
pin basis and explicit mode in history. A base change, renamed test or scenario
label never transfers acceptance. Returning to an earlier snapshot also requires
explicit receipt attachment and review, rather than silently resurrecting an old
accepted decision. Late/mismatched receipts are rejected for attachment and
remain separately inspectable in storage.

## Applicability and limits

The view derives applicability; receipt flags and human decisions cannot override
it. Both sides' exact snapshot IDs, scenario/input/driver/observer/rule identities,
environment/toolchain/dependency digests and argv must match. Rule identities
include masks. This first implementation uses whole-project invalidation: even a
harmless README edit can reopen evidence, with an explicit explanation. Identical
complete bindings permit bounded reuse, never universal equivalence or automatic
acceptance after reopening.

Incomplete snapshots, any inventoried exclusions/unsupported entries, or missing
runtime/dependency bindings produce `unknown`. This is deliberately stricter than
capture completeness within its selected-file policy. Dependencies outside the
captured project must be represented in the environment identities; API callers
must leave an environment absent when that footprint is unknown. The shipped
payment runner binds the whole captured source, frozen standard-library driver,
pinned image and sandbox policy. CLI selection reconstructs that runner's planned
identities without executing. Unsupported/incomplete captures still select and
reopen with unknown bindings; they do not gain predicted results or execution.
Use the same run-limit configuration when selecting and preparing the rerun.

Legacy pins without scope/context remain readable with unknown applicability.
Repin explicitly to use the new workflow; there is no inferred migration. Reasons
and expectations are limited to 4096 bytes; history is limited to 256 events and
the existing 4 MiB record bound. Reaching a limit fails rather than pruning old
history. Storage keeps its private permissions, content hashes, atomic immutable
publication, redaction, and single-writer process lock. Digests are not signatures:
these APIs consume the local trusted store, not arbitrary repository JSON.

## Verification

`mise exec -- task check:go` covers parameterized identity invalidation, harmless
edits, incomplete/excluded/unknown footprints, exact reuse, original-base versus
follow-up selection, separate acceptance, invalid histories, late results,
immutable receipts and restart in a separate process. Unit receipts are explicitly
synthetic, not claimed execution evidence.

With the provisioned pinned Docker image and explicit `AFTER_DOCKER_BINARY` and
`AFTER_DOCKER_HOST`, `mise exec -- task cli:proof` executes the real payment fixture
and the full pin/select/reopen/authorized-rerun/attach/accept flow across native
CLI process restarts. It retains the original one-versus-two twelve-hour provider
observations and unchanged thirty-second control, then reruns the original pair
against itself. The new receipt is attached without accepting the expectation;
only the following human-decision command accepts it. Ordinary checks never
contact Docker. The [TUI review loop](TUI.md) exposes finite-count pinning, deliberate snapshot selection, and exact-plan authorized reruns over the same immutable records.
