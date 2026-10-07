package cli

import (
	"fmt"
	"strings"

	"github.com/brettinternet/after/internal/terminal"
	ucli "github.com/urfave/cli/v2"
)

func formatDiagnostic(problem, fix string) string {
	problem = cleanDiagnosticText(terminal.Sanitize(problem))
	fix = cleanDiagnosticText(terminal.Sanitize(fix))
	problem = strings.TrimPrefix(problem, "after: ")
	return fmt.Sprintf("after: %s — %s", problem, fix)
}

func unknownCommandDiagnostic(input string) string {
	input = safeDiagnosticInput(input)
	if suggestion, ok := closestMatch(input, []string{"capture", "import", "inspect", "compare", "export", "run", "pin", "review", "config", "version", "help"}); ok {
		return formatDiagnostic(fmt.Sprintf("unknown command %q", input), "did you mean "+suggestion+"?")
	}
	return formatDiagnostic(fmt.Sprintf("unknown command %q", input), "run after --help to list available commands")
}

func safeDiagnosticInput(input string) string {
	input = cleanDiagnosticText(terminal.Sanitize(input))
	if len(input) > 128 {
		return input[:128] + "…"
	}
	return input
}

func unknownFlagName(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	message := err.Error()
	for _, marker := range []string{"flag provided but not defined:", "unknown flag:"} {
		if index := strings.LastIndex(message, marker); index >= 0 {
			value := strings.TrimSpace(message[index+len(marker):])
			value = strings.Trim(value, `"'`)
			if value == "" || len(value) > 128 {
				return "", false
			}
			if strings.HasPrefix(value, "--") {
				return value, true
			}
			if strings.HasPrefix(value, "-") {
				return "-" + value, true
			}
			return "--" + value, true
		}
	}
	return "", false
}

func unknownFlagError(command, input string, flags []ucli.Flag) error {
	input = safeDiagnosticInput(input)
	candidates := []string{}
	for _, flag := range flags {
		for _, name := range flag.Names() {
			if strings.HasPrefix(name, "-") {
				candidates = append(candidates, name)
			} else {
				candidates = append(candidates, "--"+name)
			}
		}
	}
	if suggestion, ok := closestMatch(input, candidates); ok {
		return invalidWithFix(fmt.Sprintf("unknown flag %q", input), "did you mean "+suggestion+"?")
	}
	return invalidWithFix(fmt.Sprintf("unknown flag %q", input), "run after "+command+" --help to list this command's options")
}

func closestMatch(input string, candidates []string) (string, bool) {
	if len(input) == 0 || len(input) > 64 {
		return "", false
	}
	best, bestDistance, count := "", 3, 0
	for _, candidate := range candidates {
		distance := editDistance(strings.ToLower(input), strings.ToLower(candidate))
		if distance < bestDistance {
			best, bestDistance, count = candidate, distance, 1
		} else if distance == bestDistance {
			count++
		}
	}
	return best, bestDistance <= 2 && count == 1
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}

func removedForm(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "inspect":
		if hasOption(args[1:], "--base") {
			return invalidWithFix("inspect no longer accepts --base as a snapshot ID", "use after inspect BASE CANDIDATE")
		}
	case "review":
		if hasOption(args[1:], "--tui") || hasOption(args[1:], "--base") || hasOption(args[1:], "--evidence") {
			return invalidWithFix("review no longer uses --tui, --base, or --evidence for snapshot pairs", "use after review BASE CANDIDATE [EVIDENCE ...]")
		}
		if hasOption(args[1:], "--select") {
			return invalidWithFix("pin selection moved out of review", "use after pin PIN --select SNAPSHOT [--mode MODE] [--reason TEXT]")
		}
		if hasOption(args[1:], "--receipt") {
			return invalidWithFix("pin receipt attachment moved out of review", "use after pin PIN --attach RECEIPT [--reason TEXT]")
		}
		if hasOption(args[1:], "--accept") {
			return invalidWithFix("pin acceptance moved out of review", "use after pin PIN --accept [--reason TEXT]")
		}
		if hasOption(args[1:], "--mode") || hasOption(args[1:], "--reason") {
			return invalidWithFix("pin decisions moved out of review", "use after pin PIN with --select, --attach, or --accept")
		}
		if len(positionalArgs(args[1:])) == 1 {
			return invalidWithFix("pin inspection moved out of review", "use after pin PIN")
		}
	}
	return nil
}

func hasOption(args []string, option string) bool {
	for _, arg := range args {
		name, _, _ := strings.Cut(arg, "=")
		if name == option {
			return true
		}
	}
	return false
}

func positionalArgs(args []string) []string {
	valueFlags := map[string]bool{
		"--config": true, "--project": true, "--import-file": true, "--producer": true,
		"--base": true, "--evidence": true, "--select": true, "--receipt": true,
		"--mode": true, "--reason": true,
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if name, _, hasValue := strings.Cut(arg, "="); valueFlags[name] {
			if !hasValue && i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		positional = append(positional, arg)
	}
	return positional
}
