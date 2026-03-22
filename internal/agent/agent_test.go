package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Phase 3: Validate tests ---

func TestValidate_BothFieldsMissing(t *testing.T) {
	cfg := &AgentConfig{}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
	want := "agent config missing required field: name"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MissingName(t *testing.T) {
	cfg := &AgentConfig{Model: "anthropic/claude-sonnet-4-20250514"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	want := "agent config missing required field: name"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MissingModel(t *testing.T) {
	cfg := &AgentConfig{Name: "test"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing model, got nil")
	}
	want := "agent config missing required field: model"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_EmptyNameWhitespace(t *testing.T) {
	cfg := &AgentConfig{Name: "   ", Model: "anthropic/claude-sonnet-4-20250514"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for whitespace-only name, got nil")
	}
	want := "agent config missing required field: name"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_EmptyModelWhitespace(t *testing.T) {
	cfg := &AgentConfig{Name: "test", Model: "   "}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for whitespace-only model, got nil")
	}
	want := "agent config missing required field: model"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &AgentConfig{Name: "test", Model: "anthropic/claude-sonnet-4-20250514"}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// --- Phase 4: Load tests ---

// helper to set up a temp XDG config dir with an agents/ subdirectory
func setupAgentsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	agentsDir := filepath.Join(tmpDir, "axe", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	return agentsDir
}

func writeAgentFile(t *testing.T, agentsDir, name, content string) {
	t.Helper()
	path := filepath.Join(agentsDir, name+".toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "full-agent"
description = "A full test agent"
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
	writeAgentFile(t, agentsDir, "full-agent", tomlContent)

	cfg, err := Load("full-agent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "full-agent" {
		t.Errorf("Name = %q, want %q", cfg.Name, "full-agent")
	}
	if cfg.Description != "A full test agent" {
		t.Errorf("Description = %q, want %q", cfg.Description, "A full test agent")
	}
	if cfg.Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", cfg.Model, "anthropic/claude-sonnet-4-20250514")
	}
	if cfg.SystemPrompt != "You are helpful." {
		t.Errorf("SystemPrompt = %q, want %q", cfg.SystemPrompt, "You are helpful.")
	}
	if cfg.Skill != "skills/sample/SKILL.md" {
		t.Errorf("Skill = %q, want %q", cfg.Skill, "skills/sample/SKILL.md")
	}
	if len(cfg.Files) != 2 || cfg.Files[0] != "src/**/*.go" || cfg.Files[1] != "README.md" {
		t.Errorf("Files = %v, want [src/**/*.go README.md]", cfg.Files)
	}
	if cfg.Workdir != "/tmp/work" {
		t.Errorf("Workdir = %q, want %q", cfg.Workdir, "/tmp/work")
	}
	if len(cfg.SubAgents) != 2 || cfg.SubAgents[0] != "helper" || cfg.SubAgents[1] != "reviewer" {
		t.Errorf("SubAgents = %v, want [helper reviewer]", cfg.SubAgents)
	}
	if !cfg.Memory.Enabled {
		t.Error("Memory.Enabled = false, want true")
	}
	if cfg.Memory.Path != "/tmp/memory" {
		t.Errorf("Memory.Path = %q, want %q", cfg.Memory.Path, "/tmp/memory")
	}
	if cfg.Params.Temperature != 0.7 {
		t.Errorf("Params.Temperature = %f, want 0.7", cfg.Params.Temperature)
	}
	if cfg.Params.MaxTokens != 4096 {
		t.Errorf("Params.MaxTokens = %d, want 4096", cfg.Params.MaxTokens)
	}
}

func TestLoad_MinimalConfig(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "minimal"
model = "openai/gpt-4o"
`
	writeAgentFile(t, agentsDir, "minimal", tomlContent)

	cfg, err := Load("minimal", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("Name = %q, want %q", cfg.Name, "minimal")
	}
	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "openai/gpt-4o")
	}
	if cfg.Description != "" {
		t.Errorf("Description = %q, want empty", cfg.Description)
	}
	if cfg.SystemPrompt != "" {
		t.Errorf("SystemPrompt = %q, want empty", cfg.SystemPrompt)
	}
	if cfg.Skill != "" {
		t.Errorf("Skill = %q, want empty", cfg.Skill)
	}
	if cfg.Files != nil {
		t.Errorf("Files = %v, want nil", cfg.Files)
	}
	if cfg.Workdir != "" {
		t.Errorf("Workdir = %q, want empty", cfg.Workdir)
	}
	if cfg.SubAgents != nil {
		t.Errorf("SubAgents = %v, want nil", cfg.SubAgents)
	}
	if cfg.Memory.Enabled {
		t.Error("Memory.Enabled = true, want false")
	}
	if cfg.Memory.Path != "" {
		t.Errorf("Memory.Path = %q, want empty", cfg.Memory.Path)
	}
	if cfg.Params.Temperature != 0 {
		t.Errorf("Params.Temperature = %f, want 0", cfg.Params.Temperature)
	}
	if cfg.Params.MaxTokens != 0 {
		t.Errorf("Params.MaxTokens = %d, want 0", cfg.Params.MaxTokens)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_ = setupAgentsDir(t)

	_, err := Load("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	want := "agent config not found: nonexistent"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "bad", "this is not [valid toml =")

	_, err := Load("bad", nil)
	if err == nil {
		t.Fatal("expected error for malformed TOML, got nil")
	}
	if !strings.Contains(err.Error(), `failed to parse agent config "bad"`) {
		t.Errorf("error %q does not contain expected prefix", err.Error())
	}
}

func TestLoad_MissingName(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "noname", `model = "anthropic/claude-sonnet-4-20250514"`)

	_, err := Load("noname", nil)
	if err == nil {
		t.Fatal("expected validation error for missing name, got nil")
	}
	want := "agent config missing required field: name"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestLoad_MissingModel(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "nomodel", `name = "nomodel"`)

	_, err := Load("nomodel", nil)
	if err == nil {
		t.Fatal("expected validation error for missing model, got nil")
	}
	want := "agent config missing required field: model"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestLoad_EmptyNameWhitespace(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "   "
model = "anthropic/claude-sonnet-4-20250514"
`
	writeAgentFile(t, agentsDir, "wsname", tomlContent)

	_, err := Load("wsname", nil)
	if err == nil {
		t.Fatal("expected validation error for whitespace name, got nil")
	}
	want := "agent config missing required field: name"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestLoad_EmptyModelWhitespace(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "test"
model = "   "
`
	writeAgentFile(t, agentsDir, "wsmodel", tomlContent)

	_, err := Load("wsmodel", nil)
	if err == nil {
		t.Fatal("expected validation error for whitespace model, got nil")
	}
	want := "agent config missing required field: model"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

// --- Phase 5: List tests ---

func TestList_EmptyDirectory(t *testing.T) {
	_ = setupAgentsDir(t) // creates empty agents/

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected empty slice, got %d agents", len(agents))
	}
}

func TestList_NoDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	// axe/ dir exists but no agents/ subdir
	if err := os.MkdirAll(filepath.Join(tmpDir, "axe"), 0755); err != nil {
		t.Fatalf("failed to create axe dir: %v", err)
	}

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected empty slice, got %d agents", len(agents))
	}
}

func TestList_MultipleAgents(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "alpha", `name = "alpha"`+"\n"+`model = "openai/gpt-4o"`)
	writeAgentFile(t, agentsDir, "beta", `name = "beta"`+"\n"+`model = "anthropic/claude-sonnet-4-20250514"`)

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
}

func TestList_SkipsInvalidFiles(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "good", `name = "good"`+"\n"+`model = "openai/gpt-4o"`)
	writeAgentFile(t, agentsDir, "bad", "not valid toml [[[")

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "good" {
		t.Errorf("expected agent name 'good', got %q", agents[0].Name)
	}
}

func TestList_IgnoresNonTOML(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "valid", `name = "valid"`+"\n"+`model = "openai/gpt-4o"`)
	// Write a .md file directly
	if err := os.WriteFile(filepath.Join(agentsDir, "notes.md"), []byte("# Notes"), 0644); err != nil {
		t.Fatalf("failed to write .md file: %v", err)
	}

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
}

func TestList_IgnoresSubdirectories(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	writeAgentFile(t, agentsDir, "valid", `name = "valid"`+"\n"+`model = "openai/gpt-4o"`)
	if err := os.MkdirAll(filepath.Join(agentsDir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	agents, err := List(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
}

// --- Phase 5 (M5): SubAgentsConfig tests ---

func TestLoad_SubAgentsConfig(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "parent"
model = "anthropic/claude-sonnet-4-20250514"
sub_agents = ["helper", "runner"]

[sub_agents_config]
max_depth = 4
parallel = true
timeout = 120
`
	writeAgentFile(t, agentsDir, "parent", tomlContent)

	cfg, err := Load("parent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SubAgentsConf.MaxDepth != 4 {
		t.Errorf("SubAgentsConf.MaxDepth = %d, want 4", cfg.SubAgentsConf.MaxDepth)
	}
	if cfg.SubAgentsConf.Parallel == nil || *cfg.SubAgentsConf.Parallel != true {
		t.Errorf("SubAgentsConf.Parallel = %v, want true", cfg.SubAgentsConf.Parallel)
	}
	if cfg.SubAgentsConf.Timeout != 120 {
		t.Errorf("SubAgentsConf.Timeout = %d, want 120", cfg.SubAgentsConf.Timeout)
	}
}

func TestValidate_SubAgentsConfigDefaults(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "minimal-parent"
model = "openai/gpt-4o"
sub_agents = ["helper"]
`
	writeAgentFile(t, agentsDir, "minimal-parent", tomlContent)

	cfg, err := Load("minimal-parent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SubAgentsConf.MaxDepth != 0 {
		t.Errorf("SubAgentsConf.MaxDepth = %d, want 0", cfg.SubAgentsConf.MaxDepth)
	}
	if cfg.SubAgentsConf.Parallel != nil {
		t.Errorf("SubAgentsConf.Parallel = %v, want nil (Go zero value for *bool)", cfg.SubAgentsConf.Parallel)
	}
	if cfg.SubAgentsConf.Timeout != 0 {
		t.Errorf("SubAgentsConf.Timeout = %d, want 0", cfg.SubAgentsConf.Timeout)
	}
}

func TestValidate_MaxDepthTooHigh(t *testing.T) {
	cfg := &AgentConfig{
		Name:          "test",
		Model:         "openai/gpt-4o",
		SubAgentsConf: SubAgentsConfig{MaxDepth: 6},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for max_depth=6, got nil")
	}
	want := "sub_agents_config.max_depth cannot exceed 5"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MaxDepthNegative(t *testing.T) {
	cfg := &AgentConfig{
		Name:          "test",
		Model:         "openai/gpt-4o",
		SubAgentsConf: SubAgentsConfig{MaxDepth: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for max_depth=-1, got nil")
	}
	want := "sub_agents_config.max_depth must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_TimeoutNegative(t *testing.T) {
	cfg := &AgentConfig{
		Name:          "test",
		Model:         "openai/gpt-4o",
		SubAgentsConf: SubAgentsConfig{Timeout: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for timeout=-1, got nil")
	}
	want := "sub_agents_config.timeout must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MaxDepthValid(t *testing.T) {
	cfg := &AgentConfig{
		Name:          "test",
		Model:         "openai/gpt-4o",
		SubAgentsConf: SubAgentsConfig{MaxDepth: 5},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for max_depth=5, got %v", err)
	}
}

// --- Phase 6 (M6): Memory Config tests ---

func TestLoad_MemoryConfig_AllFields(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "mem-agent"
model = "openai/gpt-4o"

[memory]
enabled = true
path = "/tmp/mem.md"
last_n = 5
max_entries = 50
`
	writeAgentFile(t, agentsDir, "mem-agent", tomlContent)

	cfg, err := Load("mem-agent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Memory.Enabled {
		t.Error("Memory.Enabled = false, want true")
	}
	if cfg.Memory.Path != "/tmp/mem.md" {
		t.Errorf("Memory.Path = %q, want %q", cfg.Memory.Path, "/tmp/mem.md")
	}
	if cfg.Memory.LastN != 5 {
		t.Errorf("Memory.LastN = %d, want 5", cfg.Memory.LastN)
	}
	if cfg.Memory.MaxEntries != 50 {
		t.Errorf("Memory.MaxEntries = %d, want 50", cfg.Memory.MaxEntries)
	}
}

func TestLoad_MemoryConfig_Defaults(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "no-mem"
model = "openai/gpt-4o"
`
	writeAgentFile(t, agentsDir, "no-mem", tomlContent)

	cfg, err := Load("no-mem", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Memory.Enabled {
		t.Error("Memory.Enabled = true, want false")
	}
	if cfg.Memory.Path != "" {
		t.Errorf("Memory.Path = %q, want empty", cfg.Memory.Path)
	}
	if cfg.Memory.LastN != 0 {
		t.Errorf("Memory.LastN = %d, want 0", cfg.Memory.LastN)
	}
	if cfg.Memory.MaxEntries != 0 {
		t.Errorf("Memory.MaxEntries = %d, want 0", cfg.Memory.MaxEntries)
	}
}

func TestValidate_MemoryLastN_Negative(t *testing.T) {
	cfg := &AgentConfig{
		Name:   "test",
		Model:  "openai/gpt-4o",
		Memory: MemoryConfig{LastN: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for last_n=-1, got nil")
	}
	want := "memory.last_n must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MemoryMaxEntries_Negative(t *testing.T) {
	cfg := &AgentConfig{
		Name:   "test",
		Model:  "openai/gpt-4o",
		Memory: MemoryConfig{MaxEntries: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for max_entries=-1, got nil")
	}
	want := "memory.max_entries must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_MemoryLastN_Zero(t *testing.T) {
	cfg := &AgentConfig{
		Name:   "test",
		Model:  "openai/gpt-4o",
		Memory: MemoryConfig{LastN: 0},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for last_n=0, got %v", err)
	}
}

func TestValidate_MemoryMaxEntries_Zero(t *testing.T) {
	cfg := &AgentConfig{
		Name:   "test",
		Model:  "openai/gpt-4o",
		Memory: MemoryConfig{MaxEntries: 0},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for max_entries=0, got %v", err)
	}
}

func TestScaffold_IncludesMemoryLastN(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "# last_n = 10") {
		t.Errorf("scaffold output missing '# last_n = 10'\nfull output:\n%s", out)
	}
}

func TestScaffold_IncludesMemoryMaxEntries(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "# max_entries = 100") {
		t.Errorf("scaffold output missing '# max_entries = 100'\nfull output:\n%s", out)
	}
}

// --- Phase 6: Scaffold tests ---

func TestScaffold_IncludesSubAgentsConfig(t *testing.T) {
	out, err := Scaffold("my-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		"# [sub_agents_config]",
		"# max_depth = 3",
		"# parallel = true",
		"# timeout = 120",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("scaffold output missing %q\nfull output:\n%s", check, out)
		}
	}
}

func TestScaffold_ContainsName(t *testing.T) {
	out, err := Scaffold("my-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `name = "my-agent"`) {
		t.Errorf("scaffold output does not contain expected name line:\n%s", out)
	}
}

func TestScaffold_ContainsModelPlaceholder(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `model = "provider/model-name"`) {
		t.Errorf("scaffold output does not contain model placeholder:\n%s", out)
	}
}

func TestScaffold_IsValidTOML(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Remove comment lines and parse
	var cleaned strings.Builder
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}

	// Replace placeholder model with a real value for valid parsing
	tomlStr := strings.Replace(cleaned.String(), `"provider/model-name"`, `"openai/gpt-4o"`, 1)

	var cfg AgentConfig
	if _, decodeErr := tomlDecode(tomlStr, &cfg); decodeErr != nil {
		t.Errorf("scaffold output (cleaned) is not valid TOML: %v\ncleaned:\n%s", decodeErr, tomlStr)
	}
}

// --- Tools field: Load tests ---

func TestLoad_WithTools(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "tooled"
model = "openai/gpt-4o"
tools = ["read_file", "list_directory"]
`
	writeAgentFile(t, agentsDir, "tooled", tomlContent)

	cfg, err := Load("tooled", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Tools) != 2 {
		t.Fatalf("Tools length = %d, want 2", len(cfg.Tools))
	}
	if cfg.Tools[0] != "read_file" {
		t.Errorf("Tools[0] = %q, want %q", cfg.Tools[0], "read_file")
	}
	if cfg.Tools[1] != "list_directory" {
		t.Errorf("Tools[1] = %q, want %q", cfg.Tools[1], "list_directory")
	}
}

func TestLoad_WithoutTools(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "no-tools"
model = "openai/gpt-4o"
`
	writeAgentFile(t, agentsDir, "no-tools", tomlContent)

	cfg, err := Load("no-tools", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tools != nil {
		t.Errorf("Tools = %v, want nil", cfg.Tools)
	}
}

func TestLoad_WithEmptyTools(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "empty-tools"
model = "openai/gpt-4o"
tools = []
`
	writeAgentFile(t, agentsDir, "empty-tools", tomlContent)

	cfg, err := Load("empty-tools", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tools == nil {
		t.Fatal("Tools is nil, want non-nil empty slice")
	}
	if len(cfg.Tools) != 0 {
		t.Errorf("Tools length = %d, want 0", len(cfg.Tools))
	}
}

// --- Tools field: Validate tests ---

func TestValidate_ValidTools(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{"read_file", "write_file"},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_UnknownTool(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{"read_file", "bogus"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	want := `unknown tool "bogus" in tools config`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_CallAgentInTools(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{"call_agent"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for call_agent in tools, got nil")
	}
	want := `unknown tool "call_agent" in tools config`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_EmptyStringTool(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{""},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty string tool, got nil")
	}
	want := `unknown tool "" in tools config`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_EmptyToolsSlice(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_NilToolsSlice(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: nil,
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_AllFiveTools(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		Tools: []string{"list_directory", "read_file", "write_file", "edit_file", "run_command"},
	}
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// --- Tools field: Scaffold tests ---

func TestScaffold_ContainsToolsComment(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "# tools = []") {
		t.Errorf("scaffold output missing '# tools = []'\nfull output:\n%s", out)
	}
}

func TestScaffold_ContainsValidToolNames(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search") {
		t.Errorf("scaffold output missing valid tool names comment\nfull output:\n%s", out)
	}
}

func TestScaffold_ToolsBeforeSubAgents(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	toolsIdx := strings.Index(out, "# tools = []")
	subAgentsIdx := strings.Index(out, "# sub_agents = []")
	if toolsIdx < 0 {
		t.Fatal("scaffold output missing '# tools = []'")
	}
	if subAgentsIdx < 0 {
		t.Fatal("scaffold output missing '# sub_agents = []'")
	}
	if toolsIdx >= subAgentsIdx {
		t.Errorf("tools section (pos %d) should appear before sub_agents section (pos %d)", toolsIdx, subAgentsIdx)
	}
}

func TestValidate_MCPServers_Valid(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "http://localhost:8080/mcp", Transport: "streamable-http"},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_MCPServers_MissingName(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "", URL: "http://localhost:8080/mcp", Transport: "sse"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `mcp_servers[0].name is required`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidate_MCPServers_MissingURL(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "", Transport: "sse"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `mcp_servers[0].url is required`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidate_MCPServers_MalformedURL(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "not a url", Transport: "sse"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mcp_servers[0].url is invalid") {
		t.Fatalf("got %q, want error containing 'mcp_servers[0].url is invalid'", err.Error())
	}
}

func TestValidate_MCPServers_InvalidScheme(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "ftp://example.com/mcp", Transport: "sse"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `mcp_servers[0].url must use http or https scheme, got "ftp"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidate_MCPServers_MissingHost(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "http:///path-only", Transport: "sse"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), "mcp_servers[0].url must include a host"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidate_MCPServers_ValidHTTPS(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "remote", URL: "https://example.com/mcp", Transport: "streamable-http"},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_MCPServers_InvalidTransport(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "http://localhost:8080/mcp", Transport: "stdio"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `mcp_servers[0].transport must be one of: sse, streamable-http`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidate_MCPServers_ValidTransports(t *testing.T) {
	tests := []struct {
		name      string
		transport string
	}{
		{name: "sse", transport: "sse"},
		{name: "streamable-http", transport: "streamable-http"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &AgentConfig{
				Name:  "test",
				Model: "openai/gpt-4o",
				MCPServers: []MCPServerConfig{
					{Name: "local", URL: "http://localhost:8080/mcp", Transport: tc.transport},
				},
			}

			if err := Validate(cfg); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidate_MCPServers_DuplicateNames(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "openai/gpt-4o",
		MCPServers: []MCPServerConfig{
			{Name: "local", URL: "http://localhost:8080/one", Transport: "sse"},
			{Name: "local", URL: "http://localhost:8080/two", Transport: "streamable-http"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), `mcp_servers names must be unique: "local"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseTOML_MCPServers(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "mcp-agent"
model = "openai/gpt-4o"

[[mcp_servers]]
name = "github"
url = "https://example.com/mcp"
transport = "streamable-http"
headers = { Authorization = "Bearer ${TOKEN}", "X-Foo" = "bar" }

[[mcp_servers]]
name = "files"
url = "https://example.com/sse"
transport = "sse"
`
	writeAgentFile(t, agentsDir, "mcp-agent", tomlContent)

	cfg, err := Load("mcp-agent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.MCPServers) != 2 {
		t.Fatalf("MCPServers length = %d, want 2", len(cfg.MCPServers))
	}
	if cfg.MCPServers[0].Name != "github" {
		t.Errorf("MCPServers[0].Name = %q, want github", cfg.MCPServers[0].Name)
	}
	if cfg.MCPServers[0].URL != "https://example.com/mcp" {
		t.Errorf("MCPServers[0].URL = %q, want https://example.com/mcp", cfg.MCPServers[0].URL)
	}
	if cfg.MCPServers[0].Transport != "streamable-http" {
		t.Errorf("MCPServers[0].Transport = %q, want streamable-http", cfg.MCPServers[0].Transport)
	}
	if len(cfg.MCPServers[0].Headers) != 2 {
		t.Fatalf("MCPServers[0].Headers length = %d, want 2", len(cfg.MCPServers[0].Headers))
	}
	if cfg.MCPServers[0].Headers["Authorization"] != "Bearer ${TOKEN}" {
		t.Errorf("MCPServers[0].Headers[Authorization] = %q, want Bearer ${TOKEN}", cfg.MCPServers[0].Headers["Authorization"])
	}
	if cfg.MCPServers[1].Name != "files" {
		t.Errorf("MCPServers[1].Name = %q, want files", cfg.MCPServers[1].Name)
	}
	if cfg.MCPServers[1].Transport != "sse" {
		t.Errorf("MCPServers[1].Transport = %q, want sse", cfg.MCPServers[1].Transport)
	}
}

func TestParseTOML_MCPServers_Empty(t *testing.T) {
	agentsDir := setupAgentsDir(t)

	tomlContent := `
name = "no-mcp"
model = "openai/gpt-4o"
`
	writeAgentFile(t, agentsDir, "no-mcp", tomlContent)

	cfg, err := Load("no-mcp", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MCPServers != nil {
		t.Errorf("MCPServers = %v, want nil", cfg.MCPServers)
	}
}

func TestScaffold_IncludesMCPServersExample(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"# MCP server connections (optional)",
		"# [[mcp_servers]]",
		"# name = \"my-tools\"",
		"# url = \"https://my-mcp-server.example.com/sse\"",
		"# transport = \"sse\"",
		"# headers = { Authorization = \"Bearer ${MY_TOKEN}\" }",
	}

	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("scaffold output missing %q", check)
		}
	}
}

// --- Retry Config tests ---

func TestValidate_RetryConfig_Valid(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			MaxRetries:     3,
			Backoff:        "exponential",
			InitialDelayMs: 500,
			MaxDelayMs:     30000,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_RetryMaxRetries_Negative(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{MaxRetries: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative max_retries, got nil")
	}
	want := "retry.max_retries must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryBackoff_Invalid(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{Backoff: "invalid"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid backoff, got nil")
	}
	want := "retry.backoff must be one of: exponential, linear, fixed"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryBackoff_WrongCase(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{Backoff: "EXPONENTIAL"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for wrong-case backoff, got nil")
	}
	want := "retry.backoff must be one of: exponential, linear, fixed"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryBackoff_Empty(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{Backoff: ""},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error for empty backoff, got %v", err)
	}
}

func TestValidate_RetryBackoff_Linear(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{Backoff: "linear"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error for linear backoff, got %v", err)
	}
}

func TestValidate_RetryBackoff_Fixed(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{Backoff: "fixed"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error for fixed backoff, got %v", err)
	}
}

func TestValidate_RetryInitialDelayMs_Negative(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{InitialDelayMs: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative initial_delay_ms, got nil")
	}
	want := "retry.initial_delay_ms must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryMaxDelayMs_Negative(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{MaxDelayMs: -1},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for negative max_delay_ms, got nil")
	}
	want := "retry.max_delay_ms must be non-negative"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryConfig_ZeroValues(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error for zero-valued retry config, got %v", err)
	}
}

func TestRetryConfig_TOMLParsing(t *testing.T) {
	input := `
name = "test"
model = "anthropic/claude-sonnet-4-20250514"

[retry]
max_retries = 3
backoff = "exponential"
initial_delay_ms = 500
max_delay_ms = 30000
`
	var cfg AgentConfig
	if _, err := tomlDecode(input, &cfg); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	if cfg.Retry.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.Retry.MaxRetries)
	}
	if cfg.Retry.Backoff != "exponential" {
		t.Errorf("Backoff = %q, want %q", cfg.Retry.Backoff, "exponential")
	}
	if cfg.Retry.InitialDelayMs != 500 {
		t.Errorf("InitialDelayMs = %d, want 500", cfg.Retry.InitialDelayMs)
	}
	if cfg.Retry.MaxDelayMs != 30000 {
		t.Errorf("MaxDelayMs = %d, want 30000", cfg.Retry.MaxDelayMs)
	}
}

func TestRetryConfig_TOMLParsing_Absent(t *testing.T) {
	input := `
name = "test"
model = "anthropic/claude-sonnet-4-20250514"
`
	var cfg AgentConfig
	if _, err := tomlDecode(input, &cfg); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	if cfg.Retry.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0", cfg.Retry.MaxRetries)
	}
	if cfg.Retry.Backoff != "" {
		t.Errorf("Backoff = %q, want empty", cfg.Retry.Backoff)
	}
	if cfg.Retry.InitialDelayMs != 0 {
		t.Errorf("InitialDelayMs = %d, want 0", cfg.Retry.InitialDelayMs)
	}
	if cfg.Retry.MaxDelayMs != 0 {
		t.Errorf("MaxDelayMs = %d, want 0", cfg.Retry.MaxDelayMs)
	}
}

func TestValidate_RetryMaxDelayMs_LessThanInitialDelayMs(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			InitialDelayMs: 500,
			MaxDelayMs:     100,
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error when max_delay_ms < initial_delay_ms, got nil")
	}
	want := "retry.max_delay_ms must be >= retry.initial_delay_ms"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestValidate_RetryMaxDelayMs_EqualToInitialDelayMs(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			InitialDelayMs: 500,
			MaxDelayMs:     500,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error when max_delay_ms == initial_delay_ms, got %v", err)
	}
}

func TestValidate_RetryMaxDelayMs_GreaterThanInitialDelayMs(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			InitialDelayMs: 500,
			MaxDelayMs:     1000,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error when max_delay_ms > initial_delay_ms, got %v", err)
	}
}

func TestValidate_RetryMaxDelayMs_ZeroWithInitialDelayMsSet(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			InitialDelayMs: 500,
			MaxDelayMs:     0,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error when max_delay_ms=0 (use default), got %v", err)
	}
}

func TestValidate_RetryInitialDelayMs_ZeroWithMaxDelayMsSet(t *testing.T) {
	cfg := &AgentConfig{
		Name:  "test",
		Model: "anthropic/claude-sonnet-4-20250514",
		Retry: RetryConfig{
			InitialDelayMs: 0,
			MaxDelayMs:     100,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error when initial_delay_ms=0 (use default), got %v", err)
	}
}

func TestScaffold_IncludesRetryConfig(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		"# [retry]",
		"# max_retries = 0",
		`# backoff = "exponential"`,
		"# initial_delay_ms = 500",
		"# max_delay_ms = 30000",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("scaffold output missing %q\nfull output:\n%s", check, out)
		}
	}
}

// --- Phase 2 (Token Budget): Budget Config tests ---

func TestValidate_BudgetMaxTokens(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *AgentConfig
		wantErr bool
		wantMsg string
	}{
		{
			name:    "negative",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Budget: BudgetConfig{MaxTokens: -1}},
			wantErr: true,
			wantMsg: "budget.max_tokens must be non-negative",
		},
		{
			name:    "zero",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Budget: BudgetConfig{MaxTokens: 0}},
			wantErr: false,
		},
		{
			name:    "positive",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Budget: BudgetConfig{MaxTokens: 10000}},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tc.wantMsg {
					t.Errorf("got %q, want %q", err.Error(), tc.wantMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestBudgetConfig_TOMLParsing(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		wantMaxTokens int
	}{
		{
			name: "with budget section",
			input: `
name = "test"
model = "anthropic/claude-sonnet-4-20250514"

[budget]
max_tokens = 5000
`,
			wantMaxTokens: 5000,
		},
		{
			name: "absent budget section",
			input: `
name = "test"
model = "anthropic/claude-sonnet-4-20250514"
`,
			wantMaxTokens: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg AgentConfig
			if _, err := tomlDecode(tc.input, &cfg); err != nil {
				t.Fatalf("failed to decode TOML: %v", err)
			}
			if cfg.Budget.MaxTokens != tc.wantMaxTokens {
				t.Errorf("Budget.MaxTokens = %d, want %d", cfg.Budget.MaxTokens, tc.wantMaxTokens)
			}
		})
	}
}

func TestLoad_BudgetConfig(t *testing.T) {
	cases := []struct {
		name          string
		tomlContent   string
		agentName     string
		wantMaxTokens int
	}{
		{
			name:      "with budget max_tokens",
			agentName: "budget-agent",
			tomlContent: `
name = "budget-agent"
model = "openai/gpt-4o"

[budget]
max_tokens = 8000
`,
			wantMaxTokens: 8000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agentsDir := setupAgentsDir(t)
			writeAgentFile(t, agentsDir, tc.agentName, tc.tomlContent)
			cfg, err := Load(tc.agentName, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Budget.MaxTokens != tc.wantMaxTokens {
				t.Errorf("Budget.MaxTokens = %d, want %d", cfg.Budget.MaxTokens, tc.wantMaxTokens)
			}
		})
	}
}

func TestScaffold_IncludesBudgetConfig(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "includes budget section comment", want: "# [budget]"},
		{name: "includes max_tokens default", want: "# max_tokens = 0"},
	}
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(out, tc.want) {
				t.Errorf("scaffold output missing %q\nfull output:\n%s", tc.want, out)
			}
		})
	}
}

// --- Artifacts Config tests ---

func TestValidate_Artifacts(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *AgentConfig
		wantErr bool
		wantMsg string
	}{
		{
			name:    "dir set with enabled false",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Artifacts: ArtifactsConfig{Enabled: false, Dir: "/tmp/artifacts"}},
			wantErr: true,
			wantMsg: "artifacts.dir is set but artifacts.enabled is false",
		},
		{
			name:    "dir contains path traversal",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Artifacts: ArtifactsConfig{Enabled: true, Dir: "../artifacts"}},
			wantErr: true,
			wantMsg: "artifacts.dir must not contain path traversal sequences",
		},
		{
			name:    "dir set with enabled true",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Artifacts: ArtifactsConfig{Enabled: true, Dir: "/tmp/artifacts"}},
			wantErr: false,
		},
		{
			name:    "no artifacts table (zero value)",
			cfg:     &AgentConfig{Name: "test", Model: "openai/gpt-4o", Artifacts: ArtifactsConfig{}},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("got %q, want error containing %q", err.Error(), tc.wantMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestScaffold_IncludesArtifactsConfig(t *testing.T) {
	out, err := Scaffold("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		"# [artifacts]",
		"# enabled = false",
		"# dir = \"\"",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("scaffold output missing %q\nfull output:\n%s", check, out)
		}
	}
}

// --- Phase 1 Multi-Directory Search Tests ---

func TestLoad_LocalDirTakesPrecedence(t *testing.T) {
	// Setup global XDG dir
	globalDir := setupAgentsDir(t)
	// Setup local dir
	localDir := filepath.Join(t.TempDir(), "local")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("failed to create local dir: %v", err)
	}

	// Write agent to global dir
	writeAgentFile(t, globalDir, "myagent", `name = "myagent"`+"\n"+`model = "openai/gpt-4o"`+"\n"+`description = "global version"`)
	// Write agent to local dir (different description)
	localAgentPath := filepath.Join(localDir, "myagent.toml")
	if err := os.WriteFile(localAgentPath, []byte(`name = "myagent"`+"\n"+`model = "openai/gpt-4o"`+"\n"+`description = "local version"`), 0644); err != nil {
		t.Fatalf("failed to write local agent file: %v", err)
	}

	// Load with local dir first - should get local version
	cfg, err := Load("myagent", []string{localDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Description != "local version" {
		t.Errorf("expected local version, got %q", cfg.Description)
	}
}

func TestLoad_FallsBackToGlobal(t *testing.T) {
	// Setup global XDG dir
	globalDir := setupAgentsDir(t)
	// Setup local dir (but don't put the agent there)
	localDir := filepath.Join(t.TempDir(), "local")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("failed to create local dir: %v", err)
	}

	// Write agent only to global dir
	writeAgentFile(t, globalDir, "myagent", `name = "myagent"`+"\n"+`model = "openai/gpt-4o"`)

	// Load with local dir first - should fall back to global
	cfg, err := Load("myagent", []string{localDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "myagent" {
		t.Errorf("expected name 'myagent', got %q", cfg.Name)
	}
}

func TestLoad_NonExistentSearchDir_Skipped(t *testing.T) {
	// Setup global XDG dir
	globalDir := setupAgentsDir(t)
	// Non-existent local dir
	nonExistentDir := filepath.Join(t.TempDir(), "doesnotexist")

	// Write agent only to global dir
	writeAgentFile(t, globalDir, "myagent", `name = "myagent"`+"\n"+`model = "openai/gpt-4o"`)

	// Load with non-existent dir first - should skip and use global
	cfg, err := Load("myagent", []string{nonExistentDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "myagent" {
		t.Errorf("expected name 'myagent', got %q", cfg.Name)
	}
}

func TestLoad_EmptyName_ReturnsError(t *testing.T) {
	_ = setupAgentsDir(t)
	_, err := Load("", nil)
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("got %q, want error containing 'must not be empty'", err.Error())
	}
}

func TestLoad_PathTraversal_ReturnsError(t *testing.T) {
	_ = setupAgentsDir(t)
	cases := []string{"../foo", "foo/bar", "foo\\bar", ".."}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(name, nil)
			if err == nil {
				t.Fatalf("expected error for name %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "must not contain path separators") {
				t.Errorf("got %q, want error containing 'must not contain path separators'", err.Error())
			}
		})
	}
}

func TestBuildSearchDirs_FlagAndBase(t *testing.T) {
	flagDir := "/custom/agents"
	baseDir := "/project"
	got := BuildSearchDirs(flagDir, baseDir)
	want := []string{"/custom/agents", "/project/axe/agents"}
	if len(got) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(got), got)
	}
	if got[0] != want[0] {
		t.Errorf("dir[0] = %q, want %q", got[0], want[0])
	}
	if got[1] != want[1] {
		t.Errorf("dir[1] = %q, want %q", got[1], want[1])
	}
}

func TestBuildSearchDirs_NoFlag(t *testing.T) {
	flagDir := ""
	baseDir := "/project"
	got := BuildSearchDirs(flagDir, baseDir)
	want := []string{"/project/axe/agents"}
	if len(got) != 1 {
		t.Fatalf("expected 1 dir, got %d: %v", len(got), got)
	}
	if got[0] != want[0] {
		t.Errorf("dir[0] = %q, want %q", got[0], want[0])
	}
}

func TestBuildSearchDirs_EmptyFlag_EmptyBase(t *testing.T) {
	flagDir := ""
	baseDir := ""
	got := BuildSearchDirs(flagDir, baseDir)
	want := "axe/agents"
	if len(got) != 1 {
		t.Fatalf("expected 1 dir, got %d: %v", len(got), got)
	}
	// filepath.Join("", "axe", "agents") returns "axe/agents" on Unix
	if got[0] != want {
		t.Errorf("dir[0] = %q, want %q", got[0], want)
	}
}

func TestAllowedHosts_TOMLParsing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHosts []string
		wantNil   bool
	}{
		{
			name:      "with allowed_hosts",
			input:     "name = \"test\"\nmodel = \"openai/gpt-4o\"\nallowed_hosts = [\"api.example.com\", \"docs.example.com\"]",
			wantHosts: []string{"api.example.com", "docs.example.com"},
		},
		{
			name:    "without allowed_hosts field",
			input:   "name = \"test\"\nmodel = \"openai/gpt-4o\"",
			wantNil: true,
		},
		{
			name:      "empty allowed_hosts",
			input:     "name = \"test\"\nmodel = \"openai/gpt-4o\"\nallowed_hosts = []",
			wantHosts: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg AgentConfig
			if _, err := tomlDecode(tt.input, &cfg); err != nil {
				t.Fatalf("unexpected decode error: %v", err)
			}
			if tt.wantNil {
				if cfg.AllowedHosts != nil {
					t.Errorf("expected nil AllowedHosts, got %v", cfg.AllowedHosts)
				}
				return
			}
			if len(cfg.AllowedHosts) != len(tt.wantHosts) {
				t.Fatalf("AllowedHosts length = %d, want %d", len(cfg.AllowedHosts), len(tt.wantHosts))
			}
			for i, got := range cfg.AllowedHosts {
				if got != tt.wantHosts[i] {
					t.Errorf("AllowedHosts[%d] = %q, want %q", i, got, tt.wantHosts[i])
				}
			}
		})
	}
}
