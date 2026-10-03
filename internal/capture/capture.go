// Package capture freezes bounded local Git content without executing project code.
package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
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
	var zero Result
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
		return persist(ctx, s, first)
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
	for _, e := range entries {
		e, err = r.object(e)
		if err != nil {
			return out, err
		}
		out.Index.Entries = append(out.Index.Entries, e)
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
		return out, errors.New("selected path is not a non-ignored untracked file")
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
	for _, e := range es {
		e, err = r.object(e)
		if err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}
func (r *reader) object(e entry) (entry, error) {
	if privatePath(e.Path) {
		e.Excluded = "private AFTER/Git storage"
		return e, nil
	}
	if e.Mode != "100644" && e.Mode != "100755" {
		e.Reason = "unsupported Git mode " + e.Mode
		return e, nil
	}
	size, err := r.run("cat-file", "-s", e.OID)
	if err != nil {
		return e, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(size)), 10, 64)
	if err != nil || n < 0 {
		return e, errors.New("invalid object size")
	}
	if n > MaxFileBytes {
		e.Reason = "file exceeds capture byte limit"
		return e, nil
	}
	e.Data, err = r.run("cat-file", "blob", e.OID)
	if err != nil {
		return e, err
	}
	if int64(len(e.Data)) != n {
		return e, ErrInconsistent
	}
	return r.account(e)
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
	r.total += len(e.Data)
	if r.total > MaxCaptureBytes {
		return e, ErrBudget
	}
	if bytes.HasPrefix(e.Data, []byte("version https://git-lfs.github.com/spec/v1\n")) {
		e.Data = nil
		e.Reason = "LFS pointer; payload not captured"
	}
	return e, nil
}

func persist(ctx context.Context, s *store.Store, scan scan) (Result, error) {
	var out Result
	diff, err := makeDiff(ctx, scan.Base, scan.Candidate)
	if err != nil {
		return out, err
	}
	a, err := s.PutArtifact(diff, "git-diff", store.MaxBlobBytes)
	if err != nil {
		return out, err
	}
	save := func(im image, index evidence.Digest) (evidence.Snapshot, error) {
		snap := evidence.Snapshot{SchemaVersion: evidence.SchemaVersion, Source: im.Source, Commit: im.Commit, Unborn: im.Unborn, Completeness: evidence.Complete, Diff: a.Content, IndexSnapshot: index, Limits: []string{"two matching reads; not an atomic filesystem snapshot", "diff includes captured regular files only; inspect excluded and unsupported inventory"}}
		if im.Source == evidence.MergeBase {
			snap.MergeBase = im.MergeBase
			snap.BaseCommit = im.BaseCommit
		}
		if a.Completeness != evidence.Complete {
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
				blob, err := s.PutArtifact(e.Data, "source", store.MaxBlobBytes)
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
		return store.Put(s, snap)
	}
	out.Base, err = save(scan.Base, "")
	if err != nil {
		return out, err
	}
	var index evidence.Digest
	if scan.HasIndex {
		snap, e := save(scan.Index, "")
		if e != nil {
			return out, e
		}
		out.Index = &snap
		index = snap.ID
	}
	if scan.Candidate.Source != evidence.WorkingTree {
		index = ""
	}
	out.Candidate, err = save(scan.Candidate, index)
	return out, err
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
	tree := func(im image) (string, error) {
		if _, err := git(ctx, dir, nil, "read-tree", "--empty"); err != nil {
			return "", err
		}
		var index bytes.Buffer
		for _, e := range im.Entries {
			if e.Excluded != "" || e.Reason != "" {
				continue
			}
			oid, err := git(ctx, dir, e.Data, "hash-object", "-w", "--stdin", "--no-filters")
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&index, "%s %s\t%s%c", e.Mode, strings.TrimSpace(string(oid)), e.Path, 0)
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
