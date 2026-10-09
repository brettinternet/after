package compare

import "github.com/brettinternet/after/internal/evidence"

// CaseOutcome is the legacy duration-based projection used only by the
// built-in payment browser card. Generic clients use CaseOutcomeID.
func (r Report) CaseOutcome(seconds int64, completeness evidence.Completeness) evidence.ComparisonOutcome {
	if !caseReportUsable(r, completeness) {
		return evidence.Incomparable
	}
	channels := reportChannels(r)
	outcome := evidence.Equal
	paired := map[string]bool{}
	for _, witness := range r.Witnesses {
		if witness.Before.Seconds != seconds && witness.After.Seconds != seconds {
			continue
		}
		if witness.Before.Seconds != seconds || witness.After.Seconds != seconds || !validWitness(witness, channels) {
			return evidence.Incomparable
		}
		if witness.Relation == "paired" {
			paired[witness.Channel] = true
		}
		outcome = mergeOutcome(outcome, witness)
	}
	for _, channel := range channels {
		if !paired[channel] {
			return evidence.Incomparable
		}
	}
	return outcome
}

// CaseOutcomeID narrows a complete comparison to one frozen definition case.
func (r Report) CaseOutcomeID(caseID string, completeness evidence.Completeness) evidence.ComparisonOutcome {
	if !caseReportUsable(r, completeness) || caseID == "" {
		return evidence.Incomparable
	}
	channels := reportChannels(r)
	outcome := evidence.Equal
	paired := map[string]bool{}
	for _, witness := range r.Witnesses {
		if witness.Before.CaseID != caseID && witness.After.CaseID != caseID {
			continue
		}
		if witness.Before.CaseID != caseID || witness.After.CaseID != caseID || !validWitness(witness, channels) {
			return evidence.Incomparable
		}
		if witness.Relation == "paired" {
			paired[witness.Channel] = true
		}
		outcome = mergeOutcome(outcome, witness)
	}
	for _, channel := range channels {
		if !paired[channel] {
			return evidence.Incomparable
		}
	}
	return outcome
}

func caseReportUsable(report Report, completeness evidence.Completeness) bool {
	return completeness == evidence.Complete && report.Version == 1 &&
		(report.Outcome == evidence.Equal || report.Outcome == evidence.Different || report.Outcome == evidence.Unstable)
}

func reportChannels(report Report) []string {
	if len(report.Channels) > 0 {
		return report.Channels
	}
	return []string{"responses", "provider"}
}

func validWitness(witness Witness, channels []string) bool {
	return (witness.Relation == "paired" || witness.Relation == "repetition") &&
		contains(channels, witness.Channel) &&
		(witness.Outcome == evidence.Equal || witness.Outcome == evidence.Different)
}

func mergeOutcome(current evidence.ComparisonOutcome, witness Witness) evidence.ComparisonOutcome {
	if witness.Outcome != evidence.Different {
		return current
	}
	if witness.Relation == "repetition" {
		return evidence.Unstable
	}
	if current == evidence.Equal {
		return evidence.Different
	}
	return current
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
