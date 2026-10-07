package cli

import (
	"errors"
	"os"
	"path/filepath"
)

// projectRoot resolves a checkout using filesystem entries only. It accepts the
// .git directory used by ordinary repositories and the .git file used by linked
// worktrees; it never invokes Git or follows a .git symlink.
func projectRoot(start string) (string, bool) {
	if start == "" {
		return "", false
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", false
	}
	for {
		info, err := os.Lstat(filepath.Join(dir, ".git"))
		if err == nil {
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, true
			}
			return "", false
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
