package api

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry is one file or directory item.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// handleListFiles returns directory listing for the file browser.
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "."
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var files []FileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(absPath, e.Name())
		files = append(files, FileEntry{
			Name:    e.Name(),
			Path:    fullPath,
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	// Sort: dirs first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Add parent directory entry if not at root
	if absPath != "/" && absPath != "." {
		files = append([]FileEntry{{
			Name:  "..",
			Path:  filepath.Dir(absPath),
			IsDir: true,
		}}, files...)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":  absPath,
		"files": files,
	})
}

// handleReadFile reads file content for the file viewer.
func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "not a file")
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	isBinary := false
	for _, b := range data {
		if b == 0 {
			isBinary = true
			break
		}
	}

	if isBinary {
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    absPath,
			"size":    info.Size(),
			"binary":  true,
			"content": base64.StdEncoding.EncodeToString(data),
		})
		return
	}

	content := string(data)
	maxLen := 200 * 1024 // 200KB limit for text display
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... (file truncated)"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    absPath,
		"size":    info.Size(),
		"binary":  false,
		"content": content,
		"lines":   strings.Count(content, "\n") + 1,
	})
}

// handleDiff compares two file paths.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	a := r.URL.Query().Get("a")
	b := r.URL.Query().Get("b")
	if a == "" || b == "" {
		writeError(w, http.StatusBadRequest, "both 'a' and 'b' query params required")
		return
	}

	// Simple line-by-line diff using LCS
	linesA, err := readLines(a)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read file a: "+err.Error())
		return
	}
	linesB, err := readLines(b)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read file b: "+err.Error())
		return
	}

	diff := computeDiff(linesA, linesB)
	writeJSON(w, http.StatusOK, map[string]any{
		"file_a": a,
		"file_b": b,
		"diff":   diff,
	})
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	return lines, nil
}

// DiffLine represents one line in a diff.
type DiffLine struct {
	Type    string `json:"type"` // "same", "add", "remove"
	Content string `json:"content"`
	LineA   int    `json:"line_a,omitempty"`
	LineB   int    `json:"line_b,omitempty"`
}

func computeDiff(a, b []string) []DiffLine {
	// Simple LCS-based diff for small files
	m, n := len(a), len(b)
	if m > 2000 || n > 2000 {
		return []DiffLine{{Type: "same", Content: "[diff too large, showing first 2000 lines each]"}}
	}

	// LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack
	var result []DiffLine
	i, j := m, n
	var temp []DiffLine
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			temp = append(temp, DiffLine{Type: "same", Content: a[i-1], LineA: i, LineB: j})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			temp = append(temp, DiffLine{Type: "add", Content: b[j-1], LineB: j})
			j--
		} else {
			temp = append(temp, DiffLine{Type: "remove", Content: a[i-1], LineA: i})
			i--
		}
	}

	// Reverse
	for k := len(temp) - 1; k >= 0; k-- {
		result = append(result, temp[k])
	}
	return result
}
