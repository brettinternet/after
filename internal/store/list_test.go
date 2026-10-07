package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
)

func TestListKindsAndSafety(t *testing.T) {
	for _, bad := range []string{"none", "name", "symlink", "permissions", "oversize"} {
		t.Run(bad, func(t *testing.T) {
			project := t.TempDir()
			s, err := Open(project, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a, err := s.PutArtifact([]byte("fixture"), "fixture", 1024)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := s.List("blob", "blob")
			if err != nil || len(entries) != 1 || entries[0].ID != a.Content || entries[0].Kind != "blob" || entries[0].Size != 7 {
				t.Fatalf("list: %+v %v", entries, err)
			}
			if entries, err := s.List("pin"); err != nil || len(entries) != 0 {
				t.Fatalf("kind filter: %+v %v", entries, err)
			}
			if _, err := s.List("../blob"); err == nil {
				t.Fatal("invalid kind accepted")
			}
			name := filepath.Join(project, ".after", "pin-"+strings.Repeat("0", 64))
			switch bad {
			case "none":
				return
			case "name":
				name = filepath.Join(project, ".after", "pin-invalid")
			case "symlink":
				if err := os.Symlink(".gitignore", name); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(name, []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
				if bad == "permissions" {
					if err := os.Chmod(name, 0644); err != nil {
						t.Fatal(err)
					}
				}
				if bad == "oversize" {
					if err := os.Truncate(name, evidence.MaxRecordBytes+1); err != nil {
						t.Fatal(err)
					}
				}
			}
			if bad == "name" {
				if err := os.WriteFile(name, []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.List("pin"); err == nil {
				t.Fatal("unsafe listing accepted")
			}
			// Invalid entries of unrelated kinds do not affect this namespace.
			if entries, err := s.List("blob"); err != nil || len(entries) != 1 {
				t.Fatalf("unrelated kind: %+v %v", entries, err)
			}
		})
	}
}

func TestListNeverReturnsPartialNamespace(t *testing.T) {
	project := t.TempDir()
	s, err := Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < MaxEntries; i++ {
		if err := os.WriteFile(filepath.Join(project, ".after", fmt.Sprintf("pending-%d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if entries, err := s.List("pin"); !errors.Is(err, ErrLimit) || entries != nil {
		t.Fatalf("partial namespace: %+v %v", entries, err)
	}
}
