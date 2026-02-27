package cmd

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

//go:embed testdata/skills/sample/SKILL.md
var testSkillsRawFS embed.FS

func testSkillsFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(testSkillsRawFS, "testdata")
	if err != nil {
		t.Fatalf("failed to create sub FS: %v", err)
	}
	return sub
}

func TestConfigPathCommand_WithXDG(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "path"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := filepath.Join(tmpDir, "axe") + "\n"
	if got != want {
		t.Errorf("config path output = %q, want %q", got, want)
	}
}

func TestConfigPathCommand_WithoutXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "path"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userCfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() failed: %v", err)
	}

	got := buf.String()
	want := filepath.Join(userCfg, "axe") + "\n"
	if got != want {
		t.Errorf("config path output = %q, want %q", got, want)
	}
}

func TestConfigInitCommand_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configDir := filepath.Join(tmpDir, "axe")

	// Check agents/ directory exists
	agentsDir := filepath.Join(configDir, "agents")
	info, err := os.Stat(agentsDir)
	if err != nil {
		t.Fatalf("agents/ directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("agents/ is not a directory")
	}

	// Check skills/sample/ directory exists
	skillsDir := filepath.Join(configDir, "skills", "sample")
	info, err = os.Stat(skillsDir)
	if err != nil {
		t.Fatalf("skills/sample/ directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("skills/sample/ is not a directory")
	}

	// Check SKILL.md was copied
	skillFile := filepath.Join(skillsDir, "SKILL.md")
	_, err = os.Stat(skillFile)
	if err != nil {
		t.Fatalf("SKILL.md not copied: %v", err)
	}

	// Verify output is the config path
	got := buf.String()
	want := configDir + "\n"
	if got != want {
		t.Errorf("config init output = %q, want %q", got, want)
	}
}

func TestConfigInitCommand_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	SetSkillsFS(testSkillsFS(t))

	// Run first time
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// Run second time — should succeed silently
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}
}

func TestConfigInitCommand_DoesNotOverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	SetSkillsFS(testSkillsFS(t))

	// Run init first
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// Modify the SKILL.md file
	skillFile := filepath.Join(tmpDir, "axe", "skills", "sample", "SKILL.md")
	customContent := []byte("custom content")
	if err := os.WriteFile(skillFile, customContent, 0644); err != nil {
		t.Fatalf("failed to write custom content: %v", err)
	}

	// Run init again
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// Verify custom content was NOT overwritten
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if string(data) != "custom content" {
		t.Errorf("SKILL.md was overwritten: got %q", string(data))
	}
}

func TestConfigInitCommand_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	// Create a read-only directory so MkdirAll fails
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", readOnlyDir)

	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for permission denied, got nil")
	}
}

// --- M4: config.toml tests ---

func TestConfigInit_CreatesConfigTOML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configTOML := filepath.Join(tmpDir, "axe", "config.toml")
	if _, err := os.Stat(configTOML); err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}
}

func TestConfigInit_ConfigTOMLPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configTOML := filepath.Join(tmpDir, "axe", "config.toml")
	info, err := os.Stat(configTOML)
	if err != nil {
		t.Fatalf("failed to stat config.toml: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestConfigInit_ConfigTOMLContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configTOML := filepath.Join(tmpDir, "axe", "config.toml")
	data, err := os.ReadFile(configTOML)
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}
	content := string(data)
	for _, expected := range []string{"providers.anthropic", "providers.openai", "providers.ollama"} {
		if !strings.Contains(content, expected) {
			t.Errorf("config.toml missing %q", expected)
		}
	}
}

func TestConfigInit_DoesNotOverwriteConfigTOML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	SetSkillsFS(testSkillsFS(t))

	// Create a custom config.toml first
	configDir := filepath.Join(tmpDir, "axe")
	os.MkdirAll(configDir, 0755)
	customContent := `[providers.anthropic]
api_key = "my-secret-key"
`
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(customContent), 0600)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify not overwritten
	data, err := os.ReadFile(filepath.Join(configDir, "config.toml"))
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("config.toml was overwritten: got %q", string(data))
	}
}

func TestConfigInitCommand_CopiesSkillContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	SetSkillsFS(testSkillsFS(t))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"config", "init"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skillFile := filepath.Join(tmpDir, "axe", "skills", "sample", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}

	content := string(data)
	// Verify the template contains the required sections per spec
	if !strings.Contains(content, "# Sample Skill") {
		t.Error("SKILL.md missing title header")
	}
	if !strings.Contains(content, "## Purpose") {
		t.Error("SKILL.md missing Purpose section")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("SKILL.md missing Instructions section")
	}
	if !strings.Contains(content, "## Output Format") {
		t.Error("SKILL.md missing Output Format section")
	}
}
