#!/usr/bin/env bash
# Browse a messy real-world change with no evidence at all: renames, deletions,
# a binary asset, an edited golden file, and secrets that must stay uncaptured.
source "$(dirname "$0")/../lib.sh"

intro "Review a messy change with no evidence at all" \
	"Renames, a deletion, a binary asset, a mode change, an edited golden file and an uncaptured secret."
workspace tui-raw
mkdir -p handlers legacy testdata assets
printf 'package handlers\n\nfunc Health() string { return "ok" }\n' >handlers/health.go
printf 'package legacy\n\n// Deprecated: v1 XML API.\nfunc XML() {}\n' >legacy/xml.go
printf '{"status":"ok"}\n' >testdata/health.golden.json
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR' >assets/logo.png
printf '#!/bin/sh\ngo build ./...\n' >build.sh
commit "base"

git mv handlers/health.go handlers/status.go
printf 'package handlers\n\nfunc Status() string { return "healthy" }\n' >handlers/status.go
git rm -q legacy/xml.go
printf '{"status":"healthy"}\n' >testdata/health.golden.json # the oracle moved too
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x01' >assets/logo.png
chmod +x build.sh
echo "STRIPE_KEY=sk_test_example" >.env.local # untracked: never captured

step "The kind of change an agent might hand you"
after 0 diff --stat
note "The untracked .env.local is listed as excluded, never read into the store."

tui_keys <<'EOF'
  1/2/3/4    Overview, Changes inventory, Diff, and Activity; active tab is highlighted
  Tab        next view; Shift+Tab previous view (inside a detail, move between sections)
  j/k Enter  inspect one entry; Changes includes deleted, added, binary, mode, and excluded paths
  /, n/N     search the current list or document; next/previous match
  ], [       next/previous file in Diff; }, { next/previous hunk
  b          toggle exact hex bytes; h/l pan long lines
  Esc        back to the previous view; no evidence was supplied
  ?          grouped contextual keys and unavailable reasons; q quits and restores the terminal
EOF
note "Plain after review starts/saves this review; run it again to resume."
note "after review --new starts fresh without deleting old evidence or pins."
open_tui
