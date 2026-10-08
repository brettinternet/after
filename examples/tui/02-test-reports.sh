#!/usr/bin/env bash
# Review test reports next to the diff: the candidate's own green suite and a
# frozen-oracle run (base tests against candidate code) that fails.
# Requires Go (mise exec -- examples/tui/02-test-reports.sh).
source "$(dirname "$0")/../lib.sh"
command -v go >/dev/null || { echo "Go is required; run via mise exec --" >&2; exit 1; }
export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off

intro "Test reports beside the diff" \
	"Shipping is free from \$50. A candidate moves the threshold to \$75 and rewrites its test to match."
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

perl -pi -e 's/5000/7500/' shipping.go
perl -pi -e 's/\{4999, 599\}, \{5000, 0\}/{7499, 599}, {7500, 0}/' shipping_test.go

step "Import the reports you produce (AFTER only reads them)"
shell 0 "go test -json ./... | after import --producer 'candidate suite'"
report=$(newest report)
frozen_tests "$WORK/frozen.jsonl"
shell 0 "after import ../frozen.jsonl --producer 'base tests on candidate code'"
frozen=$(newest report)
pair=$(after_json 0 status | jq -r '.data.capture | "\(.base_snapshot) \(.candidate_snapshot)"')
read -r base candidate <<<"$pair"

tui_keys <<EOF
  1/2/3/4    Overview, Changes, Diff, Activity; Tab/Shift+Tab cycle views
  /, n/N     search the current list or document; next/previous match
  j/k Enter  inspect a row: producer, output, and inputs/effects "unavailable" (reported, not observed)
  2          Changes shows shipping_test.go as a potential oracle
  c          capture again after editing the checkout in another terminal, e.g.
               cd $PROJECT && echo '// free over \$75' >> shipping.go
  u          choose original base or last inspected with Left/Right; confirm with a reason
  4          inspect capture/selection activity and full references
EOF
open_tui "$(short "$base")" "$(short "$candidate")" "$report" "$frozen"
