package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/brettinternet/after/internal/evidence"
)

type Record interface {
	evidence.Snapshot | evidence.Capture | evidence.Scenario | evidence.Receipt | evidence.Comparison | evidence.Pin
	Validate() error
}

func identity[T Record](r *T) (string, *evidence.Digest) {
	switch v := any(r).(type) {
	case *evidence.Snapshot:
		return "snapshot", &v.ID
	case *evidence.Capture:
		return "capture", &v.ID
	case *evidence.Scenario:
		return "scenario", &v.ID
	case *evidence.Receipt:
		return "receipt", &v.ID
	case *evidence.Comparison:
		return "comparison", &v.ID
	case *evidence.Pin:
		return "pin", &v.ID
	}
	panic("unreachable record type")
}

// Put assigns a content identity when ID is empty; a supplied ID must match.
// All versions, including pin history revisions, are immutable objects.
func Put[T Record](s *Store, record T) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero T
	if s.lock == nil {
		return zero, ErrReadOnly
	}
	kind, id := identity(&record)
	requested := *id
	*id = ""
	raw, err := json.Marshal(record)
	if err != nil {
		return zero, errors.New("cannot encode record")
	}
	if len(raw) > evidence.MaxRecordBytes {
		return zero, ErrLimit
	}
	// Decode JSON strings before redacting, so escaped control characters and
	// quoting cannot hide a configured secret. Never print raw values on failure.
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, errors.New("cannot sanitize record")
	}
	changed := s.sanitize(value)
	safe, err := json.Marshal(value)
	if err != nil {
		return zero, errors.New("cannot sanitize record")
	}
	record = zero
	if err := json.Unmarshal(safe, &record); err != nil {
		return zero, errors.New("redaction changed a structural field")
	}
	if changed {
		switch r := any(&record).(type) {
		case *evidence.Receipt:
			r.Redacted = true
			r.RedactionPolicy = redactionPolicy
			r.Completeness = evidence.Incomplete
		case *evidence.Snapshot:
			r.Completeness = evidence.Incomplete
			r.Limits = append(r.Limits, "literal-v1 redaction")
		case *evidence.Scenario:
			r.Limits = append(r.Limits, "literal-v1 redaction")
		case *evidence.Comparison:
			r.Completeness = evidence.Incomplete
			r.Limits = append(r.Limits, "literal-v1 redaction")
		}
	}
	_, id = identity(&record)
	*id = ""
	raw, err = json.Marshal(record)
	if err != nil {
		return zero, errors.New("cannot encode record")
	}
	*id = evidence.Digest(hash(raw))
	if requested != "" && requested != *id {
		return zero, errors.New("record identity does not match content")
	}
	if err := record.Validate(); err != nil {
		return zero, err
	}
	if err := references(s, record); err != nil {
		return zero, err
	}
	raw, err = json.Marshal(record)
	if err != nil {
		return zero, errors.New("cannot encode record")
	}
	if len(raw) > evidence.MaxRecordBytes {
		return zero, ErrLimit
	}
	name, _ := key(kind, string(*id))
	if err := s.publish(name, raw); err != nil {
		return zero, err
	}
	return record, nil
}

func (s *Store) sanitize(value any) bool {
	changed := false
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			if str, ok := item.(string); ok {
				// Only free text carries secrets. Hashes, enums and timestamps
				// are structural identities, not literal secret-bearing fields.
				switch k {
				case "path", "reason", "boundary", "author", "channel", "expectation":
				default:
					continue
				}
				safe, yes := s.redact([]byte(str))
				v[k] = string(safe)
				changed = changed || yes
			} else {
				changed = s.sanitize(item) || changed
			}
		}
	case []any:
		for i, item := range v {
			if str, ok := item.(string); ok {
				safe, yes := s.redact([]byte(str))
				v[i] = string(safe)
				changed = changed || yes
			} else {
				changed = s.sanitize(item) || changed
			}
		}
	}
	return changed
}

// Get verifies the canonical content identity, shape, and locally stored inputs.
// Digests still do not authenticate a producer or authorize execution.
func Get[T Record](s *Store, id evidence.Digest) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return get[T](s, id)
}

// ReadRawRecord returns bounded exact bytes for a known record namespace without
// interpreting them. It is only for visibly limited inspection fallbacks; callers
// must not use these bytes as typed evidence.
func ReadRawRecord(s *Store, kind string, id evidence.Digest) ([]byte, error) {
	if kind != "snapshot" && kind != "capture" && kind != "scenario" && kind != "receipt" && kind != "comparison" && kind != "pin" {
		return nil, errors.New("unsupported record kind")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name, err := key(kind, string(id))
	if err != nil {
		return nil, err
	}
	return s.read(name, evidence.MaxRecordBytes)
}
func get[T Record](s *Store, id evidence.Digest) (T, error) {
	record, err := readRecord[T](s, id)
	if err != nil {
		var zero T
		return zero, err
	}
	if err := references(s, record); err != nil {
		var zero T
		return zero, err
	}
	return record, nil
}

func readRecord[T Record](s *Store, id evidence.Digest) (T, error) {
	var zero T
	kind, _ := identity(&zero)
	name, err := key(kind, string(id))
	if err != nil {
		return zero, err
	}
	data, err := s.read(name, evidence.MaxRecordBytes)
	if err != nil {
		return zero, err
	}
	record, err := evidence.Decode[T](bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	_, actual := identity(&record)
	if *actual != id {
		return zero, ErrCorrupt
	}
	*actual = ""
	canonical, _ := json.Marshal(record)
	if hash(canonical) != string(id) {
		return zero, ErrCorrupt
	}
	*actual = id
	return record, nil
}

func references[T Record](s *Store, record T) error {
	var err error
	switch r := any(record).(type) {
	case evidence.Snapshot:
		if r.IndexSnapshot != "" {
			index, e := get[evidence.Snapshot](s, r.IndexSnapshot)
			if e != nil {
				return e
			}
			if index.Source != evidence.Index || index.Commit != r.Commit || index.Unborn != r.Unborn {
				return errors.New("snapshot index association mismatch")
			}
		}
		if _, err = s.blob(r.Diff); err != nil {
			return fmt.Errorf("snapshot diff: %w", err)
		}
		for _, f := range r.Files {
			if _, err = s.blob(f.Content); err != nil {
				return fmt.Errorf("snapshot content: %w", err)
			}
		}
	case evidence.Capture:
		base, e := get[evidence.Snapshot](s, r.Base)
		if e != nil {
			return e
		}
		candidate, e := get[evidence.Snapshot](s, r.Candidate)
		if e != nil {
			return e
		}
		if base.Source != evidence.Commit {
			return errors.New("capture base must be a commit snapshot")
		}
		switch r.Mode {
		case evidence.WorkingTree:
			index, e := get[evidence.Snapshot](s, r.Index)
			if e != nil {
				return e
			}
			if candidate.Source != evidence.WorkingTree || candidate.IndexSnapshot != r.Index || index.Source != evidence.Index || index.Commit != candidate.Commit || index.Unborn != candidate.Unborn || base.Commit != candidate.Commit || base.Unborn != candidate.Unborn {
				return errors.New("working-tree capture snapshot associations differ")
			}
		case evidence.Index:
			if candidate.Source != evidence.Index || r.Index != r.Candidate || base.Commit != candidate.Commit || base.Unborn != candidate.Unborn {
				return errors.New("index capture snapshot associations differ")
			}
		case evidence.MergeBase:
			if candidate.Source != evidence.MergeBase || candidate.MergeBase != base.Commit || candidate.Unborn || base.Unborn {
				return errors.New("merge-base capture snapshot associations differ")
			}
		}
	case evidence.Scenario:
		_, err = s.blob(r.Input)
	case evidence.Receipt:
		if err = snapshots(s, r.Snapshots); err != nil {
			return err
		}
		if r.Bindings != nil {
			scenario, e := get[evidence.Scenario](s, r.Bindings.Scenario)
			if e != nil {
				return e
			}
			if scenario.Input != r.Bindings.Input || scenario.Driver != r.Bindings.Driver || scenario.Observer != r.Bindings.Observer || scenario.Rules != r.Bindings.Rules {
				return errors.New("receipt frozen bindings mismatch")
			}
		}
		for _, a := range r.Artifacts {
			if err = s.artifact(a); err != nil {
				return err
			}
		}
	case evidence.Comparison:
		if r.Details != nil {
			if err = s.artifact(*r.Details); err != nil {
				return err
			}
		}
		receipt, e := get[evidence.Receipt](s, r.Receipt)
		if e != nil {
			return e
		}
		if r.Completeness == evidence.Complete && receipt.Completeness != evidence.Complete {
			return errors.New("comparison cannot complete partial receipt")
		}
		if r.Outcome == evidence.Equal || r.Outcome == evidence.Different || r.Outcome == evidence.Unstable {
			if receipt.State.Kind != evidence.Observed || receipt.State.Execution != evidence.Completed {
				return errors.New("conclusive comparison requires observed receipt")
			}
		}
	case evidence.Pin:
		if _, err = get[evidence.Scenario](s, r.Scenario); err != nil {
			return err
		}
		receipt, e := get[evidence.Receipt](s, r.BasisReceipt)
		if e != nil {
			return e
		}
		if receipt.Snapshots != r.BasisSnapshots || receipt.Bindings == nil || receipt.Bindings.Scenario != r.Scenario {
			return errors.New("pin receipt and scenario/snapshot basis differ")
		}
		for _, event := range r.History {
			if event.Review == nil {
				continue
			}
			c := event.Review
			if err = snapshots(s, c.Target.Snapshots); err != nil {
				return err
			}
			if b := c.Target.Bindings; b != nil {
				scenario, e := get[evidence.Scenario](s, b.Scenario)
				if e != nil {
					return e
				}
				if scenario.Input != b.Input || scenario.Driver != b.Driver || scenario.Observer != b.Observer || scenario.Rules != b.Rules {
					return errors.New("review frozen bindings mismatch")
				}
			}
			if c.Receipt != "" {
				bound, e := get[evidence.Receipt](s, c.Receipt)
				if e != nil {
					return e
				}
				if bound.State.Producer != evidence.Runner || !reflect.DeepEqual(evidence.BasisOf(bound), c.Target) {
					return errors.New("review receipt does not match selected basis")
				}
			}
		}
		err = snapshots(s, r.BasisSnapshots)
	}
	if err != nil {
		return fmt.Errorf("record reference unavailable: %w", err)
	}
	return nil
}

func snapshots(s *Store, pair evidence.SnapshotPair) error {
	if pair.Base != "" {
		if _, err := get[evidence.Snapshot](s, pair.Base); err != nil {
			return err
		}
	}
	_, err := get[evidence.Snapshot](s, pair.Candidate)
	return err
}
