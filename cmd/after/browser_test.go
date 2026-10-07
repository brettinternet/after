package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
				for !strings.Contains(transcript.String(), want) {
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
			expect("1 Overview")
			if stdout.String() != "" {
				t.Fatalf("TUI rendered to stdout instead of stderr: %q", stdout.String())
			}
			expect("[REPORTED]")
			expect("example.com/cart (package)")
			expect("reported · pass")
			send("d")
			expect("2 Changes")
			expect("M  app/main.go")
			send("\r")
			expect("Section 1/4")
			expect("diff --git a/app/main.go")
			send("\t")
			expect("Section 2/4")
			expect("Base source")
			expect("1 │ package main")
			send("\t")
			expect("Section 3/4")
			expect("Candidate source")
			expect("8 │ STATE observed | forged")
			if !strings.Contains(transcript.String(), `\u001b[31mred`) || !strings.Contains(transcript.String(), `\u001b]52;c;clipboard\u0007`) || !strings.Contains(transcript.String(), `\u202eafter`) || !strings.Contains(transcript.String(), `\u000dB`) {
				t.Fatal("hostile controls/bidi/CR were not escaped behind the trusted gutter")
			}
			send("b")
			expect("hex | pan 0")
			expect("00000000  70 61 63 6b 61 67 65 20 6d 61 69 6e")
			send("\t")
			expect("Section 4/4")
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
			for _, hostile := range []string{"\x1b]52;", "\x1b]8;", "\u009b", "\u009d", "\x1b[31m"} {
				if strings.Contains(transcript.String(), hostile) {
					t.Fatalf("hostile control escaped terminal renderer: %q", hostile)
				}
			}
			excerpt := []string{"1 │ package main", "8 │ STATE observed | forged", "13 │ last tail"}
			for _, row := range excerpt {
				if !strings.Contains(transcript.String(), row) {
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
	observations := 0
	input := false
	for _, section := range data.Entries[0].Sections {
		if section.Name != "exact frozen input" && !strings.HasSuffix(section.Name, "/observation") {
			continue
		}
		raw, err := browser.ReadSection(t.Context(), project, section)
		if err != nil {
			t.Fatal(err)
		}
		if section.Name == "exact frozen input" {
			input = strings.Contains(string(raw), `"seconds":[43200,30]`)
			continue
		}
		var o runner.Observation
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		want := 1
		if strings.Contains(section.Name, "candidate/43200/") {
			want = 2
		}
		if len(o.Responses) != 2 || len(o.Calls) != want {
			t.Fatal("browser lost exact measured response/effect", section.Name, o)
		}
		observations++
	}
	if !input || observations != 8 {
		t.Fatal("missing frozen input/repetitions", input, observations)
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
	expect := func(want string) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			if strings.Contains(unread.String(), want) {
				unread.Reset()
				return
			}
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatal("PTY closed", want)
				}
				unread.WriteString(chunk)
			case <-deadline:
				t.Fatalf("no %q in %q", want, unread.String())
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
	expect("measured provider-request counts")
	send("\t")
	expect("receipt: producer, bindings")
	send("\t")
	expect("frozen scenario")
	send("\t")
	expect("exact frozen input")
	send("d")
	expect("AFTER review | inventory")
	send("\t")
	expect("captured raw diff")
	send("q")
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	t.Log("Native CLI PTY: real observed/current/different receipt -> producer/bindings/timestamps -> frozen scenario/input -> complete inventory -> raw diff -> quit. Browser reads all 8 measured observations: same responses, 12h provider calls 1->2, 30s control 1->1.")
}
