package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	ucli "github.com/urfave/cli/v2"
)

var inspectKinds = []string{"snapshot", "receipt", "comparison", "report", "artifact"}
var browserKinds = []string{"receipt", "comparison", "report", "pin"}

// Check syntax before opening storage, including an absent store. Git refs for
// capture and authorization digests for run are intentionally not ID prefixes.
func validateIDInputs(ctx *ucli.Context) error {
	var values []string
	switch ctx.Command.Name {
	case "inspect", "export", "compare", "pin", "review", "run":
		values = append(values, ctx.Args().Slice()...)
	}
	var flags []string
	switch ctx.Command.Name {
	case "import":
		flags = []string{"snapshot"}
	case "inspect", "export":
		flags = []string{"base"}
	case "review":
		flags = []string{"base", "select", "receipt"}
	}
	for _, flag := range flags {
		if ctx.IsSet(flag) {
			values = append(values, ctx.String(flag))
		}
	}
	if ctx.Command.Name == "review" {
		values = append(values, ctx.StringSlice("evidence")...)
	}
	for _, value := range values {
		if _, err := idPrefix(value); err != nil {
			return err
		}
	}
	return nil
}

func idPrefix(value string) (string, error) {
	prefix := strings.TrimPrefix(strings.ToLower(value), "sha256:")
	if len(prefix) < 4 {
		return "", invalid("ID prefixes require at least 4 hex characters")
	}
	if len(prefix) > 64 {
		return "", invalid("ID must contain 4 to 64 hex characters, optionally prefixed with sha256:")
	}
	for _, c := range prefix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", invalid("ID must contain 4 to 64 hex characters, optionally prefixed with sha256:")
		}
	}
	return prefix, nil
}

func resolveID(s *store.Store, value string, kinds ...string) (evidence.Digest, error) {
	prefix, err := idPrefix(value)
	if err != nil {
		return "", err
	}
	storageKinds := make([]string, 0, len(kinds))
	allowed := map[string]bool{}
	for _, kind := range kinds {
		allowed[kind] = true
		if kind == "report" || kind == "artifact" {
			kind = "blob"
		}
		storageKinds = append(storageKinds, kind)
	}
	entries, err := s.List(storageKinds...)
	if err != nil {
		return "", operational("cannot resolve IDs: store index is unavailable or exceeds limits")
	}
	type match struct {
		id                evidence.Digest
		kind, description string
	}
	var matches []match
	var bytes int64
	for _, entry := range entries {
		if !strings.HasPrefix(string(entry.ID)[7:], prefix) {
			continue
		}
		bytes += entry.Size
		if bytes > 32<<20 {
			return "", operational("ID lookup exceeds read limit; use more characters")
		}
		kind, description, err := describeID(s, entry)
		if err != nil {
			return "", operational("matching record is corrupt or unavailable")
		}
		if !allowed[kind] {
			continue
		}
		matches = append(matches, match{entry.ID, kind, description})
	}
	if len(matches) == 1 {
		return matches[0].id, nil
	}
	if len(matches) == 0 {
		return "", noIDMatch(prefix, kinds...)
	}
	var message strings.Builder
	fmt.Fprintf(&message, "after: %s matches %d records — use more characters:", prefix, len(matches))
	for _, match := range matches[:min(10, len(matches))] {
		// Flatten line breaks before the existing terminal sanitizer; no repository
		// string can add a diagnostic line, OSC, bidi control or unbounded output.
		description := terminal.Line(strings.Join(strings.Fields(match.description), " "), 48)
		fmt.Fprintf(&message, "\n  %s  %-10s %s", shortID(match.id), match.kind, description)
	}
	return "", &exitError{code: ExitInvalid, diagnostic: message.String()}
}

func noIDMatch(value string, kinds ...string) error {
	prefix, err := idPrefix(value)
	if err != nil {
		return err
	}
	return &exitError{code: ExitInvalid, diagnostic: fmt.Sprintf("after: no %s matches %s — after log lists recent records", strings.Join(kinds, " or "), prefix)}
}

func resolveBrowserSelection(project, base, candidate string, ids []string) (browser.Selection, error) {
	selection := browser.Selection{Project: project}
	s, err := store.Open(project, false, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return selection, noIDMatch(candidate, "snapshot")
		}
		return selection, operational("cannot open private evidence store for reading")
	}
	defer s.Close()
	selection.Pair.Base, err = resolveID(s, base, "snapshot")
	if err != nil {
		return selection, err
	}
	selection.Pair.Candidate, err = resolveID(s, candidate, "snapshot")
	if err != nil {
		return selection, err
	}
	for _, id := range ids {
		resolved, err := resolveID(s, id, browserKinds...)
		if err != nil {
			return selection, err
		}
		selection.Evidence = append(selection.Evidence, resolved)
	}
	return selection, nil
}

func describeID(s *store.Store, entry store.Entry) (string, string, error) {
	switch entry.Kind {
	case "snapshot":
		v, err := store.Get[evidence.Snapshot](s, entry.ID)
		return "snapshot", strings.ReplaceAll(string(v.Source), "_", " "), err
	case "receipt":
		v, err := store.Get[evidence.Receipt](s, entry.ID)
		return "receipt", fmt.Sprintf("run %s → %s · %s", shortID(v.Snapshots.Base), shortID(v.Snapshots.Candidate), v.State.Execution), err
	case "comparison":
		v, err := store.Get[evidence.Comparison](s, entry.ID)
		return "comparison", fmt.Sprintf("%s · receipt %s", v.Outcome, shortID(v.Receipt)), err
	case "pin":
		v, err := store.Get[evidence.Pin](s, entry.ID)
		return "pin", v.Expectation, err
	case "blob":
		raw, err := s.ReadBlob(entry.ID)
		if err != nil {
			return "", "", err
		}
		var report gotestreport.Report
		if strictJSON(raw, &report) == nil && validImportedReport(report) {
			return "report", "reported · " + report.Metadata.Producer, nil
		}
		return "artifact", fmt.Sprintf("%d bytes · no producer established", len(raw)), nil
	default:
		return "", "", fmt.Errorf("unsupported record kind")
	}
}
