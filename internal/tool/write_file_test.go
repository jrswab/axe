package tool

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/provider"
)

func TestWriteFile_CreateNewFile(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-1",
		Name:      "write_file",
		Arguments: map[string]string{"path": "output.txt", "content": "hello world"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "output.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content: got %q, want %q", string(data), "hello world")
	}

	// Verify success message.
	want := "wrote 11 bytes to output.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}

	// Verify CallID.
	if result.CallID != "wf-1" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "wf-1")
	}
}

func TestWriteFile_OverwriteExisting(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "existing.txt"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-2",
		Name:      "write_file",
		Arguments: map[string]string{"path": "existing.txt", "content": "new content"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify old content is completely replaced.
	data, err := os.ReadFile(filepath.Join(tmpdir, "existing.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content: got %q, want %q", string(data), "new content")
	}

	want := "wrote 11 bytes to existing.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_CreateWithNestedDirs(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-3",
		Name:      "write_file",
		Arguments: map[string]string{"path": "a/b/c/deep.txt", "content": "nested"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "a", "b", "c", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("file content: got %q, want %q", string(data), "nested")
	}

	// Verify intermediate directories exist.
	for _, dir := range []string{"a", "a/b", "a/b/c"} {
		info, err := os.Stat(filepath.Join(tmpdir, dir))
		if err != nil {
			t.Errorf("directory %q does not exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestWriteFile_PathTraversalRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-4",
		Name:      "write_file",
		Arguments: map[string]string{"path": "../../escape.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path escapes workdir") {
		t.Errorf("Content %q should contain 'path escapes workdir'", result.Content)
	}
}

func TestWriteFile_AbsolutePathRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-5",
		Name:      "write_file",
		Arguments: map[string]string{"path": "/tmp/absolute.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "absolute paths") {
		t.Errorf("Content %q should mention absolute paths", result.Content)
	}
}

func TestWriteFile_EmptyContent(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-6",
		Name:      "write_file",
		Arguments: map[string]string{"path": "empty.txt", "content": ""},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file exists with 0 bytes.
	data, err := os.ReadFile(filepath.Join(tmpdir, "empty.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}

	want := "wrote 0 bytes to empty.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_MissingContentArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-7",
		Name:      "write_file",
		Arguments: map[string]string{"path": "nokey.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify file exists with 0 bytes.
	data, err := os.ReadFile(filepath.Join(tmpdir, "nokey.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes, got %d", len(data))
	}

	want := "wrote 0 bytes to nokey.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}
}

func TestWriteFile_MissingPathArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-8",
		Name:      "write_file",
		Arguments: map[string]string{},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is required") {
		t.Errorf("Content %q should contain 'path is required'", result.Content)
	}
}

func TestWriteFile_SymlinkEscapeRejected(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir()

	// Create symlink: workdir/link -> outsideDir
	linkPath := filepath.Join(tmpdir, "link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Fatal(err)
	}

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-9",
		Name:      "write_file",
		Arguments: map[string]string{"path": "link/escape.txt", "content": "bad"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}

	// Verify no file was created in the outside directory.
	escapedFile := filepath.Join(outsideDir, "escape.txt")
	if _, err := os.Stat(escapedFile); err == nil {
		t.Error("file was created outside workdir via symlink escape")
	}
}

func TestWriteFile_CallIDPassthrough(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	callID := "wf-unique-99"
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        callID,
		Name:      "write_file",
		Arguments: map[string]string{"path": "test.txt", "content": "x"},
	}, ExecContext{Workdir: tmpdir})

	if result.CallID != callID {
		t.Errorf("CallID: got %q, want %q", result.CallID, callID)
	}
}

func TestWriteFile_ByteCountAccurate(t *testing.T) {
	tmpdir := t.TempDir()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "wf-11",
		Name:      "write_file",
		Arguments: map[string]string{"path": "unicode.txt", "content": "日本語"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// "日本語" is 3 characters × 3 bytes each = 9 bytes in UTF-8.
	want := "wrote 9 bytes to unicode.txt"
	if result.Content != want {
		t.Errorf("Content: got %q, want %q", result.Content, want)
	}

	// Verify on disk.
	data, err := os.ReadFile(filepath.Join(tmpdir, "unicode.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if len(data) != 9 {
		t.Errorf("file size: got %d bytes, want 9", len(data))
	}
}

func TestWriteFile_Artifact_VerboseLogging(t *testing.T) {
	var stderr bytes.Buffer
	tmpdir := t.TempDir()
	artifactDir := t.TempDir()

	tracker := artifact.NewTracker()

	entry := writeFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:   "wf-artifact-verbose",
		Name: "write_file",
		Arguments: map[string]string{
			"path":     "test.txt",
			"content":  "hello artifact",
			"artifact": "true",
		},
	}, ExecContext{
		Workdir:         tmpdir,
		ArtifactDir:     artifactDir,
		ArtifactTracker: tracker,
		Verbose:         true,
		Stderr:          &stderr,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	// Verify verbose log was written to stderr for artifact write
	got := stderr.String()
	want := `[tool] write_file: path "test.txt" (14 bytes) (success)` + "\n"
	if got != want {
		t.Errorf("verbose log: got %q, want %q", got, want)
	}
}

func TestWriteFile_Artifact(t *testing.T) {
	tests := []struct {
		name             string
		artifactArg      string
		artifactDir      string
		path             string
		content          string
		wantError        bool
		wantErrorContain string
		wantContent      string
		wantInArtifact   bool
		wantRecorded     bool
	}{
		{
			name:           "artifact true with valid dir writes to artifact",
			artifactArg:    "true",
			artifactDir:    "",
			path:           "test.txt",
			content:        "hello artifact",
			wantError:      false,
			wantContent:    "wrote 14 bytes to test.txt (artifact)",
			wantInArtifact: true,
			wantRecorded:   true,
		},
		{
			name:             "artifact true with no artifact dir errors",
			artifactArg:      "true",
			artifactDir:      "",
			path:             "test.txt",
			content:          "hello",
			wantError:        true,
			wantErrorContain: "artifact directory not configured for this agent",
			wantInArtifact:   false,
			wantRecorded:     false,
		},
		{
			name:             "artifact true with path traversal rejected",
			artifactArg:      "true",
			artifactDir:      "artifact",
			path:             "../escape.txt",
			content:          "bad",
			wantError:        true,
			wantErrorContain: "path escapes artifact directory",
			wantInArtifact:   false,
			wantRecorded:     false,
		},
		{
			name:           "artifact false writes to workdir",
			artifactArg:    "false",
			artifactDir:    "artifact",
			path:           "workdir.txt",
			content:        "in workdir",
			wantError:      false,
			wantContent:    "wrote 10 bytes to workdir.txt",
			wantInArtifact: false,
			wantRecorded:   false,
		},
		{
			name:           "artifact absent writes to workdir",
			artifactArg:    "",
			artifactDir:    "artifact",
			path:           "nowhere.txt",
			content:        "in workdir",
			wantError:      false,
			wantContent:    "wrote 10 bytes to nowhere.txt",
			wantInArtifact: false,
			wantRecorded:   false,
		},
		{
			name:           "artifact TRUE uppercase treated as true",
			artifactArg:    "TRUE",
			artifactDir:    "",
			path:           "upper.txt",
			content:        "uppercase",
			wantError:      false,
			wantContent:    "wrote 9 bytes to upper.txt (artifact)",
			wantInArtifact: true,
			wantRecorded:   true,
		},
		{
			name:             "artifact true with empty path errors",
			artifactArg:      "true",
			artifactDir:      "artifact",
			path:             "",
			content:          "content",
			wantError:        true,
			wantErrorContain: "path is required",
			wantInArtifact:   false,
			wantRecorded:     false,
		},
		{
			name:             "artifact true with absolute path rejected",
			artifactArg:      "true",
			artifactDir:      "artifact",
			path:             "/tmp/absolute.txt",
			content:          "bad",
			wantError:        true,
			wantErrorContain: "absolute paths are not allowed",
			wantInArtifact:   false,
			wantRecorded:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			artifactDir := tt.artifactDir
			if artifactDir == "" && (tt.artifactArg == "true" || tt.artifactArg == "TRUE") && !tt.wantError {
				// For successful artifact tests, create an artifact dir
				artifactDir = t.TempDir()
			} else if artifactDir == "artifact" {
				// For error tests that still need an artifact dir set
				artifactDir = t.TempDir()
			}

			tracker := artifact.NewTracker()

			args := map[string]string{
				"path":    tt.path,
				"content": tt.content,
			}
			if tt.artifactArg != "" {
				args["artifact"] = tt.artifactArg
			}

			entry := writeFileEntry()
			result := entry.Execute(context.Background(), provider.ToolCall{
				ID:        "wf-artifact",
				Name:      "write_file",
				Arguments: args,
			}, ExecContext{
				Workdir:         tmpdir,
				ArtifactDir:     artifactDir,
				ArtifactTracker: tracker,
			})

			if tt.wantError {
				if !result.IsError {
					t.Errorf("expected error, got success with content: %s", result.Content)
					return
				}
				if !strings.Contains(result.Content, tt.wantErrorContain) {
					t.Errorf("error content %q should contain %q", result.Content, tt.wantErrorContain)
				}
				return
			}

			if result.IsError {
				t.Fatalf("unexpected error: %s", result.Content)
			}

			if result.Content != tt.wantContent {
				t.Errorf("Content: got %q, want %q", result.Content, tt.wantContent)
			}

			// Verify file location
			if tt.wantInArtifact {
				data, err := os.ReadFile(filepath.Join(artifactDir, tt.path))
				if err != nil {
					t.Errorf("file not found in artifact dir: %v", err)
				} else if string(data) != tt.content {
					t.Errorf("artifact file content: got %q, want %q", string(data), tt.content)
				}

				// Verify NOT in workdir
				if _, err := os.Stat(filepath.Join(tmpdir, tt.path)); err == nil {
					t.Error("file should not exist in workdir when artifact=true")
				}
			} else {
				// Verify in workdir
				data, err := os.ReadFile(filepath.Join(tmpdir, tt.path))
				if err != nil {
					t.Errorf("file not found in workdir: %v", err)
				} else if string(data) != tt.content {
					t.Errorf("workdir file content: got %q, want %q", string(data), tt.content)
				}
			}

			// Verify tracker
			if tt.wantRecorded {
				entries := tracker.Entries()
				if len(entries) != 1 {
					t.Errorf("expected 1 tracker entry, got %d", len(entries))
				} else {
					if entries[0].Path != tt.path {
						t.Errorf("tracker path: got %q, want %q", entries[0].Path, tt.path)
					}
					if entries[0].Size != int64(len(tt.content)) {
						t.Errorf("tracker size: got %d, want %d", entries[0].Size, len(tt.content))
					}
				}
			} else {
				entries := tracker.Entries()
				if len(entries) != 0 {
					t.Errorf("expected 0 tracker entries, got %d", len(entries))
				}
			}
		})
	}
}
