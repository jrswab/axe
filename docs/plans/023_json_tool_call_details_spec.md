# 023 — JSON Tool Call Details Spec

Status: **Draft**
Depends on: M3 Single Agent Run (complete), M5 Sub-Agents (complete), M8 Integration & Polish (complete)
GitHub Issue: [#5 — Add tool call details to --json output](https://github.com/jrswab/axe/issues/5)

---

## Goal

Add a `tool_call_details` array to the `--json` output envelope so that callers have per-call visibility into which tools were invoked, what arguments were passed, what the tool returned, and whether an error occurred. This enables audit trails and debugging when axe runs inside orchestration systems that only see the final JSON output.

---

## Non-Goals

- No changes to non-JSON output (plain text to stdout is unchanged).
- No changes to `provider.ToolCall`, `provider.ToolResult`, or any struct in `internal/provider/`.
- No changes to `internal/tool/` (registry, executors, sub-agent logic).
- No changes to `--dry-run` output.
- No new CLI flags.
- No new external dependencies.
- No tracking of sub-agent internal tool calls — the parent sees `call_agent` as a single tool call with the sub-agent's final text as output. Sub-agent internals remain opaque per AGENTS.md: "Sub-agents are opaque to parents."

---

## Current Behavior

The `--json` envelope in `cmd/run.go:374-383` produces:

```json
{
  "model": "gpt-4o",
  "content": "Done.",
  "input_tokens": 1234,
  "output_tokens": 567,
  "stop_reason": "stop",
  "duration_ms": 4200,
  "tool_calls": 3
}
```

`tool_calls` is an integer — the total count of tool calls made during the conversation loop. It tells the caller how many tool calls happened but not what they were.

---

## Requested Behavior

Add a `tool_call_details` array alongside the existing `tool_calls` count:

```json
{
  "model": "gpt-4o",
  "content": "Done.",
  "input_tokens": 1234,
  "output_tokens": 567,
  "stop_reason": "stop",
  "duration_ms": 4200,
  "tool_calls": 3,
  "tool_call_details": [
    { "name": "read_file", "input": { "path": "src/main.go" }, "output": "package main...", "is_error": false },
    { "name": "run_command", "input": { "command": "go test ./..." }, "output": "PASS", "is_error": false },
    { "name": "edit_file", "input": { "path": "src/main.go", "old_string": "foo", "new_string": "bar" }, "output": "replaced 1 occurrence(s) in src/main.go", "is_error": false }
  ]
}
```

---

## Scope

Three work areas:

1. **Accumulate tool call details** — Collect per-call detail during the conversation loop in `cmd/run.go`, gated on `--json`.
2. **Serialize into JSON envelope** — Add `tool_call_details` to the output map.
3. **Update tests** — Unit tests, integration tests, golden file masking and regeneration.

---

## 1. Tool Call Detail Structure

### Fields

Each entry in the `tool_call_details` array has exactly four fields:

| Field | JSON Key | Type | Description |
|-------|----------|------|-------------|
| Name | `name` | `string` | The tool name (e.g., `"read_file"`, `"call_agent"`, `"run_command"`). Matches `provider.ToolCall.Name`. |
| Input | `input` | `object` (string keys, string values) | The arguments passed to the tool. Matches `provider.ToolCall.Arguments` (`map[string]string`). |
| Output | `output` | `string` | The tool's result content. Matches `provider.ToolResult.Content`. Truncated to 1024 bytes if longer (see Section 2). |
| IsError | `is_error` | `bool` | Whether the tool call resulted in an error. Matches `provider.ToolResult.IsError`. |

### Type

Define a private struct in `cmd/run.go`:

```go
type toolCallDetail struct {
    Name    string            `json:"name"`
    Input   map[string]string `json:"input"`
    Output  string            `json:"output"`
    IsError bool              `json:"is_error"`
}
```

### Nil Input Handling

If `provider.ToolCall.Arguments` is `nil`, the `Input` field must serialize as an empty JSON object `{}`, not `null`. When building the detail entry, if `tc.Arguments` is `nil`, substitute an empty `map[string]string{}`.

### Ordering

Entries appear in the order tool calls were executed across all conversation turns. Within a single turn, entries appear in the order the LLM requested them (matching the order in `resp.ToolCalls`). Entries from turn 1 come before entries from turn 2, and so on.

---

## 2. Output Truncation

### Rationale

Tool outputs can be large (e.g., `read_file` on a big file, `run_command` with verbose output). To keep the JSON envelope predictable in size, output is truncated by default.

### Rules

- Maximum output size: **1024 bytes**.
- Truncation is byte-level (not rune-level). This is acceptable because tool outputs are overwhelmingly ASCII. If truncation lands in the middle of a multi-byte UTF-8 character, the truncated output may contain an incomplete character — this is tolerable for an audit field.
- When truncated, append the literal string `"... (truncated)"` after the first 1024 bytes.
- The truncation suffix itself is NOT counted toward the 1024-byte limit. The total output string length is at most `1024 + len("... (truncated)")` = 1039 bytes.
- Outputs at or under 1024 bytes are passed through unchanged.

### Truncation Helper

Define a private function in `cmd/run.go`:

```
truncateOutput(s string) string
```

- If `len(s) <= 1024`: return `s` unchanged.
- Otherwise: return `s[:1024] + "... (truncated)"`.

### Constant

Define a private constant:

```
maxToolOutputBytes = 1024
```

---

## 3. Accumulation Logic

### Gate on `--json`

Tool call details are **only accumulated when `--json` is requested**. When `--json` is not set, no `toolCallDetail` slice is allocated and no detail-building code runs. This avoids unnecessary memory allocation and work for the common case (plain text output).

### Location

The accumulation happens in the `runAgent` function in `cmd/run.go`, inside the conversation loop (lines 306-356 in the current code).

### Variable Declaration

Declare `var allToolCallDetails []toolCallDetail` alongside the existing `totalToolCalls` variable (line ~283). Do NOT eagerly allocate with `make` — leave as `nil` until needed.

### Building Details

After `executeToolCalls` returns `results` (currently line 347-348), if `jsonOutput` is true, iterate over `resp.ToolCalls` and `results` in lockstep:

```
if jsonOutput {
    for i, tc := range resp.ToolCalls {
        input := tc.Arguments
        if input == nil {
            input = map[string]string{}
        }
        allToolCallDetails = append(allToolCallDetails, toolCallDetail{
            Name:    tc.Name,
            Input:   input,
            Output:  truncateOutput(results[i].Content),
            IsError: results[i].IsError,
        })
    }
}
```

### Invariant

After the conversation loop completes, `len(allToolCallDetails)` must equal `totalToolCalls` when `jsonOutput` is true. This is guaranteed by construction since both are incremented from the same `resp.ToolCalls` slice.

### Single-Shot Path

When no tools are configured (the single-shot path at lines 285-303), `allToolCallDetails` remains `nil`. This is handled in Section 4.

---

## 4. JSON Envelope Serialization

### Location

The JSON envelope construction at `cmd/run.go:374-383`.

### Change

Before adding `tool_call_details` to the envelope, ensure the slice is non-nil:

```
if allToolCallDetails == nil {
    allToolCallDetails = make([]toolCallDetail, 0)
}
```

Then add to the envelope map:

```
envelope["tool_call_details"] = allToolCallDetails
```

### Output Guarantee

- When tools were called: `tool_call_details` is a non-empty JSON array.
- When no tools were called: `tool_call_details` is an empty JSON array `[]`.
- The field is never `null` and never omitted.

### Backward Compatibility

The existing `tool_calls` integer field is unchanged. The new `tool_call_details` array is purely additive. Consumers that ignore unknown fields will not break.

---

## 5. Edge Cases

### 5a. `call_agent` Tool Calls

`call_agent` entries appear in `tool_call_details` like any other tool:
- `name`: `"call_agent"`
- `input`: `{"agent": "...", "task": "...", "context": "..."}` (context may be absent if not provided by the LLM)
- `output`: The sub-agent's final text result (truncated to 1024 bytes)
- `is_error`: `true` if the sub-agent failed, `false` otherwise

The sub-agent's own internal tool calls do NOT appear in the parent's `tool_call_details`. Only the `call_agent` invocation itself is recorded.

### 5b. Error Tool Results

When a tool call fails (e.g., path traversal rejected, command not found, sub-agent depth exceeded):
- `is_error`: `true`
- `output`: The error message string from `provider.ToolResult.Content` (truncated to 1024 bytes)

### 5c. Parallel Tool Execution

When multiple tool calls execute in parallel within a single turn, all entries are recorded in the order they appear in `resp.ToolCalls` (the LLM's requested order), NOT in the order they complete. This is deterministic because `results` is a positional slice matching `resp.ToolCalls` by index.

### 5d. Maximum Conversation Turns Exceeded

If the agent hits the 50-turn safety limit, the function returns an `ExitError` before reaching the JSON envelope construction. No JSON output is produced. `tool_call_details` is not relevant in this case.

### 5e. Empty Arguments

If the LLM sends a tool call with no arguments (empty map), `input` serializes as `{}`.

### 5f. Empty Output

If a tool returns an empty string as content, `output` serializes as `""`. No special handling needed.

---

## 6. Golden File Updates

### Masking

The `maskJSONOutput` function in `cmd/golden_test.go` parses the JSON envelope and replaces non-deterministic values. It currently masks `duration_ms`.

For `tool_call_details`, the `output` field may contain non-deterministic content (e.g., `list_directory` output depends on the filesystem). The masking function must replace each `output` value in the `tool_call_details` array with the placeholder `"{{TOOL_OUTPUT}}"`.

### Masking Logic

After replacing `duration_ms`, check if `tool_call_details` is present and is an array. For each entry, if the entry is a map and has an `"output"` key, replace its value with `"{{TOOL_OUTPUT}}"`.

### Affected Golden Files

All 6 JSON golden files must be regenerated:

| File | Expected `tool_call_details` |
|------|------------------------------|
| `basic.json` | `[]` |
| `with_skill.json` | `[]` |
| `with_files.json` | `[]` |
| `with_memory.json` | `[]` |
| `with_subagents.json` | Array with 2 entries: `call_agent` for "basic" and `call_agent` for "with_skill" |
| `with_tools.json` | Array with 1 entry: `list_directory` with `path: "."` |

### Regeneration

Run tests with `UPDATE_GOLDEN=1` or `--update-golden` flag after implementation to regenerate all golden files.

---

## 7. Tests

All tests call real code. No mocking of tool executors or the `truncateOutput` function. The mock provider controls what tool calls the LLM requests; actual tool execution happens against real filesystems / real shell.

### 7a. Unit Test: `truncateOutput`

**File:** `cmd/run_test.go`

**Test Name:** `TestTruncateOutput`

Table-driven test with these cases:

| Case | Input | Expected Output |
|------|-------|-----------------|
| Short string | `"hello"` (5 bytes) | `"hello"` (unchanged) |
| Exactly at limit | 1024-byte string | Same string (unchanged) |
| Over limit | 1025-byte string | First 1024 bytes + `"... (truncated)"` |
| Well over limit | 2048-byte string | First 1024 bytes + `"... (truncated)"` |
| Empty string | `""` | `""` |

### 7b. Unit Test: `toolCallDetail` JSON Serialization

**File:** `cmd/run_test.go`

**Test Name:** `TestToolCallDetailJSON`

Verify that `json.Marshal` on a `toolCallDetail` struct produces the expected JSON with correct field names:

- All fields present with correct `json` tags (`name`, `input`, `output`, `is_error`).
- `is_error` serializes as `false` (not omitted) when not an error.
- `input` serializes as `{}` when the map is empty.
- `input` serializes as `null` is NOT acceptable — test must verify `{}` when map is `map[string]string{}`.

### 7c. Integration Test: Zero Tool Calls

**File:** `cmd/run_integration_test.go`

Update `TestIntegration_JSONOutput_Structure` (the existing test at line ~660).

**Additional Assertions:**

- `result["tool_call_details"]` exists and is a JSON array.
- The array has length 0.
- The value is `[]`, not `null`.

### 7d. Integration Test: Tool Calls with Sub-Agents

**File:** `cmd/run_integration_test.go`

Update `TestIntegration_JSONOutput_WithToolCalls` (the existing test at line ~727).

**Additional Assertions:**

- `result["tool_call_details"]` exists and is a JSON array.
- The array has length 1.
- Entry 0: `name` is `"call_agent"`, `input` contains keys `"agent"` and `"task"`, `is_error` is `false`, `output` is a non-empty string (the sub-agent's response).

### 7e. Integration Test: Built-in Tool Calls

**File:** `cmd/run_integration_test.go`

Update `TestIntegration_JSONOutput_BuiltinTools` (the existing test at line ~1390).

**Additional Assertions:**

- `result["tool_call_details"]` exists and is a JSON array.
- The array has length 3 (matching `tool_calls: 3`).
- Each entry has all four fields (`name`, `input`, `output`, `is_error`).
- Entry names match the tool calls in the mock response queue order.
- All entries have `is_error: false`.

### 7f. Integration Test: Truncation

**File:** `cmd/run_integration_test.go`

**Test Name:** `TestIntegration_JSONOutput_ToolCallDetails_Truncation`

**Purpose:** Verify that tool output exceeding 1024 bytes is truncated in `tool_call_details`.

**Setup:**

1. Create a mock server with a 2-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `read_file` with `path: "big.txt"`.
   - Response 2: `AnthropicResponse("done")`.
2. Agent config: `tools = ["read_file"]`.
3. Create temp workdir with `big.txt` containing 2048 bytes of content.
4. Pass `--json` flag.

**Assertions:**

- Parse stdout as JSON.
- `result["tool_call_details"]` array has length 1.
- Entry 0 `output` string ends with `"... (truncated)"`.
- Entry 0 `output` string has length `1024 + len("... (truncated)")` = 1039.

### 7g. Integration Test: Error Tool Result in Details

**File:** `cmd/run_integration_test.go`

**Test Name:** `TestIntegration_JSONOutput_ToolCallDetails_Error`

**Purpose:** Verify that a failed tool call appears in `tool_call_details` with `is_error: true`.

**Setup:**

1. Create a mock server with a 2-response queue:
   - Response 1: `AnthropicToolUseResponse` requesting `read_file` with `path: "../escape.txt"` (path traversal — will be rejected by tool executor).
   - Response 2: `AnthropicResponse("handled")`.
2. Agent config: `tools = ["read_file"]`.
3. Create temp workdir.
4. Pass `--json` flag.

**Assertions:**

- Parse stdout as JSON.
- `result["tool_call_details"]` array has length 1.
- Entry 0 `name` is `"read_file"`.
- Entry 0 `is_error` is `true`.
- Entry 0 `output` contains an error message (non-empty string).

### 7h. Unit Test: No Accumulation Without `--json`

**File:** `cmd/run_test.go`

**Test Name:** `TestRun_ToolCallDetails_NotAccumulated_WithoutJSON`

**Purpose:** Verify that tool call details are not accumulated (and `tool_call_details` does not appear in output) when `--json` is not passed.

**Setup:**

1. Create a mock server with a tool-call conversation (2 responses).
2. Agent config with tools.
3. Do NOT pass `--json`.

**Assertions:**

- `rootCmd.Execute()` returns `nil`.
- Stdout does NOT contain `"tool_call_details"`.
- Stdout contains the plain text response (not JSON).

### 7i. Golden File Tests

All 6 JSON golden files are verified via the existing `TestGolden` test after regeneration. No new golden test cases are needed — the existing matrix covers all fixture agents.

---

## 8. Full Test Pass

After all changes, `make test` must pass with zero failures. This includes:

- All existing unit tests.
- All existing integration tests (with updated assertions).
- All golden file tests (with regenerated golden files).
- All smoke tests.
- All new tests from Section 7.

---

## File Change Summary

| File | Type | Description |
|------|------|-------------|
| `cmd/run.go` | Modify | Add `toolCallDetail` struct, `truncateOutput` helper, `maxToolOutputBytes` constant, accumulation logic in conversation loop, `tool_call_details` in JSON envelope |
| `cmd/run_test.go` | Modify | Add `TestTruncateOutput`, `TestToolCallDetailJSON`, `TestRun_ToolCallDetails_NotAccumulated_WithoutJSON` |
| `cmd/run_integration_test.go` | Modify | Update 3 existing tests with `tool_call_details` assertions, add `TestIntegration_JSONOutput_ToolCallDetails_Truncation`, add `TestIntegration_JSONOutput_ToolCallDetails_Error` |
| `cmd/golden_test.go` | Modify | Update `maskJSONOutput` to mask `output` values in `tool_call_details` entries |
| `cmd/testdata/golden/json/basic.json` | Regenerate | Add `tool_call_details: []` |
| `cmd/testdata/golden/json/with_skill.json` | Regenerate | Add `tool_call_details: []` |
| `cmd/testdata/golden/json/with_files.json` | Regenerate | Add `tool_call_details: []` |
| `cmd/testdata/golden/json/with_memory.json` | Regenerate | Add `tool_call_details: []` |
| `cmd/testdata/golden/json/with_subagents.json` | Regenerate | Add `tool_call_details` with 2 `call_agent` entries |
| `cmd/testdata/golden/json/with_tools.json` | Regenerate | Add `tool_call_details` with 1 `list_directory` entry |

---

## Constraints

- No new external dependencies.
- No changes to `internal/` packages.
- The `tool_calls` integer field is preserved — `tool_call_details` is purely additive.
- Detail accumulation is gated on `--json` — zero overhead for non-JSON runs.
- Output truncation at 1024 bytes by default — no flag to override (defer to a future issue if needed).
- All tests call real tool executors — no mocking of tool logic.
- No `t.Parallel()` in integration tests (global cobra state + `t.Setenv`).
- Agent configs in integration tests are written inline (not fixture files).
- All test helpers call `t.Helper()`.

---

## Acceptance Criteria

1. `--json` output includes `tool_call_details` as an array (empty `[]` when no tools called, populated when tools called).
2. Each entry has exactly four fields: `name`, `input`, `output`, `is_error`.
3. `input` is always a JSON object (`{}` when empty, never `null`).
4. `output` is truncated to 1024 bytes with a `"... (truncated)"` suffix when exceeded.
5. `tool_calls` integer field is unchanged (backward compatible).
6. `tool_call_details` is NOT accumulated when `--json` is not passed.
7. Sub-agent internal tool calls do NOT appear in the parent's `tool_call_details`.
8. Error tool results appear with `is_error: true` and the error message as `output`.
9. All 6 JSON golden files are updated and pass.
10. `make test` passes with zero failures.
