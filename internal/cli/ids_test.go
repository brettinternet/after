package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func prefixFor(id evidence.Digest) string { return strings.ToUpper(string(id)[7:19]) }

func TestIDPrefixSyntax(t *testing.T) {
	for _, value := range []string{"abcd", "AbCdE", "SHA256:ABCD", strings.Repeat("A", 64), "sha256:" + strings.Repeat("a", 64)} {
		if _, err := idPrefix(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"", "abc", "SHA256:aBc"} {
		_, err := idPrefix(value)
		if err == nil || !strings.Contains(err.Error(), "at least 4") {
			t.Errorf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"../a", "abcd\n", "abcd\x1b]52;c;bad\a", "0xabcdef", strings.Repeat("a", 65)} {
		if _, err := idPrefix(value); err == nil || strings.ContainsAny(err.Error(), "\x1b\a") {
			t.Errorf("%q: %v", value, err)
		}
	}
}

func TestIDResolutionKindsAmbiguityAndHostileDescriptions(t *testing.T) {
	project := t.TempDir()
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	report, err := gotestreport.Import(strings.NewReader("{\"Action\":\"pass\",\"Package\":\"fixture\"}\n"), gotestreport.Metadata{Producer: "hostile\n\x1b]52;c;clipboard\a\u202eproducer", ImportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(report)
	artifact, err := s.PutArtifact(raw, "report", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	prefix := string(artifact.Content)[7:11]
	// Real hash-prefix collisions, not fabricated filenames or invalid records.
	var collisions []evidence.Digest
	for i := 0; len(collisions) < 11; i++ {
		raw := []byte(fmt.Sprintf("prefix collision fixture %d", i))
		digest := sha256.Sum256(raw)
		if !strings.HasPrefix(hex.EncodeToString(digest[:]), prefix) {
			continue
		}
		a, err := s.PutArtifact(raw, "fixture", 1024)
		if err != nil {
			t.Fatal(err)
		}
		collisions = append(collisions, a.Content)
		if len(collisions) == 1 {
			_, err := resolveID(s, prefix, "report", "artifact")
			if err == nil || !strings.Contains(err.Error(), "matches 2 records") || !strings.Contains(err.Error(), "hostile") || !strings.Contains(err.Error(), "report") || !strings.Contains(err.Error(), "artifact") || strings.Count(err.Error(), "\n") != 2 || strings.ContainsAny(err.Error(), "\x1b\a\u202e") {
				t.Fatalf("cross-kind hostile ambiguity: %v", err)
			}
		}
	}
	id, err := resolveID(s, strings.ToUpper("sha256:"+prefix), "report")
	if err != nil || id != artifact.Content {
		t.Fatalf("kind-scoped unique resolution: %s %v", id, err)
	}
	for _, value := range []string{string(artifact.Content), strings.ToUpper(string(artifact.Content)), string(artifact.Content)[7:]} {
		id, err := resolveID(s, value, inspectKinds...)
		if err != nil || id != artifact.Content {
			t.Fatalf("full ID %s %v", id, err)
		}
	}
	_, err = resolveID(s, prefix, "report", "artifact")
	var diagnostic *exitError
	if !errors.As(err, &diagnostic) || diagnostic.code != ExitInvalid || !strings.Contains(diagnostic.Error(), "matches 12 records") || strings.Count(diagnostic.Error(), "\n") != 10 || strings.ContainsAny(diagnostic.Error(), "\x1b\a\u202e") {
		t.Fatalf("ambiguity: %v", err)
	}
	_, err = resolveID(s, prefix, "receipt")
	if !errors.As(err, &diagnostic) || diagnostic.code != ExitInvalid || !strings.Contains(err.Error(), "no receipt matches") || !strings.Contains(err.Error(), "after log") {
		t.Fatalf("no match: %v", err)
	}
	if id, err := resolveID(s, prefixFor(collisions[0]), "artifact"); err != nil || id != collisions[0] {
		t.Fatalf("artifact: %s %v", id, err)
	}
}

func TestPinOlderRevisionReadableAndJSON(t *testing.T) {
	project := t.TempDir()
	base, candidate := fixtureCommits(t, project)
	code, output, diagnostic := invoke([]string{"capture", "--project", project, "--base", base, "--target", candidate, "--json"}, false, "")
	if code != 0 {
		t.Fatalf("capture %d %s", code, diagnostic)
	}
	var capture struct {
		Data struct {
			Base      snapshotSummary `json:"base_snapshot"`
			Candidate snapshotSummary `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &capture); err != nil {
		t.Fatal(err)
	}
	receipt, _ := seedCLIProcessRecords(t, project, string(capture.Data.Base.ID), string(capture.Data.Candidate.ID))
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := review.Create(s, receipt, "explicit old expectation", evidence.HumanIntent, "initial")
	if err != nil {
		t.Fatal(err)
	}
	first, err := review.Attach(s, p.ID, receipt, "first fork")
	if err != nil {
		t.Fatal(err)
	}
	second, err := review.Attach(s, p.ID, receipt, "second fork")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	args := []string{"review", prefixFor(p.ID), "--project", project}
	code, output, diagnostic = invoke(args, false, "")
	if code != 0 || diagnostic != "" || !strings.Contains(output, "Pin "+shortID(p.ID)) || !strings.Contains(output, "Historical revision") || !strings.Contains(output, shortID(first.ID)) || !strings.Contains(output, shortID(second.ID)) {
		t.Fatalf("readable %d %s %s", code, output, diagnostic)
	}
	code, output, diagnostic = invoke(append(args, "--json"), false, "")
	var envelope struct {
		Data review.View `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatal(err)
	}
	if code != 0 || envelope.Data.Pin.ID != p.ID || len(envelope.Data.Pin.History) != 1 || strings.Contains(output, "newer_heads") {
		t.Fatalf("explicit JSON %d %s %s", code, output, diagnostic)
	}
	// All TUI positions use the same resolver, restricted to browser evidence.
	selection, err := resolveBrowserSelection(project, prefixFor(capture.Data.Base.ID), prefixFor(capture.Data.Candidate.ID), []string{prefixFor(receipt), prefixFor(p.ID)})
	if err != nil || selection.Pair.Base != capture.Data.Base.ID || selection.Pair.Candidate != capture.Data.Candidate.ID || len(selection.Evidence) != 2 || selection.Evidence[1] != p.ID {
		t.Fatalf("browser IDs: %+v %v", selection, err)
	}
	for _, approval := range []string{"abcd", "sha256:abcd", "", strings.ToUpper(string(receipt))} {
		code, output, diagnostic := invoke([]string{"run", "--project", project, "--plan-file", "does-not-exist", "--approve", approval}, false, "")
		if code != ExitInvalid || output != "" || !strings.Contains(diagnostic, "--approve sha256:<64 lowercase hex characters>") {
			t.Fatalf("approval %d %q %q", code, output, diagnostic)
		}
	}
	// Even without storage, too-short IDs have the specified syntax error.
	empty := t.TempDir()
	if err := os.Mkdir(filepath.Join(empty, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	code, _, diagnostic = invoke([]string{"inspect", "abc", "--project", empty}, false, "")
	if code != ExitInvalid || !strings.Contains(diagnostic, "at least 4") {
		t.Fatalf("short no-store %d %s", code, diagnostic)
	}
}
