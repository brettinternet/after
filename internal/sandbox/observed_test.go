package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExperimentConsent(t *testing.T) {
	app, e := Prepare(digest([]byte("app")), map[string][]byte{"main.go": []byte("app")}, []string{"/bin/true"}, Limits{10, 1024})
	if e != nil {
		t.Fatal(e)
	}
	observer, e := Prepare(digest([]byte("observer")), map[string][]byte{"main.go": []byte("observer")}, []string{"/bin/true"}, Limits{10, 1024})
	if e != nil {
		t.Fatal(e)
	}
	p, e := PrepareExperiment(app, observer)
	if e != nil {
		t.Fatal(e)
	}
	_, aid := app.Preview()
	_, id := p.Preview()
	if aid == id {
		t.Fatal("topology not bound")
	}
	r, e := (Docker{}).Observe(t.Context(), p, aid)
	if !errors.Is(e, ErrConsent) || r.App.Container != "" {
		t.Fatal("individual consent authorized topology")
	}
}

func TestObservedProof(t *testing.T) {
	if os.Getenv("AFTER_RUNNER_PROOF") != "1" {
		t.Skip("task runner:proof authorizes synthetic lifecycle probes")
	}
	d := Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}
	config := t.TempDir()
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		t.Fatal(e)
	}
	token := hex.EncodeToString(nonce[:])
	sentinel := "after-sentinel-" + token
	if _, e := d.query(t.Context(), config, nil, "create", "--name", sentinel, "--label", "after.owner="+sentinel, "--network=none", "--pull=never", Image, "/bin/sleep", "300"); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := d.removeContainer(context.Background(), config, sentinel, sentinel); e != nil {
			t.Error(e)
		}
	}()
	if _, e := d.query(t.Context(), config, nil, "start", sentinel); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"isolation", "output", "startup", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			script := "sleep 300 & printf 'CHILD READY\\n'; printf private > /work/app-private; wait"
			obs := "test ! -e /work/app-private && test ! -e /input/app.txt && printf 'OBSERVER ONLY\\n'"
			limit := Limits{30, 4096}
			switch mode {
			case "output":
				obs = "while :; do printf 'observer stdout'; printf 'observer stderr' >&2; done"
			case "startup":
				script = "exit 42"
			case "timeout":
				limit.Seconds = 5
				obs = "sleep 300 & printf 'OBSERVER CHILD READY\\n'; wait"
			case "cancel":
				obs = "sleep 300 & printf 'OBSERVER CHILD READY\\n'; printf '" + token + "' > /work/ready; wait"
			}
			app, e := Prepare(digest([]byte("app")), map[string][]byte{"app.txt": []byte("private")}, []string{"/bin/sh", "-c", script}, limit)
			if e != nil {
				t.Fatal(e)
			}
			observer, e := Prepare(digest([]byte("observer")), map[string][]byte{"observer.txt": []byte("private")}, []string{"/bin/sh", "-c", obs}, limit)
			if e != nil {
				t.Fatal(e)
			}
			p, e := PrepareExperiment(app, observer)
			if e != nil {
				t.Fatal(e)
			}
			preview, id := p.Preview()
			t.Logf("approved %s: %s", id, preview)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			watched := make(chan string, 1)
			if mode == "cancel" {
				go func() {
					ticker := time.NewTicker(20 * time.Millisecond)
					defer ticker.Stop()
					timeout := time.NewTimer(20 * time.Second)
					defer timeout.Stop()
					for {
						select {
						case <-ctx.Done():
							watched <- ""
							return
						case <-timeout.C:
							watched <- ""
							cancel()
							return
						case <-ticker.C:
							b, e := d.query(ctx, config, nil, "container", "ls", "--filter", "label=after.owner", "--format", "{{.Names}}")
							if e != nil {
								continue
							}
							for _, name := range strings.Fields(string(b)) {
								b, e := d.query(ctx, config, nil, "inspect", "--format", "{{json .Config.Cmd}}", name)
								if e != nil || !strings.Contains(string(b), token) {
									continue
								}
								b, e = d.query(ctx, config, nil, "exec", name, "/bin/cat", "/work/ready")
								if e == nil && string(b) == token {
									watched <- name
									cancel()
									return
								}
							}
						}
					}
				}()
			}
			r, e := d.Observe(ctx, p, id)
			cancel()
			t.Logf("result %+v error %v", r, e)
			if mode == "isolation" {
				if e != nil || !strings.Contains(r.Observer.Output, "OBSERVER ONLY") {
					t.Fatal("isolation probe failed", e)
				}
			} else if e == nil {
				t.Fatal("negative probe succeeded")
			}
			if mode == "output" && (!errors.Is(e, ErrOutput) || !r.Observer.Truncated) {
				t.Fatal("output not bounded")
			}
			if mode == "timeout" && (!errors.Is(e, context.DeadlineExceeded) || !strings.Contains(r.Observer.Output, "OBSERVER CHILD READY")) {
				t.Fatal("deadline before descendants", e)
			}
			if mode == "cancel" {
				name := <-watched
				if name == "" || name != r.Observer.Container || !strings.Contains(r.Observer.Output, "OBSERVER CHILD READY") || !errors.Is(e, context.Canceled) {
					t.Fatal("cancellation did not observe owned descendants", e)
				}
			}
			for _, owned := range []Result{r.App, r.Observer} {
				if owned.Container != "" && !owned.Cleaned {
					t.Fatal("cleanup failed", owned.Container)
				}
			}
			b, e := d.query(t.Context(), config, nil, "inspect", "--format", "{{.State.Running}}", sentinel)
			if e != nil || string(b) != "true\n" {
				t.Fatal("unrelated container stopped")
			}
		})
	}
}
