#!/usr/bin/env bash
# Review uncommitted work: capture HEAD versus the working tree, list every
# changed path (including ones AFTER cannot diff) and read the raw patch.
source "$(dirname "$0")/../lib.sh"

section "A small service with a rate limit"
workspace working-tree
mkdir -p limits
cat >limits/limits.go <<'EOF'
package limits

// RequestsPerMinute caps each API key.
const RequestsPerMinute = 600
EOF
printf '# Rate limits\n\nEach API key may send 600 requests per minute.\n' >README.md
commit "base"

section "Edit: raise the limit, forget the docs, add a scratch file"
perl -pi -e 's/600/1200/' limits/limits.go
echo "load test notes" >notes.txt

section "Capture (reads Git only; never builds or runs the project)"
capture=$(after 0 capture --json)
capture_ids "$capture"
note "base:      $BASE"
note "candidate: $CANDIDATE"

section "Readable defaults resolve the newest capture (no IDs or jq needed)"
after 0 status
after 0 inspect
after 0 log -n 3

section "Inventory: every path stays visible, even untracked ones that were excluded"
inventory "$BASE" "$CANDIDATE"

section "Stream the captured patch (not the live checkout)"
after 0 diff
note "Use after diff --raw > exact.patch for exact bytes; --raw refuses terminal output."

section "Opt in to an untracked file by exact path"
capture=$(after 0 capture --json --include-untracked notes.txt)
capture_ids "$capture"
inventory "$BASE" "$CANDIDATE"

note "Note the README still says 600: the inventory shows what changed, not what should have."
