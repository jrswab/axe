# Interfaces and APIs

## Provider Interface

### Core Interface

```go
type Provider interface {
    Send(ctx context.Context, req Request) (Response, error)
}
```

### Request Structure

```go
type Request struct {
    Model       string
    System      string
    Messages    []Message
    Tools       []Tool
    Temperature float64
    MaxTokens   int
}

type Message struct {
    Role        string  // "user", "assistant"
    Content     string
    ToolCalls   []ToolCall
    ToolResults []ToolResult
}

type Tool struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
}

type ToolCall struct {
    ID    string
    Name  string
    Input map[string]interface{}
}

type ToolResult struct {
    ToolCallID string
    Content    string
    IsError    bool
}
```

### Response Structure

```go
type Response struct {
    Content      string
    StopReason   string
    ToolCalls    []ToolCall
    InputTokens  int
    OutputTokens int
}
```

### Error Handling

```go
type ProviderError struct {
    Category ErrorCategory
    Message  string
    Err      error
}

type ErrorCategory int
const (
    ErrorCategoryAuth       // 401, 403
    ErrorCategoryRateLimit  // 429
    ErrorCategoryServer     // 500+
    ErrorCategoryInput      // 400
    ErrorCategoryNetwork    // Connection errors
    ErrorCategoryUnknown    // Other
)
```

## Tool Interface

### Tool Definition

```go
type Definition struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
}

type Executor func(ctx ExecuteContext, args map[string]interface{}) (string, error)
```

### Tool Registry

```go
type Registry interface {
    Register(name string, def Definition, exec Executor)
    Has(name string) bool
    Resolve(names []string) ([]Definition, error)
    Dispatch(ctx ExecuteContext, name string, args map[string]interface{}) (string, error)
}
```

### Execute Context

```go
type ExecuteContext struct {
    Workdir      string
    CallID       string
    Verbose      bool
    AgentName    string
    AgentConfig  *agent.Agent
    Depth        int
    SubAgents    []string
    Timeout      time.Duration
}
```

## MCP Client Interface

### Client Operations

```go
type Client interface {
    Name() string
    ListTools(ctx context.Context) ([]Tool, error)
    CallTool(ctx context.Context, name string, args map[string]interface{}) ([]Content, error)
    Close() error
}
```

### MCP Tool Structure

```go
type Tool struct {
    Name        string
    Description string
    InputSchema JSONSchema
}

type Content struct {
    Type string  // "text", "image", "resource"
    Text string
}
```

### Router

```go
type Router interface {
    Register(serverName string, client Client) error
    Has(toolName string) bool
    Dispatch(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
    ServerName(toolName string) string
    Close() error
}
```

## Agent Configuration Schema

### TOML Structure

```toml
name = "agent-name"
description = "Agent description"
model = "provider/model-name"
system_prompt = "System instructions"
skill = "path/to/SKILL.md"
files = ["pattern1", "pattern2"]
workdir = "/path/to/workdir"
tools = ["read_file", "write_file"]
sub_agents = ["agent1", "agent2"]

[sub_agents_config]
max_depth = 3
parallel = true
timeout = 120

[memory]
enabled = true
last_n = 10
max_entries = 100
path = "/custom/path/memory.md"

[params]
temperature = 0.7
max_tokens = 4096

[[mcp_servers]]
name = "server-name"
transport = "stdio"
command = "/path/to/server"
args = ["--flag"]
env = {"KEY" = "value"}

[[mcp_servers]]
name = "http-server"
transport = "https"
url = "https://example.com/mcp"
headers = {"Authorization" = "Bearer ${API_KEY}"}
```

### Go Structure

```go
type Agent struct {
    Name            string
    Description     string
    Model           string
    SystemPrompt    string
    Skill           string
    Files           []string
    Workdir         string
    Tools           []string
    SubAgents       []string
    SubAgentsConfig SubAgentsConfig
    Memory          MemoryConfig
    MCPServers      []MCPServer
    Params          Params
}

type SubAgentsConfig struct {
    MaxDepth int
    Parallel bool
    Timeout  int
}

type MemoryConfig struct {
    Enabled    bool
    LastN      int
    MaxEntries int
    Path       string
}

type MCPServer struct {
    Name      string
    Transport string
    Command   string
    Args      []string
    Env       map[string]string
    URL       string
    Headers   map[string]string
}

type Params struct {
    Temperature float64
    MaxTokens   int
}
```

## CLI Interface

### Command Structure

```
axe [command] [flags]

Commands:
  run <agent>           Run an agent
  agents list           List all agents
  agents show <agent>   Show agent configuration
  agents init <agent>   Create new agent
  agents edit <agent>   Edit agent in $EDITOR
  config init           Initialize config directory
  config path           Print config path
  gc <agent>            Run garbage collection
  gc --all              Run GC on all agents
  version               Show version
```

### Run Command Flags

```
--model string       Override model (provider/model-name)
--skill string       Override skill file path
--workdir string     Override working directory
--timeout int        Request timeout in seconds (default 120)
--dry-run            Show context without calling LLM
--verbose, -v        Print debug information
--json               Output as JSON
```

### JSON Output Format

```json
{
  "agent": "agent-name",
  "model": "provider/model-name",
  "output": "Agent response text",
  "refusal_detected": false,
  "input_tokens": 1234,
  "output_tokens": 567,
  "tool_calls": [
    {
      "name": "read_file",
      "arguments": {"path": "file.txt"},
      "result": "File contents...",
      "truncated": false,
      "error": ""
    }
  ]
}
```

### Exit Codes

- `0` - Success
- `1` - Runtime error
- `2` - Configuration error

## Environment Variables

### Provider Configuration

```bash
# API Keys
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...

# Base URLs (optional overrides)
AXE_ANTHROPIC_BASE_URL=https://api.anthropic.com
AXE_OPENAI_BASE_URL=https://api.openai.com
AXE_OLLAMA_BASE_URL=http://localhost:11434
AXE_OPENCODE_BASE_URL=https://api.opencode.ai

# Web Search
AXE_WEB_SEARCH_API_KEY=...
AXE_WEB_SEARCH_BASE_URL=https://api.search.brave.com/res/v1/web/search
```

### XDG Directories

```bash
XDG_CONFIG_HOME=~/.config  # Config directory base
XDG_DATA_HOME=~/.local/share  # Data directory base
```

### Editor

```bash
EDITOR=vim  # Used by 'axe agents edit'
```

## File System Conventions

### Directory Structure

```
$XDG_CONFIG_HOME/axe/
├── config.toml           # Global configuration
├── agents/
│   ├── agent1.toml
│   └── agent2.toml
└── skills/
    ├── skill1/
    │   └── SKILL.md
    └── skill2/
        ├── SKILL.md
        └── scripts/
            └── helper.sh

$XDG_DATA_HOME/axe/
└── memory/
    ├── agent1.md
    └── agent2.md
```

### Memory File Format

```markdown
## 2026-03-12T09:32:21-05:00

**Task:**
User input or piped content

**Result:**
Agent output (truncated to 1000 chars if needed)

## 2026-03-12T10:15:30-05:00

**Task:**
Another task

**Result:**
Another result
```

## Integration Points

### Stdin Piping

```bash
git diff | axe run reviewer
cat error.log | axe run analyzer
echo "task description" | axe run agent-name
```

### Cron Integration

```cron
0 * * * * axe run hourly-task
0 0 * * * axe gc --all
```

### Git Hooks

```bash
#!/bin/sh
# .git/hooks/pre-commit
git diff --cached | axe run pre-commit-reviewer
```

### Docker

```bash
docker run --rm -i \
  -v ./config:/home/axe/.config/axe \
  -e ANTHROPIC_API_KEY \
  axe run agent-name
```

## Model String Format

Format: `provider/model-name`

**Examples:**
- `anthropic/claude-sonnet-4-20250514`
- `openai/gpt-4`
- `ollama/llama3`
- `opencode/claude-3-5-sonnet-20241022`
- `opencode/gpt-4o`
- `opencode/gemini-2.0-flash-exp`

**Provider Detection:** Split on first `/`, case-sensitive
