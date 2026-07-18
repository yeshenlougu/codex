package api

import (
	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/store"
)

// agentStoreAdapter bridges store.Store → agent.AgentDataStore.
type agentStoreAdapter struct {
	store *store.Store
}

func (a *agentStoreAdapter) GetAgent(name string) (*agent.AgentRow, error) {
	sr, err := a.store.GetAgent(name)
	if err != nil {
		return nil, err
	}
	if sr == nil {
		return nil, nil
	}
	return &agent.AgentRow{
		Name:         sr.Name,
		DisplayName:  sr.DisplayName,
		Provider:     sr.Provider,
		Model:        sr.Model,
		SystemPrompt: sr.SystemPrompt,
		MaxTurns:     sr.MaxTurns,
	}, nil
}
