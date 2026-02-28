# Specification: M5 - Sub-Agents

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-27
**Scope:** Tool-calling infrastructure, `call_agent` tool injection, sub-agent execution, depth limiting, parallel execution, error handling

---

## 1. Purpose

Enable parent agents to delegate work to child agents via LLM tool calling. When an agent's TOML configuration includes a `sub_agents` list, Axe injects a `call_agent` tool into the LLM request. The LLM can then invoke sub-agents by name, passing a task description and optional context. The sub-agent runs independently with its own configuration (system prompt, skill, files) plus the parent's task and context. Only the sub-agent's final text result returns to the parent. This keeps the parent's context window small while leveraging specialized agents for subtasks.

This milestone introduces tool-calling support into the provider layer, a multi-turn conversation loop in the run command, and the `call_agent` tool execution engine.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Tool-calling scope:** Only the `call_agent` tool is supported in M5. No other tools are defined. The tool-calling infrastructure must support future tools, but M5 only implements `call_agent`.
2. **Provider-specific tool formats:** Each provider has its own wire format for tool definitions and tool-call responses. The provider layer abstracts these differences behind shared types (`Tool`, `ToolCall`, `ToolResult`). Provider implementations translate to/from their native format.
3. **Single tool name:** The injected tool name is always `call_agent`. This name is a constant, not configurable.
4. **Sub-agent isolation:** A sub-agent receives its own TOML-defined configuration (system prompt, skill, files, workdir). It does NOT inherit the parent's system prompt, skill, files, conversation history, or workdir. The only data flowing from parent to sub-agent is the `task` and `context` strings from the tool call.
5. **Result format:** Sub-agents return plain text only (v1). The parent receives the sub-agent's final `Response.Content` as the tool result text. No structured output, no metadata, no token counts.
6. **Depth tracking:** Depth is tracked via a parameter passed through the call chain, not via global state. The initial `axe run` invocation starts at depth 0. Each sub-agent invocation increments depth by 1.
7. **No new CLI commands or flags:** M5 does not add new subcommands or flags to the CLI. The behavior is triggered by the presence of `sub_agents` in the agent TOML.
8. **Dependencies:** Continue using stdlib-only (`net/http`, `encoding/json`). No LLM SDK dependencies.

---

## 3. Requirements

### 3.1 Tool-Calling Types (`internal/provider/`)

**Requirement 1.1:** Define a `Tool` struct representing a tool definition sent to the LLM:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Name` | `string` | Tool name (e.g. `"call_agent"`) |
| `Description` | `string` | Human-readable description for the LLM |
| `Parameters` | `map[string]ToolParameter` | Map of parameter name to parameter definition |

**Requirement 1.2:** Define a `ToolParameter` struct:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Type` | `string` | JSON Schema type (e.g. `"string"`) |
| `Description` | `string` | Human-readable description for the LLM |
| `Required` | `bool` | Whether the parameter is required |

**Requirement 1.3:** Define a `ToolCall` struct representing a tool invocation from the LLM:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `ID` | `string` | Provider-assigned unique ID for this tool call |
| `Name` | `string` | Tool name (e.g. `"call_agent"`) |
| `Arguments` | `map[string]string` | Parsed arguments (parameter name to value) |

**Requirement 1.4:** Define a `ToolResult` struct representing the result of a tool execution:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `CallID` | `string` | The `ToolCall.ID` this result corresponds to |
| `Content` | `string` | The text result of the tool execution |
| `IsError` | `bool` | Whether the result represents an error |

**Requirement 1.5:** Add a `Tools` field to the existing `Request` struct:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Tools` | `[]Tool` | Tool definitions to send to the LLM. If nil or empty, no tools are sent. |

**Requirement 1.6:** Add a `ToolCalls` field to the existing `Response` struct:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `ToolCalls` | `[]ToolCall` | Tool calls requested by the LLM. Empty if the LLM did not call any tools. |

**Requirement 1.7:** The `Message` struct must be extended to support tool-call and tool-result message roles. Add a `ToolCalls` field and a `ToolResults` field:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `ToolCalls` | `[]ToolCall` | Tool calls in an assistant message (non-nil when role is `"assistant"` and LLM called tools) |
| `ToolResults` | `[]ToolResult` | Tool results in a tool-result message (non-nil when role is `"tool"`) |

When `ToolCalls` is non-nil, the `Content` field may be empty (the LLM may or may not include text alongside tool calls). When `ToolResults` is non-nil, the `Content` field is ignored.

**Requirement 1.8:** The new fields (`Tools` on `Request`, `ToolCalls` on `Response`, `ToolCalls`/`ToolResults` on `Message`) must have zero-value defaults that preserve existing behavior. When `Tools` is nil, the provider must not include tool definitions in the API request. When `ToolCalls` is nil on a response, the caller treats it as a text-only response. This ensures all existing code (M1-M4) continues to work without modification.

**Requirement 1.9:** The `StopReason` field on `Response` must reflect tool-call stops. When the LLM stops because it wants to call tools:
- Anthropic: `StopReason` will be `"tool_use"`
- OpenAI: `StopReason` will be `"tool_calls"`
- Ollama: `StopReason` will be `"tool_calls"` (if supported) or empty string

The caller uses `len(Response.ToolCalls) > 0` to detect tool calls, not the `StopReason` string. `StopReason` is informational only.

### 3.2 Anthropic Provider Tool-Calling Support

**Requirement 2.1:** When `Request.Tools` is non-nil and non-empty, the Anthropic provider must include a `tools` array in the JSON request body. Each tool must conform to the Anthropic tool format:

```json
{
  "tools": [
    {
      "name": "call_agent",
      "description": "...",
      "input_schema": {
        "type": "object",
        "properties": {
          "agent": {"type": "string", "description": "..."},
          "task": {"type": "string", "description": "..."},
          "context": {"type": "string", "description": "..."}
        },
        "required": ["agent", "task"]
      }
    }
  ]
}
```

**Requirement 2.2:** The `input_schema` must be a JSON Schema object. The `properties` map is built from `Tool.Parameters`. The `required` array contains names of parameters where `ToolParameter.Required` is `true`.

**Requirement 2.3:** When `Request.Tools` is nil or empty, the `tools` key must be omitted from the JSON request body entirely. This preserves M4 behavior.

**Requirement 2.4:** The Anthropic provider must parse tool-call content blocks from the response. The Anthropic API returns tool calls as content blocks with `type: "tool_use"`:

```json
{
  "content": [
    {"type": "text", "text": "I'll delegate this..."},
    {"type": "tool_use", "id": "toolu_xxx", "name": "call_agent", "input": {"agent": "test-runner", "task": "run tests"}}
  ],
  "stop_reason": "tool_use"
}
```

For each `tool_use` content block, create a `ToolCall` with:
- `ID`: the `id` field
- `Name`: the `name` field
- `Arguments`: the `input` object with all values converted to strings

**Requirement 2.5:** When tool-call content blocks are present alongside text content blocks, the `Response.Content` field must contain the concatenated text from all `text` content blocks. If there are no `text` content blocks, `Response.Content` must be an empty string.

**Requirement 2.6:** The Anthropic provider must support sending tool results back to the LLM. When a `Message` has role `"tool"` and non-nil `ToolResults`, the Anthropic provider must format it as a message with role `"user"` containing `tool_result` content blocks:

```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "<CallID>",
      "content": "<Content>",
      "is_error": <IsError>
    }
  ]
}
```

**Requirement 2.7:** The Anthropic provider must support sending assistant messages that contain tool calls. When a `Message` has role `"assistant"` and non-nil `ToolCalls`, the provider must format it with `tool_use` content blocks (in addition to any text content) to maintain conversation history:

```json
{
  "role": "assistant",
  "content": [
    {"type": "text", "text": "<Content>"},
    {"type": "tool_use", "id": "<ID>", "name": "<Name>", "input": <Arguments>}
  ]
}
```

If `Content` is empty, omit the `text` content block. There must be at least one content block.

### 3.3 OpenAI Provider Tool-Calling Support

**Requirement 3.1:** When `Request.Tools` is non-nil and non-empty, the OpenAI provider must include a `tools` array in the JSON request body. Each tool must conform to the OpenAI tool format:

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "call_agent",
        "description": "...",
        "parameters": {
          "type": "object",
          "properties": {
            "agent": {"type": "string", "description": "..."},
            "task": {"type": "string", "description": "..."},
            "context": {"type": "string", "description": "..."}
          },
          "required": ["agent", "task"]
        }
      }
    }
  ]
}
```

**Requirement 3.2:** When `Request.Tools` is nil or empty, the `tools` key must be omitted from the JSON request body entirely.

**Requirement 3.3:** The OpenAI provider must parse tool calls from the response. The OpenAI API returns tool calls in the `tool_calls` array of the choice message:

```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [
        {
          "id": "call_xxx",
          "type": "function",
          "function": {
            "name": "call_agent",
            "arguments": "{\"agent\": \"test-runner\", \"task\": \"run tests\"}"
          }
        }
      ]
    },
    "finish_reason": "tool_calls"
  }]
}
```

For each tool call, create a `ToolCall` with:
- `ID`: the `id` field
- `Name`: the `function.name` field
- `Arguments`: parse `function.arguments` as JSON and convert all values to strings

**Requirement 3.4:** If `function.arguments` is not valid JSON, create a `ToolCall` with empty `Arguments` and set the `Name` field normally. The caller will handle the missing arguments as an error when executing the tool.

**Requirement 3.5:** When the OpenAI response contains `tool_calls` and `content` is null or empty, `Response.Content` must be an empty string.

**Requirement 3.6:** The OpenAI provider must support sending tool results. When a `Message` has role `"tool"` and non-nil `ToolResults`, the OpenAI provider must send one message per tool result:

```json
{
  "role": "tool",
  "tool_call_id": "<CallID>",
  "content": "<Content>"
}
```

Each `ToolResult` becomes a separate message in the messages array.

**Requirement 3.7:** The OpenAI provider must support sending assistant messages with tool calls. When a `Message` has role `"assistant"` and non-nil `ToolCalls`, the provider must include the `tool_calls` array:

```json
{
  "role": "assistant",
  "content": "<Content or null>",
  "tool_calls": [
    {
      "id": "<ID>",
      "type": "function",
      "function": {
        "name": "<Name>",
        "arguments": "<JSON-encoded arguments>"
      }
    }
  ]
}
```

### 3.4 Ollama Provider Tool-Calling Support

**Requirement 4.1:** When `Request.Tools` is non-nil and non-empty, the Ollama provider must include a `tools` array in the JSON request body. Each tool must conform to the Ollama tool format:

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "call_agent",
        "description": "...",
        "parameters": {
          "type": "object",
          "properties": {
            "agent": {"type": "string", "description": "..."},
            "task": {"type": "string", "description": "..."},
            "context": {"type": "string", "description": "..."}
          },
          "required": ["agent", "task"]
        }
      }
    }
  ]
}
```

**Requirement 4.2:** When `Request.Tools` is nil or empty, the `tools` key must be omitted from the JSON request body entirely.

**Requirement 4.3:** The Ollama provider must parse tool calls from the response. The Ollama API returns tool calls in the `message.tool_calls` array:

```json
{
  "message": {
    "role": "assistant",
    "content": "",
    "tool_calls": [
      {
        "function": {
          "name": "call_agent",
          "arguments": {"agent": "test-runner", "task": "run tests"}
        }
      }
    ]
  }
}
```

For each tool call, create a `ToolCall` with:
- `ID`: Ollama does not return tool call IDs. Generate a unique ID using the format `ollama_<index>` where `<index>` is the zero-based position in the `tool_calls` array (e.g. `"ollama_0"`, `"ollama_1"`).
- `Name`: the `function.name` field
- `Arguments`: the `function.arguments` object with all values converted to strings

**Requirement 4.4:** The Ollama provider must support sending tool results. When a `Message` has role `"tool"` and non-nil `ToolResults`, the Ollama provider must send a message with role `"tool"`:

```json
{
  "role": "tool",
  "content": "<Content>"
}
```

Ollama does not support `tool_call_id` in tool messages. If multiple `ToolResult` entries exist, send one `"tool"` message per result.

**Requirement 4.5:** The Ollama provider must support sending assistant messages with tool calls. When a `Message` has role `"assistant"` and non-nil `ToolCalls`, include the `tool_calls` array in the assistant message.

**Requirement 4.6:** Not all Ollama models support tool calling. If a model does not support tools and `Request.Tools` is non-nil, the model will ignore the tools and return a text-only response. The provider must handle this gracefully: if the response has no `tool_calls` array, return a normal text response with empty `ToolCalls`. The caller (conversation loop) handles the case where the LLM never calls the injected tool.

### 3.5 Agent Config Extension (`internal/agent/`)

**Requirement 5.1:** Add a `SubAgentsConfig` struct to the agent package:

| Go Field | TOML Key | Go Type | Default | Description |
|----------|----------|---------|---------|-------------|
| `MaxDepth` | `max_depth` | `int` | `0` | Maximum sub-agent nesting depth. `0` means use the system default (3). |
| `Parallel` | `parallel` | `bool` | `true` | Whether to run concurrent tool calls in parallel. |
| `Timeout` | `timeout` | `int` | `0` | Per sub-agent timeout in seconds. `0` means use the parent's `--timeout` value. |

**Requirement 5.2:** Add a `SubAgentsConf` field to `AgentConfig`:

```go
SubAgentsConf SubAgentsConfig `toml:"sub_agents_config"`
```

**Requirement 5.3:** The existing `SubAgents []string` field on `AgentConfig` is unchanged. It remains the list of allowed sub-agent names.

**Requirement 5.4:** Validation: If `SubAgentsConf.MaxDepth` is set to a value greater than 5, the `Validate` function must return an error: `sub_agents_config.max_depth cannot exceed 5`. If `MaxDepth` is negative, return an error: `sub_agents_config.max_depth must be non-negative`.

**Requirement 5.5:** Validation: If `SubAgentsConf.Timeout` is negative, the `Validate` function must return an error: `sub_agents_config.timeout must be non-negative`.

**Requirement 5.6:** The `Scaffold` function must include a commented-out `[sub_agents_config]` section in the template:

```toml
# [sub_agents_config]
# max_depth = 3
# parallel = true
# timeout = 120
```

**Requirement 5.7:** The `agents show` command must display `SubAgentsConfig` fields when `SubAgents` is non-empty.

### 3.6 `call_agent` Tool Definition (`internal/tool/`)

**Requirement 6.1:** Create a new package `internal/tool/` for tool definitions and execution logic.

**Requirement 6.2:** Define a constant for the tool name:

```go
const CallAgentToolName = "call_agent"
```

**Requirement 6.3:** Define a function that returns the `call_agent` tool definition:

```go
func CallAgentTool(allowedAgents []string) provider.Tool
```

The returned `Tool` must have:
- `Name`: `"call_agent"`
- `Description`: `"Delegate a task to a sub-agent. The sub-agent runs independently with its own context and returns only its final result. Available agents: <comma-separated list of allowedAgents>"`
- `Parameters`: three parameters:
  - `"agent"`: type `"string"`, description `"Name of the sub-agent to invoke (must be one of: <comma-separated list>)"`, required `true`
  - `"task"`: type `"string"`, description `"What you need the sub-agent to do"`, required `true`
  - `"context"`: type `"string"`, description `"Additional context from your conversation to pass along"`, required `false`

**Requirement 6.4:** The `allowedAgents` list in the description and parameter description must be dynamically populated from the parent's `SubAgents` config field. This tells the LLM exactly which agent names are valid.

### 3.7 Sub-Agent Execution (`internal/tool/`)

**Requirement 7.1:** Define a function for executing a `call_agent` tool call:

```go
func ExecuteCallAgent(ctx context.Context, call provider.ToolCall, opts ExecuteOptions) provider.ToolResult
```

This function always returns a `ToolResult` (never an error). Errors are communicated via the `ToolResult.Content` and `ToolResult.IsError` fields.

**Requirement 7.2:** Define an `ExecuteOptions` struct:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `AllowedAgents` | `[]string` | List of agent names the parent is allowed to call |
| `ParentModel` | `string` | The parent's full `provider/model` string (used as fallback for sub-agent if sub-agent has same provider) |
| `Depth` | `int` | Current nesting depth (0 = top-level) |
| `MaxDepth` | `int` | Maximum allowed depth (default 3, hard max 5) |
| `Timeout` | `int` | Per sub-agent timeout in seconds |
| `GlobalConfig` | `*config.GlobalConfig` | Global config for API key/base URL resolution |
| `Verbose` | `bool` | Whether to print verbose output to stderr |
| `Stderr` | `io.Writer` | Writer for verbose/debug output |

**Requirement 7.3:** The `ExecuteCallAgent` function must perform the following steps in order:

1. Extract `agent`, `task`, and `context` arguments from `call.Arguments`.
2. Validate that `agent` is non-empty. If empty, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: \"agent\" argument is required"`.
3. Validate that `task` is non-empty. If empty, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: \"task\" argument is required"`.
4. Validate that `agent` is in the `AllowedAgents` list. If not, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: agent \"<name>\" is not in this agent's sub_agents list"`.
5. Check depth: if `Depth >= MaxDepth`, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: maximum sub-agent depth (<MaxDepth>) reached"`.
6. Load the sub-agent's TOML config via `agent.Load(agentName)`. If loading fails, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: failed to load agent \"<name>\": <error>"`.
7. Parse the sub-agent's model string. If parsing fails, return a `ToolResult` with `IsError: true` and `Content: "call_agent error: invalid model for agent \"<name>\": <error>"`.
8. Resolve the sub-agent's working directory, file globs, skill, and system prompt using the sub-agent's own TOML config (NOT the parent's config).
9. Resolve the sub-agent's API key and base URL via `GlobalConfig`.
10. Create the sub-agent's provider via `provider.New(...)`.
11. Build the sub-agent's user message by combining `task` and `context`:
    - If `context` is non-empty: `"Task: <task>\n\nContext:\n<context>"`
    - If `context` is empty: `"Task: <task>"`
12. Build the sub-agent's `Request` with the sub-agent's own model, system prompt, the constructed user message, the sub-agent's temperature and max_tokens, and tools (if the sub-agent itself has `sub_agents` AND `Depth + 1 < MaxDepth`).
13. Create a timeout context. If `Timeout > 0`, use that value. Otherwise, use the context deadline from the parent context (if any).
14. Call the sub-agent's provider. If the sub-agent has tools (nested sub-agents), run the same conversation loop described in Requirement 8.1.
15. Return a `ToolResult` with `CallID: call.ID`, `Content: <final response text>`, `IsError: false`.

**Requirement 7.4:** If the sub-agent's provider returns an error at any point (API error, timeout, etc.), return a `ToolResult` with `IsError: true` and `Content: "Error: sub-agent \"<name>\" failed - <error message>. You may retry or proceed without this result."`.

**Requirement 7.5:** If verbose mode is enabled, print the following to stderr before the sub-agent call:

```
[sub-agent] Calling "<agent>" (depth <N>) with task: <first 80 chars of task>...
```

And after the sub-agent call completes:

```
[sub-agent] "<agent>" completed in <N>ms (<M> chars returned)
```

Or on failure:

```
[sub-agent] "<agent>" failed: <error>
```

### 3.8 Conversation Loop (`cmd/run.go`)

**Requirement 8.1:** Replace the current single-shot `prov.Send(ctx, req)` call with a conversation loop. The loop must:

1. Send the request to the provider.
2. If `Response.ToolCalls` is empty, the loop ends. Output the response as before.
3. If `Response.ToolCalls` is non-empty:
   a. Append an assistant message to the conversation. This message has `Role: "assistant"`, `Content: Response.Content` (may be empty), and `ToolCalls: Response.ToolCalls`.
   b. Execute each tool call (see Requirement 8.2).
   c. Append a tool-result message to the conversation. This message has `Role: "tool"` and `ToolResults` containing the results of all tool calls.
   d. Send the updated conversation back to the provider (go to step 1).

**Requirement 8.2:** Tool call execution:
- If `SubAgentsConf.Parallel` is `true` (default) and there are multiple tool calls, execute them concurrently using goroutines. Collect all results before proceeding.
- If `SubAgentsConf.Parallel` is `false`, execute tool calls sequentially in the order they appear.
- Each tool call must be dispatched by name. In M5, only `"call_agent"` is a valid tool name. If the LLM calls an unknown tool name, return a `ToolResult` with `IsError: true` and `Content: "Unknown tool: \"<name>\""`.

**Requirement 8.3:** The conversation loop must have a maximum iteration limit of 50 turns to prevent runaway loops. If the loop exceeds 50 iterations, return an `ExitError` with code 1 and message: `agent exceeded maximum conversation turns (50)`. A "turn" is one send-receive cycle.

**Requirement 8.4:** The conversation loop must only be active when `Request.Tools` is non-nil. If no tools are defined (agent has no `sub_agents`), the current single-shot behavior is preserved exactly.

**Requirement 8.5:** The `--dry-run` output must be extended to show sub-agent information. After the existing sections, add:

```
--- Sub-Agents ---
<agent1>, <agent2>, ...
Max Depth: <N>
Parallel:  <yes|no>
Timeout:   <N>s
```

If `sub_agents` is empty, print `(none)` and omit the configuration lines.

**Requirement 8.6:** The `--json` output must be extended with a `tool_calls` field when the final response was preceded by tool-call turns:

```json
{
  "model": "...",
  "content": "...",
  "input_tokens": <total_across_all_turns>,
  "output_tokens": <total_across_all_turns>,
  "stop_reason": "...",
  "duration_ms": <total_wall_clock>,
  "tool_calls": <number_of_tool_calls_executed>
}
```

If no tool calls were made, the `tool_calls` field must be `0`.

**Requirement 8.7:** The `--verbose` output must log each conversation turn. Before each send:

```
[turn <N>] Sending request (<M> messages, <T> tool calls pending)
```

After each receive:

```
[turn <N>] Received response: <stop_reason> (<K> tool calls)
```

**Requirement 8.8:** Token counts in verbose output and JSON output must be cumulative across all turns. `InputTokens` is the sum of all `Response.InputTokens` across all turns. `OutputTokens` is the sum of all `Response.OutputTokens` across all turns.

### 3.9 Tool Injection in Run Command

**Requirement 9.1:** After loading the agent config and before building the request, check if `cfg.SubAgents` is non-nil and non-empty. If so, and the current depth is less than the effective max depth, call `tool.CallAgentTool(cfg.SubAgents)` to get the tool definition and include it in `Request.Tools`.

**Requirement 9.2:** If `cfg.SubAgents` is nil or empty, set `Request.Tools` to nil. No tool injection occurs. The run command behaves identically to M4.

**Requirement 9.3:** The effective max depth is determined as follows:
- If `cfg.SubAgentsConf.MaxDepth` is greater than 0 and less than or equal to 5, use that value.
- If `cfg.SubAgentsConf.MaxDepth` is 0 (not set), use the system default of 3.
- The hard maximum is 5. Values above 5 are rejected at validation time (Requirement 5.4).

**Requirement 9.4:** At the top-level `axe run` invocation, depth is 0. This value is not exposed to the user. It is only relevant internally when executing `call_agent`.

### 3.10 Depth Tracking

**Requirement 10.1:** Depth is an integer starting at 0 for the top-level agent.

**Requirement 10.2:** When `ExecuteCallAgent` invokes a sub-agent, it passes `Depth + 1` to the sub-agent's execution context.

**Requirement 10.3:** When building the sub-agent's request tools: if the sub-agent has `sub_agents` defined AND `Depth + 1 < MaxDepth`, inject the `call_agent` tool. Otherwise, do not inject any tools. This means the sub-agent at the depth limit runs as a simple single-shot agent with no tool-calling capability.

**Requirement 10.4:** The hard maximum depth is 5. Even if a parent sets `max_depth = 5`, a chain of 5 sub-agent calls is the absolute limit. The 6th level would have `Depth = 5` which is `>= MaxDepth`, so no tools are injected.

### 3.11 Parallel Execution

**Requirement 11.1:** When multiple `call_agent` tool calls appear in a single LLM response and `SubAgentsConf.Parallel` is `true`, execute all tool calls concurrently using goroutines.

**Requirement 11.2:** Each goroutine must have its own timeout context derived from the parent context. If the parent context is cancelled, all sub-agent goroutines must be cancelled.

**Requirement 11.3:** Results must be collected in the same order as the original `ToolCalls` slice, regardless of completion order.

**Requirement 11.4:** If one sub-agent fails, the others must not be cancelled. Each sub-agent runs independently. The failed sub-agent returns an error `ToolResult`; the successful ones return their results. All results are sent back to the parent LLM together.

**Requirement 11.5:** When `SubAgentsConf.Parallel` is `false`, execute tool calls sequentially in the order they appear in the `ToolCalls` slice.

### 3.12 Timeout Handling

**Requirement 12.1:** If `SubAgentsConf.Timeout` is greater than 0, each sub-agent invocation must use a `context.WithTimeout` derived from the parent context, with the specified timeout in seconds.

**Requirement 12.2:** If `SubAgentsConf.Timeout` is 0, sub-agent invocations inherit the parent's context directly (which has the `--timeout` flag's deadline). This means sub-agents share the parent's overall timeout budget.

**Requirement 12.3:** If a sub-agent's context deadline is exceeded, the sub-agent's provider call is aborted and the `ExecuteCallAgent` function returns a `ToolResult` with `IsError: true` and `Content: "Error: sub-agent \"<name>\" failed - timeout after <N>s. You may retry or proceed without this result."`.

### 3.13 Error Handling

**Requirement 13.1:** Sub-agent errors must never crash the parent agent. All errors within `ExecuteCallAgent` must be caught and returned as `ToolResult` values with `IsError: true`.

**Requirement 13.2:** The following errors must be caught and returned as tool results:
- Agent config not found
- Agent config invalid (bad TOML, missing required fields)
- Invalid model format in sub-agent config
- API key not configured for sub-agent's provider
- Provider creation failure
- API errors (auth, rate limit, timeout, server error)
- Context cancellation (parent timeout exceeded)
- Sub-agent conversation loop exceeding 50 turns

**Requirement 13.3:** Error tool result messages must follow this format:
```
Error: sub-agent "<name>" failed - <specific error description>. You may retry or proceed without this result.
```

**Requirement 13.4:** The parent LLM receives the error as a tool result and decides how to proceed. Axe does not retry sub-agent calls automatically.

**Requirement 13.5:** If the parent's own provider call fails during the conversation loop (not a sub-agent failure, but the parent LLM API itself returning an error), the conversation loop terminates and the error propagates to the caller via `mapProviderError`, identical to M4 behavior.

---

## 4. Project Structure

After M5 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── run.go                    # MODIFIED: conversation loop, tool injection, depth tracking
│   ├── run_test.go               # MODIFIED: tests for conversation loop, sub-agent execution
│   ├── agents.go                 # MODIFIED: display sub_agents_config in agents show
│   ├── agents_test.go            # MODIFIED: test sub_agents_config display
│   ├── config.go                 # UNCHANGED
│   ├── config_test.go            # UNCHANGED
│   ├── exit.go                   # UNCHANGED
│   ├── exit_test.go              # UNCHANGED
│   ├── root.go                   # UNCHANGED
│   ├── root_test.go              # UNCHANGED
│   ├── version.go                # UNCHANGED
│   └── version_test.go           # UNCHANGED
├── internal/
│   ├── tool/
│   │   ├── tool.go               # NEW: CallAgentTool definition, ExecuteCallAgent
│   │   └── tool_test.go          # NEW: tests for tool definition and execution
│   ├── agent/
│   │   ├── agent.go              # MODIFIED: SubAgentsConfig struct, updated Validate, updated Scaffold
│   │   └── agent_test.go         # MODIFIED: tests for SubAgentsConfig validation
│   ├── provider/
│   │   ├── provider.go           # MODIFIED: Tool, ToolCall, ToolResult types; extended Request, Response, Message
│   │   ├── provider_test.go      # MODIFIED: tests for new types
│   │   ├── anthropic.go          # MODIFIED: tool-calling request/response support
│   │   ├── anthropic_test.go     # MODIFIED: tests for tool-calling format
│   │   ├── openai.go             # MODIFIED: tool-calling request/response support
│   │   ├── openai_test.go        # MODIFIED: tests for tool-calling format
│   │   ├── ollama.go             # MODIFIED: tool-calling request/response support
│   │   ├── ollama_test.go        # MODIFIED: tests for tool-calling format
│   │   ├── registry.go           # UNCHANGED
│   │   └── registry_test.go      # UNCHANGED
│   ├── config/                   # UNCHANGED
│   │   ├── config.go
│   │   └── config_test.go
│   ├── resolve/                  # UNCHANGED
│   │   ├── resolve.go
│   │   └── resolve_test.go
│   └── xdg/                     # UNCHANGED
│       ├── xdg.go
│       └── xdg_test.go
├── go.mod                        # UNCHANGED (no new dependencies)
├── go.sum                        # UNCHANGED
└── ...
```

---

## 5. Edge Cases

### 5.1 Tool-Calling Types

| Scenario | Behavior |
|----------|----------|
| `Request.Tools` is nil | No tools sent to LLM; identical to M4 behavior |
| `Request.Tools` is empty slice | No tools sent to LLM; identical to M4 behavior |
| `Response.ToolCalls` is nil | Normal text response; no conversation loop iteration |
| LLM returns text alongside tool calls | `Response.Content` contains the text; `Response.ToolCalls` contains the tool calls |
| LLM returns only tool calls, no text | `Response.Content` is empty string; `Response.ToolCalls` is populated |
| LLM returns tool call with empty arguments | `ToolCall.Arguments` is an empty map |
| LLM returns tool call with unknown name | Handled by dispatcher: `ToolResult` with `IsError: true` |

### 5.2 Sub-Agent Config

| Scenario | Behavior |
|----------|----------|
| `sub_agents` is empty, `sub_agents_config` is set | Config is parsed but ignored (no tools injected) |
| `sub_agents` has names, no `sub_agents_config` | Defaults: max_depth=3, parallel=true, timeout=0 (inherit parent) |
| `sub_agents` has duplicate names | Duplicates are allowed in config but have no special behavior |
| `sub_agents` lists a non-existent agent | Not validated at config load time. Error surfaces when `call_agent` is executed and `agent.Load()` fails |
| `max_depth = 0` | Uses system default (3) |
| `max_depth = 1` | Parent can call sub-agents, but sub-agents cannot call their own sub-agents |
| `max_depth = 5` | Maximum allowed nesting |
| `max_depth = 6` | Validation error at config load time |
| `max_depth = -1` | Validation error at config load time |
| `timeout = 0` | Sub-agents inherit parent's timeout context |
| `timeout = -1` | Validation error at config load time |

### 5.3 Agent Name Validation

| Scenario | Behavior |
|----------|----------|
| LLM calls agent not in `sub_agents` list | `ToolResult` with `IsError: true` |
| LLM calls agent with empty name | `ToolResult` with `IsError: true`, "agent argument is required" |
| LLM calls agent with empty task | `ToolResult` with `IsError: true`, "task argument is required" |
| LLM calls agent with no context | Valid; context is optional |
| LLM calls agent with empty context string | Valid; treated same as no context |

### 5.4 Depth Limiting

| Scenario | Behavior |
|----------|----------|
| Depth 0, max_depth 3 | Tools injected; agent can call sub-agents |
| Depth 2, max_depth 3 | Tools injected; sub-agent can call its own sub-agents (at depth 3) |
| Depth 3, max_depth 3 | No tools injected; agent runs as single-shot |
| Depth 4, max_depth 5 | Tools injected (depth < max) |
| Depth 5, max_depth 5 | No tools injected |
| Sub-agent has `sub_agents` but depth limit reached | Sub-agent runs without tools; its `sub_agents` config is ignored at runtime |

### 5.5 Parallel Execution

| Scenario | Behavior |
|----------|----------|
| Single tool call, parallel=true | Runs in main goroutine (no goroutine overhead for single call) |
| Multiple tool calls, parallel=true | All run concurrently; results collected in order |
| Multiple tool calls, parallel=false | Run sequentially in order |
| One sub-agent fails, others succeed (parallel) | Failed one returns error result; successful ones return normal results; all results sent to parent LLM |
| All sub-agents fail (parallel) | All error results sent to parent LLM; parent decides what to do |
| Parent context cancelled during parallel execution | All sub-agent goroutines receive cancellation via context |

### 5.6 Conversation Loop

| Scenario | Behavior |
|----------|----------|
| LLM never calls tools | Single-shot; identical to M4 |
| LLM calls tools once, then produces final text | Two turns: initial + tool results; final response printed |
| LLM calls tools in a loop (always requesting more tool calls) | Loop continues up to 50 turns, then error |
| LLM calls tools, receives results, calls different tools | Normal behavior; loop continues |
| Parent API fails mid-conversation | Error propagated; exit code per `mapProviderError` |
| Context timeout during conversation loop | Provider returns timeout error; propagated to caller |
| LLM returns empty content and no tool calls | Loop ends; empty content printed (or empty JSON content field) |

### 5.7 Error Propagation

| Scenario | Behavior |
|----------|----------|
| Sub-agent TOML not found | `ToolResult` error: `failed to load agent "<name>": agent config not found: <name>` |
| Sub-agent has invalid model format | `ToolResult` error with model parse error |
| Sub-agent's provider API key missing | `ToolResult` error with API key message |
| Sub-agent's API returns 401 | `ToolResult` error with auth failure message |
| Sub-agent's API returns 429 | `ToolResult` error with rate limit message |
| Sub-agent times out | `ToolResult` error with timeout message |
| Sub-agent's internal conversation loop hits 50 turns | `ToolResult` error with max turns message |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** No streaming. All provider interactions receive complete responses before returning.

**Constraint 3:** No retry logic. Failed sub-agent calls are not retried by Axe. The parent LLM decides whether to retry.

**Constraint 4:** No caching of responses, configs, or provider instances.

**Constraint 5:** All user-facing output must go through `cmd.OutOrStdout()` (stdout) or `cmd.ErrOrStderr()` (stderr) for testability. Sub-agent verbose output goes to stderr.

**Constraint 6:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows.

**Constraint 7:** The `Provider` interface signature must not change:

```go
type Provider interface {
    Send(ctx context.Context, req *Request) (*Response, error)
}
```

The `Request`, `Response`, and `Message` structs are extended with new fields, but existing fields and their semantics remain unchanged. All existing code that constructs `Request`, reads `Response`, or creates `Message` values continues to work without modification due to zero-value defaults on new fields.

**Constraint 8:** The `call_agent` tool is the only tool in M5. The tool infrastructure must support future tools (the dispatch is by tool name), but no other tools are implemented.

**Constraint 9:** Sub-agents run the same code path as top-level agents. There is no separate "sub-agent runner." The `ExecuteCallAgent` function reuses the same resolution, provider creation, and conversation loop logic used by `axe run`.

**Constraint 10:** No structured output from sub-agents. The only data returned is `Response.Content` as a plain string. Structured JSON sub-agent output is a v2 feature.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M1-M4:

- **Package-level tests:** Tests live in the same package (e.g. `package tool`, `package provider`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real I/O. HTTP tests must use `httptest.NewServer`. Tests that mock the `Provider` interface to verify "mock returns X, response is X" are not acceptable. Each test must fail if the code under test is deleted.
- **Run tests with:** `make test`
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.

### 7.2 `internal/provider/provider.go` Tests (Modified)

**Test: `TestTool_ZeroValue`** -- Verify that a zero-value `Tool` struct has empty `Name`, `Description`, and nil `Parameters`.

**Test: `TestToolCall_ZeroValue`** -- Verify that a zero-value `ToolCall` has empty fields.

**Test: `TestToolResult_ZeroValue`** -- Verify that a zero-value `ToolResult` has empty fields and `IsError` is false.

**Test: `TestRequest_NilTools_BackwardsCompatible`** -- Create a `Request` with `Tools: nil`. Verify it marshals/works identically to M4 behavior.

**Test: `TestResponse_NilToolCalls_BackwardsCompatible`** -- Create a `Response` with `ToolCalls: nil`. Verify `len(resp.ToolCalls)` is 0.

**Test: `TestMessage_ToolCallsField`** -- Create a `Message` with `Role: "assistant"` and populated `ToolCalls`. Verify fields are accessible.

**Test: `TestMessage_ToolResultsField`** -- Create a `Message` with `Role: "tool"` and populated `ToolResults`. Verify fields are accessible.

### 7.3 `internal/provider/anthropic.go` Tests (Modified)

**Test: `TestAnthropic_Send_WithTools`** -- Use `httptest.NewServer` that inspects the request. Send a request with `Tools` populated. Verify the request body contains a `tools` array with correct Anthropic format (`name`, `description`, `input_schema` with `properties` and `required`).

**Test: `TestAnthropic_Send_WithoutTools`** -- Send a request with `Tools: nil`. Verify the request body does NOT contain a `tools` key.

**Test: `TestAnthropic_Send_ToolCallResponse`** -- Server returns a response with `tool_use` content blocks. Verify `Response.ToolCalls` is populated with correct `ID`, `Name`, `Arguments`.

**Test: `TestAnthropic_Send_ToolCallWithText`** -- Server returns a response with both `text` and `tool_use` content blocks. Verify `Response.Content` contains the text AND `Response.ToolCalls` is populated.

**Test: `TestAnthropic_Send_ToolCallNoText`** -- Server returns a response with only `tool_use` content blocks, no `text`. Verify `Response.Content` is empty string and `Response.ToolCalls` is populated.

**Test: `TestAnthropic_Send_ToolResultMessage`** -- Send a request with a message containing `ToolResults`. Use `httptest.NewServer` to inspect the request body. Verify the message is formatted as a `user` message with `tool_result` content blocks including `tool_use_id`, `content`, and `is_error`.

**Test: `TestAnthropic_Send_AssistantToolCallMessage`** -- Send a request with a message containing `ToolCalls` (assistant turn in history). Verify the message is formatted with `tool_use` content blocks.

**Test: `TestAnthropic_Send_ToolsStopReason`** -- Server returns `stop_reason: "tool_use"`. Verify `Response.StopReason` is `"tool_use"`.

### 7.4 `internal/provider/openai.go` Tests (Modified)

**Test: `TestOpenAI_Send_WithTools`** -- Send a request with `Tools` populated. Verify the request body contains a `tools` array with correct OpenAI format (`type: "function"`, `function.name`, `function.description`, `function.parameters`).

**Test: `TestOpenAI_Send_WithoutTools`** -- Send a request with `Tools: nil`. Verify the request body does NOT contain a `tools` key.

**Test: `TestOpenAI_Send_ToolCallResponse`** -- Server returns a response with `tool_calls` in the choice message. Verify `Response.ToolCalls` is populated with correct `ID`, `Name`, `Arguments`.

**Test: `TestOpenAI_Send_ToolCallNullContent`** -- Server returns tool calls with `content: null`. Verify `Response.Content` is empty string.

**Test: `TestOpenAI_Send_ToolResultMessage`** -- Send a request with a message containing `ToolResults`. Verify each `ToolResult` becomes a separate `role: "tool"` message with `tool_call_id` and `content`.

**Test: `TestOpenAI_Send_AssistantToolCallMessage`** -- Send a request with a message containing `ToolCalls`. Verify the message includes `tool_calls` array with `id`, `type: "function"`, and `function` fields.

**Test: `TestOpenAI_Send_InvalidToolCallArguments`** -- Server returns a tool call with invalid JSON in `function.arguments`. Verify `ToolCall` is created with empty `Arguments`.

**Test: `TestOpenAI_Send_ToolsStopReason`** -- Server returns `finish_reason: "tool_calls"`. Verify `Response.StopReason` is `"tool_calls"`.

### 7.5 `internal/provider/ollama.go` Tests (Modified)

**Test: `TestOllama_Send_WithTools`** -- Send a request with `Tools` populated. Verify the request body contains a `tools` array.

**Test: `TestOllama_Send_WithoutTools`** -- Send a request with `Tools: nil`. Verify no `tools` key.

**Test: `TestOllama_Send_ToolCallResponse`** -- Server returns a response with `tool_calls` in the message. Verify `Response.ToolCalls` is populated. Verify generated IDs follow `"ollama_<index>"` format.

**Test: `TestOllama_Send_NoToolCallsWithTools`** -- Send request with tools, but server returns normal text response (model doesn't support tools). Verify `Response.ToolCalls` is nil/empty and `Response.Content` is populated.

**Test: `TestOllama_Send_ToolResultMessage`** -- Send a request with a message containing `ToolResults`. Verify each result becomes a `role: "tool"` message.

### 7.6 `internal/agent/agent.go` Tests (Modified)

**Test: `TestValidate_MaxDepthTooHigh`** -- Set `SubAgentsConf.MaxDepth` to 6. Verify validation error: `sub_agents_config.max_depth cannot exceed 5`.

**Test: `TestValidate_MaxDepthNegative`** -- Set `SubAgentsConf.MaxDepth` to -1. Verify validation error.

**Test: `TestValidate_TimeoutNegative`** -- Set `SubAgentsConf.Timeout` to -1. Verify validation error.

**Test: `TestValidate_MaxDepthValid`** -- Set `SubAgentsConf.MaxDepth` to 5. Verify no validation error.

**Test: `TestValidate_SubAgentsConfigDefaults`** -- Load agent with no `[sub_agents_config]`. Verify `SubAgentsConf` has zero values (MaxDepth=0, Parallel=false, Timeout=0). Note: `Parallel` defaults to `false` in Go's zero value; the run command must treat `false` as "use default true" when `SubAgents` is non-empty. See Requirement 9.5.

**Test: `TestLoad_SubAgentsConfig`** -- Write agent TOML with `[sub_agents_config]` section. Verify fields are parsed correctly.

**Test: `TestScaffold_IncludesSubAgentsConfig`** -- Verify scaffold output contains commented `[sub_agents_config]` section.

### 7.7 `internal/tool/tool.go` Tests

**Test: `TestCallAgentTool_Definition`** -- Call `CallAgentTool([]string{"helper", "runner"})`. Verify returned `Tool` has correct `Name`, `Description` containing agent names, and three parameters with correct types and required flags.

**Test: `TestCallAgentTool_EmptyAgents`** -- Call `CallAgentTool([]string{})`. Verify returned `Tool` still has a valid structure (empty agent list in description).

**Test: `TestExecuteCallAgent_Success`** -- Set up a temp agent TOML, start `httptest.NewServer` as mock provider. Call `ExecuteCallAgent` with valid arguments. Verify returned `ToolResult` has `IsError: false` and `Content` matching the mock response.

**Test: `TestExecuteCallAgent_AgentNotAllowed`** -- Call with agent name not in `AllowedAgents`. Verify `ToolResult` has `IsError: true` and message about agent not in sub_agents list.

**Test: `TestExecuteCallAgent_EmptyAgentName`** -- Call with empty `agent` argument. Verify `ToolResult` has `IsError: true`.

**Test: `TestExecuteCallAgent_EmptyTask`** -- Call with empty `task` argument. Verify `ToolResult` has `IsError: true`.

**Test: `TestExecuteCallAgent_DepthLimitReached`** -- Call with `Depth >= MaxDepth`. Verify `ToolResult` has `IsError: true` and message about max depth.

**Test: `TestExecuteCallAgent_AgentNotFound`** -- Call with agent name that has no TOML file. Verify `ToolResult` has `IsError: true` with load error.

**Test: `TestExecuteCallAgent_APIError`** -- Start `httptest.NewServer` returning 500. Verify `ToolResult` has `IsError: true` with error message.

**Test: `TestExecuteCallAgent_Timeout`** -- Start slow `httptest.NewServer`, use short timeout. Verify `ToolResult` has `IsError: true` with timeout message.

**Test: `TestExecuteCallAgent_WithContext`** -- Call with non-empty `context` argument. Inspect the request received by the mock server. Verify user message contains both task and context.

**Test: `TestExecuteCallAgent_WithoutContext`** -- Call with empty `context` argument. Verify user message contains only task.

### 7.8 `cmd/run_test.go` Tests (Modified)

**Test: `TestRun_SubAgentToolInjection`** -- Create parent agent with `sub_agents = ["helper"]`. Start `httptest.NewServer` that inspects the request for `tools` array. Verify the `call_agent` tool is present in the request. Server returns a text response (no tool calls).

**Test: `TestRun_NoSubAgents_NoTools`** -- Create agent without `sub_agents`. Start `httptest.NewServer` that inspects the request. Verify NO `tools` key in the request body.

**Test: `TestRun_ConversationLoop_ToolCall`** -- Create parent agent with `sub_agents = ["helper"]`. Create helper agent TOML. Start two `httptest.NewServer` instances: one for the parent's provider (returns a tool call on first request, then a text response on second request) and one for the sub-agent's provider (returns a text response). Verify the final output contains the parent's second response.

**Test: `TestRun_ConversationLoop_MaxTurns`** -- Create parent agent. Start `httptest.NewServer` that always returns tool calls (never a final text response). Verify error about exceeding maximum conversation turns.

**Test: `TestRun_SubAgent_Error_PropagatesAsToolResult`** -- Create parent agent with `sub_agents = ["nonexistent"]`. Start `httptest.NewServer` for parent: first response is a tool call to "nonexistent", second response is text. Verify the conversation continues (parent receives error tool result and produces final output).

**Test: `TestRun_DryRun_ShowsSubAgents`** -- Create agent with `sub_agents`. Run with `--dry-run`. Verify output contains "Sub-Agents" section with agent names.

**Test: `TestRun_DryRun_NoSubAgents`** -- Create agent without `sub_agents`. Run with `--dry-run`. Verify output contains `(none)` for sub-agents section.

**Test: `TestRun_JSON_IncludesToolCalls`** -- Create parent agent with sub-agents. Mock a conversation with one tool call. Run with `--json`. Verify JSON output includes `tool_calls` field.

**Test: `TestRun_Verbose_ConversationTurns`** -- Create parent agent with sub-agents. Mock a conversation with tool calls. Run with `--verbose`. Verify stderr contains turn-by-turn log messages.

**Test: `TestRun_ParallelToolCalls`** -- Create parent agent with `sub_agents = ["a", "b"]`, `parallel = true`. Start mock servers. Parent returns two concurrent tool calls. Verify both sub-agents are called and results returned.

**Test: `TestRun_SequentialToolCalls`** -- Create parent agent with `sub_agents = ["a", "b"]`, `parallel = false`. Parent returns two tool calls. Verify tool calls execute sequentially (second starts after first completes).

### 7.9 Running Tests

All tests must pass when run with:

```bash
make test
```

No test may make real HTTP requests to external APIs. All HTTP interactions must use `httptest.NewServer`.

---

## 8. Exit Codes

The full exit code table after M5 (unchanged from M4):

| Code | Meaning | Used By |
|------|---------|---------|
| 0 | Success | All commands on success, `--dry-run` |
| 1 | Agent/general error | Invalid model format, unsupported provider, bad request, max conversation turns exceeded |
| 2 | Config error | Agent not found, invalid TOML, missing required fields, invalid skill path, invalid glob, malformed `config.toml`, invalid `sub_agents_config` |
| 3 | API error | Auth failure, rate limit, timeout, server error, overloaded, missing API key |

Note: Sub-agent errors do NOT affect the parent's exit code. Sub-agent failures are returned as tool results to the parent LLM. The parent's exit code is determined by its own execution result.

---

## 9. Acceptance Criteria

| Criterion | Test |
|-----------|------|
| Tool Types | `Tool`, `ToolCall`, `ToolResult` structs defined in `internal/provider/` |
| Request.Tools | `Request` struct has `Tools` field; nil means no tools |
| Response.ToolCalls | `Response` struct has `ToolCalls` field; nil means no tool calls |
| Message Extension | `Message` supports `ToolCalls` and `ToolResults` fields |
| Backwards Compatible | Existing M4 code works unchanged with nil `Tools`/`ToolCalls` |
| Anthropic Tools | Anthropic provider sends tools in `input_schema` format |
| Anthropic Tool Response | Anthropic provider parses `tool_use` content blocks into `ToolCall` |
| Anthropic Tool Result | Anthropic provider formats `ToolResult` as `tool_result` content block |
| OpenAI Tools | OpenAI provider sends tools in `function` format |
| OpenAI Tool Response | OpenAI provider parses `tool_calls` array into `ToolCall` |
| OpenAI Tool Result | OpenAI provider formats `ToolResult` as `role: "tool"` message |
| Ollama Tools | Ollama provider sends tools in `function` format |
| Ollama Tool Response | Ollama provider parses tool calls and generates IDs |
| Ollama No Tool Support | Graceful fallback when model ignores tools |
| call_agent Definition | `tool.CallAgentTool()` returns correctly formatted tool with agent names |
| Sub-Agent Execution | `ExecuteCallAgent` loads sub-agent config, resolves context, calls LLM, returns result |
| Agent Validation | Invalid `sub_agents_config` values rejected at validation time |
| Conversation Loop | Parent cycles through tool calls until LLM produces final text |
| Max Turns | Loop terminates after 50 turns with error |
| No Tools = Single Shot | Agents without `sub_agents` behave identically to M4 |
| Depth Tracking | Depth increments per nesting level; tools removed at depth limit |
| Depth Default | Default max depth is 3 when not configured |
| Depth Hard Max | Max depth cannot exceed 5 |
| Parallel Execution | Multiple tool calls run concurrently when parallel=true |
| Sequential Execution | Tool calls run in order when parallel=false |
| Sub-Agent Timeout | Per-agent timeout enforced via context |
| Error as Tool Result | Sub-agent errors returned as `ToolResult` with `IsError: true` |
| Parent Not Crashed | Sub-agent failures never crash the parent |
| Dry Run Sub-Agents | `--dry-run` shows sub-agent configuration |
| JSON Tool Calls | `--json` output includes `tool_calls` count |
| Verbose Turns | `--verbose` logs each conversation turn |
| Cumulative Tokens | Token counts summed across all turns |
| No New Deps | `go.mod` unchanged |
| All Tests Pass | `make test` passes with 0 failures |

---

## 10. Out of Scope

The following items are explicitly **not** included in M5:

1. Memory read/write operations (M6)
2. Garbage collection (M7)
3. Streaming output (Future)
4. Structured JSON output from sub-agents (Future v2)
5. Streaming partial results from sub-agents to parent (Future v2)
6. Sub-agent cost/token tracking returned to parent (Future v2)
7. Shared memory between parent and sub-agents (Future v2)
8. Retry logic or exponential backoff for sub-agent calls
9. Response caching
10. Tools other than `call_agent` (e.g. file read, shell exec)
11. User-configurable tool definitions in TOML
12. Tool approval/confirmation prompts before execution
13. Sub-agent output streaming to the terminal while parent waits
14. Circuit breaker or rate limiting for sub-agent calls
15. Sub-agent call logging or audit trail (beyond verbose output)
16. `axe tools list` or similar tool discovery commands
17. Weighted or priority-based sub-agent selection
18. Sub-agent result caching across conversation turns
19. Token budget enforcement across parent + sub-agent calls

---

## 11. References

- Milestone Definition: `docs/plans/000_milestones.md` (M5 section)
- Sub-Agent Design Doc: `docs/design/sub-agent-pattern.md`
- M4 Spec: `docs/plans/004_multi_provider_support_spec.md`
- M3 Spec: `docs/plans/003_single_agent_run_spec.md`
- Agent Config Schema: `docs/design/agent-config-schema.md`
- CLI Structure: `docs/design/cli-structure.md`
- Anthropic Tool Use: https://docs.anthropic.com/en/docs/build-with-claude/tool-use
- OpenAI Function Calling: https://platform.openai.com/docs/guides/function-calling
- Ollama Tool Support: https://github.com/ollama/ollama/blob/main/docs/api.md

---

## 12. Notes

- The `Provider` interface is not changed (still a single `Send` method). Tool-calling is achieved by extending the `Request`, `Response`, and `Message` structs with optional fields. This preserves backwards compatibility: all M1-M4 code that uses `nil` tools continues to work.
- The conversation loop replaces the single-shot call only when tools are present. The check is `len(req.Tools) > 0`, not the presence of `sub_agents` in config. This means the loop logic is reusable for future tools beyond `call_agent`.
- Sub-agents reuse the same code path as top-level agents (`ExecuteCallAgent` calls the same resolution + provider + loop logic). This avoids code duplication and ensures sub-agents get the same features (multi-provider support, config resolution, etc.).
- The `Parallel` field in `SubAgentsConfig` defaults to Go's zero value `false`. However, the design doc specifies `parallel = true` as the default behavior. The run command must check: if `SubAgents` is non-empty and `SubAgentsConf.Parallel` is `false` and the TOML did not explicitly set it, use `true`. The simplest approach: TOML `parallel` defaults to `true` in the scaffold, and the run command treats `false` as explicit opt-out. Since TOML bool defaults to `false` when absent, an alternative is to use a `*bool` pointer type. The implementation must resolve this ambiguity — the spec requires that the default behavior (when `[sub_agents_config]` is absent) is parallel execution.
- Ollama tool calling support varies by model. Some models (e.g. llama3.1, mistral) support tools; others do not. The provider handles this gracefully by checking for tool calls in the response. If the model ignores tools, the parent LLM simply never receives tool call responses and must produce its final answer without delegation.
- The 50-turn limit is a safety valve, not a tunable parameter. It prevents infinite loops if the LLM keeps calling tools without converging. In practice, most conversations should complete in 2-5 turns. If a legitimate use case needs more than 50 turns, this limit can be raised in a future version.
- Error messages returned as tool results include the suggestion "You may retry or proceed without this result." This gives the parent LLM explicit permission to handle errors gracefully rather than failing the entire task.
- The `call_agent` tool description includes the list of available agent names. This is critical for LLM usability: without seeing the names, the LLM cannot know what to call. The names come from the parent's `sub_agents` config.
- Token counts are cumulative across turns to give an accurate total cost picture. Per-turn token breakdown is available in verbose output.
