package runner

import "io"

// Options configures a single agent execution run.
type Options struct {
	// Agent loading
	AgentName   string   // Required: name of the agent to run
	AgentsDirs  []string // Optional: additional directories to search for agent TOML files

	// Overrides (zero value means "not overridden")
	Model        string // Override model (provider/model-name)
	Skill        string // Override skill path
	Workdir      string // Override working directory
	Prompt       string // Inline prompt (takes precedence over stdin)
	Timeout      int    // Request timeout in seconds (0 = use agent config or default)
	MaxTokens    int    // Budget max tokens override (0 = use agent config)
	ArtifactDir  string // Override artifact directory
	KeepArtifacts bool  // Preserve auto-generated artifact directories
	Stream       bool   // Enable streaming
	DryRun       bool   // Show resolved context without calling LLM
	Verbose      bool   // Print debug info to stderr
	JSON         bool   // Wrap output in JSON envelope

	// I/O (nil defaults to os.Stdout / os.Stderr / os.Stdin)
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}
