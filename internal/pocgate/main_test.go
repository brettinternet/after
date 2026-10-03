package main

import "testing"

func TestGateRequiresExecutedProofs(t *testing.T) {
	for _, kind := range []string{"missing", "skip", "fail", "pass", "package-only"} {
		t.Run(kind, func(t *testing.T) {
			r := results{passed: map[string]bool{}, failed: map[string]bool{}}
			e := event{Package: module + "internal/runner", Test: "TestRunnerProof", Action: kind}
			if kind == "package-only" {
				e.Test = ""
				e.Action = "pass"
			}
			r.add(e)
			err := r.require([]string{"internal/runner/TestRunnerProof"})
			if (err == nil) != (kind == "pass") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
