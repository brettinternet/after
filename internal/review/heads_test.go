package review

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func TestHeadsRecomputedForksAndExactHistory(t *testing.T) {
	s, _, receipt := fixture(t)
	root := pin(t, s, receipt, evidence.FiniteExample)
	assertHeads := func(want ...evidence.Digest) {
		t.Helper()
		heads, err := Heads(s)
		if err != nil {
			t.Fatal(err)
		}
		got := []evidence.Digest{}
		for _, p := range heads {
			got = append(got, p.ID)
		}
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("heads %v want %v", got, want)
		}
	}
	assertHeads(root.ID)
	first, err := Accept(s, root.ID, "first fork")
	if err != nil {
		t.Fatal(err)
	}
	assertHeads(first.ID)
	second, err := Accept(s, root.ID, "second fork")
	if err != nil {
		t.Fatal(err)
	}
	assertHeads(first.ID, second.ID)
	newest, err := Attach(s, first.ID, receipt.ID, "extend first fork")
	if err != nil {
		t.Fatal(err)
	}
	assertHeads(newest.ID, second.ID)
	heads, err := NewerHeads(s, first)
	if err != nil || !reflect.DeepEqual(heads, []evidence.Digest{newest.ID}) {
		t.Fatalf("descendants %v %v", heads, err)
	}
	heads, err = NewerHeads(s, root)
	if err != nil || len(heads) != 2 {
		t.Fatalf("root descendants %v %v", heads, err)
	}
	if v := view(t, s, root); v.Pin.ID != root.ID || len(v.Pin.History) != 1 || v.Pin.Decision != evidence.Pinned {
		t.Fatalf("explicit revision changed: %+v", v)
	}
	// Same events but a changed expectation or scope are not an extension.
	changed := newest
	changed.ID = ""
	changed.Expectation = "a different requirement"
	changed = put(t, s, changed)
	assertHeads(newest.ID, second.ID, changed.ID)
	heads, err = NewerHeads(s, root)
	if err != nil || len(heads) != 2 {
		t.Fatalf("unrelated basis counted as descendant: %v %v", heads, err)
	}
}

func TestHeadsEmptyCorruptAndBounded(t *testing.T) {
	s, dir, receipt := fixture(t)
	heads, err := Heads(s)
	if err != nil || len(heads) != 0 {
		t.Fatalf("empty %v %v", heads, err)
	}
	root := pin(t, s, receipt, evidence.FiniteExample)
	path := filepath.Join(dir, ".after", "pin-"+strings.TrimPrefix(string(root.ID), "sha256:"))
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Heads(s); err == nil {
		t.Fatal("corrupt pin ignored")
	}
	// Listing must fail closed before any record is decoded when over budget.
	for i := 0; i < maxPinScan; i++ {
		name := fmt.Sprintf("%064x", i)
		if err := os.WriteFile(filepath.Join(dir, ".after", "pin-"+name), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Heads(s); !errors.Is(err, store.ErrLimit) {
		t.Fatalf("scan limit: %v", err)
	}
}
