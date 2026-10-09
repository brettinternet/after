package compare

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

type commandSample struct {
	ref       SampleRef
	status    int
	stdout    []byte
	stderr    []byte
	stdoutRef evidence.Digest
	stderrRef evidence.Digest
}

func compareCommandReceipt(s *store.Store, receipt evidence.Receipt, report *Report, basis runner.ComparisonBasis) error {
	if basis.Command == nil {
		return errors.New("missing frozen command definition")
	}
	report.Limits = []string{
		"Only the finite explicitly declared argv cases on this captured pair; no universal correctness, causation or performance claim.",
		"Exit status is the Docker-inspected container status; statuses 125–255 are incomplete. Text compares exact bytes (including invalid UTF-8); declared JSON is structural and invalid JSON is incomparable.",
		"Build output is discarded; helper diagnostics appear only on incomplete runs and cannot become target output. No TTY, output tree, observer container, external network, image pull or host fallback.",
	}
	report.DefinitionName = basis.DefinitionName
	report.Channels = append([]string(nil), basis.Channels...)
	for _, scenarioCase := range basis.CommandCases {
		report.Cases = append(report.Cases, scenarioCase.ID)
		report.CaseTitles = append(report.CaseTitles, scenarioCase.Title)
	}
	artifacts := map[string]evidence.Artifact{}
	for _, artifact := range receipt.Artifacts {
		if _, duplicate := artifacts[artifact.Channel]; duplicate {
			return errors.New("duplicate artifact channel")
		}
		artifacts[artifact.Channel] = artifact
	}
	policy, ok := artifacts["comparison-rules"]
	if !ok || policy.Content != receipt.Bindings.Rules || policy.Completeness != evidence.Complete || policy.Redacted || policy.Truncated {
		return errors.New("missing, changed, or incomplete command comparison policy")
	}
	policyBytes, err := s.ReadBlob(policy.Content)
	if err != nil || string(policyBytes) != runner.CommandComparisonRules {
		return errors.New("unsupported command comparison policy")
	}
	allowed := map[string]bool{"comparison-rules": true, "execution-plan": true, "preparation-result": true, "preparation-diagnostics": true, "launcher-executable": true}
	samples := map[string]commandSample{}
	for repetition := 0; repetition < basis.Repetitions; repetition++ {
		for _, side := range []string{"base", "candidate"} {
			for _, scenarioCase := range basis.CommandCases {
				prefix := fmt.Sprintf("%s/%s/%d/", side, scenarioCase.ID, repetition)
				for _, suffix := range []string{"sample", "stdout", "stderr"} {
					allowed[prefix+suffix] = true
				}
				meta, ok := artifacts[prefix+"sample"]
				stdout, stdoutOK := artifacts[prefix+"stdout"]
				stderr, stderrOK := artifacts[prefix+"stderr"]
				if !ok || !stdoutOK || !stderrOK {
					return errors.New("missing command sample metadata or output stream")
				}
				for _, artifact := range []evidence.Artifact{meta, stdout, stderr} {
					if artifact.Completeness != evidence.Complete || artifact.Redacted || artifact.Truncated {
						return errors.New("redacted, truncated, or incomplete command channel")
					}
				}
				metadata, err := s.ReadBlob(meta.Content)
				if err != nil {
					return err
				}
				var sample runner.Sample
				if !commandExitStatusPresent(metadata) || strict(metadata, &sample) != nil {
					return errors.New("invalid command sample metadata")
				}
				if sample.RequestID != receipt.RequestID || sample.Snapshots != receipt.Snapshots || sample.Side != side || sample.CaseID != scenarioCase.ID || sample.CaseSeconds != 0 || sample.Repetition != repetition || sample.Status != "completed" || sample.StartedAt.Before(receipt.StartedAt) || sample.FinishedAt.After(receipt.FinishedAt) || sample.FinishedAt.Before(sample.StartedAt) || !sample.Execution.App.Cleaned || sample.Execution.App.Container == "" || sample.Execution.App.Truncated || sample.Execution.App.OOMKilled || sample.Execution.App.ExitCode < 0 || sample.Execution.App.ExitCode >= 125 || sample.Execution.App.Stdout != "" || sample.Execution.App.Stderr != "" || sample.Execution.App.Output != "" || sample.Execution.Observer != (sandbox.Result{}) {
					return errors.New("incompatible, incomplete, or misbound command sample")
				}
				if !basis.MatchesSample(sample) {
					return errors.New("command sample execution plan mismatch")
				}
				stdoutBytes, err := s.ReadBlob(stdout.Content)
				if err != nil {
					return err
				}
				stderrBytes, err := s.ReadBlob(stderr.Content)
				if err != nil {
					return err
				}
				if int64(len(stdoutBytes)) != stdout.Bytes || int64(len(stderrBytes)) != stderr.Bytes || len(stdoutBytes)+len(stderrBytes) > basis.Command.Limits.OutputBytes {
					return errors.New("command stream size exceeds the frozen output limit")
				}
				seen := map[string]bool{}
				for _, artifact := range sample.Artifacts {
					stored, ok := artifacts[artifact.Channel]
					if !ok || stored != artifact || seen[artifact.Channel] || !strings.HasPrefix(artifact.Channel, prefix) {
						return errors.New("command sample artifact binding mismatch")
					}
					seen[artifact.Channel] = true
				}
				if len(seen) != 2 || !seen[stdout.Channel] || !seen[stderr.Channel] {
					return errors.New("command stdout/stderr are not both bound to the sample")
				}
				samples[fmt.Sprintf("%s/%s/%d", side, scenarioCase.ID, repetition)] = commandSample{
					ref:    SampleRef{Side: side, CaseID: scenarioCase.ID, Repetition: repetition, Metadata: meta.Content},
					status: sample.Execution.App.ExitCode, stdout: stdoutBytes, stderr: stderrBytes, stdoutRef: stdout.Content, stderrRef: stderr.Content,
				}
			}
		}
	}
	for channel := range artifacts {
		if !allowed[channel] {
			return errors.New("new or unsupported command artifact channel")
		}
	}
	report.Outcome = evidence.Equal
	changesTotal := 0
	compare := func(before, after commandSample, relation, channel string) error {
		var a, b any
		var beforeRef, afterRef evidence.Digest
		switch channel {
		case "exit_status":
			a, b = json.Number(strconv.Itoa(before.status)), json.Number(strconv.Itoa(after.status))
			beforeRef, afterRef = before.ref.Metadata, after.ref.Metadata
		case "stdout":
			beforeRef, afterRef = before.stdoutRef, after.stdoutRef
			if basis.Command.Comparison.Stdout == "json" {
				var err error
				a, err = parse(before.stdout)
				if err != nil {
					return errors.New("declared stdout JSON is invalid or over budget")
				}
				b, err = parse(after.stdout)
				if err != nil {
					return errors.New("declared stdout JSON is invalid or over budget")
				}
			} else {
				changes := exactBytes(before.stdout, after.stdout)
				return appendCommandWitness(report, &changesTotal, before, after, beforeRef, afterRef, relation, channel, changes)
			}
		case "stderr":
			beforeRef, afterRef = before.stderrRef, after.stderrRef
			if basis.Command.Comparison.Stderr == "json" {
				var err error
				a, err = parse(before.stderr)
				if err != nil {
					return errors.New("declared stderr JSON is invalid or over budget")
				}
				b, err = parse(after.stderr)
				if err != nil {
					return errors.New("declared stderr JSON is invalid or over budget")
				}
			} else {
				changes := exactBytes(before.stderr, after.stderr)
				return appendCommandWitness(report, &changesTotal, before, after, beforeRef, afterRef, relation, channel, changes)
			}
		default:
			return errors.New("unsupported command comparison channel")
		}
		changes := []Change{}
		if err := walk(a, b, "", &changes); err != nil {
			return err
		}
		return appendCommandWitness(report, &changesTotal, before, after, beforeRef, afterRef, relation, channel, changes)
	}
	for _, scenarioCase := range basis.CommandCases {
		for repetition := 0; repetition < basis.Repetitions; repetition++ {
			base := samples[fmt.Sprintf("base/%s/%d", scenarioCase.ID, repetition)]
			candidate := samples[fmt.Sprintf("candidate/%s/%d", scenarioCase.ID, repetition)]
			for _, channel := range basis.Channels {
				if err := compare(base, candidate, "paired", channel); err != nil {
					return err
				}
			}
			if repetition > 0 {
				for _, side := range []string{"base", "candidate"} {
					first := samples[fmt.Sprintf("%s/%s/0", side, scenarioCase.ID)]
					current := samples[fmt.Sprintf("%s/%s/%d", side, scenarioCase.ID, repetition)]
					for _, channel := range basis.Channels {
						if err := compare(first, current, "repetition", channel); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func commandExitStatusPresent(raw []byte) bool {
	value, err := parse(raw)
	if err != nil {
		return false
	}
	root, ok := value.(map[string]any)
	if !ok {
		return false
	}
	execution, ok := root["execution"].(map[string]any)
	if !ok {
		return false
	}
	app, ok := execution["app"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = app["exit_code"].(json.Number)
	return ok
}

func exactBytes(before, after []byte) []Change {
	if bytes.Equal(before, after) {
		return []Change{}
	}
	encode := func(value []byte) json.RawMessage {
		raw, _ := json.Marshal(struct {
			Base64 string `json:"base64"`
		}{base64.StdEncoding.EncodeToString(value)})
		return raw
	}
	return []Change{{Path: "/base64", Kind: "changed", Before: encode(before), After: encode(after)}}
}

func appendCommandWitness(report *Report, total *int, before, after commandSample, beforeRef, afterRef evidence.Digest, relation, channel string, changes []Change) error {
	*total += len(changes)
	if *total > maxChanges {
		return errors.New("comparison witness budget exceeded")
	}
	outcome := evidence.Equal
	if len(changes) > 0 {
		outcome = evidence.Different
		if relation == "repetition" {
			report.Outcome = evidence.Unstable
		} else if report.Outcome == evidence.Equal {
			report.Outcome = evidence.Different
		}
	}
	before.ref.Observation = beforeRef
	after.ref.Observation = afterRef
	report.Witnesses = append(report.Witnesses, Witness{Relation: relation, Channel: channel, Before: before.ref, After: after.ref, Outcome: outcome, Changes: changes})
	return nil
}
