package main

import (
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/creack/pty"
)

func paymentBrowserProof(t *testing.T, exe, root, home, project, config string, r evidence.Receipt, c evidence.Comparison) {
	t.Helper()
	selection := browser.Selection{Project: project, Pair: r.Snapshots, Evidence: []evidence.Digest{c.ID}}
	data, err := browser.Load(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Entries) != 2 || !strings.Contains(data.Entries[0].Label, "observed | current | completed | different") {
		t.Fatal("real evidence label", data.Entries)
	}
	observations := 0
	input := false
	for _, section := range data.Entries[0].Sections {
		if section.Name != "exact frozen input" && !strings.HasSuffix(section.Name, "/observation") {
			continue
		}
		page, err := browser.ReadPage(t.Context(), project, section, 0)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total > len(page.Bytes) {
			t.Fatal("unexpected large fixture")
		}
		if section.Name == "exact frozen input" {
			input = strings.Contains(string(page.Bytes), `"seconds":[43200,30]`)
			continue
		}
		var o runner.Observation
		if err := json.Unmarshal(page.Bytes, &o); err != nil {
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
	args := []string{"review", string(r.Snapshots.Candidate), "--tui", "--base", string(r.Snapshots.Base), "--evidence", string(c.ID), "--project", project, "--config", config}
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
	expect("STATE observed | current | completed | different")
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
