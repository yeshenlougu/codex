package api

import (
	"net/http"
	"strconv"

	"github.com/yeshenlougu/codex/internal/store"
)

// handleUsage routes GET /api/usage and GET /api/usage/summary.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/usage/summary" || path == "/api/usage/summary/":
		s.handleUsageSummary(w, r)
	case path == "/api/usage" || path == "/api/usage/":
		s.handleUsageLogs(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"summary": []interface{}{}})
		return
	}

	providerID := r.URL.Query().Get("provider")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	summary, err := s.store.UsageDaily(providerID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "usage query: "+err.Error())
		return
	}
	if summary == nil {
		summary = []store.UsageSummary{}
	}

	// Compute totals
	var totalInput, totalOutput, totalRequests int
	var totalCost float64
	for _, u := range summary {
		totalInput += u.InputTokens
		totalOutput += u.OutputTokens
		totalRequests += u.RequestCount
		totalCost += u.CostEst
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":        summary,
		"total_input":    totalInput,
		"total_output":   totalOutput,
		"total_requests": totalRequests,
		"total_cost":     totalCost,
	})
}

func (s *Server) handleUsageLogs(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"logs": []interface{}{}})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	logs, err := s.store.UsageLogs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "usage logs: "+err.Error())
		return
	}
	if logs == nil {
		logs = []store.UsageLogInput{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}
