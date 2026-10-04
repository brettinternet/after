// Command demo walks the native CLI over an owned, synthetic repository only.
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
)

type workspace struct {
	root, token string
	info        os.FileInfo
}

func createWorkspace() (*workspace, error) {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(tmp, "after-demo-")
	if err != nil {
		return nil, err
	}
	w := &workspace{root: root, token: rand.Text()}
	w.info, err = os.Lstat(root)
	if err != nil {
		return nil, err
	}
	return w, os.WriteFile(filepath.Join(root, ".owner"), []byte(w.token), 0600)
}
func (w *workspace) cleanup() error {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	info, err := os.Lstat(w.root)
	if err != nil {
		return err
	}
	marker, err := os.ReadFile(filepath.Join(w.root, ".owner"))
	if err != nil {
		return err
	}
	if filepath.Dir(w.root) != tmp || !strings.HasPrefix(filepath.Base(w.root), "after-demo-") || !info.IsDir() || !os.SameFile(info, w.info) || string(marker) != w.token {
		return errors.New("demo ownership changed; retained workspace")
	}
	return os.RemoveAll(w.root)
}
func put(root, name string, data []byte) error {
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

type demo struct {
	root, project, binary string
	env                   []string
	input                 *bufio.Reader
	proof                 bool
}

// Bound child output just as the CLI bounds its JSON envelope.
type bounded struct{ bytes.Buffer }

func (b *bounded) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 16<<20 {
		return 0, errors.New("demo output limit")
	}
	return b.Buffer.Write(p)
}
func (d *demo) command(binary string, args ...string) ([]byte, int, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = d.project
	cmd.Env = d.env
	var out, diagnostic bounded
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return nil, -1, err
		}
		code = exit.ExitCode()
	}
	if diagnostic.Len() != 0 {
		return out.Bytes(), code, fmt.Errorf("command diagnostics: %q", diagnostic.String())
	}
	return out.Bytes(), code, nil
}
func (d *demo) cli(want int, value any, args ...string) error {
	fmt.Printf("after %s\n", strings.Join(args, " "))
	raw, code, err := d.command(d.binary, append(args, "--project", d.project)...)
	if err != nil {
		return err
	}
	if code != want {
		return fmt.Errorf("%s exited %d (wanted %d); output %s; see docs/DEMO.md recovery", args[0], code, want, raw)
	}
	if err = put(d.root, fmt.Sprintf("step-%02d.json", step), raw); err != nil {
		return err
	}
	step++
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if value != nil {
		return json.Unmarshal(envelope.Data, value)
	}
	return nil
}

var step int

func (d *demo) setup() error {
	if err := os.MkdirAll(d.project, 0700); err != nil {
		return err
	}
	for _, name := range []string{"go.mod", "app/main.go", "app/config.go", "driver/main.go"} {
		data, err := os.ReadFile(filepath.Join("internal/paymentfixture/testdata/payment", name))
		if err != nil {
			return err
		}
		if err = put(d.project, name, data); err != nil {
			return err
		}
	}
	// No recursive copying, caller repository operations, global Git config or hooks.
	for _, args := range [][]string{{"init", "-q", "--template=", "-b", "main"}, {"add", "go.mod", "app", "driver"}, {"commit", "-qm", "synthetic base"}} {
		if _, code, err := d.command("/usr/bin/git", args...); err != nil || code != 0 {
			return fmt.Errorf("Git setup failed (install /usr/bin/git): %d %w", code, err)
		}
	}
	return d.edit(false)
}
func (d *demo) edit(restore bool) error {
	data, err := os.ReadFile("internal/paymentfixture/testdata/payment/app/config.go")
	if err != nil {
		return err
	}
	if !restore {
		if strings.Count(string(data), "24 * 60 * 60") != 1 {
			return errors.New("fixture mutation anchor changed")
		}
		data = []byte(strings.Replace(string(data), "24 * 60 * 60", "5 * 60", 1))
	} else {
		data = append(data, []byte("\n// Demo repair restores retention; captured as a new snapshot.\n")...)
	}
	return put(d.project, "app/config.go", data)
}

type snapshot struct {
	ID evidence.Digest `json:"id"`
}
type capture struct {
	Base      snapshot `json:"base_snapshot"`
	Candidate snapshot `json:"candidate_snapshot"`
}
type execution struct {
	Status  string           `json:"status"`
	Receipt evidence.Receipt `json:"receipt"`
	Samples []runner.Sample  `json:"samples"`
}

func (d *demo) execute(c capture, name string, finding bool) (execution, error) {
	var result execution
	var preview struct {
		Digest string          `json:"authorization_digest"`
		Plan   json.RawMessage `json:"plan"`
	}
	plan := filepath.Join(d.root, name+"-plan.json")
	if err := d.cli(3, &preview, "run", string(c.Base.ID), string(c.Candidate.ID), "--plan-out", plan); err != nil {
		return result, err
	}
	// Escaping JSON strings keeps untrusted control characters out of the terminal.
	fmt.Printf("Exact synthetic execution plan %s:\n%s\n", preview.Digest, preview.Plan)
	if !d.proof {
		fmt.Print("Type the exact digest above to authorize only this plan: ")
		line, err := d.input.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != preview.Digest {
			return result, errors.New("execution declined; no Docker call authorized")
		}
	} else {
		fmt.Println("task demo:proof authorizes this synthetic plan only")
	}
	code := 0
	if finding {
		code = 4
	}
	err := d.cli(code, &result, "run", "--plan-file", plan, "--approve", preview.Digest)
	if err != nil {
		return result, err
	}
	if result.Status != "completed" || result.Receipt.State.Kind != evidence.Observed || result.Receipt.State.Execution != evidence.Completed || len(result.Samples) != 4 {
		return result, errors.New("incomplete observed demo receipt")
	}
	observations := map[string]runner.Observation{}
	for _, sample := range result.Samples {
		if sample.Status != "completed" || !sample.Execution.App.Cleaned || !sample.Execution.Observer.Cleaned {
			return result, errors.New("sample incomplete or cleanup failed")
		}
		for _, artifact := range sample.Artifacts {
			if strings.HasSuffix(artifact.Channel, "/observation") {
				var page struct {
					Document runner.Observation `json:"document"`
				}
				if err = d.cli(0, &page, "inspect", string(artifact.Content)); err != nil {
					return result, err
				}
				observations[fmt.Sprintf("%s/%d", sample.Side, sample.CaseSeconds)] = page.Document
			}
		}
	}
	for _, seconds := range []int{43200, 30} {
		base, candidate := observations[fmt.Sprintf("base/%d", seconds)], observations[fmt.Sprintf("candidate/%d", seconds)]
		count := 1
		if finding && seconds == 43200 {
			count = 2
		}
		if len(base.Responses) != 2 || !reflect.DeepEqual(base.Responses, candidate.Responses) || len(base.Calls) != 1 || len(candidate.Calls) != count {
			return result, errors.New("actual observer response/count assertion failed")
		}
		fmt.Printf("Observed %ds: identical responses; provider requests %d -> %d\n", seconds, len(base.Calls), len(candidate.Calls))
	}
	if err := writeJSON(d.root, name+"-observations.json", observations); err != nil {
		return result, err
	}
	return result, nil
}
func (d *demo) walk(execute bool) error {
	if err := d.setup(); err != nil {
		return err
	}
	var c capture
	if err := d.cli(0, &c, "capture"); err != nil {
		return err
	}
	if err := d.cli(0, nil, "inspect", string(c.Candidate.ID), "--base", string(c.Base.ID)); err != nil {
		return err
	}
	report, err := os.ReadFile("internal/gotestreport/testdata/tests.jsonl")
	if err != nil {
		return err
	}
	reportPath := filepath.Join(d.root, "report.jsonl")
	if err = put(d.root, "report.jsonl", report); err != nil {
		return err
	}
	var imported struct {
		ID    string `json:"id"`
		Cards []struct {
			State evidence.EvidenceState `json:"state"`
		} `json:"cards"`
	}
	if err = d.cli(0, &imported, "import", reportPath, "--producer", "checked-in synthetic Go report; not executed by this demo"); err != nil {
		return err
	}
	if len(imported.Cards) == 0 {
		return errors.New("missing imported cards")
	}
	for _, card := range imported.Cards {
		if card.State.Kind != evidence.Reported || card.State.Execution != evidence.NotRun || card.State.Applicability != evidence.Unknown {
			return errors.New("import promoted evidence")
		}
	}
	if err = d.cli(0, nil, "inspect", imported.ID); err != nil {
		return err
	}
	fmt.Println("Imported report: reported / unknown / not_run. Raw diff remains available; no payment observation yet.")
	if !execute {
		return nil
	}
	first, err := d.execute(c, "initial", true)
	if err != nil {
		return err
	}
	var pinned review.View
	if err = d.cli(0, &pinned, "pin", string(first.Receipt.ID), "--scope", "finite_example", "--expectation", "Twelve-hour retries should make one provider request", "--reason", "synthetic demo pin, not human validation"); err != nil {
		return err
	}
	if err = d.edit(true); err != nil {
		return err
	}
	if err = d.cli(0, &c, "capture"); err != nil {
		return err
	}
	var reopened review.View
	if err = d.cli(0, &reopened, "review", string(pinned.Pin.ID), "--select", string(c.Candidate.ID), "--mode", "original_base", "--reason", "demo repair changes captured basis"); err != nil {
		return err
	}
	if !reopened.MissingCurrentResult || reopened.CurrentReceipt != nil || reopened.Applicability != evidence.Stale {
		return errors.New("selection did not reopen without results")
	}
	fmt.Println("Pin reopened: stale; missing current result. No invented observation.")
	// Capture records include their paired diff: even unchanged base content can
	// have a new record ID. Rerun the pin's selected pair, not capture's new base.
	c.Base.ID = reopened.Pin.History[len(reopened.Pin.History)-1].Review.Target.Snapshots.Base
	second, err := d.execute(c, "rerun", false)
	if err != nil {
		return err
	}
	var attached review.View
	if err = d.cli(0, &attached, "review", string(reopened.Pin.ID), "--receipt", string(second.Receipt.ID), "--reason", "attach real authorized rerun; not acceptance"); err != nil {
		return err
	}
	if attached.MissingCurrentResult || attached.Applicability != evidence.Current {
		return errors.New("rerun not current")
	}
	fmt.Printf("Rerun attached, not human-accepted. Pin revision: %s\n", attached.Pin.ID)
	return nil
}
func run() (err error) {
	execute := flag.Bool("execute", false, "walk both offline payment runs, requiring exact-plan consent")
	keep := flag.Bool("keep", false, "retain the owned private workspace and receipts for inspection")
	study := flag.Bool("study", false, "generate study v1 cases (author rehearsal, not human research)")
	binary := flag.String("binary", "bin/after", "native AFTER binary")
	assignmentSeed := flag.String("assign-seed", "", "print reproducible anonymous study assignments; no workspace or execution")
	participants := flag.Int("participants", 12, "anonymous assignment slots (1–18)")
	flag.Parse()
	if *assignmentSeed != "" {
		rows, err := assignments(*assignmentSeed, *participants)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(rows)
	}
	if flag.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	exe, err := filepath.Abs(*binary)
	if err != nil {
		return err
	}
	if _, err = os.Stat(exe); err != nil {
		return errors.New("native binary missing; run task build")
	}
	if *execute && (!filepath.IsAbs(os.Getenv("AFTER_DOCKER_BINARY")) || !strings.HasPrefix(os.Getenv("AFTER_DOCKER_HOST"), "unix:///")) {
		return errors.New("set absolute AFTER_DOCKER_BINARY and local unix:/// AFTER_DOCKER_HOST; explicitly provision pinned image first; see docs/DEMO.md")
	}
	w, err := createWorkspace()
	if err != nil {
		return err
	}
	defer func() {
		if *keep || err != nil {
			fmt.Printf("Retained private workspace: %s (see docs/DEMO.md)\n", w.root)
			return
		}
		err = errors.Join(err, w.cleanup())
		if err == nil {
			fmt.Println("Owned demo workspace cleaned; unrelated resources untouched.")
		}
	}()
	fmt.Printf("Owned synthetic workspace: %s\n", w.root)
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + w.root, "XDG_CONFIG_HOME=" + filepath.Join(w.root, "config"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=AFTER demo", "GIT_AUTHOR_EMAIL=demo@example.invalid", "GIT_COMMITTER_NAME=AFTER demo", "GIT_COMMITTER_EMAIL=demo@example.invalid", "AFTER_INTERACTIVE=false"}
	if *execute {
		env = append(env, "AFTER_DOCKER_BINARY="+os.Getenv("AFTER_DOCKER_BINARY"), "AFTER_DOCKER_HOST="+os.Getenv("AFTER_DOCKER_HOST"))
	}
	d := demo{root: w.root, project: filepath.Join(w.root, "payment"), binary: exe, env: env, input: bufio.NewReader(io.LimitReader(os.Stdin, 1024)), proof: os.Getenv("AFTER_DEMO_PROOF") == "1"}
	if *study {
		return d.study(*execute)
	}
	return d.walk(*execute)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "DEMO FAILED:", err)
		os.Exit(1)
	}
}
