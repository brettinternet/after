package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
)

func captured(t *testing.T, mutation string) (*store.Store, evidence.SnapshotPair) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, b)
		}
	}
	put := func(name string, b []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	git("init", "-q", "--template=", "-b", "main")
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"} {
		b, e := os.ReadFile("../paymentfixture/testdata/payment/" + name)
		if e != nil {
			t.Fatal(e)
		}
		put(name, b)
	}
	git("add", ".")
	git("commit", "-qm", "synthetic base")
	b, e := os.ReadFile(filepath.Join(dir, "app/config.go"))
	if e != nil {
		t.Fatal(e)
	}
	put("app/config.go", []byte(strings.Replace(string(b), "24 * 60 * 60", "5 * 60", 1)))
	// A candidate-owned driver and test cannot become the trusted observer.
	put("driver/main.go", []byte("package main\nfunc main(){panic(\"candidate oracle ran\")}\n"))
	put("app/oracle_test.go", []byte("package main\nimport \"testing\"\nfunc TestOracle(t *testing.T){panic(\"candidate suite ran\")}\n"))
	if mutation != "" {
		put("app/attack.go", []byte(mutation))
	}
	s, e := store.Open(dir, true, []string{"synthetic-secret"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	pair, e := capture.Capture(t.Context(), dir, s, capture.Options{Mode: evidence.WorkingTree, IncludeUntracked: []string{"app/oracle_test.go"}})
	if mutation != "" {
		pair, e = capture.Capture(t.Context(), dir, s, capture.Options{Mode: evidence.WorkingTree, IncludeUntracked: []string{"app/oracle_test.go", "app/attack.go"}})
	}
	if e != nil {
		t.Fatal(e)
	}
	return s, evidence.SnapshotPair{Base: pair.Base.ID, Candidate: pair.Candidate.ID}
}
func prepared(t *testing.T, s *store.Store, pair evidence.SnapshotPair, reps int) *Plan {
	t.Helper()
	p, e := Prepare(s, pair, reps, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func success(sec int64, body string) sandbox.ExperimentResult {
	b, _ := json.Marshal(Observation{Version: 1, Seconds: sec, Responses: []Response{{200, body}, {200, body}}, Calls: []Call{}})
	return sandbox.ExperimentResult{App: sandbox.Result{Cleaned: true, Output: "synthetic-secret"}, Observer: sandbox.Result{Cleaned: true, ExitCode: 0, Output: string(b)}}
}
func TestFrozenPlanAndDenial(t *testing.T) {
	s, pair := captured(t, "")
	p := prepared(t, s, pair, 1)
	preview, id := p.Preview()
	preview[0] = 'x'
	next, id2 := p.Preview()
	if next[0] == 'x' || id != id2 {
		t.Fatal("mutable plan")
	}
	calls := 0
	e := Executor{observe: func(context.Context, *sandbox.Experiment, string) (sandbox.ExperimentResult, error) {
		calls++
		panic("denied")
	}}
	r, err := e.Run(t.Context(), s, p, "wrong")
	if err == nil || calls != 0 || r.Receipt.State.Execution != evidence.NotRun || r.Receipt.Completeness != evidence.Incomplete || len(r.Samples) != 4 {
		t.Fatalf("denial: %+v %v", r, err)
	}
	stored, err := store.Get[evidence.Receipt](s, r.Receipt.ID)
	if err != nil || stored.RequestID != p.RequestID() {
		t.Fatal("request not persisted", err)
	}
	for _, sample := range r.Samples {
		if sample.Status != "permission_denied" {
			t.Fatal(sample.Status)
		}
	}
	changed := prepared(t, s, pair, 2)
	_, changedID := changed.Preview()
	if changedID == id {
		t.Fatal("repetition/request not bound")
	}
}
func TestLateResultAndInstability(t *testing.T) {
	s, pair := captured(t, "")
	p := prepared(t, s, pair, 2)
	entered, release := make(chan struct{}), make(chan struct{})
	n := 0
	e := Executor{observe: func(ctx context.Context, _ *sandbox.Experiment, _ string) (sandbox.ExperimentResult, error) {
		if n == 0 {
			close(entered)
			<-release
		}
		index := n
		n++
		sec := seconds[index%2]
		body := "same"
		if index == 6 {
			body = "changed"
		}
		r := success(sec, body)
		r.App.Output = ""
		return r, nil
	}}
	type finished struct {
		r Result
		e error
	}
	done := make(chan finished, 1)
	go func() { _, id := p.Preview(); r, err := e.Run(t.Context(), s, p, id); done <- finished{r, err} }()
	<-entered
	selected := prepared(t, s, evidence.SnapshotPair{Base: pair.Candidate, Candidate: pair.Base}, 1)
	close(release)
	got := <-done
	if got.e != nil || got.r.Receipt.State.Comparison != evidence.Unstable || len(got.r.Samples) != 8 {
		t.Fatalf("samples: %+v %v", got.r.Receipt, got.e)
	}
	if got.r.Receipt.Snapshots != pair || got.r.Receipt.RequestID == selected.RequestID() {
		t.Fatal("late result rebound")
	}
	for _, sample := range got.r.Samples {
		if sample.Snapshots != pair || sample.RequestID != p.RequestID() {
			t.Fatal("sample rebound")
		}
	}
	if _, err := store.Get[evidence.Receipt](s, got.r.Receipt.ID); err != nil {
		t.Fatal(err)
	}
}
func TestFailuresAndRedaction(t *testing.T) {
	for _, kind := range []string{"timeout", "cancelled", "build", "missing", "protocol", "output", "cleanup", "redaction"} {
		t.Run(kind, func(t *testing.T) {
			s, pair := captured(t, "")
			p := prepared(t, s, pair, 1)
			n := 0
			e := Executor{observe: func(context.Context, *sandbox.Experiment, string) (sandbox.ExperimentResult, error) {
				r := success(seconds[n%2], "ok")
				n++
				switch kind {
				case "timeout":
					return r, context.DeadlineExceeded
				case "cancelled":
					return r, context.Canceled
				case "build":
					return r, errors.New("synthetic-secret compiler failure")
				case "missing":
					r.Observer.Output = `{"version":1,"seconds":43200,"responses":[]}`
				case "protocol":
					r.Observer.Output = `{"version":99}`
				case "output":
					r.App.Truncated = true
					return r, sandbox.ErrOutput
				case "cleanup":
					r.App.Container = "owned"
					r.App.Cleaned = false
				}
				return r, nil
			}}
			_, id := p.Preview()
			r, err := e.Run(t.Context(), s, p, id)
			if r.Receipt.Completeness != evidence.Incomplete || r.Receipt.State.Comparison != evidence.Incomparable || r.Receipt.State.Kind == evidence.Observed {
				t.Fatalf("failure promoted: %+v %v", r.Receipt, err)
			}
			if kind == "cleanup" && n != 1 {
				t.Fatal("continued after failed cleanup")
			}
			for _, a := range r.Receipt.Artifacts {
				b, err := s.ReadBlob(a.Content)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(b), "synthetic-secret") {
					t.Fatal("secret retained")
				}
			}
		})
	}
}
func TestConcurrencyAndQueuedCancellation(t *testing.T) {
	s, pair := captured(t, "")
	p := prepared(t, s, pair, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	e := Executor{observe: func(context.Context, *sandbox.Experiment, string) (sandbox.ExperimentResult, error) {
		n := int(calls.Add(1)) - 1
		if n == 0 {
			close(entered)
			<-release
		}
		r := success(seconds[n%2], "ok")
		r.App.Output = ""
		return r, nil
	}}
	done := make(chan error, 1)
	_, id := p.Preview()
	go func() { _, err := e.Run(t.Context(), s, p, id); done <- err }()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	other := prepared(t, s, pair, 1)
	_, oid := other.Preview()
	r, err := e.Run(ctx, s, other, oid)
	if err == nil || r.Receipt.State.Execution != evidence.Cancelled || calls.Load() != 1 {
		t.Fatal("queued cancellation executed")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunnerProof(t *testing.T) {
	if os.Getenv("AFTER_RUNNER_PROOF") != "1" {
		t.Skip("task runner:proof authorizes synthetic paired experiments")
	}
	// Forged stdout and a candidate file cannot replace the separate observer.
	attack := `package main
import("fmt";"os")
func init(){
 if _,err:=os.ReadFile("/input/observer.go");err==nil{panic("observer filesystem leaked")}
 if err:=os.WriteFile("/input/observer.go",[]byte("forged"),0600);err==nil{panic("observer input writable")}
 os.WriteFile("/work/observation.json",[]byte("forged"),0600)
 fmt.Println("[{\"provider_calls\":[]}]")
}
`
	s, pair := captured(t, attack)
	p := prepared(t, s, pair, 2)
	preview, id := p.Preview()
	t.Logf("Approved synthetic runner plan %s: %s", id, preview)
	e := Executor{Docker: sandbox.Docker{Binary: os.Getenv("AFTER_DOCKER_BINARY"), Host: os.Getenv("AFTER_DOCKER_HOST")}}
	result, err := e.Run(t.Context(), s, p, id)
	if err != nil {
		for _, sample := range result.Samples {
			for _, a := range sample.Artifacts {
				b, _ := s.ReadBlob(a.Content)
				t.Logf("%s: %s", a.Channel, b)
			}
		}
		t.Fatal(err)
	}
	if result.Receipt.Completeness != evidence.Complete || result.Receipt.State.Comparison != evidence.NotCompared || len(result.Samples) != 8 {
		t.Fatalf("incomplete/unstable proof: %+v", result.Receipt)
	}
	for _, sample := range result.Samples {
		if !sample.Execution.App.Cleaned || !sample.Execution.Observer.Cleaned {
			t.Fatal("leaked resources")
		}
		var observed Observation
		for _, a := range sample.Artifacts {
			if strings.HasSuffix(a.Channel, "/observation") {
				b, e := s.ReadBlob(a.Content)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(b, &observed); e != nil {
					t.Fatal(e)
				}
			}
		}
		want := 1
		if sample.Side == "candidate" && sample.CaseSeconds == 43200 {
			want = 2
		}
		if len(observed.Calls) != want {
			t.Fatalf("%s %d: got %d calls want %d", sample.Side, sample.CaseSeconds, len(observed.Calls), want)
		}
		for _, r := range observed.Responses {
			if r.Status != 200 || r.Body != `{"payment":"synthetic-accepted"}` {
				t.Fatal("response changed", r)
			}
		}
		t.Logf("%s %ds repetition %d: %d received calls, identical responses", sample.Side, sample.CaseSeconds, sample.Repetition, len(observed.Calls))
	}
	if _, err := store.Get[evidence.Receipt](s, result.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"exited-parent", "refused-upstream", "real-502"} {
		t.Run(mode, func(t *testing.T) {
			source := `package main
import("fmt";"net";"net/http";"os";"os/exec";"io";"time")
func init(){
 mode:="` + mode + `"
 if mode=="exited-parent" && (len(os.Args)!=2 || os.Args[1]!="child") {
  r,w,_:=os.Pipe()
  c:=exec.Command("/work/app","child");c.ExtraFiles=[]*os.File{os.NewFile(3,"ready"),w}
  if err:=c.Start();err!=nil{panic(err)};w.Close();io.ReadFull(r,make([]byte,1));os.Exit(42)
 }
 l,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{panic(err)}
 address:=l.Addr().String()
 if mode=="refused-upstream"{l.Close()}
 ready:=os.NewFile(3,"ready");fmt.Fprintln(ready,"http://"+address);ready.Close()
 if mode=="exited-parent"{p:=os.NewFile(4,"parent");p.Write([]byte{1});p.Close()}
 if mode=="refused-upstream"{time.Sleep(time.Hour)}
 http.Serve(l,http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(502);fmt.Fprint(w,"application response")}))
}
`
			st, sp := captured(t, source)
			plan := prepared(t, st, sp, 1)
			_, approval := plan.Preview()
			got, runErr := e.Run(t.Context(), st, plan, approval)
			if mode == "real-502" {
				if runErr != nil || got.Receipt.Completeness != evidence.Complete {
					t.Fatalf("real app 502 lost: %v", runErr)
				}
				for _, sample := range got.Samples {
					if sample.Side != "candidate" {
						continue
					}
					for _, a := range sample.Artifacts {
						if strings.HasSuffix(a.Channel, "/observation") {
							b, _ := st.ReadBlob(a.Content)
							var o Observation
							json.Unmarshal(b, &o)
							for _, r := range o.Responses {
								if r.Status != 502 || r.Body != "application response" {
									t.Fatal("real response not preserved")
								}
							}
						}
					}
				}
			} else {
				if runErr == nil || got.Receipt.Completeness != evidence.Incomplete || got.Receipt.State.Kind == evidence.Observed {
					t.Fatalf("failure promoted: %+v %v", got.Receipt, runErr)
				}
				for _, sample := range got.Samples {
					if sample.Side == "candidate" && sample.Status == "completed" {
						t.Fatal("failed candidate completed")
					}
				}
			}
			for _, sample := range got.Samples {
				for _, r := range []sandbox.Result{sample.Execution.App, sample.Execution.Observer} {
					if r.Container != "" && !r.Cleaned {
						t.Fatal("descendant cleanup failed")
					}
				}
			}
		})
	}
}
