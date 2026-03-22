package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestReadFile_FullFile(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-1",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	want := "1: line1\n2: line2\n3: line3"
	if result.Content != want {
		t.Errorf("Content:\ngot  %q\nwant %q", result.Content, want)
	}
	if result.CallID != "rf-1" {
		t.Errorf("CallID: got %q, want %q", result.CallID, "rf-1")
	}
}

func TestReadFile_WithOffset(t *testing.T) {
	tmpdir := t.TempDir()
	var lines []string
	for i := 1; i <= 6; i++ {
		lines = append(lines, "line"+string(rune('0'+i)))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-2",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "3"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.HasPrefix(result.Content, "3: ") {
		t.Errorf("expected output to start with '3: ', got %q", result.Content)
	}
}

func TestReadFile_WithLimit(t *testing.T) {
	tmpdir := t.TempDir()
	var lines []string
	for i := 1; i <= 12; i++ {
		lines = append(lines, "line"+strings.Repeat("x", i))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-3",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "limit": "3"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	outLines := strings.Split(result.Content, "\n")
	if len(outLines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(outLines), result.Content)
	}
	if !strings.HasPrefix(outLines[0], "1: ") {
		t.Errorf("first line should start with '1: ', got %q", outLines[0])
	}
}

func TestReadFile_WithOffsetAndLimit(t *testing.T) {
	tmpdir := t.TempDir()
	var lines []string
	for i := 1; i <= 12; i++ {
		lines = append(lines, "line"+strings.Repeat("x", i))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-4",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "4", "limit": "2"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	outLines := strings.Split(result.Content, "\n")
	if len(outLines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(outLines), result.Content)
	}
	if !strings.HasPrefix(outLines[0], "4: ") {
		t.Errorf("first line should start with '4: ', got %q", outLines[0])
	}
	if !strings.HasPrefix(outLines[1], "5: ") {
		t.Errorf("second line should start with '5: ', got %q", outLines[1])
	}
}

func TestReadFile_OffsetPastEOF(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-5",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "10"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "offset 10 exceeds file length of 5 lines") {
		t.Errorf("Content %q should contain offset exceeds message", result.Content)
	}
}

func TestReadFile_LimitExceedsRemaining(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-6",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "4", "limit": "100"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	outLines := strings.Split(result.Content, "\n")
	if len(outLines) != 2 {
		t.Errorf("expected 2 lines (4 and 5), got %d: %q", len(outLines), result.Content)
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "empty.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-7",
		Name:      "read_file",
		Arguments: map[string]string{"path": "empty.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
}

func TestReadFile_EmptyFileWithOffsetTwo(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "empty.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-8",
		Name:      "read_file",
		Arguments: map[string]string{"path": "empty.txt", "offset": "2"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "offset 2 exceeds file length of 0 lines") {
		t.Errorf("Content %q should contain offset exceeds message", result.Content)
	}
}

func TestReadFile_NonexistentFile(t *testing.T) {
	tmpdir := t.TempDir()

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-9",
		Name:      "read_file",
		Arguments: map[string]string{"path": "no_such_file.txt"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestReadFile_BinaryFileRejected(t *testing.T) {
	tmpdir := t.TempDir()
	// Create file with NUL byte in first 512 bytes
	data := []byte("hello\x00world")
	if err := os.WriteFile(filepath.Join(tmpdir, "binary.bin"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-10",
		Name:      "read_file",
		Arguments: map[string]string{"path": "binary.bin"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "binary file detected") {
		t.Errorf("Content %q should contain 'binary file detected'", result.Content)
	}
}

func TestReadFile_DirectoryRejected(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpdir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-11",
		Name:      "read_file",
		Arguments: map[string]string{"path": "subdir"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is a directory, not a file") {
		t.Errorf("Content %q should contain directory error message", result.Content)
	}
}

func TestReadFile_AbsolutePathRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-12",
		Name:      "read_file",
		Arguments: map[string]string{"path": "/etc/passwd"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "absolute paths") {
		t.Errorf("Content %q should mention absolute paths", result.Content)
	}
}

func TestReadFile_ParentTraversalRejected(t *testing.T) {
	tmpdir := t.TempDir()

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-13",
		Name:      "read_file",
		Arguments: map[string]string{"path": "../../etc/passwd"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path escapes") {
		t.Errorf("Content %q should mention path escaping", result.Content)
	}
}

func TestReadFile_SymlinkEscapeRejected(t *testing.T) {
	tmpdir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(tmpdir, "escape.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-14",
		Name:      "read_file",
		Arguments: map[string]string{"path": "escape.txt"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestReadFile_MissingPathArgument(t *testing.T) {
	tmpdir := t.TempDir()

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-15",
		Name:      "read_file",
		Arguments: map[string]string{},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "path is required") {
		t.Errorf("Content %q should mention path required", result.Content)
	}
}

func TestReadFile_InvalidOffset(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-16",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "abc"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestReadFile_ZeroOffset(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-17",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "offset": "0"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "offset must be >= 1") {
		t.Errorf("Content %q should contain 'offset must be >= 1'", result.Content)
	}
}

func TestReadFile_InvalidLimit(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-18",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "limit": "xyz"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
}

func TestReadFile_ZeroLimit(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-19",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt", "limit": "0"},
	}, ExecContext{Workdir: tmpdir})

	if !result.IsError {
		t.Fatal("expected error, got success")
	}
	if !strings.Contains(result.Content, "limit must be >= 1") {
		t.Errorf("Content %q should contain 'limit must be >= 1'", result.Content)
	}
}

func TestReadFile_TrailingNewline(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-20",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	want := "1: a\n2: b"
	if result.Content != want {
		t.Errorf("Content:\ngot  %q\nwant %q", result.Content, want)
	}
}

func TestReadFile_NoTrailingNewline(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("a\nb"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-21",
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	want := "1: a\n2: b"
	if result.Content != want {
		t.Errorf("Content:\ngot  %q\nwant %q", result.Content, want)
	}
}

func TestReadFile_CallIDPassthrough(t *testing.T) {
	tmpdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	callID := "unique-call-id-99"
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        callID,
		Name:      "read_file",
		Arguments: map[string]string{"path": "test.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.CallID != callID {
		t.Errorf("CallID: got %q, want %q", result.CallID, callID)
	}
}

func TestReadFile_NestedPath(t *testing.T) {
	tmpdir := t.TempDir()
	nested := filepath.Join(tmpdir, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("nested content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := readFileEntry()
	result := entry.Execute(context.Background(), provider.ToolCall{
		ID:        "rf-23",
		Name:      "read_file",
		Arguments: map[string]string{"path": "sub/deep/file.txt"},
	}, ExecContext{Workdir: tmpdir})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	want := "1: nested content"
	if result.Content != want {
		t.Errorf("Content:\ngot  %q\nwant %q", result.Content, want)
	}
}

func TestReadFile_Artifact(t *testing.T) {
	tests := []struct {
		name           string
		artifactArg    string
		artifactDir    string
		workdirFile    string
		artifactFile   string
		path           string
		wantContent    string
		wantError      bool
		errorSubstring string
	}{
		{
			name:         "artifact_true_reads_from_artifact_dir",
			artifactArg:  "true",
			artifactDir:  "artifact",
			workdirFile:  "workdir content",
			artifactFile: "artifact content",
			path:         "test.txt",
			wantContent:  "1: artifact content",
		},
		{
			name:           "artifact_true_no_dir_configured_error",
			artifactArg:    "true",
			artifactDir:    "",
			workdirFile:    "workdir content",
			path:           "test.txt",
			wantError:      true,
			errorSubstring: "artifact directory not configured",
		},
		{
			name:           "artifact_true_path_traversal_error",
			artifactArg:    "true",
			artifactDir:    "artifact",
			artifactFile:   "secret content",
			path:           "../escape",
			wantError:      true,
			errorSubstring: "path escapes artifact directory",
		},
		{
			name:         "artifact_false_reads_from_workdir",
			artifactArg:  "false",
			artifactDir:  "artifact",
			workdirFile:  "workdir content",
			artifactFile: "artifact content",
			path:         "test.txt",
			wantContent:  "1: workdir content",
		},
		{
			name:         "artifact_absent_reads_from_workdir",
			artifactArg:  "",
			artifactDir:  "artifact",
			workdirFile:  "workdir content",
			artifactFile: "artifact content",
			path:         "test.txt",
			wantContent:  "1: workdir content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			artifactDir := ""

			// Create workdir file
			if err := os.WriteFile(filepath.Join(tmpdir, "test.txt"), []byte(tt.workdirFile), 0o644); err != nil {
				t.Fatal(err)
			}

			// Create artifact dir and file if specified
			if tt.artifactDir != "" {
				artifactDir = filepath.Join(tmpdir, tt.artifactDir)
				if err := os.MkdirAll(artifactDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(artifactDir, "test.txt"), []byte(tt.artifactFile), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			entry := readFileEntry()
			args := map[string]string{"path": tt.path}
			if tt.artifactArg != "" {
				args["artifact"] = tt.artifactArg
			}

			result := entry.Execute(context.Background(), provider.ToolCall{
				ID:        "rf-artifact",
				Name:      "read_file",
				Arguments: args,
			}, ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir})

			if tt.wantError {
				if !result.IsError {
					t.Fatalf("expected error, got success with content: %s", result.Content)
				}
				if !strings.Contains(result.Content, tt.errorSubstring) {
					t.Errorf("Content %q should contain %q", result.Content, tt.errorSubstring)
				}
				return
			}

			if result.IsError {
				t.Fatalf("unexpected error: %s", result.Content)
			}
			if result.Content != tt.wantContent {
				t.Errorf("Content:\ngot  %q\nwant %q", result.Content, tt.wantContent)
			}
		})
	}
}
