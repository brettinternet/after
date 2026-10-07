package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

const reportPageSize = 128
const maxReportPageSize = 256
const inventoryPageSize = 128
const maxInventoryPageSize = 256

func commands(state *invocation) []*ucli.Command {
	return []*ucli.Command{
		pinCommand(state), reviewCommand(state),
		{
			Name: "capture", Usage: "capture a bounded local Git comparison without running project code",
			Before:    outputBefore(state),
			ArgsUsage: "[--staged | --base BASE --target TARGET]", Flags: append(commonFlags(),
				&ucli.BoolFlag{Name: "staged", Usage: "capture HEAD versus the index"},
				&ucli.StringFlag{Name: "base", Usage: "base commit for explicit merge-base capture"},
				&ucli.StringFlag{Name: "target", Usage: "target commit for explicit merge-base capture"},
				&ucli.StringSliceFlag{Name: "include-untracked", Usage: "select an exact non-ignored untracked file (repeatable)"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return captureCommand(state, ctx) },
		},
		{
			Name: "import", Usage: "import bounded stock go test -json as reported evidence",
			Before:    outputBefore(state),
			ArgsUsage: "<go-test-json-file>", Flags: append(commonFlags(),
				&ucli.StringFlag{Name: "producer", Usage: "required caller-supplied producer/version provenance"},
				&ucli.StringFlag{Name: "captured-at", Usage: "optional caller-supplied RFC3339 capture time"},
				&ucli.StringFlag{Name: "snapshot", Usage: "bind the report to a validated snapshot ID (not proof of applicability)"},
				&ucli.IntFlag{Name: "offset", Usage: "first imported report card (default 0)"},
				&ucli.IntFlag{Name: "limit", Usage: "report cards to return (1-256; default 128)"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return importCommand(state, ctx) },
		},
		{
			Name: "inspect", Usage: "inspect a snapshot, receipt, comparison, report, or artifact by stable ID",
			Before:    outputBefore(state),
			ArgsUsage: "<stable-id>", Flags: append(commonFlags(), inspectionFlags()...),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return inspectCommand(state, ctx, false) },
		},
		{
			Name: "compare", Usage: "compare a persisted execution receipt without running code",
			Before:    outputBefore(state),
			ArgsUsage: "<receipt-id>", Flags: commonFlags(),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return compareCommand(state, ctx) },
		},
		{
			Name: "export", Usage: "export a bounded machine-readable comparison or evidence page",
			Before:    outputBefore(state),
			ArgsUsage: "<stable-id>", Flags: append(commonFlags(), inspectionFlags()...),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return inspectCommand(state, ctx, true) },
		},
		{
			Name: "run", Usage: "preview, explicitly authorize, and compare the frozen offline payment experiment",
			Before:    outputBefore(state),
			ArgsUsage: "<base-snapshot-id> <candidate-snapshot-id> (or --plan-file FILE --approve DIGEST)",
			Flags: append(commonFlags(),
				&ucli.StringFlag{Name: "plan-file", Usage: "reconstruct an exact previously saved execution preview"},
				&ucli.StringFlag{Name: "plan-out", Usage: "create a private file containing the exact preview for later approval"},
				&ucli.StringFlag{Name: "approve", Usage: "approve only this exact preview digest; never a blanket consent"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return runCommand(state, ctx) },
		},
		{
			Name: "config", Usage: "show effective configuration and per-setting provenance without execution",
			Before: outputBefore(state),
			Flags:  commonFlags(), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return configCommand(state, ctx) },
		},
		{
			Name: "version", Usage: "print the AFTER version",
			Action: func(ctx *ucli.Context) error {
				if err := requireArgs(ctx, 0); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(state.stdout, "after %s\n", Version); err != nil {
					return operational("cannot write version")
				}
				return nil
			},
		},
	}
}

func inspectionFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "base", Usage: "matching base snapshot ID for raw diff and complete inventory"},
		&ucli.IntFlag{Name: "diff-offset", Usage: "raw diff byte offset (default 0)"},
		&ucli.IntFlag{Name: "diff-size", Usage: "raw diff page bytes (1-65536; default 65536)"},
		&ucli.IntFlag{Name: "inventory-offset", Usage: "first changed/unknown inventory entry (default 0)"},
		&ucli.IntFlag{Name: "inventory-limit", Usage: "inventory entries to return (1-256; default 128)"},
		&ucli.IntFlag{Name: "card-offset", Usage: "first report card (default 0)"},
		&ucli.IntFlag{Name: "card-limit", Usage: "report cards to return (1-256; default 128)"},
		&ucli.IntFlag{Name: "artifact-offset", Usage: "stored artifact byte offset (default 0)"},
		&ucli.IntFlag{Name: "artifact-size", Usage: "stored artifact page bytes (1-65536; default 65536)"},
	}
}

func configCommand(state *invocation, ctx *ucli.Context) error {
	if err := requireArgs(ctx, 0); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	type setting struct {
		Name   string `json:"name"`
		Value  any    `json:"value"`
		Source string `json:"source"`
	}
	configValue := "not selected"
	if cfg.ConfigPath != "" {
		configValue = "selected path hidden"
	}
	dockerBinary, dockerHost := "not configured (value hidden)", "not configured (value hidden)"
	if cfg.DockerBinary != "" {
		dockerBinary = "configured (value hidden)"
	}
	if cfg.DockerHost != "" {
		dockerHost = "configured (value hidden)"
	}
	settings := []setting{
		{"config_path", configValue, cfg.Sources["config_path"]},
		{"project", "selected path hidden", cfg.Sources["project"]},
		{"repetitions", cfg.Repetitions, cfg.Sources["repetitions"]},
		{"run_seconds", cfg.RunSeconds, cfg.Sources["run_seconds"]},
		{"output_bytes", cfg.OutputBytes, cfg.Sources["output_bytes"]},
		{"interactive", cfg.Interactive, cfg.Sources["interactive"]},
		{"raw_diff", cfg.RawDiff, cfg.Sources["raw_diff"]},
		{"diff_bytes", cfg.DiffBytes, cfg.Sources["diff_bytes"]},
		{"docker_binary", dockerBinary, cfg.Sources["docker_binary"]},
		{"docker_host", dockerHost, cfg.Sources["docker_host"]},
	}
	return writeResult(state, "configuration", struct {
		Settings []setting `json:"settings"`
	}{settings})
}

func captureCommand(state *invocation, ctx *ucli.Context) error {
	if err := requireArgs(ctx, 0); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	options := capture.Options{}
	baseSet, targetSet := ctx.IsSet("base"), ctx.IsSet("target")
	if ctx.IsSet("staged") && ctx.Bool("staged") {
		if baseSet || targetSet {
			return invalid("--staged cannot be combined with --base/--target")
		}
		options.Mode = evidence.Index
	} else if baseSet || targetSet {
		if !baseSet || !targetSet || strings.TrimSpace(ctx.String("base")) == "" || strings.TrimSpace(ctx.String("target")) == "" {
			return invalid("--base and --target are required together")
		}
		options.Mode = evidence.MergeBase
		options.Base, options.Target = ctx.String("base"), ctx.String("target")
	}
	if ctx.IsSet("include-untracked") {
		options.IncludeUntracked = ctx.StringSlice("include-untracked")
	}
	s, err := store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	stopNotice := startElapsedNotice(state, "capture", time.Second)
	defer stopNotice()
	result, err := capture.Capture(state.ctx, cfg.Project, s, options)
	if err != nil {
		return captureFailure(err)
	}
	summary := func(snapshot evidence.Snapshot) snapshotSummary {
		return snapshotSummary{snapshot.ID, snapshot.Source, snapshot.Completeness, len(snapshot.Files), len(snapshot.Excluded), len(snapshot.Unsupported), snapshot.Diff, append([]string(nil), snapshot.Limits...)}
	}
	var index *snapshotSummary
	if result.Index != nil {
		value := summary(*result.Index)
		index = &value
	}
	return writeResult(state, "capture", struct {
		Base      snapshotSummary  `json:"base_snapshot"`
		Candidate snapshotSummary  `json:"candidate_snapshot"`
		Index     *snapshotSummary `json:"index_snapshot,omitempty"`
	}{summary(result.Base), summary(result.Candidate), index})
}

func importCommand(state *invocation, ctx *ucli.Context) error {
	if err := requireArgs(ctx, 1); err != nil {
		return err
	}
	if !ctx.IsSet("producer") || strings.TrimSpace(ctx.String("producer")) == "" {
		return invalid("import requires caller-supplied --producer provenance")
	}
	producer := ctx.String("producer")
	if strings.TrimSpace(producer) != producer || len(producer) > 256 {
		return invalid("invalid producer provenance")
	}
	var capturedAt *time.Time
	if ctx.IsSet("captured-at") {
		parsed, err := time.Parse(time.RFC3339Nano, ctx.String("captured-at"))
		if err != nil {
			return invalid("captured-at must be an RFC3339 timestamp")
		}
		parsed = parsed.UTC()
		capturedAt = &parsed
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	offset, limit, err := pageArgs(ctx, "offset", "limit", reportPageSize, maxReportPageSize)
	if err != nil {
		return err
	}
	var snapshot evidence.Digest
	file, err := openInput(ctx.Args().Get(0))
	if err != nil {
		return operational("cannot read Go test report file")
	}
	defer file.Close()
	s, err := store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	if ctx.IsSet("snapshot") {
		snapshot, err = resolveID(s, ctx.String("snapshot"), "snapshot")
		if err != nil {
			return err
		}
	}
	stopNotice := startElapsedNotice(state, "import", time.Second)
	defer stopNotice()
	report, err := gotestreport.Import(file, gotestreport.Metadata{Producer: producer, CapturedAt: capturedAt, Snapshot: snapshot, ImportedAt: time.Now().UTC()})
	if err != nil {
		return invalid("report is invalid or exceeds the 8 MiB input limit")
	}
	encoded, err := json.Marshal(report)
	if err != nil || len(encoded) > store.MaxBlobBytes {
		return operational("imported report exceeds the private artifact limit")
	}
	artifact, err := s.PutArtifact(encoded, "go-test-report-v1", store.MaxBlobBytes)
	if err != nil || artifact.Completeness != evidence.Complete {
		return operational("cannot persist complete imported report")
	}
	return writeResult(state, "import", reportView(artifact.Content, report, offset, limit))
}

func reportView(id evidence.Digest, report gotestreport.Report, offset, limit int) any {
	end := min(len(report.Cards), offset+limit)
	cards := []gotestreport.Card{}
	if offset < len(report.Cards) {
		cards = append(cards, report.Cards[offset:end]...)
	}
	counts := map[evidence.ReportOutcome]int{}
	for _, card := range report.Cards {
		counts[card.State.Report]++
	}
	return reportViewData{id, report.SchemaVersion, report.Dialect, report.Metadata, report.OriginalDigest, report.Completeness, counts, cards, offset, len(report.Cards), end < len(report.Cards), report.Diagnostics, report.SuppressedDiagnostics}
}

func inspectCommand(state *invocation, ctx *ucli.Context, exporting bool) error {
	if err := requireArgs(ctx, 1); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	options, err := inspectionOptions(ctx)
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.Project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return noIDMatch(ctx.Args().Get(0), inspectKinds...)
		}
		return operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	id, err := resolveID(s, ctx.Args().Get(0), inspectKinds...)
	if err != nil {
		return err
	}
	if ctx.IsSet("base") {
		base, err := resolveID(s, ctx.String("base"), "snapshot")
		if err != nil {
			return err
		}
		options.base = string(base)
	}
	kind := "inspection"
	if exporting {
		kind = "export"
	}
	if value, ok, err := optionalGet[evidence.Comparison](s, id); err != nil {
		return operational("comparison record is corrupt or unavailable")
	} else if ok {
		receipt, err := store.Get[evidence.Receipt](s, value.Receipt)
		if err != nil {
			return operational("comparison receipt is corrupt or unavailable")
		}
		var report *compare.Report
		if value.Details != nil {
			raw, err := s.ReadBlob(value.Details.Content)
			if err != nil {
				return operational("comparison detail artifact is corrupt or unavailable")
			}
			var decoded compare.Report
			if err := strictJSON(raw, &decoded); err != nil {
				return operational("comparison detail artifact is invalid")
			}
			report = &decoded
		}
		var snapshots *snapshotView
		if receipt.Snapshots.Base != "" {
			view, err := snapshotBundle(s, receipt.Snapshots.Base, receipt.Snapshots.Candidate, options, !state.jsonOutput && !state.forceJSON && !exporting)
			if err != nil {
				return operational("snapshot inventory or diff is unavailable")
			}
			snapshots = &view
		}
		if outcome := comparisonExit(value.Outcome); outcome != ExitOK {
			state.exit = outcome
		}
		kind := "comparison"
		if exporting {
			kind = "export"
		}
		return writeResult(state, kind, struct {
			Comparison evidence.Comparison `json:"comparison"`
			Receipt    evidence.Receipt    `json:"receipt"`
			Details    *compare.Report     `json:"details,omitempty"`
			Snapshots  *snapshotView       `json:"snapshots,omitempty"`
		}{value, receipt, report, snapshots})
	}
	if value, ok, err := optionalGet[evidence.Receipt](s, id); err != nil {
		return operational("receipt record is corrupt or unavailable")
	} else if ok {
		return writeResult(state, "receipt", value)
	}
	if value, ok, err := optionalGet[evidence.Snapshot](s, id); err != nil {
		return operational("snapshot record is corrupt or unavailable")
	} else if ok {
		if options.base == "" {
			if exporting {
				return invalid("export requires a comparison ID")
			}
			inspection := snapshotInspection{Snapshot: value}
			if !state.jsonOutput && !state.forceJSON {
				inspection.CaptureHistory = captureHistoryForSnapshot(s, id)
			}
			return writeResult(state, "snapshot", inspection)
		}
		view, err := snapshotBundle(s, evidence.Digest(options.base), id, options, !state.jsonOutput && !state.forceJSON && !exporting)
		if err != nil {
			return operational("snapshot comparison page is unavailable")
		}
		kind := "snapshot"
		if exporting {
			kind = "export"
		}
		return writeResult(state, kind, view)
	}
	if raw, err := s.ReadBlob(id); err == nil {
		var report gotestreport.Report
		if strictJSON(raw, &report) == nil && validImportedReport(report) {
			if options.cardOffset < 0 || options.cardLimit < 1 || options.cardLimit > maxReportPageSize {
				return invalid("report card page is outside supported bounds")
			}
			return writeResult(state, kind, reportView(id, report, options.cardOffset, options.cardLimit))
		}
		offset := min(options.artifactOffset, len(raw))
		end := offset + min(options.artifactSize, len(raw)-offset)
		var document json.RawMessage
		if offset == 0 && end == len(raw) && json.Valid(raw) {
			document = raw
		}
		// Blob content alone establishes no producer or evidence state. Keep
		// exact bytes available even for binary, malformed, or partial JSON.
		return writeResult(state, "artifact", struct {
			ID       evidence.Digest `json:"id"`
			Offset   int             `json:"offset"`
			Next     int             `json:"next"`
			Total    int             `json:"total"`
			More     bool            `json:"more"`
			Base64   string          `json:"base64"`
			Document json.RawMessage `json:"document,omitempty"`
		}{id, offset, end, len(raw), end < len(raw), base64.StdEncoding.EncodeToString(raw[offset:end]), document})
	} else if !errors.Is(err, os.ErrNotExist) {
		return operational("stored artifact is corrupt or unavailable")
	}
	return invalid("stable ID was not found")
}

type inspectOptions struct {
	base            string
	diffOffset      int
	diffSize        int
	inventoryOffset int
	inventoryLimit  int
	cardOffset      int
	cardLimit       int
	artifactOffset  int
	artifactSize    int
}

func inspectionOptions(ctx *ucli.Context) (inspectOptions, error) {
	cfg, err := configFlags(ctx)
	if err != nil {
		return inspectOptions{}, err
	}
	options := inspectOptions{base: ctx.String("base"), diffSize: cfg.DiffBytes, inventoryLimit: inventoryPageSize, cardLimit: reportPageSize}
	options.artifactOffset, options.artifactSize, err = pageArgs(ctx, "artifact-offset", "artifact-size", rawdiff.MaxPageBytes, rawdiff.MaxPageBytes)
	if err != nil {
		return inspectOptions{}, err
	}
	if !cfg.RawDiff {
		options.diffSize = 0
	}
	if ctx.IsSet("diff-offset") {
		options.diffOffset = ctx.Int("diff-offset")
	}
	if ctx.IsSet("diff-size") {
		options.diffSize = ctx.Int("diff-size")
	}
	if ctx.IsSet("inventory-offset") {
		options.inventoryOffset = ctx.Int("inventory-offset")
	}
	if ctx.IsSet("inventory-limit") {
		options.inventoryLimit = ctx.Int("inventory-limit")
	}
	if ctx.IsSet("card-offset") {
		options.cardOffset = ctx.Int("card-offset")
	}
	if ctx.IsSet("card-limit") {
		options.cardLimit = ctx.Int("card-limit")
	}
	if options.diffOffset < 0 || options.diffSize < 0 || options.diffSize > rawdiff.MaxPageBytes || options.inventoryOffset < 0 || options.inventoryLimit < 1 || options.inventoryLimit > maxInventoryPageSize || options.cardOffset < 0 || options.cardLimit < 1 || options.cardLimit > maxReportPageSize {
		return inspectOptions{}, invalid("inspection page is outside supported bounds")
	}
	return options, nil
}

type inventoryItem struct {
	Path            string         `json:"path"`
	Change          string         `json:"change"`
	PotentialOracle bool           `json:"potential_oracle"`
	Binary          bool           `json:"binary"`
	Base            *evidence.File `json:"base,omitempty"`
	Candidate       *evidence.File `json:"candidate,omitempty"`
	Limits          []string       `json:"limits"`
}

type snapshotCaptureHistory struct {
	Records     []store.CaptureSummary
	Limited     bool
	More        bool
	Unavailable bool
}

type snapshotInspection struct {
	evidence.Snapshot
	CaptureHistory snapshotCaptureHistory `json:"-"`
}

type snapshotView struct {
	Base                    evidence.Digest        `json:"base_snapshot"`
	Candidate               evidence.Digest        `json:"candidate_snapshot"`
	BaseRecord              evidence.Snapshot      `json:"-"`
	CandidateRecord         evidence.Snapshot      `json:"-"`
	BaseCaptureHistory      snapshotCaptureHistory `json:"-"`
	CandidateCaptureHistory snapshotCaptureHistory `json:"-"`
	Inventory               []inventoryItem        `json:"inventory"`
	InventoryOffset         int                    `json:"inventory_offset"`
	InventoryTotal          int                    `json:"inventory_total"`
	InventoryMore           bool                   `json:"inventory_more"`
	Diff                    struct {
		Content   evidence.Digest `json:"content"`
		Offset    int             `json:"offset"`
		Next      int             `json:"next"`
		Total     int             `json:"total"`
		More      bool            `json:"more"`
		Base64    string          `json:"base64"`
		Available bool            `json:"available"`
		Limits    []string        `json:"limits"`
	} `json:"diff"`
	Limits []string `json:"limits"`
}

func snapshotBundle(s *store.Store, baseID, candidateID evidence.Digest, options inspectOptions, includeCaptureHistory bool) (snapshotView, error) {
	var result snapshotView
	base, err := store.Get[evidence.Snapshot](s, baseID)
	if err != nil {
		return result, err
	}
	candidate, err := store.Get[evidence.Snapshot](s, candidateID)
	if err != nil {
		return result, err
	}
	view, err := rawdiff.Open(s, base, candidate)
	if err != nil {
		return result, err
	}
	inventory := view.Inventory()
	end := len(inventory)
	page := []inventoryItem{}
	if options.inventoryOffset < len(inventory) {
		end = options.inventoryOffset + min(options.inventoryLimit, len(inventory)-options.inventoryOffset)
		page = make([]inventoryItem, 0, end-options.inventoryOffset)
		for _, entry := range inventory[options.inventoryOffset:end] {
			page = append(page, inventoryItem{entry.Path, entry.Change, entry.PotentialOracle, entry.Binary, entry.Base, entry.Candidate, append([]string(nil), entry.Limits...)})
		}
	}
	var raw rawdiff.Page
	if options.diffSize > 0 {
		raw, err = view.Raw(options.diffOffset, options.diffSize)
		if err != nil && !errors.Is(err, rawdiff.ErrDiff) {
			return result, err
		}
	}
	result.Base, result.Candidate = baseID, candidateID
	result.BaseRecord, result.CandidateRecord = base, candidate
	if includeCaptureHistory {
		result.BaseCaptureHistory = captureHistoryForSnapshot(s, baseID)
		result.CandidateCaptureHistory = captureHistoryForSnapshot(s, candidateID)
	}
	result.Inventory, result.InventoryOffset, result.InventoryTotal, result.InventoryMore = page, options.inventoryOffset, len(inventory), end < len(inventory)
	result.Limits = view.Limits()
	result.Diff.Content = candidate.Diff
	result.Diff.Offset = raw.Offset
	result.Diff.Next = raw.Next
	result.Diff.Total = raw.Total
	result.Diff.More = raw.More
	result.Diff.Available = options.diffSize > 0 && !errors.Is(err, rawdiff.ErrDiff)
	result.Diff.Base64 = base64.StdEncoding.EncodeToString(raw.Bytes)
	result.Diff.Limits = view.Limits()
	return result, nil
}

func captureHistoryForSnapshot(s *store.Store, id evidence.Digest) snapshotCaptureHistory {
	history, err := store.CapturesForSnapshot(s, id)
	if err != nil {
		return snapshotCaptureHistory{Unavailable: true}
	}
	return snapshotCaptureHistory{Records: history.Records, Limited: history.Limited, More: history.More}
}

func optionalGet[T store.Record](s *store.Store, id evidence.Digest) (T, bool, error) {
	value, err := store.Get[T](s, id)
	if err == nil {
		return value, true, nil
	}
	var zero T
	if errors.Is(err, os.ErrNotExist) {
		return zero, false, nil
	}
	return zero, false, err
}

func validImportedReport(report gotestreport.Report) bool {
	if report.SchemaVersion != 1 || report.Dialect != gotestreport.Dialect {
		return false
	}
	for _, card := range report.Cards {
		if card.State.Validate() != nil || card.State.Producer != evidence.Importer || card.State.Kind != evidence.Reported || card.State.Applicability != evidence.Unknown || card.State.Execution != evidence.NotRun || card.State.Comparison != evidence.NotCompared {
			return false
		}
	}
	return true
}

func strictJSON(raw []byte, target any) error {
	if len(raw) > store.MaxBlobBytes {
		return errors.New("JSON artifact exceeds bounds")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func pageArgs(ctx *ucli.Context, offsetName, limitName string, defaultLimit, maxLimit int) (int, int, error) {
	offset, limit := 0, defaultLimit
	if ctx.IsSet(offsetName) {
		offset = ctx.Int(offsetName)
	}
	if ctx.IsSet(limitName) {
		limit = ctx.Int(limitName)
	}
	if offset < 0 || limit < 1 || limit > maxLimit {
		return 0, 0, invalid("page is outside supported bounds")
	}
	return offset, limit, nil
}

func openInput(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("empty input path")
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("input must be a regular file")
	}
	return file, nil
}

func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}

func comparisonExit(outcome evidence.ComparisonOutcome) int {
	if outcome == evidence.Different || outcome == evidence.Unstable {
		return ExitFinding
	}
	if outcome != evidence.Equal {
		return ExitOperational
	}
	return ExitOK
}

func compareCommand(state *invocation, ctx *ucli.Context) error {
	if err := requireArgs(ctx, 1); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	id, err := resolveID(s, ctx.Args().Get(0), "receipt")
	if err != nil {
		return err
	}
	comparisonRecord, err := compare.Run(s, id)
	if err != nil {
		return operational("comparison could not be persisted")
	}
	state.exit = comparisonExit(comparisonRecord.Outcome)
	var details compare.Report
	if comparisonRecord.Details != nil {
		raw, err := s.ReadBlob(comparisonRecord.Details.Content)
		if err != nil || strictJSON(raw, &details) != nil {
			return operational("comparison details are unavailable")
		}
	}
	return writeResult(state, "comparison", struct {
		Comparison evidence.Comparison `json:"comparison"`
		Details    compare.Report      `json:"details"`
	}{comparisonRecord, details})
}

func runCommand(state *invocation, ctx *ucli.Context) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	planPath := strings.TrimSpace(ctx.String("plan-file"))
	planOut := strings.TrimSpace(ctx.String("plan-out"))
	approvalSet := ctx.IsSet("approve") && strings.TrimSpace(ctx.String("approve")) != ""
	if ctx.IsSet("approve") && !validDigest(ctx.String("approve")) {
		return invalid("--approve requires the full digest: --approve sha256:<64 lowercase hex characters>; copy the authorization digest from the preview")
	}
	if planPath != "" && planOut != "" {
		return invalid("--plan-file cannot be combined with --plan-out")
	}
	var s *store.Store
	if planPath != "" {
		if ctx.Args().Len() != 0 {
			return invalid("saved-plan execution does not accept snapshot arguments")
		}
	} else if ctx.Args().Len() != 2 {
		return invalid("run requires two snapshot IDs or --plan-file")
	}
	if approvalSet && planPath == "" {
		return invalid("noninteractive approval requires --plan-file to reconstruct the exact saved preview")
	}
	if planOut != "" && ctx.IsSet("approve") {
		return invalid("--plan-out is only valid when preparing a preview")
	}
	s, err = store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	var plan *runner.Plan
	if planPath != "" {
		preview, err := readBounded(planPath, 1<<20)
		if err != nil {
			return operational("cannot read saved execution plan")
		}
		plan, err = runner.PrepareFromPreview(s, preview)
		if err != nil {
			return invalid("saved execution plan is invalid or no longer matches stored snapshots")
		}
	} else {
		base, err := resolveID(s, ctx.Args().Get(0), "snapshot")
		if err != nil {
			return err
		}
		candidate, err := resolveID(s, ctx.Args().Get(1), "snapshot")
		if err != nil {
			return err
		}
		pair := evidence.SnapshotPair{Base: base, Candidate: candidate}
		plan, err = runner.Prepare(s, pair, cfg.Repetitions, sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes})
		if err != nil {
			return operational("cannot prepare the bounded payment execution plan")
		}
	}
	preview, digest := plan.Preview()
	if planOut != "" {
		if err := writePrivateNew(planOut, preview); err != nil {
			return operational("cannot save execution preview without overwriting an existing file")
		}
	}
	if approvalSet {
		if ctx.String("approve") != digest {
			state.exit = ExitDenied
			return writeResult(state, "execution_preview", struct {
				Authorization string          `json:"authorization_digest"`
				Status        string          `json:"status"`
				Plan          json.RawMessage `json:"plan"`
			}{digest, "authorization_mismatch", preview})
		}
	} else if cfg.Interactive && state.tty {
		if !prompt(state, digest, preview) {
			state.exit = ExitDenied
			return writeResult(state, "execution_preview", struct {
				Authorization string          `json:"authorization_digest"`
				Status        string          `json:"status"`
				Plan          json.RawMessage `json:"plan"`
			}{digest, "operator_declined", preview})
		}
	} else {
		state.exit = ExitDenied
		return writeResult(state, "execution_preview", struct {
			Authorization string          `json:"authorization_digest"`
			Status        string          `json:"status"`
			Plan          json.RawMessage `json:"plan"`
		}{digest, "authorization_required", preview})
	}
	if cfg.DockerBinary == "" || cfg.DockerHost == "" {
		return operational("execution requires explicit AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST; no host fallback is supported")
	}
	result, runErr := (runner.Executor{Docker: sandbox.Docker{Binary: cfg.DockerBinary, Host: cfg.DockerHost}}).Run(state.ctx, s, plan, digest)
	if err := ensureReceipt(result.Receipt); err != nil {
		return operational("runner did not persist a usable receipt")
	}
	comparisonRecord, compareErr := compare.Run(s, result.Receipt.ID)
	if compareErr != nil {
		return operational("comparison could not be persisted")
	}
	comparisonBytes, err := s.ReadBlob(comparisonRecord.Details.Content)
	if err != nil {
		return operational("comparison details are unavailable")
	}
	var details compare.Report
	if err := strictJSON(comparisonBytes, &details); err != nil {
		return operational("comparison details are invalid")
	}
	status := "completed"
	if runErr != nil || result.Receipt.State.Execution != evidence.Completed {
		status = "incomplete"
	}
	response := struct {
		Status        string              `json:"status"`
		Authorization string              `json:"authorization_digest"`
		Plan          json.RawMessage     `json:"plan"`
		Receipt       evidence.Receipt    `json:"receipt"`
		Comparison    evidence.Comparison `json:"comparison"`
		Details       compare.Report      `json:"details"`
		Samples       []runner.Sample     `json:"samples"`
	}{status, digest, preview, result.Receipt, comparisonRecord, details, result.Samples}
	if runErr != nil {
		state.exit = ExitOperational
	} else {
		state.exit = comparisonExit(comparisonRecord.Outcome)
	}
	return writeResult(state, "run", response)
}

func ensureReceipt(receipt evidence.Receipt) error {
	if receipt.ID == "" || !validDigest(string(receipt.ID)) {
		return errors.New("receipt unavailable")
	}
	return nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := openInput(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("file exceeds size limit")
	}
	return data, nil
}

func writePrivateNew(path string, content []byte) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	if n, writeErr := file.Write(content); writeErr != nil || n != len(content) {
		_ = file.Close()
		return errors.Join(writeErr, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
