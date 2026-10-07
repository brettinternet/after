package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
)

const redactionPolicy = "literal-v1"

// redact covers all occurrences, including overlapping literals. The replacement
// is not rescanned and literal values never become policy metadata or errors.
func (s *Store) redact(data []byte) ([]byte, bool) {
	mask := make([]bool, len(data))
	changed := false
	for _, secret := range s.secrets {
		for start := 0; start < len(data); {
			i := bytes.Index(data[start:], []byte(secret))
			if i < 0 {
				break
			}
			i += start
			for j := i; j < i+len(secret); j++ {
				mask[j] = true
			}
			changed = true
			start = i + 1
		}
	}
	if !changed {
		return data, false
	}
	var out bytes.Buffer
	for i := 0; i < len(data); {
		if !mask[i] {
			out.WriteByte(data[i])
			i++
			continue
		}
		out.WriteString("[REDACTED]")
		for i < len(data) && mask[i] {
			i++
		}
	}
	return out.Bytes(), true
}

// PutArtifact bounds input, redacts in memory, then truncates retained bytes.
// Secrets must be supplied explicitly; this is not automatic secret discovery.
func (s *Store) PutArtifact(data []byte, channel string, max int64) (evidence.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero evidence.Artifact
	if s.lock == nil {
		return zero, ErrReadOnly
	}
	if len(data) > MaxBlobBytes || max <= 0 || max > MaxBlobBytes || len(channel) > 256 || strings.TrimSpace(channel) == "" {
		return zero, ErrLimit
	}
	safe, redacted := s.redact(data)
	safeChannel, channelRedacted := s.redact([]byte(channel))
	a := evidence.Artifact{Channel: string(safeChannel), MaxBytes: max, Completeness: evidence.Complete, Redacted: redacted || channelRedacted, RedactionPolicy: redactionPolicy}
	if int64(len(safe)) > max {
		safe = safe[:max]
		a.Truncated = true
	}
	if a.Redacted || a.Truncated {
		a.Completeness = evidence.Incomplete
	}
	a.Bytes = int64(len(safe))
	a.Content = evidence.Digest(hash(safe))
	name, _ := key("blob", string(a.Content))
	if err := s.publish(name, safe); err != nil {
		return zero, err
	}
	descriptor, _ := json.Marshal(a)
	name, _ = key("artifact", hash(descriptor))
	if err := s.publish(name, descriptor); err != nil {
		return zero, err
	}
	return a, nil
}

// ReadBlob accepts only a digest, never an arbitrary repository path.
func (s *Store) ReadBlob(id evidence.Digest) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blob(id)
}
func (s *Store) blob(id evidence.Digest) ([]byte, error) {
	name, err := key("blob", string(id))
	if err != nil {
		return nil, err
	}
	data, err := s.read(name, MaxBlobBytes)
	if err != nil {
		return nil, err
	}
	if hash(data) != string(id) {
		return nil, ErrCorrupt
	}
	return data, nil
}

// ReadArtifactMetadata verifies a content-addressed artifact descriptor without
// reading its referenced blob. Discovery can filter by trusted descriptor fields
// before opening report bodies or large captured source blobs.
func (s *Store) ReadArtifactMetadata(id evidence.Digest) (evidence.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero evidence.Artifact
	name, err := key("artifact", string(id))
	if err != nil {
		return zero, err
	}
	raw, err := s.read(name, evidence.MaxRecordBytes)
	if err != nil {
		return zero, err
	}
	if hash(raw) != string(id) {
		return zero, ErrCorrupt
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var artifact evidence.Artifact
	if err := decoder.Decode(&artifact); err != nil || decoder.Decode(new(any)) != io.EOF {
		return zero, ErrCorrupt
	}
	if artifact.Content == "" || len(artifact.Channel) == 0 || len(artifact.Channel) > 256 || artifact.Bytes < 0 || artifact.MaxBytes <= 0 || artifact.MaxBytes > MaxBlobBytes || artifact.Bytes > artifact.MaxBytes || (artifact.Completeness != evidence.Complete && artifact.Completeness != evidence.Incomplete) || ((artifact.Redacted || artifact.Truncated) && artifact.Completeness == evidence.Complete) {
		return zero, ErrCorrupt
	}
	if artifact.RedactionPolicy != "" && artifact.RedactionPolicy != redactionPolicy || artifact.Redacted && artifact.RedactionPolicy != redactionPolicy {
		return zero, ErrCorrupt
	}
	return artifact, nil
}

func (s *Store) artifact(a evidence.Artifact) error {
	descriptor, err := json.Marshal(a)
	if err != nil {
		return err
	}
	name, _ := key("artifact", hash(descriptor))
	actual, err := s.read(name, evidence.MaxRecordBytes)
	if err != nil {
		return fmt.Errorf("artifact metadata unavailable: %w", err)
	}
	if !bytes.Equal(actual, descriptor) {
		return ErrCorrupt
	}
	data, err := s.blob(a.Content)
	if err != nil {
		return fmt.Errorf("artifact unavailable: %w", err)
	}
	if int64(len(data)) != a.Bytes {
		return errors.New("artifact size mismatch")
	}
	return nil
}
