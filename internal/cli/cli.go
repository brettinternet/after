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

	"github.com/brettinternet/after/internal/config"
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
	ctx        context.Context
	stdout     io.Writer
	stderr     io.Writer
	reader     io.Reader
	tty        bool
	exit       int
	diagnostic string
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
	state := &invocation{ctx: ctx, stdout: stdout, stderr: stderr, reader: reader, tty: tty}
	app := &ucli.App{
		Name:                      "after",
		Usage:                     "local change evidence without implicit project execution",
		Description:               "Capture local Git, import Go reports, inspect evidence and review pins in a TUI.\n   Execution supports only the frozen offline payment experiment with exact consent.\n   No general application adapter, GitHub sync, or universal behavior guarantee.",
		Version:                   Version,
		Writer:                    stdout,
		ErrWriter:                 stderr,
		Reader:                    reader,
		DisableSliceFlagSeparator: true,
		HideHelpCommand:           false,
		Action: func(ctx *ucli.Context) error {
			if ctx.NArg() > 0 {
				return invalid("unknown command; use --help")
			}
			return ucli.ShowAppHelp(ctx)
		},
		OnUsageError:   usageError,
		ExitErrHandler: func(*ucli.Context, error) {},
		CommandNotFound: func(*ucli.Context, string) {
			state.exit = ExitInvalid
			state.diagnostic = "after: unknown command; use --help"
		},
		Commands: commands(state),
	}
	for _, command := range app.Commands {
		if command.OnUsageError == nil {
			command.OnUsageError = usageError
		}
	}
	if len(args) > 0 && !knownCommand(args[0]) && !strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, "after: unknown command; use --help")
		return ExitInvalid
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
			fmt.Fprintf(stderr, "after: invalid configuration setting %s (%s): %s\n", configErr.Setting, configErr.Source, configErr.Reason)
			return ExitInvalid
		}
		fmt.Fprintln(stderr, "after: operation failed")
		return ExitOperational
	}
	if state.diagnostic != "" {
		fmt.Fprintln(stderr, state.diagnostic)
	}
	return state.exit
}

func knownCommand(value string) bool {
	switch value {
	case "capture", "import", "inspect", "compare", "export", "run", "pin", "review", "config", "version", "help", "h":
		return true
	default:
		return false
	}
}

// urfave/cli uses Go's flag parser, which stops at the first positional value.
// Move only recognized command flags before positionals so both common CLI
// spellings work, without interpreting unknown or repository-controlled text.
func normalizeArgs(args []string) []string {
	if len(args) < 2 {
		return args
	}
	valueFlags := map[string]bool{
		"--config": true, "--project": true, "--docker-binary": true, "--docker-host": true,
		"--repetitions": true, "--run-seconds": true, "--output-bytes": true, "--diff-bytes": true,
		"--base": true, "--target": true, "--include-untracked": true, "--snapshot": true,
		"--producer": true, "--captured-at": true,
		"--offset": true, "--limit": true, "--diff-offset": true, "--diff-size": true,
		"--inventory-offset": true, "--inventory-limit": true, "--card-offset": true,
		"--card-limit": true, "--plan-file": true, "--plan-out": true, "--approve": true,
		"--artifact-offset": true, "--artifact-size": true,
		"--evidence": true, "--import-file": true,
		"--expectation": true, "--scope": true, "--reason": true, "--select": true, "--mode": true, "--receipt": true,
	}
	boolFlags := map[string]bool{"--tui": true, "--accept": true, "--interactive": true, "--raw-diff": true, "--staged": true, "--help": true, "-h": true}
	var flags, positionals []string
	for i := 1; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i:]...)
			break
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
		if strings.HasPrefix(token, "-") && len(positionals) == 0 {
			flags = append(flags, token)
			continue
		}
		positionals = append(positionals, token)
	}
	out := make([]string, 1, len(args))
	out[0] = args[0]
	out = append(out, flags...)
	out = append(out, positionals...)
	return out
}

func commonFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "config", Usage: "select a YAML configuration file"},
		&ucli.StringFlag{Name: "project", Usage: "select the project directory"},
		&ucli.StringFlag{Name: "docker-binary", Usage: "trusted absolute Docker CLI path"},
		&ucli.StringFlag{Name: "docker-host", Usage: "explicit local unix:/// Docker socket"},
		&ucli.IntFlag{Name: "repetitions", Usage: "paired run repetitions (1-5)"},
		&ucli.IntFlag{Name: "run-seconds", Usage: "sandbox time limit (1-300 seconds)"},
		&ucli.IntFlag{Name: "output-bytes", Usage: "per-container output limit (1-1048576)"},
		&ucli.BoolFlag{Name: "interactive", Usage: "allow an exact-plan confirmation prompt on a terminal"},
		&ucli.BoolFlag{Name: "raw-diff", Usage: "include a bounded captured patch"},
		&ucli.IntFlag{Name: "diff-bytes", Usage: "maximum raw patch bytes (0-65536)"},
	}
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
	return config.Load(config.Input{Flags: values})
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

func usageError(_ *ucli.Context, _ error, _ bool) error {
	return invalid("invalid command arguments or flags")
}

func requireArgs(ctx *ucli.Context, count int) error {
	if ctx.NArg() != count {
		return invalid("unexpected or missing command arguments")
	}
	return nil
}

func invalid(message string) error {
	return &exitError{code: ExitInvalid, diagnostic: "after: invalid input: " + message}
}

func operational(message string) error {
	return &exitError{code: ExitOperational, diagnostic: "after: " + message}
}

func terminalInput(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func prompt(state *invocation, digest string, preview []byte) bool {
	if _, err := fmt.Fprintf(state.stderr, "Exact execution plan %s:\n%s\nType yes to authorize this plan: ", digest, preview); err != nil {
		return false
	}
	reader := bufio.NewReaderSize(state.reader, 256)
	line, err := reader.ReadSlice('\n')
	return err == nil && len(line) <= 256 && strings.TrimSpace(string(line)) == "yes"
}
