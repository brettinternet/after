// Command pocgate orchestrates existing Go tests; it never substitutes fixtures
// or precomputed receipts for the engine's opt-in Docker/PTY proofs.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const module = "github.com/brettinternet/after/"

// Each design check has an executable anchor. All other tests also run.
var required = []string{
	"internal/compare/TestComparisonProof",
	"internal/compare/TestPythonHTTPServiceProof",
	"internal/cli/TestCommandCLIProof",
	"internal/runner/TestRunnerProof",
	"internal/rawdiff/TestCapturedModesInventoryAndFrozenContext",
	"internal/review/TestInvalidationMatrix",
	"internal/browser/TestLoopConsentAndSnapshotBarrier",
	"internal/runner/TestFailuresAndRedaction",
	"internal/browser/TestStatesAndLatePersistedRun",
	"internal/rawdiff/TestMissingArtifactsRedactionAndInvalidPair",
	"cmd/after/TestPaymentCLIProof",
	"internal/cli/TestReviewLoopPTYProof",
	"internal/capture/TestHostileGitConfigurationAndEnvironment",
	"internal/sandbox/TestDockerProof",
	"internal/sandbox/TestObservedProof",
	"internal/paymentfixture/TestPaymentProof",
	"internal/terminal/TestCapturedDiffBudget",
}

type event struct{ Action, Package, Test, Output string }
type results struct {
	passed, failed map[string]bool
	skipped        bool
	tail           string
}

func (r *results) add(e event) {
	r.tail += e.Output
	if len(r.tail) > 64<<10 {
		r.tail = r.tail[len(r.tail)-(64<<10):]
	}
	key := strings.TrimPrefix(e.Package, module) + "/" + e.Test
	switch e.Action {
	case "pass":
		r.passed[key] = true
	case "fail":
		r.failed[key] = true
	case "skip":
		r.skipped = true
	}
}
func (r results) require(names []string) error {
	if r.skipped {
		return errors.New("test skips are forbidden in the POC execution gate")
	}
	if len(r.failed) != 0 {
		return errors.New("test failure in POC gate")
	}
	for _, name := range names {
		if !r.passed[name] {
			return fmt.Errorf("required proof did not pass: %s", name)
		}
	}
	return nil
}

func tests(ctx context.Context, args ...string) (results, error) {
	r := results{passed: map[string]bool{}, failed: map[string]bool{}}
	cmd := exec.CommandContext(ctx, "go", append([]string{"test", "-json", "-count=1", "-p=1", "-failfast", "-timeout=12m"}, args...)...)
	cmd.Stderr = os.Stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return r, err
	}
	if err = cmd.Start(); err != nil {
		return r, err
	}
	scan := bufio.NewScanner(pipe)
	scan.Buffer(make([]byte, 4096), 4<<20)
	for scan.Scan() {
		var e event
		if err = json.Unmarshal(scan.Bytes(), &e); err != nil {
			break
		}
		r.add(e)
		fmt.Print(e.Output)
	}
	if scan.Err() != nil {
		err = scan.Err()
	}
	if err != nil {
		_ = cmd.Process.Kill()
	}
	return r, errors.Join(err, cmd.Wait())
}

// Overlay production code only. The checkout and the assertions stay unchanged.
// A compile error, skip, or unrelated failing test cannot kill a mutant.
func mutation(ctx context.Context, file, old, replacement, pkg, test, diagnostic, assertion string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if strings.Count(string(data), old) != 1 {
		return fmt.Errorf("mutation site changed: %s", file)
	}
	dir, err := os.MkdirTemp("", "after-poc-overlay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	changed := filepath.Join(dir, "mutant.go")
	if err = os.WriteFile(changed, []byte(strings.Replace(string(data), old, replacement, 1)), 0600); err != nil {
		return err
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"Replace": map[string]string{absolute: changed}})
	if err != nil {
		return err
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(overlay, b, 0600); err != nil {
		return err
	}
	fmt.Printf("\nNegative control: %s (%s)\n", diagnostic, file)
	r, runErr := tests(ctx, "-overlay="+overlay, "./"+pkg, "-run=^"+test+"$")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if runErr == nil || r.skipped || !r.failed[pkg+"/"+test] || !strings.Contains(r.tail, assertion) {
		return fmt.Errorf("negative control did not fail its assertion: %s", diagnostic)
	}
	fmt.Printf("Negative control killed: %s\n", diagnostic)
	return nil
}

// CI runs these package slices independently; the default still checks all
// anchors together. Reject unknown slices rather than succeeding with no proof.
func requiredForPackage(pkg string) ([]string, error) {
	if pkg == "" {
		return required, nil
	}
	var names []string
	for _, name := range required {
		if strings.HasPrefix(name, pkg+"/") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no required proofs in package %q", pkg)
	}
	return names, nil
}

func run() error {
	pkg := flag.String("package", "", "run one required proof package; empty runs the complete gate")
	flag.Parse()
	names, err := requiredForPackage(*pkg)
	if err != nil || flag.NArg() != 0 {
		return errors.New("select a required proof package with --package; no positional arguments")
	}
	if os.Getenv("AFTER_POC_PROOF") != "1" {
		return errors.New("use task test:poc to authorize synthetic Docker proofs")
	}
	if !filepath.IsAbs(os.Getenv("AFTER_DOCKER_BINARY")) || !strings.HasPrefix(os.Getenv("AFTER_DOCKER_HOST"), "unix:///") {
		return errors.New("explicit AFTER_DOCKER_BINARY and local AFTER_DOCKER_HOST required; provision the pinned image separately (docs/SANDBOX.md)")
	}
	for _, flag := range []string{"SANDBOX", "PAYMENT", "RUNNER", "COMPARISON", "HTTP", "COMMAND", "CLI", "TUI"} {
		if err := os.Setenv("AFTER_"+flag+"_PROOF", "1"); err != nil {
			return err
		}
	}
	budget := 60 * time.Minute
	pattern := "./..."
	if *pkg != "" {
		budget = 13 * time.Minute
		pattern = "./" + *pkg
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	r, err := tests(ctx, pattern)
	if err != nil {
		return err
	}
	if err = r.require(names); err != nil {
		return err
	}
	if *pkg == "" || *pkg == "internal/review" {
		if err = mutation(ctx, "internal/review/review.go", "case old.Snapshots != b.Snapshots:", "case false:", "internal/review", "TestInvalidationMatrix", "snapshot freshness binding", "bad reopening:"); err != nil {
			return err
		}
	}
	// Go overlays apply to embedded files too: mutate the actual frozen observer,
	// not application stdout or the expected observation.
	if *pkg == "" || *pkg == "internal/runner" {
		if err = mutation(ctx, "internal/runner/runtime/observer.go", "calls = append(calls, call{At: now, Endpoint: endpoint.Name, Destination: net.JoinHostPort(\"127.0.0.1\", strconv.Itoa(endpoint.Port)), Method: r.Method, Path: r.URL.Path, Key: r.Header.Get(\"Idempotency-Key\"), Body: string(body)})", "if len(calls) == 0 { calls = append(calls, call{At: now, Endpoint: endpoint.Name, Destination: net.JoinHostPort(\"127.0.0.1\", strconv.Itoa(endpoint.Port)), Method: r.Method, Path: r.URL.Path, Key: r.Header.Get(\"Idempotency-Key\"), Body: string(body)}) }", "internal/runner", "TestRunnerProof", "observer drops repeated provider requests", "calls want 2"); err != nil {
			return err
		}
	}
	fmt.Printf("POC GATE PASSED (%s): selected required proofs ran without skips; applicable negative controls failed as expected. Finite synthetic evidence only.\n", pattern)
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "POC GATE FAILED:", err)
		os.Exit(1)
	}
}
