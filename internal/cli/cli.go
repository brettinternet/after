// Package cli wires the bounded evidence APIs to urfave/cli.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/terminal"
	ucli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// Version is replaced by the unpublished package build task.
var Version = "0.1.0-dev"

const (
	MaxCLIOutput    = 16 << 20
	ExitOK          = 0
	ExitOperational = 1
	ExitInvalid     = 2
	ExitDenied      = 3
	ExitFinding     = 4
)

type invocation struct {
	ctx             context.Context
	stdout          io.Writer
	stderr          io.Writer
	reader          io.Reader
	tty             bool
	stdoutTTY       bool
	stderrTTY       bool
	columns         int
	project         string
	jsonOutput      bool
	forceJSON       bool
	suggestionFlags string
	theme           terminal.Theme
	location        *time.Location
	now             func() time.Time
	exit            int
	diagnostic      string
}

type exitError struct {
	code       int
	diagnostic string
}

func (e *exitError) Error() string { return e.diagnostic }

// Run executes the command layer with separate, caller-supplied output streams.
func Run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, args, stdout, stderr, os.Stdin, terminalInput(os.Stdin))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, reader io.Reader, tty bool) int {
	state := &invocation{
		ctx: ctx, stdout: stdout, stderr: stderr, reader: reader, tty: tty,
		stdoutTTY: terminalOutput(stdout), stderrTTY: terminalOutput(stderr), columns: 80,
		theme: terminal.DefaultTheme(), location: time.Local, now: time.Now,
	}
	if file, ok := stdout.(*os.File); ok && state.stdoutTTY {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			state.columns = width
		}
	}
	if len(args) > 0 && args[0] == "__complete" {
		return runCompletionBackend(args[1:], stdout)
	}
	if name, requested, helpErr := helpRequest(args); requested {
		if helpErr != nil {
			fmt.Fprintln(stderr, helpErr.Error())
			return ExitInvalid
		}
		_, _ = io.WriteString(stdout, commandHelp(name))
		return ExitOK
	}
	if migration := removedForm(args); migration != nil {
		fmt.Fprintln(stderr, migration.Error())
		return ExitInvalid
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !knownCommand(args[0]) {
		fmt.Fprintln(stderr, unknownCommandDiagnostic(args[0]))
		return ExitInvalid
	}
	app := &ucli.App{
		Name:                      "after",
		Usage:                     "local change evidence without implicit project execution",
		Description:               "Capture local Git, import Go reports, inspect evidence and review pins in a TUI.\n   Execution supports only the frozen offline payment experiment with exact consent.\n   No general application adapter, GitHub sync, or universal behavior guarantee.\n   Stored IDs accept unique 4+ hex prefixes (optional sha256:, any case).\n   --approve requires the full lowercase sha256: digest; prefixes never authorize execution.",
		Version:                   Version,
		Writer:                    stdout,
		ErrWriter:                 stderr,
		Reader:                    reader,
		DisableSliceFlagSeparator: true,
		HideHelpCommand:           false,
		Flags:                     append(globalFlags(), storedFlag()),
		Action: func(ctx *ucli.Context) error {
			if ctx.NArg() > 0 {
				return invalidWithFix("unexpected arguments", "run after --help to list available commands")
			}
			state.jsonOutput = ctx.Bool("json")
			cfg, err := configFlags(ctx)
			if err != nil {
				var commandErr *exitError
				if errors.As(err, &commandErr) && commandErr.code == ExitInvalid && strings.Contains(commandErr.diagnostic, "not inside a Git repository") {
					_, writeErr := io.WriteString(stdout, topLevelHelp())
					return writeErr
				}
				return err
			}
			state.suggestionFlags = suggestionFlags(ctx)
			return statusCommand(state, cfg.Project, ctx.Bool("stored"))
		},
		OnUsageError:   usageError,
		ExitErrHandler: func(*ucli.Context, error) {},
		CommandNotFound: func(_ *ucli.Context, name string) {
			state.exit = ExitInvalid
			state.diagnostic = unknownCommandDiagnostic(name)
		},
		Commands: commands(state),
	}
	for _, command := range app.Commands {
		if command.OnUsageError == nil {
			command.OnUsageError = usageError
		}
	}
	err := app.RunContext(ctx, append([]string{"after"}, normalizeArgs(args)...))
	if err != nil {
		var commandErr *exitError
		if errors.As(err, &commandErr) {
			if commandErr.diagnostic != "" {
				fmt.Fprintln(stderr, commandErr.diagnostic)
			}
			return commandErr.code
		}
		var configErr *config.Error
		if errors.As(err, &configErr) {
			fix := "correct that setting and retry after config"
			if configErr.Setting == "docker_binary" || configErr.Setting == "docker_host" {
				fix = dockerConfigFix()
			}
			fmt.Fprintln(stderr, formatDiagnostic(fmt.Sprintf("invalid configuration setting %s (%s): %s", configErr.Setting, configErr.Source, configErr.Reason), fix))
			return ExitInvalid
		}
		fmt.Fprintln(stderr, "after: operation failed — check the local checkout and configuration, then retry")
		return ExitOperational
	}
	if state.diagnostic != "" {
		fmt.Fprintln(stderr, state.diagnostic)
	}
	return state.exit
}

func knownCommand(value string) bool {
	switch value {
	case "capture", "import", "inspect", "compare", "export", "run", "pin", "review", "status", "log", "diff", "config", "completion", "version", "help":
		return true
	default:
		return false
	}
}

// urfave/cli uses Go's flag parser, which stops at the first positional value.
// Move flags before positionals so both common spellings work and unknown flags
// reach the command parser for a useful suggestion. Use -- for dash-prefixed IDs.
func normalizeArgs(args []string) []string {
	if len(args) < 2 {
		return args
	}
	commandFirst := !strings.HasPrefix(args[0], "-")
	start := 0
	if commandFirst {
		start = 1
	}
	valueFlags := map[string]bool{
		"--config": true, "--project": true, "--docker-binary": true, "--docker-host": true,
		"--repetitions": true, "--run-seconds": true, "--output-bytes": true, "--diff-bytes": true,
		"--base": true, "--target": true, "--include-untracked": true, "--snapshot": true,
		"--producer": true, "--captured-at": true,
		"--offset": true, "--limit": true, "-n": true, "--n": true, "--diff-offset": true, "--diff-size": true,
		"--inventory-offset": true, "--inventory-limit": true, "--card-offset": true,
		"--card-limit": true, "--plan-file": true, "--plan-out": true, "--approve": true,
		"--artifact-offset": true, "--artifact-size": true,
		"--evidence": true, "--import-file": true,
		"--expectation": true, "--scope": true, "--reason": true, "--select": true, "--mode": true, "--receipt": true, "--attach": true,
	}
	boolFlags := map[string]bool{"--tui": true, "--accept": true, "--interactive": true, "--raw-diff": true, "--staged": true, "--new": true, "--json": true, "--raw": true, "--stat": true, "--help": true, "-h": true}
	var flags, positionals []string
	for i := start; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if token == "-" {
			positionals = append(positionals, token)
			continue
		}
		name, _, hasValue := strings.Cut(token, "=")
		if valueFlags[name] || boolFlags[name] {
			flags = append(flags, token)
			if valueFlags[name] && !hasValue {
				if i+1 == len(args) {
					continue
				}
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		if strings.HasPrefix(token, "-") {
			flags = append(flags, token)
			continue
		}
		positionals = append(positionals, token)
	}
	out := make([]string, 0, len(args))
	if commandFirst {
		out = append(out, args[0])
	}
	out = append(out, flags...)
	out = append(out, positionals...)
	return out
}

func globalFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "config", Usage: "select a YAML configuration file"},
		&ucli.StringFlag{Name: "project", Usage: "select a checkout path; root is its nearest .git ancestor"},
		&ucli.BoolFlag{Name: "json", Usage: "print the versioned JSON result"},
	}
}

func runFlags() []ucli.Flag {
	defaults := config.Defaults()
	return append(globalFlags(),
		&ucli.StringFlag{Name: "docker-binary", Usage: "trusted absolute Docker CLI path"},
		&ucli.StringFlag{Name: "docker-host", Usage: "explicit local unix:/// Docker socket"},
		&ucli.IntFlag{Name: "repetitions", Value: defaults.Repetitions, Usage: "paired run repetitions (1-5)"},
		&ucli.IntFlag{Name: "run-seconds", Value: defaults.RunSeconds, Usage: "sandbox time limit (1-300 seconds)"},
		&ucli.IntFlag{Name: "output-bytes", Value: defaults.OutputBytes, Usage: "per-container output limit (1-1048576)"},
		&ucli.BoolFlag{Name: "interactive", Value: defaults.Interactive, Usage: "allow an exact-plan confirmation prompt on a terminal"},
	)
}

func configFlags(ctx *ucli.Context) (config.Config, error) {
	values := map[string]config.FlagValue{}
	for cliName, key := range map[string]string{
		"config": "config", "project": "project", "docker-binary": "docker_binary",
		"docker-host": "docker_host", "repetitions": "repetitions", "run-seconds": "run_seconds",
		"output-bytes": "output_bytes", "interactive": "interactive", "raw-diff": "raw_diff",
		"diff-bytes": "diff_bytes",
	} {
		if !ctx.IsSet(cliName) {
			continue
		}
		var value any
		switch key {
		case "interactive", "raw_diff":
			value = ctx.Bool(cliName)
		case "repetitions", "run_seconds", "output_bytes", "diff_bytes":
			value = ctx.Int(cliName)
		default:
			value = ctx.String(cliName)
		}
		values[key] = config.FlagValue{Value: value, Set: true}
	}
	cfg, err := config.Load(config.Input{Flags: values})
	if err != nil {
		return cfg, err
	}
	if ctx.Command != nil && ctx.Command.Name == "config" {
		return cfg, nil
	}
	root, ok := projectRoot(cfg.Project)
	if !ok {
		return config.Config{}, &exitError{
			code:       ExitInvalid,
			diagnostic: "after: not inside a Git repository — run AFTER in a checkout, or pass --project DIR",
		}
	}
	cfg.Project = root
	return cfg, nil
}

func writeResult(state *invocation, kind string, data any) error {
	if state.jsonOutput || state.forceJSON || kind == "export" {
		return writeJSON(state, kind, data)
	}
	return writeReadable(state, kind, data)
}

func writeJSON(state *invocation, kind string, data any) error {
	result := struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Data          any    `json:"data"`
	}{1, kind, data}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxCLIOutput {
		return operational("machine-readable response exceeds output limit")
	}
	encoded = append(encoded, '\n')
	n, err := state.stdout.Write(encoded)
	if err != nil || n != len(encoded) {
		return operational("cannot write machine-readable response")
	}
	return nil
}

func usageError(ctx *ucli.Context, err error, _ bool) error {
	command := "after"
	var flags []ucli.Flag
	if ctx != nil && ctx.Command != nil {
		command = ctx.Command.Name
		flags = ctx.Command.Flags
	}
	if name, ok := unknownFlagName(err); ok {
		return unknownFlagError(command, name, flags)
	}
	if command == "after" {
		return invalidWithFix("invalid command arguments or flag value", "run after --help to list available commands")
	}
	return invalidWithFix("invalid command arguments or flag value", "check after "+command+" --help for valid syntax")
}

func requireArgs(ctx *ucli.Context, count int) error {
	if ctx.NArg() == count {
		return nil
	}
	command := ctx.Command.Name
	if ctx.NArg() < count {
		return invalidWithFix(missingArgument(command, count-ctx.NArg()), "try: "+commandExample(command))
	}
	return invalidWithFix("unexpected arguments", "try: "+commandExample(command))
}

func invalid(message string) error {
	return invalidWithFix("invalid input: "+message, "check the command's --help for a valid form")
}

func invalidWithFix(problem, fix string) error {
	return &exitError{code: ExitInvalid, diagnostic: formatDiagnostic(problem, fix)}
}

func operational(message string) error {
	return &exitError{code: ExitOperational, diagnostic: formatDiagnostic(message, "check the local checkout and configuration, then retry")}
}

func terminalInput(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func terminalOutput(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func outputBefore(state *invocation) func(*ucli.Context) error {
	return func(ctx *ucli.Context) error {
		state.jsonOutput = ctx.Bool("json")
		state.forceJSON = ctx.Command != nil && ctx.Command.Name == "export"
		state.suggestionFlags = suggestionFlags(ctx)
		return validateIDInputs(ctx)
	}
}

func startElapsedNotice(state *invocation, action string, delay time.Duration) func() {
	return startElapsedNoticeWith(state, action, delay, nil)
}

func startElapsedNoticeWith(state *invocation, action string, delay time.Duration, noticed chan<- struct{}) func() {
	if !state.stderrTTY {
		return func() {}
	}
	started := time.Now()
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			_, _ = fmt.Fprintf(state.stderr, "after: %s still running · %s\n", action, time.Since(started).Round(time.Second))
			if noticed != nil {
				noticed <- struct{}{}
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

func prompt(state *invocation, digest string, summary []byte, summaryErr error, planBytes int, using *resolvedIDs) bool {
	if _, err := fmt.Fprintf(state.stderr, "Nothing has run. Plan size %s.\nExact authorization digest:\n%s\n", formatPlanSize(planBytes), digest); err != nil {
		return false
	}
	if using != nil {
		message := terminal.Line(usingCaptureLine(*using).text, max(1, state.columns))
		if _, err := fmt.Fprintln(state.stderr, message); err != nil {
			return false
		}
	}
	if summaryErr != nil {
		message := terminal.Line(summaryErr.Error(), max(1, state.columns-len("Consent summary unavailable: ")))
		if _, err := fmt.Fprintf(state.stderr, "Consent summary unavailable: %s\n", message); err != nil {
			return false
		}
	} else {
		if _, err := io.WriteString(state.stderr, "Consent summary:\n"); err != nil {
			return false
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(summary), "\n"), "\n") {
			message := terminal.Line(line, max(1, state.columns-2))
			if _, err := fmt.Fprintf(state.stderr, "  %s\n", message); err != nil {
				return false
			}
		}
	}
	if _, err := fmt.Fprint(state.stderr, "Type yes to run exactly these plan bytes: "); err != nil {
		return false
	}
	reader := bufio.NewReaderSize(state.reader, 256)
	line, err := reader.ReadSlice('\n')
	return err == nil && len(line) <= 256 && strings.TrimSpace(string(line)) == "yes"
}
