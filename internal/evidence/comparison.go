package evidence

import "errors"

// Comparison stores a receipt-bound result. Details points to bounded exact
// witnesses; older records without details remain readable.
type Comparison struct {
	SchemaVersion int               `json:"schema_version"`
	ID            Digest            `json:"id"`
	Receipt       Digest            `json:"receipt"`
	Outcome       ComparisonOutcome `json:"outcome"`
	Completeness  Completeness      `json:"completeness"`
	Limits        []string          `json:"limits"`
	Details       *Artifact         `json:"details,omitempty"`
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
	if c.Details != nil {
		a := c.Details
		if !digest(a.Content) || a.Channel != "comparison-details-v1" || a.Bytes < 0 || a.MaxBytes <= 0 || a.Bytes > a.MaxBytes || !oneOf(a.Completeness, Complete, Incomplete) {
			return errors.New("invalid comparison details")
		}
		if c.Completeness == Complete && (a.Completeness != Complete || a.Redacted || a.Truncated) {
			return errors.New("partial details cannot be conclusive")
		}
	}
	return nil
}
