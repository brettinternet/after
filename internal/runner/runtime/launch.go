//go:build ignore

// Trusted launcher for the narrow payment ABI; runs ONLY in the app sandbox.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "/usr/local/go/bin/go", "build", "-trimpath", "-o", "/work/app", "./app")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		panic("application build failed")
	}
	read, write, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	defer read.Close()
	app := exec.CommandContext(ctx, "/work/app", "http://127.0.0.1:18081", "http://127.0.0.1:18082")
	app.Env = []string{"GOMAXPROCS=2"}
	app.ExtraFiles = []*os.File{write}
	app.Stdout, app.Stderr = os.Stdout, os.Stderr
	if err = app.Start(); err != nil {
		panic("application startup failed")
	}
	write.Close()
	// A serving descendant must not hide failure of the application process.
	// Reap continuously, including while readiness blocks or requests are active.
	go func() {
		app.Wait()
		fmt.Fprintln(os.Stderr, "application exited before experiment teardown")
		os.Exit(1)
	}()
	address, err := io.ReadAll(io.LimitReader(read, 256))
	if err != nil {
		panic("readiness failed")
	}
	target, err := url.Parse(strings.TrimSpace(string(address)))
	if err != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.Port() == "" || target.User != nil || target.Path != "" {
		panic("incompatible application readiness ABI")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Transport errors are missing observations, not application-generated 502s.
	proxy.ErrorHandler = func(http.ResponseWriter, *http.Request, error) { panic(http.ErrAbortHandler) }
	proxy.Transport = &http.Transport{Proxy: nil, ResponseHeaderTimeout: 2 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /after-ready", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ready") })
	mux.Handle("POST /payments", proxy)
	server := &http.Server{Addr: "127.0.0.1:18080", Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	if err = server.ListenAndServe(); err != nil {
		panic(err)
	}
}
