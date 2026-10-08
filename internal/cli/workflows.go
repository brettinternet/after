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

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/rawdiff"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	ucli "github.com/urfave/cli/v2"
)

const reportPageSize = 128
const maxReportPageSize = 256
const inventoryPageSize = 128
const maxInventoryPageSize = 256

func isImportedReportArtifact(s *store.Store, content evidence.Digest) bool {
	entries, err := s.List("artifact")
	if err != nil {
		return false
	}
	var readBytes int64
	for _, entry := range entries {
		if entry.Size < 0 || entry.Size > int64(store.MaxBlobBytes)-readBytes {
			return false
		}
		readBytes += entry.Size
		artifact, err := s.ReadArtifactMetadata(entry.ID)
		if err == nil && artifact.Content == content && gotestreport.ReportChannel(artifact.Channel) {
			return true
		}
	}
	return false
}

func commands(state *invocation) []*ucli.Command {
	return []*ucli.Command{
		pinCommand(state), reviewCommand(state),
		{
			Name: "status", Usage: "summarize stored review state and whether the checkout still matches it",
			Before: outputBefore(state), Flags: append(globalFlags(), storedFlag()),
			Action: func(ctx *ucli.Context) error {
				if err := requireArgs(ctx, 0); err != nil {
					return err
				}
				cfg, err := configFlags(ctx)
				if err != nil {
					return err
				}
				return statusCommand(state, cfg.Project, ctx.Bool("stored"))
			},
		},
		{
			Name: "diff", Usage: "print the checkout's current change without storing it, or a stored patch",
			Before: outputBefore(state), ArgsUsage: "[--staged | --base REF [--target REF]] | --stored | BASE CANDIDATE", Flags: append(globalFlags(),
				&ucli.BoolFlag{Name: "stat", Usage: "print per-file changed line counts instead of the patch"},
				&ucli.BoolFlag{Name: "raw", Usage: "write exact patch bytes; stdout must not be a terminal"},
				&ucli.BoolFlag{Name: "stored", Usage: "print the newest stored capture's patch instead of reading the checkout"},
				&ucli.BoolFlag{Name: "staged", Usage: "read HEAD versus the index"},
				&ucli.StringFlag{Name: "base", Usage: "base commit for a merge-base comparison"},
				&ucli.StringFlag{Name: "target", Usage: "target commit for a merge-base comparison"},
				&ucli.StringSliceFlag{Name: "include-untracked", Usage: "include an exact non-ignored untracked file (repeatable)"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return diffCommand(state, ctx) },
		},
		{
			Name: "log", Usage: "list recent stored captures, runs, reports, and pin events",
			Before: outputBefore(state), ArgsUsage: "[-n N]", Flags: append(globalFlags(), &ucli.IntFlag{Name: "n", Value: defaultLogLimit, Usage: "number of newest stored events to show (default: 20)"}),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return logCommand(state, ctx) },
		},
		{
			Name: "capture", Usage: "capture a bounded local Git comparison without running project code",
			Before:    outputBefore(state),
			ArgsUsage: "[--staged | --base REF [--target REF]]", Flags: append(globalFlags(),
				&ucli.BoolFlag{Name: "staged", Usage: "capture HEAD versus the index"},
				&ucli.StringFlag{Name: "base", Usage: "base commit for explicit merge-base capture"},
				&ucli.StringFlag{Name: "target", Usage: "target commit for explicit merge-base capture"},
				&ucli.StringSliceFlag{Name: "include-untracked", Usage: "select an exact non-ignored untracked file (repeatable)"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return captureCommand(state, ctx) },
		},
		{
			Name: "import", Usage: "import bounded Go test JSON or JUnit XML as reported evidence",
			Before:    outputBefore(state),
			ArgsUsage: "[FILE|-]", Flags: append(globalFlags(),
				&ucli.StringFlag{Name: "producer", Usage: "optional caller-supplied producer/version claim"},
				&ucli.StringFlag{Name: "format", Value: "auto", Usage: "report format: auto, go-test-json, or junit"},
				&ucli.StringFlag{Name: "captured-at", Usage: "optional caller-supplied RFC3339 capture time"},
				&ucli.StringFlag{Name: "snapshot", Usage: "bind the report to a validated snapshot ID (not proof of applicability)"},
				&ucli.IntFlag{Name: "offset", Usage: "first imported report card (default 0)"},
				&ucli.IntFlag{Name: "limit", Usage: "report cards to return (1-256; default 128)"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return importCommand(state, ctx) },
		},
		{
			Name: "inspect", Usage: "inspect a snapshot, receipt, comparison, report, or artifact by stable ID",
			Before:    outputBefore(state),
			ArgsUsage: "<stable-id> OR <base-snapshot-id> <candidate-snapshot-id>", Flags: append(append(globalFlags(), inspectionFlags()...), storedFlag()),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return inspectCommand(state, ctx, false) },
		},
		{
			Name: "compare", Usage: "compare a persisted execution receipt without running code",
			Before:    outputBefore(state),
			ArgsUsage: "<receipt-id>", Flags: globalFlags(),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return compareCommand(state, ctx) },
		},
		{
			Name: "export", Usage: "export a bounded machine-readable comparison or evidence page",
			Before:    outputBefore(state),
			ArgsUsage: "<stable-id> OR <base-snapshot-id> <candidate-snapshot-id>", Flags: append(globalFlags(), inspectionFlags()...),
			OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return inspectCommand(state, ctx, true) },
		},
		{
			Name: "run", Usage: "preview, explicitly authorize, and compare the frozen offline payment experiment",
			Before:    outputBefore(state),
			ArgsUsage: "[<base-snapshot-id> <candidate-snapshot-id>] (or --approve DIGEST or --plan-file FILE)",
			Flags: append(runFlags(),
				&ucli.StringFlag{Name: "plan-file", Usage: "reconstruct an exact previously saved execution preview"},
				&ucli.StringFlag{Name: "plan-out", Usage: "create a private file containing the exact preview for later approval"},
				&ucli.StringFlag{Name: "approve", Usage: "approve only this exact preview digest; never a blanket consent"},
			), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return runCommand(state, ctx) },
		},
		{
			Name: "config", Usage: "show effective configuration and per-setting provenance without execution",
			Before: outputBefore(state),
			Flags:  globalFlags(), OnUsageError: usageError, Action: func(ctx *ucli.Context) error { return configCommand(state, ctx) },
		},
		completionCommand(state),
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

// storedFlag skips the checkout freshness check of a bare stored summary.
func storedFlag() ucli.Flag {
	return &ucli.BoolFlag{Name: "stored", Usage: "show the stored summary without checking whether the checkout still matches"}
}

func inspectionFlags() []ucli.Flag {
	defaults := config.Defaults()
	return []ucli.Flag{
		&ucli.BoolFlag{Name: "raw-diff", Value: defaults.RawDiff, Usage: "include a bounded captured patch"},
		&ucli.IntFlag{Name: "diff-bytes", Value: defaults.DiffBytes, Usage: "maximum raw patch bytes (0-65536)"},
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
	var dockerConfigErr *config.Error
	if err != nil {
		if !errors.As(err, &dockerConfigErr) || (dockerConfigErr.Setting != "docker_binary" && dockerConfigErr.Setting != "docker_host") {
			return err
		}
		state.exit = ExitInvalid
		problem := fmt.Sprintf("invalid configuration setting %s (%s): %s", dockerConfigErr.Setting, dockerConfigErr.Source, dockerConfigErr.Reason)
		state.diagnostic = formatDiagnostic(problem, dockerConfigFix())
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
	settings := []configSetting{
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
	setup := dockerSetupStatus(cfg)
	if dockerConfigErr != nil {
		setup.Problems = append([]string{fmt.Sprintf("Invalid Docker configuration: %s.", dockerConfigErr.Reason)}, setup.Problems...)
	}
	return writeResult(state, "configuration", configurationResult{Settings: settings, Setup: setup})
}

type configSetting struct {
	Name   string `json:"name"`
	Value  any    `json:"value"`
	Source string `json:"source"`
}

type configurationResult struct {
	Settings []configSetting `json:"settings"`
	Setup    dockerSetup     `json:"docker_setup"`
}

func (result configurationResult) nextBlock() ([]nextCommand, bool) {
	if result.Setup.Apply != "" {
		return []nextCommand{
			next(result.Setup.Apply, "use the detected Docker CLI and socket in this shell"),
			next("after config", "check the settings again"),
		}, true
	}
	return []nextCommand{next("after status", "show this checkout's review state")}, true
}

func captureCommand(state *invocation, ctx *ucli.Context) error {
	if err := requireArgs(ctx, 0); err != nil {
		return err
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	state.project = cfg.Project
	options, err := captureOptionsFromFlags(ctx, "capture")
	if err != nil {
		return err
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
	pair := evidence.SnapshotPair{Base: result.Base.ID, Candidate: result.Candidate.ID}
	guidance, empty, err := buildEmptyCaptureGuidance(state.ctx, cfg.Project, pair, options, s)
	if err != nil {
		return operational("cannot inspect the captured change")
	}
	response := captureOutput{Base: summary(result.Base), Candidate: summary(result.Candidate), Index: index}
	if empty {
		response.Guidance = guidance
	} else {
		response.Next = []nextCommand{captureReviewNext(cfg.Project, pair, options), next("after diff --stored", "print this captured patch")}
	}
	return writeResult(state, "capture", response)
}

func captureOptionsFromFlags(ctx *ucli.Context, command string) (capture.Options, error) {
	options := capture.Options{Mode: evidence.WorkingTree}
	baseSet, targetSet := ctx.IsSet("base"), ctx.IsSet("target")
	if ctx.IsSet("staged") && ctx.Bool("staged") {
		if baseSet || targetSet {
			return options, invalid("--staged cannot be combined with --base/--target")
		}
		options.Mode = evidence.Index
	} else if baseSet || targetSet {
		if !baseSet || strings.TrimSpace(ctx.String("base")) == "" || (targetSet && strings.TrimSpace(ctx.String("target")) == "") {
			return options, invalidWithFix("--base REF is required when --target is used", "try: after "+command+" --base main [--target HEAD]")
		}
		options.Mode = evidence.MergeBase
		options.Base, options.Target = ctx.String("base"), "HEAD"
		if targetSet {
			options.Target = ctx.String("target")
		}
	}
	if ctx.IsSet("include-untracked") {
		options.IncludeUntracked = ctx.StringSlice("include-untracked")
	}
	return options, nil
}

func importCommand(state *invocation, ctx *ucli.Context) error {
	if ctx.NArg() > 1 {
		return invalidWithFix("import accepts one report input", "use after import FILE, after import -, or go test -json ./... | after import")
	}
	inputName := "-"
	if ctx.NArg() == 1 {
		inputName = ctx.Args().First()
	} else if state.tty {
		return invalidWithFix("missing test report input", "use after import FILE, after import -, or go test -json ./... | after import")
	}
	producer := ""
	if ctx.IsSet("producer") {
		producer = ctx.String("producer")
		if strings.TrimSpace(producer) != producer || producer == "" || len(producer) > 256 {
			return invalid("invalid producer provenance")
		}
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
	state.project = cfg.Project
	offset, limit, err := pageArgs(ctx, "offset", "limit", reportPageSize, maxReportPageSize)
	if err != nil {
		return err
	}
	var snapshot evidence.Digest
	var input io.Reader
	var inputFile *os.File
	if inputName == "-" {
		input = state.reader
	} else {
		inputFile, err = openInput(inputName)
		if err != nil {
			return operational("cannot read test report file")
		}
		defer inputFile.Close()
		input = inputFile
	}
	stopNotice := startElapsedNotice(state, "import", time.Second)
	defer stopNotice()
	importedAt := time.Now().UTC()
	if state.now != nil {
		importedAt = state.now().UTC()
	}
	// Parse before opening storage so empty or unrelated input changes nothing.
	report, err := gotestreport.ImportFormat(input, gotestreport.Metadata{Producer: producer, CapturedAt: capturedAt, ImportedAt: importedAt}, ctx.String("format"))
	if err != nil {
		return invalid(err.Error())
	}
	if len(report.Cards) == 0 && report.Dialect == gotestreport.Dialect {
		problem := "the input contains no go test -json events; nothing was imported"
		if report.OriginalBytes == 0 {
			problem = "the report input is empty; nothing was imported"
		}
		return invalidWithFix(problem, "pass -json to go test, for example: go test -json ./... | after import")
	}
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
		report.Metadata.Snapshot = snapshot
	}
	var binding *reportBindingView
	if ctx.IsSet("snapshot") {
		bound, err := store.Get[evidence.Snapshot](s, snapshot)
		if err != nil {
			return invalidWithFix("binding snapshot is unavailable", "use --snapshot ID from this private store")
		}
		binding = &reportBindingView{}
		for _, excluded := range bound.Excluded {
			if excluded.Reason == "untracked; not selected" {
				binding.UntrackedExcluded++
			}
		}
	} else {
		captured, captureErr := capture.Capture(state.ctx, cfg.Project, s, capture.Options{Mode: evidence.WorkingTree})
		if captureErr != nil {
			return importBindingCaptureFailure(captureErr)
		}
		report.Metadata.Snapshot = captured.Candidate.ID
		binding = &reportBindingView{Source: captured.Candidate.Source, CapturedNow: true}
		for _, excluded := range captured.Candidate.Excluded {
			if excluded.Reason == "untracked; not selected" {
				binding.UntrackedExcluded++
			}
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil || len(encoded) > store.MaxBlobBytes {
		return operational("imported report exceeds the private artifact limit")
	}
	artifact, err := s.PutArtifact(encoded, gotestreport.ArtifactChannel(report.Dialect), store.MaxBlobBytes)
	if err != nil || artifact.Completeness != evidence.Complete {
		return operational("cannot persist complete imported report")
	}
	view := reportView(artifact.Content, report, offset, limit)
	view.Binding = binding
	return writeResult(state, "import", view)
}

func importBindingCaptureFailure(err error) error {
	captured := captureFailure(err)
	var commandErr *exitError
	if !errors.As(captured, &commandErr) {
		return captured
	}
	problem := strings.TrimPrefix(commandErr.diagnostic, "after: ")
	problem = strings.TrimPrefix(problem, "capture failed: ")
	reason, fix, _ := strings.Cut(problem, " — ")
	return &exitError{code: ExitOperational, diagnostic: formatDiagnostic("capture failed: "+reason, fix+", or bind an existing candidate with --snapshot ID")}
}

func reportView(id evidence.Digest, report gotestreport.Report, offset, limit int) reportViewData {
	end := min(len(report.Cards), offset+limit)
	cards := []gotestreport.Card{}
	if offset < len(report.Cards) {
		cards = append(cards, report.Cards[offset:end]...)
	}
	counts := map[evidence.ReportOutcome]int{}
	for _, card := range report.Cards {
		counts[card.State.Report]++
	}
	return reportViewData{
		ID: id, SchemaVersion: report.SchemaVersion, Dialect: report.Dialect, Metadata: report.Metadata,
		OriginalDigest: report.OriginalDigest, Completeness: report.Completeness, ReportedOutcomes: counts,
		Cards: cards, CardOffset: offset, CardTotal: len(report.Cards), CardsMore: end < len(report.Cards),
		Diagnostics: report.Diagnostics, SuppressedDiagnostics: report.SuppressedDiagnostics,
	}
}

func inspectCommand(state *invocation, ctx *ucli.Context, exporting bool) error {
	if ctx.NArg() == 0 {
		return inspectNewestCommand(state, ctx, exporting)
	}
	if ctx.NArg() == 2 {
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
				return noIDMatch(ctx.Args().Get(0), "snapshot")
			}
			return operational("cannot open private evidence store for reading")
		}
		defer s.Close()
		base, err := resolveID(s, ctx.Args().Get(0), "snapshot")
		if err != nil {
			return err
		}
		candidate, err := resolveID(s, ctx.Args().Get(1), "snapshot")
		if err != nil {
			return err
		}
		view, err := snapshotBundle(s, base, candidate, options, !state.jsonOutput && !state.forceJSON && !exporting)
		if err != nil {
			return operational("snapshot comparison page is unavailable")
		}
		kind := "snapshot"
		if exporting {
			kind = "export"
		}
		return writeResult(state, kind, view)
	}
	if err := requireArgs(ctx, 1); err != nil {
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
			return noIDMatch(ctx.Args().Get(0), inspectKinds...)
		}
		return operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	id, err := resolveID(s, ctx.Args().Get(0), inspectKinds...)
	if err != nil {
		if fullID, ok := exactDigest(ctx.Args().Get(0)); ok {
			entries, listErr := s.List("snapshot", "capture", "scenario", "receipt", "comparison", "pin", "blob")
			if listErr == nil {
				found := false
				for _, entry := range entries {
					if entry.ID == fullID {
						found = true
						break
					}
				}
				if !found {
					return writeResult(state, "inspection", inspectionCardReference{ID: fullID})
				}
			}
		}
		return err
	}
	kind := "inspection"
	if exporting {
		kind = "export"
	}
	if raw, err := s.ReadPlan(id); err == nil {
		if exporting {
			return invalidWithFix("export requires a comparison ID", "try: after export COMPARISON_ID")
		}
		return writeResult(state, "plan", inspectPlan(id, raw))
	} else if !errors.Is(err, os.ErrNotExist) {
		return operational("stored execution plan is corrupt or unavailable")
	}
	if value, ok, err := optionalGet[evidence.Comparison](s, id); err != nil {
		if !exporting && !state.jsonOutput && !state.forceJSON {
			return writeResult(state, "inspection", inspectionCardReference{ID: id})
		}
		return operational("comparison record is corrupt or unavailable")
	} else if ok {
		receipt, err := store.Get[evidence.Receipt](s, value.Receipt)
		if err != nil {
			if !exporting && !state.jsonOutput && !state.forceJSON {
				return writeResult(state, "inspection", inspectionCardReference{ID: id})
			}
			return operational("comparison receipt is corrupt or unavailable")
		}
		var report *compare.Report
		if value.Details != nil {
			raw, err := s.ReadBlob(value.Details.Content)
			if err != nil {
				if !exporting && !state.jsonOutput && !state.forceJSON {
					return writeResult(state, "inspection", inspectionCardReference{ID: id})
				}
				return operational("comparison detail artifact is corrupt or unavailable")
			}
			var decoded compare.Report
			if err := strictJSON(raw, &decoded); err != nil {
				if !exporting && !state.jsonOutput && !state.forceJSON {
					return writeResult(state, "inspection", inspectionCardReference{ID: id})
				}
				return operational("comparison detail artifact is invalid")
			}
			report = &decoded
		}
		var snapshots *snapshotView
		if receipt.Snapshots.Base != "" {
			view, err := snapshotBundle(s, receipt.Snapshots.Base, receipt.Snapshots.Candidate, options, !state.jsonOutput && !state.forceJSON && !exporting)
			if err != nil {
				if !exporting && !state.jsonOutput && !state.forceJSON {
					return writeResult(state, "inspection", inspectionCardReference{ID: id})
				}
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
		return writeResult(state, kind, comparisonResult{Comparison: value, Receipt: receipt, Details: report, Snapshots: snapshots, InspectCard: true})
	}
	if value, ok, err := optionalGet[evidence.Receipt](s, id); err != nil {
		return operational("receipt record is corrupt or unavailable")
	} else if ok {
		return writeResult(state, "receipt", value)
	}
	if value, ok, err := optionalGet[evidence.Snapshot](s, id); err != nil {
		return operational("snapshot record is corrupt or unavailable")
	} else if ok {
		if exporting {
			return invalidWithFix("export requires a comparison ID", "try: after export COMPARISON_ID")
		}
		inspection := snapshotInspection{Snapshot: value}
		if !state.jsonOutput && !state.forceJSON {
			inspection.CaptureHistory = captureHistoryForSnapshot(s, id)
		}
		return writeResult(state, "snapshot", inspection)
	}
	if raw, err := s.ReadBlob(id); err == nil {
		var report gotestreport.Report
		if strictJSON(raw, &report) == nil && validImportedReport(report) {
			if options.cardOffset < 0 || options.cardLimit < 1 || options.cardLimit > maxReportPageSize {
				return invalid("report card page is outside supported bounds")
			}
			return writeResult(state, kind, reportView(id, report, options.cardOffset, options.cardLimit))
		}
		if !exporting && isImportedReportArtifact(s, id) {
			return writeResult(state, kind, inspectionCardReference{ID: id})
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
	options := inspectOptions{diffSize: cfg.DiffBytes, inventoryLimit: inventoryPageSize, cardLimit: reportPageSize}
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
	Using                   *resolvedIDs           `json:"using,omitempty"`
	Freshness               *freshness             `json:"freshness,omitempty"`
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
	if report.SchemaVersion != 1 || !gotestreport.SupportedDialect(report.Dialect) {
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
	if ctx.NArg() > 1 {
		return requireArgs(ctx, 1)
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	state.project = cfg.Project
	var id evidence.Digest
	var using *resolvedIDs
	if ctx.NArg() == 0 {
		readOnly, err := store.Open(cfg.Project, false, nil)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return missingStoredRecord("capture", "run after capture to create one")
			}
			return operational("cannot open private evidence store for reading")
		}
		capture, err := newestCaptureForStore(readOnly)
		if err != nil {
			readOnly.Close()
			return err
		}
		pair := evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate}
		receipt, err := newestReceiptForStore(readOnly, pair)
		readOnly.Close()
		if err != nil {
			return err
		}
		id = receipt.ID
		using = &resolvedIDs{Capture: capture.ID, Base: pair.Base, Candidate: pair.Candidate, Receipt: receipt.ID}
	} else {
		id, err = resolveIDFromProject(cfg.Project, ctx.Args().Get(0), "receipt")
		if err != nil {
			return err
		}
	}
	s, err := store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	comparisonRecord, err := compare.Run(s, id)
	if err != nil {
		return operational("comparison could not be persisted")
	}
	receipt, err := store.Get[evidence.Receipt](s, id)
	if err != nil {
		return operational("compared run receipt is corrupt or unavailable")
	}
	state.exit = comparisonExit(comparisonRecord.Outcome)
	var details *compare.Report
	if comparisonRecord.Details != nil {
		raw, err := s.ReadBlob(comparisonRecord.Details.Content)
		if err != nil {
			return operational("comparison details are unavailable")
		}
		var decoded compare.Report
		if err := strictJSON(raw, &decoded); err != nil {
			return operational("comparison details are invalid")
		}
		details = &decoded
	}
	return writeResult(state, "comparison", comparisonResult{Comparison: comparisonRecord, Receipt: receipt, Details: details, Using: using})
}

func resolveIDFromProject(project, value string, kinds ...string) (evidence.Digest, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", noIDMatch(value, kinds...)
		}
		return "", operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	return resolveID(s, value, kinds...)
}

type planInspection struct {
	ID           evidence.Digest `json:"id"`
	SizeBytes    int             `json:"size_bytes"`
	Consent      string          `json:"consent_summary,omitempty"`
	ConsentError string          `json:"consent_summary_error,omitempty"`
	Plan         json.RawMessage `json:"plan,omitempty"`
	Base64       string          `json:"base64"`
	raw          []byte          `json:"-"`
}

func formatPlanSize(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KiB", float64(size)/1024)
}

func inspectPlan(id evidence.Digest, raw []byte) planInspection {
	view := planInspection{ID: id, SizeBytes: len(raw), Base64: base64.StdEncoding.EncodeToString(raw), raw: append([]byte(nil), raw...)}
	if json.Valid(raw) {
		view.Plan = append(json.RawMessage(nil), raw...)
	}
	if summary, err := browser.ConsentSummary(raw); err != nil {
		view.ConsentError = terminal.Sanitize(err.Error())
	} else {
		view.Consent = strings.TrimSuffix(string(summary), "\n")
	}
	return view
}

type executionPreview struct {
	Authorization string          `json:"authorization_digest"`
	Status        string          `json:"status"`
	Plan          json.RawMessage `json:"plan"`
	PlanBytes     int             `json:"plan_size_bytes"`
	Consent       string          `json:"consent_summary,omitempty"`
	ConsentError  string          `json:"consent_summary_error,omitempty"`
	Using         *resolvedIDs    `json:"using,omitempty"`
	DockerSetup   dockerSetup     `json:"docker_setup"`
	Next          []nextCommand   `json:"next"`
}

func executionPreviewNext(status, digest string) []nextCommand {
	if status == "authorization_required" || status == "operator_declined" {
		return []nextCommand{
			next("after run --approve "+digest, "authorize exactly this stored plan"),
			next("after inspect "+digest, "inspect the exact stored plan"),
			next("after config", "check Docker setup without contacting an endpoint"),
		}
	}
	return []nextCommand{next("after inspect "+digest, "inspect the exact stored plan")}
}

func writeExecutionPreview(state *invocation, result executionPreview) error {
	result.Next = executionPreviewNext(result.Status, result.Authorization)
	return writeResult(state, "execution_preview", result)
}

func runCommand(state *invocation, ctx *ucli.Context) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	state.project = cfg.Project
	planPath := strings.TrimSpace(ctx.String("plan-file"))
	planOut := strings.TrimSpace(ctx.String("plan-out"))
	approvalSet := ctx.IsSet("approve") && strings.TrimSpace(ctx.String("approve")) != ""
	if ctx.IsSet("approve") && !validDigest(ctx.String("approve")) {
		return invalid("--approve requires the full digest: --approve sha256:<64 lowercase hex characters>; copy the authorization digest from the preview")
	}
	if planPath != "" && planOut != "" {
		return invalid("--plan-file cannot be combined with --plan-out")
	}
	if planPath != "" && ctx.Args().Len() != 0 {
		return invalidWithFix("saved-plan execution does not accept snapshot arguments", "use after run --plan-file FILE --approve FULL_DIGEST")
	}
	if approvalSet && planPath == "" && ctx.Args().Len() != 0 {
		return invalidWithFix("stored-plan approval does not accept snapshot arguments", "use after run --approve FULL_DIGEST")
	}
	if planPath == "" && !approvalSet && ctx.Args().Len() != 0 && ctx.Args().Len() != 2 {
		return invalidWithFix("run requires no arguments or two snapshot IDs", "try: after run or after run BASE CANDIDATE")
	}
	if planOut != "" && ctx.IsSet("approve") {
		return invalid("--plan-out is only valid when preparing a preview")
	}
	s, err := store.Open(cfg.Project, true, nil)
	if err != nil {
		return operational("cannot open private evidence store")
	}
	defer s.Close()
	var plan *runner.Plan
	var preview []byte
	var using *resolvedIDs
	switch {
	case planPath != "":
		preview, err = readBounded(planPath, store.MaxPlanBytes)
		if err != nil {
			return operational("cannot read saved execution plan")
		}
		plan, err = runner.PrepareFromPreview(s, preview)
		if err != nil {
			return invalid("saved execution plan is invalid or no longer matches stored snapshots")
		}
	case approvalSet:
		preview, err = s.ReadPlan(evidence.Digest(ctx.String("approve")))
		if errors.Is(err, os.ErrNotExist) {
			return invalidWithFix("no stored execution plan matches that full digest", "prepare a plan with after run, then copy its full authorization digest")
		}
		if err != nil {
			return operational("stored execution plan is corrupt or unavailable")
		}
		plan, err = runner.PrepareFromPreview(s, preview)
		if err != nil {
			return invalid("stored execution plan no longer matches its captured inputs")
		}
	default:
		var pair evidence.SnapshotPair
		if ctx.Args().Len() == 0 {
			captureRecord, captureErr := newestCaptureForStore(s)
			if captureErr != nil {
				return captureErr
			}
			pair = evidence.SnapshotPair{Base: captureRecord.Base, Candidate: captureRecord.Candidate}
			using = &resolvedIDs{Capture: captureRecord.ID, Base: pair.Base, Candidate: pair.Candidate}
		} else {
			base, resolveErr := resolveID(s, ctx.Args().Get(0), "snapshot")
			if resolveErr != nil {
				return resolveErr
			}
			candidate, resolveErr := resolveID(s, ctx.Args().Get(1), "snapshot")
			if resolveErr != nil {
				return resolveErr
			}
			pair = evidence.SnapshotPair{Base: base, Candidate: candidate}
		}
		plan, err = runner.Prepare(s, pair, cfg.Repetitions, sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes})
		if err != nil {
			return preparationFailure(err)
		}
	}
	preview, digest := plan.Preview()
	if len(preview) > store.MaxPlanBytes {
		return operational("execution plan exceeds the private storage limit")
	}
	if planOut != "" {
		if err := writePrivateNew(planOut, preview); err != nil {
			return operational("cannot save execution preview without overwriting an existing file")
		}
	}
	planID := evidence.Digest(digest)
	if err := s.PutPlan(planID, preview); err != nil {
		return operational("cannot store the immutable private execution plan")
	}
	consent, consentErr := browser.ConsentSummary(preview)
	result := executionPreview{
		Authorization: digest, Plan: json.RawMessage(preview), PlanBytes: len(preview), Using: using,
		DockerSetup: dockerSetupStatus(cfg),
	}
	if consentErr != nil {
		result.ConsentError = terminal.Sanitize(consentErr.Error())
	} else {
		result.Consent = strings.TrimSuffix(string(consent), "\n")
	}
	if approvalSet && ctx.String("approve") != digest {
		state.exit = ExitDenied
		result.Status = "authorization_mismatch"
		return writeExecutionPreview(state, result)
	}
	if !approvalSet {
		if cfg.Interactive && state.tty {
			if !prompt(state, digest, consent, consentErr, len(preview), using) {
				state.exit = ExitDenied
				result.Status = "operator_declined"
				return writeExecutionPreview(state, result)
			}
		} else {
			state.exit = ExitDenied
			result.Status = "authorization_required"
			return writeExecutionPreview(state, result)
		}
	}
	if len(result.DockerSetup.Problems) > 0 {
		return dockerSetupError(result.DockerSetup)
	}
	stopNotice := startElapsedNotice(state, "run", time.Second)
	runResult, runErr := (runner.Executor{Docker: sandbox.Docker{Binary: cfg.DockerBinary, Host: cfg.DockerHost}}).Run(state.ctx, s, plan, digest)
	stopNotice()
	if err := ensureReceipt(runResult.Receipt); err != nil {
		return operational("runner did not persist a usable receipt")
	}
	comparisonRecord, compareErr := compare.Run(s, runResult.Receipt.ID)
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
	if runErr != nil || runResult.Receipt.State.Execution != evidence.Completed {
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
	}{status, digest, preview, runResult.Receipt, comparisonRecord, details, runResult.Samples}
	if runErr != nil {
		state.exit = ExitOperational
	} else {
		state.exit = comparisonExit(comparisonRecord.Outcome)
	}
	return writeResult(state, "run", response)
}

// preparationFailure names the snapshot property that blocked a plan. Nothing
// has run, so these are invalid inputs rather than operational failures.
func preparationFailure(err error) error {
	switch {
	case errors.Is(err, runner.ErrUnsupportedProject):
		return invalidWithFix("this capture has no go.mod and app/main.go; after run supports only the offline payment experiment", "use after review to inspect this change without running it")
	case errors.Is(err, runner.ErrReservedPath):
		return invalidWithFix("the capture contains after-launch.go, a path the payment runner reserves", "rename after-launch.go, capture again, then retry after run")
	case errors.Is(err, runner.ErrIncompleteSnapshot):
		return invalidWithFix("the capture is incomplete, so it cannot execute", "run after inspect to see unsupported paths, then capture a complete change")
	case errors.Is(err, runner.ErrSnapshotBudget):
		return invalidWithFix("the capture exceeds the sandbox budget of 255 files and 8 MiB", "run the payment experiment on a smaller checkout")
	}
	return operational("cannot prepare the bounded payment execution plan")
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
