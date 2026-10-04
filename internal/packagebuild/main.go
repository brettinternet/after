// Command packagebuild produces unpublished native binaries and SHA-256 sums.
package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var targets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}

func build(parent string) error {
	revision, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return err
	}
	version := "0.1.0-dev." + strings.TrimSpace(string(revision))
	status, err := exec.Command("git", "status", "--porcelain", "--untracked-files=normal").Output()
	if err != nil {
		return err
	}
	if len(status) != 0 {
		version += ".dirty"
	}
	// A unique directory avoids overwriting or mixing older release artifacts.
	if err = os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, version+"-")
	if err != nil {
		return err
	}
	fmt.Println("Unpublished package:", dir)
	var sums strings.Builder
	for _, target := range targets {
		pair := strings.Split(target, "/")
		name := "after_" + version + "_" + pair[0] + "_" + pair[1]
		path := filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags=-X github.com/brettinternet/after/internal/cli.Version="+version, "-o", path, "./cmd/after")
		cmd.Env = append(os.Environ(), "GOOS="+pair[0], "GOARCH="+pair[1], "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOFLAGS=", "GOAMD64=v1", "GOARM64=v8.0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			return err
		}
		settings := map[string]string{}
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
		baselineKey, baseline := "GOAMD64", "v1"
		if pair[1] == "arm64" {
			baselineKey, baseline = "GOARM64", "v8.0"
		}
		if info.GoVersion != runtime.Version() || settings["GOOS"] != pair[0] || settings["GOARCH"] != pair[1] || settings["CGO_ENABLED"] != "0" || settings[baselineKey] != baseline {
			return fmt.Errorf("packaged toolchain/architecture mismatch: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(data), name)
		if target == runtime.GOOS+"/"+runtime.GOARCH {
			output, err := exec.Command(path, "--version").Output()
			if err != nil {
				return err
			}
			if !strings.Contains(string(output), version) {
				return fmt.Errorf("packaged version mismatch: %q", output)
			}
			if err = exec.Command(path, "--help").Run(); err != nil {
				return err
			}
		}
	}
	if err = os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums.String()), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "BUILD.txt"), []byte("version="+version+"\ntoolchain="+runtime.Version()+"\nCGO_ENABLED=0\nNo signing, publication or cross-host execution claimed.\n"), 0644)
}
func main() {
	if err := build("dist"); err != nil {
		fmt.Fprintln(os.Stderr, "PACKAGE FAILED:", err)
		os.Exit(1)
	}
}
