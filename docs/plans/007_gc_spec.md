# Specification: M7 - Garbage Collection

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-28
**Scope:** Pattern detection, memory trimming, `axe gc` command with `--dry-run` and `--all` flags

---

## 1. Purpose

Over time an agent's memory file grows without bound. The `axe gc` command analyzes memory entries for patterns, prints structured suggestions to stdout, and trims the file down to a bounded number of entries. This keeps memory files small and relevant.

GC is a maintenance command. It is separate from `axe run`. It calls the LLM once to analyze memory content, prints the analysis, and then overwrites the memory file with only the most recent entries.

This milestone does **not** include shared memory between agents (v2), streaming output (v2), or automatic/scheduled GC triggers.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Model selection:** The GC command uses the agent's configured `cfg.Model` by default. A `--model` flag on the `axe gc` command overrides this.
2. **Partial failure on `--all`:** When `axe gc --all` encounters an error on one agent, it continues processing the remaining agents. All errors are reported per-agent. The command exits non-zero if any agent failed.
3. **Trim target when `last_n` is 0:** If the agent's `memory.last_n` is `0` (meaning "load all"), the trim target falls back to `memory.max_entries`. If both `last_n` and `max_entries` are `0`, trimming is skipped entirely (only pattern detection and suggestions run).
4. **Structured suggestions:** The LLM is prompted to return specific sections: "Patterns Found", "Repeated Work", and "Recommendations". The output is still plain text but with expected headings.
5. **Auto-trim without confirmation:** `axe gc <agent>` (without `--dry-run`) prints suggestions to stdout, then trims the memory file automatically. No interactive confirmation prompt. This keeps the command scriptable and non-interactive.
6. **Memory-disabled agents:** If an agent has `memory.enabled = false`, `axe gc <agent>` prints a warning to stderr and exits with code 0. For `--all`, the agent is silently skipped.
7. **No new external dependencies.** Continue using stdlib only plus existing `spf13/cobra` and `BurntSushi/toml`.
8. **Single LLM call.** GC makes exactly one LLM call per agent for pattern detection. No conversation loop, no tool calls.
9. **Memory file rewrite.** Trimming replaces the memory file atomically: write to a temp file in the same directory, then rename. This prevents data loss if the process is interrupted.

---

## 3. Requirements

### 3.1 New Memory Function: `TrimEntries` (`internal/memory/`)

**Requirement 1.1:** Add a `TrimEntries` function to the `memory` package:

```go
func TrimEntries(path string, keepN int) (removed int, err error)
```

- Read the memory file at `path`.
- Parse entries by `## ` header lines (same boundary detection as `LoadEntries`).
- If `keepN` is `0`, return `(0, nil)` without modifying the file. Zero means "keep all" -- trimming is a no-op.
- If `keepN` is negative, return an error: `"keepN must be non-negative"`.
- If the total number of entries is less than or equal to `keepN`, return `(0, nil)` without modifying the file. There is nothing to trim.
- Otherwise, keep only the last `keepN` entries. Write the kept entries to a temporary file in the same directory as `path`, then rename the temporary file to `path`. This is an atomic replace.
- Return the number of entries removed and `nil` on success.
- If the file at `path` does not exist, return `(0, nil)`. Nothing to trim.
- If any I/O operation fails, return `(0, err)` with the error. The original file must remain unmodified on failure.

**Requirement 1.2:** The temporary file used during trimming must be created in the same directory as the memory file. This ensures the rename operation is atomic on the same filesystem.

**Requirement 1.3:** The kept entries must preserve their original format exactly, including all whitespace and blank lines. The resulting file must be byte-identical to what `LoadEntries(path, keepN)` would return for those entries.

### 3.2 CLI Command: `axe gc` (`cmd/gc.go`)

**Requirement 2.1:** Register a new top-level cobra command `gc` on `rootCmd`:

- **Use:** `gc`
- **Short:** `Analyze and trim agent memory`
- **Args:** Accepts zero or one positional argument. Zero arguments is valid only when `--all` is set. One argument is the agent name.

**Requirement 2.2:** Register the following flags on the `gc` command:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | `bool` | `false` | Analyze and print suggestions without trimming the memory file. |
| `--all` | `bool` | `false` | Run GC on all agents that have `memory.enabled = true`. |
| `--model` | `string` | `""` | Override the model used for pattern detection (provider/model-name format). |

**Requirement 2.3:** Argument validation:

- If `--all` is set and a positional argument is also provided, return an error: `"cannot specify both --all and an agent name"`. Exit code 1.
- If `--all` is not set and no positional argument is provided, return an error: `"agent name is required (or use --all)"`. Exit code 1.

### 3.3 Single-Agent GC Flow

When `axe gc <agent>` is invoked (without `--all`):

**Requirement 3.1:** Load the agent config via `agent.Load(agentName)`. If loading fails, return an error with exit code 2.

**Requirement 3.2:** If `cfg.Memory.Enabled` is `false`, print to stderr:

```
Warning: agent "<agentName>" does not have memory enabled. Skipping.
```

Return `nil` (exit code 0).

**Requirement 3.3:** If `--model` flag is set, use that value instead of `cfg.Model` for the LLM call. Parse the model string using the same `provider/model-name` format as `axe run`.

**Requirement 3.4:** Resolve the memory file path via `memory.FilePath(agentName, cfg.Memory.Path)`. If this fails, return an error with exit code 2.

**Requirement 3.5:** Load all entries from the memory file via `memory.LoadEntries(path, 0)`. If the file does not exist or is empty, print to stdout:

```
No memory entries for agent "<agentName>". Nothing to do.
```

Return `nil` (exit code 0).

**Requirement 3.6:** Count entries via `memory.CountEntries(path)`. Print to stdout:

```
Agent: <agentName>
Entries: <count>
```

**Requirement 3.7:** Send the memory content to the LLM for pattern detection. The request must be:

- **System prompt:** The pattern detection prompt defined in Requirement 4.1.
- **User message:** The full memory content (all entries, returned by `LoadEntries(path, 0)`).
- **Temperature:** `0.3` (deterministic analysis).
- **MaxTokens:** `4096`.
- **Tools:** None. No tool calls.

**Requirement 3.8:** Load the global config via `config.Load()` to resolve the API key and base URL for the provider. If the API key is missing (and the provider requires one), return an error with exit code 3.

**Requirement 3.9:** Print the LLM response (suggestions) to stdout under a heading:

```
--- Analysis ---
<LLM response content>
```

**Requirement 3.10:** If `--dry-run` is set, stop after printing the analysis. Do not trim.

Print to stdout after the analysis:

```
Dry run: no entries trimmed.
```

Return `nil` (exit code 0).

**Requirement 3.11:** If `--dry-run` is not set, determine the trim target:

1. If `cfg.Memory.LastN > 0`, the trim target is `cfg.Memory.LastN`.
2. Else if `cfg.Memory.MaxEntries > 0`, the trim target is `cfg.Memory.MaxEntries`.
3. Else (both are `0`), skip trimming. Print to stdout:

```
No trim target configured (last_n and max_entries are both 0). Skipping trim.
```

Return `nil` (exit code 0).

**Requirement 3.12:** If a trim target is determined, call `memory.TrimEntries(path, trimTarget)`. On success, print to stdout:

```
Trimmed: <removed> entries removed, <keepN> entries kept.
```

If `removed` is `0` (nothing to trim), print instead:

```
No trimming needed: <count> entries within limit (<keepN>).
```

**Requirement 3.13:** If `TrimEntries` returns an error, print the error to stderr and return an error with exit code 1.

### 3.4 Pattern Detection Prompt

**Requirement 4.1:** The pattern detection system prompt sent to the LLM must be exactly:

```
You are a memory analyst for an AI agent. You will receive a log of the agent's past tasks and results. Analyze the entries and provide a structured report.

Your report MUST contain exactly these three sections with these exact headings:

## Patterns Found
Identify recurring themes, common task types, or behavioral patterns across the entries. If no patterns exist, state "No clear patterns detected."

## Repeated Work
Identify any tasks that appear to be duplicated or that the agent has done multiple times with the same or similar inputs. If no repetition is found, state "No repeated work detected."

## Recommendations
Based on the patterns and repetitions found, suggest concrete actions the user could take to improve the agent's configuration, skill, or workflow. If no recommendations apply, state "No specific recommendations."

Be concise. Reference specific entries by their timestamps when relevant.
```

**Requirement 4.2:** The pattern detection prompt must not be stored in a TOML config file or made user-configurable. It is a hard-coded string within the `gc` command or an internal package.

### 3.5 All-Agents GC Flow (`--all`)

**Requirement 5.1:** When `--all` is set, call `agent.List()` to discover all agents.

**Requirement 5.2:** Filter the list to agents where `cfg.Memory.Enabled == true`. If no agents have memory enabled, print to stdout:

```
No agents with memory enabled.
```

Return `nil` (exit code 0).

**Requirement 5.3:** Process each qualifying agent sequentially (not in parallel). For each agent, run the single-agent GC flow (Requirements 3.1 through 3.13).

**Requirement 5.4:** Before processing each agent, print a separator to stdout:

```
=== GC: <agentName> ===
```

**Requirement 5.5:** If an agent's GC fails, print the error to stderr:

```
Error: gc failed for agent "<agentName>": <error message>
```

Continue processing remaining agents.

**Requirement 5.6:** After all agents are processed, if one or more agents failed, return an error with exit code 1 and the message:

```
gc completed with errors: <N> of <total> agents failed
```

Where `<N>` is the number of agents that failed and `<total>` is the number of agents that were processed (memory-enabled only).

**Requirement 5.7:** If all agents succeed, return `nil` (exit code 0).

### 3.6 Exit Codes

**Requirement 6.1:** The `gc` command uses the same exit code scheme as the rest of Axe:

| Code | Meaning |
|------|---------|
| 0 | Success (includes: nothing to do, dry-run, skip memory-disabled) |
| 1 | Agent error (trim failure, argument validation, partial failure on `--all`) |
| 2 | Config error (agent config not found, memory path resolution failure) |
| 3 | API error (LLM call failure, missing API key) |

### 3.7 Verbose and JSON Output

**Requirement 7.1:** The `gc` command does NOT support `--verbose` or `--json` flags. These are `axe run` features. GC output goes directly to stdout (suggestions, status messages) and stderr (warnings, errors).

---

## 4. Project Structure

After M7 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── gc.go              # NEW: axe gc command, single-agent and --all flows
│   ├── gc_test.go         # NEW: tests for gc command
│   ├── run.go             # UNCHANGED
│   ├── root.go            # UNCHANGED
│   └── ...                # all other cmd files UNCHANGED
├── internal/
│   ├── memory/
│   │   ├── memory.go      # MODIFIED: add TrimEntries function
│   │   └── memory_test.go # MODIFIED: add TrimEntries tests
│   ├── agent/             # UNCHANGED
│   ├── config/            # UNCHANGED
│   ├── provider/          # UNCHANGED
│   ├── resolve/           # UNCHANGED
│   ├── tool/              # UNCHANGED
│   └── xdg/              # UNCHANGED
├── go.mod                 # UNCHANGED (no new dependencies)
└── ...
```

---

## 5. Edge Cases

### 5.1 Memory File

| Scenario | Behavior |
|----------|----------|
| Memory file does not exist | `LoadEntries` returns `("", nil)`. GC prints "No memory entries" and exits 0. `TrimEntries` returns `(0, nil)`. |
| Memory file exists but is empty | Same as above. |
| Memory file has 3 entries, `last_n = 10` | `TrimEntries(path, 10)` returns `(0, nil)`. "No trimming needed" message. |
| Memory file has 50 entries, `last_n = 5` | `TrimEntries(path, 5)` removes 45 entries, keeps last 5. |
| Memory file has 50 entries, `last_n = 0`, `max_entries = 20` | Falls back to `max_entries`. `TrimEntries(path, 20)` removes 30, keeps last 20. |
| Memory file has 50 entries, `last_n = 0`, `max_entries = 0` | No trim target. Pattern detection runs, suggestions printed, trimming skipped. |
| Memory file is not readable | `LoadEntries` returns error. GC returns error with exit code 1. |
| Memory file's parent directory is not writable | `TrimEntries` fails because temp file cannot be created. Returns error. Original file unmodified. |
| Memory file is on a different filesystem than temp file | Not possible. Temp file is created in the same directory (Requirement 1.2), ensuring same filesystem for atomic rename. |
| Memory file has content before the first `## ` header | `TrimEntries` discards any pre-header content. Only `## `-delimited entries are preserved. This matches how `LoadEntries(path, N)` works when `N > 0`. |
| Memory file contains `## ` headers that are not timestamps | Treated as entry boundaries. GC does not validate timestamp format. Same behavior as existing `LoadEntries`. |
| Concurrent `axe run` appends during GC | The atomic rename in `TrimEntries` replaces the file. An entry appended between the read and the rename is lost. This is an accepted race condition. Users should not run `axe run` and `axe gc` on the same agent simultaneously. |

### 5.2 Agent Config

| Scenario | Behavior |
|----------|----------|
| Agent config does not exist | `agent.Load` returns error. GC exits with code 2. |
| Agent has `memory.enabled = false` | Warning printed to stderr, exit 0. |
| Agent has `memory.enabled = true`, no memory file | "No memory entries" message, exit 0. |
| Agent's model is invalid format | `parseModel` returns error. GC exits with code 1. |
| `--model` override is invalid format | Same. `parseModel` returns error, exit code 1. |
| Agent has `memory.path` set to custom path | Custom path is used for all memory operations. |
| Agent has no `[memory]` section | `Memory.Enabled` defaults to `false`. Warning and skip. |

### 5.3 LLM Call

| Scenario | Behavior |
|----------|----------|
| LLM returns empty response | Empty analysis section printed. Trimming still proceeds based on config. |
| LLM call times out | Return error with exit code 3. No trimming performed. |
| LLM returns error (rate limit, auth, server) | Return error with exit code 3. No trimming performed. |
| API key missing for provider | Return error with exit code 3 before making the call. |
| Provider is `ollama` (no API key needed) | Proceed without API key check, same as `axe run`. |

### 5.4 `--all` Flag

| Scenario | Behavior |
|----------|----------|
| No agents directory | `agent.List` returns empty slice. "No agents with memory enabled." message. Exit 0. |
| All agents have `memory.enabled = false` | Same as above. |
| 5 agents, 2 with memory enabled | Only the 2 memory-enabled agents are processed. |
| 5 agents with memory, 1 fails | 4 succeed, 1 error reported. Exit code 1 with summary. |
| 5 agents with memory, all fail | All 5 errors reported. Exit code 1. |
| `--all` combined with `--dry-run` | Each agent runs in dry-run mode (analysis only, no trim). |
| `--all` combined with `--model` | The `--model` override applies to all agents. |

### 5.5 Trim Target Resolution

| `last_n` | `max_entries` | Trim target | Behavior |
|-----------|---------------|-------------|----------|
| `5` | `100` | `5` | Keep last 5 entries. |
| `5` | `0` | `5` | Keep last 5 entries. |
| `0` | `100` | `100` | Fallback to max_entries. Keep last 100. |
| `0` | `0` | None | No trimming. Suggestions only. |
| `10` | `5` | `10` | `last_n` takes precedence. Keep last 10. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** No interactive prompts. The `gc` command is fully non-interactive and scriptable. No confirmation dialogs.

**Constraint 3:** No conversation loop. The LLM call for pattern detection is a single request/response. No tool calls are injected.

**Constraint 4:** No parallel GC. When `--all` is set, agents are processed sequentially. This avoids rate-limiting issues and keeps output readable.

**Constraint 5:** The `gc` command does not modify any agent TOML config files. It only reads agent configs and modifies memory data files.

**Constraint 6:** Memory failures during GC are hard errors (unlike `axe run` where memory is best-effort). If the memory file cannot be read or trimmed, GC reports the error and exits non-zero.

**Constraint 7:** The pattern detection prompt is hard-coded. It is not user-configurable, not loaded from a skill file, and not stored in TOML.

**Constraint 8:** The `TrimEntries` function performs an atomic file replace (write temp + rename). It must not truncate or partially write the original file.

**Constraint 9:** GC does not append a memory entry. The LLM call made by GC is a maintenance operation, not an agent run. No entry is written to the memory file for the GC analysis itself.

**Constraint 10:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. The atomic rename (`os.Rename`) works across all platforms when source and destination are in the same directory.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M1-M6:

- **Package-level tests:** Tests live in the same package (e.g., `package memory`, `package cmd`).
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real I/O.
- **Run tests with:** `make test`
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.

### 7.2 `internal/memory/memory_test.go` Tests (Modified)

**Test: `TestTrimEntries_FileDoesNotExist`** -- Call `TrimEntries` with a non-existent path and `keepN = 5`. Verify returns `(0, nil)`.

**Test: `TestTrimEntries_EmptyFile`** -- Create an empty file. Call `TrimEntries(path, 5)`. Verify returns `(0, nil)`. Verify file is still empty.

**Test: `TestTrimEntries_KeepNZero`** -- Create a file with 5 entries. Call `TrimEntries(path, 0)`. Verify returns `(0, nil)`. Verify file is unchanged (all entries preserved).

**Test: `TestTrimEntries_KeepNNegative`** -- Call `TrimEntries(path, -1)`. Verify returns error containing `"keepN must be non-negative"`.

**Test: `TestTrimEntries_EntriesWithinLimit`** -- Create a file with 3 entries. Call `TrimEntries(path, 5)`. Verify returns `(0, nil)`. Verify file is unchanged.

**Test: `TestTrimEntries_EntriesEqualToLimit`** -- Create a file with 5 entries. Call `TrimEntries(path, 5)`. Verify returns `(0, nil)`. Verify file is unchanged.

**Test: `TestTrimEntries_TrimsOldEntries`** -- Create a file with 10 entries (each with a unique timestamp). Call `TrimEntries(path, 3)`. Verify returns `(7, nil)`. Read the file and verify it contains exactly the last 3 entries. Verify the content is byte-identical to what `LoadEntries(path, 3)` would have returned before trimming.

**Test: `TestTrimEntries_PreservesEntryFormat`** -- Create a file with entries containing multi-line results. Call `TrimEntries`. Verify all whitespace, blank lines, and formatting in kept entries is preserved exactly.

**Test: `TestTrimEntries_DiscardsPreHeaderContent`** -- Create a file with text before the first `## ` header, followed by 5 entries. Call `TrimEntries(path, 3)`. Verify the pre-header text is not in the resulting file. Verify only the last 3 entries remain.

**Test: `TestTrimEntries_SingleEntryKeepOne`** -- Create a file with 1 entry. Call `TrimEntries(path, 1)`. Verify returns `(0, nil)`. File unchanged.

**Test: `TestTrimEntries_OriginalUnmodifiedOnWriteError`** -- Create a file with 5 entries in a directory. Make the directory read-only (so temp file creation fails). Call `TrimEntries`. Verify the original file is unmodified. Restore directory permissions in cleanup.

### 7.3 `cmd/gc_test.go` Tests (New)

**Test: `TestGC_NoArgsNoAll`** -- Run `axe gc` with no arguments and no `--all` flag. Verify error message: `"agent name is required (or use --all)"`. Verify exit code 1.

**Test: `TestGC_AllWithAgentName`** -- Run `axe gc myagent --all`. Verify error message: `"cannot specify both --all and an agent name"`. Verify exit code 1.

**Test: `TestGC_AgentNotFound`** -- Run `axe gc nonexistent`. Verify exit code 2.

**Test: `TestGC_MemoryDisabled`** -- Create agent with `memory.enabled = false`. Run `axe gc <agent>`. Verify stderr contains warning about memory not enabled. Verify exit code 0.

**Test: `TestGC_NoMemoryFile`** -- Create agent with `memory.enabled = true`. Do not create a memory file. Run `axe gc <agent>`. Verify stdout contains "No memory entries". Verify exit code 0. No LLM call is made.

**Test: `TestGC_EmptyMemoryFile`** -- Create agent with `memory.enabled = true`. Create an empty memory file. Run `axe gc <agent>`. Verify stdout contains "No memory entries". Verify exit code 0.

**Test: `TestGC_AnalyzeAndTrim`** -- Create agent with `memory.enabled = true`, `last_n = 3`. Populate memory file with 10 entries. Start mock LLM server that returns a canned analysis response. Run `axe gc <agent>`. Verify stdout contains "--- Analysis ---" followed by the LLM response. Verify stdout contains "Trimmed: 7 entries removed, 3 entries kept." Verify the memory file now contains exactly 3 entries.

**Test: `TestGC_DryRun`** -- Create agent with `memory.enabled = true`, `last_n = 3`. Populate memory file with 10 entries. Start mock LLM server. Run `axe gc <agent> --dry-run`. Verify stdout contains the analysis. Verify stdout contains "Dry run: no entries trimmed." Verify the memory file still contains 10 entries.

**Test: `TestGC_NoTrimTarget`** -- Create agent with `memory.enabled = true`, `last_n = 0`, `max_entries = 0`. Populate memory file with 10 entries. Start mock LLM server. Run `axe gc <agent>`. Verify stdout contains the analysis. Verify stdout contains "No trim target configured". Verify memory file is unchanged.

**Test: `TestGC_FallbackToMaxEntries`** -- Create agent with `memory.enabled = true`, `last_n = 0`, `max_entries = 5`. Populate memory file with 10 entries. Start mock LLM server. Run `axe gc <agent>`. Verify stdout contains "Trimmed: 5 entries removed, 5 entries kept." Verify memory file has 5 entries.

**Test: `TestGC_ModelOverride`** -- Create agent with model `anthropic/claude-3`. Start mock LLM server. Run `axe gc <agent> --model ollama/llama3`. Verify the LLM request was sent with model `llama3` (not `claude-3`).

**Test: `TestGC_LLMError`** -- Create agent with `memory.enabled = true`. Populate memory file. Start mock LLM server that returns HTTP 500. Run `axe gc <agent>`. Verify exit code 3. Verify memory file is unchanged.

**Test: `TestGC_NothingToTrim`** -- Create agent with `memory.enabled = true`, `last_n = 10`. Populate memory file with 3 entries. Start mock LLM server. Run `axe gc <agent>`. Verify stdout contains "No trimming needed: 3 entries within limit (10)."

**Test: `TestGC_AllFlag_NoMemoryAgents`** -- Create agents directory with 2 agents, both `memory.enabled = false`. Run `axe gc --all`. Verify stdout contains "No agents with memory enabled." Verify exit code 0.

**Test: `TestGC_AllFlag_MultipleAgents`** -- Create 3 agents: agent-a (memory enabled, 10 entries), agent-b (memory disabled), agent-c (memory enabled, 5 entries). Start mock LLM server. Run `axe gc --all`. Verify agent-a and agent-c are processed (separators printed). Verify agent-b is not processed. Verify exit code 0.

**Test: `TestGC_AllFlag_PartialFailure`** -- Create 2 agents with memory enabled. Set up agent-a with a valid memory file and agent-b with an unreadable memory file. Start mock LLM server. Run `axe gc --all`. Verify stderr contains error for agent-b. Verify agent-a was processed successfully. Verify exit code 1 with summary message.

**Test: `TestGC_AllFlag_WithDryRun`** -- Create 2 agents with memory enabled. Populate their memory files. Start mock LLM server. Run `axe gc --all --dry-run`. Verify both agents show analysis but no trimming occurs. Verify both memory files are unchanged.

**Test: `TestGC_PatternDetectionPrompt`** -- Create agent with memory. Start mock LLM server that captures the request body. Run `axe gc <agent>`. Verify the system prompt in the request matches the exact pattern detection prompt from Requirement 4.1. Verify the user message contains the full memory content. Verify temperature is `0.3` and max_tokens is `4096`.

---

## 8. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. `internal/memory/` has a `TrimEntries` function that atomically rewrites the memory file with only the last N entries.
3. `axe gc <agent>` loads the agent's memory, sends it to the LLM for analysis, prints structured suggestions to stdout, and trims the file to the configured limit.
4. `axe gc <agent> --dry-run` prints analysis without trimming.
5. `axe gc --all` processes all memory-enabled agents sequentially, continues on error, and reports a summary.
6. The pattern detection prompt produces output with three sections: "Patterns Found", "Repeated Work", and "Recommendations".
7. Trim target resolves as: `last_n` if positive, else `max_entries` if positive, else no trimming.
8. Agents with `memory.enabled = false` are warned and skipped.
9. No new external dependencies are introduced.
10. All exit codes match the documented scheme (0/1/2/3).
11. No interactive prompts or confirmation dialogs.
12. GC does not append any memory entries for its own operations.
