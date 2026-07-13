package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// InstalledSkill tracks a skill installed from a GitHub repo.
type InstalledSkill struct {
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

// SkillRepo defines a GitHub repository to scan for skills.
type SkillRepo struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Enabled bool   `json:"enabled"`
}

// SkillStoreData is the on-disk structure.
type SkillStoreData struct {
	Skills []InstalledSkill `json:"skills"`
	Repos  []SkillRepo      `json:"repos"`
}

// SkillStore persists skill data to a JSON file.
type SkillStore struct {
	mu   sync.RWMutex
	path string
	data SkillStoreData
}

// NewSkillStore creates or loads the skill store.
func NewSkillStore(path string) (*SkillStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("skill store: mkdir: %w", err)
	}
	s := &SkillStore{
		path: path,
		data: SkillStoreData{
			Skills: []InstalledSkill{},
			Repos:  defaultSkillRepos(),
		},
	}
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("skill store: read: %w", err)
		}
		if len(b) > 0 {
			if err := json.Unmarshal(b, &s.data); err != nil {
				s.data = SkillStoreData{Skills: []InstalledSkill{}, Repos: defaultSkillRepos()}
			}
		}
	}
	return s, nil
}

func defaultSkillRepos() []SkillRepo {
	return []SkillRepo{
		{Owner: "anthropics", Name: "skills", Branch: "main", Enabled: true},
		{Owner: "ComposioHQ", Name: "awesome-claude-skills", Branch: "master", Enabled: true},
		{Owner: "cexll", Name: "myclaude", Branch: "master", Enabled: true},
		{Owner: "JimLiu", Name: "baoyu-skills", Branch: "main", Enabled: true},
	}
}

// Skills returns all installed skills.
func (s *SkillStore) Skills() []InstalledSkill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InstalledSkill, len(s.data.Skills))
	copy(out, s.data.Skills)
	return out
}

// GetSkill returns a skill by ID.
func (s *SkillStore) GetSkill(id string) (*InstalledSkill, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, sk := range s.data.Skills {
		if sk.ID == id {
			cp := sk
			return &cp, i
		}
	}
	return nil, -1
}

// AddSkill inserts a new skill.
func (s *SkillStore) AddSkill(sk *InstalledSkill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if sk.InstalledAt == 0 {
		sk.InstalledAt = now
	}
	sk.UpdatedAt = now
	s.data.Skills = append(s.data.Skills, *sk)
	return s.save()
}

// RemoveSkill deletes a skill by ID.
func (s *SkillStore) RemoveSkill(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sk := range s.data.Skills {
		if sk.ID == id {
			s.data.Skills = append(s.data.Skills[:i], s.data.Skills[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("skill %s not found", id)
}

// UpdateSkill modifies an existing skill by ID.
func (s *SkillStore) UpdateSkill(id string, fn func(sk *InstalledSkill)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sk := range s.data.Skills {
		if sk.ID == id {
			fn(&s.data.Skills[i])
			s.data.Skills[i].UpdatedAt = time.Now().Unix()
			return s.save()
		}
	}
	return fmt.Errorf("skill %s not found", id)
}

// SkillExistsByKey checks if a skill identified by "owner:repo:dir" is already installed.
func (s *SkillStore) SkillExistsByKey(owner, repo, directory string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sk := range s.data.Skills {
		if sk.RepoOwner == owner && sk.RepoName == repo && sk.Directory == directory {
			return true
		}
	}
	return false
}

// Repos returns the configured skill repositories.
func (s *SkillStore) Repos() []SkillRepo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SkillRepo, len(s.data.Repos))
	copy(out, s.data.Repos)
	return out
}

// AddRepo adds a new repo.
func (s *SkillStore) AddRepo(repo SkillRepo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Repos = append(s.data.Repos, repo)
	return s.save()
}

// RemoveRepo deletes a repo by owner/name.
func (s *SkillStore) RemoveRepo(owner, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data.Repos {
		if r.Owner == owner && r.Name == name {
			s.data.Repos = append(s.data.Repos[:i], s.data.Repos[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("repo %s/%s not found", owner, name)
}

func (s *SkillStore) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("skill store: marshal: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0644); err != nil {
		return fmt.Errorf("skill store: write: %w", err)
	}
	return nil
}
