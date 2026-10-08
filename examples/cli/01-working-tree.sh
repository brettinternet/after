#!/usr/bin/env bash
# Review uncommitted work: capture HEAD versus the working tree, list every
# changed path (including ones AFTER cannot diff), read the current patch, and
# see status notice when the checkout moves past the stored capture.
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

section "The current change, like git diff HEAD (read live; nothing stored)"
after 0 diff --stat
after 0 diff
note "Use after diff --raw > exact.patch for exact bytes; --raw refuses terminal output."

section "Keep editing: status says the stored capture is out of date"
perl -pi -e 's/1200/1500/' limits/limits.go
after 0 status
note "Status compares file contents, not timestamps, and stores nothing."

section "The stored capture's patch still says 1200"
after 0 diff --stored
note "Bare after diff now shows 600 -> 1500; after review captures the current change."

section "Opt in to an untracked file by exact path"
capture=$(after 0 capture --json --include-untracked notes.txt)
capture_ids "$capture"
inventory "$BASE" "$CANDIDATE"

note "Note the README still says 600: the inventory shows what changed, not what should have."
