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

func TestListDirectory_Artifact(t *testing.T) {
	tests := []struct {
		name           string
		artifactArg    string // value for "artifact" argument (empty = absent)
		path           string
		artifactDirSet bool // if true, use temp dir; if false, empty string
		workdirFiles   []string
		artifactFiles  []string
		artifactDirs   []string
		wantError      bool
		wantContain    string // substring expected in Content
		wantNotContain string // substring that must NOT appear in Content
		wantExact      string // exact match for Content (after trimming)
		wantLines      int    // expected number of lines
		wantLine0      string
		wantLine1      string
	}{
		{
			name:           "true with valid dir lists artifact dir",
			artifactArg:    "true",
			path:           ".",
			artifactDirSet: true,
			workdirFiles:   []string{"work.txt"},
			artifactFiles:  []string{"artifact.txt"},
			artifactDirs:   []string{"build"},
			wantError:      false,
			wantLines:      2,
			wantLine0:      "artifact.txt",
			wantLine1:      "build/",
			wantNotContain: "work.txt",
		},
		{
			name:           "mixed-case TrUe treated as true",
			artifactArg:    "TrUe",
			path:           ".",
			artifactDirSet: true,
			workdirFiles:   []string{"work.txt"},
			artifactFiles:  []string{"artifact.txt"},
			artifactDirs:   []string{"build"},
			wantError:      false,
			wantLines:      2,
			wantLine0:      "artifact.txt",
			wantLine1:      "build/",
			wantNotContain: "work.txt",
		},
		{
			name:           "true with path traversal escapes",
			artifactArg:    "true",
			path:           "../escape",
			artifactDirSet: true,
			wantError:      true,
			wantContain:    "path escapes artifact directory",
		},
		{
			name:           "true with empty artifact dir errors",
			artifactArg:    "true",
			path:           ".",
			artifactDirSet: false,
			wantError:      true,
			wantContain:    "artifact directory not configured",
		},
		{
			name:           "false lists workdir not artifact dir",
			artifactArg:    "false",
			path:           ".",
			artifactDirSet: true,
			workdirFiles:   []string{"work.txt"},
			artifactFiles:  []string{"artifact.txt"},
			wantError:      false,
			wantExact:      "work.txt",
			wantNotContain: "artifact.txt",
		},
		{
			name:           "absent lists workdir not artifact dir",
			artifactArg:    "",
			path:           ".",
			artifactDirSet: true,
			workdirFiles:   []string{"work.txt"},
			artifactFiles:  []string{"artifact.txt"},
			wantError:      false,
			wantExact:      "work.txt",
			wantNotContain: "artifact.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()

			var artifactDir string
			if tt.artifactDirSet {
				artifactDir = t.TempDir()
			}

			// Create files in workdir
			for _, f := range tt.workdirFiles {
				if err := os.WriteFile(filepath.Join(tmpdir, f), []byte("w"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Create files in artifact dir
			for _, f := range tt.artifactFiles {
				if err := os.WriteFile(filepath.Join(artifactDir, f), []byte("a"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Create subdirs in artifact dir
			for _, d := range tt.artifactDirs {
				if err := os.Mkdir(filepath.Join(artifactDir, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			args := map[string]string{"path": tt.path}
			if tt.artifactArg != "" {
				args["artifact"] = tt.artifactArg
			}

			entry := listDirectoryEntry()
			result := entry.Execute(context.Background(), provider.ToolCall{
				ID:        "test-artifact-" + tt.name,
				Name:      "list_directory",
				Arguments: args,
			}, ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir})

			if tt.wantError {
				if !result.IsError {
					t.Fatal("expected error, got success")
				}
				if tt.wantContain != "" && !strings.Contains(result.Content, tt.wantContain) {
					t.Errorf("content %q should contain %q", result.Content, tt.wantContain)
				}
				return
			}

			if result.IsError {
				t.Fatalf("unexpected error: %s", result.Content)
			}

			if tt.wantNotContain != "" && strings.Contains(result.Content, tt.wantNotContain) {
				t.Errorf("content %q should not contain %q", result.Content, tt.wantNotContain)
			}

			if tt.wantExact != "" {
				if got := strings.TrimRight(result.Content, "\n"); got != tt.wantExact {
					t.Errorf("got %q, want %q", got, tt.wantExact)
				}
			}

			if tt.wantLines > 0 {
				lines := strings.Split(strings.TrimRight(result.Content, "\n"), "\n")
				if len(lines) != tt.wantLines {
					t.Fatalf("expected %d lines, got %d: %q", tt.wantLines, len(lines), result.Content)
				}
				if tt.wantLine0 != "" && lines[0] != tt.wantLine0 {
					t.Errorf("line 0: got %q, want %q", lines[0], tt.wantLine0)
				}
				if tt.wantLine1 != "" && lines[1] != tt.wantLine1 {
					t.Errorf("line 1: got %q, want %q", lines[1], tt.wantLine1)
				}
			}
		})
	}
}
