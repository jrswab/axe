package resolve

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Workdir resolves the working directory using a priority chain:
// flagValue (from --workdir) > tomlValue (from agent TOML) > os.Getwd() > "."
func Workdir(flagValue, tomlValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if tomlValue != "" {
		return tomlValue
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// FileContent holds a matched file's relative path and its text content.
type FileContent struct {
	Path    string
	Content string
}

// Files resolves file glob patterns relative to workdir and returns their contents.
// It supports simple globs (via filepath.Glob) and ** patterns (via filepath.WalkDir).
// Binary files, symlinks pointing outside workdir, and duplicates are skipped.
func Files(patterns []string, workdir string) ([]FileContent, error) {
	if len(patterns) == 0 {
		return []FileContent{}, nil
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workdir: %w", err)
	}

	seen := make(map[string]bool)
	var results []FileContent

	for _, pattern := range patterns {
		var matches []string
		var matchErr error

		if strings.Contains(pattern, "**") {
			matches, matchErr = doubleStarGlob(pattern, absWorkdir)
		} else {
			matches, matchErr = simpleGlob(pattern, absWorkdir)
		}

		if matchErr != nil {
			return nil, matchErr
		}

		for _, absPath := range matches {
			relPath, err := filepath.Rel(absWorkdir, absPath)
			if err != nil {
				continue
			}

			if seen[relPath] {
				continue
			}

			// Check if symlink points outside workdir
			if isSymlinkOutside(absPath, absWorkdir) {
				continue
			}

			content, err := readTextFile(absPath)
			if err != nil {
				// Skip files that can't be read
				continue
			}

			seen[relPath] = true
			results = append(results, FileContent{
				Path:    filepath.ToSlash(relPath),
				Content: content,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	return results, nil
}

// simpleGlob resolves a single-level glob pattern relative to workdir.
func simpleGlob(pattern, absWorkdir string) ([]string, error) {
	fullPattern := filepath.Join(absWorkdir, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
	}
	return matches, nil
}

// doubleStarGlob resolves a ** glob pattern by walking the directory tree.
func doubleStarGlob(pattern, absWorkdir string) ([]string, error) {
	// Split the pattern on ** segments and match via WalkDir
	var matches []string

	err := filepath.WalkDir(absWorkdir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(absWorkdir, path)
		if err != nil {
			return nil
		}

		if doubleStarMatch(pattern, filepath.ToSlash(relPath)) {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for pattern %q: %w", pattern, err)
	}

	return matches, nil
}

// doubleStarMatch checks if a relative path matches a pattern containing **.
// It supports patterns like **/*.go, a/**/b.go, **/*.ext, etc.
func doubleStarMatch(pattern, path string) bool {
	// Split pattern and path into segments
	patParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchParts(patParts, pathParts)
}

// matchParts recursively matches pattern parts against path parts.
func matchParts(patParts, pathParts []string) bool {
	for len(patParts) > 0 && len(pathParts) > 0 {
		if patParts[0] == "**" {
			// ** matches zero or more path segments
			rest := patParts[1:]
			// Try matching rest against every suffix of pathParts
			for i := 0; i <= len(pathParts); i++ {
				if matchParts(rest, pathParts[i:]) {
					return true
				}
			}
			return false
		}

		matched, err := filepath.Match(patParts[0], pathParts[0])
		if err != nil || !matched {
			return false
		}

		patParts = patParts[1:]
		pathParts = pathParts[1:]
	}

	// Handle trailing ** which matches zero segments
	for len(patParts) > 0 && patParts[0] == "**" {
		patParts = patParts[1:]
	}

	return len(patParts) == 0 && len(pathParts) == 0
}

// isSymlinkOutside checks if a path is a symlink that resolves outside the workdir.
func isSymlinkOutside(path, absWorkdir string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}

	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return true // can't resolve, skip to be safe
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return true
	}

	// Check if target is within workdir
	return !strings.HasPrefix(absTarget, absWorkdir+string(filepath.Separator)) && absTarget != absWorkdir
}

// readTextFile reads a file and returns its content, or an error if it's binary.
// Binary detection: if any null byte exists in the first 512 bytes, it's binary.
func readTextFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Check first 512 bytes for null bytes (binary detection)
	checkLen := 512
	if len(data) < checkLen {
		checkLen = len(data)
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return "", fmt.Errorf("binary file detected: %s", path)
		}
	}

	return string(data), nil
}

// Stdin reads stdin content if it is piped (not a terminal).
// Returns empty string if stdin is a terminal (interactive).
func Stdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat stdin: %w", err)
	}

	// If ModeCharDevice is set, stdin is a terminal (not piped)
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}

	return string(data), nil
}

// BuildSystemPrompt assembles a single system prompt string from non-empty sections.
// Sections are: system prompt (as-is), skill (with delimiter), files (with delimiter and code blocks).
func BuildSystemPrompt(systemPrompt, skillContent string, files []FileContent) string {
	var b strings.Builder

	if systemPrompt != "" {
		b.WriteString(systemPrompt)
	}

	if skillContent != "" {
		b.WriteString("\n\n---\n\n## Skill\n\n")
		b.WriteString(skillContent)
	}

	if len(files) > 0 {
		b.WriteString("\n\n---\n\n## Context Files\n\n")
		for i, f := range files {
			if i > 0 {
				b.WriteString("\n\n")
			}
			ext := strings.TrimPrefix(filepath.Ext(f.Path), ".")
			b.WriteString("### ")
			b.WriteString(f.Path)
			b.WriteString("\n```")
			b.WriteString(ext)
			b.WriteString("\n")
			b.WriteString(f.Content)
			b.WriteString("\n```")
		}
	}

	return b.String()
}

// Skill loads skill content from a file path.
// If skillPath is empty, returns empty string. If relative, resolves against configDir.
func Skill(skillPath, configDir string) (string, error) {
	if skillPath == "" {
		return "", nil
	}

	resolved := skillPath
	if !filepath.IsAbs(skillPath) {
		resolved = filepath.Join(configDir, skillPath)
	}

	if _, err := os.Stat(resolved); os.IsNotExist(err) {
		return "", fmt.Errorf("skill not found: %s", resolved)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read skill: %w", err)
	}

	return string(data), nil
}
