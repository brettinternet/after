#!/usr/bin/env bash
# Browse a messy real-world change with no evidence at all: renames, deletions,
# a binary asset, an edited golden file, and secrets that must stay uncaptured.
source "$(dirname "$0")/../lib.sh"

intro "Review a messy change with no evidence at all" \
	"Renames, a deletion, a binary asset, a mode change, an edited golden file and an uncaptured secret."
messy_project tui-raw

step "The kind of change an agent might hand you"
after 0 diff --stat
note "The untracked .env.local is listed as excluded, never read into the store."

tui_keys <<'EOF'
  1/2/3/4    Overview, Changes inventory, Diff, and Activity; active tab is highlighted
  Tab        next view; Shift+Tab previous view (inside a detail, move between sections)
  j/k Enter  inspect one entry (l/h also open/back in lists; Ctrl-N/P, Ctrl-D/U work too);
             Changes includes deleted, added, binary, mode, and excluded paths
  /, n/N     search the current list or document; next/previous match
  ], [       next/previous file in Diff; }, { next/previous hunk; changed words are emphasized
  b          toggle exact hex bytes; h/l pan long lines
  Esc        back to the previous view; no evidence was supplied
  ?          grouped contextual keys and unavailable reasons; q quits and restores the terminal
EOF
note "Plain after review starts/saves this review; run it again to resume."
note "after review --new starts fresh without deleting old evidence or pins."
open_tui
