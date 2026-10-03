// This deliberately small synthetic application is not a production billing API.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type payment struct {
	created int64
	body    []byte
}

func main() {
	if len(os.Args) != 3 {
		panic("expected clock and provider URLs")
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	var mu sync.Mutex
	keys := map[string]payment{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments", func(w http.ResponseWriter, r *http.Request) {
		// Serialize this intentionally sequential in-memory fixture. The same
		// fetched clock value drives BOTH insertion and expiry; no wall-clock TTL.
		mu.Lock()
		defer mu.Unlock()
		key := r.Header.Get("Idempotency-Key")
		body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
		if err != nil || len(body) > 4096 || key == "" || len(key) > 128 || !json.Valid(body) {
			http.Error(w, "invalid synthetic payment", http.StatusBadRequest)
			return
		}
		clock, err := client.Get(os.Args[1])
		if err != nil {
			http.Error(w, "clock unavailable", http.StatusBadGateway)
			return
		}
		var now int64
		err = json.NewDecoder(io.LimitReader(clock.Body, 128)).Decode(&now)
		clock.Body.Close()
		if err != nil || clock.StatusCode != http.StatusOK {
			http.Error(w, "invalid clock", http.StatusBadGateway)
			return
		}
		if prior, ok := keys[key]; deduplicate && ok && now < prior.created+retentionSeconds {
			w.Header().Set("Content-Type", "application/json")
			w.Write(prior.body)
			return
		}
		req, err := http.NewRequest(http.MethodPost, os.Args[2]+"/charges", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "invalid provider", http.StatusBadGateway)
			return
		}
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "provider unavailable", http.StatusBadGateway)
			return
		}
		result, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
		resp.Body.Close()
		if err != nil || len(result) > 4096 || resp.StatusCode != http.StatusOK {
			http.Error(w, "provider failed", http.StatusBadGateway)
			return
		}
		keys[key] = payment{created: now, body: result}
		// Intentionally untrustworthy diagnostic. The observer never reads it.
		fmt.Println("provider_requests=" + printedCount)
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	ready := os.NewFile(3, "ready")
	if ready == nil {
		panic("missing readiness pipe")
	}
	if _, err = fmt.Fprintln(ready, "http://"+listener.Addr().String()); err != nil {
		panic(err)
	}
	ready.Close()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	if err = server.Serve(listener); err != nil {
		panic(err)
	}
}
