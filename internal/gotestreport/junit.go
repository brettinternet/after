package gotestreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brettinternet/after/internal/evidence"
)

const (
	JUnitDialect         = "junit-xml-v1"
	MaxXMLDepth          = 32
	MaxXMLElements       = 100000
	MaxXMLAttributes     = 64
	MaxXMLAttributeBytes = 16 << 10
	MaxXMLTextBytes      = 64 << 10
	MaxReportOutputBytes = 4 << 20
)

func SupportedDialect(dialect string) bool { return dialect == Dialect || dialect == JUnitDialect }

func DialectLabel(dialect string) string {
	if dialect == JUnitDialect {
		return "JUnit XML"
	}
	return "go test JSON"
}

func ArtifactChannel(dialect string) string {
	if dialect == JUnitDialect {
		return "junit-report-v1"
	}
	return "go-test-report-v1"
}

func ReportChannel(channel string) bool {
	return channel == "go-test-report-v1" || channel == "junit-report-v1"
}

// ImportFormat never uses a filename, producer claim, or repository configuration
// to select a parser. Auto recognizes only XML markup or a JSON object prefix.
func ImportFormat(src io.Reader, meta Metadata, format string) (Report, error) {
	if format != "" && format != "auto" && format != "go-test-json" && format != "junit" {
		return Report{}, errors.New("unknown report format; use --format go-test-json or --format junit")
	}
	if src == nil {
		return Report{}, errors.New("cannot read report")
	}
	raw, err := io.ReadAll(io.LimitReader(src, MaxBytes+1))
	if err != nil {
		return Report{}, errors.New("cannot read report")
	}
	if len(raw) > MaxBytes {
		return Report{}, errors.New("report exceeds 8 MiB limit; nothing imported")
	}
	if format == "" || format == "auto" {
		prefix := bytes.TrimSpace(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}))
		switch {
		case len(prefix) == 0, prefix[0] == '{':
			format = "go-test-json"
		case prefix[0] == '<':
			format = "junit"
		default:
			return Report{}, errors.New("unrecognized report format; use --format go-test-json or --format junit")
		}
	}
	if format == "junit" {
		return ImportJUnit(bytes.NewReader(raw), meta)
	}
	return Import(bytes.NewReader(raw), meta)
}

// ImportJUnit accepts the bounded junit-xml-v1 dialect. encoding/xml has no
// external entity resolver; directives (including DTDs) are rejected outright.
// Malformed XML stops parsing, retaining only already closed cases. Unsupported
// well-formed subtrees are diagnosed and skipped without losing sibling cases.
func ImportJUnit(src io.Reader, meta Metadata) (Report, error) {
	if src == nil || !validMetadata(meta) {
		return Report{}, errors.New("invalid report metadata")
	}
	raw, err := io.ReadAll(io.LimitReader(src, MaxBytes+1))
	if err != nil {
		return Report{}, errors.New("cannot read report")
	}
	if len(raw) > MaxBytes {
		return Report{}, errors.New("report exceeds 8 MiB limit; nothing imported")
	}
	sum := sha256.Sum256(raw)
	r := Report{SchemaVersion: 1, Dialect: JUnitDialect, Metadata: meta, OriginalDigest: evidence.Digest("sha256:" + hex.EncodeToString(sum[:])), OriginalBytes: len(raw), Cards: []Card{}, Diagnostics: []Diagnostic{}, Completeness: evidence.Complete}
	diagnostic := func(line int, code string) {
		r.Completeness = evidence.Incomplete
		if len(r.Diagnostics) < MaxDiagnostics {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{line, code})
		} else {
			r.SuppressedDiagnostics++
		}
	}
	type frame struct {
		name     string
		suite    string
		card     *Card
		invalid  bool
		outcomes int
		skip     bool
	}
	stack := []frame{}
	attempts := map[string]int{}
	retained, identities, elements, roots, cases := 0, 0, 0, 0, 0
	decoder := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	currentCard := func() *frame {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].card != nil {
				return &stack[i]
			}
		}
		return nil
	}
	appendOutput := func(c *Card, text string, line int) {
		if c.OutputTruncated || text == "" {
			return
		}
		available := min(MaxOutputBytes-len(c.Output), MaxReportOutputBytes-retained)
		if len(text) > available {
			diagnostic(line, "output_limit")
			c.OutputTruncated = true
			for available > 0 && !utf8.ValidString(text[:available]) {
				available--
			}
			text = text[:available]
		}
		c.Output += text
		retained += len(text)
	}
	newCard := func(pkg, test, scope string) *Card {
		return &Card{Package: pkg, Test: test, Scope: scope, Attempt: 1, State: evidence.EvidenceState{Producer: evidence.Importer, Kind: evidence.Reported, Applicability: evidence.Unknown, Execution: evidence.NotRun, Comparison: evidence.NotCompared, Report: evidence.NoReport}, Inputs: "unavailable", ExpectedValues: "unavailable", Effects: "unavailable"}
	}
	for {
		line, _ := decoder.InputPos()
		token, tokenErr := decoder.Token()
		if tokenErr == io.EOF {
			break
		}
		if tokenErr != nil {
			if syntax, ok := tokenErr.(*xml.SyntaxError); ok {
				line = syntax.Line
			}
			diagnostic(line, "invalid_xml")
			break
		}
		switch t := token.(type) {
		case xml.Directive:
			diagnostic(line, "forbidden_directive")
			return r, nil
		case xml.ProcInst:
			if t.Target != "xml" || roots != 0 {
				diagnostic(line, "unsupported_processing_instruction")
			}
		case xml.StartElement:
			elements++
			if elements > MaxXMLElements || len(stack) >= MaxXMLDepth {
				diagnostic(line, "xml_structure_limit")
				return r, nil
			}
			if len(t.Attr) > MaxXMLAttributes {
				diagnostic(line, "attribute_limit")
				return r, nil
			}
			for _, a := range t.Attr {
				if len(a.Name.Local)+len(a.Name.Space)+len(a.Value) > MaxXMLAttributeBytes {
					diagnostic(line, "attribute_limit")
					return r, nil
				}
			}
			parent := ""
			f := frame{name: t.Name.Local}
			if len(stack) > 0 {
				parent = stack[len(stack)-1].name
				f.skip = stack[len(stack)-1].skip
			}
			if len(stack) == 0 {
				roots++
				if roots > 1 {
					diagnostic(line, "multiple_roots")
					return r, nil
				}
			}
			allowed := false
			switch f.name {
			case "testsuites":
				allowed = parent == "" || parent == "testsuites"
			case "testsuite":
				allowed = parent == "" || parent == "testsuites" || parent == "testsuite"
			case "testcase":
				allowed = parent == "testsuite"
			case "failure", "error", "skipped":
				allowed = parent == "testcase"
			case "system-out", "system-err":
				allowed = parent == "testcase" || parent == "testsuite"
			case "properties":
				allowed = parent == "testsuite" || parent == "testcase"
			case "property":
				allowed = parent == "properties"
			}
			if !f.skip && (!allowed || t.Name.Space != "") {
				diagnostic(line, "unsupported_element")
				if c := currentCard(); c != nil {
					c.invalid = true
				}
				f.skip = true
			}
			attrs := map[string]string{}
			if !f.skip {
				for _, a := range t.Attr {
					if _, duplicate := attrs[a.Name.Local]; duplicate {
						diagnostic(line, "duplicate_attribute")
						f.invalid = true
					}
					attrs[a.Name.Local] = a.Value
					schemaHint := (f.name == "testsuite" || f.name == "testsuites") && ((a.Name.Space == "xmlns" && a.Name.Local == "xsi" && a.Value == "http://www.w3.org/2001/XMLSchema-instance") || (a.Name.Space == "http://www.w3.org/2001/XMLSchema-instance" && a.Name.Local == "noNamespaceSchemaLocation"))
					if !schemaHint && (a.Name.Space != "" || !junitAttribute(f.name, a.Name.Local)) {
						diagnostic(line, "unsupported_attribute")
						f.invalid = true
					}
				}
				if f.invalid {
					if c := currentCard(); c != nil {
						c.invalid = true
					}
				}
				switch f.name {
				case "testsuite":
					f.suite = strconv.Quote(attrs["name"])
					for i := len(stack) - 1; i >= 0; i-- {
						if stack[i].suite != "" {
							f.suite = stack[i].suite + " / " + f.suite
							break
						}
					}
				case "testcase":
					cases++
					if cases > MaxCards {
						diagnostic(line, "card_limit")
						f.skip = true
						break
					}
					pkg := stack[len(stack)-1].suite
					if attrs["classname"] != "" {
						pkg += " :: " + strconv.Quote(attrs["classname"])
					}
					identities += len(pkg) + len(attrs["name"])
					if len(pkg) > MaxXMLAttributeBytes || identities > MaxReportOutputBytes {
						diagnostic(line, "identity_limit")
						f.skip = true
						break
					}
					f.card = newCard(pkg, attrs["name"], "test")
					if attrs["name"] == "" {
						diagnostic(line, "missing_test_name")
						f.invalid = true
						f.card.Test = "(unnamed test)"
					}
				case "failure", "error", "skipped":
					c := currentCard()
					c.outcomes++
					if c.outcomes > 1 {
						diagnostic(line, "conflicting_outcomes")
						c.invalid = true
					}
					c.card.State.Report = evidence.ReportFail
					if f.name == "skipped" {
						c.card.State.Report = evidence.ReportSkip
					}
					appendOutput(c.card, "["+f.name+"] "+attrs["type"]+" "+attrs["message"]+"\n", line)
				case "system-out", "system-err":
					if parent == "testsuite" {
						// Suite output is not attributed to every test or used to infer success.
						pkg := stack[len(stack)-1].suite
						identities += len(pkg)
						if len(pkg) > MaxXMLAttributeBytes || identities > MaxReportOutputBytes {
							diagnostic(line, "identity_limit")
							f.skip = true
							break
						}
						f.card = newCard(pkg, "", "package")
					}
				}
			}
			stack = append(stack, f)
			if !f.skip && (f.name == "system-out" || f.name == "system-err") {
				appendOutput(currentCard().card, "["+f.name+"]\n", line)
			}
		case xml.CharData:
			if len(t) > MaxXMLTextBytes {
				diagnostic(line, "text_limit")
				if c := currentCard(); c != nil {
					c.invalid = true
				}
				end := MaxXMLTextBytes
				for end > 0 && !utf8.Valid(t[:end]) {
					end--
				}
				t = t[:end]
			}
			if len(stack) == 0 {
				if len(bytes.TrimSpace(t)) > 0 {
					diagnostic(line, "text_outside_root")
				}
				continue
			}
			f := &stack[len(stack)-1]
			if f.skip {
				continue
			}
			switch f.name {
			case "failure", "error", "skipped", "system-out", "system-err":
				appendOutput(currentCard().card, string(t), line)
			case "property": // Known metadata, never interpreted as paths or evidence.
			default:
				if len(bytes.TrimSpace(t)) > 0 {
					diagnostic(line, "unsupported_text")
					if c := currentCard(); c != nil {
						c.invalid = true
					}
				}
			}
		case xml.EndElement:
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if f.card == nil || f.skip {
				continue
			}
			if len(r.Cards) >= MaxCards {
				diagnostic(line, "card_limit")
				continue
			}
			if f.name == "testcase" {
				if f.invalid {
					f.card.State.Report = evidence.NoReport
				} else if f.outcomes == 0 {
					f.card.State.Report = evidence.ReportPass
				}
			}
			key := f.card.Package + "\x00" + f.card.Test
			attempts[key]++
			f.card.Attempt = attempts[key]
			r.Cards = append(r.Cards, *f.card)
		}
	}
	if cases == 0 {
		diagnostic(1, "no_cases")
	}
	return r, nil
}

func junitAttribute(element, attribute string) bool {
	var allowed string
	switch element {
	case "testsuites", "testsuite":
		allowed = " name tests failures errors skipped disabled time timestamp hostname id package assertions version "
	case "testcase":
		allowed = " name classname time file line assertions "
	case "failure", "error", "skipped":
		allowed = " message type "
	case "property":
		allowed = " name value "
	}
	return strings.Contains(allowed, " "+attribute+" ")
}
