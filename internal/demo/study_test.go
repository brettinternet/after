package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAssignments(t *testing.T) {
	for _, n := range []int{6, 12, 16, 18} {
		rows, err := assignments("fixed-seed", n)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := assignments("fixed-seed", n)
		if !reflect.DeepEqual(rows, again) {
			t.Fatal("not reproducible")
		}
		periods := map[string]int{}
		pairs := map[string]int{}
		for _, a := range rows {
			cases, conditions := map[string]bool{}, map[string]bool{}
			for i, tr := range a.Trials {
				cases[tr.Case] = true
				conditions[tr.Condition] = true
				periods[string(rune('0'+i))+tr.Condition]++
				pairs[tr.Case+tr.Condition]++
			}
			if len(cases) != 3 || len(conditions) != 3 {
				t.Fatal("repeated case or condition")
			}
		}
		if n%6 == 0 {
			for _, counts := range []map[string]int{periods, pairs} {
				for _, v := range counts {
					if v != n/3 {
						t.Fatalf("unbalanced: %v", counts)
					}
				}
			}
		}
	}
	a, _ := assignments("one", 12)
	b, _ := assignments("two", 12)
	if reflect.DeepEqual(a, b) {
		t.Fatal("seed ignored")
	}
	if _, err := assignments("", 12); err == nil {
		t.Fatal("accepted empty seed")
	}
	if _, err := assignments("seed", 19); err == nil {
		t.Fatal("accepted excess slots")
	}
}

func TestStudySetup(t *testing.T) {
	t.Chdir("../..")
	binary := filepath.Join(t.TempDir(), "after")
	if out, err := exec.Command("go", "build", "-o", binary, "./cmd/after").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	for _, c := range studyCases {
		t.Run(c.ID, func(t *testing.T) {
			w, err := createWorkspace()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := w.cleanup(); err != nil {
					t.Error(err)
				}
			}()
			d := demo{root: w.root, project: filepath.Join(w.root, "payment"), binary: binary, env: []string{"PATH=/usr/bin:/bin", "HOME=" + w.root, "XDG_CONFIG_HOME=" + w.root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=demo", "GIT_AUTHOR_EMAIL=demo@example.invalid", "GIT_COMMITTER_NAME=demo", "GIT_COMMITTER_EMAIL=demo@example.invalid"}}
			if err = d.studyCase(c, false); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(w.root, "missing.json"))
			if err != nil {
				t.Fatal(err)
			}
			var phase studyPhase
			if err = json.Unmarshal(raw, &phase); err != nil || phase.Receipt != "" || phase.Pin != "" || phase.Capture.Candidate.ID == "" {
				t.Fatal("invalid missing phase", err)
			}
			raw, err = os.ReadFile(filepath.Join(w.root, "initial.diff"))
			if err != nil || !strings.Contains(string(raw), "config_test.go") || !strings.Contains(string(raw), c.Retention) {
				t.Fatal("missing candidate/oracle diff", err)
			}
			checkpoint := d
			checkpoint.project = filepath.Join(w.root, "missing-project")
			if err = checkpoint.cli(0, nil, "inspect", string(phase.Capture.Candidate.ID), "--base", string(phase.Capture.Base.ID)); err != nil {
				t.Fatal("copied store not inspectable", err)
			}
			if err = d.studyEdit(c); err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(filepath.Join(d.project, "app/config.go"))
			if err != nil {
				t.Fatal(err)
			}
			want := c.Retention
			if c.Followup == "repair" {
				want = "24 * 60 * 60"
			}
			if !strings.Contains(string(config), want) {
				t.Fatal("wrong follow-up")
			}
			// The missing checkpoint must remain unchanged by follow-up edits.
			frozen, err := os.ReadFile(filepath.Join(w.root, "missing-project", "app/config.go"))
			if err != nil || !strings.Contains(string(frozen), c.Retention) {
				t.Fatal("checkpoint changed", err)
			}
		})
	}
}
