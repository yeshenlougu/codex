// Package api — Proxy Server
// Provides a lightweight OpenAI-compatible HTTP proxy backed by SQLite provider config.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	mux.HandleFunc("/v1/aliases", cors(p.handleAliases))
	mux.HandleFunc("/v1/aliases/", cors(p.handleAliases))

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

	// Apply model aliases: resolve the model name in the request body
	resolvedBody := p.resolveModelInRequest(body)

	// Sanitize media: strip image content if backend lacks vision capability
	sanitizedBody, _ := provider.SanitizeRequestForBackend(entry, resolvedBody)

	proxyReq, err := http.NewRequestWithContext(r.Context(), "POST", targetURL, bytes.NewReader(sanitizedBody))
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

	// Log usage (async, best-effort)
	go p.logUsage(entry.Label, body, resp.StatusCode)

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

// logUsage records a proxy request to usage_logs.
func (p *ProxyServer) logUsage(label string, requestBody []byte, statusCode int) {
	if p.store == nil {
		return
	}

	// Find current provider
	providers, _ := p.store.ListProviders()
	var providerID string
	for _, prov := range providers {
		if prov.IsCurrent {
			providerID = prov.ID
			break
		}
	}

	// Extract model name from request body
	modelName := "unknown"
	var req map[string]any
	if err := json.Unmarshal(requestBody, &req); err == nil {
		if m, ok := req["model"].(string); ok {
			modelName = m
		}
	}

	// Count approximate tokens from messages
	var inputTokens int
	if msgs, ok := req["messages"].([]any); ok {
		for _, m := range msgs {
			if msg, ok := m.(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					inputTokens += len(content) / 4
				}
			}
		}
	}

	_ = label
	_ = statusCode

	p.store.LogUsage(store.UsageLogInput{
		ProviderID:  providerID,
		Model:       modelName,
		InputTokens: inputTokens,
	})
}

// resolveModelInRequest parses a JSON chat completion request body,
// resolves model aliases, and returns the modified body.
func (p *ProxyServer) resolveModelInRequest(body []byte) []byte {
	if p.store == nil {
		return body
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body // not valid JSON — forward as-is
	}

	modelName, ok := req["model"].(string)
	if !ok || modelName == "" {
		return body
	}

	// Find current provider ID
	providers, err := p.store.ListProviders()
	if err != nil {
		return body
	}
	var providerID string
	for _, prov := range providers {
		if prov.IsCurrent {
			providerID = prov.ID
			break
		}
	}
	if providerID == "" && len(providers) > 0 {
		providerID = providers[0].ID
	}
	if providerID == "" {
		return body
	}

	resolved := p.store.ResolveModel(providerID, modelName)
	if resolved != modelName {
		req["model"] = resolved
		modified, err := json.Marshal(req)
		if err != nil {
			return body
		}
		log.Printf("[proxy] model alias: %s → %s", modelName, resolved)
		return modified
	}
	return body
}

// handleAliases manages model aliases for the current provider.
func (p *ProxyServer) handleAliases(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeError(w, http.StatusInternalServerError, "store not available")
		return
	}

	// Find current provider
	providers, _ := p.store.ListProviders()
	var providerID string
	for _, prov := range providers {
		if prov.IsCurrent {
			providerID = prov.ID
			break
		}
	}
	if providerID == "" && len(providers) > 0 {
		providerID = providers[0].ID
	}

	switch r.Method {
	case http.MethodGet:
		aliases, err := p.store.ListModelAliases(providerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases, "provider_id": providerID})

	case http.MethodPost:
		var input struct {
			Alias    string `json:"alias"`
			RealName string `json:"real_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if input.Alias == "" || input.RealName == "" {
			writeError(w, http.StatusBadRequest, "alias and real_name required")
			return
		}
		if err := p.store.UpsertModelAlias(providerID, input.Alias, input.RealName); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"alias": input.Alias, "real_name": input.RealName})

	case http.MethodDelete:
		// Delete by alias ID: /v1/aliases/123
		path := r.URL.Path
		idStr := ""
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			idStr = path[idx+1:]
		}
		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid alias ID")
			return
		}
		if err := p.store.DeleteModelAlias(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": idStr})

	default:
		writeError(w, http.StatusMethodNotAllowed, "use GET/POST/DELETE")
	}
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
