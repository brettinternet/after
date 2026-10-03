// Package gotestreport imports stock Go test JSON as unverified reported data.
// It never opens paths, fetches URLs, runs commands, or produces observations.
package gotestreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
)

const (
	Dialect        = "go-test-json-v1"
	MaxBytes       = 8 << 20
	MaxLineBytes   = 64 << 10
	MaxCards       = 4096
	MaxOutputBytes = 16 << 10
	MaxDiagnostics = 100
)

// Metadata is caller-supplied, not authenticated. A snapshot digest binds a
// report to the caller's choice; it does not prove the tests ran on that source.
type Metadata struct {
	Producer   string          `json:"producer"`
	Snapshot   evidence.Digest `json:"snapshot,omitempty"`
	CapturedAt *time.Time      `json:"captured_at,omitempty"`
	ImportedAt time.Time       `json:"imported_at"`
}

type Diagnostic struct {
	Line int    `json:"line"`
	Code string `json:"code"`
}

type Card struct {
	Package         string                 `json:"package"`
	Test            string                 `json:"test,omitempty"`
	Scope           string                 `json:"scope"` // package, test, or build
	Attempt         int                    `json:"attempt"`
	State           evidence.EvidenceState `json:"state"`
	Output          string                 `json:"output"`
	OutputTruncated bool                   `json:"output_truncated"`
	FirstEventAt    *time.Time             `json:"first_event_at,omitempty"`
	LastEventAt     *time.Time             `json:"last_event_at,omitempty"`
	Inputs          string                 `json:"inputs"`
	ExpectedValues  string                 `json:"expected_values"`
	Effects         string                 `json:"effects"`
}

type Report struct {
	SchemaVersion         int                   `json:"schema_version"`
	Dialect               string                `json:"dialect"`
	Metadata              Metadata              `json:"metadata"`
	OriginalDigest        evidence.Digest       `json:"original_digest"`
	OriginalBytes         int                   `json:"original_bytes"`
	Cards                 []Card                `json:"cards"`
	Completeness          evidence.Completeness `json:"completeness"`
	Diagnostics           []Diagnostic          `json:"diagnostics"`
	SuppressedDiagnostics int                   `json:"suppressed_diagnostics"`
}

type event struct {
	Time        *time.Time
	Action      string
	Package     string
	ImportPath  string
	Test        string
	Elapsed     *float64
	Output      string
	OutputType  string
	FailedBuild string
}

func validDigest(d evidence.Digest) bool {
	s := string(d)
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}

// Import consumes at most MaxBytes+1 bytes. Oversized/unreadable sources fail
// outright (no partial digest); recoverable line errors retain unrelated cards.
// Callers own reader cancellation/deadlines. Report strings remain untrusted.
func Import(src io.Reader, meta Metadata) (Report, error) {
	var result Report
	if src == nil || len(meta.Producer) > 256 || strings.TrimSpace(meta.Producer) == "" || meta.ImportedAt.IsZero() || (meta.CapturedAt != nil && meta.CapturedAt.IsZero()) || (meta.Snapshot != "" && !validDigest(meta.Snapshot)) {
		return result, errors.New("invalid report metadata")
	}
	raw, err := io.ReadAll(io.LimitReader(src, MaxBytes+1))
	if err != nil {
		return result, errors.New("cannot read report")
	}
	if len(raw) > MaxBytes {
		return result, errors.New("report exceeds 8 MiB limit; nothing imported")
	}
	sum := sha256.Sum256(raw)
	result = Report{SchemaVersion: 1, Dialect: Dialect, Metadata: meta, OriginalDigest: evidence.Digest("sha256:" + hex.EncodeToString(sum[:])), OriginalBytes: len(raw), Cards: []Card{}, Diagnostics: []Diagnostic{}, Completeness: evidence.Complete}
	diagnostic := func(line int, code string) {
		result.Completeness = evidence.Incomplete
		if len(result.Diagnostics) < MaxDiagnostics {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{line, code})
		} else {
			result.SuppressedDiagnostics++
		}
	}
	type key struct{ pkg, test, scope string }
	indices := map[key]int{}
	lines := bytes.Split(raw, []byte{'\n'})
	for n, line := range lines {
		if len(line) == 0 && n == len(lines)-1 {
			continue
		}
		if len(line) > MaxLineBytes {
			diagnostic(n+1, "line_limit")
			continue
		}
		if !utf8.Valid(line) {
			diagnostic(n+1, "invalid_utf8")
			continue
		}
		var e event
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&e); err != nil {
			diagnostic(n+1, "invalid_or_unsupported_json")
			continue
		}
		if decoder.Decode(new(any)) != io.EOF {
			diagnostic(n+1, "trailing_json")
			continue
		}
		scope := "package"
		if e.Test != "" {
			scope = "test"
		}
		switch e.Action {
		case "build-output", "build-fail":
			if e.ImportPath == "" || e.Package != "" || e.Test != "" {
				diagnostic(n+1, "unsupported_event")
				continue
			}
			scope = "build"
			e.Package = e.ImportPath
		case "start", "run", "pause", "cont", "pass", "fail", "skip", "output":
			if e.Package == "" || e.ImportPath != "" || (e.Action == "start" && e.Test != "") || ((e.Action == "run" || e.Action == "pause" || e.Action == "cont") && e.Test == "") {
				diagnostic(n+1, "unsupported_event")
				continue
			}
		default:
			diagnostic(n+1, "unsupported_event")
			continue
		}
		if (e.Elapsed != nil && *e.Elapsed < 0) || (e.OutputType != "" && e.OutputType != "frame" && e.OutputType != "error") || (e.OutputType != "" && e.Action != "output") || (e.Output != "" && e.Action != "output" && e.Action != "build-output") || (e.FailedBuild != "" && (e.Action != "fail" || scope != "package")) {
			diagnostic(n+1, "unsupported_event")
			continue
		}
		k := key{e.Package, e.Test, scope}
		i, exists := indices[k]
		attempt := 1
		if exists {
			attempt = result.Cards[i].Attempt
		}
		if exists && result.Cards[i].State.Report != evidence.NoReport {
			// Keep repetitions distinct, including streams concatenated by a caller.
			if e.Action == "run" || e.Action == "start" || (scope == "build" && e.Action == "build-output") {
				exists = false
				attempt++
			} else {
				diagnostic(n+1, "event_after_completion")
				continue
			}
		}
		if !exists {
			if len(result.Cards) == MaxCards {
				diagnostic(n+1, "card_limit")
				continue
			}
			i = len(result.Cards)
			indices[k] = i
			result.Cards = append(result.Cards, Card{Package: e.Package, Test: e.Test, Scope: scope, Attempt: attempt, State: evidence.EvidenceState{Producer: evidence.Importer, Kind: evidence.Reported, Applicability: evidence.Unknown, Execution: evidence.NotRun, Comparison: evidence.NotCompared, Report: evidence.NoReport}, Inputs: "unavailable", ExpectedValues: "unavailable", Effects: "unavailable"})
		}
		c := &result.Cards[i]
		if e.Time != nil && !e.Time.IsZero() {
			if c.FirstEventAt == nil || e.Time.Before(*c.FirstEventAt) {
				c.FirstEventAt = e.Time
			}
			if c.LastEventAt == nil || e.Time.After(*c.LastEventAt) {
				c.LastEventAt = e.Time
			}
		}
		switch e.Action {
		case "pass":
			c.State.Report = evidence.ReportPass
		case "fail", "build-fail":
			c.State.Report = evidence.ReportFail
		case "skip":
			c.State.Report = evidence.ReportSkip
		}
		if c.OutputTruncated {
			continue
		}
		available := MaxOutputBytes - len(c.Output)
		if len(e.Output) > available {
			if !c.OutputTruncated {
				diagnostic(n+1, "output_limit")
			}
			c.OutputTruncated = true
			// Preserve valid UTF-8 at the retained byte boundary.
			for available > 0 && !utf8.ValidString(e.Output[:available]) {
				available--
			}
			c.Output += e.Output[:available]
		} else {
			c.Output += e.Output
		}
	}
	if len(result.Cards) == 0 {
		diagnostic(0, "no_cases")
	}
	for _, c := range result.Cards {
		if c.State.Report == evidence.NoReport {
			diagnostic(0, "missing_completion")
		}
	}
	return result, nil
}
