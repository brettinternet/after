package gotestreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/after/internal/evidence"
)

func metadata() Metadata {
	return Metadata{Producer: "go1.27.1 (caller-supplied)", ImportedAt: time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC), Snapshot: evidence.Digest("sha256:" + strings.Repeat("a", 64))}
}
func parse(t *testing.T, data []byte) Report {
	t.Helper()
	r, err := Import(bytes.NewReader(data), metadata())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if string(r.OriginalDigest) != fmt.Sprintf("sha256:%x", sum) || r.OriginalBytes != len(data) {
		t.Fatal("lost original identity")
	}
	for _, c := range r.Cards {
		if err := c.State.Validate(); err != nil {
			t.Fatal(err)
		}
		if c.State.Kind != evidence.Reported || c.State.Applicability != evidence.Unknown || c.State.Execution != evidence.NotRun || c.Inputs != "unavailable" || c.ExpectedValues != "unavailable" || c.Effects != "unavailable" {
			t.Fatalf("promoted report: %+v", c)
		}
	}
	return r
}
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestCapturedFixtures(t *testing.T) {
	r := parse(t, fixture(t, "tests"))
	if r.Completeness != evidence.Complete || len(r.Cards) != 11 {
		t.Fatalf("%+v", r)
	}
	found := map[string]evidence.ReportOutcome{}
	for _, c := range r.Cards {
		found[c.Package+":"+c.Test] = c.State.Report
		if c.FirstEventAt == nil || c.LastEventAt == nil {
			t.Fatal("lost timestamps")
		}
	}
	for name, want := range map[string]evidence.ReportOutcome{
		"cases:TestPass": evidence.ReportPass, "other:TestPass": evidence.ReportPass,
		"cases:TestFail": evidence.ReportFail, "cases:TestSkip": evidence.ReportSkip,
		"cases:TestSubtests/pass": evidence.ReportPass, "cases:TestSubtests/skip": evidence.ReportSkip,
		"cases:TestParallelA": evidence.ReportPass, "cases:TestParallelB": evidence.ReportPass,
		"cases:": evidence.ReportFail,
	} {
		if found["example.invalid/reportfixture/"+name] != want {
			t.Errorf("%s: %v", name, found)
		}
	}
	build := parse(t, fixture(t, "build"))
	if len(build.Cards) != 2 || build.Completeness != evidence.Complete || build.Cards[0].Scope != "build" || build.Cards[0].State.Report != evidence.ReportFail || build.Cards[0].FirstEventAt != nil || build.Cards[1].State.Report != evidence.ReportFail {
		t.Fatalf("%+v", build)
	}
	noTests := parse(t, fixture(t, "no-tests"))
	if len(noTests.Cards) != 1 || noTests.Cards[0].Scope != "package" {
		t.Fatalf("%+v", noTests)
	}
	empty := parse(t, fixture(t, "empty"))
	if len(empty.Cards) != 0 || empty.Completeness != evidence.Incomplete {
		t.Fatalf("%+v", empty)
	}
}
func TestMalformedAndUnsupportedKeepUnrelatedCards(t *testing.T) {
	valid := `{"Action":"pass","Package":"good","Test":"TestPass"}`
	for _, bad := range []string{
		`{"Action":`, `null`, `[]`, `{"Action":"future","Package":"x"}`,
		`{"Action":"pass","Package":"x","Observed":true}`,
		`{"Action":"pass","Package":"x"} {}`, `{"Action":"start","Package":"x","Test":"TestX"}`,
		`{"Action":"pass","Package":"x","Elapsed":-1}`, `{"Action":"output","Package":"x","OutputType":"future"}`,
		string([]byte{0xff}), strings.Repeat("x", MaxLineBytes+1),
	} {
		r := parse(t, []byte(bad+"\n"+valid+"\n"))
		if r.Completeness != evidence.Incomplete || len(r.Diagnostics) == 0 || len(r.Cards) != 1 || r.Cards[0].Package != "good" {
			t.Fatalf("bad input recovery: %+v", r)
		}
	}
	r := parse(t, []byte(valid+"\n{\"Action\":"))
	if len(r.Cards) != 1 || r.Completeness != evidence.Incomplete {
		t.Fatal("truncated line lost valid card")
	}
}
func TestIncompleteAndRepeatedCases(t *testing.T) {
	r := parse(t, []byte(`{"Action":"run","Package":"p","Test":"T"}`))
	if r.Cards[0].State.Report != evidence.NoReport || r.Completeness != evidence.Incomplete {
		t.Fatal(r)
	}
	sequence := "{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"T\"}\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"T\"}\n"
	r = parse(t, []byte(sequence+sequence))
	if len(r.Cards) != 2 || r.Cards[1].Attempt != 2 {
		t.Fatal(r)
	}
	r = parse(t, []byte(sequence+`{"Action":"fail","Package":"p","Test":"T"}`))
	if r.Completeness != evidence.Incomplete || r.Cards[0].State.Report != evidence.ReportPass {
		t.Fatal("silently overwrote terminal status")
	}
}
func TestBounds(t *testing.T) {
	r := parse(t, []byte(strings.Repeat("bad\n", MaxDiagnostics+17)))
	if len(r.Diagnostics) != MaxDiagnostics || r.SuppressedDiagnostics != 18 {
		t.Fatalf("diagnostic cap: %+v", r)
	}
	var data bytes.Buffer
	for n := 0; n < MaxCards+2; n++ {
		fmt.Fprintf(&data, "{\"Action\":\"pass\",\"Package\":\"p%d\"}\n", n)
	}
	r = parse(t, data.Bytes())
	if len(r.Cards) != MaxCards || len(r.Diagnostics) != 2 {
		t.Fatal("card cap")
	}
	output, _ := json.Marshal(event{Action: "output", Package: "p", Output: strings.Repeat("a", MaxOutputBytes) + "界"})
	r = parse(t, append(output, []byte("\n{\"Action\":\"pass\",\"Package\":\"p\"}")...))
	if !r.Cards[0].OutputTruncated || len(r.Cards[0].Output) != MaxOutputBytes || r.Completeness != evidence.Incomplete {
		t.Fatal("output cap")
	}
	source := &countingReader{remaining: MaxBytes + 100}
	if _, err := Import(source, metadata()); err == nil || source.read != MaxBytes+1 {
		t.Fatalf("unbounded read %d: %v", source.read, err)
	}
}

type countingReader struct{ remaining, read int }

func (r *countingReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	clear(p[:n])
	r.remaining -= n
	r.read += n
	return n, nil
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, fmt.Errorf("secret reader error") }
func TestMetadataAndReadErrors(t *testing.T) {
	for _, mutate := range []func(*Metadata){func(m *Metadata) { m.Producer = "" }, func(m *Metadata) { m.ImportedAt = time.Time{} }, func(m *Metadata) { m.Snapshot = "../../file" }, func(m *Metadata) { m.CapturedAt = &time.Time{} }} {
		m := metadata()
		mutate(&m)
		if _, err := Import(strings.NewReader(""), m); err == nil {
			t.Fatal("accepted invalid metadata")
		}
	}
	if _, err := Import(brokenReader{}, metadata()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("leaked reader error")
	}
	m := metadata()
	captured := m.ImportedAt.Add(-time.Hour)
	m.CapturedAt = &captured
	r, err := Import(strings.NewReader(""), m)
	if err != nil || r.Metadata != m {
		t.Fatal("lost supplied provenance")
	}
}
func TestImportOnlyHostileData(t *testing.T) {
	// This test needs no executable, network, credentials, Docker, or project build.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOPROXY", "off")
	t.Setenv("DOCKER_HOST", "unix:///unavailable")
	hostile := "\x1b]52;c;clipboard\a\x1b[31m https://example.invalid ../../private $(command)"
	data, _ := json.Marshal(event{Action: "output", Package: hostile, Test: "TestObservedPaymentCountIsOne", Output: hostile})
	completion, _ := json.Marshal(event{Action: "pass", Package: hostile, Test: "TestObservedPaymentCountIsOne"})
	r := parse(t, append(append(data, '\n'), completion...))
	if r.Cards[0].Output != hostile || r.Cards[0].Package != hostile {
		t.Fatal("data interpreted")
	}
	encoded, err := json.Marshal(r)
	if err != nil || bytes.Contains(encoded, []byte{0x1b}) {
		t.Fatal("JSON emitted raw terminal escape")
	}
}
