package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: set up a temp XDG config dir with an agents/ subdirectory
func setupTestAgentsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	return agentsDir
}

func writeTestAgent(t *testing.T, agentsDir, name, content string) {
	t.Helper()
	path := filepath.Join(agentsDir, name+".toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
}

// --- Phase 7: agents parent command ---

func TestAgentsCommand_ShowsHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "agent") {
		t.Error("agents help output missing expected content")
	}
	if !strings.Contains(output, "Available Commands") {
		t.Error("agents help output missing subcommand listing")
	}
}

// --- Phase 8: agents list ---

func TestAgentsList_Empty(t *testing.T) {
	_ = setupTestAgentsDir(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected no output for empty agents dir, got %q", buf.String())
	}
}

func TestAgentsList_WithAgents(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "alpha", "name = \"alpha\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "alpha") {
		t.Errorf("output missing agent name 'alpha': %q", buf.String())
	}
}

func TestAgentsList_AlphabeticalOrder(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "zebra", "name = \"zebra\"\nmodel = \"openai/gpt-4o\"")
	writeTestAgent(t, agentsDir, "alpha", "name = \"alpha\"\nmodel = \"openai/gpt-4o\"")
	writeTestAgent(t, agentsDir, "mid", "name = \"mid\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "alpha") {
		t.Errorf("line 0 = %q, want prefix 'alpha'", lines[0])
	}
	if !strings.HasPrefix(lines[1], "mid") {
		t.Errorf("line 1 = %q, want prefix 'mid'", lines[1])
	}
	if !strings.HasPrefix(lines[2], "zebra") {
		t.Errorf("line 2 = %q, want prefix 'zebra'", lines[2])
	}
}

func TestAgentsList_WithDescription(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "helper", "name = \"helper\"\nmodel = \"openai/gpt-4o\"\ndescription = \"A helpful agent\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "helper - A helpful agent\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestAgentsList_WithoutDescription(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "bare", "name = \"bare\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "bare\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

// --- Phase 9: agents show ---

func TestAgentsShow_ValidAgent(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)

	toml := `name = "full"
description = "A full agent"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are helpful."
skill = "skills/sample/SKILL.md"
files = ["src/**/*.go", "README.md"]
workdir = "/tmp/work"
sub_agents = ["helper", "reviewer"]

[memory]
enabled = true
path = "/tmp/memory"

[params]
temperature = 0.7
max_tokens = 4096
`
	writeTestAgent(t, agentsDir, "full", toml)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "show", "full"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	checks := []string{
		"Name:",
		"full",
		"Description:",
		"A full agent",
		"Model:",
		"anthropic/claude-sonnet-4-20250514",
		"System Prompt:",
		"You are helpful.",
		"Skill:",
		"skills/sample/SKILL.md",
		"Files:",
		"src/**/*.go, README.md",
		"Workdir:",
		"/tmp/work",
		"Sub-Agents:",
		"helper, reviewer",
		"Memory Enabled:",
		"true",
		"Memory Path:",
		"/tmp/memory",
		"Temperature:",
		"0.7",
		"Max Tokens:",
		"4096",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("show output missing %q\nfull output:\n%s", check, output)
		}
	}
}

func TestAgentsShow_SubAgentsConfig(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)

	toml := `name = "parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper", "runner"]

[sub_agents_config]
max_depth = 4
parallel = true
timeout = 120
`
	writeTestAgent(t, agentsDir, "parent", toml)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "show", "parent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	checks := []string{
		"Sub-Agents:",
		"helper, runner",
		"Max Depth:",
		"4",
		"Parallel:",
		"true",
		"Timeout:",
		"120",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("show output missing %q\nfull output:\n%s", check, output)
		}
	}
}

func TestAgentsShow_NoSubAgentsConfig(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "nosubagents", "name = \"nosubagents\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "show", "nosubagents"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, absent := range []string{"Max Depth:", "Parallel:", "Timeout:"} {
		if strings.Contains(output, absent) {
			t.Errorf("show output should not contain %q when sub_agents is empty\nfull output:\n%s", absent, output)
		}
	}
}

func TestAgentsShow_MinimalAgent(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "minimal", "name = \"minimal\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "show", "minimal"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Name:") {
		t.Error("show output missing Name field")
	}
	if !strings.Contains(output, "Model:") {
		t.Error("show output missing Model field")
	}
	// Optional fields should NOT appear
	for _, absent := range []string{"Description:", "System Prompt:", "Skill:", "Files:", "Workdir:", "Sub-Agents:", "Memory Enabled:", "Memory Path:", "Temperature:", "Max Tokens:"} {
		if strings.Contains(output, absent) {
			t.Errorf("show output should not contain %q for minimal agent\nfull output:\n%s", absent, output)
		}
	}
}

func TestAgentsShow_MissingAgent(t *testing.T) {
	_ = setupTestAgentsDir(t)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "show", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
}

func TestAgentsShow_NoArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "show"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
}

// --- Phase 10: agents init ---

func TestAgentsInit_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "init", "my-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(agentsDir, "my-agent.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), `name = "my-agent"`) {
		t.Errorf("file content missing agent name:\n%s", string(data))
	}
}

func TestAgentsInit_RefusesOverwrite(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "existing", "name = \"existing\"\nmodel = \"openai/gpt-4o\"")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "init", "existing"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
	if !strings.Contains(err.Error(), "agent config already exists") {
		t.Errorf("error = %q, want to contain 'agent config already exists'", err.Error())
	}
}

func TestAgentsInit_CreatesAgentsDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	// Do NOT create the agents/ subdirectory

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "init", "new-agent"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "axe", "agents", "new-agent.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created (agents dir should have been created): %v", err)
	}
}

func TestAgentsInit_OutputIsPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"agents", "init", "path-test"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(agentsDir, "path-test.toml") + "\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestAgentsInit_NoArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "init"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
}

// --- Phase 11: agents edit ---

func TestAgentsEdit_MissingEditor(t *testing.T) {
	agentsDir := setupTestAgentsDir(t)
	writeTestAgent(t, agentsDir, "test", "name = \"test\"\nmodel = \"openai/gpt-4o\"")
	t.Setenv("EDITOR", "")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "edit", "test"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing EDITOR, got nil")
	}
	if !strings.Contains(err.Error(), "$EDITOR environment variable is not set") {
		t.Errorf("error = %q, want to contain '$EDITOR environment variable is not set'", err.Error())
	}
}

func TestAgentsEdit_MissingAgent(t *testing.T) {
	_ = setupTestAgentsDir(t)
	t.Setenv("EDITOR", "vim")

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "edit", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !strings.Contains(err.Error(), "agent config not found: nonexistent") {
		t.Errorf("error = %q, want to contain 'agent config not found: nonexistent'", err.Error())
	}
}

func TestAgentsEdit_NoArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"agents", "edit"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
}
