package browser

import (
	"sort"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

type discoveredPin struct {
	id           evidence.Digest
	needsAnother bool
}

type discoveredRun struct {
	id       evidence.Digest
	finished time.Time
}

type discoveredComparison struct {
	id       evidence.Digest
	finished time.Time
}

type discoveredReport struct {
	id       evidence.Digest
	imported time.Time
}

// discoverEvidence finds matching immutable records, orders pins needing review
// first and then newest pair/candidate evidence, and reports the exact number
// omitted by the TUI's 32-record bound.
func discoverEvidence(s *store.Store, pair evidence.SnapshotPair) ([]evidence.Digest, int, error) {
	pins, err := review.Heads(s)
	if err != nil {
		return nil, 0, err
	}
	pinRows := make([]discoveredPin, 0, len(pins))
	for _, pin := range pins {
		view, inspectErr := review.Inspect(s, pin.ID)
		needsAnother := inspectErr != nil || pin.Decision == evidence.Reopened || view.Applicability != evidence.Current || view.MissingCurrentResult
		pinRows = append(pinRows, discoveredPin{id: pin.ID, needsAnother: needsAnother})
	}
	sort.Slice(pinRows, func(i, j int) bool {
		if pinRows[i].needsAnother != pinRows[j].needsAnother {
			return pinRows[i].needsAnother
		}
		return pinRows[i].id < pinRows[j].id
	})

	entries, err := s.List("receipt", "comparison")
	if err != nil {
		return nil, 0, err
	}
	runs := []discoveredRun{}
	receiptTimes := map[evidence.Digest]time.Time{}
	for _, entry := range entries {
		if entry.Kind != "receipt" {
			continue
		}
		receipt, err := store.Get[evidence.Receipt](s, entry.ID)
		if err != nil {
			return nil, 0, err
		}
		if receipt.Snapshots != pair {
			continue
		}
		runs = append(runs, discoveredRun{id: receipt.ID, finished: receipt.FinishedAt})
		receiptTimes[receipt.ID] = receipt.FinishedAt
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].finished.Equal(runs[j].finished) {
			return runs[i].finished.After(runs[j].finished)
		}
		return runs[i].id > runs[j].id
	})
	comparisons := []discoveredComparison{}
	for _, entry := range entries {
		if entry.Kind != "comparison" {
			continue
		}
		comparison, err := store.Get[evidence.Comparison](s, entry.ID)
		if err != nil {
			return nil, 0, err
		}
		finished, matches := receiptTimes[comparison.Receipt]
		if matches {
			comparisons = append(comparisons, discoveredComparison{id: comparison.ID, finished: finished})
		}
	}
	sort.Slice(comparisons, func(i, j int) bool {
		if !comparisons[i].finished.Equal(comparisons[j].finished) {
			return comparisons[i].finished.After(comparisons[j].finished)
		}
		return comparisons[i].id > comparisons[j].id
	})

	artifacts, err := s.List("artifact")
	if err != nil {
		return nil, 0, err
	}
	reports := []discoveredReport{}
	for _, entry := range artifacts {
		metadata, err := s.ReadArtifactMetadata(entry.ID)
		if err != nil {
			return nil, 0, err
		}
		if metadata.Channel != "report" || metadata.Bytes > gotestreport.MaxBytes {
			continue
		}
		raw, err := s.ReadBlob(metadata.Content)
		if err != nil {
			return nil, 0, err
		}
		var report gotestreport.Report
		if decode(raw, &report) != nil || !validDiscoveredReport(report) || report.Metadata.Snapshot != pair.Candidate {
			continue
		}
		reports = append(reports, discoveredReport{id: metadata.Content, imported: report.Metadata.ImportedAt})
	}
	sort.Slice(reports, func(i, j int) bool {
		if !reports[i].imported.Equal(reports[j].imported) {
			return reports[i].imported.After(reports[j].imported)
		}
		return reports[i].id > reports[j].id
	})

	all := make([]evidence.Digest, 0, len(pinRows)+len(runs)+len(comparisons)+len(reports))
	seen := make(map[evidence.Digest]bool)
	appendID := func(id evidence.Digest) {
		if id != "" && !seen[id] {
			seen[id] = true
			all = append(all, id)
		}
	}
	for _, pin := range pinRows {
		appendID(pin.id)
	}
	for _, run := range runs {
		appendID(run.id)
	}
	for _, comparison := range comparisons {
		appendID(comparison.id)
	}
	for _, report := range reports {
		appendID(report.id)
	}
	omitted := max(len(all)-MaxEvidence, 0)
	return all[:min(len(all), MaxEvidence)], omitted, nil
}

func validDiscoveredReport(report gotestreport.Report) bool {
	if report.SchemaVersion != 1 || report.Dialect != gotestreport.Dialect || (report.Completeness != evidence.Complete && report.Completeness != evidence.Incomplete) || len(report.Cards) > gotestreport.MaxCards || report.Metadata.Snapshot == "" || report.Metadata.ImportedAt.IsZero() {
		return false
	}
	for _, card := range report.Cards {
		if card.State.Validate() != nil || card.State.Producer != evidence.Importer || card.State.Kind != evidence.Reported || card.State.Applicability != evidence.Unknown || card.State.Execution != evidence.NotRun || card.State.Comparison != evidence.NotCompared {
			return false
		}
	}
	return true
}
