package cli

import (
	"os"
	"strings"
	"testing"
)

func TestReviewInputSafety(t *testing.T) {
	project := t.TempDir()
	t.Setenv("AFTER_PROJECT", project)
	id := "sha256:" + strings.Repeat("a", 64)
	for _, args := range [][]string{
		{"pin", id},
		{"pin", id, "--expectation", "x", "--scope", "universal", "--reason", "x"},
		{"pin", id, "--expectation", "x", "--scope", "finite_example", "--reason", strings.Repeat("x", 4097)},
		{"review", id, "--select", id, "--reason", "x"},
		{"review", id, "--mode", "original_base"},
		{"review", id, "--accept", "--receipt", id, "--reason", "x"},
		{"review", id, "--accept=false", "--reason", "x"},
		{"review", id, "--receipt", "bad\x1b]52;c;clipboard\a", "--reason", "x"},
		{"review", id, "--reason", "no mutation"},
		{"review", id, "--accept"},
	} {
		code, out, stderr := invoke(args, false, "")
		if code != ExitInvalid || out != "" || strings.ContainsAny(stderr, "\x1b\a") {
			t.Fatalf("%v: %d %q %q", args, code, out, stderr)
		}
	}
	for _, args := range [][]string{{"pin", "--help"}, {"review", "--help"}} {
		code, _, _ := invoke(args, false, "")
		if code != 0 {
			t.Fatal(code)
		}
	}
	entries, err := os.ReadDir(project)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid input/help wrote project: %v %v", entries, err)
	}
}
