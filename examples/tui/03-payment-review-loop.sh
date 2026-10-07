#!/usr/bin/env bash
# Guard known-good behavior: observe the payment service, pin "a 12h retry
# makes one provider charge", then let a later edit reopen the pin and rerun it
# from the TUI to catch the regression. Requires the offline Docker sandbox.
source "$(dirname "$0")/../lib.sh"
require_docker

section "Observe the current service (base and candidate are both 24h retention)"
payment_project tui-payment
capture_ids "$(after 0 capture)"
result=$(run_pair "$BASE" "$CANDIDATE" 0)
provider_counts "$result"

# A later "small" change, applied from another terminal while the TUI is open.
regress=$WORK/apply-regression.sh
cat >"$regress" <<EOF
#!/bin/sh
# Simulates an agent "tidying" retention to 5 minutes.
cd $(printf %q "$PROJECT") && perl -pi -e 's/24 \\* 60 \\* 60/5 * 60/' app/config.go && git diff --stat
echo "Edited app/config.go; press c in the TUI."
EOF
chmod +x "$regress"

cat >&2 <<EOF

Try in the TUI:
  1. Enter on a case row: exact responses and provider calls. Esc.
  2. On the 43200s (12h) row, p: review the exact receipt/pair, then Enter pins its
     measured count (1). Ctrl-U clears the prefilled reason; Esc cancels. Press p again
     to see the duplicate-basis refusal and existing pin ID.
  3. In another terminal:  $regress
  4. c captures the edit; u confirms the new candidate ID, changed-path count and
     original base. Esc cancels; Enter selects it. End selects the pin: reopened | stale
     | missing current evidence. Enter shows why.
  5. r previews the exact offline rerun plan; n denies (nothing runs), y approves once.
     Navigation stays live during the run; x cancels it.
  6. New rows appear: 43200s base=[1] candidate=[2] breaks the pinned expectation;
     the 30s control is unchanged. Press a, review the pin/current-receipt IDs and
     reason, then Enter records acceptance; Esc cancels. The row shows [ACCEPTED].
  7. s shows snapshot/pin/result IDs for restarting later; q quits.
EOF
open_tui "$BASE" "$CANDIDATE" "$(jq -r .data.comparison.id <<<"$result")"
