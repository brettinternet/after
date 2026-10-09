package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/sandbox"
)

func validCommandDefinition() CommandDefinition {
	return CommandDefinition{
		Version: 1, Kind: "command", Name: "synthetic-cli", Platform: "linux/arm64", Image: sandbox.Image,
		BuildArgv: []string{"/usr/local/go/bin/go", "build", "-o", "/work/cli", "/input/main.go"},
		Cases: []CommandCase{{
			ID: "changed", Title: "changed behavior", Argv: []string{"/work/cli", "changed"}, Stdin: []byte{0, 255, 1}, Environment: []string{"FIXTURE_MODE=finite"},
			InputFiles: []CommandInputFile{{Path: "fixtures/input.bin", Content: []byte{0, 255}}},
		}},
		Repetitions: 2, Limits: DefinitionLimits{Seconds: 90, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: CommandComparison{Stdout: "text", Stderr: "json"},
	}
}

func TestCommandDefinitionStrictBoundsAndBase64Bytes(t *testing.T) {
	definition := validCommandDefinition()
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCommandDefinition(raw)
	if err != nil || string(parsed.Cases[0].Stdin) != string(definition.Cases[0].Stdin) || string(parsed.Cases[0].InputFiles[0].Content) != string(definition.Cases[0].InputFiles[0].Content) {
		t.Fatalf("binary byte inputs did not survive strict decoding: %+v %v", parsed, err)
	}
	if kind, err := ParseSelectedDefinition(raw); err != nil || kind != "command" {
		t.Fatalf("selected command kind: %q %v", kind, err)
	}
	for name, bad := range map[string][]byte{
		"unknown top-level field":    append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"future_control":true}`)...),
		"trailing JSON":              append(append([]byte(nil), raw...), []byte(` {}`)...),
		"duplicate definition field": []byte(strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1)),
		"duplicate case field":       []byte(strings.Replace(string(raw), `"title":"changed behavior"`, `"title":"other","title":"changed behavior"`, 1)),
		"null fixed environment":     []byte(strings.Replace(string(raw), `"environment":["FIXTURE_MODE=finite"]`, `"environment":null`, 1)),
		"null input files":           []byte(strings.Replace(string(raw), `"input_files":[{"path":"fixtures/input.bin","content_base64":"AP8="}]`, `"input_files":null`, 1)),
		"missing stdin bytes":        []byte(strings.Replace(string(raw), `"stdin_base64":"AP8B"`, `"environment":["FIXTURE_MODE=finite"]`, 1)),
		"invalid base64":             []byte(strings.Replace(string(raw), `"content_base64":"AP8="`, `"content_base64":"!"`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCommandDefinition(bad); err == nil {
				t.Fatal("invalid command definition was accepted")
			}
		})
	}
}

func TestCommandDefinitionRejectsUnsafeArgvEnvironmentAndPathCollisions(t *testing.T) {
	for name, mutate := range map[string]func(*CommandDefinition){
		"shell entrypoint":      func(d *CommandDefinition) { d.Cases[0].Argv = []string{"/bin/sh", "-c", "echo unsafe"} },
		"shell command string":  func(d *CommandDefinition) { d.BuildArgv = []string{"/usr/local/bin/go", "run", "-c", "unsafe"} },
		"ambient PATH":          func(d *CommandDefinition) { d.Cases[0].Environment = []string{"PATH=/tmp"} },
		"path traversal":        func(d *CommandDefinition) { d.Cases[0].InputFiles[0].Path = "../escape" },
		"directory path":        func(d *CommandDefinition) { d.Cases[0].InputFiles[0].Path = "." },
		"reserved runtime file": func(d *CommandDefinition) { d.Cases[0].InputFiles[0].Path = "after/launcher/extra" },
		"file ancestor collision": func(d *CommandDefinition) {
			d.Cases[0].InputFiles = append(d.Cases[0].InputFiles, CommandInputFile{Path: "fixtures", Content: []byte("ancestor")})
		},
		"oversized stdin": func(d *CommandDefinition) { d.Cases[0].Stdin = make([]byte, maxCommandStdinBytes+1) },
		"oversized file count": func(d *CommandDefinition) {
			for index := 1; index <= maxCommandInputFiles; index++ {
				d.Cases[0].InputFiles = append(d.Cases[0].InputFiles, CommandInputFile{Path: "extra/file" + string(rune('a'+index)), Content: []byte("x")})
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			definition := validCommandDefinition()
			mutate(&definition)
			raw, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseCommandDefinition(raw); err == nil {
				t.Fatal("unsafe or over-bound definition was accepted")
			}
		})
	}
}
