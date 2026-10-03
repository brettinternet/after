//go:build ignore

// Frozen HTTP driver, clock and fake provider. Never built with candidate files.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const input = `{"amount_cents":1200,"currency":"USD"}`

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
	Version   int        `json:"version"`
	Seconds   int64      `json:"seconds"`
	Responses []response `json:"responses"`
	Calls     []call     `json:"provider_calls"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("scenario required")
	}
	seconds, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil || (seconds != 30 && seconds != 43200) {
		return fmt.Errorf("unsupported scenario")
	}
	var mu sync.Mutex
	now := int64(1735689600)
	calls := []call{}
	overflow := false
	start := func(port string, handler http.HandlerFunc) (*http.Server, error) {
		l, e := net.Listen("tcp", "127.0.0.1:"+port)
		if e != nil {
			return nil, e
		}
		s := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxHeaderBytes: 4096}
		go s.Serve(l)
		return s, nil
	}
	clock, err := start("18081", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(now)
	})
	if err != nil {
		return fmt.Errorf("clock startup failed")
	}
	defer clock.Close()
	provider, err := start("18082", func(w http.ResponseWriter, r *http.Request) {
		body, e := io.ReadAll(io.LimitReader(r.Body, 4097))
		mu.Lock()
		defer mu.Unlock()
		if e != nil || len(body) > 4096 || len(calls) >= 128 || len(r.URL.Path) > 256 || len(r.Header.Get("Idempotency-Key")) > 128 {
			overflow = true
			http.Error(w, "observer budget", 413)
			return
		}
		calls = append(calls, call{now, r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), string(body)})
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"payment":"synthetic-accepted"}`)
	})
	if err != nil {
		return fmt.Errorf("provider startup failed")
	}
	defer provider.Close()
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// Readiness polling is bounded wall time, never used to simulate fixture time.
	deadline := time.Now().Add(120 * time.Second)
	for {
		r, e := client.Get("http://127.0.0.1:18080/after-ready")
		if e == nil {
			body, _ := io.ReadAll(io.LimitReader(r.Body, 32))
			r.Body.Close()
			if r.StatusCode == 200 && string(body) == "ready" {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("application readiness timeout or incompatible driver")
		}
		time.Sleep(20 * time.Millisecond)
	}
	result := observation{Version: 1, Seconds: seconds}
	for _, after := range []int64{0, seconds} {
		mu.Lock()
		now = 1735689600 + after
		mu.Unlock()
		req, _ := http.NewRequest("POST", "http://127.0.0.1:18080/payments", bytes.NewBufferString(input))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "synthetic-key-a")
		r, e := client.Do(req)
		if e != nil {
			return fmt.Errorf("application request failed")
		}
		body, e := io.ReadAll(io.LimitReader(r.Body, 4097))
		r.Body.Close()
		if e != nil || len(body) > 4096 {
			return fmt.Errorf("response budget or channel failure")
		}
		result.Responses = append(result.Responses, response{r.StatusCode, string(body)})
	}
	mu.Lock()
	defer mu.Unlock()
	if overflow {
		return fmt.Errorf("provider channel incomplete")
	}
	result.Calls = calls
	return json.NewEncoder(os.Stdout).Encode(result)
}
