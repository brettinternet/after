# Persistent expectations and conservative reopening

`internal/review` stores immutable pin revisions: human expectations stay separate from observations, applicability, execution and acceptance. The `pin` command and `review BASE CANDIDATE` open or update review state; neither executes project code. Only the [runner](RUNNER.md), after exact-plan authorization, can execute.

## Create and inspect a pin

A finite-example pin requires a complete observed runner receipt. `human_intent` can begin from an incomplete receipt, but neither scope turns prose into an executable assertion or universal proof. Imported test statuses cannot be a runner basis.

```sh
after pin RECEIPT_ID --project /work/payment \
  --expectation "Twelve-hour retries must not duplicate provider calls" \
  --reason "Preserve this concrete case"
after pin PIN_REVISION_ID --project /work/payment
```

`--scope` defaults to `finite_example`; when a receipt cannot support it, the CLI suggests `--scope human_intent`. Reasons and expectations are limited to 4,096 bytes. Use `compare RECEIPT_ID` and inspect its artifacts for exact outputs/effects. Human acceptance does not change comparison outcome; an accepted expectation is not a passing test.

Each mutation writes a new immutable revision ID. Use the returned ID for the next action. An older ID opens that exact historical state; it never means “latest.” Pin records contain the full bounded chronological history; there is no mutable latest pointer, file watcher or title-based association. A decision from an old revision creates a fork, not an overwrite. `review.Heads` derives all terminal heads by exact basis/history, not timestamp or title.

Bare `after pin` lists heads without selecting one. `after pin --expectation TEXT` can default its receipt to the newest run for the newest capture pair, but it never defaults a pin revision. Explicit IDs are required for decisions. Lookup scans at most 512 revisions / 16 MiB of pin records; corruption or a reached bound reports heads as unavailable, never a partial list. Inspection of an older revision names newer descendant heads but remains on the requested revision. Readable CLI/TUI share a card view; JSON remains the versioned record contract. IDs accept unique prefixes; see [CLI ID rules](CLI.md#short-ids-and-pin-heads).

The first pin event binds the original scenario, receipt and snapshot pair. Later selection and reruns cannot change the expectation or that basis. Legacy pins without explicit scope/context remain readable with unknown applicability; repin explicitly to use this workflow. No migration is inferred.

## Select, rerun, decide

Selection accepts a captured candidate for inspection, not its behavior. It reopens changed bindings, clears the current receipt and returns `missing_current_result: true`; it predicts no new output. Old receipts remain in history. Attach a new, authorized receipt and review it separately:

```sh
after pin PIN_REVISION_ID --project /work/payment \
  --select NEW_CANDIDATE_ID --mode original_base --reason "Inspect new capture"
after run BASE_ID NEW_CANDIDATE_ID --project /work/payment \
  --plan-out /work/payment/.after/rerun-preview.json
# Read the preview, then approve its exact full digest:
after run --project /work/payment \
  --plan-file /work/payment/.after/rerun-preview.json --approve sha256:<full-digest>
after pin REOPENED_REVISION_ID --project /work/payment \
  --attach NEW_RECEIPT_ID --reason "Attach authorized rerun"
after pin ATTACHED_REVISION_ID --project /work/payment \
  --accept --reason "Explicitly reviewed this finite result"
```

Only one action is allowed per `pin` command. `--mode` defaults to `original_base`; `last_inspected` compares the previous selected candidate with the new candidate. Both modes preserve the original pin basis and are recorded in history. `original_base` keeps the initial base. Changing base, renaming tests or changing labels never transfers acceptance. Returning to an earlier snapshot still requires explicit receipt attachment and review; it does not resurrect an old acceptance. A late or mismatched receipt cannot be attached, but stays inspectable.

Attachment checks selected bindings but never accepts. A different receipt attached to an accepted pin reopens it. Acceptance is a separate human action and requires a current complete receipt; failures and incomplete receipts may be attached but cannot be accepted. No command evaluates expectation text as an oracle.

The TUI's `u` confirmation shows both exact pair choices: `original base <id> → candidate <new-id>` (default) and `last inspected <id> → candidate <new-id>`. It requires a reason and advances the selected pin with `review.Select`. Late results stay bound to their submitted pair. See [the TUI review loop](TUI.md).

## Applicability and limits

Applicability is derived from records; receipt flags and human decisions cannot override it. Both exact snapshot IDs and scenario/input/driver/observer/rules, environment/toolchain/dependency digests and argv must match. Rule identities include masks. The implementation invalidates against the whole captured project: even a README edit can reopen evidence, with a reason. Exact complete bindings permit bounded reuse, not universal equivalence or automatic re-acceptance.

Incomplete snapshots, any excluded/unsupported entries, or missing runtime/dependency bindings produce `unknown`. This is stricter than completeness within capture's selected-file policy. Dependencies outside the repository must be represented in environment identities; leave them absent when their footprint is unknown. The payment runner binds captured source, frozen standard-library driver, pinned image and sandbox policy. Selecting an incomplete capture can reopen with unknown bindings, never predicted results or execution. Use the same run limits for selection and rerun preparation.

History has a 256-event cap and the existing 4 MiB pin-record bound; reaching a limit fails instead of pruning. The store also enforces private permissions, content hashes, atomic immutable publication, redaction and a single-writer lock. Digests are not signatures; these APIs consume the local trusted store, not arbitrary repository JSON.

`mise exec -- task check:go` covers whole-project identity invalidation, harmless edits, incomplete/unknown footprints, exact reuse, both selection modes, separate acceptance, invalid histories, late results and restart in a separate process. Unit receipts are synthetic. `mise exec -- task cli:proof` is the opt-in Docker proof of the real payment select/reopen/rerun/attach/accept flow across CLI process restarts; it requires the pinned image and explicit `AFTER_DOCKER_BINARY` / `AFTER_DOCKER_HOST`. Ordinary checks never contact Docker.
