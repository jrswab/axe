package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/provider"
)

func TestRunCommand_Success(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-1",
		Name:      "run_command",
		Arguments: map[string]string{"command": "echo hello"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "hello\n" {
		t.Errorf("Content: got %q, want %q", result.Content, "hello\n")
	}
	if result.CallID != "rc-1" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "rc-1")
	}
}

func TestRunCommand_FailingCommand(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-2",
		Name:      "run_command",
		Arguments: map[string]string{"command": "exit 42"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "exit code 42") {
		t.Errorf("Content %q should contain 'exit code 42'", result.Content)
	}
}

func TestRunCommand_OutputCapture(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-3",
		Name:      "run_command",
		Arguments: map[string]string{"command": "echo stdout; echo stderr >&2"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "stdout") {
		t.Errorf("Content %q should contain 'stdout'", result.Content)
	}
	if !strings.Contains(result.Content, "stderr") {
		t.Errorf("Content %q should contain 'stderr'", result.Content)
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	tmpdir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	entry := runCommandEntry()
	result := entry.Execute(ctx, provider.ToolCall{
		ID:        "rc-4",
		Name:      "run_command",
		Arguments: map[string]string{"command": "sleep 60"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestRunCommand_LargeOutputTruncation(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-5",
		Name:      "run_command",
		Arguments: map[string]string{"command": "dd if=/dev/zero bs=1024 count=200 2>/dev/null | tr '\\0' 'A'"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "[output truncated, exceeded 100KB]") {
		t.Error("Content should contain truncation notice")
	}
	contentLen := len(result.Content)
	if contentLen <= 102400 {
		t.Errorf("Content length %d should be > 102400 (includes truncation notice)", contentLen)
	}
	if contentLen > 150000 {
		t.Errorf("Content length %d should be significantly less than 200KB (proving truncation)", contentLen)
	}
}

func TestRunCommand_MissingCommand(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-6",
		Name:      "run_command",
		Arguments: map[string]string{},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "command is required") {
		t.Errorf("Content %q should contain 'command is required'", result.Content)
	}
}

func TestRunCommand_EmptyCommand(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-7",
		Name:      "run_command",
		Arguments: map[string]string{"command": ""},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "command is required") {
		t.Errorf("Content %q should contain 'command is required'", result.Content)
	}
}

func TestRunCommand_WorkdirRespected(t *testing.T) {
	tmpdir := t.TempDir()

	// Resolve symlinks since t.TempDir() may involve symlinks on some platforms
	resolvedTmpdir, err := filepath.EvalSymlinks(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-8",
		Name:      "run_command",
		Arguments: map[string]string{"command": "pwd"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	outputDir := strings.TrimSpace(result.Content)
	resolvedOutputDir, err := filepath.EvalSymlinks(outputDir)
	if err != nil {
		t.Fatalf("failed to resolve output path %q: %v", outputDir, err)
	}
	if resolvedOutputDir != resolvedTmpdir {
		t.Errorf("pwd output %q resolved to %q, want %q", outputDir, resolvedOutputDir, resolvedTmpdir)
	}
}

func TestRunCommand_CallIDPassthrough(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-unique-77",
		Name:      "run_command",
		Arguments: map[string]string{"command": "true"},
	}, ExecContext{Workdir: tmpdir})

	if result.CallID != "rc-unique-77" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "rc-unique-77")
	}
}

func TestRunCommand_FailingCommandWithOutput(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rc-10",
		Name:      "run_command",
		Arguments: map[string]string{"command": "echo before-fail; exit 1"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "exit code 1") {
		t.Errorf("Content %q should contain 'exit code 1'", result.Content)
	}
	if !strings.Contains(result.Content, "before-fail") {
		t.Errorf("Content %q should contain 'before-fail'", result.Content)
	}
}

func TestRunCommand_RejectsAbsolutePathOutsideWorkdir(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-1",
		Name:      "run_command",
		Arguments: map[string]string{"command": "cat /etc/passwd"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error for absolute path outside workdir, got success")
	}
	if !strings.Contains(result.Content, "absolute path") {
		t.Errorf("Content %q should contain 'absolute path'", result.Content)
	}
}

func TestRunCommand_RejectsParentTraversal(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-2",
		Name:      "run_command",
		Arguments: map[string]string{"command": "cat ../../etc/passwd"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error for parent traversal, got success")
	}
	if !strings.Contains(result.Content, "parent traversal") {
		t.Errorf("Content %q should contain 'parent traversal'", result.Content)
	}
}

func TestRunCommand_RejectsTildeExpansion(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-3",
		Name:      "run_command",
		Arguments: map[string]string{"command": "cat ~/secrets"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error for tilde expansion, got success")
	}
}

func TestRunCommand_AllowsAbsolutePathInsideWorkdir(t *testing.T) {
	tmpdir := t.TempDir()

	// Create a file inside tmpdir
	testFile := filepath.Join(tmpdir, "file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-4",
		Name:      "run_command",
		Arguments: map[string]string{"command": "cat " + tmpdir + "/file.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "test content") {
		t.Errorf("Content %q should contain 'test content'", result.Content)
	}
}

func TestRunCommand_HomeSetToWorkdir(t *testing.T) {
	tmpdir := t.TempDir()

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-5",
		Name:      "run_command",
		Arguments: map[string]string{"command": "echo $HOME"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	outputDir := strings.TrimSpace(result.Content)
	resolvedOutput, err := filepath.EvalSymlinks(outputDir)
	if err != nil {
		t.Fatalf("failed to resolve output path %q: %v", outputDir, err)
	}
	resolvedTmpdir, err := filepath.EvalSymlinks(tmpdir)
	if err != nil {
		t.Fatalf("failed to resolve tmpdir %q: %v", tmpdir, err)
	}

	if resolvedOutput != resolvedTmpdir {
		t.Errorf("HOME %q resolved to %q, want %q", outputDir, resolvedOutput, resolvedTmpdir)
	}
}

func TestRunCommand_StripsEnvVars(t *testing.T) {
	tmpdir := t.TempDir()

	t.Setenv("SECRET_TOKEN", "leaked")

	entry := runCommandEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "sandbox-6",
		Name:      "run_command",
		Arguments: map[string]string{"command": "echo $SECRET_TOKEN"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if strings.TrimSpace(result.Content) != "" {
		t.Errorf("Content %q should be empty (env var stripped)", result.Content)
	}
}
