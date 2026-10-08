#!/usr/bin/env bash
# "All tests pass" after a change that also edited its tests. AFTER imports
# your go test -json output as reported evidence, flags the edited test as a
# potential oracle, and keeps the result separate from observed behavior.
# Requires Go (mise exec -- examples/cli/03-moved-oracle.sh).
source "$(dirname "$0")/../lib.sh"
command -v go >/dev/null || { echo "Go is required; run via mise exec --" >&2; exit 1; }
export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off

section "Checkout total: members get 10% off"
workspace moved-oracle
printf 'module example.com/shop\n\ngo 1.24\n' >go.mod
cat >price.go <<'EOF'
package shop

// Total applies the member discount to a cart subtotal in cents.
func Total(cents int, member bool) int {
	if member {
		return cents * 90 / 100
	}
	return cents
}
EOF
cat >price_test.go <<'EOF'
package shop

import "testing"

func TestMemberDiscount(t *testing.T) {
	if got := Total(1000, true); got != 900 {
		t.Fatalf("Total = %d, want 900", got)
	}
}
EOF
commit "member discount"

section "Candidate: discount becomes 15%, and the test is updated to agree"
perl -pi -e 's/90 \/ 100/85 \/ 100/' price.go
perl -pi -e 's/900/850/g' price_test.go
capture_ids "$(after 0 capture --json)"
inventory "$BASE" "$CANDIDATE"

section "You run the candidate's own tests (AFTER never runs them)"
note "Import reads redirected stdin and captures a binding by default; --producer is an optional caller claim."
go test -json ./... >"$WORK/candidate-tests.jsonl"
report=$(after 0 import --json <"$WORK/candidate-tests.jsonl" \
	--producer "$(go version); candidate tests on candidate code")
cards "$report"

section "Freeze the oracle: base tests against candidate code"
cp price_test.go "$WORK/candidate_test.go.txt"
git show HEAD:price_test.go >price_test.go
if go test -json ./... >"$WORK/frozen-tests.jsonl"; then
	echo "Expected the frozen oracle to fail" >&2; exit 1
else
	test "$?" -eq 1
fi
cp "$WORK/candidate_test.go.txt" price_test.go
frozen=$(after 0 import --json "$WORK/frozen-tests.jsonl" --snapshot "$CANDIDATE" \
	--producer "$(go version); base tests on candidate code")
cards "$frozen"

note "Both reports stay 'reported': AFTER records who said what, not that it ran on this snapshot."
note "The [potential oracle] flag is your cue to review the test edit as a behavior change."
note "Stored report (inspect it later by ID): $(jq -r .data.id <<<"$frozen")"
