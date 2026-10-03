// Package rawdiff provides bounded, read-only access to captured changes.
// Returned bytes and paths are untrusted data, not terminal-safe display text.
package rawdiff

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

const (
	MaxPageBytes = 64 << 10
	MaxHunks     = 100000
)

var (
	ErrWindow  = errors.New("invalid byte window")
	ErrContext = errors.New("captured context unavailable")
	ErrDiff    = errors.New("captured diff unavailable")
)

type Side string

const (
	Base      Side = "base"
	Candidate Side = "candidate"
)

// Entry includes known changes and every excluded/unsupported path, even when
// its change status cannot be determined. Unknown is never silently unchanged.
type Entry struct {
	Path            string
	Change          string // added, deleted, modified, or unknown
	Base, Candidate *evidence.File
	PotentialOracle bool
	Binary          bool // Git binary patch, not inferred behavior
	Limits          []string
}

// Hunk identifies a byte range in the immutable raw patch, including its header.
type Hunk struct {
	ID         evidence.Digest
	Path       string
	Start, End int
}

// Page is a bounded byte window. More means another window is available, not
// that bytes were discarded. Limits apply even when the final window is reached.
type Page struct {
	Bytes               []byte
	Offset, Next, Total int
	More                bool
	Limits              []string
}

type View struct {
	s               *store.Store
	base, candidate map[string]evidence.File
	entries         []Entry
	hunks           []Hunk
	raw             []byte
	available       bool
	limits          []string
	indexComplete   bool
}

// Open accepts captured manifests (or snapshots loaded through store.Get).
// It never reads a repository, runs Git, imports evidence or changes a scenario.
// Missing diff bytes leave a usable inventory and an explicit limitation.
func Open(s *store.Store, base, candidate evidence.Snapshot) (*View, error) {
	if s == nil {
		return nil, errors.New("store required")
	}
	if err := base.Validate(); err != nil {
		return nil, err
	}
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	v := &View{s: s, base: files(base), candidate: files(candidate), indexComplete: true}
	v.limits = append(v.limits, base.Limits...)
	v.limits = append(v.limits, candidate.Limits...)
	if base.Completeness != evidence.Complete || candidate.Completeness != evidence.Complete {
		v.indexComplete = false
		v.limits = append(v.limits, "incomplete capture; available bytes are not a complete original diff/context")
	}
	paths := map[string]bool{}
	limitations := map[string][]string{}
	for _, snap := range []evidence.Snapshot{base, candidate} {
		for _, f := range snap.Files {
			paths[f.Path] = true
		}
		for _, group := range []struct {
			name    string
			entries []evidence.Limitation
		}{{"excluded", snap.Excluded}, {"unsupported", snap.Unsupported}} {
			for _, e := range group.entries {
				paths[e.Path] = true
				limitations[e.Path] = append(limitations[e.Path], group.name+": "+e.Reason)
			}
		}
	}
	for p := range paths {
		b, bok := v.base[p]
		c, cok := v.candidate[p]
		equal := bok && cok && b == c
		if len(limitations[p]) == 0 && equal && base.Completeness == evidence.Complete && candidate.Completeness == evidence.Complete {
			continue
		}
		e := Entry{Path: p, PotentialOracle: potentialOracle(p), Limits: limitations[p]}
		if equal && (base.Completeness != evidence.Complete || candidate.Completeness != evidence.Complete) {
			e.Limits = append(e.Limits, "incomplete capture; equal stored content does not establish unchanged original content")
		}
		if bok {
			e.Base = &b
		}
		if cok {
			e.Candidate = &c
		}
		switch {
		case len(e.Limits) > 0:
			e.Change = "unknown"
		case !bok:
			e.Change = "added"
		case !cok:
			e.Change = "deleted"
		default:
			e.Change = "modified"
		}
		v.entries = append(v.entries, e)
	}
	sort.Slice(v.entries, func(i, j int) bool { return v.entries[i].Path < v.entries[j].Path })
	if base.Diff != candidate.Diff {
		v.indexComplete = false
		v.limits = append(v.limits, "snapshots do not share a captured diff; complete stored inventory and both sources remain available, but no patch is claimed for this pair")
		return v, nil
	}
	var err error
	v.raw, err = s.ReadBlob(candidate.Diff)
	if err != nil {
		v.indexComplete = false
		v.limits = append(v.limits, "captured diff unavailable; inventory retained")
		return v, nil
	}
	v.available = true
	v.index(candidate.Diff)
	return v, nil
}

func files(s evidence.Snapshot) map[string]evidence.File {
	out := map[string]evidence.File{}
	for _, f := range s.Files {
		out[f.Path] = f
	}
	return out
}

// Inventory and Hunks return copies; consumers cannot change the view's basis.
func (v *View) Inventory() []Entry {
	out := append([]Entry(nil), v.entries...)
	for i := range out {
		out[i].Limits = append([]string(nil), out[i].Limits...)
		if out[i].Base != nil {
			f := *out[i].Base
			out[i].Base = &f
		}
		if out[i].Candidate != nil {
			f := *out[i].Candidate
			out[i].Candidate = &f
		}
	}
	return out
}
func (v *View) Hunks() []Hunk    { return append([]Hunk(nil), v.hunks...) }
func (v *View) Limits() []string { return append([]string(nil), v.limits...) }

func window(data []byte, offset, size int, limits []string) (Page, error) {
	if offset < 0 || offset > len(data) || size <= 0 || size > MaxPageBytes {
		return Page{}, ErrWindow
	}
	end := offset + min(size, len(data)-offset)
	return Page{Bytes: append([]byte(nil), data[offset:end]...), Offset: offset, Next: end, Total: len(data), More: end < len(data), Limits: append([]string(nil), limits...)}, nil
}
func (v *View) Raw(offset, size int) (Page, error) {
	if !v.available {
		return Page{}, ErrDiff
	}
	return window(v.raw, offset, size, v.limits)
}

// Context matches an exact manifest path, never an OS path. Byte windows may
// split UTF-8 or a long line; renderers must decode/sanitize across windows.
// Store reads verify the whole bounded blob before returning a window.
func (v *View) Context(side Side, path string, offset, size int) (Page, error) {
	if offset < 0 || size <= 0 || size > MaxPageBytes {
		return Page{}, ErrWindow
	}
	var f evidence.File
	var ok bool
	switch side {
	case Base:
		f, ok = v.base[path]
	case Candidate:
		f, ok = v.candidate[path]
	default:
		return Page{}, ErrContext
	}
	if !ok {
		return Page{}, ErrContext
	}
	data, err := v.s.ReadBlob(f.Content)
	if err != nil {
		return Page{}, ErrContext
	}
	return window(data, offset, size, v.limits)
}

// Counts partitions indexed hunks. Folded hunks must be mapped; Mapped counts
// visible mapped hunks, Folded counts hidden mapped hunks. Duplicate references
// across examples never increase totals. Complete is false for a partial index.
type Counts struct {
	Total, Mapped, Folded, Unclassified int
	Complete                            bool
}

func (v *View) Count(examples [][]evidence.Digest, folded []evidence.Digest) (Counts, error) {
	known := map[evidence.Digest]bool{}
	for _, h := range v.hunks {
		known[h.ID] = true
	}
	mapped := map[evidence.Digest]bool{}
	for _, ids := range examples {
		for _, id := range ids {
			if !known[id] {
				return Counts{}, errors.New("unknown mapped hunk")
			}
			mapped[id] = true
		}
	}
	hidden := map[evidence.Digest]bool{}
	for _, id := range folded {
		if !mapped[id] {
			return Counts{}, errors.New("folded hunk must be mapped")
		}
		hidden[id] = true
	}
	return Counts{Total: len(known), Mapped: len(mapped) - len(hidden), Folded: len(hidden), Unclassified: len(known) - len(mapped), Complete: v.indexComplete}, nil
}

func potentialOracle(path string) bool {
	p := strings.ToLower(path)
	for _, token := range []string{"test", "spec", "fixture", "mask", "golden", "expected", "snapshot", "comparison", "compare-policy"} {
		if strings.Contains(p, token) {
			return true
		}
	}
	return false
}

// Git's default core.quotePath encoding: spaces stay literal, while controls,
// quotes, backslashes and high bytes use C quoting (including octal UTF-8 bytes).
func gitPath(p string) string {
	var b strings.Builder
	quoted := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '\a':
			b.WriteString(`\a`)
			quoted = true
		case '\b':
			b.WriteString(`\b`)
			quoted = true
		case '\t':
			b.WriteString(`\t`)
			quoted = true
		case '\n':
			b.WriteString(`\n`)
			quoted = true
		case '\v':
			b.WriteString(`\v`)
			quoted = true
		case '\f':
			b.WriteString(`\f`)
			quoted = true
		case '\r':
			b.WriteString(`\r`)
			quoted = true
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
			quoted = true
		default:
			if c < 32 || c >= 127 {
				fmt.Fprintf(&b, "\\%03o", c)
				quoted = true
			} else {
				b.WriteByte(c)
			}
		}
	}
	if quoted {
		return `"` + b.String() + `"`
	}
	return b.String()
}

func (v *View) index(diff evidence.Digest) {
	headers := map[string]int{}
	for i, e := range v.entries {
		headers["diff --git "+gitPath("a/"+e.Path)+" "+gitPath("b/"+e.Path)] = i
	}
	entry := -1
	active := -1
	finish := func(end int) {
		if active >= 0 {
			v.hunks[active].End = end
			active = -1
		}
	}
	limited := func(reason string) {
		v.indexComplete = false
		if !slices.Contains(v.limits, reason) {
			v.limits = append(v.limits, reason)
		}
	}
	for start := 0; start < len(v.raw); {
		end := bytes.IndexByte(v.raw[start:], '\n')
		if end < 0 {
			end = len(v.raw)
		} else {
			end += start
		}
		line := v.raw[start:end]
		switch {
		case bytes.HasPrefix(line, []byte("diff --git ")):
			finish(start)
			var ok bool
			entry, ok = headers[string(line)]
			if !ok {
				entry = -1
				limited("unmatched diff path; raw bytes retained")
			}
		case bytes.Equal(line, []byte("GIT binary patch")):
			if entry >= 0 {
				v.entries[entry].Binary = true
			}
		case bytes.HasPrefix(line, []byte("@@ ")):
			finish(start)
			if len(v.hunks) == MaxHunks {
				limited("hunk index limit reached; remaining raw bytes retained")
				return
			}
			p := ""
			if entry >= 0 {
				p = v.entries[entry].Path
			}
			sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", diff, start)))
			v.hunks = append(v.hunks, Hunk{ID: evidence.Digest(fmt.Sprintf("sha256:%x", sum)), Path: p, Start: start, End: len(v.raw)})
			active = len(v.hunks) - 1
		}
		start = end + 1
	}
	finish(len(v.raw))
}
