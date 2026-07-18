package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/yeshenlougu/codex/internal/mcp"
	"github.com/yeshenlougu/codex/internal/store"
)

// MCPServerItem is the frontend-facing MCP server structure.
type MCPServerItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Enabled     bool              `json:"enabled"`
	Status      string            `json:"status"`
	Error       string            `json:"error,omitempty"`
	ToolCount   int               `json:"tool_count"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// toItem converts a store definition to an API item.
func toItem(def *store.MCPServerDef) *MCPServerItem {
	return &MCPServerItem{
		ID:          def.ID,
		Name:        def.Name,
		Description: def.Description,
		Command:     def.Command,
		Args:        def.Args,
		Env:         def.Env,
		Enabled:     def.Enabled,
		Status:      def.Status,
		Error:       def.Error,
		ToolCount:   def.ToolCount,
		CreatedAt:   def.CreatedAt,
		UpdatedAt:   def.UpdatedAt,
	}
}

// handleMCPServers dispatches /api/mcp/servers and /api/mcp/servers/{id}.
func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/mcp")
	path = strings.TrimPrefix(path, "/")

	// /api/mcp/servers
	if path == "servers" {
		switch r.Method {
		case http.MethodGet:
			s.listMCPServers(w, r)
		case http.MethodPost:
			s.createMCPServer(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "not allowed")
		}
		return
	}

	// /api/mcp/servers/{id}
	if strings.HasPrefix(path, "servers/") {
		id := strings.TrimPrefix(path, "servers/")
		// Handle sub-actions: /api/mcp/servers/{id}/restart
		if strings.HasSuffix(id, "/restart") {
			id = strings.TrimSuffix(id, "/restart")
			if r.Method == http.MethodPost {
				s.restartMCPServer(w, r, id)
				return
			}
		}
		s.handleMCPServerByID(w, r, id)
		return
	}

	// /api/mcp/presets
	if path == "presets" && r.Method == http.MethodGet {
		s.getMCPPresets(w, r)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// listMCPServers returns all MCP server definitions (SQLite + JSON).
func (s *Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	allMap := s.mcpStore.All()
	all := make([]*store.MCPServerDef, 0, len(allMap)+10)
	for _, def := range allMap {
		all = append(all, def)
	}

	// Merge SQLite MCP servers into the list
	if s.store != nil {
		dbServers, err := s.store.ListAllMCPServers()
		if err == nil {
			jsonNames := make(map[string]bool)
			for _, def := range all {
				jsonNames[def.Name] = true
			}
			for _, db := range dbServers {
				if jsonNames[db.Name] {
					continue
				}
				all = append(all, &store.MCPServerDef{
					Name:        db.Name,
					Description: db.Description,
					Command:     db.Command,
					Args:        store.ParseJSONList(db.Args),
					Env:         map[string]string{},
					Enabled:     db.Enabled,
				})
			}
		}
	}

	items := make([]*MCPServerItem, 0, len(all))
	for _, def := range all {
		items = append(items, toItem(def))
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": items})
}

// createMCPServer adds a new MCP server and starts it if enabled.
func (s *Server) createMCPServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Command     string            `json:"command"`
		Args        []string          `json:"args"`
		Env         map[string]string `json:"env"`
		Enabled     *bool             `json:"enabled"` // default true
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if input.Name == "" || input.Command == "" {
		writeError(w, http.StatusBadRequest, "name and command required")
		return
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	def := &store.MCPServerDef{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Description: input.Description,
		Command:     input.Command,
		Args:        input.Args,
		Env:         input.Env,
		Enabled:     enabled,
		Status:      "disconnected",
	}

	if err := s.mcpStore.Add(def); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Write-through to SQLite
	if s.store != nil {
		argsJSON, _ := json.Marshal(input.Args)
		envJSON, _ := json.Marshal(input.Env)
		if err := s.store.CreateMCPServer(input.Name, input.Description, input.Command, string(argsJSON), string(envJSON)); err != nil {
			log.Printf("[api] MCP SQLite write: %v", err)
		}
	}

	// Start the client if enabled
	if enabled {
		s.startMCPClient(def)
	}

	item := toItem(def)
	writeJSON(w, http.StatusCreated, item)
}

// handleMCPServerByID handles GET/PUT/DELETE for a single MCP server.
func (s *Server) handleMCPServerByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		def, ok := s.mcpStore.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "mcp server not found")
			return
		}
		writeJSON(w, http.StatusOK, toItem(def))

	case http.MethodPut:
		var input struct {
			Name        *string            `json:"name"`
			Description *string            `json:"description"`
			Command     *string            `json:"command"`
			Args        []string           `json:"args"`
			Env         map[string]string  `json:"env"`
			Enabled     *bool              `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		err := s.mcpStore.Update(id, func(def *store.MCPServerDef) error {
			if input.Name != nil {
				def.Name = *input.Name
			}
			if input.Description != nil {
				def.Description = *input.Description
			}
			if input.Command != nil {
				def.Command = *input.Command
			}
			if input.Args != nil {
				def.Args = input.Args
			}
			if input.Env != nil {
				def.Env = input.Env
			}
			if input.Enabled != nil {
				def.Enabled = *input.Enabled
			}
			return nil
		})
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		// Write-through to SQLite
		def, _ := s.mcpStore.Get(id)
		if s.store != nil && def != nil {
			argsJSON, _ := json.Marshal(def.Args)
			envJSON, _ := json.Marshal(def.Env)
			s.store.UpdateMCPServer(def.Name, def.Description, def.Command, string(argsJSON), string(envJSON), def.Enabled)
		}

		// Restart client with new config
		s.stopMCPClient(id)
		def, _ = s.mcpStore.Get(id)
		if def != nil && def.Enabled {
			s.startMCPClient(def)
		}

		def, _ = s.mcpStore.Get(id)
		writeJSON(w, http.StatusOK, toItem(def))

	case http.MethodDelete:
		s.stopMCPClient(id)
		// Look up the name before removal (for SQLite write-through)
		def, _ := s.mcpStore.Get(id)
		if err := s.mcpStore.Remove(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// Write-through delete to SQLite
		if s.store != nil && def != nil {
			if err := s.store.DeleteMCPServer(def.Name); err != nil {
				log.Printf("[api] MCP SQLite delete: %v", err)
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": id})

	default:
		writeError(w, http.StatusMethodNotAllowed, "not allowed")
	}
}

// restartMCPServer stops and restarts an MCP server.
func (s *Server) restartMCPServer(w http.ResponseWriter, r *http.Request, id string) {
	s.stopMCPClient(id)
	def, ok := s.mcpStore.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "mcp server not found")
		return
	}
	if def.Enabled {
		s.startMCPClient(def)
	}
	def, _ = s.mcpStore.Get(id)
	writeJSON(w, http.StatusOK, toItem(def))
}

// startMCPClient creates and registers an MCP client.
func (s *Server) startMCPClient(def *store.MCPServerDef) {
	s.mcpMu.Lock()
	defer s.mcpMu.Unlock()

	// Skip if already running
	if _, exists := s.mcpClients[def.ID]; exists {
		return
	}

	client, err := mcp.NewMCPClient(def.Command, def.Args...)
	if err != nil {
		log.Printf("[api] MCP server %s (%s) start error: %v", def.Name, def.ID, err)
		s.mcpStore.Update(def.ID, func(d *store.MCPServerDef) error {
			d.Status = "error"
			d.Error = err.Error()
			return nil
		})
		return
	}

	s.mcpClients[def.ID] = client

	// Register tools into the shared MCP registry
	for _, t := range client.Tools {
		wrapped := mcp.NewToolWrapper(client, t)
		s.mcpRegistry.Register(wrapped)
	}

	s.mcpStore.Update(def.ID, func(d *store.MCPServerDef) error {
		d.Status = "connected"
		d.ToolCount = len(client.Tools)
		d.Error = ""
		return nil
	})
	log.Printf("[api] MCP server connected: %s (%d tools)", def.Name, len(client.Tools))
}

// stopMCPClient stops and unregisters an MCP client.
func (s *Server) stopMCPClient(id string) {
	s.mcpMu.Lock()
	defer s.mcpMu.Unlock()

	client, exists := s.mcpClients[id]
	if !exists {
		return
	}

	// Unregister tools
	for _, t := range client.Tools {
		s.mcpRegistry.Unregister(t.Name)
	}

	client.Close()
	delete(s.mcpClients, id)

	s.mcpStore.Update(id, func(d *store.MCPServerDef) error {
		d.Status = "disconnected"
		d.Error = ""
		d.ToolCount = 0
		return nil
	})
	log.Printf("[api] MCP server stopped: %s", id)
}

// getMCPPresets returns the built-in MCP preset templates.
func (s *Server) getMCPPresets(w http.ResponseWriter, r *http.Request) {
	presets := []map[string]any{
		{
			"name":        "Filesystem",
			"description": "安全的文件系统操作（读写、搜索、编辑）",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"},
			"env":         map[string]string{},
		},
		{
			"name":        "GitHub",
			"description": "GitHub API 集成（仓库、Issues、PR 管理）",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-github"},
			"env":         map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "<your-token>"},
		},
		{
			"name":        "PostgreSQL",
			"description": "PostgreSQL 数据库查询（只读 Schema 检查）",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"},
			"env":         map[string]string{},
		},
		{
			"name":        "Brave Search",
			"description": "Brave Search API 网页和本地搜索",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-brave-search"},
			"env":         map[string]string{"BRAVE_API_KEY": "<your-api-key>"},
		},
		{
			"name":        "Memory",
			"description": "基于知识图谱的持久记忆系统",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-memory"},
			"env":         map[string]string{},
		},
		{
			"name":        "Puppeteer",
			"description": "浏览器自动化（截图、点击、表单填写）",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-puppeteer"},
			"env":         map[string]string{},
		},
		{
			"name":        "Fetch",
			"description": "网页内容获取和转换为 Markdown",
			"command":     "uvx",
			"args":        []string{"mcp-server-fetch"},
			"env":         map[string]string{},
		},
		{
			"name":        "Sequential Thinking",
			"description": "动态和反思性问题解决的思维工具",
			"command":     "npx",
			"args":        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
			"env":         map[string]string{},
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": presets})
}
