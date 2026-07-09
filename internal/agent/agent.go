package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/provider"
	"github.com/yeshenlougu/codex/internal/session"
	"github.com/yeshenlougu/codex/internal/tool"
)

// Agent is the core coding agent.
type Agent struct {
	cfg      *config.Config
	client   *provider.Client
	registry *tool.Registry
	pool     *provider.KeyPool
	store    *session.Store

	// Session state
	sessionID string
	messages  []provider.Message
	turnCount int
	running   bool
}

// New creates an Agent with all enabled tools.
func New(cfg *config.Config) *Agent {
	registry := tool.NewRegistry()
	registry.Register(tool.NewShellTool())
	registry.Register(tool.NewFileReadTool())
	registry.Register(tool.NewFileEditTool())
	registry.Register(tool.NewWriteFileTool())
	registry.Register(tool.NewGrepTool())
	registry.Register(tool.NewLsTool())
	registry.Register(tool.NewGitTool())
	registry.Register(tool.NewWebFetchTool())

	// Build key pool if multiple keys configured
	pool := provider.NewKeyPool(cfg.Provider.PoolStrategy)
	if cfg.Provider.APIKey != "" {
		pool.Add(cfg.Provider.APIKey, "default", cfg.Provider.BaseURL)
	}
	for _, kc := range cfg.Provider.ExtraKeys {
		pool.Add(kc.Key, kc.Label, cfg.Provider.BaseURL)
	}

	return &Agent{
		cfg:      cfg,
		registry: registry,
		pool:     pool,
		messages: []provider.Message{
			{Role: "system", Content: cfg.Agent.SystemPrompt},
		},
	}
}

// WithStore attaches a session store for auto-save.
func (a *Agent) WithStore(store *session.Store) *Agent {
	a.store = store
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
}

// SessionID returns the current session ID.
func (a *Agent) SessionID() string { return a.sessionID }

func (a *Agent) resolveClient() *provider.Client {
	if a.client != nil {
		return a.client
	}
	entry, ok := a.pool.Select()
	if !ok {
		a.client = provider.NewClient(a.cfg.Provider.BaseURL, a.cfg.Provider.APIKey, a.cfg.Model.Model)
	} else {
		a.client = provider.NewClient(a.cfg.Provider.BaseURL, entry.Key, a.cfg.Model.Model)
	}
	return a.client
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

// Run executes the think→act→observe loop.
func (a *Agent) Run(userMessage string, onChunk func(chunk string)) (string, error) {
	a.AddMessage("user", userMessage)
	a.running = true
	defer a.maybeSave()

	for a.turnCount < a.cfg.Agent.MaxTurns && a.running {
		a.turnCount++

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

		var fullContent strings.Builder
		var toolCalls []provider.ToolCall

		client := a.resolveClient()
		usedKey := client.APIKey

		err := client.ChatStream(a.messages, providerToolDefs, a.cfg.Model.ReasoningEffort,
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

		if err != nil {
			if a.pool.Len() > 1 {
				a.pool.MarkFailure(usedKey)
			}
			return "", fmt.Errorf("API call failed (turn %d): %w", a.turnCount, err)
		}
		a.pool.MarkSuccess(usedKey)

		if len(toolCalls) == 0 {
			assistantMsg := fullContent.String()
			if assistantMsg != "" {
				a.messages = append(a.messages, provider.Message{Role: "assistant", Content: assistantMsg})
			}
			return assistantMsg, nil
		}

		content := fullContent.String()
		a.messages = append(a.messages, provider.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		})

		for _, tc := range toolCalls {
			result, execErr := a.registry.Execute(tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				return "", fmt.Errorf("tool execution failed (%s): %w", tc.Function.Name, execErr)
			}
			resultText := result.Output
			if !result.Success {
				resultText = fmt.Sprintf("Error: %s\nOutput: %s", result.Error, result.Output)
			}
			a.AddToolResult(tc.ID, resultText)
		}
	}

	if a.turnCount >= a.cfg.Agent.MaxTurns {
		return "", fmt.Errorf("max turns (%d) reached", a.cfg.Agent.MaxTurns)
	}
	return "", fmt.Errorf("agent stopped unexpectedly")
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

// Config returns the agent config.
func (a *Agent) Config() *config.Config { return a.cfg }

// Messages returns conversation history.
func (a *Agent) Messages() []provider.Message { return a.messages }

// TurnCount returns how many model calls were made.
func (a *Agent) TurnCount() int { return a.turnCount }

// NewSessionID generates a timestamp-based session ID.
func NewSessionID() string {
	return time.Now().Format("20060102-150405")
}
