package rawdiff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
)

const (
	ComputedOrigin         = "computed from captured sources — not Git's patch"
	MaxComputedFileBytes   = 1 << 20
	MaxComputedPathBytes   = 4 << 10
	MaxComputedTotalBytes  = 8 << 20
	MaxComputedLines       = 20000
	MaxComputedWork        = 1_000_000
	MaxComputedOutputBytes = 16 << 20
	computedContextLines   = 3
	computedOutputNotice   = "computed diff output limit reached; remaining paths stay in Changes and captured sources"
	computedOutputReserve  = 256
)

// ComputedDiff is a display-only projection of stored source blobs. Its file
// and hunk offsets are for navigation; it deliberately has no hunk identities
// and is never included in View.Count or View.Hunks.
type ComputedDiff struct {
	Raw         []byte
	Files       []PatchFile
	HunkOffsets []int
	Inventory   []Entry
	Added       int
	Deleted     int
	Limits      []string
}

type diffOpKind uint8

const (
	diffEqual diffOpKind = iota
	diffDelete
	diffAdd
)

type diffOp struct {
	kind diffOpKind
	line []byte
}

type lineRange struct{ start, end int }

// ComputeSourceDiff returns a deterministic Git-style unified presentation for
// one known inventory entry. The line algorithm is pure Go and never invokes
// Git or another process.
func ComputeSourceDiff(entry Entry, base, candidate []byte) ([]byte, []int, int, int, bool, error) {
	if entry.Change == "unknown" {
		return nil, nil, 0, 0, false, nil
	}
	if len(entry.Path) > MaxComputedPathBytes {
		return nil, nil, 0, 0, false, errors.New("too large to diff here — open both sources")
	}
	binary := hasNULPrefix(base) || hasNULPrefix(candidate)
	if binary {
		return binaryPatch(entry), nil, 0, 0, true, nil
	}
	if len(base)+len(candidate) > MaxComputedFileBytes {
		return nil, nil, 0, 0, false, errors.New("too large to diff here — open both sources")
	}
	oldLines, newLines := splitLines(base), splitLines(candidate)
	if len(oldLines)+len(newLines) > MaxComputedLines {
		return nil, nil, 0, 0, false, errors.New("too large to diff here — open both sources")
	}
	ops, err := myers(oldLines, newLines, MaxComputedWork)
	if err != nil {
		return nil, nil, 0, 0, false, errors.New("too large to diff here — open both sources")
	}
	patch, hunks, added, deleted := unifiedPatch(entry, ops)
	return patch, hunks, added, deleted, false, nil
}

// Compute reads only the already captured content-addressed sources. It is
// intended to run as part of browser.Load's background job, never in Update or
// View. A captured pair is deliberately rejected so its raw patch/index stays
// the only source of captured hunk IDs and Count results.
func (v *View) Compute(ctx context.Context) (*ComputedDiff, error) {
	if v == nil || v.s == nil {
		return nil, errors.New("captured source view required")
	}
	if v.available {
		return nil, errors.New("computed diffs are only for pairs without a shared captured patch")
	}
	result := &ComputedDiff{Inventory: v.Inventory(), Limits: v.Limits()}
	var raw bytes.Buffer
	var sourceBytes int
	outputLimited := false
	for i := range result.Inventory {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry := &result.Inventory[i]
		patch := []byte(nil)
		var hunkOffsets []int
		added, deleted := 0, 0
		if len(entry.Path) > MaxComputedPathBytes {
			entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
		} else if entry.Change == "unknown" {
			patch = limitationPatch(*entry, "unknown path; no computed diff")
		} else if sameContentModeChange(*entry) {
			if outputLimited || sourceBytes >= MaxComputedTotalBytes {
				entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
				patch = append(modePatch(*entry), []byte("# too large to diff here — open both sources\n")...)
			} else {
				data, err := readEntrySource(v, entry.Base)
				if err != nil {
					entry.Limits = appendUnique(entry.Limits, "captured source unavailable; no computed diff")
				} else {
					pairBytes := 2 * len(data)
					sourceBytes += pairBytes
					entry.Binary = hasNULPrefix(data)
					if pairBytes > MaxComputedFileBytes || sourceBytes > MaxComputedTotalBytes {
						entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
					}
					if sourceBytes > MaxComputedTotalBytes {
						outputLimited = true
					}
				}
				if entry.Binary {
					patch = binaryPatch(*entry)
				} else {
					patch = modePatch(*entry)
				}
				if slices.Contains(entry.Limits, "too large to diff here — open both sources") {
					patch = append(patch, []byte("# too large to diff here — open both sources\n")...)
				}
			}
		} else if outputLimited || sourceBytes >= MaxComputedTotalBytes {
			entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
			patch = limitationPatch(*entry, "too large to diff here — open both sources")
			outputLimited = outputLimited || raw.Len()+len(patch) > MaxComputedOutputBytes-computedOutputReserve
		} else {
			base, baseErr := readEntrySource(v, entry.Base)
			candidate, candidateErr := readEntrySource(v, entry.Candidate)
			if baseErr != nil || candidateErr != nil {
				entry.Limits = appendUnique(entry.Limits, "captured source unavailable; no computed diff")
				patch = limitationPatch(*entry, "captured source unavailable; no computed diff")
			} else {
				sourceBytes += len(base) + len(candidate)
				entry.Binary = hasNULPrefix(base) || hasNULPrefix(candidate)
				if entry.Binary {
					patch = binaryPatch(*entry)
					if len(base)+len(candidate) > MaxComputedFileBytes || sourceBytes > MaxComputedTotalBytes {
						entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
						patch = append(patch, []byte("# too large to diff here — open both sources\n")...)
					}
					if sourceBytes > MaxComputedTotalBytes {
						outputLimited = true
					}
				} else if sourceBytes > MaxComputedTotalBytes {
					entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
					patch = limitationPatch(*entry, "too large to diff here — open both sources")
					outputLimited = true
				} else {
					patch, hunkOffsets, added, deleted, entry.Binary, baseErr = ComputeSourceDiff(*entry, base, candidate)
					if baseErr != nil {
						entry.Limits = appendUnique(entry.Limits, baseErr.Error())
						patch = limitationPatch(*entry, baseErr.Error())
					}
				}
			}
		}
		if patch == nil {
			continue
		}
		if raw.Len()+len(patch) > MaxComputedOutputBytes-computedOutputReserve {
			entry.Limits = appendUnique(entry.Limits, "too large to diff here — open both sources")
			patch = limitationPatch(*entry, "too large to diff here — open both sources")
			outputLimited = true
		}
		if raw.Len()+len(patch) > MaxComputedOutputBytes-computedOutputReserve {
			outputLimited = true
			continue
		}
		start := raw.Len()
		raw.Write(patch)
		end := raw.Len()
		result.Files = append(result.Files, PatchFile{Path: entry.Path, Start: start, End: end})
		for _, offset := range hunkOffsets {
			result.HunkOffsets = append(result.HunkOffsets, start+offset)
		}
		result.Added += added
		result.Deleted += deleted
	}
	if outputLimited {
		result.Limits = appendUnique(result.Limits, computedOutputNotice)
		if raw.Len()+len(computedOutputNotice)+1 <= MaxComputedOutputBytes {
			raw.WriteString(computedOutputNotice + "\n")
		}
	}
	result.Raw = raw.Bytes()
	return result, nil
}

func readEntrySource(v *View, file *evidence.File) ([]byte, error) {
	if file == nil {
		return nil, nil
	}
	if v.s == nil {
		return nil, ErrContext
	}
	return v.s.ReadBlob(file.Content)
}

func sameContentModeChange(entry Entry) bool {
	return entry.Base != nil && entry.Candidate != nil && entry.Base.Content == entry.Candidate.Content && entry.Base.Mode != entry.Candidate.Mode
}

func hasNULPrefix(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}

func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := make([][]byte, 0, bytes.Count(data, []byte{'\n'})+1)
	for start := 0; start < len(data); {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			end = len(data)
		} else {
			end += start + 1
		}
		lines = append(lines, data[start:end])
		start = end
	}
	return lines
}

// myers retains only the diagonal span visited at each edit distance. The
// explicit work cap also bounds pathological mostly-different files.
func myers(a, b [][]byte, maxWork int) ([]diffOp, error) {
	n, m := len(a), len(b)
	max := n + m
	offset := max + 1
	v := make([]int, 2*max+3)
	v[offset+1] = 0
	trace := make([][]int, 0, min(max+1, 128))
	work := 0
	endDistance := -1
	for d := 0; d <= max; d++ {
		current := make([]int, d+1)
		for k := -d; k <= d; k += 2 {
			work++
			if work > maxWork {
				return nil, errors.New("diff work limit")
			}
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m {
				work++
				if work > maxWork {
					return nil, errors.New("diff work limit")
				}
				if !bytes.Equal(a[x], b[y]) {
					break
				}
				x++
				y++
			}
			v[offset+k] = x
			current[(k+d)/2] = x
			if x >= n && y >= m {
				trace = append(trace, current)
				endDistance = d
				break
			}
		}
		if endDistance >= 0 {
			break
		}
		trace = append(trace, current)
	}
	if endDistance < 0 {
		return nil, errors.New("diff path unavailable")
	}
	ops := make([]diffOp, 0, n+m)
	x, y := n, m
	for d := endDistance; d > 0; d-- {
		previous := trace[d-1]
		k := x - y
		var previousK int
		if k == -d || (k != d && diagonal(previous, d-1, k-1) < diagonal(previous, d-1, k+1)) {
			previousK = k + 1
		} else {
			previousK = k - 1
		}
		previousX := diagonal(previous, d-1, previousK)
		previousY := previousX - previousK
		for x > previousX && y > previousY {
			x--
			y--
			ops = append(ops, diffOp{kind: diffEqual, line: a[x]})
		}
		if x == previousX {
			y--
			ops = append(ops, diffOp{kind: diffAdd, line: b[y]})
		} else {
			x--
			ops = append(ops, diffOp{kind: diffDelete, line: a[x]})
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		ops = append(ops, diffOp{kind: diffEqual, line: a[x]})
	}
	if x != 0 || y != 0 {
		return nil, errors.New("diff path did not reach origin")
	}
	for left, right := 0, len(ops)-1; left < right; left, right = left+1, right-1 {
		ops[left], ops[right] = ops[right], ops[left]
	}
	return ops, nil
}

func diagonal(trace []int, d, k int) int {
	if k < -d || k > d || (k+d)%2 != 0 {
		return -1
	}
	return trace[(k+d)/2]
}

func changedRanges(ops []diffOp) []lineRange {
	var ranges []lineRange
	for i := 0; i < len(ops); {
		if ops[i].kind == diffEqual {
			i++
			continue
		}
		first, last := i, i
		for i+1 < len(ops) && ops[i+1].kind != diffEqual {
			i++
			last = i
		}
		start := max(first-computedContextLines, 0)
		end := min(last+computedContextLines+1, len(ops))
		if len(ranges) > 0 && start <= ranges[len(ranges)-1].end {
			ranges[len(ranges)-1].end = end
		} else {
			ranges = append(ranges, lineRange{start: start, end: end})
		}
		i++
	}
	return ranges
}

func unifiedPatch(entry Entry, ops []diffOp) ([]byte, []int, int, int) {
	var patch bytes.Buffer
	writeDiffHeader(&patch, entry.Path)
	if entry.Change == "added" {
		mode := "100644"
		if entry.Candidate != nil {
			mode = entry.Candidate.Mode
		}
		fmt.Fprintf(&patch, "new file mode %s\n", mode)
	} else if entry.Change == "deleted" {
		mode := "100644"
		if entry.Base != nil {
			mode = entry.Base.Mode
		}
		fmt.Fprintf(&patch, "deleted file mode %s\n", mode)
	} else if entry.Base != nil && entry.Candidate != nil && entry.Base.Mode != entry.Candidate.Mode {
		fmt.Fprintf(&patch, "old mode %s\nnew mode %s\n", entry.Base.Mode, entry.Candidate.Mode)
	}
	oldName, newName := "a/"+entry.Path, "b/"+entry.Path
	if entry.Change == "added" {
		oldName = "/dev/null"
	}
	if entry.Change == "deleted" {
		newName = "/dev/null"
	}
	fmt.Fprintf(&patch, "--- %s\n+++ %s\n", gitPath(oldName), gitPath(newName))
	ranges := changedRanges(ops)
	var hunkOffsets []int
	oldCursor, newCursor, opCursor := 0, 0, 0
	added, deleted := 0, 0
	for _, r := range ranges {
		for opCursor < r.start {
			switch ops[opCursor].kind {
			case diffEqual:
				oldCursor++
				newCursor++
			case diffDelete:
				oldCursor++
			case diffAdd:
				newCursor++
			}
			opCursor++
		}
		oldStart, newStart := oldCursor, newCursor
		oldCount, newCount := 0, 0
		for i := r.start; i < r.end; i++ {
			switch ops[i].kind {
			case diffEqual:
				oldCount++
				newCount++
			case diffDelete:
				oldCount++
				deleted++
			case diffAdd:
				newCount++
				added++
			}
		}
		hunkOffsets = append(hunkOffsets, patch.Len())
		fmt.Fprintf(&patch, "@@ -%d,%d +%d,%d @@\n", unifiedStart(oldStart, oldCount), oldCount, unifiedStart(newStart, newCount), newCount)
		for i := r.start; i < r.end; i++ {
			op := ops[i]
			prefix := byte(' ')
			switch op.kind {
			case diffDelete:
				prefix = '-'
			case diffAdd:
				prefix = '+'
			}
			patch.WriteByte(prefix)
			patch.Write(op.line)
			if len(op.line) == 0 || op.line[len(op.line)-1] != '\n' {
				patch.WriteByte('\n')
				patch.WriteString("\\ No newline at end of file\n")
			}
		}
		oldCursor += oldCount
		newCursor += newCount
		opCursor = r.end
	}
	return patch.Bytes(), hunkOffsets, added, deleted
}

func unifiedStart(consumed, count int) int {
	if count == 0 {
		return consumed
	}
	return consumed + 1
}

func writeDiffHeader(out *bytes.Buffer, path string) {
	fmt.Fprintf(out, "diff --git %s %s\n", gitPath("a/"+path), gitPath("b/"+path))
}

func binaryPatch(entry Entry) []byte {
	var patch bytes.Buffer
	writeDiffHeader(&patch, entry.Path)
	if entry.Change == "added" && entry.Candidate != nil {
		fmt.Fprintf(&patch, "new file mode %s\n", entry.Candidate.Mode)
	} else if entry.Change == "deleted" && entry.Base != nil {
		fmt.Fprintf(&patch, "deleted file mode %s\n", entry.Base.Mode)
	} else if entry.Base != nil && entry.Candidate != nil && entry.Base.Mode != entry.Candidate.Mode {
		fmt.Fprintf(&patch, "old mode %s\nnew mode %s\n", entry.Base.Mode, entry.Candidate.Mode)
	}
	oldName, newName := gitPath("a/"+entry.Path), gitPath("b/"+entry.Path)
	if entry.Change == "added" {
		oldName = "/dev/null"
	}
	if entry.Change == "deleted" {
		newName = "/dev/null"
	}
	fmt.Fprintf(&patch, "Binary files %s and %s differ\n", oldName, newName)
	return patch.Bytes()
}

func modePatch(entry Entry) []byte {
	var patch bytes.Buffer
	writeDiffHeader(&patch, entry.Path)
	fmt.Fprintf(&patch, "old mode %s\nnew mode %s\n", entry.Base.Mode, entry.Candidate.Mode)
	return patch.Bytes()
}

func limitationPatch(entry Entry, reason string) []byte {
	var patch bytes.Buffer
	writeDiffHeader(&patch, entry.Path)
	patch.WriteString("# ")
	patch.WriteString(strings.ReplaceAll(reason, "\n", " "))
	patch.WriteByte('\n')
	return patch.Bytes()
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
