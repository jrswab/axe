package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- FilePath tests ---

func TestFilePath_Default(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	got, err := FilePath("myagent", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(tmpDir, "axe", "memory", "myagent.md")
	if got != want {
		t.Errorf("FilePath() = %q, want %q", got, want)
	}
}

func TestFilePath_CustomPath(t *testing.T) {
	got, err := FilePath("myagent", "/custom/path/mem.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/custom/path/mem.md"
	if got != want {
		t.Errorf("FilePath() = %q, want %q", got, want)
	}
}

func TestFilePath_EmptyAgentName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	got, err := FilePath("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(got, filepath.Join("memory", ".md")) {
		t.Errorf("FilePath() = %q, want suffix %q", got, filepath.Join("memory", ".md"))
	}
}

// --- AppendEntry tests ---

func TestAppendEntry_CreatesFileAndDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 2, 28, 15, 4, 5, 0, time.UTC) }
	defer func() { Now = origNow }()

	err := AppendEntry(path, "do the thing", "it worked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "## 2026-02-28T15:04:05Z") {
		t.Errorf("missing timestamp header in content:\n%s", content)
	}
	if !strings.Contains(content, "**Task:** do the thing") {
		t.Errorf("missing task in content:\n%s", content)
	}
	if !strings.Contains(content, "**Result:** it worked") {
		t.Errorf("missing result in content:\n%s", content)
	}
}

func TestAppendEntry_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "first task", "first result"); err != nil {
		t.Fatalf("first append error: %v", err)
	}

	Now = func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }

	if err := AppendEntry(path, "second task", "second result"); err != nil {
		t.Fatalf("second append error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "first task") {
		t.Errorf("missing first task in content:\n%s", content)
	}
	if !strings.Contains(content, "second task") {
		t.Errorf("missing second task in content:\n%s", content)
	}

	// Verify both entries have headers
	count := strings.Count(content, "## ")
	if count != 2 {
		t.Errorf("expected 2 entry headers, got %d", count)
	}
}

func TestAppendEntry_Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 2, 28, 15, 4, 5, 0, time.UTC) }
	defer func() { Now = origNow }()

	err := AppendEntry(path, "do the thing", "it worked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	want := "## 2026-02-28T15:04:05Z\n**Task:** do the thing\n**Result:** it worked\n\n"
	got := string(data)
	if got != want {
		t.Errorf("content mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestAppendEntry_EmptyTask(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "", "result"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(data), "**Task:** (none)") {
		t.Errorf("expected '**Task:** (none)' in content:\n%s", string(data))
	}
}

func TestAppendEntry_EmptyResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "task", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(data), "**Result:** (none)") {
		t.Errorf("expected '**Result:** (none)' in content:\n%s", string(data))
	}
}

func TestAppendEntry_ResultTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	longResult := strings.Repeat("x", 1001)
	if err := AppendEntry(path, "task", longResult); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	// The result line should contain exactly 1000 'x' chars followed by '...'
	wantResult := "**Result:** " + strings.Repeat("x", 1000) + "..."
	if !strings.Contains(content, wantResult) {
		t.Errorf("expected truncated result in content, result line not as expected")
	}
}

func TestAppendEntry_ResultExactly1000(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	exactResult := strings.Repeat("x", 1000)
	if err := AppendEntry(path, "task", exactResult); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	// Should NOT have '...' appended
	wantResult := "**Result:** " + strings.Repeat("x", 1000) + "\n"
	if !strings.Contains(content, wantResult) {
		t.Errorf("expected non-truncated result in content")
	}
	if strings.Contains(content, "...") {
		t.Errorf("result should not be truncated for exactly 1000 chars")
	}
}

func TestAppendEntry_TaskNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "line1\nline2\nline3", "result"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(data), "**Task:** line1 line2 line3") {
		t.Errorf("expected newlines replaced with spaces in task:\n%s", string(data))
	}
}

func TestAppendEntry_ResultNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "task", "line1\nline2\nline3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "**Result:** line1\nline2\nline3") {
		t.Errorf("expected newlines preserved in result:\n%s", content)
	}
}

func TestAppendEntry_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "newdir")
	path := filepath.Join(subDir, "agent.md")

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	if err := AppendEntry(path, "task", "result"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check directory permissions
	dirInfo, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("failed to stat dir: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0755 {
		t.Errorf("directory permissions = %o, want %o", dirPerm, 0755)
	}

	// Check file permissions
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	filePerm := fileInfo.Mode().Perm()
	if filePerm != 0644 {
		t.Errorf("file permissions = %o, want %o", filePerm, 0644)
	}
}

// --- LoadEntries tests ---

func TestLoadEntries_FileDoesNotExist(t *testing.T) {
	got, err := LoadEntries("/nonexistent/path/agent.md", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("LoadEntries() = %q, want empty", got)
	}
}

func TestLoadEntries_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("LoadEntries() = %q, want empty", got)
	}
}

func TestLoadEntries_AllEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "## 2026-01-01T00:00:00Z\n**Task:** t1\n**Result:** r1\n\n" +
		"## 2026-01-02T00:00:00Z\n**Task:** t2\n**Result:** r2\n\n" +
		"## 2026-01-03T00:00:00Z\n**Task:** t3\n**Result:** r3\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("LoadEntries(0) did not return all content:\ngot:\n%q\nwant:\n%q", got, content)
	}
}

func TestLoadEntries_LastN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := ""
	for i := 1; i <= 5; i++ {
		content += "## 2026-01-0" + string(rune('0'+i)) + "T00:00:00Z\n**Task:** t" + string(rune('0'+i)) + "\n**Result:** r" + string(rune('0'+i)) + "\n\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain only last 2 entries (t4 and t5)
	if !strings.Contains(got, "**Task:** t4") {
		t.Errorf("expected t4 in result:\n%s", got)
	}
	if !strings.Contains(got, "**Task:** t5") {
		t.Errorf("expected t5 in result:\n%s", got)
	}
	if strings.Contains(got, "**Task:** t3") {
		t.Errorf("did not expect t3 in result:\n%s", got)
	}
}

func TestLoadEntries_LastN_ExceedsCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "## 2026-01-01T00:00:00Z\n**Task:** t1\n**Result:** r1\n\n" +
		"## 2026-01-02T00:00:00Z\n**Task:** t2\n**Result:** r2\n\n" +
		"## 2026-01-03T00:00:00Z\n**Task:** t3\n**Result:** r3\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return all 3 entries
	if !strings.Contains(got, "**Task:** t1") {
		t.Errorf("expected t1 in result:\n%s", got)
	}
	if !strings.Contains(got, "**Task:** t3") {
		t.Errorf("expected t3 in result:\n%s", got)
	}
}

func TestLoadEntries_LastN_One(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := ""
	for i := 1; i <= 5; i++ {
		content += "## 2026-01-0" + string(rune('0'+i)) + "T00:00:00Z\n**Task:** t" + string(rune('0'+i)) + "\n**Result:** r" + string(rune('0'+i)) + "\n\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "**Task:** t5") {
		t.Errorf("expected only t5 in result:\n%s", got)
	}
	if strings.Contains(got, "**Task:** t4") {
		t.Errorf("did not expect t4 in result:\n%s", got)
	}
}

func TestLoadEntries_PreservesFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "## 2026-02-28T15:04:05Z\n**Task:** do the thing\n**Result:** it worked\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := LoadEntries(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != content {
		t.Errorf("format not preserved:\ngot:\n%q\nwant:\n%q", got, content)
	}
}

func TestLoadEntries_ContentBeforeFirstEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "Some preamble text\n\n## 2026-01-01T00:00:00Z\n**Task:** t1\n**Result:** r1\n\n## 2026-01-02T00:00:00Z\n**Task:** t2\n**Result:** r2\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// lastN=1 should return only the last entry, excluding preamble
	got1, err := LoadEntries(path, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got1, "preamble") {
		t.Errorf("lastN=1 should not include preamble:\n%s", got1)
	}
	if !strings.Contains(got1, "**Task:** t2") {
		t.Errorf("lastN=1 should include last entry:\n%s", got1)
	}

	// lastN=0 should return all content including preamble
	got0, err := LoadEntries(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got0 != content {
		t.Errorf("lastN=0 should return all content:\ngot:\n%q\nwant:\n%q", got0, content)
	}
}

// --- TrimEntries tests ---

// helper: generate N memory entries as a string.
func generateEntries(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## 2026-01-%02dT00:00:00Z\n**Task:** task%d\n**Result:** result%d\n\n", i, i, i)
	}
	return b.String()
}

func TestTrimEntries_FileDoesNotExist(t *testing.T) {
	removed, err := TrimEntries("/nonexistent/path/agent.md", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}
}

func TestTrimEntries_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %q", string(data))
	}
}

func TestTrimEntries_KeepNZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(5)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged:\ngot:\n%q\nwant:\n%q", string(data), content)
	}
}

func TestTrimEntries_KeepNNegative(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	_, err := TrimEntries(path, -1)
	if err == nil {
		t.Fatal("expected error for negative keepN, got nil")
	}
	if !strings.Contains(err.Error(), "keepN must be non-negative") {
		t.Errorf("expected 'keepN must be non-negative' error, got: %v", err)
	}
}

func TestTrimEntries_EntriesWithinLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(3)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged:\ngot:\n%q\nwant:\n%q", string(data), content)
	}
}

func TestTrimEntries_EntriesEqualToLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(5)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged:\ngot:\n%q\nwant:\n%q", string(data), content)
	}
}

func TestTrimEntries_TrimsOldEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(10)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Get what LoadEntries would return for the last 3 before trimming
	expected, err := LoadEntries(path, 3)
	if err != nil {
		t.Fatalf("failed to load entries: %v", err)
	}

	removed, err := TrimEntries(path, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 7 {
		t.Errorf("TrimEntries() removed = %d, want 7", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != expected {
		t.Errorf("trimmed file not byte-identical to LoadEntries(path, 3):\ngot:\n%q\nwant:\n%q", string(data), expected)
	}
}

func TestTrimEntries_PreservesEntryFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	// Multi-line entries with varied formatting
	content := "## 2026-01-01T00:00:00Z\n**Task:** task1\n**Result:** line1\nline2\nline3\n\n" +
		"## 2026-01-02T00:00:00Z\n**Task:** task2\n**Result:** result with   extra   spaces\n\n" +
		"## 2026-01-03T00:00:00Z\n**Task:** task3\n**Result:** result3\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	expected, err := LoadEntries(path, 2)
	if err != nil {
		t.Fatalf("failed to load entries: %v", err)
	}

	removed, err := TrimEntries(path, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("TrimEntries() removed = %d, want 1", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != expected {
		t.Errorf("format not preserved:\ngot:\n%q\nwant:\n%q", string(data), expected)
	}
}

func TestTrimEntries_DiscardsPreHeaderContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "Some preamble text\nAnother line\n\n" + generateEntries(5)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 2 {
		t.Errorf("TrimEntries() removed = %d, want 2", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	result := string(data)

	if strings.Contains(result, "preamble") {
		t.Errorf("pre-header content should be discarded, got:\n%s", result)
	}
	if !strings.Contains(result, "task3") || !strings.Contains(result, "task4") || !strings.Contains(result, "task5") {
		t.Errorf("expected last 3 entries, got:\n%s", result)
	}
	if strings.Contains(result, "task1") || strings.Contains(result, "task2") {
		t.Errorf("should not contain trimmed entries, got:\n%s", result)
	}
}

func TestTrimEntries_SingleEntryKeepOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(1)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	removed, err := TrimEntries(path, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("TrimEntries() removed = %d, want 0", removed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged:\ngot:\n%q\nwant:\n%q", string(data), content)
	}
}

func TestTrimEntries_OriginalUnmodifiedOnWriteError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := generateEntries(5)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Make directory read-only to prevent temp file creation
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}
	defer os.Chmod(dir, 0755) // restore for cleanup

	_, err := TrimEntries(path, 2)
	if err == nil {
		t.Fatal("expected error when directory is read-only, got nil")
	}

	// Restore permissions to read the file
	os.Chmod(dir, 0755)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("original file should be unmodified:\ngot:\n%q\nwant:\n%q", string(data), content)
	}
}

// --- CountEntries tests ---

func TestCountEntries_FileDoesNotExist(t *testing.T) {
	got, err := CountEntries("/nonexistent/path/agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("CountEntries() = %d, want 0", got)
	}
}

func TestCountEntries_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := CountEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("CountEntries() = %d, want 0", got)
	}
}

func TestCountEntries_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := ""
	for i := 1; i <= 5; i++ {
		content += "## 2026-01-0" + string(rune('0'+i)) + "T00:00:00Z\n**Task:** t\n**Result:** r\n\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := CountEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5 {
		t.Errorf("CountEntries() = %d, want 5", got)
	}
}

func TestCountEntries_NoEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	content := "Just some plain text\nwith no entry headers\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	got, err := CountEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("CountEntries() = %d, want 0", got)
	}
}
