#!/usr/bin/env bash
# Choose what to review: the index you are about to commit, or a feature
# branch against the point it forked from main.
source "$(dirname "$0")/../lib.sh"

intro "Choose what to review" \
	"Stage a fix but not a debug edit; then review a feature branch after main moved on."
workspace staged-and-branches
printf 'package client\n\nconst MaxAttempts = 3\n' >retry.go
printf 'package client\n\nconst TimeoutSeconds = 10\n' >timeout.go
commit "base"
perl -pi -e 's/= 3/= 5/' retry.go
git add retry.go
perl -pi -e 's/= 10/= 999 \/\/ DEBUG: do not commit/' timeout.go

step "Before committing: --staged reviews exactly what the commit will contain"
after 0 diff --staged --stat
note "The default (HEAD versus the working tree) also shows the unstaged DEBUG edit:"
after 0 diff --stat
after 0 capture --staged

git commit -qm "retry five times"
git checkout -q timeout.go
git switch -qc feature/jitter
printf 'package client\n\nconst JitterMillis = 250\n' >jitter.go
commit "add retry jitter"
git switch -q main
perl -pi -e 's/= 10/= 30/' timeout.go
commit "unrelated: longer timeout on main"
git switch -q feature/jitter

step "On a committed feature branch, the clean working tree is empty"
after 0 capture
note "The empty capture points you to the branch's committed changes."

step "Review the branch like a pull request"
note "--base/--target capture merge-base(main, feature) → feature;"
note "main's later timeout change is not attributed to the branch."
after 0 capture --base main --target feature/jitter
after 0 diff --stored
