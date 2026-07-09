package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ApprovalLevel controls what requires user confirmation.
type ApprovalLevel string

const (
	ApprovalNone   ApprovalLevel = "none"   // auto-approve all
	ApprovalSafe   ApprovalLevel = "safe"   // approve safe ops, ask for risky
	ApprovalAlways ApprovalLevel = "always" // ask for everything
)

var (
	DefaultLevel  = ApprovalSafe
	pendingMu     sync.Mutex
	pendingID     int
	pendingChecks = make(map[int]chan bool)
)

// Check is an approval request.
type Check struct {
	ID          int    `json:"id"`
	Tool        string `json:"tool"`
	Args        string `json:"args"`
	Risk        string `json:"risk"` // "safe", "warning", "danger"
	Description string `json:"description"`
}

// IsSafe determines if a tool call is safe to auto-approve.
func IsSafe(toolName, args string) bool {
	// Safe tools: read-only operations
	safe := map[string]bool{
		"read_file": true,
		"grep":      true,
		"ls":        true,
		"web_fetch": true,
	}
	if safe[toolName] {
		return true
	}

	// Git: safe subcommands only
	if toolName == "git" {
		if strings.Contains(args, `"command":"status"`) ||
			strings.Contains(args, `"command":"diff"`) ||
			strings.Contains(args, `"command":"log"`) ||
			strings.Contains(args, `"command":"branch"`) ||
			strings.Contains(args, `"command":"show"`) ||
			strings.Contains(args, `"command":"stash"`) {
			return true
		}
		return false
	}

	// Shell: commands that don't modify files
	if toolName == "shell" {
		riskCmds := []string{"rm ", "sudo ", "chmod ", "chown ", ">", "mv ", "dd ", "mkfs", "fdisk", "reboot", "shutdown"}
		for _, rc := range riskCmds {
			if strings.Contains(args, rc) {
				return false
			}
		}
		return strings.Contains(args, `"command":"`) && !strings.Contains(args, "|")
	}

	// Edit/write: always need approval
	return false
}

// RiskLevel returns the risk classification.
func RiskLevel(toolName, args string) string {
	if IsSafe(toolName, args) {
		return "safe"
	}
	if toolName == "shell" {
		if strings.Contains(args, "rm ") || strings.Contains(args, "sudo ") {
			return "danger"
		}
		return "warning"
	}
	return "warning"
}

// RequestApproval blocks until the user approves or rejects.
// This is called before executing a tool.
func RequestApproval(check Check) bool {
	if DefaultLevel == ApprovalNone {
		return true
	}
	if DefaultLevel == ApprovalSafe && check.Risk == "safe" {
		return true
	}

	pendingMu.Lock()
	pendingID++
	id := pendingID
	ch := make(chan bool, 1)
	pendingChecks[id] = ch
	pendingMu.Unlock()

	// TODO: send check to API/WebSocket for frontend notification
	fmt.Printf("\n[APPROVAL #%d] %s (%s): %s\n", id, check.Tool, check.Risk, check.Description)
	fmt.Printf("  Args: %s\n", check.Args)
	fmt.Print("Approve? [y/N]: ")

	var response string
	fmt.Scanln(&response)
	result := strings.ToLower(strings.TrimSpace(response)) == "y"

	pendingMu.Lock()
	delete(pendingChecks, id)
	pendingMu.Unlock()

	ch <- result
	return result
}

// SandboxConfig controls execution isolation.
type SandboxConfig struct {
	Enabled      bool     `yaml:"enabled"`
	ReadOnly     []string `yaml:"read_only"`     // paths accessible read-only
	ReadWrite    []string `yaml:"read_write"`    // paths accessible read-write
	AllowNetwork bool     `yaml:"allow_network"` // allow outbound network
}

// DefaultSandbox returns a safe default sandbox config.
func DefaultSandbox() SandboxConfig {
	home, _ := os.UserHomeDir()
	return SandboxConfig{
		Enabled:   false,
		ReadWrite: []string{home, "/tmp"},
		ReadOnly:  []string{"/usr", "/etc", "/opt"},
	}
}

// WrapCommand wraps a command in a sandbox if enabled.
// Uses bubblewrap on Linux if available.
func WrapCommand(cfg SandboxConfig, cmd string, args []string) *exec.Cmd {
	if !cfg.Enabled {
		return exec.Command(cmd, args...)
	}

	// Check if bubblewrap is available
	if _, err := exec.LookPath("bwrap"); err != nil {
		return exec.Command(cmd, args...)
	}

	// Build bubblewrap args
	bwrapArgs := []string{
		"--unshare-all",
		"--clearenv",
		"--new-session",
		"--die-with-parent",
	}

	// Set environment
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "PATH=") || strings.HasPrefix(env, "HOME=") ||
			strings.HasPrefix(env, "USER=") || strings.HasPrefix(env, "TERM=") ||
			strings.HasPrefix(env, "LANG=") {
			bwrapArgs = append(bwrapArgs, "--setenv", env[:strings.Index(env, "=")], env[strings.Index(env, "=")+1:])
		}
	}

	// Mount filesystems
	for _, p := range cfg.ReadWrite {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			bwrapArgs = append(bwrapArgs, "--bind", real, real)
		}
	}
	for _, p := range cfg.ReadOnly {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			bwrapArgs = append(bwrapArgs, "--ro-bind", real, real)
		}
	}

	// Temp
	bwrapArgs = append(bwrapArgs, "--tmpfs", "/tmp")
	bwrapArgs = append(bwrapArgs, "--tmpfs", "/var/tmp")

	// Proc
	bwrapArgs = append(bwrapArgs, "--proc", "/proc")

	// Network
	if !cfg.AllowNetwork {
		bwrapArgs = append(bwrapArgs, "--unshare-net")
	}

	// Working directory
	wd, _ := os.Getwd()
	bwrapArgs = append(bwrapArgs, "--chdir", wd)

	// Command to run
	bwrapArgs = append(bwrapArgs, "--")
	bwrapArgs = append(bwrapArgs, cmd)
	bwrapArgs = append(bwrapArgs, args...)

	return exec.Command("bwrap", bwrapArgs...)
}

// PendingChecks returns list of approval checks awaiting user action.
func PendingChecks() []Check {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	var checks []Check
	for id := range pendingChecks {
		// In real implementation, store check data
		checks = append(checks, Check{ID: id, Description: "pending"})
	}
	return checks
}
