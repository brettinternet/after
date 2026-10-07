// Package store persists bounded private evidence. It never executes project code.
package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
)

var (
	ErrLocked   = errors.New("store already has a writer")
	ErrReadOnly = errors.New("store is read-only or closed")
	ErrLimit    = errors.New("store limit exceeded")
	ErrCorrupt  = errors.New("store content is corrupt")
)

const MaxBlobBytes = 16 << 20
const MaxEntries = 10000
const MaxStoreBytes int64 = 512 << 20

// Store holds a directory capability and, for writers, a lifetime OS lock.
// The mutex serializes calls within one writer; readers do not acquire the lock.
type Store struct {
	mu      sync.Mutex
	root    *os.Root
	lock    *os.File
	secrets []string
	// Tests inject failures at durability boundaries; never a public hook.
	fault func(string) error
}

// Open takes a trusted project directory, not a repository-supplied store path.
// Read-only opens never create files. Writers use private .after storage.
func Open(project string, writable bool, secrets []string) (*Store, error) {
	if len(secrets) > 128 {
		return nil, ErrLimit
	}
	for _, s := range secrets {
		if s == "" || len(s) > 4096 {
			return nil, errors.New("invalid redaction literal")
		}
	}
	parent, err := os.OpenRoot(project)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}
	defer parent.Close()
	if writable {
		if err := parent.Mkdir(".after", 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create store: %w", err)
		}
	}
	info, err := parent.Lstat(".after")
	if err != nil {
		return nil, fmt.Errorf("inspect store: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.New("store must be a private non-symlink directory (0700)")
	}
	root, err := parent.OpenRoot(".after")
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	s := &Store{root: root, secrets: append([]string(nil), secrets...)}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, errors.New("store changed while opening")
	}
	if writable {
		lock, err := root.OpenFile("writer.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
		if err != nil {
			root.Close()
			return nil, fmt.Errorf("open writer lock: %w", err)
		}
		if err = privateFile(lock); err == nil {
			err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		}
		if err != nil {
			lock.Close()
			root.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return nil, ErrLocked
			}
			return nil, fmt.Errorf("acquire writer lock: %w", err)
		}
		s.lock = lock
		if err := s.publish(".gitignore", []byte("*\n")); err != nil {
			s.Close()
			return nil, fmt.Errorf("publish store ignore file: %w", err)
		}
	}
	return s, nil
}

func privateFile(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("store entry must be a private regular file (0600)")
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) {
		return errors.New("store entry has unsafe ownership or hard links")
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if s.lock != nil {
		err = s.lock.Close()
		s.lock = nil
	}
	if s.root != nil {
		err = errors.Join(err, s.root.Close())
		s.root = nil
	}
	return err
}

func hash(data []byte) string { h := sha256.Sum256(data); return "sha256:" + hex.EncodeToString(h[:]) }
func key(kind, id string) (string, error) {
	if len(id) != 71 || !strings.HasPrefix(id, "sha256:") || strings.ToLower(id) != id {
		return "", errors.New("invalid content identity")
	}
	if _, err := hex.DecodeString(id[7:]); err != nil {
		return "", errors.New("invalid content identity")
	}
	return kind + "-" + id[7:], nil
}

func (s *Store) read(name string, max int64) ([]byte, error) {
	if s.root == nil {
		return nil, ErrReadOnly
	}
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open store entry: %w", err)
	}
	defer f.Close()
	if err := privateFile(f); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, fmt.Errorf("read store entry: %w", err)
	}
	if int64(len(data)) > max {
		return nil, ErrLimit
	}
	return data, nil
}

func (s *Store) budget(extra int64) error {
	return s.budgetReplacing("", extra)
}

func (s *Store) budgetReplacing(replaced string, extra int64) error {
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(MaxEntries + 1)
	if err != nil && err != io.EOF {
		return err
	}
	total, count := extra, 0
	for _, entry := range entries {
		if entry.Name() == replaced {
			continue
		}
		count++
		if count >= MaxEntries {
			return ErrLimit
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Name() == "session.json" {
			// Invalid UI state remains replaceable, but regular files still
			// consume their full size until a session save replaces them.
			if info.Mode().IsRegular() {
				total += info.Size()
			}
			if total > MaxStoreBytes {
				return ErrLimit
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return errors.New("unsafe entry in store")
		}
		total += info.Size()
		if total > MaxStoreBytes {
			return ErrLimit
		}
	}
	return nil
}

func (s *Store) checkpoint(stage string) error {
	if s.fault != nil {
		return s.fault(stage)
	}
	return nil
}

// publish never replaces an existing name. Link publishes a fully synced inode.
func (s *Store) publish(name string, data []byte) error {
	if s.lock == nil || s.root == nil {
		return ErrReadOnly
	}
	old, err := s.read(name, int64(len(data)))
	if err == nil {
		if bytes.Equal(old, data) {
			return nil
		}
		return ErrCorrupt
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.budget(int64(len(data))); err != nil {
		return err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	tmp := "pending-" + hex.EncodeToString(random[:])
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fmt.Errorf("create pending entry: %w", err)
	}
	defer s.root.Remove(tmp)
	defer f.Close()
	if err := s.checkpoint("created"); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	if err := s.checkpoint("written"); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync entry: %w", err)
	}
	if err := s.checkpoint("synced"); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := s.root.Link(tmp, name); err != nil {
		return fmt.Errorf("publish entry: %w", err)
	}
	// Remove the extra link before readers accept the file; incomplete publication
	// may produce a safe error, never a partial record, after a crash here.
	if err := s.root.Remove(tmp); err != nil {
		return err
	}
	if err := s.checkpoint("published"); err != nil {
		return err
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync store: %w", err)
	}
	return s.checkpoint("directory-synced")
}
