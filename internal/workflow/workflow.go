// Package workflow provides spec/plan/tasks workflow helpers shared between CLI and API.
package workflow

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// ---- Prompt Templates ----

// SpecPromptTemplate is the prompt for generating a SPEC document via LLM.
// First %s = feature description, second %s = output filename.
const SpecPromptTemplate = `You are writing a technical specification document. Generate a SPEC document based on this description:

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

// SpecPromptTemplateWorktree is the prompt for /spec when a git worktree has been
// created.  Args: desc, worktreePath, filename, branchName.
const SpecPromptTemplateWorktree = `You are writing a technical specification.

A dedicated git worktree has been created — work inside this directory:
  %s
Branch: %s

Feature to spec: %s
Output file: %s

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

IMPORTANT: Work ONLY inside the worktree directory. Do not modify files outside it.
Write the spec to the output file shown above.`

// SteerPromptTemplate is the unified /steer prompt — one command that drives
// the complete spec→plan→tasks→implement workflow.  %s = feature description.
const SteerPromptTemplate = `You are in guided /steer mode.  Execute the complete development workflow for this feature:

**%s**

## Instructions — follow these phases IN ORDER:

### Phase 1: SPEC
Write a detailed specification document.  Include:
- Background & motivation (why this feature)
- Goals (measurable, specific)
- Design (architecture, data structures, flow)
- Impact analysis (affected systems, migration)
- Implementation roadmap (phases)

Write the spec file following this naming: SPEC-<slug>.md where <slug> is a short
English slug derived from the feature.  Use the write_file tool.

### Phase 2: PLAN
Read the SPEC file you just created (use read_file).  Based on it, write PLAN.md
with:
- Phased implementation plan (Phase 1, Phase 2, …)
- Each phase contains checkbox tasks: "- [ ] Task N: <description> — 预计 <N>天"
- Acceptance criteria

Use the write_file tool to create PLAN.md.

### Phase 3: IMPLEMENT
Go through the tasks in PLAN.md from Phase 1 one by one.  For each task:
1. Read relevant files with read_file
2. Implement the changes with edit_file or write_file
3. Verify with shell commands (build, test)
4. Mark the task as done by changing "- [ ]" to "- [x]" in PLAN.md

IMPORTANT RULES:
- Complete all three phases in this single turn
- Use Chinese for spec/plan files if the feature description is in Chinese
- After each phase, briefly report what you completed
- If you encounter errors during implementation, fix them before moving on
- Only work on tasks in PLAN.md — do not add extra features`

// PlanPromptTemplate is the prompt for generating a PLAN.md from a spec via LLM.
// The %s is the spec filename to read.
const PlanPromptTemplate = `You are writing an implementation plan. Read the specification file %s (use read_file tool), then generate a detailed implementation plan.

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

// SpecFileTemplate is the template for creating a new spec file via CLI.
// The %s is the feature name for the title.
const SpecFileTemplate = `# %s

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

// ---- Task Types ----

// Task represents a single task from PLAN.md.
type Task struct {
	Number    int    `json:"number"`
	Content   string `json:"content"`
	Completed bool   `json:"completed"`
	Phase     string `json:"phase"`
}

// TaskList is the JSON response for GET /api/tasks.
type TaskList struct {
	Tasks []Task `json:"tasks"`
}

// ---- Plan Parsing ----

// ParseTasks reads a PLAN.md file and extracts all checkbox tasks.
func ParseTasks(filename string) (*TaskList, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	tasks := &TaskList{}
	taskNum := 0
	phase := ""

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Track phase headings
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			phase = line
			continue
		}

		if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
			taskNum++
			content := strings.TrimPrefix(strings.TrimPrefix(line, "- [x] "), "- [X] ")
			tasks.Tasks = append(tasks.Tasks, Task{
				Number:    taskNum,
				Content:   content,
				Completed: true,
				Phase:     phase,
			})
		} else if strings.HasPrefix(line, "- [ ]") {
			taskNum++
			content := strings.TrimPrefix(line, "- [ ] ")
			tasks.Tasks = append(tasks.Tasks, Task{
				Number:    taskNum,
				Content:   content,
				Completed: false,
				Phase:     phase,
			})
		}
	}

	return tasks, nil
}

// MarkTaskAsDone updates PLAN.md to mark a specific task number as completed.
// Returns the task content that was marked, or an error.
func MarkTaskAsDone(filename string, taskNum int) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	taskIdx := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
			taskIdx++
			if taskIdx == taskNum {
				if strings.Contains(line, "[x]") || strings.Contains(line, "[X]") {
					return "", fmt.Errorf("task %d is already completed", taskNum)
				}
				content := strings.TrimPrefix(trimmed, "- [ ] ")
				lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
				if err := os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644); err != nil {
					return "", err
				}
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("task %d not found (total: %d tasks)", taskNum, taskIdx)
}

// ---- Filename helpers ----

// Slugify converts a description to a filename-safe slug.
// Keeps letters, digits, underscores, hyphens; converts spaces/punctuation to hyphens.
func Slugify(s string) string {
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
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "feature"
	}
	return strings.ToLower(result)
}

// SpecFilename returns a SPEC-<slug>.md filename for a feature description.
func SpecFilename(desc string) string {
	return fmt.Sprintf("SPEC-%s.md", Slugify(desc))
}

// ---- Text rendering (for chat display) ----

// FormatTasksForChat formats a task list as human-readable text for the chat UI.
func FormatTasksForChat(tl *TaskList) string {
	if len(tl.Tasks) == 0 {
		return "No tasks found in PLAN.md."
	}

	var b strings.Builder
	lastPhase := ""
	for _, t := range tl.Tasks {
		if t.Phase != "" && t.Phase != lastPhase {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(t.Phase + "\n")
			lastPhase = t.Phase
		}
		icon := "⬜"
		if t.Completed {
			icon = "✅"
		}
		b.WriteString(fmt.Sprintf("  %d. %s %s\n", t.Number, icon, t.Content))
	}
	return b.String()
}
