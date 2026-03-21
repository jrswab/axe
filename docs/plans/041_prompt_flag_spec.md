# 041 — Inline Prompt Flag (`-p`) for `axe run`

**GitHub Issue:** [#43](https://github.com/jrswab/axe/issues/43)
**Milestone:** v1.6.0

---

## Section 1: Context & Constraints

### Milestone Entry

Issue #43 requests that `axe run` support user-provided prompts without requiring piped stdin. The solution is a `-p` / `--prompt` flag that accepts an inline string as the user message. This keeps axe fully non-interactive — no TUI, no REPL, no terminal UI library.

### Research Findings

**Current user message resolution (cmd/run.go, ~line 274):**

The user message sent to the LLM is determined by two sources today:

1. **Piped stdin** — If stdin is not a terminal (`resolve.Stdin()` checks `ModeCharDevice`), its content is read and used as the user message.
2. **Default message** — If stdin is a terminal (or empty), the constant `defaultUserMessage` (`"Execute the task described in your instructions."`) is used.

There is no way to provide a user message inline from the command line without piping.

**Flag registration pattern (cmd/run.go, `init()`):**

All `axe run` flags are registered in the `init()` function using cobra's `Flags()` API. Short flags use `StringP`/`BoolP` (e.g., `-v` for `--verbose`). The `-p` short flag is currently unused.

**Test pattern (cmd/run_test.go):**

Tests use `resetRunCmd()` to reset all flags between tests. Any new flag must be added to this function. Tests set stdin via `rootCmd.SetIn()` and capture output via `rootCmd.SetOut()` / `rootCmd.SetErr()`. A mock Anthropic server (`startMockAnthropicServer`) is used for integration-style tests.

**Stdin handling (internal/resolve/resolve.go, `Stdin()`):**

`resolve.Stdin()` returns an empty string when stdin is a terminal. In tests, `cmd.InOrStdin()` is checked against `os.Stdin` to allow test overrides. The `-p` flag must integrate with this existing flow without changing `resolve.Stdin()` itself.

**JSON output envelope (cmd/run.go, ~line 541):**

The `--json` flag wraps output in a metadata envelope. No changes to the JSON schema are required for this feature.

### Decisions Already Made

- **`-p` / `--prompt` flag, not a new subcommand.** A separate `axe chat` command was considered and rejected. The feature is a flag on `axe run`.
- **Not interactive.** No TUI library, no readline, no multi-turn user input. Axe remains a non-interactive CLI.
- **Silent stdin ignore when `-p` is provided.** No warning to stderr. This must be documented in the README.
- **Empty `-p` treated as absent.** `-p ""` or `-p "   "` falls through to stdin, then default. This must be documented in the README.
- **No JSON schema changes.** The `--json` output envelope is unchanged. No `prompt_source` field.
- **Scope: `axe run` only.** The `-p` flag is not added to `axe gc` or any other subcommand.

### Approaches Ruled Out

- **Full TUI with bubbletea/charmbracelet** — Contradicts axe's non-interactive design philosophy.
- **`axe chat <agent>` subcommand** — Overengineered for the actual need. A flag is sufficient.
- **Auto-detect interactive mode from terminal** — Axe already detects terminals to *ignore* them. Adding auto-interactive behavior would break the Unix citizen contract.
- **Warning on stderr when `-p` overrides stdin** — Rejected in favor of silent behavior to keep output clean and pipeable.
- **Error on empty `-p`** — Rejected. Treating empty as absent is more forgiving and consistent with how other flags behave.

### Constraints

- **Precedence order is strict:** `-p` flag > piped stdin > default message. No exceptions.
- **`resolve.Stdin()` must not be modified.** The `-p` logic lives entirely in `cmd/run.go`.
- **The `-p` short flag must not collide** with any existing short flag on `runCmd`. Current short flags: `-v` (verbose). `-p` is available.
- **README documentation is required** as part of this feature, not as a follow-up.

---

## Section 2: Requirements

### R1: Flag Definition

`axe run` must accept a `-p` / `--prompt` flag of type string. The flag's help text must describe its purpose and its precedence over stdin.

### R2: User Message Precedence

The user message sent to the LLM must be resolved in this order:

1. **`-p` flag value** — If the flag is provided and its trimmed value is non-empty, use it as the user message.
2. **Piped stdin** — If `-p` is absent or empty/whitespace-only, and stdin contains piped data, use stdin as the user message.
3. **Default message** — If neither `-p` nor stdin provides content, use the existing default message (`"Execute the task described in your instructions."`).

No other resolution paths exist.

### R3: Empty/Whitespace `-p` Handling

If `-p` is provided with an empty string or a string containing only whitespace, it must be treated as if the flag was not provided. The resolution falls through to stdin, then to the default message.

### R4: Silent Stdin Override

When `-p` is provided with a non-empty value and stdin also contains piped data, the piped stdin must be silently ignored. No warning, no error, no output to stderr.

### R5: No Behavioral Changes to Existing Flows

When `-p` is not provided, all existing behavior must be preserved exactly:
- Piped stdin is read and used as the user message.
- Terminal stdin is ignored (returns empty via `resolve.Stdin()`).
- Default message is used when no input is available.

### R6: Dry-Run Compatibility

When `--dry-run` is used with `-p`, the resolved context output must reflect the prompt flag value as the user message source.

### R7: JSON Output Compatibility

The `--json` output envelope must remain unchanged. No new fields are added. The `content` field continues to contain the LLM's response regardless of how the user message was provided.

### R8: README Documentation

The README must be updated to document:
- The `-p` / `--prompt` flag and its purpose.
- The precedence order: `-p` > stdin > default.
- That `-p` silently overrides piped stdin when both are present.
- That an empty or whitespace-only `-p` value is treated as absent.
- At least one usage example (e.g., `axe run my-agent -p "Summarize the README"`).

### R9: Test Coverage

Tests must cover all of the following cases:
- `-p` with a non-empty value → prompt flag used as user message.
- `-p` with a non-empty value AND piped stdin → prompt flag wins, stdin ignored.
- No `-p`, piped stdin → stdin used (existing behavior).
- No `-p`, no piped stdin → default message used (existing behavior).
- `-p ""` (empty string) → treated as absent, falls through.
- `-p "   "` (whitespace only) → treated as absent, falls through.

### Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `-p "hello"` with no stdin | User message = `"hello"` |
| `-p "hello"` with stdin `"world"` | User message = `"hello"`, stdin silently ignored |
| `-p ""` with stdin `"world"` | User message = `"world"` (empty -p treated as absent) |
| `-p "   "` with stdin `"world"` | User message = `"world"` (whitespace -p treated as absent) |
| `-p ""` with no stdin | User message = default message |
| `-p "hello"` with `--dry-run` | Dry-run output shows `"hello"` as user message |
| `-p "hello"` with `--json` | JSON envelope unchanged; LLM response in `content` field |
| `-p` flag not provided at all | Existing behavior unchanged |
