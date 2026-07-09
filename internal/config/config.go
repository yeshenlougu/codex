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
}

// ModelConfig is the model selection configuration.
type ModelConfig struct {
	Provider        string `yaml:"provider"`         // e.g. "openai", "beecode", "custom"
	Model           string `yaml:"model"`            // e.g. "gpt-5.5", "deepseek-v4-pro"
	ReasoningEffort string `yaml:"reasoning_effort"` // "low", "medium", "high", "xhigh"
}

// ToolsConfig controls which tools are enabled.
type ToolsConfig struct {
	Shell    bool `yaml:"shell"`     // shell execution
	FileRead bool `yaml:"file_read"` // file reading
	FileEdit bool `yaml:"file_edit"` // file editing
}

// AgentConfig controls agent behavior.
type AgentConfig struct {
	MaxTurns     int    `yaml:"max_turns"`
	SystemPrompt string `yaml:"system_prompt"`
}

// ProviderConfig holds provider-specific settings.
type ProviderConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	WireAPI string `yaml:"wire_api"` // "chat_completions" or "responses"
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
- When editing files, show what you changed and why`,
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
