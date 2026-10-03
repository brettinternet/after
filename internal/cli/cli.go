// Package cli provides the side-effect-free foundation command.
package cli

import (
	"fmt"
	"io"
)

const Version = "0.1.0-dev"

const help = `AFTER — local change evidence (foundation only)

Usage:
  after help | --help | -h
  after version | --version

No evidence loaded. Capture, import, execution and review are not implemented.
This command does not inspect a repository or execute project commands.

Exit status: 0 help/version, 1 output failure, 2 invalid arguments.
`

// Run writes only to the supplied streams. It does not read stdin, files or
// environment variables, and cannot launch processes or make network requests.
func Run(args []string, stdout, stderr io.Writer) int {
	output := help
	if len(args) > 1 {
		fmt.Fprintln(stderr, "after: expected help or version; use --help")
		return 2
	}
	if len(args) == 1 {
		switch args[0] {
		case "help", "--help", "-h":
		case "version", "--version":
			output = "after " + Version + "\n"
		default:
			// Never echo untrusted arguments into a terminal.
			fmt.Fprintln(stderr, "after: unknown argument; use --help")
			return 2
		}
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintln(stderr, "after: unable to write output")
		return 1
	}
	return 0
}
