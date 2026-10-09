package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
	"github.com/urfave/cli/v2"
)

const defaultLogLimit = 20
const maxLogLimit = store.MaxEntries

type nextCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func next(command, description string) nextCommand {
	return nextCommand{Command: command, Description: description}
}

var fullIDPattern = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

// shortenCommandIDs replaces full stored IDs in a readable suggestion with
// their shortest unique prefix of at least eight hex characters. --approve keeps
// the full digest because consent never accepts a prefix. JSON keeps full IDs.
func shortenCommandIDs(project, command string) string {
	if project == "" || !fullIDPattern.MatchString(command) {
		return command
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		return command
	}
	defer s.Close()
	entries, err := s.List("snapshot", "capture", "scenario", "receipt", "comparison", "pin", "blob", "artifact", "plan")
	if err != nil {
		return command
	}
	fields := strings.Split(command, " ")
	for i, field := range fields {
		if !fullIDPattern.MatchString(field) || len(field) != len("sha256:")+64 || (i > 0 && fields[i-1] == "--approve") {
			continue
		}
		fields[i] = uniquePrefix(entries, evidence.Digest(field))
	}
	return strings.Join(fields, " ")
}

// uniquePrefix compares against every stored ID, regardless of kind, so the
// prefix stays unambiguous for any argument position that accepts it.
func uniquePrefix(entries []store.Entry, id evidence.Digest) string {
	hex := strings.TrimPrefix(string(id), "sha256:")
	length := 8
	for _, entry := range entries {
		other := strings.TrimPrefix(string(entry.ID), "sha256:")
		if other == hex {
			continue
		}
		shared := 0
		for shared < len(hex) && shared < len(other) && hex[shared] == other[shared] {
			shared++
		}
		length = max(length, shared+1)
	}
	return hex[:min(length, len(hex))]
}

func suggestionFlags(ctx *cli.Context) string {
	var flags []string
	for _, name := range []string{"project", "config"} {
		if ctx.IsSet(name) {
			value := ctx.String(name)
			if !printableArgument(value) {
				continue
			}
			flags = append(flags, "--"+name+" "+shellQuote(value))
		}
	}
	if len(flags) == 0 {
		return ""
	}
	return " " + strings.Join(flags, " ")
}

func printableArgument(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func shellQuote(value string) string {
	if value != "" && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:@%+,=-") == "" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

type resolvedIDs struct {
	Capture    evidence.Digest `json:"capture_record,omitempty"`
	Base       evidence.Digest `json:"base_snapshot,omitempty"`
	Candidate  evidence.Digest `json:"candidate_snapshot,omitempty"`
	Receipt    evidence.Digest `json:"receipt,omitempty"`
	Comparison evidence.Digest `json:"comparison,omitempty"`
}

type storedHistory struct {
	captures          []evidence.Capture
	receipts          []evidence.Receipt
	comparisons       []evidence.Comparison
	comparisonDetails map[evidence.Digest]compare.Report
	pins              []evidence.Pin
	reports           []storedReport
}

type storedReport struct {
	ID     evidence.Digest
	Report gotestreport.Report
}

// readHistory consumes the store's complete bounded index. A corrupt record or
// report descriptor is visible as an error rather than silently omitted.
func readHistory(s *store.Store) (storedHistory, error) {
	var history storedHistory
	entries, err := s.List("capture", "receipt", "comparison", "pin", "artifact")
	if err != nil {
		return history, err
	}
	var artifactEntries []store.Entry
	for _, entry := range entries {
		switch entry.Kind {
		case "capture":
			value, err := store.Get[evidence.Capture](s, entry.ID)
			if err != nil {
				return history, err
			}
			history.captures = append(history.captures, value)
		case "receipt":
			value, err := store.Get[evidence.Receipt](s, entry.ID)
			if err != nil {
				return history, err
			}
			history.receipts = append(history.receipts, value)
		case "comparison":
			value, err := store.Get[evidence.Comparison](s, entry.ID)
			if err != nil {
				return history, err
			}
			history.comparisons = append(history.comparisons, value)
		case "pin":
			value, err := store.Get[evidence.Pin](s, entry.ID)
			if err != nil {
				return history, err
			}
			history.pins = append(history.pins, value)
		case "artifact":
			artifactEntries = append(artifactEntries, entry)
		}
	}
	history.comparisonDetails = map[evidence.Digest]compare.Report{}
	for _, comparison := range history.comparisons {
		if comparison.Details == nil {
			continue
		}
		raw, err := s.ReadBlob(comparison.Details.Content)
		if err != nil {
			return history, err
		}
		var details compare.Report
		if strictJSON(raw, &details) != nil {
			return history, fmt.Errorf("%w: comparison details are invalid", store.ErrCorrupt)
		}
		history.comparisonDetails[comparison.ID] = details
	}
	for _, entry := range artifactEntries {
		artifact, err := s.ReadArtifactMetadata(entry.ID)
		if err != nil {
			return history, err
		}
		if !gotestreport.ReportChannel(artifact.Channel) {
			continue
		}
		raw, err := s.ReadBlob(artifact.Content)
		if err != nil {
			return history, err
		}
		var report gotestreport.Report
		if strictJSON(raw, &report) != nil || !validImportedReport(report) {
			return history, fmt.Errorf("%w: imported report is invalid", store.ErrCorrupt)
		}
		history.reports = append(history.reports, storedReport{ID: artifact.Content, Report: report})
	}
	return history, nil
}

func newestCapture(records []evidence.Capture) (evidence.Capture, bool) {
	if len(records) == 0 {
		return evidence.Capture{}, false
	}
	sorted := append([]evidence.Capture(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CapturedAt.Equal(sorted[j].CapturedAt) {
			return sorted[i].ID > sorted[j].ID
		}
		return sorted[i].CapturedAt.After(sorted[j].CapturedAt)
	})
	return sorted[0], true
}

func newestReceipt(records []evidence.Receipt, pair evidence.SnapshotPair) (evidence.Receipt, bool) {
	matches := make([]evidence.Receipt, 0)
	for _, record := range records {
		if record.Snapshots == pair {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return evidence.Receipt{}, false
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].FinishedAt.Equal(matches[j].FinishedAt) {
			if matches[i].StartedAt.Equal(matches[j].StartedAt) {
				return matches[i].ID > matches[j].ID
			}
			return matches[i].StartedAt.After(matches[j].StartedAt)
		}
		return matches[i].FinishedAt.After(matches[j].FinishedAt)
	})
	return matches[0], true
}

// latestComparison finds the comparison associated with the latest run of this
// pair that has a stored comparison. Comparison records have no timestamp, so
// receipt completion time is the available ordering key; content-addressed IDs
// break ties deterministically.
func latestComparison(history storedHistory, pair evidence.SnapshotPair) (evidence.Comparison, evidence.Receipt, bool) {
	receipts := map[evidence.Digest]evidence.Receipt{}
	for _, receipt := range history.receipts {
		if receipt.Snapshots == pair {
			receipts[receipt.ID] = receipt
		}
	}
	type item struct {
		comparison evidence.Comparison
		receipt    evidence.Receipt
	}
	items := []item{}
	for _, comparison := range history.comparisons {
		if receipt, ok := receipts[comparison.Receipt]; ok {
			items = append(items, item{comparison, receipt})
		}
	}
	if len(items) == 0 {
		return evidence.Comparison{}, evidence.Receipt{}, false
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].receipt.FinishedAt.Equal(items[j].receipt.FinishedAt) {
			return items[i].comparison.ID > items[j].comparison.ID
		}
		return items[i].receipt.FinishedAt.After(items[j].receipt.FinishedAt)
	})
	return items[0].comparison, items[0].receipt, true
}

func missingStoredRecord(record, fix string) error {
	return &exitError{code: ExitInvalid, diagnostic: formatDiagnostic("no stored "+record+" is available", fix)}
}

func newestCaptureForStore(s *store.Store) (evidence.Capture, error) {
	entries, err := s.List("capture")
	if err != nil {
		return evidence.Capture{}, operational("capture history is corrupt, unavailable, or exceeds its read bounds")
	}
	captures := make([]evidence.Capture, 0, len(entries))
	for _, entry := range entries {
		value, err := store.Get[evidence.Capture](s, entry.ID)
		if err != nil {
			return evidence.Capture{}, operational("capture history is corrupt or unavailable")
		}
		captures = append(captures, value)
	}
	capture, ok := newestCapture(captures)
	if !ok {
		return evidence.Capture{}, missingStoredRecord("capture", "run after capture to create one")
	}
	return capture, nil
}

func readReceipts(s *store.Store) ([]evidence.Receipt, error) {
	entries, err := s.List("receipt")
	if err != nil {
		return nil, operational("run history is corrupt, unavailable, or exceeds its read bounds")
	}
	records := make([]evidence.Receipt, 0, len(entries))
	for _, entry := range entries {
		value, err := store.Get[evidence.Receipt](s, entry.ID)
		if err != nil {
			return nil, operational("run history is corrupt or unavailable")
		}
		records = append(records, value)
	}
	return records, nil
}

func readComparisons(s *store.Store) ([]evidence.Comparison, error) {
	entries, err := s.List("comparison")
	if err != nil {
		return nil, operational("comparison history is corrupt, unavailable, or exceeds its read bounds")
	}
	records := make([]evidence.Comparison, 0, len(entries))
	for _, entry := range entries {
		value, err := store.Get[evidence.Comparison](s, entry.ID)
		if err != nil {
			return nil, operational("comparison history is corrupt or unavailable")
		}
		records = append(records, value)
	}
	return records, nil
}

func newestRunForProject(project string) (evidence.Capture, evidence.Receipt, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return evidence.Capture{}, evidence.Receipt{}, missingStoredRecord("capture", "run after capture to create one")
		}
		return evidence.Capture{}, evidence.Receipt{}, operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	capture, err := newestCaptureForStore(s)
	if err != nil {
		return evidence.Capture{}, evidence.Receipt{}, err
	}
	receipt, err := newestReceiptForStore(s, evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate})
	return capture, receipt, err
}

func newestReceiptForStore(s *store.Store, pair evidence.SnapshotPair) (evidence.Receipt, error) {
	records, err := readReceipts(s)
	if err != nil {
		return evidence.Receipt{}, err
	}
	receipt, ok := newestReceipt(records, pair)
	if !ok {
		return evidence.Receipt{}, missingStoredRecord("run receipt for newest capture", "run after run to prepare one for the newest capture")
	}
	return receipt, nil
}

func historyForProject(project string) (*store.Store, storedHistory, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, storedHistory{}, nil
		}
		return nil, storedHistory{}, operational("cannot open private evidence store for reading")
	}
	history, err := readHistory(s)
	if err != nil {
		s.Close()
		return nil, storedHistory{}, operational("stored history is corrupt, unavailable, or exceeds its read bounds")
	}
	return s, history, nil
}

func inspectNewestCommand(state *invocation, ctx *cli.Context, exporting bool) error {
	if err := requireArgs(ctx, 0); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	state.project = cfg.Project
	options, err := inspectionOptions(ctx)
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.Project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return missingStoredRecord("capture", "run after review to capture and review this checkout, or after diff to print its current change")
		}
		return operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	capture, err := newestCaptureForStore(s)
	if err != nil {
		return err
	}
	pair := evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate}
	using := &resolvedIDs{Capture: capture.ID, Base: capture.Base, Candidate: capture.Candidate}
	if exporting {
		receipts, err := readReceipts(s)
		if err != nil {
			return err
		}
		comparisonRecords, err := readComparisons(s)
		if err != nil {
			return err
		}
		comparison, receipt, ok := latestComparison(storedHistory{receipts: receipts, comparisons: comparisonRecords}, pair)
		if !ok {
			if _, hasRun := newestReceipt(receipts, pair); !hasRun {
				return missingStoredRecord("run receipt for newest capture", "run after run to prepare one for the newest capture")
			}
			return missingStoredRecord("comparison for newest capture", "run after compare to compare its newest run")
		}
		using.Receipt, using.Comparison = receipt.ID, comparison.ID
		return writeStoredComparison(state, s, comparison, receipt, options, true, using)
	}
	view, err := snapshotBundle(s, pair.Base, pair.Candidate, options, true)
	if err != nil {
		return operational("newest capture inventory or diff is unavailable")
	}
	view.Using = using
	checked := checkoutFreshness(state, cfg.Project, s, capture, view.BaseRecord, view.CandidateRecord, ctx.Bool("stored"))
	view.Freshness = &checked
	return writeResult(state, "snapshot", view)
}

func writeStoredComparison(state *invocation, s *store.Store, comparison evidence.Comparison, receipt evidence.Receipt, options inspectOptions, exporting bool, using *resolvedIDs) error {
	var details *compare.Report
	if comparison.Details != nil {
		raw, err := s.ReadBlob(comparison.Details.Content)
		if err != nil {
			return operational("comparison detail artifact is corrupt or unavailable")
		}
		var decoded compare.Report
		if err := strictJSON(raw, &decoded); err != nil {
			return operational("comparison detail artifact is invalid")
		}
		details = &decoded
	}
	var snapshots *snapshotView
	if receipt.Snapshots.Base != "" {
		view, err := snapshotBundle(s, receipt.Snapshots.Base, receipt.Snapshots.Candidate, options, !state.jsonOutput && !state.forceJSON && !exporting)
		if err != nil {
			return operational("snapshot inventory or diff is unavailable")
		}
		snapshots = &view
	}
	kind := "comparison"
	if exporting {
		kind = "export"
	}
	return writeResult(state, kind, comparisonResult{Comparison: comparison, Receipt: receipt, Details: details, Snapshots: snapshots, Using: using})
}

type comparisonResult struct {
	Comparison  evidence.Comparison `json:"comparison"`
	Receipt     evidence.Receipt    `json:"receipt"`
	Details     *compare.Report     `json:"details,omitempty"`
	Snapshots   *snapshotView       `json:"snapshots,omitempty"`
	Using       *resolvedIDs        `json:"using,omitempty"`
	InspectCard bool                `json:"-"`
}

type pinListView struct {
	Pins []review.View `json:"pins"`
	Next []nextCommand `json:"next"`
}

func listPinHeads(state *invocation, project string) error {
	state.project = project
	s, err := store.Open(project, false, nil)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return operational("cannot open private evidence store for reading")
		}
		view := pinListView{Pins: []review.View{}, Next: firstReviewNext()}
		for i := range view.Next {
			view.Next[i].Command += state.suggestionFlags
		}
		return writeResult(state, "pins", view)
	}
	defer s.Close()
	heads, err := review.Heads(s)
	if err != nil {
		return operational("pin heads are corrupt, unavailable, or exceed lookup bounds")
	}
	pins := make([]review.View, 0, len(heads))
	for _, head := range heads {
		view, err := review.Inspect(s, head.ID)
		if err != nil {
			return operational("pin head or its evidence is corrupt or unavailable")
		}
		pins = append(pins, view)
	}
	nexts := []nextCommand{}
	history, err := readHistory(s)
	if err != nil {
		return err
	}
	capture, captured := newestCapture(history.captures)
	if captured {
		if receipt, ok := newestReceipt(history.receipts, evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate}); ok {
			suggestions, err := browser.PinSuggestions(s, receipt)
			if err != nil {
				return operational("pin suggestions are unavailable: case artifacts are missing or invalid")
			}
			for _, suggestion := range suggestions {
				pinned := false
				for _, pin := range pins {
					if pin.Pin.BasisReceipt == receipt.ID && pin.Pin.Expectation == suggestion {
						pinned = true
						break
					}
				}
				if !pinned {
					nexts = append(nexts, next("after pin "+string(receipt.ID), "pin an expectation from the newest run"))
					break
				}
			}
		}
	}
	switch {
	case len(pins) > 0:
		nexts = append(nexts, next("after status", "show the current capture and review state"))
	case captured:
		nexts = append(nexts, next("after review", "open the review; pin expectations from its run results"))
	default:
		nexts = append(nexts, firstReviewNext()...)
	}
	for i := range nexts {
		nexts[i].Command += state.suggestionFlags
	}
	return writeResult(state, "pins", pinListView{Pins: pins, Next: nexts})
}

func statusCommand(state *invocation, project string, stored bool) error {
	state.project = project
	s, history, err := historyForProject(project)
	if err != nil {
		return err
	}
	if s != nil {
		defer s.Close()
	}
	view, err := buildStatus(s, history, func(record evidence.Capture, base, candidate evidence.Snapshot) freshness {
		return checkoutFreshness(state, project, s, record, base, candidate, stored)
	})
	if err != nil {
		return err
	}
	for i := range view.Next {
		view.Next[i].Command += state.suggestionFlags
	}
	return writeResult(state, "status", view)
}

type statusCapture struct {
	ID                evidence.Digest     `json:"id"`
	CapturedAt        time.Time           `json:"captured_at"`
	Mode              evidence.SourceMode `json:"mode"`
	Base              evidence.Digest     `json:"base_snapshot"`
	Candidate         evidence.Digest     `json:"candidate_snapshot"`
	SelectedUntracked int                 `json:"selected_untracked"`
}

type statusSnapshot struct {
	ID           evidence.Digest       `json:"id"`
	Source       evidence.SourceMode   `json:"source"`
	Commit       string                `json:"commit,omitempty"`
	Completeness evidence.Completeness `json:"completeness"`
	Files        int                   `json:"files"`
	Excluded     int                   `json:"excluded"`
	Unsupported  int                   `json:"unsupported"`
}

// freshness says whether the checkout still holds a stored capture's content
// within its recorded scope. It never changes what the stored records mean.
type freshness struct {
	State  string              `json:"state"` // matches, changed, unknown, or not_checked
	Scope  evidence.SourceMode `json:"scope,omitempty"`
	Reason string              `json:"reason,omitempty"`
}

const (
	freshMatches    = "matches"
	freshChanged    = "changed"
	freshUnknown    = "unknown"
	freshNotChecked = "not_checked"
)

// checkoutFreshness rereads the checkout for a working-tree or index capture
// and compares content hashes, never timestamps. It writes nothing. A failed
// read keeps the stored summary but makes the command exit 1.
func checkoutFreshness(state *invocation, project string, s *store.Store, record evidence.Capture, base, candidate evidence.Snapshot, stored bool) freshness {
	result := freshness{Scope: record.Mode}
	switch {
	case stored:
		result.State, result.Reason = freshNotChecked, "stored_requested"
		return result
	case record.Mode == evidence.MergeBase:
		result.State, result.Reason = freshNotChecked, "immutable_comparison"
		return result
	}
	var index *evidence.Snapshot
	if record.Index != "" {
		snapshot, err := store.Get[evidence.Snapshot](s, record.Index)
		if err != nil {
			result.State, result.Reason = freshUnknown, "stored_index_unavailable"
			return result
		}
		index = &snapshot
	}
	for _, snapshot := range []*evidence.Snapshot{&base, &candidate, index} {
		if snapshot != nil && snapshot.Completeness != evidence.Complete {
			result.State, result.Reason = freshUnknown, "incomplete_capture"
			return result
		}
	}
	stopNotice := startElapsedNotice(state, "checkout check", time.Second)
	unchanged, err := capture.Unchanged(state.ctx, project, record, base, candidate, index)
	stopNotice()
	switch {
	case err != nil:
		failure := captureFailureFor(err, captureFailureInfo{reason: "the checkout could not be read", fix: "check the checkout, then retry"})
		state.exit = ExitOperational
		state.diagnostic = fmt.Sprintf("after: cannot tell whether the checkout still matches the stored capture: %s — %s; after status --stored skips this check", failure.reason, failure.fix)
		result.State, result.Reason = freshUnknown, "checkout_unreadable"
	case unchanged:
		result.State = freshMatches
	default:
		result.State = freshChanged
	}
	return result
}

// staleOrUnreadable means stored results must not be offered as though they
// describe the current checkout.
func (f *freshness) staleOrUnreadable() bool {
	return f != nil && (f.State == freshChanged || f.Reason == "checkout_unreadable")
}

// freshnessNext offers the current change. A plain review would resume a
// saved review, so --new is needed to capture the current change instead.
func freshnessNext(savedReview bool) []nextCommand {
	review := next("after review", "capture the current change and review it")
	if savedReview {
		review = next("after review --new", "capture the current change and replace the saved review")
	}
	return []nextCommand{next("after diff", "show the current change without storing it"), review}
}

type statusView struct {
	Freshness         *freshness             `json:"freshness"`
	SavedReview       *evidence.SnapshotPair `json:"saved_review,omitempty"`
	Capture           *statusCapture         `json:"capture,omitempty"`
	Base              *statusSnapshot        `json:"base_snapshot,omitempty"`
	Candidate         *statusSnapshot        `json:"candidate_snapshot,omitempty"`
	ChangedPaths      int                    `json:"changed_paths,omitempty"`
	ExcludedPaths     int                    `json:"excluded_paths,omitempty"`
	UntrackedExcluded bool                   `json:"untracked_excluded,omitempty"`
	Receipt           *evidence.Receipt      `json:"receipt,omitempty"`
	Comparison        *evidence.Comparison   `json:"comparison,omitempty"`
	Details           *compare.Report        `json:"details,omitempty"`
	Pins              []statusPin            `json:"pins"`
	Reports           []statusReport         `json:"reports"`
	PriorRuns         bool                   `json:"prior_runs"`
	Limits            []string               `json:"limits"`
	Next              []nextCommand          `json:"next"`
}

type statusPin struct {
	ID                   evidence.Digest        `json:"id"`
	Decision             evidence.HumanDecision `json:"decision"`
	Applicability        evidence.Applicability `json:"applicability"`
	Expectation          string                 `json:"expectation"`
	MissingCurrentResult bool                   `json:"missing_current_result"`
	CanAccept            bool                   `json:"can_accept"`
}

type statusReport struct {
	ID       evidence.Digest `json:"id"`
	Imported time.Time       `json:"imported_at"`
	Producer string          `json:"producer,omitempty"`
	Pass     int             `json:"pass"`
	Fail     int             `json:"fail"`
	Skip     int             `json:"skip"`
}

func buildStatus(s *store.Store, history storedHistory, check func(evidence.Capture, evidence.Snapshot, evidence.Snapshot) freshness) (statusView, error) {
	view := statusView{Pins: []statusPin{}, Reports: []statusReport{}, Limits: []string{}, Freshness: &freshness{State: freshNotChecked, Reason: "no_capture"}}
	capture, ok := newestCapture(history.captures)
	if !ok {
		view.Next = firstReviewNext()
		return view, nil
	}
	view.Capture = &statusCapture{ID: capture.ID, CapturedAt: capture.CapturedAt, Mode: capture.Mode, Base: capture.Base, Candidate: capture.Candidate, SelectedUntracked: len(capture.SelectedUntracked)}
	base, err := store.Get[evidence.Snapshot](s, capture.Base)
	if err != nil {
		return statusView{}, operational("newest capture snapshot is corrupt or unavailable")
	}
	candidate, err := store.Get[evidence.Snapshot](s, capture.Candidate)
	if err != nil {
		return statusView{}, operational("newest capture snapshot is corrupt or unavailable")
	}
	view.Base = &statusSnapshot{ID: base.ID, Source: base.Source, Commit: base.Commit, Completeness: base.Completeness, Files: len(base.Files), Excluded: len(base.Excluded), Unsupported: len(base.Unsupported)}
	view.Candidate = &statusSnapshot{ID: candidate.ID, Source: candidate.Source, Commit: candidate.Commit, Completeness: candidate.Completeness, Files: len(candidate.Files), Excluded: len(candidate.Excluded), Unsupported: len(candidate.Unsupported)}
	if check != nil {
		checked := check(capture, base, candidate)
		view.Freshness = &checked
	}
	diff, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		return statusView{}, operational("newest capture inventory is corrupt or unavailable")
	}
	view.ChangedPaths = len(diff.Inventory())
	for _, entry := range diff.Inventory() {
		if excludedUntracked(entry) {
			view.ExcludedPaths++
		}
	}
	view.Limits = append(view.Limits, diff.Limits()...)
	for _, excluded := range candidate.Excluded {
		if strings.Contains(strings.ToLower(excluded.Reason), "untracked") {
			view.UntrackedExcluded = true
			break
		}
	}
	pair := evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate}
	if s != nil {
		raw, err := s.ReadReviewSession()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return statusView{}, operational("saved review is corrupt or unavailable")
		}
		if err == nil {
			session, err := browser.DecodeReviewSession(raw)
			if err != nil {
				return statusView{}, operational("saved review is invalid")
			}
			view.SavedReview = &session.Pair
		}
	}
	if receipt, found := newestReceipt(history.receipts, pair); found {
		view.Receipt = &receipt
		if comparison, _, found := latestComparison(history, pair); found && comparison.Receipt == receipt.ID {
			view.Comparison = &comparison
			if details, ok := history.comparisonDetails[comparison.ID]; ok {
				view.Details = &details
			}
		}
	} else {
		for _, earlier := range history.captures {
			if earlier.ID != capture.ID {
				if _, found := newestReceipt(history.receipts, evidence.SnapshotPair{Base: earlier.Base, Candidate: earlier.Candidate}); found {
					view.PriorRuns = true
					break
				}
			}
		}
	}
	if s != nil {
		heads, err := review.Heads(s)
		if err != nil {
			return statusView{}, operational("pin heads are corrupt, unavailable, or exceed lookup bounds")
		}
		for _, pin := range heads {
			selected := pin.BasisSnapshots
			if len(pin.History) > 0 && pin.History[len(pin.History)-1].Review != nil {
				selected = pin.History[len(pin.History)-1].Review.Target.Snapshots
			}
			if selected != pair {
				continue
			}
			pinView, err := review.Inspect(s, pin.ID)
			if err != nil {
				return statusView{}, operational("pin review state is corrupt or unavailable")
			}
			view.Pins = append(view.Pins, statusPin{
				ID: pin.ID, Decision: pin.Decision, Applicability: pinView.Applicability,
				Expectation: pin.Expectation, MissingCurrentResult: pinView.MissingCurrentResult,
				CanAccept: pin.Decision == evidence.Reopened && pinView.Applicability == evidence.Current && !pinView.MissingCurrentResult && pinView.CurrentReceipt != nil,
			})
		}
	}
	for _, report := range history.reports {
		if report.Report.Metadata.Snapshot != capture.Candidate {
			continue
		}
		passes, failures, skips := reportCounts(report.Report)
		view.Reports = append(view.Reports, statusReport{ID: report.ID, Imported: report.Report.Metadata.ImportedAt, Producer: report.Report.Metadata.Producer,
			Pass: passes, Fail: failures, Skip: skips})
	}
	sort.Slice(view.Reports, func(i, j int) bool {
		if view.Reports[i].Imported.Equal(view.Reports[j].Imported) {
			return view.Reports[i].ID > view.Reports[j].ID
		}
		return view.Reports[i].Imported.After(view.Reports[j].Imported)
	})
	view.Next = statusNext(view, history)
	return view, nil
}

// reviewSuggestion keeps older pairs directly addressable while offering the
// ordinary launch/resume workflow for the newest capture or saved selection.
func reviewSuggestion(project string, pair evidence.SnapshotPair) string {
	explicit := "after review " + string(pair.Base) + " " + string(pair.Candidate)
	if project == "" {
		return explicit
	}
	if saved, found, _, err := readReviewSession(project); err == nil && found && saved.Pair == pair {
		return "after review"
	}
	s, err := store.Open(project, false, nil)
	if err != nil {
		return explicit
	}
	defer s.Close()
	if latest, err := newestCaptureForStore(s); err == nil && pair == (evidence.SnapshotPair{Base: latest.Base, Candidate: latest.Candidate}) {
		return "after review"
	}
	return explicit
}

func statusNext(view statusView, history storedHistory) []nextCommand {
	if view.Capture == nil {
		return firstReviewNext()
	}
	if view.SavedReview != nil && *view.SavedReview != (evidence.SnapshotPair{Base: view.Capture.Base, Candidate: view.Capture.Candidate}) {
		return []nextCommand{
			next("after review", "resume the saved review"),
			next("after review --new", "capture and start a new review"),
		}
	}
	if view.Freshness.staleOrUnreadable() {
		return freshnessNext(view.SavedReview != nil)
	}
	review := next("after review", "open a review of this capture")
	if view.SavedReview != nil {
		review = next("after review", "resume the saved review")
	}
	for _, pin := range view.Pins {
		if pin.Decision == evidence.Reopened || pin.Applicability != evidence.Current || pin.MissingCurrentResult {
			commands := []nextCommand{review}
			if pin.CanAccept {
				commands = append(commands, next("after pin "+string(pin.ID)+" --accept", "accept this explicitly selected current result"))
			}
			return commands
		}
	}
	if view.Receipt == nil && view.PriorRuns {
		return []nextCommand{next("after run", "prepare the newest capture; execution still requires consent")}
	}
	if view.Receipt != nil && view.Comparison == nil {
		return []nextCommand{
			next("after compare", "compare the newest run without executing project code"),
			review,
			next("after diff --stored", "print this captured patch"),
		}
	}
	commands := []nextCommand{
		review,
		next("after diff --stored", "print this captured patch"),
	}
	if view.Comparison != nil {
		commands = append(commands, next("after inspect "+string(view.Comparison.ID), "inspect the stored comparison"))
	}
	return commands
}

func logCommand(state *invocation, ctx *cli.Context) error {
	if err := requireArgs(ctx, 0); err != nil {
		return err
	}
	limit := defaultLogLimit
	if ctx.IsSet("n") {
		limit = ctx.Int("n")
	}
	if limit < 1 || limit > maxLogLimit {
		return invalid("-n must be between 1 and 10000")
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	s, history, err := historyForProject(cfg.Project)
	if err != nil {
		return err
	}
	if s != nil {
		defer s.Close()
	}
	view, err := buildLog(s, history, limit)
	if err != nil {
		return err
	}
	view.Next = logNext(view)
	for i := range view.Next {
		view.Next[i].Command += state.suggestionFlags
	}
	return writeResult(state, "log", view)
}

type logRow struct {
	Kind        string                     `json:"kind"`
	ID          evidence.Digest            `json:"id"`
	At          time.Time                  `json:"at"`
	Base        evidence.Digest            `json:"base_snapshot,omitempty"`
	Candidate   evidence.Digest            `json:"candidate_snapshot,omitempty"`
	Receipt     evidence.Digest            `json:"receipt,omitempty"`
	Mode        evidence.SourceMode        `json:"mode,omitempty"`
	Paths       int                        `json:"paths"`
	Excluded    int                        `json:"excluded_paths,omitempty"`
	Outcome     evidence.ComparisonOutcome `json:"outcome,omitempty"`
	Summary     string                     `json:"summary,omitempty"`
	Decision    evidence.HumanDecision     `json:"decision,omitempty"`
	Action      string                     `json:"action,omitempty"`
	Expectation string                     `json:"expectation,omitempty"`
	Producer    string                     `json:"producer,omitempty"`
	Pass        int                        `json:"pass,omitempty"`
	Fail        int                        `json:"fail,omitempty"`
	Skip        int                        `json:"skip,omitempty"`
}

type logView struct {
	Rows  []logRow      `json:"rows"`
	Total int           `json:"total"`
	Shown int           `json:"shown"`
	Next  []nextCommand `json:"next"`
}

func buildLog(s *store.Store, history storedHistory, limit int) (logView, error) {
	rows := []logRow{}
	for _, record := range history.captures {
		rows = append(rows, logRow{Kind: "capture", ID: record.ID, At: record.CapturedAt, Base: record.Base, Candidate: record.Candidate, Mode: record.Mode})
	}
	for _, record := range history.receipts {
		row := logRow{Kind: "run", ID: record.ID, At: record.StartedAt, Base: record.Snapshots.Base, Candidate: record.Snapshots.Candidate, Outcome: record.State.Comparison}
		for _, comparison := range history.comparisons {
			if comparison.Receipt == record.ID {
				row.Outcome = comparison.Outcome
				if details, ok := history.comparisonDetails[comparison.ID]; ok {
					row.Summary = compactComparison(details)
				}
				break
			}
		}
		rows = append(rows, row)
	}
	for _, record := range history.reports {
		passes, failures, skips := reportCounts(record.Report)
		rows = append(rows, logRow{Kind: "report", ID: record.ID, At: record.Report.Metadata.ImportedAt, Candidate: record.Report.Metadata.Snapshot, Producer: record.Report.Metadata.Producer,
			Pass: passes, Fail: failures, Skip: skips})
	}
	for _, record := range history.pins {
		if len(record.History) == 0 {
			continue
		}
		event := record.History[len(record.History)-1]
		action := ""
		if event.Review != nil {
			action = event.Review.Action
		}
		rows = append(rows, logRow{Kind: "pin", ID: record.ID, At: event.At, Base: record.BasisSnapshots.Base, Candidate: record.BasisSnapshots.Candidate, Receipt: record.BasisReceipt, Decision: event.Decision, Action: action, Expectation: record.Expectation})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].At.Equal(rows[j].At) {
			if rows[i].ID == rows[j].ID {
				return rows[i].Kind < rows[j].Kind
			}
			return rows[i].ID > rows[j].ID
		}
		return rows[i].At.After(rows[j].At)
	})
	total := len(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if len(rows) > 0 && s == nil {
		return logView{}, operational("stored history is unavailable")
	}
	captures := make(map[evidence.Digest]evidence.Capture, len(history.captures))
	for _, capture := range history.captures {
		captures[capture.ID] = capture
	}
	for i := range rows {
		if rows[i].Kind != "capture" {
			continue
		}
		capture := captures[rows[i].ID]
		base, err := store.Get[evidence.Snapshot](s, capture.Base)
		if err != nil {
			return logView{}, operational("capture snapshot is corrupt or unavailable")
		}
		candidate, err := store.Get[evidence.Snapshot](s, capture.Candidate)
		if err != nil {
			return logView{}, operational("capture snapshot is corrupt or unavailable")
		}
		view, err := rawdiff.Open(s, base, candidate)
		if err != nil {
			return logView{}, operational("capture inventory is corrupt or unavailable")
		}
		rows[i].Paths = len(view.Inventory())
		for _, entry := range view.Inventory() {
			if excludedUntracked(entry) {
				rows[i].Excluded++
			}
		}
	}
	return logView{Rows: rows, Total: total, Shown: len(rows), Next: []nextCommand{}}, nil
}

func compactComparison(report compare.Report) string {
	if report.DefinitionName != "" && !report.BuiltInPayment {
		parts := []string{}
		for index, caseID := range report.Cases {
			outcome := evidence.ComparisonOutcome("")
			for _, witness := range report.Witnesses {
				if witness.Relation != "paired" || witness.Before.CaseID != caseID {
					continue
				}
				if outcome == "" || witness.Outcome != evidence.Equal {
					outcome = witness.Outcome
				}
			}
			if outcome == "" {
				continue
			}
			title := caseID
			if index < len(report.CaseTitles) && report.CaseTitles[index] != "" {
				title = report.CaseTitles[index]
			}
			parts = append(parts, fmt.Sprintf("%s [%s]", title, strings.ToUpper(string(outcome))))
		}
		return strings.Join(parts, " · ")
	}
	parts := []string{}
	for _, seconds := range []int64{43200, 30} {
		outcome := evidence.ComparisonOutcome("")
		countChange := ""
		for _, witness := range report.Witnesses {
			if witness.Relation != "paired" || witness.Before.Seconds != seconds || (witness.Channel != "provider" && witness.Channel != "provider_calls") {
				continue
			}
			if outcome == "" || witness.Outcome != evidence.Equal {
				outcome = witness.Outcome
			}
			for _, change := range witness.Changes {
				if strings.HasSuffix(change.Path, "/count") && len(change.Before) > 0 && len(change.After) > 0 {
					countChange = rawValue(change.Before) + " → " + rawValue(change.After)
				}
			}
		}
		if outcome == "" {
			continue
		}
		part := fmt.Sprintf("%s [%s]", formatDelay(seconds), strings.ToUpper(string(outcome)))
		if countChange != "" {
			part += " " + countChange
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " · ")
}

func logNext(view logView) []nextCommand {
	if view.Total > view.Shown {
		nextLimit := max(defaultLogLimit*2, view.Shown+1)
		if nextLimit > view.Total {
			nextLimit = view.Total
		}
		return []nextCommand{next(fmt.Sprintf("after log -n %d", nextLimit), "show older stored records")}
	}
	if view.Total > 0 {
		return []nextCommand{next("after status", "show the current capture and review state")}
	}
	return firstReviewNext()
}

// firstReviewNext starts the main workflow in a checkout with no capture.
func firstReviewNext() []nextCommand {
	return []nextCommand{
		next("after review", "capture this checkout and open a review"),
		next("after capture", "capture without opening a review"),
	}
}

func reportCounts(report gotestreport.Report) (int, int, int) {
	var pass, fail, skip int
	for _, card := range report.Cards {
		switch card.State.Report {
		case evidence.ReportPass:
			pass++
		case evidence.ReportFail:
			fail++
		case evidence.ReportSkip:
			skip++
		}
	}
	return pass, fail, skip
}
