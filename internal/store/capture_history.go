package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/evidence"
)

const (
	maxCaptureHistoryScan  = 512
	maxCaptureHistoryBytes = 16 << 20
	maxCaptureHistoryShown = 8
)

// CaptureSummary is the small display-safe portion of an immutable capture record.
type CaptureSummary struct {
	ID                evidence.Digest
	CapturedAt        time.Time
	Mode              evidence.SourceMode
	Base              evidence.Digest
	Candidate         evidence.Digest
	Index             evidence.Digest
	SelectedUntracked int
}

// CaptureHistory reports whether the bounded metadata lookup omitted possible records.
type CaptureHistory struct {
	Records []CaptureSummary
	Limited bool
	More    bool
}

// CapturesForSnapshot scans a bounded number of immutable capture records. A
// limited result is partial and must not be presented as complete history.
func CapturesForSnapshot(s *Store, id evidence.Digest) (CaptureHistory, error) {
	if _, err := key("snapshot", string(id)); err != nil {
		return CaptureHistory{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return CaptureHistory{}, ErrReadOnly
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return CaptureHistory{}, err
	}
	entries, err := directory.ReadDir(MaxEntries + 1)
	closeErr := directory.Close()
	if err != nil {
		return CaptureHistory{}, err
	}
	if closeErr != nil {
		return CaptureHistory{}, closeErr
	}
	history := CaptureHistory{}
	if len(entries) > MaxEntries {
		history.Limited = true
		entries = entries[:MaxEntries]
	}
	var scanned, totalBytes int
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "capture-") {
			continue
		}
		if scanned == maxCaptureHistoryScan {
			history.Limited = true
			break
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > evidence.MaxRecordBytes {
			return CaptureHistory{}, fmt.Errorf("%w: invalid capture record file", ErrCorrupt)
		}
		if int64(totalBytes)+info.Size() > maxCaptureHistoryBytes {
			history.Limited = true
			break
		}
		rawID := strings.TrimPrefix(name, "capture-")
		if len(rawID) != 64 {
			return CaptureHistory{}, fmt.Errorf("%w: invalid capture record name", ErrCorrupt)
		}
		recordID := evidence.Digest("sha256:" + rawID)
		record, err := get[evidence.Capture](s, recordID)
		if err != nil {
			return CaptureHistory{}, fmt.Errorf("%w: invalid capture record", ErrCorrupt)
		}
		scanned++
		totalBytes += int(info.Size())
		if record.Base == id || record.Candidate == id || record.Index == id {
			history.Records = append(history.Records, CaptureSummary{
				ID: record.ID, CapturedAt: record.CapturedAt, Mode: record.Mode,
				Base: record.Base, Candidate: record.Candidate, Index: record.Index,
				SelectedUntracked: len(record.SelectedUntracked),
			})
		}
	}
	sort.Slice(history.Records, func(i, j int) bool {
		if history.Records[i].CapturedAt.Equal(history.Records[j].CapturedAt) {
			return history.Records[i].ID > history.Records[j].ID
		}
		return history.Records[i].CapturedAt.After(history.Records[j].CapturedAt)
	})
	if len(history.Records) > maxCaptureHistoryShown {
		history.More = true
		history.Records = history.Records[:maxCaptureHistoryShown]
	}
	return history, nil
}
