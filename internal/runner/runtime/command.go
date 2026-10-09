//go:build ignore

// Trusted direct-command launcher; runs only in the approved command container.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type commandConfig struct {
	Version     int      `json:"version"`
	BuildArgv   []string `json:"build_argv,omitempty"`
	Argv        []string `json:"argv"`
	Environment []string `json:"environment"`
}

func main() {
	if err := run(); err != nil {
		_, _ = io.WriteString(os.Stderr, "AFTER command helper failed\n")
		os.Exit(126)
	}
}

func run() error {
	raw, err := os.ReadFile("/input/after/service.json")
	if err != nil || len(raw) > 64<<10 {
		return errors.New("invalid frozen command configuration")
	}
	var cfg commandConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cfg) != nil || decoder.Decode(new(any)) != io.EOF || cfg.Version != 1 || len(cfg.Argv) == 0 || len(cfg.Argv) > 64 || len(cfg.BuildArgv) > 64 || len(cfg.Environment) > 64 {
		return errors.New("unsupported command configuration")
	}
	for _, argv := range [][]string{cfg.BuildArgv, cfg.Argv} {
		for _, arg := range argv {
			if arg == "" || len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
				return errors.New("invalid direct command argv")
			}
		}
	}
	environment, err := fixedEnvironment(cfg.Environment)
	if err != nil {
		return err
	}
	if len(cfg.BuildArgv) > 0 {
		build := exec.Command(cfg.BuildArgv[0], cfg.BuildArgv[1:]...)
		build.Env = fixedEnvironmentBase()
		build.Stdin = strings.NewReader("")
		build.Stdout, build.Stderr = io.Discard, io.Discard
		if err := build.Run(); err != nil {
			return errors.New("command build failed")
		}
	}
	if err := syscall.Exec(cfg.Argv[0], cfg.Argv, environment); err != nil {
		return errors.New("command exec failed")
	}
	return nil
}

func fixedEnvironment(values []string) ([]string, error) {
	out := fixedEnvironmentBase()
	seen := map[string]bool{}
	for _, entry := range out {
		key, _, _ := strings.Cut(entry, "=")
		seen[key] = true
	}
	for _, entry := range values {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || seen[key] || strings.ContainsAny(entry, "\x00\r\n") {
			return nil, errors.New("invalid or duplicate fixed environment entry")
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out, nil
}

func fixedEnvironmentBase() []string {
	return []string{"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", "HOME=/work", "TMPDIR=/work", "GOCACHE=/work/cache", "GOPATH=/work/go", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOMAXPROCS=2", "LANG=C"}
}
