package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/api"
	"github.com/yeshenlougu/codex/internal/ccswitch"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/session"
	"github.com/yeshenlougu/codex/internal/skill"
)

// Build info (injected by Makefile via -ldflags)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

var (
	configPath   = flag.String("config", "", "Path to config file")
	model        = flag.String("model", "", "Override model name")
	providerName = flag.String("provider", "", "Override provider")
	apiKey       = flag.String("api-key", "", "Override API key")
	baseURL      = flag.String("base-url", "", "Override base URL")
	prompt       = flag.String("prompt", "", "Single prompt mode")
	systemPrompt = flag.String("system-prompt", "", "Override system prompt")
	resume       = flag.String("resume", "", "Resume a session by ID")
	listSessions = flag.Bool("list", false, "List saved sessions")
	deleteID     = flag.String("delete", "", "Delete a session by ID")
	showVersion  = flag.Bool("version", false, "Show version info")
	serve        = flag.Bool("serve", false, "Start HTTP/WebSocket API server")
	serveAddr    = flag.String("addr", ":1977", "API server listen address")
	skillsDir    = flag.String("skills-dir", "", "Additional skills directory")
)

func main() {
	flag.Parse()

	// --version
	if *showVersion {
		fmt.Printf("Codex Go %s\n", version)
		fmt.Printf("  build: %s\n", buildTime)
		fmt.Printf("  commit: %s\n", gitCommit)
		return
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	applyOverrides(cfg)

	// Subcommand: provider import/export (cc-switch migration)
	if flag.NArg() >= 2 && flag.Arg(0) == "provider" {
		handleProviderCmd(cfg, flag.Arg(1), flag.Args()[2:])
		return
	}

	if cfg.Provider.APIKey == "" && len(cfg.Provider.Backends) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No API key configured. Set OPENAI_API_KEY, use --api-key, or configure backends in config.yaml")
		os.Exit(1)
	}

	// Session store
	sessionDir := filepath.Join(configDir(), "sessions")
	store, err := session.NewStore(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Skills registry
	skillsRegistry := loadSkills()

	// --list
	if *listSessions {
		listSessionsCmd(store)
		return
	}

	// --delete
	if *deleteID != "" {
		if err := store.Delete(*deleteID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Session %s deleted.\n", *deleteID)
		return
	}

	// --serve
	if *serve {
		serveCmd(cfg, store, skillsRegistry)
		return
	}

	// Create agent
	ag := agent.New(cfg).WithStore(store).WithSkills(skillsRegistry)
	printBanner(cfg, skillsRegistry)

	// --resume
	if *resume != "" {
		if err := ag.LoadSession(*resume); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Resumed session %s (%d messages)\n\n", *resume, len(ag.Messages()))
		runInteractive(ag)
		return
	}

	// --prompt
	if *prompt != "" {
		ag.SetSessionID(agent.NewSessionID())
		runOnce(ag, *prompt)
		return
	}

	// Piped stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		var input strings.Builder
		for scanner.Scan() {
			input.WriteString(scanner.Text() + "\n")
		}
		if input.Len() > 0 {
			ag.SetSessionID(agent.NewSessionID())
			runOnce(ag, strings.TrimSpace(input.String()))
		}
		return
	}

	// Interactive
	ag.SetSessionID(agent.NewSessionID())
	runInteractive(ag)
}

func loadSkills() *skill.Registry {
	r := skill.NewRegistry()

	// Default skills directories
	home, _ := os.UserHomeDir()
	defaults := []string{
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}

	for _, d := range defaults {
		r.AddDir(d)
	}

	if *skillsDir != "" {
		for _, d := range strings.Split(*skillsDir, ",") {
			r.AddDir(strings.TrimSpace(d))
		}
	}

	if err := r.LoadAll(); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] skill loading: %v\n", err)
	}

	return r
}

func serveCmd(cfg *config.Config, store *session.Store, skillsRegistry *skill.Registry) {
	srv := api.New(cfg, store, *serveAddr)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		os.Exit(0)
	}()

	fmt.Printf("Codex API Server · %s/%s\n", cfg.Model.Provider, cfg.Model.Model)
	fmt.Printf("Listening on %s\n", *serveAddr)
	fmt.Printf("WebSocket: ws://localhost%s/ws\n", *serveAddr)
	fmt.Printf("Skills: %d loaded\n", len(skillsRegistry.All()))

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func applyOverrides(cfg *config.Config) {
	if *model != "" {
		cfg.Model.Model = *model
	}
	if *providerName != "" {
		cfg.Model.Provider = *providerName
	}
	if *apiKey != "" {
		cfg.Provider.APIKey = *apiKey
	}
	if *baseURL != "" {
		cfg.Provider.BaseURL = *baseURL
	}
	if *systemPrompt != "" {
		cfg.Agent.SystemPrompt = *systemPrompt
	}
}

func printBanner(cfg *config.Config, skillsReg *skill.Registry) {
	toolNames := []string{"shell", "read_file", "edit_file", "write_file", "grep", "ls", "git", "web_fetch"}
	skillCount := len(skillsReg.All())
	fmt.Printf("Codex Go · %s/%s · %d tools · %d skills\n",
		cfg.Model.Provider, cfg.Model.Model, len(toolNames), skillCount)
}

func listSessionsCmd(store *session.Store) {
	summaries, err := store.List(50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(summaries) == 0 {
		fmt.Println("No saved sessions.")
		return
	}
	for _, s := range summaries {
		ago := time.Since(s.UpdatedAt).Round(time.Minute)
		fmt.Printf("  %s  %s/%s  %d msgs  %s  %s\n",
			s.ID, s.Provider, s.Model, s.MsgCount, ago, s.Title)
	}
}

func handleProviderCmd(cfg *config.Config, action string, args []string) {
	switch action {
	case "import":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: codex-go provider import <cc-switch-config.yaml|json>")
			os.Exit(1)
		}
		path := args[0]
		backends, strategy, err := ccswitch.ImportFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
			os.Exit(1)
		}
		ccswitch.MergeIntoConfig(cfg, backends, strategy)

		// Save merged config
		configPath := configFilePath()
		if err := saveConfig(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Save config failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Imported %d backends from %s\n", len(backends), path)
		fmt.Printf("Strategy: %s\n", strategy)
		fmt.Printf("Config saved: %s\n", configPath)
		for _, be := range backends {
			keyPreview := be.Key
			if len(keyPreview) > 12 {
				keyPreview = keyPreview[:12] + "..."
			}
			fmt.Printf("  - %s (%s, weight=%d)\n", be.Label, keyPreview, be.Weight)
		}

	case "export":
		outPath := "cc-switch.yaml"
		if len(args) > 0 {
			outPath = args[0]
		}
		backends := cfg.Provider.Backends
		if len(backends) == 0 && cfg.Provider.APIKey != "" {
			backends = append(backends, config.BackendConfig{
				Label: "default", Key: cfg.Provider.APIKey,
				BaseURL: cfg.Provider.BaseURL, Weight: 1,
			})
		}
		if err := ccswitch.ExportToCCSwitch(backends, cfg.Provider.PoolStrategy, outPath); err != nil {
			fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %d backends to %s\n", len(backends), outPath)

	case "status":
		fmt.Printf("Pool strategy: %s\n", cfg.Provider.PoolStrategy)
		fmt.Printf("Backends: %d configured\n", len(cfg.Provider.Backends))
		for _, be := range cfg.Provider.Backends {
			keyPreview := be.Key
			if len(keyPreview) > 12 {
				keyPreview = keyPreview[:12] + "..."
			}
			fmt.Printf("  - %s → %s (weight=%d, key=%s)\n", be.Label, be.BaseURL, be.Weight, keyPreview)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown provider subcommand: %s\n", action)
		fmt.Fprintln(os.Stderr, "Usage: codex-go provider <import|export|status> [args...]")
		os.Exit(1)
	}
}

func configFilePath() string {
	if *configPath != "" {
		return *configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.yaml")
}

func saveConfig(cfg *config.Config, path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func runOnce(ag *agent.Agent, promptText string) {
	fmt.Printf("\n> %s\n\n", promptText)
	result, err := ag.Run(promptText, func(chunk string) {
		fmt.Print(chunk)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	_ = result
}

func runInteractive(ag *agent.Agent) {
	sid := ag.SessionID()
	if sid != "" {
		fmt.Printf("Session: %s\n", sid)
	}
	fmt.Println("Type /exit, /history, /clear, /save, /sessions")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "/exit" || input == "/quit":
			fmt.Println("Goodbye!")
			return
		case input == "/history":
			fmt.Println()
			for i, msg := range ag.Messages() {
				if i == 0 && msg.Role == "system" {
					continue
				}
				content := msg.Content
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				fmt.Printf("[%s] %s\n", msg.Role, content)
			}
			fmt.Println()
		case input == "/clear":
			ag = agent.New(ag.Config())
			fmt.Println("Conversation cleared.")
		case input == "/save":
			fmt.Printf("Session %s auto-saved.\n", ag.SessionID())
		case input == "/sessions":
			if ag.Config() == nil {
				continue
			}
			home, _ := os.UserHomeDir()
			store, _ := session.NewStore(filepath.Join(home, ".codex", "sessions"))
			if store != nil {
				listSessionsCmd(store)
			}
		default:
			fmt.Println()
			_, err := ag.Run(input, func(chunk string) {
				fmt.Print(chunk)
			})
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}
