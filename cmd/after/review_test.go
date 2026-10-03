package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
)

// Uses separate native processes for every mutation/inspection, including the
// real runner receipt. No mutable in-memory selection survives between steps.
func paymentReviewProof(t *testing.T, exe, root, home, project, configPath string, r evidence.Receipt, dockerEnv []string) {
	t.Helper()
	command := func(args ...string) review.View {
		t.Helper()
		args = append(args, "--project", project, "--config", configPath)
		code, out, stderr := native(exe, root, home, args, nil)
		if code != 0 || stderr != "" {
			t.Fatalf("review exit=%d stderr=%q stdout=%q", code, stderr, out)
		}
		var envelope struct {
			Data review.View `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}
	p := command("pin", string(r.ID), "--expectation", "twelve-hour retries must not duplicate provider calls", "--scope", "finite_example", "--reason", "explicit finite expectation")
	if p.Applicability != evidence.Current || p.Pin.Decision != evidence.Pinned || p.CurrentReceipt.ID != r.ID {
		t.Fatalf("pin %+v", p)
	}
	accepted := command("review", string(p.Pin.ID), "--accept", "--reason", "operator reviewed this finite example")
	selected := command("review", string(accepted.Pin.ID), "--select", string(r.Snapshots.Base), "--mode", "original_base", "--reason", "inspect captured original implementation again")
	if selected.Pin.Decision != evidence.Reopened || selected.Applicability != evidence.Stale || !selected.MissingCurrentResult || selected.CurrentReceipt != nil || selected.Pin.Expectation != p.Pin.Expectation {
		t.Fatalf("selection predicted values %+v", selected)
	}
	restarted := command("review", string(selected.Pin.ID))
	if restarted.Pin.ID != selected.Pin.ID || len(restarted.Pin.History) != 3 || !restarted.MissingCurrentResult {
		t.Fatal("restart lost history")
	}
	for _, args := range [][]string{
		{"review", string(selected.Pin.ID), "--receipt", string(r.ID), "--reason", "late prior result"},
		{"review", string(selected.Pin.ID), "--accept", "--reason", "cannot accept missing result"},
	} {
		code, _, _ := native(exe, root, home, append(args, "--project", project, "--config", configPath), nil)
		if code != 2 {
			t.Fatalf("unsafe review action exit=%d", code)
		}
	}
	planPath := filepath.Join(root, "review-rerun-preview.json")
	code, out, stderr := native(exe, root, home, []string{"run", string(r.Snapshots.Base), string(r.Snapshots.Base), "--project", project, "--config", configPath, "--plan-out", planPath}, nil)
	if code != 3 || stderr != "" {
		t.Fatalf("rerun preview %d %s %s", code, stderr, out)
	}
	var preview struct {
		Data struct {
			Authorization string          `json:"authorization_digest"`
			Plan          json.RawMessage `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err)
	}
	t.Logf("Authorizing real synthetic pin rerun %s: %s", preview.Data.Authorization, preview.Data.Plan)
	code, out, stderr = native(exe, root, home, []string{"run", "--plan-file", planPath, "--approve", preview.Data.Authorization, "--project", project, "--config", configPath}, dockerEnv)
	if code != 0 || stderr != "" {
		t.Fatalf("real rerun %d %s %s", code, stderr, out)
	}
	var result struct {
		Data struct {
			Receipt evidence.Receipt `json:"receipt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Receipt.ID == r.ID || result.Data.Receipt.State.Kind != evidence.Observed {
		t.Fatal("no new observed receipt")
	}
	attached := command("review", string(selected.Pin.ID), "--receipt", string(result.Data.Receipt.ID), "--reason", "attach authorized rerun")
	if attached.Pin.Decision != evidence.Reopened || attached.Applicability != evidence.Current || attached.MissingCurrentResult || attached.Pin.BasisReceipt != r.ID {
		t.Fatalf("rerun silently accepted %+v", attached)
	}
	final := command("review", string(attached.Pin.ID), "--accept", "--reason", "operator explicitly re-reviewed")
	if final.Pin.Decision != evidence.Accepted || len(final.Pin.History) != 5 {
		t.Fatal("acceptance/history missing")
	}
	original := command("review", string(p.Pin.ID))
	if original.CurrentReceipt.ID != r.ID || original.Pin.Expectation != final.Pin.Expectation {
		t.Fatal("rewrote original basis")
	}
	follow := command("review", string(final.Pin.ID), "--select", string(r.Snapshots.Candidate), "--mode", "last_inspected", "--reason", "explicit follow-up")
	c := follow.Pin.History[len(follow.Pin.History)-1].Review
	if c.Mode != evidence.FollowUp || c.Target.Snapshots.Base != r.Snapshots.Base || follow.CurrentReceipt != nil || !strings.Contains(follow.Reason, "no attached current result") {
		t.Fatalf("follow-up %+v", follow)
	}
	t.Log("Real pin/reopen/authorized-rerun/attach/separate-accept flow passed across native process restarts; original observations preserved.")
}
