package main

import (
	"slices"
	"testing"
)

func TestGateRequiresExecutedProofs(t *testing.T) {
	if _, err := requiredForPackage("internal/runner", "TestTypo"); err == nil {
		t.Fatal("unknown test would pass without a required proof")
	}
	if names, err := requiredForPackage("internal/runner", "TestRunnerFailureProof"); err != nil || !slices.Equal(names, []string{"internal/runner/TestRunnerFailureProof"}) {
		t.Fatal("failure proof slice lost its required test")
	}
	if _, err := requiredForPackage("internal/typo", ""); err == nil {
		t.Fatal("unknown package would pass without running a proof")
	}
	if names, err := requiredForPackage("", ""); err != nil || !slices.Equal(names, required) {
		t.Fatal("default gate lost required proofs")
	}
	for _, kind := range []string{"missing", "skip", "fail", "pass", "package-only"} {
		t.Run(kind, func(t *testing.T) {
			names, err := requiredForPackage("internal/review", "")
			if err != nil || !slices.Equal(names, []string{"internal/review/TestInvalidationMatrix"}) {
				t.Fatalf("wrong package requirements: %v: %v", names, err)
			}
			r := results{passed: map[string]bool{}, failed: map[string]bool{}}
			e := event{Package: module + "internal/review", Test: "TestInvalidationMatrix", Action: kind}
			if kind == "package-only" {
				e.Test = ""
				e.Action = "pass"
			}
			r.add(e)
			err = r.require(names)
			if (err == nil) != (kind == "pass") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
