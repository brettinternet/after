// The driver and fake provider are frozen across all application revisions.
// This is a synthetic fixture, not a runner for arbitrary untrusted projects.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const epoch int64 = 1735689600 // 2025-01-01T00:00:00Z, synthetic.
const input = `{"amount_cents":1200,"currency":"USD"}`

type step struct {
	AfterSeconds int64  `json:"after_seconds"`
	Key          string `json:"key"`
}
type scenario struct {
	Name  string `json:"name"`
	Steps []step `json:"steps"`
}
type response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}
type call struct {
	At     int64  `json:"at"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Body   string `json:"body"`
}
type observation struct {
	Scenario  scenario   `json:"scenario"`
	Responses []response `json:"responses"`
	Calls     []call     `json:"provider_calls"`
}

func scenarios() []scenario {
	var out []scenario
	for _, c := range []struct {
		name    string
		seconds int64
	}{
		{"same-key-12h", 43200}, {"same-key-30s", 30},
		{"expired-key-25h", 90000}, {"before-5m", 299},
		{"at-5m", 300}, {"before-24h", 86399}, {"at-24h", 86400},
	} {
		out = append(out, scenario{c.name, []step{{0, "synthetic-key-a"}, {c.seconds, "synthetic-key-a"}}})
	}
	out = append(out,
		scenario{"different-key-30s", []step{{0, "synthetic-key-a"}, {30, "synthetic-key-b"}}},
		scenario{"no-sliding-expiry", []step{{0, "synthetic-key-a"}, {240, "synthetic-key-a"}, {360, "synthetic-key-a"}}},
	)
	return out
}

func run(binary string, s scenario) (observation, error) {
	result := observation{Scenario: s, Calls: []call{}}
	// Nothing is shared between cases: new clock, log, servers and app process.
	var mu sync.Mutex
	now := epoch
	var calls []call
	clock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(now)
	}))
	defer clock.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
		if err != nil || len(body) > 4096 {
			http.Error(w, "body limit", 400)
			return
		}
		mu.Lock()
		// Append EVERY request, including repeats with the same key. There is no
		// provider deduplication and no expected-count input to this recorder.
		calls = append(calls, call{now, r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), string(body)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"payment":"synthetic-accepted"}`)
	}))
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	read, write, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer read.Close()
	defer write.Close()
	cmd := exec.CommandContext(ctx, binary, clock.URL, provider.URL)
	cmd.Env = []string{"GOMAXPROCS=2"}
	cmd.ExtraFiles = []*os.File{write}
	// App-printed counters cannot enter the independent observation channel.
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return result, err
	}
	write.Close()
	defer func() { cancel(); cmd.Wait() }()
	address, err := io.ReadAll(io.LimitReader(read, 256))
	if err != nil {
		return result, err
	}
	url := strings.TrimSpace(string(address))
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		return result, fmt.Errorf("app did not become ready: %q", address)
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport}
	for _, action := range s.Steps {
		mu.Lock()
		now = epoch + action.AfterSeconds
		mu.Unlock()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/payments", bytes.NewBufferString(input))
		if err != nil {
			return result, err
		}
		req.Header.Set("Idempotency-Key", action.Key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return result, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
		resp.Body.Close()
		if err != nil || len(body) > 4096 {
			return result, fmt.Errorf("invalid response: %v", err)
		}
		result.Responses = append(result.Responses, response{resp.StatusCode, string(body)})
	}
	mu.Lock()
	result.Calls = append(result.Calls, calls...)
	mu.Unlock()
	return result, nil
}

func main() {
	// Build only the captured synthetic app, inside the already authorized
	// sandbox. No module downloads, host execution or repository build scripts.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	binary := "/work/payment-app"
	build := exec.CommandContext(ctx, "/usr/local/go/bin/go", "build", "-trimpath", "-o", binary, "./app")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var results []observation
	for _, s := range scenarios() {
		result, err := run(binary, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, s.Name, err)
			os.Exit(1)
		}
		results = append(results, result)
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}
