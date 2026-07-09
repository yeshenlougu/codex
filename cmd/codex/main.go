package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/api"
	"github.com/yeshenlougu/codex/internal/ccswitch"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/logger"
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

	// Init persistent logging
	logsDir := filepath.Join(configDir(), "logs")
	if err := logger.Init(logsDir); err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
	}
	defer logger.Close()

	// Rotate log daily + clean old logs
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			logger.RotateIfNeeded()
		}
	}()
	logger.CleanOld(30) // keep 30 days

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

	// Subcommand: spec (new, show)
	if flag.NArg() >= 1 && flag.Arg(0) == "spec" {
		handleSpecCLI(flag.Args()[1:])
		return
	}

	// Subcommand: plan (generate, list)
	if flag.NArg() >= 1 && flag.Arg(0) == "plan" {
		handlePlanCLI(cfg, flag.Args()[1:])
		return
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

	// --serve (skip API key check — configurable via web UI)
	if *serve {
		serveCmd(cfg, store, skillsRegistry)
		return
	}

	// CLI mode: require API key
	if cfg.Provider.APIKey == "" && len(cfg.Provider.Backends) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No API key configured. Set OPENAI_API_KEY, use --api-key, or configure backends in config.yaml")
		os.Exit(1)
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
	fmt.Println("      /spec <desc>, /plan [spec], /tasks, /implement <n>")

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
		case strings.HasPrefix(input, "/spec"):
			handleSpecCommand(ag, input)
		case strings.HasPrefix(input, "/plan"):
			handlePlanCommand(ag, input)
		case input == "/tasks":
			handleTasksCommand()
		case strings.HasPrefix(input, "/implement"):
			handleImplementCommand(input)
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

// ---- spec / plan / tasks / implement handlers ----

const specPromptTemplate = `You are writing a technical specification document. Generate a SPEC document based on this description:

%s

Write the complete specification document to %s using the write_file tool.

The specification must follow this format in Chinese:

# <Feature Name>

## 1. 背景与动机
Why this feature is needed. What problem it solves.

## 2. 目标
Clear, measurable goals.

## 3. 设计方案
### 3.1 架构
High-level architecture. Data flow. Component diagram described in text.

### 3.2 数据结构
Key data structures, API shapes, config schemas. Use code blocks.

### 3.3 流程
Key workflows as step-by-step sequences.

## 4. 影响分析
What existing systems are affected. Migration path if any. Breaking changes.

## 5. 实施路线
### Phase 1: ...
### Phase 2: ...
### Phase 3: ...

Be thorough but concise. Every section should have substance, not placeholder text.`

const planPromptTemplate = `You are writing an implementation plan. Read the specification file %s (use read_file tool), then generate a detailed implementation plan.

Write the plan to PLAN.md using the write_file tool.

The plan must follow this format in Chinese:

# Implementation Plan for <Feature>

## Phase 1: <Phase Name>
- [ ] Task 1.1: <task description> — 预计 <N>天
- [ ] Task 1.2: <task description> — 预计 <N>天

## Phase 2: <Phase Name>
- [ ] Task 2.1: <task description> — 预计 <N>天
...

## 验收标准
- [ ] 标准 1
- [ ] 标准 2

Guidelines:
- Each phase should be independently shippable
- Tasks should be small enough to complete in 0.5-2 days
- Include testing, documentation, and code review as tasks where appropriate
- Acceptance criteria should be verifiable and concrete`

func handleSpecCommand(ag *agent.Agent, input string) {
	desc := strings.TrimSpace(strings.TrimPrefix(input, "/spec"))
	if desc == "" {
		fmt.Println("Usage: /spec <feature description>")
		fmt.Println("Example: /spec 添加暗色模式支持")
		return
	}

	filename := fmt.Sprintf("SPEC-%s.md", slugify(desc))
	prompt := fmt.Sprintf(specPromptTemplate, desc, filename)

	fmt.Printf("\n📝 Generating spec → %s ...\n\n", filename)
	_, err := ag.Run(prompt, func(chunk string) {
		fmt.Print(chunk)
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		fmt.Printf("\n✅ Spec saved → %s\n", filename)
	}
}

func handlePlanCommand(ag *agent.Agent, input string) {
	specFile := strings.TrimSpace(strings.TrimPrefix(input, "/plan"))
	if specFile == "" {
		specFile = "SPEC.md"
	}

	prompt := fmt.Sprintf(planPromptTemplate, specFile)

	fmt.Printf("\n📋 Generating plan from %s → PLAN.md ...\n\n", specFile)
	_, err := ag.Run(prompt, func(chunk string) {
		fmt.Print(chunk)
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		fmt.Println("\n✅ Plan saved → PLAN.md")
	}
}

func handleTasksCommand() {
	data, err := os.ReadFile("PLAN.md")
	if err != nil {
		fmt.Println("No PLAN.md found in current directory. Use /plan to generate one.")
		return
	}

	fmt.Println()
	taskNum := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### "):
			fmt.Printf("\n%s\n", trimmed)
		case strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]"):
			taskNum++
			desc := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- [x] "), "- [X] ")
			fmt.Printf("  %d. ✅ %s\n", taskNum, desc)
		case strings.HasPrefix(trimmed, "- [ ]"):
			taskNum++
			desc := strings.TrimPrefix(trimmed, "- [ ] ")
			fmt.Printf("  %d. ⬜ %s\n", taskNum, desc)
		}
	}
	fmt.Println()
}

func handleImplementCommand(input string) {
	taskID := strings.TrimSpace(strings.TrimPrefix(input, "/implement"))
	if taskID == "" {
		fmt.Println("Usage: /implement <task-number>")
		fmt.Println("Example: /implement 3")
		return
	}

	data, err := os.ReadFile("PLAN.md")
	if err != nil {
		fmt.Println("No PLAN.md found in current directory.")
		return
	}

	lines := strings.Split(string(data), "\n")
	taskIdx := 0
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			taskIdx++
			target := strconv.Itoa(taskIdx)
			if target == taskID {
				if strings.Contains(line, "[x]") || strings.Contains(line, "[X]") {
					fmt.Printf("Task %s is already completed.\n", taskID)
					return
				}
				lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
				found = true
				break
			}
		}
	}

	if !found {
		fmt.Printf("Task %s not found in PLAN.md (total: %d tasks)\n", taskID, taskIdx)
		return
	}

	if err := os.WriteFile("PLAN.md", []byte(strings.Join(lines, "\n")), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing PLAN.md: %v\n", err)
		return
	}
	fmt.Printf("✅ Task %s marked as completed in PLAN.md\n", taskID)
}

// slugify converts Chinese/English description to a filename-safe slug.
func slugify(s string) string {
	// Take first 30 chars, keep letters/digits, replace spaces with dash
	var b strings.Builder
	runes := []rune(s)
	maxLen := 30
	if len(runes) < maxLen {
		maxLen = len(runes)
	}
	for i := 0; i < maxLen; i++ {
		r := runes[i]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == ',' || r == '、' {
			b.WriteRune('-')
		}
		// Other characters (Chinese, etc.) are skipped
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "feature"
	}
	return strings.ToLower(result)
}

// ---- CLI subcommand handlers (spec / plan) ----

const specTemplate = `# %s

## 1. 背景与动机
<!-- 描述为什么需要这个功能，它解决什么问题 -->

## 2. 目标
<!-- 可衡量的目标 -->

## 3. 设计方案
### 3.1 架构
<!-- 高层架构、数据流、组件关系 -->

### 3.2 数据结构
<!-- 关键数据结构、API 形态、配置 schema -->

### 3.3 流程
<!-- 核心工作流的步骤序列 -->

## 4. 影响分析
<!-- 影响哪些现有系统、迁移路径、破坏性变更 -->

## 5. 实施路线
### Phase 1: <!-- 名称 -->
<!-- 描述 -->

### Phase 2: <!-- 名称 -->
<!-- 描述 -->
`

func handleSpecCLI(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: codex-go spec <new|show> [args...]")
		fmt.Fprintln(os.Stderr, "  spec new <name>     Create a new spec from template")
		fmt.Fprintln(os.Stderr, "  spec show [file]    Display a spec file (default: SPEC.md)")
		os.Exit(1)
	}

	switch args[0] {
	case "new":
		name := "feature"
		if len(args) > 1 {
			name = args[1]
		}
		filename := fmt.Sprintf("SPEC-%s.md", slugify(name))
		if _, err := os.Stat(filename); err == nil {
			fmt.Fprintf(os.Stderr, "Error: %s already exists.\n", filename)
			os.Exit(1)
		}
		content := fmt.Sprintf(specTemplate, name)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Created %s — fill in the sections and use /plan to generate a plan.\n", filename)

	case "show":
		file := "SPEC.md"
		if len(args) > 1 {
			file = args[1]
		}
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	default:
		fmt.Fprintf(os.Stderr, "Unknown spec subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: codex-go spec <new|show> [args...]")
		os.Exit(1)
	}
}

func handlePlanCLI(cfg *config.Config, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: codex-go plan <generate|list> [args...]")
		fmt.Fprintln(os.Stderr, "  plan generate [spec]  Generate PLAN.md from a spec (default: SPEC.md)")
		fmt.Fprintln(os.Stderr, "  plan list             List tasks from PLAN.md")
		os.Exit(1)
	}

	switch args[0] {
	case "generate":
		specFile := "SPEC.md"
		if len(args) > 1 {
			specFile = args[1]
		}
		if _, err := os.Stat(specFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s not found.\n", specFile)
			fmt.Fprintf(os.Stderr, "Run 'codex-go spec new <name>' to create one.\n")
			os.Exit(1)
		}
		if cfg.Provider.APIKey == "" && len(cfg.Provider.Backends) == 0 {
			fmt.Fprintln(os.Stderr, "Error: No API key configured. Plan generation requires an LLM backend.")
			os.Exit(1)
		}
		ag := agent.New(cfg)
		prompt := fmt.Sprintf(planPromptTemplate, specFile)
		fmt.Printf("📋 Generating plan from %s → PLAN.md ...\n", specFile)
		result, err := ag.Run(prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		_ = result
		fmt.Println("✅ Plan saved → PLAN.md")

	case "list":
		handleTasksCommand()

	default:
		fmt.Fprintf(os.Stderr, "Unknown plan subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: codex-go plan <generate|list> [args...]")
		os.Exit(1)
	}
}
