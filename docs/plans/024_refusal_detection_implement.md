# Implementation: Refusal Detection with `refused` Flag in `--json` Output

**Spec:** [024_refusal_detection_spec.md](./024_refusal_detection_spec.md)
**GitHub Issue:** [#8](https://github.com/jrswab/axe/issues/8)
**Created:** 2026-03-05

---

## Phase 1: Refusal Detection Package (TDD)

Create `internal/refusal/` package with `Detect(content string) bool`.

### 1.1 Write Tests (Red)

- [x] Create `internal/refusal/refusal_test.go` with `package refusal`
- [x] Implement `TestDetect_EmptyString` — verify `Detect("")` returns `false`
- [x] Implement `TestDetect_NormalContent` — table-driven, verify `false` for all 7 normal strings from spec section 7.2
- [x] Implement `TestDetect_SimpleRefusals` — table-driven, verify `true` for all 9 refusal patterns from Requirement 1.2
- [x] Implement `TestDetect_CaseInsensitive` — table-driven, verify `true` for 4 varied-casing refusals
- [x] Implement `TestDetect_RefusalAfterPreamble` — table-driven, verify `true` for 4 preamble+refusal strings
- [x] Implement `TestDetect_AsAnAI_Compound` — table-driven, verify `true` for 4 compound patterns
- [x] Implement `TestDetect_AsAnAI_NoRefusalIndicator` — table-driven, verify `false` for 2 `"as an ai"` strings without refusal indicators
- [x] Implement `TestDetect_WhitespaceOnly` — verify `Detect("   \n\t  ")` returns `false`
- [x] Implement `TestDetect_LongContent_RefusalAtEnd` — 10,000 chars of normal text + refusal at end, verify `true`
- [x] Implement `TestDetect_LongContent_NoRefusal` — 10,000 chars of normal text, no refusal, verify `false`
- [x] Run tests, confirm all fail (red phase)

### 1.2 Implement Detection (Green)

- [x] Create `internal/refusal/refusal.go` with `package refusal`
- [x] Define `Detect(content string) bool` function
- [x] Add the 9 simple refusal patterns as a `[]string` slice (lowercase): `"i cannot"`, `"i can't"`, `"i'm unable to"`, `"i am unable to"`, `"i'm not able to"`, `"i am not able to"`, `"i must decline"`, `"i don't have the ability"`, `"i do not have the ability"`
- [x] Add the 7 compound refusal indicators as a `[]string` slice (lowercase): `"i cannot"`, `"i can't"`, `"i'm unable"`, `"i am unable"`, `"i'm not able"`, `"i am not able"`, `"i must decline"`
- [x] Implement: `strings.ToLower` the content once, then iterate simple patterns with `strings.Contains`; if none match, check compound logic (`"as an ai"` present AND any compound indicator present)
- [x] Ensure function is pure/read-only — no modification of input, no side effects
- [x] No regex — only `strings.ToLower` + `strings.Contains`
- [x] Run tests, confirm all pass (green phase)

---

## Phase 2: JSON Envelope Integration (TDD)

Wire `refusal.Detect` into the `--json` output in `cmd/run.go`.

### 2.1 Write/Update Tests (Red)

- [x] In `cmd/run_test.go`: modify `TestRun_JSONOutput` (~line 467) to assert `"refused"` field is present and equals `false` for the mock response `"Hello from mock"`
- [x] In `cmd/run_integration_test.go`: modify `TestIntegration_JSONOutput_Structure` (~line 663) to add `"refused"` to the `requiredFields` slice and verify value is `false`
- [x] In `cmd/run_integration_test.go`: add `TestIntegration_JSONOutput_RefusalDetected` — mock server returns `"I'm sorry, but I cannot assist with that request."`, run with `--json`, verify `"refused": true`, all other fields present, exit code 0
- [x] In `cmd/run_integration_test.go`: add `TestIntegration_JSONOutput_RefusalDetected_WithToolCalls` — mock server: turn 1 returns tool call, turn 2 returns refusal, verify `"refused": true`, `"tool_calls": 1`, exit code 0
- [x] In `cmd/run_integration_test.go`: add `TestIntegration_JSONOutput_NoRefusal_WithToolCalls` — mock server: turn 1 returns tool call, turn 2 returns normal response, verify `"refused": false`, `"tool_calls": 1`, exit code 0
- [x] Run tests, confirm new/modified tests fail (red phase)

### 2.2 Implement Envelope Change (Green)

- [x] In `cmd/run.go`: add import for `"github.com/jrswab/axe/internal/refusal"`
- [x] In `cmd/run.go` (~line 420): add `"refused": refusal.Detect(resp.Content)` to the envelope `map[string]interface{}`
- [x] Run tests, confirm all pass (green phase)

---

## Phase 3: Golden File Updates

Update all golden JSON files to include the new `"refused"` field.

- [x] Run golden tests with `-update-golden` flag to regenerate all golden JSON files: `go test ./cmd/ -run TestGolden -update-golden`
- [x] Verify `basic.json` includes `"refused": false`
- [x] Verify `with_skill.json` includes `"refused": false`
- [x] Verify `with_files.json` includes `"refused": false`
- [x] Verify `with_memory.json` includes `"refused": false`
- [x] Verify `with_subagents.json` includes `"refused": false`
- [x] Verify `with_tools.json` includes `"refused": false`
- [x] Verify `maskJSONOutput` in `cmd/golden_test.go` does NOT need changes (`"refused"` is deterministic, no masking needed)

---

## Phase 4: Full Test Suite Validation

- [x] Run `make test` — confirm zero failures
- [x] Verify no new dependencies in `go.mod` (only `spf13/cobra` and `BurntSushi/toml`)
- [x] Verify plain text output (without `--json`) is unchanged — no refusal information leaks to stdout/stderr
- [x] Verify exit code is 0 for refusal responses

---

## Constraints Checklist

- [x] No new external dependencies (`go.mod` unchanged)
- [x] No regex in `internal/refusal/` — only `strings.ToLower` + `strings.Contains`
- [x] `Detect` is pure/read-only (no side effects, no input modification)
- [x] `"refused"` field only in `--json` output, never in plain text, stderr, verbose, or dry-run
- [x] `"refused"` always present in `--json` (never conditionally omitted)
- [x] Exit codes unchanged (refusals still exit 0)
- [x] Memory unchanged (refusals still appended)
- [x] Pattern list is hard-coded (not configurable)
- [x] Cross-platform: Linux, macOS, Windows (stdlib string ops only)
