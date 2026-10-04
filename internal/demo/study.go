package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
)

// The study reuses the demo's owned synthetic workspace and exact-plan consent.
// Keys are checked against protected observer artifacts, never candidate tests.
type studyCase struct {
	ID        string `json:"id"`
	Retention string `json:"retention"`
	Followup  string `json:"followup"`
}

var studyCases = []studyCase{
	{"A", "5 * 60", "repair"},
	{"B", "60 * 60", "comment"},
	{"C", "6 * 60 * 60", "oracle"},
}

type studyPhase struct {
	Capture capture         `json:"capture"`
	Pin     evidence.Digest `json:"pin,omitempty"`
	Receipt evidence.Digest `json:"receipt,omitempty"`
}
type studyKey struct {
	Version          int             `json:"version"`
	Case             studyCase       `json:"case"`
	Initial          studyPhase      `json:"initial"`
	Followup         studyPhase      `json:"followup"`
	VerifiedFollowup evidence.Digest `json:"verified_followup_receipt"`
	InitialCounts    [2]int          `json:"initial_12h_counts"`
	FollowupCounts   [2]int          `json:"followup_12h_counts"`
	ControlCounts    [2]int          `json:"control_30s_counts"`
}

func writeJSON(root, name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return put(root, name, append(raw, '\n'))
}
func retentionTest(expression string) []byte {
	return []byte("package main\nimport \"testing\"\nfunc TestRetentionPolicy(t *testing.T) { if retentionSeconds != " + expression + " { t.Fatal(\"retention policy\") } }\n")
}

// Copy only this harness's synthetic project; CopyFS rejects symlinks. Restore
// private modes because CopyFS defaults would make the evidence store unreadable.
func copyStudyProject(destination, source string) error {
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		return err
	}
	return filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := fs.FileMode(0600)
		if entry.IsDir() {
			mode = 0700
		}
		return os.Chmod(path, mode)
	})
}

func (d *demo) studySetup(c studyCase) error {
	if err := d.setup(); err != nil {
		return err
	}
	// Add a tracked baseline oracle before making the candidate edit.
	if err := put(d.project, "app/config_test.go", retentionTest("24 * 60 * 60")); err != nil {
		return err
	}
	for _, args := range [][]string{{"add", "app/config_test.go"}, {"commit", "-qm", "synthetic baseline test"}} {
		if _, code, err := d.command("/usr/bin/git", args...); err != nil || code != 0 {
			return fmt.Errorf("study baseline: %d %w", code, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(d.project, "app/config.go"))
	if err != nil {
		return err
	}
	raw = []byte(strings.Replace(string(raw), "5 * 60", c.Retention, 1))
	if err = put(d.project, "app/config.go", raw); err != nil {
		return err
	}
	return put(d.project, "app/config_test.go", retentionTest(c.Retention))
}
func (d *demo) studyEdit(c studyCase) error {
	switch c.Followup {
	case "repair":
		if err := d.edit(true); err != nil {
			return err
		}
		return put(d.project, "app/config_test.go", retentionTest("24 * 60 * 60"))
	case "comment":
		raw, err := os.ReadFile(filepath.Join(d.project, "app/config.go"))
		if err != nil {
			return err
		}
		return put(d.project, "app/config.go", append(raw, []byte("\n// Retention is expressed in seconds.\n")...))
	case "oracle":
		return put(d.project, "app/config_test.go", []byte("package main\nimport \"testing\"\nfunc TestRetentionPolicy(t *testing.T) { if retentionSeconds <= 0 { t.Fatal(\"positive retention\") } }\n"))
	default:
		return fmt.Errorf("unknown follow-up %q", c.Followup)
	}
}
func (d *demo) studyDiff(name string) error {
	raw, code, err := d.command("/usr/bin/git", "diff", "--no-ext-diff", "--no-textconv", "HEAD", "--", "app")
	if err != nil || code != 0 {
		return fmt.Errorf("study diff: %d %w", code, err)
	}
	return put(d.root, name, raw)
}
func (d *demo) study(execute bool) error {
	fmt.Println("Study kit v1: AUTHOR REHEARSAL ONLY; no participants or usability results.")
	for _, c := range studyCases {
		child := *d
		child.root = filepath.Join(d.root, c.ID)
		child.project = filepath.Join(child.root, "payment")
		if err := child.studyCase(c, execute); err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}
	}
	return nil
}
func (d *demo) studyCase(c studyCase, execute bool) error {
	if err := d.studySetup(c); err != nil {
		return err
	}
	key := studyKey{Version: 1, Case: c, InitialCounts: [2]int{1, 2}, FollowupCounts: [2]int{1, 2}, ControlCounts: [2]int{1, 1}}
	if c.Followup == "repair" {
		key.FollowupCounts = [2]int{1, 1}
	}
	if err := d.cli(0, &key.Initial.Capture, "capture"); err != nil {
		return err
	}
	if err := d.studyDiff("initial.diff"); err != nil {
		return err
	}
	// This phase is genuinely missing evidence: capture is not execution.
	if err := writeJSON(d.root, "missing.json", key.Initial); err != nil {
		return err
	}
	if err := copyStudyProject(filepath.Join(d.root, "missing-project"), d.project); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	first, err := d.execute(key.Initial.Capture, "initial", true)
	if err != nil {
		return err
	}
	key.Initial.Receipt = first.Receipt.ID
	var pinned review.View
	if err = d.cli(0, &pinned, "pin", string(first.Receipt.ID), "--scope", "finite_example", "--expectation", "Twelve-hour retries should make one provider request", "--reason", "study setup; not participant judgment"); err != nil {
		return err
	}
	key.Initial.Pin = pinned.Pin.ID
	if err = writeJSON(d.root, "initial.json", key.Initial); err != nil {
		return err
	}
	if err = copyStudyProject(filepath.Join(d.root, "initial-project"), d.project); err != nil {
		return err
	}
	if err = d.studyEdit(c); err != nil {
		return err
	}
	if err = d.cli(0, &key.Followup.Capture, "capture"); err != nil {
		return err
	}
	if err = d.studyDiff("followup.diff"); err != nil {
		return err
	}
	var reopened review.View
	if err = d.cli(0, &reopened, "review", string(pinned.Pin.ID), "--select", string(key.Followup.Capture.Candidate.ID), "--mode", "original_base", "--reason", "controlled study follow-up"); err != nil {
		return err
	}
	if !reopened.MissingCurrentResult || reopened.CurrentReceipt != nil || reopened.Applicability != evidence.Stale {
		return fmt.Errorf("follow-up did not reopen stale/missing")
	}
	key.Followup.Pin = reopened.Pin.ID
	key.Followup.Capture.Base.ID = reopened.Pin.History[len(reopened.Pin.History)-1].Review.Target.Snapshots.Base
	if err = writeJSON(d.root, "followup.json", key.Followup); err != nil {
		return err
	}
	// Run the key in a separate synthetic store so browsing cannot leak the result.
	keyProject := filepath.Join(d.root, "facilitator-project")
	if err = copyStudyProject(keyProject, d.project); err != nil {
		return err
	}
	verifier := *d
	verifier.project = keyProject
	second, err := verifier.execute(key.Followup.Capture, "key", c.Followup != "repair")
	if err != nil {
		return err
	}
	key.VerifiedFollowup = second.Receipt.ID
	return writeJSON(d.root, "facilitator-key.json", key)
}
