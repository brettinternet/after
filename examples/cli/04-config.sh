#!/usr/bin/env bash
# Configuration layers: flag > AFTER_* environment > YAML > default. `after
# config` shows each effective value and which layer won, without running anything.
source "$(dirname "$0")/../lib.sh"

intro "Configuration you can explain" \
	"Every effective setting names the layer it came from: flag, environment, YAML or default."
workspace config

step "Team defaults in YAML: three repetitions to catch flaky behavior"
mkdir -p "$XDG_CONFIG_HOME/after"
cat >"$XDG_CONFIG_HOME/after/config.yaml" <<'EOF'
repetitions: 3
run_seconds: 120
diff_bytes: 0 # inventory only, no patch bytes (zero is a value, not "unset")
EOF
after 0 config

step "The environment overrides YAML"
shell 0 "AFTER_REPETITIONS=2 AFTER_INTERACTIVE=false after config | grep -E 'repetitions|run_seconds|interactive'"
note "Run limits are flags on after run; its help lists their defaults and ranges:"
shell 0 "after run --help | grep -E -- '--(repetitions|run-seconds)'"

step "Invalid values fail fast with the setting and its source (exit 2)"
shell 2 "AFTER_REPETITIONS=9 after config"

note "There is deliberately no setting that grants execution consent."
