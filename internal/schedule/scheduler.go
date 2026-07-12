// Package schedule provides cron-based task scheduling for the Codex agent.
// Schedules are persisted as JSON files and executed by a background goroutine
// that checks for due tasks every minute.
package schedule

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Task is a scheduled agent task.
type Task struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"` // what to send to the agent
	CronExpr    string    `json:"cron_expr"`
	Category    string    `json:"category"` // daily, weekly, monitor
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	NextRun     time.Time `json:"next_run"`
	LastRun     time.Time `json:"last_run,omitempty"`
	LastResult  string    `json:"last_result,omitempty"`
}

// Engine runs scheduled tasks.
type Engine struct {
	dir    string
	mu     sync.RWMutex
	tasks  map[string]*Task
	stopCh chan struct{}

	// OnTrigger is called when a task fires. The caller wires it to the agent.
	OnTrigger func(task Task)
}

// NewEngine creates a schedule engine.
func NewEngine(dir string) (*Engine, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create schedule dir: %w", err)
	}
	e := &Engine{
		dir:    dir,
		tasks:  make(map[string]*Task),
		stopCh: make(chan struct{}),
	}
	if err := e.loadAll(); err != nil {
		log.Printf("[schedule] load: %v", err)
	}
	return e, nil
}

// Start begins the scheduling loop.
func (e *Engine) Start() {
	go e.loop()
	log.Printf("[schedule] engine started — %d tasks loaded", len(e.tasks))
}

// Stop shuts down the scheduler.
func (e *Engine) Stop() {
	close(e.stopCh)
}

// Create adds a new scheduled task and persists it.
func (e *Engine) Create(task Task) (*Task, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("sch-%d", time.Now().UnixNano())
	}
	if _, exists := e.tasks[task.ID]; exists {
		return nil, fmt.Errorf("task %s already exists", task.ID)
	}
	task.CreatedAt = time.Now()
	task.UpdatedAt = task.CreatedAt
	task.Enabled = true
	task.NextRun = e.nextRunTime(task.CronExpr)
	e.tasks[task.ID] = &task
	if err := e.save(&task); err != nil {
		delete(e.tasks, task.ID)
		return nil, err
	}
	return &task, nil
}

// List returns all tasks sorted by next run time.
func (e *Engine) List() []Task {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]Task, 0, len(e.tasks))
	for _, t := range e.tasks {
		tasks = append(tasks, *t)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].NextRun.Before(tasks[j].NextRun)
	})
	return tasks
}

// Delete removes a task.
func (e *Engine) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.tasks[id]; !ok {
		return fmt.Errorf("task %s not found", id)
	}
	delete(e.tasks, id)
	return os.Remove(e.path(id))
}

// Toggle enables or disables a task.
func (e *Engine) Toggle(id string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t, ok := e.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	t.Enabled = enabled
	t.UpdatedAt = time.Now()
	return e.save(t)
}

// UpdateLastRun records a completed execution.
func (e *Engine) UpdateLastRun(id string, result string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.tasks[id]; ok {
		t.LastRun = time.Now()
		t.LastResult = truncate(result, 200)
		t.NextRun = e.nextRunTime(t.CronExpr)
		t.UpdatedAt = time.Now()
		e.save(t)
	}
}

// ---- internal ----

func (e *Engine) path(id string) string {
	return filepath.Join(e.dir, id+".json")
}

func (e *Engine) save(t *Task) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.path(t.ID), data, 0600)
}

func (e *Engine) loadAll() error {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(e.dir, entry.Name()))
		if err != nil {
			continue
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		e.tasks[t.ID] = &t
	}
	return nil
}

// nextRunTime computes the next run from a simple cron expression.
// Supports: "daily 08:00", "weekdays 09:00", "weekly fri 16:00", "every Nm"
func (e *Engine) nextRunTime(expr string) time.Time {
	now := time.Now()
	expr = strings.TrimSpace(strings.ToLower(expr))

	// "every 30m" or "every 1h"
	if strings.HasPrefix(expr, "every ") {
		dur, err := parseDuration(expr[6:])
		if err == nil {
			return now.Add(dur)
		}
	}

	// "daily 08:00", "daily"
	if strings.HasPrefix(expr, "daily") {
		parts := strings.Fields(expr)
		h, m := 9, 0
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d:%d", &h, &m)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}

	// "weekdays 08:00"
	if strings.HasPrefix(expr, "weekdays") {
		parts := strings.Fields(expr)
		h, m := 9, 0
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d:%d", &h, &m)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		for next.Before(now) || next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}

	// "weekly fri 16:00"
	if strings.HasPrefix(expr, "weekly") {
		parts := strings.Fields(expr)
		h, m, day := 9, 0, time.Monday
		if len(parts) >= 2 {
			day = parseWeekday(parts[1])
		}
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d:%d", &h, &m)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		for next.Before(now) || next.Weekday() != day {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}

	// Default: tomorrow 9am
	return time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
}

func (e *Engine) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkAndFire()
		}
	}
}

func (e *Engine) checkAndFire() {
	e.mu.Lock()
	now := time.Now()
	var due []Task
	for _, t := range e.tasks {
		if t.Enabled && !t.NextRun.IsZero() && now.After(t.NextRun) {
			due = append(due, *t)
			// Advance next run so it doesn't fire twice
			t.NextRun = e.nextRunTime(t.CronExpr)
			e.save(t)
		}
	}
	e.mu.Unlock()

	for _, task := range due {
		log.Printf("[schedule] firing task: %s (%s)", task.Name, task.ID)
		if e.OnTrigger != nil {
			e.OnTrigger(task)
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		var mins int
		fmt.Sscanf(s, "%dm", &mins)
		return time.Duration(mins) * time.Minute, nil
	}
	if strings.HasSuffix(s, "h") {
		var hrs int
		fmt.Sscanf(s, "%dh", &hrs)
		return time.Duration(hrs) * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown duration: %s", s)
}

func parseWeekday(s string) time.Weekday {
	switch strings.ToLower(s[:3]) {
	case "mon":
		return time.Monday
	case "tue":
		return time.Tuesday
	case "wed":
		return time.Wednesday
	case "thu":
		return time.Thursday
	case "fri":
		return time.Friday
	case "sat":
		return time.Saturday
	case "sun":
		return time.Sunday
	}
	return time.Monday
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
