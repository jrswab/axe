# Implementation Checklist: M5 - Sub-Agents

**Based on:** 005_sub_agents_spec.md
**Status:** In Progress
**Created:** 2026-02-27

---

## Phase 1: Tool-Calling Types (`internal/provider/provider.go`) (Spec §3.1)

### 1a: New Structs

- [x] Write `TestTool_ZeroValue` — verify zero-value `Tool` has empty `Name`, `Description`, nil `Parameters` (Req 1.1, 1.2)
- [x] Write `TestToolCall_ZeroValue` — verify zero-value `ToolCall` has empty fields (Req 1.3)
- [x] Write `TestToolResult_ZeroValue` — verify zero-value `ToolResult` has empty fields and `IsError` is false (Req 1.4)
- [x] Define `ToolParameter` struct with `Type`, `Description`, `Required` fields (Req 1.2)
- [x] Define `Tool` struct with `Name`, `Description`, `Parameters map[string]ToolParameter` (Req 1.1)
- [x] Define `ToolCall` struct with `ID`, `Name`, `Arguments map[string]string` (Req 1.3)
- [x] Define `ToolResult` struct with `CallID`, `Content`, `IsError` (Req 1.4)
- [x] Run tests — all new struct zero-value tests pass

### 1b: Extend Existing Structs

- [x] Write `TestRequest_NilTools_BackwardsCompatible` — `Request` with `Tools: nil` works like M4 (Req 1.5, 1.8)
- [x] Write `TestResponse_NilToolCalls_BackwardsCompatible` — `Response` with `ToolCalls: nil`; `len(resp.ToolCalls)` is 0 (Req 1.6, 1.8)
- [x] Write `TestMessage_ToolCallsField` — `Message` with `Role: "assistant"` and populated `ToolCalls`; verify accessible (Req 1.7)
- [x] Write `TestMessage_ToolResultsField` — `Message` with `Role: "tool"` and populated `ToolResults`; verify accessible (Req 1.7)
- [x] Add `Tools []Tool` field to `Request` struct (Req 1.5)
- [x] Add `ToolCalls []ToolCall` field to `Response` struct (Req 1.6)
- [x] Add `ToolCalls []ToolCall` field to `Message` struct (Req 1.7)
- [x] Add `ToolResults []ToolResult` field to `Message` struct (Req 1.7)
- [x] Run tests — all backwards-compatibility and field tests pass
- [x] Run `make test` — verify all existing M4 tests still pass (zero-value defaults preserve behavior)

---

## Phase 2: Anthropic Provider Tool-Calling Support (`internal/provider/anthropic.go`) (Spec §3.2)

### 2a: Tool Definitions in Request

- [x] Write `TestAnthropic_Send_WithTools` — inspect request body for `tools` array with Anthropic format (`name`, `description`, `input_schema` with `properties` and `required`) (Req 2.1, 2.2)
- [x] Write `TestAnthropic_Send_WithoutTools` — `Tools: nil`; verify no `tools` key in request body (Req 2.3)
- [x] Implement tools serialization in Anthropic `Send`: build `tools` array from `Request.Tools` with `input_schema` format; omit when nil/empty (Req 2.1–2.3)
- [x] Run tests — tool definition request tests pass

### 2b: Tool-Call Response Parsing

- [x] Write `TestAnthropic_Send_ToolCallResponse` — server returns `tool_use` content blocks; verify `Response.ToolCalls` has correct `ID`, `Name`, `Arguments` (Req 2.4)
- [x] Write `TestAnthropic_Send_ToolCallWithText` — server returns both `text` and `tool_use` blocks; verify `Response.Content` has text AND `Response.ToolCalls` is populated (Req 2.5)
- [x] Write `TestAnthropic_Send_ToolCallNoText` — server returns only `tool_use` blocks; verify `Response.Content` is empty, `Response.ToolCalls` populated (Req 2.5)
- [x] Write `TestAnthropic_Send_ToolsStopReason` — server returns `stop_reason: "tool_use"`; verify `Response.StopReason` is `"tool_use"` (Req 1.9)
- [x] Implement response parsing: handle `tool_use` content blocks, populate `Response.ToolCalls`; concatenate text blocks into `Response.Content` (Req 2.4, 2.5)
- [x] Run tests — tool-call response parsing tests pass

### 2c: Tool-Result and Assistant Tool-Call Messages

- [x] Write `TestAnthropic_Send_ToolResultMessage` — message with `ToolResults`; verify formatted as `user` role with `tool_result` content blocks (Req 2.6)
- [x] Write `TestAnthropic_Send_AssistantToolCallMessage` — message with `ToolCalls`; verify formatted with `tool_use` content blocks (Req 2.7)
- [x] Implement message serialization: handle `Message.ToolResults` as `user` + `tool_result` blocks; handle `Message.ToolCalls` as `assistant` + `tool_use` blocks (Req 2.6, 2.7)
- [x] Run tests — all Anthropic tool-calling tests pass
- [x] Run `make test` — verify all existing Anthropic tests still pass

---

## Phase 3: OpenAI Provider Tool-Calling Support (`internal/provider/openai.go`) (Spec §3.3)

### 3a: Tool Definitions in Request

- [x] Write `TestOpenAI_Send_WithTools` — inspect request body for `tools` array with OpenAI format (`type: "function"`, `function.name`, `function.description`, `function.parameters`) (Req 3.1)
- [x] Write `TestOpenAI_Send_WithoutTools` — `Tools: nil`; verify no `tools` key in request body (Req 3.2)
- [x] Implement tools serialization in OpenAI `Send`: build `tools` array with `type: "function"` wrapper; omit when nil/empty (Req 3.1, 3.2)
- [x] Run tests — tool definition request tests pass

### 3b: Tool-Call Response Parsing

- [x] Write `TestOpenAI_Send_ToolCallResponse` — server returns `tool_calls` in choice message; verify `Response.ToolCalls` with correct `ID`, `Name`, `Arguments` (Req 3.3)
- [x] Write `TestOpenAI_Send_ToolCallNullContent` — server returns tool calls with `content: null`; verify `Response.Content` is empty (Req 3.5)
- [x] Write `TestOpenAI_Send_InvalidToolCallArguments` — server returns invalid JSON in `function.arguments`; verify `ToolCall` with empty `Arguments` (Req 3.4)
- [x] Write `TestOpenAI_Send_ToolsStopReason` — server returns `finish_reason: "tool_calls"`; verify `Response.StopReason` is `"tool_calls"` (Req 1.9)
- [x] Implement response parsing: handle `tool_calls` array in choice message; parse `function.arguments` as JSON; handle invalid JSON gracefully (Req 3.3–3.5)
- [x] Run tests — tool-call response parsing tests pass

### 3c: Tool-Result and Assistant Tool-Call Messages

- [x] Write `TestOpenAI_Send_ToolResultMessage` — message with `ToolResults`; verify each becomes separate `role: "tool"` message with `tool_call_id` (Req 3.6)
- [x] Write `TestOpenAI_Send_AssistantToolCallMessage` — message with `ToolCalls`; verify `tool_calls` array with `id`, `type: "function"`, `function` fields (Req 3.7)
- [x] Implement message serialization: handle `Message.ToolResults` as individual `tool` messages; handle `Message.ToolCalls` as `assistant` message with `tool_calls` array (Req 3.6, 3.7)
- [x] Run tests — all OpenAI tool-calling tests pass
- [x] Run `make test` — verify all existing OpenAI tests still pass

---

## Phase 4: Ollama Provider Tool-Calling Support (`internal/provider/ollama.go`) (Spec §3.4)

### 4a: Tool Definitions in Request

- [x] Write `TestOllama_Send_WithTools` — inspect request body for `tools` array (Req 4.1)
- [x] Write `TestOllama_Send_WithoutTools` — `Tools: nil`; verify no `tools` key (Req 4.2)
- [x] Implement tools serialization in Ollama `Send`: build `tools` array with `type: "function"` wrapper; omit when nil/empty (Req 4.1, 4.2)
- [x] Run tests — tool definition request tests pass

### 4b: Tool-Call Response Parsing

- [x] Write `TestOllama_Send_ToolCallResponse` — server returns `tool_calls` in message; verify `Response.ToolCalls` with generated `"ollama_<index>"` IDs (Req 4.3)
- [x] Write `TestOllama_Send_NoToolCallsWithTools` — request has tools but server returns text-only response; verify `Response.ToolCalls` is empty, `Response.Content` populated (Req 4.6)
- [x] Implement response parsing: handle `tool_calls` array; generate `"ollama_<index>"` IDs; graceful fallback for models that ignore tools (Req 4.3, 4.6)
- [x] Run tests — tool-call response parsing tests pass

### 4c: Tool-Result and Assistant Tool-Call Messages

- [x] Write `TestOllama_Send_ToolResultMessage` — message with `ToolResults`; verify each becomes `role: "tool"` message (Req 4.4)
- [x] Write `TestOllama_Send_AssistantToolCallMessage` — message with `ToolCalls`; verify `tool_calls` array in assistant message (Req 4.5)
- [x] Implement message serialization: handle `Message.ToolResults` as individual `tool` messages; handle `Message.ToolCalls` as `assistant` message with `tool_calls` (Req 4.4, 4.5)
- [x] Run tests — all Ollama tool-calling tests pass
- [x] Run `make test` — verify all existing Ollama tests still pass

---

## Phase 5: Agent Config Extension (`internal/agent/agent.go`) (Spec §3.5)

### 5a: SubAgentsConfig Struct and Parsing

- [x] Write `TestLoad_SubAgentsConfig` — TOML with `[sub_agents_config]` section; verify fields parsed correctly (Req 5.1, 5.2)
- [x] Write `TestValidate_SubAgentsConfigDefaults` — load agent with no `[sub_agents_config]`; verify zero values (Req 5.1)
- [x] Define `SubAgentsConfig` struct with `MaxDepth int`, `Parallel bool`, `Timeout int` (TOML tags: `max_depth`, `parallel`, `timeout`) (Req 5.1)
- [x] Add `SubAgentsConf SubAgentsConfig` field to `AgentConfig` (TOML tag: `sub_agents_config`) (Req 5.2)
- [x] Run tests — parsing tests pass

### 5b: Validation

- [x] Write `TestValidate_MaxDepthTooHigh` — `MaxDepth = 6`; verify error `sub_agents_config.max_depth cannot exceed 5` (Req 5.4)
- [x] Write `TestValidate_MaxDepthNegative` — `MaxDepth = -1`; verify error `sub_agents_config.max_depth must be non-negative` (Req 5.4)
- [x] Write `TestValidate_TimeoutNegative` — `Timeout = -1`; verify error `sub_agents_config.timeout must be non-negative` (Req 5.5)
- [x] Write `TestValidate_MaxDepthValid` — `MaxDepth = 5`; verify no error (Req 5.4)
- [x] Implement validation rules in `Validate`: check `MaxDepth` bounds (non-negative, ≤5) and `Timeout` non-negative (Req 5.4, 5.5)
- [x] Run tests — all validation tests pass

### 5c: Scaffold and Show

- [x] Write `TestScaffold_IncludesSubAgentsConfig` — verify scaffold output contains commented `[sub_agents_config]` section (Req 5.6)
- [x] Update `Scaffold` function to include commented-out `[sub_agents_config]` section (Req 5.6)
- [x] Update `agents show` command to display `SubAgentsConfig` fields when `SubAgents` is non-empty (Req 5.7)
- [x] Run tests — scaffold and show tests pass
- [x] Run `make test` — verify all existing agent tests still pass

---

## Phase 6: `call_agent` Tool Definition (`internal/tool/`) (Spec §3.6)

- [x] Create `internal/tool/` directory
- [x] Write `TestCallAgentTool_Definition` — `CallAgentTool([]string{"helper", "runner"})`; verify `Name`, `Description` with agent names, three parameters with correct types and required flags (Req 6.2, 6.3, 6.4)
- [x] Write `TestCallAgentTool_EmptyAgents` — `CallAgentTool([]string{})`; verify valid structure with empty agent list in description (Req 6.4)
- [x] Define `const CallAgentToolName = "call_agent"` (Req 6.2)
- [x] Implement `CallAgentTool(allowedAgents []string) provider.Tool` — returns tool definition with dynamic agent names in description and parameter descriptions (Req 6.3, 6.4)
- [x] Run tests — tool definition tests pass

---

## Phase 7: Sub-Agent Execution (`internal/tool/`) (Spec §3.7)

### 7a: ExecuteOptions and Argument Validation

- [x] Write `TestExecuteCallAgent_EmptyAgentName` — empty `agent` argument; verify `ToolResult` with `IsError: true` (Req 7.3 step 2)
- [x] Write `TestExecuteCallAgent_EmptyTask` — empty `task` argument; verify `ToolResult` with `IsError: true` (Req 7.3 step 3)
- [x] Write `TestExecuteCallAgent_AgentNotAllowed` — agent not in `AllowedAgents`; verify `ToolResult` with `IsError: true` and correct message (Req 7.3 step 4)
- [x] Write `TestExecuteCallAgent_DepthLimitReached` — `Depth >= MaxDepth`; verify `ToolResult` with `IsError: true` and depth message (Req 7.3 step 5)
- [x] Define `ExecuteOptions` struct with all fields (Req 7.2)
- [x] Implement `ExecuteCallAgent` argument extraction and validation (steps 1–5 of Req 7.3)
- [x] Run tests — validation tests pass

### 7b: Sub-Agent Loading and Execution

- [x] Write `TestExecuteCallAgent_AgentNotFound` — agent TOML does not exist; verify `ToolResult` with `IsError: true` and load error (Req 7.3 step 6)
- [x] Write `TestExecuteCallAgent_Success` — temp agent TOML + `httptest.NewServer`; verify `ToolResult` with `IsError: false` and correct content (Req 7.3 steps 6–15)
- [x] Write `TestExecuteCallAgent_WithContext` — non-empty `context` argument; verify user message contains task + context (Req 7.3 step 11)
- [x] Write `TestExecuteCallAgent_WithoutContext` — empty `context`; verify user message contains only task (Req 7.3 step 11)
- [x] Implement sub-agent loading, provider creation, request building, and execution (Req 7.3 steps 6–15)
- [x] Run tests — success path tests pass

### 7c: Error Handling and Verbose Output

- [x] Write `TestExecuteCallAgent_APIError` — `httptest.NewServer` returning 500; verify `ToolResult` with `IsError: true` and error message (Req 7.4)
- [x] Write `TestExecuteCallAgent_Timeout` — slow `httptest.NewServer` + short timeout; verify `ToolResult` with `IsError: true` and timeout message (Req 7.4, Req 12.3)
- [x] Implement error handling: catch all errors, return as `ToolResult` with `IsError: true` (Req 7.4, 13.1–13.4)
- [x] Implement verbose output: log before/after sub-agent calls to stderr (Req 7.5)
- [x] Implement timeout handling: `context.WithTimeout` when `Timeout > 0`, otherwise inherit parent context (Req 12.1–12.3)
- [x] Run tests — error handling and timeout tests pass
- [x] Run `make test` — all tests pass

---

## Phase 8: Conversation Loop (`cmd/run.go`) (Spec §3.8)

### 8a: Tool Injection

- [ ] Write `TestRun_SubAgentToolInjection` — parent with `sub_agents = ["helper"]`; verify `tools` array in request (Req 9.1)
- [ ] Write `TestRun_NoSubAgents_NoTools` — agent without `sub_agents`; verify no `tools` key (Req 9.2)
- [ ] Implement tool injection: check `cfg.SubAgents`, compute effective max depth, call `tool.CallAgentTool()`, set `Request.Tools` (Req 9.1–9.4)
- [ ] Run tests — tool injection tests pass

### 8b: Conversation Loop Core

- [ ] Write `TestRun_ConversationLoop_ToolCall` — parent + helper agents; parent provider returns tool call then text; helper provider returns text; verify final output (Req 8.1)
- [ ] Write `TestRun_ConversationLoop_MaxTurns` — server always returns tool calls; verify error about exceeding 50 turns (Req 8.3)
- [ ] Write `TestRun_SubAgent_Error_PropagatesAsToolResult` — parent calls nonexistent sub-agent; verify conversation continues with error tool result (Req 13.1, 13.4)
- [ ] Implement conversation loop: replace single-shot `prov.Send` with loop that handles tool calls, appends messages, re-sends (Req 8.1–8.4)
- [ ] Implement tool call dispatch: route by tool name, handle unknown tools (Req 8.2)
- [ ] Implement 50-turn safety limit (Req 8.3)
- [ ] Ensure single-shot behavior preserved when `Request.Tools` is nil (Req 8.4)
- [ ] Run tests — conversation loop tests pass

### 8c: Parallel and Sequential Execution

- [ ] Write `TestRun_ParallelToolCalls` — parent with `parallel = true`, two concurrent tool calls; verify both called and results returned (Req 11.1–11.4)
- [ ] Write `TestRun_SequentialToolCalls` — parent with `parallel = false`, two tool calls; verify sequential execution (Req 11.5)
- [ ] Implement parallel execution: goroutines for concurrent tool calls when `Parallel` is true and multiple calls (Req 11.1–11.4)
- [ ] Implement sequential execution: iterate in order when `Parallel` is false (Req 11.5)
- [ ] Handle single tool call optimization: no goroutine for single call (Edge case §5.5)
- [ ] Run tests — parallel and sequential tests pass

### 8d: Output Extensions (Dry-Run, JSON, Verbose)

- [ ] Write `TestRun_DryRun_ShowsSubAgents` — agent with `sub_agents`; verify `--dry-run` output contains Sub-Agents section (Req 8.5)
- [ ] Write `TestRun_DryRun_NoSubAgents` — no `sub_agents`; verify output contains `(none)` (Req 8.5)
- [ ] Write `TestRun_JSON_IncludesToolCalls` — conversation with tool calls + `--json`; verify JSON has `tool_calls` field (Req 8.6)
- [ ] Write `TestRun_Verbose_ConversationTurns` — conversation with tool calls + `--verbose`; verify stderr has turn-by-turn logs (Req 8.7)
- [ ] Implement `--dry-run` extension: print Sub-Agents section with names, max depth, parallel, timeout (Req 8.5)
- [ ] Implement `--json` extension: add `tool_calls` count field; cumulative token counts (Req 8.6, 8.8)
- [ ] Implement `--verbose` extension: log each conversation turn with message count and tool call info (Req 8.7, 8.8)
- [ ] Run tests — all output extension tests pass

---

## Phase 9: Depth Tracking (Spec §3.10)

- [ ] Verify depth starts at 0 for top-level `axe run` invocation (Req 10.1, 9.4)
- [ ] Verify `ExecuteCallAgent` passes `Depth + 1` to sub-agent (Req 10.2)
- [ ] Verify sub-agents at depth limit run without tools (Req 10.3, 10.4)
- [ ] Run `make test` — all depth-related tests pass (covered by Phase 7 and 8 tests)

---

## Phase 10: Final Verification

- [ ] Run `make test` — all tests pass with 0 failures
- [ ] Verify `go.mod` still contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies (Constraint 1)
- [ ] Verify `Provider` interface signature is unchanged: `Send(ctx context.Context, req *Request) (*Response, error)` (Constraint 7)
- [ ] Verify all existing M4 tests still pass unchanged (Req 1.8)
- [ ] Verify exit codes match spec table — sub-agent errors don't affect parent exit code (§8)
- [ ] Verify no real HTTP requests in tests — all use `httptest.NewServer` (§7.9)
- [ ] Verify `--dry-run`, `--json`, `--verbose` flags all work with and without sub-agents
