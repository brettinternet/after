#!/usr/bin/env bash
# Observe a CLI at its process boundary: run base and candidate builds on frozen
# stdin cases and compare exit status, stdout (as JSON) and stderr.
#
# The candidate "tidies" a quote CLI: a struct replaces a map (output key order
# changes) and a one-line decode replaces the strict decoder. Normal quotes stay
# equal as JSON; a misspelled field now exits 0 with a $0 quote (exit 4).
source "$(dirname "$0")/../lib.sh"
require_docker
case $(uname -m) in
arm64 | aarch64) platform=linux/arm64 ;;
x86_64 | amd64) platform=linux/amd64 ;;
*) echo "unsupported platform: $(uname -m)" >&2; exit 1 ;;
esac
# The already provisioned sandbox Go image (docs/SANDBOX.md); runs never pull.
image=$(sed -n 's/^const Image = "\(.*\)"$/\1/p' "$REPO/internal/sandbox/plan.go")

intro "Same quotes, different failures" \
	"quote reads an order on stdin and prints a JSON quote; a bad order must exit 2, never quote \$0."
workspace command
cat >main.go <<'EOF'
// quote prints a JSON price quote for an order read from stdin.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type line struct {
	SKU   string `json:"sku"`
	Qty   int    `json:"qty"`
	Cents int    `json:"cents"`
}

func main() {
	var order struct {
		Lines []line `json:"lines"`
	}
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&order); err != nil {
		fail(err)
	}
	total := 0
	for _, l := range order.Lines {
		total += l.Qty * l.Cents
	}
	json.NewEncoder(os.Stdout).Encode(map[string]int{"lines": len(order.Lines), "total_cents": total})
}

func fail(err error) {
	json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
	fmt.Fprintln(os.Stderr, "quote:", err)
	os.Exit(2)
}
EOF
commit "quote CLI"

# The candidate: a struct for the output and a shorter decode.
perl -0pi -e 's/\tdecoder := json.NewDecoder\(os.Stdin\)\n\tdecoder.DisallowUnknownFields\(\)\n\tif err := decoder.Decode/\tif err := json.NewDecoder(os.Stdin).Decode/' main.go
perl -0pi -e 's/\tjson.NewEncoder\(os.Stdout\).Encode\(map\[string\]int\{"lines": len\(order.Lines\), "total_cents": total\}\)/\tjson.NewEncoder(os.Stdout).Encode(quote{TotalCents: total, Lines: len(order.Lines)})/' main.go
cat >>main.go <<'EOF'

type quote struct {
	TotalCents int `json:"total_cents"`
	Lines      int `json:"lines"`
}
EOF

# The operator-selected definition lives outside the project: repository content
# never selects or authorizes what runs. Stdin is frozen as base64 bytes.
jq -n --arg platform "$platform" --arg image "$image" \
	--arg order '{"lines":[{"sku":"tea","qty":2,"cents":450},{"sku":"mug","qty":1,"cents":1200}]}' \
	--arg typo '{"lines":[{"sku":"tea","quantity":2,"cents":450}]}' '{
	version: 1, kind: "command", name: "quote-cli", platform: $platform, image: $image,
	build_argv: ["/usr/local/go/bin/go", "build", "-o", "/work/quote", "/input/main.go"],
	cases: [
		{id: "order", title: "A valid two-line order", argv: ["/work/quote"], stdin_base64: ($order | @base64), environment: []},
		{id: "typo", title: "A misspelled quantity field", argv: ["/work/quote"], stdin_base64: ($typo | @base64), environment: []}
	],
	repetitions: 2,
	limits: {seconds: 60, output_bytes: 65536, preparation_seconds: 90},
	comparison: {stdout: "json", stderr: "text"}
}' >"$WORK/quote-cases.json"

step "The change looks like a tidy refactor"
after 0 diff
after 0 capture
pair=$(after_json 0 status | jq -r '.data.capture | "\(.base_snapshot) \(.candidate_snapshot)"')
read -r base candidate <<<"$pair"

step "Freeze the cases you want observed"
note "Two stdin cases, each run twice per side. stdout is declared JSON, so key order"
note "is ignored; stderr and exit status compare exactly."
shell 0 "jq '.cases[] | {id, title, stdin: (.stdin_base64 | @base64d)}' ../quote-cases.json"

step "Run both builds in fresh offline containers"
note "Each case, side and repetition gets its own container; AFTER records exit status"
note "and streams from the Docker attachment, not from anything the program reports."
run_note
after 4 run "$(short "$base")" "$(short "$candidate")" --definition ../quote-cases.json

step "See exactly what differed"
note "after compare re-derives the result from stored streams without running anything."
note "The order case's stdout bytes changed (key order) but compare equal as JSON."
note "Text witnesses are base64 bytes, so program output never reaches your terminal raw."
shell 4 "after compare --json | jq -c '.data.details.witnesses[] | select(.relation == \"paired\" and .outcome == \"different\" and .before.repetition == 0) | {case: .before.case_id, channel, changes}'"

takeaway "a struct instead of a map and a shorter decode" \
	"the valid quote is equal as JSON, but a misspelled field now exits 0 with a \$0 quote"
note "Exit 4. Pin it like any run (examples/cli/06-pin-reopen-rerun.sh); editing the code"
note "or the definition reopens the pin."
