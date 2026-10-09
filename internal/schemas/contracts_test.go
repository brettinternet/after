package schemas_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaDir = "../../schemas/v1"

type registry map[string]*jsonschema.Schema

func loadSchemas(t *testing.T) registry {
	t.Helper()
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		t.Fatal(err)
	}
	compiled := registry{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(schemaDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var document any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatalf("%s is not JSON: %v", entry.Name(), err)
		}
		id := "https://after.invalid/schemas/v1/" + entry.Name()
		compiler := jsonschema.NewCompiler()
		compiler.DefaultDraft(jsonschema.Draft2020)
		compiler.AssertFormat()
		if err := compiler.AddResource(id, document); err != nil {
			t.Fatalf("add schema %s: %v", entry.Name(), err)
		}
		schema, err := compiler.Compile(id)
		if err != nil {
			t.Fatalf("compile schema %s: %v", entry.Name(), err)
		}
		compiled[entry.Name()] = schema
	}
	if len(compiled) == 0 {
		t.Fatal("no versioned schemas were compiled")
	}
	return compiled
}

func validate(t *testing.T, schemas registry, name string, raw []byte) {
	t.Helper()
	schema, ok := schemas[name]
	if !ok {
		t.Fatalf("missing schema %s", name)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode value for %s: %v", name, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("value for %s has trailing JSON: %v", name, err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("value does not match %s: %v", name, err)
	}
}

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func digest(letter byte) evidence.Digest {
	return evidence.Digest("sha256:" + strings.Repeat(string(letter), 64))
}

func TestBuiltInContractsValidateAgainstVersionedSchemas(t *testing.T) {
	schemas := loadSchemas(t)

	payment := runner.BuiltinPaymentDefinition(2, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	paymentBytes, err := payment.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ParseDefinition(paymentBytes); err != nil {
		t.Fatalf("built-in payment strict decoder: %v", err)
	}
	validate(t, schemas, "http-service-definition-v1.schema.json", paymentBytes)

	pythonBytes, err := os.ReadFile("../runner/testdata/python-service/http-service.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ParseDefinition(pythonBytes); err != nil {
		t.Fatalf("Python service strict decoder: %v", err)
	}
	validate(t, schemas, "http-service-definition-v1.schema.json", pythonBytes)

	commandBytes, err := os.ReadFile("../runner/testdata/command-proof.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ParseCommandDefinition(commandBytes); err != nil {
		t.Fatalf("command proof strict decoder: %v", err)
	}
	validate(t, schemas, "command-definition-v1.schema.json", commandBytes)

	for _, definition := range []runner.Definition{payment, mustParseDefinition(t, pythonBytes)} {
		for _, scenarioCase := range definition.Cases {
			responses := make([]runner.Response, len(scenarioCase.Requests))
			for index := range responses {
				responses[index] = runner.Response{Status: 200, Body: `{"result":"synthetic"}`}
			}
			observation := runner.Observation{
				Version: 1, CaseID: scenarioCase.ID, Seconds: runner.CaseDuration(scenarioCase),
				Responses: responses, Calls: []runner.Call{},
			}
			validate(t, schemas, "http-observation-v1.schema.json", marshal(t, observation))
		}
	}

	started := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	commandSample := runner.Sample{
		RequestID: digest('a'), Snapshots: evidence.SnapshotPair{Base: digest('b'), Candidate: digest('c')},
		Side: "base", CaseID: "changed", Repetition: 0, StartedAt: started, FinishedAt: started.Add(time.Second), Status: "completed",
		Execution: sandbox.ExperimentResult{App: sandbox.Result{Plan: "synthetic-plan", Container: "synthetic-container", InputImage: "sha256:" + strings.Repeat("d", 64), Truncated: false, ExitCode: 0, OOMKilled: false, Cleaned: true}},
		Artifacts: []evidence.Artifact{
			{Content: digest('e'), Channel: "base/changed/0/stdout", Bytes: 7, MaxBytes: 65536, Completeness: evidence.Complete, Redacted: false, Truncated: false},
			{Content: digest('f'), Channel: "base/changed/0/stderr", Bytes: 0, MaxBytes: 65536, Completeness: evidence.Complete, Redacted: false, Truncated: false},
		},
	}
	validate(t, schemas, "command-observation-v1.schema.json", marshal(t, commandSample))

	validate(t, schemas, "http-comparison-rules-v1.schema.json", []byte(runner.ComparisonRules))
	validate(t, schemas, "command-comparison-rules-v1.schema.json", []byte(runner.CommandComparisonRules))

	importedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	metadata := gotestreport.Metadata{Producer: "synthetic schema check", Snapshot: digest('1'), ImportedAt: importedAt}
	goReportBytes, err := os.ReadFile("../gotestreport/testdata/tests.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	goReport, err := gotestreport.Import(bytes.NewReader(goReportBytes), metadata)
	if err != nil {
		t.Fatal(err)
	}
	if goReport.Dialect != gotestreport.Dialect {
		t.Fatalf("unexpected Go report dialect: %s", goReport.Dialect)
	}
	validate(t, schemas, "report-cards-v1.schema.json", marshal(t, goReport))

	junitBytes, err := os.ReadFile("../gotestreport/testdata/junit/pytest.xml")
	if err != nil {
		t.Fatal(err)
	}
	junitReport, err := gotestreport.ImportJUnit(bytes.NewReader(junitBytes), metadata)
	if err != nil {
		t.Fatal(err)
	}
	if junitReport.Dialect != gotestreport.JUnitDialect {
		t.Fatalf("unexpected JUnit report dialect: %s", junitReport.Dialect)
	}
	validate(t, schemas, "report-cards-v1.schema.json", marshal(t, junitReport))
}

func mustParseDefinition(t *testing.T, raw []byte) runner.Definition {
	t.Helper()
	definition, err := runner.ParseDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func TestSchemasRejectStrictDecoderDriftAndMutableRules(t *testing.T) {
	schemas := loadSchemas(t)

	httpBytes, err := os.ReadFile("../runner/testdata/python-service/http-service.json")
	if err != nil {
		t.Fatal(err)
	}
	unknownHTTP := withUnknownField(httpBytes)
	if _, err := runner.ParseDefinition(unknownHTTP); err == nil {
		t.Fatal("HTTP strict decoder accepted an unknown field")
	}
	if err := schemaValidationError(schemas, "http-service-definition-v1.schema.json", unknownHTTP); err == nil {
		t.Fatal("HTTP schema accepted a field rejected by the strict decoder")
	}
	unpinnedHTTP := bytes.Replace(httpBytes, []byte("@sha256:cf97c3b79da1c706532c7042f851654d97d3b88c8ca6520acda9cf4479ea9b14"), []byte(":latest"), 1)
	if _, err := runner.ParseDefinition(unpinnedHTTP); err == nil {
		t.Fatal("HTTP strict decoder accepted an unpinned image")
	}
	if err := schemaValidationError(schemas, "http-service-definition-v1.schema.json", unpinnedHTTP); err == nil {
		t.Fatal("HTTP schema accepted an unpinned image")
	}

	commandBytes, err := os.ReadFile("../runner/testdata/command-proof.json")
	if err != nil {
		t.Fatal(err)
	}
	unknownCommand := withUnknownField(commandBytes)
	if _, err := runner.ParseCommandDefinition(unknownCommand); err == nil {
		t.Fatal("command strict decoder accepted an unknown field")
	}
	if err := schemaValidationError(schemas, "command-definition-v1.schema.json", unknownCommand); err == nil {
		t.Fatal("command schema accepted a field rejected by the strict decoder")
	}

	changedRules := bytes.Replace([]byte(runner.CommandComparisonRules), []byte(`"masks":[]`), []byte(`"masks":["/secret"]`), 1)
	if string(changedRules) == runner.CommandComparisonRules {
		t.Fatal("test did not alter the fixed command rules")
	}
	if err := schemaValidationError(schemas, "command-comparison-rules-v1.schema.json", changedRules); err == nil {
		t.Fatal("command rules schema accepted a mutable mask")
	}
}

func withUnknownField(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	object := append([]byte(nil), trimmed[:len(trimmed)-1]...)
	return append(object, []byte(`,"future_execution_control":true}`)...)
}

func schemaValidationError(schemas registry, name string, raw []byte) error {
	schema, ok := schemas[name]
	if !ok {
		return fmt.Errorf("missing schema %s", name)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	return schema.Validate(value)
}
