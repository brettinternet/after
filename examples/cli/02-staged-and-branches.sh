#!/usr/bin/env bash
# Choose what to review: the index you are about to commit, or a feature
# branch against the point it forked from main.
source "$(dirname "$0")/../lib.sh"

section "Base project"
workspace staged-and-branches
cat >retry.go <<'EOF'
package client

const MaxAttempts = 3
EOF
cat >timeout.go <<'EOF'
package client

const TimeoutSeconds = 10
EOF
commit "base"

section "Pre-commit: stage the real fix, leave a debug edit unstaged"
perl -pi -e 's/= 3/= 5/' retry.go
git add retry.go
perl -pi -e 's/= 10/= 999 \/\/ DEBUG: do not commit/' timeout.go

note "--staged reviews HEAD vs the index: only what the commit will contain."
capture_ids "$(after 0 capture --json --staged)"
inventory "$BASE" "$CANDIDATE"

note "The default reviews HEAD vs the working tree: the debug edit appears."
capture_ids "$(after 0 capture --json)"
inventory "$BASE" "$CANDIDATE"

git commit -qm "retry five times"
git checkout -q timeout.go

section "Branch review: feature work while main moves on"
git switch -qc feature/jitter
cat >jitter.go <<'EOF'
package client

const JitterMillis = 250
EOF
commit "add retry jitter"
git switch -q main
perl -pi -e 's/= 10/= 30/' timeout.go
commit "unrelated: longer timeout on main"

section "A clean working tree is not the branch diff"
git switch -q feature/jitter
after 0 capture
note "The empty working-tree capture suggests reviewing committed branch changes."

note "--base/--target capture merge-base(main, feature) -> feature, like a PR diff."
note "main's later timeout change is not attributed to the branch."
capture_ids "$(after 0 capture --json --base main --target feature/jitter)"
inventory "$BASE" "$CANDIDATE"
patch "$BASE" "$CANDIDATE"
