# shellcheck shell=bash
# Shared helpers for AFTER examples. Source from an example script.
set -euo pipefail

EXAMPLES=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO=$(dirname "$EXAMPLES")
AFTER_BIN=${AFTER_BIN:-$REPO/bin/after}
PAYMENT_FIXTURE=$REPO/internal/paymentfixture/testdata/payment

command -v jq >/dev/null || { echo "jq is required (mise install)" >&2; exit 1; }
[[ -x $AFTER_BIN ]] || { echo "missing $AFTER_BIN; run: mise exec -- task build" >&2; exit 1; }

# Reproducible Git: no user config, hooks, signing or global identity.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME="AFTER example" GIT_AUTHOR_EMAIL=example@example.invalid
export GIT_COMMITTER_NAME="AFTER example" GIT_COMMITTER_EMAIL=example@example.invalid

# section TITLE: print a heading.
section() { printf '\n== %s\n' "$*" >&2; }
# note TEXT: print an explanation.
note() { printf '   %s\n' "$*" >&2; }

# workspace NAME: create a private Git project under the OS temp directory and
# cd into it. Workspaces are retained for exploration; move them to Trash when done.
workspace() {
	local tmp=${TMPDIR:-/tmp}
	WORK=$(mktemp -d "${tmp%/}/after-example-$1-XXXXXX")
	PROJECT=$WORK/project
	export XDG_CONFIG_HOME=$WORK/config # ignore any personal AFTER config
	mkdir -p "$PROJECT"
	cd "$PROJECT"
	git init -q --template= -b main
	echo .after/ >.gitignore # private evidence store; never commit it
	note "workspace: $WORK"
}

# commit MESSAGE: commit every tracked and new file.
commit() { git add -A && git commit -qm "$1"; }

# after WANT ARGS...: run AFTER, require exit status WANT, print stdout.
# Pass --json explicitly when consuming the result in a script.
after() {
	local want=$1 out code=0
	shift
	printf '$ after %s\n' "$*" | sed -E 's/(sha256:[0-9a-f]{8})[0-9a-f]{56}/\1…/g' >&2
	out=$("$AFTER_BIN" "$@") || code=$?
	if [[ $code != "$want" ]]; then
		printf '%s\n' "$out" >&2
		echo "FAILED: expected exit $want, got $code" >&2
		exit 1
	fi
	printf '%s\n' "$out"
}

# capture_ids JSON: set BASE and CANDIDATE from a capture result.
# shellcheck disable=SC2034 # set for callers
capture_ids() {
	BASE=$(jq -r .data.base_snapshot.id <<<"$1")
	CANDIDATE=$(jq -r .data.candidate_snapshot.id <<<"$1")
}

# inventory BASE CANDIDATE: print one line per changed path.
inventory() {
	after 0 inspect --json "$1" "$2" | jq -r '.data.inventory[] |
		"   \(.change | .[0:9] | . + " " * (9 - length)) \(.path)" +
		(if .potential_oracle then "  [potential oracle]" else "" end) +
		(if .binary then "  [binary]" else "" end) +
		(if .base.mode and .candidate.mode and .base.mode != .candidate.mode then "  [mode \(.base.mode) -> \(.candidate.mode)]" else "" end) +
		(if .limits then "  (" + (.limits | join("; ")) + ")" else "" end)' >&2
}

# patch BASE CANDIDATE: stream the complete terminal-safe captured diff.
patch() { after 0 diff "$1" "$2" >&2; }

# cards IMPORT_JSON: print imported report cards with their evidence state.
cards() {
	jq -r '.data.cards[] | "   \(.test // .package): \(.state.report)  [\(.state.kind) / applicability \(.state.applicability) / \(.state.execution); inputs \(.inputs)]" +
		(if .state.report == "fail" and .test then "\n" + (.output | split("\n") | map(select(test("want"))) | map("      " + .) | join("\n")) else "" end)' <<<"$1" >&2
}

# open_tui ARGS...: print the TUI command, then open it after Enter. Set
# AFTER_EXAMPLES_PREPARE_ONLY=1 to stop after preparing state (smoke tests).
open_tui() {
	printf '\nReopen later with:\n ' >&2
	printf ' %q' "$AFTER_BIN" review "$@" --project "$PROJECT" >&2
	echo >&2
	[[ -z ${AFTER_EXAMPLES_PREPARE_ONLY:-} ]] || return 0
	read -r -p "   Press Enter to open the TUI (q quits, ? shows keys)..." </dev/tty
	exec "$AFTER_BIN" review "$@"
}

# payment_project: seed the synthetic payment app (24h idempotency retention).
payment_project() {
	workspace "${1:-payment}"
	mkdir -p app driver
	cp "$PAYMENT_FIXTURE/go.mod" .
	cp "$PAYMENT_FIXTURE/app/main.go" "$PAYMENT_FIXTURE/app/config.go" app/
	cp "$PAYMENT_FIXTURE/driver/main.go" driver/
	commit "payment service: 24h idempotency retention"
}

# set_config RETENTION DEDUPLICATE PRINTED: rewrite app/config.go.
set_config() {
	printf 'package main\n\nconst retentionSeconds int64 = %s\nconst deduplicate = %s\nconst printedCount = "%s"\n' "$@" >app/config.go
}

require_docker() {
	if [[ ${AFTER_DOCKER_BINARY:-} != /* || ${AFTER_DOCKER_HOST:-} != unix:///* ]]; then
		cat >&2 <<-'EOF'
			This example executes code in AFTER's offline Docker sandbox. Set both
			(after config prints an export line for a detected Docker CLI and socket):
			  export AFTER_DOCKER_BINARY="$(command -v docker)"
			  export AFTER_DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
			and provision the pinned image once (see docs/SANDBOX.md).
		EOF
		exit 1
	fi
}

# run_pair BASE CANDIDATE WANT: preview the frozen experiment, ask for consent on
# the terminal, then run exactly that plan. Prints the run JSON. WANT is 0 for
# equal results or 4 for a finding.
run_pair() {
	local preview digest answer
	preview=$(after 3 run --json "$1" "$2" --interactive=false)
	digest=$(jq -r .data.authorization_digest <<<"$preview")
	note "Preview only; nothing executed. Exact plan stored privately in .after/."
	after 0 inspect "$digest" >&2
	read -r -p "   Type yes to run exactly $digest: " answer </dev/tty
	[[ $answer == yes ]] || { note "Declined; nothing executed."; exit 3; }
	note "Running 2 versions x 2 cases in the offline sandbox (1-2 minutes)..."
	after "$3" run --json --approve "$digest"
}

# witnesses COMPARISON_JSON: print paired witnesses and their exact changes.
witnesses() {
	jq -r '.data.details.witnesses[] | select(.relation == "paired") |
		"   \(.before.seconds)s \(.channel): \(.outcome)" +
		([.changes[] | "\n      \(.kind) \(.path): " + (if has("before") then (.before | tojson) + " -> " else "" end) + (.after | tojson)] | join(""))' <<<"$1" >&2
}

# provider_counts RUN_JSON: print provider requests per case from observed artifacts.
provider_counts() {
	local id side seconds count
	while read -r id side seconds; do
		count=$("$AFTER_BIN" inspect "$id" --json | jq '.data.document.provider_calls | length')
		printf '   %-9s %5ss retry: %s provider request(s)\n' "$side" "$seconds" "$count" >&2
	done < <(jq -r '.data.samples[] | . as $s | .artifacts[] | select(.channel | endswith("/observation")) | "\(.content) \($s.side) \($s.case_seconds)"' <<<"$1")
}
