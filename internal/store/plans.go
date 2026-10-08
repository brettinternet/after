package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
)

const MaxPlanBytes = 1 << 20

// PutPlan publishes exact preview bytes under their full authorization digest.
// A second publish of identical bytes is harmless; different bytes can never
// replace the stored plan.
func (s *Store) PutPlan(id evidence.Digest, preview []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return ErrReadOnly
	}
	if len(preview) == 0 || len(preview) > MaxPlanBytes || hash(preview) != string(id) {
		return errors.New("execution plan digest or size is invalid")
	}
	name, err := planKey(id)
	if err != nil {
		return err
	}
	return s.publish(name, preview)
}

// ReadPlan returns exact bytes only when the immutable name and content digest
// agree. It never interprets or executes stored plan content.
func (s *Store) ReadPlan(id evidence.Digest) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, err := planKey(id)
	if err != nil {
		return nil, err
	}
	preview, err := s.read(name, MaxPlanBytes)
	if err != nil {
		return nil, err
	}
	if hash(preview) != string(id) {
		return nil, ErrCorrupt
	}
	return preview, nil
}

func planKey(id evidence.Digest) (string, error) {
	value := string(id)
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || value != strings.ToLower(value) {
		return "", fmt.Errorf("invalid execution plan digest")
	}
	if _, err := key("plan", value); err != nil {
		return "", err
	}
	return key("plan", value)
}
