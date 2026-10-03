package evidence

import "errors"

// Comparison stores a receipt-bound result, not a comparator implementation.
// Detailed channel witnesses are added by the comparison task.
type Comparison struct {
	SchemaVersion int               `json:"schema_version"`
	ID            Digest            `json:"id"`
	Receipt       Digest            `json:"receipt"`
	Outcome       ComparisonOutcome `json:"outcome"`
	Completeness  Completeness      `json:"completeness"`
	Limits        []string          `json:"limits"`
}

func (c Comparison) Validate() error {
	if err := header(c.SchemaVersion, c.ID); err != nil {
		return err
	}
	if !digest(c.Receipt) || !oneOf(c.Outcome, NotCompared, Equal, Different, Incomparable, Unstable) || !oneOf(c.Completeness, Complete, Incomplete) || len(c.Limits) == 0 {
		return errors.New("invalid comparison bindings, outcome or scope")
	}
	if c.Completeness != Complete && oneOf(c.Outcome, Equal, Different, Unstable) {
		return errors.New("partial comparison cannot be conclusive")
	}
	return nil
}
