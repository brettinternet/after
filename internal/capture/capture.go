// Package capture freezes bounded local Git content without executing project code.
package capture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/store"
)

const (
	MaxFiles        = 2000
	MaxFileBytes    = 8 << 20
	MaxCaptureBytes = 64 << 20
	MaxAttempts     = 3
)

var (
	ErrBudget       = errors.New("capture budget exceeded")
	ErrInconsistent = errors.New("repository changed during capture; retry when writers are idle")
	errSelection    = errors.New("selected path is not a non-ignored untracked file")
)

type Options struct {
	// Mode defaults to WorkingTree. MergeBase requires Base and Target commit-ish values.
	Mode         evidence.SourceMode
	Base, Target string
	// Exact repository-relative non-ignored untracked paths; no globs or directories.
	IncludeUntracked []string
}
type Result struct {
	Base, Candidate evidence.Snapshot
	Index           *evidence.Snapshot
}
type entry struct {
	Path, Mode, OID  string
	Data             []byte
	Reason, Excluded string
}
type image struct {
	Source                        evidence.SourceMode
	Commit, MergeBase, BaseCommit string
	Unborn                        bool
	Entries                       []entry
}
type scan struct {
	Base, Candidate, Index image
	HasIndex               bool
}
type reader struct {
	ctx   context.Context
	dir   string
	root  *os.Root
	total int
}

// Capture requires a trusted repository root and an already-open private store.
// It publishes records only after two identical full reads, with at most three
// attempts. This detects ordinary overlapping writes, not adversarial ABA edits.
func Capture(ctx context.Context, dir string, s *store.Store, opts Options) (Result, error) {
	return capture(ctx, dir, s, opts, nil)
}
func capture(ctx context.Context, dir string, s *store.Store, opts Options, between func(int)) (Result, error) {
	if opts.Mode == "" {
		opts.Mode = evidence.WorkingTree
	}
	frozen, err := consistentScan(ctx, dir, opts, between)
	if err != nil {
		return Result{}, err
	}
	return persist(ctx, s, frozen, opts)
}

// consistentScan returns the first of two identical full reads, with at most
// three attempts. It writes nothing.
func consistentScan(ctx context.Context, dir string, opts Options, between func(int)) (scan, error) {
	var zero scan
	if opts.Mode == "" {
		opts.Mode = evidence.WorkingTree
	}
	if opts.Mode != evidence.WorkingTree && opts.Mode != evidence.Index && opts.Mode != evidence.MergeBase {
		return zero, errors.New("unsupported capture mode")
	}
	if (opts.Mode == evidence.MergeBase) != (opts.Base != "" && opts.Target != "") || (opts.Mode != evidence.MergeBase && (opts.Base != "" || opts.Target != "")) {
		return zero, errors.New("merge-base capture requires both refs only in merge-base mode")
	}
	if opts.Mode != evidence.WorkingTree && len(opts.IncludeUntracked) > 0 {
		return zero, errors.New("untracked selection requires working-tree mode")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return zero, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return zero, errors.New("cannot open repository root")
	}
	defer root.Close()
	// No upward repository discovery: callers must supply the root, not a subdirectory.
	if _, err := root.Lstat(".git"); err != nil {
		return zero, errors.New("repository root must contain .git")
	}
	r := reader{ctx: ctx, dir: dir, root: root}
	for attempt := 0; attempt < MaxAttempts; attempt++ {
		first, err := r.scan(opts)
		if err != nil {
			return zero, err
		}
		if between != nil {
			between(attempt)
		}
		second, err := r.scan(opts)
		if err != nil {
			return zero, err
		}
		if !reflect.DeepEqual(first, second) {
			continue
		}
		return first, nil
	}
	return zero, ErrInconsistent
}
func (r *reader) run(args ...string) ([]byte, error) { return git(r.ctx, r.dir, nil, args...) }
func (r *reader) resolve(ref string) (string, error) {
	if ref == "" || strings.ContainsRune(ref, 0) {
		return "", errors.New("invalid revision")
	}
	b, err := r.run("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(b))
	if !validOID(oid) {
		return "", errors.New("invalid resolved commit")
	}
	return oid, nil
}
func validOID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func validPath(p string) bool {
	return utf8.ValidString(p) && p != "" && p != "." && path.Clean(p) == p && !path.IsAbs(p) && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\x00\\:")
}
func privatePath(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if strings.EqualFold(part, ".after") || strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}
func (r *reader) scan(o Options) (scan, error) {
	var out scan
	r.total = 0
	for _, key := range []string{"core.sparseCheckout", "extensions.partialClone"} {
		b, _ := r.run("config", "--get", key)
		if v := strings.TrimSpace(string(b)); v != "" && v != "false" {
			return out, errors.New("sparse or partial repositories are unsupported")
		}
	}
	shallow, err := r.run("rev-parse", "--is-shallow-repository")
	if err != nil {
		return out, err
	}
	if strings.TrimSpace(string(shallow)) != "false" {
		return out, errors.New("shallow repositories are unsupported")
	}
	if o.Mode == evidence.MergeBase {
		base, err := r.resolve(o.Base)
		if err != nil {
			return out, err
		}
		target, err := r.resolve(o.Target)
		if err != nil {
			return out, err
		}
		mb, err := r.run("merge-base", "--all", base, target)
		if err != nil {
			return out, err
		}
		common := strings.TrimSpace(string(mb))
		if !validOID(common) {
			return out, errors.New("comparison needs exactly one merge base")
		}
		out.Base, err = r.tree(common)
		if err != nil {
			return out, err
		}
		out.Candidate, err = r.tree(target)
		if err != nil {
			return out, err
		}
		out.Candidate.Source = evidence.MergeBase
		out.Candidate.MergeBase = common
		// Preserve both resolved inputs even if a branch name moves between reads.
		out.Candidate.BaseCommit = base
		return out, nil
	}
	head, err := r.resolve("HEAD")
	unborn := false
	if err != nil {
		// A symbolic HEAD with no branch ref is unborn; a corrupt existing ref is not.
		sym, e := r.run("symbolic-ref", "-q", "HEAD")
		if e != nil {
			return out, err
		}
		ref := strings.TrimSpace(string(sym))
		if !strings.HasPrefix(ref, "refs/heads/") {
			return out, err
		}
		refs, e := r.run("for-each-ref", "--format=%(refname)", ref)
		if e != nil || len(refs) != 0 {
			return out, err
		}
		unborn = true
	}
	if unborn {
		out.Base = image{Source: evidence.Commit, Unborn: true}
	} else {
		out.Base, err = r.tree(head)
		if err != nil {
			return out, err
		}
	}
	raw, err := r.run("ls-files", "--stage", "-z")
	if err != nil {
		return out, err
	}
	entries, err := parseEntries(raw, true)
	if err != nil {
		return out, err
	}
	// Skip-worktree and assume-unchanged flags make ordinary filesystem capture misleading.
	flags, err := r.run("ls-files", "-v", "-z")
	if err != nil {
		return out, err
	}
	for _, line := range bytes.Split(flags, []byte{0}) {
		if len(line) > 0 && (line[0] == 'S' || line[0] >= 'a' && line[0] <= 'z') {
			return out, errors.New("sparse/skip-worktree or assume-unchanged index is unsupported")
		}
	}
	out.Index = image{Source: evidence.Index, Commit: head, Unborn: unborn}
	out.Index.Entries, err = r.objects(append([]entry(nil), entries...))
	if err != nil {
		return out, err
	}
	out.HasIndex = true
	// Private storage must not enumerate its own artifacts or change identities
	// each time a capture is persisted. Git ignores this basename at any depth.
	untracked, err := r.run("ls-files", "--others", "--exclude-standard", "--exclude=.after", "--exclude=.git", "-z")
	if err != nil {
		return out, err
	}
	for _, p := range strings.Split(string(untracked), "\x00") {
		if p == "" {
			continue
		}
		if !validPath(p) {
			return out, errors.New("unsupported path encoding")
		}
		out.Index.Entries = append(out.Index.Entries, entry{Path: p, Excluded: "untracked; not selected"})
	}
	if len(out.Index.Entries) > MaxFiles {
		return out, ErrBudget
	}
	sort.Slice(out.Index.Entries, func(i, j int) bool { return out.Index.Entries[i].Path < out.Index.Entries[j].Path })
	if o.Mode == evidence.Index {
		out.Candidate = out.Index
		return out, nil
	}
	out.Candidate = image{Source: evidence.WorkingTree, Commit: head, Unborn: unborn}
	for _, e := range entries {
		// Never traverse submodules or tracked symlinks, even if replaced locally.
		if e.Mode != "100644" && e.Mode != "100755" {
			e.Reason = "unsupported Git mode " + e.Mode
			out.Candidate.Entries = append(out.Candidate.Entries, e)
			continue
		}
		e, exists, err := r.file(e)
		if err != nil {
			return out, err
		}
		if exists {
			out.Candidate.Entries = append(out.Candidate.Entries, e)
		}
	}
	selected := map[string]bool{}
	for _, p := range o.IncludeUntracked {
		if !validPath(p) || privatePath(p) {
			return out, errors.New("invalid or private untracked selection")
		}
		selected[p] = true
	}
	for _, p := range strings.Split(string(untracked), "\x00") {
		if p == "" {
			continue
		}
		if !validPath(p) {
			return out, errors.New("unsupported path encoding")
		}
		e := entry{Path: p, Excluded: "untracked; not selected"}
		if privatePath(p) {
			e.Excluded = "private AFTER/Git storage"
		} else if selected[p] {
			delete(selected, p)
			var exists bool
			e, exists, err = r.file(entry{Path: p})
			if err != nil {
				return out, err
			}
			if !exists {
				return out, ErrInconsistent
			}
		}
		out.Candidate.Entries = append(out.Candidate.Entries, e)
	}
	if len(selected) != 0 {
		return out, errSelection
	}
	sort.Slice(out.Candidate.Entries, func(i, j int) bool { return out.Candidate.Entries[i].Path < out.Candidate.Entries[j].Path })
	if len(out.Candidate.Entries) > MaxFiles {
		return out, ErrBudget
	}
	return out, nil
}
func parseEntries(raw []byte, index bool) ([]entry, error) {
	var out []entry
	for _, line := range bytes.Split(raw, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		parts := bytes.SplitN(line, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid Git inventory")
		}
		fields := strings.Fields(string(parts[0]))
		if len(fields) != 3 {
			return nil, errors.New("invalid Git inventory")
		}
		p := string(parts[1])
		if !validPath(p) {
			return nil, errors.New("unsupported path encoding")
		}
		if index && fields[2] != "0" {
			return nil, errors.New("unmerged index is unsupported")
		}
		oid := fields[2]
		if index {
			oid = fields[1]
		}
		if !validOID(oid) {
			return nil, errors.New("invalid Git object identity")
		}
		out = append(out, entry{Path: p, Mode: fields[0], OID: oid})
		if len(out) > MaxFiles {
			return nil, ErrBudget
		}
	}
	return out, nil
}
func (r *reader) tree(commit string) (image, error) {
	out := image{Source: evidence.Commit, Commit: commit}
	b, err := r.run("ls-tree", "-r", "-z", commit)
	if err != nil {
		return out, err
	}
	es, err := parseEntries(b, false)
	if err != nil {
		return out, err
	}
	out.Entries, err = r.objects(es)
	return out, err
}

// objects reads every regular blob with one cat-file --batch-check and one
// cat-file --batch per output-sized chunk, instead of two processes per file.
// Only validated OIDs are sent; replies must match in count, order, identity,
// type and exact length, and payloads are framed by size, never split by line.
func (r *reader) objects(es []entry) ([]entry, error) {
	var wanted []int
	var request bytes.Buffer
	for i := range es {
		switch {
		case privatePath(es[i].Path):
			es[i].Excluded = "private AFTER/Git storage"
		case es[i].Mode != "100644" && es[i].Mode != "100755":
			es[i].Reason = "unsupported Git mode " + es[i].Mode
		case !validOID(es[i].OID):
			return es, errors.New("invalid Git object identity")
		default:
			wanted = append(wanted, i)
			request.WriteString(es[i].OID + "\n")
		}
	}
	if len(wanted) == 0 {
		return es, nil
	}
	checked, err := r.batch(request.Bytes(), "--batch-check")
	if err != nil {
		return es, err
	}
	sizes := map[int]int{}
	var read []int
	for _, i := range wanted {
		n, rest, err := objectHeader(checked, es[i].OID)
		if err != nil {
			return es, err
		}
		checked = rest
		if n > MaxFileBytes {
			es[i].Reason = "file exceeds capture byte limit"
			continue
		}
		// Charge the budget before any payload is read.
		if err := r.charge(n); err != nil {
			return es, err
		}
		sizes[i] = n
		read = append(read, i)
	}
	if len(checked) != 0 {
		return es, errors.New("unexpected Git object reply")
	}
	for start := 0; start < len(read); {
		// Each reply is "<oid> blob <size>\n<payload>\n"; keep a chunk's framed
		// output below the hardened runner's output ceiling.
		request.Reset()
		end, framed := start, 0
		for end < len(read) {
			cost := sizes[read[end]] + len(es[read[end]].OID) + 32
			if end > start && framed+cost > maxOutput {
				break
			}
			framed += cost
			request.WriteString(es[read[end]].OID + "\n")
			end++
		}
		out, err := r.batch(request.Bytes(), "--batch")
		if err != nil {
			return es, err
		}
		for _, i := range read[start:end] {
			n, rest, err := objectHeader(out, es[i].OID)
			if err != nil {
				return es, err
			}
			if n != sizes[i] || len(rest) < n+1 || rest[n] != '\n' {
				return es, ErrInconsistent
			}
			es[i].Data, out = rest[:n:n], rest[n+1:]
		}
		if len(out) != 0 {
			return es, errors.New("unexpected Git object reply")
		}
		start = end
	}
	for _, i := range read {
		es[i] = lfsPointer(es[i])
	}
	return es, nil
}
func (r *reader) batch(request []byte, mode string) ([]byte, error) {
	return git(r.ctx, r.dir, request, "cat-file", mode)
}

// objectHeader parses one "<oid> blob <size>" reply line for the expected OID.
func objectHeader(reply []byte, oid string) (int, []byte, error) {
	line, rest, ok := bytes.Cut(reply, []byte{'\n'})
	if !ok {
		return 0, nil, errors.New("truncated Git object reply")
	}
	fields := strings.Split(string(line), " ")
	if len(fields) != 3 || fields[0] != oid || fields[1] != "blob" {
		return 0, nil, errors.New("Git object is missing or not a file")
	}
	n, err := strconv.Atoi(fields[2])
	if err != nil || n < 0 || strconv.Itoa(n) != fields[2] {
		return 0, nil, errors.New("invalid object size")
	}
	return n, rest, nil
}
func (r *reader) file(e entry) (entry, bool, error) {
	if privatePath(e.Path) {
		e.Excluded = "private AFTER/Git storage"
		return e, true, nil
	}
	parts := strings.Split(e.Path, "/")
	for i := 1; i <= len(parts); i++ {
		info, err := r.root.Lstat(strings.Join(parts[:i], "/"))
		if errors.Is(err, os.ErrNotExist) {
			return e, false, nil
		}
		if err != nil {
			return e, false, errors.New("cannot inspect captured path")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			e.Reason = "symlink or symlink parent unsupported"
			return e, true, nil
		}
		if i < len(parts) && !info.IsDir() {
			e.Reason = "non-directory parent unsupported"
			return e, true, nil
		}
	}
	f, err := r.root.OpenFile(e.Path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return e, false, errors.New("cannot safely open captured file")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return e, false, err
	}
	if !before.Mode().IsRegular() {
		e.Reason = "non-regular file unsupported"
		return e, true, nil
	}
	e.Mode = "100644"
	if before.Mode().Perm()&0111 != 0 {
		e.Mode = "100755"
	}
	if before.Size() > MaxFileBytes {
		e.Reason = "file exceeds capture byte limit"
		return e, true, nil
	}
	e.Data, err = io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return e, false, errors.New("cannot read captured file")
	}
	after, err := f.Stat()
	if err != nil {
		return e, false, err
	}
	if len(e.Data) > MaxFileBytes || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return e, false, ErrInconsistent
	}
	e, err = r.account(e)
	return e, true, err
}
func (r *reader) account(e entry) (entry, error) {
	if err := r.charge(len(e.Data)); err != nil {
		return e, err
	}
	return lfsPointer(e), nil
}
func (r *reader) charge(n int) error {
	r.total += n
	if r.total > MaxCaptureBytes {
		return ErrBudget
	}
	return nil
}
func lfsPointer(e entry) entry {
	if bytes.HasPrefix(e.Data, []byte("version https://git-lfs.github.com/spec/v1\n")) {
		e.Data = nil
		e.Reason = "LFS pointer; payload not captured"
	}
	return e
}

func persist(ctx context.Context, s *store.Store, scan scan, opts Options) (Result, error) {
	var out Result
	// Each snapshot's Diff is the patch from the captured base to that snapshot.
	diff := func(candidate image) (evidence.Artifact, error) {
		patch, err := makeDiff(ctx, scan.Base, candidate)
		if err != nil {
			return evidence.Artifact{}, err
		}
		return s.PutArtifact(patch, "git-diff", store.MaxBlobBytes)
	}
	a, err := diff(scan.Candidate)
	if err != nil {
		return out, err
	}
	save := func(im image, index evidence.Digest, a evidence.Artifact) (evidence.Snapshot, error) {
		snap, err := manifest(im, index, a, func(data []byte) (evidence.Artifact, error) {
			return s.PutArtifact(data, "source", store.MaxBlobBytes)
		})
		if err != nil {
			return snap, err
		}
		return store.Put(s, snap)
	}
	out.Base, err = save(scan.Base, "", a)
	if err != nil {
		return out, err
	}
	var index evidence.Digest
	if scan.HasIndex {
		indexDiff := a
		if scan.Candidate.Source == evidence.WorkingTree {
			// Never label the unstaged patch as the staged one.
			if indexDiff, err = diff(scan.Index); err != nil {
				return out, err
			}
		}
		snap, e := save(scan.Index, "", indexDiff)
		if e != nil {
			return out, e
		}
		out.Index = &snap
		index = snap.ID
	}
	if scan.Candidate.Source != evidence.WorkingTree {
		index = ""
	}
	out.Candidate, err = save(scan.Candidate, index, a)
	if err != nil {
		return out, err
	}
	var indexID evidence.Digest
	if out.Index != nil {
		indexID = out.Index.ID
	}
	selected := append([]string{}, opts.IncludeUntracked...)
	sort.Strings(selected)
	unique := selected[:0]
	for _, path := range selected {
		if len(unique) == 0 || unique[len(unique)-1] != path {
			unique = append(unique, path)
		}
	}
	_, err = store.Put(s, evidence.Capture{
		SchemaVersion:     evidence.SchemaVersion,
		CapturedAt:        time.Now().UTC(),
		Mode:              opts.Mode,
		Base:              out.Base.ID,
		Candidate:         out.Candidate.ID,
		Index:             indexID,
		SelectedUntracked: unique,
	})
	return out, err
}

// manifest describes a frozen image; content records one file's bytes.
func manifest(im image, index evidence.Digest, diff evidence.Artifact, content func([]byte) (evidence.Artifact, error)) (evidence.Snapshot, error) {
	snap := evidence.Snapshot{SchemaVersion: evidence.SchemaVersion, Source: im.Source, Commit: im.Commit, Unborn: im.Unborn, Completeness: evidence.Complete, Diff: diff.Content, IndexSnapshot: index, Limits: []string{"two matching reads; not an atomic filesystem snapshot", "diff includes captured regular files only; inspect excluded and unsupported inventory"}}
	if im.Source == evidence.MergeBase {
		snap.MergeBase = im.MergeBase
		snap.BaseCommit = im.BaseCommit
	}
	if diff.Completeness != evidence.Complete {
		snap.Completeness = evidence.Incomplete
		snap.Limits = append(snap.Limits, "diff redacted or truncated")
	}
	for _, e := range im.Entries {
		switch {
		case e.Excluded != "":
			snap.Excluded = append(snap.Excluded, evidence.Limitation{Path: e.Path, Reason: e.Excluded})
		case e.Reason != "":
			snap.Unsupported = append(snap.Unsupported, evidence.Limitation{Path: e.Path, Reason: e.Reason})
			snap.Completeness = evidence.Incomplete
		default:
			blob, err := content(e.Data)
			if err != nil {
				return snap, err
			}
			if blob.Completeness != evidence.Complete {
				snap.Completeness = evidence.Incomplete
				snap.Limits = append(snap.Limits, fmt.Sprintf("source redacted or truncated: %s", e.Path))
			}
			snap.Files = append(snap.Files, evidence.File{Path: e.Path, Mode: e.Mode, Content: blob.Content})
		}
	}
	return snap, nil
}

// unstored identifies bytes the way the store would without retaining them.
func unstored(data []byte) (evidence.Artifact, error) {
	sum := sha256.Sum256(data)
	return evidence.Artifact{Content: evidence.Digest("sha256:" + hex.EncodeToString(sum[:])), Bytes: int64(len(data)), Completeness: evidence.Complete}, nil
}

// Live is the checkout's current change as Capture would read it. Its
// snapshots have no record IDs, and file and diff identities are SHA-256
// digests of the unredacted bytes read; nothing is stored.
type Live struct {
	Base, Candidate evidence.Snapshot
	Patch           []byte
}

// ReadLive performs Capture's consistent read and patch generation without
// writing evidence. Only private temporary files hold bytes for Git's diff.
func ReadLive(ctx context.Context, dir string, opts Options) (Live, error) {
	if opts.Mode == "" {
		opts.Mode = evidence.WorkingTree
	}
	frozen, err := consistentScan(ctx, dir, opts, nil)
	if err != nil {
		return Live{}, err
	}
	patch, err := makeDiff(ctx, frozen.Base, frozen.Candidate)
	if err != nil {
		return Live{}, err
	}
	diff, _ := unstored(patch)
	out := Live{Patch: patch}
	if out.Base, err = manifest(frozen.Base, "", diff, unstored); err != nil {
		return Live{}, err
	}
	out.Candidate, err = manifest(frozen.Candidate, "", diff, unstored)
	return out, err
}

// Unchanged reports whether the checkout still holds a stored capture's
// content in its recorded scope: the HEAD commit and index, plus, for a
// working-tree capture, tracked files, the recorded untracked selection and the
// excluded untracked inventory. It reads like Capture and writes nothing. Only
// complete snapshots identify original bytes, so callers must not pass
// incomplete ones. Merge-base captures compare commits, not checkout state.
func Unchanged(ctx context.Context, dir string, record evidence.Capture, base, candidate evidence.Snapshot, index *evidence.Snapshot) (bool, error) {
	if record.Mode != evidence.WorkingTree && record.Mode != evidence.Index {
		return false, errors.New("only working-tree and index captures describe checkout state")
	}
	frozen, err := consistentScan(ctx, dir, Options{Mode: record.Mode, IncludeUntracked: record.SelectedUntracked}, nil)
	if errors.Is(err, errSelection) {
		return false, nil // a selected untracked file was removed or added to Git
	}
	if err != nil {
		return false, err
	}
	compare := []struct {
		image  image
		stored evidence.Snapshot
	}{{frozen.Base, base}, {frozen.Candidate, candidate}}
	if index != nil {
		compare = append(compare, struct {
			image  image
			stored evidence.Snapshot
		}{frozen.Index, *index})
	}
	for _, item := range compare {
		live, err := manifest(item.image, "", evidence.Artifact{Completeness: evidence.Complete}, unstored)
		if err != nil {
			return false, err
		}
		if live.Source != item.stored.Source || live.Commit != item.stored.Commit || live.Unborn != item.stored.Unborn ||
			!slices.Equal(live.Files, item.stored.Files) || !slices.Equal(live.Excluded, item.stored.Excluded) || !slices.Equal(live.Unsupported, item.stored.Unsupported) {
			return false, nil
		}
	}
	return true, nil
}

// Diff only frozen bytes in a private Git object database. No repository config,
// attributes, hooks, filters or working-tree contents participate.
func makeDiff(ctx context.Context, base, candidate image) ([]byte, error) {
	dir, err := os.MkdirTemp("", "after-capture-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if _, err = git(ctx, dir, nil, "init", "--quiet", "--template=", dir); err != nil {
		return nil, err
	}
	// Write each distinct frozen payload once under a generated name and hash
	// them all in one process. Checkout paths never reach hash-object.
	payloads := filepath.Join(dir, "payloads")
	if err := os.Mkdir(payloads, 0o700); err != nil {
		return nil, err
	}
	names := map[[sha256.Size]byte]int{}
	var paths bytes.Buffer
	for _, im := range []image{base, candidate} {
		for _, e := range im.Entries {
			sum := sha256.Sum256(e.Data)
			if _, ok := names[sum]; ok || e.Excluded != "" || e.Reason != "" {
				continue
			}
			name := "payloads/" + strconv.Itoa(len(names))
			if err := os.WriteFile(filepath.Join(dir, name), e.Data, 0o600); err != nil {
				return nil, err
			}
			names[sum] = len(names)
			paths.WriteString(name + "\n")
		}
	}
	oids := make([]string, len(names))
	if len(names) > 0 {
		hashed, err := git(ctx, dir, paths.Bytes(), "hash-object", "-w", "--no-filters", "--stdin-paths")
		if err != nil {
			return nil, err
		}
		lines := strings.Split(strings.TrimSuffix(string(hashed), "\n"), "\n")
		if len(lines) != len(oids) {
			return nil, errors.New("unexpected Git object reply")
		}
		for i, oid := range lines {
			if !validOID(oid) {
				return nil, errors.New("invalid Git object identity")
			}
			oids[i] = oid
		}
	}
	tree := func(im image) (string, error) {
		if _, err := git(ctx, dir, nil, "read-tree", "--empty"); err != nil {
			return "", err
		}
		var index bytes.Buffer
		for _, e := range im.Entries {
			if e.Excluded != "" || e.Reason != "" {
				continue
			}
			fmt.Fprintf(&index, "%s %s\t%s%c", e.Mode, oids[names[sha256.Sum256(e.Data)]], e.Path, 0)
		}
		if _, err := git(ctx, dir, index.Bytes(), "update-index", "-z", "--index-info"); err != nil {
			return "", err
		}
		id, err := git(ctx, dir, nil, "write-tree")
		return strings.TrimSpace(string(id)), err
	}
	b, err := tree(base)
	if err != nil {
		return nil, err
	}
	c, err := tree(candidate)
	if err != nil {
		return nil, err
	}
	return git(ctx, dir, nil, "diff-tree", "-p", "--binary", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-color", b, c, "--")
}
