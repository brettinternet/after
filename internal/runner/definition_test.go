package runner

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

func TestParseDefinitionRejectsUntrustedCommandAndSchemaChanges(t *testing.T) {
	definition := BuiltinPaymentDefinition(1, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	raw, err := definition.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := ParseDefinition(raw); err != nil || parsed.Name != definition.Name {
		t.Fatalf("valid definition rejected: %+v %v", parsed, err)
	}

	cases := map[string]func(Definition) Definition{
		"unpinned image":   func(d Definition) Definition { d.Image = "docker.io/library/python:latest"; return d },
		"shell entrypoint": func(d Definition) Definition { d.StartArgv = []string{"/bin/sh", "-c", "python3 app.py"}; return d },
		"shell command string": func(d Definition) Definition {
			d.StartArgv = []string{"/usr/local/bin/python3", "-c", "import os; os.system('id')"}
			return d
		},
		"unsupported readiness": func(d Definition) Definition { d.Readiness.Protocol = "poll-http"; return d },
		"oversized case count":  func(d Definition) Definition { d.Cases = append(d.Cases, d.Cases[0], d.Cases[0], d.Cases[0]); return d },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed, err := mutate(definition).CanonicalBytes()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseDefinition(changed); err == nil {
				t.Fatal("unsafe definition was accepted")
			}
		})
	}
	unknown := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"future_control":true}`)...)
	if _, err := ParseDefinition(unknown); err == nil {
		t.Fatal("unknown definition field was accepted")
	}
	trailing := append(append([]byte(nil), raw...), []byte(` {}`)...)
	if _, err := ParseDefinition(trailing); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
}

func TestSelectedDefinitionIsFrozenAndChangedOracleNeverSelectsSnapshotCopy(t *testing.T) {
	s, pair := captured(t, "")
	raw, err := os.ReadFile("testdata/python-service/http-service.json")
	if err != nil {
		t.Fatal(err)
	}
	const path = "zzzz-http-service.json"
	mutateSnapshot := func(id evidence.Digest, content []byte) evidence.Digest {
		t.Helper()
		snapshot, err := store.Get[evidence.Snapshot](s, id)
		if err != nil {
			t.Fatal(err)
		}
		blob, err := s.PutArtifact(content, "source", MaxDefinitionBytes)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.ID = ""
		snapshot.Files = append(snapshot.Files, evidence.File{Path: path, Content: blob.Content, Mode: "100644"})
		stored, err := store.Put(s, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return stored.ID
	}
	pair.Base = mutateSnapshot(pair.Base, []byte(`{"snapshot":"base"}`))
	pair.Candidate = mutateSnapshot(pair.Candidate, []byte(`{"snapshot":"candidate is invalid selected configuration"`))

	plan, err := PrepareDefinition(s, pair, raw, DefinitionSource{Kind: "operator-selected-file", RepositoryPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.definitionSource.ChangedOracle || plan.definition.Name != "python-stdlib-echo" || plan.definitionDigest != hash(raw) {
		t.Fatalf("explicitly selected definition was not retained as a changed oracle: %+v", plan.definitionSource)
	}
	var preview struct {
		Definition       Definition       `json:"definition"`
		DefinitionDigest evidence.Digest  `json:"definition_digest"`
		Source           DefinitionSource `json:"definition_source"`
	}
	if err := json.Unmarshal(plan.preview, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Definition.Name != "python-stdlib-echo" || preview.DefinitionDigest != hash(raw) || !preview.Source.ChangedOracle || preview.Source.RepositoryPath != path {
		t.Fatalf("preview did not bind the selected definition and changed-oracle notice: %+v", preview)
	}
}
