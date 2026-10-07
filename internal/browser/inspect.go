package browser

import (
	"errors"
	"os"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
)

// Inspect returns the same immutable detail Cards used by the TUI for one
// receipt, comparison, pin revision, or imported report. It never executes code.
func Inspect(project string, id evidence.Digest) ([]Entry, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	var pair evidence.SnapshotPair
	receiptCard := false
	if pin, err := store.Get[evidence.Pin](s, id); err == nil {
		if len(pin.History) > 0 && pin.History[len(pin.History)-1].Review != nil {
			pair = pin.History[len(pin.History)-1].Review.Target.Snapshots
		} else {
			pair = pin.BasisSnapshots
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return []Entry{unavailableEntry(id, rawUnavailableFallback(s, id, err))}, nil
	} else if comparison, err := store.Get[evidence.Comparison](s, id); err == nil {
		receiptCard = true
		receipt, err := store.Get[evidence.Receipt](s, comparison.Receipt)
		if err != nil {
			return []Entry{unavailableEntry(id, rawUnavailableFallback(s, id, err))}, nil
		}
		pair = receipt.Snapshots
	} else if !errors.Is(err, os.ErrNotExist) {
		return []Entry{unavailableEntry(id, rawUnavailableFallback(s, id, err))}, nil
	} else if receipt, err := store.Get[evidence.Receipt](s, id); err == nil {
		receiptCard = true
		pair = receipt.Snapshots
	} else if !errors.Is(err, os.ErrNotExist) {
		return []Entry{unavailableEntry(id, rawUnavailableFallback(s, id, err))}, nil
	} else {
		raw, readErr := s.ReadBlob(id)
		if readErr != nil {
			return []Entry{{Unavailable: true, Name: string(id), Sections: unavailableCard(id, "stored ID", "No supported receipt, comparison, pin, or report record was found."), IDs: []evidence.Digest{id}}}, nil
		}
		var report gotestreport.Report
		if err := decode(raw, &report); err != nil || report.SchemaVersion != 1 || report.Dialect != gotestreport.Dialect {
			return []Entry{{Unavailable: true, Name: string(id), Sections: rawFallbackSections("artifact", id, raw, "Strict decoding or expected Card shape failed; showing the full raw artifact."), IDs: []evidence.Digest{id}}}, nil
		}
		pair.Candidate = report.Metadata.Snapshot
	}
	entries, err := loadEvidence(s, id, pair)
	if err != nil {
		return []Entry{unavailableEntry(id, rawUnavailableFallback(s, id, err))}, nil
	}
	if receiptCard && len(entries) > 0 {
		for _, name := range []string{"Receipt Card", "Card"} {
			for _, section := range entries[0].Sections {
				if section.Name == name {
					entry := entries[0]
					entry.Name = string(id)
					section.Name = "Card"
					entry.Sections = []Section{section}
					return []Entry{entry}, nil
				}
			}
		}
	}
	return entries, nil
}
