package review

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

const maxPinScan = 512
const maxPinScanBytes = 16 << 20

// lineageKey retains every immutable basis field and exact event. Decision is
// derived from the last event, so only ID and Decision are omitted. With the
// final array bracket removed, extension is exactly key + comma + more events.
func lineageKey(p evidence.Pin) string {
	history := p.History
	p.ID, p.Decision, p.History = "", "", nil
	basis, _ := json.Marshal(p)
	events, _ := json.Marshal(history)
	return string(basis) + "/" + strings.TrimSuffix(string(events), "]")
}

// Heads computes all terminal revisions on each call, including every fork.
// It never chooses a winner by timestamp or persists a mutable pointer. A
// bounded scan fails closed rather than claiming an incomplete set is current.
func Heads(s *store.Store) ([]evidence.Pin, error) {
	entries, err := s.List("pin")
	if err != nil {
		return nil, err
	}
	if len(entries) > maxPinScan {
		return nil, store.ErrLimit
	}
	type revision struct {
		pin evidence.Pin
		key string
	}
	revisions := make([]revision, 0, len(entries))
	var bytes int64
	for _, entry := range entries {
		bytes += entry.Size
		if bytes > maxPinScanBytes {
			return nil, store.ErrLimit
		}
		p, err := store.Get[evidence.Pin](s, entry.ID)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision{p, lineageKey(p)})
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].key < revisions[j].key })
	heads := []evidence.Pin{}
	for i, revision := range revisions {
		if i+1 < len(revisions) && strings.HasPrefix(revisions[i+1].key, revision.key+",") {
			continue
		}
		heads = append(heads, revision.pin)
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].ID < heads[j].ID })
	return heads, nil
}

// NewerHeads returns only strict descendants of this exact revision. Explicit
// inspection and mutations remain bound to the requested revision, not a head.
func NewerHeads(s *store.Store, p evidence.Pin) ([]evidence.Digest, error) {
	heads, err := Heads(s)
	if err != nil {
		return nil, err
	}
	prefix := lineageKey(p) + ","
	ids := []evidence.Digest{}
	for _, head := range heads {
		if strings.HasPrefix(lineageKey(head), prefix) {
			ids = append(ids, head.ID)
		}
	}
	return ids, nil
}
