# Implementation Checklist: M6 - Memory

**Based on:** 006_memory_spec.md
**Status:** Complete
**Created:** 2026-02-28

---

## Phase 1: XDG Data Directory (`internal/xdg/xdg.go`) (Spec §3.1)

### 1a: Tests (Red)

- [x] Write `TestGetDataDir_XDGDataHomeSet` — set `XDG_DATA_HOME` to temp dir; verify returns `<tempDir>/axe` (Req 1.1)
- [x] Write `TestGetDataDir_XDGDataHomeEmpty` — set `XDG_DATA_HOME` to `""`; verify returns `<homeDir>/.local/share/axe` (Req 1.2)
- [x] Write `TestGetDataDir_XDGDataHomeUnset` — ensure `XDG_DATA_HOME` not set; verify returns `<homeDir>/.local/share/axe` (Req 1.2)
- [x] Write `TestGetDataDir_DoesNotCreateDirectory` — call `GetDataDir` with `XDG_DATA_HOME` pointing to temp dir; verify returned path does NOT exist on disk (Req 1.3)
- [x] Run tests — confirm all 4 tests fail (red)

### 1b: Implementation (Green)

- [x] Implement `GetDataDir() (string, error)` in `internal/xdg/xdg.go` — checks `$XDG_DATA_HOME`, falls back to `$HOME/.local/share/axe`, wraps errors with `"unable to determine data directory: <err>"` (Req 1.1, 1.2, 1.3)
- [x] Run tests — all `GetDataDir` tests pass (green)
- [x] Verify `GetConfigDir` remains unchanged (Req 1.4)
- [x] Run `make test` — all existing tests still pass

---

## Phase 2: Memory Config Extension (`internal/agent/agent.go`) (Spec §3.2)

### 2a: Tests for New Fields (Red)

- [x] Write `TestLoad_MemoryConfig_AllFields` — TOML with `[memory]` section containing all four fields; verify `LastN` and `MaxEntries` parsed correctly (Req 2.1)
- [x] Write `TestLoad_MemoryConfig_Defaults` — TOML without `[memory]` section; verify `Memory.LastN` is `0`, `Memory.MaxEntries` is `0` (Req 2.1)
- [x] Run tests — confirm new field tests fail (red)

### 2b: Implement New Fields (Green)

- [x] Add `LastN int` (`toml:"last_n"`) and `MaxEntries int` (`toml:"max_entries"`) to `MemoryConfig` struct (Req 2.1)
- [x] Run tests — field parsing tests pass (green)

### 2c: Validation Tests (Red)

- [x] Write `TestValidate_MemoryLastN_Negative` — set `Memory.LastN` to `-1`; verify error `"memory.last_n must be non-negative"` (Req 2.2)
- [x] Write `TestValidate_MemoryMaxEntries_Negative` — set `Memory.MaxEntries` to `-1`; verify error `"memory.max_entries must be non-negative"` (Req 2.3)
- [x] Write `TestValidate_MemoryLastN_Zero` — set `Memory.LastN` to `0`; verify no validation error (Req 2.2)
- [x] Write `TestValidate_MemoryMaxEntries_Zero` — set `Memory.MaxEntries` to `0`; verify no validation error (Req 2.3)
- [x] Run tests — confirm negative-value tests fail (red)

### 2d: Implement Validation (Green)

- [x] Add validation rules in `Validate`: `LastN >= 0` and `MaxEntries >= 0` with specified error messages (Req 2.2, 2.3)
- [x] Run tests — all validation tests pass (green)

### 2e: Scaffold Tests (Red)

- [x] Write `TestScaffold_IncludesMemoryLastN` — verify scaffold output contains `# last_n = 10` (Req 2.4)
- [x] Write `TestScaffold_IncludesMemoryMaxEntries` — verify scaffold output contains `# max_entries = 100` (Req 2.4)
- [x] Run tests — confirm scaffold tests fail (red)

### 2f: Update Scaffold (Green)

- [x] Update `Scaffold` function `[memory]` section to include all four fields: `enabled`, `path`, `last_n = 10`, `max_entries = 100` (Req 2.4)
- [x] Run tests — scaffold tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 3: Memory Package Core (`internal/memory/memory.go`) (Spec §3.3)

### 3a: FilePath Tests (Red)

- [x] Create `internal/memory/` directory and empty `memory.go` file with `package memory`
- [x] Write `TestFilePath_Default` — set `XDG_DATA_HOME` to temp dir; call `FilePath("myagent", "")`; verify `<tempDir>/axe/memory/myagent.md` (Req 3.2)
- [x] Write `TestFilePath_CustomPath` — call `FilePath("myagent", "/custom/path/mem.md")`; verify returns `/custom/path/mem.md` (Req 3.2)
- [x] Write `TestFilePath_EmptyAgentName` — call `FilePath("", "")`; verify ends with `/memory/.md` (Req 3.2)
- [x] Run tests — confirm all fail (red)

### 3b: Implement FilePath (Green)

- [x] Implement `FilePath(agentName, customPath string) (string, error)` — custom path passthrough, default via `xdg.GetDataDir()`, no directory creation (Req 3.2)
- [x] Run tests — `FilePath` tests pass (green)

### 3c: AppendEntry Tests (Red)

- [x] Define `var Now func() time.Time = time.Now` package-level variable (Req 3.4)
- [x] Write `TestAppendEntry_CreatesFileAndDirs` — path in `t.TempDir()` where parent doesn't exist; verify file and dirs created with correct content (Req 3.3)
- [x] Write `TestAppendEntry_AppendsToExistingFile` — create file with one entry; append another; verify both entries present (Req 3.3)
- [x] Write `TestAppendEntry_Format` — override `Now` to fixed time; verify exact format matches spec (Req 3.3, 3.4)
- [x] Write `TestAppendEntry_EmptyTask` — empty task; verify `**Task:** (none)` (Req 3.3)
- [x] Write `TestAppendEntry_EmptyResult` — empty result; verify `**Result:** (none)` (Req 3.3)
- [x] Write `TestAppendEntry_ResultTruncation` — 1001-char result; verify stored as 1000 chars + `...` (Req 3.3)
- [x] Write `TestAppendEntry_ResultExactly1000` — 1000-char result; verify no truncation (Req 3.3)
- [x] Write `TestAppendEntry_TaskNewlines` — task with `\n`; verify newlines replaced with spaces (Req 3.3)
- [x] Write `TestAppendEntry_ResultNewlines` — result with `\n`; verify newlines preserved (Req 3.3)
- [x] Write `TestAppendEntry_FilePermissions` — verify file `0644`, directory `0755` (Req 3.3)
- [x] Run tests — confirm all fail (red)

### 3d: Implement AppendEntry (Green)

- [x] Implement `AppendEntry(path, task, result string) error` — `MkdirAll`, open append mode, write formatted entry with timestamp, handle empty/truncation/newline rules (Req 3.3, 3.4)
- [x] Run tests — all `AppendEntry` tests pass (green)

### 3e: LoadEntries Tests (Red)

- [x] Write `TestLoadEntries_FileDoesNotExist` — non-existent path; verify `("", nil)` (Req 3.5)
- [x] Write `TestLoadEntries_EmptyFile` — empty file; verify `("", nil)` (Req 3.5)
- [x] Write `TestLoadEntries_AllEntries` — 3 entries, `lastN=0`; verify all returned (Req 3.5)
- [x] Write `TestLoadEntries_LastN` — 5 entries, `lastN=2`; verify only last 2 returned (Req 3.5)
- [x] Write `TestLoadEntries_LastN_ExceedsCount` — 3 entries, `lastN=10`; verify all returned (Req 3.5)
- [x] Write `TestLoadEntries_LastN_One` — 5 entries, `lastN=1`; verify only last entry returned (Req 3.5)
- [x] Write `TestLoadEntries_PreservesFormat` — known content; verify exact match (Req 3.5)
- [x] Write `TestLoadEntries_ContentBeforeFirstEntry` — text before first `## `; `lastN=1` excludes it, `lastN=0` includes all (Req 3.5)
- [x] Run tests — confirm all fail (red)

### 3f: Implement LoadEntries (Green)

- [x] Implement `LoadEntries(path string, lastN int) (string, error)` — handle missing file, parse entries by `## ` headers, return last N or all (Req 3.5)
- [x] Run tests — all `LoadEntries` tests pass (green)

### 3g: CountEntries Tests (Red)

- [x] Write `TestCountEntries_FileDoesNotExist` — non-existent path; verify `(0, nil)` (Req 3.6)
- [x] Write `TestCountEntries_EmptyFile` — empty file; verify `(0, nil)` (Req 3.6)
- [x] Write `TestCountEntries_MultipleEntries` — 5 entries; verify `(5, nil)` (Req 3.6)
- [x] Write `TestCountEntries_NoEntries` — content but no `## ` headers; verify `(0, nil)` (Req 3.6)
- [x] Run tests — confirm all fail (red)

### 3h: Implement CountEntries (Green)

- [x] Implement `CountEntries(path string) (int, error)` — handle missing file, count `## ` header lines (Req 3.6)
- [x] Run tests — all `CountEntries` tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 4: Top-Level Run Integration (`cmd/run.go`) (Spec §3.4)

### 4a: Memory Load into System Prompt Tests (Red)

- [x] Write `TestRun_MemoryDisabled_NoFileCreated` — agent with `memory.enabled = false`; mock provider; verify no memory file after run (Req 4.1)
- [x] Write `TestRun_MemoryEnabled_LoadsIntoPrompt` — pre-populated memory file; mock provider captures system prompt; verify `## Memory` section present (Req 4.1)
- [x] Write `TestRun_MemoryEnabled_LastN` — `last_n = 2`, 5 entries; verify only last 2 in system prompt (Req 4.1)
- [x] Run tests — confirm fail (red)

### 4b: Implement Memory Load (Green)

- [x] After `BuildSystemPrompt` (line 141) and before dry-run check: if `cfg.Memory.Enabled`, resolve path, load entries, append `\n\n---\n\n## Memory\n\n<entries>` to `systemPrompt` (Req 4.1, 6.1, 6.2)
- [x] Handle load errors: print warning to stderr, continue without memory (Req 4.2)
- [x] Run tests — memory load tests pass (green)

### 4c: Max Entries Warning Tests (Red)

- [x] Write `TestRun_MemoryEnabled_MaxEntriesWarning` — `max_entries = 3`, 3 entries; verify stderr contains warning (Req 4.1)
- [x] Write `TestRun_MemoryEnabled_MaxEntriesNoWarningWhenBelow` — `max_entries = 10`, 3 entries; verify no warning (Req 4.1)
- [x] Run tests — confirm fail (red)

### 4d: Implement Max Entries Warning (Green)

- [x] After loading entries: if `MaxEntries > 0` and count `>= MaxEntries`, print warning to stderr (Req 4.1)
- [x] Run tests — warning tests pass (green)

### 4e: Memory Append After Response Tests (Red)

- [x] Write `TestRun_MemoryEnabled_AppendsEntry` — `memory.enabled = true`; mock provider; verify memory file created with correct entry (Req 4.3)
- [x] Write `TestRun_MemoryEnabled_APIError_NoEntryAppended` — mock provider returns 500; verify no memory file (Req 4.4)
- [x] Run tests — confirm fail (red)

### 4f: Implement Memory Append (Green)

- [x] After successful LLM response and output: if `cfg.Memory.Enabled` and no error, resolve path and call `AppendEntry` (Req 4.3)
- [x] If `AppendEntry` errors, print warning to stderr; do not change exit code (Req 4.3)
- [x] Ensure no append on LLM error (Req 4.4)
- [x] Run tests — append tests pass (green)

### 4g: Dry-Run Memory Display Tests (Red)

- [x] Write `TestRun_MemoryEnabled_DryRun` — pre-populated memory; `--dry-run`; verify `--- Memory ---` section with entries; no new entry appended (Req 4.5, 4.6)
- [x] Write `TestRun_MemoryEnabled_DryRun_NoMemoryFile` — no memory file; `--dry-run`; verify `--- Memory ---` section with `(none)` (Req 4.5)
- [x] Run tests — confirm fail (red)

### 4h: Implement Dry-Run Memory Display (Green)

- [x] In `printDryRun`: if `cfg.Memory.Enabled`, display `--- Memory ---` section between Stdin and Sub-Agents; show entries or `(none)` or `(error: <msg>)` (Req 4.5, 4.6)
- [x] Run tests — dry-run memory tests pass (green)

### 4i: Verbose Memory Output Tests (Red)

- [x] Write `TestRun_MemoryEnabled_Verbose` — 3 entries; `--verbose`; verify stderr contains `Memory:   <N> entries loaded from <path>` (Req 4.7)
- [x] Run tests — confirm fail (red)

### 4j: Implement Verbose Memory Output (Green)

- [x] In verbose output block: if `cfg.Memory.Enabled`, print entry count and path to stderr (Req 4.7)
- [x] Handle case with no memory file: print `Memory:   0 entries (no memory file)` (Req 4.7)
- [x] Run tests — verbose tests pass (green)

### 4k: Custom Path Test (Red)

- [x] Write `TestRun_MemoryEnabled_CustomPath` — `memory.path = "<tempDir>/custom.md"`; verify custom path used for both read and write (Req 4.1, 4.3)
- [x] Run tests — confirm fail (red)

### 4l: Verify Custom Path (Green)

- [x] Run tests — custom path test should pass with existing implementation (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 5: Sub-Agent Memory Integration (`internal/tool/tool.go`) (Spec §3.5)

### 5a: Sub-Agent Memory Tests (Red)

- [x] Write `TestExecuteCallAgent_MemoryEnabled_LoadsIntoPrompt` — sub-agent with `memory.enabled = true`; pre-populated memory file; mock provider captures request; verify `## Memory` section in system prompt (Req 5.1)
- [x] Write `TestExecuteCallAgent_MemoryEnabled_AppendsEntry` — sub-agent with `memory.enabled = true`; mock provider; verify memory entry in sub-agent's memory file (Req 5.2)
- [x] Write `TestExecuteCallAgent_MemoryDisabled_NoFileCreated` — sub-agent with `memory.enabled = false`; verify no memory file created (Req 5.1)
- [x] Write `TestExecuteCallAgent_MemoryEnabled_Error_NoEntryAppended` — sub-agent with `memory.enabled = true`; mock provider returns error; verify no memory entry appended (Req 5.4)
- [x] Run tests — confirm all fail (red)

### 5b: Implement Sub-Agent Memory (Green)

- [x] In `ExecuteCallAgent`: after `BuildSystemPrompt` and before request build, if sub-agent's `cfg.Memory.Enabled`, load entries into system prompt (Req 5.1)
- [x] In `ExecuteCallAgent`: after successful conversation loop and before returning `ToolResult`, if sub-agent's `cfg.Memory.Enabled`, append memory entry (Req 5.2)
- [x] Handle memory errors gracefully: log warnings to `opts.Stderr` if `opts.Verbose`, never fail execution (Req 5.3)
- [x] No memory append on sub-agent error (Req 5.4)
- [x] Run tests — all sub-agent memory tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 6: Agents Show Command (`cmd/agents.go`) (Spec §3.2)

### 6a: Agents Show Tests (Red)

- [x] Write `TestAgentsShow_MemoryAllFields` — agent with all memory fields set; verify output contains `Memory LastN:` and `Memory MaxEntries:` lines (Req 2.5)
- [x] Write `TestAgentsShow_MemoryDefaults` — agent without `[memory]` section; verify memory fields not displayed (Req 2.5)
- [x] Run tests — confirm fail (red)

### 6b: Implement Agents Show Update (Green)

- [x] Update `agents show` to display `Memory LastN` and `Memory MaxEntries` alongside existing fields when any memory field has non-zero value (Req 2.5)
- [x] Run tests — agents show tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 7: Final Verification

- [x] Run `make test` — all tests pass with 0 failures
- [x] Verify `go.mod` still contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies (Constraint 1)
- [x] Verify `Provider` interface signature is unchanged (Constraint 8)
- [x] Verify `BuildSystemPrompt` function signature is unchanged (Constraint 9)
- [x] Verify no `internal/memory/` external dependencies — stdlib only (Constraint 1, Decision 14)
- [x] Verify memory failures never cause run to fail or change exit code (Constraint 6)
- [x] Verify `--dry-run` displays memory but does not append entries (Req 4.5, 4.6)
- [x] Verify `--verbose` displays memory loading info (Req 4.7)
- [x] Verify max_entries warning prints to stderr (Req 4.1)
- [x] Verify sub-agents independently load/append their own memory files (Req 5.1, 5.2)
- [x] Verify all existing M1-M5 tests still pass unchanged
