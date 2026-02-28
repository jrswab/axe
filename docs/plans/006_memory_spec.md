# Specification: M6 - Memory

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-28
**Scope:** Append-only run log, context loading, memory config fields, XDG data directory support

---

## 1. Purpose

Give agents awareness of their past runs. After each invocation, Axe appends a timestamped entry to a per-agent markdown file. On the next run, Axe loads the most recent entries into the system prompt so the agent can see what it did before — spot patterns, avoid repeating work, and build context over time.

Memory is a run log (what happened), not instructions (what to do). The file is plain markdown, append-only from Axe's perspective, and readable/editable by humans.

This milestone does **not** include garbage collection (M7) or shared memory between parent and sub-agents (v2).

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Storage format:** One plain markdown file per agent. No database, no SQLite, no embeddings.
2. **Storage location:** `$XDG_DATA_HOME/axe/memory/<agent-name>.md` by default. The `memory.path` config field overrides this with an absolute file path.
3. **Append-only:** Axe only appends entries. It never deletes or modifies existing entries. Users may manually edit or delete the file.
4. **Entry format:** Each entry is a level-2 markdown heading with a UTC RFC 3339 timestamp, followed by `**Task:**` and `**Result:**` lines. Entries are separated by a blank line.
5. **Result truncation:** The result stored in memory is truncated to 1000 characters maximum. If truncated, `...` is appended.
6. **Task source:** The task stored in memory is the user message sent to the LLM (stdin content if piped, otherwise the default user message). It is **not** the system prompt or skill content.
7. **Sub-agent isolation:** Sub-agents have their own memory files. A parent agent's memory does not include sub-agent call results. Sub-agents log to their own memory files if their config has `memory.enabled = true`.
8. **`last_n` semantics:** `0` means load all entries. Any positive integer loads that many most-recent entries. The scaffold template shows `last_n = 10` as the recommended value.
9. **`max_entries` warning:** When the number of entries in the memory file meets or exceeds `max_entries`, Axe prints a warning to stderr on every run (not gated by `--verbose`). The warning suggests running `axe gc <agent>`.
10. **`max_entries` zero value:** `0` means no limit — no warning is ever printed.
11. **Memory disabled by default:** The `memory.enabled` field defaults to `false`. Agents without an explicit `[memory]` section have no memory behavior.
12. **No memory on error:** If the LLM call fails (API error, timeout, etc.), no memory entry is appended.
13. **No memory on dry-run:** `--dry-run` does not append a memory entry but does display what memory would be loaded.
14. **Dependencies:** No new external dependencies. Continue using stdlib only.

---

## 3. Requirements

### 3.1 XDG Data Directory (`internal/xdg/`)

**Requirement 1.1:** Add a `GetDataDir` function to the `xdg` package:

```go
func GetDataDir() (string, error)
```

This function returns the Axe data directory path following the XDG Base Directory specification. It resolves to `$XDG_DATA_HOME/axe` if the `XDG_DATA_HOME` environment variable is set and non-empty.

**Requirement 1.2:** If `XDG_DATA_HOME` is not set or empty, the function must fall back to `$HOME/.local/share/axe` on all platforms. Use `os.UserHomeDir()` to resolve `$HOME`. If `os.UserHomeDir()` returns an error, `GetDataDir` must return that error wrapped with the message: `"unable to determine data directory: <original error>"`.

**Requirement 1.3:** `GetDataDir` must NOT create the directory. It returns the path only. Directory creation is the caller's responsibility.

**Requirement 1.4:** The existing `GetConfigDir` function must remain unchanged.

### 3.2 Memory Config (`internal/agent/`)

**Requirement 2.1:** Extend the `MemoryConfig` struct with two new fields:

| Go Field | TOML Key | Go Type | Zero Value | Description |
|----------|----------|---------|------------|-------------|
| `Enabled` | `enabled` | `bool` | `false` | Enable persistent memory. Existing field, unchanged. |
| `Path` | `path` | `string` | `""` | Custom memory file path (absolute). Existing field, unchanged. If empty, Axe uses the default path. |
| `LastN` | `last_n` | `int` | `0` | Number of most-recent entries to load into context. `0` = load all entries. |
| `MaxEntries` | `max_entries` | `int` | `0` | Warn when entry count meets or exceeds this value. `0` = no limit, no warning. |

**Requirement 2.2:** Validation: If `LastN` is negative, `Validate` must return an error: `"memory.last_n must be non-negative"`.

**Requirement 2.3:** Validation: If `MaxEntries` is negative, `Validate` must return an error: `"memory.max_entries must be non-negative"`.

**Requirement 2.4:** The `Scaffold` function must update the commented `[memory]` section to include all four fields:

```toml
# [memory]
# enabled = false
# path = ""
# last_n = 10
# max_entries = 100
```

**Requirement 2.5:** The `agents show` command must display `Memory LastN` and `Memory MaxEntries` alongside the existing `Memory Enabled` and `Memory Path` fields when any memory field has a non-zero value.

### 3.3 Memory Package (`internal/memory/`)

**Requirement 3.1:** Create a new package `internal/memory/` containing all memory read/write logic.

**Requirement 3.2:** Define a `FilePath` function:

```go
func FilePath(agentName, customPath string) (string, error)
```

- If `customPath` is non-empty, return `customPath` as-is.
- If `customPath` is empty, call `xdg.GetDataDir()` and return `<dataDir>/memory/<agentName>.md`.
- If `xdg.GetDataDir()` returns an error, propagate the error.
- The function must NOT create any directories or files.

**Requirement 3.3:** Define an `AppendEntry` function:

```go
func AppendEntry(path, task, result string) error
```

- Create the parent directory of `path` using `os.MkdirAll` with permissions `0755` if it does not exist.
- Open the file in append mode: `os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)`.
- Write one entry in this exact format:

```
## <RFC3339 UTC timestamp>
**Task:** <task>
**Result:** <result>

```

The entry ends with exactly one trailing blank line (two newlines after the result line).

- The timestamp must be the current time in UTC, formatted as RFC 3339 (e.g., `2026-02-28T15:04:05Z`).
- If `task` is empty, write `**Task:** (none)`.
- If `result` is empty, write `**Result:** (none)`.
- If `result` exceeds 1000 characters, truncate it to 1000 characters and append `...` (total stored length: 1003 characters).
- If `task` contains newlines, replace all newlines with spaces before writing. The `**Task:**` line must be a single line.
- If `result` contains newlines, they are preserved as-is in the written output. The result may span multiple lines.
- Return any I/O error encountered during directory creation, file opening, or file writing.

**Requirement 3.4:** The `AppendEntry` function must use a package-level variable for the time source to allow deterministic testing:

```go
var Now func() time.Time = time.Now
```

Tests override `Now` to return a fixed time. The `AppendEntry` function calls `Now().UTC()` to get the timestamp.

**Requirement 3.5:** Define a `LoadEntries` function:

```go
func LoadEntries(path string, lastN int) (string, error)
```

- If the file at `path` does not exist, return `("", nil)`. This is not an error — it means the agent has not run before.
- Read the entire file content.
- Parse the file into individual entries. An entry starts with a line matching the pattern `## ` (the characters `#`, `#`, space) at the beginning of a line. The entry includes all content from that line until the next entry header or end of file.
- If `lastN` is `0`, return the entire file content as-is (all entries).
- If `lastN` is greater than `0`, return only the last `lastN` entries. Each returned entry retains its original format including its `## ` header and trailing whitespace.
- If `lastN` exceeds the total number of entries, return all entries.
- Entries are returned as a single concatenated string, preserving their original formatting.
- If the file exists but is empty, return `("", nil)`.
- If reading the file fails for any reason other than non-existence, return the error.

**Requirement 3.6:** Define a `CountEntries` function:

```go
func CountEntries(path string) (int, error)
```

- If the file at `path` does not exist, return `(0, nil)`.
- Read the file and count the number of lines that start with `## ` (entry headers).
- Return the count.
- If reading the file fails for any reason other than non-existence, return the error.

### 3.4 Memory Integration — Top-Level Run (`cmd/run.go`)

**Requirement 4.1:** After building the system prompt (current step 10, line 141) and before the dry-run check, if `cfg.Memory.Enabled` is `true`:

1. Resolve the memory file path via `memory.FilePath(agentName, cfg.Memory.Path)`.
2. Load entries via `memory.LoadEntries(path, cfg.Memory.LastN)`.
3. If loaded content is non-empty, append a memory section to the system prompt string:

```
\n\n---\n\n## Memory\n\n<loaded entries>
```

4. Count entries via `memory.CountEntries(path)`.
5. If `cfg.Memory.MaxEntries > 0` and the count is `>= cfg.Memory.MaxEntries`, print a warning to stderr:

```
Warning: agent "<agentName>" memory has <count> entries (max_entries: <max>). Run 'axe gc <agentName>' to trim.
```

This warning is printed regardless of the `--verbose` flag.

**Requirement 4.2:** If any memory operation (FilePath, LoadEntries, CountEntries) returns an error while loading memory before the LLM call, the run must NOT fail. Instead, print a warning to stderr:

```
Warning: failed to load memory for "<agentName>": <error>
```

Continue the run without memory content in the system prompt.

**Requirement 4.3:** After the LLM response is fully received and output is complete (after the final `fmt.Fprint` or JSON output), if `cfg.Memory.Enabled` is `true` AND the LLM call succeeded (no error):

1. Resolve the memory file path via `memory.FilePath(agentName, cfg.Memory.Path)`.
2. Call `memory.AppendEntry(path, userMessage, resp.Content)`.
3. If `AppendEntry` returns an error, print a warning to stderr:

```
Warning: failed to save memory for "<agentName>": <error>
```

Do NOT return an error or change the exit code. The agent's run was successful even if memory write failed.

**Requirement 4.4:** If the LLM call fails (provider error, timeout, context cancellation, max conversation turns exceeded), do NOT append a memory entry. The error condition means there is no meaningful result to record.

**Requirement 4.5:** If `--dry-run` is set and `cfg.Memory.Enabled` is `true`, the dry-run output must include a memory section:

```
--- Memory ---
<loaded entries or "(none)" if empty>
```

This section is displayed after the existing "Stdin" section and before the "Sub-Agents" section. If memory loading fails, display `(error: <message>)` instead of entries.

**Requirement 4.6:** If `--dry-run` is set, do NOT append a memory entry regardless of memory config.

**Requirement 4.7:** If `--verbose` is set and `cfg.Memory.Enabled` is `true`, print to stderr before the LLM call:

```
Memory:   <N> entries loaded from <path>
```

If no entries are loaded (file doesn't exist or is empty):

```
Memory:   0 entries (no memory file)
```

### 3.5 Memory Integration — Sub-Agent Execution (`internal/tool/`)

**Requirement 5.1:** In `ExecuteCallAgent`, after building the sub-agent's system prompt and before building the request: if the sub-agent's `cfg.Memory.Enabled` is `true`, resolve the memory file path and load entries into the system prompt using the same logic as Requirement 4.1. Use the sub-agent's `agentName` and `cfg.Memory` config, not the parent's.

**Requirement 5.2:** In `ExecuteCallAgent`, after the sub-agent's conversation loop completes successfully and before returning the `ToolResult`: if the sub-agent's `cfg.Memory.Enabled` is `true`, append a memory entry using the same logic as Requirement 4.3. The "task" is the user message sent to the sub-agent (the combined task + context string). The "result" is `resp.Content`.

**Requirement 5.3:** If memory loading fails for a sub-agent, proceed without memory. If memory appending fails for a sub-agent, do not fail the sub-agent execution. In both cases, if `opts.Verbose` is `true`, print warnings to `opts.Stderr`.

**Requirement 5.4:** If the sub-agent's conversation loop fails (error return), do NOT append a memory entry. Same rule as Requirement 4.4.

### 3.6 Memory Section in System Prompt (`internal/resolve/`)

**Requirement 6.1:** The memory section is appended to the system prompt string **outside** of `BuildSystemPrompt`. The caller (either `cmd/run.go` or `internal/tool/tool.go`) appends the memory section directly to the system prompt string after calling `BuildSystemPrompt`. The `BuildSystemPrompt` function signature and behavior remain unchanged.

**Requirement 6.2:** The memory section format is:

```
\n\n---\n\n## Memory\n\n<entries>
```

This matches the existing section delimiter pattern used by `BuildSystemPrompt` for Skill and Context Files sections.

---

## 4. Project Structure

After M6 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── run.go                    # MODIFIED: memory load before LLM, memory append after response, dry-run memory display, verbose memory output
│   ├── run_test.go               # MODIFIED: tests for memory integration in run command
│   ├── agents.go                 # MODIFIED: display memory.last_n, memory.max_entries in agents show
│   ├── agents_test.go            # MODIFIED: test updated agents show output
│   ├── config.go                 # UNCHANGED
│   ├── config_test.go            # UNCHANGED
│   ├── exit.go                   # UNCHANGED
│   ├── exit_test.go              # UNCHANGED
│   ├── root.go                   # UNCHANGED
│   ├── root_test.go              # UNCHANGED
│   ├── version.go                # UNCHANGED
│   └── version_test.go           # UNCHANGED
├── internal/
│   ├── memory/
│   │   ├── memory.go             # NEW: FilePath, AppendEntry, LoadEntries, CountEntries, Now variable
│   │   └── memory_test.go        # NEW: full test coverage for memory package
│   ├── agent/
│   │   ├── agent.go              # MODIFIED: MemoryConfig extended with LastN, MaxEntries; updated Validate, updated Scaffold
│   │   └── agent_test.go         # MODIFIED: tests for new MemoryConfig fields and validation
│   ├── tool/
│   │   ├── tool.go               # MODIFIED: memory load/append hooks in ExecuteCallAgent
│   │   └── tool_test.go          # MODIFIED: tests for sub-agent memory behavior
│   ├── xdg/
│   │   ├── xdg.go                # MODIFIED: add GetDataDir function
│   │   └── xdg_test.go           # MODIFIED: tests for GetDataDir
│   ├── config/                   # UNCHANGED
│   │   ├── config.go
│   │   └── config_test.go
│   ├── provider/                 # UNCHANGED
│   │   ├── provider.go
│   │   ├── provider_test.go
│   │   ├── anthropic.go
│   │   ├── anthropic_test.go
│   │   ├── openai.go
│   │   ├── openai_test.go
│   │   ├── ollama.go
│   │   ├── ollama_test.go
│   │   ├── registry.go
│   │   └── registry_test.go
│   └── resolve/                  # UNCHANGED
│       ├── resolve.go
│       └── resolve_test.go
├── go.mod                        # UNCHANGED (no new dependencies)
├── go.sum                        # UNCHANGED
└── ...
```

---

## 5. Edge Cases

### 5.1 Memory File

| Scenario | Behavior |
|----------|----------|
| Memory file does not exist | `LoadEntries` returns `("", nil)`. `CountEntries` returns `(0, nil)`. `AppendEntry` creates the file and parent directories. |
| Memory file exists but is empty | `LoadEntries` returns `("", nil)`. `CountEntries` returns `(0, nil)`. |
| Memory file has 1 entry, `last_n = 10` | `LoadEntries` returns that 1 entry. |
| Memory file has 50 entries, `last_n = 5` | `LoadEntries` returns only the last 5 entries. |
| Memory file has 50 entries, `last_n = 0` | `LoadEntries` returns all 50 entries. |
| Memory file is not readable (permission denied) | `LoadEntries` and `CountEntries` return an error. The run continues without memory (Requirement 4.2). |
| Memory file's parent directory is not writable | `AppendEntry` returns an error. Warning printed, run still succeeds (Requirement 4.3). |
| Memory file contains non-entry content before the first `## ` header | That content is included when `last_n = 0` (all entries). When `last_n > 0`, only parsed entries (starting with `## `) are counted and returned. Content before the first `## ` header is NOT counted as an entry and is NOT included in `last_n` results. |
| Memory file contains `## ` headers that are not timestamps | They are still treated as entry boundaries. The memory system does not validate timestamp format. |
| Concurrent writes to the same memory file | Not explicitly handled. The append-only, single-write-per-run pattern makes concurrent corruption unlikely in practice. If two runs append simultaneously, both entries will be written but may interleave. This is acceptable for M6. |

### 5.2 Memory Config

| Scenario | Behavior |
|----------|----------|
| `[memory]` section absent from TOML | All `MemoryConfig` fields are zero values: `Enabled=false`, `Path=""`, `LastN=0`, `MaxEntries=0`. No memory behavior. |
| `memory.enabled = true`, all other fields default | Memory enabled. Default path. `LastN=0` (load all). `MaxEntries=0` (no warning). |
| `memory.enabled = false` (explicitly set) | No memory behavior, regardless of other memory fields. |
| `memory.path` set to a relative path | Used as-is. The caller is responsible for ensuring the path is valid. No path resolution is performed on custom paths. |
| `memory.path` set to an absolute path | Used as-is. |
| `memory.last_n = -1` | Validation error: `"memory.last_n must be non-negative"`. |
| `memory.max_entries = -1` | Validation error: `"memory.max_entries must be non-negative"`. |
| `memory.last_n = 0` | Load all entries. |
| `memory.max_entries = 0` | No entry count warning. |

### 5.3 Entry Content

| Scenario | Behavior |
|----------|----------|
| Task is empty string | Stored as `**Task:** (none)` |
| Task contains newlines | Newlines replaced with spaces |
| Task is very long (>1000 chars) | Stored as-is. Only the result is truncated. |
| Result is empty string | Stored as `**Result:** (none)` |
| Result is exactly 1000 characters | Stored as-is, no truncation. |
| Result is 1001 characters | Truncated to 1000 characters + `...` appended. |
| Result contains markdown formatting | Stored as-is. No escaping. |
| Result contains `## ` on a line | Stored as-is. This may cause incorrect entry boundary parsing on subsequent reads. This is an accepted limitation — results rarely contain `## ` at the start of a line. |

### 5.4 Run Integration

| Scenario | Behavior |
|----------|----------|
| Memory enabled, first run (no file) | Memory section not added to prompt (empty). Entry appended, file created. |
| Memory enabled, subsequent run | Previous entries loaded into prompt. New entry appended. |
| Memory enabled, LLM call fails | Memory loaded into prompt. No entry appended after failure. |
| Memory enabled, `--dry-run` | Memory loaded and displayed. No entry appended. |
| Memory disabled | No memory loaded. No entry appended. No file created. No warnings. |
| Memory enabled, memory file unreadable | Warning to stderr. Run continues without memory. No entry appended (cannot determine file path / state). |
| Memory enabled, memory file not writable | Run succeeds, output printed. Warning to stderr about failed memory save. |
| Memory enabled, `--json` output | Memory entry still appended after JSON output. Memory content is in the system prompt, not in the JSON envelope. |
| Memory enabled, sub-agents present | Parent's memory loaded into parent's prompt. Sub-agent's memory (if enabled) loaded into sub-agent's prompt independently. |

### 5.5 Sub-Agent Memory

| Scenario | Behavior |
|----------|----------|
| Parent has memory enabled, sub-agent does not | Parent logs to its own memory. Sub-agent has no memory file. |
| Parent has memory disabled, sub-agent has memory enabled | Parent has no memory. Sub-agent logs to its own memory file. |
| Both parent and sub-agent have memory enabled | Each writes to its own file. Parent's memory does not contain sub-agent results. |
| Sub-agent call fails | No memory entry for the sub-agent. Parent may record the overall result (which includes the error-handling response). |
| Multiple sub-agents called in parallel, all with memory enabled | Each writes to its own file. No locking concerns because each agent has a separate file. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** No database. Memory is a plain text markdown file.

**Constraint 3:** No garbage collection. M6 only appends and reads entries. Trimming, pattern detection, and the `axe gc` command are M7 scope.

**Constraint 4:** No shared memory. Parent and sub-agent memory files are completely independent. Shared memory is v2 scope.

**Constraint 5:** No streaming. The full LLM response must be received before a memory entry is appended.

**Constraint 6:** Memory failures must never cause a run to fail. Memory is best-effort. All memory errors are reported as warnings to stderr and do not affect the exit code.

**Constraint 7:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. The `GetDataDir` fallback path (`$HOME/.local/share`) applies to all platforms.

**Constraint 8:** The `Provider` interface, `Request`, `Response`, and `Message` types remain unchanged. Memory is injected into the system prompt string, not into new fields on these types.

**Constraint 9:** The `BuildSystemPrompt` function signature remains unchanged. Memory injection is performed by the caller after `BuildSystemPrompt` returns.

**Constraint 10:** All user-facing output goes through `cmd.OutOrStdout()` (stdout) or `cmd.ErrOrStderr()` (stderr) for testability. Memory warnings go to stderr.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M1-M5:

- **Package-level tests:** Tests live in the same package (e.g. `package memory`, `package xdg`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real I/O.
- **Run tests with:** `make test`
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.

### 7.2 `internal/xdg/xdg_test.go` Tests (Modified)

**Test: `TestGetDataDir_XDGDataHomeSet`** — Set `XDG_DATA_HOME` to a temp directory. Verify `GetDataDir()` returns `<tempDir>/axe`.

**Test: `TestGetDataDir_XDGDataHomeEmpty`** — Set `XDG_DATA_HOME` to `""`. Verify `GetDataDir()` returns `<homeDir>/.local/share/axe` where `<homeDir>` is `os.UserHomeDir()`.

**Test: `TestGetDataDir_XDGDataHomeUnset`** — Ensure `XDG_DATA_HOME` is not set. Verify `GetDataDir()` returns `<homeDir>/.local/share/axe`.

**Test: `TestGetDataDir_DoesNotCreateDirectory`** — Call `GetDataDir()` with `XDG_DATA_HOME` pointing to a temp dir. Verify the returned path does NOT exist on disk.

### 7.3 `internal/agent/agent_test.go` Tests (Modified)

**Test: `TestLoad_MemoryConfig_AllFields`** — Write agent TOML with `[memory]` section containing `enabled = true`, `path = "/tmp/mem.md"`, `last_n = 5`, `max_entries = 50`. Verify all four fields are parsed correctly.

**Test: `TestLoad_MemoryConfig_Defaults`** — Write agent TOML without `[memory]` section. Verify `Memory.Enabled` is `false`, `Memory.Path` is `""`, `Memory.LastN` is `0`, `Memory.MaxEntries` is `0`.

**Test: `TestValidate_MemoryLastN_Negative`** — Set `Memory.LastN` to `-1`. Verify validation error: `"memory.last_n must be non-negative"`.

**Test: `TestValidate_MemoryMaxEntries_Negative`** — Set `Memory.MaxEntries` to `-1`. Verify validation error: `"memory.max_entries must be non-negative"`.

**Test: `TestValidate_MemoryLastN_Zero`** — Set `Memory.LastN` to `0`. Verify no validation error.

**Test: `TestValidate_MemoryMaxEntries_Zero`** — Set `Memory.MaxEntries` to `0`. Verify no validation error.

**Test: `TestScaffold_IncludesMemoryLastN`** — Verify scaffold output contains `# last_n = 10`.

**Test: `TestScaffold_IncludesMemoryMaxEntries`** — Verify scaffold output contains `# max_entries = 100`.

### 7.4 `internal/memory/memory_test.go` Tests (New)

**Test: `TestFilePath_Default`** — Set `XDG_DATA_HOME` to a temp dir. Call `FilePath("myagent", "")`. Verify result is `<tempDir>/axe/memory/myagent.md`.

**Test: `TestFilePath_CustomPath`** — Call `FilePath("myagent", "/custom/path/mem.md")`. Verify result is `/custom/path/mem.md`.

**Test: `TestFilePath_EmptyAgentName`** — Call `FilePath("", "")`. Verify the result ends with `/memory/.md` (the function does not validate the agent name).

**Test: `TestAppendEntry_CreatesFileAndDirs`** — Use a path within `t.TempDir()` where the parent directory does not exist. Call `AppendEntry`. Verify the file is created with correct content and the parent directory was created.

**Test: `TestAppendEntry_AppendsToExistingFile`** — Create a file with one existing entry. Call `AppendEntry`. Verify the file now contains both entries.

**Test: `TestAppendEntry_Format`** — Override `Now` to return a fixed time. Call `AppendEntry("path", "do the thing", "it worked")`. Verify the exact file content matches the expected format with the fixed timestamp.

**Test: `TestAppendEntry_EmptyTask`** — Call `AppendEntry` with empty task. Verify output contains `**Task:** (none)`.

**Test: `TestAppendEntry_EmptyResult`** — Call `AppendEntry` with empty result. Verify output contains `**Result:** (none)`.

**Test: `TestAppendEntry_ResultTruncation`** — Call `AppendEntry` with a result of 1001 characters. Verify stored result is exactly 1000 characters followed by `...`.

**Test: `TestAppendEntry_ResultExactly1000`** — Call `AppendEntry` with a result of exactly 1000 characters. Verify no truncation occurs.

**Test: `TestAppendEntry_TaskNewlines`** — Call `AppendEntry` with a task containing `\n`. Verify newlines are replaced with spaces in the stored task.

**Test: `TestAppendEntry_ResultNewlines`** — Call `AppendEntry` with a result containing `\n`. Verify newlines are preserved in the stored result.

**Test: `TestAppendEntry_FilePermissions`** — Verify created file has `0644` permissions. Verify created directory has `0755` permissions.

**Test: `TestLoadEntries_FileDoesNotExist`** — Call `LoadEntries` with a non-existent path. Verify returns `("", nil)`.

**Test: `TestLoadEntries_EmptyFile`** — Create an empty file. Call `LoadEntries`. Verify returns `("", nil)`.

**Test: `TestLoadEntries_AllEntries`** — Create a file with 3 entries. Call `LoadEntries(path, 0)`. Verify all 3 entries are returned.

**Test: `TestLoadEntries_LastN`** — Create a file with 5 entries. Call `LoadEntries(path, 2)`. Verify only the last 2 entries are returned.

**Test: `TestLoadEntries_LastN_ExceedsCount`** — Create a file with 3 entries. Call `LoadEntries(path, 10)`. Verify all 3 entries are returned.

**Test: `TestLoadEntries_LastN_One`** — Create a file with 5 entries. Call `LoadEntries(path, 1)`. Verify only the last entry is returned.

**Test: `TestLoadEntries_PreservesFormat`** — Create a file with known exact content. Call `LoadEntries`. Verify the returned string matches the expected content exactly.

**Test: `TestLoadEntries_ContentBeforeFirstEntry`** — Create a file with text before the first `## ` header. Call `LoadEntries(path, 1)`. Verify only the last entry is returned (pre-header content excluded). Call `LoadEntries(path, 0)`. Verify all content including pre-header text is returned.

**Test: `TestCountEntries_FileDoesNotExist`** — Call `CountEntries` with a non-existent path. Verify returns `(0, nil)`.

**Test: `TestCountEntries_EmptyFile`** — Create an empty file. Call `CountEntries`. Verify returns `(0, nil)`.

**Test: `TestCountEntries_MultipleEntries`** — Create a file with 5 entries. Call `CountEntries`. Verify returns `(5, nil)`.

**Test: `TestCountEntries_NoEntries`** — Create a file with content but no `## ` headers. Call `CountEntries`. Verify returns `(0, nil)`.

### 7.5 `cmd/run_test.go` Tests (Modified)

**Test: `TestRun_MemoryDisabled_NoFileCreated`** — Create agent with `memory.enabled = false`. Run the agent (mock provider). Verify no memory file exists after the run.

**Test: `TestRun_MemoryEnabled_AppendsEntry`** — Create agent with `memory.enabled = true`. Set `XDG_DATA_HOME` to temp dir. Start mock provider returning a known response. Run the agent. Verify the memory file exists and contains one entry with the correct task and result.

**Test: `TestRun_MemoryEnabled_LoadsIntoPrompt`** — Create agent with `memory.enabled = true`. Pre-populate a memory file with entries. Start mock provider that captures the system prompt from the request body. Run the agent. Verify the system prompt contains a `## Memory` section with the pre-populated entries.

**Test: `TestRun_MemoryEnabled_LastN`** — Create agent with `memory.enabled = true`, `memory.last_n = 2`. Pre-populate a memory file with 5 entries. Start mock provider that captures the system prompt. Run the agent. Verify only the last 2 entries appear in the system prompt.

**Test: `TestRun_MemoryEnabled_MaxEntriesWarning`** — Create agent with `memory.enabled = true`, `memory.max_entries = 3`. Pre-populate a memory file with 3 entries. Run the agent. Verify stderr contains the max_entries warning message.

**Test: `TestRun_MemoryEnabled_MaxEntriesNoWarningWhenBelow`** — Create agent with `memory.enabled = true`, `memory.max_entries = 10`. Pre-populate a memory file with 3 entries. Run the agent. Verify stderr does NOT contain a warning message.

**Test: `TestRun_MemoryEnabled_APIError_NoEntryAppended`** — Create agent with `memory.enabled = true`. Start mock provider that returns a 500 error. Run the agent (expect failure). Verify no memory file was created or written to.

**Test: `TestRun_MemoryEnabled_DryRun`** — Create agent with `memory.enabled = true`. Pre-populate a memory file. Run with `--dry-run`. Verify output contains `--- Memory ---` section with the entries. Verify no new entry was appended to the memory file.

**Test: `TestRun_MemoryEnabled_DryRun_NoMemoryFile`** — Create agent with `memory.enabled = true`, no memory file. Run with `--dry-run`. Verify output contains `--- Memory ---` section with `(none)`.

**Test: `TestRun_MemoryEnabled_Verbose`** — Create agent with `memory.enabled = true`. Pre-populate a memory file with 3 entries. Run with `--verbose`. Verify stderr contains `Memory:` line with entry count and path.

**Test: `TestRun_MemoryEnabled_CustomPath`** — Create agent with `memory.enabled = true`, `memory.path = "<tempDir>/custom.md"`. Run the agent. Verify the custom path is used for both reading and writing.

### 7.6 `internal/tool/tool_test.go` Tests (Modified)

**Test: `TestExecuteCallAgent_MemoryEnabled_AppendsEntry`** — Create a sub-agent with `memory.enabled = true`. Set `XDG_DATA_HOME` to temp dir. Execute the sub-agent (mock provider). Verify a memory entry exists in the sub-agent's memory file.

**Test: `TestExecuteCallAgent_MemoryEnabled_LoadsIntoPrompt`** — Create a sub-agent with `memory.enabled = true`. Pre-populate the sub-agent's memory file. Execute the sub-agent (mock provider that captures request). Verify the system prompt contains a `## Memory` section.

**Test: `TestExecuteCallAgent_MemoryDisabled_NoFileCreated`** — Create a sub-agent with `memory.enabled = false`. Execute the sub-agent. Verify no memory file was created.

**Test: `TestExecuteCallAgent_MemoryEnabled_Error_NoEntryAppended`** — Create a sub-agent with `memory.enabled = true`. Mock provider returns error. Verify no memory entry was appended.

### 7.7 `cmd/agents_test.go` Tests (Modified)

**Test: `TestAgentsShow_MemoryAllFields`** — Create agent with all memory fields set. Run `agents show`. Verify output contains `Memory LastN:` and `Memory MaxEntries:` lines.

**Test: `TestAgentsShow_MemoryDefaults`** — Create agent without `[memory]` section. Run `agents show`. Verify memory fields are not displayed (all zero values).

---

## 8. Acceptance Criteria

The milestone is complete when all of the following are true:

1. `make test` passes with zero failures.
2. A new `internal/memory/` package exists with `FilePath`, `AppendEntry`, `LoadEntries`, and `CountEntries` functions.
3. `internal/xdg/` has a `GetDataDir` function that follows the XDG Base Directory specification.
4. `internal/agent/MemoryConfig` has `LastN` and `MaxEntries` fields that are parsed from TOML and validated.
5. Running `axe run <agent>` with `memory.enabled = true` appends a timestamped entry to the agent's memory file after a successful LLM call.
6. Running `axe run <agent>` with `memory.enabled = true` loads the last N entries into the system prompt before calling the LLM.
7. The `--dry-run` flag displays the memory section without appending an entry.
8. The `--verbose` flag displays memory loading info on stderr.
9. A warning is printed to stderr when `max_entries` is met or exceeded.
10. Sub-agents with `memory.enabled = true` independently load and append to their own memory files.
11. Memory failures never cause a run to fail or change the exit code.
12. No new external dependencies are introduced.
