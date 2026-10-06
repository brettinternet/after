package browser

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/terminal"
)

var updateViews = flag.Bool("update", false, "regenerate deterministic browser golden views")

func TestMain(m *testing.M) {
	os.Setenv("NO_COLOR", "1")
	os.Exit(m.Run())
}

func TestBadgePrecedence(t *testing.T) {
	observed := Entry{State: evidence.EvidenceState{Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.Equal}, Completeness: evidence.Complete}
	// Start with a conclusive observation and layer every higher-priority rule.
	steps := []struct {
		word   string
		change func(*Entry)
	}{
		{"EQUAL", func(e *Entry) {}},
		{"DIFFERENT", func(e *Entry) { e.State.Comparison = evidence.Different }},
		{"UNSTABLE", func(e *Entry) { e.State.Comparison = evidence.Unstable }},
		{"INCOMPLETE", func(e *Entry) { e.State.Comparison = evidence.Incomparable }},
		{"INCOMPLETE", func(e *Entry) { e.State.Comparison = evidence.Equal; e.Completeness = evidence.Incomplete }},
		{"CANCELLED", func(e *Entry) { e.State.Execution = evidence.Cancelled }},
		{"FAILED", func(e *Entry) { e.State.Execution = evidence.Failed }},
		{"UNKNOWN", func(e *Entry) { e.State.Applicability = evidence.Unknown }},
		{"STALE", func(e *Entry) { e.State.Applicability = evidence.Stale }},
		{"UNAVAILABLE", func(e *Entry) { e.Unavailable = true }},
	}
	for _, s := range steps {
		s.change(&observed)
		if got := badgeFor(observed).word; got != s.word {
			t.Fatalf("%s != %s", got, s.word)
		}
	}
	for _, kind := range []evidence.Kind{evidence.Reported, evidence.NoEvidence} {
		e := Entry{State: evidence.EvidenceState{Kind: kind, Applicability: evidence.Unknown}}
		want := "NOT CHECKED"
		if kind == evidence.Reported {
			want = "REPORTED"
		}
		for _, outcome := range []evidence.ReportOutcome{evidence.ReportPass, evidence.ReportFail, evidence.ReportSkip, evidence.NoReport} {
			e.State.Report = outcome
			if badgeFor(e).word != want {
				t.Fatal(e)
			}
		}
		e.State.Applicability = evidence.Stale
		if badgeFor(e).word != "STALE" {
			t.Fatal(e)
		}
		e.Unavailable = true
		if badgeFor(e).word != "UNAVAILABLE" {
			t.Fatal(e)
		}
	}
	e := Entry{State: evidence.EvidenceState{Applicability: evidence.Current}, Decision: evidence.Pinned}
	for _, s := range []struct {
		word   string
		change func(*Entry)
	}{
		{"PINNED", func(e *Entry) {}},
		{"ACCEPTED", func(e *Entry) { e.Decision = evidence.Accepted }},
		{"UNKNOWN", func(e *Entry) { e.State.Applicability = evidence.Unknown }},
		{"STALE", func(e *Entry) { e.State.Applicability = evidence.Stale }},
		{"REOPENED", func(e *Entry) { e.Decision = evidence.Reopened }},
		{"UNAVAILABLE", func(e *Entry) { e.Unavailable = true }},
	} {
		s.change(&e)
		if badgeFor(e).word != s.word {
			t.Fatal(e, s.word)
		}
	}
	e = Entry{State: evidence.EvidenceState{Kind: evidence.Observed, Applicability: evidence.Current, Execution: evidence.Completed, Comparison: evidence.NotCompared}, Completeness: evidence.Complete}
	if badgeFor(e).word != "NOT COMPARED" {
		t.Fatal(e)
	}
	e.Name, e.Summary = "[EQUAL]", "observed"
	if badgeFor(e).word != "NOT COMPARED" {
		t.Fatal("prose changed badge")
	}
}

func TestReadableFields(t *testing.T) {
	for seconds, want := range map[int64]string{43200: "12h", 30: "30s", 90: "1m30s", 3661: "1h1m1s"} {
		if delayText(seconds) != want {
			t.Fatal(seconds)
		}
	}
	for _, tc := range []struct {
		counts []int
		want   string
	}{{[]int{1, 1}, "1"}, {[]int{2, 1}, "2,1"}} {
		if countText(tc.counts) != tc.want {
			t.Fatal(tc)
		}
	}
	m := New(t.Context(), Selection{}, Jobs{})
	defer m.Close()
	m.setClock(func() time.Time { return time.Date(2025, 1, 2, 20, 0, 0, 0, time.UTC) }, time.UTC)
	e := Entry{Decision: evidence.Accepted, DecisionAt: time.Date(2025, 1, 2, 19, 58, 0, 0, time.UTC)}
	if m.trailing(e) != "accepted 19:58:00" {
		t.Fatal(m.trailing(e))
	}
	e.DecisionAt = e.DecisionAt.Add(-24 * time.Hour)
	if m.trailing(e) != "accepted Jan 1 19:58" {
		t.Fatal(m.trailing(e))
	}
	m.theme = terminal.Theme{Color: true}
	m.width = 120
	e.Name = "\u0301\x1b[32m[EQUAL]\r\nforged"
	line := m.entryLine(e, true)
	if strings.Contains(line, "\x1b[32m") || !strings.Contains(line, "◌") || strings.ContainsAny(line, "\r\n") {
		t.Fatal(line)
	}
}
