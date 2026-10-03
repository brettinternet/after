package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		code     int
		contains string
	}{
		{nil, 0, "No evidence loaded"},
		{[]string{"help"}, 0, "foundation only"},
		{[]string{"--help"}, 0, "Exit status:"},
		{[]string{"-h"}, 0, "not implemented"},
		{[]string{"version"}, 0, "after " + Version},
		{[]string{"--version"}, 0, "after " + Version},
		{[]string{"capture"}, 2, ""},
		{[]string{"--version", "extra"}, 2, ""},
		{[]string{"--"}, 2, ""},
		{[]string{""}, 2, ""},
		{[]string{"\x1b]52;c;bad\a"}, 2, ""},
	} {
		t.Run(strings.Join(tc.args, "/"), func(t *testing.T) {
			var out, err bytes.Buffer
			if code := Run(tc.args, &out, &err); code != tc.code {
				t.Fatalf("exit = %d", code)
			}
			if !strings.Contains(out.String(), tc.contains) {
				t.Fatalf("output = %q", out.String())
			}
			if tc.code != 0 && (out.Len() != 0 || err.Len() == 0) {
				t.Fatal("invalid error streams")
			}
			if tc.code == 0 && err.Len() != 0 {
				t.Fatal(err.String())
			}
			if strings.ContainsAny(err.String(), "\x1b\a") {
				t.Fatal("argument leaked terminal controls")
			}
		})
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken") }

func TestOutputFailure(t *testing.T) {
	var err bytes.Buffer
	if Run(nil, brokenWriter{}, &err) != 1 {
		t.Fatal("expected output failure")
	}
}
