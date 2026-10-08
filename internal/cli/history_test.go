package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/capture"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func TestBareDefaultsAreStoredOnlyAndNameResolvedRecords(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project's checkout")
	configFile := filepath.Join(root, "settings file.yaml")
	if err := os.WriteFile(configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// captured change\n")

	for _, args := range [][]string{{"--project", project, "--config", configFile}, {"status", "--project", project, "--config", configFile}} {
		code, output, stderr := invoke(args, false, "")
		if code != ExitOK || stderr != "" || !strings.Contains(output, "No capture has been stored") || !strings.Contains(output, "after capture") {
			t.Fatalf("bare status: %v: exit=%d stdout=%q stderr=%q", args, code, output, stderr)
		}
		if _, err := os.Stat(filepath.Join(project, ".after")); !os.IsNotExist(err) {
			t.Fatalf("read-only status created the store: %v", err)
		}
	}
	for _, args := range [][]string{{"inspect", "--project", project}, {"compare", "--project", project}, {"export", "--project", project}, {"pin", "--project", project, "--expectation", "test", "--scope", "human_intent"}} {
		code, _, diagnostic := invoke(args, false, "")
		if code != ExitInvalid || !strings.Contains(diagnostic, "no stored capture") || !strings.Contains(diagnostic, "after capture") && !strings.Contains(diagnostic, "after review") {
			t.Fatalf("missing capture default was not actionable: %v: %d %q", args, code, diagnostic)
		}
	}
	code, jsonStatus, stderr := invoke([]string{"status", "--project", project, "--config", configFile, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("empty status JSON: %d %q", code, stderr)
	}
	var empty struct {
		Kind string `json:"kind"`
		Data struct {
			Capture *json.RawMessage `json:"capture"`
			Next    []nextCommand    `json:"next"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStatus), &empty); err != nil || empty.Kind != "status" || empty.Data.Capture != nil || len(empty.Data.Next) == 0 || !strings.Contains(empty.Data.Next[0].Command, "--project ") || !strings.Contains(empty.Data.Next[0].Command, "--config ") {
		t.Fatalf("empty status did not preserve facts/global flags: %s %v", jsonStatus, err)
	}

	code, captured, stderr := invoke([]string{"capture", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("capture: %d %q", code, stderr)
	}
	var captureEnvelope struct {
		Data struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(captured), &captureEnvelope); err != nil {
		t.Fatal(err)
	}
	base, candidate := captureEnvelope.Data.Base.ID, captureEnvelope.Data.Candidate.ID
	if !validDigest(string(base)) || !validDigest(string(candidate)) {
		t.Fatalf("capture IDs unavailable: %+v", captureEnvelope.Data)
	}
	code, statusJSON, stderr := invoke([]string{"status", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("status: %d %q", code, stderr)
	}
	var status struct {
		Data statusView `json:"data"`
	}
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatal(err)
	}
	if status.Data.Capture == nil || status.Data.Capture.Base != base || status.Data.Capture.Candidate != candidate || status.Data.Base == nil || status.Data.Candidate == nil || status.Data.ChangedPaths != 1 || status.Data.Receipt != nil || status.Data.Comparison != nil {
		t.Fatalf("status facts are wrong or imply unrecorded behavior: %+v", status.Data)
	}
	if !strings.Contains(statusJSON, string(status.Data.Capture.ID)) || !strings.Contains(statusJSON, string(candidate)) {
		t.Fatalf("status did not retain full record IDs: %s", statusJSON)
	}
	if status.Data.Freshness == nil || *status.Data.Freshness != (freshness{State: freshMatches, Scope: evidence.WorkingTree}) {
		t.Fatalf("an unchanged checkout must match its capture: %+v", status.Data.Freshness)
	}
	for _, args := range [][]string{{"compare", "--project", project}, {"export", "--project", project}, {"pin", "--project", project, "--expectation", "test", "--scope", "human_intent"}} {
		code, _, diagnostic := invoke(args, false, "")
		if code != ExitInvalid || !strings.Contains(diagnostic, "no stored run receipt") || !strings.Contains(diagnostic, "run after run to prepare one for the newest capture") {
			t.Fatalf("missing run default did not suggest bare run: %v: %d %q", args, code, diagnostic)
		}
	}
	code, statusText, stderr := invoke([]string{"status", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(statusText, "Captured") || !strings.Contains(statusText, "after review --project "+shellQuote(project)) || !strings.Contains(statusText, "after diff") {
		t.Fatalf("readable status or runnable Next is incorrect: %d %q %q", code, statusText, stderr)
	}

	code, inspectText, stderr := invoke([]string{"inspect", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(inspectText, "Using the newest capture") || !strings.Contains(inspectText, "after review") {
		t.Fatalf("bare inspect default: %d %q %q", code, inspectText, stderr)
	}
	code, inspectJSON, stderr := invoke([]string{"inspect", "--project", project, "--json"}, false, "")
	var inspected struct {
		Kind string `json:"kind"`
		Data struct {
			Base      evidence.Digest `json:"base_snapshot"`
			Candidate evidence.Digest `json:"candidate_snapshot"`
			Using     resolvedIDs     `json:"using"`
		} `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(inspectJSON), &inspected) != nil || inspected.Kind != "snapshot" || inspected.Data.Base != base || inspected.Data.Candidate != candidate || inspected.Data.Using.Capture == "" || inspected.Data.Using.Candidate != candidate {
		t.Fatalf("bare inspect JSON lacks full resolved IDs: %d %q %q", code, inspectJSON, stderr)
	}

	receiptID, _ := seedCLIProcessRecords(t, project, string(base), string(candidate))
	code, compareText, stderr := invoke([]string{"compare", "--project", project}, false, "")
	if code != ExitOperational || stderr != "" || !strings.Contains(compareText, "Using the newest run of base") || !strings.Contains(compareText, shortID(receiptID)) || strings.Contains(compareText, "provider requests 1 → 2") {
		t.Fatalf("bare compare must identify incomplete stored evidence without invented observations: %d %q %q", code, compareText, stderr)
	}
	code, compareJSON, stderr := invoke([]string{"compare", "--project", project, "--json"}, false, "")
	var compared struct {
		Data comparisonResult `json:"data"`
	}
	if code != ExitOperational || stderr != "" || json.Unmarshal([]byte(compareJSON), &compared) != nil || compared.Data.Using == nil || compared.Data.Using.Capture == "" || compared.Data.Using.Receipt != receiptID || compared.Data.Comparison.ID == "" || compared.Data.Receipt.ID != receiptID {
		t.Fatalf("bare compare JSON lacks full resolved IDs: %d %q %q", code, compareJSON, stderr)
	}
	comparisonID := compared.Data.Comparison.ID

	code, exportJSON, stderr := invoke([]string{"export", "--project", project}, false, "")
	var exported struct {
		Kind string `json:"kind"`
		Data struct {
			Comparison evidence.Comparison `json:"comparison"`
			Using      resolvedIDs         `json:"using"`
		} `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(exportJSON), &exported) != nil || exported.Kind != "export" || exported.Data.Comparison.ID != comparisonID || exported.Data.Using.Capture == "" || exported.Data.Using.Receipt != receiptID || exported.Data.Using.Comparison != comparisonID {
		t.Fatalf("bare export JSON did not identify the newest stored comparison: %d %q %q", code, exportJSON, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(exportJSON), "{") || strings.Contains(exportJSON, "Next") {
		t.Fatalf("export stopped being JSON-only: %q", exportJSON)
	}

	code, pinnedJSON, stderr := invoke([]string{"pin", "--project", project, "--expectation", "finite example expectation", "--scope", "human_intent", "--json"}, false, "")
	var pinned struct {
		Data struct {
			review.View
			Using *resolvedIDs `json:"using"`
		} `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(pinnedJSON), &pinned) != nil || pinned.Data.Pin.BasisReceipt != receiptID || pinned.Data.Using == nil || pinned.Data.Using.Capture == "" || pinned.Data.Using.Receipt != receiptID {
		t.Fatalf("pin expectation default did not identify its receipt: %d %q %q", code, pinnedJSON, stderr)
	}
	pinID := pinned.Data.Pin.ID
	code, pinListJSON, stderr := invoke([]string{"pin", "--project", project, "--json"}, false, "")
	var pinList struct {
		Data pinListView `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(pinListJSON), &pinList) != nil || len(pinList.Data.Pins) != 1 || pinList.Data.Pins[0].Pin.ID != pinID {
		t.Fatalf("bare pin did not list its computed head: %d %q %q", code, pinListJSON, stderr)
	}
	code, _, diagnostic := invoke([]string{"pin", "--project", project, "--accept"}, false, "")
	if code != ExitInvalid || !strings.Contains(diagnostic, "missing pin revision ID") {
		t.Fatalf("pin mutation implicitly selected a revision: %d %q", code, diagnostic)
	}

	reportPath := filepath.Join(root, "go test.jsonl")
	reportBytes, err := os.ReadFile(filepath.Join("..", "gotestreport", "testdata", "tests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, reportBytes, 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = invoke([]string{"import", reportPath, "--producer", "synthetic report", "--snapshot", string(candidate), "--project", project}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("report fixture import: %d %q", code, stderr)
	}
	code, logJSON, stderr := invoke([]string{"log", "-n", "50", "--project", project, "--json"}, false, "")
	var log struct {
		Data logView `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(logJSON), &log) != nil {
		t.Fatalf("log JSON: %d %q %q", code, logJSON, stderr)
	}
	found := map[string]bool{}
	for _, row := range log.Data.Rows {
		found[row.Kind] = true
		if !validDigest(string(row.ID)) {
			t.Errorf("log row has no full record ID: %+v", row)
		}
	}
	for _, kind := range []string{"capture", "run", "report", "pin"} {
		if !found[kind] {
			t.Errorf("log omitted %s events: %+v", kind, log.Data.Rows)
		}
	}
	if log.Data.Total != log.Data.Shown || len(log.Data.Rows) != log.Data.Shown {
		t.Fatalf("log totals disagree: %+v", log.Data)
	}
	code, shortLogJSON, stderr := invoke([]string{"log", "-n", "2", "--project", project, "--json"}, false, "")
	var shortLog struct {
		Data logView `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(shortLogJSON), &shortLog) != nil || shortLog.Data.Shown != 2 || shortLog.Data.Total <= 2 {
		t.Fatalf("-n did not change the bounded newest-first log: %d %s %q", code, shortLogJSON, stderr)
	}
	if shortLog.Data.Rows[0].At.Before(shortLog.Data.Rows[1].At) {
		t.Fatalf("log is not newest first: %+v", shortLog.Data.Rows)
	}
	code, logText, stderr := invoke([]string{"log", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(logText, "Recent stored records") || !strings.Contains(logText, "newest of") || !strings.Contains(logText, "Next") {
		t.Fatalf("readable log missing newest-first summary/footer/Next: %d %q %q", code, logText, stderr)
	}

	readOnly, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	pins, err := review.Heads(readOnly)
	if err != nil || len(pins) != 1 || pins[0].ID != pinID {
		t.Fatalf("pin head changed after bare defaults: %+v %v", pins, err)
	}
	readOnly.Close()

	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// second captured change\n")
	code, secondCapture, stderr := invoke([]string{"capture", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("second capture: %d %q", code, stderr)
	}
	var second struct {
		Data struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(secondCapture), &second); err != nil {
		t.Fatal(err)
	}
	code, statusAfterChange, stderr := invoke([]string{"status", "--project", project, "--json"}, false, "")
	var nextStatus struct {
		Data statusView `json:"data"`
	}
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(statusAfterChange), &nextStatus) != nil || len(nextStatus.Data.Next) == 0 || nextStatus.Data.Next[0].Command != "after run --project "+shellQuote(project) {
		t.Fatalf("no-run Next did not suggest the bare newest-capture run: %d %s %q", code, statusAfterChange, stderr)
	}
	before, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	beforeRuns, err := before.List("receipt")
	before.Close()
	if err != nil {
		t.Fatal(err)
	}
	suggestionCode, suggestionOutput, err := runShellSuggestion(t, root, nextStatus.Data.Next[0].Command)
	if err != nil || suggestionCode != ExitDenied || !strings.Contains(suggestionOutput, "Nothing has run") {
		t.Fatalf("run suggestion was not safe/runnable as written: exit=%d output=%q err=%v", suggestionCode, suggestionOutput, err)
	}
	after, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	afterRuns, err := after.List("receipt")
	after.Close()
	if err != nil || len(afterRuns) != len(beforeRuns) {
		t.Fatalf("suggested preview executed or persisted a run: before=%d after=%d err=%v", len(beforeRuns), len(afterRuns), err)
	}

	// A saved review on the older pair outranks the no-run suggestion.
	savedPair := evidence.SnapshotPair{Base: base, Candidate: candidate}
	if err := saveReviewSessionFile(project, browser.NewReviewSession(savedPair, capture.Options{Mode: evidence.WorkingTree})); err != nil {
		t.Fatal(err)
	}
	code, output, stderr := invoke([]string{"status", "--project", project, "--json"}, false, "")
	if code != ExitOK || stderr != "" || json.Unmarshal([]byte(output), &nextStatus) != nil || nextStatus.Data.SavedReview == nil || *nextStatus.Data.SavedReview != savedPair {
		t.Fatalf("saved review missing from status: %d %s %q", code, output, stderr)
	}
	if len(nextStatus.Data.Next) != 2 || nextStatus.Data.Next[0].Command != "after review --project "+shellQuote(project) || nextStatus.Data.Next[1].Command != "after review --new --project "+shellQuote(project) {
		t.Fatalf("saved review must take precedence: %+v", nextStatus.Data.Next)
	}
	code, output, stderr = invoke([]string{"status", "--project", project}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, "Saved review") || !strings.Contains(output, shortID(candidate)) {
		t.Fatalf("saved review missing from readable status: %d %s %q", code, output, stderr)
	}
}

func TestLatestComparisonUsesCompletionThenComparisonID(t *testing.T) {
	finished := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	pair := evidence.SnapshotPair{Base: "base", Candidate: "candidate"}
	history := storedHistory{
		receipts: []evidence.Receipt{
			{ID: "run-a", Snapshots: pair, StartedAt: finished.Add(-time.Minute), FinishedAt: finished},
			{ID: "run-b", Snapshots: pair, StartedAt: finished.Add(-time.Hour), FinishedAt: finished},
		},
		comparisons: []evidence.Comparison{{ID: "comparison-a", Receipt: "run-a"}, {ID: "comparison-b", Receipt: "run-b"}},
	}
	comparison, receipt, ok := latestComparison(history, pair)
	if !ok || comparison.ID != "comparison-b" || receipt.ID != "run-b" {
		t.Fatalf("completion tie must use comparison ID, not start time: %+v %+v %t", comparison, receipt, ok)
	}
}

// Bare diff must show edits made after the newest capture, and bare status
// must not present that capture as current, without storing anything.
func TestLiveDiffAndFreshnessNeverReuseOrStoreStaleCaptures(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// first edit\n")
	code, live, stderr := invoke([]string{"diff", "--raw", "--project", project}, false, "")
	if code != ExitOK || !strings.Contains(live, "+// first edit") || !strings.Contains(stderr, "not stored") {
		t.Fatalf("live diff without a store: %d %q %q", code, live, stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("live diff created the store: %v", err)
	}
	if code, _, stderr := invoke([]string{"capture", "--project", project}, false, ""); code != ExitOK {
		t.Fatalf("capture: %d %q", code, stderr)
	}
	stored := storeListing(t, project)

	// A same-size edit with the original timestamp is still a change.
	path := filepath.Join(project, "app/main.go")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// later edit\n")
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	code, live, _ = invoke([]string{"diff", "--raw", "--project", project}, false, "")
	if code != ExitOK || !strings.Contains(live, "+// later edit") || strings.Contains(live, "first edit") {
		t.Fatalf("bare diff did not show the current checkout: %d %q", code, live)
	}
	code, old, _ := invoke([]string{"diff", "--stored", "--raw", "--project", project}, false, "")
	if code != ExitOK || !strings.Contains(old, "+// first edit") || strings.Contains(old, "later edit") {
		t.Fatalf("diff --stored did not print the stored capture: %d %q", code, old)
	}
	for _, args := range [][]string{{"status"}, {"inspect"}} {
		code, output, stderr := invoke(append(args, "--project", project, "--json"), false, "")
		var result struct {
			Data struct {
				Freshness *freshness    `json:"freshness"`
				Next      []nextCommand `json:"next"`
			} `json:"data"`
		}
		if code != ExitOK || stderr != "" || json.Unmarshal([]byte(output), &result) != nil || result.Data.Freshness == nil || result.Data.Freshness.State != freshChanged {
			t.Fatalf("%v presented a stale capture as current: %d %q %q", args, code, output, stderr)
		}
		if args[0] == "status" && (len(result.Data.Next) == 0 || !strings.HasPrefix(result.Data.Next[0].Command, "after diff --project")) {
			t.Fatalf("stale status must lead with the current change: %+v", result.Data.Next)
		}
	}
	code, output, _ := invoke([]string{"status", "--stored", "--project", project, "--json"}, false, "")
	if code != ExitOK || !strings.Contains(output, `"freshness":{"state":"not_checked","scope":"working_tree","reason":"stored_requested"}`) {
		t.Fatalf("--stored must skip the checkout check: %d %q", code, output)
	}

	// An unreadable checkout prints no patch and never claims freshness.
	gitRun(t, project, "config", "core.sparseCheckout", "true")
	code, live, stderr = invoke([]string{"diff", "--project", project}, false, "")
	if code != ExitOperational || live != "" || !strings.Contains(stderr, "no patch was printed") || !strings.Contains(stderr, "after diff --stored") {
		t.Fatalf("failed live read fell back or was silent: %d %q %q", code, live, stderr)
	}
	code, output, stderr = invoke([]string{"status", "--project", project}, false, "")
	if code != ExitOperational || !strings.Contains(output, "Stored capture") || !strings.Contains(output, "not verified") || !strings.Contains(stderr, "after status --stored") {
		t.Fatalf("failed freshness check must keep the labeled stored summary and fail: %d %q %q", code, output, stderr)
	}
	if after := storeListing(t, project); !slices.Equal(after, stored) {
		t.Fatalf("read commands changed the store:\nbefore %v\nafter  %v", stored, after)
	}
}

func storeListing(t *testing.T, project string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(filepath.Join(project, ".after"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		names = append(names, fmt.Sprintf("%s %d %s", path, info.Size(), info.ModTime()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestNoCheckoutBareAfterShowsHelpAndProjectSuggestionsExecuteInShell(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "checkout's dir")
	makeProject(t, project)
	configFile := filepath.Join(root, "config with spaces.yaml")
	if err := os.WriteFile(configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(outside)
	for _, args := range [][]string{nil, {"--project", outside}} {
		code, stdout, stderr := invoke(args, false, "")
		if code != ExitOK || stderr != "" || !strings.Contains(stdout, "Everyday") || strings.Contains(stdout, "No capture has been stored") {
			t.Fatalf("bare outside checkout did not print short help: %v: %d %q %q", args, code, stdout, stderr)
		}
	}

	code, stdout, stderr := invoke([]string{"--project", project, "--config", configFile}, false, "")
	if code != ExitOK || stderr != "" {
		t.Fatalf("bare status: %d %q %q", code, stdout, stderr)
	}
	if first := firstNextCommand(t, stdout); !strings.HasPrefix(first, "after review ") {
		t.Fatalf("an empty checkout must suggest the review flow first: %q", first)
	}
	command := nextCommandWith(t, stdout, "after capture ")
	if !strings.Contains(command, "--project "+shellQuote(project)) || !strings.Contains(command, "--config "+shellQuote(configFile)) {
		t.Fatalf("suggestion omitted or failed to quote selected globals: %q", command)
	}
	code, output, err := runShellSuggestion(t, outside, command)
	if err != nil || code != ExitOK || !strings.Contains(output, "Captured candidate") {
		t.Fatalf("suggested command did not run as written: code=%d output=%q err=%v", code, output, err)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := s.List("capture")
	s.Close()
	if err != nil || len(entries) != 1 {
		t.Fatalf("shell suggestion did not capture into the selected checkout: %d entries, %v", len(entries), err)
	}
}

func firstNextCommand(t *testing.T, output string) string {
	t.Helper()
	return nextCommandWith(t, output, "after ")
}

func nextCommandWith(t *testing.T, output, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return trimmed
		}
	}
	t.Fatalf("no %q Next command in output: %q", prefix, output)
	return ""
}

func runShellSuggestion(t *testing.T, directory, suggestion string) (int, string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		return -1, "", err
	}
	binDir := t.TempDir()
	wrapper := filepath.Join(binDir, "after")
	script := "#!/bin/sh\nexec " + shellQuote(executable) + " -test.run='^TestCLIProcessHelper$' -- \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		return -1, "", err
	}
	cmd := exec.Command("sh", "-c", suggestion)
	cmd.Dir = directory
	cmd.Env = []string{"PATH=" + binDir + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir(), cliProcessHelperEnv + "=1", "TERM=dumb"}
	output, runErr := cmd.CombinedOutput()
	code, exitErr := processExitCode(runErr)
	if exitErr != nil {
		return code, string(output), exitErr
	}
	return code, string(output), nil
}

func TestStatusJSONDefaultsNeverCreateOrExecute(t *testing.T) {
	project := filepath.Join(t.TempDir(), "checkout")
	makeProject(t, project)
	for _, args := range [][]string{{"--project", project, "--json"}, {"status", "--project", project, "--json"}} {
		code, output, stderr := invoke(args, false, "")
		if code != ExitOK || stderr != "" || !strings.Contains(output, `"kind":"status"`) {
			t.Fatalf("status default: %v: %d %q %q", args, code, output, stderr)
		}
		if _, err := os.Stat(filepath.Join(project, ".after")); !os.IsNotExist(err) {
			t.Fatalf("bare status created storage: %v", err)
		}
	}
}

func TestNextBlockHasAtMostThreeCommands(t *testing.T) {
	view := logView{Rows: []logRow{}, Next: []nextCommand{next("after log -n 40", "more"), next("after status", "status")}}
	state := &invocation{stdout: &strings.Builder{}}
	if err := writeReadable(state, "log", view); err != nil {
		t.Fatal(err)
	}
	output := state.stdout.(*strings.Builder).String()
	if strings.Count(output, "  after ") != 2 || !strings.HasSuffix(output, "    status\n") {
		t.Fatalf("Next block was not bounded and terminal: %q", output)
	}
}
