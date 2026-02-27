package resolve

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// --- Workdir Tests ---

func TestWorkdir_FlagOverride(t *testing.T) {
	result := Workdir("/flag/path", "/toml/path")
	if result != "/flag/path" {
		t.Errorf("expected /flag/path, got %s", result)
	}
}

func TestWorkdir_TOMLFallback(t *testing.T) {
	result := Workdir("", "/toml/path")
	if result != "/toml/path" {
		t.Errorf("expected /toml/path, got %s", result)
	}
}

func TestWorkdir_CWDFallback(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	result := Workdir("", "")
	if result != cwd {
		t.Errorf("expected %s, got %s", cwd, result)
	}
}

// --- Files Tests ---

func TestFiles_EmptyPatterns(t *testing.T) {
	dir := t.TempDir()
	result, err := Files(nil, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}

	result, err = Files([]string{}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestFiles_SimpleGlob(t *testing.T) {
	dir := t.TempDir()
	// Create test files
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello content"), 0644)
	os.WriteFile(filepath.Join(dir, "world.txt"), []byte("world content"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("markdown"), 0644)

	result, err := Files([]string{"*.txt"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}
	// Results should be sorted by path
	if result[0].Path != "hello.txt" {
		t.Errorf("expected hello.txt, got %s", result[0].Path)
	}
	if result[0].Content != "hello content" {
		t.Errorf("expected 'hello content', got %q", result[0].Content)
	}
	if result[1].Path != "world.txt" {
		t.Errorf("expected world.txt, got %s", result[1].Path)
	}
}

func TestFiles_DoubleStarGlob(t *testing.T) {
	dir := t.TempDir()
	// Create nested directory structure
	sub := filepath.Join(dir, "sub", "deep")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "mid.go"), []byte("package sub"), 0644)
	os.WriteFile(filepath.Join(sub, "deep.go"), []byte("package deep"), 0644)
	os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("not go"), 0644)

	result, err := Files([]string{"**/*.go"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(result), result)
	}

	paths := make([]string, len(result))
	for i, f := range result {
		paths[i] = f.Path
	}
	sort.Strings(paths)
	expected := []string{"root.go", "sub/deep/deep.go", "sub/mid.go"}
	for i, p := range expected {
		if paths[i] != p {
			t.Errorf("expected %s at index %d, got %s", p, i, paths[i])
		}
	}
}

func TestFiles_InvalidPattern(t *testing.T) {
	dir := t.TempDir()
	_, err := Files([]string{"["}, dir)
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

func TestFiles_InvalidDoubleStarPattern(t *testing.T) {
	dir := t.TempDir()
	// Create a file so the directory isn't empty
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	_, err := Files([]string{"**/["}, dir)
	if err == nil {
		t.Fatal("expected error for invalid ** pattern '**/[', got nil")
	}
	if !strings.Contains(err.Error(), "invalid glob pattern") {
		t.Errorf("expected error to contain 'invalid glob pattern', got %q", err.Error())
	}
}

func TestFiles_NoMatches(t *testing.T) {
	dir := t.TempDir()
	result, err := Files([]string{"*.xyz"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestFiles_Deduplication(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	result, err := Files([]string{"*.txt", "file.txt"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 file (deduplicated), got %d", len(result))
	}
}

func TestFiles_SortedOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "charlie.txt"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "bravo.txt"), []byte("b"), 0644)

	result, err := Files([]string{"*.txt"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}
	if result[0].Path != "alpha.txt" || result[1].Path != "bravo.txt" || result[2].Path != "charlie.txt" {
		t.Errorf("files not sorted: %s, %s, %s", result[0].Path, result[1].Path, result[2].Path)
	}
}

func TestFiles_SkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a file with null bytes in the first 512 bytes
	binaryContent := bytes.Repeat([]byte("A"), 100)
	binaryContent[50] = 0x00
	os.WriteFile(filepath.Join(dir, "binary.dat"), binaryContent, 0644)
	os.WriteFile(filepath.Join(dir, "text.dat"), []byte("hello world"), 0644)

	result, err := Files([]string{"*.dat"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 file (binary skipped), got %d", len(result))
	}
	if result[0].Path != "text.dat" {
		t.Errorf("expected text.dat, got %s", result[0].Path)
	}
}

func TestFiles_SymlinkOutsideWorkdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}
	dir := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0644)

	// Create symlink inside workdir pointing outside
	os.Symlink(outsideFile, filepath.Join(dir, "link.txt"))
	os.WriteFile(filepath.Join(dir, "local.txt"), []byte("local"), 0644)

	result, err := Files([]string{"*.txt"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 file (symlink skipped), got %d", len(result))
	}
	if result[0].Path != "local.txt" {
		t.Errorf("expected local.txt, got %s", result[0].Path)
	}
}

// --- Skill Tests ---

func TestSkill_EmptyPath(t *testing.T) {
	result, err := Skill("", "/some/config/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSkill_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	skillFile := filepath.Join(dir, "SKILL.md")
	os.WriteFile(skillFile, []byte("# My Skill\nDo stuff."), 0644)

	result, err := Skill(skillFile, "/irrelevant/config/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "# My Skill\nDo stuff." {
		t.Errorf("unexpected content: %q", result)
	}
}

func TestSkill_RelativePath(t *testing.T) {
	configDir := t.TempDir()
	skillFile := filepath.Join(configDir, "skills", "test.md")
	os.MkdirAll(filepath.Join(configDir, "skills"), 0755)
	os.WriteFile(skillFile, []byte("relative skill"), 0644)

	result, err := Skill("skills/test.md", configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "relative skill" {
		t.Errorf("unexpected content: %q", result)
	}
}

func TestSkill_NotFound(t *testing.T) {
	_, err := Skill("/nonexistent/SKILL.md", "/some/dir")
	if err == nil {
		t.Error("expected error for missing skill, got nil")
	}
	expected := "skill not found: /nonexistent/SKILL.md"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

// --- Stdin Tests ---

func TestStdin_NotPiped(t *testing.T) {
	// When running under `go test`, stdin is typically a terminal (not piped).
	// This test may need to be skipped in CI environments where stdin behavior differs.
	result, err := Stdin()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string when stdin is not piped, got %q", result)
	}
}

// --- BuildSystemPrompt Tests ---

func TestBuildSystemPrompt_AllSections(t *testing.T) {
	files := []FileContent{
		{Path: "main.go", Content: "package main"},
		{Path: "util.go", Content: "package util"},
	}
	result := BuildSystemPrompt("You are helpful.", "Do the task.", files)

	// Check system prompt is at the start
	if !strings.HasPrefix(result, "You are helpful.") {
		t.Errorf("expected system prompt at start, got %q", result[:50])
	}
	// Check skill section
	if !strings.Contains(result, "\n\n---\n\n## Skill\n\nDo the task.") {
		t.Error("expected skill section with delimiter")
	}
	// Check files section
	if !strings.Contains(result, "\n\n---\n\n## Context Files\n\n") {
		t.Error("expected context files section with delimiter")
	}
	// Check file formatting
	if !strings.Contains(result, "### main.go\n```go\npackage main\n```") {
		t.Error("expected main.go formatted with fenced code block")
	}
	if !strings.Contains(result, "### util.go\n```go\npackage util\n```") {
		t.Error("expected util.go formatted with fenced code block")
	}
}

func TestBuildSystemPrompt_SystemPromptOnly(t *testing.T) {
	result := BuildSystemPrompt("You are helpful.", "", nil)
	if result != "You are helpful." {
		t.Errorf("expected just system prompt, got %q", result)
	}
	// Should NOT contain any delimiters
	if strings.Contains(result, "---") {
		t.Error("should not contain section delimiters when only system prompt is present")
	}
}

func TestBuildSystemPrompt_AllEmpty(t *testing.T) {
	result := BuildSystemPrompt("", "", nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestBuildSystemPrompt_SkillOnly(t *testing.T) {
	result := BuildSystemPrompt("", "Do the task.", nil)
	expected := "\n\n---\n\n## Skill\n\nDo the task."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildSystemPrompt_FilesOnly(t *testing.T) {
	files := []FileContent{
		{Path: "readme.md", Content: "# Hello"},
	}
	result := BuildSystemPrompt("", "", files)
	if !strings.HasPrefix(result, "\n\n---\n\n## Context Files\n\n") {
		t.Errorf("expected context files section at start, got %q", result)
	}
	if !strings.Contains(result, "### readme.md\n```md\n# Hello\n```") {
		t.Errorf("expected readme.md formatted with fenced code block, got %q", result)
	}
}
