package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestPrecedencePresenceAndSources(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config with spaces")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "after.yaml")
	if err := os.WriteFile(configPath, []byte("project: yaml-project\nrepetitions: 2\nrun_seconds: 90\noutput_bytes: 8192\ninteractive: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env := environment(map[string]string{
		"AFTER_CONFIG": configPath, "AFTER_PROJECT": " env-project ",
		"AFTER_REPETITIONS": "3", "AFTER_RUN_SECONDS": "120",
		"AFTER_OUTPUT_BYTES": "16384", "AFTER_INTERACTIVE": "false",
		"AFTER_DOCKER_BINARY": "/trusted/bin/docker", "AFTER_DOCKER_HOST": "unix:///tmp/docker.sock",
	})
	cfg, err := Load(Input{Env: env, WorkingDir: dir, Flags: map[string]FlagValue{
		"project":      {Value: " flag-project ", Set: true},
		"repetitions":  {Value: 4, Set: true},
		"interactive":  {Value: false, Set: true},
		"output_bytes": {Set: false, Value: 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wantProject := filepath.Join(dir, "flag-project")
	if cfg.Project != wantProject || cfg.Repetitions != 4 || cfg.RunSeconds != 120 || cfg.OutputBytes != 16384 || cfg.Interactive {
		t.Fatalf("resolved values: %+v", cfg)
	}
	for key, want := range map[string]string{
		"project": "flag", "repetitions": "flag", "run_seconds": "env",
		"output_bytes": "env", "interactive": "flag", "config_path": "env",
		"docker_binary": "env", "docker_host": "env",
	} {
		if got := cfg.Sources[key]; got != want {
			t.Errorf("source %s = %q, want %q", key, got, want)
		}
	}
	if cfg.DockerBinary != "/trusted/bin/docker" || cfg.DockerHost != "unix:///tmp/docker.sock" {
		t.Fatal("explicit Docker environment lost")
	}
}

func TestFileAndDefaultSourcesIncludingExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	configPath := filepath.Join(home, ".config", "after", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("project: ''\nrepetitions: 0\nrun_seconds: 60\noutput_bytes: 2048\ninteractive: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env := environment(map[string]string{"HOME": home, "AFTER_REPETITIONS": " ", "AFTER_INTERACTIVE": ""})
	cfg, err := Load(Input{Env: env, WorkingDir: dir})
	if err == nil || !strings.Contains(err.Error(), "repetitions from file") {
		t.Fatalf("explicit zero should be validated, not treated as absent: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("project: ''\nrepetitions: 1\nrun_seconds: 60\noutput_bytes: 2048\ninteractive: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(Input{Env: env, WorkingDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project != dir || cfg.Interactive || cfg.Sources["interactive"] != "file" || cfg.Sources["project"] != "default" || cfg.Sources["repetitions"] != "file" || cfg.Sources["config_path"] != "default" {
		t.Fatalf("file/default resolution: %+v", cfg)
	}
}

func TestOptionalDiscoveryAndExplicitMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(Input{Env: environment(map[string]string{"HOME": dir}), WorkingDir: dir})
	if err != nil || cfg.ConfigPath != filepath.Join(dir, ".config", "after", "config.yaml") || cfg.Sources["project"] != "default" {
		t.Fatalf("missing discovered file should be optional: %+v %v", cfg, err)
	}
	_, err = Load(Input{Env: environment(map[string]string{"HOME": dir, "AFTER_CONFIG": filepath.Join(dir, "missing.yaml")}), WorkingDir: dir})
	if err == nil || !strings.Contains(err.Error(), "config_path from env") {
		t.Fatalf("explicit missing environment file should fail with source: %v", err)
	}
	_, err = Load(Input{Env: environment(map[string]string{}), WorkingDir: dir, Flags: map[string]FlagValue{"config": {Value: filepath.Join(dir, "missing flag.yaml"), Set: true}}})
	if err == nil || !strings.Contains(err.Error(), "config_path from flag") {
		t.Fatalf("explicit missing flag file should fail with source: %v", err)
	}
}

func TestStrictYAMLFailures(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"unknown":            "extra: value\n",
		"consent-setting":    "authorization: true\n",
		"duplicate":          "interactive: true\ninteractive: false\n",
		"duplicate-null":     "raw_diff: null\nraw_diff: false\n",
		"wrong-type":         "interactive: \"false\"\n",
		"nested":             "project: {path: x}\n",
		"multiple-documents": "project: x\n---\nproject: y\n",
		"malformed":          "project: [\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".yaml")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(Input{WorkingDir: dir, Env: environment(map[string]string{}), Flags: map[string]FlagValue{"config": {Value: path, Set: true}}})
			if err == nil || !strings.Contains(err.Error(), "from file") {
				t.Fatalf("invalid YAML accepted or missing source: %v", err)
			}
		})
	}
}

func TestInvalidDockerSettingReturnsEffectiveConfigurationForSetupDisplay(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(Input{WorkingDir: dir, Env: environment(map[string]string{
		"AFTER_DOCKER_BINARY": "relative/docker",
	})})
	var configErr *Error
	if err == nil || !errors.As(err, &configErr) || configErr.Setting != "docker_binary" {
		t.Fatalf("invalid Docker setting was not identified: %v", err)
	}
	if cfg.DockerBinary != "relative/docker" || cfg.Sources["docker_binary"] != "env" {
		t.Fatalf("effective invalid value/source lost for setup reporting: %+v", cfg)
	}
}

func TestFalseAndSupportedZeroKeepTheirSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("raw_diff: true\ndiff_bytes: 4096\ninteractive: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env := environment(map[string]string{
		"AFTER_CONFIG": path, "AFTER_RAW_DIFF": "false", "AFTER_DIFF_BYTES": "0",
		"AFTER_INTERACTIVE": "false",
	})
	cfg, err := Load(Input{WorkingDir: dir, Env: env, Flags: map[string]FlagValue{
		"config":      {Value: "", Set: true},
		"raw_diff":    {Value: false, Set: true},
		"diff_bytes":  {Value: 0, Set: true},
		"interactive": {Value: true, Set: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RawDiff || cfg.DiffBytes != 0 || !cfg.Interactive || cfg.Sources["raw_diff"] != "flag" || cfg.Sources["diff_bytes"] != "flag" || cfg.Sources["interactive"] != "flag" {
		t.Fatalf("explicit false/zero lost: %+v", cfg)
	}

	cfg, err = Load(Input{WorkingDir: dir, Env: env})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RawDiff || cfg.DiffBytes != 0 || cfg.Interactive || cfg.Sources["raw_diff"] != "env" || cfg.Sources["diff_bytes"] != "env" || cfg.Sources["interactive"] != "env" {
		t.Fatalf("environment false/zero lost: %+v", cfg)
	}

	env = environment(map[string]string{"HOME": dir})
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, ".config", "after", "config.yaml")), 0700); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(dir, ".config", "after", "config.yaml")
	if err := os.WriteFile(defaultPath, []byte("raw_diff: false\ndiff_bytes: 0\ninteractive: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(Input{WorkingDir: dir, Env: env})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RawDiff || cfg.DiffBytes != 0 || cfg.Interactive || cfg.Sources["raw_diff"] != "file" || cfg.Sources["diff_bytes"] != "file" {
		t.Fatalf("YAML false/zero lost: %+v", cfg)
	}
}

func TestInvalidEnvironmentAndFlagZeroAreNotDefaults(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(Input{WorkingDir: dir, Env: environment(map[string]string{"AFTER_REPETITIONS": "0"})})
	if err == nil || !strings.Contains(err.Error(), "repetitions from env") {
		t.Fatalf("zero environment value lost: %v", err)
	}
	_, err = Load(Input{WorkingDir: dir, Env: environment(map[string]string{"AFTER_INTERACTIVE": "invalid"})})
	if err == nil || !strings.Contains(err.Error(), "interactive from env") {
		t.Fatalf("invalid environment boolean accepted: %v", err)
	}
	_, err = Load(Input{WorkingDir: dir, Env: environment(map[string]string{}), Flags: map[string]FlagValue{"repetitions": {Value: 0, Set: true}}})
	if err == nil || !strings.Contains(err.Error(), "repetitions from flag") {
		t.Fatalf("zero explicit flag lost: %v", err)
	}
}
