// Package plugin provides plugin loading and process management.
package plugin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
)

// PluginProcess manages an external plugin subprocess communicating via stdin/stdout JSON-RPC.
type PluginProcess struct {
	name    string
	command string
	args   []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	closed bool
	status string // "running" | "stopped" | "error"
	errMsg string
}

// NewPluginProcess creates a new plugin process and starts it.
func NewPluginProcess(name, command string, args ...string) (*PluginProcess, error) {
	p := &PluginProcess{
		name:    name,
		command: command,
		args:    args,
		status:  "stopped",
	}
	if err := p.Start(); err != nil {
		return nil, err
	}
	return p, nil
}

// Start launches the plugin subprocess.
func (p *PluginProcess) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == "running" {
		return fmt.Errorf("plugin %s already running", p.name)
	}

	p.status = "starting"
	p.errMsg = ""
	p.closed = false

	cmd := exec.Command(p.command, p.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		p.status = "error"
		p.errMsg = err.Error()
		return err
	}
	p.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.status = "error"
		p.errMsg = err.Error()
		return err
	}
	p.stdout = bufio.NewScanner(stdout)
	p.stdout.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if err := cmd.Start(); err != nil {
		p.status = "error"
		p.errMsg = err.Error()
		return err
	}
	p.cmd = cmd
	p.status = "running"
	log.Printf("[plugin] process started: %s (%s %v)", p.name, p.command, p.args)
	return nil
}

// Stop terminates the plugin process.
func (p *PluginProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true
	p.status = "stopped"

	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	log.Printf("[plugin] process stopped: %s", p.name)
	return nil
}

// Status returns the current process status.
func (p *PluginProcess) Status() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// CallTool sends a JSON-RPC tool call to the plugin and returns the response.
func (p *PluginProcess) CallTool(method string, params map[string]any) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status != "running" {
		return "", fmt.Errorf("plugin %s not running (status: %s)", p.name, p.status)
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	if _, err := fmt.Fprintf(p.stdin, "%s\n", b); err != nil {
		return "", fmt.Errorf("write to plugin: %w", err)
	}

	if !p.stdout.Scan() {
		if err := p.stdout.Err(); err != nil {
			return "", fmt.Errorf("read from plugin: %w", err)
		}
		return "", fmt.Errorf("plugin EOF")
	}

	var resp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(p.stdout.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse plugin response: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("plugin error: %s", resp.Error.Message)
	}
	return resp.Result, nil
}
