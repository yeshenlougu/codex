// Package mcp provides a Model Context Protocol (MCP) client over stdio.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

// ToolDescription describes a tool exposed by an MCP server.
type ToolDescription struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// MCPClient wraps a stdio connection to an MCP server.
type MCPClient struct {
	command string
	args    []string
	env     []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	closed bool

	// Server info
	ServerName    string `json:"serverName"`
	ServerVersion string `json:"serverVersion"`

	// Discovered tools
	Tools []ToolDescription `json:"tools"`

	// Status
	status string // "disconnected" | "connecting" | "connected" | "error"
	errMsg string
}

// NewMCPClient creates a new MCP client for a stdio-based MCP server.
func NewMCPClient(command string, args ...string) (*MCPClient, error) {
	c := &MCPClient{
		command: command,
		args:    args,
		status:  "disconnected",
	}
	return c, c.Start()
}

// Start launches the MCP server process and initializes the connection.
func (c *MCPClient) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == "connected" || c.status == "connecting" {
		return fmt.Errorf("MCP client already running")
	}

	c.status = "connecting"
	c.errMsg = ""

	cmd := exec.Command(c.command, c.args...)
	if len(c.env) > 0 {
		cmd.Env = c.env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.status = "error"
		c.errMsg = fmt.Sprintf("stdin pipe: %v", err)
		return err
	}
	c.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.status = "error"
		c.errMsg = fmt.Sprintf("stdout pipe: %v", err)
		return err
	}
	c.stdout = bufio.NewScanner(stdout)
	c.stdout.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	if err := cmd.Start(); err != nil {
		c.status = "error"
		c.errMsg = fmt.Sprintf("start: %v", err)
		return err
	}
	c.cmd = cmd

	// Initialize MCP session
	if err := c.initialize(); err != nil {
		c.status = "error"
		c.errMsg = fmt.Sprintf("initialize: %v", err)
		cmd.Process.Kill()
		return err
	}

	// Discover tools
	tools, err := c.listTools()
	if err != nil {
		c.status = "error"
		c.errMsg = fmt.Sprintf("list tools: %v", err)
		cmd.Process.Kill()
		return err
	}
	c.Tools = tools

	c.status = "connected"
	log.Printf("[mcp] server connected: %s %s (%d tools)", c.ServerName, c.ServerVersion, len(c.Tools))
	return nil
}

// Stop terminates the MCP server process.
func (c *MCPClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.status = "disconnected"

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	log.Printf("[mcp] server stopped: %s", c.ServerName)
	return nil
}

// Status returns the current connection status.
func (c *MCPClient) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// SetEnv sets environment variables for the MCP server process.
func (c *MCPClient) SetEnv(env map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.env = nil
	for k, v := range env {
		c.env = append(c.env, k+"="+v)
	}
}

// Close is an alias for Stop.
func (c *MCPClient) Close() {
	c.Stop()
}

// initialize sends the MCP initialize request.
func (c *MCPClient) initialize() error {
	resp, err := c.request("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "codex-go",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return err
	}
	if info, ok := resp["serverInfo"].(map[string]any); ok {
		if name, ok := info["name"].(string); ok {
			c.ServerName = name
		}
		if ver, ok := info["version"].(string); ok {
			c.ServerVersion = ver
		}
	}
	// Send initialized notification
	c.sendNotification("notifications/initialized", nil)
	return nil
}

// listTools sends tools/list and returns tool descriptions.
func (c *MCPClient) listTools() ([]ToolDescription, error) {
	resp, err := c.request("tools/list", nil)
	if err != nil {
		return nil, err
	}
	toolsRaw, ok := resp["tools"]
	if !ok {
		return nil, nil
	}
	toolsArr, ok := toolsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("tools/list response is not an array")
	}

	var tools []ToolDescription
	for _, t := range toolsArr {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		td := ToolDescription{}
		if n, ok := tm["name"].(string); ok {
			td.Name = n
		}
		if d, ok := tm["description"].(string); ok {
			td.Description = d
		}
		if s, ok := tm["inputSchema"].(map[string]any); ok {
			td.InputSchema = s
		}
		tools = append(tools, td)
	}
	return tools, nil
}

// CallTool invokes a named tool on the MCP server.
func (c *MCPClient) CallTool(name string, arguments map[string]any) (string, error) {
	result, err := c.request("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return "", err
	}
	content, ok := result["content"]
	if !ok {
		return "", fmt.Errorf("no content in tool response")
	}
	contentArr, ok := content.([]any)
	if !ok {
		return "", fmt.Errorf("unexpected content type")
	}
	var output string
	for _, item := range contentArr {
		im, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := im["text"].(string); ok {
			output += text
		}
	}
	return output, nil
}

// request sends a JSON-RPC request and returns the result.
func (c *MCPClient) request(method string, params any) (map[string]any, error) {
	id := uuid.New().String()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", reqBytes); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("unexpected EOF from MCP server")
	}

	var resp struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      string         `json:"id"`
		Result  map[string]any `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// sendNotification sends a JSON-RPC notification (no id, no response expected).
func (c *MCPClient) sendNotification(method string, params any) {
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		notif["params"] = params
	}
	b, _ := json.Marshal(notif)
	if c.stdin != nil {
		fmt.Fprintf(c.stdin, "%s\n", b)
	}
}
