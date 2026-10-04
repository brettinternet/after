#!/usr/bin/env bash
# Browse a messy real-world change with no evidence at all: renames, deletions,
# a binary asset, an edited golden file, and secrets that must stay uncaptured.
source "$(dirname "$0")/../lib.sh"

section "Base: a small HTTP service"
workspace tui-raw
mkdir -p handlers legacy testdata assets
printf 'package handlers\n\nfunc Health() string { return "ok" }\n' >handlers/health.go
printf 'package legacy\n\n// Deprecated: v1 XML API.\nfunc XML() {}\n' >legacy/xml.go
printf '{"status":"ok"}\n' >testdata/health.golden.json
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR' >assets/logo.png
printf '#!/bin/sh\ngo build ./...\n' >build.sh
commit "base"

section "Candidate: the kind of change an agent might hand you"
git mv handlers/health.go handlers/status.go
printf 'package handlers\n\nfunc Status() string { return "healthy" }\n' >handlers/status.go
git rm -q legacy/xml.go
printf '{"status":"healthy"}\n' >testdata/health.golden.json # the oracle moved too
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x01' >assets/logo.png
chmod +x build.sh
echo "STRIPE_KEY=sk_test_example" >.env.local # untracked: never captured
capture_ids "$(after 0 capture)"
inventory "$BASE" "$CANDIDATE"

cat >&2 <<'EOF'

Try in the TUI:
  d          complete inventory: deleted, added (a rename is both), binary, mode change, excluded .env.local
  j/k Enter  inspect one entry; Tab/Shift+Tab move between sections
  Tab        (from inventory) the captured raw patch; [ and ] page through bytes
  Esc        back to the list; the STATE row says "not checked" - no evidence was supplied
  ?          every key;  q quits and restores the terminal
EOF
open_tui "$CANDIDATE" --base "$BASE"
