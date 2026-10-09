//go:build ignore

// Trusted language-neutral HTTP-service launcher; runs ONLY in the app sandbox.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const proxyPort = 18080

type launchConfig struct {
	Version                int      `json:"version"`
	BuildArgv              []string `json:"build_argv,omitempty"`
	StartArgv              []string `json:"start_argv"`
	Environment            []string `json:"environment"`
	ReadinessProtocol      string   `json:"readiness_protocol"`
	ReadinessTimeoutSecond int      `json:"readiness_timeout_seconds"`
	Routes                 []route  `json:"routes"`
	ReservedPorts          []int    `json:"reserved_ports"`
}
type route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func main() {
	if err := run(); err != nil {
		_, _ = io.WriteString(os.Stderr, "AFTER service launcher failed\n")
		os.Exit(1)
	}
}

func run() error {
	configBytes, err := os.ReadFile("/input/after/service.json")
	if err != nil || len(configBytes) > 64<<10 {
		return errors.New("invalid service configuration")
	}
	var cfg launchConfig
	decoder := json.NewDecoder(bytes.NewReader(configBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cfg) != nil || decoder.Decode(new(any)) != io.EOF || cfg.Version != 1 || cfg.ReadinessProtocol != "fd3-http-url-v1" || cfg.ReadinessTimeoutSecond < 1 || cfg.ReadinessTimeoutSecond > 120 || len(cfg.StartArgv) == 0 || len(cfg.StartArgv) > 64 || len(cfg.Routes) == 0 || len(cfg.ReservedPorts) == 0 {
		return errors.New("unsupported service configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	for _, arg := range append(append([]string(nil), cfg.BuildArgv...), cfg.StartArgv...) {
		if arg == "" || len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return errors.New("invalid direct argv")
		}
	}
	if len(cfg.BuildArgv) > 0 {
		build := exec.CommandContext(ctx, cfg.BuildArgv[0], cfg.BuildArgv[1:]...)
		build.Env = serviceEnvironment(cfg.Environment)
		build.Stdout, build.Stderr = os.Stdout, os.Stderr
		if err := build.Run(); err != nil {
			return errors.New("service build failed")
		}
	}
	read, write, err := os.Pipe()
	if err != nil {
		return errors.New("readiness pipe unavailable")
	}
	defer read.Close()
	app := exec.CommandContext(ctx, cfg.StartArgv[0], cfg.StartArgv[1:]...)
	app.Env = serviceEnvironment(cfg.Environment)
	app.ExtraFiles = []*os.File{write}
	app.Stdout, app.Stderr = os.Stdout, os.Stderr
	if err := app.Start(); err != nil {
		write.Close()
		return errors.New("service startup failed")
	}
	_ = write.Close()
	appDone := make(chan struct{})
	go func() {
		_ = app.Wait()
		close(appDone)
		// Reap the service parent throughout readiness and all observation. A
		// serving descendant cannot hide the declared process's early exit.
		os.Exit(1)
	}()
	readiness := make(chan []byte, 1)
	readinessErr := make(chan error, 1)
	go func() {
		data, e := io.ReadAll(io.LimitReader(read, 258))
		if e != nil {
			readinessErr <- e
			return
		}
		readiness <- data
	}()
	var address []byte
	select {
	case address = <-readiness:
	case err = <-readinessErr:
		return errors.New("readiness stream failed")
	case <-time.After(time.Duration(cfg.ReadinessTimeoutSecond) * time.Second):
		return errors.New("service readiness timeout")
	case <-ctx.Done():
		return errors.New("service deadline")
	case <-appDone:
		return errors.New("service exited before readiness")
	}
	if len(address) < 2 || len(address) > 257 || address[len(address)-1] != '\n' || bytes.Count(address, []byte{'\n'}) != 1 {
		return errors.New("readiness must be one bounded newline-terminated URL followed by closure")
	}
	target, err := url.Parse(strings.TrimSuffix(string(address), "\n"))
	if err != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.User != nil || target.Path != "" || target.RawQuery != "" || target.Fragment != "" || target.Opaque != "" {
		return errors.New("incompatible fd3 readiness URL")
	}
	port, err := strconv.Atoi(target.Port())
	if err != nil || port < 1 || port > 65535 {
		return errors.New("readiness URL requires a valid loopback port")
	}
	for _, reserved := range cfg.ReservedPorts {
		if port == reserved {
			return errors.New("service readiness uses a reserved observer or proxy port")
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, ResponseHeaderTimeout: 2 * time.Second}
	proxy.ErrorHandler = func(http.ResponseWriter, *http.Request, error) { panic(http.ErrAbortHandler) }
	routes := map[string]bool{}
	for _, allowed := range cfg.Routes {
		key := allowed.Method + " " + allowed.Path
		if allowed.Method == "" || !strings.HasPrefix(allowed.Path, "/") || strings.ContainsAny(allowed.Path, "?#\r\n") || key == "GET /after-ready" || routes[key] {
			return errors.New("invalid or reserved approved route")
		}
		routes[key] = true
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/after-ready" {
			_, _ = io.WriteString(w, "ready")
			return
		}
		if !routes[r.Method+" "+r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(proxyPort), Handler: handler, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, MaxHeaderBytes: 8192}
	if err := server.ListenAndServe(); err != nil {
		return errors.New("app-side loopback proxy failed")
	}
	return nil
}

func serviceEnvironment(values []string) []string {
	environment := []string{"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", "HOME=/work", "TMPDIR=/work", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOMAXPROCS=2", "LANG=C"}
	seen := map[string]bool{}
	for _, value := range values {
		key, _, ok := strings.Cut(value, "=")
		if !ok || key == "" || seen[key] || strings.ContainsAny(value, "\x00\r\n") {
			continue
		}
		seen[key] = true
		environment = append(environment, value)
	}
	return environment
}
