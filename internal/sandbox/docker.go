package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Docker selects a trusted absolute CLI binary and local Unix daemon explicitly.
// Ambient Docker contexts/config, credentials, proxy and loader variables are ignored.
type Docker struct{ Binary, Host string }

type Result struct {
	Plan       string `json:"plan"`
	Container  string `json:"container,omitempty"`
	InputImage string `json:"input_image,omitempty"`
	Output     string `json:"output"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Truncated  bool   `json:"truncated"`
	ExitCode   int    `json:"exit_code"`
	OOMKilled  bool   `json:"oom_killed"`
	Cleaned    bool   `json:"cleaned"`
}

type output struct {
	mu        sync.Mutex
	b         bytes.Buffer
	max       int
	truncated bool
	cancel    context.CancelFunc
}

func (b *output) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if len(p) > b.max-b.b.Len() {
		p = p[:b.max-b.b.Len()]
		b.truncated = true
		if b.cancel != nil {
			b.cancel()
		}
	}
	b.b.Write(p)
	return n, nil
}

func (d Docker) command(ctx context.Context, config string, input []byte, out io.Writer, args ...string) error {
	return d.commandStreams(ctx, config, input, out, out, args...)
}

func (d Docker) commandStreams(ctx context.Context, config string, input []byte, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, d.Binary, append([]string{"--config", config, "--host", d.Host}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + config, "LANG=C"}
	cmd.Dir = config
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second
	return cmd.Run()
}
func (d Docker) query(ctx context.Context, config string, input []byte, args ...string) ([]byte, error) {
	child, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	b := &output{max: 1 << 20, cancel: cancel}
	err := d.command(child, config, input, b, args...)
	if b.truncated {
		return nil, ErrOutput
	}
	if child.Err() != nil {
		return nil, child.Err()
	}
	if err != nil {
		detail := b.b.Bytes()
		if len(detail) > 4096 {
			detail = detail[:4096]
		}
		return nil, fmt.Errorf("Docker %s failed: %w (%q)", args[0], err, detail)
	}
	return b.b.Bytes(), nil
}

func (d Docker) preflight(ctx context.Context, config, image, platform string) error {
	b, err := d.query(ctx, config, nil, "info", "--format", "{{json .}}")
	if err != nil {
		return err
	}
	var info struct {
		OSType, CgroupVersion                          string
		MemoryLimit, SwapLimit, PidsLimit, CPUCfsQuota bool
		SecurityOptions                                []string
	}
	if err = json.Unmarshal(b, &info); err != nil {
		return err
	}
	if info.OSType != "linux" || info.CgroupVersion != "2" || !info.MemoryLimit || !info.SwapLimit || !info.PidsLimit || !info.CPUCfsQuota || !strings.Contains(strings.Join(info.SecurityOptions, " "), "name=seccomp,profile=builtin") {
		return errors.New("required Linux cgroup-v2/resource/default-seccomp isolation unavailable")
	}
	b, err = d.query(ctx, config, nil, "image", "inspect", image)
	if err != nil {
		return errors.New("pinned image missing; explicit separate provisioning required")
	}
	var images []struct {
		Architecture string
		Os           string
		Config       struct {
			Volumes map[string]json.RawMessage
			OnBuild []string
		}
	}
	if err = json.Unmarshal(b, &images); err != nil {
		return err
	}
	parts := strings.Split(platform, "/")
	if len(images) != 1 || len(parts) != 2 || images[0].Os != parts[0] || images[0].Architecture != parts[1] {
		return errors.New("pinned image does not match the approved platform")
	}
	if len(images[0].Config.Volumes) != 0 || len(images[0].Config.OnBuild) != 0 {
		return errors.New("image has unsupported implicit volumes or build hooks")
	}
	return nil
}

// Execute starts nothing (not even a Docker query) without matching consent.
// Callers must obtain approval from the operator, never a repository file/report.
// Every error is incomplete execution, not an observation of equivalent behavior.
func (d Docker) Execute(ctx context.Context, p *Plan, approved string) (Result, error) {
	return d.execute(ctx, p, approved, "", nil, false)
}

// ExecuteBinary captures a bounded binary artifact on stdout and diagnostics on
// stderr. Overflow of either stream cancels the approved workload.
func (d Docker) ExecuteBinary(ctx context.Context, p *Plan, approved string) (Result, []byte, error) {
	result, err := d.execute(ctx, p, approved, "", nil, true)
	return result, []byte(result.Stdout), err
}

func (d Docker) execute(ctx context.Context, p *Plan, approved, network string, observe func(context.Context, string) error, binaryOutput bool) (result Result, err error) {
	result.ExitCode = -1
	if p == nil {
		return result, ErrConsent
	}
	_, id := p.Preview()
	result.Plan = id
	if approved != id {
		return result, ErrConsent
	}
	if !filepath.IsAbs(d.Binary) || !strings.HasPrefix(d.Host, "unix:///") || strings.ContainsAny(d.Host, "\x00\r\n") {
		return result, errors.New("explicit absolute Docker binary and local Unix socket required")
	}
	config, err := os.MkdirTemp("", "after-docker-config-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(config)
	ctx, cancel := context.WithTimeout(ctx, p.timeout())
	defer cancel()
	if err = d.preflight(ctx, config, p.spec.Image, p.spec.Platform); err != nil {
		return result, err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return result, err
	}
	name := "after-sandbox-" + hex.EncodeToString(random[:])
	result.Container = name
	stage := name + "-input"
	tag := name + ":input"
	// Only unpredictable names minted by this call can be removed. Never prune,
	// delete by a shared label, touch host paths, or terminate unrelated services.
	defer func() {
		cleanCtx, stop := context.WithTimeout(context.Background(), 60*time.Second)
		defer stop()
		var cleanupErr error
		for _, owned := range []string{name, stage} {
			cleanupErr = errors.Join(cleanupErr, d.removeContainer(cleanCtx, config, owned, name))
		}
		b, e := d.query(cleanCtx, config, nil, "image", "ls", "--filter", "reference="+tag, "--format", "{{.ID}}")
		if e == nil && strings.TrimSpace(string(b)) != "" {
			b, e = d.query(cleanCtx, config, nil, "image", "inspect", "--format", "{{index .Config.Labels \"after.owner\"}}", tag)
			if e == nil && strings.TrimSpace(string(b)) != name {
				e = errors.New("image ownership mismatch")
			}
			if e == nil {
				_, e = d.query(cleanCtx, config, nil, "image", "rm", tag)
			}
		}
		cleanupErr = errors.Join(cleanupErr, e)
		result.Cleaned = cleanupErr == nil
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup failed for owned resources %s: %w", name, cleanupErr))
		}
	}()
	// Docker cannot copy to a read-only rootfs. This staging container is NEVER
	// started: only bounded regular input files are copied and its layer committed.
	// No Dockerfile, RUN, build hook or repository executable is evaluated here.
	if _, err = d.query(ctx, config, nil, "create", "--platform="+p.spec.Platform, "--name", stage, "--label", "after.owner="+name, "--network=none", "--pull=never", p.spec.Image); err != nil {
		return result, err
	}
	if _, err = d.query(ctx, config, p.archive, "cp", "-", stage+":/"); err != nil {
		return result, err
	}
	b, err := d.query(ctx, config, nil, "commit", stage, tag)
	if err != nil {
		return result, err
	}
	result.InputImage = strings.TrimSpace(string(b))
	if !validDigest(result.InputImage) {
		return result, errors.New("invalid committed image identity")
	}
	args := []string{"create", "--platform=" + p.spec.Platform, "--name", name, "--label", "after.owner=" + name}
	for _, flag := range p.spec.Policy {
		if network != "" && flag == "--network=none" {
			flag = "--network=container:" + network
		}
		args = append(args, flag)
	}
	args = append(args, result.InputImage, "-i")
	args = append(args, p.spec.Environment...)
	args = append(args, p.spec.Argv...)
	if _, err = d.query(ctx, config, nil, args...); err != nil {
		return result, err
	}
	if observe != nil {
		return d.serve(ctx, config, name, p, result, observe)
	}
	if binaryOutput {
		return d.runBinary(ctx, config, name, p, result)
	}
	return d.run(ctx, config, name, p, result)
}

func (d Docker) removeContainer(ctx context.Context, config, name, owner string) error {
	b, e := d.query(ctx, config, nil, "container", "ls", "--all", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}")
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(b)) == "" {
		return nil
	}
	b, e = d.query(ctx, config, nil, "inspect", "--format", "{{index .Config.Labels \"after.owner\"}}", name)
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(b)) != owner {
		return errors.New("container ownership mismatch")
	}
	if _, e = d.query(ctx, config, nil, "container", "rm", "--force", name); e != nil {
		return e
	}
	b, e = d.query(ctx, config, nil, "container", "ls", "--all", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}")
	if e == nil && strings.TrimSpace(string(b)) != "" {
		return errors.New("owned container remains")
	}
	return e
}

func (d Docker) runBinary(ctx context.Context, config, name string, p *Plan, result Result) (Result, error) {
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	stdout := &output{max: 8 << 20, cancel: stop}
	stderr := &output{max: p.spec.Limits.OutputBytes, cancel: stop}
	runErr := d.commandStreams(runCtx, config, nil, stdout, stderr, "start", "--attach", name)
	result.Stdout, result.Stderr = stdout.b.String(), stderr.b.String()
	result.Truncated = stdout.truncated || stderr.truncated
	if result.Truncated {
		return result, ErrOutput
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	b, err := d.query(ctx, config, nil, "inspect", "--format", "{{json .State}}", name)
	if err != nil {
		return result, err
	}
	var state struct {
		ExitCode           int
		OOMKilled, Running bool
		Error              string
	}
	if err = json.Unmarshal(b, &state); err != nil {
		return result, err
	}
	result.ExitCode = state.ExitCode
	result.OOMKilled = state.OOMKilled
	if state.Running || state.Error != "" || state.OOMKilled || state.ExitCode != 0 || runErr != nil {
		return result, fmt.Errorf("sandbox binary preparation failed (exit=%d, oom=%t, attach=%v)", state.ExitCode, state.OOMKilled, runErr)
	}
	return result, nil
}

func (d Docker) run(ctx context.Context, config, name string, p *Plan, result Result) (Result, error) {
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	out := &output{max: p.spec.Limits.OutputBytes, cancel: stop}
	runErr := d.command(runCtx, config, nil, out, "start", "--attach", name)
	result.Output = out.b.String()
	result.Truncated = out.truncated
	// A cancelled CLI need not have stopped the container. Deferred force removal
	// kills the entire namespace, including descendants, before Execute returns.
	if out.truncated {
		return result, ErrOutput
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	b, e := d.query(ctx, config, nil, "inspect", "--format", "{{json .State}}", name)
	if e != nil {
		return result, e
	}
	var state struct {
		ExitCode           int
		OOMKilled, Running bool
		Error              string
	}
	if e = json.Unmarshal(b, &state); e != nil {
		return result, e
	}
	result.ExitCode = state.ExitCode
	result.OOMKilled = state.OOMKilled
	if state.Running || state.Error != "" || state.OOMKilled || state.ExitCode != 0 || runErr != nil {
		return result, fmt.Errorf("sandbox workload failed (exit=%d, oom=%t, attach=%v)", state.ExitCode, state.OOMKilled, runErr)
	}
	return result, nil
}
