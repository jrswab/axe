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

// ValidationError represents an agent configuration validation failure.
type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string {
	return e.msg
}

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
		return &ValidationError{msg: "agent config missing required field: name"}
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return &ValidationError{msg: "agent config missing required field: model"}
	}
	if cfg.SubAgentsConf.MaxDepth < 0 {
		return &ValidationError{msg: "sub_agents_config.max_depth must be non-negative"}
	}
	if cfg.SubAgentsConf.MaxDepth > 5 {
		return &ValidationError{msg: "sub_agents_config.max_depth cannot exceed 5"}
	}
	if cfg.SubAgentsConf.Timeout < 0 {
		return &ValidationError{msg: "sub_agents_config.timeout must be non-negative"}
	}
	if cfg.Memory.LastN < 0 {
		return &ValidationError{msg: "memory.last_n must be non-negative"}
	}
	if cfg.Memory.MaxEntries < 0 {
		return &ValidationError{msg: "memory.max_entries must be non-negative"}
	}
	validTools := toolname.ValidNames()
	for _, name := range cfg.Tools {
		if !validTools[name] {
			return &ValidationError{msg: fmt.Sprintf("unknown tool %q in tools config", name)}
		}
	}

	seenMCPNames := make(map[string]struct{}, len(cfg.MCPServers))
	for i, server := range cfg.MCPServers {
		if strings.TrimSpace(server.Name) == "" {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].name is required", i)}
		}
		if strings.TrimSpace(server.URL) == "" {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].url is required", i)}
		}
		parsedURL, err := url.ParseRequestURI(strings.TrimSpace(server.URL))
		if err != nil {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].url is invalid: %v", i, err)}
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].url must use http or https scheme, got %q", i, parsedURL.Scheme)}
		}
		if parsedURL.Host == "" {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].url must include a host", i)}
		}
		if server.Transport != "sse" && server.Transport != "streamable-http" {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers[%d].transport must be one of: sse, streamable-http", i)}
		}
		if _, exists := seenMCPNames[server.Name]; exists {
			return &ValidationError{msg: fmt.Sprintf("mcp_servers names must be unique: %q", server.Name)}
		}
		seenMCPNames[server.Name] = struct{}{}
	}

	if cfg.Retry.MaxRetries < 0 {
		return &ValidationError{msg: "retry.max_retries must be non-negative"}
	}
	if cfg.Retry.Backoff != "" && cfg.Retry.Backoff != "exponential" && cfg.Retry.Backoff != "linear" && cfg.Retry.Backoff != "fixed" {
		return &ValidationError{msg: "retry.backoff must be one of: exponential, linear, fixed"}
	}
	if cfg.Retry.InitialDelayMs < 0 {
		return &ValidationError{msg: "retry.initial_delay_ms must be non-negative"}
	}
	if cfg.Retry.MaxDelayMs < 0 {
		return &ValidationError{msg: "retry.max_delay_ms must be non-negative"}
	}
	if cfg.Retry.InitialDelayMs > 0 && cfg.Retry.MaxDelayMs > 0 && cfg.Retry.MaxDelayMs < cfg.Retry.InitialDelayMs {
		return &ValidationError{msg: "retry.max_delay_ms must be >= retry.initial_delay_ms"}
	}

	if cfg.Budget.MaxTokens < 0 {
		return &ValidationError{msg: "budget.max_tokens must be non-negative"}
	}

	return nil
}

// loadFromPath reads and parses an agent TOML configuration file from a specific path.
// It returns the raw read/parse/validate errors (without any wrapping) so the caller
// can handle them appropriately.
func loadFromPath(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg AgentConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, err
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Load reads and parses an agent TOML configuration file by name.
// The name parameter is the agent name without the .toml extension.
// searchDirs is a list of directories to search first, before falling back to the global XDG config dir.
func Load(name string, searchDirs []string) (*AgentConfig, error) {
	// Validate name: must be non-empty and must not contain path separators or traversal sequences
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("agent name must not be empty")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("agent name %q must not contain path separators or traversal sequences", name)
	}
	for _, dir := range searchDirs {
		path := filepath.Join(dir, name+".toml")
		cfg, err := loadFromPath(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // file doesn't exist in this dir, try next
			}
			// File exists but failed to load (parse error, validation error, etc.)
			// Check if it's a validation error - pass through unchanged
			var valErr *ValidationError
			if errors.As(err, &valErr) {
				return nil, err
			}
			// Wrap other errors with context
			return nil, fmt.Errorf("failed to parse agent config %q: %w", name, err)
		}
		return cfg, nil
	}

	// Fall back to global XDG config dir
	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "agents", name+".toml")
	cfg, err := loadFromPath(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("agent config not found: %s", name)
		}
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to parse agent config %q: %w", name, err)
	}
	return cfg, nil
}

// List returns all valid agent configurations from the agents directory.
// Invalid files are silently skipped. If the agents directory does not exist,
// an empty slice is returned.
// searchDirs is a list of directories to search first, before falling back to the global XDG config dir.
func List(searchDirs []string) ([]AgentConfig, error) {
	seen := make(map[string]struct{})
	var agents []AgentConfig

	// Collect all directories to search: searchDirs first, then global XDG dir
	var allDirs []string
	allDirs = append(allDirs, searchDirs...)

	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return nil, err
	}
	allDirs = append(allDirs, filepath.Join(configDir, "agents"))

	// Process directories in order (earlier = higher precedence)
	for _, agentsDir := range allDirs {
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			// Silently skip non-existent directories
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".toml") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".toml")

			// Skip if we've already seen this agent name (first occurrence wins)
			if _, exists := seen[name]; exists {
				continue
			}

			// Try to load from this specific path
			path := filepath.Join(agentsDir, entry.Name())
			cfg, err := loadFromPath(path)
			if err != nil {
				continue // skip invalid files
			}

			seen[name] = struct{}{}
			agents = append(agents, *cfg)
		}
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

// BuildSearchDirs builds the search directories slice from a flag value and base directory.
// If flagDir is non-empty, it is appended as the first element.
// The auto-discovery path (baseDir/axe/agents) is always appended.
func BuildSearchDirs(flagDir string, baseDir string) []string {
	var dirs []string
	if flagDir != "" {
		dirs = append(dirs, flagDir)
	}
	dirs = append(dirs, filepath.Join(baseDir, "axe", "agents"))
	return dirs
}

// tomlDecode is a package-level wrapper for toml.Decode, used by tests.
var tomlDecode = toml.Decode
