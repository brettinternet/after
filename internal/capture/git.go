package capture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

const maxOutput = 16 << 20

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > maxOutput-b.Len() {
		return 0, ErrBudget
	}
	return b.Buffer.Write(p)
}

// No inherited Git, loader, shell, credential, tracing or proxy environment.
// Local configuration cannot enable helpers for the plumbing commands we use.
func git(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	// Supported macOS/Linux hosts provide Git here. Never resolve it through
	// an ambient PATH that may include the repository or an attacker directory.
	const executable = "/usr/bin/git"
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	prefix := []string{"--no-optional-locks", "--no-replace-objects", "--literal-pathspecs",
		"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null",
		"-c", "core.untrackedCache=false", "-c", "core.attributesFile=/dev/null",
		"-c", "core.excludesFile=/dev/null", "-c", "protocol.allow=never",
		"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "core.pager=cat",
		"-c", "core.bare=false", "-c", "core.worktree=" + dir}
	cmd := exec.CommandContext(ctx, executable, append(prefix, args...)...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "HOME=/dev/null", "XDG_CONFIG_HOME=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_ATTR_NOSYSTEM=1"}
	cmd.Stdin = bytes.NewReader(input)
	var out boundedBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard // Never echo repository-controlled output or private paths.
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrBudget) {
			return nil, ErrBudget
		}
		return nil, errors.New("Git plumbing failed (unsupported repository, missing object or output budget)")
	}
	return out.Bytes(), nil
}
