# Data Models

## Agent Configuration

```go
type Agent struct {
    Name            string          // Required: Agent identifier
    Description     string          // Optional: Human-readable description
    Model           string          // Required: provider/model-name format
    SystemPrompt    string          // Optional: Base system instructions
    Skill           string          // Optional: Path to SKILL.md file
    Files           []string        // Optional: Glob patterns for context files
    Workdir         string          // Optional: Working directory (default: cwd)
    Tools           []string        // Optional: Enabled tool names
    SubAgents       []string        // Optional: Allowed sub-agent names
    SubAgentsConfig SubAgentsConfig // Optional: Sub-agent execution config
    Memory          MemoryConfig    // Optional: Memory system config
    MCPServers      []MCPServer     // Optional: MCP server configurations
    Params          Params          // Optional: Model parameters
}

type SubAgentsConfig struct {
    MaxDepth int  // Default: 3, Max: 5
    Parallel bool // Default: true
    Timeout  int  // Default: 120 seconds
}

type MemoryConfig struct {
    Enabled    bool   // Default: false
    LastN      int    // Default: 10
    MaxEntries int    // Default: 100
    Path       string // Optional: Custom memory file path
}

type MCPServer struct {
    Name      string            // Required: Server identifier
    Transport string            // Required: "stdio", "http", "https"
    Command   string            // Required for stdio
    Args      []string          // Optional: Command arguments
    Env       map[string]string // Optional: Environment variables
    URL       string            // Required for http/https
    Headers   map[string]string // Optional: HTTP headers (supports ${VAR})
}

type Params struct {
    Temperature float64 // Default: 0 (omitted if 0)
    MaxTokens   int     // Default: 0 (provider default)
}
```

## Provider Models

### Request

```go
type Request struct {
    Model       string    // provider/model-name
    System      string    // System prompt
    Messages    []Message // Conversation history
    Tools       []Tool    // Available tools
    Temperature float64   // Sampling temperature
    MaxTokens   int       // Max output tokens
}
```

### Response

```go
type Response struct {
    Content      string     // Text response
    StopReason   string     // "end_turn", "tool_use", "max_tokens"
    ToolCalls    []ToolCall // Requested tool invocations
    InputTokens  int        // Tokens in request
    OutputTokens int        // Tokens in response
}
```

### Message

```go
type Message struct {
    Role        string       // "user" or "assistant"
    Content     string       // Text content
    ToolCalls   []ToolCall   // Tools called by assistant
    ToolResults []ToolResult // Results from tool execution
}
```

### Tool

```go
type Tool struct {
    Name        string                 // Tool identifier
    Description string                 // Human-readable description
    InputSchema map[string]interface{} // JSON Schema for parameters
}
```

### ToolCall

```go
type ToolCall struct {
    ID    string                 // Unique call identifier
    Name  string                 // Tool name
    Input map[string]interface{} // Tool arguments
}
```

### ToolResult

```go
type ToolResult struct {
    ToolCallID string // Matches ToolCall.ID
    Content    string // Result text
    IsError    bool   // Whether execution failed
}
```

### ProviderError

```go
type ProviderError struct {
    Category ErrorCategory // Error classification
    Message  string        // Human-readable message
    Err      error         // Underlying error
}

type ErrorCategory int
const (
    ErrorCategoryAuth       // Authentication/authorization
    ErrorCategoryRateLimit  // Rate limiting
    ErrorCategoryServer     // Server errors
    ErrorCategoryInput      // Invalid input
    ErrorCategoryNetwork    // Network/connection errors
    ErrorCategoryUnknown    // Unclassified
)
```

## Tool Models

### Definition

```go
type Definition struct {
    Name        string                 // Tool identifier
    Description string                 // Purpose and usage
    InputSchema map[string]interface{} // JSON Schema
}
```

### ExecuteContext

```go
type ExecuteContext struct {
    Workdir     string        // Working directory for file operations
    CallID      string        // Unique call identifier for logging
    Verbose     bool          // Enable verbose logging
    AgentName   string        // Current agent name
    AgentConfig *agent.Agent  // Full agent configuration
    Depth       int           // Current sub-agent depth
    SubAgents   []string      // Allowed sub-agent names
    Timeout     time.Duration // Execution timeout
}
```

## Memory Models

### Entry Format

Markdown structure with timestamp headers:

```markdown
## 2026-03-12T09:32:21-05:00

**Task:**
[User input, piped content, or task description]

**Result:**
[Agent output, truncated to 1000 chars if needed]
```

### Memory Operations

```go
// FilePath returns resolved memory file path
func FilePath(agentName, customPath string) (string, error)

// AppendEntry adds new timestamped entry
func AppendEntry(path, task, result string) error

// LoadEntries reads last N entries
func LoadEntries(path string, lastN int) (string, error)

// TrimEntries keeps only last N entries
func TrimEntries(path string, keepN int) error

// CountEntries returns total entry count
func CountEntries(path string) (int, error)
```

## Configuration Models

### Global Config

```go
type Config struct {
    Providers map[string]ProviderConfig
}

type ProviderConfig struct {
    APIKey  string
    BaseURL string
}
```

**File Location:** `$XDG_CONFIG_HOME/axe/config.toml`

**Example:**
```toml
[providers.anthropic]
api_key = "sk-ant-..."
base_url = "https://api.anthropic.com"

[providers.openai]
api_key = "sk-..."
base_url = "https://api.openai.com"
```

## MCP Models

### MCP Tool

```go
type Tool struct {
    Name        string
    Description string
    InputSchema JSONSchema
}

type JSONSchema struct {
    Type       string
    Properties map[string]Property
    Required   []string
}

type Property struct {
    Type        string
    Description string
}
```

### MCP Content

```go
type Content struct {
    Type string // "text", "image", "resource"
    Text string // Content payload
}
```

## CLI Output Models

### JSON Output

```go
type JSONOutput struct {
    Agent           string         `json:"agent"`
    Model           string         `json:"model"`
    Output          string         `json:"output"`
    RefusalDetected bool           `json:"refusal_detected"`
    InputTokens     int            `json:"input_tokens"`
    OutputTokens    int            `json:"output_tokens"`
    ToolCalls       []ToolCallJSON `json:"tool_calls,omitempty"`
}

type ToolCallJSON struct {
    Name      string                 `json:"name"`
    Arguments map[string]interface{} `json:"arguments"`
    Result    string                 `json:"result"`
    Truncated bool                   `json:"truncated"`
    Error     string                 `json:"error,omitempty"`
}
```

### Dry-Run Output

Text format showing resolved context:

```
Agent: agent-name
Model: provider/model-name
Workdir: /path/to/workdir

System Prompt:
[Full system prompt with skill and files]

Tools:
- tool_name_1
- tool_name_2

Sub-agents:
- sub_agent_1
- sub_agent_2

MCP Servers:
- server_name (transport: stdio, command: /path/to/server)
- http_server (transport: https, url: https://example.com)
```

## Validation Rules

### Agent Validation

- `name`: Required, non-empty after trimming whitespace
- `model`: Required, non-empty after trimming whitespace, format `provider/model`
- `tools`: Must be valid tool names, cannot include "call_agent"
- `sub_agents_config.max_depth`: 1-5 (default: 3)
- `sub_agents_config.timeout`: Must be positive (default: 120)
- `memory.last_n`: Must be positive (default: 10)
- `memory.max_entries`: Must be positive (default: 100)
- `mcp_servers[].name`: Required, non-empty
- `mcp_servers[].transport`: Must be "stdio", "http", or "https"
- `mcp_servers[].command`: Required for stdio transport
- `mcp_servers[].url`: Required for http/https, must be valid URL
- `mcp_servers[].name`: Must be unique across all servers

### Tool Argument Validation

Each tool defines its own JSON Schema for arguments. Common patterns:

**Path arguments:**
- Must be relative (no leading `/`)
- No parent traversal (`..`)
- No symlink escapes outside workdir

**String arguments:**
- Typically required unless marked optional
- May have length limits (e.g., web search query)

**Numeric arguments:**
- Must be within specified ranges
- May have defaults (e.g., offset=0, limit=0 means no limit)

## Constants

### Tool Names

```go
const (
    ListDirectory = "list_directory"
    ReadFile      = "read_file"
    WriteFile     = "write_file"
    EditFile      = "edit_file"
    RunCommand    = "run_command"
    URLFetch      = "url_fetch"
    WebSearch     = "web_search"
    CallAgent     = "call_agent"
)
```

### Limits

```go
const (
    MaxConversationTurns = 50
    MaxSubAgentDepth     = 5
    DefaultSubAgentDepth = 3
    DefaultTimeout       = 120 // seconds
    MemoryResultTruncate = 1000 // characters
    URLFetchTruncate     = 50000 // characters
    CommandOutputTruncate = 10000 // characters
)
```

### Exit Codes

```go
const (
    ExitSuccess = 0
    ExitError   = 1
    ExitConfig  = 2
)
```

## Type Conversions

### Model String Parsing

```go
// Input: "anthropic/claude-sonnet-4-20250514"
// Output: provider="anthropic", model="claude-sonnet-4-20250514"

func parseModel(modelStr string) (provider, model string, err error) {
    parts := strings.SplitN(modelStr, "/", 2)
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid model format")
    }
    return parts[0], parts[1], nil
}
```

### Path Expansion

```go
// Tilde expansion: ~/path → /home/user/path
// Env var expansion: $HOME/path → /home/user/path
// Env var expansion: ${VAR}/path → /value/path

func ExpandPath(path string) string
```

### Glob Pattern Matching

```go
// Simple: *.go, src/*.rs
// Recursive: **/*.go, src/**/*.md

func Files(patterns []string, workdir string) ([]string, error)
```
