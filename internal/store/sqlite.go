// Package store provides the SQLite-backed data layer for Codex Go.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var migration001 string

// InitDB opens (or creates) the SQLite database at the given path,
// runs all pending migrations, and seeds default data.
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("store: mkdir %s: %w", filepath.Dir(dbPath), err)
	}

	// Open with WAL mode + foreign keys
	dsn := dbPath + "?_journal_mode=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	// SQLite only supports a single writer
	db.SetMaxOpenConns(1)

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrations: %w", err)
	}

	// Seed defaults (idempotent)
	if err := seedDefaults(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: seed: %w", err)
	}

	return db, nil
}

// runMigrations applies SQL migrations in version order.
func runMigrations(db *sql.DB) error {
	// Ensure schema_version table exists (bootstrap)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}

	// Get current version
	var currentVer int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&currentVer); err != nil {
		// Table might be empty — that's fine
		currentVer = 0
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{1, migration001},
	}

	for _, m := range migrations {
		if m.version <= currentVer {
			continue
		}

		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %d: %w", m.version, err)
		}

		if _, err := db.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().Unix()); err != nil {
			return fmt.Errorf("migration %d record: %w", m.version, err)
		}

		fmt.Printf("[store] migration %d applied\n", m.version)
	}

	return nil
}

// seedDefaults inserts default tools, presets, and default agent when tables are empty.
func seedDefaults(db *sql.DB) error {
	now := time.Now().Unix()

	// ── System tools ──
	var toolCount int
	db.QueryRow(`SELECT COUNT(*) FROM tools`).Scan(&toolCount)
	if toolCount == 0 {
		tools := []struct {
			name, desc, category, risk string
			approval                   int
			sort                       int
		}{
			{"read_file", "Read file contents", "system", "low", 0, 1},
			{"write_file", "Create or overwrite a file", "system", "low", 0, 2},
			{"edit_file", "Find-and-replace edits in a file", "system", "medium", 0, 3},
			{"grep", "Search file contents with regex", "system", "low", 0, 4},
			{"ls", "List directory contents", "system", "low", 0, 5},
			{"shell", "Execute shell commands", "optional", "high", 1, 10},
			{"git", "Git version control operations", "optional", "medium", 0, 11},
			{"web_fetch", "Make HTTP requests", "optional", "medium", 0, 12},
			{"git_worktree", "Parallel Git worktrees", "optional", "medium", 0, 13},
			{"image_gen", "AI image generation", "optional", "low", 0, 14},
			{"browser", "Playwright browser automation", "optional", "high", 1, 15},
			{"code_review", "Code review with diff and lint", "optional", "low", 0, 16},
		}
		for _, t := range tools {
			db.Exec(`INSERT OR IGNORE INTO tools (name, description, category, risk, approval_required, sort_order) VALUES (?,?,?,?,?,?)`,
				t.name, t.desc, t.category, t.risk, t.approval, t.sort)
		}
	}

	// ── Provider presets ──
	var presetCount int
	db.QueryRow(`SELECT COUNT(*) FROM provider_presets`).Scan(&presetCount)
	if presetCount == 0 {
		presets := []struct {
			name, category, icon, iconColor, website, apiKeyURL string
			sort                                                int
		}{
			{"OpenAI", "official", "openai", "#10a37f", "https://platform.openai.com", "https://platform.openai.com/api-keys", 1},
			{"Anthropic", "official", "anthropic", "#d97757", "https://console.anthropic.com", "https://console.anthropic.com/settings/keys", 2},
			{"Google AI", "official", "google", "#4285f4", "https://aistudio.google.com", "https://aistudio.google.com/apikey", 3},
			{"DeepSeek", "official", "deepseek", "#4d6bfe", "https://platform.deepseek.com", "https://platform.deepseek.com/api_keys", 4},
			{"Groq", "third_party", "zap", "#f55036", "https://console.groq.com", "https://console.groq.com/keys", 5},
			{"Together AI", "third_party", "together", "#0f6fff", "https://api.together.ai", "https://api.together.ai/settings/api-keys", 6},
			{"Fireworks AI", "third_party", "fireworks", "#ff6b35", "https://fireworks.ai", "https://fireworks.ai/account/api-keys", 7},
			{"OpenRouter", "third_party", "openrouter", "#6366f1", "https://openrouter.ai", "https://openrouter.ai/keys", 8},
			{"Mistral AI", "official", "mistral", "#f90", "https://console.mistral.ai", "https://console.mistral.ai/api-keys", 9},
			{"Cohere", "official", "cohere", "#39594d", "https://dashboard.cohere.com", "https://dashboard.cohere.com/api-keys", 10},
			{"Perplexity", "official", "perplexity", "#1db5a8", "https://www.perplexity.ai", "https://www.perplexity.ai/settings/api", 11},
			{"xAI (Grok)", "official", "xai", "#000000", "https://x.ai", "https://console.x.ai", 12},
			{"Meta Llama", "official", "meta", "#0668e1", "https://llama.meta.com", "https://llama.meta.com", 13},
			{"BeeCode", "partner", "beecode", "#f5a623", "https://beecode.cc", "https://beecode.cc", 20},
			{"cc-switch", "partner", "ccswitch", "#6366f1", "https://github.com/farion1231/cc-switch", "https://github.com/farion1231/cc-switch", 21},
		}
		for _, p := range presets {
			db.Exec(`INSERT OR IGNORE INTO provider_presets (name, category, icon, icon_color, website_url, api_key_url, sort_order) VALUES (?,?,?,?,?,?,?)`,
				p.name, p.category, p.icon, p.iconColor, p.website, p.apiKeyURL, p.sort)
		}
	}

	// ── Default agent ──
	var agentCount int
	db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&agentCount)
	if agentCount == 0 {
		db.Exec(`INSERT OR IGNORE INTO agents (name, display_name, provider, model, system_prompt, max_turns, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			"default", "Default Agent", "", "",
			"You are Codex, an AI coding agent. Help the user write, debug, and understand code.",
			50, now, now)

		// Ensure default agent directory exists
		home, _ := os.UserHomeDir()
		agentDir := filepath.Join(home, ".codex", "agents", "default")
		os.MkdirAll(filepath.Join(agentDir, "rules"), 0755)
		os.MkdirAll(filepath.Join(agentDir, "skills"), 0755)
		soulPath := filepath.Join(agentDir, "soul.md")
		if _, err := os.Stat(soulPath); os.IsNotExist(err) {
			os.WriteFile(soulPath, []byte("# Default Agent\n\nA general-purpose AI coding assistant.\n"), 0644)
		}
	}

	return nil
}
