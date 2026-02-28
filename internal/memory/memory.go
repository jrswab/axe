package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jrswab/axe/internal/xdg"
)

// Now is the time source used by AppendEntry. Override in tests for
// deterministic timestamps.
var Now func() time.Time = time.Now

// FilePath returns the memory file path for the given agent.
// If customPath is non-empty it is returned as-is.
// Otherwise the default path is <xdg-data-dir>/memory/<agentName>.md.
// The function does not create any directories or files.
func FilePath(agentName, customPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}

	dataDir, err := xdg.GetDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dataDir, "memory", agentName+".md"), nil
}

// AppendEntry appends a timestamped memory entry to the file at path.
// Parent directories are created if they do not exist.
func AppendEntry(path, task, result string) error {
	// Create parent directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open memory file: %w", err)
	}
	defer f.Close()

	// Format timestamp
	ts := Now().UTC().Format(time.RFC3339)

	// Handle empty task
	if task == "" {
		task = "(none)"
	} else {
		// Replace newlines with spaces in task
		task = strings.ReplaceAll(task, "\n", " ")
	}

	// Handle empty result
	if result == "" {
		result = "(none)"
	} else if len(result) > 1000 {
		// Truncate result to 1000 characters and append "..."
		result = result[:1000] + "..."
	}

	// Write entry
	entry := fmt.Sprintf("## %s\n**Task:** %s\n**Result:** %s\n\n", ts, task, result)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write memory entry: %w", err)
	}

	return nil
}

// LoadEntries reads memory entries from the file at path.
// If the file does not exist, it returns ("", nil).
// If lastN is 0, all content is returned.
// If lastN > 0, only the last N entries (starting with "## ") are returned.
func LoadEntries(path string, lastN int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	content := string(data)
	if content == "" {
		return "", nil
	}

	// If lastN is 0, return all content
	if lastN == 0 {
		return content, nil
	}

	// Parse entries by finding lines starting with "## "
	lines := strings.SplitAfter(content, "\n")
	var entryStarts []int // indices into lines where entries begin
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			entryStarts = append(entryStarts, i)
		}
	}

	if len(entryStarts) == 0 {
		return "", nil
	}

	// Determine which entries to return
	startIdx := 0
	if lastN < len(entryStarts) {
		startIdx = len(entryStarts) - lastN
	}

	// Build result from the selected entries
	firstLine := entryStarts[startIdx]
	var result strings.Builder
	for i := firstLine; i < len(lines); i++ {
		result.WriteString(lines[i])
	}

	return result.String(), nil
}

// CountEntries counts the number of entries in the memory file at path.
// An entry is any line starting with "## ".
// If the file does not exist, it returns (0, nil).
func CountEntries(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	content := string(data)
	if content == "" {
		return 0, nil
	}

	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}

	return count, nil
}
