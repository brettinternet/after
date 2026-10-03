// Package runner executes only the frozen sequential payment ABI. It is not an
// arbitrary command runner and never loads driver/oracle code from a candidate.
package runner

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
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
const rules = "v1: exact response status/body and ordered provider traffic; no candidate masks; repetitions retained; no cross-version comparison in runner"

var scope = []string{"Sequential synthetic payment ABI only; two same-key requests at 12h and 30s; finite observation window, not production billing or universal behavior.", "Candidate suites are not run; candidate test/driver/mask files remain source inventory, not the frozen oracle.", "Docker daemon, CLI, image and kernel trusted; app and observer share only offline loopback networking; denial of service fails the run."}

type Plan struct {
	request      evidence.Digest
	pair         evidence.SnapshotPair
	scenario     evidence.Scenario
	repetitions  int
	limits       sandbox.Limits
	experiments  [2][2]*sandbox.Experiment
	environments [2]evidence.Environment
	preview      []byte
	id           string
}

func hash(b []byte) evidence.Digest {
	return evidence.Digest(fmt.Sprintf("sha256:%x", sha256.Sum256(b)))
}
func (p *Plan) Preview() ([]byte, string)  { return append([]byte(nil), p.preview...), p.id }
func (p *Plan) RequestID() evidence.Digest { return p.request }

// Prepare reads validated immutable store records; it neither contacts Docker nor
// builds anything. Every future execution, including builds, is in the preview.
func Prepare(s *store.Store, pair evidence.SnapshotPair, repetitions int, limits sandbox.Limits) (*Plan, error) {
	if pair.Base == "" || repetitions < 1 || repetitions > 5 {
		return nil, errors.New("base and 1-5 repetitions required")
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
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	p := &Plan{request: hash(nonce), pair: pair, scenario: scenario, repetitions: repetitions, limits: limits}
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
		appPreview, _ := app.Preview()
		p.environments[side] = evidence.Environment{Environment: hash(appPreview), Toolchain: evidence.Digest(strings.Split(sandbox.Image, "@")[1]), Dependencies: id, Argv: argv}
		for c, sec := range seconds {
			obs, e := sandbox.Prepare(string(scenario.ID), map[string][]byte{"observer.go": observer}, []string{"/usr/local/go/bin/go", "run", "/input/observer.go", fmt.Sprint(sec)}, limits)
			if e != nil {
				return nil, e
			}
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
		Concurrency    int                   `json:"concurrency"`
		Experiments    [2][2]json.RawMessage `json:"experiments"`
		BuildArgv      []string              `json:"build_argv"`
		AppArgv        []string              `json:"app_argv"`
		AppEnvironment []string              `json:"app_environment"`
	}{1, p.request, p.pair, p.scenario, repetitions, 1, previews,
		[]string{"/usr/local/go/bin/go", "build", "-trimpath", "-o", "/work/app", "./app"},
		[]string{"/work/app", "http://127.0.0.1:18081", "http://127.0.0.1:18082"}, []string{"GOMAXPROCS=2"}}, "", "  ")
	p.id = string(hash(p.preview))
	return p, err
}
