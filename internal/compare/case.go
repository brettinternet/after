package compare

import "github.com/brettinternet/after/internal/evidence"

// CaseOutcome narrows a complete comparison to one frozen case. A receipt-wide
// difference (or instability in another case) does not transfer to this case.
// Missing witnesses and incomplete comparisons never become equality.
func (r Report) CaseOutcome(seconds int64, completeness evidence.Completeness) evidence.ComparisonOutcome {
	if completeness != evidence.Complete || r.Version != 1 ||
		(r.Outcome != evidence.Equal && r.Outcome != evidence.Different && r.Outcome != evidence.Unstable) {
		return evidence.Incomparable
	}
	outcome := evidence.Equal
	paired := map[string]bool{}
	for _, w := range r.Witnesses {
		if w.Before.Seconds != seconds && w.After.Seconds != seconds {
			continue
		}
		if w.Before.Seconds != seconds || w.After.Seconds != seconds ||
			(w.Relation != "paired" && w.Relation != "repetition") ||
			(w.Channel != "provider" && w.Channel != "responses") ||
			(w.Outcome != evidence.Equal && w.Outcome != evidence.Different) {
			return evidence.Incomparable
		}
		if w.Relation == "paired" {
			paired[w.Channel] = true
		}
		if w.Outcome == evidence.Different {
			if w.Relation == "repetition" {
				outcome = evidence.Unstable
			} else if outcome == evidence.Equal {
				outcome = evidence.Different
			}
		}
	}
	if !paired["responses"] || !paired["provider"] {
		return evidence.Incomparable
	}
	return outcome
}
