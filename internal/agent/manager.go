package agent

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/provider"
	"github.com/yeshenlougu/codex/internal/session"
	"github.com/yeshenlougu/codex/internal/tool"
)

// Manager orchestrates multiple Agent instances in a chat-room model:
// each session can have multiple agent participants alongside the user.
// The default is 1 user + 1 system agent.  Additional agents can be added
// or removed, and individual agents respond when @mentioned.
type Manager struct {
	mu       sync.RWMutex
	baseCfg  *config.Config
	store    *session.Store
	registry *Registry

	// SQLite data store for agent configuration loading
	dataStore AgentDataStore

	// Multi-provider failover router
	router *provider.ProviderRouter

	// Chat rooms: key = session_id, value = set of agent profile names
	roomAgents map[string]map[string]bool // sessionID -> {agentName: true}

	// Active agent instances: sessionID:agentName -> *Agent
	active map[string]*Agent // key = sessionID + ":" + agentName

	// Shared MCP tool registry (injected into every new agent)
	mcpRegistry *tool.Registry
}

// AgentRow is a minimal interface for SQLite agent row.
type AgentRow struct {
	Name         string
	DisplayName  string
	Provider     string
	Model        string
	SystemPrompt string
	MaxTurns     int
}

// AgentDataStore is the interface for loading agent config from persistent storage.
type AgentDataStore interface {
	GetAgent(name string) (*AgentRow, error)
}

// NewManager creates an agent manager.
func NewManager(baseCfg *config.Config, store *session.Store, registry *Registry) *Manager {
	return &Manager{
		baseCfg:    baseCfg,
		store:      store,
		registry:   registry,
		roomAgents: make(map[string]map[string]bool),
		active:     make(map[string]*Agent),
	}
}

// Registry returns the agent profile registry.
func (m *Manager) Registry() *Registry { return m.registry }

// SetMCPRegistry injects a shared MCP tool registry for all new agents.
func (m *Manager) SetMCPRegistry(reg *tool.Registry) {
	m.mcpRegistry = reg
}

// SetDataStore injects a SQLite-backed agent data loader.
// When set, agent configuration is loaded from the database first,
// SetDataStore injects a SQLite-backed agent data loader.
func (m *Manager) SetDataStore(ds AgentDataStore) {
	m.dataStore = ds
}

// SetRouter sets the multi-provider failover router.
func (m *Manager) SetRouter(router *provider.ProviderRouter) {
	m.router = router
}

// agentKey builds the internal key for active agent lookup.
func agentKey(sessionID, agentName string) string {
	return sessionID + ":" + agentName
}

// ===================== Session / Chat Room =====================

// CreateSession initializes a new chat room with the default agent.
func (m *Manager) CreateSession(sessionID string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.roomAgents[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	m.roomAgents[sessionID] = map[string]bool{"default": true}

	ag, err := m.createAgentLocked(sessionID, "default")
	if err != nil {
		delete(m.roomAgents, sessionID)
		return nil, err
	}
	ag.SetSessionID(sessionID)

	// Try to restore session from disk
	if _, err := m.store.Load(sessionID); err == nil {
		if err := ag.LoadSession(sessionID); err != nil {
			ag.SetSessionID(sessionID)
		}
	}
	return ag, nil
}

// AddAgent invites an agent into an existing chat room.
func (m *Manager) AddAgent(sessionID, agentName string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.roomAgents[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if room[agentName] {
		return nil, fmt.Errorf("agent %s is already in session %s", agentName, sessionID)
	}

	ag, err := m.createAgentLocked(sessionID, agentName)
	if err != nil {
		return nil, err
	}
	ag.SetSessionID(sessionID)
	room[agentName] = true
	return ag, nil
}

// RemoveAgent removes an agent from a chat room.
func (m *Manager) RemoveAgent(sessionID, agentName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.roomAgents[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if !room[agentName] {
		return fmt.Errorf("agent %s is not in session %s", agentName, sessionID)
	}
	if agentName == "default" && len(room) == 1 {
		return fmt.Errorf("cannot remove the only agent (default) from session")
	}

	key := agentKey(sessionID, agentName)
	if ag, ok := m.active[key]; ok {
		ag.Close()
		delete(m.active, key)
	}
	delete(room, agentName)
	return nil
}

// ListAgents returns the agent names currently in a session.
func (m *Manager) ListAgents(sessionID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.roomAgents[sessionID]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(room))
	for n := range room {
		names = append(names, n)
	}
	return names
}

// GetAgent returns a specific agent from a session, or the default if
// agentName is empty.
func (m *Manager) GetAgent(sessionID, agentName string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if agentName == "" {
		agentName = "default"
	}
	key := agentKey(sessionID, agentName)
	ag, ok := m.active[key]
	if !ok {
		return nil, fmt.Errorf("agent %s not found in session %s", agentName, sessionID)
	}
	return ag, nil
}

// RemoveSession tears down an entire chat room and all its agents.
func (m *Manager) RemoveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.roomAgents[sessionID]
	if !ok {
		return
	}
	for agentName := range room {
		key := agentKey(sessionID, agentName)
		if ag, ok := m.active[key]; ok {
			ag.Close()
		}
		delete(m.active, key)
	}
	delete(m.roomAgents, sessionID)
}

// ActiveSessions returns the count of active chat rooms.
func (m *Manager) ActiveSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.roomAgents)
}

// AllAgents returns every active Agent instance across all sessions.
func (m *Manager) AllAgents() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]*Agent, 0, len(m.active))
	for _, ag := range m.active {
		agents = append(agents, ag)
	}
	return agents
}

// ActiveAgentCount returns total number of active agent instances.
func (m *Manager) ActiveAgentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.active)
}

// ===================== Message Routing =====================

// SendMessage routes a user message to the appropriate agent(s).
//
// Routing rules:
//  1. @agent-name mention → only that agent responds
//  2. No mention → default agent responds
func (m *Manager) SendMessage(sessionID, userMessage string, onChunk func(string)) (response string, respondingAgent string, err error) {
	mentions := extractMentions(userMessage)

	m.mu.RLock()
	room, ok := m.roomAgents[sessionID]
	if !ok {
		m.mu.RUnlock()
		return "", "", fmt.Errorf("session %s not found", sessionID)
	}
	roomCopy := make(map[string]bool, len(room))
	for k, v := range room {
		roomCopy[k] = v
	}
	m.mu.RUnlock()

	if len(mentions) > 0 {
		var results []string
		for _, mention := range mentions {
			if !roomCopy[mention.name] {
				log.Printf("[manager] agent %s not in session %s — auto-inviting", mention.name, sessionID)
				if _, invErr := m.AddAgent(sessionID, mention.name); invErr != nil {
					results = append(results, fmt.Sprintf("[@%s] Error inviting: %v", mention.name, invErr))
					continue
				}
			}
			ag, agErr := m.GetAgent(sessionID, mention.name)
			if agErr != nil {
				results = append(results, fmt.Sprintf("[@%s] Error: %v", mention.name, agErr))
				continue
			}
			r, runErr := ag.Run(mention.message, nil)
			if runErr != nil {
				results = append(results, fmt.Sprintf("[@%s] Error: %v", mention.name, runErr))
			} else {
				results = append(results, fmt.Sprintf("[@%s]\n%s", mention.name, r))
			}
		}
		return strings.Join(results, "\n\n"), agentNames(mentions), nil
	}

	// Default: send to default agent
	ag, agErr := m.GetAgent(sessionID, "default")
	if agErr != nil {
		return "", "", agErr
	}
	resp, runErr := ag.Run(userMessage, onChunk)
	return resp, "default", runErr
}

// ===================== Internal Helpers =====================

func (m *Manager) createAgentLocked(sessionID, agentName string) (*Agent, error) {
	key := agentKey(sessionID, agentName)
	if _, exists := m.active[key]; exists {
		return m.active[key], nil
	}

	// Load agent profile from registry (YAML fallback)
	profile := m.registry.Get(agentName)
	if profile == nil {
		return nil, fmt.Errorf("agent profile %q not found", agentName)
	}

	cfg := profile.ApplyToConfig(m.baseCfg)

	// ── Override with SQLite config when available ──
	if m.dataStore != nil {
		if dbAgent, err := m.dataStore.GetAgent(agentName); err == nil && dbAgent != nil {
			if dbAgent.Provider != "" {
				cfg.Model.Provider = dbAgent.Provider
			}
			if dbAgent.Model != "" {
				cfg.Model.Model = dbAgent.Model
			}
			if dbAgent.SystemPrompt != "" {
				cfg.Agent.SystemPrompt = dbAgent.SystemPrompt
			}
			if dbAgent.MaxTurns > 0 {
				cfg.Agent.MaxTurns = dbAgent.MaxTurns
			}
			log.Printf("[manager] agent %q loaded from SQLite: provider=%s model=%s",
				agentName, cfg.Model.Provider, cfg.Model.Model)
		}
	}

	ag := New(cfg, agentName).WithStore(m.store)

	// Inject shared MCP tools into this agent's registry
	if m.mcpRegistry != nil {
		for _, t := range m.mcpRegistry.AllTools() {
			ag.registry.Register(t)
		}
	}

	// Inject failover router
	if m.router != nil {
		ag.WithRouter(m.router)
	}

	// Inject usage logger
	if m.dataStore != nil {
		// usageLog is set via server or via WithUsageLog
	}

	m.active[key] = ag
	return ag, nil
}

func agentNames(mentions []mention) string {
	names := make([]string, len(mentions))
	for i, m := range mentions {
		names[i] = m.name
	}
	return strings.Join(names, ",")
}

// ===================== @mention Parsing =====================

type mention struct {
	name    string
	message string
}

var mentionRe = regexp.MustCompile(`@([a-zA-Z][a-zA-Z0-9_-]*)\b`)

func extractMentions(msg string) []mention {
	matches := mentionRe.FindAllStringSubmatch(msg, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var mentions []mention
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		cleanMsg := strings.TrimSpace(strings.Replace(msg, "@"+name, "", 1))
		cleanMsg = regexp.MustCompile(`\s+`).ReplaceAllString(cleanMsg, " ")
		mentions = append(mentions, mention{name: name, message: cleanMsg})
	}
	return mentions
}
