package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/terminal"
)

type dockerSetup struct {
	Problems    []string `json:"problems"`
	Suggestions []string `json:"suggested_settings,omitempty"`
	// Apply exports detected values for one shell only when each missing
	// setting has exactly one detected candidate. Copying it runs nothing.
	Apply string `json:"apply_command,omitempty"`
}

// dockerSetupStatus uses configuration and filesystem metadata only. It never
// runs the Docker CLI, opens a socket, chooses an endpoint, or changes config.
func dockerSetupStatus(cfg config.Config) dockerSetup {
	status := dockerSetup{Problems: []string{}}
	binaryValid := validDockerBinary(cfg.DockerBinary)
	socketValid := validDockerSocket(cfg.DockerHost)
	switch {
	case cfg.DockerBinary == "" && cfg.DockerHost == "":
		status.Problems = append(status.Problems, "Docker execution is not configured; set docker_binary and docker_host as a pair.")
	case cfg.DockerBinary == "":
		status.Problems = append(status.Problems, "docker_binary is not configured; docker_binary and docker_host must be set as a pair.")
	case cfg.DockerHost == "":
		status.Problems = append(status.Problems, "docker_host is not configured; docker_binary and docker_host must be set as a pair.")
	}
	if cfg.DockerBinary != "" && !binaryValid {
		status.Problems = append(status.Problems, "docker_binary must name an existing executable file.")
	}
	if cfg.DockerHost != "" && !socketValid {
		status.Problems = append(status.Problems, "docker_host must name an existing local Unix socket file.")
	}
	if len(status.Problems) == 0 {
		return status
	}
	var exports []string
	ambiguous := false
	if cfg.DockerBinary == "" || !binaryValid {
		binarySuggestion := detectedDockerBinary()
		comment := ""
		if binarySuggestion == "" {
			binarySuggestion = "/absolute/path/to/docker"
			ambiguous = true
		} else {
			comment = "  # found on PATH; not executed"
			exports = append(exports, "AFTER_DOCKER_BINARY="+shellQuote(binarySuggestion))
		}
		status.Suggestions = append(status.Suggestions, "docker_binary: "+strconv.Quote(binarySuggestion)+comment)
	}
	if cfg.DockerHost == "" || !socketValid {
		sockets := detectedDockerSockets()
		if len(sockets) > 0 {
			for _, socket := range sockets {
				status.Suggestions = append(status.Suggestions, "docker_host: "+strconv.Quote("unix://"+socket)+"  # existing socket; not contacted")
			}
		} else {
			status.Suggestions = append(status.Suggestions, `docker_host: "unix:///absolute/path/to/local/docker.sock"`)
		}
		if len(sockets) == 1 {
			exports = append(exports, "AFTER_DOCKER_HOST="+shellQuote("unix://"+sockets[0]))
		} else {
			ambiguous = true
		}
	}
	if !ambiguous && len(exports) > 0 {
		status.Apply = "export " + strings.Join(exports, " ")
	}
	return status
}

func validDockerBinary(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func validDockerSocket(host string) bool {
	if !strings.HasPrefix(host, "unix:///") || strings.ContainsAny(host, "\x00\r\n") {
		return false
	}
	info, err := os.Stat(strings.TrimPrefix(host, "unix://"))
	return err == nil && info.Mode()&os.ModeSocket != 0
}

func detectedDockerBinary() string {
	path, err := exec.LookPath("docker")
	if err != nil {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil || !validDockerBinary(absolute) || !printableArgument(absolute) {
		return ""
	}
	return absolute
}

func detectedDockerSockets() []string {
	candidates := []string{"/var/run/docker.sock", "/run/docker.sock"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".colima", "default", "docker.sock"))
	}
	seen := map[string]bool{}
	var seenSockets []os.FileInfo
	var sockets []string
	for _, candidate := range candidates {
		if seen[candidate] || !filepath.IsAbs(candidate) || !printableArgument(candidate) {
			continue
		}
		seen[candidate] = true
		info, err := os.Stat(candidate)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}
		duplicate := false
		for _, known := range seenSockets {
			if os.SameFile(known, info) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			sockets = append(sockets, candidate)
			seenSockets = append(seenSockets, info)
		}
	}
	return sockets
}

func dockerSetupError(status dockerSetup) error {
	problem := strings.Join(status.Problems, " ")
	fix := "configure the named Docker setting(s), then retry; AFTER has not contacted Docker"
	if len(status.Suggestions) > 0 {
		fix += ": " + strings.Join(status.Suggestions, "; ")
	}
	return &exitError{code: ExitOperational, diagnostic: formatDiagnostic(problem, fix)}
}

func dockerConfigFix() string {
	status := dockerSetupStatus(config.Config{})
	lines := status.Suggestions
	return "configure both Docker settings in the AFTER configuration or set AFTER_DOCKER_BINARY and AFTER_DOCKER_HOST; " + strings.Join(lines, "; ") + "; AFTER does not contact Docker for setup checks"
}

func dockerSetupReadableLines(state *invocation, status dockerSetup) []readableLine {
	if len(status.Problems) == 0 {
		return nil
	}
	lines := []readableLine{textLine("Docker setup", terminal.Attention)}
	for _, problem := range status.Problems {
		lines = append(lines, textLine("  "+problem, terminal.Attention))
	}
	if len(status.Suggestions) > 0 {
		lines = append(lines, textLine("  Suggested settings for the AFTER config file (not selected):", terminal.Muted))
		for _, suggestion := range status.Suggestions {
			line := "    " + suggestion
			if state.stdoutTTY {
				line = terminal.Line(line, max(1, state.columns))
			}
			lines = append(lines, fullLine(line))
		}
	}
	return lines
}
