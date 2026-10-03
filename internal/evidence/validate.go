package evidence

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

func oneOf[T comparable](value T, allowed ...T) bool { return slices.Contains(allowed, value) }

func digest(d Digest) bool {
	s := string(d)
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}

func digests(ds ...Digest) bool {
	for _, d := range ds {
		if !digest(d) {
			return false
		}
	}
	return true
}

func header(version int, id Digest) error {
	if version != SchemaVersion {
		return fmt.Errorf("unsupported schema version: %d", version)
	}
	if !digest(id) {
		return errors.New("invalid record identity")
	}
	return nil
}

func relativePath(p string) bool {
	return p != "" && p != "." && !strings.ContainsAny(p, "\\\x00") && !strings.Contains(p, ":") && !path.IsAbs(p) && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}

func commitID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

func (s Snapshot) Validate() error {
	if err := header(s.SchemaVersion, s.ID); err != nil {
		return err
	}
	if !oneOf(s.Source, Commit, WorkingTree, Index, MergeBase) || (!s.Unborn && !commitID(s.Commit)) || (s.Unborn && (s.Commit != "" || s.Source == MergeBase)) || !digest(s.Diff) {
		return errors.New("snapshot requires source, resolved commit and diff identity")
	}
	if (s.Source == MergeBase && (!commitID(s.MergeBase) || (s.BaseCommit != "" && !commitID(s.BaseCommit)))) || (s.Source != MergeBase && (s.MergeBase != "" || s.BaseCommit != "")) {
		return errors.New("merge-base identity does not match source mode")
	}
	if s.IndexSnapshot != "" && (s.Source != WorkingTree || !digest(s.IndexSnapshot)) {
		return errors.New("index association requires working-tree source and digest")
	}
	if !oneOf(s.Completeness, Complete, Incomplete) || (s.Completeness == Complete && len(s.Unsupported) > 0) || (s.Completeness == Incomplete && len(s.Limits) == 0 && len(s.Unsupported) == 0) {
		return errors.New("invalid capture completeness or missing limitation")
	}
	seen := map[string]bool{}
	for _, f := range s.Files {
		if !relativePath(f.Path) || seen[f.Path] || !digest(f.Content) || !oneOf(f.Mode, "100644", "100755") {
			return errors.New("invalid or duplicate snapshot file")
		}
		seen[f.Path] = true
	}
	for _, entries := range [][]Limitation{s.Excluded, s.Unsupported} {
		for _, entry := range entries {
			if !relativePath(entry.Path) || seen[entry.Path] || strings.TrimSpace(entry.Reason) == "" {
				return errors.New("invalid or duplicate inventory limitation")
			}
			seen[entry.Path] = true
		}
	}
	return nil
}

func (s Scenario) Validate() error {
	if err := header(s.SchemaVersion, s.ID); err != nil {
		return err
	}
	if !digests(s.Input, s.Driver, s.Observer, s.Rules) || strings.TrimSpace(s.Boundary) == "" || strings.TrimSpace(s.Author) == "" || len(s.Limits) == 0 {
		return errors.New("scenario requires frozen bindings, boundary, author and limits")
	}
	return nil
}

func (s EvidenceState) Validate() error {
	if !oneOf(s.Applicability, Unknown, Current, Stale) || !oneOf(s.Execution, NotRun, Completed, Failed, Cancelled) || !oneOf(s.Comparison, NotCompared, Equal, Different, Incomparable, Unstable) || !oneOf(s.Report, NoReport, ReportPass, ReportFail, ReportSkip) {
		return errors.New("unknown evidence state")
	}
	switch s.Producer {
	case Importer:
		if s.Kind != Reported || s.Applicability != Unknown || s.Execution != NotRun || s.Comparison != NotCompared {
			return errors.New("imported reports cannot establish observed/current evidence or AFTER execution")
		}
	case Runner:
		if s.Report != NoReport || !oneOf(s.Kind, Observed, NoEvidence) || (s.Kind == Observed && s.Execution != Completed) || (s.Kind == NoEvidence && s.Execution == Completed) {
			return errors.New("observations require completed runner execution, not report status")
		}
		if s.Kind == NoEvidence && (s.Applicability == Current || !oneOf(s.Comparison, NotCompared, Incomparable)) {
			return errors.New("absent observations cannot be current or compared")
		}
	default:
		return errors.New("unknown evidence producer")
	}
	return nil
}

func environment(e *Environment) bool {
	if e == nil || !digests(e.Environment, e.Toolchain, e.Dependencies) || len(e.Argv) == 0 || e.Argv[0] == "" {
		return false
	}
	for _, arg := range e.Argv {
		if strings.ContainsRune(arg, 0) {
			return false
		}
	}
	return true
}

func (r Receipt) Validate() error {
	if err := header(r.SchemaVersion, r.ID); err != nil {
		return err
	}
	if err := r.State.Validate(); err != nil {
		return err
	}
	if !digest(r.Snapshots.Candidate) || (r.Snapshots.Base != "" && !digest(r.Snapshots.Base)) {
		return errors.New("receipt requires snapshot identity")
	}
	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) {
		return errors.New("receipt requires ordered timestamps")
	}
	if !oneOf(r.Completeness, Complete, Incomplete) || len(r.Limits) == 0 {
		return errors.New("receipt requires completeness and scoped limits")
	}
	if r.State.Producer == Runner {
		b := r.Bindings
		if !digest(r.Snapshots.Base) || b == nil || !digests(b.Scenario, b.Input, b.Driver, b.Observer, b.Rules) || !environment(r.BaseEnvironment) || !environment(r.CandidateEnvironment) || !digest(r.Authorization) {
			return errors.New("runner receipt requires both snapshots, frozen bindings, environments, argv and authorization")
		}
	} else if r.Bindings != nil || r.BaseEnvironment != nil || r.CandidateEnvironment != nil || r.Authorization != "" {
		return errors.New("import cannot assert runner bindings or authorization")
	}
	if (r.Redacted && r.RedactionPolicy != "literal-v1") || (r.RedactionPolicy != "" && r.RedactionPolicy != "literal-v1") {
		return errors.New("invalid receipt redaction policy")
	}
	complete := r.Completeness == Complete && len(r.Artifacts) > 0 && !r.Redacted
	for _, a := range r.Artifacts {
		if !digest(a.Content) || strings.TrimSpace(a.Channel) == "" || a.Bytes < 0 || a.MaxBytes <= 0 || a.Bytes > a.MaxBytes || !oneOf(a.Completeness, Complete, Incomplete) {
			return errors.New("invalid artifact identity, channel, size or completeness")
		}
		if (a.Redacted && a.RedactionPolicy != "literal-v1") || (a.RedactionPolicy != "" && a.RedactionPolicy != "literal-v1") {
			return errors.New("invalid artifact redaction policy")
		}
		if a.Redacted || a.Truncated || a.Completeness != Complete {
			complete = false
		}
	}
	if r.Completeness == Complete && !complete {
		return errors.New("complete receipt requires complete, unredacted, untruncated artifacts")
	}
	if r.State.Kind == NoEvidence && r.Completeness == Complete {
		return errors.New("absent evidence cannot be complete")
	}
	if oneOf(r.State.Comparison, Equal, Different, Unstable) && !complete {
		return errors.New("conclusive comparison requires complete observations")
	}
	return nil
}

func (p Pin) Validate() error {
	if err := header(p.SchemaVersion, p.ID); err != nil {
		return err
	}
	if !digests(p.Scenario, p.BasisReceipt, p.BasisSnapshots.Base, p.BasisSnapshots.Candidate) || strings.TrimSpace(p.Expectation) == "" || len(p.History) == 0 {
		return errors.New("pin requires scenario, expectation, receipt, snapshots and history")
	}
	for i, event := range p.History {
		if !oneOf(event.Decision, Pinned, Accepted, Reopened) || event.At.IsZero() || strings.TrimSpace(event.Reason) == "" || (i > 0 && event.At.Before(p.History[i-1].At)) {
			return errors.New("invalid pin history")
		}
	}
	if p.History[0].Decision != Pinned || p.Decision != p.History[len(p.History)-1].Decision {
		return errors.New("pin decision must follow its history")
	}
	return nil
}
