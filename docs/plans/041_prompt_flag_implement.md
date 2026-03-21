```markdown
# 041 — Inline Prompt Flag (`-p`) Implementation Guide

**Spec:** `docs/plans/041_prompt_flag_spec.md`

---

## Section 1: Context Summary

Issue #43 asks for a way to pass a prompt to `axe run` without piping stdin. The solution is a `-p` / `--prompt` string flag on `axe run` that, when provided with a non-empty, non-whitespace value, is used as the user message sent to the LLM — taking precedence over piped stdin and the default fallback message. Empty or whitespace-only values are treated as absent. When `-p` wins over piped stdin, the override is silent (no stderr warning). No TUI, no interactive mode, no new subcommand, no changes to `resolve.Stdin()`, and no changes to the JSON output envelope. README documentation is a required deliverable of this milestone.

---

## Section 2: Implementation Checklist

### Task 1 — Register the `-p` / `--prompt` flag

- [x] `cmd/run.go`: `init()` — Add `runCmd.Flags().StringP("prompt", "p", "", "Inline prompt to use as the user message (takes precedence over stdin; empty or whitespace is treated as absent)")` after the existing flag registrations.

---

### Task 2 — Update the `runCmd` long description

- [x] `cmd/run.go`: `runCmd` (`Long` field) — Extend the long description to mention that the user message can be provided via `-p`, piped stdin, or defaults to a built-in fallback, in that precedence order.

---

### Task 3 — Implement user message precedence in `runAgent`

- [x] `cmd/run.go`: `runAgent()` — After the existing stdin read block (Step 9, ~line 183), read the `--prompt` flag value with `cmd.Flags().GetString("prompt")`. Replace the current two-branch user message resolution (lines 274–278) with a three-branch resolution:
  1. If `strings.TrimSpace(promptFlag) != ""` → use `promptFlag` as `userMessage`.
  2. Else if `strings.TrimSpace(stdinContent) != ""` → use `stdinContent` as `userMessage`.
  3. Else → use `defaultUserMessage`.

  No warning or error is emitted when `-p` overrides stdin.

---

### Task 4 — Pass prompt flag value through dry-run

- [x] `cmd/run.go`: `runAgent()` — When `dryRun` is true (~line 229), the resolved `userMessage` (after applying the three-branch precedence from Task 3) must be passed to `printDryRun` so the dry-run output reflects the actual user message that would be sent.

- [x] `cmd/run.go`: `printDryRun()` (line 593) — Add a `userMessage string` parameter. Replace the current `--- Stdin ---` section (lines 626–632) with a `--- User Message ---` section that prints `userMessage` (or `"(default)"` if it equals `defaultUserMessage` and no stdin or prompt was provided). Update the call site in `runAgent()` to pass the resolved `userMessage`.

  > **Note:** The `--- Stdin ---` label is replaced with `--- User Message ---` because the dry-run output should show what the LLM will actually receive, not the raw source. This is a display-only change; no behavioral change.

---

### Task 5 — Update `resetRunCmd` in tests

- [x] `cmd/run_test.go`: `resetRunCmd()` — Add `_ = runCmd.Flags().Set("prompt", "")` alongside the existing flag resets to ensure the `-p` flag is cleared between tests.

---

### Task 6 — Write tests for prompt flag precedence

- [x] `cmd/run_test.go` — Add a table-driven test function `TestRun_PromptFlag` covering all six cases from R9. Each case must use `startMockAnthropicServer`, `setupRunTestAgent`, and `resetRunCmd`. The test must verify the user message sent to the mock server by inspecting the request body (the `messages[0].content` field in the Anthropic API request JSON). Cases:

  | Test name | `-p` value | stdin | Expected user message |
  |---|---|---|---|
  | `prompt_flag_used` | `"hello from flag"` | (none) | `"hello from flag"` |
  | `prompt_flag_wins_over_stdin` | `"flag wins"` | `"stdin content"` | `"flag wins"` |
  | `no_flag_stdin_used` | (absent) | `"piped content"` | `"piped content"` |
  | `no_flag_no_stdin_default` | (absent) | (none) | `"Execute the task described in your instructions."` |
  | `empty_flag_falls_through_to_stdin` | `""` | `"piped content"` | `"piped content"` |
  | `whitespace_flag_falls_through_to_stdin` | `"   "` | `"piped content"` | `"piped content"` |

---

### Task 7 — Write test for dry-run with `-p`

- [x] `cmd/run_test.go` — Add a test `TestRun_DryRun_PromptFlag` that runs `axe run <agent> --dry-run -p "my prompt"` and asserts:
  - The output contains `--- User Message ---`.
  - The output contains `"my prompt"`.
  - The output does NOT contain `--- Stdin ---` (label has changed).
  - No LLM call is made (no mock server needed; dry-run exits before calling the provider).

---

### Task 8 — Update README

- [x] `README.md`: **Run Flags** table (line ~265) — Add a row for the new flag:
  ```
  | `-p` / `--prompt <string>` | (none) | Inline prompt used as the user message; takes precedence over stdin |
  ```

- [x] `README.md`: **Run Flags** section — Add a prose paragraph after the flags table explaining:
  - The user message precedence order: `-p` > piped stdin > built-in default.
  - That `-p` silently overrides piped stdin when both are present.
  - That an empty or whitespace-only `-p` value is treated as absent (falls through to stdin, then default).
  - At least one usage example:
    ```bash
    axe run my-agent -p "Summarize the README"
    ```
```