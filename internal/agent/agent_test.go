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

	cfg, err := Load("full-agent")
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

	cfg, err := Load("minimal")
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

	_, err := Load("nonexistent")
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

	_, err := Load("bad")
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

	_, err := Load("noname")
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

	_, err := Load("nomodel")
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

	_, err := Load("wsname")
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

	_, err := Load("wsmodel")
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

	agents, err := List()
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

	agents, err := List()
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

	agents, err := List()
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

	agents, err := List()
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

	agents, err := List()
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

	agents, err := List()
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

	cfg, err := Load("parent")
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

	cfg, err := Load("minimal-parent")
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

	cfg, err := Load("mem-agent")
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

	cfg, err := Load("no-mem")
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
