// Package runner executes only the frozen sequential payment ABI. It is not an
// arbitrary command runner and never loads driver/oracle code from a candidate.
package runner

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

//go:embed runtime/launch.go
var launcher []byte

//go:embed runtime/observer.go
var observer []byte

var seconds = []int64{43200, 30}

const inputs = `{"epoch":1735689600,"seconds":[43200,30],"key":"synthetic-key-a","body":{"amount_cents":1200,"currency":"USD"}}`

// ComparisonRules is immutable policy text retained with receipts. Any change
// changes the scenario digest and requires a new authorized run.
const ComparisonRules = `{"version":1,"responses":"ordered status and body; JSON bodies structural, otherwise exact text","provider":"ordered timestamp, method, fixed observer destination 127.0.0.1:18082 plus path, key and body; count is log length","json":"exact decimal values, object key order ignored, arrays ordered, null distinct from missing; reject duplicate keys and invalid Unicode","masks":[],"normalization":[],"scope":"finite sequential synthetic requests only"}`

const rules = ComparisonRules

var scope = []string{"Sequential synthetic payment ABI only; two same-key requests at 12h and 30s; finite observation window, not production billing or universal behavior.", "Candidate suites are not run; candidate test/driver/mask files remain source inventory, not the frozen oracle.", "Docker daemon, CLI, image and kernel trusted; app and observer share only offline loopback networking; denial of service fails the run."}

type Plan struct {
	request      evidence.Digest
	pair         evidence.SnapshotPair
	scenario     evidence.Scenario
	repetitions  int
	limits       sandbox.Limits
	experiments  [2][2]*sandbox.Experiment
	environments [2]evidence.Environment
	planIDs      [2][2][2]string // side, case, app/observer
	preview      []byte
	id           string
}

func hash(b []byte) evidence.Digest {
	return evidence.Digest(fmt.Sprintf("sha256:%x", sha256.Sum256(b)))
}

func validRequestID(id evidence.Digest) bool {
	s := string(id)
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil
}
func (p *Plan) Preview() ([]byte, string)  { return append([]byte(nil), p.preview...), p.id }
func (p *Plan) RequestID() evidence.Digest { return p.request }

// Prepare reads validated immutable store records; it neither contacts Docker nor
// builds anything. Every future execution, including builds, is in the preview.
func Prepare(s *store.Store, pair evidence.SnapshotPair, repetitions int, limits sandbox.Limits) (*Plan, error) {
	return prepare(s, pair, repetitions, limits, "")
}

func prepare(s *store.Store, pair evidence.SnapshotPair, repetitions int, limits sandbox.Limits, request evidence.Digest) (*Plan, error) {
	if pair.Base == "" || repetitions < 1 || repetitions > 5 {
		return nil, errors.New("base and 1-5 repetitions required")
	}
	if request != "" && !validRequestID(request) {
		return nil, errors.New("invalid saved request identity")
	}
	input, err := s.PutArtifact([]byte(inputs), "frozen-input", 4096)
	if err != nil {
		return nil, err
	}
	if input.Completeness != evidence.Complete {
		return nil, errors.New("frozen input was redacted")
	}
	scenario, err := store.Put(s, evidence.Scenario{SchemaVersion: 1, Input: input.Content, Driver: hash(append(append([]byte(nil), launcher...), observer...)), Observer: hash(observer), Rules: hash([]byte(rules)), Boundary: "payment HTTP responses and observer-received provider traffic", Author: "AFTER frozen payment driver", Limits: scope})
	if err != nil {
		return nil, err
	}
	if request == "" {
		nonce := make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return nil, err
		}
		request = hash(nonce)
	}
	p := &Plan{request: request, pair: pair, scenario: scenario, repetitions: repetitions, limits: limits}
	var previews [2][2]json.RawMessage
	for side, id := range []evidence.Digest{pair.Base, pair.Candidate} {
		snap, e := store.Get[evidence.Snapshot](s, id)
		if e != nil {
			return nil, e
		}
		if snap.Completeness != evidence.Complete {
			return nil, errors.New("incomplete snapshot cannot execute")
		}
		if len(snap.Files) > 255 {
			return nil, errors.New("snapshot exceeds sandbox file budget")
		}
		files := map[string][]byte{}
		total := len(launcher)
		for _, f := range snap.Files {
			if f.Path == "after-launch.go" {
				return nil, errors.New("reserved launcher path in snapshot")
			}
			data, e := s.ReadBlob(f.Content)
			if e != nil {
				return nil, e
			}
			total += len(data)
			if total > 8<<20 {
				return nil, errors.New("snapshot exceeds sandbox input budget")
			}
			files[f.Path] = data
		}
		if len(files["go.mod"]) == 0 || len(files["app/main.go"]) == 0 {
			return nil, errors.New("unsupported payment driver: go.mod and app/main.go required")
		}
		files["after-launch.go"] = launcher
		argv := []string{"/usr/local/go/bin/go", "run", "/input/after-launch.go"}
		app, e := sandbox.Prepare(string(id), files, argv, limits)
		if e != nil {
			return nil, e
		}
		appPreview, appID := app.Preview()
		p.environments[side] = evidence.Environment{Environment: hash(appPreview), Toolchain: evidence.Digest(strings.Split(sandbox.Image, "@")[1]), Dependencies: id, Argv: argv}
		for c, sec := range seconds {
			obs, e := sandbox.Prepare(string(scenario.ID), map[string][]byte{"observer.go": observer}, []string{"/usr/local/go/bin/go", "run", "/input/observer.go", fmt.Sprint(sec)}, limits)
			if e != nil {
				return nil, e
			}
			_, observerID := obs.Preview()
			p.planIDs[side][c] = [2]string{appID, observerID}
			p.experiments[side][c], e = sandbox.PrepareExperiment(app, obs)
			if e != nil {
				return nil, e
			}
			previews[side][c], _ = p.experiments[side][c].Preview()
		}
	}
	p.preview, err = json.MarshalIndent(struct {
		Version        int                   `json:"version"`
		Request        evidence.Digest       `json:"request"`
		Snapshots      evidence.SnapshotPair `json:"snapshots"`
		Scenario       evidence.Scenario     `json:"scenario"`
		Repetitions    int                   `json:"repetitions"`
		Limits         sandbox.Limits        `json:"limits"`
		Concurrency    int                   `json:"concurrency"`
		Experiments    [2][2]json.RawMessage `json:"experiments"`
		BuildArgv      []string              `json:"build_argv"`
		AppArgv        []string              `json:"app_argv"`
		AppEnvironment []string              `json:"app_environment"`
	}{1, p.request, p.pair, p.scenario, repetitions, limits, 1, previews,
		[]string{"/usr/local/go/bin/go", "build", "-trimpath", "-o", "/work/app", "./app"},
		[]string{"/work/app", "http://127.0.0.1:18081", "http://127.0.0.1:18082"}, []string{"GOMAXPROCS=2"}}, "", "  ")
	p.id = string(hash(p.preview))
	return p, err
}

// PrepareFromPreview reconstructs a previously saved preview from its frozen
// request identity and bounds. The rebuilt bytes must match exactly before the
// returned plan can be approved; caller-supplied text never becomes a plan.
func PrepareFromPreview(s *store.Store, preview []byte) (*Plan, error) {
	var saved struct {
		Version     int                   `json:"version"`
		Request     evidence.Digest       `json:"request"`
		Snapshots   evidence.SnapshotPair `json:"snapshots"`
		Scenario    evidence.Scenario     `json:"scenario"`
		Repetitions int                   `json:"repetitions"`
		Limits      sandbox.Limits        `json:"limits"`
		Concurrency int                   `json:"concurrency"`
		Experiments [2][2]json.RawMessage `json:"experiments"`
		BuildArgv   []string              `json:"build_argv"`
		AppArgv     []string              `json:"app_argv"`
		Environment []string              `json:"app_environment"`
	}
	if len(preview) == 0 || len(preview) > 1<<20 {
		return nil, errors.New("saved execution plan exceeds bounds")
	}
	decoder := json.NewDecoder(bytes.NewReader(preview))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid saved execution plan")
	}
	if saved.Version != 1 || saved.Request == "" || saved.Snapshots.Base == "" || saved.Snapshots.Candidate == "" || saved.Repetitions < 1 || saved.Repetitions > 5 || saved.Limits.Seconds < 1 || saved.Limits.Seconds > 300 || saved.Limits.OutputBytes < 1 || saved.Limits.OutputBytes > 1<<20 || saved.Concurrency != 1 {
		return nil, errors.New("invalid saved execution plan bindings")
	}
	p, err := prepare(s, saved.Snapshots, saved.Repetitions, saved.Limits, saved.Request)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(p.preview, preview) {
		return nil, errors.New("saved execution plan no longer matches stored inputs")
	}
	return p, nil
}
