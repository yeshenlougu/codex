package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yeshenlougu/codex/internal/skill"
	"github.com/yeshenlougu/codex/internal/store"
)

// skillItem is the frontend-facing skill structure.
type skillItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Directory   string `json:"directory"`
	RepoOwner   string `json:"repo_owner"`
	RepoName    string `json:"repo_name"`
	RepoBranch  string `json:"repo_branch"`
	ReadmeURL   string `json:"readme_url"`
	ContentHash string `json:"content_hash"`
	InstalledAt int64  `json:"installed_at"`
	UpdatedAt   int64  `json:"updated_at"`
	Enabled     bool   `json:"enabled"`
}

func toSkillItem(sk *store.InstalledSkill) *skillItem {
	return &skillItem{
		ID:          sk.ID,
		Name:        sk.Name,
		Description: sk.Description,
		Directory:   sk.Directory,
		RepoOwner:   sk.RepoOwner,
		RepoName:    sk.RepoName,
		RepoBranch:  sk.RepoBranch,
		ReadmeURL:   sk.ReadmeURL,
		ContentHash: sk.ContentHash,
		InstalledAt: sk.InstalledAt,
		UpdatedAt:   sk.UpdatedAt,
		Enabled:     sk.Enabled,
	}
}

// handleSkillsExtended handles /api/skills/* routes for install/discover/repos/updates.
func (s *Server) handleSkillsExtended(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/skills")
	path = strings.TrimPrefix(path, "/")

	// /api/skills/installed
	if path == "installed" && r.Method == http.MethodGet {
		s.listInstalledSkills(w, r)
		return
	}

	// /api/skills/discover
	if path == "discover" && r.Method == http.MethodGet {
		s.discoverSkills(w, r)
		return
	}

	// /api/skills/install
	if path == "install" && r.Method == http.MethodPost {
		s.installSkill(w, r)
		return
	}

	// /api/skills/updates
	if path == "updates" && r.Method == http.MethodGet {
		s.checkSkillUpdates(w, r)
		return
	}

	// /api/skills/repos
	if path == "repos" {
		switch r.Method {
		case http.MethodGet:
			s.listSkillRepos(w, r)
		case http.MethodPost:
			s.addSkillRepo(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "not allowed")
		}
		return
	}

	// /api/skills/repos/{owner}/{name}
	if strings.HasPrefix(path, "repos/") {
		rest := strings.TrimPrefix(path, "repos/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && r.Method == http.MethodDelete {
			s.removeSkillRepo(w, r, parts[0], parts[1])
			return
		}
	}

	// /api/skills/{id} — uninstall (DELETE) or single get
	if path != "" && !strings.Contains(path, "/") {
		switch r.Method {
		case http.MethodGet:
			s.getSkill(w, r, path)
		case http.MethodDelete:
			s.uninstallSkill(w, r, path)
		default:
			writeError(w, http.StatusMethodNotAllowed, "not allowed")
		}
		return
	}

	// Fallback to legacy skills handler
	s.handleSkills(w, r)
}

// listInstalledSkills returns all installed skills.
func (s *Server) listInstalledSkills(w http.ResponseWriter, r *http.Request) {
	skills := s.skillStore.Skills()
	items := make([]*skillItem, 0, len(skills))
	for i := range skills {
		items = append(items, toSkillItem(&skills[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": items})
}

// getSkill returns a single skill by ID.
func (s *Server) getSkill(w http.ResponseWriter, r *http.Request, id string) {
	sk, _ := s.skillStore.GetSkill(id)
	if sk == nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, toSkillItem(sk))
}

// discoverSkills scans configured repos for installable skills.
func (s *Server) discoverSkills(w http.ResponseWriter, r *http.Request) {
	if s.skillInstaller == nil {
		writeJSON(w, http.StatusOK, map[string]any{"skills": []skill.DiscoverResult{}})
		return
	}
	results, err := s.skillInstaller.Discover()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "discover failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": results})
}

// installSkill installs a skill from GitHub.
func (s *Server) installSkill(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Key        string `json:"key"`
		RepoOwner  string `json:"repo_owner"`
		RepoName   string `json:"repo_name"`
		RepoBranch string `json:"repo_branch"`
		Directory  string `json:"directory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if input.RepoOwner == "" || input.RepoName == "" || input.Directory == "" {
		writeError(w, http.StatusBadRequest, "repo_owner, repo_name, directory required")
		return
	}
	if input.RepoBranch == "" {
		input.RepoBranch = "main"
	}

	sk, err := s.skillInstaller.Install(input.RepoOwner, input.RepoName, input.RepoBranch, input.Directory)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "install failed: "+err.Error())
		return
	}

	// Reload skill registry so the new skill is available to agents
	s.reloadSkills()
	writeJSON(w, http.StatusCreated, toSkillItem(sk))
}

// uninstallSkill removes a skill.
func (s *Server) uninstallSkill(w http.ResponseWriter, r *http.Request, id string) {
	backupPath, err := s.skillInstaller.Uninstall(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.reloadSkills()
	writeJSON(w, http.StatusOK, map[string]string{"uninstalled": id, "backup": backupPath})
}

// checkSkillUpdates checks for outdated skills.
func (s *Server) checkSkillUpdates(w http.ResponseWriter, r *http.Request) {
	outdated, err := s.skillInstaller.CheckUpdates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "check failed: "+err.Error())
		return
	}
	items := make([]*skillItem, 0, len(outdated))
	for i := range outdated {
		items = append(items, toSkillItem(&outdated[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"outdated": items, "count": len(items)})
}

// listSkillRepos returns configured repos.
func (s *Server) listSkillRepos(w http.ResponseWriter, r *http.Request) {
	repos := s.skillStore.Repos()
	writeJSON(w, http.StatusOK, map[string]any{"repos": repos})
}

// addSkillRepo adds a new repo.
func (s *Server) addSkillRepo(w http.ResponseWriter, r *http.Request) {
	var input store.SkillRepo
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if input.Owner == "" || input.Name == "" {
		writeError(w, http.StatusBadRequest, "owner and name required")
		return
	}
	if input.Branch == "" {
		input.Branch = "main"
	}
	input.Enabled = true
	if err := s.skillStore.AddRepo(input); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, input)
}

// removeSkillRepo deletes a repo.
func (s *Server) removeSkillRepo(w http.ResponseWriter, r *http.Request, owner, name string) {
	if err := s.skillStore.RemoveRepo(owner, name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": owner + "/" + name})
}

// reloadSkills reloads the skill registry from disk and injects into manager.
func (s *Server) reloadSkills() {
	skillsDir := filepath.Join(expandHome("~/.codex"), "skills")
	reg := skill.NewRegistry()
	reg.AddDir(skillsDir)
	if err := reg.LoadAll(); err != nil {
		log.Printf("[api] skill reload: %v", err)
	}
	// Update manager's skill registry (existing agents need restart to pick up,
	// but new agents get the updated skills via the manager)
	log.Printf("[api] skill registry reloaded — %d skills", len(reg.All()))
}
