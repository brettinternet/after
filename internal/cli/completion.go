package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/store"
	"github.com/brettinternet/after/internal/terminal"
	ucli "github.com/urfave/cli/v2"
)

const completionIDReadLimit int64 = 32 << 20

var completionShells = []string{"bash", "zsh", "fish"}

type completionCandidate struct {
	word        string
	description string
}

func completionCommand(state *invocation) *ucli.Command {
	return &ucli.Command{
		Name:      "completion",
		Usage:     "print a shell completion script",
		ArgsUsage: "[bash|zsh|fish]",
		Flags:     globalFlags(),
		Action: func(ctx *ucli.Context) error {
			if ctx.NArg() > 1 {
				return invalidWithFix("completion accepts one shell name", "try: after completion [bash|zsh|fish]")
			}
			shell := ""
			if ctx.NArg() == 1 {
				shell = ctx.Args().First()
			} else {
				shell = filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
			}
			script, ok := completionScript(shell)
			if !ok {
				return invalidWithFix("unsupported completion shell "+fmt.Sprintf("%q", safeDiagnosticInput(shell)), "supported shells: bash, zsh, fish")
			}
			if _, err := io.WriteString(state.stdout, script); err != nil {
				return operational("cannot write shell completion script")
			}
			return nil
		},
	}
}

func completionScript(shell string) (string, bool) {
	switch shell {
	case "bash":
		return bashCompletionScript, true
	case "zsh":
		return zshCompletionScript, true
	case "fish":
		return fishCompletionScript, true
	default:
		return "", false
	}
}

const bashCompletionScript = `# AFTER Bash completion. Bash's Readline API displays words, not descriptions.
_after_complete() {
  local current candidate description
  local -a candidates
  current=${COMP_WORDS[COMP_CWORD]}
  candidates=()
  while IFS=$'\t' read -r candidate description; do
    [[ $candidate == "$current"* ]] && candidates+=("$candidate")
  done < <(command after __complete bash "${COMP_WORDS[@]:1:COMP_CWORD}" 2>/dev/null)
  COMPREPLY=("${candidates[@]}")
}
complete -F _after_complete after
`

const zshCompletionScript = `#compdef after

_after() {
  local candidate description
  local -a after_args candidates descriptions
  after_args=()
  (( CURRENT > 1 )) && after_args=("${(@)words[2,CURRENT]}")
  candidates=()
  descriptions=()
  while IFS=$'\t' read -r candidate description; do
    candidates+=("$candidate")
    descriptions+=("$description")
  done < <(command after __complete zsh "${after_args[@]}" 2>/dev/null)
  compadd -o nosort -d descriptions -- "${candidates[@]}"
}

compdef _after after
`

const fishCompletionScript = `# AFTER Fish completion.
function __after_complete
    set -l after_words (commandline -opc)
    if test (count $after_words) -gt 0
        set -e after_words[1]
    end
    set -a after_words (commandline -ct)
    command after __complete fish $after_words 2>/dev/null
end
complete -c after -k -f -a '(__after_complete)'
`

func completionShellRequest(args []string, stdout io.Writer) int {
	if len(args) == 0 {
		return ExitOK
	}
	if _, ok := completionScript(args[0]); !ok {
		return ExitOK
	}
	candidates, err := complete(args[1:])
	if err != nil {
		return ExitOK
	}
	for _, candidate := range candidates {
		if _, err := fmt.Fprintf(stdout, "%s\t%s\n", candidate.word, safeCompletionDescription(candidate.description)); err != nil {
			return ExitOK
		}
	}
	return ExitOK
}

func safeCompletionDescription(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return terminal.Line(value, 120)
}

func complete(words []string) ([]completionCandidate, error) {
	allCommands := commands(&invocation{})
	command, commandIndex := completionCommandFor(words, allCommands)
	if command == nil {
		if len(words) > 0 && strings.HasPrefix(words[len(words)-1], "-") {
			return completeFlags(&ucli.Command{Name: "after", Flags: globalFlags()}, words[len(words)-1]), nil
		}
		return completeCommands(words, allCommands), nil
	}
	args := words[commandIndex+1:]
	current := ""
	if len(args) > 0 {
		current = args[len(args)-1]
	}
	prior := args
	if len(prior) > 0 {
		prior = prior[:len(prior)-1]
	}
	if command.Name == "completion" {
		var candidates []completionCandidate
		for _, shell := range completionShells {
			if strings.HasPrefix(shell, current) {
				candidates = append(candidates, completionCandidate{word: shell, description: "shell completion script"})
			}
		}
		return candidates, nil
	}
	flags := completionFlagMap(command)
	expected, positionals, afterDash := completionPosition(prior, command, flags)
	if current == "" && expected != "" {
		return completeFlagValue(command.Name, expected, current, words, allCommands)
	}
	if !afterDash && strings.HasPrefix(current, "-") {
		if name, value, hasValue := strings.Cut(current, "="); hasValue && flags[name] {
			candidates, err := completeFlagValue(command.Name, name, value, words, allCommands)
			for i := range candidates {
				candidates[i].word = name + "=" + candidates[i].word
			}
			return prefixCandidates(candidates, current), err
		}
		return completeFlags(command, current), nil
	}
	if expected != "" {
		return completeFlagValue(command.Name, expected, current, words, allCommands)
	}
	if current != "" && !afterDash {
		if name, value, hasValue := strings.Cut(current, "="); hasValue && flags[name] {
			candidates, err := completeFlagValue(command.Name, name, value, words, allCommands)
			for i := range candidates {
				candidates[i].word = name + "=" + candidates[i].word
			}
			return prefixCandidates(candidates, current), err
		}
	}
	kinds, ok := completionPositionalKinds(command.Name, positionals, prior)
	if !ok || len(kinds) == 0 {
		return nil, nil
	}
	project, ok := completionProject(words, command)
	if !ok {
		return nil, nil
	}
	candidates, err := completionIDs(project, kinds)
	return prefixCandidates(candidates, current), err
}

func prefixCandidates(candidates []completionCandidate, prefix string) []completionCandidate {
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.word, prefix) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func completionCommandFor(words []string, allCommands []*ucli.Command) (*ucli.Command, int) {
	root := &ucli.Command{Name: "after", Flags: globalFlags()}
	rootFlags := completionFlagMap(root)
	for i := 0; i < len(words); i++ {
		token := words[i]
		if token == "--" {
			return nil, -1
		}
		name, _, hasValue := strings.Cut(token, "=")
		if rootFlags[name] {
			if completionFlagTakesValue(name, root) && !hasValue {
				i++
			}
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		for _, command := range allCommands {
			if token == command.Name {
				return command, i
			}
		}
		return nil, -1
	}
	return nil, -1
}

func completeCommands(words []string, allCommands []*ucli.Command) []completionCandidate {
	prefix := ""
	if len(words) > 0 {
		prefix = words[len(words)-1]
	}
	commandsByName := make(map[string]*ucli.Command, len(allCommands))
	for _, command := range allCommands {
		commandsByName[command.Name] = command
	}
	ordered := []string{"review", "capture", "diff", "log", "inspect", "run", "import", "compare", "pin", "export", "status", "config", "completion", "version", "help"}
	var candidates []completionCandidate
	for _, name := range ordered {
		command := commandsByName[name]
		if name == "help" || command != nil {
			if strings.HasPrefix(name, prefix) {
				candidates = append(candidates, completionCandidate{word: name, description: commandDescription(command, name)})
			}
		}
	}
	return candidates
}

func commandDescription(command *ucli.Command, name string) string {
	if command == nil {
		return "show command help"
	}
	return command.Usage
}

func completionFlagMap(command *ucli.Command) map[string]bool {
	flags := map[string]bool{}
	for _, flag := range globalFlags() {
		for _, name := range flag.Names() {
			flags["--"+name] = true
		}
	}
	if command != nil {
		for _, flag := range command.Flags {
			for _, name := range flag.Names() {
				flags["--"+name] = true
				if len(name) == 1 {
					flags["-"+name] = true
				}
			}
		}
	}
	flags["--help"] = true
	flags["-h"] = true
	if command != nil && command.Name == "after" {
		flags["--version"] = true
		flags["-v"] = true
	}
	return flags
}

func completionFlagTakesValue(name string, command *ucli.Command) bool {
	if command != nil {
		for _, flag := range command.Flags {
			for _, option := range flag.Names() {
				if "--"+option == name || (len(option) == 1 && "-"+option == name) {
					return !isBoolCLIFlag(flag)
				}
			}
		}
	}
	for _, flag := range globalFlags() {
		for _, option := range flag.Names() {
			if "--"+option == name {
				return !isBoolCLIFlag(flag)
			}
		}
	}
	return false
}

func isBoolCLIFlag(flag ucli.Flag) bool {
	_, ok := flag.(*ucli.BoolFlag)
	return ok
}

func completeFlags(command *ucli.Command, prefix string) []completionCandidate {
	flags := completionFlagMap(command)
	candidates := []completionCandidate{}
	for name := range flags {
		if strings.HasPrefix(name, prefix) {
			candidates = append(candidates, completionCandidate{word: name, description: "command option"})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].word < candidates[j].word })
	return candidates
}

func completionPosition(prior []string, command *ucli.Command, flags map[string]bool) (expected string, positionals []string, afterDash bool) {
	for i := 0; i < len(prior); i++ {
		token := prior[i]
		if token == "--" {
			afterDash = true
			continue
		}
		name, _, hasValue := strings.Cut(token, "=")
		if !afterDash && strings.HasPrefix(token, "-") {
			if flags[name] && completionFlagTakesValue(name, command) && !hasValue {
				if i+1 < len(prior) {
					i++
				} else {
					expected = name
				}
			}
			continue
		}
		positionals = append(positionals, token)
	}
	if len(prior) > 0 {
		last := prior[len(prior)-1]
		if flags[last] && completionFlagTakesValue(last, command) {
			expected = last
		}
	}
	return expected, positionals, afterDash
}

func completeFlagValue(command, flag, prefix string, words []string, allCommands []*ucli.Command) ([]completionCandidate, error) {
	values := []completionCandidate{}
	switch flag {
	case "--scope":
		values = []completionCandidate{
			{word: string(evidence.FiniteExample), description: "finite example"},
			{word: string(evidence.HumanIntent), description: "human intent"},
		}
	case "--mode":
		values = []completionCandidate{
			{word: string(evidence.OriginalBase), description: "compare against the original base"},
			{word: string(evidence.FollowUp), description: "compare against the last inspected snapshot"},
		}
	case "--snapshot", "--select", "--attach":
		kind := "snapshot"
		if flag == "--attach" {
			kind = "receipt"
		}
		commandDef, _ := completionCommandFor(words, allCommands)
		if commandDef == nil || commandDef.Name != command {
			return nil, nil
		}
		project, ok := completionProject(words, commandDef)
		if !ok {
			return nil, nil
		}
		candidates, err := completionIDs(project, []string{kind})
		if err != nil {
			return nil, nil
		}
		values = candidates
	default:
		return nil, nil
	}
	filtered := values[:0]
	for _, candidate := range values {
		if strings.HasPrefix(candidate.word, prefix) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func completionPositionalKinds(command string, positionals, prior []string) ([]string, bool) {
	position := len(positionals)
	switch command {
	case "inspect", "export":
		if position == 0 {
			return inspectKinds, true
		}
		if position == 1 {
			return []string{"snapshot"}, true
		}
	case "compare":
		if position == 0 {
			return []string{"receipt"}, true
		}
	case "diff", "run":
		if command == "run" && hasCompletionFlag(prior, "--plan-file", "--approve") {
			return nil, false
		}
		if position < 2 {
			return []string{"snapshot"}, true
		}
	case "pin":
		if position == 0 {
			if hasCompletionFlag(prior, "--expectation") {
				return []string{"receipt"}, true
			}
			return []string{"pin"}, true
		}
	case "review":
		if position == 0 {
			return []string{"snapshot", "receipt", "comparison", "report", "pin"}, true
		}
		if position == 1 {
			return []string{"snapshot"}, true
		}
		if position <= 34 {
			return browserKinds, true
		}
	}
	return nil, false
}

func hasCompletionFlag(words []string, names ...string) bool {
	for _, word := range words {
		for _, name := range names {
			if word == name || strings.HasPrefix(word, name+"=") {
				return true
			}
		}
	}
	return false
}

func completionProject(words []string, command *ucli.Command) (string, bool) {
	values := map[string]config.FlagValue{}
	allFlags := map[string]bool{}
	for name := range completionFlagMap(command) {
		allFlags[name] = completionFlagTakesValue(name, command)
	}
	for i := 0; i < len(words); i++ {
		token := words[i]
		if token == "--" {
			break
		}
		name, value, hasValue := strings.Cut(token, "=")
		if (name == "--project" || name == "--config") && hasValue {
			key := strings.TrimPrefix(name, "--")
			values[key] = config.FlagValue{Value: value, Set: true}
			continue
		}
		if name == "--project" || name == "--config" {
			if i+1 < len(words) {
				key := strings.TrimPrefix(name, "--")
				values[key] = config.FlagValue{Value: words[i+1], Set: true}
				i++
			}
			continue
		}
		if allFlags[name] && !hasValue && completionFlagTakesValue(name, command) {
			if i+1 < len(words) {
				i++
			}
		}
	}
	cfg, err := config.Load(config.Input{Flags: values})
	if err != nil {
		return "", false
	}
	root, ok := projectRoot(cfg.Project)
	if !ok {
		return "", false
	}
	return root, true
}

func completionIDs(project string, kinds []string) ([]completionCandidate, error) {
	s, err := store.Open(project, false, nil)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	allowed := map[string]bool{}
	storageKinds := []string{}
	seenStorage := map[string]bool{}
	for _, kind := range kinds {
		allowed[kind] = true
		storageKind := kind
		if kind == "report" || kind == "artifact" {
			storageKind = "blob"
		}
		if !seenStorage[storageKind] {
			storageKinds = append(storageKinds, storageKind)
			seenStorage[storageKind] = true
		}
	}
	entries, err := s.List(storageKinds...)
	if err != nil {
		return nil, err
	}
	var readBytes int64
	for _, entry := range entries {
		if entry.Size < 0 || entry.Size > completionIDReadLimit-readBytes {
			return nil, store.ErrLimit
		}
		readBytes += entry.Size
	}
	captureTimes := map[evidence.Digest]time.Time{}
	if allowed["snapshot"] {
		captureEntries, err := s.List("capture")
		if err != nil {
			return nil, err
		}
		for _, entry := range captureEntries {
			if entry.Size < 0 || entry.Size > completionIDReadLimit-readBytes {
				return nil, store.ErrLimit
			}
			readBytes += entry.Size
			capture, err := store.Get[evidence.Capture](s, entry.ID)
			if err != nil {
				return nil, err
			}
			for _, id := range []evidence.Digest{capture.Base, capture.Candidate, capture.Index} {
				if id != "" && capture.CapturedAt.After(captureTimes[id]) {
					captureTimes[id] = capture.CapturedAt
				}
			}
		}
	}
	type timedCandidate struct {
		completionCandidate
		at time.Time
	}
	matches := make([]timedCandidate, 0, len(entries))
	for _, entry := range entries {
		kind, description, err := describeID(s, entry)
		if err != nil {
			return nil, err
		}
		if !allowed[kind] {
			continue
		}
		at, err := completionRecordTime(s, entry, kind, captureTimes)
		if err != nil {
			return nil, err
		}
		matches = append(matches, timedCandidate{
			completionCandidate: completionCandidate{word: shortID(entry.ID), description: kind + ": " + description},
			at:                  at,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].at.Equal(matches[j].at) {
			return matches[i].word > matches[j].word
		}
		return matches[i].at.After(matches[j].at)
	})
	if len(matches) > 50 {
		matches = matches[:50]
	}
	result := make([]completionCandidate, len(matches))
	for i := range matches {
		result[i] = matches[i].completionCandidate
	}
	return result, nil
}

func completionRecordTime(s *store.Store, entry store.Entry, kind string, captureTimes map[evidence.Digest]time.Time) (time.Time, error) {
	switch kind {
	case "snapshot":
		return captureTimes[entry.ID], nil
	case "receipt":
		record, err := store.Get[evidence.Receipt](s, entry.ID)
		return record.FinishedAt, err
	case "comparison":
		record, err := store.Get[evidence.Comparison](s, entry.ID)
		if err != nil {
			return time.Time{}, err
		}
		receipt, err := store.Get[evidence.Receipt](s, record.Receipt)
		return receipt.FinishedAt, err
	case "pin":
		record, err := store.Get[evidence.Pin](s, entry.ID)
		if err != nil {
			return time.Time{}, err
		}
		return record.History[len(record.History)-1].At, nil
	case "report":
		raw, err := s.ReadBlob(entry.ID)
		if err != nil {
			return time.Time{}, err
		}
		var report gotestreport.Report
		if strictJSON(raw, &report) != nil || !validImportedReport(report) {
			return time.Time{}, store.ErrCorrupt
		}
		return report.Metadata.ImportedAt, nil
	default:
		return time.Time{}, nil
	}
}

func runCompletionBackend(args []string, stdout io.Writer) int {
	return completionShellRequest(args, stdout)
}
