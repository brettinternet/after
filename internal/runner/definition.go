package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/sandbox"
)

const MaxDefinitionBytes = 64 << 10
const maxCases = 4
const maxRequestsPerCase = 16
const maxUpstreams = 4
const maxProviderCalls = 128
const readinessABI = "fd3-http-url-v1"
const preparerBudgetSeconds = 90
const launcherMaxBytes = 8 << 20

var definitionName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var headerName = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]{1,64}$`)
var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
var pinnedImage = regexp.MustCompile(`^[A-Za-z0-9._/:+-]+@sha256:[a-f0-9]{64}$`)

// Definition is the closed, versioned http-service input. The exact source
// bytes, not a re-encoding of these fields, are frozen as the scenario input.
type Definition struct {
	Version     int              `json:"version"`
	Kind        string           `json:"kind"`
	Name        string           `json:"name"`
	Platform    string           `json:"platform"`
	Image       string           `json:"image"`
	BuildArgv   []string         `json:"build_argv"`
	StartArgv   []string         `json:"start_argv"`
	Environment []string         `json:"environment"`
	Readiness   Readiness        `json:"readiness"`
	Epoch       int64            `json:"epoch"`
	Upstreams   []Upstream       `json:"upstreams"`
	Cases       []Case           `json:"cases"`
	Channels    []string         `json:"channels"`
	Repetitions int              `json:"repetitions"`
	Limits      DefinitionLimits `json:"limits"`
}

type Readiness struct {
	Protocol       string `json:"protocol"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type Upstream struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Port           int    `json:"port"`
	ResponseStatus int    `json:"response_status,omitempty"`
	ResponseBody   string `json:"response_body,omitempty"`
}

type Case struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Requests []Request `json:"requests"`
}

type Request struct {
	AfterSeconds int64             `json:"after_seconds"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body"`
}

type DefinitionLimits struct {
	Seconds            int `json:"seconds"`
	OutputBytes        int `json:"output_bytes"`
	PreparationSeconds int `json:"preparation_seconds"`
}

type DefinitionSource struct {
	Kind           string `json:"kind"`
	RepositoryPath string `json:"repository_path,omitempty"`
	ChangedOracle  bool   `json:"changed_oracle"`
}

func ParseDefinition(raw []byte) (Definition, error) {
	var definition Definition
	if len(raw) == 0 || len(raw) > MaxDefinitionBytes || !utf8.Valid(raw) {
		return definition, errors.New("http-service definition is empty, oversized, or invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return Definition{}, errors.New("invalid versioned http-service definition")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Definition{}, errors.New("definition must contain exactly one JSON value")
	}
	if err := definition.Validate(); err != nil {
		return Definition{}, err
	}
	return definition, nil
}

func (d Definition) Validate() error {
	if d.Version != 1 || d.Kind != "http-service" || !definitionName.MatchString(d.Name) ||
		(d.Platform != "linux/amd64" && d.Platform != "linux/arm64") || !pinnedImage.MatchString(d.Image) ||
		len(d.BuildArgv) > 64 || len(d.StartArgv) == 0 || len(d.StartArgv) > 64 ||
		d.Readiness.Protocol != readinessABI || d.Readiness.TimeoutSeconds < 1 || d.Readiness.TimeoutSeconds > 120 ||
		d.Epoch < 1 || len(d.Upstreams) > maxUpstreams || len(d.Cases) < 1 || len(d.Cases) > maxCases ||
		d.Repetitions < 1 || d.Repetitions > 5 || d.Limits.Seconds < 1 || d.Limits.Seconds > 300 ||
		d.Limits.OutputBytes < 1 || d.Limits.OutputBytes > 1<<20 || d.Limits.PreparationSeconds < 1 || d.Limits.PreparationSeconds > 300 {
		return errors.New("http-service definition fields are incomplete or outside bounded v1 limits")
	}
	if err := validateArgv(d.BuildArgv, true); err != nil {
		return fmt.Errorf("invalid build_argv: %w", err)
	}
	if err := validateArgv(d.StartArgv, false); err != nil {
		return fmt.Errorf("invalid start_argv: %w", err)
	}
	seenEnvironment := map[string]bool{}
	for _, entry := range d.Environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !envName.MatchString(key) || value == "" || len(entry) > 1024 || strings.ContainsAny(entry, "\x00\r\n") || seenEnvironment[key] || restrictedEnvironment(key) {
			return errors.New("invalid or unsafe service environment entry")
		}
		seenEnvironment[key] = true
	}
	ports := map[int]bool{18080: true}
	names := map[string]bool{}
	clockCount := 0
	for _, endpoint := range d.Upstreams {
		if !definitionName.MatchString(endpoint.Name) || names[endpoint.Name] || ports[endpoint.Port] || endpoint.Port < 18081 || endpoint.Port > 18080+maxUpstreams || (endpoint.Kind != "clock" && endpoint.Kind != "recording") {
			return errors.New("invalid or duplicate fake upstream endpoint")
		}
		if endpoint.Kind == "clock" {
			clockCount++
			if endpoint.ResponseStatus != 0 || endpoint.ResponseBody != "" {
				return errors.New("clock endpoint cannot declare a fixed response")
			}
		} else if endpoint.ResponseStatus < 100 || endpoint.ResponseStatus > 599 || len(endpoint.ResponseBody) > 4096 {
			return errors.New("recording endpoint response is invalid or over budget")
		}
		ports[endpoint.Port], names[endpoint.Name] = true, true
	}
	if clockCount != 1 {
		return errors.New("exactly one controlled-clock fake upstream is required")
	}
	channelSeen := map[string]bool{}
	if len(d.Channels) == 0 || len(d.Channels) > 2 {
		return errors.New("at least one supported compared channel is required")
	}
	for _, channel := range d.Channels {
		if (channel != "responses" && channel != "provider_calls") || channelSeen[channel] {
			return errors.New("unsupported or duplicate compared channel")
		}
		channelSeen[channel] = true
	}
	if !channelSeen["responses"] {
		return errors.New("responses is a required compared channel")
	}
	caseIDs := map[string]bool{}
	for _, scenarioCase := range d.Cases {
		if !definitionName.MatchString(scenarioCase.ID) || caseIDs[scenarioCase.ID] || strings.TrimSpace(scenarioCase.Title) == "" || len(scenarioCase.Title) > 128 || len(scenarioCase.Requests) < 1 || len(scenarioCase.Requests) > maxRequestsPerCase {
			return errors.New("invalid or duplicate request case")
		}
		caseIDs[scenarioCase.ID] = true
		previous := int64(-1)
		for _, request := range scenarioCase.Requests {
			if request.AfterSeconds < 0 || request.AfterSeconds > 365*24*60*60 || request.AfterSeconds < previous || !validMethod(request.Method) || !validRequestPath(request.Path) || len(request.Headers) > 16 || len(request.Body) > 4096 || strings.ContainsRune(request.Body, 0) {
				return errors.New("invalid or out-of-bound HTTP request stimulus")
			}
			previous = request.AfterSeconds
			for key, value := range request.Headers {
				if !headerName.MatchString(key) || len(value) > 1024 || strings.ContainsAny(value, "\x00\r\n") {
					return errors.New("invalid HTTP request header")
				}
			}
		}
	}
	return nil
}

func validateArgv(argv []string, optional bool) error {
	if (!optional && len(argv) == 0) || len(argv) > 64 {
		return errors.New("argv is outside bounds")
	}
	for _, arg := range argv {
		if arg == "" || len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return errors.New("argv item is empty or outside bounds")
		}
	}
	if len(argv) == 0 {
		return nil
	}
	base := path.Base(argv[0])
	if base == "sh" || base == "bash" || base == "dash" || base == "zsh" || base == "ash" || base == "busybox" {
		return errors.New("shell entrypoints are not supported; provide direct argv")
	}
	for _, arg := range argv[1:] {
		if arg == "-c" || arg == "--command" || strings.HasPrefix(arg, "-c ") {
			return errors.New("shell command strings are not supported")
		}
	}
	return nil
}

func restrictedEnvironment(key string) bool {
	return key == "PATH" || key == "HOME" || key == "TMPDIR" || key == "LD_PRELOAD" || key == "LD_LIBRARY_PATH" || strings.HasPrefix(key, "DYLD_") || strings.HasSuffix(key, "_PROXY") || key == "DOCKER_HOST" || key == "DOCKER_CONFIG"
}

func validMethod(method string) bool {
	if method == "" || len(method) > 16 {
		return false
	}
	for _, r := range method {
		if !(r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

func validRequestPath(value string) bool {
	if value == "" || len(value) > 256 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#\\\x00\r\n") {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Path == value && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.Contains(value, "..")
}

func BuiltinPaymentDefinition(repetitions int, limits sandbox.Limits) Definition {
	platform := "linux/" + runtime.GOARCH
	if platform != "linux/amd64" && platform != "linux/arm64" {
		platform = "linux/arm64"
	}
	return Definition{
		Version: 1, Kind: "http-service", Name: "synthetic-payment", Platform: platform,
		Image:     sandbox.Image,
		BuildArgv: []string{"/usr/local/go/bin/go", "build", "-trimpath", "-o", "/work/app", "./app"},
		StartArgv: []string{"/work/app", "http://127.0.0.1:18081", "http://127.0.0.1:18082"}, Environment: []string{},
		Readiness: Readiness{Protocol: readinessABI, TimeoutSeconds: 120}, Epoch: 1735689600,
		Upstreams: []Upstream{{Name: "clock", Kind: "clock", Port: 18081}, {Name: "provider", Kind: "recording", Port: 18082, ResponseStatus: 200, ResponseBody: `{"payment":"synthetic-accepted"}`}},
		Cases: []Case{
			{ID: "43200", Title: "12h same-key retry", Requests: []Request{{AfterSeconds: 0, Method: "POST", Path: "/payments", Headers: map[string]string{"Content-Type": "application/json", "Idempotency-Key": "synthetic-key-a"}, Body: `{"amount_cents":1200,"currency":"USD"}`}, {AfterSeconds: 43200, Method: "POST", Path: "/payments", Headers: map[string]string{"Content-Type": "application/json", "Idempotency-Key": "synthetic-key-a"}, Body: `{"amount_cents":1200,"currency":"USD"}`}}},
			{ID: "30", Title: "30s same-key retry", Requests: []Request{{AfterSeconds: 0, Method: "POST", Path: "/payments", Headers: map[string]string{"Content-Type": "application/json", "Idempotency-Key": "synthetic-key-a"}, Body: `{"amount_cents":1200,"currency":"USD"}`}, {AfterSeconds: 30, Method: "POST", Path: "/payments", Headers: map[string]string{"Content-Type": "application/json", "Idempotency-Key": "synthetic-key-a"}, Body: `{"amount_cents":1200,"currency":"USD"}`}}},
		},
		Channels: []string{"responses", "provider_calls"}, Repetitions: repetitions,
		Limits: DefinitionLimits{Seconds: limits.Seconds, OutputBytes: limits.OutputBytes, PreparationSeconds: preparerBudgetSeconds},
	}
}

func (d Definition) CanonicalBytes() ([]byte, error) { return json.Marshal(d) }

func (d Definition) caseMap() map[string]Case {
	out := make(map[string]Case, len(d.Cases))
	for _, scenarioCase := range d.Cases {
		out[scenarioCase.ID] = scenarioCase
	}
	return out
}

func (d Definition) routeKeys() []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, scenarioCase := range d.Cases {
		for _, req := range scenarioCase.Requests {
			key := req.Method + " " + req.Path
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys
}
