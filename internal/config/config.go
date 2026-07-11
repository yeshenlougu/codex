package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the Codex agent.
type Config struct {
	Model    ModelConfig    `yaml:"model"`
	Tools    ToolsConfig    `yaml:"tools"`
	Agent    AgentConfig    `yaml:"agent"`
	Provider ProviderConfig `yaml:"provider"`
	MCP      MCPConfig      `yaml:"mcp"`
	Skills   SkillsConfig   `yaml:"skills"`
	Plugins  PluginsConfig  `yaml:"plugins"`
	Hooks    HooksConfig    `yaml:"hooks"`
	Agents   AgentsConfig   `yaml:"agents"`
}

// ModelConfig is the model selection configuration.
type ModelConfig struct {
	Provider        string `yaml:"provider" json:"provider"`
	Model           string `yaml:"model" json:"model"`
	ReasoningEffort string `yaml:"reasoning_effort" json:"reasoning_effort"`
}

// ToolsConfig controls which tools are enabled.
type ToolsConfig struct {
	Shell    bool `yaml:"shell" json:"shell"`
	FileRead bool `yaml:"file_read" json:"file_read"`
	FileEdit bool `yaml:"file_edit" json:"file_edit"`
}

// AgentConfig controls agent behavior.
type AgentConfig struct {
	MaxTurns     int    `yaml:"max_turns" json:"max_turns"`
	SystemPrompt string `yaml:"system_prompt" json:"system_prompt"`
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	BaseURL      string          `yaml:"base_url"`
	APIKey       string          `yaml:"api_key"`
	WireAPI      string          `yaml:"wire_api"`
	PoolStrategy string          `yaml:"pool_strategy"` // "fill_first", "round_robin", "random"
	ExtraKeys    []KeyConfig     `yaml:"extra_keys"`    // legacy: additional keys sharing base_url
	Backends     []BackendConfig `yaml:"backends"`      // multi-endpoint pool (each with own key + base_url)
}

// KeyConfig is an additional API key entry (legacy).
type KeyConfig struct {
	Key   string `yaml:"key"`
	Label string `yaml:"label"`
}

// BackendConfig is a fully independent API endpoint entry.
// When configured, Codex acts as its own cc-switch: routing requests
// across multiple backends with automatic failover and health checks.
// Models field optionally declares known models and their types;
// if empty, models are auto-discovered from the /models endpoint during health checks.
type BackendConfig struct {
	Key      string            `yaml:"key" json:"key"`
	Label    string            `yaml:"label" json:"label"`
	BaseURL  string            `yaml:"base_url" json:"base_url"`
	Weight   int               `yaml:"weight" json:"weight"`
	Models   []ModelEntry      `yaml:"models,omitempty" json:"models,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// ModelEntry declares a known model with optional metadata.
// Type can be left empty to auto-detect from the model name.
type ModelEntry struct {
	Name          string `yaml:"name" json:"name"`
	Type          string `yaml:"type,omitempty" json:"type,omitempty"`
	ContextLength int    `yaml:"context_length,omitempty" json:"context_length,omitempty"`
}

// MCPConfig configures Model Context Protocol servers.
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers" json:"servers"`
}

// MCPServerConfig is a single MCP server (stdio transport).
type MCPServerConfig struct {
	Name    string   `yaml:"name" json:"name"`
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args" json:"args"`
	Env     []string `yaml:"env,omitempty" json:"env,omitempty"`
	Enabled bool     `yaml:"enabled" json:"enabled"`
}

// SkillsConfig configures skill directory scanning.
type SkillsConfig struct {
	Dirs []string `yaml:"dirs" json:"dirs"`
}

// PluginsConfig configures plugin directory scanning.
type PluginsConfig struct {
	Dirs []string `yaml:"dirs" json:"dirs"`
}

// HooksConfig configures lifecycle and tool hooks.
type HooksConfig struct {
	PreTool         string `yaml:"pre_tool" json:"pre_tool"`
	PostTool        string `yaml:"post_tool" json:"post_tool"`
	OnSessionStart  string `yaml:"on_session_start" json:"on_session_start"`
	OnSessionEnd    string `yaml:"on_session_end" json:"on_session_end"`
	PostToolMessage string `yaml:"post_tool_message" json:"post_tool_message"`
}

// AgentsConfig configures the multi-agent system.
type AgentsConfig struct {
	Dir       string `yaml:"dir"`        // directory for user agent profiles (default ~/.codex/agents)
	Default   string `yaml:"default"`    // default agent profile name (default "default")
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		Model: ModelConfig{
			Provider:        "openai",
			Model:           "gpt-4o",
			ReasoningEffort: "high",
		},
		Tools: ToolsConfig{
			Shell:    true,
			FileRead: true,
			FileEdit: true,
		},
		Agent: AgentConfig{
			MaxTurns: 60,
			SystemPrompt: `You are Codex, an AI coding agent that runs in the terminal.
You have access to shell commands, file operations, and a conversation history.
Use these tools to help the user write, debug, and understand code.

Rules:
- Read files before editing them
- Use shell commands to build, test, and explore
- Explain your reasoning before taking action
- When editing files, show what you changed and why

Slash Commands:
The user may use slash commands at the start of a message:

/steer <description>
  Enter guided development mode for a feature.  Go through these phases IN ORDER:
  1. SPEC phase: Write a specification document (SPEC-<slug>.md) covering
     background, goals, design, impact, and roadmap.
  2. PLAN phase: Read the SPEC you just created, then write PLAN.md with
     phased implementation plan.  Each phase has checkbox tasks:
     "- [ ] Task N: <description> — 预计 <N>天"
  3. IMPLEMENT phase: Work through Phase 1 tasks in PLAN.md one by one.
     For each task: read relevant files, implement changes, verify with
     shell commands, then mark done by changing "- [ ]" to "- [x]".
  Important: Complete all three phases.  Report progress after each phase.
  Use Chinese for spec/plan files if the description is in Chinese.

/spec <description>
  Write a specification only (Phase 1 of /steer).  Create SPEC-<slug>.md.

/plan
  Read the existing SPEC file and write PLAN.md (Phase 2 of /steer).

/tasks
  Parse PLAN.md and list all tasks with completion status.

/implement <number>
  Mark a specific task as done in PLAN.md.

/execute <number>
  Read the task from PLAN.md and implement it now (run shell, edit files).

These commands are just conversational cues — process them within your normal
tool-calling flow.  Do not wait for confirmation between phases.`,
		},
		Provider: ProviderConfig{
			WireAPI: "chat_completions",
		},
	}
}

// Load reads config from the given path, falling back to defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".codex", "config.yaml")
		}
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
			// File doesn't exist — continue with defaults + env
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		}
	}

	// Always resolve API key and base URL from environment as fallback
	if cfg.Provider.APIKey == "" {
		cfg.Provider.APIKey = resolveAPIKey(cfg.Model.Provider)
	}
	if cfg.Provider.BaseURL == "" {
		cfg.Provider.BaseURL = resolveBaseURL(cfg.Model.Provider)
	}

	return cfg, nil
}

func resolveAPIKey(provider string) string {
	switch provider {
	case "openai", "beecode", "custom":
		return os.Getenv("OPENAI_API_KEY")
	case "deepseek":
		return os.Getenv("DEEPSEEK_API_KEY")
	default:
		return os.Getenv("OPENAI_API_KEY")
	}
}

func resolveBaseURL(provider string) string {
	if url := os.Getenv("OPENAI_BASE_URL"); url != "" {
		return url
	}
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	default:
		return "https://api.openai.com/v1"
	}
}
