# Implement: Extract core execution logic into `pkg/runner`

## Context Summary

The goal is to extract ~900 lines of orchestration from `cmd/run.go` into a new public `pkg/runner` package while keeping `cmd/run.go` as a thin Cobra wrapper. This enables external modules to call Axe programmatically. All existing `internal/` packages remain unchanged.

## Tasks

### Phase 1: Package Skeleton & Types
- [ ] **Task 1**: Create `pkg/runner/` directory with `options.go` defining `Options` struct matching all CLI overrides
- [ ] **Task 2**: Create `pkg/runner/result.go` defining `Result` struct with all required fields
- [ ] **Task 3**: Create `pkg/runner/errors.go` defining exported error types: `ConfigError`, `RuntimeError`, `ProviderCategoryError` so CLI can map to exit codes 2, 1, 3, 4

### Phase 2: Core Execution (Tracer Bullet)
- [ ] **Task 4**: Create `pkg/runner/run.go` with `Run()` function implementing: agent loading, override application, workdir resolution, file/skill resolution, system prompt building, provider setup, and single-shot non-streaming execution with no tools. Include dry-run support.
- [ ] **Task 5**: Write `pkg/runner/run_test.go` with tracer bullet test: `TestRun_SingleShot_NoTools` using a mock provider, verifying Result content and token counts.

### Phase 3: Conversation Loop & Tools
- [ ] **Task 6**: Add conversation loop to `Run()` — tool call handling, message history, parallel/sequential execution, max 50 turns
- [ ] **Task 7**: Write tests for conversation loop: `TestRun_ConversationLoop_OneToolCall`, `TestRun_ParallelToolExecution`

### Phase 4: Streaming
- [ ] **Task 8**: Add streaming support to `Run()` — detect stream capability, write chunks to stdout, reconstruct response
- [ ] **Task 9**: Write test `TestRun_Streaming` using a mock stream provider

### Phase 5: Advanced Features
- [ ] **Task 10**: Add budget tracking, memory load/append, artifact tracking, MCP server connection, sub-agent delegation to `Run()`
- [ ] **Task 11**: Write tests for budget tracking, memory, artifacts, sub-agent depth limits

### Phase 6: CLI Refactor
- [ ] **Task 12**: Refactor `cmd/run.go` to thin wrapper: parse flags → construct `runner.Options` → call `runner.Run()` → map errors to `ExitError` codes. Remove all internal orchestration logic from `cmd/run.go`.
- [ ] **Task 13**: Ensure all existing `cmd/` tests pass (no breaking changes to CLI behavior, flags, or output)

### Phase 7: Polish
- [ ] **Task 14**: Add verbose logging support in runner (write diagnostics to stderr writer)
- [ ] **Task 15**: Add JSON output support in runner (marshal Result to stdout when JSON mode enabled)
- [ ] **Task 16**: Run full test suite, fix any regressions. Ensure `go build` succeeds.

## TDD Rules
- One behavior per test cycle (vertical slicing)
- Tests exercise public `runner.Run()` API only
- No mocking of internal packages — use real types or minimal test doubles
