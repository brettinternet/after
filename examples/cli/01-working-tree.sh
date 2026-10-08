#!/usr/bin/env bash
# Review uncommitted work: see the live change, capture it as immutable
# snapshots, keep every path visible (including ones the patch leaves out), and
# notice when the checkout moves past the capture.
source "$(dirname "$0")/../lib.sh"

intro "Review uncommitted work" \
	"A rate limit doubles, the README still says 600, and a scratch file sits untracked."
rate_limit_project working-tree

step "Read the current change, like git diff HEAD (nothing is stored)"
after 0 diff
note "Stderr names what was compared and every path the patch does not cover."

step "Capture it: base and candidate snapshots, read from Git only"
after 0 capture
note "Nothing was built or run. Runs, reports and pins bind to these exact snapshots."

step "Inspect every path, not just the ones with a patch"
after 0 inspect

step "Keep editing: status notices the stored capture is out of date"
perl -pi -e 's/1200/1500/' limits/limits.go
after 0 status
note "Status compares file contents, not timestamps, and stores nothing."
note "after diff now shows 600 → 1500; after diff --stored still shows the captured 1200."

step "Opt in to the scratch file by exact path"
after 0 capture --include-untracked notes.txt
after 0 diff --stored --stat

takeaway "the lines that changed in tracked files" \
	"a stored, reopenable pair covering every path, including what it left out"
note "The README still says 600: capture shows what changed, not what should have."
