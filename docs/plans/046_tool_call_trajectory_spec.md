---

# 046 — Tool Call Trajectory in `--json` Output

## Section 1: Context & Constraints

### Milestone Source

**GitHub Issue #62** — `feat: include tool call sequence in --json output`
**Milestone:** 1.7.0
**Labels:** enhancement, priority: medium

### Codebase Patterns Relevant to This Milestone

**JSON output envelope** (`cmd/run.go`):
The `--json` flag produces a single JSON object on stdout. The envelope is built as `map[string]interface{}` and marshaled at the end of `runAgent`. The relevant existing fields:

- `"tool_calls"` — `int`: total count of tool calls across all turns (already exists; must not be renamed or removed)
- `"tool_call_details"` — `[]toolCallDetail`: flat array of per-call metadata (already exists; must be enhanced, not replaced)

**`toolCallDetail` struct** (`cmd/run.go:39-44`):
```go
type toolCallDetail struct {
    Name    string            `json:"name"`
    Input   map[string]string `json:"input"`
    Output  string            `json:"output"`
    IsError bool              `json:"is_error"`
}
```

**Conversation loop** (`cmd/run.go:531-608`):
- Iterates `for turn := 0; turn < maxConversationTurns; turn++`
- Each iteration: calls `prov.Send()`, checks for tool calls, calls `executeToolCalls()`, appends results to messages, loops
- Tool call details are collected inside the loop at lines 586-600 (only when `jsonOutput` is true)
- The loop variable `turn` is 0-indexed internally

**`executeToolCalls` function** (`cmd/run.go:792-853`):
- Returns `[]provider.ToolResult`
- Supports both sequential (single call or `parallel=false`) and parallel (goroutines) execution paths
- Does not currently measure per-call duration

**Golden test sanitizer** (`cmd/golden_test.go:81-91`):
- Masks `duration_ms` at the top-level envelope
- Masks `output` inside each `tool_call_details` entry
- Must be updated to also mask `duration_ms` inside each `tool_call_details` entry (non-deterministic)

**No-tools path** (`cmd/run.go:~490-528`):
- Handles agents with no tools configured; calls `Send()` once and exits without entering the conversation loop
- `tool_call_details` is always `[]` on this path; no special handling required

### Decisions Already Made

1. **Enhance `tool_call_details`, do not add a new field.** Adding `turn` and `duration_ms` to existing entries is the least disruptive approach. The existing `"tool_calls"` integer count field is not renamed.
2. **Keep the `output` field.** Backward compatibility is preserved. The output is already truncated to 1024 bytes.
3. **`turn` is 0-indexed.** The `turn` value in each entry matches the internal loop variable directly (first LLM response with tool calls = `turn: 0`).
4. **`duration_ms` is per-tool-call.** Measured from immediately before dispatch to immediately after, for each individual tool call. For parallel calls, each call is timed independently.

### Approaches Ruled Out

- **Renaming `"tool_calls"` integer to `"tool_call_count"`** — breaking change, rejected.
- **Adding a separate `"tool_call_trajectory"` field** — redundant with `tool_call_details`, rejected.
- **Removing the `output` field** — breaking change, rejected.
- **1-indexed `turn`** — rejected in favor of 0-indexed to match the internal loop variable.
- **Measuring duration at the provider level** — timing belongs at the dispatch layer in `cmd/run.go`, not in the provider interface.

### Constraints

- The `"tool_calls"` integer field and all other existing envelope fields must remain unchanged.
- The `output` and `is_error` fields in `tool_call_details` entries must remain unchanged.
- `duration_ms` inside `tool_call_details` entries is non-deterministic and must be masked in golden tests.
- `turn` inside `tool_call_details` entries is deterministic and must NOT be masked in golden tests.
- Both sequential and parallel tool execution paths must produce correct `duration_ms` values.
- The no-tools path (no conversation loop) requires no changes.

---

## Section 2: Requirements

### 2.1 Enhanced `tool_call_details` Entry Shape

Each entry in the `tool_call_details` array must include two new fields in addition to the existing `name`, `input`, `output`, and `is_error`:

| Field | Type | Description |
|-------|------|-------------|
| `turn` | integer | The 0-indexed conversation turn in which this tool call was made. All tool calls from the same LLM response share the same `turn` value. |
| `duration_ms` | integer | Wall-clock time in milliseconds from when the tool dispatch began to when it returned a result. |

**Resulting entry shape:**
```json
{
  "turn": 0,
  "name": "read_file",
  "input": { "path": "main.go" },
  "output": "...",
  "is_error": false,
  "duration_ms": 42
}
```

### 2.2 Turn Numbering

- `turn` is 0-indexed: the first LLM response that produces tool calls has `turn: 0`, the second has `turn: 1`, and so on.
- All tool calls produced by the same LLM response (including parallel calls) share the same `turn` value.
- `turn` values are sequential integers with no gaps, incrementing by 1 for each LLM response that produces tool calls.
- If the agent produces no tool calls at all, `tool_call_details` remains an empty array and no `turn` values are emitted.

### 2.3 Duration Measurement

- `duration_ms` measures the wall-clock time for a single tool call dispatch: from immediately before the dispatch call to immediately after it returns.
- For parallel tool calls (multiple calls in the same turn), each call is timed independently. Their `duration_ms` values may overlap in real time but are each measured individually.
- For sequential tool calls, each call is timed independently in sequence.
- `duration_ms` must be a non-negative integer. A tool call that completes in under 1ms must report `0`.
- `duration_ms` applies to all tool types: built-in tools, MCP tools, and `call_agent` sub-agent invocations.

### 2.4 Backward Compatibility

- The existing `"tool_calls"` integer field in the envelope is unchanged.
- The existing `name`, `input`, `output`, and `is_error` fields in each `tool_call_details` entry are unchanged.
- All other envelope fields are unchanged.
- Consumers that do not read `turn` or `duration_ms` are unaffected.

### 2.5 Non-JSON Path

- When `--json` is not set, no timing or turn tracking occurs. The plain text output path is unchanged.

### 2.6 Golden Test Sanitization

- The golden test sanitizer must mask `duration_ms` inside each `tool_call_details` entry (replace with a placeholder, e.g., `"{{TOOL_DURATION_MS}}"`) because it is non-deterministic.
- The `turn` field inside `tool_call_details` entries must NOT be masked; it is deterministic and its value must be verified.

### 2.7 Edge Cases

**Single tool call, single turn:**
- `tool_call_details` has one entry with `turn: 0` and a non-negative `duration_ms`.

**Multiple parallel tool calls in one turn:**
- All entries share `turn: 0`. Each has its own independently measured `duration_ms`.

**Multiple sequential turns:**
- Turn 0 entries have `turn: 0`, turn 1 entries have `turn: 1`, etc.

**Tool call that errors:**
- `is_error: true`, `duration_ms` is still measured and reported (the duration of the failed call).

**`call_agent` sub-agent invocation:**
- Appears in `tool_call_details` with `name: "call_agent"`, `turn` and `duration_ms` populated like any other tool call. No additional sub-agent-specific fields are added.

**MCP tool call:**
- Appears in `tool_call_details` with the namespaced tool name (e.g., `"server-name.tool-name"`), `turn` and `duration_ms` populated like any other tool call.

**Budget exceeded mid-loop:**
- Any tool calls that were dispatched before the budget check still appear in `tool_call_details` with their `turn` and `duration_ms`.

**Maximum turns reached (50):**
- All tool calls from all turns that were dispatched appear in `tool_call_details` with correct `turn` values (0 through 49).

**No tool calls (agent has no tools or LLM never calls tools):**
- `tool_call_details` is `[]`. No `turn` or `duration_ms` values are emitted.

---