package browser

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

func sectionByName(t *testing.T, entry Entry, name string) Section {
	t.Helper()
	for _, section := range entry.Sections {
		if section.Name == name {
			return section
		}
	}
	t.Fatalf("entry %q has no %q section", entry.Name, name)
	return Section{}
}

func TestReceiptCaseCardsKeepReceiptScenarioAndArtifactReachability(t *testing.T) {
	s, selection := setup(t, false)
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	data, err := Load(t.Context(), Selection{Project: selection.Project, Pair: selection.Pair, Evidence: []evidence.Digest{comparisonID}})
	if err != nil {
		t.Fatal(err)
	}
	entry := data.Entries[0]
	for _, name := range []string{"Card", "Receipt Card", "Witnesses", "Artifacts", "Receipt", "Scenario", "Plan", "IDs"} {
		if len(sectionByName(t, entry, name).Parts) == 0 {
			t.Fatalf("%s section has no reachable content", name)
		}
	}
	receipt, err := store.Get[evidence.Receipt](s, entry.SourceReceipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range receipt.Artifacts {
		found := false
		for _, caseEntry := range data.Entries {
			for _, part := range sectionByName(t, caseEntry, "Artifacts").Parts {
				if part.Blob == artifact.Content {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found || !containsDigest(entry.IDs, artifact.Content) {
			t.Fatalf("receipt artifact %s is not reachable from any case Card", artifact.Content)
		}
	}
	for _, name := range []string{"Receipt Card", "Witnesses", "Artifacts", "Receipt", "Scenario", "Plan"} {
		content, err := ReadSection(t.Context(), selection.Project, sectionByName(t, entry, name))
		if err != nil || len(content) == 0 {
			t.Fatalf("%s content unavailable: %q %v", name, content, err)
		}
	}
	idParts := sectionByName(t, entry, "IDs").Parts
	if len(idParts) != len(entry.IDs) {
		t.Fatalf("full ID section has %d entries, want %d", len(idParts), len(entry.IDs))
	}
	for index, id := range entry.IDs {
		if string(id) != string(idParts[index].Content) {
			t.Fatalf("full ID %d changed: %q != %q", index, idParts[index].Content, id)
		}
	}
}

func TestPinHistoryCardsRetainHistoricalReceiptArtifacts(t *testing.T) {
	selection := viewPaymentLoopSelection(t)
	data, err := Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	var pinEntry *Entry
	for index := range data.Entries {
		if data.Entries[index].Decision != "" {
			pinEntry = &data.Entries[index]
			break
		}
	}
	if pinEntry == nil {
		t.Fatal("pin Card was not loaded")
	}
	history := sectionByName(t, *pinEntry, "History")
	if len(history.Parts) < 2 {
		t.Fatalf("pin history lost event or receipt parts: %d", len(history.Parts))
	}
	content, err := ReadSection(t.Context(), selection.Project, history)
	if err != nil || !strings.Contains(string(content), "\"decision\"") || !strings.Contains(string(content), "sha256:") {
		t.Fatalf("pin history is not byte-reachable: %q %v", content, err)
	}
	ids := sectionByName(t, *pinEntry, "IDs").Parts
	if len(ids) != len(pinEntry.IDs) {
		t.Fatalf("pin history IDs incomplete: %d != %d", len(ids), len(pinEntry.IDs))
	}
	for _, id := range pinEntry.IDs {
		found := false
		for _, part := range ids {
			if string(part.Content) == string(id) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("historical pin ID %s is not reachable", id)
		}
	}
}

func TestInspectReturnsSharedReceiptPinReportAndUnavailableCards(t *testing.T) {
	s, selection := setup(t, false)
	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	comparison, err := store.Get[evidence.Comparison](s, comparisonID)
	if err != nil {
		t.Fatal(err)
	}
	pinSelection := viewPaymentLoopSelection(t)
	reportSelection := viewReportSelection(t)
	unknown := evidence.Digest("sha256:" + strings.Repeat("0", 64))
	for _, item := range []struct {
		name string
		root string
		id   evidence.Digest
	}{
		{"receipt", selection.Project, comparison.Receipt},
		{"comparison", selection.Project, comparisonID},
		{"pin", pinSelection.Project, pinSelection.Evidence[0]},
		{"report", reportSelection.Project, reportSelection.Evidence[0]},
		{"unavailable", selection.Project, unknown},
	} {
		entries, err := Inspect(item.root, item.id)
		if err != nil || len(entries) == 0 || len(entries[0].Sections) == 0 || entries[0].Sections[0].Name != "Card" {
			t.Fatalf("%s inspection did not return a shared Card: %+v %v", item.name, entries, err)
		}
	}
}

func TestOtherArtifactTitlesEscapeUntrustedChannels(t *testing.T) {
	s, selection := setup(t, false)
	hostileChannel := "provider\x1b]52;c;clipboard\a\nfixture"
	artifact, err := s.PutArtifact([]byte("synthetic artifact bytes"), hostileChannel, store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	section, ids := artifactsSection(s, []evidence.Artifact{artifact}, nil)
	if len(section.Parts) != 1 || !containsDigest(ids, artifact.Content) {
		t.Fatalf("other artifact was not retained: %+v %v", section, ids)
	}
	part := section.Parts[0]
	if strings.ContainsAny(part.Title, "\x1b\a\n") || !strings.Contains(part.Title, `\x1b`) || !strings.Contains(part.Title, `\n`) {
		t.Fatalf("hostile channel forged its trusted divider: %q", part.Title)
	}
	content, err := ReadSection(t.Context(), selection.Project, Section{Parts: []Part{part}})
	if err != nil || string(content) != "synthetic artifact bytes" {
		t.Fatalf("escaped channel hid its exact artifact bytes: %q %v", content, err)
	}
}

func TestMalformedCardsRetainFullRawContent(t *testing.T) {
	s, selection := setup(t, false)
	badReportRaw := []byte(`{"schema_version":1,"dialect":"go test JSON v1","cards":[{"future_field":{"bytes":"retained"}}]}`)
	badReport, err := s.PutArtifact(badReportRaw, "report", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	reportData, err := Load(t.Context(), Selection{Project: selection.Project, Pair: selection.Pair, Evidence: []evidence.Digest{badReport.Content}})
	if err != nil {
		t.Fatal(err)
	}
	reportEntry := reportData.Entries[0]
	reportRaw, err := ReadSection(t.Context(), selection.Project, sectionByName(t, reportEntry, "Raw content"))
	if err != nil || !bytes.Equal(reportRaw, badReportRaw) || !reportEntry.Unavailable {
		t.Fatalf("malformed report raw bytes were not fully retained: %q %v", reportRaw, err)
	}

	comparisonID := viewComparison(t, s, selection, false, evidence.Complete)
	comparison, err := store.Get[evidence.Comparison](s, comparisonID)
	if err != nil {
		t.Fatal(err)
	}
	badDetailsRaw := []byte(`{"version":1,"future_field":{"bytes":"complete detail fallback"}}`)
	badDetails, err := s.PutArtifact(badDetailsRaw, "comparison-details-v1", store.MaxBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	comparison.ID = ""
	comparison.Details = &badDetails
	comparison, err = store.Put(s, comparison)
	if err != nil {
		t.Fatal(err)
	}
	comparisonData, err := Load(t.Context(), Selection{Project: selection.Project, Pair: selection.Pair, Evidence: []evidence.Digest{comparison.ID}})
	if err != nil {
		t.Fatal(err)
	}
	comparisonEntry := comparisonData.Entries[0]
	comparisonRaw, err := ReadSection(t.Context(), selection.Project, sectionByName(t, comparisonEntry, "Raw content"))
	if err != nil {
		t.Fatal("raw comparison fallback could not be read", err)
	}
	if !bytes.Contains(comparisonRaw, badDetailsRaw) || !comparisonEntry.Unavailable || !containsDigest(comparisonEntry.IDs, badDetails.Content) {
		t.Fatalf("malformed comparison fallback incomplete: unavailable=%t id=%t raw=%t", comparisonEntry.Unavailable, containsDigest(comparisonEntry.IDs, badDetails.Content), bytes.Contains(comparisonRaw, badDetailsRaw))
	}
}
