// Package paymentfixture verifies the shipped synthetic experiment. No captured
// application is built or run on the host; execution requires the opt-in Task.
package paymentfixture

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

var paths = []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"}

type revision struct {
	name     string
	snapshot evidence.Snapshot
	files    map[string][]byte
}

func revisions(t *testing.T) []revision {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	put := func(path string, data []byte) {
		t.Helper()
		name := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	original := map[string][]byte{}
	for _, path := range paths {
		data, err := os.ReadFile("testdata/payment/" + path)
		if err != nil {
			t.Fatal(err)
		}
		original[path] = data
		put(path, data)
	}
	git("add", "--", "go.mod", "app", "driver")
	git("commit", "-qm", "synthetic base")
	base := git("rev-parse", "HEAD")
	s, err := store.Open(dir, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var out []revision
	for _, change := range []struct{ name, old, replacement string }{
		{"base", "", ""},
		{"candidate", "24 * 60 * 60", "5 * 60"},
		{"disabled-dedup", "deduplicate = true", "deduplicate = false"},
		{"printed-count", `printedCount = "1"`, `printedCount = "999"`},
	} {
		config := string(original["app/config.go"])
		if change.old != "" {
			if strings.Count(config, change.old) != 1 {
				t.Fatal("mutation must match exactly once")
			}
			config = strings.Replace(config, change.old, change.replacement, 1)
			put("app/config.go", []byte(config))
			git("add", "--", "app/config.go")
			git("commit", "-qm", "synthetic "+change.name)
		}
		pair, err := capture.Capture(t.Context(), dir, s, capture.Options{Mode: evidence.MergeBase, Base: base, Target: "HEAD"})
		if err != nil {
			t.Fatal(err)
		}
		snap := pair.Candidate
		if snap.Completeness != evidence.Complete || len(snap.Files) != len(paths) || len(snap.Excluded) != 0 || len(snap.Unsupported) != 0 {
			t.Fatalf("incomplete fixture capture: %+v", snap)
		}
		files := map[string][]byte{}
		for _, file := range snap.Files {
			data, err := s.ReadBlob(file.Content)
			if err != nil {
				t.Fatal(err)
			}
			files[file.Path] = data
			want := original[file.Path]
			if file.Path == "app/config.go" {
				want = []byte(config)
			}
			if !bytes.Equal(data, want) {
				t.Fatalf("unexpected changed source: %s", file.Path)
			}
		}
		out = append(out, revision{change.name, snap, files})
	}
	return out
}

func TestCapturedFixture(t *testing.T) {
	revs := revisions(t)
	seen := map[evidence.Digest]bool{}
	for _, rev := range revs {
		if seen[rev.snapshot.ID] {
			t.Fatal("revisions must have distinct identities")
		}
		seen[rev.snapshot.ID] = true
		// Inspection/capture and plan preparation do not contact Docker or build.
		p, err := sandbox.Prepare(string(rev.snapshot.ID), rev.files, []string{"/usr/local/go/bin/go", "run", "./driver"}, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
		if err != nil {
			t.Fatal(err)
		}
		_, digest := p.Preview()
		if digest == "" {
			t.Fatal("missing plan identity")
		}
	}
}

type action struct {
	AfterSeconds int64  `json:"after_seconds"`
	Key          string `json:"key"`
}
type httpResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
type providerCall struct {
	At     int64  `json:"at"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Body   string `json:"body"`
}
type observation struct {
	Scenario struct {
		Name  string   `json:"name"`
		Steps []action `json:"steps"`
	} `json:"scenario"`
	Responses []httpResponse `json:"responses"`
	Calls     []providerCall `json:"provider_calls"`
}

func TestPaymentProof(t *testing.T) {
	if os.Getenv("AFTER_PAYMENT_PROOF") != "1" {
		t.Skip("run task fixtures:payment to authorize only the synthetic payment plans")
	}
	docker := sandbox.Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}
	results := map[string][]observation{}
	for _, rev := range revisions(t) {
		t.Run(rev.name, func(t *testing.T) {
			p, err := sandbox.Prepare(string(rev.snapshot.ID), rev.files, []string{"/usr/local/go/bin/go", "run", "./driver"}, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
			if err != nil {
				t.Fatal(err)
			}
			preview, consent := p.Preview()
			t.Logf("Approved synthetic plan %s:\n%s", consent, preview)
			r, err := docker.Execute(context.Background(), p, consent)
			if err != nil || !r.Cleaned || r.Truncated || r.ExitCode != 0 {
				t.Fatalf("incomplete execution: %+v; %v", r, err)
			}
			var observations []observation
			if err := json.Unmarshal([]byte(r.Output), &observations); err != nil {
				t.Fatalf("invalid observations: %v: %s", err, r.Output)
			}
			verify(t, rev.name, observations)
			results[rev.name] = observations
		})
	}
	if len(results) != 4 {
		t.Fatal("all four executions are required")
	}
	// The changed effects must not be inferred from differing HTTP responses.
	for name, observations := range results {
		for i, got := range observations {
			base := results["base"][i]
			if !reflect.DeepEqual(got.Responses, base.Responses) || !reflect.DeepEqual(got.Scenario, base.Scenario) {
				t.Fatalf("%s changed frozen inputs or HTTP outputs for %s", name, got.Scenario.Name)
			}
		}
	}
	if !reflect.DeepEqual(results["base"], results["printed-count"]) {
		t.Fatal("app-printed count changed independent observations")
	}
}

func verify(t *testing.T, revision string, observations []observation) {
	t.Helper()
	cases := []struct {
		name            string
		seconds         []int64
		base, candidate []int
	}{
		{"same-key-12h", []int64{0, 43200}, []int{0}, []int{0, 1}},
		{"same-key-30s", []int64{0, 30}, []int{0}, []int{0}},
		{"expired-key-25h", []int64{0, 90000}, []int{0, 1}, []int{0, 1}},
		{"before-5m", []int64{0, 299}, []int{0}, []int{0}},
		{"at-5m", []int64{0, 300}, []int{0}, []int{0, 1}},
		{"before-24h", []int64{0, 86399}, []int{0}, []int{0, 1}},
		{"at-24h", []int64{0, 86400}, []int{0, 1}, []int{0, 1}},
		{"different-key-30s", []int64{0, 30}, []int{0, 1}, []int{0, 1}},
		{"no-sliding-expiry", []int64{0, 240, 360}, []int{0}, []int{0, 2}},
	}
	if len(observations) != len(cases) {
		t.Fatalf("expected %d cases, got %d", len(cases), len(observations))
	}
	for i, c := range cases {
		got := observations[i]
		var steps []action
		for j, seconds := range c.seconds {
			key := "synthetic-key-a"
			if c.name == "different-key-30s" && j == 1 {
				key = "synthetic-key-b"
			}
			steps = append(steps, action{seconds, key})
		}
		if got.Scenario.Name != c.name || !reflect.DeepEqual(got.Scenario.Steps, steps) || len(got.Responses) != len(steps) {
			t.Fatalf("changed/missing scenario: %+v", got)
		}
		for _, resp := range got.Responses {
			if resp.Status != 200 || resp.Body != `{"payment":"synthetic-accepted"}` {
				t.Fatalf("failed HTTP request: %+v", resp)
			}
		}
		indices := c.base
		if revision == "candidate" {
			indices = c.candidate
		}
		if revision == "disabled-dedup" {
			indices = nil
			for j := range steps {
				indices = append(indices, j)
			}
		}
		var want []providerCall
		for _, j := range indices {
			want = append(want, providerCall{1735689600 + steps[j].AfterSeconds, "POST", "/charges", steps[j].Key, `{"amount_cents":1200,"currency":"USD"}`})
		}
		if !reflect.DeepEqual(got.Calls, want) {
			t.Fatalf("%s %s: traffic %+v; want %+v", revision, c.name, got.Calls, want)
		}
		t.Logf("%s: %d actual provider requests; HTTP responses %+v; traffic %+v", c.name, len(got.Calls), got.Responses, got.Calls)
	}
}
