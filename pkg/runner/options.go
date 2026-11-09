package runner

import "io"

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

	// I/O (nil defaults to os.Stdout / os.Stderr / os.Stdin)
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}
