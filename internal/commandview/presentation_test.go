package commandview

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/compare"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/rivo/uniseg"
)

func presentationDefinition(stdoutMode, stderrMode string, cases ...runner.CommandCase) runner.CommandDefinition {
	return runner.CommandDefinition{
		Version: 1, Kind: "command", Name: "presentation-test", Platform: "linux/amd64", Image: sandbox.Image,
		Cases: cases, Repetitions: 1,
		Limits:     runner.DefinitionLimits{Seconds: 30, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: runner.CommandComparison{Stdout: stdoutMode, Stderr: stderrMode},
	}
}

func presentationCase(id string) runner.CommandCase {
	return runner.CommandCase{ID: id, Title: "Case " + id, Argv: []string{"/work/fixture"}, Stdin: []byte{}, Environment: []string{}}
}

func presentationReport(definition runner.CommandDefinition, witnesses ...compare.Witness) compare.Report {
	cases := make([]string, len(definition.Cases))
	outcome := evidence.Equal
	for index, scenarioCase := range definition.Cases {
		cases[index] = scenarioCase.ID
	}
	for _, witness := range witnesses {
		if witness.Outcome == evidence.Different {
			outcome = evidence.Different
			if witness.Relation == "repetition" {
				outcome = evidence.Unstable
			}
		}
	}
	return compare.Report{DefinitionName: definition.Name, Cases: cases, Channels: []string{"exit_status", "stdout", "stderr"}, Outcome: outcome, Witnesses: witnesses}
}

func paired(caseID, channel string, outcome evidence.ComparisonOutcome, changes ...compare.Change) compare.Witness {
	return compare.Witness{
		Relation: "paired", Channel: channel,
		Before:  compare.SampleRef{Side: "base", CaseID: caseID, Repetition: 0},
		After:   compare.SampleRef{Side: "candidate", CaseID: caseID, Repetition: 0},
		Outcome: outcome, Changes: changes,
	}
}

func byteChange(before, after []byte) compare.Change {
	encode := func(value []byte) json.RawMessage {
		raw, _ := json.Marshal(struct {
			Base64 string `json:"base64"`
		}{base64.StdEncoding.EncodeToString(value)})
		return raw
	}
	return compare.Change{Path: "/base64", Kind: "changed", Before: encode(before), After: encode(after)}
}

func TestDeclaredJSONBase64PathIsNotDecodedAsText(t *testing.T) {
	definition := presentationDefinition("json", "text", presentationCase("json"))
	witness := paired("json", "stdout", evidence.Different, compare.Change{
		Path: "/base64", Kind: "changed",
		Before: json.RawMessage(`{"base64":"dGVzdA=="}`), After: json.RawMessage(`{"base64":"bm90"}`),
	})
	presentation, ok := Format(presentationReport(definition, witness), definition)
	if !ok {
		t.Fatal("valid command report was not formatted")
	}
	output := strings.Join(presentation.Lines, "\n")
	if !strings.Contains(output, `{"base64":"`) || !strings.Contains(output, `{"base64":"bm90"}`) || strings.Contains(output, "text before:") || strings.Contains(output, "test → not") {
		t.Fatalf("declared JSON /base64 values were treated as text wrappers: %q", output)
	}
}

func TestTextWitnessSanitizesControlsAndMarksBinaryBytes(t *testing.T) {
	definition := presentationDefinition("json", "text", presentationCase("hostile"))
	control := []byte("bad \x1b]52;c;clipboard\a\r\noutput")
	for _, item := range []struct {
		name  string
		value []byte
		want  string
	}{
		{"terminal-controls", control, `\u001b`},
		{"invalid-utf8", []byte{0xff, 0x00, 'x'}, "[invalid UTF-8, 3 bytes]"},
		{"nul-binary", []byte{'a', 0, 'b'}, "[binary, 3 bytes]"},
	} {
		t.Run(item.name, func(t *testing.T) {
			witness := paired("hostile", "stderr", evidence.Different, byteChange(item.value, []byte("safe")))
			presentation, ok := Format(presentationReport(definition, witness), definition)
			if !ok {
				t.Fatal("valid command report was not formatted")
			}
			output := strings.Join(presentation.Lines, "\n")
			if strings.ContainsAny(output, "\x1b\a\r") || !strings.Contains(output, item.want) {
				t.Fatalf("unsafe or missing text witness display: %q", output)
			}
		})
	}
}

func TestEqualCaseIsOneLineAndManyChangesAreBounded(t *testing.T) {
	definition := presentationDefinition("json", "text", presentationCase("equal"))
	equal := presentationReport(definition,
		paired("equal", "exit_status", evidence.Equal),
		paired("equal", "stdout", evidence.Equal),
		paired("equal", "stderr", evidence.Equal),
	)
	presentation, ok := Format(equal, definition)
	if !ok || presentation.More || len(presentation.Lines) != 1 || !strings.Contains(presentation.Lines[0], "[EQUAL]") {
		t.Fatalf("equal case was not kept to one line: %+v ok=%t", presentation, ok)
	}
	missing := presentationReport(definition, paired("equal", "exit_status", evidence.Equal))
	incomplete, ok := Format(missing, definition)
	if !ok || strings.Contains(strings.Join(incomplete.Lines, "\n"), "[EQUAL]") || !strings.Contains(strings.Join(incomplete.Lines, "\n"), "no complete command witnesses") {
		t.Fatalf("missing channels were presented as equal: %+v ok=%t", incomplete, ok)
	}

	many := presentationDefinition("json", "text", presentationCase("many"))
	changes := make([]compare.Change, 100)
	for index := range changes {
		changes[index] = compare.Change{Path: "/field", Kind: "changed", Before: json.RawMessage("1"), After: json.RawMessage("2")}
	}
	bounded, ok := Format(presentationReport(many,
		paired("many", "exit_status", evidence.Different, changes...),
		paired("many", "stdout", evidence.Different, changes...),
		paired("many", "stderr", evidence.Different, byteChange([]byte("old"), []byte("new"))),
	), many)
	if !ok || !bounded.More || len(bounded.Lines) > 24 {
		t.Fatalf("large witness presentation was not bounded with a continuation pointer: %+v ok=%t", bounded, ok)
	}
	for _, line := range bounded.Lines {
		if width := uniseg.StringWidth(line); width > 76 {
			t.Errorf("display line exceeds width bound (%d): %q", width, line)
		}
	}
}

func TestIsCommandReportRequiresCompleteCommandChannels(t *testing.T) {
	report := compare.Report{DefinitionName: "definition", Channels: []string{"exit_status", "stdout", "stderr"}}
	if !IsCommandReport(report) {
		t.Fatal("command channel set was not recognized")
	}
	report.Channels = []string{"responses", "provider_calls", "stdout"}
	if IsCommandReport(report) {
		t.Fatal("non-command report was recognized as a command")
	}
}
