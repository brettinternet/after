//go:build ignore

// Frozen HTTP request driver, controlled clock and fake upstream observer. It is
// built from shipped AFTER source in a separate offline observer container.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type definition struct {
	Version     int            `json:"version"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Platform    string         `json:"platform"`
	Image       string         `json:"image"`
	BuildArgv   []string       `json:"build_argv"`
	StartArgv   []string       `json:"start_argv"`
	Environment []string       `json:"environment"`
	Readiness   readiness      `json:"readiness"`
	Epoch       int64          `json:"epoch"`
	Upstreams   []upstream     `json:"upstreams"`
	Cases       []scenarioCase `json:"cases"`
	Channels    []string       `json:"channels"`
	Repetitions int            `json:"repetitions"`
	Limits      limits         `json:"limits"`
}
type readiness struct {
	Protocol       string `json:"protocol"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type limits struct {
	Seconds            int `json:"seconds"`
	OutputBytes        int `json:"output_bytes"`
	PreparationSeconds int `json:"preparation_seconds"`
}
type upstream struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Port           int    `json:"port"`
	ResponseStatus int    `json:"response_status,omitempty"`
	ResponseBody   string `json:"response_body,omitempty"`
}
type scenarioCase struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Requests []request `json:"requests"`
}
type request struct {
	AfterSeconds int64             `json:"after_seconds"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body"`
}
type response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
type call struct {
	At          int64  `json:"at"`
	Endpoint    string `json:"endpoint"`
	Destination string `json:"destination"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Key         string `json:"key"`
	Body        string `json:"body"`
}
type observation struct {
	Version   int        `json:"version"`
	CaseID    string     `json:"case_id"`
	Seconds   int64      `json:"seconds,omitempty"`
	Responses []response `json:"responses"`
	Calls     []call     `json:"provider_calls"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "frozen observer failed")
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return errors.New("definition and case required")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil || len(raw) > 64<<10 {
		return errors.New("invalid frozen definition")
	}
	var def definition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&def) != nil || decoder.Decode(new(any)) != io.EOF || def.Version != 1 || def.Epoch < 1 || len(def.Upstreams) > 4 || len(def.Cases) > 8 {
		return errors.New("unsupported frozen definition")
	}
	var current *scenarioCase
	for i := range def.Cases {
		if def.Cases[i].ID == os.Args[2] {
			current = &def.Cases[i]
			break
		}
	}
	if current == nil || len(current.Requests) == 0 || len(current.Requests) > 16 {
		return errors.New("unknown frozen case")
	}
	var mu sync.Mutex
	now := def.Epoch
	calls := make([]call, 0)
	overflow := false
	servers := make([]*http.Server, 0, len(def.Upstreams))
	for _, endpoint := range def.Upstreams {
		endpoint := endpoint
		listener, listenErr := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(endpoint.Port)))
		if listenErr != nil {
			return errors.New("fake upstream startup failed")
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if endpoint.Kind == "clock" {
				mu.Lock()
				clock := now
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, "%d\n", clock)
				return
			}
			body, readErr := io.ReadAll(io.LimitReader(r.Body, 4097))
			mu.Lock()
			defer mu.Unlock()
			if readErr != nil || len(body) > 4096 || len(calls) >= 128 || len(r.URL.Path) > 256 || len(r.Header.Get("Idempotency-Key")) > 128 {
				overflow = true
				http.Error(w, "observer budget", http.StatusRequestEntityTooLarge)
				return
			}
			calls = append(calls, call{At: now, Endpoint: endpoint.Name, Destination: net.JoinHostPort("127.0.0.1", strconv.Itoa(endpoint.Port)), Method: r.Method, Path: r.URL.Path, Key: r.Header.Get("Idempotency-Key"), Body: string(body)})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(endpoint.ResponseStatus)
			_, _ = io.WriteString(w, endpoint.ResponseBody)
		})
		server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxHeaderBytes: 8192}
		servers = append(servers, server)
		go func() { _ = server.Serve(listener) }()
	}
	defer func() {
		for _, server := range servers {
			_ = server.Close()
		}
	}()
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 3 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	deadline := time.Now().Add(120 * time.Second)
	for {
		response, requestErr := client.Get("http://127.0.0.1:18080/after-ready")
		if requestErr == nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 32))
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK && string(body) == "ready" {
				break
			}
		}
		if time.Now().After(deadline) {
			return errors.New("service readiness deadline")
		}
		time.Sleep(20 * time.Millisecond)
	}
	result := observation{Version: 1, CaseID: current.ID, Seconds: current.Requests[len(current.Requests)-1].AfterSeconds, Responses: make([]response, 0, len(current.Requests)), Calls: calls}
	for _, input := range current.Requests {
		mu.Lock()
		now = def.Epoch + input.AfterSeconds
		mu.Unlock()
		req, reqErr := http.NewRequest(input.Method, "http://127.0.0.1:18080"+input.Path, strings.NewReader(input.Body))
		if reqErr != nil {
			return errors.New("invalid frozen HTTP stimulus")
		}
		for key, value := range input.Headers {
			req.Header.Set(key, value)
		}
		resp, reqErr := client.Do(req)
		if reqErr != nil {
			return errors.New("application request failed")
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4097))
		_ = resp.Body.Close()
		if readErr != nil || len(body) > 4096 {
			return errors.New("application response exceeded channel bounds")
		}
		result.Responses = append(result.Responses, response{Status: resp.StatusCode, Body: string(body)})
	}
	mu.Lock()
	defer mu.Unlock()
	if overflow {
		return errors.New("fake upstream recording budget exceeded")
	}
	result.Calls = append([]call{}, calls...)
	return json.NewEncoder(os.Stdout).Encode(result)
}
