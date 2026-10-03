package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A synthetic CLI returns only capability metadata; no real Docker is contacted.
func TestMissingCapabilities(t *testing.T) {
	for _, missing := range []string{"OSType", "CgroupVersion", "MemoryLimit", "SwapLimit", "PidsLimit", "CPUCfsQuota", "SecurityOptions"} {
		t.Run(missing, func(t *testing.T) {
			info := map[string]any{
				"OSType": "linux", "CgroupVersion": "2", "MemoryLimit": true,
				"SwapLimit": true, "PidsLimit": true, "CPUCfsQuota": true,
				"SecurityOptions": []string{"name=seccomp,profile=builtin"},
			}
			delete(info, missing)
			data, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			binary := filepath.Join(dir, "docker")
			// All commands after info are errors and leave a marker.
			script := "#!/bin/sh\nif [ \"$5\" = info ]; then printf '%s' '" + string(data) + "'; else printf unexpected > unexpected; exit 1; fi\n"
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			d := Docker{Binary: binary, Host: "unix:///synthetic.sock"}
			if err := d.preflight(context.Background(), dir, Image); err == nil || !strings.Contains(err.Error(), "isolation unavailable") {
				t.Fatalf("missing capability did not fail closed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "unexpected")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("preflight continued after unsupported isolation")
			}
		})
	}
}

// AFTER_SANDBOX_PROOF is explicit authorization for ONLY these synthetic plans.
// Ordinary go test never contacts Docker or executes repository inputs.
func TestDockerProof(t *testing.T) {
	if os.Getenv("AFTER_SANDBOX_PROOF") != "1" {
		t.Skip("run task sandbox:proof to authorize synthetic offline probes")
	}
	d := Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}
	source, err := os.ReadFile("testdata/probe.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"probe.go": source}
	prepare := func(t *testing.T, args []string, seconds, outputBytes int) *Plan {
		t.Helper()
		p, e := Prepare(digest(source), files, args, Limits{seconds, outputBytes})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	execute := func(t *testing.T, ctx context.Context, p *Plan) (Result, error) {
		t.Helper()
		preview, id := p.Preview()
		t.Logf("Approved synthetic plan %s:\n%s", id, preview)
		r, e := d.Execute(ctx, p, id)
		t.Logf("result: %+v; error: %v", r, e)
		if r.Container != "" && !r.Cleaned {
			t.Fatal("owned container cleanup failed")
		}
		return r, e
	}
	t.Setenv("AFTER_HOST_SECRET", "synthetic-sentinel")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "synthetic-sentinel")
	t.Setenv("DOCKER_HOST", "tcp://untrusted.invalid:2375")
	t.Setenv("HTTP_PROXY", "http://untrusted.invalid")
	t.Run("http-and-attacks", func(t *testing.T) {
		p := prepare(t, []string{"/usr/local/go/bin/go", "run", "/input/probe.go"}, 180, 16384)
		r, e := execute(t, context.Background(), p)
		if e != nil || !strings.Contains(r.Output, "PROOF PASSED") {
			t.Fatalf("proof failed: %v", e)
		}
	})
	t.Run("denied", func(t *testing.T) {
		p := prepare(t, []string{"/bin/true"}, 30, 4096)
		r, e := d.Execute(context.Background(), p, "")
		if !errors.Is(e, ErrConsent) || r.Container != "" {
			t.Fatal("denied execution started")
		}
	})
	t.Run("missing-image", func(t *testing.T) {
		p := prepare(t, []string{"/bin/true"}, 30, 4096)
		p.spec.Image = "docker.io/library/golang@" + digest([]byte("nonexistent"))
		r, e := execute(t, context.Background(), p)
		if e == nil || r.ExitCode != -1 {
			t.Fatal("missing image did not fail closed")
		}
	})
	t.Run("missing-dependency", func(t *testing.T) {
		p, e := Prepare(digest(source), map[string][]byte{"go.mod": []byte("module proof\ngo 1.27.1\nrequire example.invalid/missing v1.0.0\n"), "main.go": []byte("package main\nimport _ \"example.invalid/missing\"\nfunc main(){}\n")}, []string{"/usr/local/go/bin/go", "run", "."}, Limits{30, 4096})
		if e != nil {
			t.Fatal(e)
		}
		r, e := execute(t, context.Background(), p)
		if e == nil || r.ExitCode == 0 {
			t.Fatal("missing dependency did not fail closed")
		}
	})
	t.Run("output", func(t *testing.T) {
		p := prepare(t, []string{"/bin/sh", "-c", "while :; do printf 'bounded-output'; done"}, 30, 1024)
		r, e := execute(t, context.Background(), p)
		if !errors.Is(e, ErrOutput) || !r.Truncated || len(r.Output) != 1024 {
			t.Fatal("output budget not enforced")
		}
	})
	t.Run("deadline-and-descendants", func(t *testing.T) {
		p := prepare(t, []string{"/bin/sh", "-c", "sleep 300 & printf 'DESCENDANT READY\\n'; wait"}, 5, 4096)
		r, e := execute(t, context.Background(), p)
		if !strings.Contains(r.Output, "DESCENDANT READY\n") {
			t.Fatal("deadline expired before descendant readiness")
		}
		if !errors.Is(e, context.DeadlineExceeded) {
			t.Fatalf("deadline not enforced: %v", e)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		var nonce [16]byte
		if _, e := rand.Read(nonce[:]); e != nil {
			t.Fatal(e)
		}
		marker := hex.EncodeToString(nonce[:])
		p := prepare(t, []string{"/bin/sh", "-c", "sleep 300 & printf '" + marker + "\\n'; printf '" + marker + "' > /work/ready; wait"}, 60, 4096)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Only this invocation's marker, written after spawning its child, permits
		// cancellation. An unrelated labelled container or watcher timeout cannot pass.
		var observed string
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			deadline := time.NewTimer(20 * time.Second)
			defer deadline.Stop()
			config, e := os.MkdirTemp("", "after-proof-config-")
			if e != nil {
				cancel()
				return
			}
			defer os.RemoveAll(config)
			for {
				select {
				case <-ctx.Done():
					return
				case <-deadline.C:
					cancel()
					return
				case <-ticker.C:
					b, e := d.query(ctx, config, nil, "container", "ls", "--filter", "label=after.owner", "--format", "{{.Names}}")
					if e != nil {
						continue
					}
					for _, name := range strings.Fields(string(b)) {
						// Inspect first: never exec in another invocation's container.
						args, e := d.query(ctx, config, nil, "inspect", "--format", "{{json .Config.Cmd}}", name)
						if e != nil || !strings.Contains(string(args), marker) {
							continue
						}
						ready, e := d.query(ctx, config, nil, "exec", name, "/bin/cat", "/work/ready")
						if e == nil && string(ready) == marker {
							observed = name
							cancel()
							return
						}
					}
				}
			}
		}()
		r, e := execute(t, ctx, p)
		cancel()
		<-done
		if observed == "" || observed != r.Container || !strings.Contains(r.Output, marker+"\n") {
			t.Fatal("cancellation did not observe this workload's descendant readiness")
		}
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("cancel not enforced: %v", e)
		}
	})
	t.Run("memory", func(t *testing.T) {
		p := prepare(t, []string{"/usr/local/go/bin/go", "run", "/input/probe.go", "oom"}, 180, 4096)
		r, e := execute(t, context.Background(), p)
		if e == nil || (!r.OOMKilled && !strings.Contains(r.Output, "signal: killed")) {
			t.Fatal("memory exhaustion was not killed")
		}
	})
	t.Run("scratch-budget", func(t *testing.T) {
		p := prepare(t, []string{"/bin/dd", "if=/dev/zero", "of=/work/fill", "bs=1048576", "count=513"}, 30, 4096)
		r, e := execute(t, context.Background(), p)
		if e == nil || !strings.Contains(r.Output, "No space left") {
			t.Fatal("scratch budget not enforced")
		}
	})
}
