// Package browser adapts immutable engine records to read-only terminal documents.
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
	"strconv"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
)

const MaxEvidence = 32

type documentFormat uint8

const (
	formatVerbatim documentFormat = iota
	formatObservation
	formatSample
	formatComparison
)

type Selection struct {
	Project          string
	Pair             evidence.SnapshotPair
	Evidence         []evidence.Digest
	Discover         bool
	OmittedEvidence  int
	DiscoveryWarning bool
}
type Section struct {
	Name    string
	Content []byte
	Blob    evidence.Digest
	format  documentFormat
}
type Entry struct {
	Receipt              evidence.Digest
	Expectation          string
	State                evidence.EvidenceState
	Completeness         evidence.Completeness
	Decision             evidence.HumanDecision
	Unavailable          bool
	MissingCurrentResult bool
	Candidate            evidence.Digest
	DecisionAt           time.Time
	Summary              string // untrusted data, never a badge
	Name                 string // untrusted description
	Change               string
	PotentialOracle      bool
	Binary               bool
	BaseMode             string
	CandidateMode        string
	Added                int
	Deleted              int
	Limits               []string
	Sections             []Section
}
type Data struct {
	Selection          Selection
	BaseSnapshot       evidence.Snapshot
	CandidateSnapshot  evidence.Snapshot
	Entries            []Entry
	Inventory          []Entry
	InventoryRows      []InventoryRow
	InventoryPositions []int
	Diff               *DiffView
	Patch              Section
	Limits             Section
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
	if selected.Discover {
		selected.Evidence, selected.OmittedEvidence, err = discoverEvidence(s, selected.Pair)
		if err != nil {
			selected.Evidence = nil
			selected.OmittedEvidence = 0
			selected.DiscoveryWarning = true
		}
	}
	d := &Data{Selection: selected, BaseSnapshot: base, CandidateSnapshot: candidate, Limits: document("capture limits", raw.Limits())}
	inventory := raw.Inventory()
	if rawdiff.CapturedPair(base, candidate) {
		patch, err := readRawDiff(raw)
		if err == nil {
			d.Diff = buildDiff(patch, raw.Files(), raw.Hunks(), inventory)
			counts, countErr := raw.Count(nil, nil)
			d.Diff.HunksComplete = countErr == nil && counts.Complete
			d.Patch = Section{Name: "captured raw diff", Content: patch}
		} else {
			d.Patch = document("captured patch unavailable; no patch is fabricated", raw.Limits())
		}
	} else {
		computed, err := raw.Compute(ctx)
		if err != nil {
			return nil, err
		}
		inventory = computed.Inventory
		d.Diff = buildComputedDiff(computed.Raw, computed.Files, computed.HunkOffsets, inventory)
		for _, entry := range inventory {
			if entry.Change == "unknown" || len(entry.Limits) > 0 {
				d.Diff.SourceLimited = true
				break
			}
		}
		d.Patch = Section{Name: rawdiff.ComputedOrigin, Content: computed.Raw}
		limits := append(raw.Limits(), computed.Limits...)
		d.Limits = document("capture and computed diff limits", limits)
	}
	counts := map[string][2]int{}
	if d.Diff != nil {
		for _, file := range d.Diff.Files {
			current := counts[file.Path]
			current[0] += file.Added
			current[1] += file.Deleted
			counts[file.Path] = current
		}
	}
	for _, item := range inventory {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count := counts[item.Path]
		added, deleted := count[0], count[1]
		baseMode, candidateMode := "", ""
		if item.Base != nil {
			baseMode = item.Base.Mode
		}
		if item.Candidate != nil {
			candidateMode = item.Candidate.Mode
		}
		flags := pathSummary(item, added, deleted)
		diffOrigin := ""
		if d.Diff != nil {
			diffOrigin = d.Diff.Origin
		}
		record := inventoryRecord{Path: item.Path, Change: item.Change, PotentialOracle: item.PotentialOracle, Binary: item.Binary, BaseMode: baseMode, CandidateMode: candidateMode, DiffOrigin: diffOrigin, Added: added, Deleted: deleted, Limits: append([]string(nil), item.Limits...)}
		sections := []Section{{Name: "Diff", Content: pathDiff(d, item.Path)}}
		sections = append(sections, sourceSection("Base source", item.Path, item.Base, item.Change, true, item.Limits))
		sections = append(sections, sourceSection("Candidate source", item.Path, item.Candidate, item.Change, false, item.Limits))
		sections = append(sections, document("Inventory record", record))
		e := Entry{Name: item.Path, Summary: flags, Change: item.Change, PotentialOracle: item.PotentialOracle, Binary: item.Binary, BaseMode: baseMode, CandidateMode: candidateMode, Added: added, Deleted: deleted, Limits: append([]string(nil), item.Limits...), Sections: sections}
		d.Inventory = append(d.Inventory, e)
	}
	d.InventoryRows, d.InventoryPositions = inventoryRows(d.Inventory)
	for _, id := range selected.Evidence {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := loadEvidence(s, id, selected.Pair)
		if err != nil {
			entries = []Entry{{Unavailable: true, Name: string(id), Sections: []Section{document("load limitation", "Evidence unavailable or invalid; raw inventory remains usable. Use after inspect for the stored ID.")}}}
		}
		d.Entries = append(d.Entries, entries...)
	}
	return d, nil
}

func pathDiff(data *Data, path string) []byte {
	if data.Diff == nil {
		return []byte("No diff is available for this pair. The captured inventory and both source sections remain available.")
	}
	for _, file := range data.Diff.Files {
		if file.Path == path {
			return data.Diff.Raw[file.Start:file.End]
		}
	}
	if data.Diff.Origin == rawdiff.ComputedOrigin {
		for _, entry := range data.Inventory {
			if entry.Name == path {
				if entry.Change == "unknown" {
					return []byte("Unknown path; no computed diff. Recorded limitation: " + strings.Join(entry.Limits, "; "))
				}
				if len(entry.Limits) > 0 {
					return []byte(strings.Join(entry.Limits, "; "))
				}
			}
		}
		return []byte("No computed diff lines for this path; both captured sources remain available.")
	}
	return []byte("No captured patch lines for this path. The captured inventory and source sections remain available.")
}

func sourceSection(name, path string, file *evidence.File, change string, base bool, limits []string) Section {
	if file != nil {
		return Section{Name: name, Blob: file.Content}
	}
	side := "base"
	if !base {
		side = "candidate"
	}
	if base && change == "added" {
		return Section{Name: name, Content: []byte("No base source: this path was added; no base file exists in the captured manifest.")}
	}
	if !base && change == "deleted" {
		return Section{Name: name, Content: []byte("No candidate source: this path was deleted; no candidate file exists in the captured manifest.")}
	}
	message := side + " source unavailable: no content for this path was retained in the " + side + " manifest."
	if len(limits) > 0 {
		message += "\nRecorded limitations: " + strings.Join(limits, "; ")
	}
	return Section{Name: name, Content: []byte(message)}
}

func loadEvidence(s *store.Store, id evidence.Digest, pair evidence.SnapshotPair) ([]Entry, error) {
	if pin, err := store.Get[evidence.Pin](s, id); err == nil {
		v, err := review.Inspect(s, id)
		if err != nil {
			return nil, err
		}
		applicability := v.Applicability
		last := pin.History[len(pin.History)-1]
		missing := v.MissingCurrentResult || pin.Scope == ""
		if last.Review != nil && last.Review.Target.Snapshots != pair {
			applicability = evidence.Stale
			missing = true
		}
		e := Entry{State: evidence.EvidenceState{Applicability: applicability}, Decision: pin.Decision, DecisionAt: last.At, MissingCurrentResult: missing, Name: pin.Expectation, Sections: []Section{document("pin revision / current evidence / exact reopening reason", v)}}
		// Every prior observation remains reachable with its original snapshot scope.
		seen := map[evidence.Digest]bool{}
		for i := len(pin.History) - 1; i >= 0; i-- {
			event := pin.History[i]
			if event.Review == nil || event.Review.Receipt == "" || seen[event.Review.Receipt] {
				continue
			}
			seen[event.Review.Receipt] = true
			rows, err := loadEvidence(s, event.Review.Receipt, pair)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				e.Sections = append(e.Sections, row.Sections...)
			}
		}
		return []Entry{e}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
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
		state.Comparison = evidence.NotCompared // only an opened comparison supplies witnesses
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
		e := Entry{State: state, Completeness: completeness, Candidate: r.Snapshots.Candidate, Name: string(r.ID), Sections: []Section{document("receipt: producer, bindings, timestamps, limits", r)}}
		if r.Bindings != nil {
			scenario, err := store.Get[evidence.Scenario](s, r.Bindings.Scenario)
			if err != nil {
				return nil, err
			}
			e.Sections = append(e.Sections, document("frozen scenario", scenario), Section{Name: "exact frozen input", Blob: scenario.Input})
		}
		var report compare.Report
		if comparison != nil {
			e.Sections = append(e.Sections, document("comparison outcome and limits", comparison))
			if comparison.Details != nil {
				raw, err := s.ReadBlob(comparison.Details.Content)
				if err != nil {
					return nil, err
				}
				if decode(raw, &report) != nil || report.Receipt != r.ID || report.Snapshots != r.Snapshots || report.Outcome != comparison.Outcome {
					return nil, errors.New("misbound comparison details")
				}
				e.Sections = append(e.Sections, Section{Name: "exact before/after channel witnesses", Blob: comparison.Details.Content, format: formatComparison})
			}
		}
		// Retain every sample, observation and diagnostic, including failed runs
		// and all unstable repetitions. Channel names remain untrusted data.
		for _, a := range r.Artifacts {
			e.Sections = append(e.Sections, Section{Name: "artifact: " + a.Channel, Blob: a.Content, format: artifactFormat(a.Channel)})
		}
		rows := []Entry{}
		if r.State.Kind == evidence.Observed && r.State.Execution == evidence.Completed && r.Completeness == evidence.Complete && !r.Redacted {
			for _, sec := range []int64{43200, 30} {
				counts := map[string][]int{}
				for _, a := range r.Artifacts {
					for _, side := range []string{"base", "candidate"} {
						if strings.HasPrefix(a.Channel, fmt.Sprintf("%s/%d/", side, sec)) && strings.HasSuffix(a.Channel, "/observation") && a.Completeness == evidence.Complete && !a.Redacted && !a.Truncated {
							raw, err := s.ReadBlob(a.Content)
							if err != nil {
								return nil, err
							}
							var o runner.Observation
							if decode(raw, &o) != nil || o.Version != 1 || o.Seconds != sec || o.Calls == nil {
								return nil, errors.New("invalid observation")
							}
							counts[side] = append(counts[side], len(o.Calls))
						}
					}
				}
				if len(counts["base"]) == 0 || len(counts["candidate"]) == 0 {
					continue
				}
				summary := "provider requests " + countText(counts["base"]) + " → " + countText(counts["candidate"])
				row := e
				row.Name = delayText(sec) + " same-key retry"
				if comparison != nil {
					row.State.Comparison = report.CaseOutcome(sec, completeness)
					if row.State.Comparison != evidence.Incomparable {
						responses := "same"
						for _, w := range report.Witnesses {
							if w.Before.Seconds == sec && w.Channel == "responses" && w.Outcome == evidence.Different {
								responses = "different"
								if w.Relation == "repetition" {
									responses = "unstable"
									break
								}
							}
						}
						summary += " · responses " + responses
					}
				}
				row.Summary = summary
				row.Sections = append([]Section{document("measured provider-request counts (original receipt scope; finite samples only)", summary)}, e.Sections...)
				same := true
				for _, count := range counts["candidate"] {
					if count != counts["candidate"][0] {
						same = false
					}
				}
				if same && r.Snapshots == pair {
					row.Receipt = r.ID
					row.Expectation = fmt.Sprintf("At %ds, expect %d provider request(s) for the frozen two same-key requests; finite example only", sec, counts["candidate"][0])
				}
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			rows = append(rows, e)
		}
		return rows, nil
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
		metadata := struct {
			Package         string                 `json:"package"`
			Test            string                 `json:"test,omitempty"`
			Scope           string                 `json:"scope"`
			Attempt         int                    `json:"attempt"`
			State           evidence.EvidenceState `json:"state"`
			OutputTruncated bool                   `json:"output_truncated"`
			FirstEventAt    *time.Time             `json:"first_event_at,omitempty"`
			LastEventAt     *time.Time             `json:"last_event_at,omitempty"`
			Inputs          string                 `json:"inputs"`
			ExpectedValues  string                 `json:"expected_values"`
			Effects         string                 `json:"effects"`
		}{card.Package, card.Test, card.Scope, card.Attempt, card.State, card.OutputTruncated, card.FirstEventAt, card.LastEventAt, card.Inputs, card.ExpectedValues, card.Effects}
		sections := []Section{document("reported case: inputs/effects unavailable, not observations", metadata), {Name: "reported output", Content: []byte(card.Output)}, summary}
		name := card.Test
		if card.Scope == "package" {
			name = card.Package + " (package)"
		}
		entries = append(entries, Entry{State: state, Completeness: report.Completeness, Name: name, Sections: sections})
	}
	if len(entries) == 0 {
		entries = append(entries, Entry{Summary: "no reported cases", Name: string(id), Sections: []Section{summary}})
	}
	return entries, nil
}
func delayText(seconds int64) string {
	h, m, s := seconds/3600, seconds%3600/60, seconds%60
	text := ""
	if h != 0 {
		text += fmt.Sprintf("%dh", h)
	}
	if m != 0 {
		text += fmt.Sprintf("%dm", m)
	}
	if s != 0 || text == "" {
		text += fmt.Sprintf("%ds", s)
	}
	return text
}

func countText(counts []int) string {
	same := true
	values := make([]string, len(counts))
	for i, n := range counts {
		values[i] = strconv.Itoa(n)
		if n != counts[0] {
			same = false
		}
	}
	if same && len(values) > 0 {
		return values[0]
	}
	return strings.Join(values, ",")
}

// ReadSection returns the exact captured bytes, never a live path or formatted
// representation. Callers doing document indexing must keep it off the event loop.
func ReadSection(ctx context.Context, project string, section Section) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw := section.Content
	if section.Blob != "" {
		s, err := store.Open(project, false, nil)
		if err != nil {
			return nil, err
		}
		defer s.Close()
		raw, err = s.ReadBlob(section.Blob)
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]byte(nil), raw...), nil
}

func artifactFormat(channel string) documentFormat {
	parts := strings.Split(channel, "/")
	if len(parts) != 4 || (parts[0] != "base" && parts[0] != "candidate") {
		return formatVerbatim
	}
	if parts[1] != "43200" && parts[1] != "30" {
		return formatVerbatim
	}
	repetition, err := strconv.Atoi(parts[2])
	if err != nil || repetition < 0 || repetition > 4 {
		return formatVerbatim
	}
	switch parts[3] {
	case "observation":
		return formatObservation
	case "sample":
		return formatSample
	default:
		return formatVerbatim
	}
}

// displayJSON indents only explicitly typed AFTER artifacts. It first decodes
// with the matching type and falls back to exact bytes on any schema, size or
// line-limit issue; the hex representation always uses the original bytes.
func displayJSON(raw []byte, format documentFormat) []byte {
	// These AFTER-produced artifacts are each stored under a 1 MiB producer
	// budget. Never allocate from a larger or unrecognized local artifact.
	if len(raw) > 1<<20 {
		return raw
	}
	switch format {
	case formatObservation:
		observation := new(runner.Observation)
		if err := decode(raw, observation); err != nil || observation.Version != 1 || (observation.Seconds != 30 && observation.Seconds != 43200) || len(observation.Responses) != 2 || observation.Calls == nil || len(observation.Calls) > 128 {
			return raw
		}
	case formatSample:
		sample := new(runner.Sample)
		if err := decode(raw, sample); err != nil || len(sample.Artifacts) > 3 {
			return raw
		}
	case formatComparison:
		report := new(compare.Report)
		if err := decode(raw, report); err != nil || len(report.Artifacts) > 256 || len(report.Witnesses) > 4096 || len(report.Limits) > 100 {
			return raw
		}
		totalChanges := 0
		for _, witness := range report.Witnesses {
			totalChanges += len(witness.Changes)
			if totalChanges > 2048 {
				return raw
			}
		}
	default:
		return raw
	}
	formatted, ok := indentJSON(raw)
	if !ok {
		return raw
	}
	return formatted
}

// indentJSON inserts bounded whitespace while preserving each JSON token's raw
// bytes. It is called only after strict decoding into the artifact's typed AFTER
// record; excess output bytes or lines fall back to the untouched stored bytes.
func indentJSON(raw []byte) ([]byte, bool) {
	out := make([]byte, 0, min(len(raw)+len(raw)/4, terminal.MaxTextBytes))
	lines := 1
	write := func(text []byte) bool {
		if len(text) > terminal.MaxTextBytes-len(out) {
			return false
		}
		for _, b := range text {
			if b == '\n' {
				lines++
				if lines > terminal.MaxLines {
					return false
				}
			}
		}
		out = append(out, text...)
		return true
	}
	indent := func(depth int) bool {
		spaces := depth * 2
		if spaces > terminal.MaxTextBytes-len(out) {
			return false
		}
		for i := 0; i < spaces; i++ {
			out = append(out, ' ')
		}
		return true
	}
	var stack []byte
	for i := 0; i < len(raw); {
		b := raw[i]
		switch b {
		case ' ', '\t', '\r', '\n':
			i++
		case '{', '[':
			close := byte('}')
			if b == '[' {
				close = ']'
			}
			next := i + 1
			for next < len(raw) && (raw[next] == ' ' || raw[next] == '\t' || raw[next] == '\r' || raw[next] == '\n') {
				next++
			}
			if !write(raw[i : i+1]) {
				return nil, false
			}
			if next < len(raw) && raw[next] == close {
				if !write(raw[next : next+1]) {
					return nil, false
				}
				i = next + 1
				continue
			}
			stack = append(stack, close)
			if !write([]byte{'\n'}) || !indent(len(stack)) {
				return nil, false
			}
			i++
		case '}', ']':
			if len(stack) == 0 || stack[len(stack)-1] != b {
				return nil, false
			}
			if !write([]byte{'\n'}) || !indent(len(stack)-1) || !write(raw[i:i+1]) {
				return nil, false
			}
			stack = stack[:len(stack)-1]
			i++
		case ',':
			if !write([]byte{',', '\n'}) || !indent(len(stack)) {
				return nil, false
			}
			i++
		case ':':
			if !write([]byte{':', ' '}) {
				return nil, false
			}
			i++
		case '"':
			start := i
			i++
			for i < len(raw) {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '"' {
					i++
					break
				}
				i++
			}
			if i > len(raw) || !write(raw[start:i]) {
				return nil, false
			}
		default:
			start := i
			for i < len(raw) && raw[i] != ',' && raw[i] != ']' && raw[i] != '}' && raw[i] != ':' && raw[i] != ' ' && raw[i] != '\t' && raw[i] != '\r' && raw[i] != '\n' {
				i++
			}
			if i == start || !write(raw[start:i]) {
				return nil, false
			}
		}
	}
	return out, len(stack) == 0
}
