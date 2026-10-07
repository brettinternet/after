//go:build darwin || linux

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
	"github.com/creack/pty"
	"golang.org/x/term"
)

// Real PTY input drives the actual CLI over real capture/import storage. No
// golden screen or invented observed outcome stands in for engine evidence.
func TestBrowserPTY(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	makeProject(t, project)
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// hostile \x1b]52;c;bad\a\n")
	code, out, diagnostic := invoke([]string{"capture", "--project", project, "--json"}, false, "")
	if code != 0 {
		t.Fatal(diagnostic)
	}
	var captured struct {
		Data struct {
			Base      struct{ ID string } `json:"base_snapshot"`
			Candidate struct{ ID string } `json:"candidate_snapshot"`
		}
	}
	if err := json.Unmarshal([]byte(out), &captured); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.jsonl")
	if err := os.WriteFile(report, []byte("{\"Action\":\"pass\",\"Package\":\"pty-case\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic = invoke([]string{"import", report, "--producer", "PTY Go report", "--project", project, "--snapshot", captured.Data.Candidate.ID, "--json"}, false, "")
	if code != 0 {
		t.Fatal(diagnostic)
	}
	writeProjectFile(t, project, "app/main.go", "package main\nfunc main() {}\n// candidate changed after launch\n")
	var imported struct{ Data struct{ ID string } }
	if err := json.Unmarshal([]byte(out), &imported); err != nil {
		t.Fatal(err)
	}
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
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
	var transcript, unread strings.Builder
	expect := func(wants ...string) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			found := true
			for _, want := range wants {
				found = found && strings.Contains(unread.String(), want)
			}
			if found {
				unread.Reset()
				return
			}
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatal("PTY closed before", wants)
				}
				transcript.WriteString(chunk)
				unread.WriteString(chunk)
			case <-deadline:
				t.Fatalf("no %v in PTY: %q", wants, transcript.String())
			}
		}
	}
	send := func(keys string) {
		t.Helper()
		if _, err := master.Write([]byte(keys)); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(t.Context(), []string{"review", strings.ToUpper(captured.Data.Base.ID[7:19]), strings.ToUpper(captured.Data.Candidate.ID[7:19]), strings.ToUpper(imported.Data.ID[7:19]), "--project", project, "--import-file", report, "--producer", "PTY Go report", "--json"}, &stdout, slave, slave, true)
	}()
	expect("[REPORTED]")
	if !strings.Contains(transcript.String(), "1 Overview") {
		t.Fatal("overview tab missing from the initial PTY frame")
	}
	send("\r")
	expect("pty-case (package)")
	send("\x1b")
	expect("1 Overview")
	send("d")
	expect("2 Changes")
	send("\t")
	expect("captured raw diff")
	send("?")
	expect("Help")
	send("\x1b")
	expect("1 Overview")
	send("c")
	expect("u reviews it")
	if !strings.Contains(transcript.String(), "New capture ") {
		t.Fatal("capture completion ID missing from the PTY")
	}
	send("u")
	expect("Snapshot selected")
	send("i")
	expect("Imported report cards loaded without restart")
	send("1")
	expect("[STALE]", "[REPORTED]")
	send("s")
	expect("4 Activity", "SESSION", "import finished")
	if err := pty.Setsize(master, &pty.Winsize{Rows: 8, Cols: 32}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	expect("AFTER ·")
	send("q")
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quit blocked")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("terminal not restored", err)
	}
	var result struct {
		Kind string `json:"kind"`
		Data struct {
			Pair struct {
				Candidate string `json:"candidate"`
			} `json:"pair"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Kind != "review_session" || result.Data.Pair.Candidate == "" {
		t.Fatalf("--json review result missing from stdout: %q %v", stdout.String(), err)
	}
	records, err := store.Open(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := records.List("artifact")
	if err != nil {
		records.Close()
		t.Fatal(err)
	}
	boundToSelected := false
	for _, artifact := range artifacts {
		metadata, err := records.ReadArtifactMetadata(artifact.ID)
		if err != nil || metadata.Channel != "report" {
			continue
		}
		raw, err := records.ReadBlob(metadata.Content)
		if err != nil {
			continue
		}
		var importedReport gotestreport.Report
		if json.Unmarshal(raw, &importedReport) == nil && importedReport.Metadata.Producer == "PTY Go report" && string(importedReport.Metadata.Snapshot) == result.Data.Pair.Candidate {
			boundToSelected = true
		}
	}
	records.Close()
	if !boundToSelected {
		t.Fatal("live i import was not bound to the candidate selected before keypress")
	}
	slave.Close()
	for chunk := range chunks {
		transcript.WriteString(chunk)
	}
	for _, bad := range []string{"\x1b]52;", "\x1b]8;", "\u009b", "\u009d"} {
		if strings.Contains(transcript.String(), bad) {
			t.Fatalf("control injection %q", bad)
		}
	}
	for _, required := range []string{"\x1b[?1049h", "\x1b[?1049l", "\x1b[?25h"} {
		if !strings.Contains(transcript.String(), required) {
			t.Fatal("missing restoration", required)
		}
	}
	t.Log("PTY: review stored report -> help -> live capture/select -> keypress-bound import -> immediate reported cards -> Activity/session -> 32x8 resize -> quit; import candidate binding and terminal restoration verified, no payload controls")
}

func TestBrowserArgumentSafety(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	for _, args := range [][]string{
		{"review", id, "--tui", "--base", id},
		{"review", id, "--evidence", id},
		{"review", id, "--tui", "--base", id, "--accept", "--reason", "bad"},
	} {
		code, _, _ := invoke(args, false, "")
		if code != ExitInvalid {
			t.Fatalf("accepted %v", args)
		}
	}
}
