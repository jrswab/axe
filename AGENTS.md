# Axe — AGENTS.md

## What This Is

A lightweight CLI for running single-purpose LLM agents. Think `make` for AI — define agents in TOML, give them focused context via SKILL.md files, and trigger them from anywhere (cron, git hooks, pipes, file watchers).

## Design Philosophy

- **Small context windows by design.** Each agent gets only what it needs. Sub-agents return results, not conversations. This is the core value prop — fight context bloat at every level.
- **Unix citizen.** Stdout is clean and pipeable. Debug goes to stderr. Exit codes are meaningful. Stdin is always accepted. Axe composes with existing tools, it doesn't replace them.
- **Executor, not scheduler.** Axe runs agents. Triggering is the user's job (cron, entr, git hooks, webhooks). No built-in daemon, no watch mode, no event loop.
- **Single binary, zero runtime.** Go. No interpreters, no node_modules, no Docker required. `go build` and ship.
- **Config over code.** Agent definitions are TOML, not Go. Users should never touch source to define or modify agents.

## Non-Obvious Constraints

- **SKILL.md is a community format** — not invented here. Treat it as an external standard. Don't add axe-specific extensions to the format itself; put axe-specific config in TOML.
- **models.dev format for model strings** — always `provider/model-name`. Don't invent a custom format.
- **XDG Base Directory spec** — config, data, and cache go where XDG says. No dotfiles in $HOME.
- **Sub-agents are opaque to parents.** A parent never sees a sub-agent's internal turns, tool calls, or files. Only the final text result crosses the boundary. This is intentional — don't leak internals upward.
- **Depth limits are safety rails, not features.** Default 3, hard max 5. If someone needs more depth, the design is wrong, not the limit.

## What Good Contributions Look Like

- Tests for every new package (table-driven, no mocks when avoidable — use the real thing or `testutil` helpers)
- Errors that help the user fix the problem, not just describe it
- No global state — pass dependencies explicitly
- Flags override TOML overrides defaults. Resolution order matters everywhere.
- When in doubt, print nothing. Axe output MUST be safe to pipe.

## Directory Structure

```text
axe/
├── cmd/                    # CLI commands (run, agents, config, gc, version)
├── internal/
│   ├── agent/             # Agent TOML loading and validation
│   ├── config/            # Global config.toml handling
│   ├── envinterp/         # ${VAR} expansion for MCP headers
│   ├── mcpclient/         # Model Context Protocol client
│   ├── memory/            # Persistent memory system
│   ├── provider/          # LLM provider implementations (Anthropic, OpenAI, Ollama, OpenCode, Bedrock)
│   ├── refusal/           # LLM refusal detection
│   ├── resolve/           # Context resolution (workdir, skill, files, stdin)
│   ├── testutil/          # Test helpers and mock servers
│   ├── tool/              # Built-in tools (file ops, shell, web)
│   ├── toolname/          # Tool name constants
│   └── xdg/               # XDG Base Directory support
├── docs/
│   ├── design/            # Architecture docs
│   └── plans/             # Implementation specs
├── examples/              # Example agents (code-reviewer, commit-msg, summarizer)
└── main.go
```

## Key Entry Points

### CLI Commands (`cmd/`)

- **run.go** - Main execution: loads agent, resolves context, calls provider, handles conversation loop (max 50 turns), executes tools (parallel by default), manages memory
- **agents.go** - Agent management: list, show, init (scaffold), edit (opens $EDITOR)
- **config.go** - Config init (creates XDG dirs, embeds sample skill), path display
- **gc.go** - Memory garbage collection with LLM-assisted pattern detection or fallback to max_entries
- **root.go** - Root command, exit code mapping (0=success, 1=runtime error, 2=config error)

### Core Packages

- **internal/agent** - `Load()` parses TOML, `Validate()` checks required fields and constraints, `Scaffold()` generates template
- **internal/provider** - `Provider` interface with `Send(ctx, Request) (Response, error)`. Implementations: Anthropic (messages API), OpenAI (chat completions), Ollama (local), OpenCode (multi-route: Claude/messages, GPT/responses, others/chat/completions), Bedrock (AWS Converse API)
- **internal/tool** - Registry pattern. Built-in tools: list_directory, read_file (with pagination), write_file, edit_file (find/replace), run_command (via sh -c), url_fetch (HTML stripping), web_search, call_agent (sub-agent delegation). All file tools sandboxed to workdir.
- **internal/resolve** - Resolution order: flags > TOML > env vars > defaults. Handles tilde/env var expansion, glob patterns (simple and **), binary file detection, symlink escape prevention.
- **internal/memory** - Timestamped markdown entries. `AppendEntry()` adds task/result, `LoadEntries()` loads last N, `TrimEntries()` keeps last N, `CountEntries()` returns total.
- **internal/mcpclient** - MCP client for stdio/HTTP transports. `Connect()`, `ListTools()`, `CallTool()`. Router manages multiple servers, namespaces tools by server name.

## Configuration

### Agent TOML (`$XDG_CONFIG_HOME/axe/agents/*.toml`)

```toml
name = "agent-name"              # Required
model = "provider/model-name"    # Required (e.g., anthropic/claude-sonnet-4-20250514)
description = "..."              # Optional
system_prompt = "..."            # Optional
skill = "path/to/SKILL.md"       # Optional (absolute, relative to config dir, or bare name)
files = ["**/*.go", "README.md"] # Optional (glob patterns)
workdir = "/path/to/dir"         # Optional (default: cwd, supports ~ and $VAR)
tools = ["read_file", "write_file"] # Optional (valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search)
sub_agents = ["agent1", "agent2"] # Optional (allowed sub-agent names)

[sub_agents_config]
max_depth = 3    # Default: 3, Max: 5
parallel = true  # Default: true (run sub-agents concurrently)
timeout = 120    # Default: 120 seconds

[memory]
enabled = true       # Default: false
last_n = 10          # Default: 10 (load last N entries into context)
max_entries = 100    # Default: 100 (warn when exceeded)
path = "/custom/path" # Optional (default: $XDG_DATA_HOME/axe/memory/{agent-name}.md)

[params]
temperature = 0.7  # Default: 0 (omitted if 0)
max_tokens = 4096  # Default: 0 (provider default)

[[mcp_servers]]
name = "server-name"
transport = "stdio"  # or "http", "https"
command = "/path/to/server"
args = ["--flag"]
env = {"KEY" = "value"}

[[mcp_servers]]
name = "http-server"
transport = "https"
url = "https://example.com/mcp"
headers = {"Authorization" = "Bearer ${API_KEY}"}  # ${VAR} expanded from env
```

### Global Config (`$XDG_CONFIG_HOME/axe/config.toml`)

```toml
[providers.anthropic]
api_key = "sk-ant-..."
base_url = "https://api.anthropic.com"

[providers.openai]
api_key = "sk-..."
base_url = "https://api.openai.com"

[providers.ollama]
base_url = "http://localhost:11434"

[providers.bedrock]
region = "us-east-1"
```

**Env vars override config file:**
- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`
- `AXE_ANTHROPIC_BASE_URL`, `AXE_OPENAI_BASE_URL`, `AXE_OLLAMA_BASE_URL`, `AXE_OPENCODE_BASE_URL`
- `AWS_REGION`, `AWS_DEFAULT_REGION` (for Bedrock)
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (for Bedrock)
- `TAVILY_API_KEY` (overrides config for web search API key), `AXE_WEB_SEARCH_BASE_URL`

## Repo-Specific Patterns

### Testing

- **Table-driven tests** - See any `*_test.go` for examples. Use `[]struct{name, input, want}` pattern.
- **Mock servers** - `internal/testutil/mockserver.go` provides `NewMockLLMServer()` for Anthropic/OpenAI responses. Use in integration tests.
- **Golden files** - `cmd/golden_test.go` and `cmd/testdata/golden/` for CLI output testing. Update with `UPDATE_GOLDEN=1 go test`.
- **Fixtures** - `cmd/testdata/agents/` and `cmd/testdata/skills/` for test data. Use `testutil.SeedFixtureAgents()` and `testutil.SeedFixtureSkills()`.
- **Integration tests** - `cmd/run_integration_test.go` and `cmd/run_mcp_integration_test.go` compile binary and run end-to-end tests.
- **Smoke tests** - `cmd/smoke_test.go` runs actual binary with real config (requires API keys in CI).

### Error Handling

- **Provider errors** - Wrapped in `ProviderError` with category (Auth, RateLimit, Server, Input, Network, Unknown). See `internal/provider/provider.go`.
- **Tool errors** - Returned as tool results to LLM, not fatal. LLM can retry or adjust.
- **Config errors** - Exit code 2. Validation happens in `internal/agent/agent.go` `Validate()`.
- **Runtime errors** - Exit code 1. Mapped in `cmd/run.go` `mapProviderError()`.

### Path Security

- **All file tools sandboxed** - `internal/tool/path_validation.go` `validatePath()` blocks absolute paths, `..` traversal, symlink escapes.
- **Working directory enforcement** - All file operations relative to agent's workdir.
- **Skill script paths must be absolute** - Scripts in SKILL.md run from agent's workdir, not skill dir. Use absolute paths like `/home/user/.config/axe/skills/my-skill/scripts/fetch.sh`.

### Conversation Loop

- **Max 50 turns** - Hard limit in `cmd/run.go` `runAgent()`. Prevents infinite loops.
- **Stops on text response** - Loop exits when LLM returns text without tool calls.
- **Parallel tool execution** - Default behavior. Set `sub_agents_config.parallel = false` for sequential.
- **Tool call details** - Captured in JSON output with `--json` flag. Includes arguments, results, truncation status, errors.

### Sub-Agent Delegation

- **Opaque execution** - Parent never sees sub-agent's internal turns, tool calls, or files. Only final text result.
- **Depth tracking** - Incremented on each delegation. `call_agent` tool removed at max depth.
- **Allowed list** - Sub-agent name must be in parent's `sub_agents` list.
- **Independent context** - Sub-agent loads its own TOML, skill, files. No context inheritance except the task string.

### Memory System

- **Markdown format** - `## YYYY-MM-DDTHH:MM:SS±HH:MM` headers, `**Task:**` and `**Result:**` sections.
- **Result truncation** - Results truncated to 1000 chars when appending.
- **Load on run** - Last N entries loaded into system prompt if memory enabled.
- **Append on success** - Entry added only if agent run succeeds (no API errors).
- **GC with LLM** - `axe gc agent` uses LLM to detect patterns and suggest keep count. Falls back to max_entries if LLM fails.

### MCP Integration

- **Tool namespacing** - MCP tools prefixed with server name: `server-name.tool-name`.
- **Built-in collision** - MCP tools with same name as built-ins are skipped (built-ins win).
- **Transport validation** - Only stdio, http, https allowed. Validated in `internal/agent/agent.go`.
- **Header expansion** - HTTP headers support `${VAR}` syntax, expanded from env vars.
- **Type coercion** - MCP tool arguments coerced to match schema (string/number/boolean).

## CI/CD

- **GitHub Actions** - `.github/workflows/go.yml` runs tests and linting on push/PR.
- **Release workflow** - `.github/workflows/release.yml` uses GoReleaser on tag push.
- **GoReleaser** - `.goreleaser.yml` builds multi-platform binaries, creates GitHub release.
- **Linting** - `.golangci.yml` configures golangci-lint. Run with `golangci-lint run`.

## Docker

- **Dockerfile** - Multi-stage build: golang:1.25-alpine → alpine:latest.
- **Security** - Non-root user (UID 10001), read-only rootfs, all caps dropped, no new privileges.
- **Volumes** - Mount config to `/home/axe/.config/axe`, data to `/home/axe/.local/share/axe`.
- **Compose** - `docker-compose.yml` includes Ollama sidecar with `--profile ollama`.

## Project Docs

Design docs live in `./docs/` — check milestones, config schema, sub-agent patterns, and CLI structure there before making architectural changes.

## Custom Instructions
<!-- This section is for human and agent-maintained operational knowledge.
     Add repo-specific conventions, gotchas, and workflow rules here.
     This section is preserved exactly as-is when re-running codebase-summary. -->
