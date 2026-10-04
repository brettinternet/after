#!/usr/bin/env bash
# Review test reports next to the diff: the candidate's own green suite and a
# frozen-oracle run (base tests against candidate code) that fails.
# Requires Go (mise exec -- examples/tui/02-test-reports.sh).
source "$(dirname "$0")/../lib.sh"
command -v go >/dev/null || { echo "Go is required; run via mise exec --" >&2; exit 1; }
export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off

section "Base: shipping is free from \$50"
workspace tui-reports
printf 'module example.com/cart\n\ngo 1.24\n' >go.mod
cat >shipping.go <<'EOF'
package cart

// ShippingCents returns the shipping fee for a subtotal in cents.
func ShippingCents(subtotal int) int {
	if subtotal >= 5000 {
		return 0
	}
	return 599
}
EOF
cat >shipping_test.go <<'EOF'
package cart

import "testing"

func TestFreeShippingThreshold(t *testing.T) {
	for _, c := range []struct{ subtotal, want int }{{4999, 599}, {5000, 0}} {
		if got := ShippingCents(c.subtotal); got != c.want {
			t.Errorf("ShippingCents(%d) = %d, want %d", c.subtotal, got, c.want)
		}
	}
}
EOF
commit "free shipping from \$50"

section "Candidate: threshold moves to \$75; its test is rewritten to match"
perl -pi -e 's/5000/7500/' shipping.go
perl -pi -e 's/\{4999, 599\}, \{5000, 0\}/{7499, 599}, {7500, 0}/' shipping_test.go
capture_ids "$(after 0 capture)"
inventory "$BASE" "$CANDIDATE"

section "Reports you produce (AFTER only imports them)"
go test -json ./... >"$WORK/candidate.jsonl" || true
report=$(after 0 import "$WORK/candidate.jsonl" --snapshot "$CANDIDATE" --producer "$(go version); candidate suite")
cards "$report"
cp shipping_test.go "$WORK/candidate_test.go.txt"
git show HEAD:shipping_test.go >shipping_test.go
go test -json ./... >"$WORK/frozen.jsonl" || true
cp "$WORK/candidate_test.go.txt" shipping_test.go
frozen=$(after 0 import "$WORK/frozen.jsonl" --snapshot "$CANDIDATE" --producer "$(go version); base tests on candidate code")
cards "$frozen"

cat >&2 <<EOF

Try in the TUI:
  j/k        rows 1-2 are the candidate suite (STATE report=pass), rows 3-4 the frozen run (report=fail)
  Enter      inspect a row: producer, output, and inputs/effects "unavailable" (reported, not observed)
  d          inventory: shipping_test.go is a potential oracle
  c          capture again after editing the checkout in another terminal, e.g.
               cd $PROJECT && echo '// free over \$75' >> shipping.go
  a          accept that capture as the reviewed candidate (selection, not behavior approval)
EOF
open_tui "$CANDIDATE" --base "$BASE" --evidence "$(jq -r .data.id <<<"$report")" --evidence "$(jq -r .data.id <<<"$frozen")"
