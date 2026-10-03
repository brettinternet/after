package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPlan(t *testing.T) {
	snapshot := digest([]byte("snapshot"))
	files := map[string][]byte{"main.go": []byte("package main")}
	argv := []string{"/usr/local/go/bin/go", "run", "/input/main.go"}
	limits := Limits{60, 4096}
	p, e := Prepare(snapshot, files, argv, limits)
	if e != nil {
		t.Fatal(e)
	}
	preview, id := p.Preview()
	files["main.go"][0] = 'x'
	argv[0] = "/bad"
	if _, after := p.Preview(); after != id {
		t.Fatal("mutable plan")
	}
	if !strings.Contains(string(preview), "--network=none") || !strings.Contains(string(preview), Image) {
		t.Fatal("missing preview policy")
	}
	for _, mutate := range []func(*specification){func(s *specification) { s.Snapshot = digest([]byte("other")) }, func(s *specification) { s.Argv = []string{"/bad"} }, func(s *specification) { s.Limits.Seconds++ }, func(s *specification) { s.Image = "other" }, func(s *specification) { s.Input = digest(nil) }, func(s *specification) { s.Policy = []string{"--network=host"} }} {
		changed := *p
		mutate(&changed.spec)
		if _, newID := changed.Preview(); newID == id {
			t.Fatal("change retained consent")
		}
		if _, err := (Docker{}).Execute(context.Background(), &changed, id); !errors.Is(err, ErrConsent) {
			t.Fatal(err)
		}
	}
	if _, err := (Docker{}).Execute(context.Background(), p, ""); !errors.Is(err, ErrConsent) {
		t.Fatal(err)
	}
	if _, err := (Docker{}).Execute(context.Background(), nil, id); !errors.Is(err, ErrConsent) {
		t.Fatal(err)
	}
}
func TestInputPaths(t *testing.T) {
	for _, name := range []string{"..", "../escape", "/absolute", "a/../../b", "a\\b", ".", "a\x00b", "a\nb"} {
		if _, err := Prepare(digest(nil), map[string][]byte{name: []byte("x")}, []string{"/bin/true"}, Limits{1, 1}); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}
func TestInputOrder(t *testing.T) {
	a, _ := Prepare(digest(nil), map[string][]byte{"a": []byte("a"), "b": []byte("b")}, []string{"/bin/true"}, Limits{1, 1})
	b, _ := Prepare(digest(nil), map[string][]byte{"b": []byte("b"), "a": []byte("a")}, []string{"/bin/true"}, Limits{1, 1})
	_, x := a.Preview()
	_, y := b.Preview()
	if x != y {
		t.Fatal("nondeterministic plan")
	}
}
func TestOutputBound(t *testing.T) {
	cancelled := false
	b := &output{max: 3, cancel: func() { cancelled = true }}
	b.Write([]byte("abcdef"))
	b.Write([]byte("more"))
	if b.b.String() != "abc" || !b.truncated || !cancelled {
		t.Fatal("unbounded output")
	}
}
