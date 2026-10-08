package store

import (
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
)

// Entry identifies an immutable object, not verified content. Call Get or
// ReadBlob before using it. Size permits callers to bound aggregate reads.
type Entry struct {
	ID   evidence.Digest
	Kind string
	Size int64
}

// List returns only requested storage kinds, in stable ID/kind order. It fails
// rather than returning a partial namespace that could make a prefix look unique.
func (s *Store) List(kinds ...string) ([]Entry, error) {
	allowed := map[string]bool{}
	for _, kind := range kinds {
		switch kind {
		case "snapshot", "capture", "scenario", "receipt", "comparison", "pin", "blob", "artifact", "plan":
			allowed[kind] = true
		default:
			return nil, errors.New("unknown store kind")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil, ErrReadOnly
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(MaxEntries + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > MaxEntries {
		return nil, ErrLimit
	}
	result := []Entry{}
	var total int64
	for _, entry := range entries {
		kind, hex, ok := strings.Cut(entry.Name(), "-")
		if !ok || !allowed[kind] {
			continue
		}
		id := evidence.Digest("sha256:" + hex)
		if _, err := key(kind, string(id)); err != nil {
			return nil, ErrCorrupt
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() < 0 {
			return nil, ErrCorrupt
		}
		limit := int64(evidence.MaxRecordBytes)
		switch kind {
		case "blob":
			limit = MaxBlobBytes
		case "plan":
			limit = MaxPlanBytes
		}
		if info.Size() > limit {
			return nil, ErrLimit
		}
		total += info.Size()
		if total > MaxStoreBytes {
			return nil, ErrLimit
		}
		result = append(result, Entry{ID: id, Kind: kind, Size: info.Size()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID == result[j].ID {
			return result[i].Kind < result[j].Kind
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
