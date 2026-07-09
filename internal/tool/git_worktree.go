package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitWorktreeTool manages isolated git worktrees for parallel development.
type GitWorktreeTool struct{}

func NewGitWorktreeTool() *GitWorktreeTool { return &GitWorktreeTool{} }

func (g *GitWorktreeTool) Name() string { return "git_worktree" }

func (g *GitWorktreeTool) Description() string {
	return "Manage git worktrees for isolated parallel development. " +
		"Actions: add (create a new worktree from a branch), list (show all worktrees), " +
		"remove (delete a worktree and prune metadata), prune (clean up stale metadata)."
}

func (g *GitWorktreeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: add, list, remove, or prune",
				"enum":        []string{"add", "list", "remove", "prune"},
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch name to checkout (for add action)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path for the new worktree (for add/remove)",
			},
			"base_branch": map[string]any{
				"type":        "string",
				"description": "Base branch to create from (for add, defaults to current)",
			},
		},
		"required": []string{"action"},
	}
}

func (g *GitWorktreeTool) Execute(rawArgs string) (*Result, error) {
	var args struct {
		Action     string `json:"action"`
		Branch     string `json:"branch"`
		Path       string `json:"path"`
		BaseBranch string `json:"base_branch"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return &Result{Success: false, Error: "invalid args: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch args.Action {
	case "list":
		return runGitCmd(ctx, "worktree", "list")
	case "prune":
		return runGitCmd(ctx, "worktree", "prune")
	case "add":
		if args.Branch == "" {
			return &Result{Success: false, Error: "branch is required for 'add'"}, nil
		}
		wtPath := args.Path
		if wtPath == "" {
			wtPath = filepath.Join("..", args.Branch)
		}
		cmdArgs := []string{"worktree", "add"}
		if args.BaseBranch != "" {
			cmdArgs = append(cmdArgs, "-b", args.Branch, wtPath, args.BaseBranch)
		} else {
			cmdArgs = append(cmdArgs, "-b", args.Branch, wtPath)
		}
		out, err := exec.CommandContext(ctx, "git", cmdArgs...).CombinedOutput()
		outStr := string(out)
		if err != nil {
			return &Result{Success: false, Output: outStr, Error: err.Error()}, nil
		}
		return &Result{Success: true, Output: fmt.Sprintf("Worktree created at %s on branch %s\n%s", wtPath, args.Branch, strings.TrimSpace(outStr))}, nil
	case "remove":
		if args.Path == "" {
			return &Result{Success: false, Error: "path is required for 'remove'"}, nil
		}
		// First remove the worktree, then prune
		out1, err1 := exec.CommandContext(ctx, "git", "worktree", "remove", args.Path, "--force").CombinedOutput()
		out2, _ := exec.CommandContext(ctx, "git", "worktree", "prune").CombinedOutput()
		combined := strings.TrimSpace(string(out1) + "\n" + string(out2))
		if err1 != nil {
			return &Result{Success: false, Output: combined, Error: err1.Error()}, nil
		}
		// Remove the directory if it still exists
		os.RemoveAll(args.Path)
		return &Result{Success: true, Output: combined}, nil
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s (use add, list, remove, prune)", args.Action)}, nil
	}
}

func runGitCmd(ctx context.Context, args ...string) (*Result, error) {
	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	outStr := string(out)
	if err != nil {
		return &Result{Success: false, Output: outStr, Error: err.Error()}, nil
	}
	return &Result{Success: true, Output: strings.TrimSpace(outStr)}, nil
}
