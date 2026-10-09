// Package commandview formats bounded command comparison witnesses for readable
// CLI and browser-card output. Exact report data remains available separately.
package commandview

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/terminal"
)

const (
	maxLines     = 24
	maxLineWidth = 76
	maxPathWidth = 20
	maxTextWidth = 20
	maxWitnesses = 4096
)

type Presentation struct {
	Lines []string
	More  bool
}

// IsCommandReport distinguishes command details by their complete declared
// channel set, not by the contents or shape of a change path.
func IsCommandReport(report compare.Report) bool {
	if report.DefinitionName == "" || len(report.Channels) != 3 {
		return false
	}
	channels := map[string]bool{}
	for _, channel := range report.Channels {
		channels[channel] = true
	}
	return channels["exit_status"] && channels["stdout"] && channels["stderr"]
}

// Format returns a bounded human-readable summary only when the definition and
// report agree. Stream modes come from the frozen definition; in particular, a
// JSON value at /base64 is never mistaken for a text-byte witness.
func Format(report compare.Report, definition runner.CommandDefinition) (Presentation, bool) {
	if !IsCommandReport(report) || definition.Kind != "command" || definition.Name != report.DefinitionName || definition.Validate() != nil || len(definition.Cases) == 0 || len(definition.Cases) != len(report.Cases) || len(report.Witnesses) > maxWitnesses {
		return Presentation{}, false
	}
	if report.Outcome == evidence.Incomparable {
		return Presentation{Lines: []string{"Command comparison is incomparable; no equality claim."}}, true
	}
	if report.Outcome != evidence.Equal && report.Outcome != evidence.Different && report.Outcome != evidence.Unstable {
		return Presentation{}, false
	}
	for index, scenarioCase := range definition.Cases {
		if scenarioCase.ID != report.Cases[index] {
			return Presentation{}, false
		}
	}

	presentation := Presentation{Lines: make([]string, 0, maxLines)}
	add := func(line string) bool {
		if len(presentation.Lines) == maxLines {
			presentation.More = true
			return false
		}
		presentation.Lines = append(presentation.Lines, terminal.Line(line, maxLineWidth))
		return true
	}
	channels := []string{"exit_status", "stdout", "stderr"}
	for _, scenarioCase := range definition.Cases {
		if len(presentation.Lines) == maxLines {
			presentation.More = true
			break
		}
		paired := make(map[string][]compare.Witness, len(channels))
		repetitions := make(map[string][]compare.Witness, len(channels))
		for _, witness := range report.Witnesses {
			if witness.Before.CaseID != scenarioCase.ID || witness.After.CaseID != scenarioCase.ID {
				continue
			}
			switch witness.Relation {
			case "paired":
				paired[witness.Channel] = append(paired[witness.Channel], witness)
			case "repetition":
				repetitions[witness.Channel] = append(repetitions[witness.Channel], witness)
			}
		}

		caseLabel := fmt.Sprintf("  %s (%s)", scenarioCase.Title, scenarioCase.ID)
		complete := true
		changed := false
		for _, channel := range channels {
			if len(paired[channel]) != definition.Repetitions || len(repetitions[channel]) != 2*(definition.Repetitions-1) {
				complete = false
			}
			for _, witness := range paired[channel] {
				if witness.Outcome == evidence.Different {
					changed = true
				} else if witness.Outcome != evidence.Equal {
					complete = false
				}
			}
			for _, witness := range repetitions[channel] {
				if witness.Outcome == evidence.Different {
					changed = true
				} else if witness.Outcome != evidence.Equal {
					complete = false
				}
			}
		}
		if !changed && complete {
			if !add(caseLabel + " [EQUAL] exit_status, stdout, stderr") {
				return presentation, true
			}
			continue
		}
		if !complete && !changed {
			if !add(caseLabel + " · no complete command witnesses") {
				return presentation, true
			}
			continue
		}
		if !complete {
			if !add(caseLabel + " · incomplete witnesses (no equality claim)") {
				return presentation, true
			}
		}

		for _, channel := range channels {
			var differences []compare.Witness
			for _, witness := range paired[channel] {
				if witness.Outcome == "different" {
					differences = append(differences, witness)
				}
			}
			if len(differences) > 0 {
				if !add(fmt.Sprintf("%s · %s [DIFFERENT]", caseLabel, channel)) {
					return presentation, true
				}
				for _, witness := range differences {
					for _, change := range witness.Changes {
						if !add(formatChange(channel, streamMode(definition, channel), witness, change)) {
							return presentation, true
						}
					}
				}
			}

			unstableSides := map[string]bool{}
			var unstable []compare.Witness
			for _, witness := range repetitions[channel] {
				if witness.Outcome == "different" {
					unstable = append(unstable, witness)
					unstableSides[witness.Before.Side] = true
				}
			}
			if len(unstable) == 0 {
				continue
			}
			sides := []string{}
			for _, side := range []string{"base", "candidate"} {
				if unstableSides[side] {
					sides = append(sides, side)
				}
			}
			if !add(fmt.Sprintf("%s · %s [UNSTABLE: %s repetitions]", caseLabel, channel, strings.Join(sides, " and "))) {
				return presentation, true
			}
			for _, witness := range unstable {
				for _, change := range witness.Changes {
					if !add(formatRepetitionChange(channel, streamMode(definition, channel), witness, change)) {
						return presentation, true
					}
				}
			}
		}
	}
	if len(presentation.Lines) == 0 {
		add("No command comparison witnesses are available.")
	}
	return presentation, true
}

func streamMode(definition runner.CommandDefinition, channel string) string {
	switch channel {
	case "stdout":
		return definition.Comparison.Stdout
	case "stderr":
		return definition.Comparison.Stderr
	default:
		return ""
	}
}

func formatChange(channel, mode string, witness compare.Witness, change compare.Change) string {
	prefix := fmt.Sprintf("    rep %d: ", witness.Before.Repetition+1)
	if channel == "exit_status" {
		return prefix + jsonValue(change.Before) + " → " + jsonValue(change.After)
	}
	if mode == "text" {
		before, beforeOK := textWitness(change.Before)
		after, afterOK := textWitness(change.After)
		if change.Path != "/base64" || change.Kind != "changed" || !beforeOK || !afterOK {
			return prefix + "text byte witness unavailable"
		}
		return prefix + "text before: " + displayBytes(before) + " → after: " + displayBytes(after)
	}
	return prefix + describeJSONChange(change)
}

func formatRepetitionChange(channel, mode string, witness compare.Witness, change compare.Change) string {
	prefix := fmt.Sprintf("    rep %d→%d: ", witness.Before.Repetition+1, witness.After.Repetition+1)
	if channel == "exit_status" {
		return prefix + jsonValue(change.Before) + " → " + jsonValue(change.After)
	}
	if mode == "text" {
		before, beforeOK := textWitness(change.Before)
		after, afterOK := textWitness(change.After)
		if change.Path != "/base64" || change.Kind != "changed" || !beforeOK || !afterOK {
			return prefix + "text byte witness unavailable"
		}
		return prefix + "text: " + displayBytes(before) + " → " + displayBytes(after)
	}
	return prefix + describeJSONChange(change)
}

func describeJSONChange(change compare.Change) string {
	path := change.Path
	if path == "" {
		path = "/ (root)"
	}
	path = terminal.Line(path, maxPathWidth)
	switch change.Kind {
	case "added":
		return path + " added " + jsonValue(change.After)
	case "removed":
		return path + " removed " + jsonValue(change.Before)
	default:
		return path + " " + jsonValue(change.Before) + " → " + jsonValue(change.After)
	}
}

func jsonValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "—"
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return "[invalid JSON value]"
	}
	if text, ok := value.(string); ok {
		return `"` + terminal.Line(text, maxTextWidth-2) + `"`
	}
	compact, err := json.Marshal(value)
	if err != nil {
		return "[invalid JSON value]"
	}
	return terminal.Line(string(compact), maxTextWidth)
}

func textWitness(raw json.RawMessage) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	key, ok := nextStringToken(decoder)
	if !ok || key != "base64" {
		return nil, false
	}
	var encoded string
	if decoder.Decode(&encoded) != nil {
		return nil, false
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') || decoder.Decode(new(any)) != io.EOF {
		return nil, false
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	return decoded, err == nil
}

func nextStringToken(decoder *json.Decoder) (string, bool) {
	token, err := decoder.Token()
	if err != nil {
		return "", false
	}
	value, ok := token.(string)
	return value, ok
}

func displayBytes(raw []byte) string {
	if !utf8.Valid(raw) {
		return fmt.Sprintf("[invalid UTF-8, %d bytes]", len(raw))
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return fmt.Sprintf("[binary, %d bytes]", len(raw))
	}
	if len(raw) == 0 {
		return "[empty]"
	}
	return terminal.Line(string(raw), maxTextWidth)
}
