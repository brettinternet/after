package browser

import (
	"fmt"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/store"
)

// PinSuggestions returns the same finite-case expectations offered by the TUI.
// Unsupported or incomplete receipts have no suggested observed expectation.
func PinSuggestions(s *store.Store, receipt evidence.Receipt) ([]string, error) {
	if receipt.Bindings == nil || receipt.State.Kind != evidence.Observed || receipt.State.Execution != evidence.Completed || receipt.Completeness != evidence.Complete || receipt.Redacted {
		return nil, nil
	}
	scenario, err := store.Get[evidence.Scenario](s, receipt.Bindings.Scenario)
	if err != nil {
		return nil, err
	}
	definitionRaw, err := s.ReadBlob(scenario.Input)
	if err != nil {
		return nil, err
	}
	if definition, parseErr := runner.ParseCommandDefinition(definitionRaw); parseErr == nil && scenario.Author == "AFTER operator-selected command v1" {
		suggestions := make([]string, 0, len(definition.Cases))
		for _, scenarioCase := range definition.Cases {
			suggestions = append(suggestions, fmt.Sprintf("For %s, preserve the declared exit status, stdout and stderr in case %s.", definition.Name, scenarioCase.Title))
		}
		return suggestions, nil
	}
	if definition, parseErr := runner.ParseDefinition(definitionRaw); parseErr == nil && scenario.Author == "AFTER operator-selected http-service v1" {
		suggestions := make([]string, 0, len(definition.Cases))
		for _, scenarioCase := range definition.Cases {
			suggestions = append(suggestions, fmt.Sprintf("For %s, preserve the declared responses and observer channels in case %s.", definition.Name, scenarioCase.Title))
		}
		return suggestions, nil
	}
	var suggestions []string
	for _, seconds := range []int64{43200, 30} {
		observations, _, err := caseObservations(s, receipt, seconds)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, pinExpectation(seconds, observations["candidate"]))
	}
	return suggestions, nil
}

func pinExpectation(seconds int64, observations []runner.Observation) string {
	return fmt.Sprintf("At %ds, expect %s provider request(s) for the frozen two same-key requests; finite example only", seconds, countText(observationCounts(observations)))
}
