package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Store is the SQLite-backed data layer for all Codex Go entities.
type Store struct {
	db   *sql.DB
	crypto *KeyEncryption
}

// NewStore wraps an existing *sql.DB connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// NewStoreWithCrypto wraps an existing *sql.DB with API key encryption.
func NewStoreWithCrypto(db *sql.DB, crypto *KeyEncryption) *Store {
	return &Store{db: db, crypto: crypto}
}

// encryptKey encrypts an API key if crypto is configured.
func (s *Store) encryptKey(key string) string {
	if s.crypto == nil || key == "" {
		return key
	}
	enc, err := s.crypto.Encrypt(key)
	if err != nil {
		return key // fallback: store plaintext if encryption fails
	}
	return enc
}

// decryptKey decrypts an API key if crypto is configured.
func (s *Store) decryptKey(key string) string {
	if s.crypto == nil || key == "" {
		return key
	}
	dec, err := s.crypto.Decrypt(key)
	if err != nil {
		return key // fallback: return as-is
	}
	return dec
}

// ── Provider ──────────────────────────────────────────────────────────────

// ProviderRow mirrors the providers table.
type ProviderRow struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Icon            string `json:"icon"`
	IconColor       string `json:"icon_color"`
	Category        string `json:"category"`
	Notes           string `json:"notes"`
	InFailoverQueue bool   `json:"in_failover_queue"`
	IsCurrent       bool   `json:"is_current"`
	APIFormat       string `json:"api_format"`
	CostMultiplier  string `json:"cost_multiplier"`
	LimitDailyUSD   string `json:"limit_daily_usd"`
	LimitMonthlyUSD string `json:"limit_monthly_usd"`
	IsFullURL       bool   `json:"is_full_url"`
	EndpointAutoSelect bool   `json:"endpoint_auto_select"`
	PromptCacheKey  string `json:"prompt_cache_key"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	CustomUserAgent string `json:"custom_user_agent"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`

	// Aggregated (not stored in providers table)
	BackendCount  int `json:"backend_count"`
	HealthyCount  int `json:"healthy_count"`
}

// ListProviders returns all providers with aggregated backend stats.
func (s *Store) ListProviders() ([]ProviderRow, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.icon, p.icon_color, p.category, p.notes,
		       p.in_failover_queue, p.is_current,
		       p.api_format, p.cost_multiplier, p.limit_daily_usd, p.limit_monthly_usd,
		       p.is_full_url, p.endpoint_auto_select, p.prompt_cache_key,
		       p.max_output_tokens, p.custom_user_agent,
		       p.created_at, p.updated_at,
		       COUNT(b.id) AS backend_count,
		       SUM(CASE WHEN b.health_status = 'healthy' THEN 1 ELSE 0 END) AS healthy_count
		FROM providers p
		LEFT JOIN backends b ON p.id = b.provider_id
		GROUP BY p.id
		ORDER BY p.name
	`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var out []ProviderRow
	for rows.Next() {
		var r ProviderRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Icon, &r.IconColor, &r.Category, &r.Notes,
			&r.InFailoverQueue, &r.IsCurrent,
			&r.APIFormat, &r.CostMultiplier, &r.LimitDailyUSD, &r.LimitMonthlyUSD,
			&r.IsFullURL, &r.EndpointAutoSelect, &r.PromptCacheKey,
			&r.MaxOutputTokens, &r.CustomUserAgent,
			&r.CreatedAt, &r.UpdatedAt,
			&r.BackendCount, &r.HealthyCount); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetProvider returns a single provider by ID.
func (s *Store) GetProvider(id string) (*ProviderRow, error) {
	r := &ProviderRow{}
	err := s.db.QueryRow(`
		SELECT p.id, p.name, p.icon, p.icon_color, p.category, p.notes,
		       p.in_failover_queue, p.is_current,
		       p.api_format, p.cost_multiplier, p.limit_daily_usd, p.limit_monthly_usd,
		       p.is_full_url, p.endpoint_auto_select, p.prompt_cache_key,
		       p.max_output_tokens, p.custom_user_agent,
		       p.created_at, p.updated_at,
		       COUNT(b.id),
		       SUM(CASE WHEN b.health_status = 'healthy' THEN 1 ELSE 0 END)
		FROM providers p
		LEFT JOIN backends b ON p.id = b.provider_id
		WHERE p.id = ?
		GROUP BY p.id
	`, id).Scan(&r.ID, &r.Name, &r.Icon, &r.IconColor, &r.Category, &r.Notes,
		&r.InFailoverQueue, &r.IsCurrent,
		&r.APIFormat, &r.CostMultiplier, &r.LimitDailyUSD, &r.LimitMonthlyUSD,
		&r.IsFullURL, &r.EndpointAutoSelect, &r.PromptCacheKey,
		&r.MaxOutputTokens, &r.CustomUserAgent,
		&r.CreatedAt, &r.UpdatedAt,
		&r.BackendCount, &r.HealthyCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return r, nil
}

// CreateProvider inserts a new provider with optional backends in a transaction.
func (s *Store) CreateProvider(name, icon, iconColor, category, apiFormat string, backends []BackendInput) (*ProviderRow, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	id := uuid.New().String()

	_, err = tx.Exec(`
		INSERT INTO providers (id, name, icon, icon_color, category, api_format, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, icon, iconColor, category, apiFormat, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert provider: %w", err)
	}

	for _, be := range backends {
		res, err := tx.Exec(`
			INSERT INTO backends (provider_id, label, api_key, base_url, weight, headers, created_at)
			VALUES (?, ?, ?, ?, ?, '{}', ?)
		`, id, be.Label, s.encryptKey(be.APIKey), be.BaseURL, be.Weight, now)
		if err != nil {
			return nil, fmt.Errorf("insert backend: %w", err)
		}
		beID, _ := res.LastInsertId()
		for _, m := range be.Models {
			tx.Exec(`INSERT OR IGNORE INTO backend_models (backend_id, name, type, context_length) VALUES (?,?,?,?)`,
				beID, m.Name, m.Type, m.ContextLength)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetProvider(id)
}

// BackendInput is used when creating providers with backends.
type BackendInput struct {
	Label   string       `json:"label"`
	APIKey  string       `json:"api_key"`
	BaseURL string       `json:"base_url"`
	Weight  int          `json:"weight"`
	Models  []ModelInput `json:"models"`
}

// ModelInput is used when creating backends with models.
type ModelInput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	ContextLength int    `json:"context_length"`
}

// UpdateProvider updates provider fields.
func (s *Store) UpdateProvider(id, name, icon, iconColor, category, notes, apiFormat string, inFailover bool) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE providers SET name=?, icon=?, icon_color=?, category=?, notes=?, api_format=?, in_failover_queue=?, updated_at=?
		WHERE id=?
	`, name, icon, iconColor, category, notes, apiFormat, boolToInt(inFailover), now, id)
	return err
}

// DeleteProvider deletes a provider and cascades to backends/usage_logs.
func (s *Store) DeleteProvider(id string) error {
	_, err := s.db.Exec(`DELETE FROM providers WHERE id = ?`, id)
	return err
}

// SwitchProvider atomically sets the current provider.
func (s *Store) SwitchProvider(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE providers SET is_current = 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE providers SET is_current = 1 WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Presets ───────────────────────────────────────────────────────────────

// PresetRow mirrors the provider_presets table.
type PresetRow struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Icon         string `json:"icon"`
	IconColor    string `json:"icon_color"`
	WebsiteURL   string `json:"website_url"`
	APIKeyURL    string `json:"api_key_url"`
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	SortOrder    int    `json:"sort_order"`
}

// ListPresets returns all provider presets sorted by sort_order.
func (s *Store) ListPresets() ([]PresetRow, error) {
	rows, err := s.db.Query(`SELECT id, name, category, icon, icon_color, website_url, api_key_url, COALESCE(base_url,''), COALESCE(default_model,''), sort_order FROM provider_presets ORDER BY sort_order, category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PresetRow
	for rows.Next() {
		var r PresetRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Category, &r.Icon, &r.IconColor, &r.WebsiteURL, &r.APIKeyURL, &r.BaseURL, &r.DefaultModel, &r.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Backend ───────────────────────────────────────────────────────────────

// BackendRow mirrors the backends table.
type BackendRow struct {
	ID           int    `json:"id"`
	ProviderID   string `json:"provider_id"`
	Label        string `json:"label"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	Weight       int    `json:"weight"`
	Headers      string `json:"headers"`
	HealthStatus string `json:"health_status"`
	LastProbeAt  int64  `json:"last_probe_at"`
	FailCount    int    `json:"fail_count"`
	CreatedAt    int64  `json:"created_at"`
	Models       string `json:"models"` // comma-separated model names
}

// ListBackends returns all backends for a provider with aggregated model names.
func (s *Store) ListBackends(providerID string) ([]BackendRow, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.provider_id, b.label, b.api_key, b.base_url, b.weight,
		       b.headers, b.health_status, b.last_probe_at, b.fail_count, b.created_at,
		       COALESCE(GROUP_CONCAT(bm.name, ', '), '') AS models
		FROM backends b
		LEFT JOIN backend_models bm ON b.id = bm.backend_id
		WHERE b.provider_id = ?
		GROUP BY b.id
		ORDER BY b.weight DESC, b.label
	`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BackendRow
	for rows.Next() {
		var r BackendRow
		if err := rows.Scan(&r.ID, &r.ProviderID, &r.Label, &r.APIKey, &r.BaseURL, &r.Weight,
			&r.Headers, &r.HealthStatus, &r.LastProbeAt, &r.FailCount, &r.CreatedAt,
			&r.Models); err != nil {
			return nil, err
		}
		r.APIKey = s.decryptKey(r.APIKey)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateBackend adds a backend with its models.
func (s *Store) CreateBackend(providerID, label, apiKey, baseURL string, weight int, models []ModelInput) (*BackendRow, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(`
		INSERT INTO backends (provider_id, label, api_key, base_url, weight, headers, created_at)
		VALUES (?, ?, ?, ?, ?, '{}', ?)
	`, providerID, label, s.encryptKey(apiKey), baseURL, weight, now)
	if err != nil {
		return nil, err
	}
	beID, _ := res.LastInsertId()
	for _, m := range models {
		s.db.Exec(`INSERT OR IGNORE INTO backend_models (backend_id, name, type, context_length) VALUES (?,?,?,?)`,
			beID, m.Name, m.Type, m.ContextLength)
	}

	// Return the created row
	r := &BackendRow{}
	s.db.QueryRow(`SELECT id, provider_id, label, api_key, base_url, weight, headers, health_status, last_probe_at, fail_count, created_at FROM backends WHERE id = ?`, beID).
		Scan(&r.ID, &r.ProviderID, &r.Label, &r.APIKey, &r.BaseURL, &r.Weight, &r.Headers, &r.HealthStatus, &r.LastProbeAt, &r.FailCount, &r.CreatedAt)
	return r, nil
}

// UpdateBackendHealth updates health-related fields after a probe.
func (s *Store) UpdateBackendHealth(id int, status string, failCount int) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE backends SET health_status=?, fail_count=?, last_probe_at=? WHERE id=?`,
		status, failCount, now, id)
	return err
}

// DeleteBackend removes a backend by ID.
func (s *Store) DeleteBackend(id int) error {
	_, err := s.db.Exec(`DELETE FROM backends WHERE id = ?`, id)
	return err
}

// ── Agent ─────────────────────────────────────────────────────────────────

// AgentRow mirrors the agents table.
type AgentRow struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	SystemPrompt    string `json:"system_prompt"`
	MaxTurns        int    `json:"max_turns"`
	ReasoningEffort string `json:"reasoning_effort"`
	ToolsMode       string `json:"tools_mode"`
	ToolsList       string `json:"tools_list"`
	MCPMode         string `json:"mcp_mode"`
	MCPList         string `json:"mcp_list"`
	SkillsMode      string `json:"skills_mode"`
	SkillsList      string `json:"skills_list"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	SessionCount    int    `json:"session_count"`
}

// ListAgents returns all agents with session count.
func (s *Store) ListAgents() ([]AgentRow, error) {
	rows, err := s.db.Query(`
		SELECT a.name, a.display_name, a.provider, a.model, a.system_prompt,
		       a.max_turns, a.reasoning_effort, a.tools_mode, a.tools_list,
		       a.mcp_mode, a.mcp_list, a.skills_mode, a.skills_list,
		       a.created_at, a.updated_at,
		       COUNT(s.id) AS session_count
		FROM agents a
		LEFT JOIN sessions s ON a.name = s.agent_name
		GROUP BY a.name
		ORDER BY a.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentRow
	for rows.Next() {
		var r AgentRow
		if err := rows.Scan(&r.Name, &r.DisplayName, &r.Provider, &r.Model, &r.SystemPrompt,
			&r.MaxTurns, &r.ReasoningEffort, &r.ToolsMode, &r.ToolsList,
			&r.MCPMode, &r.MCPList, &r.SkillsMode, &r.SkillsList,
			&r.CreatedAt, &r.UpdatedAt, &r.SessionCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetAgent returns a single agent.
func (s *Store) GetAgent(name string) (*AgentRow, error) {
	r := &AgentRow{}
	err := s.db.QueryRow(`
		SELECT name, display_name, provider, model, system_prompt,
		       max_turns, reasoning_effort, tools_mode, tools_list,
		       mcp_mode, mcp_list, skills_mode, skills_list,
		       created_at, updated_at
		FROM agents WHERE name = ?
	`, name).Scan(&r.Name, &r.DisplayName, &r.Provider, &r.Model, &r.SystemPrompt,
		&r.MaxTurns, &r.ReasoningEffort, &r.ToolsMode, &r.ToolsList,
		&r.MCPMode, &r.MCPList, &r.SkillsMode, &r.SkillsList,
		&r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// CreateAgent inserts a new agent row.
func (s *Store) CreateAgent(name, displayName, provider, model, systemPrompt string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO agents (name, display_name, provider, model, system_prompt, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?)
	`, name, displayName, provider, model, systemPrompt, now, now)
	return err
}

// UpdateAgent updates agent fields.
func (s *Store) UpdateAgent(name, displayName, provider, model, systemPrompt string, maxTurns int) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		UPDATE agents SET display_name=?, provider=?, model=?, system_prompt=?, max_turns=?, updated_at=?
		WHERE name=?
	`, displayName, provider, model, systemPrompt, maxTurns, now, name)
	return err
}

// DeleteAgent deletes an agent (CASCADE to sessions/messages/agent_memory).
func (s *Store) DeleteAgent(name string) error {
	_, err := s.db.Exec(`DELETE FROM agents WHERE name = ?`, name)
	return err
}

// CopyAgent creates a copy of an existing agent with a new name.
func (s *Store) CopyAgent(source, target string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO agents (name, display_name, provider, model, system_prompt, max_turns,
		                    reasoning_effort, tools_mode, tools_list,
		                    mcp_mode, mcp_list, skills_mode, skills_list,
		                    created_at, updated_at)
		SELECT ?, display_name || ' (copy)', provider, model, system_prompt, max_turns,
		       reasoning_effort, tools_mode, tools_list,
		       mcp_mode, mcp_list, skills_mode, skills_list,
		       ?, ?
		FROM agents WHERE name = ?
	`, target, now, now, source)
	return err
}

// ── Agent Memory ──────────────────────────────────────────────────────────

// SetMemory upserts a key-value pair for an agent.
func (s *Store) SetMemory(agentName, key, value string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO agent_memory (agent_name, key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(agent_name, key) DO UPDATE SET value = ?, updated_at = ?
	`, agentName, key, value, now, now, value, now)
	return err
}

// GetMemory returns a single memory value.
func (s *Store) GetMemory(agentName, key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM agent_memory WHERE agent_name = ? AND key = ?`, agentName, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// ListMemory returns all key-value pairs for an agent.
func (s *Store) ListMemory(agentName string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM agent_memory WHERE agent_name = ?`, agentName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// ── Session ───────────────────────────────────────────────────────────────

// SessionRow mirrors the sessions table.
type SessionRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	AgentName string `json:"agent_name"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	MsgCount  int    `json:"msg_count"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// CreateSession inserts a new session.
func (s *Store) CreateSession(id, agentName string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO sessions (id, agent_name, created_at, updated_at) VALUES (?,?,?,?)`, id, agentName, now, now)
	return err
}

// GetSession returns a session by ID.
func (s *Store) GetSession(id string) (*SessionRow, error) {
	r := &SessionRow{}
	err := s.db.QueryRow(`SELECT id, title, agent_name, provider, model, msg_count, created_at, updated_at FROM sessions WHERE id = ?`, id).
		Scan(&r.ID, &r.Title, &r.AgentName, &r.Provider, &r.Model, &r.MsgCount, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// ListSessions returns sessions for an agent, newest first.
func (s *Store) ListSessions(agentName string, limit int) ([]SessionRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, title, agent_name, provider, model, msg_count, created_at, updated_at FROM sessions WHERE agent_name = ? ORDER BY updated_at DESC LIMIT ?`, agentName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionRow
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.Title, &r.AgentName, &r.Provider, &r.Model, &r.MsgCount, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteSession deletes a session (CASCADE messages).
func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// ── Messages ──────────────────────────────────────────────────────────────

// MessageRow mirrors the messages table.
type MessageRow struct {
	ID         int    `json:"id"`
	SessionID  string `json:"session_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCalls  string `json:"tool_calls"`
	TokenCount int    `json:"token_count"`
	CreatedAt  int64  `json:"created_at"`
}

// AddMessage inserts a new message and updates the session.
func (s *Store) AddMessage(sessionID, role, content string) error {
	now := time.Now().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO messages (session_id, role, content, created_at) VALUES (?,?,?,?)`,
		sessionID, role, content, now); err != nil {
		return err
	}

	// Update title from first user message
	if role == "user" {
		title := content
		if len(title) > 100 {
			title = title[:100]
		}
		tx.Exec(`UPDATE sessions SET title = CASE WHEN title = '' THEN ? ELSE title END WHERE id = ?`, title, sessionID)
	}

	if _, err := tx.Exec(`UPDATE sessions SET msg_count = msg_count + 1, provider = COALESCE(NULLIF(provider,''),''), model = COALESCE(NULLIF(model,''),''), updated_at = ? WHERE id = ?`, now, sessionID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetMessages returns all messages for a session in order.
func (s *Store) GetMessages(sessionID string) ([]MessageRow, error) {
	rows, err := s.db.Query(`SELECT id, session_id, role, content, tool_calls, token_count, created_at FROM messages WHERE session_id = ? ORDER BY id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MessageRow
	for rows.Next() {
		var r MessageRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &r.ToolCalls, &r.TokenCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Tools ─────────────────────────────────────────────────────────────────

// ToolRow mirrors the tools table.
type ToolRow struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Risk             string `json:"risk"`
	ApprovalRequired bool   `json:"approval_required"`
	Enabled          bool   `json:"enabled"`
	SortOrder        int    `json:"sort_order"`
}

// ListTools returns tools filtered by category (empty = all).
func (s *Store) ListTools(category string) ([]ToolRow, error) {
	var rows *sql.Rows
	var err error
	if category == "" {
		rows, err = s.db.Query(`SELECT name, description, category, risk, approval_required, enabled, sort_order FROM tools ORDER BY sort_order`)
	} else {
		rows, err = s.db.Query(`SELECT name, description, category, risk, approval_required, enabled, sort_order FROM tools WHERE category = ? ORDER BY sort_order`, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ToolRow
	for rows.Next() {
		var r ToolRow
		if err := rows.Scan(&r.Name, &r.Description, &r.Category, &r.Risk, &r.ApprovalRequired, &r.Enabled, &r.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateTool updates tool metadata.
func (s *Store) UpdateTool(name, description, risk string, approvalRequired, enabled bool) (bool, error) {
	res, err := s.db.Exec(`UPDATE tools SET description=?, risk=?, approval_required=?, enabled=? WHERE name=?`,
		description, risk, boolToInt(approvalRequired), boolToInt(enabled), name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ── MCP Servers ───────────────────────────────────────────────────────────

// MCPServerRow mirrors the mcp_servers table.
type MCPServerRow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Args        string `json:"args"`
	Env         string `json:"env"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

// ListMCPServers returns all enabled MCP servers.
func (s *Store) ListMCPServers() ([]MCPServerRow, error) {
	rows, err := s.db.Query(`SELECT name, description, command, args, env, enabled, sort_order FROM mcp_servers WHERE enabled = 1 ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MCPServerRow
	for rows.Next() {
		var r MCPServerRow
		if err := rows.Scan(&r.Name, &r.Description, &r.Command, &r.Args, &r.Env, &r.Enabled, &r.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAllMCPServers returns all MCP servers regardless of enabled state.
func (s *Store) ListAllMCPServers() ([]MCPServerRow, error) {
	rows, err := s.db.Query(`SELECT name, description, command, args, env, enabled, sort_order FROM mcp_servers ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MCPServerRow
	for rows.Next() {
		var r MCPServerRow
		if err := rows.Scan(&r.Name, &r.Description, &r.Command, &r.Args, &r.Env, &r.Enabled, &r.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateMCPServer inserts a new MCP server.
func (s *Store) CreateMCPServer(name, description, command, args, env string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO mcp_servers (name, description, command, args, env, enabled, sort_order) VALUES (?, ?, ?, ?, ?, 1, 0)`,
		name, description, command, args, env)
	return err
}

// UpdateMCPServer updates an MCP server's configuration.
func (s *Store) UpdateMCPServer(name, description, command, args, env string, enabled bool) error {
	_, err := s.db.Exec(`UPDATE mcp_servers SET description=?, command=?, args=?, env=?, enabled=? WHERE name=?`,
		description, command, args, env, boolToInt(enabled), name)
	return err
}

// DeleteMCPServer removes an MCP server.
func (s *Store) DeleteMCPServer(name string) error {
	_, err := s.db.Exec(`DELETE FROM mcp_servers WHERE name = ?`, name)
	return err
}

// ── Scheduled Jobs ──────────────────────────────────────────────────────

// JobRow mirrors the scheduled_jobs table.
type JobRow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CronExpr  string `json:"cron_expr"`
	Prompt    string `json:"prompt"`
	AgentName string `json:"agent_name"`
	Enabled   bool   `json:"enabled"`
	LastRun   int64  `json:"last_run"`
	NextRun   int64  `json:"next_run"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListJobs returns all scheduled jobs.
func (s *Store) ListJobs() ([]JobRow, error) {
	rows, err := s.db.Query(`SELECT id, name, cron_expr, prompt, agent_name, enabled, last_run, next_run, created_at, updated_at FROM scheduled_jobs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobRow
	for rows.Next() {
		var j JobRow
		if err := rows.Scan(&j.ID, &j.Name, &j.CronExpr, &j.Prompt, &j.AgentName, &j.Enabled, &j.LastRun, &j.NextRun, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// CreateJob inserts a new scheduled job.
func (s *Store) CreateJob(name, cronExpr, prompt, agentName string) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(`INSERT INTO scheduled_jobs (name, cron_expr, prompt, agent_name, enabled, created_at, updated_at) VALUES (?,?,?,?,1,?,?)`,
		name, cronExpr, prompt, agentName, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteJob removes a scheduled job.
func (s *Store) DeleteJob(id int) error {
	_, err := s.db.Exec(`DELETE FROM scheduled_jobs WHERE id = ?`, id)
	return err
}

// ── Usage ─────────────────────────────────────────────────────────────────

// UsageLogInput is the data for a single API call.
type UsageLogInput struct {
	ProviderID   string  `json:"provider_id"`
	BackendID    int     `json:"backend_id"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostEst      float64 `json:"cost_est"`
}

// LogUsage writes a usage log and upserts the daily aggregate.
func (s *Store) LogUsage(u UsageLogInput) error {
	now := time.Now().Unix()
	date := time.Now().Format("2006-01-02")

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO usage_logs (provider_id, backend_id, model, input_tokens, output_tokens, cost_est, created_at) VALUES (?,?,?,?,?,?,?)`,
		u.ProviderID, u.BackendID, u.Model, u.InputTokens, u.OutputTokens, u.CostEst, now); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO usage_daily (date, provider_id, model, input_tokens, output_tokens, request_count, cost_est)
		VALUES (?,?,?,?,?,1,?)
		ON CONFLICT(date, provider_id, model) DO UPDATE SET
			input_tokens = input_tokens + ?,
			output_tokens = output_tokens + ?,
			request_count = request_count + 1,
			cost_est = cost_est + ?
	`, date, u.ProviderID, u.Model, u.InputTokens, u.OutputTokens, u.CostEst, u.InputTokens, u.OutputTokens, u.CostEst); err != nil {
		return err
	}

	return tx.Commit()
}

// ── Usage Queries ──────────────────────────────────────────────────────────

// UsageSummary holds aggregated usage stats.
type UsageSummary struct {
	ProviderID   string  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	RequestCount int     `json:"request_count"`
	CostEst      float64 `json:"cost_est"`
}

// UsageDaily returns daily aggregated usage for a date range.
func (s *Store) UsageDaily(providerID, from, to string) ([]UsageSummary, error) {
	query := `SELECT d.provider_id, COALESCE(p.name,''), d.model, d.input_tokens, d.output_tokens, d.request_count, d.cost_est
		FROM usage_daily d
		LEFT JOIN providers p ON d.provider_id = p.id
		WHERE 1=1`
	args := []interface{}{}
	if providerID != "" {
		query += " AND d.provider_id = ?"
		args = append(args, providerID)
	}
	if from != "" {
		query += " AND d.date >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND d.date <= ?"
		args = append(args, to)
	}
	query += " ORDER BY d.date DESC, d.provider_id, d.model"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageSummary
	for rows.Next() {
		var u UsageSummary
		if err := rows.Scan(&u.ProviderID, &u.ProviderName, &u.Model, &u.InputTokens, &u.OutputTokens, &u.RequestCount, &u.CostEst); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UsageLogs returns recent usage log entries.
func (s *Store) UsageLogs(limit int) ([]UsageLogInput, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT provider_id, backend_id, model, input_tokens, output_tokens, cost_est FROM usage_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageLogInput
	for rows.Next() {
		var u UsageLogInput
		if err := rows.Scan(&u.ProviderID, &u.BackendID, &u.Model, &u.InputTokens, &u.OutputTokens, &u.CostEst); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ── Skills Index ──────────────────────────────────────────────────────────

// SkillIndexRow mirrors the skills table (SQLite index of parsed SKILL.md files).
type SkillIndexRow struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	FilePath    string `json:"file_path"`
	Source      string `json:"source"`
	Enabled     int    `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
}

// UpsertSkill inserts or updates a skill in the SQLite skills index.
func (s *Store) UpsertSkill(name, description, tags, filePath, source string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO skills (name, description, tags, file_path, source, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			tags = excluded.tags,
			file_path = excluded.file_path,
			source = excluded.source
	`, name, description, tags, filePath, source, now)
	return err
}

// SearchSkills performs FTS5 full-text search across skill name, description, and tags.
func (s *Store) SearchSkills(query string, limit int) ([]SkillIndexRow, error) {
	if limit <= 0 {
		limit = 50
	}

	// Try FTS5 first; fall back to LIKE if FTS table doesn't exist
	rows, err := s.db.Query(`
		SELECT s.id, s.name, s.description, s.tags, s.file_path, s.source, s.enabled, s.created_at
		FROM skills s
		JOIN skills_fts fts ON s.id = fts.rowid
		WHERE skills_fts MATCH ? AND s.enabled = 1
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		// Fallback: LIKE-based search
		like := "%" + query + "%"
		rows, err = s.db.Query(`
			SELECT id, name, description, tags, file_path, source, enabled, created_at
			FROM skills
			WHERE enabled = 1 AND (name LIKE ? OR description LIKE ? OR tags LIKE ?)
			ORDER BY name
			LIMIT ?
		`, like, like, like, limit)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var out []SkillIndexRow
	for rows.Next() {
		var r SkillIndexRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Tags, &r.FilePath, &r.Source, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListIndexedSkills returns all indexed skills.
func (s *Store) ListIndexedSkills() ([]SkillIndexRow, error) {
	rows, err := s.db.Query(`SELECT id, name, description, tags, file_path, source, enabled, created_at FROM skills WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SkillIndexRow
	for rows.Next() {
		var r SkillIndexRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Tags, &r.FilePath, &r.Source, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IndexSkillsFromDirs scans directories for SKILL.md files and upserts them into SQLite.
// dirs is a list of absolute paths to scan (e.g. ~/.codex/skills, ~/.claude/skills).
func (s *Store) IndexSkillsFromDirs(dirs []string, registrySkillParser func(path string) (name, desc, tags string, err error)) (int, error) {
	count := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // skip inaccessible dirs
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(dir, entry.Name())
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				skillFile = filepath.Join(skillDir, "skill.md")
				if _, err := os.Stat(skillFile); os.IsNotExist(err) {
					continue
				}
			}
			name, desc, tags, err := registrySkillParser(skillFile)
			if err != nil || name == "" {
				continue
			}
			if err := s.UpsertSkill(name, desc, tags, skillFile, dir); err != nil {
				continue
			}
			count++
		}
	}
	return count, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

// ── Model Aliases ─────────────────────────────────────────────────────────

// ModelAliasRow mirrors the model_aliases table.
type ModelAliasRow struct {
	ID         int    `json:"id"`
	ProviderID string `json:"provider_id"`
	Alias      string `json:"alias"`
	RealName   string `json:"real_name"`
	CreatedAt  int64  `json:"created_at"`
}

// ListModelAliases returns all aliases for a provider.
func (s *Store) ListModelAliases(providerID string) ([]ModelAliasRow, error) {
	rows, err := s.db.Query(`SELECT id, provider_id, alias, real_name, created_at FROM model_aliases WHERE provider_id = ? ORDER BY alias`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelAliasRow
	for rows.Next() {
		var r ModelAliasRow
		if err := rows.Scan(&r.ID, &r.ProviderID, &r.Alias, &r.RealName, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAllModelAliases returns all aliases across all providers.
func (s *Store) ListAllModelAliases() ([]ModelAliasRow, error) {
	rows, err := s.db.Query(`SELECT id, provider_id, alias, real_name, created_at FROM model_aliases ORDER BY provider_id, alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelAliasRow
	for rows.Next() {
		var r ModelAliasRow
		if err := rows.Scan(&r.ID, &r.ProviderID, &r.Alias, &r.RealName, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertModelAlias inserts or updates a model alias.
func (s *Store) UpsertModelAlias(providerID, alias, realName string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO model_aliases (provider_id, alias, real_name, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(provider_id, alias) DO UPDATE SET real_name = excluded.real_name
	`, providerID, alias, realName, now)
	return err
}

// DeleteModelAlias removes an alias by ID.
func (s *Store) DeleteModelAlias(id int) error {
	_, err := s.db.Exec(`DELETE FROM model_aliases WHERE id = ?`, id)
	return err
}

// ResolveModel maps an alias to its real name for a given provider.
// Returns the original name if no alias is found.
func (s *Store) ResolveModel(providerID, modelName string) string {
	var realName string
	err := s.db.QueryRow(`SELECT real_name FROM model_aliases WHERE provider_id = ? AND alias = ?`, providerID, modelName).Scan(&realName)
	if err != nil {
		return modelName // no alias — return as-is
	}
	return realName
}

// EncryptLegacyKeys scans all backends and encrypts any plaintext api_key values.
// Called once after store initialization with crypto enabled.
func (s *Store) EncryptLegacyKeys() (int, error) {
	if s.crypto == nil {
		return 0, nil
	}

	rows, err := s.db.Query(`SELECT id, api_key FROM backends`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id  int
		key string
	}
	var toUpdate []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.key); err != nil {
			return 0, err
		}
		// If decrypt fails or returns different value, it's plaintext
		dec, decErr := s.crypto.Decrypt(r.key)
		if decErr != nil || dec == r.key {
			// Plaintext — needs encryption
			toUpdate = append(toUpdate, r)
		}
	}

	count := 0
	for _, r := range toUpdate {
		enc, err := s.crypto.Encrypt(r.key)
		if err != nil {
			continue
		}
		if _, err := s.db.Exec(`UPDATE backends SET api_key = ? WHERE id = ?`, enc, r.id); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ParseJSONList parses a JSON array string into a string slice.
func ParseJSONList(raw string) []string {
	if raw == "" || raw == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
