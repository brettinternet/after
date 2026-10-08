// Package config resolves bounded, non-authorizing CLI configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

const MaxFileBytes = 64 << 10

// FlagValue keeps explicit parser presence separate from parser defaults.
type FlagValue struct {
	Value any
	Set   bool
}

// Input makes CLI presence, environment lookup, and cwd explicit and testable.
type Input struct {
	Flags      map[string]FlagValue
	Env        func(string) string
	WorkingDir string
}

// Config contains effective settings and the source which supplied each one.
// Docker paths are intentionally omitted from JSON serialization by callers.
type Config struct {
	Project      string            `json:"project"`
	ConfigPath   string            `json:"config_path"`
	Repetitions  int               `json:"repetitions"`
	RunSeconds   int               `json:"run_seconds"`
	OutputBytes  int               `json:"output_bytes"`
	Interactive  bool              `json:"interactive"`
	RawDiff      bool              `json:"raw_diff"`
	DiffBytes    int               `json:"diff_bytes"`
	DockerBinary string            `json:"-"`
	DockerHost   string            `json:"-"`
	Sources      map[string]string `json:"sources"`
}

// Error identifies the setting and layer that needs correction without
// echoing configuration contents or filesystem paths.
type Error struct {
	Setting string
	Source  string
	Reason  string
}

// Defaults returns the shipped configuration defaults without consulting the
// environment, filesystem, or user configuration.
func Defaults() Config {
	return Config{
		Repetitions: 1, RunSeconds: 180, OutputBytes: 65536,
		Interactive: true, RawDiff: true, DiffBytes: 64 << 10,
		Sources: map[string]string{
			"config_path": "default", "project": "default", "repetitions": "default",
			"run_seconds": "default", "output_bytes": "default", "interactive": "default",
			"raw_diff": "default", "diff_bytes": "default", "docker_binary": "default", "docker_host": "default",
		},
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("invalid configuration setting %s from %s: %s", e.Setting, e.Source, e.Reason)
}

type rawValue struct {
	value  any
	source string
	set    bool
}

type envParseError struct{}

var settingKeys = map[string]bool{
	"project": true, "repetitions": true, "run_seconds": true,
	"output_bytes": true, "interactive": true, "raw_diff": true,
	"diff_bytes": true, "docker_binary": true, "docker_host": true,
}

var envNames = map[string]string{
	"config_path": "AFTER_CONFIG", "project": "AFTER_PROJECT",
	"repetitions": "AFTER_REPETITIONS", "run_seconds": "AFTER_RUN_SECONDS",
	"output_bytes": "AFTER_OUTPUT_BYTES", "interactive": "AFTER_INTERACTIVE",
	"raw_diff": "AFTER_RAW_DIFF", "diff_bytes": "AFTER_DIFF_BYTES",
	"docker_binary": "AFTER_DOCKER_BINARY", "docker_host": "AFTER_DOCKER_HOST",
}

// Load applies explicit flag > AFTER_* environment > strict YAML > defaults.
// Empty strings and YAML null are unset. Explicit false and zero remain values.
func Load(in Input) (Config, error) {
	env := in.Env
	if env == nil {
		env = os.Getenv
	}
	cwd := in.WorkingDir
	if strings.TrimSpace(cwd) == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return Config{}, invalid("project", "default", "cannot determine working directory")
		}
	}
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return Config{}, invalid("project", "default", "invalid working directory")
	}
	flags := in.Flags
	if flags == nil {
		flags = map[string]FlagValue{}
	}
	for key, value := range flags {
		if key != "config" && !settingKeys[key] {
			return Config{}, invalid(safeSetting(key), "flag", "unknown setting")
		}
		if !value.Set {
			continue
		}
		if !validFlagType(key, value.Value) {
			return Config{}, invalid(key, "flag", "has an invalid value type")
		}
	}

	configSelection := flagRaw(flags, "config")
	if !configSelection.set {
		configSelection = envRaw(env, "config_path")
	}
	configPath, configSource, optional := "", "default", true
	if configSelection.set && !emptyString(configSelection.value) {
		configPath = configSelection.value.(string)
		configSource, optional = configSelection.source, false
	} else {
		configPath = defaultConfigPath(env)
	}
	if configPath != "" {
		configPath, err = absolutePath(configPath, cwd, env)
		if err != nil {
			return Config{}, invalid("config_path", configSource, "must be a valid path")
		}
	}
	fileValues := map[string]any{}
	if configPath != "" {
		fileValues, err = loadFile(configPath, optional, configSource)
		if err != nil {
			return Config{}, err
		}
	}

	resolved := make(map[string]rawValue, len(settingKeys))
	for key := range settingKeys {
		resolved[key] = fileRaw(fileValues, key)
		if value := envRaw(env, key); value.set && !emptyString(value.value) {
			resolved[key] = value
		}
		if value := flagRaw(flags, key); value.set && !emptyString(value.value) {
			resolved[key] = value
		}
	}
	cfg := Defaults()
	cfg.ConfigPath = configPath
	cfg.Sources["config_path"] = configSource
	project := cwd
	if err := setString("project", resolved, &project, cfg.Sources); err != nil {
		return Config{}, err
	}
	project, err = absolutePath(strings.TrimSpace(project), cwd, env)
	if err != nil {
		return Config{}, invalid("project", cfg.Sources["project"], "must be a valid path")
	}
	cfg.Project = project
	if err := setInt("repetitions", resolved, &cfg.Repetitions, cfg.Sources, 1, 5); err != nil {
		return Config{}, err
	}
	if err := setInt("run_seconds", resolved, &cfg.RunSeconds, cfg.Sources, 1, 300); err != nil {
		return Config{}, err
	}
	if err := setInt("output_bytes", resolved, &cfg.OutputBytes, cfg.Sources, 1, 1<<20); err != nil {
		return Config{}, err
	}
	if err := setBool("interactive", resolved, &cfg.Interactive, cfg.Sources); err != nil {
		return Config{}, err
	}
	if err := setBool("raw_diff", resolved, &cfg.RawDiff, cfg.Sources); err != nil {
		return Config{}, err
	}
	if err := setInt("diff_bytes", resolved, &cfg.DiffBytes, cfg.Sources, 0, 64<<10); err != nil {
		return Config{}, err
	}
	if err := setString("docker_binary", resolved, &cfg.DockerBinary, cfg.Sources); err != nil {
		return Config{}, err
	}
	if err := setString("docker_host", resolved, &cfg.DockerHost, cfg.Sources); err != nil {
		return Config{}, err
	}
	if cfg.DockerBinary != "" && !filepath.IsAbs(cfg.DockerBinary) {
		return cfg, invalid("docker_binary", cfg.Sources["docker_binary"], "must be an absolute path")
	}
	if cfg.DockerHost != "" && (!strings.HasPrefix(cfg.DockerHost, "unix:///") || strings.ContainsAny(cfg.DockerHost, "\x00\r\n")) {
		return cfg, invalid("docker_host", cfg.Sources["docker_host"], "must be an explicit local Unix socket")
	}
	if (cfg.DockerBinary == "") != (cfg.DockerHost == "") {
		return cfg, invalid("docker_binary", cfg.Sources["docker_binary"], "binary and local socket must both be configured")
	}
	return cfg, nil
}

func validFlagType(key string, value any) bool {
	switch key {
	case "config":
		_, ok := value.(string)
		return ok
	case "project", "docker_binary", "docker_host":
		_, ok := value.(string)
		return ok
	case "interactive", "raw_diff":
		_, ok := value.(bool)
		return ok
	case "repetitions", "run_seconds", "output_bytes", "diff_bytes":
		_, ok := value.(int)
		return ok
	default:
		return false
	}
}

func flagRaw(flags map[string]FlagValue, key string) rawValue {
	value, ok := flags[key]
	if !ok || !value.Set || emptyString(value.Value) {
		return rawValue{}
	}
	return rawValue{value: value.Value, source: "flag", set: true}
}

func envRaw(env func(string) string, key string) rawValue {
	name, ok := envNames[key]
	if !ok {
		return rawValue{}
	}
	text := env(name)
	if strings.TrimSpace(text) == "" {
		return rawValue{}
	}
	var value any = text
	switch key {
	case "interactive", "raw_diff":
		parsed, err := strconv.ParseBool(strings.TrimSpace(text))
		if err != nil {
			value = envParseError{}
		} else {
			value = parsed
		}
	case "repetitions", "run_seconds", "output_bytes", "diff_bytes":
		parsed, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			value = envParseError{}
		} else {
			value = parsed
		}
	}
	return rawValue{value: value, source: "env", set: true}
}

func fileRaw(values map[string]any, key string) rawValue {
	value, ok := values[key]
	if !ok || emptyString(value) {
		return rawValue{}
	}
	return rawValue{value: value, source: "file", set: true}
}

func emptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func setString(key string, values map[string]rawValue, dst *string, sources map[string]string) error {
	value := values[key]
	if !value.set {
		return nil
	}
	text, ok := value.value.(string)
	if !ok {
		return invalid(key, value.source, "must be a string")
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	*dst = strings.TrimSpace(text)
	sources[key] = value.source
	return nil
}

func setInt(key string, values map[string]rawValue, dst *int, sources map[string]string, min, max int) error {
	value := values[key]
	if !value.set {
		return nil
	}
	n, ok := value.value.(int)
	if !ok {
		return invalid(key, value.source, "must be an integer")
	}
	if n < min || n > max {
		return invalid(key, value.source, fmt.Sprintf("must be between %d and %d", min, max))
	}
	*dst = n
	sources[key] = value.source
	return nil
}

func setBool(key string, values map[string]rawValue, dst *bool, sources map[string]string) error {
	value := values[key]
	if !value.set {
		return nil
	}
	b, ok := value.value.(bool)
	if !ok {
		return invalid(key, value.source, "must be a boolean")
	}
	*dst = b
	sources[key] = value.source
	return nil
}

func defaultConfigPath(env func(string) string) string {
	if base := strings.TrimSpace(env("XDG_CONFIG_HOME")); base != "" {
		return filepath.Join(base, "after", "config.yaml")
	}
	if home := strings.TrimSpace(env("HOME")); home != "" {
		return filepath.Join(home, ".config", "after", "config.yaml")
	}
	return ""
}

func absolutePath(value, cwd string, env func(string) string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("empty path")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home := strings.TrimSpace(env("HOME"))
		if home == "" {
			return "", errors.New("home is unavailable")
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	return filepath.Abs(filepath.Clean(value))
}

func loadFile(path string, optional bool, selectionSource string) (map[string]any, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && optional {
			return map[string]any{}, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, invalid("config_path", selectionSource, "explicit configuration file does not exist")
		}
		return nil, invalid("config_path", selectionSource, "cannot open configuration file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, invalid("config_path", selectionSource, "must be a regular non-symlink file")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, invalid("config_path", selectionSource, "cannot read configuration file")
	}
	if len(data) > MaxFileBytes {
		return nil, invalid("config", "file", "exceeds the 64 KiB limit")
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, invalid("config", "file", yamlFailure(err))
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, invalid("config", "file", "must contain exactly one YAML document")
	}
	if len(root.Content) == 0 || root.Content[0].Kind == 0 {
		return map[string]any{}, nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, invalid("config", "file", "must be a mapping")
	}
	values := make(map[string]any)
	seen := make(map[string]bool)
	for i := 0; i < len(doc.Content); i += 2 {
		keyNode, valueNode := doc.Content[i], doc.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			return nil, invalid("config", "file", "keys must be strings")
		}
		key := keyNode.Value
		if !settingKeys[key] {
			return nil, invalid(safeSetting(key), "file", "unknown setting")
		}
		if seen[key] {
			return nil, invalid(key, "file", "duplicate setting")
		}
		seen[key] = true
		if valueNode.Kind == yaml.AliasNode || valueNode.Anchor != "" {
			return nil, invalid(key, "file", "aliases and anchors are not supported")
		}
		if valueNode.Tag == "!!null" {
			continue
		}
		if valueNode.Kind != yaml.ScalarNode {
			return nil, invalid(key, "file", "must be a scalar")
		}
		var value any
		switch key {
		case "project", "docker_binary", "docker_host":
			if valueNode.Tag != "!!str" {
				return nil, invalid(key, "file", "must be a string")
			}
			value = valueNode.Value
		case "interactive", "raw_diff":
			if valueNode.Tag != "!!bool" {
				return nil, invalid(key, "file", "must be a boolean")
			}
			parsed, err := strconv.ParseBool(valueNode.Value)
			if err != nil {
				return nil, invalid(key, "file", "must be a boolean")
			}
			value = parsed
		default:
			if valueNode.Tag != "!!int" {
				return nil, invalid(key, "file", "must be an integer")
			}
			var parsed int
			if err := valueNode.Decode(&parsed); err != nil {
				return nil, invalid(key, "file", "must be a supported integer")
			}
			value = parsed
		}
		values[key] = value
	}
	return values, nil
}

func safeSetting(key string) string {
	if key == "" {
		return "<unknown>"
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "<unknown>"
		}
	}
	return key
}

func yamlFailure(err error) string {
	line := 0
	for _, field := range strings.Fields(err.Error()) {
		if strings.HasPrefix(field, "line") {
			_, _ = fmt.Sscanf(strings.TrimSuffix(field, ":"), "line%d", &line)
		}
	}
	if line > 0 {
		return fmt.Sprintf("malformed YAML at line %d", line)
	}
	return "malformed YAML"
}

func invalid(setting, source, reason string) error {
	return &Error{Setting: safeSetting(setting), Source: safeSetting(source), Reason: reason}
}
