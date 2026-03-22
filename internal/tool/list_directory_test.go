package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestListDirectory_ExistingDir(t *testing.T) {
	tmpdir := t.TempDir()

	// Create two files and one subdirectory
	if err := os.WriteFile(filepath.Join(tmpdir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpdir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-1",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "."},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	lines := strings.Split(strings.TrimRight(result.Content, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), result.Content)
	}

	// os.ReadDir returns lexicographic order
	if lines[0] != "a.txt" {
		t.Errorf("line 0: got %q, want %q", lines[0], "a.txt")
	}
	if lines[1] != "b.txt" {
		t.Errorf("line 1: got %q, want %q", lines[1], "b.txt")
	}
	if lines[2] != "sub/" {
		t.Errorf("line 2: got %q, want %q", lines[2], "sub/")
	}
}

func TestListDirectory_NestedPath(t *testing.T) {
	tmpdir := t.TempDir()
	sub := filepath.Join(tmpdir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-2",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "sub"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	want := "nested.txt"
	if strings.TrimRight(result.Content, "\n") != want {
		t.Errorf("got %q, want %q", result.Content, want)
	}
}

func TestListDirectory_EmptyDir(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-3",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "."},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
}

func TestListDirectory_NonexistentPath(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-4",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "no_such_dir"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestListDirectory_AbsolutePathRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-5",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "/etc"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "absolute paths") {
		t.Errorf("content %q should mention absolute paths", result.Content)
	}
}

func TestListDirectory_ParentTraversalRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-6",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "../../etc"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path escapes") {
		t.Errorf("content %q should mention path escaping", result.Content)
	}
}

func TestListDirectory_SymlinkEscapeRejected(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir()
	linkPath := filepath.Join(tmpdir, "escape")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-7",
		Name:      "list_directory",
		Arguments: map[string]string{"path": "escape"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestListDirectory_MissingPathArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "test-8",
		Name:      "list_directory",
		Arguments: map[string]string{},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is required") {
		t.Errorf("content %q should mention path required", result.Content)
	}
}

func TestListDirectory_Artifact_TrueWithValidDir(t *testing.T) {
	tmpdir := t.TempDir()
	artifactDir := t.TempDir()

	// Create files in artifact dir
	if err := os.WriteFile(filepath.Join(artifactDir, "artifact.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(artifactDir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create files in workdir (should NOT be listed)
	if err := os.WriteFile(filepath.Join(tmpdir, "work.txt"), []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "test-artifact-1",
		Name: "list_directory",
		Arguments: map[string]string{
			"path":     ".",
			"artifact": "true",
		},
	}, ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	lines := strings.Split(strings.TrimRight(result.Content, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines from artifact dir, got %d: %q", len(lines), result.Content)
	}

	// os.ReadDir returns lexicographic order
	if lines[0] != "artifact.txt" {
		t.Errorf("line 0: got %q, want %q", lines[0], "artifact.txt")
	}
	if lines[1] != "build/" {
		t.Errorf("line 1: got %q, want %q", lines[1], "build/")
	}
}

func TestListDirectory_Artifact_TrueWithEmptyArtifactDir(t *testing.T) {
	tmpdir := t.TempDir()

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "test-artifact-2",
		Name: "list_directory",
		Arguments: map[string]string{
			"path":     ".",
			"artifact": "true",
		},
	}, ExecContext{Workdir: tmpdir, ArtifactDir: ""})

	if !result.IsError {
		t.Fatal("expected error when artifact=true but ArtifactDir is empty, got success")
	}
	if !strings.Contains(result.Content, "artifact directory not configured") {
		t.Errorf("content %q should mention 'artifact directory not configured'", result.Content)
	}
}

func TestListDirectory_Artifact_False(t *testing.T) {
	tmpdir := t.TempDir()
	artifactDir := t.TempDir()

	// Create file in workdir (should be listed)
	if err := os.WriteFile(filepath.Join(tmpdir, "work.txt"), []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create file in artifact dir (should NOT be listed)
	if err := os.WriteFile(filepath.Join(artifactDir, "artifact.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "test-artifact-3",
		Name: "list_directory",
		Arguments: map[string]string{
			"path":     ".",
			"artifact": "false",
		},
	}, ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	want := "work.txt"
	if strings.TrimRight(result.Content, "\n") != want {
		t.Errorf("got %q, want %q (should list workdir, not artifact dir)", result.Content, want)
	}
}

func TestListDirectory_Artifact_Absent(t *testing.T) {
	tmpdir := t.TempDir()
	artifactDir := t.TempDir()

	// Create file in workdir (should be listed)
	if err := os.WriteFile(filepath.Join(tmpdir, "work.txt"), []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create file in artifact dir (should NOT be listed)
	if err := os.WriteFile(filepath.Join(artifactDir, "artifact.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := listDirectoryEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "test-artifact-4",
		Name: "list_directory",
		Arguments: map[string]string{
			"path": ".",
			// artifact parameter is absent
		},
	}, ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	want := "work.txt"
	if strings.TrimRight(result.Content, "\n") != want {
		t.Errorf("got %q, want %q (should list workdir when artifact param absent)", result.Content, want)
	}
}
