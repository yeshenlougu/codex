package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// parseArgs unmarshals JSON string args into a struct.
func parseArgs(rawArgs string, v any) error {
	if rawArgs == "" {
		rawArgs = "{}"
	}
	return json.Unmarshal([]byte(rawArgs), v)
}

// readFileWithLineNumbers reads a file and adds line numbers.
func readFileWithLineNumbers(path string, offset, limit int, maxSize int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("cannot stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	if info.Size() > maxSize {
		return "", fmt.Errorf("file too large: %d bytes (max: %d)", info.Size(), maxSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	if offset > totalLines {
		offset = totalLines
	}
	endLine := offset + limit - 1
	if endLine > totalLines {
		endLine = totalLines
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s (%d lines total, showing lines %d-%d)\n\n", path, totalLines, offset, endLine))

	for i := offset - 1; i < endLine; i++ {
		sb.WriteString(fmt.Sprintf("%6d|%s\n", i+1, lines[i]))
	}

	if endLine < totalLines {
		sb.WriteString(fmt.Sprintf("\n... (%d more lines, use offset=%d to continue)", totalLines-endLine, endLine+1))
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// editFile performs find-and-replace on a file.
func editFile(path, oldStr, newStr string, replaceAll bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", path, err)
	}

	content := string(data)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	if !replaceAll && count > 1 {
		return "", fmt.Errorf("old_string found %d times in %s — use replace_all:true or provide more context to make it unique", count, path)
	}

	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		newContent = strings.Replace(content, oldStr, newStr, 1)
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", path, err)
	}

	replaced := count
	if !replaceAll {
		replaced = 1
	}
	return fmt.Sprintf("Edited %s: %d occurrence(s) replaced", path, replaced), nil
}
