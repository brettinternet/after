package store

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReviewSessionAtomicPrivateReplacement(t *testing.T) {
	store, project := openTest(t)
	path := filepath.Join(project, ".after", "session.json")
	first := []byte(`{"pair":"first"}`)
	second := []byte(`{"pair":"second"}`)
	if err := store.WriteReviewSession(first); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("session permissions: %v %v", info, err)
	}

	store.fault = func(stage string) error {
		if stage == "session-written" {
			return syscall.ENOSPC
		}
		return nil
	}
	if err := store.WriteReviewSession(second); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("interrupted write: %v", err)
	}
	store.fault = nil
	got, err := store.ReadReviewSession()
	if err != nil || string(got) != string(first) {
		t.Fatalf("previous session was not preserved: %q %v", got, err)
	}
	if err := store.WriteReviewSession(second); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadReviewSession()
	if err != nil || string(got) != string(second) {
		t.Fatalf("session replacement: %q %v", got, err)
	}
}

func TestReviewSessionRejectsUnsafeInputAndReplacesLink(t *testing.T) {
	store, project := openTest(t)
	path := filepath.Join(project, ".after", "session.json")
	outside := filepath.Join(project, "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadReviewSession(); err == nil {
		t.Fatal("followed a session symlink")
	}
	if err := store.WriteReviewSession([]byte(`{"pair":"safe"}`)); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(outside); err != nil || string(raw) != "outside" {
		t.Fatalf("outside target changed: %q %v", raw, err)
	}
	if raw, err := store.ReadReviewSession(); err != nil || string(raw) != `{"pair":"safe"}` {
		t.Fatalf("replacement session: %q %v", raw, err)
	}

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadReviewSession(); err == nil {
		t.Fatal("accepted a non-private session")
	}
	if err := store.WriteReviewSession([]byte(`{"pair":"private"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadReviewSession(); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxReviewSessionBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadReviewSession(); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized session: %v", err)
	}
	if err := store.WriteReviewSession([]byte(`{"pair":"small"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadReviewSession(); err == nil {
		t.Fatal("accepted a non-regular session")
	}
	if _, err := store.PutArtifact([]byte("safe"), "test", 16); err != nil {
		t.Fatalf("unsafe UI state blocked evidence storage: %v", err)
	}
	if err := store.WriteReviewSession([]byte(`{"pair":"regular"}`)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("non-regular session was not replaced: %v %v", info, err)
	}
}

func TestOversizedReviewSessionCountsTowardStoreBudget(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0644} {
		t.Run(mode.String(), func(t *testing.T) {
			store, project := openTest(t)
			path := filepath.Join(project, ".after", "session.json")
			if err := os.WriteFile(path, []byte("{}"), mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(path, MaxStoreBytes+1); err != nil {
				t.Fatal(err)
			}
			if _, err := store.PutArtifact([]byte("blocked"), "test", 16); !errors.Is(err, ErrLimit) {
				t.Fatalf("oversized session escaped evidence budget: %v", err)
			}
			if err := store.WriteReviewSession([]byte("{}")); err != nil {
				t.Fatalf("cannot replace oversized session: %v", err)
			}
			if _, err := store.PutArtifact([]byte("allowed"), "test", 16); err != nil {
				t.Fatalf("replacement did not restore evidence budget: %v", err)
			}
		})
	}
}

func TestReviewSessionRequiresBoundedDataAndWriter(t *testing.T) {
	store, _ := openTest(t)
	if err := store.WriteReviewSession(nil); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if err := store.WriteReviewSession(make([]byte, maxReviewSessionBytes+1)); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteReviewSession([]byte("{}")); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
}
