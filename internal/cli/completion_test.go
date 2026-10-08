package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
)

func TestCompletionShellSelectionAndHelp(t *testing.T) {
	for _, shell := range completionShells {
		t.Run(shell, func(t *testing.T) {
			code, stdout, stderr := invoke([]string{"completion", shell}, false, "")
			script, _ := completionScript(shell)
			if code != ExitOK || stderr != "" || stdout != script {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
	t.Setenv("SHELL", "/bin/zsh")
	code, stdout, stderr := invoke([]string{"completion"}, false, "")
	if want, _ := completionScript("zsh"); code != ExitOK || stderr != "" || stdout != want {
		t.Fatalf("default SHELL: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke([]string{"completion", "tcsh"}, false, "")
	if code != ExitInvalid || stdout != "" || !strings.Contains(stderr, "supported shells: bash, zsh, fish") {
		t.Fatalf("unknown shell: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke([]string{"--help"}, false, "")
	if code != ExitOK || !strings.Contains(stdout, "after completion") || stderr != "" {
		t.Fatalf("help omits completion: %d %q %q", code, stdout, stderr)
	}
}

func TestCompletionOffersApplicationAndCommandFlags(t *testing.T) {
	root, err := complete([]string{"--v"})
	if err != nil || len(root) != 1 || root[0].word != "--version" {
		t.Fatalf("application version flag: %+v %v", root, err)
	}
	command, err := complete([]string{"diff", "--ra"})
	if err != nil || len(command) != 1 || command[0].word != "--raw" {
		t.Fatalf("diff command flags: %+v %v", command, err)
	}
}

func TestCompletionGrammarSelectsValidIDKinds(t *testing.T) {
	project, _ := completionFixture(t)
	cases := []struct {
		name  string
		words []string
		kinds []string
	}{
		{"top-level commands", []string{"ins"}, nil},
		{"pin scope", []string{"pin", "--scope", "fi"}, nil},
		{"pin mode", []string{"pin", "PIN", "--select", "SNAPSHOT", "--mode", "la"}, nil},
		{"comparison receipt", []string{"compare", "--project", project, ""}, []string{"receipt"}},
		{"diff snapshots", []string{"diff", "--project", project, ""}, []string{"snapshot"}},
		{"pin revision", []string{"pin", "--project", project, ""}, []string{"pin"}},
		{"pin creation receipt", []string{"pin", "--project", project, "--expectation", "text", ""}, []string{"receipt"}},
		{"pin attachment receipt", []string{"pin", "--project", project, "--attach", ""}, []string{"receipt"}},
		{"pin selection snapshot", []string{"pin", "--project", project, "--select", ""}, []string{"snapshot"}},
		{"import binding snapshot", []string{"import", "--project", project, "--snapshot", ""}, []string{"snapshot"}},
		{"review ID", []string{"review", "--project", project, ""}, []string{"snapshot", "receipt", "comparison", "report", "pin"}},
		{"review pair second ID", []string{"review", "BASE", "--project", project, ""}, []string{"snapshot"}},
		{"review evidence ID", []string{"review", "BASE", "CANDIDATE", "--project", project, ""}, browserKinds},
		{"inspect record", []string{"inspect", "--project", project, ""}, inspectKinds},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidates, err := complete(tc.words)
			if err != nil {
				t.Fatal(err)
			}
			if tc.kinds == nil {
				if len(candidates) == 0 {
					t.Fatal("no static candidates")
				}
				if tc.name == "pin scope" && candidates[0].word != "finite_example" {
					t.Fatalf("scope candidates: %+v", candidates)
				}
				if tc.name == "pin mode" && candidates[0].word != "last_inspected" {
					t.Fatalf("mode candidates: %+v", candidates)
				}
				return
			}
			if len(candidates) > 50 {
				t.Fatalf("completion returned %d candidates", len(candidates))
			}
			allowed := map[string]bool{}
			for _, kind := range tc.kinds {
				allowed[kind] = true
			}
			for _, candidate := range candidates {
				kind, _, _ := strings.Cut(candidate.description, ": ")
				if !allowed[kind] {
					t.Errorf("unexpected %s ID candidate: %+v", kind, candidate)
				}
			}
		})
	}
}

func TestCompletionReadOnlyIDsKindsOrderingAndLimits(t *testing.T) {
	project, ids := completionFixture(t)

	receipts, err := completionIDs(project, []string{"receipt"})
	if err != nil || len(receipts) != 2 || receipts[0].word != shortID(ids.newestReceipt) || receipts[1].word != shortID(ids.receipt) {
		t.Fatalf("newest matching receipts first: %+v %v", receipts, err)
	}
	for _, candidate := range receipts {
		if !strings.HasPrefix(candidate.description, "receipt: ") {
			t.Fatalf("receipt candidate description: %+v", candidate)
		}
	}
	pins, err := completionIDs(project, []string{"pin"})
	if err != nil || len(pins) != 1 || pins[0].word != shortID(ids.pin) || !strings.HasPrefix(pins[0].description, "pin: ") {
		t.Fatalf("pin-only candidates: %+v %v", pins, err)
	}
	inspect, err := completionIDs(project, inspectKinds)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, candidate := range inspect {
		kind, _, _ := strings.Cut(candidate.description, ": ")
		kinds[kind]++
		if len(candidate.word) != 8 {
			t.Fatalf("completion ID is not short: %+v", candidate)
		}
	}
	for _, kind := range []string{"snapshot", "receipt", "comparison", "report", "artifact", "plan"} {
		if kinds[kind] == 0 {
			t.Errorf("inspect candidates omit valid kind %q: %+v", kind, kinds)
		}
	}
	if kinds["pin"] != 0 {
		t.Fatalf("inspect offered an invalid pin ID: %+v", inspect)
	}

	ordered, err := completionIDs(project, []string{"snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	newerPosition, olderPosition := candidatePosition(ordered, shortID(ids.newestSnapshot)), candidatePosition(ordered, shortID(ids.snapshot))
	if newerPosition < 0 || olderPosition < 0 || newerPosition >= olderPosition {
		t.Fatalf("snapshot candidates are not newest first: %+v", ordered)
	}

	moreProject := filepath.Join(t.TempDir(), "large-project")
	if err := os.MkdirAll(filepath.Join(moreProject, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(moreProject, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	baseDiff, err := s.PutArtifact([]byte("completion base diff"), "completion-diff", 1024)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	base, err := store.Put(s, evidence.Snapshot{
		SchemaVersion: evidence.SchemaVersion, Source: evidence.Commit, Commit: strings.Repeat("a", 40),
		Files: []evidence.File{}, Excluded: []evidence.Limitation{}, Unsupported: []evidence.Limitation{},
		Completeness: evidence.Complete, Diff: baseDiff.Content, Limits: []string{},
	})
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	for i := 1; i <= 60; i++ {
		diff, err := s.PutArtifact([]byte(fmt.Sprintf("completion candidate diff %d", i)), "completion-diff", 1024)
		if err != nil {
			s.Close()
			t.Fatal(err)
		}
		candidate := evidence.Snapshot{
			SchemaVersion: evidence.SchemaVersion, Source: evidence.MergeBase, Commit: strings.Repeat("a", 40),
			MergeBase: strings.Repeat("a", 40), Files: []evidence.File{}, Excluded: []evidence.Limitation{},
			Unsupported: []evidence.Limitation{}, Completeness: evidence.Complete,
			Diff: diff.Content, Limits: []string{},
		}
		candidate, err = store.Put(s, candidate)
		if err != nil {
			s.Close()
			t.Fatal(err)
		}
		capture := evidence.Capture{
			SchemaVersion: evidence.SchemaVersion, CapturedAt: time.Unix(int64(i), 0).UTC(),
			Mode: evidence.MergeBase, Base: base.ID, Candidate: candidate.ID, SelectedUntracked: []string{},
		}
		if _, err := store.Put(s, capture); err != nil {
			s.Close()
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	limited, err := completionIDs(moreProject, []string{"snapshot"})
	if err != nil || len(limited) != 50 || limited[0].word != shortID(lastSnapshotID(moreProject, t)) {
		t.Fatalf("expected newest 50 snapshot IDs: len=%d first=%+v err=%v", len(limited), limited[0], err)
	}
	for i := 1; i < len(limited); i++ {
		if limited[i-1].word == limited[i].word {
			t.Fatalf("duplicate candidate: %+v", limited[i])
		}
	}

	missing := filepath.Join(t.TempDir(), "missing-project")
	if err := os.MkdirAll(filepath.Join(missing, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"__complete", "bash", "compare", "--project", missing, ""}
	code, stdout, stderr := invoke(args, false, "")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("absent-store completion: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(missing, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completion created an absent store: %v", err)
	}
}

type completionFixtureIDs struct {
	snapshot, newestSnapshot evidence.Digest
	receipt, newestReceipt   evidence.Digest
	pin                      evidence.Digest
	marker                   string
}

func candidatePosition(candidates []completionCandidate, word string) int {
	for i, candidate := range candidates {
		if candidate.word == word {
			return i
		}
	}
	return -1
}

func completionFixture(t *testing.T) (string, completionFixtureIDs) {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	capture := func() evidence.SnapshotPair {
		t.Helper()
		code, output, stderr := invoke([]string{"capture", "--project", project, "--json"}, false, "")
		if code != ExitOK || stderr != "" {
			t.Fatalf("capture setup: exit=%d stdout=%q stderr=%q", code, output, stderr)
		}
		var result struct {
			Data struct {
				Base struct {
					ID evidence.Digest `json:"id"`
				} `json:"base_snapshot"`
				Candidate struct {
					ID evidence.Digest `json:"id"`
				} `json:"candidate_snapshot"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		return evidence.SnapshotPair{Base: result.Data.Base.ID, Candidate: result.Data.Candidate.ID}
	}
	first := capture()
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// newest candidate\n")
	second := capture()

	receiptID, _ := seedCLIProcessRecords(t, project, string(second.Base), string(second.Candidate))
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.Get[evidence.Receipt](s, receiptID)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	newestSnapshot, err := store.Get[evidence.Snapshot](s, second.Candidate)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	newestCapture := evidence.Capture{
		SchemaVersion: evidence.SchemaVersion, CapturedAt: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		Mode: evidence.WorkingTree, Base: second.Base, Candidate: second.Candidate,
		Index: newestSnapshot.IndexSnapshot, SelectedUntracked: []string{},
	}
	if _, err := store.Put(s, newestCapture); err != nil {
		s.Close()
		t.Fatal(err)
	}
	newest := receipt
	newest.ID = ""
	newest.StartedAt = newest.StartedAt.Add(24 * time.Hour)
	newest.FinishedAt = newest.FinishedAt.Add(24 * time.Hour)
	newest, err = store.Put(s, newest)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	comparison := evidence.Comparison{
		SchemaVersion: evidence.SchemaVersion, Receipt: receiptID, Outcome: evidence.Incomparable,
		Completeness: evidence.Incomplete, Limits: []string{"synthetic completion fixture"},
	}
	comparison, err = store.Put(s, comparison)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	newestComparison := evidence.Comparison{
		SchemaVersion: evidence.SchemaVersion, Receipt: newest.ID, Outcome: evidence.Incomparable,
		Completeness: evidence.Incomplete, Limits: []string{"synthetic newest completion fixture"},
	}
	if _, err := store.Put(s, newestComparison); err != nil {
		s.Close()
		t.Fatal(err)
	}

	marker := filepath.Join(t.TempDir(), "completion-injected")
	hostile := fmt.Sprintf("q:'\"\n$(printf pwn > %q)\x1b]52;c;clipboard\a\u202e", marker)
	pin := evidence.Pin{
		SchemaVersion: evidence.SchemaVersion, Scenario: receipt.Bindings.Scenario,
		Expectation: hostile, BasisReceipt: newest.ID,
		BasisSnapshots: newest.Snapshots, Decision: evidence.Pinned,
		History: []evidence.DecisionEvent{{Decision: evidence.Pinned, At: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC), Reason: "synthetic hostile description"}},
	}
	pin, err = store.Put(s, pin)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	planBytes := []byte("{\"plan\":\"synthetic exact plan\"}")
	planID := evidence.Digest("sha256:" + fmt.Sprintf("%x", sha256Sum(planBytes)))
	if err := s.PutPlan(planID, planBytes); err != nil {
		s.Close()
		t.Fatal(err)
	}
	report, err := gotestreport.Import(strings.NewReader("{\"Action\":\"pass\",\"Package\":\"fixture\"}\n"), gotestreport.Metadata{
		Producer:   "hostile: 'quoted' \"value\"\nnext\trow\x1b[31m\a",
		ImportedAt: time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	reportBytes, err := json.Marshal(report)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	if _, err := s.PutArtifact(reportBytes, "go-test-report-v1", store.MaxBlobBytes); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test description unexpectedly executed: %v", err)
	}
	return project, completionFixtureIDs{
		snapshot: first.Candidate, newestSnapshot: second.Candidate,
		receipt: receiptID, newestReceipt: newest.ID, pin: pin.ID, marker: marker,
	}
}

func lastSnapshotID(project string, t *testing.T) evidence.Digest {
	t.Helper()
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	entries, err := s.List("snapshot")
	if err != nil || len(entries) == 0 {
		t.Fatalf("snapshot index: %v %v", entries, err)
	}
	captureEntries, err := s.List("capture")
	if err != nil {
		t.Fatal(err)
	}
	latest := time.Time{}
	var newest evidence.Digest
	for _, entry := range captureEntries {
		capture, err := store.Get[evidence.Capture](s, entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		if capture.CapturedAt.After(latest) {
			latest, newest = capture.CapturedAt, capture.Candidate
		}
	}
	return newest
}

func sha256Sum(raw []byte) [32]byte {
	return sha256.Sum256(raw)
}

func TestCompletionScriptsInBashZshAndFish(t *testing.T) {
	project, ids := completionFixture(t)
	testBin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	shim := filepath.Join(shimDir, "after")
	wrapper := "#!/bin/sh\nAFTER_CLI_PROCESS_HELPER=1 exec " + shellQuote(testBin) + " -test.run=^TestCLIProcessHelper$ -- \"$@\"\n"
	if err := os.WriteFile(shim, []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	scriptDir := t.TempDir()
	scripts := map[string]string{}
	for _, shell := range completionShells {
		text, _ := completionScript(shell)
		path := filepath.Join(scriptDir, shell+".completion")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		scripts[shell] = path
	}
	env := completionTestEnv(shimDir, project)

	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("Bash is required for completion tests:", err)
	}
	bashProgram := `source "$1"
complete_line() {
  COMP_WORDS=(after "$@")
  COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
  _after_complete
  printf '%s\n' "${COMPREPLY[@]}"
}
printf 'COMMANDS\n'; complete_line ''
printf 'FLAGS\n'; complete_line pin --mo
printf 'SCOPE\n'; complete_line pin --scope ''
printf 'MODE\n'; complete_line pin --select x --mode ''
printf 'COMPARE\n'; complete_line compare ''
printf 'ATTACH\n'; complete_line pin pin --attach ''
printf 'DIFF\n'; complete_line diff ''
printf 'SELECT\n'; complete_line pin pin --select ''
printf 'IMPORT\n'; complete_line import --snapshot ''
`
	bashOut := runShell(t, bash, []string{"--noprofile", "--norc", "-c", bashProgram, "bash", scripts["bash"]}, project, env)
	for _, want := range []string{"review", "--mode", "finite_example", "human_intent", "original_base", "last_inspected", shortID(ids.newestReceipt), shortID(ids.newestSnapshot)} {
		if !strings.Contains(bashOut, want) {
			t.Errorf("Bash completion missing %q: %s", want, bashOut)
		}
	}
	if strings.Index(bashOut, shortID(ids.newestReceipt)) > strings.Index(bashOut, shortID(ids.receipt)) {
		t.Errorf("Bash receipt suggestions are not newest first: %s", bashOut)
	}

	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Fatal("Zsh is required for completion tests:", err)
	}
	zshProgram := `autoload -Uz compinit
compinit -D
source "$1"
function compadd {
  local candidate i=1
  while [[ "$1" != "--" && $# -gt 0 ]]; do
    shift
  done
  shift
  for candidate in "$@"; do
    printf '%s\t%s\n' "$candidate" "${descriptions[i]}"
    (( i++ ))
  done
}
complete_line() {
  words=(after "$@")
  CURRENT=${#words}
  PREFIX=${words[CURRENT]}
  _after
}
printf 'COMMANDS\n'; complete_line ''
printf 'FLAGS\n'; complete_line pin --mo
printf 'SCOPE\n'; complete_line pin --scope ''
printf 'MODE\n'; complete_line pin --select x --mode ''
printf 'COMPARE\n'; complete_line compare ''
printf 'ATTACH\n'; complete_line pin pin --attach ''
printf 'DIFF\n'; complete_line diff ''
printf 'SELECT\n'; complete_line pin pin --select ''
printf 'IMPORT\n'; complete_line import --snapshot ''
printf 'PIN\n'; complete_line pin ''
`
	zshOut := runShell(t, zsh, []string{"-fc", zshProgram, "zsh", scripts["zsh"]}, project, env)
	for _, want := range []string{"review", "--mode", "finite_example", "human_intent", "original_base", "last_inspected", shortID(ids.newestReceipt), shortID(ids.newestSnapshot), "pin: q:"} {
		if !strings.Contains(zshOut, want) {
			t.Errorf("Zsh completion missing %q: %s", want, zshOut)
		}
	}
	if strings.ContainsAny(zshOut, "\x1b\a\u202e") || !strings.Contains(zshOut, "$(printf pwn") {
		t.Errorf("unsafe completion description reached Zsh: %q", zshOut)
	}
	if strings.Index(zshOut, shortID(ids.newestReceipt)) > strings.Index(zshOut, shortID(ids.receipt)) {
		t.Errorf("Zsh receipt suggestions are not newest first: %s", zshOut)
	}

	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Fatal("Fish is required for completion tests; install the pinned mise tool:", err)
	}
	fishProgram := `source $argv[1]
printf 'COMMANDS\n'
complete -C 'after '
printf 'FLAGS\n'
complete -C 'after pin --mo'
printf 'SCOPE\n'
complete -C 'after pin --scope '
printf 'MODE\n'
complete -C 'after pin --select x --mode '
printf 'COMPARE\n'
complete -C 'after compare '
printf 'ATTACH\n'
complete -C 'after pin pin --attach '
printf 'DIFF\n'
complete -C 'after diff '
printf 'SELECT\n'
complete -C 'after pin pin --select '
printf 'IMPORT\n'
complete -C 'after import --snapshot '
printf 'PIN\n'
complete -C 'after pin '
`
	fishOut := runShell(t, fish, []string{"-c", fishProgram, scripts["fish"]}, project, env)
	for _, want := range []string{"review", "--mode", "finite_example", "human_intent", "original_base", "last_inspected", shortID(ids.newestReceipt), shortID(ids.newestSnapshot)} {
		if !strings.Contains(fishOut, want) {
			t.Errorf("Fish completion missing %q: %s", want, fishOut)
		}
	}
	if strings.ContainsAny(fishOut, "\x1b\a\u202e") || !strings.Contains(fishOut, "$(printf pwn") {
		t.Errorf("unsafe completion description reached Fish: %q", fishOut)
	}
	if strings.Index(fishOut, shortID(ids.newestReceipt)) > strings.Index(fishOut, shortID(ids.receipt)) {
		t.Errorf("Fish receipt suggestions are not newest first: %s", fishOut)
	}
	if _, err := os.Stat(ids.marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hostile completion description executed: %v", err)
	}
}

func completionTestEnv(shimDir, project string) []string {
	values := map[string]string{}
	for _, item := range os.Environ() {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			values[name] = value
		}
	}
	values["PATH"] = shimDir + string(os.PathListSeparator) + values["PATH"]
	values["AFTER_PROJECT"] = project
	values["AFTER_CLI_PROCESS_HELPER"] = "1"
	delete(values, "NO_COLOR")
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func runShell(t *testing.T, shell string, args []string, project string, env []string) string {
	t.Helper()
	command := exec.Command(shell, args...)
	command.Dir = project
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%s %q failed: %v\nstdout: %s\nstderr: %s", shell, args, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestCompletionBackendDoesNotRunDataCommands(t *testing.T) {
	project := filepath.Join(t.TempDir(), "empty-checkout")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := invoke([]string{"__complete", "bash", "inspect", "--project", project, ""}, false, "")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("completion backend: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completion opened an absent store for writing: %v", err)
	}
}

func TestCompletionOutputDescriptionsAreSingleLineAndSanitized(t *testing.T) {
	project, ids := completionFixture(t)
	code, output, stderr := invoke([]string{"__complete", "zsh", "pin", "--project", project, ""}, false, "")
	if code != ExitOK || stderr != "" || !strings.Contains(output, shortID(ids.pin)+"\t") {
		t.Fatalf("completion output: exit=%d stdout=%q stderr=%q", code, output, stderr)
	}
	for _, candidate := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if strings.ContainsAny(candidate, "\r\x1b\a\u202e") {
			t.Fatalf("untrusted data escaped the completion protocol: %q", candidate)
		}
		if strings.Count(candidate, "\t") != 1 {
			t.Fatalf("candidate has an invalid shell-protocol delimiter: %q", candidate)
		}
	}
	if !strings.Contains(output, "$(printf pwn") {
		t.Fatalf("hostile description was not visibly escaped: %q", output)
	}
	if _, err := os.Stat(ids.marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completion description executed: %v", err)
	}
}

func TestCompletionStoreOpenIsReadOnly(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".after"), 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(filepath.Join(project, ".after"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadDir(filepath.Join(project, ".after"))
	if err != nil || len(before) != len(after) {
		t.Fatalf("read-only open changed store: before=%v after=%v err=%v", before, after, err)
	}
}
