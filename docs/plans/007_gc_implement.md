# Implementation Checklist: M7 - Garbage Collection

**Based on:** 007_gc_spec.md
**Status:** Pending
**Created:** 2026-02-28

---

## Phase 1: `TrimEntries` Function (`internal/memory/memory.go`) (Spec §3.1)

### 1a: Tests (Red)

- [x] Write `TestTrimEntries_FileDoesNotExist` — call `TrimEntries` with non-existent path and `keepN=5`; verify returns `(0, nil)` (Req 1.1)
- [x] Write `TestTrimEntries_EmptyFile` — create empty file; call `TrimEntries(path, 5)`; verify returns `(0, nil)` and file is still empty (Req 1.1)
- [x] Write `TestTrimEntries_KeepNZero` — create file with 5 entries; call `TrimEntries(path, 0)`; verify returns `(0, nil)` and file unchanged (Req 1.1)
- [x] Write `TestTrimEntries_KeepNNegative` — call `TrimEntries(path, -1)`; verify error contains `"keepN must be non-negative"` (Req 1.1)
- [x] Write `TestTrimEntries_EntriesWithinLimit` — file with 3 entries, `keepN=5`; verify returns `(0, nil)` and file unchanged (Req 1.1)
- [x] Write `TestTrimEntries_EntriesEqualToLimit` — file with 5 entries, `keepN=5`; verify returns `(0, nil)` and file unchanged (Req 1.1)
- [x] Write `TestTrimEntries_TrimsOldEntries` — file with 10 entries, `keepN=3`; verify returns `(7, nil)` and file has exactly last 3 entries, byte-identical to `LoadEntries(path, 3)` result (Req 1.1, 1.3)
- [x] Write `TestTrimEntries_PreservesEntryFormat` — file with multi-line entries; verify whitespace, blank lines, and formatting preserved exactly in kept entries (Req 1.3)
- [x] Write `TestTrimEntries_DiscardsPreHeaderContent` — file with text before first `## ` header + 5 entries; `keepN=3`; verify pre-header text discarded, only last 3 entries remain (Req 1.1)
- [x] Write `TestTrimEntries_SingleEntryKeepOne` — file with 1 entry, `keepN=1`; verify returns `(0, nil)` and file unchanged (Req 1.1)
- [x] Write `TestTrimEntries_OriginalUnmodifiedOnWriteError` — file with 5 entries in read-only directory; verify original file unmodified on failure (Req 1.1, 1.2)
- [x] Run tests — confirm all 11 tests fail (red)

### 1b: Implementation (Green)

- [x] Implement `TrimEntries(path string, keepN int) (removed int, err error)` in `internal/memory/memory.go` — read file, parse `## ` boundaries, validate keepN, atomic write-temp-then-rename (Req 1.1, 1.2, 1.3)
- [x] Run tests — all `TrimEntries` tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 2: `gc` Command Registration and Argument Validation (`cmd/gc.go`) (Spec §3.2)

### 2a: Tests (Red)

- [x] Write `TestGC_NoArgsNoAll` — run `axe gc` with no args, no `--all`; verify error `"agent name is required (or use --all)"` and exit code 1 (Req 2.1, 2.3)
- [x] Write `TestGC_AllWithAgentName` — run `axe gc myagent --all`; verify error `"cannot specify both --all and an agent name"` and exit code 1 (Req 2.3)
- [x] Run tests — confirm both fail (red)

### 2b: Implementation (Green)

- [x] Create `cmd/gc.go` with `gcCmd` cobra command: `Use: "gc"`, `Short: "Analyze and trim agent memory"`, `RunE: runGC` (Req 2.1)
- [x] Register flags: `--dry-run` (bool, false), `--all` (bool, false), `--model` (string, "") (Req 2.2)
- [x] Implement argument validation: mutually exclusive `--all` and positional arg, require one or the other (Req 2.3)
- [x] Register `gcCmd` on `rootCmd` in `init()` (Req 2.1)
- [x] Run tests — argument validation tests pass (green)
- [x] Run `make test` — all existing tests still pass

---

## Phase 3: Single-Agent GC Flow — Config Loading and Memory Check (`cmd/gc.go`) (Spec §3.3, Req 3.1–3.6)

### 3a: Tests (Red)

- [ ] Write `TestGC_AgentNotFound` — run `axe gc nonexistent`; verify exit code 2 (Req 3.1)
- [ ] Write `TestGC_MemoryDisabled` — create agent with `memory.enabled = false`; run `axe gc <agent>`; verify stderr has warning, exit code 0 (Req 3.2)
- [ ] Write `TestGC_NoMemoryFile` — create agent with `memory.enabled = true`, no memory file; verify stdout has "No memory entries", exit code 0 (Req 3.5)
- [ ] Write `TestGC_EmptyMemoryFile` — create agent with `memory.enabled = true`, empty memory file; verify stdout has "No memory entries", exit code 0 (Req 3.5)
- [ ] Run tests — confirm all fail (red)

### 3b: Implementation (Green)

- [ ] Implement agent config loading via `agent.Load(agentName)`, return `ExitError{Code: 2}` on failure (Req 3.1)
- [ ] Implement memory-disabled check: print warning to stderr, return nil (Req 3.2)
- [ ] Implement memory file path resolution via `memory.FilePath(agentName, cfg.Memory.Path)` (Req 3.4)
- [ ] Implement load-all-entries check: if `LoadEntries(path, 0)` returns empty string, print "No memory entries" and return nil (Req 3.5)
- [ ] Implement entry count display: `CountEntries(path)` and print `Agent: <name>`, `Entries: <count>` (Req 3.6)
- [ ] Run tests — config/memory-check tests pass (green)
- [ ] Run `make test` — all existing tests still pass

---

## Phase 4: Single-Agent GC Flow — LLM Pattern Detection (`cmd/gc.go`) (Spec §3.3, Req 3.7–3.9; §3.4)

### 4a: Tests (Red)

- [ ] Write `TestGC_PatternDetectionPrompt` — create agent with memory; start mock LLM server capturing request body; run `axe gc <agent>`; verify system prompt matches exact text from Req 4.1, user message is full memory content, temperature is `0.3`, max_tokens is `4096`, tools is empty (Req 3.7, 4.1, 4.2)
- [ ] Write `TestGC_LLMError` — create agent with memory; mock LLM returns HTTP 500; verify exit code 3 and memory file unchanged (Req 3.7)
- [ ] Write `TestGC_ModelOverride` — create agent with model `anthropic/claude-3`; start mock LLM; run `axe gc <agent> --model ollama/llama3`; verify LLM request uses `llama3` not `claude-3` (Req 3.3)
- [ ] Run tests — confirm all fail (red)

### 4b: Implementation (Green)

- [ ] Define the pattern detection prompt as a hard-coded `const` in `cmd/gc.go` (Req 4.1, 4.2)
- [ ] Implement `--model` override: if flag set, use it; else use `cfg.Model` (Req 3.3)
- [ ] Load global config via `config.Load()` and resolve API key/base URL for the provider (Req 3.8)
- [ ] Build `provider.Request` with system prompt, user message (memory content), temperature `0.3`, max_tokens `4096`, no tools (Req 3.7)
- [ ] Create provider via `provider.New()` and call `Send()` (Req 3.7)
- [ ] Print analysis to stdout: `--- Analysis ---\n<response>` (Req 3.9)
- [ ] Return `ExitError{Code: 3}` on LLM/API errors (Req 3.8)
- [ ] Run tests — LLM integration tests pass (green)
- [ ] Run `make test` — all existing tests still pass

---

## Phase 5: Single-Agent GC Flow — Dry-Run and Trim (`cmd/gc.go`) (Spec §3.3, Req 3.10–3.13)

### 5a: Tests (Red)

- [ ] Write `TestGC_DryRun` — agent with `last_n=3`, 10 entries; mock LLM; run `axe gc <agent> --dry-run`; verify analysis printed, "Dry run: no entries trimmed." printed, memory file still has 10 entries (Req 3.10)
- [ ] Write `TestGC_AnalyzeAndTrim` — agent with `last_n=3`, 10 entries; mock LLM; run `axe gc <agent>`; verify analysis printed, "Trimmed: 7 entries removed, 3 entries kept.", memory file has exactly 3 entries (Req 3.11, 3.12)
- [ ] Write `TestGC_NoTrimTarget` — agent with `last_n=0`, `max_entries=0`, 10 entries; mock LLM; run `axe gc <agent>`; verify analysis printed, "No trim target configured", memory file unchanged (Req 3.11)
- [ ] Write `TestGC_FallbackToMaxEntries` — agent with `last_n=0`, `max_entries=5`, 10 entries; mock LLM; run `axe gc <agent>`; verify "Trimmed: 5 entries removed, 5 entries kept.", memory file has 5 entries (Req 3.11, 3.12)
- [ ] Write `TestGC_NothingToTrim` — agent with `last_n=10`, 3 entries; mock LLM; run `axe gc <agent>`; verify "No trimming needed: 3 entries within limit (10)." (Req 3.12)
- [ ] Run tests — confirm all fail (red)

### 5b: Implementation (Green)

- [ ] Implement dry-run check: if `--dry-run`, print "Dry run: no entries trimmed." and return nil (Req 3.10)
- [ ] Implement trim target resolution: `cfg.Memory.LastN > 0` → use it, else `cfg.Memory.MaxEntries > 0` → use it, else skip trimming with message (Req 3.11)
- [ ] Call `memory.TrimEntries(path, trimTarget)` and print appropriate message based on `removed` count (Req 3.12)
- [ ] Handle `TrimEntries` error: print to stderr, return `ExitError{Code: 1}` (Req 3.13)
- [ ] Run tests — dry-run/trim tests pass (green)
- [ ] Run `make test` — all existing tests still pass

---

## Phase 6: All-Agents GC Flow (`cmd/gc.go`) (Spec §3.5)

### 6a: Tests (Red)

- [ ] Write `TestGC_AllFlag_NoMemoryAgents` — 2 agents both with `memory.enabled = false`; run `axe gc --all`; verify "No agents with memory enabled.", exit code 0 (Req 5.1, 5.2)
- [ ] Write `TestGC_AllFlag_MultipleAgents` — 3 agents: agent-a (memory enabled, 10 entries), agent-b (memory disabled), agent-c (memory enabled, 5 entries); mock LLM; run `axe gc --all`; verify agent-a and agent-c processed with separators, agent-b skipped, exit code 0 (Req 5.1–5.4, 5.7)
- [ ] Write `TestGC_AllFlag_PartialFailure` — 2 agents with memory; agent-b has unreadable memory file; mock LLM; run `axe gc --all`; verify stderr has error for agent-b, agent-a processed, exit code 1 with summary (Req 5.5, 5.6)
- [ ] Write `TestGC_AllFlag_WithDryRun` — 2 agents with memory; mock LLM; run `axe gc --all --dry-run`; verify analysis shown for both, no trimming, memory files unchanged (Req 5.3)
- [ ] Run tests — confirm all fail (red)

### 6b: Implementation (Green)

- [ ] Implement `--all` flow: call `agent.List()`, filter to `Memory.Enabled == true` (Req 5.1, 5.2)
- [ ] Print "No agents with memory enabled." if none qualify (Req 5.2)
- [ ] Process each agent sequentially with `=== GC: <agentName> ===` separator (Req 5.3, 5.4)
- [ ] On per-agent failure: print error to stderr, continue processing (Req 5.5)
- [ ] After all agents: if any failed, return `ExitError{Code: 1}` with summary `"gc completed with errors: <N> of <total> agents failed"` (Req 5.6)
- [ ] Run tests — all-agents tests pass (green)
- [ ] Run `make test` — all existing tests still pass

---

## Phase 7: Final Verification

- [ ] Run `make test` — all tests pass with 0 failures
- [ ] Verify `go.mod` has no new direct dependencies (Constraint 1)
- [ ] Verify `TrimEntries` uses atomic write-temp-then-rename (Constraint 8)
- [ ] Verify pattern detection prompt is hard-coded, not in TOML config (Constraint 7)
- [ ] Verify GC makes exactly one LLM call per agent, no tool calls (Constraint 3)
- [ ] Verify no interactive prompts or confirmations (Constraint 2)
- [ ] Verify GC does not append memory entries for its own operations (Constraint 9)
- [ ] Verify exit codes: 0 (success/skip/dry-run), 1 (agent error), 2 (config error), 3 (API error) (Req 6.1)
- [ ] Verify `--all` processes agents sequentially, not in parallel (Constraint 4)
- [ ] Verify agents with `memory.enabled = false` are warned/skipped correctly (Design Decision 6)
- [ ] Verify trim target resolution: `last_n` → `max_entries` → no trim (Design Decision 3)
- [ ] Update implementation file status to **Complete**
