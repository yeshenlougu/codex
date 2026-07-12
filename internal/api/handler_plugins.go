package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/plugin"
	"github.com/yeshenlougu/codex/internal/skill"
)

// handlePlugins lists installed plugins and allows install/uninstall.
// GET /api/plugins — list installed plugin names
// POST /api/plugins/install — install from JSON body (writes .plugin.json to dir)
// DELETE /api/plugins/{name} — remove plugin file
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/plugins")
	path = strings.TrimPrefix(path, "/")

	pluginsDir := s.cfg.Plugins.Dirs
	dir := "."
	if len(pluginsDir) > 0 {
		dir = pluginsDir[0]
	}
	dir = expandHome(dir)
	os.MkdirAll(dir, 0700)

	switch {
	case r.Method == http.MethodGet && path == "":
		// List installed
		tools, _ := plugin.LoadDir(dir)
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, t.Name())
		}
		writeJSON(w, http.StatusOK, map[string]any{"plugins": names, "dir": dir})

	case r.Method == http.MethodPost && path == "install":
		s.installPlugin(w, r, dir)

	case r.Method == http.MethodDelete && path != "":
		name := path
		pluginPath := filepath.Join(dir, name+".plugin.json")
		if err := os.Remove(pluginPath); err != nil {
			writeError(w, http.StatusNotFound, "plugin not found: "+name)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"uninstalled": name})

	default:
		writeError(w, http.StatusMethodNotAllowed, "not allowed")
	}
}

func (s *Server) installPlugin(w http.ResponseWriter, r *http.Request, dir string) {
	var def plugin.PluginDef
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeError(w, http.StatusBadRequest, "invalid plugin JSON: "+err.Error())
		return
	}
	if def.Name == "" || def.Command == "" {
		writeError(w, http.StatusBadRequest, "name and command required")
		return
	}

	pluginPath := filepath.Join(dir, def.Name+".plugin.json")
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal: "+err.Error())
		return
	}
	if err := os.WriteFile(pluginPath, data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, def)
}

// handleSkills lists loaded skills.
// GET /api/skills
func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	skillList := []map[string]string{}

	if len(skillList) == 0 {
		// Fallback: load from configured dirs
		r := skill.NewRegistry()
		for _, d := range s.cfg.Skills.Dirs {
			r.AddDir(expandHome(d))
		}
		r.LoadAll()
		for name, sk := range r.All() {
			skillList = append(skillList, map[string]string{
				"name":        name,
				"description": sk.Description,
				"category":    sk.Category,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"skills": skillList,
	})
}

// handleTerminal executes an arbitrary shell command and returns output.
// POST /api/terminal  body: {"command": "ls -la", "workdir": "."}
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var body struct {
		Command string `json:"command"`
		Workdir string `json:"workdir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Command == "" {
		writeError(w, http.StatusBadRequest, "command required")
		return
	}

	// Execute with 30s timeout
	out, err := runShellCmd(body.Command, body.Workdir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"output": out, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}

func runShellCmd(command, workdir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}
	output, err := cmd.CombinedOutput()
	outStr := string(output)
	if len(outStr) > 16000 {
		outStr = outStr[:16000] + "\n...(truncated)"
	}
	return outStr, err
}
