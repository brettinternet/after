//go:build ignore

// Trusted one-request helper: compile only AFTER's embedded launcher and stream
// the completed executable to stdout before this compiler container exits.
package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/local/go/bin/go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", "/work/after-launcher", "/input/launch.go")
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	binary, err := os.Open("/work/after-launcher")
	if err != nil {
		os.Exit(1)
	}
	defer binary.Close()
	if _, err := io.Copy(os.Stdout, io.LimitReader(binary, (8<<20)+1)); err != nil {
		os.Exit(1)
	}
}
