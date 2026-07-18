package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/hook"
	"github.com/yeshenlougu/codex/internal/mcp"
	"github.com/yeshenlougu/codex/internal/plugin"
	"github.com/yeshenlougu/codex/internal/provider"
	"github.com/yeshenlougu/codex/internal/sandbox"
	"github.com/yeshenlougu/codex/internal/session"
	"github.com/yeshenlougu/codex/internal/skill"
	"github.com/yeshenlougu/codex/internal/tool"
)

// Agent is the core coding agent.
type Agent struct {
	cfg      *config.Config
	registry *tool.Registry
	pool     *provider.Pool      // multi-endpoint pool (replaces external cc-switch)
	router   *provider.ProviderRouter // multi-provider failover (per SPEC §3.5)
	store    *session.Store
	skills   *skill.Registry
	hooks    *hook.Runner
	mcpClients []*mcp.MCPClient

	// Agent identity (per SPEC §4.2)
	agentName string // profile name ("default", "test-agent", etc.)

	// Usage logger (per SPEC Phase 2.4)
	usageLog func(providerID string, backendID int, model string, inputTokens, outputTokens int, costEst float64)

	// Session state
	sessionID string
	messages  []provider.Message
	turnCount int
	running   bool
}

// New creates an Agent with all enabled tools, MCP servers, skills, plugins, and hooks.
// agentName identifies which profile this agent represents (used to load soul.md).
func New(cfg *config.Config, agentName string) *Agent {
	registry := tool.NewRegistry()

	// ── Built-in tools (always registered; can be filtered later via WithTools) ──
	registerBuiltinTools(registry)

	// ---- Plugin tools (.plugin.json files) ----
	for _, dir := range cfg.Plugins.Dirs {
		dir = expandHome(dir)
		pluginTools, err := plugin.LoadDir(dir)
		if err != nil {
			log.Printf("[agent] plugin load %s: %v", dir, err)
			continue
		}
		for _, pt := range pluginTools {
			registry.Register(pt)
			log.Printf("[agent] plugin loaded: %s", pt.Name())
		}
	}

	// ---- MCP servers ----
	var mcpClients []*mcp.MCPClient
	for _, srv := range cfg.MCP.Servers {
		if !srv.Enabled {
			continue
		}
		client, err := mcp.NewMCPClient(srv.Command, srv.Args...)
		if err != nil {
			log.Printf("[agent] MCP server %s (%s): %v", srv.Name, srv.Command, err)
			continue
		}
		mcpClients = append(mcpClients, client)
		// Register each MCP tool as a wrapped tool
		for _, t := range client.Tools {
			wrapped := mcp.NewToolWrapper(client, t)
			registry.Register(wrapped)
			log.Printf("[agent] MCP tool loaded: %s (from %s)", t.Name, srv.Name)
		}
	}

	// ---- Skill registry ----
	skills := skill.NewRegistry()
	for _, dir := range cfg.Skills.Dirs {
		dir = expandHome(dir)
		skills.AddDir(dir)
	}
	if err := skills.LoadAll(); err != nil {
		log.Printf("[agent] skill load: %v", err)
	}

	// ---- Hook runner ----
	hookRunner := hook.NewRunner(
		expandHome(cfg.Hooks.PreTool),
		expandHome(cfg.Hooks.PostTool),
		expandHome(cfg.Hooks.OnSessionStart),
		expandHome(cfg.Hooks.OnSessionEnd),
		expandHome(cfg.Hooks.PostToolMessage),
	)

	// ---- Backend pool ----
	pool := buildPool(cfg)
	pool.StartHealthCheck()

	// Wire image generation tool to pool's image_gen backends
	if entry, _, ok := pool.SelectFor(provider.ModelImageGen); ok {
		if imgT, exists := registry.Get("image_gen"); exists {
			if imgTool, ok := imgT.(*tool.ImageGenTool); ok {
				imgTool.SetBackend(entry.BaseURL, entry.Key, cfg.Model.Model)
				log.Printf("[agent] image_gen backend: %s (%s)", entry.BaseURL, entry.Label)
			}
		}
	}

	// ── System prompt with soul.md + skills ──
	systemPrompt := buildSystemPrompt(cfg.Agent.SystemPrompt, agentName, skills)

	return &Agent{
		cfg:        cfg,
		registry:   registry,
		pool:       pool,
		hooks:      hookRunner,
		skills:     skills,
		mcpClients: mcpClients,
		agentName:  agentName,
		messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// buildSystemPrompt assembles the final system prompt per SPEC §4.4:
//   [Agent Soul]
//   <soul.md content>
//
//   [System]
//   <system_prompt>
//
// followed by skill prompts when available.
func buildSystemPrompt(systemPrompt, agentName string, skills *skill.Registry) string {
	var sb strings.Builder

	// 1. Agent Soul (from ~/.codex/agents/<name>/soul.md)
	soul := loadSoul(agentName)
	if soul != "" {
		sb.WriteString("[Agent Soul]\n")
		sb.WriteString(soul)
		sb.WriteString("\n\n")
	}

	// 2. System prompt
	sb.WriteString("[System]\n")
	sb.WriteString(systemPrompt)

	// 3. Skills
	if skills != nil {
		sb.WriteString(skills.SystemPrompt())
	}

	return sb.String()
}

// loadSoul reads the soul.md file for a named agent.
// Returns empty string if the file doesn't exist or can't be read.
func loadSoul(agentName string) string {
	if agentName == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".codex", "agents", agentName, "soul.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "" // file not found or unreadable — no soul
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}
	return content
}

// expandHome replaces ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
}

// Close shuts down MCP clients.
func (a *Agent) Close() {
	for _, c := range a.mcpClients {
		c.Close()
	}
}

// buildPool creates a Pool from config, preferring backends over legacy keys.
func buildPool(cfg *config.Config) *provider.Pool {
	strategy := cfg.Provider.PoolStrategy
	if strategy == "" {
		strategy = "round_robin"
	}
	pool := provider.NewPool(strategy)

	// Helper: convert config ModelEntry to provider ModelInfo
	toModelInfo := func(entries []config.ModelEntry) []provider.ModelInfo {
		out := make([]provider.ModelInfo, 0, len(entries))
		for _, e := range entries {
			mt, auto := provider.ModelType(e.Type), true
			if e.Type != "" {
				auto = false // user explicitly set type
			} else {
				mt, _ = provider.DetectModelType(e.Name)
			}
			out = append(out, provider.ModelInfo{
				Name: e.Name,
				Type: mt,
				Auto: auto,
			})
		}
		return out
	}

	// New-style: multi-endpoint backends
	if len(cfg.Provider.Backends) > 0 {
		for _, be := range cfg.Provider.Backends {
			baseURL := be.BaseURL
			if baseURL == "" {
				baseURL = cfg.Provider.BaseURL
			}
			pool.Add(be.Key, be.Label, baseURL, be.Weight, toModelInfo(be.Models))
		}
		return pool
	}

	// Legacy: single base_url + api_key + extra_keys
	baseURL := cfg.Provider.BaseURL

	if cfg.Provider.APIKey != "" {
		pool.Add(cfg.Provider.APIKey, "default", baseURL, 1, nil)
	}
	for _, kc := range cfg.Provider.ExtraKeys {
		pool.Add(kc.Key, kc.Label, baseURL, 1, nil)
	}

	return pool
}

// WithStore attaches a session store for auto-save.
func (a *Agent) WithStore(store *session.Store) *Agent {
	a.store = store
	return a
}

// WithSkills attaches a skill registry and injects skills into system prompt.
func (a *Agent) WithSkills(skills *skill.Registry) *Agent {
	a.skills = skills
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		a.messages[0].Content += skills.SystemPrompt()
	}
	return a
}

// WithTools applies tool filtering from a data store (per SPEC §4.3).
// Currently applies no filtering as tools config is loaded at agent creation time.
func (a *Agent) WithTools(ds ToolDataStore) *Agent {
	_ = ds
	return a
}

// WithRouter sets the multi-provider failover router.
func (a *Agent) WithRouter(router *provider.ProviderRouter) *Agent {
	a.router = router
	return a
}

// WithUsageLog sets the usage logging callback (per SPEC Phase 2.4).
func (a *Agent) WithUsageLog(fn func(providerID string, backendID int, model string, inputTokens, outputTokens int, costEst float64)) *Agent {
	a.usageLog = fn
	return a
}

// LoadSession restores a session from the store.
func (a *Agent) LoadSession(id string) error {
	if a.store == nil {
		return fmt.Errorf("no session store configured")
	}
	sess, err := a.store.Load(id)
	if err != nil {
		return err
	}
	a.sessionID = sess.ID
	a.messages = sess.Messages
	return nil
}

// SetSessionID sets the current session ID (for new sessions).
func (a *Agent) SetSessionID(id string) {
	a.sessionID = id
	// Fire session-start hook
	wd, _ := os.Getwd()
	if err := a.hooks.RunOnSessionStart(id, wd); err != nil {
		log.Printf("[agent] session-start hook: %v", err)
	}
}

// SessionID returns the current session ID.
func (a *Agent) SessionID() string { return a.sessionID }

// resolveClient selects a backend and creates a fresh Client for this request.
// Returns the client and the selected entry (for marking success/failure).
// Falls back through the ProviderRouter when all backends are exhausted.
func (a *Agent) resolveClient() (*provider.Client, *provider.PoolEntry) {
	entry, ok := a.pool.Select()
	if !ok {
		if a.router != nil {
			if failoverEntry, switched := a.router.SelectBackend(a.pool); switched && failoverEntry != nil {
				client := provider.NewClientFromEntry(failoverEntry, a.cfg.Model.Model)
				a.attachUsageCallback(client)
				return client, failoverEntry
			}
		}
		client := provider.NewClient(a.cfg.Provider.BaseURL, a.cfg.Provider.APIKey, a.cfg.Model.Model)
		a.attachUsageCallback(client)
		return client, nil
	}
	client := provider.NewClientFromEntry(entry, a.cfg.Model.Model)
	a.attachUsageCallback(client)
	return client, entry
}

// attachUsageCallback wires the agent's usage logger to the client.
func (a *Agent) attachUsageCallback(client *provider.Client) {
	if a.usageLog == nil {
		return
	}
	usageLog := a.usageLog
	model := a.cfg.Model.Model
	client.UsageCallback = func(inputTokens, outputTokens int, _ string) {
		usageLog("", 0, model, inputTokens, outputTokens, 0)
	}
}

// AddMessage appends a message.
func (a *Agent) AddMessage(role, content string) {
	a.messages = append(a.messages, provider.Message{Role: role, Content: content})
}

// AddToolResult appends a tool result with tool_call_id.
func (a *Agent) AddToolResult(toolCallID, content string) {
	a.messages = append(a.messages, provider.Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// Run executes the think→act→observe loop with automatic backend failover.
func (a *Agent) Run(userMessage string, onChunk func(chunk string)) (string, error) {
	msg := userMessage
	if a.skills != nil && strings.HasPrefix(strings.TrimSpace(userMessage), "/") {
		skillName := strings.TrimSpace(userMessage[1:])
		if s, ok := a.skills.Get(skillName); ok {
			msg = fmt.Sprintf("Use the following skill instructions:\n\n%s\n\n---\n\nNow help me with this skill.", s.Content)
		}
	}

	a.AddMessage("user", msg)
	a.running = true
	defer func() {
		a.running = false
		a.maybeSave()
	}()

	for a.turnCount < a.cfg.Agent.MaxTurns && a.running {
		a.turnCount++

		// Auto-compress context when message count exceeds threshold
		if len(a.messages) > 40 {
			a.CompressContext(8) // keep last 8 user-assistant pairs
		}

		toolDefs := a.registry.List()
		providerToolDefs := make([]provider.ToolDef, len(toolDefs))
		for i, td := range toolDefs {
			providerToolDefs[i] = provider.ToolDef{
				Type: "function",
				Function: provider.FunctionDef{
					Name:        td.Function.Name,
					Description: td.Function.Description,
					Parameters:  td.Function.Parameters,
				},
			}
		}

		// Try each available backend (max 3 retries)
		result, err := a.tryChatWithRetry(providerToolDefs, onChunk)
		if err != nil {
			return "", err
		}

		if result.assistantMsg != "" {
			a.messages = append(a.messages, provider.Message{Role: "assistant", Content: result.assistantMsg})
			return result.assistantMsg, nil
		}

		// Tool calls: add assistant message + execute tools
		a.messages = append(a.messages, provider.Message{
			Role:      "assistant",
			Content:   result.content,
			ToolCalls: result.toolCalls,
		})

		for _, tc := range result.toolCalls {
			// Sandbox approval gate
			risk := sandbox.RiskLevel(tc.Function.Name, tc.Function.Arguments)
			check := sandbox.Check{
				Tool:        tc.Function.Name,
				Args:        tc.Function.Arguments,
				Risk:        risk,
				Description: fmt.Sprintf("Execute %s", tc.Function.Name),
			}
			if !sandbox.RequestApproval(check) {
				a.AddToolResult(tc.ID, fmt.Sprintf("Tool execution rejected by user (risk: %s)", risk))
				continue
			}

			// Pre-tool hook
			if err := a.hooks.RunPreTool(hook.Context{
				SessionID: a.sessionID,
				ToolName:  tc.Function.Name,
				ToolArgs:  tc.Function.Arguments,
			}); err != nil {
				log.Printf("[agent] pre-tool hook blocked %s: %v", tc.Function.Name, err)
				a.AddToolResult(tc.ID, fmt.Sprintf("Tool execution blocked by pre-tool hook: %v", err))
				continue
			}

			execResult, execErr := a.registry.Execute(tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				return "", fmt.Errorf("tool execution failed (%s): %w", tc.Function.Name, execErr)
			}
			resultText := execResult.Output
			if !execResult.Success {
				resultText = fmt.Sprintf("Error: %s\nOutput: %s", execResult.Error, execResult.Output)
			}

			// Post-tool hook
			postOutput, postErr := a.hooks.RunPostTool(hook.Context{
				SessionID:  a.sessionID,
				ToolName:   tc.Function.Name,
				ToolArgs:   tc.Function.Arguments,
				ToolOutput: resultText,
				ToolError:  execResult.Error,
			})
			if postErr != nil {
				log.Printf("[agent] post-tool hook: %v", postErr)
			}
			if postOutput != "" {
				resultText = postOutput + "\n" + resultText
			}

			a.AddToolResult(tc.ID, resultText)
		}
	}

	if a.turnCount >= a.cfg.Agent.MaxTurns {
		return "", fmt.Errorf("max turns (%d) reached", a.cfg.Agent.MaxTurns)
	}
	return "", fmt.Errorf("agent stopped unexpectedly")
}

// chatResult holds the outcome of one ChatStream call.
type chatResult struct {
	assistantMsg string
	content      string
	toolCalls    []provider.ToolCall
}

// tryChatWithRetry attempts a chat call with up to 3 backend switches.
func (a *Agent) tryChatWithRetry(toolDefs []provider.ToolDef, onChunk func(string)) (*chatResult, error) {
	maxRetries := 3
	if a.pool.Len() < maxRetries {
		maxRetries = a.pool.Len()
	}
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error
	attempts := 0
	for attempts < maxRetries {
		attempts++
		client, entry := a.resolveClient()

		var fullContent strings.Builder
		var toolCalls []provider.ToolCall

		err := client.ChatStream(a.messages, toolDefs, a.cfg.Model.ReasoningEffort,
			func(delta string) {
				fullContent.WriteString(delta)
				if onChunk != nil {
					onChunk(delta)
				}
			},
			func(tcs []provider.ToolCall) {
				toolCalls = tcs
			},
		)

		if err == nil {
			if entry != nil {
				entry.MarkSuccess()
			}
			content := fullContent.String()
			if len(toolCalls) == 0 {
				return &chatResult{assistantMsg: content}, nil
			}
			return &chatResult{content: content, toolCalls: toolCalls}, nil
		}

		lastErr = err
		isRetryable := isRetryableError(err)
		if entry != nil {
			entry.MarkFailure(isRetryable)
		}

		if !isRetryable {
			break
		}
	}

	return nil, fmt.Errorf("all backends failed (tried %d/%d): %w", attempts, maxRetries, lastErr)
}

// isRetryableError determines if an error might succeed on another backend.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Rate limiting, server errors, timeouts — retry on another backend
	retryable := []string{"429", "500", "502", "503", "504", "timeout", "connection refused", "EOF", "reset by peer"}
	for _, s := range retryable {
		if strings.Contains(msg, s) {
			return true
		}
	}
	// Auth errors, bad requests — not retryable
	return false
}

func (a *Agent) maybeSave() {
	if a.store == nil || a.sessionID == "" {
		return
	}
	sess := &session.Session{
		ID:       a.sessionID,
		Model:    a.cfg.Model.Model,
		Provider: a.cfg.Model.Provider,
		Messages: a.messages,
	}
	if err := a.store.Save(sess); err != nil {
		fmt.Printf("[warn] session save failed: %v\n", err)
	}
}

// Stop halts the agent.
func (a *Agent) Stop() { a.running = false }

// CompressContext reduces message history when token count is high.
func (a *Agent) CompressContext(keepPairs int) {
	if len(a.messages) <= 2+keepPairs*2 {
		return
	}
	var compressed []provider.Message
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		compressed = append(compressed, a.messages[0])
	}
	start := len(a.messages) - keepPairs*2
	if start < 1 {
		start = 1
	}
	compressed = append(compressed, a.messages[start:]...)
	a.messages = compressed
}

// Config returns the agent's configuration.
func (a *Agent) Config() *config.Config { return a.cfg }

// AgentName returns the agent profile name.
func (a *Agent) AgentName() string { return a.agentName }

// Messages returns conversation history.
func (a *Agent) Messages() []provider.Message { return a.messages }

// TurnCount returns how many model calls were made.
func (a *Agent) TurnCount() int { return a.turnCount }

// IsThinking returns true while the agent is running a turn.
func (a *Agent) IsThinking() bool { return a.running }

// Pool returns the backend pool (for status reporting).
func (a *Agent) Pool() *provider.Pool { return a.pool }

// NewSessionID generates a timestamp-based session ID.
func NewSessionID() string {
	return time.Now().Format("20060102-150405")
}
