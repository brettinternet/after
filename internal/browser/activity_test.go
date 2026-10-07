package browser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func TestActivityDetailsBoundAndNoEvidenceRows(t *testing.T) {
	s, selected := setup(t, false)
	defer s.Close()
	selected.Evidence = []evidence.Digest{imported(t, s, selected.Pair)}
	data, err := Load(t.Context(), selected)
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), data.Selection, Jobs{})
	defer m.Close()
	m.data = data
	m.theme.Color = false
	m.width, m.height = 120, 40
	clock := time.Date(2026, 1, 2, 19, 59, 10, 0, time.UTC)
	m.setClock(func() time.Time { return clock }, time.UTC)
	fullError := "runner failed\n\x1b]52;c;clipboard\a"
	m.recordActivity("run failed", "incomplete result retained", []evidence.Digest{selected.Pair.Base, selected.Pair.Candidate, selected.Evidence[0]}, fullError)
	entriesBefore := len(m.data.Entries)

	press(m, "s")
	press(m, "s")
	if m.screen != "activity" || len(m.data.Entries) != entriesBefore {
		t.Fatalf("s polluted evidence or failed to open Activity: screen=%s entries=%d", m.screen, len(m.data.Entries))
	}
	if len(m.activity) != 2 {
		t.Fatalf("repeated s grew Activity, got %d events", len(m.activity))
	}
	for _, want := range []string{"SESSION", "pair       base ", "loaded", "ACTIVITY", "session opened"} {
		if !strings.Contains(m.View(), want) {
			t.Fatalf("Activity view missing %q:\n%s", want, m.View())
		}
	}
	press(m, "j") // select the run-failure event below the session event
	press(m, "enter")
	if m.doc == nil {
		t.Fatal("Enter did not open the selected Activity detail")
	}
	detail := string(m.doc.RawBytes())
	for _, want := range []string{string(selected.Pair.Base), string(selected.Pair.Candidate), string(selected.Evidence[0]), `runner failed\\u000a\\u001b]52;c;clipboard\\u0007`} {
		if !strings.Contains(detail, want) {
			t.Fatalf("full Activity detail missing %q:\n%s", want, detail)
		}
	}
	if strings.ContainsAny(m.View(), "\x1b\a\r") {
		t.Fatal("Activity detail emitted terminal controls")
	}
	m.screen = "activity"
	for i := 0; i < MaxActivityEvents; i++ {
		m.recordActivity("session opened", "repeated session reference", []evidence.Digest{selected.Pair.Candidate}, "")
	}
	if len(m.activity) != MaxActivityEvents || m.activityDropped != 2 || !strings.Contains(m.View(), "2 older activity events dropped") {
		t.Fatalf("Activity bound/drop count missing: events=%d dropped=%d\n%s", len(m.activity), m.activityDropped, m.View())
	}
	if len(m.data.Entries) != entriesBefore {
		t.Fatal("Activity events became evidence rows")
	}
}

func TestActivityClockTicksOnlyWhileWorkIsActive(t *testing.T) {
	clock := time.Date(2026, 1, 2, 19, 59, 10, 0, time.UTC)
	m := New(context.Background(), Selection{}, Jobs{})
	defer m.Close()
	m.setClock(func() time.Time { return clock }, time.UTC)
	if m.scheduleClock() != nil || m.clockScheduled {
		t.Fatal("idle review scheduled a clock tick")
	}
	for _, active := range []struct {
		name string
		set  func()
		want string
	}{
		{"capture", func() { m.capturing, m.captureStarted = true, clock.Add(-65*time.Second) }, "capturing 1:05"},
		{"import", func() { m.busy, m.busyKind, m.busyStarted = true, "import", clock.Add(-65*time.Second) }, "import 1:05"},
		{"run", func() { m.running, m.runStarted = true, clock.Add(-65*time.Second) }, "running 1:05"},
	} {
		t.Run(active.name, func(t *testing.T) {
			m.capturing, m.busy, m.running = false, false, false
			m.busyKind = ""
			active.set()
			if m.scheduleClock() == nil || !m.clockScheduled {
				t.Fatal("active operation did not schedule its clock")
			}
			frame := func() string {
				header, extra := m.frameHeader()
				return header + "\n" + strings.Join(extra, "\n")
			}
			if !strings.Contains(frame(), active.want) {
				t.Fatalf("elapsed time not shown for %s: header=%q status=%q", active.name, frame(), m.statusLine())
			}
			clock = clock.Add(time.Second)
			if !strings.Contains(frame(), elapsed(time.Second*66)) {
				t.Fatalf("elapsed time did not update for %s: %q", active.name, frame())
			}
			if strings.Contains(m.View(), "%") {
				t.Fatal("elapsed frame invented a percentage")
			}
			m.capturing, m.busy, m.running = false, false, false
			cmd, handled := m.updateLoop(runClockTick{})
			if !handled || cmd != nil || m.clockScheduled {
				t.Fatal("clock continued after work became idle")
			}
		})
	}
}

func TestImportAtEvidenceLimitRetainsActivityID(t *testing.T) {
	s, selected := setup(t, false)
	defer s.Close()
	report := imported(t, s, selected.Pair)
	selected.Evidence = make([]evidence.Digest, MaxEvidence)
	for i := range selected.Evidence {
		selected.Evidence[i] = evidence.Digest("sha256:" + strings.Repeat("a", 63) + string(rune('0'+i%10)))
	}
	m := New(context.Background(), selected, Jobs{})
	defer m.Close()
	m.data = &Data{Selection: selected}
	m.busy, m.busyKind, m.jobID = true, "import", 1
	m.Update(finished{request: 1, kind: "import", selection: selected, result: string(report)})
	if len(m.selected.Evidence) != MaxEvidence || len(m.data.Entries) != 0 || !strings.Contains(m.status, "32 evidence IDs") {
		t.Fatalf("32-ID import changed loaded evidence: status=%q", m.status)
	}
	if len(m.activity) != 1 || !containsDigest(m.activity[0].IDs, report) {
		t.Fatalf("stored report ID missing from Activity: %+v", m.activity)
	}
	storeHandle, err := store.Open(selected.Project, false, nil)
	if err != nil {
		t.Fatal("imported report was not left in the store", err)
	}
	if _, err := storeHandle.ReadBlob(report); err != nil {
		t.Fatal("stored report unavailable", err)
	}
	storeHandle.Close()
}
