#!/usr/bin/env bash
# The full headless review loop: pin an expectation against observed behavior,
# watch a later edit reopen it, rerun with consent, attach, then accept.
# Every pin/review step returns a NEW immutable revision ID; always use the latest.
source "$(dirname "$0")/../lib.sh"
require_docker

state() {
	jq -r '.data | "   revision \(.pin.id[0:15])...  decision=\(.pin.decision)  applicability=\(.applicability)  missing_current_result=\(.missing_current_result)\n   \(.reason)"' <<<"$1" >&2
}

section "1. A candidate shortens idempotency retention to 5 minutes"
payment_project review-loop
set_config 300 true 1
capture_ids "$(after 0 capture)"
first=$(run_pair "$BASE" "$CANDIDATE" 4)
provider_counts "$first"

section "2. Reviewer pins the behavior they expect (the candidate violates it)"
view=$(after 0 pin "$(jq -r .data.receipt.id <<<"$first")" --scope finite_example \
	--expectation "A same-key retry after 12h makes exactly one provider charge" \
	--reason "Clients retry for up to 24h; duplicate charges are unacceptable")
state "$view"

section "3. Author fixes retention; the new capture reopens the pin"
set_config "24 * 60 * 60" true 1
echo "// Retention must outlive the 24h client retry window." >>app/config.go
capture_ids "$(after 0 capture)"
view=$(after 0 pin "$(jq -r .data.pin.id <<<"$view")" --select "$CANDIDATE" \
	--mode original_base --reason "Review the retention fix")
state "$view"
note "Stale and missing: AFTER does not predict the fix's behavior from old receipts."

section "4. Rerun the pin's pair: its ORIGINAL base versus the fixed candidate"
original_base=$(jq -r '.data.pin.history[-1].review.target.snapshots.base' <<<"$view")
second=$(run_pair "$original_base" "$CANDIDATE" 0)
provider_counts "$second"

section "5. Attach the new receipt (still not accepted), then accept explicitly"
view=$(after 0 pin "$(jq -r .data.pin.id <<<"$view")" \
	--attach "$(jq -r .data.receipt.id <<<"$second")" --reason "Attach authorized rerun")
state "$view"
view=$(after 0 pin "$(jq -r .data.pin.id <<<"$view")" --accept \
	--reason "12h retry now makes one charge; 30s control unchanged")
state "$view"

section "History is append-only: every decision and its reason"
jq -r '.data.pin.history[] | "   \(.decision | . + " " * (9 - length)) \(.review.action // "pin" | . + " " * (8 - length)) \(.reason)"' <<<"$view" >&2
note "Inspect any pin revision read-only: after pin <revision-id> --project $PROJECT"
