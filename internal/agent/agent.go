package agent

import (
	"fmt"
	"strings"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/provider"
	"github.com/yeshenlougu/codex/internal/tool"
)

// Agent is the core coding agent that runs the think→act→observe loop.
type Agent struct {
	cfg      *config.Config
	client   *provider.Client
	registry *tool.Registry

	// Conversation history
	messages []provider.Message

	// State
	turnCount int
	running   bool
}

// New creates a new Agent instance.
func New(cfg *config.Config) *Agent {
	client := provider.NewClient(cfg.Provider.BaseURL, cfg.Provider.APIKey, cfg.Model.Model)
	registry := tool.NewRegistry()

	// Register enabled tools
	if cfg.Tools.Shell {
		registry.Register(tool.NewShellTool())
	}
	if cfg.Tools.FileRead {
		registry.Register(tool.NewFileReadTool())
	}
	if cfg.Tools.FileEdit {
		registry.Register(tool.NewFileEditTool())
	}

	return &Agent{
		cfg:      cfg,
		client:   client,
		registry: registry,
		messages: []provider.Message{
			{Role: "system", Content: cfg.Agent.SystemPrompt},
		},
	}
}

// AddMessage appends a message to the conversation history.
func (a *Agent) AddMessage(role, content string) {
	a.messages = append(a.messages, provider.Message{Role: role, Content: content})
}

// AddToolResult appends a tool result message with the tool_call_id.
func (a *Agent) AddToolResult(toolCallID, content string) {
	a.messages = append(a.messages, provider.Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// Run starts the agent with an initial user message.
// It runs the think→act→observe loop until the model responds without tool calls
// or the maximum number of turns is reached.
func (a *Agent) Run(userMessage string, onChunk func(chunk string)) (string, error) {
	a.AddMessage("user", userMessage)
	a.running = true

	for a.turnCount < a.cfg.Agent.MaxTurns && a.running {
		a.turnCount++

		// Get enabled tool definitions
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

		// Call the model with streaming
		var fullContent strings.Builder
		var toolCalls []provider.ToolCall

		err := a.client.ChatStream(
			a.messages,
			providerToolDefs,
			a.cfg.Model.ReasoningEffort,
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
			return "", fmt.Errorf("API call failed (turn %d): %w", a.turnCount, err)
		}

		// If the model responded with content but no tool calls, we're done
		if len(toolCalls) == 0 {
			assistantMsg := fullContent.String()
			if assistantMsg != "" {
				a.AddMessage("assistant", assistantMsg)
			}
			return assistantMsg, nil
		}

		// Model requested tool calls — execute them
		content := fullContent.String()

		// Store assistant message with tool calls for proper API format
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

			// Add tool result to conversation
			resultText := result.Output
			if !result.Success {
				resultText = fmt.Sprintf("Error: %s\nOutput: %s", result.Error, result.Output)
			}

			a.AddToolResult(tc.ID, resultText)
		}
	}

	if a.turnCount >= a.cfg.Agent.MaxTurns {
		return "", fmt.Errorf("reached maximum turns (%d) without completion", a.cfg.Agent.MaxTurns)
	}

	return "", fmt.Errorf("agent stopped unexpectedly")
}

// Stop gracefully stops the agent loop.
func (a *Agent) Stop() {
	a.running = false
}

// Config returns the agent's configuration.
func (a *Agent) Config() *config.Config {
	return a.cfg
}

// Messages returns the current conversation history.
func (a *Agent) Messages() []provider.Message {
	return a.messages
}

// TurnCount returns how many model calls have been made.
func (a *Agent) TurnCount() int {
	return a.turnCount
}
