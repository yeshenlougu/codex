// Package api — Proxy Server
// Provides a lightweight OpenAI-compatible HTTP proxy backed by SQLite provider config.
package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yeshenlougu/codex/internal/provider"
	"github.com/yeshenlougu/codex/internal/store"
)

// ProxyServer is a headless OpenAI-compatible API proxy.
// Loads provider configuration from SQLite and exposes standard
// /v1/chat/completions and /v1/models endpoints, routing through
// the Provider Pool with CircuitBreaker + failover.
type ProxyServer struct {
	store   *store.Store
	pool    *provider.Pool
	addr    string
	httpSrv *http.Server
}

// NewProxyServer creates a proxy server backed by the SQLite store.
func NewProxyServer(dataStore *store.Store, addr string) *ProxyServer {
	return &ProxyServer{
		store: dataStore,
		addr:  addr,
	}
}

// Start initializes the proxy from SQLite and begins listening.
func (p *ProxyServer) Start() error {
	if err := p.reloadFromStore(); err != nil {
		return fmt.Errorf("proxy init: %w", err)
	}

	mux := http.NewServeMux()

	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, r *http.Request) {
		available := 0
		if p.pool != nil {
			available = p.pool.Available()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"mode":      "proxy",
			"backends":  p.pool.Len(),
			"available": available,
		})
	}))

	mux.HandleFunc("/v1/models", cors(p.handleModels))
	mux.HandleFunc("/v1/chat/completions", cors(p.handleChatCompletions))
	mux.HandleFunc("/v1/config/reload", cors(p.handleConfigReload))

	p.httpSrv = &http.Server{
		Addr:         p.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("[proxy] listening on %s", p.addr)
	return p.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the proxy server.
func (p *ProxyServer) Shutdown(ctx context.Context) error {
	return p.httpSrv.Shutdown(ctx)
}

// reloadFromStore reads providers from SQLite and builds the request pool.
func (p *ProxyServer) reloadFromStore() error {
	providers, err := p.store.ListProviders()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	if len(providers) == 0 {
		log.Printf("[proxy] no providers in SQLite — proxy will return 503 until configured")
		return nil
	}

	// Find current provider
	var current *store.ProviderRow
	for i := range providers {
		if providers[i].IsCurrent {
			current = &providers[i]
			break
		}
	}
	if current == nil {
		current = &providers[0]
	}

	// Build pool from current provider's backends
	backends, err := p.store.ListBackends(current.ID)
	if err != nil {
		return fmt.Errorf("list backends for %s: %w", current.ID, err)
	}

	pool := provider.NewPool("round_robin")
	for _, be := range backends {
		pool.Add(be.APIKey, be.Label, be.BaseURL, be.Weight, nil)
	}
	pool.StartHealthCheck()
	p.pool = pool

	log.Printf("[proxy] loaded provider %q with %d backends", current.Name, len(backends))
	return nil
}

// handleModels returns available models from the current provider's backends.
func (p *ProxyServer) handleModels(w http.ResponseWriter, r *http.Request) {
	if p.pool == nil || p.pool.Len() == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": []any{}})
		return
	}

	modelSet := make(map[string]bool)
	for _, e := range p.pool.Entries() {
		for _, m := range e.Models {
			modelSet[m.Name] = true
		}
	}

	var data []map[string]any
	for name := range modelSet {
		data = append(data, map[string]any{
			"id":       name,
			"object":   "model",
			"owned_by": "codex-proxy",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// handleChatCompletions proxies a chat completion request to the backend pool.
func (p *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if p.pool == nil || p.pool.Len() == 0 {
		writeError(w, http.StatusServiceUnavailable,
			"no configured provider — configure via Codex Go web UI or POST /v1/config/reload")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Select a backend from the pool
	entry, ok := p.pool.Select()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no healthy backends available")
		return
	}

	// Proxy the request
	targetURL := entry.BaseURL + "/chat/completions"
	proxyReq, err := http.NewRequestWithContext(r.Context(), "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "proxy request: "+err.Error())
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")

	// Use original Authorization if provided, otherwise use pool entry key
	if auth := r.Header.Get("Authorization"); auth != "" {
		proxyReq.Header.Set("Authorization", auth)
	} else {
		proxyReq.Header.Set("Authorization", "Bearer "+entry.Key)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(proxyReq)
	if err != nil {
		entry.MarkFailure(true)
		writeError(w, http.StatusBadGateway, "backend error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Mark success for 2xx, failure for 5xx
	if resp.StatusCode >= 500 {
		entry.MarkFailure(true)
	} else {
		entry.MarkSuccess()
	}

	// Copy response headers
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleConfigReload re-reads provider configuration from SQLite.
func (p *ProxyServer) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	// Stop old pool
	if p.pool != nil {
		p.pool.Stop()
	}

	if err := p.reloadFromStore(); err != nil {
		writeError(w, http.StatusInternalServerError, "reload failed: "+err.Error())
		return
	}

	backendCount := 0
	if p.pool != nil {
		backendCount = p.pool.Len()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "reloaded",
		"backends": backendCount,
	})
}

// ── CLI entry point ──

// RunProxy starts the proxy server and blocks until SIGINT/SIGTERM.
func RunProxy(dataStore *store.Store, addr string) {
	p := NewProxyServer(dataStore, addr)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\n[proxy] shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.Shutdown(ctx)
		os.Exit(0)
	}()

	fmt.Printf("Codex Proxy · listening on %s\n", addr)
	fmt.Printf("  GET  /health\n")
	fmt.Printf("  GET  /v1/models\n")
	fmt.Printf("  POST /v1/chat/completions\n")
	fmt.Printf("  POST /v1/config/reload\n")

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[proxy] server error: %v\n", err)
		os.Exit(1)
	}
}
