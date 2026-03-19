package agent

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jrswab/axe/internal/toolname"
	"github.com/jrswab/axe/internal/xdg"
)

// MemoryConfig holds memory sub-configuration for an agent.
type MemoryConfig struct {
	Enabled    bool   `toml:"enabled"`
	Path       string `toml:"path"`
	LastN      int    `toml:"last_n"`
	MaxEntries int    `toml:"max_entries"`
}

// ParamsConfig holds model parameter overrides for an agent.
type ParamsConfig struct {
	Temperature float64 `toml:"temperature"`
	MaxTokens   int     `toml:"max_tokens"`
}

// RetryConfig holds retry sub-configuration for an agent.
type RetryConfig struct {
	MaxRetries     int    `toml:"max_retries"`
	Backoff        string `toml:"backoff"`
	InitialDelayMs int    `toml:"initial_delay_ms"`
	MaxDelayMs     int    `toml:"max_delay_ms"`
}

// BudgetConfig holds budget sub-configuration for an agent.
type BudgetConfig struct {
	MaxTokens int `toml:"max_tokens"`
}

// SubAgentsConfig holds sub-agent execution configuration for an agent.
type SubAgentsConfig struct {
	MaxDepth int   `toml:"max_depth"`
	Parallel *bool `toml:"parallel"`
	Timeout  int   `toml:"timeout"`
}

// MCPServerConfig holds Model Context Protocol server configuration.
type MCPServerConfig struct {
	Name      string            `toml:"name"`
	URL       string            `toml:"url"`
	Transport string            `toml:"transport"`
	Headers   map[string]string `toml:"headers"`
}

// AgentConfig represents a parsed agent TOML configuration file.
type AgentConfig struct {
	Name          string            `toml:"name"`
	Description   string            `toml:"description"`
	Model         string            `toml:"model"`
	SystemPrompt  string            `toml:"system_prompt"`
	Skill         string            `toml:"skill"`
	Files         []string          `toml:"files"`
	Workdir       string            `toml:"workdir"`
	Tools         []string          `toml:"tools"`
	MCPServers    []MCPServerConfig `toml:"mcp_servers"`
	SubAgents     []string          `toml:"sub_agents"`
	SubAgentsConf SubAgentsConfig   `toml:"sub_agents_config"`
	Memory        MemoryConfig      `toml:"memory"`
	Params        ParamsConfig      `toml:"params"`
	Retry         RetryConfig       `toml:"retry"`
	Budget        BudgetConfig      `toml:"budget"`
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
	if cfg.Memory.LastN < 0 {
		return errors.New("memory.last_n must be non-negative")
	}
	if cfg.Memory.MaxEntries < 0 {
		return errors.New("memory.max_entries must be non-negative")
	}
	validTools := toolname.ValidNames()
	for _, name := range cfg.Tools {
		if !validTools[name] {
			return fmt.Errorf("unknown tool %q in tools config", name)
		}
	}

	seenMCPNames := make(map[string]struct{}, len(cfg.MCPServers))
	for i, server := range cfg.MCPServers {
		if strings.TrimSpace(server.Name) == "" {
			return fmt.Errorf("mcp_servers[%d].name is required", i)
		}
		if strings.TrimSpace(server.URL) == "" {
			return fmt.Errorf("mcp_servers[%d].url is required", i)
		}
		parsedURL, err := url.ParseRequestURI(strings.TrimSpace(server.URL))
		if err != nil {
			return fmt.Errorf("mcp_servers[%d].url is invalid: %v", i, err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("mcp_servers[%d].url must use http or https scheme, got %q", i, parsedURL.Scheme)
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("mcp_servers[%d].url must include a host", i)
		}
		if server.Transport != "sse" && server.Transport != "streamable-http" {
			return fmt.Errorf("mcp_servers[%d].transport must be one of: sse, streamable-http", i)
		}
		if _, exists := seenMCPNames[server.Name]; exists {
			return fmt.Errorf("mcp_servers names must be unique: %q", server.Name)
		}
		seenMCPNames[server.Name] = struct{}{}
	}

	if cfg.Retry.MaxRetries < 0 {
		return errors.New("retry.max_retries must be non-negative")
	}
	if cfg.Retry.Backoff != "" && cfg.Retry.Backoff != "exponential" && cfg.Retry.Backoff != "linear" && cfg.Retry.Backoff != "fixed" {
		return errors.New("retry.backoff must be one of: exponential, linear, fixed")
	}
	if cfg.Retry.InitialDelayMs < 0 {
		return errors.New("retry.initial_delay_ms must be non-negative")
	}
	if cfg.Retry.MaxDelayMs < 0 {
		return errors.New("retry.max_delay_ms must be non-negative")
	}
	if cfg.Retry.InitialDelayMs > 0 && cfg.Retry.MaxDelayMs > 0 && cfg.Retry.MaxDelayMs < cfg.Retry.InitialDelayMs {
		return errors.New("retry.max_delay_ms must be >= retry.initial_delay_ms")
	}

	if cfg.Budget.MaxTokens < 0 {
		return errors.New("budget.max_tokens must be non-negative")
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

# Tools this agent can use (optional)
# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search
# tools = []

# Sub-agents this agent can invoke (optional)
# sub_agents = []

# [sub_agents_config]
# max_depth = 3
# parallel = true
# timeout = 120

# MCP server connections (optional)
# [[mcp_servers]]
# name = "my-tools"
# url = "https://my-mcp-server.example.com/sse"
# transport = "sse"
# headers = { Authorization = "Bearer ${MY_TOKEN}" }

# [memory]
# enabled = false
# path = ""
# last_n = 10
# max_entries = 100

# [params]
# temperature = 0.3
# max_tokens = 4096

# [retry]
# max_retries = 0
# backoff = "exponential"
# initial_delay_ms = 500
# max_delay_ms = 30000

# [budget]
# max_tokens = 0
`
	return tmpl, nil
}

// tomlDecode is a package-level wrapper for toml.Decode, used by tests.
var tomlDecode = toml.Decode
