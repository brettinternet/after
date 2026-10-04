package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagesWithContaminatedCPUSettings(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("GOAMD64", "v3")
	t.Setenv("GOARM64", "v8.2")
	parent := t.TempDir()
	// build verifies embedded OS/architecture/toolchain/CPU baseline metadata
	// for every target, and executes help/version for the native target.
	if err := build(parent); err != nil {
		t.Fatal(err)
	}
	dirs, err := os.ReadDir(parent)
	if err != nil || len(dirs) != 1 {
		t.Fatalf("package directories: %v %v", dirs, err)
	}
	dir := filepath.Join(parent, dirs[0].Name())
	sums, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 4 {
		t.Fatalf("checksums: %s", sums)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("checksum line: %s", line)
		}
		data, err := os.ReadFile(filepath.Join(dir, fields[1]))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("checksum mismatch: %s", fields[1])
		}
	}
}
