# Components

## CLI Commands (`cmd/`)

### run.go
**Purpose:** Execute agents with LLM providers  
**Key Functions:**
- `runAgent()` - Main execution logic with conversation loop
- `executeToolCalls()` - Parallel/sequential tool execution
- `dispatchToolCall()` - Route tool calls to registry or MCP
- `mapProviderError()` - Convert provider errors to exit codes

**Features:** Dry-run mode, JSON output, verbose logging, timeout handling, memory integration

### agents.go
**Purpose:** Agent management commands  
**Subcommands:**
- `list` - Display all agents with descriptions
- `show` - Display full agent configuration
- `init` - Scaffold new agent TOML
- `edit` - Open agent in $EDITOR

### config.go
**Purpose:** Configuration initialization  
**Commands:**
- `init` - Create config directory structure and sample skill
- `path` - Print config directory path

**Embedded:** Sample skill file via `embed.FS`

### gc.go
**Purpose:** Memory garbage collection  
**Features:**
- Single agent GC with LLM-assisted pattern detection
- `--all` flag for batch processing
- `--dry-run` for preview
- Fallback to max_entries trimming

### root.go
**Purpose:** Root command and error handling  
**Exit Codes:** 0 (success), 1 (runtime error), 2 (config error)

## Internal Packages

### internal/agent
**Files:** agent.go, agent_test.go  
**Key Functions:**
- `Load()` - Parse agent TOML
- `Validate()` - Validate configuration
- `List()` - Enumerate agents directory
- `Scaffold()` - Generate template TOML

**Validation Rules:**
- Name and model required
- Tools must be valid (no call_agent in tools list)
- Sub-agent depth 1-5
- Memory last_n and max_entries must be positive
- MCP servers require name, valid transport, proper URL format

### internal/provider
**Files:** provider.go, anthropic.go, openai.go, ollama.go, opencode.go, registry.go

**Provider Interface:**
```go
type Provider interface {
    Send(ctx context.Context, req Request) (Response, error)
}
```

**Implementations:**
- **Anthropic:** Claude models, messages API, tool use blocks
- **OpenAI:** GPT models, chat completions API, function calling
- **Ollama:** Local models, chat API, tool support
- **OpenCode:** Multi-route provider (Claude/messages, GPT/responses, others/chat/completions)

**Error Categories:** Auth, RateLimit, Server, Input, Network, Unknown

**Registry:** Factory pattern for provider instantiation with base URL overrides

### internal/tool
**Files:** tool.go, registry.go, list_directory.go, read_file.go, write_file.go, edit_file.go, run_command.go, url_fetch.go, web_search.go, path_validation.go

**Tool Registry:**
- `Register()` - Add tool definition
- `Resolve()` - Get tool definitions for LLM
- `Dispatch()` - Execute tool by name
- `RegisterAll()` - Register all built-in tools

**Tool Implementations:**

| Tool | Purpose | Key Features |
|------|---------|--------------|
| list_directory | List dir contents | Relative paths only, sorted output |
| read_file | Read with pagination | Line numbers, offset/limit, binary detection |
| write_file | Create/overwrite | Creates parent dirs, byte count |
| edit_file | Find/replace | Exact match, replace_all option |
| run_command | Shell execution | Via `sh -c`, output truncation, timeout |
| url_fetch | HTTP GET | HTML stripping, truncation, per-request timeout |
| web_search | Web search API | Result truncation, API key from env |
| call_agent | Sub-agent delegation | Depth tracking, parallel execution |

**Path Validation:**
- `validatePath()` - Ensure paths stay within workdir
- Blocks absolute paths, parent traversal, symlink escapes

**Conversation Loop:**
- Max 50 turns
- Stops on text response or max turns
- Parallel tool execution by default

### internal/memory
**Files:** memory.go, memory_test.go

**Key Functions:**
- `FilePath()` - Resolve memory file path (default or custom)
- `AppendEntry()` - Add timestamped task/result entry
- `LoadEntries()` - Load last N entries
- `TrimEntries()` - Keep only last N entries
- `CountEntries()` - Count total entries

**Format:** Markdown with `## YYYY-MM-DDTHH:MM:SS±HH:MM` headers

**Features:**
- Tilde and env var expansion in paths
- Result truncation (1000 chars)
- Atomic file operations
- Preserves file permissions

### internal/resolve
**Files:** resolve.go, resolve_test.go

**Key Functions:**
- `Workdir()` - Resolve working directory (flag > TOML > cwd)
- `Skill()` - Resolve skill file path (absolute, relative, bare name)
- `Files()` - Glob pattern matching for context files
- `Stdin()` - Read piped stdin
- `BuildSystemPrompt()` - Construct full system prompt
- `ExpandPath()` - Tilde and env var expansion

**Skill Resolution:**
1. Absolute path → use as-is
2. Relative path → resolve from config dir
3. Bare name → `$XDG_CONFIG_HOME/axe/skills/{name}/SKILL.md`
4. Directory → append `/SKILL.md`

**Glob Patterns:**
- Simple globs: `*.go`, `src/*.rs`
- Double-star: `**/*.go` (recursive)
- Deduplication and sorting
- Binary file detection and skipping
- Symlink escape prevention

### internal/config
**Files:** config.go, config_test.go

**Global Config Structure:**
```go
type Config struct {
    Providers map[string]ProviderConfig
}
```

**Functions:**
- `Load()` - Parse config.toml
- `ResolveAPIKey()` - Env var > config file
- `ResolveBaseURL()` - Env var > config file
- `APIKeyEnvVar()` - Get env var name for provider

**Env Vars:** `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `AXE_*_BASE_URL`

### internal/mcpclient
**Files:** mcpclient.go, router.go, mcpclient_test.go, router_test.go

**MCPClient:**
- `Connect()` - Establish connection (HTTP/stdio)
- `ListTools()` - Discover available tools
- `CallTool()` - Execute tool with arguments
- `Close()` - Cleanup connection

**Features:**
- Header injection with env var expansion
- Type coercion for tool arguments
- Schema conversion to Axe tool format
- Transport validation (http, https, stdio)

**Router:**
- Manages multiple MCP servers
- Namespaces tools by server name
- Deduplicates clients for cleanup
- Skips built-in tool name collisions

### internal/refusal
**Files:** refusal.go, refusal_test.go

**Purpose:** Detect LLM refusals to perform tasks

**Detection Logic:**
- Checks first 500 chars
- Case-insensitive indicators: "I cannot", "I can't", "I'm not able", "I am not able"
- Requires word boundaries (not mid-word matches)
- Ignores "As an AI" without refusal indicator

### internal/xdg
**Files:** xdg.go, xdg_test.go

**Functions:**
- `GetConfigDir()` - `$XDG_CONFIG_HOME/axe` or `~/.config/axe`
- `GetDataDir()` - `$XDG_DATA_HOME/axe` or `~/.local/share/axe`

**Behavior:** Returns paths without creating directories

### internal/envinterp
**Files:** envinterp.go, envinterp_test.go

**Purpose:** Expand `${VAR}` in MCP server headers

**Function:** `ExpandHeaders()` - Replace `${VAR}` with env values

### internal/toolname
**Files:** toolname.go, toolname_test.go

**Purpose:** Centralized tool name constants

**Constants:** ListDirectory, ReadFile, WriteFile, EditFile, RunCommand, URLFetch, WebSearch, CallAgent

**Function:** `ValidNames()` - Map of valid tool names (excludes CallAgent)

### internal/testutil
**Files:** testutil.go, mockserver.go, testutil_test.go, mockserver_test.go

**Testing Utilities:**
- `SetupXDGDirs()` - Create temp XDG directories
- `SeedFixtureAgents()` - Copy test agents
- `SeedFixtureSkills()` - Copy test skills
- `SeedGlobalConfig()` - Write test config.toml
- `BuildBinary()` - Compile axe for integration tests
- `CleanupBinary()` - Remove compiled binary

**Mock Server:**
- `NewMockLLMServer()` - HTTP server for testing
- Anthropic/OpenAI response generators
- Request capture and counting
- Configurable delays and errors

## Test Organization

**Test Types:**
- Unit tests: `*_test.go` alongside implementation
- Integration tests: `cmd/*_integration_test.go`
- Smoke tests: `cmd/smoke_test.go`
- Golden file tests: `cmd/golden_test.go`

**Test Fixtures:**
- `cmd/testdata/agents/` - Sample agent configs
- `cmd/testdata/skills/` - Sample skills
- `cmd/testdata/golden/` - Expected CLI outputs

**Coverage:** 962 test functions across all packages
