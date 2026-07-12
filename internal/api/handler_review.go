package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"time"
)

// handleGitStatus returns git status --short for the current directory.
// GET /api/git/status
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statusOut, _ := exec.CommandContext(ctx, "git", "status", "--short").CombinedOutput()
	branchOut, _ := exec.CommandContext(ctx, "git", "branch", "--show-current").CombinedOutput()
	logOut, _ := exec.CommandContext(ctx, "git", "log", "--oneline", "-5").CombinedOutput()

	writeJSON(w, http.StatusOK, map[string]string{
		"branch": string(branchOut),
		"status": string(statusOut),
		"log":    string(logOut),
	})
}

// handleGitDiff returns the diff for a list of files.
// POST /api/git/diff  body: {"files": ["a.txt", "b.txt"]}
func (s *Server) handleGitDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var body struct {
		Files []string `json:"files"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{"diff", "--color=never"}
	if len(body.Files) > 0 {
		args = append(args, "--")
		args = append(args, body.Files...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	outStr := string(out)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"diff": outStr, "error": err.Error()})
		return
	}
	if outStr == "" {
		outStr = "No changes."
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": outStr})
}
