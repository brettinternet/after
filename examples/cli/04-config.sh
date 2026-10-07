#!/usr/bin/env bash
# Configuration layers: flag > AFTER_* environment > YAML > default. `after
# config` shows each effective value and which layer won, without running anything.
source "$(dirname "$0")/../lib.sh"

workspace config
settings() {
	jq -r '.data.settings[] | select(.name == ("repetitions", "run_seconds", "diff_bytes", "interactive")) |
		"   \(.name | . + " " * (12 - length)) \(.value | tostring | . + " " * (6 - length)) from \(.source)"' >&2
}

section "Team defaults in YAML: three repetitions to catch flaky behavior"
mkdir -p "$XDG_CONFIG_HOME/after"
cat >"$XDG_CONFIG_HOME/after/config.yaml" <<'EOF'
repetitions: 3
run_seconds: 120
diff_bytes: 0 # inventory only, no patch bytes (zero is a value, not "unset")
EOF
after 0 config | settings

section "Environment overrides YAML; run options stay on the run command"
note "AFTER_REPETITIONS=2 AFTER_INTERACTIVE=false"
AFTER_REPETITIONS=2 AFTER_INTERACTIVE=false after 0 config | settings
note "after run --help shows its command-scoped run options and configuration defaults"
after 0 run --help | sed -n '/--repetitions/ p; /--run-seconds/ p; /--output-bytes/ p' >&2

section "Invalid values fail fast with the setting and its source (exit 2)"
note "AFTER_REPETITIONS=9"
AFTER_REPETITIONS=9 "$AFTER_BIN" config 2>&1 >/dev/null | sed 's/^/   /' >&2 || true

note "There is deliberately no setting that grants execution consent."
