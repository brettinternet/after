package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
)

func openTest(t *testing.T, secrets ...string) (*Store, string) {
	t.Helper()
	project := t.TempDir()
	s, err := Open(project, true, secrets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, project
}
func mustPut[T Record](t *testing.T, s *Store, r T) T {
	t.Helper()
	result, err := Put(s, r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func artifactTest(t *testing.T, s *Store, data string) evidence.Artifact {
	t.Helper()
	a, err := s.PutArtifact([]byte(data), "stdout", 1024)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func fixtures(t *testing.T, s *Store) (evidence.Snapshot, evidence.Scenario, evidence.Receipt, evidence.Comparison, evidence.Pin) {
	t.Helper()
	a := artifactTest(t, s, "synthetic body")
	snapshot := mustPut(t, s, evidence.Snapshot{SchemaVersion: 1, Source: evidence.WorkingTree, Commit: strings.Repeat("a", 40), Files: []evidence.File{{Path: "unicode-世界.go", Content: a.Content, Mode: "100644"}}, Completeness: evidence.Complete, Diff: a.Content})
	scenario := mustPut(t, s, evidence.Scenario{SchemaVersion: 1, Input: a.Content, Driver: a.Content, Observer: a.Content, Rules: a.Content, Boundary: "synthetic", Author: "test", Limits: []string{"synthetic only"}})
	env := &evidence.Environment{Environment: a.Content, Toolchain: a.Content, Dependencies: a.Content, Argv: []string{"synthetic"}}
	receipt := mustPut(t, s, evidence.Receipt{SchemaVersion: 1, State: evidence.EvidenceState{Producer: evidence.Runner, Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.NotCompared, Report: evidence.NoReport}, Snapshots: evidence.SnapshotPair{Base: snapshot.ID, Candidate: snapshot.ID}, Bindings: &evidence.Bindings{Scenario: scenario.ID, Input: a.Content, Driver: a.Content, Observer: a.Content, Rules: a.Content}, BaseEnvironment: env, CandidateEnvironment: env, Authorization: a.Content, StartedAt: time.Unix(100, 0).UTC(), FinishedAt: time.Unix(101, 0).UTC(), Completeness: evidence.Complete, Artifacts: []evidence.Artifact{a}, Limits: []string{"synthetic only"}})
	comparison := mustPut(t, s, evidence.Comparison{SchemaVersion: 1, Receipt: receipt.ID, Outcome: evidence.Equal, Completeness: evidence.Complete, Limits: []string{"synthetic only"}})
	pin := mustPut(t, s, evidence.Pin{SchemaVersion: 1, Scenario: scenario.ID, Expectation: "specific synthetic expectation", BasisReceipt: receipt.ID, BasisSnapshots: receipt.Snapshots, Decision: evidence.Pinned, History: []evidence.DecisionEvent{{Decision: evidence.Pinned, At: time.Unix(102, 0).UTC(), Reason: "synthetic"}}})
	return snapshot, scenario, receipt, comparison, pin
}
func roundTrip[T Record](t *testing.T, s *Store, r T) {
	t.Helper()
	_, id := identity(&r)
	actual, err := Get[T](s, *id)
	if err != nil || !reflect.DeepEqual(actual, r) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestWritableOpenPublishesGitignoreAndReadOnlyOpenDoesNot(t *testing.T) {
	project := t.TempDir()
	writer, err := Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(project, ".after", ".gitignore")
	assertIgnore := func() {
		t.Helper()
		data, err := os.ReadFile(ignore)
		if err != nil || string(data) != "*\n" {
			t.Fatalf("store ignore file: %q %v", data, err)
		}
		info, err := os.Stat(ignore)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("store ignore permissions: %v %v", info, err)
		}
	}
	assertIgnore()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ignore); err != nil {
		t.Fatal(err)
	}
	reader, err := Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ignore); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only open created the ignore file: %v", err)
	}
	writer, err = Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	assertIgnore()
}

func TestRoundTripImmutableRecords(t *testing.T) {
	s, project := openTest(t)
	snapshot, scenario, receipt, comparison, pin := fixtures(t, s)
	s.Close()
	read, err := Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	roundTrip(t, read, snapshot)
	roundTrip(t, read, scenario)
	roundTrip(t, read, receipt)
	roundTrip(t, read, comparison)
	roundTrip(t, read, pin)
	if _, err := Put(read, scenario); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
	writer, err := Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if same := mustPut(t, writer, scenario); same.ID != scenario.ID {
		t.Fatal("identity changed")
	}
	scenario.Author = "changed"
	if _, err := Put(writer, scenario); err == nil {
		t.Fatal("overwrote immutable identity")
	}
	scenario.ID = ""
	changed := mustPut(t, writer, scenario)
	if changed.ID == snapshot.ID {
		t.Fatal("changed identity collision")
	}
	roundTrip(t, read, receipt)
	entries, err := os.ReadDir(filepath.Join(project, ".after"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, _ := entry.Info()
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
	}
}

func TestCrashBoundariesAndIOErrors(t *testing.T) {
	for _, stage := range []string{"created", "written", "synced", "published", "directory-synced"} {
		t.Run(stage, func(t *testing.T) {
			s, project := openTest(t)
			_, scenario, _, _, _ := fixtures(t, s)
			changed := scenario
			changed.ID = ""
			changed.Author = "new author"
			s.fault = func(actual string) error {
				if actual == stage {
					return syscall.ENOSPC
				}
				return nil
			}
			if _, err := Put(s, changed); !errors.Is(err, syscall.ENOSPC) {
				t.Fatalf("disk-full error lost: %v", err)
			}
			s.Close()
			reopened, err := Open(project, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			roundTrip(t, reopened, scenario)
		})
	}
	s, _ := openTest(t)
	s.fault = func(string) error { return syscall.EACCES }
	if _, err := s.PutArtifact([]byte("body"), "out", 4); !errors.Is(err, syscall.EACCES) {
		t.Fatal(err)
	}
}

func TestCorruptRecordsAndMissingArtifacts(t *testing.T) {
	for _, mutation := range []string{"truncated", "corrupt", "version", "missing"} {
		t.Run(mutation, func(t *testing.T) {
			s, project := openTest(t)
			_, scenario, receipt, _, _ := fixtures(t, s)
			name, _ := key("scenario", string(scenario.ID))
			path := filepath.Join(project, ".after", name)
			switch mutation {
			case "truncated":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				scenario.Author = "tampered"
				data, _ := json.Marshal(scenario)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "version":
				scenario.SchemaVersion = 99
				data, _ := json.Marshal(scenario)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				blob, _ := key("blob", string(scenario.Input))
				if err := os.Rename(filepath.Join(project, ".after", blob), filepath.Join(project, "saved-blob")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Get[evidence.Scenario](s, scenario.ID); err == nil {
				t.Fatal("accepted damaged history")
			}
			if _, err := Get[evidence.Receipt](s, receipt.ID); err == nil {
				t.Fatal("accepted missing reference")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("history erased", err)
			}
		})
	}
}

func TestConfinement(t *testing.T) {
	s, project := openTest(t)
	for _, id := range []evidence.Digest{"../escape", "/tmp/escape", evidence.Digest("sha256:" + strings.Repeat("../", 22)), evidence.Digest("sha256:" + strings.Repeat("A", 64))} {
		if _, err := s.ReadBlob(id); err == nil {
			t.Fatal("accepted path", id)
		}
	}
	outside := filepath.Join(project, "outside")
	if err := os.WriteFile(outside, []byte("do not touch"), 0600); err != nil {
		t.Fatal(err)
	}
	id := evidence.Digest(hash([]byte("do not touch")))
	name, _ := key("blob", string(id))
	if err := os.Symlink(outside, filepath.Join(project, ".after", name)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadBlob(id); err == nil {
		t.Fatal("followed symlink")
	}
	if _, err := s.PutArtifact([]byte("do not touch"), "out", 100); err == nil {
		t.Fatal("wrote symlink")
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "do not touch" {
		t.Fatal("outside changed")
	}
	other := t.TempDir()
	if err := os.Symlink(filepath.Join(project, ".after"), filepath.Join(other, ".after")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(other, true, nil); err == nil {
		t.Fatal("followed store symlink")
	}
}

func TestUnsafeFilesAndLimits(t *testing.T) {
	for _, kind := range []string{"public", "hardlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			s, project := openTest(t)
			body := []byte("body")
			id := evidence.Digest(hash(body))
			name, _ := key("blob", string(id))
			path := filepath.Join(project, ".after", name)
			switch kind {
			case "public":
				if err := os.WriteFile(path, body, 0644); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				outside := filepath.Join(project, "outside")
				if err := os.WriteFile(outside, body, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(outside, path); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.ReadBlob(id); err == nil {
				t.Fatal("accepted unsafe file")
			}
		})
	}
	s, project := openTest(t)
	if _, err := s.PutArtifact(make([]byte, MaxBlobBytes+1), "out", 1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err := s.PutArtifact(nil, "out", MaxBlobBytes+1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(project, ".after", "budget"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxStoreBytes); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := s.PutArtifact([]byte("x"), "out", 1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	count, dir := openTest(t)
	for i := 0; i < MaxEntries-1; i++ {
		if err := os.WriteFile(filepath.Join(dir, ".after", fmt.Sprint(i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := count.PutArtifact([]byte("x"), "out", 1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestRedactionAndFalseCompleteness(t *testing.T) {
	s, project := openTest(t, "sensitive-value", "line\nsecret", "abc", "bcd")
	snapshot, scenario, receipt, _, _ := fixtures(t, s)
	a, err := s.PutArtifact([]byte("abcd sensitive-value line\nsecret end"), "sensitive-value", 12)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Redacted || !a.Truncated || a.Completeness != evidence.Incomplete || a.RedactionPolicy != redactionPolicy {
		t.Fatal(a)
	}
	data, err := s.ReadBlob(a.Content)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "abcd") || len(data) != 12 {
		t.Fatal("redaction/truncation failed")
	}
	receipt.ID = ""
	receipt.Artifacts = []evidence.Artifact{a}
	receipt.Completeness = evidence.Incomplete
	receipt.BaseEnvironment.Argv = []string{"sensitive-value", "line\nsecret"}
	safe := mustPut(t, s, receipt)
	if !safe.Redacted || safe.RedactionPolicy != redactionPolicy || safe.BaseEnvironment.Argv[0] != "[REDACTED]" {
		t.Fatal("metadata not redacted")
	}
	if receipt.BaseEnvironment.Argv[0] != "sensitive-value" {
		t.Fatal("mutated caller")
	}
	roundTrip(t, s, safe)
	forged := safe
	forged.ID = ""
	forged.Completeness = evidence.Complete
	if _, err := Put(s, forged); err == nil {
		t.Fatal("complete redacted receipt")
	}
	comparison := evidence.Comparison{SchemaVersion: 1, Receipt: safe.ID, Outcome: evidence.Equal, Completeness: evidence.Complete, Limits: []string{"scope"}}
	if _, err := Put(s, comparison); err == nil {
		t.Fatal("equal partial artifacts")
	}
	forged = safe
	forged.ID = ""
	forged.Artifacts = append([]evidence.Artifact(nil), safe.Artifacts...)
	forged.Artifacts[0].Redacted = false
	forged.Artifacts[0].Truncated = false
	forged.Artifacts[0].Completeness = evidence.Complete
	if _, err := Put(s, forged); err == nil {
		t.Fatal("forged artifact metadata")
	}
	entries, _ := os.ReadDir(filepath.Join(project, ".after"))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(project, ".after", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"sensitive-value", "line\\nsecret", "abcd"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatal("secret persisted")
			}
		}
	}
	roundTrip(t, s, snapshot)
	roundTrip(t, s, scenario)
}

func TestWriterProcessHelper(t *testing.T) {
	if os.Getenv("AFTER_LOCK_HELPER") != "1" {
		return
	}
	s, err := Open(os.Getenv("AFTER_TEST_PROJECT"), true, nil)
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(2)
	}
	defer s.Close()
	fmt.Println("LOCKED")
	var b [1]byte
	os.Stdin.Read(b[:])
	os.Exit(0)
}

func TestWriterLockAndInterruptedRecovery(t *testing.T) {
	project := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWriterProcessHelper$")
	cmd.Env = append(os.Environ(), "AFTER_LOCK_HELPER=1", "AFTER_TEST_PROJECT="+project)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ready := make([]byte, 7)
	if _, err := stdout.Read(ready); err != nil || string(ready) != "LOCKED\n" {
		t.Fatalf("helper: %q %v", ready, err)
	}
	if s, err := Open(project, true, nil); !errors.Is(err, ErrLocked) {
		if s != nil {
			s.Close()
		}
		t.Fatalf("second writer: %v", err)
	}
	reader, err := Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	unrelated := filepath.Join(project, ".after", "pending-unrelated")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	recovered, err := Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	artifactTest(t, recovered, "after interrupted owner")
	if data, err := os.ReadFile(unrelated); err != nil || string(data) != "keep" {
		t.Fatal("recovery deleted unrelated file")
	}
	if s, err := Open(project, true, nil); !errors.Is(err, ErrLocked) {
		if s != nil {
			s.Close()
		}
		t.Fatal("stole live lock", err)
	}
}
