package compare

import (
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
)

func TestCaseOutcome(t *testing.T) {
	s, pair := setup(t)
	c, report := result(t, s, receipt(t, s, pair, nil, nil))
	if report.CaseOutcome(43200, c.Completeness) != evidence.Different || report.CaseOutcome(30, c.Completeness) != evidence.Equal {
		t.Fatal("receipt outcome leaked across cases", report)
	}
	c, report = result(t, s, receipt(t, s, pair, func(o *runner.Observation, side string, rep int) {
		if o.Seconds == 30 && side == "candidate" && rep == 1 {
			o.Calls = append(o.Calls, o.Calls[0])
		}
	}, nil))
	if report.CaseOutcome(30, c.Completeness) != evidence.Unstable || report.CaseOutcome(43200, c.Completeness) != evidence.Different {
		t.Fatal("repetition outcome leaked across cases")
	}
	if report.CaseOutcome(30, evidence.Incomplete) != evidence.Incomparable || report.CaseOutcome(90, evidence.Complete) != evidence.Incomparable {
		t.Fatal("incomplete or absent case became conclusive")
	}
	report.Outcome = evidence.Incomparable
	if report.CaseOutcome(30, evidence.Complete) != evidence.Incomparable {
		t.Fatal("incomparable promoted")
	}
	report.Outcome = evidence.Equal
	report.Witnesses = nil
	if report.CaseOutcome(30, evidence.Complete) != evidence.Incomparable {
		t.Fatal("missing witnesses equal")
	}
}
