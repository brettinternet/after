#!/usr/bin/env bash
# Select a non-payment HTTP definition explicitly; never install or pull images.
source "$(dirname "$0")/../lib.sh"
require_docker
# The existing Python proof pins an arm64 manifest, not a multi-platform index.
case $(uname -m) in
arm64 | aarch64) ;;
*) echo "This example requires the pinned linux/arm64 Python service image and a matching host; see docs/SANDBOX.md." >&2; exit 1 ;;
esac

intro "Same HTTP response, changed upstream request" \
	"An echo proxy starts uppercasing request bodies; a lowercase case changes, an uppercase case stays equal."
workspace http-service
cp "$REPO/internal/runner/testdata/python-service/app.py" app.py
commit "Python stdlib echo proxy"
perl -pi -e 's/body = self.rfile.read\(length\)/body = self.rfile.read(length).upper()/' app.py

# Keep the operator-selected definition outside the captured project. Reuse the
# separately provisioned pinned Python image, fd3 ABI and independent observer.
jq '.name = "echo-normalization" | .repetitions = 2 | .cases = [
  {id: "lowercase", title: "Lowercase payload changes upstream", requests: [
    {after_seconds: 0, method: "POST", path: "/echo", headers: {"Content-Type": "text/plain"}, body: "hello"}
  ]},
  {id: "uppercase", title: "Uppercase payload stays equal", requests: [
    {after_seconds: 0, method: "POST", path: "/echo", headers: {"Content-Type": "text/plain"}, body: "HELLO"}
  ]}
]' "$REPO/internal/runner/testdata/python-service/http-service.json" >"$WORK/echo-cases.json"

step "Inspect the source change without executing it"
after 0 diff
after 0 capture
pair=$(after_json 0 status | jq -r '.data.capture | "\(.base_snapshot) \(.candidate_snapshot)"')
read -r base candidate <<<"$pair"

step "Select the frozen HTTP cases, image and observer channels"
shell 0 "jq '{kind, image, platform, cases, channels, repetitions}' ../echo-cases.json"
note "Provision the Python service AND trusted Go observer images separately (docs/SANDBOX.md)."
note "No service dependencies, image pulls or host execution are hidden in this run."

step "Approve the exact plan for both captured versions"
run_note
after 4 run "$(short "$base")" "$(short "$candidate")" --definition ../echo-cases.json

step "Read the independent observer's stored witnesses without rerunning"
after 4 compare
shell 4 "after compare --json | jq -c '.data.details.witnesses[] | select(.relation == \"paired\" and .before.repetition == 0) | {case: .before.case_id, channel, outcome, changes}'"

takeaway "a one-line normalization" \
	"equal HTTP responses in both cases; changed provider body for lowercase, equal for uppercase"
note "Exit 4 means a difference, not incomplete execution. This is a finite synthetic observation."
