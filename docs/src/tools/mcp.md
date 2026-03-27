# MCP Tools

Agents can use tools from external [MCP](https://modelcontextprotocol.io/) servers. Declare servers in the agent TOML with `[[mcp_servers]]`:

```toml
[[mcp_servers]]
name = "my-tools"
url = "https://my-mcp-server.example.com/sse"
transport = "sse"
headers = { Authorization = "Bearer ${MY_TOKEN}" }
```

At startup, axe connects to each declared server, discovers available tools via `tools/list`, and makes them available to the LLM alongside built-in tools.

## Configuration Fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Human-readable identifier for the server |
| `url` | Yes | MCP server endpoint URL |
| `transport` | Yes | `"sse"` or `"streamable-http"` |
| `headers` | No | HTTP headers; values support `${ENV_VAR}` interpolation |

## stdio Transport

For local MCP servers using stdio transport:

```toml
[[mcp_servers]]
name = "filesystem"
transport = "stdio"
command = "/usr/local/bin/mcp-server-filesystem"
args = ["--root", "/home/user/projects"]
```

## Precedence

MCP tools are controlled entirely by `[[mcp_servers]]` — they are not listed in the `tools` field. If an MCP tool has the same name as an enabled built-in tool, the built-in takes precedence.
