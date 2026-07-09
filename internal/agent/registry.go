package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Registry manages agent profiles — the built-in default plus user-created
// profiles stored under ~/.codex/agents/.
type Registry struct {
	mu       sync.RWMutex
	dir      string                // ~/.codex/agents
	profiles map[string]*AgentProfile // keyed by name
}

// NewRegistry creates a new agent profile registry.
func NewRegistry(dir string) *Registry {
	r := &Registry{
		dir:      dir,
		profiles: make(map[string]*AgentProfile),
	}
	// Always register the built-in default
	b := BuiltinDefaultProfile()
	r.profiles[b.Name] = b
	return r
}

// LoadAll scans the agents directory and loads all .yaml files.
// The built-in default is always present and never replaced by a file.
func (r *Registry) LoadAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dir == "" {
		return nil
	}
	if err := os.MkdirAll(r.dir, 0700); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("scan agents dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(r.dir, entry.Name())
		profile, err := LoadProfile(path)
		if err != nil {
			log.Printf("[agent/registry] skip %s: %v", path, err)
			continue
		}
		// Never overwrite built-in
		if profile.Name == "default" {
			log.Printf("[agent/registry] ignoring file profile named 'default' — built-in is immutable")
			continue
		}
		r.profiles[profile.Name] = profile
	}

	return nil
}

// Get returns a profile by name.  Returns nil if not found.
func (r *Registry) Get(name string) *AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.profiles[name]
}

// List returns all profiles sorted by name (built-in first).
func (r *Registry) List() []*AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profiles := make([]*AgentProfile, 0, len(r.profiles))
	for _, p := range r.profiles {
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].IsBuiltin != profiles[j].IsBuiltin {
			return profiles[i].IsBuiltin
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
}

// Create clones the built-in default and saves as a new profile.
func (r *Registry) Create(name string) (*AgentProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[name]; exists {
		return nil, fmt.Errorf("agent profile %q already exists", name)
	}
	if !isValidName(name) {
		return nil, fmt.Errorf("invalid agent name %q: use letters, digits, hyphens", name)
	}

	profile := BuiltinDefaultProfile().Clone(name)
	profile.FilePath = filepath.Join(r.dir, name+".yaml")

	if err := profile.Save(); err != nil {
		return nil, err
	}

	r.profiles[name] = profile
	return profile, nil
}

// CloneFrom creates a new profile by cloning an existing one.
func (r *Registry) CloneFrom(sourceName, newName string) (*AgentProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	src := r.profiles[sourceName]
	if src == nil {
		return nil, fmt.Errorf("source profile %q not found", sourceName)
	}
	if _, exists := r.profiles[newName]; exists {
		return nil, fmt.Errorf("agent profile %q already exists", newName)
	}
	if !isValidName(newName) {
		return nil, fmt.Errorf("invalid agent name %q", newName)
	}

	profile := src.Clone(newName)
	profile.FilePath = filepath.Join(r.dir, newName+".yaml")

	if err := profile.Save(); err != nil {
		return nil, err
	}

	r.profiles[newName] = profile
	return profile, nil
}

// Update overwrites an existing non-built-in profile and persists it.
func (r *Registry) Update(name string, updates *AgentProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing := r.profiles[name]
	if existing == nil {
		return fmt.Errorf("profile %q not found", name)
	}
	if existing.IsBuiltin {
		return fmt.Errorf("cannot update built-in profile %q", name)
	}

	// Preserve identity fields
	updates.Name = name
	updates.FilePath = existing.FilePath
	updates.IsBuiltin = false

	if err := updates.Save(); err != nil {
		return err
	}

	r.profiles[name] = updates
	return nil
}

// Delete removes a non-built-in profile from disk and memory.
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.profiles[name]
	if p == nil {
		return fmt.Errorf("profile %q not found", name)
	}
	if p.IsBuiltin {
		return fmt.Errorf("cannot delete built-in profile %q", name)
	}

	if err := os.Remove(p.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}

	delete(r.profiles, name)
	return nil
}

// Dir returns the agents directory.
func (r *Registry) Dir() string { return r.dir }

func isValidName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
