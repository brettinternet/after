package cli

import (
	"fmt"
	"strings"

	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
)

func helpRequest(args []string) (string, bool, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		return "", true, nil
	}
	if args[0] == "help" {
		if len(args) == 1 || (len(args) == 2 && (args[1] == "--help" || args[1] == "-h")) {
			return "", true, nil
		}
		name := args[1]
		if !knownCommand(name) {
			return "", true, fmt.Errorf("%s", unknownCommandDiagnostic(name))
		}
		if len(args) > 2 {
			return "", true, invalidWithFix("help accepts one command name", "try: after help COMMAND")
		}
		return name, true, nil
	}
	if knownCommand(args[0]) {
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				return args[0], true, nil
			}
		}
	}
	return "", false, nil
}

func commandHelp(name string) string {
	if name == "" || name == "help" {
		return topLevelHelp()
	}
	defaults := config.Defaults()
	global := fmt.Sprintf(`
Global
  --project DIR   checkout's Git root (default: nearest .git root)
  --config FILE   configuration file (default: XDG_CONFIG_HOME/after/config.yaml
                  or ~/.config/after/config.yaml, if present)
  --json          print the versioned JSON result
`)
	var body string
	switch name {
	case "capture":
		body = `Capture a bounded local Git comparison. Project code never runs; untracked
files are excluded by default.

Usage
  after capture
  after capture --staged
  after capture --base main [--target HEAD]
  after capture --include-untracked PATH

Options
  --staged                   capture HEAD versus the index
  --base REF                 capture the merge base with a Git ref
  --target REF               other side of a --base capture (default: HEAD)
  --include-untracked PATH   include one non-ignored untracked file (repeatable)
` + global
	case "import":
		body = `Import stock go test -json as reported evidence; AFTER does not run tests.

Usage
  after import FILE --producer TEXT [--snapshot ID]

Options
  --producer TEXT   caller-supplied producer and version provenance (required)
  --captured-at RFC3339   optional caller-supplied capture time
  --snapshot ID     bind the report to a stored snapshot
  --offset N        first report card (default: 0)
  --limit N         report cards to return (default: 128; range: 1–256)
` + global
	case "inspect":
		body = `Inspect one stored record or the complete diff between two stored snapshots.

Usage
  after inspect ID
  after inspect BASE CANDIDATE

Options
  --raw-diff               include a bounded captured patch (default: ` + fmt.Sprint(defaults.RawDiff) + `)
  --diff-bytes N           maximum raw patch bytes (default: ` + fmt.Sprint(defaults.DiffBytes) + `)
  --diff-offset N          raw diff byte offset (default: 0)
  --diff-size N            raw diff page bytes (default: 65536)
  --inventory-offset N     first changed/unknown inventory entry (default: 0)
  --inventory-limit N      entries to return (default: 128; range: 1–256)
  --card-offset N          first report card (default: 0)
  --card-limit N           report cards to return (default: 128; range: 1–256)
  --artifact-offset N      stored artifact byte offset (default: 0)
  --artifact-size N        stored artifact page bytes (default: 65536)
` + global
	case "compare":
		body = `Compare a persisted execution receipt without running project code.

Usage
  after compare RECEIPT

Options
  No command-specific options.
` + global
	case "export":
		body = `Export a bounded machine-readable comparison or evidence page as JSON.

Usage
  after export ID
  after export BASE CANDIDATE

Options
  --raw-diff               include a bounded captured patch (default: ` + fmt.Sprint(defaults.RawDiff) + `)
  --diff-bytes N           maximum raw patch bytes (default: ` + fmt.Sprint(defaults.DiffBytes) + `)
  --diff-offset N          raw diff byte offset (default: 0)
  --diff-size N            raw diff page bytes (default: 65536)
  --inventory-offset N     first changed/unknown inventory entry (default: 0)
  --inventory-limit N      entries to return (default: 128; range: 1–256)
  --card-offset N          first report card (default: 0)
  --card-limit N           report cards to return (default: 128; range: 1–256)
  --artifact-offset N      stored artifact byte offset (default: 0)
  --artifact-size N        stored artifact page bytes (default: 65536)
` + global
	case "run":
		body = fmt.Sprintf(`Prepare the frozen offline experiment; exact consent is required before it runs.

Usage
  after run BASE CANDIDATE
  after run BASE CANDIDATE --plan-out FILE
  after run --plan-file FILE --approve FULL_DIGEST

Options
  --plan-file FILE       reconstruct an exact saved execution preview
  --plan-out FILE        save a private preview without overwriting a file
  --approve DIGEST       approve only this full lowercase sha256 digest
  --docker-binary PATH   trusted Docker CLI path (default: not configured)
  --docker-host SOCKET   local Unix socket (default: not configured)
  --repetitions N        paired run repetitions (default: %d; range: 1–5)
  --run-seconds N        sandbox time limit (default: %d; range: 1–300)
  --output-bytes N       container output (default: %d; range: 1–1048576)
  --interactive BOOL     allow exact-plan terminal confirmation (default: %t)
`, defaults.Repetitions, defaults.RunSeconds, defaults.OutputBytes, defaults.Interactive) + global
	case "pin":
		body = fmt.Sprintf(`Create or inspect a pin, or record one explicit decision. Pin actions never
execute project code.

Usage
  after pin RECEIPT --expectation TEXT [--scope SCOPE] [--reason TEXT]
  after pin PIN
  after pin PIN --accept [--reason TEXT]
  after pin PIN --attach RECEIPT [--reason TEXT]
  after pin PIN --select SNAPSHOT [--mode MODE] [--reason TEXT]

Options
  --expectation TEXT   human expectation to pin (maximum 4096 bytes)
  --scope SCOPE        finite_example or human_intent (default: %s)
  --accept             accept the currently reviewed complete result
  --attach RECEIPT     attach an authorized receipt without accepting it
  --select SNAPSHOT    select a captured candidate snapshot
  --mode MODE          original_base or last_inspected (default: %s)
  --reason TEXT        verbatim reason stored in pin history (optional)
`, evidence.FiniteExample, evidence.OriginalBase) + global
	case "review":
		body = `Browse a captured snapshot pair in the terminal review; opening it runs no
project code.

Usage
  after review BASE CANDIDATE [EVIDENCE ...]
  after review BASE CANDIDATE --import-file FILE --producer TEXT

Options
  --import-file FILE   file to read only after the explicit TUI import action
  --producer TEXT      caller provenance required with --import-file
` + global
	case "config":
		body = `Show effective configuration and setting sources without exposing secrets.

Usage
  after config

Options
  No command-specific options.
` + global
	case "version":
		body = `Print the AFTER version.

Usage
  after version

Options
  No command-specific options.
`
	default:
		return topLevelHelp()
	}
	return body
}

func topLevelHelp() string {
	return `Review local changes and their evidence without running project code unless
you approve an exact plan.

Everyday
  after review BASE CANDIDATE   browse a captured pair in the terminal review
  after capture                 capture a local Git comparison
  after inspect ID              inspect a stored record or snapshot pair

Evidence
  after run                     prepare a consented offline payment run
  after import                  read a go test -json report file
  after compare                 compare a stored execution receipt
  after pin                     create, inspect, or update an expectation
  after export                  print a comparison or evidence page as JSON

Setup
  after config                  show settings and their sources
  after version                 print the AFTER version

Global options: --project DIR, --config FILE, --json
Run after COMMAND --help for that command's options.
`
}

func helpNames() []string {
	return []string{"", "capture", "import", "inspect", "compare", "export", "run", "pin", "review", "config", "version"}
}

func commandExample(name string) string {
	examples := map[string]string{
		"capture": "after capture",
		"import":  "after import FILE --producer TEXT",
		"inspect": "after inspect ID",
		"compare": "after compare RECEIPT",
		"export":  "after export ID",
		"run":     "after run BASE CANDIDATE",
		"pin":     "after pin RECEIPT --expectation TEXT",
		"review":  "after review BASE CANDIDATE",
		"config":  "after config",
		"version": "after version",
	}
	if example, ok := examples[name]; ok {
		return example
	}
	return "after --help"
}

func missingArgument(name string, count int) string {
	missing := map[string]string{
		"import":  "a Go test JSON file",
		"inspect": "a record ID or BASE CANDIDATE snapshot pair",
		"compare": "a receipt ID",
		"export":  "a record ID",
		"run":     "two snapshot IDs or a saved plan",
		"pin":     "a receipt ID or pin ID",
		"review":  "a BASE CANDIDATE snapshot pair",
	}
	if value, ok := missing[name]; ok {
		return "missing " + value
	}
	return fmt.Sprintf("missing %d required argument(s)", count)
}

func cleanDiagnosticText(value string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(value), " "))
}
