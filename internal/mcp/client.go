package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// ServerInfo is the response to initialize.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolDescription describes a tool from an MCP server.
type ToolDescription struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// MCPClient connects to an MCP server via stdio.
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	id     int
	Info   ServerInfo
	Tools  []ToolDescription
}

// NewMCPClient starts an MCP server process.
func NewMCPClient(command string, args ...string) (*MCPClient, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// MCP uses stderr for logging, not protocol
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start server: %w", err)
	}

	c := &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}

	// Initialize
	if err := c.initialize(); err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	// List tools
	if err := c.listTools(); err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	return c, nil
}

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *MCPClient) send(method string, params interface{}) (*jsonrpcResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// MCP line-delimited JSON
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	if err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	if !c.stdout.Scan() {
		return nil, fmt.Errorf("no response from MCP server")
	}

	line := c.stdout.Text()
	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

func (c *MCPClient) initialize() error {
	resp, err := c.send("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "codex-go",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return err
	}

	// Send initialized notification
	fmt.Fprintf(c.stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")

	if err := json.Unmarshal(resp.Result, &c.Info); err != nil {
		return fmt.Errorf("parse server info: %w", err)
	}

	return nil
}

func (c *MCPClient) listTools() error {
	resp, err := c.send("tools/list", nil)
	if err != nil {
		return err
	}

	var result struct {
		Tools []ToolDescription `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse tools: %w", err)
	}

	c.Tools = result.Tools
	return nil
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(name string, args map[string]any) (string, error) {
	resp, err := c.send("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return string(resp.Result), nil
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	output := strings.Join(texts, "\n")
	if result.IsError {
		return output, fmt.Errorf("MCP tool error: %s", output)
	}

	return output, nil
}

// Close kills the MCP server process.
func (c *MCPClient) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
