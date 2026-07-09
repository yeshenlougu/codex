// Package hook provides lifecycle and tool-execution hooks for the agent.
// Hooks are shell scripts that receive structured context via environment variables
// and JSON on stdin, following the Codex hook protocol.
package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Context carries information about the current operation.
type Context struct {
	SessionID  string `json:"session_id"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolArgs   string `json:"tool_args,omitempty"`
	ToolOutput string `json:"tool_output,omitempty"`
	ToolError  string `json:"tool_error,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	Event      string `json:"event"` // "pre_tool", "post_tool", "session_start", "session_end"
}

// Runner executes hook scripts.
type Runner struct {
	preTool         string
	postTool        string
	onSessionStart  string
	onSessionEnd    string
	postToolMessage string
	timeout         time.Duration
}

// NewRunner creates a hook runner from config.
func NewRunner(pre, post, onStart, onEnd, postMsg string) *Runner {
	return &Runner{
		preTool:         pre,
		postTool:        post,
		onSessionStart:  onStart,
		onSessionEnd:    onEnd,
		postToolMessage: postMsg,
		timeout:         30 * time.Second,
	}
}

// RunPreTool fires the pre-tool hook. Returns an error to block tool execution.
func (r *Runner) RunPreTool(ctx Context) error {
	if r.preTool == "" {
		return nil
	}
	ctx.Event = "pre_tool"
	return r.run(r.preTool, ctx)
}

// RunPostTool fires the post-tool hook. Output is captured but errors are non-fatal.
func (r *Runner) RunPostTool(ctx Context) (string, error) {
	if r.postTool == "" {
		return "", nil
	}
	ctx.Event = "post_tool"
	return r.runCapture(r.postTool, ctx)
}

// RunOnSessionStart fires when a session begins.
func (r *Runner) RunOnSessionStart(sessionID, workingDir string) error {
	if r.onSessionStart == "" {
		return nil
	}
	return r.run(r.onSessionStart, Context{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Event:      "session_start",
	})
}

// RunOnSessionEnd fires when a session ends.
func (r *Runner) RunOnSessionEnd(sessionID string) error {
	if r.onSessionEnd == "" {
		return nil
	}
	return r.run(r.onSessionEnd, Context{
		SessionID: sessionID,
		Event:     "session_end",
	})
}

// RunPostToolMessage fires after the LLM responds with tool calls.
func (r *Runner) RunPostToolMessage(sessionID string, messagesJSON string) error {
	if r.postToolMessage == "" {
		return nil
	}
	ctx := Context{
		SessionID:  sessionID,
		Event:      "post_tool_message",
		ToolOutput: messagesJSON,
	}
	return r.run(r.postToolMessage, ctx)
}

func (r *Runner) run(script string, ctx Context) error {
	ctxJSON, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("hook marshal: %w", err)
	}

	execCtx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"CODEX_SESSION_ID="+ctx.SessionID,
		"CODEX_EVENT="+ctx.Event,
		"CODEX_TOOL_NAME="+ctx.ToolName,
	)

	stdin, _ := cmd.StdinPipe()
	go func() {
		stdin.Write(ctxJSON)
		stdin.Close()
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook %s failed: %w\n%s", ctx.Event, err, string(out))
	}
	return nil
}

func (r *Runner) runCapture(script string, ctx Context) (string, error) {
	ctxJSON, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("hook marshal: %w", err)
	}

	execCtx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"CODEX_SESSION_ID="+ctx.SessionID,
		"CODEX_EVENT="+ctx.Event,
		"CODEX_TOOL_NAME="+ctx.ToolName,
	)

	stdin, _ := cmd.StdinPipe()
	go func() {
		stdin.Write(ctxJSON)
		stdin.Close()
	}()

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("hook %s failed: %w", ctx.Event, err)
	}
	return output, nil
}
