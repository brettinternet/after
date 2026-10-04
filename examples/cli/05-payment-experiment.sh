#!/usr/bin/env bash
# Observe behavior, not just diffs: run a payment service's base and candidate
# in AFTER's offline sandbox and compare HTTP responses AND provider requests.
#
#   retention  24h -> 5min idempotency window: identical responses, duplicate charge (exit 4)
#   refactor   86400 -> 24*60*60 rewrite: behavior equal on the frozen cases (exit 0)
#   no-dedup   idempotency disabled: even the 30s control double-charges (exit 4)
#   fake-log   app prints a different request count: the observer ignores app logs (exit 0)
#
# Usage: examples/cli/05-payment-experiment.sh [retention|refactor|no-dedup|fake-log]
source "$(dirname "$0")/../lib.sh"
variant=${1:-retention}
case $variant in
retention) change=(300 true 1) want=4 ;;
refactor) change=(86400 true 1) want=0 ;;
no-dedup) change=("24 * 60 * 60" false 1) want=4 ;;
fake-log) change=("24 * 60 * 60" true 999) want=0 ;;
*) echo "unknown variant: $variant" >&2; exit 2 ;;
esac
require_docker

section "Payment API: retries with the same Idempotency-Key must not charge twice"
payment_project "payment-$variant"
set_config "${change[@]}"
capture_ids "$(after 0 capture)"
patch "$BASE" "$CANDIDATE"

section "Frozen experiment: POST /payments, then retry the same key after 12h and after 30s"
note "AFTER's observer owns the clock and a fake provider that records every charge request."
result=$(run_pair "$BASE" "$CANDIDATE" "$want")
receipt=$(jq -r .data.receipt.id <<<"$result")

section "Observed provider requests (independent log, not app output)"
provider_counts "$result"

section "Deterministic comparison of the stored receipt (no execution)"
comparison=$(after "$want" compare "$receipt")
note "outcome: $(jq -r .data.comparison.outcome <<<"$comparison")"
witnesses "$comparison"

note "Receipt: $receipt"
note "Machine-readable bundle: after export $(jq -r .data.comparison.id <<<"$comparison") --project $PROJECT"
