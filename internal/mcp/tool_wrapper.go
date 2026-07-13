package mcp

import (
	"encoding/json"

	"github.com/yeshenlougu/codex/internal/tool"
)

// ToolWrapper implements the tool.Tool interface for an MCP tool.
type ToolWrapper struct {
	client *MCPClient
	desc   ToolDescription
}

// NewToolWrapper creates a tool wrapper for use in a tool.Registry.
func NewToolWrapper(client *MCPClient, desc ToolDescription) *ToolWrapper {
	return &ToolWrapper{client: client, desc: desc}
}

func (w *ToolWrapper) Name() string       { return w.desc.Name }
func (w *ToolWrapper) Description() string { return w.desc.Description }
func (w *ToolWrapper) Schema() map[string]any {
	if w.desc.InputSchema != nil {
		return w.desc.InputSchema
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (w *ToolWrapper) Execute(rawArgs string) (*tool.Result, error) {
	var args map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return &tool.Result{Success: false, Error: "invalid mcp args: " + err.Error()}, nil
		}
	}
	output, err := w.client.CallTool(w.desc.Name, args)
	if err != nil {
		return &tool.Result{Success: false, Error: err.Error(), Output: output}, nil
	}
	return &tool.Result{Success: true, Output: output}, nil
}
