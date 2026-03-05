# 023 — JSON Tool Call Details: Implementation Checklist

Spec: `docs/plans/023_json_tool_call_details_spec.md`
GitHub Issue: [#5](https://github.com/jrswab/axe/issues/5)

---

## Phase 1: Core Types and Helpers (`cmd/run.go`)

These are pure additions with no dependencies on other changes. All subsequent phases depend on these.

- [x] Add `maxToolOutputBytes = 1024` private constant (after existing constants, before `runAgent`)
- [x] Add `truncateOutput(s string) string` private function — returns `s` unchanged if `len(s) <= 1024`, otherwise returns `s[:1024] + "... (truncated)"`
- [x] Add `toolCallDetail` private struct with JSON tags: `Name string "json:\"name\""`, `Input map[string]string "json:\"input\""`, `Output string "json:\"output\""`, `IsError bool "json:\"is_error\""`

## Phase 2: Unit Tests for Phase 1 (`cmd/run_test.go`)

Tests for the types and helpers added in Phase 1. Write these before Phase 3 (red/green TDD).

- [x] Add `TestTruncateOutput` — table-driven with 5 cases: short string (5 bytes, unchanged), exactly 1024 bytes (unchanged), 1025 bytes (truncated), 2048 bytes (truncated), empty string (unchanged). Verify output length is correct and suffix `"... (truncated)"` is present when truncated.
- [x] Add `TestToolCallDetailJSON` — marshal a `toolCallDetail` struct and verify: all four JSON keys present (`name`, `input`, `output`, `is_error`), `is_error` serializes as `false` (not omitted), empty `map[string]string{}` serializes as `{}` (not `null`), non-empty input map serializes correctly with string keys and values.

## Phase 3: Accumulation Logic (`cmd/run.go`)

Depends on Phase 1. Adds the collection logic gated on `--json`.

- [x] Declare `var allToolCallDetails []toolCallDetail` alongside `totalToolCalls` at line ~283. Leave as `nil` (no eager allocation).
- [x] After `executeToolCalls` returns `results` (line ~347-348), add a `if jsonOutput { ... }` block that iterates `resp.ToolCalls` and `results` in lockstep: for each pair, build a `toolCallDetail` with `tc.Name`, `tc.Arguments` (substituting `map[string]string{}` if `nil`), `truncateOutput(results[i].Content)`, and `results[i].IsError`. Append each to `allToolCallDetails`.

## Phase 4: JSON Envelope Update (`cmd/run.go`)

Depends on Phase 3. Adds the new field to the JSON output.

- [x] Before the existing envelope map construction (line ~374), add nil-guard: `if allToolCallDetails == nil { allToolCallDetails = make([]toolCallDetail, 0) }` — ensures the field serializes as `[]` not `null`.
- [x] Add `envelope["tool_call_details"] = allToolCallDetails` to the envelope map (after the existing `"tool_calls"` entry).

## Phase 5: Golden File Masking (`cmd/golden_test.go`)

Depends on Phase 4 (golden tests will fail without the new field). Must be done before golden file regeneration.

- [x] Update `maskJSONOutput` (line 67): after the existing `duration_ms` masking, check if `envelope["tool_call_details"]` exists and is a `[]interface{}`. For each entry that is a `map[string]interface{}` with an `"output"` key, replace the value with `"{{TOOL_OUTPUT}}"`.

## Phase 6: Golden File Regeneration

Depends on Phase 5. Run regeneration to update all 6 JSON golden files.

- [x] Run `UPDATE_GOLDEN=1 go test ./cmd/ -run TestGolden` to regenerate all golden files.
- [x] Verify `cmd/testdata/golden/json/basic.json` contains `"tool_call_details": []`
- [x] Verify `cmd/testdata/golden/json/with_skill.json` contains `"tool_call_details": []`
- [x] Verify `cmd/testdata/golden/json/with_files.json` contains `"tool_call_details": []`
- [x] Verify `cmd/testdata/golden/json/with_memory.json` contains `"tool_call_details": []`
- [x] Verify `cmd/testdata/golden/json/with_subagents.json` contains `"tool_call_details"` with 2 `call_agent` entries (outputs masked as `"{{TOOL_OUTPUT}}"`)
- [x] Verify `cmd/testdata/golden/json/with_tools.json` contains `"tool_call_details"` with 1 `list_directory` entry (output masked as `"{{TOOL_OUTPUT}}"`)

## Phase 7: Update Existing Integration Tests (`cmd/run_integration_test.go`)

Depends on Phase 4. Adds `tool_call_details` assertions to existing tests.

- [x] Update `TestIntegration_JSONOutput_Structure` (line 663): add `"tool_call_details"` to the required-fields check (line ~695), assert `result["tool_call_details"]` is a JSON array of length 0, assert the value is `[]` not `null`.
- [x] Update `TestIntegration_JSONOutput_WithToolCalls` (line 727): assert `result["tool_call_details"]` is a JSON array of length 1, assert entry 0 has `name` == `"call_agent"`, `input` contains keys `"agent"` and `"task"`, `is_error` == `false`, `output` is a non-empty string.
- [x] Update `TestIntegration_JSONOutput_WithBuiltInToolCalls` (line 1402): assert `result["tool_call_details"]` is a JSON array of length 3, assert each entry has all four fields (`name`, `input`, `output`, `is_error`), assert entry names match tool call order from mock response queue, assert all entries have `is_error` == `false`.

## Phase 8: New Integration Tests (`cmd/run_integration_test.go`)

Depends on Phase 4. New test functions for truncation and error cases.

- [x] Add `TestIntegration_JSONOutput_ToolCallDetails_Truncation`: mock server returns `read_file` tool call for `big.txt` (2048 bytes), then final response. Assert `tool_call_details[0].output` ends with `"... (truncated)"` and has length 1039 (1024 + 15).
- [x] Add `TestIntegration_JSONOutput_ToolCallDetails_Error`: mock server returns `read_file` with path `"../escape.txt"` (path traversal rejection), then final response. Assert `tool_call_details[0].name` == `"read_file"`, `is_error` == `true`, `output` is a non-empty error message string.

## Phase 9: Non-JSON Gate Test (`cmd/run_test.go`)

Depends on Phase 3. Verifies detail accumulation is skipped without `--json`.

- [x] Add `TestRun_ToolCallDetails_NotAccumulated_WithoutJSON`: mock server with a tool-call conversation (2 responses), agent config with tools, do NOT pass `--json`. Assert `rootCmd.Execute()` returns `nil`, stdout does NOT contain `"tool_call_details"`, stdout contains the plain text response.

## Phase 10: Full Test Pass

Depends on all previous phases.

- [x] Run `make test` and verify zero failures across all unit, integration, golden, and smoke tests.
