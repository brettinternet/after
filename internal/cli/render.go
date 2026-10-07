package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
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
	case "configuration":
		var result struct {
			Settings []struct {
				Name   string `json:"name"`
				Value  any    `json:"value"`
				Source string `json:"source"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		lines := []readableLine{textLine("Effective AFTER configuration", terminal.Strong)}
		for _, setting := range result.Settings {
			lines = append(lines, readableRow(setting.Name, fmt.Sprintf("%v · %s", setting.Value, setting.Source)))
		}
		return lines, nil
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
		var report reportViewData
		if err := json.Unmarshal(raw, &report); err == nil && report.ID != "" && report.Cards != nil {
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
		return comparisonLines(state, raw)
	case "review":
		var view review.View
		if err := json.Unmarshal(raw, &view); err != nil {
			return nil, err
		}
		lines := reviewLines(state, view)
		if inspection, ok := original.(reviewInspection); ok {
			if inspection.HeadsUnavailable {
				lines = append(lines, textLine("Newer heads unavailable: pin history is corrupt or exceeds lookup limits; showing the requested revision.", terminal.Attention))
			} else if len(inspection.NewerHeads) > 0 {
				lines = append(lines, textLine("Historical revision — newer heads (not selected):", terminal.Attention))
				for _, id := range inspection.NewerHeads {
					lines = append(lines, readableRow("Head", shortID(id)))
				}
				lines = addIDs(lines, inspection.NewerHeads...)
			}
		}
		return lines, nil
	case "execution_preview":
		var preview struct {
			Authorization string          `json:"authorization_digest"`
			Status        string          `json:"status"`
			Plan          json.RawMessage `json:"plan"`
		}
		if err := json.Unmarshal(raw, &preview); err != nil {
			return nil, err
		}
		return previewLines(preview.Status, preview.Authorization, preview.Plan), nil
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
		producer = "not stated"
	}
	lines = append(lines, readableRow("Producer", producer))
	lines = append(lines, readableRow("Imported", state.formatTime(report.Metadata.ImportedAt)))
	lines = append(lines, readableRow("Binding", shortID(report.Metadata.Snapshot)))
	lines = append(lines, textLine("Reported by go test; AFTER did not run or observe these tests", terminal.Muted))
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
		return addIDs(lines, view.Base, view.Candidate), nil
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
	var result struct {
		Comparison evidence.Comparison `json:"comparison"`
		Receipt    evidence.Receipt    `json:"receipt"`
		Details    *compare.Report     `json:"details,omitempty"`
		Snapshots  *snapshotView       `json:"snapshots,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	comparison := result.Comparison
	lines := []readableLine{textLine(fmt.Sprintf("Comparison %s · %s · %s", shortID(comparison.ID), comparison.Outcome, comparison.Completeness), badgeStyle(string(comparison.Outcome)))}
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

func previewLines(status, authorization string, raw json.RawMessage) []readableLine {
	var plan struct {
		Snapshots   evidence.SnapshotPair `json:"snapshots"`
		Repetitions int                   `json:"repetitions"`
		Concurrency int                   `json:"concurrency"`
		Limits      sandbox.Limits        `json:"limits"`
	}
	_ = json.Unmarshal(raw, &plan)
	statusText := strings.ReplaceAll(status, "_", " ")
	lines := []readableLine{textLine("Nothing has run. Execution preview requires exact authorization.", terminal.Attention)}
	if statusText != "" {
		lines = append(lines, readableRow("Status", statusText))
	}
	lines = append(lines, readableRow("Snapshots", fmt.Sprintf("%s → %s", shortID(plan.Snapshots.Base), shortID(plan.Snapshots.Candidate))))
	lines = append(lines, readableRow("Runs", fmt.Sprintf("2 sides × 2 cases × %d repetitions · concurrency %d", plan.Repetitions, plan.Concurrency)))
	lines = append(lines, readableRow("Limits", fmt.Sprintf("%ds · %d output bytes/container", plan.Limits.Seconds, plan.Limits.OutputBytes)))
	if authorization != "" {
		lines = append(lines, textLine("Authorization digest for this exact plan:", terminal.Plain))
		lines = append(lines, fullLine(authorization))
	}
	return lines
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
