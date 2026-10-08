#!/usr/bin/env bash
# "All tests pass" after a change that also edited its tests. AFTER imports your
# go test -json output as reported evidence, flags the edited test as a
# potential oracle, and keeps reports separate from observed behavior.
# Requires Go (mise exec -- examples/cli/03-moved-oracle.sh).
source "$(dirname "$0")/../lib.sh"

intro "All tests pass, because the test changed too" \
	"Members get 10% off. A candidate makes it 15% and edits the test to agree."
discount_project moved-oracle

step "The change: one line of code and the assertion that checked it"
after 0 diff

step "The candidate's own tests are green"
shell 0 "go test ./..."
note "AFTER never runs your tests. Pipe the JSON output in as a reported result:"
shell 0 "go test -json ./... | after import --producer 'candidate tests'"

step "The edited test is flagged as a potential oracle"
after 0 inspect

step "Freeze the oracle: run the base tests against the candidate code"
note "(The script swaps in the base test, runs go test -json, and restores the candidate.)"
frozen_tests "$WORK/frozen.jsonl"
shell 0 "after import ../frozen.jsonl --producer 'base tests on candidate code'"

takeaway "two edited files and a green test run" \
	"the test edit is a potential oracle, and the original tests fail: behavior changed"
note "Both reports stay 'reported': AFTER records who said what, not that it ran here."
