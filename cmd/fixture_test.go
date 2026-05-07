package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/jrswab/axe/internal/agent"
)

func TestFixtureAgents_AllParseAndValidate(t *testing.T) {
	agentsDir := "testdata/agents"
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		t.Fatalf("failed to read agents dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(agentsDir, entry.Name()))
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			var cfg agent.AgentConfig
			if _, err := toml.Decode(string(data), &cfg); err != nil {
				t.Fatalf("failed to parse %s: %v", entry.Name(), err)
			}

			if err := agent.Validate(&cfg); err != nil {
				t.Fatalf("validation failed for %s: %v", entry.Name(), err)
			}
		})
	}
}

func TestFixtureAgents_BasicHasOnlyRequiredFields(t *testing.T) {
	data, err := os.ReadFile("testdata/agents/basic.toml")
	if err != nil {
		t.Fatalf("failed to read basic.toml: %v", err)
	}

	var cfg agent.AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		t.Fatalf("failed to parse basic.toml: %v", err)
	}

	if cfg.Name != "basic" {
		t.Errorf("Name = %q, want %q", cfg.Name, "basic")
	}
	if cfg.Model != "openrouter/openai/gpt-4o-mini" {
		t.Errorf("Model = %q, want %q", cfg.Model, "openrouter/openai/gpt-4o-mini")
	}

	// Verify all optional fields are zero-valued
	if cfg.Description != "" {
		t.Errorf("Description = %q, want empty", cfg.Description)
	}
	if cfg.Skill != "" {
		t.Errorf("Skill = %q, want empty", cfg.Skill)
	}
	if cfg.Files != nil {
		t.Errorf("Files = %v, want nil", cfg.Files)
	}
	if cfg.SubAgents != nil {
		t.Errorf("SubAgents = %v, want nil", cfg.SubAgents)
	}
	if cfg.Memory.Enabled {
		t.Error("Memory.Enabled = true, want false")
	}
}

func TestFixtureAgents_WithSkillReferencesStubSkill(t *testing.T) {
	data, err := os.ReadFile("testdata/agents/with_skill.toml")
	if err != nil {
		t.Fatalf("failed to read with_skill.toml: %v", err)
	}

	var cfg agent.AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		t.Fatalf("failed to parse with_skill.toml: %v", err)
	}

	if cfg.Skill != "skills/stub/SKILL.md" {
		t.Errorf("Skill = %q, want %q", cfg.Skill, "skills/stub/SKILL.md")
	}
}

func TestFixtureAgents_WithMemoryConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/agents/with_memory.toml")
	if err != nil {
		t.Fatalf("failed to read with_memory.toml: %v", err)
	}

	var cfg agent.AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		t.Fatalf("failed to parse with_memory.toml: %v", err)
	}

	if !cfg.Memory.Enabled {
		t.Error("Memory.Enabled = false, want true")
	}
	if cfg.Memory.LastN != 5 {
		t.Errorf("Memory.LastN = %d, want 5", cfg.Memory.LastN)
	}
	if cfg.Memory.MaxEntries != 50 {
		t.Errorf("Memory.MaxEntries = %d, want 50", cfg.Memory.MaxEntries)
	}
	if cfg.Memory.Path != "" {
		t.Errorf("Memory.Path = %q, want empty", cfg.Memory.Path)
	}
}

func TestFixtureAgents_WithSubagentsConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/agents/with_subagents.toml")
	if err != nil {
		t.Fatalf("failed to read with_subagents.toml: %v", err)
	}

	var cfg agent.AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		t.Fatalf("failed to parse with_subagents.toml: %v", err)
	}

	// Verify sub_agents list
	if len(cfg.SubAgents) != 2 {
		t.Fatalf("SubAgents length = %d, want 2", len(cfg.SubAgents))
	}
	if cfg.SubAgents[0] != "basic" {
		t.Errorf("SubAgents[0] = %q, want %q", cfg.SubAgents[0], "basic")
	}
	if cfg.SubAgents[1] != "with_skill" {
		t.Errorf("SubAgents[1] = %q, want %q", cfg.SubAgents[1], "with_skill")
	}

	// Verify sub_agents_config
	if cfg.SubAgentsConf.MaxDepth != 3 {
		t.Errorf("SubAgentsConf.MaxDepth = %d, want 3", cfg.SubAgentsConf.MaxDepth)
	}
	if cfg.SubAgentsConf.Parallel == nil {
		t.Fatal("SubAgentsConf.Parallel is nil, want non-nil")
	}
	if !*cfg.SubAgentsConf.Parallel {
		t.Error("SubAgentsConf.Parallel = false, want true")
	}
	if cfg.SubAgentsConf.Timeout != 60 {
		t.Errorf("SubAgentsConf.Timeout = %d, want 60", cfg.SubAgentsConf.Timeout)
	}
}

func TestFixtureSkills_StubSkillExists(t *testing.T) {
	data, err := os.ReadFile("testdata/skills/stub/SKILL.md")
	if err != nil {
		t.Fatalf("stub SKILL.md not found: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "# Stub Skill") {
		t.Error("stub SKILL.md missing title header")
	}
	if !strings.Contains(content, "## Purpose") {
		t.Error("stub SKILL.md missing Purpose section")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("stub SKILL.md missing Instructions section")
	}
	if !strings.Contains(content, "## Output Format") {
		t.Error("stub SKILL.md missing Output Format section")
	}
}

func TestFixtureAgents_WithToolsConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/agents/with_tools.toml")
	if err != nil {
		t.Fatalf("failed to read with_tools.toml: %v", err)
	}

	var cfg agent.AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		t.Fatalf("failed to parse with_tools.toml: %v", err)
	}

	if err := agent.Validate(&cfg); err != nil {
		t.Fatalf("validation failed for with_tools.toml: %v", err)
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
