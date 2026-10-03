package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxRecordBytes bounds manifest decoding. Artifact bodies are not inline JSON.
const MaxRecordBytes = 4 << 20

// Decode checks shape and invariants, not provenance. Untrusted reports must be
// converted to importer records, never decoded as trusted runner receipts.
func Decode[T interface {
	Snapshot | Scenario | Receipt | Comparison | Pin
	Validate() error
}](r io.Reader) (T, error) {
	var zero T
	data, err := io.ReadAll(io.LimitReader(r, MaxRecordBytes+1))
	if err != nil {
		return zero, fmt.Errorf("read record: %w", err)
	}
	if len(data) > MaxRecordBytes {
		return zero, errors.New("record exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record T
	if err := decoder.Decode(&record); err != nil {
		return zero, fmt.Errorf("decode record: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return zero, errors.New("record must contain exactly one JSON value")
	}
	if err := record.Validate(); err != nil {
		return zero, err
	}
	return record, nil
}
