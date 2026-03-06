# Specification: MCP (Model Context Protocol) Tool Support

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-05
**GitHub Issue:** https://github.com/jrswab/axe/issues/9
**Scope:** MCP client support — connect to external MCP servers, discover tools, and route LLM tool calls to them

---

## 1. Purpose

Add support for connecting to MCP servers as a tool source, allowing agents to use tools
exposed by external MCP servers alongside axe's built-in tools.

Axe acts as an **MCP client only** (tool consumer). Exposing axe's own tools as an MCP
server is out of scope.

An agent declares MCP servers in its TOML config. At startup, axe connects to each
declared server, discovers available tools via the MCP `tools/list` method, converts them
to `provider.Tool` definitions, and appends them to the LLM request. During the
conversation loop, tool calls targeting MCP tools are routed to the appropriate MCP
server session via the MCP `tools/call` method.

This feature introduces one new external dependency:
`github.com/modelcontextprotocol/go-sdk` (the official MCP Go SDK).

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Official Go SDK.** Use `github.com/modelcontextprotocol/go-sdk/mcp` for MCP client
   functionality. This provides spec-compliant transport implementations
   (`StreamableClientTransport`, `SSEClientTransport`), session management, and
   JSON-RPC handling. Pin to a specific release version in `go.mod`.

2. **Two transport types.** Support `"sse"` and `"streamable-http"` as transport values.
   `"sse"` uses the SDK's `SSEClientTransport`. `"streamable-http"` uses the SDK's
   `StreamableClientTransport`. No other transport types are supported.

3. **Eager connection at startup.** All declared MCP servers are connected before the
   first LLM call. Tool discovery (`tools/list`) happens immediately after connection.
   Sessions remain open for the duration of the conversation loop and are closed when
   the agent run completes.

4. **Fail fast on connection failure.** If any declared MCP server cannot be reached
   (connection refused, DNS failure, timeout, TLS error, initialization handshake
   failure), the agent fails immediately with exit code 3 (API error).

5. **Runtime-only tool validation.** MCP tool names are discovered dynamically via
   `tools/list`. They are not added to `toolname.ValidNames()`. The `tools` field in
   agent TOML controls only built-in tools. MCP tools are controlled entirely by the
   `[[mcp_servers]]` configuration.

6. **Built-in tool wins on name collision.** If an MCP server exposes a tool whose name
   matches a built-in tool that is enabled for this agent (present in `cfg.Tools`), the
   MCP tool is silently skipped. A warning is logged to stderr in verbose mode. The
   built-in tool takes precedence because it has known, tested behavior.

7. **MCP-to-MCP collision is an error.** If two different MCP servers expose tools with
   the same name, the agent fails at startup with exit code 2 (config error). The error
   message names both MCP server names and the colliding tool name.

8. **Flatten complex JSON Schema types.** MCP tools use full JSON Schema for
   `InputSchema`. Axe's `provider.ToolParameter` has `Type`, `Description`, and
   `Required` fields. Simple JSON Schema types (`string`, `integer`, `number`,
   `boolean`) map directly to the `Type` field. Complex types (`object`, `array`,
   `oneOf`, `anyOf`, `allOf`, or any type not in the simple set) are mapped to
   `Type: "string"` with the full schema description preserved in the `Description`
   field. The LLM reads the description and formats accordingly.

9. **Concatenate text content from results.** MCP `CallToolResult` can contain multiple
   content items (`TextContent`, `ImageContent`, `EmbeddedResource`, etc.). All
   `TextContent` items are extracted and concatenated with newline (`"\n"`) separators.
   Non-text content items are ignored (axe is a text-only CLI tool). If no `TextContent`
   items exist in the result, the tool returns `ToolResult{IsError: true}` with content
   `"MCP tool returned no text content"`.

10. **Pass arguments as strings.** Axe's `ToolCall.Arguments` is `map[string]string`.
    When dispatching to an MCP server, each string value is placed into a
    `map[string]any` as-is (the `any` value is the original `string`). The MCP server is
    responsible for any type coercion. This is consistent with how axe handles built-in
    tool arguments.

11. **Environment variable interpolation for headers.** Header values support `${VAR}`
    syntax. The pattern `${VAR_NAME}` is replaced with `os.Getenv("VAR_NAME")`. If the
    referenced environment variable is not set or is empty, the agent fails at config
    resolution time with exit code 2 (config error). The error message names the missing
    variable and the MCP server name. Static header values (no `${}` pattern) pass
    through unchanged.

12. **Dry-run shows config only, no connection.** In `--dry-run` mode, MCP server
    configuration (name, URL, transport) is displayed in a `--- MCP Servers ---` section.
    No connections are established. Tool names are not shown because they require a live
    connection.

13. **Sub-agents get their own MCP connections.** When a sub-agent has `[[mcp_servers]]`
    in its own config, it establishes its own MCP connections independently. The parent
    agent's MCP connections are never shared with sub-agents. This preserves the
    sub-agent opacity principle.

14. **MCP tool calls appear in `--json` output.** MCP tool calls are included in the
    `tool_call_details` array with the same structure as built-in tool calls: `name`,
    `input`, `output`, `is_error`. There is no distinction between MCP and built-in tool
    calls in the JSON envelope.

15. **Connection timeout inherits `--timeout`.** MCP server connections use the same
    timeout context as the agent run. If the `--timeout` flag is 120 seconds, MCP
    connections and tool calls share that deadline.

---

## 3. Requirements

### 3.1 Agent Config: `MCPServerConfig` Struct

**Requirement 1.1:** Add a new exported struct `MCPServerConfig` to
`internal/agent/agent.go` with the following fields:

| Field | Go Type | TOML Tag | Description |
|-------|---------|----------|-------------|
| `Name` | `string` | `toml:"name"` | Human-readable identifier for this MCP server |
| `URL` | `string` | `toml:"url"` | The MCP server endpoint URL |
| `Transport` | `string` | `toml:"transport"` | Transport type: `"sse"` or `"streamable-http"` |
| `Headers` | `map[string]string` | `toml:"headers"` | Optional HTTP headers (supports `${VAR}` interpolation) |

**Requirement 1.2:** Add a field `MCPServers []MCPServerConfig` with tag
`toml:"mcp_servers"` to the `AgentConfig` struct.

**Requirement 1.3:** The TOML syntax for declaring MCP servers is `[[mcp_servers]]`
(array of tables). Example:

```toml
[[mcp_servers]]
name = "my-tools"
url = "https://my-mcp-server.example.com/sse"
transport = "sse"

[[mcp_servers]]
name = "pipedream"
url = "https://remote.mcp.pipedream.net"
transport = "streamable-http"
headers = { Authorization = "Bearer ${PIPEDREAM_TOKEN}" }
```

### 3.2 Agent Config Validation

**Requirement 2.1:** Add validation for `MCPServers` in the `Validate()` function.
Validation runs after existing field checks (name, model, etc.).

**Requirement 2.2:** For each entry in `MCPServers`:
- `Name` must be non-empty after trimming whitespace. Error:
  `"mcp_servers[%d]: name is required"` (0-indexed).
- `URL` must be non-empty after trimming whitespace. Error:
  `"mcp_servers[%d]: url is required"` (0-indexed).
- `Transport` must be exactly `"sse"` or `"streamable-http"`. Error:
  `"mcp_servers[%d]: transport must be \"sse\" or \"streamable-http\", got %q"`
  (0-indexed).

**Requirement 2.3:** MCP server names must be unique within the agent. If two entries
have the same `Name` (case-sensitive, after trimming whitespace), return error:
`"mcp_servers: duplicate server name %q"`.

**Requirement 2.4:** Validation does NOT check URL format, header values, or environment
variable availability. Those are runtime concerns resolved at connection time.

### 3.3 Environment Variable Interpolation

**Requirement 3.1:** Create a new package `internal/envinterp/` with a single file
`envinterp.go`.

**Requirement 3.2:** Export a function
`ExpandHeaders(headers map[string]string) (map[string]string, error)`.

**Requirement 3.3:** The function iterates over each key-value pair in the input map. For
each value, it scans for patterns matching the regex `\$\{([^}]+)\}`. For each match:
- Extract the variable name (the content between `${` and `}`).
- Look up the value via `os.Getenv(varName)`.
- If the environment variable is not set or is empty, return an error:
  `"environment variable %q is not set"`.
- Replace the `${VAR_NAME}` pattern with the environment variable value.

**Requirement 3.4:** Multiple `${VAR}` patterns in a single header value are all
expanded. Example: `"${SCHEME}://${HOST}"` with `SCHEME=https` and `HOST=example.com`
produces `"https://example.com"`.

**Requirement 3.5:** Values with no `${...}` patterns are returned unchanged (static
values).

**Requirement 3.6:** If the input map is nil, return nil and no error.

**Requirement 3.7:** The returned map is a new map instance. The input map is not
modified.

### 3.4 MCP Client Package

**Requirement 4.1:** Create a new package `internal/mcpclient/` with a file
`mcpclient.go`.

**Requirement 4.2:** Export a struct `Client` that wraps an MCP client session. The
struct holds:
- The server name (from `MCPServerConfig.Name`)
- The SDK `*mcp.ClientSession`

**Requirement 4.3:** Export a function
`Connect(ctx context.Context, cfg agent.MCPServerConfig) (*Client, error)`.

This function:
1. Expands environment variables in `cfg.Headers` via `envinterp.ExpandHeaders()`.
   If expansion fails, returns the error (caller maps to exit code 2).
2. Creates an `*http.Client` with a custom `http.RoundTripper` that injects the
   expanded headers into every outgoing HTTP request.
3. Based on `cfg.Transport`:
   - `"sse"`: Creates an `mcp.SSEClientTransport` with the custom HTTP client and
     `cfg.URL` as the endpoint.
   - `"streamable-http"`: Creates an `mcp.StreamableClientTransport` with the custom
     HTTP client and `cfg.URL` as the endpoint.
4. Creates an `mcp.Client` with implementation info `{Name: "axe", Version: <axe version>}`.
5. Calls `client.Connect(ctx, transport)` to establish the session (performs the MCP
   initialize/initialized handshake).
6. Returns the wrapped `*Client` or an error if connection fails.

**Requirement 4.4:** Export a method
`(c *Client) ListTools(ctx context.Context) ([]provider.Tool, error)`.

This method:
1. Calls `session.ListTools(ctx, nil)` to get the MCP `ListToolsResult`.
2. Converts each `mcp.Tool` to a `provider.Tool`:
   - `Name`: from `mcp.Tool.Name`.
   - `Description`: from `mcp.Tool.Description`.
   - `Parameters`: Extracted from `mcp.Tool.InputSchema`. The `InputSchema` from the
     client side is a `map[string]any` representing a JSON Schema object. Extract the
     `"properties"` key (expected to be `map[string]any`). For each property, extract
     `"type"` and `"description"`. The `"required"` array from the top-level schema
     determines which parameters are required.
   - **Type mapping for parameters:**
     - `"string"`, `"integer"`, `"number"`, `"boolean"` map directly to the
       `ToolParameter.Type` field.
     - Any other type value (including `"object"`, `"array"`, empty, missing, or
       composite schemas like `oneOf`) maps to `Type: "string"`. The original type
       information is prepended to the description:
       `"(JSON Schema type: <original_type>) <description>"`.
     - If the `"type"` key is missing from a property, use `Type: "string"`.
3. Returns the converted slice or an error.

**Requirement 4.5:** Export a method
`(c *Client) CallTool(ctx context.Context, call provider.ToolCall) (provider.ToolResult, error)`.

This method:
1. Converts `call.Arguments` (`map[string]string`) to `map[string]any` by assigning
   each string value as-is.
2. Creates `mcp.CallToolParams{Name: call.Name, Arguments: args}`.
3. Calls `session.CallTool(ctx, params)`.
4. Converts the result:
   - Iterates over `result.Content`. For each item, checks if it is `*mcp.TextContent`.
     If so, appends the `Text` field to a slice.
   - Joins all collected text items with `"\n"`.
   - If no text items are found, returns
     `ToolResult{CallID: call.ID, Content: "MCP tool returned no text content", IsError: true}`.
   - If `result.IsError` is true, returns
     `ToolResult{CallID: call.ID, Content: <joined text>, IsError: true}`.
   - Otherwise returns
     `ToolResult{CallID: call.ID, Content: <joined text>, IsError: false}`.

**Requirement 4.6:** Export a method `(c *Client) Name() string` that returns the server
name.

**Requirement 4.7:** Export a method `(c *Client) Close() error` that closes the MCP
session.

### 3.5 MCP Tool Router

**Requirement 5.1:** Create a file `internal/mcpclient/router.go`.

**Requirement 5.2:** Export a struct `Router` that maps tool names to their owning
`*Client`.

**Requirement 5.3:** Export a function `NewRouter() *Router` that returns an initialized
router with an empty map.

**Requirement 5.4:** Export a method
`(r *Router) Register(client *Client, tools []provider.Tool, builtinNames map[string]bool) ([]provider.Tool, error)`.

This method:
1. For each tool in `tools`:
   - If the tool name exists in `builtinNames`, skip it (built-in wins). If verbose
     logging is needed, the caller handles that — this method is silent on skips but
     returns only the non-skipped tools.
   - If the tool name is already registered in the router from a different client,
     return an error:
     `"MCP tool name collision: tool %q is exposed by both %q and %q"` where the two
     values are the server names.
   - Otherwise, add the mapping from tool name to client.
2. Returns the filtered slice of `provider.Tool` definitions (excluding skipped tools)
   and nil error on success.

**Requirement 5.5:** Export a method
`(r *Router) Dispatch(ctx context.Context, call provider.ToolCall) (provider.ToolResult, error)`.

This method:
1. Looks up the client by `call.Name`.
2. If not found, returns `(zero ToolResult, error)` with
   `"unknown MCP tool %q"`.
3. Delegates to `client.CallTool(ctx, call)`.

**Requirement 5.6:** Export a method `(r *Router) Has(name string) bool` that returns
true if the tool name is registered in the router.

**Requirement 5.7:** Export a method `(r *Router) Close() error` that calls `Close()` on
every unique client. Each client is closed exactly once even if it registered multiple
tools. Returns the first error encountered (or nil).

### 3.6 Integration into `cmd/run.go`

**Requirement 6.1:** After creating the tool registry and resolving built-in tools
(current Step 16b), add MCP server handling:

1. If `cfg.MCPServers` is non-empty:
   a. Create an `mcpclient.Router` via `mcpclient.NewRouter()`.
   b. `defer router.Close()` immediately after creation.
   c. For each `MCPServerConfig` in `cfg.MCPServers`:
      - Call `mcpclient.Connect(ctx, serverCfg)`. On error, return
        `&ExitError{Code: 3, Err: ...}` for connection/transport errors or
        `&ExitError{Code: 2, Err: ...}` for env var interpolation errors.
      - Call `client.ListTools(ctx)`. On error, return `&ExitError{Code: 3, Err: ...}`.
      - Build a `builtinNames` map from the agent's `cfg.Tools` (the built-in tool
        names that are active for this agent).
      - Call `router.Register(client, tools, builtinNames)`. On error (MCP-to-MCP
        collision), return `&ExitError{Code: 2, Err: ...}`.
      - Append the returned (filtered) tools to `req.Tools`.
   d. Log to stderr in verbose mode: MCP server name, number of tools discovered, and
      any skipped tool names (built-in collision).

**Requirement 6.2:** In `executeToolCalls()`, add the router as a parameter. Before
falling through to `registry.Dispatch()`, check if `router != nil && router.Has(tc.Name)`.
If true, dispatch via `router.Dispatch(ctx, tc)`.

**Requirement 6.3:** Pass the router through `ExecuteOptions` (add a field) so that
`tool.ExecuteCallAgent()` and `runConversationLoop()` can also dispatch MCP tool calls
for the parent agent's conversation loop. Note: this is for the parent's conversation
loop only. Sub-agents do NOT inherit the parent's router.

**Requirement 6.4:** In `tool.ExecuteCallAgent()`, when a sub-agent has its own
`cfg.MCPServers`, establish separate MCP connections for the sub-agent. The sub-agent
creates its own `mcpclient.Router`, connects to its own declared servers, discovers its
own tools, and closes its own sessions when done. The parent's router is never passed to
or shared with sub-agents.

### 3.7 Dry-Run Output

**Requirement 7.1:** Add a `--- MCP Servers ---` section to `printDryRun()` output,
displayed after the `--- Tools ---` section and before `--- Sub-Agents ---`.

**Requirement 7.2:** If `cfg.MCPServers` is non-empty, print each server as:
```
<name>: <url> (<transport>)
```
One server per line.

**Requirement 7.3:** If `cfg.MCPServers` is empty, print `(none)`.

**Requirement 7.4:** No MCP connections are established during dry-run. Tool names
from MCP servers are not displayed.

### 3.8 Verbose Output

**Requirement 8.1:** When `--verbose` is set and MCP servers are configured, log the
following to stderr:

- Before connecting: `[mcp] Connecting to %q (%s, %s)\n` with server name, URL,
  transport.
- After tool discovery: `[mcp] %q: discovered %d tools\n` with server name, count.
- On built-in skip: `[mcp] %q: skipping tool %q (built-in takes precedence)\n` with
  server name, tool name.
- On tool call dispatch: `[mcp] Routing tool %q to server %q\n` with tool name,
  server name.

### 3.9 Agent Scaffold Update

**Requirement 9.1:** Update the `Scaffold()` function in `internal/agent/agent.go` to
include a commented-out MCP servers example:

```toml
# MCP server connections (optional)
# [[mcp_servers]]
# name = "my-tools"
# url = "https://my-mcp-server.example.com/sse"
# transport = "sse"
# headers = { Authorization = "Bearer ${MY_TOKEN}" }
```

This is added after the `sub_agents_config` section and before the `[memory]` section.

### 3.10 JSON Output

**Requirement 10.1:** MCP tool calls appear in the `tool_call_details` array with the
same structure as built-in tool calls. No new fields or distinctions are added to the
JSON envelope.

**Requirement 10.2:** The `tool_calls` count in the JSON envelope includes MCP tool
calls.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── go.mod                                # MODIFIED: add github.com/modelcontextprotocol/go-sdk dependency
├── go.sum                                # MODIFIED: updated checksums
├── internal/
│   ├── agent/
│   │   ├── agent.go                      # MODIFIED: MCPServerConfig struct, MCPServers field, Validate(), Scaffold()
│   │   └── agent_test.go                 # MODIFIED: tests for MCP config parsing/validation
│   ├── envinterp/
│   │   ├── envinterp.go                  # NEW: ExpandHeaders function
│   │   └── envinterp_test.go             # NEW: tests
│   ├── mcpclient/
│   │   ├── mcpclient.go                  # NEW: Client, Connect, ListTools, CallTool, Close
│   │   ├── mcpclient_test.go             # NEW: tests
│   │   ├── router.go                     # NEW: Router, Register, Dispatch, Has, Close
│   │   └── router_test.go               # NEW: tests
│   └── tool/
│       └── tool.go                       # MODIFIED: add router field to ExecuteOptions, dispatch MCP in runConversationLoop
├── cmd/
│   ├── run.go                            # MODIFIED: MCP connection, discovery, dispatch integration, dry-run, verbose
│   ├── run_test.go                       # MODIFIED: tests for MCP in run command
│   └── testdata/
│       └── agents/                       # MODIFIED: new fixture TOMLs with MCP config
└── ...                                   # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Config Parsing

| Scenario | Input | Behavior |
|----------|-------|----------|
| No MCP servers | `mcp_servers` key absent | `cfg.MCPServers` is nil. No MCP logic executes. Agent runs normally. |
| Empty MCP servers array | `mcp_servers = []` | Same as absent. Zero-length slice. No MCP logic executes. |
| Missing name | `[[mcp_servers]]` with no `name` key | Validation error: `"mcp_servers[0]: name is required"`. Exit 2. |
| Missing URL | `name = "x"` but no `url` | Validation error: `"mcp_servers[0]: url is required"`. Exit 2. |
| Missing transport | `name = "x"`, `url = "..."` but no `transport` | Validation error: transport must be `"sse"` or `"streamable-http"`, got `""`. Exit 2. |
| Invalid transport | `transport = "websocket"` | Validation error: transport must be `"sse"` or `"streamable-http"`, got `"websocket"`. Exit 2. |
| Duplicate server names | Two entries with `name = "tools"` | Validation error: `"mcp_servers: duplicate server name \"tools\""`. Exit 2. |
| No headers | `headers` key absent | `cfg.Headers` is nil. No header injection. |
| Empty headers map | `headers = {}` | Empty map. No header injection. |

### 5.2 Environment Variable Interpolation

| Scenario | Input Value | Env State | Behavior |
|----------|-------------|-----------|----------|
| Static value | `"Bearer my-token"` | N/A | Returned unchanged. |
| Single variable | `"Bearer ${TOKEN}"` | `TOKEN=abc123` | Expanded to `"Bearer abc123"`. |
| Multiple variables | `"${SCHEME}://${HOST}"` | Both set | Expanded to full string. |
| Missing variable | `"Bearer ${TOKEN}"` | `TOKEN` not set | Error: `"environment variable \"TOKEN\" is not set"`. Exit 2. |
| Empty variable | `"Bearer ${TOKEN}"` | `TOKEN=""` | Error: `"environment variable \"TOKEN\" is not set"`. Exit 2. (Empty is treated as unset.) |
| Nested braces | `"${FOO${BAR}}"` | N/A | The regex `\$\{([^}]+)\}` matches `${FOO${BAR}` — the first `}` terminates the match. Variable name is `FOO${BAR` which will likely fail lookup. This is not a supported pattern. |
| No dollar-brace pattern | `"plain-value"` | N/A | Returned unchanged. |
| Dollar without brace | `"$TOKEN"` | N/A | Returned unchanged. Only `${...}` syntax is expanded. |

### 5.3 MCP Connection

| Scenario | Behavior |
|----------|----------|
| Server unreachable (connection refused) | `Connect()` returns error. Agent exits with code 3. |
| DNS resolution failure | `Connect()` returns error. Agent exits with code 3. |
| TLS certificate error | `Connect()` returns error. Agent exits with code 3. |
| Server rejects initialize (unsupported protocol version) | `Connect()` returns error. Agent exits with code 3. |
| Server responds but `tools/list` fails | `ListTools()` returns error. Agent exits with code 3. |
| Server returns zero tools | `ListTools()` returns empty slice. No tools added. Agent proceeds (not an error). |
| Connection timeout (exceeds `--timeout`) | Context deadline exceeded. Agent exits with code 3. |

### 5.4 Tool Name Collisions

| Scenario | Behavior |
|----------|----------|
| MCP tool `read_file`, built-in `read_file` enabled | MCP tool skipped. Built-in wins. Verbose log emitted. |
| MCP tool `read_file`, built-in `read_file` NOT in `cfg.Tools` | MCP tool is NOT skipped. It is registered because the built-in is not active for this agent. `builtinNames` only contains tools from `cfg.Tools`. |
| MCP-A has `search`, MCP-B has `search` | Error at startup: `"MCP tool name collision: tool \"search\" is exposed by both \"server-a\" and \"server-b\""`. Exit 2. |
| MCP tool `call_agent` | `call_agent` is not in `builtinNames` (it is special-cased, not in the tools registry). The MCP tool named `call_agent` is registered. This is valid — the MCP tool dispatch happens before the special-case `call_agent` check in `executeToolCalls()`. However, this is an unlikely edge case. |

### 5.5 Tool Schema Conversion

| JSON Schema Type | `ToolParameter.Type` | Notes |
|-----------------|---------------------|-------|
| `"string"` | `"string"` | Direct mapping. |
| `"integer"` | `"integer"` | Direct mapping. |
| `"number"` | `"number"` | Direct mapping. |
| `"boolean"` | `"boolean"` | Direct mapping. |
| `"object"` | `"string"` | Description prepended with `"(JSON Schema type: object) "`. |
| `"array"` | `"string"` | Description prepended with `"(JSON Schema type: array) "`. |
| Missing `"type"` key | `"string"` | Default. No prefix added to description. |
| `"null"` | `"string"` | Description prepended with `"(JSON Schema type: null) "`. |
| Property has no `"description"` | `""` | Description is empty string. Type prefix still applied if complex. |
| `InputSchema` is nil or not a map | Zero parameters | Tool has no parameters. |
| `InputSchema` has no `"properties"` key | Zero parameters | Tool has no parameters. |
| `InputSchema.required` lists a property | `Required: true` | Parameter is marked required. |
| `InputSchema.required` omits a property | `Required: false` | Parameter is optional. |
| `InputSchema.required` is absent | All `Required: false` | No parameters are marked required. |

### 5.6 Tool Call Dispatch

| Scenario | Behavior |
|----------|----------|
| LLM calls MCP tool by name | Router finds owning client, delegates `CallTool`. Result returned to LLM. |
| LLM calls built-in tool by name | Router check returns false. Falls through to registry dispatch. Normal built-in execution. |
| LLM calls `call_agent` | Special-cased before router check (existing behavior). Router never consulted. |
| LLM calls unknown tool name | Router returns false. Registry dispatch returns error. Error returned to LLM as `ToolResult{IsError: true}`. |
| MCP server returns error on `tools/call` | `CallTool` returns error. Wrapped as `ToolResult{IsError: true}`. LLM sees the error message. |
| MCP server returns `IsError: true` in result | `ToolResult{IsError: true}` with the text content. |
| MCP server returns only ImageContent | `ToolResult{IsError: true, Content: "MCP tool returned no text content"}`. |
| MCP server returns mixed TextContent + ImageContent | Only TextContent is extracted. ImageContent ignored. |
| MCP server returns multiple TextContent items | All text items concatenated with `"\n"`. |

### 5.7 Sub-Agent MCP

| Scenario | Behavior |
|----------|----------|
| Parent has MCP, sub-agent has no MCP | Sub-agent runs without MCP tools. Parent's MCP connections are not shared. |
| Parent has no MCP, sub-agent has MCP | Sub-agent establishes its own MCP connections, discovers tools, closes on completion. |
| Parent and sub-agent both have MCP | Independent connections. No sharing. |
| Sub-agent MCP connection fails | Sub-agent returns `ToolResult{IsError: true}`. Parent handles gracefully (existing behavior). |

---

## 6. Constraints

**Constraint 1:** One new external dependency: `github.com/modelcontextprotocol/go-sdk`.
Pin to a specific release version. No other new dependencies.

**Constraint 2:** No changes to `internal/provider/provider.go`. The `Tool`,
`ToolCall`, `ToolResult`, and `Message` types are unchanged.

**Constraint 3:** No changes to `internal/toolname/toolname.go`. MCP tool names are
not added to `ValidNames()`. MCP tools are not part of the built-in tool system.

**Constraint 4:** `call_agent` remains special-cased outside the registry. MCP tools
do not affect the `call_agent` dispatch path.

**Constraint 5:** The `tools` field in agent TOML continues to control only built-in
tools. MCP tools are controlled exclusively by `[[mcp_servers]]`.

**Constraint 6:** MCP connections are never shared across agents. Each agent (or
sub-agent) establishes and owns its own connections.

**Constraint 7:** Axe is an MCP client only. It does not expose its own tools as an
MCP server.

**Constraint 8:** Only `"sse"` and `"streamable-http"` transport types are supported.
`"stdio"` is not supported (axe does not launch MCP servers as subprocesses).

**Constraint 9:** No retry logic for MCP connections or tool calls. A single attempt is
made. If it fails, the error is returned.

**Constraint 10:** No caching of MCP tool definitions. `tools/list` is called once per
connection at startup. Tool definitions are not refreshed during the conversation loop.

**Constraint 11:** The `Headers` field in `MCPServerConfig` applies to all HTTP requests
to that MCP server (transport negotiation, initialize, tools/list, tools/call). This is
achieved via a custom `RoundTripper` on the `http.Client`.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in the existing codebase:

- **Package-level tests:** Tests live in the same package (e.g., `package mcpclient`).
- **Standard library only for test framework:** Use `testing` package. No test frameworks.
- **`net/http/httptest`:** Use `httptest.NewServer` to create real HTTP test servers that
  simulate MCP servers. Tests must make real HTTP requests.
- **MCP SDK test server:** If the SDK provides server-side utilities for creating test
  MCP servers, use them. Otherwise, build a minimal MCP server handler using httptest
  that speaks the JSON-RPC protocol.
- **Environment variable isolation:** Tests that set environment variables must use
  `t.Setenv()`.
- **Descriptive names:** `TestComponentName_Scenario` with underscores.
- **Test real code, not mocks.** Every test must fail if the code under test is deleted.
- **Red/green TDD.**
- **Run tests with:** `make test`

### 7.2 `internal/envinterp/envinterp_test.go`

**Test: `TestExpandHeaders_StaticValues`** — Input `{"Content-Type": "application/json"}`.
No env vars needed. Verify output equals input. Verify no error.

**Test: `TestExpandHeaders_SingleVar`** — `t.Setenv("MY_TOKEN", "secret123")`. Input
`{"Authorization": "Bearer ${MY_TOKEN}"}`. Verify output is
`{"Authorization": "Bearer secret123"}`. Verify no error.

**Test: `TestExpandHeaders_MultipleVarsInOneValue`** — Set `SCHEME=https` and
`HOST=example.com`. Input `{"URL": "${SCHEME}://${HOST}"}`. Verify output is
`{"URL": "https://example.com"}`.

**Test: `TestExpandHeaders_MultipleHeaders`** — Set `TOKEN=abc` and `REGION=us-east`.
Input `{"Auth": "Bearer ${TOKEN}", "X-Region": "${REGION}"}`. Verify both are expanded.

**Test: `TestExpandHeaders_MissingVar`** — Do not set `MISSING_VAR`. Input
`{"Auth": "Bearer ${MISSING_VAR}"}`. Verify error is returned. Verify error message
contains `"MISSING_VAR"`. Verify error message contains `"not set"`.

**Test: `TestExpandHeaders_EmptyVar`** — `t.Setenv("EMPTY_VAR", "")`. Input
`{"Auth": "Bearer ${EMPTY_VAR}"}`. Verify error is returned (empty is treated as unset).

**Test: `TestExpandHeaders_NilMap`** — Input `nil`. Verify output is nil. Verify no error.

**Test: `TestExpandHeaders_EmptyMap`** — Input `map[string]string{}`. Verify output is
empty map. Verify no error.

**Test: `TestExpandHeaders_NoPattern`** — Input `{"Key": "plain-value"}`. Verify output
equals input unchanged.

**Test: `TestExpandHeaders_DollarWithoutBrace`** — Input `{"Key": "$TOKEN"}`. Verify
output equals input unchanged (only `${...}` is expanded).

**Test: `TestExpandHeaders_InputNotModified`** — Create input map. Call
`ExpandHeaders`. Verify original input map is unchanged (new map returned).

### 7.3 `internal/mcpclient/mcpclient_test.go`

These tests require a real MCP server (use the SDK's server utilities or a minimal
httptest-based JSON-RPC handler that speaks the MCP protocol).

**Test: `TestConnect_StreamableHTTP_Success`** — Start a test MCP server using
`streamable-http` transport. Create `MCPServerConfig` with the test server URL and
`transport = "streamable-http"`. Call `Connect()`. Verify no error. Verify `Client` is
non-nil. Call `client.Close()`.

**Test: `TestConnect_InvalidTransport`** — Create `MCPServerConfig` with
`transport = "invalid"`. Call `Connect()`. Verify error is returned.

**Test: `TestConnect_UnreachableServer`** — Create `MCPServerConfig` with URL pointing
to a closed port (`http://127.0.0.1:<closed-port>`). Call `Connect()` with a short
timeout context. Verify error is returned.

**Test: `TestConnect_HeaderInjection`** — Start a test MCP server that records incoming
request headers. Set `t.Setenv("TEST_TOKEN", "mytoken")`. Create `MCPServerConfig` with
`headers = {"Authorization": "Bearer ${TEST_TOKEN}"}`. Call `Connect()`. Verify the
server received the `Authorization: Bearer mytoken` header.

**Test: `TestConnect_HeaderEnvVarMissing`** — Create `MCPServerConfig` with
`headers = {"Auth": "Bearer ${MISSING}"}`. Do not set `MISSING`. Call `Connect()`. Verify
error is returned. Verify error message contains `"MISSING"`.

**Test: `TestListTools_ReturnsTools`** — Start a test MCP server that exposes 2 tools
(e.g., `greet` with string param `name`, `add` with integer params `a` and `b`). Connect.
Call `ListTools()`. Verify 2 tools returned. Verify tool names match. Verify parameter
types and required flags are correct.

**Test: `TestListTools_EmptyTools`** — Start a test MCP server that exposes zero tools.
Connect. Call `ListTools()`. Verify empty slice returned. Verify no error.

**Test: `TestListTools_ComplexSchema`** — Start a test MCP server with a tool that has
an `object`-typed parameter. Connect. Call `ListTools()`. Verify the parameter type is
`"string"`. Verify the description contains `"(JSON Schema type: object)"`.

**Test: `TestCallTool_Success`** — Start a test MCP server with a tool `echo` that
returns the input as `TextContent`. Connect. Call `CallTool()` with arguments. Verify
result `IsError` is false. Verify result `Content` contains the expected text. Verify
`CallID` matches.

**Test: `TestCallTool_ToolError`** — Start a test MCP server with a tool that returns
`IsError: true`. Connect. Call `CallTool()`. Verify result `IsError` is true. Verify
content contains error text.

**Test: `TestCallTool_NoTextContent`** — Start a test MCP server with a tool that
returns only non-text content (or empty content array). Connect. Call `CallTool()`.
Verify result `IsError` is true. Verify content contains
`"MCP tool returned no text content"`.

**Test: `TestCallTool_MultipleTextContent`** — Start a test MCP server with a tool that
returns two `TextContent` items. Connect. Call `CallTool()`. Verify result content
contains both texts joined by `"\n"`.

**Test: `TestClientName`** — Create a client with server name `"my-server"`. Verify
`client.Name()` returns `"my-server"`.

### 7.4 `internal/mcpclient/router_test.go`

**Test: `TestRouter_RegisterAndDispatch`** — Create a router. Register a mock client
with one tool. Verify `Has()` returns true for that tool name. Dispatch a call. Verify
the call reaches the client and returns a result.

**Test: `TestRouter_Has_UnknownTool`** — Create an empty router. Verify `Has("unknown")`
returns false.

**Test: `TestRouter_Dispatch_UnknownTool`** — Create an empty router. Call `Dispatch()`
with unknown tool name. Verify error returned. Verify error message contains the tool name.

**Test: `TestRouter_Register_SkipsBuiltins`** — Create a router. Register a client with
tools `["read_file", "custom_tool"]` and `builtinNames = {"read_file": true}`. Verify
`Has("read_file")` returns false. Verify `Has("custom_tool")` returns true. Verify
returned tools slice has length 1.

**Test: `TestRouter_Register_MCPCollision`** — Create a router. Register client-A with
tool `"search"`. Then register client-B with tool `"search"`. Verify error returned.
Verify error contains both server names and the tool name `"search"`.

**Test: `TestRouter_Close_ClosesAllClients`** — Create a router with 2 clients, each
owning different tools. Call `Close()`. Verify both clients' sessions are closed.

**Test: `TestRouter_Close_DeduplicatesClients`** — Create a router. Register one client
with 3 tools. Call `Close()`. Verify the client is closed exactly once (not 3 times).

### 7.5 `internal/agent/agent_test.go` Additions

**Test: `TestValidate_MCPServers_Valid`** — Config with one valid MCP server entry.
Verify `Validate()` returns nil.

**Test: `TestValidate_MCPServers_MissingName`** — Config with MCP server entry where
`Name` is empty. Verify error contains `"mcp_servers[0]: name is required"`.

**Test: `TestValidate_MCPServers_MissingURL`** — Config with MCP server entry where
`URL` is empty. Verify error contains `"mcp_servers[0]: url is required"`.

**Test: `TestValidate_MCPServers_InvalidTransport`** — Config with
`Transport = "websocket"`. Verify error contains `"transport must be"` and `"websocket"`.

**Test: `TestValidate_MCPServers_ValidTransports`** — Verify both `"sse"` and
`"streamable-http"` pass validation.

**Test: `TestValidate_MCPServers_DuplicateNames`** — Config with two MCP server entries
both named `"tools"`. Verify error contains `"duplicate server name"`.

**Test: `TestParseTOML_MCPServers`** — Parse a TOML string with `[[mcp_servers]]`
entries. Verify all fields are populated correctly including `Headers` map.

**Test: `TestParseTOML_MCPServers_Empty`** — Parse a TOML string with no `mcp_servers`.
Verify `cfg.MCPServers` is nil.

### 7.6 `cmd/run_test.go` Additions

**Test: `TestRunCmd_DryRun_WithMCPServers`** — Create a fixture agent with MCP server
config. Run with `--dry-run`. Verify stdout contains `--- MCP Servers ---` section.
Verify the server name, URL, and transport appear in the output.

**Test: `TestRunCmd_DryRun_NoMCPServers`** — Existing fixture agent with no MCP config.
Run with `--dry-run`. Verify stdout contains `--- MCP Servers ---` followed by `(none)`.

### 7.7 Existing Tests

All existing tests must continue to pass without modification:

- All tests in `internal/tool/`
- All tests in `internal/agent/` (existing tests)
- All tests in `internal/provider/`
- All tests in `internal/resolve/`
- All tests in `internal/memory/`
- All tests in `internal/toolname/`
- All tests in `cmd/` (existing tests)

### 7.8 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `MCPServerConfig` struct exists with correct fields and tags | `TestParseTOML_MCPServers` passes |
| `MCPServers` field on `AgentConfig` parses `[[mcp_servers]]` | `TestParseTOML_MCPServers` passes |
| Validation rejects missing name | `TestValidate_MCPServers_MissingName` passes |
| Validation rejects missing URL | `TestValidate_MCPServers_MissingURL` passes |
| Validation rejects invalid transport | `TestValidate_MCPServers_InvalidTransport` passes |
| Validation accepts `"sse"` and `"streamable-http"` | `TestValidate_MCPServers_ValidTransports` passes |
| Validation rejects duplicate server names | `TestValidate_MCPServers_DuplicateNames` passes |
| `envinterp.ExpandHeaders` expands `${VAR}` patterns | `TestExpandHeaders_SingleVar` passes |
| `envinterp.ExpandHeaders` fails on missing env var | `TestExpandHeaders_MissingVar` passes |
| `envinterp.ExpandHeaders` fails on empty env var | `TestExpandHeaders_EmptyVar` passes |
| `envinterp.ExpandHeaders` handles nil input | `TestExpandHeaders_NilMap` passes |
| MCP client connects to streamable-http server | `TestConnect_StreamableHTTP_Success` passes |
| MCP client injects headers into requests | `TestConnect_HeaderInjection` passes |
| MCP client fails on missing header env var | `TestConnect_HeaderEnvVarMissing` passes |
| MCP client discovers tools | `TestListTools_ReturnsTools` passes |
| MCP client handles complex schema types | `TestListTools_ComplexSchema` passes |
| MCP client calls tools and returns text result | `TestCallTool_Success` passes |
| MCP client handles tool errors | `TestCallTool_ToolError` passes |
| MCP client handles no-text-content results | `TestCallTool_NoTextContent` passes |
| MCP client concatenates multiple text contents | `TestCallTool_MultipleTextContent` passes |
| Router registers tools and dispatches correctly | `TestRouter_RegisterAndDispatch` passes |
| Router skips built-in collisions | `TestRouter_Register_SkipsBuiltins` passes |
| Router errors on MCP-to-MCP collisions | `TestRouter_Register_MCPCollision` passes |
| Router closes all clients on Close() | `TestRouter_Close_ClosesAllClients` passes |
| Dry-run shows MCP server config | `TestRunCmd_DryRun_WithMCPServers` passes |
| Dry-run shows `(none)` when no MCP servers | `TestRunCmd_DryRun_NoMCPServers` passes |
| Scaffold includes MCP server example | Manual verification of `Scaffold()` output |
| JSON output includes MCP tool calls | Integration test verification |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Axe as an MCP server (exposing axe tools via MCP)
2. `"stdio"` transport (launching MCP servers as subprocesses)
3. MCP resources (`resources/list`, `resources/read`)
4. MCP prompts (`prompts/list`, `prompts/get`)
5. MCP sampling (client-side LLM calls requested by the server)
6. MCP roots (filesystem root exposure)
7. MCP progress notifications
8. MCP cancellation notifications
9. MCP logging from server
10. Tool list change notifications (`listChanged`)
11. Retry logic for MCP connections or tool calls
12. Connection pooling or reuse across agent runs
13. Caching of tool definitions
14. Per-MCP-server timeout configuration (inherits `--timeout`)
15. Filtering which tools to expose from an MCP server (all discovered tools are used, minus collisions)
16. OAuth or other advanced authentication flows (only static headers with env var interpolation)
17. MCP session resumability or reconnection
18. Streaming tool results from MCP servers
19. `ImageContent` or `EmbeddedResource` handling (non-text content is dropped)
20. Changes to `internal/provider/provider.go` types
21. Changes to `internal/toolname/toolname.go`
22. MCP tool names in `--dry-run` output (requires live connection)

---

## 10. References

- GitHub Issue: https://github.com/jrswab/axe/issues/9
- MCP Specification (2025-03-26): https://modelcontextprotocol.io/specification/2025-03-26
- MCP Transports: https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
- MCP Lifecycle: https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle
- Official Go SDK: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
- `AgentConfig` struct: `internal/agent/agent.go:37`
- `Validate()` function: `internal/agent/agent.go:55`
- `Scaffold()` function: `internal/agent/agent.go:157`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ExecuteOptions` struct: `internal/tool/tool.go:26`
- `executeToolCalls()` function: `cmd/run.go:540`
- `runConversationLoop()` function: `internal/tool/tool.go:304`
- `RegisterAll()` function: `internal/tool/registry.go:92`
- Tool call milestones: `docs/plans/000_tool_call_milestones.md`
- Main milestones: `docs/plans/000_milestones.md`

---

## 11. Notes

- **Why the official SDK?** The MCP protocol involves JSON-RPC 2.0 framing, SSE parsing,
  streamable HTTP content-type negotiation, session ID tracking, and the
  initialize/initialized handshake. Implementing this correctly from scratch would be
  hundreds of lines of protocol code. The official Go SDK is maintained by the MCP
  project team and handles all transport edge cases. The trade-off (one new dependency)
  is justified by the complexity avoided.

- **Why built-in wins on collision?** Built-in tools have known, tested behavior within
  axe. An MCP server could theoretically expose a tool with the same name but different
  semantics. Letting the built-in win prevents surprises. Users who want the MCP version
  can remove the built-in from their `tools` list.

- **Why fail-fast on MCP connection failure?** The agent author declared the MCP server
  as a dependency. If it's unreachable, the agent cannot fulfill its purpose. Proceeding
  without tools leads to silent degradation that's hard to debug. Fail-fast with a clear
  error message is more useful.

- **Why no `stdio` transport?** Axe is "executor, not scheduler." The `stdio` transport
  requires launching and managing a child process (the MCP server). This adds process
  lifecycle management (startup, health, shutdown, signal handling) that conflicts with
  axe's "single binary, zero runtime" philosophy. SSE and streamable-HTTP connect to
  already-running servers, which aligns with the Unix composition model.

- **Sub-agent isolation.** Each sub-agent establishes its own MCP connections because
  sub-agents are opaque to parents (AGENTS.md constraint). Sharing connections would
  leak the parent's state to the sub-agent and vice versa. The overhead of extra
  connections is acceptable given that axe runs single-purpose agents that exit quickly.

- **JSON Schema flattening.** The flattening strategy (complex types become `"string"`)
  works because the LLM reads the tool description and parameter descriptions to
  understand expected formats. The `Type` field is a hint, not a strict contract. LLMs
  already handle this well — they produce JSON strings that the MCP server can parse.

- **`map[string]string` to `map[string]any`.** Axe's provider abstraction normalizes all
  tool arguments to `map[string]string`. This is a simplification that works for all
  built-in tools. For MCP tools, converting string values to `any` and letting the MCP
  server handle type coercion is the least-invasive approach. Changing the provider
  abstraction to `map[string]any` would require touching every provider implementation
  and every built-in tool — a much larger change that should be considered separately if
  needed.
