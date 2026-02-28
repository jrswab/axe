package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jrswab/axe/internal/xdg"
)

// MemoryConfig holds memory sub-configuration for an agent.
type MemoryConfig struct {
	Enabled bool   `toml:"enabled"`
	Path    string `toml:"path"`
}

// ParamsConfig holds model parameter overrides for an agent.
type ParamsConfig struct {
	Temperature float64 `toml:"temperature"`
	MaxTokens   int     `toml:"max_tokens"`
}

// SubAgentsConfig holds sub-agent execution configuration for an agent.
type SubAgentsConfig struct {
	MaxDepth int   `toml:"max_depth"`
	Parallel *bool `toml:"parallel"`
	Timeout  int   `toml:"timeout"`
}

// AgentConfig represents a parsed agent TOML configuration file.
type AgentConfig struct {
	Name          string          `toml:"name"`
	Description   string          `toml:"description"`
	Model         string          `toml:"model"`
	SystemPrompt  string          `toml:"system_prompt"`
	Skill         string          `toml:"skill"`
	Files         []string        `toml:"files"`
	Workdir       string          `toml:"workdir"`
	SubAgents     []string        `toml:"sub_agents"`
	SubAgentsConf SubAgentsConfig `toml:"sub_agents_config"`
	Memory        MemoryConfig    `toml:"memory"`
	Params        ParamsConfig    `toml:"params"`
}

// Validate checks that required fields are present in the agent configuration.
// It checks name first (fail-fast): if name is missing, it returns that error
// without checking model.
func Validate(cfg *AgentConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("agent config missing required field: name")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return errors.New("agent config missing required field: model")
	}
	if cfg.SubAgentsConf.MaxDepth < 0 {
		return errors.New("sub_agents_config.max_depth must be non-negative")
	}
	if cfg.SubAgentsConf.MaxDepth > 5 {
		return errors.New("sub_agents_config.max_depth cannot exceed 5")
	}
	if cfg.SubAgentsConf.Timeout < 0 {
		return errors.New("sub_agents_config.timeout must be non-negative")
	}
	return nil
}

// Load reads and parses an agent TOML configuration file by name.
// The name parameter is the agent name without the .toml extension.
func Load(name string) (*AgentConfig, error) {
	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(configDir, "agents", name+".toml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent config not found: %s", name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent config %q: %w", name, err)
	}

	var cfg AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse agent config %q: %w", name, err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// List returns all valid agent configurations from the agents directory.
// Invalid files are silently skipped. If the agents directory does not exist,
// an empty slice is returned.
func List() ([]AgentConfig, error) {
	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return nil, err
	}

	agentsDir := filepath.Join(configDir, "agents")

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []AgentConfig{}, nil
		}
		return nil, err
	}

	var agents []AgentConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		cfg, err := Load(name)
		if err != nil {
			continue // skip invalid files
		}
		agents = append(agents, *cfg)
	}

	return agents, nil
}

// Scaffold returns a TOML template string for a new agent configuration.
// The name argument is interpolated into the template.
func Scaffold(name string) (string, error) {
	tmpl := `name = "` + name + `"
description = ""

# Full provider/model per models.dev
model = "provider/model-name"

# Agent persona (optional)
# system_prompt = ""

# Default skill (optional, can be overridden with --skill flag)
# skill = ""

# Context files - glob patterns resolved from workdir or cwd (optional)
# files = []

# Working directory (optional)
# workdir = ""

# Sub-agents this agent can invoke (optional)
# sub_agents = []

# [sub_agents_config]
# max_depth = 3
# parallel = true
# timeout = 120

# [memory]
# enabled = false
# path = ""

# [params]
# temperature = 0.3
# max_tokens = 4096
`
	return tmpl, nil
}

// tomlDecode is a package-level wrapper for toml.Decode, used by tests.
var tomlDecode = toml.Decode
