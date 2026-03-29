# 046 — Tool Call Trajectory in `--json` Output: Implementation Guide

## Section 1: Context Summary

**Spec:** `docs/plans/046_tool_call_trajectory_spec.md`

The `--json` output envelope already captures tool call metadata in a `tool_call_details` array, but each entry lacks the conversation turn it occurred in and how long it took. This implementation adds `turn` (0-indexed int matching the conversation loop variable) and `duration_ms` (per-call wall-clock int) to every `toolCallDetail` entry. The existing `tool_calls` integer count, `output`, `is_error`, `name`, and `input` fields are untouched. Timing is measured at the dispatch layer inside `executeToolCalls`, not in the provider. The golden test sanitizer must be extended to mask `duration_ms` inside entries (non-deterministic) while leaving `turn` unmasked (deterministic).

---

## Section 2: Implementation Checklist

### Task 1 — Add `Turn` and `DurationMs` fields to `toolCallDetail`

- [x] `cmd/run.go`: Add `Turn int` (json tag `"turn"`) and `DurationMs int64` (json tag `"duration_ms"`) fields to the `toolCallDetail` struct (currently at lines 39–44).

---

### Task 2 — Introduce `toolExecResult` wrapper struct

- [x] `cmd/run.go`: Define a new unexported struct `toolExecResult` immediately after `toolCallDetail`:
  ```
  type toolExecResult struct {
      Result   provider.ToolResult
      Duration time.Duration
  }
  ```
  This bundles each dispatch result with its measured wall-clock duration, keeping timing data co-located with the result it describes.

---

### Task 3 — Change `executeToolCalls` return type to `[]toolExecResult`

- [x] `cmd/run.go: executeToolCalls()`: Change the return type from `[]provider.ToolResult` to `[]toolExecResult`.
- [x] `cmd/run.go: executeToolCalls()`: In the sequential path (the `if len(toolCalls) == 1 || !parallel` branch), wrap each dispatch call with `time.Now()` / `time.Since()` and store into `toolExecResult{Result: ..., Duration: ...}`.
- [x] `cmd/run.go: executeToolCalls()`: In the parallel path (the goroutine branch), wrap each dispatch call inside the goroutine with `time.Now()` / `time.Since()` and send `toolExecResult{Result: ..., Duration: ...}` on the channel. Update the `indexedResult` struct to hold `toolExecResult` instead of `provider.ToolResult`.

---

### Task 4 — Update the conversation loop to use `[]toolExecResult`

- [x] `cmd/run.go: runAgent()`: Update the call site of `executeToolCalls` (line ~583) to receive `[]toolExecResult` instead of `[]provider.ToolResult`.
- [x] `cmd/run.go: runAgent()`: In the `tool_call_details` collection block (lines ~586–600), populate `Turn` from the loop variable `turn` and `DurationMs` from `results[i].Duration.Milliseconds()`. Access the result content and error flag via `results[i].Result.Content` and `results[i].Result.IsError`.
- [x] `cmd/run.go: runAgent()`: In the tool message construction block (lines ~602–607), extract `[]provider.ToolResult` from the `[]toolExecResult` slice before building `provider.Message{Role: "tool", ToolResults: ...}`.

---

### Task 5 — Update `TestToolCallDetailJSON` for new fields

- [x] `cmd/run_test.go: TestToolCallDetailJSON()`: Add `Turn: 0` and `DurationMs: 42` to the `toolCallDetail` literal. Extend the key-presence check to include `"turn"` and `"duration_ms"`. Assert `turn` marshals as a JSON number equal to `0` and `duration_ms` marshals as a JSON number equal to `42`.

---

### Task 6 — Add unit test for turn numbering across multiple conversation turns

- [x] `cmd/run_test.go`: Add a new test `TestRun_ToolCallDetails_TurnAndDuration` that:
  - Starts a mock Anthropic HTTP server that returns a tool-use response on the first call (one tool call), then a second tool-use response on the second call (one tool call), then a final text response on the third call.
  - Runs `axe run <agent> --json` against the mock.
  - Asserts `tool_call_details` has exactly 2 entries.
  - Asserts `tool_call_details[0].turn == 0` and `tool_call_details[1].turn == 1`.
  - Asserts both entries have a `duration_ms` key whose value is a non-negative number.
  - Asserts existing fields (`name`, `input`, `output`, `is_error`) are still present on both entries.

---

### Task 7 — Add unit test for parallel tool calls sharing the same turn

- [x] `cmd/run_test.go`: Add a new test `TestRun_ToolCallDetails_ParallelSameTurn` that:
  - Starts a mock Anthropic HTTP server that returns two tool calls in a single response on the first call, then a final text response on the second call.
  - Runs `axe run <agent> --json` against the mock.
  - Asserts `tool_call_details` has exactly 2 entries.
  - Asserts both entries have `turn == 0`.
  - Asserts each entry has its own `duration_ms` value (both non-negative).

---

### Task 8 — Update integration test key assertions for `tool_call_details` entries

- [x] `cmd/run_integration_test.go`: In every test that iterates `tool_call_details` entries and checks for keys (e.g., the loop at line ~1687 checking `[]string{"name", "input", "output", "is_error"}`), extend the key list to include `"turn"` and `"duration_ms"`.
- [x] `cmd/run_integration_test.go`: In the multi-turn integration test (around line 1665), after verifying entry names, assert that entries from the first LLM turn have `turn == 0` and entries from the second LLM turn have `turn == 1`. Assert all `duration_ms` values are non-negative numbers.
- [x] `cmd/run_integration_test.go`: In the `call_agent` integration test (around line 954), assert that the single `tool_call_details` entry has a `turn` key with value `0` and a `duration_ms` key with a non-negative value.

---

### Task 9 — Update golden test sanitizer to mask `duration_ms` inside entries

- [x] `cmd/golden_test.go: maskJSONOutput()`: Inside the `tool_call_details` loop (lines 81–91), after masking `output`, add a check: if the entry has a `"duration_ms"` key, replace its value with the string `"{{TOOL_DURATION_MS}}"`. Do not mask `"turn"`.

---

### Task 10 — Update `TestMaskJSONOutput` for the new sanitizer behaviour

- [x] `cmd/golden_test.go: TestMaskJSONOutput()`: Add a new table entry `"masks tool_call_details duration_ms"` whose input JSON includes a `tool_call_details` entry with a numeric `duration_ms` and a numeric `turn`. Assert the output contains `"duration_ms": "{{TOOL_DURATION_MS}}"` and that the `turn` value is preserved as a number (not replaced).

---

### Task 11 — Update any golden fixture files that contain `tool_call_details`

- [x] `cmd/testdata/golden/`: For each golden file that contains a `tool_call_details` array, regenerate it by running `UPDATE_GOLDEN=1 go test ./cmd/...` after all code changes are complete. Verify the regenerated files contain `"turn"` (numeric) and `"duration_ms": "{{TOOL_DURATION_MS}}"` in each entry.

---