package tool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ShellTool executes shell commands.
type ShellTool struct {
	Timeout time.Duration
}

func NewShellTool() *ShellTool {
	return &ShellTool{Timeout: 120 * time.Second}
}

func (s *ShellTool) Name() string {
	return "shell"
}

func (s *ShellTool) Description() string {
	return "Execute a shell command in the terminal. Use for building, testing, git operations, " +
		"and exploring the filesystem. Returns stdout and stderr. " +
		"Long-running commands will be terminated after the timeout."
}

func (s *ShellTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory (optional, defaults to current)",
			},
		},
		"required": []string{"command"},
	}
}

func (s *ShellTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Command string `json:"command"`
		Workdir string `json:"workdir"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if args.Command == "" {
		return &Result{Success: false, Error: "command is required"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	if args.Workdir != "" {
		cmd.Dir = args.Workdir
	}

	output, err := cmd.CombinedOutput()

	// Trim output to reasonable size
	outStr := string(output)
	if len(outStr) > 10000 {
		outStr = outStr[:10000] + fmt.Sprintf("\n... (truncated, total %d bytes)", len(output))
	}

	if ctx.Err() == context.DeadlineExceeded {
		return &Result{
			Success: false,
			Output:  outStr,
			Error:   fmt.Sprintf("command timed out after %v", s.Timeout),
		}, nil
	}

	if err != nil {
		return &Result{
			Success: false,
			Output:  outStr,
			Error:   err.Error(),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  strings.TrimSpace(outStr),
	}, nil
}

// FileReadTool reads files from the filesystem.
type FileReadTool struct {
	MaxSize int64 // max bytes to read
}

func NewFileReadTool() *FileReadTool {
	return &FileReadTool{MaxSize: 100 * 1024} // 100KB default
}

func (f *FileReadTool) Name() string {
	return "read_file"
}

func (f *FileReadTool) Description() string {
	return "Read the contents of a file. Returns the file content with line numbers. " +
		"Use this before editing files to understand their current state."
}

func (f *FileReadTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-indexed, default: 1)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum lines to read (default: 200)",
			},
		},
		"required": []string{"path"},
	}
}

func (f *FileReadTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	if args.Offset < 1 {
		args.Offset = 1
	}
	if args.Limit < 1 {
		args.Limit = 200
	}

	// Path validation: disallow absolute paths outside the workspace
	// For now, just read the file directly
	content, err := readFileWithLineNumbers(args.Path, args.Offset, args.Limit, f.MaxSize)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  content,
	}, nil
}

// FileEditTool edits files with find-and-replace.
type FileEditTool struct{}

func NewFileEditTool() *FileEditTool {
	return &FileEditTool{}
}

func (f *FileEditTool) Name() string {
	return "edit_file"
}

func (f *FileEditTool) Description() string {
	return "Edit a file using find-and-replace. Provide the exact text to find and its replacement. " +
		"The tool will replace the first occurrence unless replace_all is true."
}

func (f *FileEditTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "Exact text to find and replace",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "Replacement text",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "Replace all occurrences (default: false)",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (f *FileEditTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	result, err := editFile(args.Path, args.OldString, args.NewString, args.ReplaceAll)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  result,
	}, nil
}
