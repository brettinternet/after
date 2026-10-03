// Command after is the native entry point. Opening it never executes project code.
package main

import (
	"os"

	"github.com/brettinternet/after/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
