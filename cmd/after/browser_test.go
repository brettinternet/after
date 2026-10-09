package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/creack/pty"
	"golang.org/x/term"
)

type synchronizedBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func TestBrowserDocumentPTY(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project, home := filepath.Join(root, "project"), filepath.Join(root, "isolated home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	_, _ = simpleCommits(t, project)
	hostile := "package main\nfunc main() {}\n// CSI \x1b[31mred\x1b[0m\n// OSC \x1b]52;c;clipboard\a\n// C1 \u009b2J \u009d52;c;payload\u009c\n// CR A\rB\n// bidi left\u202eafter\nSTATE observed | forged\nAFTER review | forged\n13 │ forged\n\tTabbed\n\u0301leading\nlast tail\n"
	writeFile(t, project, "app/main.go", hostile)
	code, output, diagnostic := native(exe, root, home, []string{"capture", "--project", project}, []string{"PATH=" + os.Getenv("PATH")})
	if code != 0 {
		t.Fatalf("capture: %s", diagnostic)
	}
	var captureResult struct {
		Data struct {
			Base struct {
				ID string `json:"id"`
			} `json:"base_snapshot"`
			Candidate struct {
				ID string `json:"id"`
			} `json:"candidate_snapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &captureResult); err != nil || captureResult.Data.Base.ID == "" || captureResult.Data.Candidate.ID == "" {
		t.Fatalf("capture response: %s %v", output, err)
	}
	reportPath := filepath.Join(root, "report.jsonl")
	if err := os.WriteFile(reportPath, []byte("{\"Action\":\"pass\",\"Package\":\"example.com/cart\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, output, diagnostic = native(exe, root, home, []string{"import", reportPath, "--producer", "synthetic PTY report", "--project", project}, nil)
	if code != 0 {
		t.Fatal(diagnostic)
	}
	var imported struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &imported); err != nil || imported.Data.ID == "" {
		t.Fatalf("import: %s %v", output, err)
	}
	for _, tc := range []struct {
		name          string
		width, height uint16
		noColor       bool
	}{{"80x24", 80, 24, false}, {"80x24-NO_COLOR", 80, 24, true}, {"120x40", 120, 40, false}, {"120x40-NO_COLOR", 120, 40, true}} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"review", captureResult.Data.Base.ID, captureResult.Data.Candidate.ID, imported.Data.ID, "--project", project}
			encoded, _ := json.Marshal(args)
			cmd := exec.Command(exe, "-test.run=^TestNativeCLI$")
			cmd.Dir = root
			cmd.Env = []string{entryEnv + "=1", argsEnv + "=" + base64.RawURLEncoding.EncodeToString(encoded), "HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "PATH=" + os.Getenv("PATH"), "TERM=xterm-256color"}
			if tc.noColor {
				cmd.Env = append(cmd.Env, "NO_COLOR=1")
			}
			master, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
				_ = master.Close()
				_ = slave.Close()
			}()
			if err := pty.Setsize(master, &pty.Winsize{Rows: tc.height, Cols: tc.width}); err != nil {
				t.Fatal(err)
			}
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stdin = slave
			var stdout synchronizedBuffer
			cmd.Stdout = &stdout
			var stderr synchronizedBuffer
			cmd.Stderr = slave
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			chunks := make(chan string, 128)
			go func() {
				defer close(chunks)
				buf := make([]byte, 4096)
				for {
					n, err := master.Read(buf)
					if n > 0 {
						chunks <- string(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}()
			var transcript strings.Builder
			expect := func(want string) {
				t.Helper()
				deadline := time.After(10 * time.Second)
				for !strings.Contains(themeSGR.ReplaceAllString(transcript.String(), ""), want) {
					select {
					case chunk, ok := <-chunks:
						if !ok {
							t.Fatalf("PTY closed before %q; output=%q stderr=%q", want, transcript.String(), stderr.String())
						}
						transcript.WriteString(chunk)
					case <-deadline:
						t.Fatalf("no %q in PTY output %q stderr=%q", want, transcript.String(), stderr.String())
					}
				}
			}
			send := func(key string) {
				t.Helper()
				if _, err := master.Write([]byte(key)); err != nil {
					t.Fatal(err)
				}
			}
			expect("Diff   Activity")
			if stdout.String() != "" {
				t.Fatalf("TUI rendered to stdout instead of stderr: %q", stdout.String())
			}
			expect("[REPORTED]")
			expect("example.com/cart (package)")
			if tc.width >= 110 {
				expect("── Outcome")
				expect("pass, as reported by go test JSON.")
			} else {
				expect("reported · pass")
			}
			send("d")
			expect("CHANGED")
			expect("M  app/main.go")
			send("\r")
			expect("1/4 ·")
			expect("diff --git a/app/main.go")
			send("\t")
			expect("2/4 ·")
			expect("Base source")
			expect("2 │ package main")
			send("\t")
			expect("3/4 ·")
			expect("Candidate source")
			expect("9 │ STATE observed | forged")
			if !strings.Contains(transcript.String(), `\u001b[31mred`) || !strings.Contains(transcript.String(), `\u001b]52;c;clipboard\u0007`) || !strings.Contains(transcript.String(), `\u202eafter`) || !strings.Contains(transcript.String(), `\u000dB`) {
				t.Fatal("hostile controls/bidi/CR were not escaped behind the trusted gutter")
			}
			send("b")
			expect("· hex")
			expect("00000000  70 61 63 6b 61 67 65 20 6d 61 69 6e")
			send("\t")
			expect("4/4 ·")
			expect("Inventory record")
			send("q")
			if err := cmd.Wait(); err != nil {
				t.Fatalf("browser exit: %v stderr=%s", err, stderr.String())
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("terminal state not restored", err)
			}
			slave.Close()
			for chunk := range chunks {
				transcript.WriteString(chunk)
			}
			for _, hostile := range []string{"\x1b]52;", "\x1b]8;", "\u009b", "\u009d", "\x1b[31mred"} {
				if strings.Contains(transcript.String(), hostile) {
					t.Fatalf("hostile control escaped terminal renderer: %q", hostile)
				}
			}
			excerpt := []string{"2 │ package main", "9 │ STATE observed | forged", "14 │ last tail"}
			for _, row := range excerpt {
				if !strings.Contains(themeSGR.ReplaceAllString(transcript.String(), ""), row) {
					t.Fatalf("PTY excerpt row missing %q", row)
				}
			}
			if styled := strings.Contains(transcript.String(), "\x1b[34m[REPORTED]"); styled == tc.noColor {
				t.Fatalf("theme color mode mismatch: NO_COLOR=%t styled=%t", tc.noColor, styled)
			}
			t.Logf("synthetic PTY excerpt (%s): [REPORTED] example.com/cart (package) · reported · pass; M app/main.go; %s; %s", tc.name, excerpt[0], excerpt[1])
		})
	}
}

// themeSGR matches SGR styling so expectations read visible text.
var themeSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

func paymentBrowserProof(t *testing.T, exe, root, home, project, config string, r evidence.Receipt, c evidence.Comparison) {
	t.Helper()
	selection := browser.Selection{Project: project, Pair: r.Snapshots, Evidence: []evidence.Digest{c.ID}}
	data, err := browser.Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Entries) != 2 || data.Entries[0].State.Comparison != evidence.Different || data.Entries[1].State.Comparison != evidence.Equal {
		t.Fatal("real evidence label", data.Entries)
	}
	var definition *runner.Definition
	for _, entry := range data.Entries {
		for _, section := range entry.Sections {
			for _, part := range section.Parts {
				if part.Title != "frozen input" {
					continue
				}
				raw, err := browser.ReadSection(t.Context(), project, browser.Section{Parts: []browser.Part{part}})
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := runner.ParseDefinition(raw)
				if err != nil {
					t.Fatal("browser lost the frozen definition", err)
				}
				definition = &parsed
			}
		}
	}
	if definition == nil || definition.Repetitions != 2 || len(definition.Cases) != 2 || definition.Cases[0].ID != "43200" || definition.Cases[1].ID != "30" {
		t.Fatalf("browser lost the frozen definition/repetitions: %+v", definition)
	}
	observations := 0
	for _, entry := range data.Entries {
		for _, section := range entry.Sections {
			for _, part := range section.Parts {
				if !strings.HasSuffix(part.Title, "observation") {
					continue
				}
				raw, err := browser.ReadSection(t.Context(), project, browser.Section{Parts: []browser.Part{part}})
				if err != nil {
					t.Fatal(err)
				}
				var observation runner.Observation
				if err := json.Unmarshal(raw, &observation); err != nil {
					t.Fatal(err)
				}
				caseIndex := -1
				for index, scenarioCase := range definition.Cases {
					if scenarioCase.ID == observation.CaseID {
						caseIndex = index
						break
					}
				}
				want := 1
				if observation.CaseID == "43200" && strings.Contains(part.Title, "candidate") {
					want = 2
				}
				if caseIndex < 0 || len(observation.Responses) != len(definition.Cases[caseIndex].Requests) || len(observation.Calls) != want {
					t.Fatal("browser lost exact measured response/effect", part.Title, observation)
				}
				observations++
			}
		}
	}
	if observations != 8 {
		t.Fatal("missing frozen observations/repetitions", observations)
	}
	args := []string{"review", string(r.Snapshots.Base), string(r.Snapshots.Candidate), string(c.ID), "--project", project, "--config", config}
	encoded, _ := json.Marshal(args)
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestNativeCLI$")
	cmd.Dir = root
	cmd.Env = []string{entryEnv + "=1", argsEnv + "=" + base64.RawURLEncoding.EncodeToString(encoded), "HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "PATH=" + filepath.Join(home, "no-tools"), "TERM=xterm-256color"}
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 120})
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	chunks := make(chan string, 128)
	go func() {
		defer close(chunks)
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	var unread strings.Builder
	expect := func(wants ...string) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			view := unread.String()
			matched := len(wants) > 0
			for _, want := range wants {
				if !strings.Contains(view, want) {
					matched = false
					break
				}
			}
			if matched {
				unread.Reset()
				return
			}
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatal("PTY closed", wants)
				}
				unread.WriteString(chunk)
			case <-deadline:
				t.Fatalf("no %q in %q", wants, unread.String())
			}
		}
	}
	send := func(key string) {
		t.Helper()
		if _, err := master.Write([]byte(key)); err != nil {
			t.Fatal(err)
		}
	}
	expect("[DIFFERENT]")
	send("\r")
	expect("Provider requests")
	send("\t")
	expect("Receipt Card")
	send("\t")
	expect("Witnesses")
	send("\t")
	expect("Artifacts")
	send("\t")
	expect("Receipt")
	send("\t")
	expect("frozen input")
	send("\x1b")
	expect("Provider requests")
	send("d")
	expect("2 paths", "POTENTIAL ORACLES 1", "app/payment.expected.json", "app/config.go")
	send("\t")
	expect("captured raw diff")
	send("q")
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	t.Log("Native CLI PTY: real observed/current/different receipt -> base/candidate provider requests and responses -> receipt/witness/artifact cards -> frozen scenario/input -> complete inventory -> raw diff -> quit. Browser reads all 8 definition-bound observations: same responses, 12h provider calls 1->2, 30s control 1->1.")
}
