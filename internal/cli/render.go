package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/terminal"
	"github.com/rivo/uniseg"
)

type snapshotSummary struct {
	ID           evidence.Digest       `json:"id"`
	Source       evidence.SourceMode   `json:"source"`
	Completeness evidence.Completeness `json:"completeness"`
	Files        int                   `json:"files"`
	Excluded     int                   `json:"excluded"`
	Unsupported  int                   `json:"unsupported"`
	Diff         evidence.Digest       `json:"diff"`
	Limits       []string              `json:"limits"`
}

type inspectionCardReference struct {
	ID evidence.Digest `json:"id"`
}

type reportBindingView struct {
	Source            evidence.SourceMode
	CapturedNow       bool
	UntrackedExcluded int
}

type reportViewData struct {
	ID                    evidence.Digest                `json:"id"`
	SchemaVersion         int                            `json:"schema_version"`
	Dialect               string                         `json:"dialect"`
	Metadata              gotestreport.Metadata          `json:"metadata"`
	OriginalDigest        evidence.Digest                `json:"original_digest"`
	Completeness          evidence.Completeness          `json:"completeness"`
	ReportedOutcomes      map[evidence.ReportOutcome]int `json:"reported_outcomes"`
	Cards                 []gotestreport.Card            `json:"cards"`
	CardOffset            int                            `json:"card_offset"`
	CardTotal             int                            `json:"card_total"`
	CardsMore             bool                           `json:"cards_more"`
	Diagnostics           []gotestreport.Diagnostic      `json:"diagnostics"`
	SuppressedDiagnostics int                            `json:"suppressed_diagnostics"`
	Binding               *reportBindingView             `json:"-"`
}

type readableLine struct {
	text  string
	style terminal.Style
	full  bool
}

func textLine(value string, style terminal.Style) readableLine {
	return readableLine{text: value, style: style}
}

func fullLine(value string) readableLine { return readableLine{text: value, full: true} }

func readableRow(label, value string) readableLine {
	return textLine(fmt.Sprintf("  %-12s %s", label, value), terminal.Plain)
}

func cardInspectionLines(state *invocation, id evidence.Digest) ([]readableLine, bool) {
	if state.project == "" || id == "" {
		return nil, false
	}
	entries, err := browser.Inspect(state.project, id)
	if err != nil || len(entries) == 0 {
		return nil, false
	}
	if entries[0].Decision != "" {
		entries = entries[:1]
	}
	columns := min(max(state.columns, 40), 240)
	lines := make([]readableLine, 0, 24)
	ids := []evidence.Digest{}
	for _, entry := range entries {
		if len(entry.Sections) == 0 || entry.Sections[0].Name != "Card" {
			continue
		}
		lines = append(lines, textLine("Card · "+entry.Name, terminal.Strong))
		ids = append(ids, entry.IDs...)
		for _, section := range entry.Sections {
			if section.Name == "IDs" {
				continue
			}
			if section.Name != "Card" {
				lines = append(lines, textLine("  "+section.Name, terminal.Strong))
			}
			for _, part := range section.Parts {
				if part.Title != "" {
					lines = append(lines, textLine("  "+part.Title, terminal.Strong))
				}
				content, err := browser.ReadSection(state.ctx, state.project, browser.Section{Parts: []browser.Part{part}})
				if err != nil {
					lines = append(lines, textLine("    Card content unavailable", terminal.Attention))
					continue
				}
				for _, paragraph := range strings.Split(string(content), "\n") {
					for _, line := range terminal.Wrap(paragraph, columns-4) {
						lines = append(lines, textLine("    "+line, terminal.Plain))
					}
				}
			}
		}
	}
	if len(lines) == 0 {
		return nil, false
	}
	return addIDs(lines, ids...), true
}

func addIDs(lines []readableLine, ids ...evidence.Digest) []readableLine {
	seen := map[evidence.Digest]bool{}
	unique := make([]evidence.Digest, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return lines
	}
	lines = append(lines, textLine("IDs", terminal.Strong))
	for _, id := range unique {
		lines = append(lines, fullLine(string(id)))
	}
	return lines
}

func insertIDs(lines []readableLine, ids ...evidence.Digest) []readableLine {
	index := -1
	for i, line := range lines {
		if line.text == "IDs" {
			index = i
		}
	}
	if index < 0 {
		return addIDs(lines, ids...)
	}
	position := index + 1
	seen := map[evidence.Digest]bool{}
	for position < len(lines) && lines[position].full {
		seen[evidence.Digest(lines[position].text)] = true
		position++
	}
	additions := []readableLine{}
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			additions = append(additions, fullLine(string(id)))
		}
	}
	oldLength := len(lines)
	lines = append(lines, make([]readableLine, len(additions))...)
	copy(lines[position+len(additions):], lines[position:oldLength])
	copy(lines[position:position+len(additions)], additions)
	return lines
}

func shortID(id evidence.Digest) string {
	value := strings.TrimPrefix(string(id), "sha256:")
	if len(value) > 8 {
		return value[:8]
	}
	return value
}

func sourceName(source evidence.SourceMode, commit string) string {
	switch source {
	case evidence.Commit:
		if commit != "" {
			return "commit " + shortCommit(commit)
		}
		return "commit"
	case evidence.WorkingTree:
		return "working tree"
	case evidence.Index:
		return "index"
	case evidence.MergeBase:
		return "merge base"
	default:
		return string(source)
	}
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func (s *invocation) safeLine(value string, style terminal.Style) string {
	if s.stdoutTTY {
		return s.theme.Render(value, s.columns, style, false)
	}
	return terminal.Sanitize(value)
}

func writeReadable(state *invocation, kind string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil || len(encoded) > MaxCLIOutput {
		return operational("readable response exceeds output limit")
	}
	lines, err := readableLines(state, kind, encoded, data)
	if err != nil {
		return operational("cannot render readable result")
	}
	lines = appendNextLines(state, kind, encoded, data, lines)
	var output bytes.Buffer
	for _, line := range lines {
		if line.full {
			output.WriteString(terminal.Sanitize(line.text))
		} else {
			output.WriteString(state.safeLine(line.text, line.style))
		}
		output.WriteByte('\n')
		if output.Len() > MaxCLIOutput {
			return operational("readable response exceeds output limit")
		}
	}
	n, err := state.stdout.Write(output.Bytes())
	if err != nil || n != output.Len() {
		return operational("cannot write readable response")
	}
	return nil
}

func readableLines(state *invocation, kind string, raw []byte, original any) ([]readableLine, error) {
	switch kind {
	case "status":
		var view statusView
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		return statusLines(state, view), nil
	case "log":
		var view logView
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		return logLines(state, view), nil
	case "pins":
		var view pinListView
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		return pinListLines(state, view), nil
	case "configuration":
		var result struct {
			Settings []struct {
				Name   string `json:"name"`
				Value  any    `json:"value"`
				Source string `json:"source"`
			} `json:"settings"`
			Setup dockerSetup `json:"docker_setup"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		lines := []readableLine{textLine("Effective AFTER configuration", terminal.Strong)}
		for _, setting := range result.Settings {
			lines = append(lines, readableRow(setting.Name, fmt.Sprintf("%v · %s", setting.Value, setting.Source)))
		}
		return append(lines, dockerSetupReadableLines(state, result.Setup)...), nil
	case "capture":
		var result struct {
			Base      snapshotSummary  `json:"base_snapshot"`
			Candidate snapshotSummary  `json:"candidate_snapshot"`
			Index     *snapshotSummary `json:"index_snapshot,omitempty"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		lines := []readableLine{textLine(fmt.Sprintf("Captured candidate %s (%s) against base %s (%s)", shortID(result.Candidate.ID), sourceName(result.Candidate.Source, ""), shortID(result.Base.ID), sourceName(result.Base.Source, "")), terminal.Strong)}
		lines = appendSnapshotSummary(lines, "Base", result.Base)
		lines = appendSnapshotSummary(lines, "Candidate", result.Candidate)
		if result.Index != nil {
			lines = appendSnapshotSummary(lines, "Index", *result.Index)
		}
		return lines, nil
	case "import", "inspection":
		if reference, ok := original.(inspectionCardReference); ok {
			if lines, ok := cardInspectionLines(state, reference.ID); ok {
				return lines, nil
			}
		}
		var report reportViewData
		if err := json.Unmarshal(raw, &report); err == nil && report.ID != "" && report.Cards != nil {
			if imported, ok := original.(reportViewData); ok {
				report.Binding = imported.Binding
			}
			if kind == "inspection" {
				if lines, ok := cardInspectionLines(state, report.ID); ok {
					return lines, nil
				}
			}
			lines := reportLines(state, report)
			if kind == "inspection" {
				lines = addIDs(lines, report.ID, report.Metadata.Snapshot)
			}
			return lines, nil
		}
		if kind == "inspection" {
			return nil, fmt.Errorf("unsupported inspection result")
		}
		return nil, fmt.Errorf("invalid imported report result")
	case "receipt":
		var receipt evidence.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return nil, err
		}
		if lines, ok := cardInspectionLines(state, receipt.ID); ok {
			return lines, nil
		}
		lines := []readableLine{textLine(fmt.Sprintf("Receipt %s · %s / %s · %s", shortID(receipt.ID), receipt.State.Kind, receipt.State.Execution, receipt.State.Comparison), badgeStyle(string(receipt.State.Comparison)))}
		lines = append(lines, readableRow("Snapshots", fmt.Sprintf("%s → %s", shortID(receipt.Snapshots.Base), shortID(receipt.Snapshots.Candidate))))
		lines = append(lines, readableRow("Started", state.formatTime(receipt.StartedAt)))
		lines = append(lines, readableRow("Finished", state.formatTime(receipt.FinishedAt)))
		lines = append(lines, readableRow("Artifacts", fmt.Sprintf("%d · %s", len(receipt.Artifacts), receipt.Completeness)))
		lines = appendLimits(lines, receipt.Limits)
		ids := []evidence.Digest{receipt.ID, receipt.Snapshots.Base, receipt.Snapshots.Candidate}
		for _, artifact := range receipt.Artifacts {
			ids = append(ids, artifact.Content)
		}
		return addIDs(lines, ids...), nil
	case "snapshot":
		return snapshotLines(state, raw, original)
	case "artifact":
		return artifactLines(state, raw)
	case "comparison":
		if inspection, ok := original.(comparisonResult); ok && inspection.InspectCard {
			if lines, ok := cardInspectionLines(state, inspection.Comparison.ID); ok {
				return lines, nil
			}
		}
		return comparisonLines(state, raw)
	case "review":
		var view review.View
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		lines := reviewLines(state, view)
		if inspection, ok := original.(reviewInspection); ok {
			if cardLines, ok := cardInspectionLines(state, inspection.Pin.ID); ok {
				lines = cardLines
			}
			if inspection.Using != nil {
				lines = append([]readableLine{usingRunLine(*inspection.Using)}, lines...)
				lines = insertIDs(lines, inspection.Using.Capture)
			}
			if inspection.HeadsUnavailable {
				lines = append(lines, textLine("Newer heads unavailable: pin history is corrupt or exceeds lookup limits; showing the requested revision.", terminal.Attention))
			} else if len(inspection.NewerHeads) > 0 {
				lines = append(lines, textLine("Historical revision — newer heads (not selected):", terminal.Attention))
				for _, id := range inspection.NewerHeads {
					lines = append(lines, readableRow("Head", shortID(id)))
				}
				lines = insertIDs(lines, inspection.NewerHeads...)
			}
		}
		return lines, nil
	case "execution_preview":
		var preview executionPreview
		if err := json.Unmarshal(raw, &preview); err != nil {
			return nil, err
		}
		return previewLines(state, preview), nil
	case "plan":
		if view, ok := original.(planInspection); ok {
			return planInspectionLines(state, view), nil
		}
		return nil, fmt.Errorf("stored execution plan is unavailable")
	case "run":
		var result struct {
			Status        string              `json:"status"`
			Authorization string              `json:"authorization_digest"`
			Plan          json.RawMessage     `json:"plan"`
			Receipt       evidence.Receipt    `json:"receipt"`
			Comparison    evidence.Comparison `json:"comparison"`
			Details       compare.Report      `json:"details"`
			Samples       []runner.Sample     `json:"samples"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		lines := []readableLine{textLine(fmt.Sprintf("Run %s · %s · %s", shortID(result.Receipt.ID), result.Status, result.Comparison.Outcome), badgeStyle(string(result.Comparison.Outcome)))}
		lines = append(lines, readableRow("Snapshots", fmt.Sprintf("%s → %s", shortID(result.Receipt.Snapshots.Base), shortID(result.Receipt.Snapshots.Candidate))))
		lines = comparisonDetailLines(lines, result.Details)
		lines = appendLimits(lines, result.Comparison.Limits)
		return lines, nil
	default:
		return nil, fmt.Errorf("no readable renderer for %s", kind)
	}
}

func statusLines(state *invocation, view statusView) []readableLine {
	if view.Capture == nil {
		return []readableLine{textLine("No capture has been stored for this checkout.", terminal.Strong)}
	}
	capture := view.Capture
	base, candidate := view.Base, view.Candidate
	baseName, candidateName := "unknown", "unknown"
	if base != nil {
		baseName = sourceName(base.Source, base.Commit)
	}
	if candidate != nil {
		candidateName = sourceName(candidate.Source, candidate.Commit)
	}
	lines := []readableLine{textLine(fmt.Sprintf("Checkout · base %s (%s) → candidate %s (%s)", shortID(capture.Base), baseName, shortID(capture.Candidate), candidateName), terminal.Strong)}
	captured := fmt.Sprintf("%s · %d %s changed", state.formatTime(capture.CapturedAt), view.ChangedPaths, plural(view.ChangedPaths, "path"))
	if view.UntrackedExcluded {
		captured += " · untracked paths excluded"
	} else if view.Capture.SelectedUntracked > 0 {
		captured += fmt.Sprintf(" · %d untracked paths selected", view.Capture.SelectedUntracked)
	} else {
		captured += " · no untracked paths selected"
	}
	lines = append(lines, readableRow("Captured", captured))
	if view.SavedReview != nil && *view.SavedReview != (evidence.SnapshotPair{Base: capture.Base, Candidate: capture.Candidate}) {
		lines = append(lines, readableRow("Saved review", fmt.Sprintf("%s → %s", shortID(view.SavedReview.Base), shortID(view.SavedReview.Candidate))))
	}
	if view.Receipt == nil {
		if view.PriorRuns {
			lines = append(lines, readableRow("Behavior", "no run for this pair · earlier runs exist for other captures"))
		} else {
			lines = append(lines, readableRow("Behavior", "no run for this pair"))
		}
	} else if view.Comparison != nil {
		lines = append(lines, readableRow("Behavior", fmt.Sprintf("run %s · %s · %s", shortID(view.Receipt.ID), view.Receipt.State.Execution, view.Comparison.Outcome)))
		if view.Details != nil && len(view.Details.Witnesses) > 0 {
			lines = comparisonDetailLines(lines, *view.Details)
		} else {
			lines = append(lines, textLine("  Stored comparison has no detailed witnesses.", terminal.Muted))
		}
		lines = appendLimits(lines, view.Comparison.Limits)
	} else {
		lines = append(lines, readableRow("Behavior", fmt.Sprintf("run %s · %s / %s · no stored comparison", shortID(view.Receipt.ID), view.Receipt.State.Execution, view.Receipt.State.Comparison)))
		lines = appendLimits(lines, view.Receipt.Limits)
	}
	if len(view.Pins) == 0 {
		lines = append(lines, readableRow("Pins", "none for this pair"))
	} else {
		pending := 0
		for _, pin := range view.Pins {
			if pin.Decision == evidence.Reopened || pin.Applicability != evidence.Current || pin.MissingCurrentResult {
				pending++
			}
		}
		lines = append(lines, readableRow("Pins", fmt.Sprintf("%d · %d need another look", len(view.Pins), pending)))
		for _, pin := range view.Pins {
			expectation := terminal.Line(pin.Expectation, 46)
			lines = append(lines, textLine(fmt.Sprintf("  [%s] %s  %s", strings.ToUpper(string(pin.Decision)), shortID(pin.ID), expectation), badgeStyle(string(pin.Decision))))
		}
	}
	if len(view.Reports) == 0 {
		lines = append(lines, readableRow("Reports", "none for this candidate"))
	} else {
		for _, report := range view.Reports {
			producer := report.Producer
			if producer == "" {
				producer = "producer not stated"
			}
			lines = append(lines, readableRow("Reports", fmt.Sprintf("%s · %d pass · %d fail · %d skip · reported, not observed", producer, report.Pass, report.Fail, report.Skip)))
		}
	}
	lines = appendLimits(lines, view.Limits)
	return lines
}

func logLines(state *invocation, view logView) []readableLine {
	lines := []readableLine{textLine("Recent stored records", terminal.Strong)}
	for _, row := range view.Rows {
		when := state.formatTime(row.At)
		id := row.ID
		description := ""
		switch row.Kind {
		case "capture":
			id = row.Candidate
			description = fmt.Sprintf("%s against %s", strings.ReplaceAll(string(row.Mode), "_", " "), shortID(row.Base))
			if row.Paths != 0 {
				description += fmt.Sprintf(" · %d %s", row.Paths, plural(row.Paths, "path"))
			}
		case "run":
			description = fmt.Sprintf("%s → %s · %s", shortID(row.Base), shortID(row.Candidate), row.Outcome)
			if row.Summary != "" {
				description += " · " + row.Summary
			} else if row.Outcome == evidence.NotCompared || row.Outcome == evidence.Incomparable {
				description += " · no conclusive comparison"
			}
		case "report":
			description = fmt.Sprintf("go test · %d pass · %d fail · %d skip · reported, not observed", row.Pass, row.Fail, row.Skip)
			if row.Candidate != "" {
				description += " · bound to " + shortID(row.Candidate)
			}
		case "pin":
			description = strings.ToUpper(string(row.Decision))
			if row.Action != "" {
				description += " · " + row.Action
			}
			if row.Receipt != "" {
				description += " " + shortID(row.Receipt)
			}
			description += " · " + terminal.Line(row.Expectation, 32)
		}
		lines = append(lines, textLine(fmt.Sprintf("%-12s %-8s %-8s %s", when, row.Kind, shortID(id), description), terminal.Plain))
	}
	if view.Total == 0 {
		lines = append(lines, textLine("No stored history yet.", terminal.Muted))
	} else {
		lines = append(lines, readableRow("History", fmt.Sprintf("%d newest of %d stored events", view.Shown, view.Total)))
	}
	return lines
}

func pinListLines(state *invocation, view pinListView) []readableLine {
	lines := []readableLine{textLine(fmt.Sprintf("%d pin %s (computed heads)", len(view.Pins), plural(len(view.Pins), "head")), terminal.Strong)}
	for _, item := range view.Pins {
		pin := item.Pin
		decision := strings.ToUpper(string(pin.Decision))
		result := "no current result"
		if item.CurrentReceipt != nil {
			result = "current result attached"
		}
		if item.MissingCurrentResult {
			result = "no current result"
		}
		expectation := terminal.Line(pin.Expectation, max(8, 80-52))
		lines = append(lines, textLine(fmt.Sprintf("  [%s] %-8s %s · %s", decision, shortID(pin.ID), expectation, result), badgeStyle(string(pin.Decision))))
	}
	if len(view.Pins) == 0 {
		lines = append(lines, textLine("No pins have been stored.", terminal.Muted))
	}
	return lines
}

func usingCaptureLine(ids resolvedIDs) readableLine {
	return textLine(fmt.Sprintf("Using the newest capture: base %s → candidate %s", shortID(ids.Base), shortID(ids.Candidate)), terminal.Muted)
}

func usingRunLine(ids resolvedIDs) readableLine {
	return textLine(fmt.Sprintf("Using the newest run of base %s → candidate %s", shortID(ids.Base), shortID(ids.Candidate)), terminal.Muted)
}

func appendNextLines(state *invocation, kind string, raw []byte, original any, lines []readableLine) []readableLine {
	var data struct {
		Next []nextCommand `json:"next"`
	}
	_ = json.Unmarshal(raw, &data)
	if len(data.Next) == 0 {
		data.Next = suggestedNext(kind, raw, original)
	}
	lines = append(lines, textLine("Next", terminal.Strong))
	if len(data.Next) == 0 {
		return append(lines, textLine("  No further command is suggested.", terminal.Muted))
	}
	for _, item := range data.Next[:min(3, len(data.Next))] {
		command := item.Command
		if state.suggestionFlags != "" && !strings.Contains(command, "--project ") && !strings.Contains(command, "--config ") {
			command += state.suggestionFlags
		}
		lines = append(lines, fullLine("  "+command))
		if item.Description != "" {
			lines = append(lines, textLine("    "+item.Description, terminal.Muted))
		}
	}
	return lines
}

func suggestedNext(kind string, raw []byte, original any) []nextCommand {
	switch kind {
	case "capture":
		var data struct {
			Candidate snapshotSummary `json:"candidate_snapshot"`
		}
		if json.Unmarshal(raw, &data) == nil && data.Candidate.ID != "" {
			return []nextCommand{
				next("after review "+string(data.Candidate.ID), "open this captured change"),
				next("after diff", "print this captured patch"),
			}
		}
	case "import":
		var data reportViewData
		if json.Unmarshal(raw, &data) == nil && data.ID != "" {
			return []nextCommand{next("after inspect "+string(data.ID), "inspect the reported test results")}
		}
	case "receipt":
		var receipt evidence.Receipt
		if json.Unmarshal(raw, &receipt) == nil && receipt.ID != "" {
			return []nextCommand{next("after compare "+string(receipt.ID), "compare the stored run")}
		}
	case "comparison":
		var result comparisonResult
		if json.Unmarshal(raw, &result) == nil && result.Comparison.ID != "" {
			return []nextCommand{next("after inspect "+string(result.Comparison.ID), "inspect the stored comparison")}
		}
	case "snapshot":
		var pair snapshotView
		if json.Unmarshal(raw, &pair) == nil && pair.Base != "" && pair.Candidate != "" {
			return []nextCommand{next("after review "+string(pair.Base)+" "+string(pair.Candidate), "review this snapshot pair")}
		}
		var snapshot evidence.Snapshot
		if json.Unmarshal(raw, &snapshot) == nil && snapshot.ID != "" {
			return []nextCommand{next("after review "+string(snapshot.ID), "open the captured snapshot")}
		}
	case "inspection":
		var report reportViewData
		if json.Unmarshal(raw, &report) == nil && report.ID != "" {
			return []nextCommand{next("after inspect "+string(report.ID), "inspect the reported test results")}
		}
	case "artifact":
		var data struct {
			ID evidence.Digest `json:"id"`
		}
		if json.Unmarshal(raw, &data) == nil && data.ID != "" {
			return []nextCommand{next("after inspect "+string(data.ID), "inspect the stored artifact")}
		}
	case "review":
		var data review.View
		if json.Unmarshal(raw, &data) == nil && data.Pin.ID != "" {
			return []nextCommand{next("after pin "+string(data.Pin.ID), "inspect this exact pin revision")}
		}
	case "configuration":
		return []nextCommand{next("after --help", "see available commands")}
	case "execution_preview":
		var data struct {
			Authorization string `json:"authorization_digest"`
			Status        string `json:"status"`
		}
		if json.Unmarshal(raw, &data) == nil && validDigest(data.Authorization) {
			if data.Status == "authorization_required" || data.Status == "operator_declined" {
				return []nextCommand{
					next("after run --approve "+data.Authorization, "authorize exactly this stored plan"),
					next("after inspect "+data.Authorization, "inspect the exact stored plan"),
					next("after config", "check Docker setup without contacting an endpoint"),
				}
			}
			return []nextCommand{next("after inspect "+data.Authorization, "inspect the exact stored plan")}
		}
	case "plan":
		var data planInspection
		if json.Unmarshal(raw, &data) == nil && data.ID != "" {
			return []nextCommand{
				next("after run --approve "+string(data.ID), "authorize exactly this stored plan"),
				next("after config", "check Docker setup without contacting an endpoint"),
			}
		}
	case "run":
		var data struct {
			Comparison evidence.Comparison `json:"comparison"`
		}
		if json.Unmarshal(raw, &data) == nil && data.Comparison.ID != "" {
			return []nextCommand{next("after inspect "+string(data.Comparison.ID), "inspect the stored comparison")}
		}
	}
	return []nextCommand{next("after --help", "see available commands")}
}

func appendSnapshotSummary(lines []readableLine, label string, snapshot snapshotSummary) []readableLine {
	lines = append(lines, readableRow(label, fmt.Sprintf("%s · %d %s · %s · %d excluded · %d unsupported", shortID(snapshot.ID), snapshot.Files, plural(snapshot.Files, "path"), snapshot.Completeness, snapshot.Excluded, snapshot.Unsupported)))
	return appendLimits(lines, snapshot.Limits)
}

func (s *invocation) formatTime(value time.Time) string {
	if value.IsZero() {
		return "not recorded"
	}
	location := s.location
	if location == nil {
		location = time.Local
	}
	value = value.In(location)
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	now = now.In(location)
	if value.Year() == now.Year() && value.YearDay() == now.YearDay() {
		return value.Format("15:04:05")
	}
	return value.Format("Jan 2 15:04")
}

func reportLines(state *invocation, report reportViewData) []readableLine {
	passes, failures, skips := report.ReportedOutcomes[evidence.ReportPass], report.ReportedOutcomes[evidence.ReportFail], report.ReportedOutcomes[evidence.ReportSkip]
	lines := []readableLine{textLine(fmt.Sprintf("Imported report %s · %d pass · %d fail · %d skip", shortID(report.ID), passes, failures, skips), terminal.Reported)}
	producer := report.Metadata.Producer
	if producer == "" {
		lines = append(lines, readableRow("Producer", "not stated"))
		lines = append(lines, textLine("             add --producer TEXT to record a caller claim", terminal.Muted))
	} else {
		lines = append(lines, readableRow("Producer", producer+" · caller claim, unverified"))
	}
	lines = append(lines, readableRow("Imported", state.formatTime(report.Metadata.ImportedAt)))
	if report.Metadata.Snapshot == "" {
		lines = append(lines, readableRow("Binding", "not supplied"))
	} else {
		lines = append(lines, readableRow("Binding", shortID(report.Metadata.Snapshot)+" · caller claim"))
		lines = append(lines, textLine("             not proof of where tests ran", terminal.Muted))
	}
	if report.Metadata.CapturedAt != nil {
		lines = append(lines, readableRow("Test time", state.formatTime(*report.Metadata.CapturedAt)+" · caller-supplied"))
	}
	if report.Binding != nil && report.Binding.CapturedNow {
		source := strings.ReplaceAll(string(report.Binding.Source), "_", " ")
		lines = append(lines, readableRow("Capture", source+" captured at import"))
	}
	if report.Binding != nil && report.Binding.UntrackedExcluded > 0 {
		count := report.Binding.UntrackedExcluded
		lines = append(lines, readableRow("Untracked", fmt.Sprintf("%d %s excluded; tests may have used them", count, plural(count, "path"))))
	}
	lines = append(lines, textLine("Reported in go test JSON; AFTER did not run or observe these tests", terminal.Muted))
	for _, card := range report.Cards {
		title := card.Test
		if title == "" {
			title = card.Package
		}
		lines = append(lines, textLine(fmt.Sprintf("  %s · %s · %s", title, card.Scope, card.State.Report), badgeStyle(string(card.State.Report))))
	}
	if len(report.Diagnostics) > 0 || report.SuppressedDiagnostics > 0 {
		lines = append(lines, readableRow("Diagnostics", fmt.Sprintf("%d · %d suppressed", len(report.Diagnostics), report.SuppressedDiagnostics)))
	}
	return lines
}

func snapshotLines(state *invocation, raw []byte, original any) ([]readableLine, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil {
		return nil, err
	}
	if _, pair := shape["base_snapshot"]; pair {
		var view snapshotView
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		lines := []readableLine{textLine(fmt.Sprintf("Captured pair %s → %s", shortID(view.Base), shortID(view.Candidate)), terminal.Strong)}
		if view.Using != nil {
			lines = append(lines, usingCaptureLine(*view.Using))
		}
		if records, ok := original.(snapshotView); ok {
			if records.BaseRecord.ID != "" {
				lines = appendSnapshotRecord(state, lines, "Base", records.BaseRecord, records.BaseCaptureHistory)
			}
			if records.CandidateRecord.ID != "" {
				lines = appendSnapshotRecord(state, lines, "Candidate", records.CandidateRecord, records.CandidateCaptureHistory)
			}
		}
		lines = append(lines, readableRow("Inventory", fmt.Sprintf("%d %s · %d shown%s", view.InventoryTotal, plural(view.InventoryTotal, "path"), len(view.Inventory), moreSuffix(view.InventoryMore))))
		for _, item := range view.Inventory {
			change := changeMark(item.Change)
			path := item.Path
			if state.stdoutTTY {
				path = middlePath(path, max(12, state.columns-8))
			}
			value := fmt.Sprintf("  %-2s %s", change, path)
			if item.PotentialOracle {
				value += " · potential oracle"
			}
			if item.Binary {
				value += " · binary"
			}
			for _, limit := range item.Limits {
				value += " · " + limit
			}
			lines = append(lines, textLine(value, terminal.Plain))
		}
		if view.Diff.Available {
			lines = append(lines, readableRow("Raw diff", fmt.Sprintf("%d bytes · page %d–%d%s", view.Diff.Total, view.Diff.Offset, view.Diff.Next, moreSuffix(view.Diff.More))))
		} else {
			lines = append(lines, readableRow("Raw diff", "not included; use --json to retrieve a page"))
		}
		lines = appendLimits(lines, view.Limits)
		ids := []evidence.Digest{view.Base, view.Candidate}
		if view.Using != nil {
			ids = append(ids, view.Using.Capture)
		}
		return addIDs(lines, ids...), nil
	}
	var snapshot evidence.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	lines := []readableLine{textLine(fmt.Sprintf("Snapshot %s · %s", shortID(snapshot.ID), sourceName(snapshot.Source, snapshot.Commit)), terminal.Strong)}
	if inspection, ok := original.(snapshotInspection); ok {
		lines = appendSnapshotCaptureHistory(state, lines, inspection.CaptureHistory)
	} else {
		lines = appendSnapshotCaptureHistory(state, lines, snapshotCaptureHistory{})
	}
	lines = append(lines, readableRow("Paths", fmt.Sprintf("%d %s · %s · %d excluded · %d unsupported", len(snapshot.Files), plural(len(snapshot.Files), "path"), snapshot.Completeness, len(snapshot.Excluded), len(snapshot.Unsupported))))
	lines = appendLimits(lines, snapshot.Limits)
	return addIDs(lines, snapshot.ID, snapshot.IndexSnapshot), nil
}

func appendSnapshotRecord(state *invocation, lines []readableLine, label string, snapshot evidence.Snapshot, history snapshotCaptureHistory) []readableLine {
	lines = append(lines, readableRow(label, fmt.Sprintf("%s · %s · %d %s · %s · %d excluded · %d unsupported", shortID(snapshot.ID), sourceName(snapshot.Source, snapshot.Commit), len(snapshot.Files), plural(len(snapshot.Files), "path"), snapshot.Completeness, len(snapshot.Excluded), len(snapshot.Unsupported))))
	return appendSnapshotCaptureHistory(state, lines, history)
}

func appendSnapshotCaptureHistory(state *invocation, lines []readableLine, history snapshotCaptureHistory) []readableLine {
	switch {
	case history.Unavailable:
		lines = append(lines, readableRow("Captured", "unavailable (capture history could not be read)"))
	case len(history.Records) == 0 && history.Limited:
		lines = append(lines, readableRow("Captured", "unavailable (history lookup limit reached)"))
	case len(history.Records) == 0:
		lines = append(lines, readableRow("Captured", "unavailable (legacy snapshot; no capture record)"))
	default:
		for _, record := range history.Records {
			value := fmt.Sprintf("%s · %s · record %s", state.formatTime(record.CapturedAt), record.Mode, shortID(record.ID))
			if record.SelectedUntracked > 0 {
				value += fmt.Sprintf(" · %d selected untracked", record.SelectedUntracked)
			}
			lines = append(lines, readableRow("Captured", value))
		}
	}
	if history.More {
		lines = append(lines, readableRow("Capture history", "additional events omitted by display limit"))
	}
	if history.Limited {
		lines = append(lines, readableRow("Capture history", "lookup bound reached; displayed history may be incomplete"))
	}
	return lines
}

func plural(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}

func changeMark(change string) string {
	switch change {
	case "added":
		return "A"
	case "deleted":
		return "D"
	case "modified":
		return "M"
	case "renamed":
		return "R"
	default:
		return "?"
	}
}

func middlePath(path string, width int) string {
	path = terminal.Sanitize(path)
	if uniseg.StringWidth(path) <= width || width < 8 {
		return path
	}
	base := path[strings.LastIndex(path, "/")+1:]
	baseWidth := uniseg.StringWidth(base)
	if baseWidth+2 >= width {
		return terminal.Line(base, width)
	}
	prefixWidth := width - baseWidth - 2
	prefix := path
	for uniseg.StringWidth(prefix) > prefixWidth && len(prefix) > 0 {
		_, size := runeFirst(prefix)
		prefix = prefix[size:]
	}
	return "…/" + prefix + base
}

func runeFirst(value string) (rune, int) {
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func moreSuffix(more bool) string {
	if more {
		return " · more"
	}
	return ""
}

func artifactLines(state *invocation, raw []byte) ([]readableLine, error) {
	var artifact struct {
		ID       evidence.Digest `json:"id"`
		Offset   int             `json:"offset"`
		Next     int             `json:"next"`
		Total    int             `json:"total"`
		More     bool            `json:"more"`
		Base64   string          `json:"base64"`
		Document json.RawMessage `json:"document,omitempty"`
	}
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, err
	}
	content, err := base64.StdEncoding.DecodeString(artifact.Base64)
	if err != nil {
		return nil, err
	}
	lines := []readableLine{textLine(fmt.Sprintf("Artifact %s · %d bytes · page %d–%d of %d%s", shortID(artifact.ID), len(content), artifact.Offset, artifact.Next, artifact.Total, moreSuffix(artifact.More)), terminal.Strong)}
	if len(artifact.Document) != 0 && artifact.Offset == 0 && artifact.Next == artifact.Total {
		content = append([]byte(nil), artifact.Document...)
	}
	if doc, err := terminal.NewDocument(content); err == nil && doc.AutoHex() {
		for row := 0; row < doc.HexRows(); row++ {
			lines = append(lines, textLine("  "+doc.HexLine(row), terminal.Plain))
		}
	} else {
		contentLines := bytes.Split(content, []byte{'\n'})
		if len(contentLines) > 1 && len(contentLines[len(contentLines)-1]) == 0 {
			contentLines = contentLines[:len(contentLines)-1]
		}
		for i, line := range contentLines {
			payload := string(line)
			if state.stdoutTTY {
				width := max(0, min(240, state.columns-10))
				payload = terminal.Line(payload, width)
			} else {
				payload = terminal.Sanitize(payload)
			}
			lines = append(lines, fullLine(fmt.Sprintf("  %4d │ %s", i+1, payload)))
		}
	}
	return addIDs(lines, artifact.ID), nil
}

func comparisonLines(state *invocation, raw []byte) ([]readableLine, error) {
	var result comparisonResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	comparison := result.Comparison
	lines := []readableLine{}
	if result.Using != nil {
		lines = append(lines, usingRunLine(*result.Using))
	}
	lines = append(lines, textLine(fmt.Sprintf("Comparison %s · %s · %s", shortID(comparison.ID), comparison.Outcome, comparison.Completeness), badgeStyle(string(comparison.Outcome))))
	if comparison.Receipt != "" {
		lines = append(lines, readableRow("Receipt", shortID(comparison.Receipt)))
	}
	if result.Receipt.ID != "" {
		lines = append(lines, readableRow("Snapshots", fmt.Sprintf("%s → %s", shortID(result.Receipt.Snapshots.Base), shortID(result.Receipt.Snapshots.Candidate))))
	}
	if result.Details != nil {
		lines = comparisonDetailLines(lines, *result.Details)
	}
	lines = appendLimits(lines, comparison.Limits)
	ids := []evidence.Digest{comparison.ID, comparison.Receipt}
	if result.Receipt.ID != "" {
		ids = append(ids, result.Receipt.ID, result.Receipt.Snapshots.Base, result.Receipt.Snapshots.Candidate)
		for _, artifact := range result.Receipt.Artifacts {
			ids = append(ids, artifact.Content)
		}
	}
	if comparison.Details != nil {
		ids = append(ids, comparison.Details.Content)
	}
	if result.Using != nil {
		ids = append(ids, result.Using.Capture)
	}
	return addIDs(lines, ids...), nil
}

func comparisonDetailLines(lines []readableLine, report compare.Report) []readableLine {
	for _, seconds := range []int64{43200, 30} {
		for _, channel := range []string{"responses", "provider"} {
			var outcomes []evidence.ComparisonOutcome
			var countChange string
			for _, witness := range report.Witnesses {
				if witness.Relation != "paired" || witness.Before.Seconds != seconds || witness.Channel != channel {
					continue
				}
				outcomes = append(outcomes, witness.Outcome)
				for _, change := range witness.Changes {
					if strings.HasSuffix(change.Path, "/count") && len(change.Before) > 0 && len(change.After) > 0 {
						countChange = rawValue(change.Before) + " → " + rawValue(change.After)
					}
				}
			}
			if len(outcomes) == 0 {
				continue
			}
			outcome := evidence.Equal
			for _, candidate := range outcomes {
				if candidate != evidence.Equal {
					outcome = candidate
					break
				}
			}
			label := "responses"
			if channel == "provider" {
				label = "provider requests"
			}
			suffix := ""
			if countChange != "" {
				suffix = " · " + countChange
			} else if channel == "provider" && outcome == evidence.Equal {
				suffix = " · count unchanged"
			}
			lines = append(lines, textLine(fmt.Sprintf("  %s %-18s [%s]%s", formatDelay(seconds), label, strings.ToUpper(string(outcome)), suffix), badgeStyle(string(outcome))))
		}
	}
	return lines
}

func rawValue(raw json.RawMessage) string {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) == nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

func formatDelay(seconds int64) string {
	if seconds >= 3600 && seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds >= 60 && seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func reviewLines(state *invocation, view review.View) []readableLine {
	pin := view.Pin
	decision := strings.ToUpper(string(pin.Decision))
	style := terminal.Decision
	if pin.Decision == evidence.Reopened {
		style = terminal.Attention
	}
	lines := []readableLine{textLine(fmt.Sprintf("Pin %s · [%s] · %s", shortID(pin.ID), decision, view.Applicability), style)}
	lines = append(lines, readableRow("Expectation", pin.Expectation))
	lines = append(lines, readableRow("Scope", strings.ReplaceAll(string(pin.Scope), "_", " ")))
	lines = append(lines, readableRow("Basis", fmt.Sprintf("%s → %s · receipt %s", shortID(pin.BasisSnapshots.Base), shortID(pin.BasisSnapshots.Candidate), shortID(pin.BasisReceipt))))
	lines = append(lines, readableRow("Review", view.Reason))
	last := pin.History[len(pin.History)-1].Review
	if last != nil {
		mode := "original base"
		if last.Mode == evidence.FollowUp {
			mode = "last inspected"
		}
		lines = append(lines, readableRow("Reviewing", fmt.Sprintf("%s %s → %s", mode, shortID(last.Target.Snapshots.Base), shortID(last.Target.Snapshots.Candidate))))
	}
	if view.CurrentReceipt != nil {
		lines = append(lines, readableRow("Current result", fmt.Sprintf("%s · %s / %s", shortID(view.CurrentReceipt.ID), view.CurrentReceipt.State.Execution, view.CurrentReceipt.State.Comparison)))
	} else if view.MissingCurrentResult {
		lines = append(lines, readableRow("Current result", "none attached"))
	}
	lines = append(lines, readableRow("Latest decision", state.formatTime(pin.History[len(pin.History)-1].At)))
	lines = appendLimits(lines, view.Limits)
	ids := []evidence.Digest{pin.ID, pin.Scenario, pin.BasisReceipt, pin.BasisSnapshots.Base, pin.BasisSnapshots.Candidate}
	for _, event := range pin.History {
		if event.Review != nil {
			ids = append(ids, event.Review.Receipt, event.Review.Target.Snapshots.Base, event.Review.Target.Snapshots.Candidate)
		}
	}
	if view.CurrentReceipt != nil {
		ids = append(ids, view.CurrentReceipt.ID)
	}
	return addIDs(lines, ids...)
}

func previewLines(state *invocation, preview executionPreview) []readableLine {
	var plan struct {
		Snapshots evidence.SnapshotPair `json:"snapshots"`
	}
	_ = json.Unmarshal(preview.Plan, &plan)
	statusText := strings.ReplaceAll(preview.Status, "_", " ")
	lines := []readableLine{textLine("Nothing has run. This exact plan is stored privately.", terminal.Attention)}
	if preview.Using != nil {
		lines = append(lines, usingCaptureLine(*preview.Using))
	}
	if statusText != "" {
		lines = append(lines, readableRow("Status", statusText))
	}
	lines = append(lines, readableRow("Snapshots", fmt.Sprintf("%s → %s", shortID(plan.Snapshots.Base), shortID(plan.Snapshots.Candidate))))
	lines = append(lines, readableRow("Plan", fmt.Sprintf("%s · %d bytes", shortID(evidence.Digest(preview.Authorization)), preview.PlanBytes)))
	if preview.ConsentError != "" {
		message := terminal.Line(preview.ConsentError, max(1, state.columns-30))
		lines = append(lines, textLine("Consent summary unavailable: "+message, terminal.Attention))
	} else if preview.Consent != "" {
		lines = append(lines, textLine("Consent summary", terminal.Strong))
		for _, line := range strings.Split(preview.Consent, "\n") {
			lines = append(lines, textLine("  "+terminal.Line(line, max(1, state.columns-2)), terminal.Plain))
		}
	}
	lines = append(lines, dockerSetupReadableLines(state, preview.DockerSetup)...)
	if preview.Authorization != "" {
		lines = append(lines, textLine("Authorization digest for this exact plan:", terminal.Plain))
		lines = append(lines, fullLine(preview.Authorization))
	}
	return lines
}

func planInspectionLines(state *invocation, view planInspection) []readableLine {
	lines := []readableLine{textLine(fmt.Sprintf("Execution plan %s · %s · exact stored bytes", shortID(view.ID), formatPlanSize(view.SizeBytes)), terminal.Strong)}
	if view.ConsentError != "" {
		message := terminal.Line(view.ConsentError, max(1, state.columns-30))
		lines = append(lines, textLine("Consent summary unavailable: "+message, terminal.Attention))
	} else {
		lines = append(lines, textLine("Consent summary", terminal.Strong))
		for _, line := range strings.Split(view.Consent, "\n") {
			lines = append(lines, textLine("  "+terminal.Line(line, max(1, state.columns-2)), terminal.Plain))
		}
	}
	lines = append(lines, textLine("Plan", terminal.Strong))
	content := strings.Split(string(view.raw), "\n")
	if len(content) > 1 && content[len(content)-1] == "" {
		content = content[:len(content)-1]
	}
	for i, line := range content {
		if state.stdoutTTY {
			line = terminal.Line(line, max(1, state.columns-12))
		} else {
			line = terminal.Sanitize(line)
		}
		lines = append(lines, fullLine(fmt.Sprintf("  %4d │ %s", i+1, line)))
	}
	return addIDs(lines, view.ID)
}

func appendLimits(lines []readableLine, limits []string) []readableLine {
	for _, limit := range limits {
		lines = append(lines, readableRow("Limit", limit))
	}
	return lines
}

func badgeStyle(value string) terminal.Style {
	switch strings.ToLower(value) {
	case "equal", "completed", "pass", "accepted", "pinned", "current":
		return terminal.Observed
	case "different", "unstable", "reopened", "stale":
		return terminal.Attention
	case "incomparable", "not_compared", "unknown", "incomplete", "skip":
		return terminal.Muted
	case "fail", "failed", "cancelled":
		return terminal.Problem
	case "reported":
		return terminal.Reported
	default:
		return terminal.Plain
	}
}
