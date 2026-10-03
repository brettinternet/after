package compare

import (
	"encoding/json"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
)

func equalEmpty(o *runner.Observation, _ string, _ int) {
	o.Calls = o.Calls[:1]
	o.Calls[0].At = 0
	o.Calls[0].Key = ""
	o.Calls[0].Body = ""
	for i := range o.Responses {
		o.Responses[i].Body = ""
	}
}

func TestSampleExecutionPlanBinding(t *testing.T) {
	for _, component := range []string{"app", "observer"} {
		for _, mode := range []string{"missing", "mismatched", "matching"} {
			t.Run(component+"/"+mode, func(t *testing.T) {
				s, pair := setup(t)
				r := receipt(t, s, pair, equalEmpty, func(m *runner.Sample) {
					if m.Side != "candidate" || m.Repetition != 0 || m.CaseSeconds != 43200 {
						return
					}
					var execution *sandbox.Result = &m.Execution.App
					if component == "observer" {
						execution = &m.Execution.Observer
					}
					switch mode {
					case "missing":
						execution.Plan = ""
					case "mismatched":
						execution.Plan = string(pair.Base)
					}
				})
				c, _ := result(t, s, r)
				want := evidence.Incomparable
				if mode == "matching" {
					want = evidence.Equal
				}
				if c.Outcome != want {
					t.Fatalf("got %s, want %s", c.Outcome, want)
				}
			})
		}
	}
}

func TestObservationRequiredFields(t *testing.T) {
	for channel, fields := range map[string][]string{
		"responses":      {"status", "body"},
		"provider_calls": {"at", "method", "path", "key", "body"},
	} {
		for _, field := range fields {
			for _, mode := range []string{"omitted", "null", "wrong-type", "explicit"} {
				t.Run(channel+"/"+field+"/"+mode, func(t *testing.T) {
					s, pair := setup(t)
					r := receipt(t, s, pair, equalEmpty, nil)
					for i := range r.Artifacts {
						if r.Artifacts[i].Channel != "candidate/43200/0/observation" {
							continue
						}
						b, err := s.ReadBlob(r.Artifacts[i].Content)
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := json.Unmarshal(b, &raw); err != nil {
							t.Fatal(err)
						}
						entry := raw[channel].([]any)[0].(map[string]any)
						switch mode {
						case "omitted":
							delete(entry, field)
						case "null":
							entry[field] = nil
						case "wrong-type":
							entry[field] = true
						}
						r.Artifacts[i] = artifact(t, s, raw, r.Artifacts[i].Channel)
						b, err = s.ReadBlob(r.Artifacts[i+1].Content)
						if err != nil {
							t.Fatal(err)
						}
						var m runner.Sample
						if err := json.Unmarshal(b, &m); err != nil {
							t.Fatal(err)
						}
						m.Artifacts = []evidence.Artifact{r.Artifacts[i]}
						r.Artifacts[i+1] = artifact(t, s, m, r.Artifacts[i+1].Channel)
					}
					c, _ := result(t, s, r)
					want := evidence.Incomparable
					if mode == "explicit" {
						want = evidence.Equal
					}
					if c.Outcome != want {
						t.Fatalf("got %s, want %s", c.Outcome, want)
					}
				})
			}
		}
	}
}
