// Package logger provides persistent file logging for the Codex agent.
// Logs are written to both stderr and ~/.codex/logs/codex.log with daily rotation.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	logFile  *os.File
	fileDate string // tracks the date of the current log file
	logDir   string
)

// Init initializes the logger to write to stderr + a daily log file under ~/.codex/logs.
// Call once at startup. Safe to call multiple times (subsequent calls are no-ops).
func Init(logsDir string) error {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		return nil // already initialized
	}

	logDir = logsDir
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("logger: mkdir %s: %w", logDir, err)
	}

	if err := rotate(); err != nil {
		return err
	}

	// Write to both stderr and the log file
	multi := io.MultiWriter(os.Stderr, logFile)
	log.SetOutput(multi)

	// Default: include file:line
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return nil
}

// rotate opens a new log file for today's date. Caller must hold mu.
func rotate() error {
	today := time.Now().Format("2006-01-02")
	if logFile != nil && fileDate == today {
		return nil
	}

	// Close previous file if any
	if logFile != nil {
		logFile.Close()
	}

	path := filepath.Join(logDir, "codex-"+today+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("logger: open %s: %w", path, err)
	}

	logFile = f
	fileDate = today

	// Also symlink the latest log
	latest := filepath.Join(logDir, "codex.log")
	os.Remove(latest)
	os.Symlink(path, latest)

	return nil
}

// RotateIfNeeded checks whether the date has changed and rotates the log file.
// Call periodically (e.g., once per minute) from a goroutine.
func RotateIfNeeded() {
	mu.Lock()
	defer mu.Unlock()
	_ = rotate()
}

// CleanOld removes log files older than keepDays from the log directory.
func CleanOld(keepDays int) error {
	mu.Lock()
	defer mu.Unlock()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Only clean files matching codex-YYYY-MM-DD.log pattern
		if len(name) < 16 || name[:6] != "codex-" || name[len(name)-4:] != ".log" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, name))
		}
	}
	return nil
}

// Printf logs to the configured output (stderr + file).
// Use this instead of log.Printf for consistent formatting.
func Printf(format string, v ...any) {
	log.Printf(format, v...)
}

// Fatalf logs and exits.
func Fatalf(format string, v ...any) {
	log.Fatalf(format, v...)
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
