package browser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

type runnerArtifact struct {
	artifact evidence.Artifact
	side     string
	caseID   string
	seconds  int64
	rep      int
	channel  string
}

func cardSection(parts ...Part) Section { return Section{Name: "Card", Parts: parts, Wrap: true} }

func textPart(title, text string) Part { return Part{Title: title, Content: []byte(text)} }

func columnPart(title, leftLabel, left, rightLabel, right string) Part {
	content := []byte(leftLabel + " " + left + "\n" + rightLabel + " " + right)
	return Part{Title: title, Content: content, columns: &cardColumns{leftLabel: leftLabel, left: left, rightLabel: rightLabel, right: right}}
}

func jsonPart(title string, value any) Part {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return textPart(title, "Content unavailable; no conclusion is drawn.")
	}
	return Part{Title: title, Content: raw}
}

func recordPart(s *store.Store, kind string, id evidence.Digest, title string) Part {
	raw, err := store.ReadRawRecord(s, kind, id)
	if err != nil {
		return textPart(title, "Exact stored record bytes unavailable; ID remains reachable: "+string(id))
	}
	return Part{Title: title, Content: raw}
}

func blobPart(s *store.Store, id evidence.Digest, title string, format documentFormat) Part {
	if id == "" {
		return textPart(title, "No content ID was recorded.")
	}
	if _, err := s.ReadBlob(id); err != nil {
		return textPart(title, "Referenced content was not retained as a readable artifact; content ID: "+string(id))
	}
	return Part{Title: title, Blob: id, format: format}
}

func idsSection(ids []evidence.Digest) Section {
	parts := make([]Part, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, textPart("full ID", string(id)))
	}
	return Section{Name: "IDs", Parts: parts}
}

func appendIDs(ids []evidence.Digest, values ...evidence.Digest) []evidence.Digest {
	seen := make(map[evidence.Digest]bool, len(ids)+len(values))
	for _, id := range ids {
		seen[id] = true
	}
	for _, id := range values {
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}

func runnerChannel(channel string) (runnerArtifact, bool) {
	parts := strings.Split(channel, "/")
	if len(parts) != 4 || (parts[0] != "base" && parts[0] != "candidate") || !validCaseID(parts[1]) {
		return runnerArtifact{}, false
	}
	repetition, err := strconv.Atoi(parts[2])
	if err != nil || repetition < 0 || repetition > 4 || strconv.Itoa(repetition) != parts[2] {
		return runnerArtifact{}, false
	}
	switch parts[3] {
	case "observation", "sample", "candidate-diagnostics", "observer-diagnostics":
		seconds, _ := strconv.ParseInt(parts[1], 10, 64)
		return runnerArtifact{side: parts[0], caseID: parts[1], seconds: seconds, rep: repetition, channel: parts[3]}, true
	default:
		return runnerArtifact{}, false
	}
}

func validCaseID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	first := value[0]
	if !(first >= 'a' && first <= 'z' || first >= '0' && first <= '9') {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func orderedArtifacts(artifacts []evidence.Artifact) ([]runnerArtifact, []evidence.Artifact) {
	known, other := []runnerArtifact{}, []evidence.Artifact{}
	for _, artifact := range artifacts {
		item, ok := runnerChannel(artifact.Channel)
		if !ok {
			other = append(other, artifact)
			continue
		}
		item.artifact = artifact
		known = append(known, item)
	}
	secondsOrder := map[int64]int{43200: 0, 30: 1}
	channelOrder := map[string]int{"observation": 0, "sample": 1, "candidate-diagnostics": 2, "observer-diagnostics": 3}
	sort.SliceStable(known, func(i, j int) bool {
		a, b := known[i], known[j]
		if a.side != b.side {
			return a.side == "base"
		}
		if a.caseID != b.caseID {
			if a.seconds != 0 && b.seconds != 0 {
				return secondsOrder[a.seconds] < secondsOrder[b.seconds]
			}
			return a.caseID < b.caseID
		}
		if a.rep != b.rep {
			return a.rep < b.rep
		}
		return channelOrder[a.channel] < channelOrder[b.channel]
	})
	sort.SliceStable(other, func(i, j int) bool { return other[i].Channel < other[j].Channel })
	return known, other
}

func artifactTitle(item runnerArtifact) string {
	caseTitle := item.caseID
	if item.seconds > 0 {
		caseTitle = delayText(item.seconds)
	}
	return fmt.Sprintf("%s · %s · repetition %d · %s", item.side, caseTitle, item.rep+1, item.channel)
}

func artifactsSection(s *store.Store, artifacts []evidence.Artifact, seconds *int64) (Section, []evidence.Digest) {
	known, other := orderedArtifacts(artifacts)
	parts := make([]Part, 0, len(known)+len(other))
	ids := []evidence.Digest{}
	for _, item := range known {
		if seconds != nil && item.seconds != *seconds {
			continue
		}
		parts = append(parts, blobPart(s, item.artifact.Content, artifactTitle(item), artifactFormat(item.artifact.Channel)))
		ids = appendIDs(ids, item.artifact.Content)
	}
	for _, artifact := range other {
		if artifact.Channel == "execution-plan" || artifact.Channel == "comparison-rules" {
			continue
		}
		title := "Other artifacts · " + strconv.Quote(artifact.Channel)
		parts = append(parts, blobPart(s, artifact.Content, title, formatVerbatim))
		ids = appendIDs(ids, artifact.Content)
	}
	return Section{Name: "Artifacts", Parts: parts}, ids
}

func planSection(s *store.Store, artifacts []evidence.Artifact) (Section, []evidence.Digest) {
	for _, artifact := range artifacts {
		if artifact.Channel == "execution-plan" {
			return Section{Name: "Plan", Parts: []Part{blobPart(s, artifact.Content, "execution plan", formatVerbatim)}}, []evidence.Digest{artifact.Content}
		}
	}
	return Section{Name: "Plan", Parts: []Part{textPart("execution plan", "No execution-plan artifact was retained.")}}, nil
}

func rulesPart(s *store.Store, artifacts []evidence.Artifact, rules evidence.Digest) (Part, []evidence.Digest) {
	for _, artifact := range artifacts {
		if artifact.Channel == "comparison-rules" {
			return blobPart(s, artifact.Content, "comparison rules", formatVerbatim), []evidence.Digest{artifact.Content}
		}
	}
	if rules != "" {
		return blobPart(s, rules, "comparison rules", formatVerbatim), []evidence.Digest{rules}
	}
	return textPart("comparison rules", "No comparison-rules artifact was retained."), nil
}

func scenarioSection(s *store.Store, scenario evidence.Scenario) (Section, []evidence.Digest) {
	parts := []Part{recordPart(s, "scenario", scenario.ID, "frozen scenario")}
	parts = append(parts,
		blobPart(s, scenario.Input, "frozen input", formatVerbatim),
		blobPart(s, scenario.Driver, "driver binding", formatVerbatim),
		blobPart(s, scenario.Observer, "observer binding", formatVerbatim),
		blobPart(s, scenario.Rules, "rules binding", formatVerbatim),
	)
	return Section{Name: "Scenario", Parts: parts}, []evidence.Digest{scenario.ID, scenario.Input, scenario.Driver, scenario.Observer, scenario.Rules}
}

func receiptIDs(receipt evidence.Receipt, comparison *evidence.Comparison, details *compare.Report, scenario *evidence.Scenario) []evidence.Digest {
	ids := []evidence.Digest{receipt.ID, receipt.Snapshots.Base, receipt.Snapshots.Candidate, receipt.RequestID, receipt.Authorization}
	if receipt.Bindings != nil {
		ids = appendIDs(ids, receipt.Bindings.Scenario, receipt.Bindings.Input, receipt.Bindings.Driver, receipt.Bindings.Observer, receipt.Bindings.Rules)
	}
	for _, environment := range []*evidence.Environment{receipt.BaseEnvironment, receipt.CandidateEnvironment} {
		if environment != nil {
			ids = appendIDs(ids, environment.Environment, environment.Toolchain, environment.Dependencies)
		}
	}
	for _, artifact := range receipt.Artifacts {
		ids = appendIDs(ids, artifact.Content)
	}
	if comparison != nil {
		ids = appendIDs(ids, comparison.ID)
		if comparison.Details != nil {
			ids = appendIDs(ids, comparison.Details.Content)
		}
	}
	if details != nil {
		ids = appendIDs(ids, details.Rules)
		for _, artifact := range details.Artifacts {
			ids = appendIDs(ids, artifact.Content)
		}
		for _, witness := range details.Witnesses {
			ids = appendIDs(ids, witness.Before.Metadata, witness.Before.Observation, witness.After.Metadata, witness.After.Observation)
		}
	}
	if scenario != nil {
		ids = appendIDs(ids, scenario.ID, scenario.Input, scenario.Driver, scenario.Observer, scenario.Rules)
	}
	return ids
}

func strictReport(raw []byte, receipt evidence.Receipt, comparison evidence.Comparison) (*compare.Report, error) {
	var report compare.Report
	if err := decode(raw, &report); err != nil {
		return nil, err
	}
	if report.Version != 1 || report.Receipt != receipt.ID || report.Snapshots != receipt.Snapshots || report.Outcome != comparison.Outcome || len(report.Artifacts) > 256 || len(report.Witnesses) > 4096 || len(report.Limits) > 100 || !sameArtifacts(report.Artifacts, receipt.Artifacts) {
		return nil, errors.New("unexpected comparison report shape")
	}
	totalChanges := 0
	for _, witness := range report.Witnesses {
		if (witness.Relation != "paired" && witness.Relation != "repetition") || witness.Channel == "" || len(witness.Channel) > 256 || !validCompareRef(witness.Before) || !validCompareRef(witness.After) || (witness.Outcome != evidence.Equal && witness.Outcome != evidence.Different) {
			return nil, errors.New("unexpected comparison witness shape")
		}
		if witness.Relation == "paired" && (witness.Before.Side != "base" || witness.After.Side != "candidate" || witness.Before.Repetition != witness.After.Repetition) {
			return nil, errors.New("unexpected paired comparison shape")
		}
		if witness.Relation == "repetition" && (witness.Before.Side != witness.After.Side || witness.Before.Repetition == witness.After.Repetition) {
			return nil, errors.New("unexpected repetition comparison shape")
		}
		totalChanges += len(witness.Changes)
		if totalChanges > 2048 {
			return nil, errors.New("comparison witness limit")
		}
		for _, change := range witness.Changes {
			// "" is the JSON Pointer root, e.g. a changed command exit status.
			if (change.Path != "" && change.Path[0] != '/') || len(change.Path) > 4096 || (change.Kind != "added" && change.Kind != "removed" && change.Kind != "changed") || !json.Valid(change.Before) && len(change.Before) > 0 || !json.Valid(change.After) && len(change.After) > 0 {
				return nil, errors.New("unexpected comparison change shape")
			}
		}
	}
	return &report, nil
}

func sameArtifacts(a, b []evidence.Artifact) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func validCompareRef(ref compare.SampleRef) bool {
	caseValid := validCaseID(ref.CaseID) || ref.CaseID == "" && (ref.Seconds == 30 || ref.Seconds == 43200)
	return (ref.Side == "base" || ref.Side == "candidate") && caseValid && ref.Repetition >= 0 && ref.Repetition < 5 && ref.Metadata != "" && ref.Observation != ""
}

func strictObservation(raw []byte, seconds int64) (runner.Observation, error) {
	var observation runner.Observation
	if err := decode(raw, &observation); err != nil {
		return observation, err
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil || !hasFields(shape, "version", "seconds", "responses", "provider_calls") || observation.Version != 1 || observation.Seconds != seconds || (observation.CaseID != "" && observation.CaseID != strconv.FormatInt(seconds, 10)) || len(observation.Responses) != 2 || observation.Calls == nil || len(observation.Calls) > 128 {
		return observation, errors.New("unexpected observation shape")
	}
	for _, response := range observation.Responses {
		if response.Status < 100 || response.Status > 599 || len(response.Body) > 4096 {
			return observation, errors.New("unexpected response shape")
		}
	}
	for _, call := range observation.Calls {
		if call.At < 0 || call.Method == "" || len(call.Method) > 16 || !strings.HasPrefix(call.Path, "/") || len(call.Path) > 256 || len(call.Key) > 128 || len(call.Body) > 4096 {
			return observation, errors.New("unexpected provider-call shape")
		}
	}
	return observation, nil
}

func hasFields(shape map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		if value, ok := shape[name]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return true
}

func strictSample(raw []byte, receipt evidence.Receipt, item runnerArtifact) (runner.Sample, error) {
	var sample runner.Sample
	if err := decode(raw, &sample); err != nil {
		return sample, err
	}
	if sample.RequestID != receipt.RequestID || sample.Snapshots != receipt.Snapshots || sample.Side != item.side || (sample.CaseID != "" && sample.CaseID != item.caseID) || sample.CaseSeconds != item.seconds || sample.Repetition != item.rep || sample.StartedAt.IsZero() || sample.FinishedAt.Before(sample.StartedAt) || sample.StartedAt.Before(receipt.StartedAt) || sample.FinishedAt.After(receipt.FinishedAt) || sample.Status == "" || len(sample.Status) > 64 || len(sample.Artifacts) > 3 {
		return sample, errors.New("unexpected sample shape")
	}
	return sample, nil
}

func strictFrozenInput(raw []byte) (struct {
	Epoch   int64   `json:"epoch"`
	Seconds []int64 `json:"seconds"`
	Key     string  `json:"key"`
	Body    struct {
		AmountCents int64  `json:"amount_cents"`
		Currency    string `json:"currency"`
	} `json:"body"`
}, error) {
	var input struct {
		Epoch   int64   `json:"epoch"`
		Seconds []int64 `json:"seconds"`
		Key     string  `json:"key"`
		Body    struct {
			AmountCents int64  `json:"amount_cents"`
			Currency    string `json:"currency"`
		} `json:"body"`
	}
	if err := decode(raw, &input); err != nil {
		return input, err
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil || !hasFields(shape, "epoch", "seconds", "key", "body") || input.Epoch <= 0 || input.Key == "" || len(input.Seconds) != 2 || input.Seconds[0] != 43200 || input.Seconds[1] != 30 {
		return input, errors.New("unexpected frozen input shape")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(shape["body"], &body); err != nil || !hasFields(body, "amount_cents", "currency") || input.Body.AmountCents <= 0 || input.Body.Currency == "" {
		return input, errors.New("unexpected frozen payment input")
	}
	return input, nil
}

func paymentInputPart(s *store.Store, scenario evidence.Scenario) (Part, []evidence.Digest, error) {
	raw, err := s.ReadBlob(scenario.Input)
	if err != nil {
		return Part{}, nil, err
	}
	if definition, definitionErr := runner.ParseDefinition(raw); definitionErr == nil && definition.Name == "synthetic-payment" && len(definition.Cases) > 0 && len(definition.Cases[0].Requests) > 0 {
		request := definition.Cases[0].Requests[0]
		return textPart("Input", fmt.Sprintf("key %s · body %s · frozen request sequence", request.Headers["Idempotency-Key"], request.Body)), []evidence.Digest{scenario.Input}, nil
	}
	input, err := strictFrozenInput(raw)
	if err != nil {
		return Part{}, nil, err
	}
	return textPart("Input", fmt.Sprintf("key %s · body %s · second request 12h later", input.Key, strings.TrimSpace(string(mustJSON(input.Body))))), []evidence.Digest{scenario.Input}, nil
}

func mustJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func caseObservations(s *store.Store, receipt evidence.Receipt, seconds int64) (map[string][]runner.Observation, map[string][]runner.Sample, error) {
	artifacts, _ := orderedArtifacts(receipt.Artifacts)
	observations := map[string][]runner.Observation{"base": {}, "candidate": {}}
	samples := map[string][]runner.Sample{"base": {}, "candidate": {}}
	for _, item := range artifacts {
		if item.seconds != seconds || (item.channel != "observation" && item.channel != "sample") {
			continue
		}
		raw, err := s.ReadBlob(item.artifact.Content)
		if err != nil {
			return nil, nil, err
		}
		if item.channel == "observation" {
			observation, err := strictObservation(raw, seconds)
			if err != nil {
				return nil, nil, err
			}
			observations[item.side] = append(observations[item.side], observation)
		} else {
			sample, err := strictSample(raw, receipt, item)
			if err != nil {
				return nil, nil, err
			}
			samples[item.side] = append(samples[item.side], sample)
		}
	}
	for _, side := range []string{"base", "candidate"} {
		if len(observations[side]) == 0 || len(observations[side]) != len(samples[side]) {
			return nil, nil, errors.New("missing or mismatched case artifacts")
		}
	}
	return observations, samples, nil
}

func caseCard(s *store.Store, receipt evidence.Receipt, scenario evidence.Scenario, comparison *evidence.Comparison, report *compare.Report, seconds int64, observations map[string][]runner.Observation, samples map[string][]runner.Sample) (Section, error) {
	input, _, err := paymentInputPart(s, scenario)
	if err != nil {
		return Section{}, err
	}
	base, candidate := observations["base"], observations["candidate"]
	parts := []Part{textPart("Payment case", delayText(seconds)+" same-key retry")}
	parts = append(parts, columnPart("Snapshots", "base", shortID(receipt.Snapshots.Base), "candidate", shortID(receipt.Snapshots.Candidate)))
	parts = append(parts, columnPart("Provider requests", "base", countText(observationCounts(base)), "candidate", countText(observationCounts(candidate))))
	for index := 0; index < 2; index++ {
		parts = append(parts, columnPart(fmt.Sprintf("Response %d", index+1), "base", fmt.Sprintf("%d %s", base[0].Responses[index].Status, base[0].Responses[index].Body), "candidate", fmt.Sprintf("%d %s", candidate[0].Responses[index].Status, candidate[0].Responses[index].Body)))
	}
	parts = append(parts, input)
	if report != nil {
		for _, witness := range report.Witnesses {
			if witness.Before.Seconds != seconds {
				continue
			}
			changes := []string{}
			for _, change := range witness.Changes {
				before, after := string(change.Before), string(change.After)
				if before == "" {
					before = "<missing>"
				}
				if after == "" {
					after = "<missing>"
				}
				changes = append(changes, fmt.Sprintf("%s %s: %s → %s", change.Kind, change.Path, before, after))
			}
			if len(changes) == 0 {
				changes = append(changes, "No differing values in this witness.")
			}
			parts = append(parts, textPart("Witness · "+witness.Relation+" · "+strconv.Quote(witness.Channel)+" · "+string(witness.Outcome), strings.Join(changes, "\n")))
		}
	}
	parts = append(parts, textPart("Samples", fmt.Sprintf("%d repetition(s) per side — one sample cannot show instability", len(samples["base"]))))
	for _, side := range []string{"base", "candidate"} {
		for index, sample := range samples[side] {
			parts = append(parts, jsonPart(fmt.Sprintf("Sample · %s · repetition %d", side, index+1), sample))
		}
	}
	parts = append(parts, textPart("Run", fmt.Sprintf("runner · %s–%s (%s) · %s · %s", receipt.StartedAt.Format(time.RFC3339Nano), receipt.FinishedAt.Format(time.RFC3339Nano), receipt.FinishedAt.Sub(receipt.StartedAt), receipt.State.Execution, redactText(receipt.Redacted))))
	parts = append(parts, textPart("Scope", strings.Join(append(append([]string(nil), scenario.Limits...), receipt.Limits...), "\n")))
	if comparison != nil {
		parts = append(parts, textPart("Comparison", string(comparison.Outcome)+" · "+string(comparison.Completeness)))
	}
	return cardSection(parts...), nil
}

func observationCounts(items []runner.Observation) []int {
	counts := make([]int, len(items))
	for index, item := range items {
		counts[index] = len(item.Calls)
	}
	return counts
}

func redactText(redacted bool) string {
	if redacted {
		return "redacted"
	}
	return "not redacted"
}

func receiptCard(receipt evidence.Receipt, scenario *evidence.Scenario, comparison *evidence.Comparison, report *compare.Report) Section {
	parts := []Part{
		textPart("Outcome", fmt.Sprintf("%s · %s · %s · %s", receipt.State.Kind, receipt.State.Execution, receipt.State.Comparison, receipt.Completeness)),
		textPart("Producer", string(receipt.State.Producer)),
		textPart("Binding", fmt.Sprintf("base %s → candidate %s", shortID(receipt.Snapshots.Base), shortID(receipt.Snapshots.Candidate))),
		textPart("Events", fmt.Sprintf("%s–%s", receipt.StartedAt.Format(time.RFC3339Nano), receipt.FinishedAt.Format(time.RFC3339Nano))),
		textPart("Scope", strings.Join(receipt.Limits, "\n")),
	}
	if receipt.Bindings != nil {
		parts = append(parts, textPart("Scenario", string(receipt.Bindings.Scenario)+" · input "+string(receipt.Bindings.Input)+" · driver "+string(receipt.Bindings.Driver)+" · observer "+string(receipt.Bindings.Observer)+" · rules "+string(receipt.Bindings.Rules)))
	}
	if scenario != nil {
		parts = append(parts, textPart("Boundary", scenario.Boundary))
	}
	if comparison != nil {
		parts = append(parts, textPart("Comparison", string(comparison.Outcome)+" · "+string(comparison.Completeness)))
	}
	if report != nil {
		parts = append(parts, textPart("Witnesses", fmt.Sprintf("%d stored witnesses", len(report.Witnesses))))
	}
	return cardSection(parts...)
}

func receiptSections(s *store.Store, receipt evidence.Receipt, comparison *evidence.Comparison, report *compare.Report, scenario *evidence.Scenario, ids []evidence.Digest) []Section {
	artifacts, _ := artifactsSection(s, receipt.Artifacts, nil)
	if comparison != nil && comparison.Details != nil {
		artifacts.Parts = append(artifacts.Parts, blobPart(s, comparison.Details.Content, "comparison details · "+shortID(comparison.Details.Content), formatComparison))
	}
	var scenarioDetail Section
	if scenario != nil {
		scenarioDetail, _ = scenarioSection(s, *scenario)
	} else {
		scenarioDetail = Section{Name: "Scenario", Parts: []Part{textPart("frozen scenario", "Receipt has no frozen scenario binding.")}}
	}
	return []Section{
		receiptCard(receipt, scenario, comparison, report),
		artifacts,
		{Name: "Receipt", Parts: []Part{recordPart(s, "receipt", receipt.ID, "receipt")}},
		scenarioDetail,
		func() Section { section, _ := planSection(s, receipt.Artifacts); return section }(),
		idsSection(ids),
	}
}

func caseWitnessesSection(s *store.Store, receipt evidence.Receipt, comparison *evidence.Comparison, report *compare.Report, seconds int64) Section {
	parts := []Part{}
	if report != nil {
		for _, witness := range report.Witnesses {
			if witness.Before.Seconds != seconds {
				continue
			}
			body := fmt.Sprintf("before %s · %s · repetition %d\nafter %s · %s · repetition %d\noutcome %s", witness.Before.Side, delayText(witness.Before.Seconds), witness.Before.Repetition+1, witness.After.Side, delayText(witness.After.Seconds), witness.After.Repetition+1, witness.Outcome)
			for _, change := range witness.Changes {
				body += fmt.Sprintf("\n%s %s = before %s · after %s", change.Kind, change.Path, bytes.TrimSpace(change.Before), bytes.TrimSpace(change.After))
			}
			parts = append(parts, textPart("Witness · "+witness.Relation+" · "+strconv.Quote(witness.Channel)+" · "+string(witness.Outcome), body))
		}
	} else {
		parts = append(parts, textPart("Witness", "No strictly decoded comparison witnesses are available."))
	}
	if comparison != nil {
		parts = append(parts, recordPart(s, "comparison", comparison.ID, "comparison record"))
		if comparison.Details != nil {
			parts = append(parts, blobPart(s, comparison.Details.Content, "comparison report · "+shortID(comparison.Details.Content), formatComparison))
		}
	}
	rules, _ := rulesPart(s, receipt.Artifacts, func() evidence.Digest {
		if receipt.Bindings != nil {
			return receipt.Bindings.Rules
		}
		return ""
	}())
	parts = append(parts, rules)
	return Section{Name: "Witnesses", Parts: parts}
}

func reportCardParts(card reportCardView) []Part {
	outcome := card.State.Report
	at := "unknown"
	if card.FirstEventAt != nil && card.LastEventAt != nil {
		at = fmt.Sprintf("%s–%s (%s)", card.FirstEventAt.Format(time.RFC3339Nano), card.LastEventAt.Format(time.RFC3339Nano), card.LastEventAt.Sub(*card.FirstEventAt))
	}
	producer := card.Producer
	if producer == "" {
		producer = "not stated"
	}
	binding := "unbound"
	if card.Binding != "" {
		binding = "candidate " + string(card.Binding) + " (caller-supplied; a digest is not a signature)"
	}
	return []Part{
		textPart("Package", card.Package),
		textPart("Test", fmt.Sprintf("%s · attempt %d", card.Test, card.Attempt)),
		textPart("Outcome", fmt.Sprintf("%s, as reported by %s. AFTER did not run or observe this test.", outcome, gotestreport.DialectLabel(card.Dialect))),
		textPart("Producer", producer),
		textPart("Bound to", binding),
		textPart("Events", at),
		textPart("Inputs", card.Inputs+"; test reports record neither inputs nor effects"),
		textPart("Expected values", card.ExpectedValues),
		textPart("Effects", card.Effects),
	}
}

type reportCardView struct {
	Package, Test, Producer, Dialect string
	Binding                          evidence.Digest
	Attempt                          int
	State                            evidence.EvidenceState
	FirstEventAt                     *time.Time
	LastEventAt                      *time.Time
	Inputs, ExpectedValues, Effects  string
}

func unavailableCard(id evidence.Digest, kind, limitation string) []Section {
	if limitation == "" {
		limitation = "Stored ID unavailable or invalid; no evidence conclusion is possible."
	}
	return []Section{
		cardSection(textPart("Limitation", fmt.Sprintf("%s %s: %s", kind, id, limitation))),
		idsSection([]evidence.Digest{id}),
	}
}

func rawFallbackSections(kind string, id evidence.Digest, raw []byte, limitation string) []Section {
	return []Section{
		cardSection(textPart("Limitation", limitation)),
		{Name: "Raw content", Parts: []Part{{Title: "raw " + kind + " · " + string(id), Content: append([]byte(nil), raw...)}}},
		idsSection([]evidence.Digest{id}),
	}
}

func unavailableEntry(id evidence.Digest, sections []Section) Entry {
	ids := []evidence.Digest{id}
	for _, section := range sections {
		if section.Name != "IDs" {
			continue
		}
		ids = ids[:0]
		for _, part := range section.Parts {
			if len(part.Content) > 0 {
				ids = append(ids, evidence.Digest(string(part.Content)))
			}
		}
		break
	}
	return Entry{Unavailable: true, Name: string(id), Sections: sections, IDs: ids}
}

func rawUnavailableFallback(s *store.Store, id evidence.Digest, cause error) []Section {
	for _, kind := range []string{"pin", "comparison", "receipt", "scenario", "snapshot", "capture"} {
		raw, err := store.ReadRawRecord(s, kind, id)
		if err != nil {
			continue
		}
		fallback := rawFallback{store: s, seen: map[evidence.Digest]bool{}}
		fallback.addRecord(kind, id, raw)
		fallback.dependencies(kind, raw)
		limitation := "Strict decoding or record validation failed; showing the full raw record and reachable artifacts. Limitation: " + cause.Error()
		return []Section{cardSection(textPart("Limitation", limitation)), {Name: "Raw content", Parts: fallback.parts}, idsSection(fallback.ids)}
	}
	raw, err := s.ReadBlob(id)
	if err == nil {
		return rawFallbackSections("artifact", id, raw, "Strict decoding or expected shape validation failed; showing the full raw artifact. Limitation: "+cause.Error())
	}
	return unavailableCard(id, "stored record", "Raw content could not be read safely; raw inventory remains usable.")
}

type rawFallback struct {
	store *store.Store
	parts []Part
	ids   []evidence.Digest
	seen  map[evidence.Digest]bool
	size  int
}

func (f *rawFallback) add(id evidence.Digest, title string, read func() ([]byte, error)) {
	if id == "" || f.seen[id] {
		return
	}
	f.seen[id] = true
	f.ids = append(f.ids, id)
	raw, err := read()
	if err != nil {
		f.parts = append(f.parts, textPart(title, "Raw content unavailable; retained ID: "+string(id)))
		return
	}
	if len(raw) > store.MaxBlobBytes-f.size {
		f.parts = append(f.parts, textPart(title, "Additional raw bytes exceed the combined fallback limit; complete content remains at ID "+string(id)))
		return
	}
	f.size += len(raw)
	f.parts = append(f.parts, Part{Title: title, Content: raw})
}

func (f *rawFallback) addRecord(kind string, id evidence.Digest, raw []byte) {
	f.add(id, "raw "+kind+" · "+string(id), func() ([]byte, error) { return raw, nil })
}

func (f *rawFallback) addStoredRecord(kind string, id evidence.Digest) []byte {
	var raw []byte
	f.add(id, "raw "+kind+" · "+string(id), func() ([]byte, error) {
		var err error
		raw, err = store.ReadRawRecord(f.store, kind, id)
		return raw, err
	})
	return raw
}

func (f *rawFallback) addBlob(id evidence.Digest, title string) {
	f.add(id, title+" · "+string(id), func() ([]byte, error) { return f.store.ReadBlob(id) })
}

func (f *rawFallback) dependencies(kind string, raw []byte) {
	switch kind {
	case "receipt":
		var receipt evidence.Receipt
		if decode(raw, &receipt) == nil {
			f.receiptDependencies(receipt)
		}
	case "comparison":
		var comparison evidence.Comparison
		if decode(raw, &comparison) == nil {
			if comparison.Details != nil {
				f.addBlob(comparison.Details.Content, "raw comparison details")
			}
			f.receiptByID(comparison.Receipt)
		}
	case "pin":
		var pin evidence.Pin
		if decode(raw, &pin) == nil {
			f.addStoredRecord("scenario", pin.Scenario)
			for _, event := range pin.History {
				if event.Review != nil {
					f.receiptByID(event.Review.Receipt)
				}
			}
		}
	}
}

func (f *rawFallback) receiptByID(id evidence.Digest) {
	if id == "" {
		return
	}
	raw := f.addStoredRecord("receipt", id)
	var receipt evidence.Receipt
	if decode(raw, &receipt) == nil {
		f.receiptDependencies(receipt)
	}
}

func (f *rawFallback) receiptDependencies(receipt evidence.Receipt) {
	if receipt.Bindings != nil {
		f.addStoredRecord("scenario", receipt.Bindings.Scenario)
	}
	for _, artifact := range receipt.Artifacts {
		f.addBlob(artifact.Content, "raw artifact · "+strconv.Quote(artifact.Channel))
	}
	comparisons, err := f.store.List("comparison")
	if err != nil {
		return
	}
	var readBytes int64
	for _, entry := range comparisons {
		if entry.Size < 0 || entry.Size > int64(store.MaxBlobBytes)-readBytes {
			f.parts = append(f.parts, textPart("comparison history limitation", "Related raw comparison records exceed the combined fallback read limit; their complete IDs remain available."))
			break
		}
		readBytes += entry.Size
		comparison, err := store.Get[evidence.Comparison](f.store, entry.ID)
		if err != nil || comparison.Receipt != receipt.ID {
			continue
		}
		raw := f.addStoredRecord("comparison", comparison.ID)
		if comparison.Details != nil {
			f.addBlob(comparison.Details.Content, "raw comparison details")
		}
		f.dependencies("comparison", raw)
	}
}

func readReportRaw(raw []byte, pair evidence.SnapshotPair) ([]reportCardView, error) {
	var report gotestreport.Report
	if err := decode(raw, &report); err != nil {
		return nil, err
	}
	if report.SchemaVersion != 1 || !gotestreport.SupportedDialect(report.Dialect) || (report.Completeness != evidence.Complete && report.Completeness != evidence.Incomplete) || len(report.Cards) > gotestreport.MaxCards || report.Metadata.ImportedAt.IsZero() {
		return nil, errors.New("unexpected report shape")
	}
	cards := make([]reportCardView, 0, len(report.Cards))
	for _, card := range report.Cards {
		if card.Package == "" || (card.Scope != "package" && card.Scope != "test" && card.Scope != "build") || card.Attempt < 1 || len(card.Output) > gotestreport.MaxOutputBytes || card.State.Validate() != nil || card.State.Kind != evidence.Reported || card.State.Producer != evidence.Importer || card.State.Execution != evidence.NotRun || card.State.Comparison != evidence.NotCompared || (card.Scope == "test" && card.Test == "") {
			return nil, errors.New("unexpected report card shape")
		}
		cards = append(cards, reportCardView{Dialect: report.Dialect, Package: card.Package, Test: card.Test, Producer: report.Metadata.Producer, Binding: report.Metadata.Snapshot, Attempt: card.Attempt, State: card.State, FirstEventAt: card.FirstEventAt, LastEventAt: card.LastEventAt, Inputs: card.Inputs, ExpectedValues: card.ExpectedValues, Effects: card.Effects})
	}
	_ = pair
	return cards, nil
}
