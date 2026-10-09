#!/usr/bin/env bash
# Generate JUnit XML live with Bun's built-in test runner; no packages to install.
source "$(dirname "$0")/../lib.sh"
command -v bun >/dev/null || { echo "Bun is required; run via mise exec --" >&2; exit 1; }

intro "A non-Go test report is still only a report" \
	"A shipping threshold changes; the unchanged tests report one failure and one pass."
workspace junit
cat >shipping.js <<'EOF'
export function shipping(cents) {
  return cents >= 5000 ? 0 : 500;
}
EOF
cat >shipping.test.js <<'EOF'
import { test, expect } from "bun:test";
import { shipping } from "./shipping.js";

test("free shipping at fifty dollars", () => expect(shipping(5000)).toBe(0));
test("small carts still pay shipping", () => expect(shipping(1000)).toBe(500));
EOF
commit "shipping threshold"
perl -pi -e 's/5000/7500/' shipping.js

step "The diff shows a changed threshold, not a test result"
after 0 diff

step "Explicitly run Bun to produce a fresh JUnit report"
note "This script runs the synthetic tests on the host; AFTER import does not run them."
note "Bun is provisioned through mise; its built-in runner needs no downloaded packages."
shell 1 "bun test --reporter=junit --reporter-outfile=../report.xml"
producer="Bun $(bun --version) test (JUnit)"
after 0 import ../report.xml --format junit --producer "$producer"

step "Read the reported cards alongside the captured change"
after 0 inspect "$(newest report)"
after 0 diff
note "Bun 1.4.2 emits a suite-level file attribute outside this importer's dialect."
note "The report stays incomplete (unsupported_attribute); both closed test cards survive."
note "The example imports the original XML unchanged, rather than hiding that limitation."
note "Import exit 0 means stored, not tests passed. Snapshot binding is not proof of execution."

takeaway "a threshold edit" \
	"one reported failure and one reported pass, with inputs and effects unavailable"
note "Use after review to browse the same cards. These are reported, never observed evidence."
