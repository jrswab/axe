# Specification: Refusal Detection with `refused` Flag in `--json` Output

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-05
**GitHub Issue:** [#8](https://github.com/jrswab/axe/issues/8)
**Scope:** Detect LLM refusals and surface a `"refused"` boolean field in `--json` output

---

## 1. Purpose

When an LLM refuses a request (e.g., "I cannot help with that"), axe currently treats it as a normal successful completion. The `content` field contains the refusal text, `stop_reason` is `"stop"` or `"end_turn"`, and exit code is 0. There is no way to programmatically distinguish a refusal from a real answer without inspecting the content.

Orchestration systems (like Agent Engine) need to detect refusals to mark runs appropriately, alert users, and potentially retry with different prompts. Centralizing this detection inside axe keeps the logic in one place rather than duplicated across every integration.

This milestone adds a `"refused"` boolean field to the `--json` output envelope. The field is always present when `--json` is used. It is `true` when the LLM response content matches common refusal patterns, `false` otherwise.

This milestone does **not** change exit codes, modify `stop_reason`, add a CLI flag to control refusal detection, or surface refusal status outside of `--json` output.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Detection scope:** Refusal patterns are matched against the full response content, not just the beginning. Refusals sometimes come after a preamble (e.g., "I appreciate your question, however I cannot...").
2. **Field presence:** The `"refused"` field is always present in `--json` output — both `true` and `false` values are emitted. It is never conditionally omitted. This provides a consistent schema for consumers.
3. **Exit code unchanged:** A refusal is still a successful LLM call. The exit code remains 0. Orchestration systems use the `"refused"` field, not the exit code, to distinguish refusals.
4. **`stop_reason` unchanged:** The original `stop_reason` from the LLM is preserved. No synthetic `"refusal"` stop reason is introduced.
5. **No effect on non-JSON output:** When `--json` is not used, stdout contains only the raw LLM content. Refusal detection does not affect plain text output in any way. Axe output must remain safe to pipe.
6. **Pattern-based, not exhaustive:** The detection uses a fixed set of common refusal patterns. False positives are less harmful than false negatives for this use case.
7. **Case-insensitive matching:** All pattern matching is case-insensitive.
8. **New package:** Refusal detection logic lives in a new `internal/refusal` package to keep it isolated and testable, separate from the CLI layer.
9. **No new external dependencies.** Continue using stdlib only plus existing `spf13/cobra` and `BurntSushi/toml`.

---

## 3. Requirements

### 3.1 Refusal Detection Package (`internal/refusal/`)

**Requirement 1.1:** Create a new package `internal/refusal` with a single exported function:

```go
func Detect(content string) bool
```

- Returns `true` if the content matches any common LLM refusal pattern.
- Returns `false` if the content does not match any refusal pattern.
- Returns `false` for empty string input.
- Matching is case-insensitive.
- Matching is performed against the full content string (not just the beginning).

**Requirement 1.2:** The function must check for the following refusal patterns (case-insensitive substring matching):

| Pattern | Rationale |
|---------|-----------|
| `"i cannot"` | Direct refusal |
| `"i can't"` | Contraction form of "I cannot" |
| `"i'm unable to"` | Polite refusal |
| `"i am unable to"` | Expanded form |
| `"i'm not able to"` | Alternative phrasing |
| `"i am not able to"` | Expanded form |
| `"i must decline"` | Formal refusal |
| `"i don't have the ability"` | Capability-based refusal |
| `"i do not have the ability"` | Expanded form |

**Requirement 1.3:** In addition to the simple substring patterns above, the function must detect the compound pattern where the content contains `"as an ai"` AND also contains at least one of these refusal indicators (case-insensitive):

- `"i cannot"`
- `"i can't"`
- `"i'm unable"`
- `"i am unable"`
- `"i'm not able"`
- `"i am not able"`
- `"i must decline"`

The compound `"as an ai"` pattern only triggers a refusal when **both** the `"as an ai"` marker and a refusal indicator are present. `"as an ai"` alone is not a refusal.

**Requirement 1.4:** The detection function must use `strings.ToLower` for case normalization and `strings.Contains` for substring matching. No regular expressions.

**Requirement 1.5:** The detection function must not modify the input content. It is a pure read-only check.

### 3.2 JSON Output Envelope (`cmd/run.go`)

**Requirement 2.1:** When the `--json` flag is set, the JSON output envelope must include a `"refused"` field of type boolean. The field must always be present (never omitted).

**Requirement 2.2:** The `"refused"` field value is the return value of `refusal.Detect(resp.Content)` where `resp.Content` is the final LLM response content.

**Requirement 2.3:** The field must appear in the envelope alongside existing fields. The complete `--json` envelope schema after this change:

```json
{
  "model": "<string>",
  "content": "<string>",
  "input_tokens": "<int>",
  "output_tokens": "<int>",
  "stop_reason": "<string>",
  "duration_ms": "<int>",
  "tool_calls": "<int>",
  "tool_call_details": "<array>",
  "refused": "<bool>"
}
```

**Requirement 2.4:** The `"refused"` field must work correctly for both single-shot responses (no tools) and conversation loop responses (with tools). In both cases, the detection runs against the final `resp.Content`.

### 3.3 Golden File Updates

**Requirement 3.1:** All golden JSON files in `cmd/testdata/golden/json/` must be updated to include the `"refused": false` field. This ensures golden file tests pass after the schema change.

**Requirement 3.2:** The golden file masking function `maskJSONOutput` in `cmd/golden_test.go` does not require changes — the `"refused"` field is deterministic (always `false` for normal mock responses) and does not need masking.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── run.go               # MODIFIED: add "refused" field to JSON envelope
│   ├── run_test.go           # MODIFIED: update TestRun_JSONOutput to verify "refused" field
│   ├── run_integration_test.go # MODIFIED: update JSON tests, add refusal detection test
│   ├── golden_test.go        # UNCHANGED
│   └── ...                   # all other cmd files UNCHANGED
├── internal/
│   ├── refusal/
│   │   ├── refusal.go        # NEW: Detect function with refusal patterns
│   │   └── refusal_test.go   # NEW: table-driven tests for Detect
│   ├── agent/                # UNCHANGED
│   ├── config/               # UNCHANGED
│   ├── memory/               # UNCHANGED
│   ├── provider/             # UNCHANGED
│   ├── resolve/              # UNCHANGED
│   ├── tool/                 # UNCHANGED
│   ├── toolname/             # UNCHANGED
│   └── xdg/                  # UNCHANGED
├── cmd/testdata/
│   └── golden/json/
│       ├── basic.json        # MODIFIED: add "refused": false
│       ├── with_skill.json   # MODIFIED: add "refused": false
│       ├── with_files.json   # MODIFIED: add "refused": false
│       ├── with_memory.json  # MODIFIED: add "refused": false
│       ├── with_subagents.json # MODIFIED: add "refused": false
│       └── with_tools.json   # MODIFIED: add "refused": false
├── go.mod                    # UNCHANGED (no new dependencies)
└── ...
```

---

## 5. Edge Cases

### 5.1 Content Patterns

| Scenario | `Detect()` returns | Rationale |
|----------|--------------------|-----------|
| `""` (empty string) | `false` | Nothing to match. |
| `"Hello, how can I help?"` | `false` | Normal response. |
| `"I cannot assist with that request."` | `true` | Direct refusal match: `"i cannot"`. |
| `"I can't help with that."` | `true` | Contraction match: `"i can't"`. |
| `"I'm unable to process this request."` | `true` | Match: `"i'm unable to"`. |
| `"I am unable to fulfill this."` | `true` | Match: `"i am unable to"`. |
| `"I'm not able to do that."` | `true` | Match: `"i'm not able to"`. |
| `"I am not able to do that."` | `true` | Match: `"i am not able to"`. |
| `"I must decline this request."` | `true` | Match: `"i must decline"`. |
| `"I don't have the ability to do that."` | `true` | Match: `"i don't have the ability"`. |
| `"I do not have the ability to help."` | `true` | Match: `"i do not have the ability"`. |
| `"I CANNOT do that."` | `true` | Case-insensitive match. |
| `"i cannot do that."` | `true` | Case-insensitive match. |
| `"I'm sorry, but I cannot assist with that."` | `true` | Contains `"i cannot"` after preamble. |
| `"I appreciate your question. However, I'm unable to help."` | `true` | Contains `"i'm unable to"` after preamble. |
| `"As an AI, I cannot provide medical advice."` | `true` | Compound match: `"as an ai"` + `"i cannot"`. |
| `"As an AI, I'm happy to help with that!"` | `false` | Contains `"as an ai"` but no refusal indicator. |
| `"As an AI language model, I must decline."` | `true` | Compound match: `"as an ai"` + `"i must decline"`. |
| `"The user said 'I cannot do this' in their message."` | `true` | False positive — acceptable per design decision 6. The content contains `"i cannot"` as a substring. |
| `"Here's how you can't go wrong with this approach."` | `true` | False positive — acceptable per design decision 6. The content contains `"i can't"` as a substring. This is a known tradeoff: false positives are less harmful than false negatives. |
| `"I can notify you when it's ready."` | `false` | `"i can"` does not match `"i cannot"` or `"i can't"`. |
| `"Unable to connect to database."` | `false` | Does not contain `"i'm unable to"` or `"i am unable to"`. The `"i"` prefix is required. |
| `"The AI cannot process this."` | `false` | Contains `"cannot"` but not `"i cannot"`. Not a first-person refusal. |
| Content with only whitespace: `"   \n\t  "` | `false` | No pattern matches. |
| Very long content (10KB) with refusal phrase at the end | `true` | Full content scan detects it. |
| Very long content (10KB) with no refusal phrases | `false` | Full content scan finds nothing. |

### 5.2 JSON Output

| Scenario | `"refused"` value | Notes |
|----------|-------------------|-------|
| Normal LLM response | `false` | Standard case. |
| LLM returns refusal text | `true` | Detection triggered. |
| LLM response with tools, final text is refusal | `true` | Detection runs on the final `resp.Content`. |
| LLM response with tools, final text is normal | `false` | Detection runs on the final `resp.Content`. |
| Single-shot (no tools), normal response | `false` | Same codepath as conversation loop final. |
| Single-shot (no tools), refusal response | `true` | Same codepath. |
| `--json` not set | N/A | Field not emitted. No `refusal.Detect` call needed. |

### 5.3 Integration with Existing Features

| Feature | Impact |
|---------|--------|
| `--verbose` | No change. Refusal detection is not logged to stderr. |
| `--dry-run` | No change. Dry-run does not call the LLM, so no refusal detection occurs. |
| Memory append | No change. Memory is appended regardless of refusal status. The refusal is a valid LLM response. |
| Exit code | No change. Exit code remains 0 for refusals. |
| `stop_reason` | No change. Original LLM stop_reason is preserved. |
| Sub-agent responses | No change. Sub-agent results are opaque to the parent. Refusal detection only runs on the top-level final response. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** No regular expressions in the detection function. Use `strings.ToLower` + `strings.Contains` only.

**Constraint 3:** Detection is read-only. The `Detect` function must not modify the input string or have any side effects.

**Constraint 4:** The `"refused"` field is only present in `--json` output. It must not appear in plain text stdout, stderr, verbose output, or dry-run output.

**Constraint 5:** The `"refused"` field must always be present in `--json` output (never conditionally omitted). The value is always a JSON boolean (`true` or `false`).

**Constraint 6:** Refusal detection does not affect exit codes. A detected refusal still exits 0.

**Constraint 7:** Refusal detection does not affect memory. If memory is enabled, the refusal response is still appended as a normal entry.

**Constraint 8:** The refusal pattern list is hard-coded. It is not user-configurable, not loaded from TOML, and not loaded from a skill file.

**Constraint 9:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Only stdlib string operations are used.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in previous milestones:

- **Package-level tests:** Tests live in the same package (e.g., `package refusal`, `package cmd`).
- **Standard library only:** Use `testing` package. No test frameworks.
- **Table-driven:** All `refusal.Detect` tests must be table-driven.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call `refusal.Detect` directly.
- **Run tests with:** `make test`
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Deletion test:** For every test, the assertion: "if I delete the code under test, does this test fail?" must be true.

### 7.2 `internal/refusal/refusal_test.go` Tests (New)

**Test: `TestDetect_EmptyString`** — Call `Detect("")`. Verify returns `false`.

**Test: `TestDetect_NormalContent`** — Table of normal (non-refusal) strings. Verify each returns `false`. Include at minimum:
- `"Hello, how can I help you today?"`
- `"Here is the solution to your problem."`
- `"The function returns true when the input is valid."`
- `"Unable to connect to the database."` (no first-person `"I"` prefix)
- `"The AI cannot process this."` (third-person, not `"I cannot"`)
- `"I can notify you when it's ready."` (does not match `"I cannot"` or `"I can't"`)
- `"As an AI, I'm happy to help with that!"` (`"as an ai"` without refusal indicator)

**Test: `TestDetect_SimpleRefusals`** — Table of simple refusal strings. Verify each returns `true`. Include at minimum one example for each of the 9 patterns from Requirement 1.2:
- `"I cannot assist with that request."`
- `"I can't help with that."`
- `"I'm unable to process this request."`
- `"I am unable to fulfill this."`
- `"I'm not able to do that."`
- `"I am not able to do that."`
- `"I must decline this request."`
- `"I don't have the ability to do that."`
- `"I do not have the ability to help."`

**Test: `TestDetect_CaseInsensitive`** — Table of refusals with varied casing. Verify each returns `true`. Include at minimum:
- `"I CANNOT do that."`
- `"i cannot do that."`
- `"I Can't Help With That."`
- `"I'M UNABLE TO ASSIST."`

**Test: `TestDetect_RefusalAfterPreamble`** — Table of refusals that appear after non-refusal preamble text. Verify each returns `true`. Include at minimum:
- `"I appreciate your question. However, I cannot help with that."`
- `"Thank you for asking. Unfortunately, I'm unable to process this request."`
- `"I understand your concern, but I must decline."`
- `"I'm sorry, but I can't assist with that request."`

**Test: `TestDetect_AsAnAI_Compound`** — Table of `"as an AI"` compound patterns. Verify each returns `true`. Include at minimum:
- `"As an AI, I cannot provide medical advice."`
- `"As an AI language model, I'm unable to help with that."`
- `"As an AI assistant, I must decline this request."`
- `"As an AI, I am not able to do this."`

**Test: `TestDetect_AsAnAI_NoRefusalIndicator`** — Verify that `"as an ai"` without a refusal indicator returns `false`. Include:
- `"As an AI, I'm happy to help you with this task."`
- `"As an AI language model, I have access to a wide range of information."`

**Test: `TestDetect_WhitespaceOnly`** — Call `Detect` with whitespace-only input (e.g., `"   \n\t  "`). Verify returns `false`.

**Test: `TestDetect_LongContent_RefusalAtEnd`** — Create a string of 10,000 characters of normal text, followed by `"I cannot help with that."`. Verify returns `true`.

**Test: `TestDetect_LongContent_NoRefusal`** — Create a string of 10,000 characters of normal text with no refusal patterns. Verify returns `false`.

### 7.3 `cmd/run_test.go` Test Modifications

**Test: `TestRun_JSONOutput` (Modified)** — Add verification that the `"refused"` field is present in the JSON output and that its value is `false` for the standard mock response `"Hello from mock"`.

### 7.4 `cmd/run_integration_test.go` Test Additions and Modifications

**Test: `TestIntegration_JSONOutput_Structure` (Modified)** — Add `"refused"` to the `requiredFields` slice. Verify the field is present and its value is `false` for the normal mock response `"json test output"`.

**Test: `TestIntegration_JSONOutput_RefusalDetected` (New)** — Set up a mock LLM server that returns a refusal response (e.g., `"I'm sorry, but I cannot assist with that request."`). Run the agent with `--json`. Parse the JSON output. Verify `"refused"` is `true`. Verify all other fields (`model`, `content`, `input_tokens`, `output_tokens`, `stop_reason`, `duration_ms`, `tool_calls`, `tool_call_details`) are still present and correct. Verify exit code is 0.

**Test: `TestIntegration_JSONOutput_RefusalDetected_WithToolCalls` (New)** — Set up a mock LLM server where: turn 1 returns a tool call, sub-agent responds, turn 2 returns a refusal response as the final content. Run with `--json`. Verify `"refused"` is `true`. Verify `"tool_calls"` is 1. Verify exit code is 0.

**Test: `TestIntegration_JSONOutput_NoRefusal_WithToolCalls` (New)** — Set up a mock LLM server where: turn 1 returns a tool call, sub-agent responds, turn 2 returns a normal (non-refusal) final response. Run with `--json`. Verify `"refused"` is `false`. Verify `"tool_calls"` is 1. Verify exit code is 0.

### 7.5 Golden File Updates

**Requirement:** After implementation, run tests with the `-update-golden` flag to regenerate all golden JSON files. Verify the regenerated files include `"refused": false`. The golden file test runner (`TestGolden`) will validate the schema automatically.

---

## 8. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. `internal/refusal/` package exists with a `Detect(content string) bool` function.
3. `Detect` correctly identifies all 9 simple refusal patterns (Requirement 1.2) and the compound `"as an ai"` pattern (Requirement 1.3), case-insensitively.
4. `Detect` returns `false` for normal content, empty strings, and whitespace-only input.
5. The `--json` output envelope includes a `"refused"` boolean field that is always present.
6. `"refused"` is `true` when refusal patterns are detected in the final response content.
7. `"refused"` is `false` when no refusal patterns are detected.
8. Plain text output (without `--json`) is unchanged — no mention of refusal anywhere.
9. Exit codes are unchanged — refusals still exit 0.
10. `stop_reason` is unchanged — the original LLM value is preserved.
11. No new external dependencies are introduced.
12. All golden JSON files are updated to include the `"refused": false` field.
13. Memory behavior is unchanged — refusals are still appended to memory.
