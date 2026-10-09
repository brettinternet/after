package runner

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"
)

const CommandComparisonRules = `{"version":1,"exit_status":"exact Docker container exit status; statuses >=125 are incomplete","stdout":"exact bytes or definition-declared structural JSON","stderr":"exact bytes or definition-declared structural JSON","text":"exact bytes including invalid UTF-8","json":"exact decimal values, object key order ignored, arrays ordered, null distinct from missing; reject duplicate keys and invalid Unicode","masks":[],"normalization":[],"scope":"finite definition-declared command cases only"}`

const maxCommandCases = 4
const maxCommandInputFiles = 8
const maxCommandStdinBytes = 16 << 10
const maxCommandInputFileBytes = 16 << 10
const maxCommandInputBytes = 24 << 10

// CommandDefinition is a closed v1 contract for direct argv programs. Byte
// inputs use standard padded base64 strings in the JSON representation.
type CommandDefinition struct {
	Version     int               `json:"version"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Platform    string            `json:"platform"`
	Image       string            `json:"image"`
	BuildArgv   []string          `json:"build_argv,omitempty"`
	Cases       []CommandCase     `json:"cases"`
	Repetitions int               `json:"repetitions"`
	Limits      DefinitionLimits  `json:"limits"`
	Comparison  CommandComparison `json:"comparison"`
}

type CommandComparison struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

type CommandCase struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Argv        []string           `json:"argv"`
	Stdin       []byte             `json:"stdin_base64"`
	Environment []string           `json:"environment"`
	InputFiles  []CommandInputFile `json:"input_files,omitempty"`
}

type CommandInputFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content_base64"`
}

func ParseCommandDefinition(raw []byte) (CommandDefinition, error) {
	var definition CommandDefinition
	if len(raw) == 0 || len(raw) > MaxDefinitionBytes || !utf8.Valid(raw) {
		return definition, errors.New("command definition is empty, oversized, or invalid UTF-8")
	}
	if err := rejectDuplicateCommandJSON(raw); err != nil {
		return definition, errors.New("command definition is invalid or ambiguous JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return CommandDefinition{}, errors.New("invalid versioned command definition")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return CommandDefinition{}, errors.New("definition must contain exactly one JSON value")
	}
	if err := commandBase64Fields(raw); err != nil {
		return CommandDefinition{}, err
	}
	if err := definition.Validate(); err != nil {
		return CommandDefinition{}, err
	}
	return definition, nil
}

func rejectDuplicateCommandJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	nodes := 0
	if err := uniqueCommandJSONValue(decoder, 0, &nodes); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}

func uniqueCommandJSONValue(decoder *json.Decoder, depth int, nodes *int) error {
	*nodes++
	if depth > 64 || *nodes > 32768 {
		return errors.New("command JSON structure exceeds bounds")
	}
	token, err := decoder.Token()
	if err != nil {
		return errors.New("invalid command JSON")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return errors.New("invalid or duplicate command JSON object key")
			}
			seen[name] = true
			if err := uniqueCommandJSONValue(decoder, depth+1, nodes); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid command JSON object")
		}
	case '[':
		for decoder.More() {
			if err := uniqueCommandJSONValue(decoder, depth+1, nodes); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid command JSON array")
		}
	default:
		return errors.New("unexpected command JSON delimiter")
	}
	return nil
}

func commandBase64Fields(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return errors.New("invalid command definition object")
	}
	casesRaw, ok := root["cases"]
	if !ok {
		return errors.New("command definition cases are required")
	}
	var cases []map[string]json.RawMessage
	if err := json.Unmarshal(casesRaw, &cases); err != nil {
		return errors.New("command definition cases are invalid")
	}
	decode := func(raw json.RawMessage) ([]byte, error) {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return nil, errors.New("command byte input must be a base64 string")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || base64.StdEncoding.EncodeToString(data) != encoded {
			return nil, errors.New("command byte input must use canonical padded base64")
		}
		return data, nil
	}
	for _, item := range cases {
		stdin, exists := item["stdin_base64"]
		if !exists {
			return errors.New("each command case requires stdin_base64, including an empty base64 string when there is no input")
		}
		if _, err := decode(stdin); err != nil {
			return err
		}
		environment, exists := item["environment"]
		if !exists || len(bytes.TrimSpace(environment)) == 0 || bytes.TrimSpace(environment)[0] != '[' {
			return errors.New("each command case requires a fixed environment array, which may be empty")
		}
		var files []map[string]json.RawMessage
		if rawFiles, exists := item["input_files"]; exists {
			trimmed := bytes.TrimSpace(rawFiles)
			if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal(rawFiles, &files) != nil {
				return errors.New("command input_files must be an array")
			}
		}
		for _, file := range files {
			content, exists := file["content_base64"]
			if !exists {
				return errors.New("each command input file requires content_base64")
			}
			if _, err := decode(content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d CommandDefinition) Validate() error {
	if d.Version != 1 || d.Kind != "command" || !definitionName.MatchString(d.Name) ||
		(d.Platform != "linux/amd64" && d.Platform != "linux/arm64") || !pinnedImage.MatchString(d.Image) ||
		len(d.Cases) < 1 || len(d.Cases) > maxCommandCases || d.Repetitions < 1 || d.Repetitions > 5 ||
		d.Limits.Seconds < 1 || d.Limits.Seconds > 300 || d.Limits.OutputBytes < 1 || d.Limits.OutputBytes > 1<<20 ||
		d.Limits.PreparationSeconds < 1 || d.Limits.PreparationSeconds > 300 ||
		(d.Comparison.Stdout != "text" && d.Comparison.Stdout != "json") || (d.Comparison.Stderr != "text" && d.Comparison.Stderr != "json") {
		return errors.New("command definition fields are incomplete or outside bounded v1 limits")
	}
	if err := validateArgv(d.BuildArgv, true); err != nil {
		return errors.New("invalid command build_argv")
	}
	seenCases := map[string]bool{}
	for _, scenarioCase := range d.Cases {
		if !definitionName.MatchString(scenarioCase.ID) || seenCases[scenarioCase.ID] || strings.TrimSpace(scenarioCase.Title) == "" || len(scenarioCase.Title) > 128 || len(scenarioCase.Stdin) > maxCommandStdinBytes {
			return errors.New("invalid or duplicate command case")
		}
		seenCases[scenarioCase.ID] = true
		if err := validateArgv(scenarioCase.Argv, false); err != nil {
			return errors.New("invalid command case argv")
		}
		seenEnvironment := map[string]bool{}
		for _, entry := range scenarioCase.Environment {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || !envName.MatchString(key) || len(entry) > 1024 || strings.ContainsAny(value, "\x00\r\n") || seenEnvironment[key] || restrictedCommandEnvironment(key) {
				return errors.New("invalid or unsafe command environment entry")
			}
			seenEnvironment[key] = true
		}
		if len(scenarioCase.Environment) > 32 || len(scenarioCase.InputFiles) > maxCommandInputFiles {
			return errors.New("command case environment or input file count exceeds bounds")
		}
		inputPaths := map[string]bool{}
		inputBytes := 0
		for _, file := range scenarioCase.InputFiles {
			if !validCommandInputPath(file.Path) || len(file.Content) > maxCommandInputFileBytes || pathConflictsRuntime(file.Path) {
				return errors.New("invalid command input file path or size")
			}
			for prior := range inputPaths {
				if pathsConflict(prior, file.Path) {
					return errors.New("command input file paths collide")
				}
			}
			inputPaths[file.Path] = true
			inputBytes += len(file.Content)
		}
		if inputBytes > maxCommandInputBytes {
			return errors.New("command input files exceed their aggregate byte limit")
		}
	}
	return nil
}

func validCommandInputPath(value string) bool {
	if value == "" || value == "." || len(value) > 240 || path.Clean(value) != value || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "../") || value == ".." || strings.ContainsAny(value, "\\\x00\r\n") {
		return false
	}
	return true
}

func restrictedCommandEnvironment(key string) bool {
	return restrictedEnvironment(key) || key == "GOCACHE" || key == "GOPATH" || key == "GOPROXY" || key == "GOSUMDB" || key == "GOTOOLCHAIN" || key == "CGO_ENABLED" || key == "GOMAXPROCS" || key == "LANG"
}

func pathConflictsRuntime(value string) bool {
	return pathsConflict(value, "after/launcher") || pathsConflict(value, "after/service.json")
}

func pathsConflict(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// ParseSelectedDefinition selects only between the two closed v1 data schemas.
// It never discovers a definition in either captured snapshot.
func ParseSelectedDefinition(raw []byte) (string, error) {
	if len(raw) == 0 || len(raw) > MaxDefinitionBytes || !utf8.Valid(raw) {
		return "", errors.New("selected definition is empty, oversized, or invalid UTF-8")
	}
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return "", errors.New("invalid selected definition")
	}
	switch header.Kind {
	case "http-service":
		_, err := ParseDefinition(raw)
		return header.Kind, err
	case "command":
		_, err := ParseCommandDefinition(raw)
		return header.Kind, err
	default:
		return "", errors.New("selected definition kind must be http-service or command")
	}
}

func commandLimits(d CommandDefinition) []string {
	return append(append([]string(nil), commandScope...),
		"Build and command run in the same fresh, offline container for every side, case and repetition; build output is discarded and helper diagnostics are kept separate from compared streams.",
		"Exit statuses 125–255 are reserved for helper/build/exec failures and possible signal termination; intentional application exits in that range are unsupported and incomplete.",
		"Optional input files are additive read-only captured overlays; paths that traverse, collide with a snapshot path, or overlap another input/runtime file are rejected.",
		"Text streams compare as exact bytes, including invalid UTF-8; declared JSON streams compare structurally and malformed JSON is incomparable.",
		fmt.Sprintf("Command definition %s is frozen independently of the snapshot pair; %d case(s), %d repetition(s), %ds sample, %d-byte combined stream and %ds preparation limits.", d.Name, len(d.Cases), d.Repetitions, d.Limits.Seconds, d.Limits.OutputBytes, d.Limits.PreparationSeconds))
}

var commandScope = []string{
	"Only finite, explicitly selected direct-argv command cases on the captured base/candidate pair; not universal correctness, causation or performance.",
	"No TTY, output file-tree comparison, shell entrypoint, dependency download, external network, host process fallback or observer container.",
	"Docker daemon, CLI, pinned image, kernel and shipped command launcher are trusted; candidate program bytes and terminal output are untrusted.",
}
