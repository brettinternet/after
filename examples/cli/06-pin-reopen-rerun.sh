#!/usr/bin/env bash
# The full headless review loop: pin an expectation against observed behavior,
# watch a later edit reopen it, rerun with consent, attach, then accept.
# Every pin decision appends a NEW immutable revision; always act on the head.
source "$(dirname "$0")/../lib.sh"
require_docker

# pin_head: the current pin revision (this example keeps a single chain).
pin_head() { short "$(after_json 0 pin | jq -r '.data.pins[0].pin.id')"; }
# decide ARGS...: show a pin decision, then the compact pin list instead of the
# full pin card the decision prints.
decide() {
	prompt "after $(shown pin "$@")"
	"$AFTER_BIN" pin "$@" >/dev/null
	after 0 pin
}

intro "Pin what matters, and know when it needs another look" \
	"A reviewer pins \"a 12h retry charges once\"; the author's fix reopens the pin until it is rerun."
payment_project review-loop
set_config 300 true 1

step "A candidate shortens idempotency retention to 5 minutes; observe it"
after 0 capture
base=$(short "$(after_json 0 status | jq -r .data.base_snapshot.id)")
run_note
after 4 run

step "Pin the behavior you expect (this candidate violates it)"
note "On a terminal, a bare after pin RECEIPT offers numbered expectations from the run."
note "Each decision prints the full pin card; this script shows the compact pin list instead."
decide --expectation "A same-key retry after 12h makes exactly one provider charge" \
	--reason "Clients retry for up to 24h; duplicate charges are unacceptable"

step "The author restores 24h retention; select the new capture"
set_config "24 * 60 * 60" true 1
echo "// Retention must outlive the 24h client retry window." >>app/config.go
after 0 capture
candidate=$(short "$(after_json 0 status | jq -r .data.candidate_snapshot.id)")
decide "$(pin_head)" --select "$candidate" --mode original_base --reason "Review the retention fix"
note "Stale and missing: AFTER does not predict the fix's behavior from old receipts."

note "Same base commit, new snapshot ID: snapshots include the paired diff. The pin keeps its original base."

step "Rerun the pin's pair: its original base versus the fixed candidate"
run_note
after 0 run "$base" "$candidate"
receipt=$(newest run)

step "Attach the new receipt (still not accepted), then accept explicitly"
decide "$(pin_head)" --attach "$receipt" --reason "Attach authorized rerun"
decide "$(pin_head)" --accept --reason "12h retry now makes one charge; 30s control unchanged"

takeaway "the fix, once, with no memory of why it mattered" \
	"a pinned expectation that reopened on the edit, was rerun on fresh evidence, and accepted with reasons"
note "History is append-only. Inspect any revision read-only: after pin REVISION --project $PROJECT"
