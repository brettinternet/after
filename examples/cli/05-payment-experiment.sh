#!/usr/bin/env bash
# Observe behavior, not just diffs: run a payment service's base and candidate
# in AFTER's offline sandbox and compare HTTP responses AND provider requests.
#
#   retention  24h -> 5min idempotency window: identical responses, duplicate charge (exit 4)
#   refactor   24*60*60 -> 86400 rewrite: behavior equal on the frozen cases (exit 0)
#   no-dedup   idempotency disabled: even the 30s control double-charges (exit 4)
#   fake-log   app prints a different request count: the observer ignores app logs (exit 0)
#
# Usage: examples/cli/05-payment-experiment.sh [retention|refactor|no-dedup|fake-log]
source "$(dirname "$0")/../lib.sh"
variant=${1:-retention}
case $variant in
retention)
	change=(300 true 1) want=4
	diff_shows="one constant: 24 * 60 * 60 → 300"
	after_shows="identical HTTP responses, but a retry after 12h now charges twice"
	;;
refactor)
	change=(86400 true 1) want=0
	diff_shows="a rewritten constant you have to check by hand"
	after_shows="responses and provider requests equal on every frozen case"
	;;
no-dedup)
	change=("24 * 60 * 60" false 1) want=4
	diff_shows="one flag: deduplicate = false"
	after_shows="identical HTTP responses, but every retry charges again, even after 30s"
	;;
fake-log)
	change=("24 * 60 * 60" true 999) want=0
	diff_shows="the app now reports 999 provider requests"
	after_shows="the provider still received one request per case: AFTER observes, it does not trust logs"
	;;
*) echo "unknown variant: $variant" >&2; exit 2 ;;
esac
require_docker

intro "Same responses, different behavior ($variant)" \
	"Clients retry a payment with the same Idempotency-Key; a retry must never charge twice."
payment_project "payment-$variant"
set_config "${change[@]}"

step "The change looks harmless"
after 0 diff
after 0 capture

step "Run base and candidate in the offline sandbox"
note "Frozen experiment: POST /payments, then retry the same key after 12h and after 30s."
note "AFTER's observer owns the clock and a fake provider that records every charge request."
run_note
after "$want" run

takeaway "$diff_shows" "$after_shows"
note "Exit $want. after compare re-derives the result from the stored receipt without running anything;"
note "after export writes the machine-readable bundle."
