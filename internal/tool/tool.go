package tool

import (
	"encoding/json"
	"fmt"
)

// Result is the output of a tool execution.
type Result struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the tool name (used in function calling).
	Name() string
	// Description returns a human-readable description.
	Description() string
	// Schema returns the JSON Schema for the tool's parameters.
	Schema() map[string]any
	// Execute runs the tool with the given JSON arguments.
	Execute(args string) (*Result, error)
}

// Registry holds all available tools and dispatches calls.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools as ToolDefs for the LLM.
func (r *Registry) List() []ToolDef {
	defs := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToolDef{
			Type: "function",
			Function: FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	return defs
}

// Execute runs a tool call and returns the result.
func (r *Registry) Execute(name, args string) (*Result, error) {
	t, ok := r.Get(name)
	if !ok {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}, nil
	}

	// Validate JSON arguments
	if args != "" {
		var v any
		if err := json.Unmarshal([]byte(args), &v); err != nil {
			return &Result{
				Success: false,
				Error:   fmt.Sprintf("invalid arguments: %v", err),
			}, nil
		}
	}

	return t.Execute(args)
}

// ---- Shared types (avoid circular imports with provider) ----

// ToolDef mirrors provider.ToolDef to avoid import cycles.
type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a function tool's schema.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
