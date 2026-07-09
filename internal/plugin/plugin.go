// Package plugin provides dynamic tool loading from plugin directories.
// Plugins are JSON files that describe simple command-based tools.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/tool"
)

// PluginDef is loaded from a .plugin.json file.
type PluginDef struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Timeout     int               `json:"timeout"`
	Schema      map[string]any    `json:"schema"`
	Env         map[string]string `json:"env,omitempty"`
}

// LoadDir scans a directory for .plugin.json files and returns tool implementations.
func LoadDir(dir string) ([]tool.Tool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tools []tool.Tool
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".plugin.json") {
			continue
		}
		pluginPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(pluginPath)
		if err != nil {
			continue
		}
		var def PluginDef
		if err := json.Unmarshal(data, &def); err != nil {
			continue
		}
		tools = append(tools, &CommandPlugin{def: def})
	}

	return tools, nil
}

// CommandPlugin wraps a shell command as a tool.
type CommandPlugin struct {
	def PluginDef
}

func (p *CommandPlugin) Name() string        { return p.def.Name }
func (p *CommandPlugin) Description() string { return p.def.Description }
func (p *CommandPlugin) Schema() map[string]any {
	if p.def.Schema != nil {
		return p.def.Schema
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (p *CommandPlugin) Execute(rawArgs string) (*tool.Result, error) {
	def := p.def

	// Parse arguments from raw JSON
	var userArgs map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &userArgs); err != nil {
			return &tool.Result{Success: false, Error: fmt.Sprintf("invalid args: %v", err)}, nil
		}
	}

	// Build command arguments by substituting placeholders
	cmdArgs := make([]string, len(def.Args))
	copy(cmdArgs, def.Args)
	for i, arg := range cmdArgs {
		for k, v := range userArgs {
			arg = strings.ReplaceAll(arg, "{{"+k+"}}", fmt.Sprint(v))
		}
		cmdArgs[i] = arg
	}

	// Build env
	env := os.Environ()
	for k, v := range def.Env {
		val := v
		for uk, uv := range userArgs {
			val = strings.ReplaceAll(val, "{{"+uk+"}}", fmt.Sprint(uv))
		}
		env = append(env, k+"="+val)
	}

	timeout := def.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	output, err := runCommand(def.Command, cmdArgs, env, timeout)
	if err != nil {
		return &tool.Result{Success: false, Error: err.Error(), Output: output}, nil
	}
	return &tool.Result{Success: true, Output: output}, nil
}

func runCommand(cmdName string, args []string, env []string, timeoutSec int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	outStr := string(out)
	if len(outStr) > 8000 {
		outStr = outStr[:8000] + "\n..."
	}
	if err != nil {
		return outStr, fmt.Errorf("%w: %s", err, outStr)
	}
	return outStr, nil
}
