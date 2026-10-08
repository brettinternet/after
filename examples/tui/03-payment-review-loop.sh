#!/usr/bin/env bash
# Guard known-good behavior: observe the payment service, pin "a 12h retry
# makes one provider charge", then let a later edit reopen the pin and rerun it
# from the TUI to catch the regression. Requires the offline Docker sandbox.
source "$(dirname "$0")/../lib.sh"
require_docker

intro "Guard known-good behavior from the TUI" \
	"Pin \"a 12h retry charges once\", let a later edit reopen it, and rerun to catch the regression."
payment_project tui-payment

step "Observe the current service (base and candidate are both 24h retention)"
after 0 capture
run_note
after 0 run
ids=$(after_json 0 status | jq -r '"\(.data.capture.base_snapshot) \(.data.capture.candidate_snapshot) \(.data.comparison.id)"')
read -r base candidate comparison <<<"$ids"

# A later "small" change, applied from another terminal while the TUI is open.
regress=$WORK/apply-regression.sh
cat >"$regress" <<EOF
#!/bin/sh
# Simulates an agent "tidying" retention to 5 minutes.
cd $(printf %q "$PROJECT") && perl -pi -e 's/24 \\* 60 \\* 60/5 * 60/' app/config.go && git diff --stat
echo "Edited app/config.go; press c in the TUI."
EOF
chmod +x "$regress"

tui_keys <<EOF
  1. Expand an Overview group with Enter if needed, then inspect a case row:
     exact responses and provider calls. Esc. Use / to search; n/N cycles matches.
  2. On the 43200s (12h) row, p: review the exact receipt/pair, then Enter pins its
     measured count (1). Ctrl-U clears the prefilled reason; Esc cancels. Press p again
     to see the duplicate-basis refusal and existing pin ID.
  3. In another terminal:  $regress
  4. c captures the edit; u shows the new candidate ID and changed-path count.
     Left/Right chooses original base (use this here) or last inspected; Enter
     confirms with the editable reason, Esc cancels. Find the pin under Needs
     another look: reopened | stale | missing current evidence. Enter shows why.
  5. r previews the offline rerun; Tab switches Summary/Exact plan.
     n denies (nothing runs), y approves the retained plan once.
     Navigation stays live during the run; x cancels it.
  6. New rows appear: 43200s base=[1] candidate=[2] breaks the pinned expectation;
     the 30s control is unchanged. Leave this regression unaccepted. To explore
     acceptance deliberately, select the pin and press a: review its exact IDs and
     reason, then Enter records the human decision; Esc cancels. Acceptance does
     not change the observed counts or make a violating result satisfy the pin.
  7. s or 4 opens Activity with full references; q quits (asks first during a run).
EOF
note "The explicit pair opens stored evidence without changing a saved review."
note "Use Activity references to reopen a new pin revision, not just the original comparison."
open_tui "$(short "$base")" "$(short "$candidate")" "$(short "$comparison")"
