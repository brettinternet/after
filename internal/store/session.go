package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"
)

const maxReviewSessionBytes = 64 << 10

// ReadReviewSession returns the bounded private UI state, without treating it
// as evidence. A missing session is reported as os.ErrNotExist.
func (s *Store) ReadReviewSession() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil, ErrReadOnly
	}
	file, err := s.root.OpenFile("session.json", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := privateFile(file); err != nil {
		return nil, fmt.Errorf("%w: unsafe review session", ErrCorrupt)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxReviewSessionBytes {
		return nil, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(file, maxReviewSessionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxReviewSessionBytes {
		return nil, ErrLimit
	}
	return data, nil
}

// WriteReviewSession atomically replaces the small UI-state file. The pending
// file is private, synced, and renamed only after its complete contents are
// durable; failures before rename preserve the previous session.
func (s *Store) WriteReviewSession(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil || s.root == nil {
		return ErrReadOnly
	}
	if len(data) == 0 || len(data) > maxReviewSessionBytes {
		return ErrLimit
	}
	if err := s.budgetReplacing("session.json", int64(len(data))); err != nil {
		return err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := "session-pending-" + hex.EncodeToString(random[:])
	file, err := s.root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fmt.Errorf("create review session: %w", err)
	}
	defer s.root.Remove(temporary)
	if err := s.checkpoint("session-created"); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write review session: %w", err)
	}
	if err := s.checkpoint("session-written"); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync review session: %w", err)
	}
	if err := s.checkpoint("session-synced"); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := s.root.Rename(temporary, "session.json"); err != nil {
		return fmt.Errorf("replace review session: %w", err)
	}
	if err := s.checkpoint("session-renamed"); err != nil {
		return err
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync review session directory: %w", err)
	}
	if err := s.checkpoint("session-directory-synced"); err != nil {
		return err
	}
	return nil
}
