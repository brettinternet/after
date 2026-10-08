#!/usr/bin/env bash
# Prepare a workspace for a VHS demo tape and print its project path. Uses the
# same fixtures as the example scripts.
# Usage: examples/demos/setup.sh payment|oracle|review
source "$(dirname "$0")/../lib.sh"

case ${1:-} in
payment)
	payment_project demo-payment
	set_config 300 true 1 # 24h -> 5 min idempotency retention
	;;
oracle)
	discount_project demo-oracle
	frozen_tests "$WORK/frozen.jsonl" # base tests on candidate code: ../frozen.jsonl
	;;
review) messy_project demo-review ;;
*) echo "usage: $0 payment|oracle|review" >&2; exit 2 ;;
esac
printf '%s\n' "$PROJECT"
