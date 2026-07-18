package agent

import (
	"log"

	"github.com/yeshenlougu/codex/internal/tool"
)

// ToolDataStore is the interface for loading tool configuration from persistent storage.
type ToolDataStore interface {
	ListTools(category string) ([]ToolConfig, error)
}

// ToolConfig is a portable tool configuration (decoupled from store package).
type ToolConfig struct {
	Name             string
	Description      string
	Category         string // "system" | "optional"
	Risk             string
	ApprovalRequired bool
	Enabled          bool
	SortOrder        int
}

// AssembleTools creates a tool Registry from the data store based on the agent's
// tools_mode and tools_list, per SPEC §4.3:
//
//	mode "all"    → load all enabled tools
//	mode "custom" → load only tools in tools_list
//	mode "none"   → load no optional tools
//
// System tools (category = "system") are ALWAYS loaded regardless of mode.
func AssembleTools(ds ToolDataStore, mode string, toolsList []string) *tool.Registry {
	reg := tool.NewRegistry()

	// Always register built-in tools first (these are the implementation layer)
	registerBuiltinTools(reg)

	// If no data store, return built-ins only
	if ds == nil {
		return reg
	}

	dbTools, err := ds.ListTools("")
	if err != nil {
		log.Printf("[agent] failed to load tools from store: %v — using built-ins only", err)
		return reg
	}

	// Build lookup maps
	enabledMap := make(map[string]bool)
	categoryMap := make(map[string]string)
	for _, t := range dbTools {
		enabledMap[t.Name] = t.Enabled
		categoryMap[t.Name] = t.Category
	}

	if mode == "none" {
		// Only system tools — already registered via registerBuiltinTools
		log.Printf("[agent] tools_mode=none — using system tools only")
		return reg
	}

	if mode == "custom" && len(toolsList) > 0 {
		// Custom mode: enable only listed tools, but keep system tools always
		customSet := make(map[string]bool)
		for _, name := range toolsList {
			customSet[name] = true
		}
		// Disable non-system, non-listed tools
		for name := range enabledMap {
			if categoryMap[name] == "system" {
				continue // always enabled
			}
			if !customSet[name] {
				reg.Unregister(name)
			}
		}
		log.Printf("[agent] tools_mode=custom list=%v — %d tools active", toolsList, len(reg.AllTools()))
		return reg
	}

	// "all" mode: disable tools that are explicitly disabled in the DB
	for name, enabled := range enabledMap {
		if categoryMap[name] == "system" {
			continue // system tools always enabled
		}
		if !enabled {
			reg.Unregister(name)
		}
	}

	log.Printf("[agent] tools_mode=%s — %d tools active (from %d DB rows)", mode, len(reg.AllTools()), len(dbTools))
	return reg
}

// registerBuiltinTools registers all available built-in tool implementations.
func registerBuiltinTools(reg *tool.Registry) {
	reg.Register(tool.NewShellTool())
	reg.Register(tool.NewFileReadTool())
	reg.Register(tool.NewFileEditTool())
	reg.Register(tool.NewWriteFileTool())
	reg.Register(tool.NewGrepTool())
	reg.Register(tool.NewLsTool())
	reg.Register(tool.NewGitTool())
	reg.Register(tool.NewWebFetchTool())
	reg.Register(tool.NewGitWorktreeTool())
	reg.Register(tool.NewImageGenTool())
}
