// Package browser adapts immutable engine records to read-only terminal pages.
// It never executes project code or derives evidence from repository prose.
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/store"
)

const MaxEvidence = 32
const PageBytes = 4096

type Selection struct {
	Project  string
	Pair     evidence.SnapshotPair
	Evidence []evidence.Digest
}
type Section struct {
	Name    string
	Content []byte
	Blob    evidence.Digest
}
type Entry struct {
	Label    string // trusted state summary, never repository prose
	Name     string // untrusted description
	Sections []Section
}
type Data struct {
	Selection Selection
	Entries   []Entry
	Inventory []Entry
	Patch     Section
	Limits    Section
}

func document(name string, value any) Section {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		raw = []byte("document unavailable")
	}
	return Section{Name: name, Content: raw}
}
func decode(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

// Load preserves inventory even when an individual evidence ID is unavailable.
// All work is bounded by the store, report adapter and explicit ID count.
func Load(ctx context.Context, selected Selection) (*Data, error) {
	if len(selected.Evidence) > MaxEvidence {
		return nil, errors.New("at most 32 evidence IDs")
	}
	s, err := store.Open(selected.Project, false, nil)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	base, err := store.Get[evidence.Snapshot](s, selected.Pair.Base)
	if err != nil {
		return nil, err
	}
	candidate, err := store.Get[evidence.Snapshot](s, selected.Pair.Candidate)
	if err != nil {
		return nil, err
	}
	raw, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		return nil, err
	}
	d := &Data{Selection: selected, Patch: Section{Name: "captured raw diff", Blob: candidate.Diff}, Limits: document("capture limits", raw.Limits())}
	for _, item := range raw.Inventory() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e := Entry{Label: fmt.Sprintf("not checked | %s | binary=%t | potential oracle=%t", item.Change, item.Binary, item.PotentialOracle), Name: item.Path, Sections: []Section{document("inventory (unclassified)", item)}}
		if item.Base != nil {
			e.Sections = append(e.Sections, Section{Name: "captured base source", Blob: item.Base.Content})
		}
		if item.Candidate != nil {
			e.Sections = append(e.Sections, Section{Name: "captured candidate source", Blob: item.Candidate.Content})
		}
		d.Inventory = append(d.Inventory, e)
	}
	for _, id := range selected.Evidence {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := loadEvidence(s, id, selected.Pair)
		if err != nil {
			entries = []Entry{{Label: "not checked | unknown | unavailable", Name: string(id), Sections: []Section{document("load limitation", "Evidence unavailable or invalid; raw inventory remains usable. Use after inspect for the stored ID.")}}}
		}
		d.Entries = append(d.Entries, entries...)
	}
	return d, nil
}

func loadEvidence(s *store.Store, id evidence.Digest, pair evidence.SnapshotPair) ([]Entry, error) {
	var comparison *evidence.Comparison
	c, err := store.Get[evidence.Comparison](s, id)
	if err == nil {
		comparison = &c
		id = c.Receipt
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	r, err := store.Get[evidence.Receipt](s, id)
	if err == nil {
		state := r.State
		completeness := r.Completeness
		// A receipt is only current for its own immutable pair. Selection never
		// creates a new result, and an unknown/stale receipt is never promoted.
		if r.Snapshots != pair {
			state.Applicability = evidence.Stale
		}
		if comparison != nil {
			state.Comparison = comparison.Outcome
			if comparison.Completeness != evidence.Complete {
				completeness = evidence.Incomplete
			}
		}
		e := Entry{Label: label(state, completeness), Name: string(r.ID), Sections: []Section{document("receipt: producer, bindings, timestamps, limits", r)}}
		if r.Bindings != nil {
			scenario, err := store.Get[evidence.Scenario](s, r.Bindings.Scenario)
			if err != nil {
				return nil, err
			}
			e.Sections = append(e.Sections, document("frozen scenario", scenario), Section{Name: "exact frozen input", Blob: scenario.Input})
		}
		if comparison != nil {
			e.Sections = append(e.Sections, document("comparison outcome and limits", comparison))
			if comparison.Details != nil {
				raw, err := s.ReadBlob(comparison.Details.Content)
				if err != nil {
					return nil, err
				}
				var report compare.Report
				if decode(raw, &report) != nil || report.Receipt != r.ID || report.Snapshots != r.Snapshots || report.Outcome != comparison.Outcome {
					return nil, errors.New("misbound comparison details")
				}
				e.Sections = append(e.Sections, Section{Name: "exact before/after channel witnesses", Blob: comparison.Details.Content})
			}
		}
		// Retain every sample, observation and diagnostic, including failed runs
		// and all unstable repetitions. Channel names remain untrusted data.
		for _, a := range r.Artifacts {
			e.Sections = append(e.Sections, Section{Name: "artifact: " + a.Channel, Blob: a.Content})
		}
		return []Entry{e}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	raw, err := s.ReadBlob(id)
	if err != nil {
		return nil, err
	}
	var report gotestreport.Report
	if decode(raw, &report) != nil || report.SchemaVersion != 1 || report.Dialect != gotestreport.Dialect || (report.Completeness != evidence.Complete && report.Completeness != evidence.Incomplete) || len(report.Cards) > gotestreport.MaxCards {
		return nil, errors.New("unsupported report")
	}
	entries := []Entry{}
	summary := document("caller provenance, snapshot binding, timestamps and report limits", struct {
		Metadata     gotestreport.Metadata     `json:"metadata"`
		Completeness evidence.Completeness     `json:"completeness"`
		Diagnostics  []gotestreport.Diagnostic `json:"diagnostics"`
		Suppressed   int                       `json:"suppressed_diagnostics"`
	}{report.Metadata, report.Completeness, report.Diagnostics, report.SuppressedDiagnostics})
	for _, card := range report.Cards {
		state := card.State
		if state.Validate() != nil || state.Kind != evidence.Reported || state.Producer != evidence.Importer || state.Applicability != evidence.Unknown || state.Execution != evidence.NotRun || state.Comparison != evidence.NotCompared {
			return nil, errors.New("invalid reported state")
		}
		if report.Metadata.Snapshot != "" && report.Metadata.Snapshot != pair.Candidate {
			state.Applicability = evidence.Stale
		}
		entries = append(entries, Entry{Label: label(state, report.Completeness), Name: card.Package + " / " + card.Test, Sections: []Section{document("reported case: inputs/effects unavailable, not observations", card), summary}})
	}
	if len(entries) == 0 {
		entries = append(entries, Entry{Label: "not checked | unknown | no reported cases", Name: string(id), Sections: []Section{summary}})
	}
	return entries, nil
}
func label(s evidence.EvidenceState, complete evidence.Completeness) string {
	kind := string(s.Kind)
	if s.Kind == evidence.NoEvidence {
		kind = "not checked"
	}
	return fmt.Sprintf("%s | %s | %s | %s | %s | report=%s", kind, s.Applicability, s.Execution, s.Comparison, complete, s.Report)
}

type Page struct {
	Bytes         []byte
	Offset, Total int
}

// ReadPage uses stored content only, never paths from the live checkout. An
// unavailable/corrupt artifact is a visible error, not an empty/equal result.
func ReadPage(ctx context.Context, project string, section Section, offset int) (Page, error) {
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	raw := section.Content
	if section.Blob != "" {
		s, err := store.Open(project, false, nil)
		if err != nil {
			return Page{}, err
		}
		defer s.Close()
		raw, err = s.ReadBlob(section.Blob)
		if err != nil {
			return Page{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	offset = min(max(offset, 0), len(raw))
	end := min(offset+PageBytes, len(raw))
	return Page{Bytes: append([]byte(nil), raw[offset:end]...), Offset: offset, Total: len(raw)}, nil
}
