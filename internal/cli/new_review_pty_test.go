//go:build darwin || linux

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/store"
)

func TestNewReviewPTYAndSuggestions(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("color-%t", noColor), func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("NO_COLOR", "")
			if noColor {
				t.Setenv("NO_COLOR", "1")
			}
			project := filepath.Join(t.TempDir(), "project")
			home := t.TempDir()
			makeProject(t, project)
			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(1) }\n")
			launch := func(args ...string) (evidence.SnapshotPair, string) {
				t.Helper()
				args = append(args, "--project", project, "--json")
				p := startLoopPTYSize(t, args, 80, 24)
				p.expect("AFTER   project")
				p.expect("Diff   Activity")
				selected, _ := p.finish()
				return selected.Pair, p.transcript.String()
			}
			first, transcript := launch("review", "--new")
			if strings.Contains(transcript, "Replaced saved review") {
				t.Fatal("replaced a nonexistent review")
			}
			receipt, _ := seedCLIProcessRecords(t, project, string(first.Base), string(first.Candidate))
			s, err := store.Open(project, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			pin, err := review.Create(s, receipt, "keep the old review expectation", evidence.HumanIntent, "synthetic test")
			if err != nil {
				t.Fatal(err)
			}
			before, err := s.List()
			if err != nil {
				t.Fatal(err)
			}
			s.Close()

			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(2) }\n")
			gitRun(t, project, "add", "app/main.go")
			writeProjectFile(t, project, "app/main.go", "package main\nfunc main() { println(3) }\n")
			second, transcript := launch("review", "--new", "--staged")
			if first == second || !strings.Contains(transcript, "Replaced saved review "+shortID(first.Base)+" → "+shortID(first.Candidate)) {
				t.Fatalf("new review did not replace/name old pair: %s", transcript)
			}
			saved, found, invalid, err := readReviewSession(project)
			if err != nil || !found || invalid || saved.Pair != second || !saved.Capture.Staged {
				t.Fatalf("wrong saved review: %+v %v", saved, err)
			}
			s, err = store.Open(project, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			after, err := s.List()
			if err != nil {
				t.Fatal(err)
			}
			entries := map[store.Entry]bool{}
			for _, entry := range after {
				entries[entry] = true
			}
			for _, entry := range before {
				if !entries[entry] {
					t.Fatalf("lost stored record: %+v", entry)
				}
			}
			s.Close()
			// Activity displays exactly which immutable records discovery loaded.
			p := startLoopPTYSize(t, []string{"review", "--project", project, "--json"}, 80, 24)
			p.expect("AFTER   project")
			p.expect("keep the old review expectation")
			p.send("s")
			p.expect(shortID(pin.ID))
			p.finish()

			// Capture a third pair while the staged review remains selected.
			code, captureText, diagnostic := invoke([]string{"capture", "--project", project}, false, "")
			if code != 0 {
				t.Fatal(diagnostic)
			}
			// A different saved review is replaced explicitly, never resumed by surprise.
			if firstNextCommand(t, captureText) != "after review --new --project "+project {
				t.Fatal(captureText)
			}
			s, err = store.Open(project, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			latest, err := newestCaptureForStore(s)
			s.Close()
			if err != nil {
				t.Fatal(err)
			}
			third := evidence.SnapshotPair{Base: latest.Base, Candidate: latest.Candidate}
			if third == second {
				t.Fatal("staged and working tree captures matched")
			}
			for _, terminal := range []bool{false, true} {
				run := runCLIPipe
				if terminal {
					run = runCLIPTY
				}
				for _, args := range [][]string{{"status"}, {"status", "--json"}, {"inspect", string(first.Base), string(first.Candidate)}, {"capture"}} {
					code, out, stderr, err := run(project, home, noColor, args)
					if code != 0 || err != nil || stderr != "" {
						t.Fatalf("%v: %d %s %s %v", args, code, out, stderr, err)
					}
					if args[0] == "status" {
						if len(args) == 1 {
							if !strings.Contains(out, "Saved review") || !strings.Contains(out, shortID(second.Candidate)) || !strings.Contains(out, "after review --new") {
								t.Fatal(out)
							}
						} else {
							var result struct{ Data statusView }
							if err := json.Unmarshal([]byte(out), &result); err != nil || result.Data.SavedReview == nil || *result.Data.SavedReview != second {
								t.Fatalf("saved JSON: %s %v", out, err)
							}
						}
					}
				}
			}
			for _, pair := range []evidence.SnapshotPair{first, third, second} {
				code, out, diagnostic := invoke([]string{"inspect", string(pair.Base), string(pair.Candidate), "--project", project}, false, "")
				if code != 0 {
					t.Fatal(diagnostic)
				}
				command := firstNextCommand(t, out)
				want := "after review"
				if pair == first {
					want += " " + shortID(pair.Base) + " " + shortID(pair.Candidate)
				}
				want += " --project " + project
				if command != want {
					t.Fatalf("suggestion=%q want=%q", command, want)
				}
				p := startLoopPTYSize(t, append(strings.Fields(strings.TrimPrefix(command, "after ")), "--json"), 80, 24)
				p.expect("AFTER   project")
				selected, _ := p.finish()
				expected := second // Plain review resumes, never silently switches to newest.
				if pair == first {
					expected = first
				}
				if selected.Pair != expected {
					t.Fatalf("suggestion opened %+v, want %+v", selected.Pair, expected)
				}
			}
			// Execute the start-over suggestion too: it captures working tree by default.
			replacement, _ := launch("review", "--new")
			if replacement != third {
				t.Fatalf("start over opened %+v want %+v", replacement, third)
			}
			for _, args := range [][]string{{"review", "--new"}, {"review", "--new", "--staged"}} {
				code, out, stderr, err := runCLIPipe(project, home, noColor, args)
				if code != ExitInvalid || out != "" || err != nil || !strings.Contains(stderr, "after status --json") {
					t.Fatalf("pipe review: %d %q %q %v", code, out, stderr, err)
				}
			}
			t.Log("80-column terminal and pipe: Saved review; after review; after review --new; replaced pair named; prior pin discovered; historical explicit-pair suggestion executed")
		})
	}
}
