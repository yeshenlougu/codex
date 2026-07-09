package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeshenlougu/codex/internal/config"
	"gopkg.in/yaml.v3"
)

// AgentProfile defines a complete, self-contained agent configuration that
// can be stored as a YAML file under ~/.codex/agents/.
// The built-in "default" profile lives in code and is never written to disk;
// users can clone it to create custom profiles.
type AgentProfile struct {
	Name        string               `yaml:"name" json:"name"`
	Description string               `yaml:"description" json:"description"`
	Avatar      string               `yaml:"avatar" json:"avatar"` // emoji or icon
	Model       ProfileModelConfig   `yaml:"model" json:"model"`
	Agent       config.AgentConfig   `yaml:"agent" json:"agent"`
	Tools       config.ToolsConfig   `yaml:"tools" json:"tools"`
	MCP         config.MCPConfig     `yaml:"mcp" json:"mcp"`
	Skills      config.SkillsConfig  `yaml:"skills" json:"skills"`
	Plugins     config.PluginsConfig `yaml:"plugins" json:"plugins"`
	Hooks       config.HooksConfig   `yaml:"hooks" json:"hooks"`
	SubAgents   []SubAgentRef        `yaml:"subagents,omitempty" json:"subagents,omitempty"`

	// FilePath is the on-disk path (empty for built-in).
	FilePath string `yaml:"-" json:"-"`
	// IsBuiltin is true for the immutable "default" profile.
	IsBuiltin bool `yaml:"-" json:"is_builtin"`
}

// ProfileModelConfig mirrors config.ModelConfig but is embedded in the profile.
type ProfileModelConfig struct {
	Provider        string `yaml:"provider" json:"provider"`
	Model           string `yaml:"model" json:"model"`
	ReasoningEffort string `yaml:"reasoning_effort,omitempty" json:"reasoning_effort,omitempty"`
}

// SubAgentRef names another agent profile that this agent can delegate to.
type SubAgentRef struct {
	Name        string `yaml:"name" json:"name"`                 // profile name
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// ProviderConfig returns a config.ProviderConfig needed to build a Pool.
// The actual API key and base URL are resolved from the profile's provider
// combined with the master config's backends.
func (p *AgentProfile) ProviderConfig() config.ProviderConfig {
	return config.ProviderConfig{}
}

// ApplyToConfig overlays profile settings onto a base config, returning a
// new config suitable for creating an Agent instance.  Fields explicitly
// set in the profile override the base; empty fields fall through.
func (p *AgentProfile) ApplyToConfig(base *config.Config) *config.Config {
	cfg := *base // shallow copy

	if p.Model.Provider != "" {
		cfg.Model.Provider = p.Model.Provider
	}
	if p.Model.Model != "" {
		cfg.Model.Model = p.Model.Model
	}
	if p.Model.ReasoningEffort != "" {
		cfg.Model.ReasoningEffort = p.Model.ReasoningEffort
	}
	if p.Agent.MaxTurns > 0 {
		cfg.Agent.MaxTurns = p.Agent.MaxTurns
	}
	if p.Agent.SystemPrompt != "" {
		cfg.Agent.SystemPrompt = p.Agent.SystemPrompt
	}

	// Tools: if profile specifies any tool, use profile's Tools block entirely;
	// otherwise keep base.  Use a zero-value sentinel: if all three are false
	// AND we're not the default (which has them all true), keep base.
	if p.Tools.Shell || p.Tools.FileRead || p.Tools.FileEdit {
		cfg.Tools = p.Tools
	}

	// MCP: merge — profile MCP servers append to base
	if len(p.MCP.Servers) > 0 {
		cfg.MCP.Servers = append(cfg.MCP.Servers, p.MCP.Servers...)
	}

	// Skills: merge dirs
	if len(p.Skills.Dirs) > 0 {
		cfg.Skills.Dirs = append(cfg.Skills.Dirs, p.Skills.Dirs...)
	}

	// Plugins: merge dirs
	if len(p.Plugins.Dirs) > 0 {
		cfg.Plugins.Dirs = append(cfg.Plugins.Dirs, p.Plugins.Dirs...)
	}

	// Hooks: profile hooks override base when non-empty
	if p.Hooks.PreTool != "" {
		cfg.Hooks.PreTool = p.Hooks.PreTool
	}
	if p.Hooks.PostTool != "" {
		cfg.Hooks.PostTool = p.Hooks.PostTool
	}
	if p.Hooks.OnSessionStart != "" {
		cfg.Hooks.OnSessionStart = p.Hooks.OnSessionStart
	}
	if p.Hooks.OnSessionEnd != "" {
		cfg.Hooks.OnSessionEnd = p.Hooks.OnSessionEnd
	}
	if p.Hooks.PostToolMessage != "" {
		cfg.Hooks.PostToolMessage = p.Hooks.PostToolMessage
	}

	return &cfg
}

// BuiltinDefaultProfile returns the immutable system default agent profile.
func BuiltinDefaultProfile() *AgentProfile {
	return &AgentProfile{
		Name:        "default",
		Description: "The built-in system agent. Cannot be modified.",
		Avatar:      "🤖",
		Model: ProfileModelConfig{
			Provider:        "openai",
			Model:           "gpt-4o",
			ReasoningEffort: "high",
		},
		Agent: config.AgentConfig{
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
		Tools: config.ToolsConfig{
			Shell:    true,
			FileRead: true,
			FileEdit: true,
		},
		IsBuiltin: true,
	}
}

// Save writes the profile to disk as YAML.  Refuses to overwrite built-in.
func (p *AgentProfile) Save() error {
	if p.IsBuiltin {
		return fmt.Errorf("cannot save built-in profile %q", p.Name)
	}
	if p.FilePath == "" {
		return fmt.Errorf("profile %q has no file path", p.Name)
	}
	dir := filepath.Dir(p.FilePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}
	return os.WriteFile(p.FilePath, data, 0600)
}

// Clone creates a copy with a new name.  The caller must set FilePath and call
// Save() to persist.
func (p *AgentProfile) Clone(newName string) *AgentProfile {
	clone := *p
	clone.Name = newName
	clone.IsBuiltin = false
	clone.FilePath = ""
	clone.Description = fmt.Sprintf("Clone of %s", p.Name)
	return &clone
}

// LoadProfile reads an agent profile from a YAML file.
func LoadProfile(path string) (*AgentProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %w", path, err)
	}
	var p AgentProfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse profile %s: %w", path, err)
	}
	// Derive name from filename when not set in YAML
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	p.FilePath = path
	p.IsBuiltin = false
	return &p, nil
}
