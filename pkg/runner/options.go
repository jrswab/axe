package runner

import "io"

// MCPServer configures a Model Context Protocol server. Its fields match the
// agent's MCP server configuration. Headers uses ${VAR} syntax; header values
// are credentials and are never printed.
type MCPServer struct {
	Name      string
	URL       string
	Transport string
	Headers   map[string]string
}

// Options configures a single agent execution run.
type Options struct {
	// Agent loading
	AgentName  string   // Required: name of the agent to run
	AgentsDirs []string // Optional: additional directories to search for agent TOML files

	// Overrides (zero value means "not overridden")
	Model         string // Override model (provider/model-name)
	Skill         string // Override skill path
	Workdir       string // Override working directory
	Prompt        string // Inline prompt (takes precedence over stdin)
	Timeout       int    // Request timeout in seconds (0 = use agent config or default)
	MaxTokens     *int   // Budget max tokens override (nil = use agent config)
	ArtifactDir   string // Override artifact directory
	KeepArtifacts bool   // Preserve auto-generated artifact directories
	Stream        bool   // Enable streaming
	DryRun        bool   // Show resolved context without calling LLM
	Verbose       bool   // Print debug info to stderr
	JSON          bool   // Wrap output in JSON envelope

	// Messages allows callers to seed the conversation with a pre-built
	// message history. When non-empty, Prompt and Stdin are ignored.
	Messages []Message

	// ToolHook decides each tool call before dispatch. It is ignored when nil
	// and is not called for dry runs (DryRun). A non-empty sub_agents list
	// with a non-nil hook is a ConfigError.
	ToolHook ToolHook

	// ToolObserver receives metadata after each turn's tool calls finish.
	// It is ignored when nil, is not called for dry runs (DryRun), and does
	// not change the result.
	ToolObserver ToolObserver

	// MCPServers adds servers merged after the agent's TOML servers in slice
	// order. A nil or empty value adds no servers. Duplicate names across or
	// within the two sources are a ConfigError. In dry runs, header variables
	// are not read, and sub-agents do not inherit the list.
	MCPServers []MCPServer

	// Tools replaces the agent's TOML built-in tool list when non-nil,
	// including an empty list. A nil value uses the agent's TOML tool list.
	// Each name is checked against the built-in set; an unknown or empty
	// name is a ConfigError. The list does not limit MCP tools or call_agent.
	// A built-in removed by the list no longer shadows a same-named MCP tool;
	// sub-agents do not inherit the list.
	Tools []string

	// MaxTurns overrides the agent's TOML max_turns when > 0. A value of 0
	// keeps the TOML value; a TOML value of 0 or unset means 50.
	// A negative value is a ConfigError in live and dry-run modes;
	// sub-agents do not inherit the value.
	MaxTurns int

	// I/O (nil defaults to os.Stdout / os.Stderr / os.Stdin)
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}
