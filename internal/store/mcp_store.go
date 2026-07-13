// Package store provides persistent storage for MCP servers and skills.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MCPServerDef stores a single MCP server configuration.
type MCPServerDef struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Enabled     bool              `json:"enabled"`
	Status      string            `json:"status"` // "connected" | "disconnected" | "error"
	Error       string            `json:"error,omitempty"`
	ToolCount   int               `json:"tool_count"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// MCPStore persists MCP server definitions to a JSON file.
type MCPStore struct {
	mu   sync.RWMutex
	path string
	data map[string]*MCPServerDef
}

// NewMCPStore creates or loads the MCP store from the given path.
func NewMCPStore(path string) (*MCPStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("mcp store: mkdir: %w", err)
	}
	s := &MCPStore{path: path, data: make(map[string]*MCPServerDef)}
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("mcp store: read: %w", err)
		}
		if len(b) > 0 {
			if err := json.Unmarshal(b, &s.data); err != nil {
				// Corrupted file — start fresh
				s.data = make(map[string]*MCPServerDef)
			}
		}
	}
	return s, nil
}

// All returns all MCP server definitions.
func (s *MCPStore) All() map[string]*MCPServerDef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*MCPServerDef, len(s.data))
	for k, v := range s.data {
		cp := *v
		out[k] = &cp
	}
	return out
}

// Get returns a single server by ID.
func (s *MCPStore) Get(id string) (*MCPServerDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[id]
	if !ok {
		return nil, false
	}
	cp := *v
	return &cp, true
}

// Add inserts a new server definition.
func (s *MCPStore) Add(def *MCPServerDef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[def.ID]; exists {
		return fmt.Errorf("mcp server %s already exists", def.ID)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if def.CreatedAt == "" {
		def.CreatedAt = now
	}
	def.UpdatedAt = now
	if def.Status == "" {
		def.Status = "disconnected"
	}
	s.data[def.ID] = def
	return s.save()
}

// Update modifies an existing server definition.
func (s *MCPStore) Update(id string, fn func(def *MCPServerDef) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.data[id]
	if !ok {
		return fmt.Errorf("mcp server %s not found", id)
	}
	if err := fn(def); err != nil {
		return err
	}
	def.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.save()
}

// Remove deletes a server definition.
func (s *MCPStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return fmt.Errorf("mcp server %s not found", id)
	}
	delete(s.data, id)
	return s.save()
}

func (s *MCPStore) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("mcp store: marshal: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0644); err != nil {
		return fmt.Errorf("mcp store: write: %w", err)
	}
	return nil
}
