package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WriteFileTool creates or overwrites files.
type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }
func (w *WriteFileTool) Name() string  { return "write_file" }
func (w *WriteFileTool) Description() string {
	return "Write content to a file, creating parent directories as needed."
}
func (w *WriteFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Path to file"},
			"content": map[string]any{"type": "string", "description": "Content to write"},
		},
		"required": []string{"path", "content"},
	}
}
func (w *WriteFileTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(rawArgs), &args)
	if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	return &Result{Success: true, Output: fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.Path)}, nil
}

// GrepTool searches file contents with regex.
type GrepTool struct{}

func NewGrepTool() *GrepTool     { return &GrepTool{} }
func (g *GrepTool) Name() string { return "grep" }
func (g *GrepTool) Description() string {
	return "Search file contents using regex. Returns matching lines with file paths and line numbers."
}
func (g *GrepTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Regex pattern to search for"},
			"path":    map[string]any{"type": "string", "description": "Directory or file (default: current dir)"},
			"glob":    map[string]any{"type": "string", "description": "File filter, e.g. '*.go'"},
		},
		"required": []string{"pattern"},
	}
}
func (g *GrepTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
	}
	json.Unmarshal([]byte(rawArgs), &args)
	if args.Path == "" {
		args.Path = "."
	}
	cmdArgs := []string{"-rn", "--color=never", "-I"}
	if args.Glob != "" {
		cmdArgs = append(cmdArgs, "--include="+args.Glob)
	}
	cmdArgs = append(cmdArgs, args.Pattern, args.Path)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "grep", cmdArgs...).CombinedOutput()
	outStr := string(out)
	if len(outStr) > 8000 {
		outStr = outStr[:8000] + fmt.Sprintf("\n... (%d total bytes)", len(out))
	}
	if err != nil {
		if len(outStr) > 0 {
			return &Result{Success: false, Output: outStr, Error: err.Error()}, nil
		}
		return &Result{Success: true, Output: "No matches found."}, nil
	}
	if strings.TrimSpace(outStr) == "" {
		return &Result{Success: true, Output: "No matches found."}, nil
	}
	return &Result{Success: true, Output: strings.TrimSpace(outStr)}, nil
}

// LsTool lists directory contents.
type LsTool struct{}

func NewLsTool() *LsTool              { return &LsTool{} }
func (l *LsTool) Name() string        { return "ls" }
func (l *LsTool) Description() string { return "List files and directories in a given path." }
func (l *LsTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Directory path (default: current dir)"},
		},
	}
}
func (l *LsTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(rawArgs), &args)
	if args.Path == "" {
		args.Path = "."
	}
	entries, err := os.ReadDir(args.Path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	var sb strings.Builder
	for _, e := range entries {
		prefix := "📄"
		if e.IsDir() {
			prefix = "📁"
		}
		info, _ := e.Info()
		size := ""
		if info != nil && !e.IsDir() {
			size = fmt.Sprintf("  %d bytes", info.Size())
		}
		sb.WriteString(fmt.Sprintf("%s %s%s\n", prefix, e.Name(), size))
	}
	if sb.Len() == 0 {
		return &Result{Success: true, Output: "(empty directory)"}, nil
	}
	return &Result{Success: true, Output: strings.TrimRight(sb.String(), "\n")}, nil
}

// GitTool runs safe git commands.
type GitTool struct{}

func NewGitTool() *GitTool      { return &GitTool{} }
func (g *GitTool) Name() string { return "git" }
func (g *GitTool) Description() string {
	return "Run a safe git command. Allowed: status, diff, log, branch, add, commit, checkout, stash, show."
}
func (g *GitTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "Git subcommand"},
			"args":    map[string]any{"type": "string", "description": "Additional arguments"},
		},
		"required": []string{"command"},
	}
}
func (g *GitTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Command string `json:"command"`
		Args    string `json:"args"`
	}
	json.Unmarshal([]byte(rawArgs), &args)
	safe := map[string]bool{
		"status": true, "diff": true, "log": true, "branch": true,
		"add": true, "commit": true, "checkout": true, "stash": true, "show": true,
	}
	if !safe[args.Command] {
		return &Result{Success: false, Error: fmt.Sprintf("git %s is not allowed", args.Command)}, nil
	}
	cmdArgs := []string{args.Command}
	if args.Args != "" {
		cmdArgs = append(cmdArgs, strings.Fields(args.Args)...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", cmdArgs...).CombinedOutput()
	outStr := string(out)
	if len(outStr) > 8000 {
		outStr = outStr[:8000] + "\n..."
	}
	if err != nil {
		return &Result{Success: false, Output: outStr, Error: err.Error()}, nil
	}
	return &Result{Success: true, Output: strings.TrimSpace(outStr)}, nil
}

// WebFetchTool fetches URL content as text.
type WebFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{client: &http.Client{Timeout: 15 * time.Second}}
}
func (w *WebFetchTool) Name() string { return "web_fetch" }
func (w *WebFetchTool) Description() string {
	return "Fetch text content from a URL. Returns plain text (HTML tags stripped)."
}
func (w *WebFetchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL to fetch"},
		},
		"required": []string{"url"},
	}
}
func (w *WebFetchTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		URL string `json:"url"`
	}
	json.Unmarshal([]byte(rawArgs), &args)
	resp, err := w.client.Get(args.URL)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &Result{Success: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 200*1024))
	text := stripHTML(string(body))
	if len(text) > 8000 {
		text = text[:8000] + "\n..."
	}
	return &Result{Success: true, Output: strings.TrimSpace(text)}, nil
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	lines := strings.Split(b.String(), "\n")
	var clean []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" {
			clean = append(clean, t)
		}
	}
	return strings.Join(clean, "\n")
}
