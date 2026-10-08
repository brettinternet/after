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

# Narration goes to stderr, styled only on a terminal (NO_COLOR and TERM=dumb
# disable it, like AFTER itself). AFTER's own output keeps its terminal theme.
if [[ -t 2 && -z ${NO_COLOR:-} && ${TERM:-} != dumb ]]; then
	S_RESET=$'\e[0m' S_BOLD=$'\e[1m' S_DIM=$'\e[2m' S_BADGE=$'\e[1;7;35m'
	S_PROMPT=$'\e[1;32m' S_CMD=$'\e[1;36m' S_GOOD=$'\e[1;32m'
else
	S_RESET='' S_BOLD='' S_DIM='' S_BADGE='' S_PROMPT='' S_CMD='' S_GOOD=''
fi
# Pause before each step on an interactive terminal; AFTER_EXAMPLES_STEP=0 disables.
if [[ ${AFTER_EXAMPLES_STEP:-1} != 0 && -t 0 && -t 2 && -z ${CI:-} ]]; then STEP_PAUSE=1; else STEP_PAUSE=; fi
STEP=0

# intro TITLE PREMISE: name the example and the question it answers.
intro() { printf '\n%s%s%s\n%s\n' "$S_BOLD" "$1" "$S_RESET" "$2" >&2; }

# step TITLE: start a numbered step (pausing first on a terminal).
step() {
	if ((STEP > 0)) && [[ -n $STEP_PAUSE ]]; then
		printf '%s   Enter: next step%s' "$S_DIM" "$S_RESET" >&2
		read -rs </dev/tty
		printf '\r\e[2K' >&2
	fi
	STEP=$((STEP + 1))
	printf '\n%s %d %s %s%s%s\n' "$S_BADGE" "$STEP" "$S_RESET" "$S_BOLD" "$*" "$S_RESET" >&2
}

# note TEXT: explain what the next command shows.
note() { printf '   %s\n' "$*" >&2; }

# takeaway DIFF AFTER: contrast what a plain diff shows with what AFTER shows.
takeaway() {
	printf '\n%s   A diff shows  %s%s\n' "$S_DIM" "$1" "$S_RESET" >&2
	printf '%s   AFTER shows   %s%s%s\n' "$S_GOOD" "$S_RESET$S_BOLD" "$2" "$S_RESET" >&2
}

# shown ARGS...: print arguments as a copyable command line.
shown() {
	local arg out=
	for arg; do
		if [[ $arg =~ ^[][A-Za-z0-9_./:=@%+,~-]+$ ]]; then out+=" $arg"; else out+=" '${arg//\'/\'\\\'\'}'"; fi
	done
	printf '%s' "${out# }"
}

# prompt COMMAND: print a shell prompt line.
prompt() { printf '\n%s$%s %s%s%s\n' "$S_PROMPT" "$S_RESET" "$S_CMD" "$1" "$S_RESET" >&2; }

# check_exit WANT CODE: stop the example on an unexpected exit status.
check_exit() {
	[[ $2 == "$1" ]] || { echo "FAILED: expected exit $1, got $2" >&2; exit 1; }
}

# after WANT ARGS...: show and run an AFTER command; its readable output goes
# straight to the terminal. WANT is the required exit status.
after() {
	local want=$1 code=0
	shift
	prompt "after $(shown "$@")"
	"$AFTER_BIN" "$@" || code=$?
	check_exit "$want" "$code"
}

# after_json WANT ARGS...: run AFTER silently with --json and print the result
# for a script to parse. Only scripts need IDs; people use short ones.
after_json() {
	local want=$1 out code=0
	shift
	out=$("$AFTER_BIN" "$@" --json) || code=$?
	[[ $code == "$want" ]] || printf '%s\n' "$out" >&2
	check_exit "$want" "$code"
	printf '%s\n' "$out"
}

# shell WANT COMMAND: show and run a shell pipeline, such as the tests you run
# yourself. COMMAND is a fixed string from the example script; inside it,
# `after` is the AFTER binary itself.
shell() {
	local code=0
	prompt "$2"
	(unset -f after; PATH="${AFTER_BIN%/*}:$PATH"; eval "$2") || code=$?
	check_exit "$1" "$code"
}

# short ID: the 8-hex prefix AFTER prints and accepts for a stored ID.
short() { local id=${1#sha256:}; printf '%s' "${id:0:8}"; }

# newest KIND: short ID of the newest stored capture, run, report or pin event.
newest() { short "$(after_json 0 log | jq -r --arg kind "$1" '[.data.rows[] | select(.kind == $kind)][0].id')"; }

# tui_keys: print the key guide read from stdin under a heading.
tui_keys() { printf '
%sTry in the TUI%s
' "$S_BOLD" "$S_RESET" >&2; cat >&2; }

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
	printf '%s   workspace %s%s\n' "$S_DIM" "$WORK" "$S_RESET" >&2
}

# commit MESSAGE: commit every tracked and new file.
commit() { git add -A && git commit -qm "$1"; }

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

# Fixtures shared by the examples and the docs/demos VHS tapes. Each creates a
# workspace with a base commit and leaves the candidate change uncommitted.

# rate_limit_project: raise a rate limit, forget the docs, add a scratch file.
rate_limit_project() {
	workspace "${1:-rate-limit}"
	mkdir -p limits
	printf 'package limits\n\n// RequestsPerMinute caps each API key.\nconst RequestsPerMinute = 600\n' >limits/limits.go
	printf '# Rate limits\n\nEach API key may send 600 requests per minute.\n' >README.md
	commit "base"
	perl -pi -e 's/600/1200/' limits/limits.go
	echo "load test notes" >notes.txt
}

# discount_project: the member discount moves 10% -> 15%, and its test is
# edited to agree. Requires Go to run the tests.
discount_project() {
	command -v go >/dev/null || { echo "Go is required; run via mise exec --" >&2; exit 1; }
	export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off
	workspace "${1:-discount}"
	printf 'module example.com/shop\n\ngo 1.24\n' >go.mod
	cat >price.go <<'EOF'
package shop

// Total applies the member discount to a cart subtotal in cents.
func Total(cents int, member bool) int {
	if member {
		return cents * 90 / 100
	}
	return cents
}
EOF
	cat >price_test.go <<'EOF'
package shop

import "testing"

func TestMemberDiscount(t *testing.T) {
	if got := Total(1000, true); got != 900 {
		t.Fatalf("Total = %d, want 900", got)
	}
}
EOF
	commit "member discount"
	perl -pi -e 's/90 \/ 100/85 \/ 100/' price.go
	perl -pi -e 's/900/850/g' price_test.go
}

# frozen_tests OUT: run the BASE tests against the candidate code, writing go
# test -json to OUT, and restore the candidate's tests. The failure is expected.
frozen_tests() {
	local test_file
	test_file=$(git diff --name-only -- '*_test.go')
	cp "$test_file" "$WORK/candidate_test.go.txt"
	git show "HEAD:$test_file" >"$test_file"
	if go test -json ./... >"$1"; then
		echo "Expected the frozen oracle to fail" >&2; exit 1
	fi
	cp "$WORK/candidate_test.go.txt" "$test_file"
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

# run_note: explain AFTER's terminal consent before a run.
run_note() {
	note "AFTER prepares an exact private plan and shows its consent summary;"
	note "type yes to run exactly those plan bytes (anything else runs nothing)."
}
