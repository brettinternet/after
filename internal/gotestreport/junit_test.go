package gotestreport

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
)

func junitMeta() Metadata {
	return Metadata{Producer: "unverified caller", ImportedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func parseJUnit(t *testing.T, input string) Report {
	t.Helper()
	r, err := ImportJUnit(strings.NewReader(input), junitMeta())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Cards {
		if c.State.Validate() != nil || c.State.Producer != evidence.Importer || c.State.Kind != evidence.Reported || c.State.Applicability != evidence.Unknown || c.State.Execution != evidence.NotRun || c.State.Comparison != evidence.NotCompared || c.Inputs != "unavailable" || c.ExpectedValues != "unavailable" || c.Effects != "unavailable" {
			t.Fatalf("promoted report: %+v", c)
		}
	}
	return r
}

func TestJUnitCapturedProducers(t *testing.T) {
	for _, tc := range []struct {
		file             string
		pass, fail, skip int
	}{{"pytest.xml", 2, 2, 1}, {"vitest.xml", 2, 2, 1}, {"surefire.xml", 1, 2, 1}, {"pytest-empty.xml", 0, 0, 0}} {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/junit/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			r := parseJUnit(t, string(raw))
			counts := map[evidence.ReportOutcome]int{}
			output := ""
			for _, c := range r.Cards {
				counts[c.State.Report]++
				output += c.Output
			}
			if counts[evidence.ReportPass] != tc.pass || counts[evidence.ReportFail] != tc.fail || counts[evidence.ReportSkip] != tc.skip {
				t.Fatalf("outcomes: %+v", r)
			}
			if tc.pass == 0 {
				if r.Completeness != evidence.Incomplete || len(r.Cards) != 0 || r.Diagnostics[0].Code != "no_cases" {
					t.Fatalf("empty report: %+v", r)
				}
			} else if r.Completeness != evidence.Complete || !strings.Contains(output, "synthetic stdout") || !strings.Contains(output, "synthetic stderr") {
				t.Fatalf("capture: %+v", r)
			}
		})
	}
}

func TestJUnitNestedSuitesAndUnrelatedCasesSurvive(t *testing.T) {
	r := parseJUnit(t, `<testsuites><testsuite name="outer"><testsuite name="first"><testcase name="same"/><testcase name="extension"><future><failure/></future></testcase></testsuite><testsuite name="second"><testcase name="same"><error message="not an observation">reported error</error></testcase></testsuite><system-out>suite-only output</system-out></testsuite></testsuites>`)
	if len(r.Cards) != 4 || r.Cards[0].Package == r.Cards[2].Package || r.Cards[0].State.Report != evidence.ReportPass || r.Cards[1].State.Report != evidence.NoReport || r.Cards[2].State.Report != evidence.ReportFail || r.Cards[3].Scope != "package" || r.Cards[3].State.Report != evidence.NoReport || r.Completeness != evidence.Incomplete {
		t.Fatalf("nested report: %+v", r)
	}
	for _, c := range r.Cards[:3] {
		if strings.Contains(c.Output, "suite-only") {
			t.Fatal("suite output assigned to case")
		}
	}
}

func TestJUnitRejectsHostileXMLAndKeepsClosedCases(t *testing.T) {
	for _, tc := range []struct {
		name, input, code string
		cards             int
	}{
		{"external-entity", `<!DOCTYPE testsuite [<!ENTITY x SYSTEM "file:///synthetic/secret">]><testsuite name="s"><testcase name="x">&x;</testcase></testsuite>`, "forbidden_directive", 0},
		{"expansion", `<!DOCTYPE testsuite [<!ENTITY x "123"><!ENTITY y "&x;&x;">]><testsuite/>`, "forbidden_directive", 0},
		{"undefined-entity", `<testsuite name="s"><testcase name="safe"/><testcase name="&external;"/></testsuite>`, "invalid_xml", 1},
		{"truncated", "<testsuite name=\"s\">\n<testcase name=\"safe\"/>\n<testcase name=\"cut\">", "invalid_xml", 1},
		{"xinclude", `<testsuite name="s"><include xmlns="http://www.w3.org/2001/XInclude" href="https://example.invalid/private"/><testcase name="safe"/></testsuite>`, "unsupported_element", 1},
		{"stylesheet", `<?xml-stylesheet href="file:///synthetic/private"?><testsuite name="s"><testcase name="safe"/></testsuite>`, "unsupported_processing_instruction", 1},
		{"multiple-roots", `<testsuite name="s"><testcase name="safe"/></testsuite><testsuite/>`, "multiple_roots", 1},
		{"conflict", `<testsuite><testcase name="x"><failure/><skipped/></testcase></testsuite>`, "conflicting_outcomes", 1},
		{"attribute", `<testsuite><testcase name="x" observed="true"/><testcase name="safe"/></testsuite>`, "unsupported_attribute", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := parseJUnit(t, tc.input)
			if r.Completeness != evidence.Incomplete || len(r.Cards) != tc.cards {
				t.Fatalf("bad recovery: %+v", r)
			}
			found := false
			for _, d := range r.Diagnostics {
				if d.Line < 1 {
					t.Fatal("missing line")
				}
				found = found || d.Code == tc.code
			}
			if !found {
				t.Fatalf("missing %s: %+v", tc.code, r.Diagnostics)
			}
			if (tc.name == "conflict" || tc.name == "attribute") && r.Cards[0].State.Report != evidence.NoReport {
				t.Fatal("unsupported case inferred pass")
			}
		})
	}
}

func TestJUnitBounds(t *testing.T) {
	attributes := ""
	for i := 0; i <= MaxXMLAttributes; i++ {
		attributes += fmt.Sprintf(" a%d=\"x\"", i)
	}
	for _, tc := range []struct{ name, input, code string }{
		{"depth", strings.Repeat("<testsuite>", MaxXMLDepth+1) + strings.Repeat("</testsuite>", MaxXMLDepth+1), "xml_structure_limit"},
		{"elements", "<testsuite><properties>" + strings.Repeat(`<property name="x"/>`, MaxXMLElements) + "</properties></testsuite>", "xml_structure_limit"},
		{"attributes", "<testsuite" + attributes + "/>", "attribute_limit"},
		{"attribute-size", `<testsuite name="` + strings.Repeat("x", MaxXMLAttributeBytes) + `"/>`, "attribute_limit"},
		{"cards", "<testsuite>" + strings.Repeat(`<testcase name="x"/>`, MaxCards+1) + "</testsuite>", "card_limit"},
		{"text", `<testsuite><testcase name="x"><system-out>` + strings.Repeat("界", MaxXMLTextBytes) + `</system-out></testcase></testsuite>`, "text_limit"},
		{"output", `<testsuite><testcase name="x"><system-out>` + strings.Repeat("界", MaxOutputBytes) + `</system-out></testcase></testsuite>`, "output_limit"},
		{"total-output", "<testsuite>" + strings.Repeat(`<testcase name="x"><system-out>`+strings.Repeat("x", MaxOutputBytes)+`</system-out></testcase>`, 260) + "</testsuite>", "output_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := parseJUnit(t, tc.input)
			found, total := false, 0
			for _, d := range r.Diagnostics {
				found = found || d.Code == tc.code
			}
			for _, c := range r.Cards {
				total += len(c.Output)
				if len(c.Output) > MaxOutputBytes || !utf8.ValidString(c.Output) {
					t.Fatal("output unbounded or invalid UTF-8")
				}
			}
			if !found || r.Completeness != evidence.Incomplete || len(r.Cards) > MaxCards || len(r.Diagnostics) > MaxDiagnostics || total > MaxReportOutputBytes {
				t.Fatalf("bounds: cards=%d diagnostics=%+v total=%d", len(r.Cards), r.Diagnostics, total)
			}
		})
	}
	if _, err := ImportJUnit(io.LimitReader(infiniteXMLReader{}, MaxBytes+2), junitMeta()); err == nil {
		t.Fatal("oversize input accepted")
	}
}

type infiniteXMLReader struct{}

func (infiniteXMLReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestReportFormatSelection(t *testing.T) {
	for _, tc := range []struct{ input, format, dialect string }{
		{`{"Action":"pass","Package":"x"}`, "auto", Dialect},
		{"\xef\xbb\xbf<testsuite><testcase name=\"x\"/></testsuite>", "auto", JUnitDialect},
		{`<testsuite/>`, "junit", JUnitDialect},
	} {
		r, err := ImportFormat(strings.NewReader(tc.input), junitMeta(), tc.format)
		if err != nil || r.Dialect != tc.dialect {
			t.Fatalf("selection: %+v %v", r, err)
		}
	}
	for _, input := range []string{"runner output", `[{"Action":"pass"}]`} {
		if _, err := ImportFormat(strings.NewReader(input), junitMeta(), "auto"); err == nil || !strings.Contains(err.Error(), "--format") {
			t.Fatalf("ambiguous input accepted: %v", err)
		}
	}
}

func FuzzJUnitNeverPromotesOrPanics(f *testing.F) {
	f.Add([]byte(`<testsuite><testcase name="x"/></testsuite>`))
	f.Add([]byte(`<!DOCTYPE testsuite><testsuite/>`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		r, err := ImportJUnit(bytes.NewReader(raw), junitMeta())
		if err != nil {
			return
		}
		if len(r.Cards) > MaxCards || len(r.Diagnostics) > MaxDiagnostics {
			t.Fatal("unbounded result")
		}
		for _, c := range r.Cards {
			if c.State.Kind != evidence.Reported || c.State.Execution != evidence.NotRun || len(c.Output) > MaxOutputBytes || !utf8.ValidString(c.Output) {
				t.Fatalf("unsafe card: %+v", c)
			}
		}
	})
}
