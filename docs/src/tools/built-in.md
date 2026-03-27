# Built-in Tools

When tools are enabled, the agent enters a conversation loop — the LLM can make tool calls, receive results, and continue reasoning for up to 50 turns.

Enable tools by adding them to the agent's `tools` field:

```toml
tools = ["read_file", "list_directory", "run_command"]
```

## Available Tools

| Tool | Description |
|---|---|
| `list_directory` | List contents of a directory relative to the working directory |
| `read_file` | Read file contents with line-numbered output and optional pagination (offset/limit) |
| `write_file` | Create or overwrite a file, creating parent directories as needed |
| `edit_file` | Find and replace exact text in a file, with optional replace-all mode |
| `run_command` | Execute a shell command via `sh -c` and return combined output |
| `url_fetch` | Fetch URL content with HTML stripping and truncation |
| `web_search` | Search the web and return results |
| `call_agent` | Delegate a task to a sub-agent (controlled via `sub_agents`, not `tools`) |

> **Note:** The `call_agent` tool is not listed in `tools` — it is automatically available when `sub_agents` is configured and the depth limit has not been reached.

## Path Security

All file tools (`list_directory`, `read_file`, `write_file`, `edit_file`) are sandboxed to the agent's working directory. Absolute paths, `..` traversal, and symlink escapes are rejected.

### run_command Sandbox

`run_command` also applies heuristic path validation — it rejects commands containing:

- `..` path components that would escape the working directory
- Absolute paths outside the working directory
- `~` in shell expansion position (home directory expansion)

> **Note:** This is a heuristic guard, not airtight sandboxing. Shell is Turing-complete, and sufficiently creative constructs (variable expansion, command substitution, `eval`, aliases) can bypass it. For full sandboxing, run axe inside Docker.

## Tool Output Truncation

Tool call outputs are truncated to **1024 bytes**. Truncated outputs end with `... (truncated)`. This affects the `tool_call_details` array in JSON output — plan accordingly if you're scripting against it.

## Parallel Execution

When an LLM returns multiple tool calls in a single turn, they run concurrently by default. This applies to both built-in tools and sub-agent calls. Disable with `parallel = false` in `[sub_agents_config]`.
