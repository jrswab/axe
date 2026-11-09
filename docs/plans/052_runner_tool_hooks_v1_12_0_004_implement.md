# Implement Milestone 004: Let a host deny or stub any tool call before dispatch with `Options.ToolHook`

## Section 1: Context Summary

Milestone 004 of the v1.12.0 runner tool-hooks release lets a host program decide, per tool call, whether axe runs the call (allow), refuses it (deny, an error result the model sees), or substitutes a stub result. The decision is made by one host closure, `Options.ToolHook`, before any dispatch of the turn, so a denied call never reaches a builtin, an MCP server, or `call_agent`. The change is additive to `pkg/runner` (new `ToolAction`, `ToolAllow`, `ToolDeny`, `ToolStub`, `ToolDecision`, `ToolHook`, and the `Options.ToolHook` field). A nil hook keeps today's behavior byte for byte. A hook with a non-empty `sub_agents` list is a `ConfigError`, because the hook does not propagate to sub-agents. Hook errors and panics deny with a fixed text so raw error text never reaches the model. Cancellation during the decision pass dispatches nothing and returns the context error. Source: `docs/plans/052_runner_tool_hooks_v1_12_0_004_spec.md` (Sections 1 and 2, R1-R14, OD1-OD9, acceptance tests 1-13, failure modes 1-9) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 004, D6). Predecessors 002 and 003 are complete. Baseline commit `09d100d`; HEAD at checklist time is `3f7f7d0`.

Commit is not authorized in this checklist. Do not push. Do not change files outside the list in each task. Do not edit the spec, the checkpoint, or the milestones document; those are orchestrator records. Execution starts from a clean tree; do not stage or revert an orchestrator edit; stop and report if one appears.

Scope of changed files (spec R14): `pkg/runner/hook.go` (new), `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`, `pkg/runner/hook_mcp_test.go` (new), and `CHANGELOG.md` (one `[Unreleased]` Added bullet, task T7.1; required by `CONTRIBUTING.md:185`). Nothing else.

Per-task gate for this milestone: every task changes only files in `./pkg/runner`, so the derived package list is `./pkg/runner` (derived with the snippet in `/home/jaron/.agents/skills/factory/SKILL.md#per-task-go-gate`), except T7.1, which changes only `CHANGELOG.md` (no Go file, so its derived package list is empty). Red tasks do not pass the gate by design; their gate is the red run recorded in the task, and the gate runs in the green task that follows.

Test-selection convention: no command in this checklist selects tests by name. Declared tests do not exist until their red task, and spec-lint rejects a test-selection command that selects no test. Red steps therefore read: run the package tests with the `go test ./pkg/runner` form; the named new test fails with the stated reason. Per-test results come from the verbose package run (`go test -v ./pkg/runner`, or with `-race`) filtered with `rg -- '--- (PASS|FAIL): <Name>'`.

---

## Section 2: Implementation Checklist

### Phase 0: Pre-flight (serial, read-only)

- [ ] **T0: Pre-flight: HEAD ancestry, tree state, baseline package tests, declared names absent**
  - Budget: 30 min
  - Files: none (read-only).
  - Commands and expected results (run in the repository root):
    - `git merge-base --is-ancestor 09d100d HEAD; echo "exit=$?"` → `exit=0`.
    - `git diff --name-only 09d100d HEAD -- . ':(exclude)docs/plans/'` → no output (no file outside `docs/plans/` changed since `09d100d`). Orchestrator checkpoint commits may add further Markdown files under `docs/plans/`; those are allowed. At checklist time the list is `docs/plans/052_runner_tool_hooks_v1_12_0_003_checkpoint.md`, `docs/plans/052_runner_tool_hooks_v1_12_0_004_checkpoint.md`, `docs/plans/052_runner_tool_hooks_v1_12_0_004_spec.md`, `docs/plans/052_runner_tool_hooks_v1_12_0_004_implement.md`.
    - `git status --porcelain` → no output (clean tree). If an orchestrator edit is present, stop and report; do not stage or revert it.
    - `go test ./pkg/runner` → `ok`. This is the R11 baseline.
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolHook'` → no output; no test with prefix `TestToolHook` exists yet.
    - `ls pkg/runner/hook.go pkg/runner/hook_mcp_test.go 2>&1` → both report "No such file".
    - `rg -n "ToolHook|ToolDecision|ToolAction" --glob '*.go' .` → no output.
  - Go toolchain pre-flight: not triggered. No checklist command runs `go install` or sets `GOPROXY=`. Recorded as "skipped, not triggered".
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files, so no race run is needed; the derivation in `/home/jaron/.agents/skills/factory/SKILL.md#per-task-go-gate` yields an empty list before any edit.

### Phase 1: Enabling surface (serial)

- [ ] **T1: Public API surface and doc contract (R1, R12)**
  - Budget: 30 min
  - Depends on: T0.
  - Enables: T2.1-T2.7 (test code needs the identifiers to compile) and T3.x (implementation).
  - Files:
    - `pkg/runner/hook.go` (new file, package `runner`): `type ToolAction string`; constants `ToolAllow ToolAction = "allow"`, `ToolDeny ToolAction = "deny"`, `ToolStub ToolAction = "stub"`; `type ToolDecision struct { Action ToolAction; Content string }`; `type ToolHook func(ctx context.Context, call ToolCall) (ToolDecision, error)`. Imports: `context` only.
    - `pkg/runner/options.go`: add field `ToolHook ToolHook` to `type Options struct` (`pkg/runner/options.go:6`), after the `Messages []Message` field (`:27`) and before the I/O block (`:29`). Field doc comment must state: the field is ignored when nil; it is not called for dry runs (`DryRun`); a non-empty `sub_agents` list with a non-nil hook is a `ConfigError`.
  - Doc comments (R12, OD1, OD2, OD6):
    - On `ToolHook`: calls are sequential on the run goroutine, in model order, before any dispatch of that turn; concurrent `Run` calls that share one hook may call it concurrently, so the host must make it safe; a deny differs from an ordinary tool error because the tool never runs.
    - On `ToolDecision` or on each action constant: `ToolDeny` content is the decision `Content` when non-empty, else the fixed text `tool call denied by host policy`; `ToolStub` content is `Content` as-is; `ToolAllow` ignores `Content`; an unknown action (including `""`) is denied with the fixed text and `Content` is ignored.
  - Do not add the unexported deny-text constant in this task (T3.1 adds it, so `unused` lint does not fire on an intermediate state).
  - Do not add any behavior. `executeToolCalls`, `run`, and `Run` are unchanged in this task.
  - Failing-first: not applicable (type declarations only). Verification instead:
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/` → exit 0.
    - `go test ./pkg/runner` → `ok` (no behavior changed; R11).
    - `rg -n "ToolHook" pkg/runner/options.go pkg/runner/hook.go` → the field and the type.
    - `rg -n "run goroutine|sub_agents" pkg/runner/hook.go pkg/runner/options.go` → at least one line each.
    - The keyed-literal compatibility of existing `Options{...}` literals is proven by `go build ./...` and `go vet ./pkg/runner/`; no separate grep is required.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 2: Red tests (serial, test-only; every declared test must fail or be marked as a guard before Phase 3)

Test file rules: the non-MCP tests go in `pkg/runner/run_test.go` (OD4), and the tests use the existing `setupTestEnv` (`pkg/runner/run_test.go:20-38`) and `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), with `t.Setenv` in `setupTestEnv`, so they are not `t.Parallel`. Each test sets `Options.Workdir` to a `t.TempDir()` when it writes files, and sets the agent `tools` list explicitly. The mock server's `Requests` slice is inspected for the tool result sent back to the model. The MCP tests go in the new file `pkg/runner/hook_mcp_test.go` (OD4).

- [ ] **T2.1: Red: acceptance tests 1 (guard) and 2 (deny builtin `write_file`) (R4, R11, R13)**
  - Budget: 30 min
  - Depends on: T1.
  - File: `pkg/runner/run_test.go`. Add two functions at the end of the file:
    - `TestToolHookNilHookUnchanged` (acceptance test 1, guard). Model: one `list_directory` call on `.`, then text. No `ToolHook` set. Create the workdir with `t.TempDir()` holding one file `a.txt` and one empty directory `sub/`, and set `Options.Workdir` to it. Assert `Content`, `ToolCalls == 1`, and that the tool result sent in the second request has `IsError: false` and content exactly `a.txt\nsub/`. The expected content follows `internal/tool/list_directory.go:78` (`os.ReadDir`, sorted by name) and `:95-104` (names joined by `\n`, directories suffixed `/`). This test is a regression guard: it passes at HEAD and after T3.x. It is recorded as a guard, not a red test.
    - `TestToolHookDenyWriteFile` (acceptance test 2). Model: one `write_file` call with `path` = `out.txt` and `content` = `hi`, then text. `ToolHook` returns `ToolDecision{Action: ToolDeny}` with empty `Content`. Assert: `filepath.Join(workdir, "out.txt")` does not exist (`os.Stat` returns `ErrNotExist`); the tool result in the second request has `IsError: true` and content `tool call denied by host policy`; `Run` returns no error. Use `tools = ["write_file"]` in the agent TOML and `Options.Workdir` = `t.TempDir()`. Add a second table case where the hook returns `ToolDecision{Action: ToolDeny, Content: "no writes"}` and asserts the content is `no writes`.
  - Red step: run `go test ./pkg/runner`; the new test `TestToolHookDenyWriteFile` fails (the file is created because the hook is never called; the tool result is not an error). Record the failing assertion text.
  - Guard: the guard test `TestToolHookNilHookUnchanged` passes in the same run now and must still pass after T3.x.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above; the race run is required once T3.1 is green.

- [ ] **T2.2: Red: acceptance tests 6 (arguments as parsed, copy) and 13 (unknown action) (R5, R8, R13)**
  - Budget: 30 min
  - Depends on: T2.1.
  - File: `pkg/runner/run_test.go`.
    - `TestToolHookArgumentsAsParsed` (acceptance test 6). Model: one `write_file` call with two string arguments (`path` = `a.txt`, `content` = `one`), then text. The hook records the received `call.Arguments` into a local map (copy) and then mutates the received map (`call.Arguments["path"] = "evil.txt"`) and returns `ToolDecision{Action: ToolAllow}`. Assert: the hook's recorded arguments equal the parsed values (`path` = `a.txt`, `content` = `one`); the file `a.txt` exists in `Options.Workdir` with content `one`; `evil.txt` does not exist.
    - `TestToolHookUnknownAction` (acceptance test 13). Table with two cases on a `write_file` call: case `empty action` returns `ToolDecision{Action: ToolAction("")}` with empty `Content`; case `bogus action` returns `ToolDecision{Action: ToolAction("bogus"), Content: "host text"}`. Assert in both cases: the file is not created; the tool result has `IsError: true`; the tool result content is exactly `tool call denied by host policy`; `Run` returns no error. In the `bogus action` case, the content `host text` must not appear in the second request body.
  - Red step: run `go test ./pkg/runner`; the new tests `TestToolHookArgumentsAsParsed` and `TestToolHookUnknownAction` fail (the hook is never called, so there are no recorded arguments and the file is created). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T2.3: Red: acceptance tests 8 (hook error) and 9 (hook panic) (R7, R13)**
  - Budget: 30 min
  - Depends on: T2.2.
  - File: `pkg/runner/run_test.go`.
    - `TestToolHookErrorDenies` (acceptance test 8). Model: one `list_directory` call on `.`, then text. The hook returns `ToolDecision{}` and `errors.New("secret-hook-detail-123")`. Assert: the tool result in the second request has `IsError: true` and content `tool call denied by host policy`; the string `secret-hook-detail-123` is absent from the second request body (`mock.Requests[1].Body`), from `result.Content`, and from `result.ToolCallDetails[0].Output` when `Options.JSON` is true (add a sub-case with `JSON: true`). `Run` returns no error.
    - `TestToolHookPanicDenies` (acceptance test 9). Model: one `list_directory` call, then text. The hook panics with `panic("secret-panic-456")`. Assert: `Run` returns a non-nil `Result` and nil error; the tool result is `IsError: true` with content `tool call denied by host policy`; the string `secret-panic-456` is absent from the second request body and from `result.Content`.
  - Red step: run `go test ./pkg/runner`; the new tests `TestToolHookErrorDenies` and `TestToolHookPanicDenies` fail (the hook is never called, so the list directory runs and returns normal content). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T2.4: Red: acceptance test 7, `TestToolHookParallelTurn` (R2, R13)**
  - Budget: 30 min
  - Depends on: T2.3.
  - File: `pkg/runner/run_test.go`. Model on `TestRun_ParallelToolExecution` (`pkg/runner/run_test.go:431-476`), with `Options.Workdir` = `t.TempDir()`.
    - Mock: one response with three `write_file` calls in that one turn (IDs `tool_1`, `tool_2`, `tool_3`; paths `a.txt`, `b.txt`, `c.txt`; content `x`), then text.
    - Agent: `tools = ["write_file"]`, parallel default (true).
    - Hook: appends the call ID to a mutex-protected log. On each invocation it asserts (with `t.Errorf`) that none of `a.txt`, `b.txt`, `c.txt` exists in the workdir. It returns `ToolDecision{Action: ToolAllow}` for every call. All three calls are allowed.
  - Assertions: the hook log is `tool_1`, `tool_2`, `tool_3` in model order; each hook invocation found none of the three files present; after the run all three files exist with content `x`; the run result has `ToolCalls == 3` and no error.
  - Observation granularity: a dispatch that finished before a later hook call would have created its file, so the in-hook file check detects it. A dispatch that has started but not yet created its file is not detected by this test. The structural guarantee is the decision loop completing before the parallel branch (`pkg/runner/run.go:945`, T3.1).
  - Red step: run `go test -race ./pkg/runner`; the new test `TestToolHookParallelTurn` fails (the hook is never called, so the hook log is empty and the in-hook file checks never run). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T2.5: Red: acceptance tests 3 (deny MCP), 4 (stub MCP), and 5 (allow MCP, guard) with the MCP helper (R3, R4, R5, R6, R13)**
  - Budget: 30 min
  - Depends on: T2.4 is not required (independent file); run after T2.3.
  - Files: new `pkg/runner/hook_mcp_test.go` (package `runner`). It defines a local helper `newHookTestMCPServer(t *testing.T) (*httptest.Server, *atomic.Int64)` modeled on `cmd/run_mcp_integration_test.go:17-46`: `mcp.NewServer(&mcp.Implementation{Name: "hook-test", Version: "1.0.0"}, nil)`, one tool named `mcp_echo` with the same input schema, a handler that increments the atomic counter and returns `hello <name>`, `mcp.NewStreamableHTTPHandler(...)` served by `httptest.NewServer`, closed by `t.Cleanup`. The helper is local to this file; do not import from `cmd`.
  - Tests in `hook_mcp_test.go`:
    - `TestToolHookMCPDeny` (acceptance test 3). Agent TOML with `[[mcp_servers]]`, `name = "local-mcp"`, `url = <server URL>`, `transport = "streamable-http"` (as `cmd/run_mcp_integration_test.go:65-69`). Model: one `mcp_echo` call with `name` = `axe`, then text. Hook denies (`ToolDeny`, empty content). Assert: the counter is 0; the second request carries `IsError: true` and content `tool call denied by host policy`. Use `setupTestEnv` (same package) and `AXE_ANTHROPIC_BASE_URL` set to the mock URL, plus `api_key` in `config.toml`.
    - `TestToolHookMCPStub` (acceptance test 4). Same setup. Hook returns `ToolDecision{Action: ToolStub, Content: "stub text"}`. Assert: counter 0; second request carries content `stub text` and `IsError: false`.
    - `TestToolHookMCPAllow` (acceptance test 5, guard). Same setup. Hook returns `ToolDecision{Action: ToolAllow, Content: "ignored"}`. Assert: counter 1; the tool result content contains `hello axe`; `ignored` is absent from the second request body; `IsError: false`. This is a guard; it passes at HEAD-with-field because the hook is ignored.
  - Red step: run `go test ./pkg/runner`; the new tests `TestToolHookMCPDeny` and `TestToolHookMCPStub` fail (counter is 1; no stub text). Guard: `TestToolHookMCPAllow` passes in the same run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T2.6: Red: acceptance test 10, sub-agents with a hook (R10, R13)**
  - Budget: 30 min
  - Depends on: T2.5.
  - File: `pkg/runner/run_test.go`. `TestToolHookSubAgentConfigError`. Agent TOML: `name = "parent-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, `sub_agents = ["child-agent"]`. Write `config.toml` with `[providers.anthropic] api_key = "fake-key"` so that, without the fix, the run would reach the provider (otherwise the API-key `ConfigError` at `pkg/runner/run.go:350` would pass for the wrong reason). Set `AXE_ANTHROPIC_BASE_URL` to a mock with one text response. `ToolHook` is a closure that returns `ToolDecision{Action: ToolAllow}`.
  - Assert: `err != nil`; `IsConfigError(err)` is true (`pkg/runner/errors.go:57-60`); `mock.RequestCount() == 0`.
  - Red step: run `go test ./pkg/runner`; the new test `TestToolHookSubAgentConfigError` fails (no ConfigError with the field present; the request reaches the mock, so `RequestCount` is 1). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T2.7: Red: acceptance test 12, cancel during decision (R9, R13)**
  - Budget: 30 min
  - Depends on: T2.6.
  - File: `pkg/runner/run_test.go`. `TestToolHookCancelDuringDecision`. Model: one response with two `write_file` calls (`tool_1` writes `a.txt`, `tool_2` writes `b.txt`), then text. Create `ctx, cancel := context.WithCancel(context.Background())` in the test and pass `ctx` to `Run`. The hook records each call ID in a local slice; on the first call (`tool_1`) it calls `cancel()` and returns `ToolDecision{Action: ToolAllow}`; on the second call it records and fails the test if reached.
  - Assert: `err != nil`; `IsRuntimeError(err)` is true (`pkg/runner/errors.go:63-66`); `errors.Is(err, context.Canceled)` is true; the hook log has exactly one entry (`tool_1`); neither `a.txt` nor `b.txt` exists in `Options.Workdir`; `Run` returned no `Result`.
  - Red step: run `go test ./pkg/runner`; the new test `TestToolHookCancelDuringDecision` fails (both calls execute and the hook is not called, or the run returns a result). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

Phase 2 red-run check: after T2.1-T2.7, run `go test -v ./pkg/runner` and filter with `rg -- '--- (PASS|FAIL): TestToolHook'`. `TestToolHookDenyWriteFile`, `TestToolHookArgumentsAsParsed`, `TestToolHookUnknownAction`, `TestToolHookErrorDenies`, `TestToolHookPanicDenies`, `TestToolHookParallelTurn`, `TestToolHookMCPDeny`, `TestToolHookMCPStub`, `TestToolHookSubAgentConfigError`, and `TestToolHookCancelDuringDecision` must show `--- FAIL`. The guards `TestToolHookNilHookUnchanged` and `TestToolHookMCPAllow` must show `--- PASS`. Record the output.

### Phase 3: Green implementation (serial, all in `pkg/runner`)

- [ ] **T3.1: Green: decision pass wiring, hook call with recover, error and panic deny (R2, R7)**
  - Budget: 30 min
  - Depends on: T2.1-T2.3 (red), T2.5 (red, MCP helper compiles).
  - Files:
    - `pkg/runner/hook.go`: add unexported `const toolDenyText = "tool call denied by host policy"` (OD1) and unexported `func callToolHook(ctx context.Context, hook ToolHook, call ToolCall) (decision ToolDecision, err error)`. It copies the call's arguments into a fresh map before calling the hook (copy is the T3.3 requirement; implement the copy here so the hook never sees the dispatched map: `args := make(map[string]string, len(call.Arguments))`, then copy each pair). It has a `defer func() { if r := recover(); r != nil { err = errHookPanic } }()` with an unexported sentinel `errHookPanic`. The recover covers only the hook call (R7).
    - `pkg/runner/run.go`: change `executeToolCalls` (`pkg/runner/run.go:890`) to take `hook ToolHook` as a new parameter (before the `results` return) and to return `([]toolExecResult, error)`. Before the branch at `pkg/runner/run.go:927`, add the decision pass: for each call in model order, call `callToolHook`; if `err != nil`, the call is denied (`provider.ToolResult{CallID: tc.ID, Content: toolDenyText, IsError: true}`) and is not dispatched; if `err == nil`, the decision is recorded for T3.2 and T3.3 (in this task, a non-error decision is treated as allow). Only calls with a deny result skip dispatch; the other calls go through the existing sequential or parallel branch.
    - `pkg/runner/run.go:662`: the single call site becomes `results, err := executeToolCalls(ctx, resp.ToolCalls, cfg, ..., hook)` followed by `if err != nil { return nil, err }`. Pass `opts.ToolHook` as the hook argument. Do not wrap the error (cancellation `RuntimeError`, OD5, in T3.4).
  - Caller list (Phase 2 gate): `executeToolCalls` has one caller, `pkg/runner/run.go:662`. Its definition is at `pkg/runner/run.go:890`. No test calls it directly (`rg -n "executeToolCalls\(" --glob '*.go' .` shows only those two lines at HEAD).
  - Green targets: T2.3 tests 8 and 9 pass. T2.4 (test 7) passes. Test 6 still red (T3.3). Tests 2, 3, 4, 10, 12, 13 still red (T3.2-T3.5).
  - Verification:
    - `go test -v ./pkg/runner`; `--- PASS: TestToolHookErrorDenies` and `--- PASS: TestToolHookPanicDenies` appear.
    - `go test ./pkg/runner` → the guards and pre-existing tests pass; the still-red tests fail as expected (record the list).
    - `go vet ./pkg/runner/` → exit 0.
    - `rg -n "executeToolCalls\(" --glob '*.go' .` → exactly the definition and the one call site.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.2: Green: deny result and unknown action (R4, R8)**
  - Budget: 30 min
  - Depends on: T3.1.
  - File: `pkg/runner/run.go`, the decision pass in `executeToolCalls`, and `pkg/runner/hook.go` for the action switch (a small unexported `func decisionAction(d ToolDecision) ToolAction` is acceptable; no exported change).
  - Behavior:
    - `ToolDeny`: not dispatched; result `CallID` = call ID, `IsError: true`, `Content` = `decision.Content` when non-empty, else `toolDenyText` (OD2).
    - Unknown action (any value other than `ToolAllow`, `ToolDeny`, `ToolStub`, including `""`): not dispatched; `IsError: true`; `Content` = `toolDenyText` always; `decision.Content` is not read (OD9, R8).
    - Other calls in the same turn are unaffected (D6, Q2).
  - Green targets: T2.1 test 2 (both table cases), T2.2 test 13 (both cases), and test 3 (`TestToolHookMCPDeny`, deny is type-independent because the deny path runs before the MCP branch).
  - Verification:
    - `go test -v ./pkg/runner`; `--- PASS` appears for `TestToolHookDenyWriteFile`, `TestToolHookUnknownAction`, and `TestToolHookMCPDeny`.
    - `go test ./pkg/runner` → guards pass; tests 6, 10, 12 still red; record.
    - `rg -n "tool call denied by host policy" pkg/runner/` → exactly one constant definition in `pkg/runner/hook.go` (plus test assertions in `run_test.go` and `hook_mcp_test.go`, which are string literals; record the lines and confirm the constant definition is the only non-test match).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.3: Green: stub result and allow ignores content (R5, R6)**
  - Budget: 30 min
  - Depends on: T3.2.
  - File: `pkg/runner/run.go` (decision pass), `pkg/runner/hook.go` if a helper is needed.
  - Behavior:
    - `ToolStub`: not dispatched; result `CallID` = call ID, `Content` = `decision.Content` exactly (empty allowed), `IsError: false` (OD2, D6).
    - `ToolAllow`: the call goes to the existing dispatch (MCP at `pkg/runner/run.go:931`, then the `call_agent` name check at `pkg/runner/run.go:933`, then the builtin registry at `:936`), sequentially or in parallel as before; `Content` is ignored (R6). A decision of `ToolAllow` together with a non-nil error is a deny (already handled in T3.1).
    - The copy of arguments from T3.1 is verified here: the dispatched call's `Arguments` are the parsed map, not the hook's copy (R5, OD3).
  - Green targets: T2.2 test 6 (`TestToolHookArgumentsAsParsed`), T2.5 test 4 (`TestToolHookMCPStub`). Guard test 5 (`TestToolHookMCPAllow`) stays `ok`.
  - Verification:
    - `go test -v ./pkg/runner`; `--- PASS` appears for `TestToolHookArgumentsAsParsed`, `TestToolHookMCPStub`, and `TestToolHookMCPAllow`.
    - `go test ./pkg/runner` → guards pass; tests 10, 12 still red; record.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.4: Green: cancellation during the decision pass (R9)**
  - Budget: 30 min
  - Depends on: T3.3.
  - File: `pkg/runner/run.go` (decision pass; the turn-level check; the error return path in `run`), `pkg/runner/hook.go` if the error construction is shared.
  - Behavior (OD5, D6):
    - After each hook call in the decision pass, check `ctx.Err()`. If non-nil, stop the pass, dispatch nothing in the turn (decided or undecided), make no further hook calls, and return `&RuntimeError{Msg: "tool hook: run canceled", Err: ctx.Err()}` from `executeToolCalls` as the error.
    - Also check `ctx.Err()` once more after the decision pass and before the branch at `pkg/runner/run.go:927`/`:945`, so a cancel that happens after the last hook call also dispatches nothing.
    - The call site at `pkg/runner/run.go:662` (from T3.1) returns this error unwrapped, so `IsRuntimeError` holds (`pkg/runner/errors.go:63-66`) and `errors.Is(err, context.Canceled)` holds through `RuntimeError.Unwrap` (`pkg/runner/errors.go:40-43`). `mapProviderError` is not applied to this error.
    - No change to `totalToolCalls` placement (`pkg/runner/run.go:653` stays before the call).
  - Green targets: T2.7 test 12 (`TestToolHookCancelDuringDecision`).
  - Verification:
    - `go test -v ./pkg/runner`; `--- PASS: TestToolHookCancelDuringDecision` appears.
    - `go test ./pkg/runner` → guards pass; test 10 still red; record.
    - `rg -n "tool hook: run canceled" pkg/runner/` → one line in `pkg/runner/run.go` or `pkg/runner/hook.go`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.5: Green: sub-agents with a hook fail closed (R10)**
  - Budget: 30 min
  - Depends on: T2.6 (red), T1.
  - File: `pkg/runner/run.go`, immediately after the agent load error check at `pkg/runner/run.go:70-73` (the `return nil, &ConfigError{Msg: "failed to load agent", Err: err}` at `:72`) and before the overrides block at `:75`.
  - Behavior: if `opts.ToolHook != nil && len(cfg.SubAgents) > 0`, return `nil, &ConfigError{Msg: "tool hook is not supported with sub_agents"}` (returned unwrapped; `IsConfigError` checks the type directly, `pkg/runner/errors.go:57-60`). This check runs before the `DryRun` return (`pkg/runner/run.go:278-279`), before the API-key check and provider construction (`pkg/runner/run.go:353-360`). No provider call is made (D6, Q3, OD7).
  - Boundary (OD7, R10): the check keys on `len(cfg.SubAgents) > 0`. Do not add other checks.
  - Green targets: T2.6 test 10 (`TestToolHookSubAgentConfigError`).
  - Verification:
    - `go test -v ./pkg/runner`; `--- PASS: TestToolHookSubAgentConfigError` appears.
    - `go test ./pkg/runner` → guards pass; record the full result (all red tests should now be green).
    - `rg -n "tool hook is not supported" pkg/runner/run.go` → one line.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 4: Full package verification (serial, after Phase 3)

- [ ] **T4: Acceptance tests 1-10, 12, 13 green in one run; R11 nil-hook check (R11, R13)**
  - Budget: 30 min
  - Depends on: T3.1-T3.5.
  - Files: none (verification only).
  - Commands:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolHook'` → every `TestToolHook*` line is `PASS` (acceptance tests 1-10, 12, 13; test 1 and test 5 are guards; test 11 is the race command in T5). Record the list.
    - `go test ./pkg/runner` → `ok` (R11; all pre-existing tests pass unchanged).
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T5: Acceptance test 7, parallel turn under the race detector (R2, R13)**
  - Budget: 30 min
  - Depends on: T2.4, T3.1, T4.
  - Command: `go test -race -v ./pkg/runner`; `--- PASS: TestToolHookParallelTurn` appears, the exit code is 0, and the output has no `DATA RACE` lines.
  - Pass condition: exit 0; the hook log is `tool_1`, `tool_2`, `tool_3`; each hook invocation saw none of `a.txt`, `b.txt`, `c.txt`; all three files exist after the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 5: Lint, API gate, scope, and records checks (serial, last)

- [ ] **T6: `golangci-lint run` (lint gate, R11 and repository lint)**
  - Budget: 30 min
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0` with no findings. Run as a single call; a timeout is not a pass, so report it.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; it runs the lint on `./pkg/runner` changes from T1-T5.
  - No file changes. Any finding in the new files (`pkg/runner/hook.go`, `pkg/runner/hook_mcp_test.go`) or in the changed regions of `pkg/runner/run.go` and `pkg/runner/run_test.go` is a failure of the task that introduced it; fix it in that task's file, then re-run T4 and T6.

- [ ] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R1, D1)**
  - Budget: 30 min
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m004-apidiff-out.txt 2>/tmp/axe-m004-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m004-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0` and treats any `-incompatible` report as failure; additions do not fail it (`docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md` probe evidence).
  - Cleanup check: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache for `go run` of the pinned apidiff. If it fails for network reasons, report it; do not change the pin.
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.ToolHook` (R14; `CONTRIBUTING.md:185`)**
  - Budget: 15 min
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]`, `### Added`, append one bullet after the last existing bullet of that section: "- `Options.ToolHook` in `pkg/runner` lets a host allow, deny, or stub each tool call before dispatch. A denied call is not run and returns an error result. A hook with a non-empty `sub_agents` list is a `ConfigError`." Form follows the milestone 002 bullet (`git show 9f1782f -- CHANGELOG.md`). No other section or line changes.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`.
  - Per-task gate: no Go file changes; `git diff --name-only -- '*.go'` is unchanged by this task, so no race run is needed.

- [ ] **T8: Scope, module, and API checks (R14, D1, R12)**
  - Budget: 30 min
  - Depends on: T7.1.
  - Files: none (verification only).
  - Commands and expected results:
    - `git diff --name-only 09d100d -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_*'` → exactly `CHANGELOG.md`, `pkg/runner/hook.go`, `pkg/runner/hook_mcp_test.go`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`. (Untracked new files appear only with `git status --porcelain`; check both.)
    - `git status --porcelain` → those six paths plus any Factory records; nothing else.
    - `git diff --exit-code 09d100d -- go.mod go.sum cmd internal cmd/testdata` → exit 0.
    - `rg -n "ToolHook" pkg/runner/options.go pkg/runner/hook.go` → the field and the type.
    - `rg -n "tool call denied by host policy" pkg/runner/` → one constant definition (in `pkg/runner/hook.go`); the other matches are test assertions.
    - `rg -n "run goroutine|fixed|sub_agents" pkg/runner/hook.go pkg/runner/options.go` → each claim present (R12).
    - `git diff 09d100d -- pkg/runner/run.go | rg -n '^-' | rg -v '^[0-9]+:---'` → review: removed lines are only the `executeToolCalls` signature, the `executeToolCalls` call at `pkg/runner/run.go:662`, and the `return results` line. Any other removed line is a failure; report it.
    - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_004_implement.md docs/plans/052_runner_tool_hooks_v1_12_0_004_spec.md` → exit 0.
    - `~/.agents/scripts/docs-lint --since 09d100d /home/jaron/dev/axe/runner-tool-hooks` → exit 0.
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T9: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_004_checkpoint.md` and the milestone checkbox `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:168` (`[ ] 004` becomes `[x] 004`). Do not change the milestone entry beyond the checkbox.
  - Owner: orchestrator after T8.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 public types and `Options.ToolHook` field | T1 |
| R2 decision pass before any dispatch, model order, in `executeToolCalls` | T2.4 (red), T3.1, T5 (race) |
| R3 allowed calls use existing dispatch paths | T3.3, T2.5 (test 5 guard) |
| R4 deny is an error result and dispatches nothing | T2.1 (red), T3.2 |
| R5 stub result; hook receives a copy of arguments | T2.2 (red, test 6), T3.1 (copy), T3.3 |
| R6 allow ignores content | T2.5 (test 5 guard), T3.3 |
| R7 hook error or panic is fixed-text deny; recover only in hook call | T2.3 (red), T3.1 |
| R8 unknown action is deny with fixed text; content ignored | T2.2 (red, test 13), T3.2 |
| R9 cancellation dispatches nothing; returns context error | T2.7 (red), T3.4 |
| R10 sub-agents with a hook fail closed (`ConfigError`, unwrapped) | T2.6 (red), T3.5 |
| R11 nil hook byte-identical; existing tests pass | T2.1 (guard 1), T2.5 (guard 5), T4, T8 |
| R12 doc comments state the contract | T1, T8 (rg checks) |
| R13 acceptance tests 1-13 | Table below |
| R14 scope boundary: five Go/test files plus `CHANGELOG.md` `[Unreleased]` bullet | T7.1, T8 |

## Acceptance Test Mapping (R13, one task per test)

| Acceptance test | Test function | Red task | Green task | Note |
|---|---|---|---|---|
| 1 No hook | `TestToolHookNilHookUnchanged` | T2.1 | T3.x (none; guard) | Guard: passes before and after. |
| 2 Deny builtin `write_file` | `TestToolHookDenyWriteFile` | T2.1 | T3.2 | |
| 3 Deny MCP tool | `TestToolHookMCPDeny` | T2.5 | T3.2 | `hook_mcp_test.go`. |
| 4 Stub MCP tool | `TestToolHookMCPStub` | T2.5 | T3.3 | `hook_mcp_test.go`. |
| 5 Allow MCP tool (content ignored) | `TestToolHookMCPAllow` | T2.5 | T3.3 | Guard: passes before and after. |
| 6 Hook receives arguments as parsed | `TestToolHookArgumentsAsParsed` | T2.2 | T3.1 (copy), T3.3 | |
| 7 Parallel turn, three `write_file` calls | `TestToolHookParallelTurn` | T2.4 | T3.1 | Race run in T5. |
| 8 Hook returns an error | `TestToolHookErrorDenies` | T2.3 | T3.1 | |
| 9 Hook panics | `TestToolHookPanicDenies` | T2.3 | T3.1 | |
| 10 Sub-agents with hook | `TestToolHookSubAgentConfigError` | T2.6 | T3.5 | |
| 11 Focused race test (command) | race run of `TestToolHookParallelTurn` (see T5) | (after T2.4) | T5 | Command, not a test function. |
| 12 Cancel during decision | `TestToolHookCancelDuringDecision` | T2.7 | T3.4 | |
| 13 Unknown action | `TestToolHookUnknownAction` | T2.2 | T3.2 | Table: `""` and `"bogus"`. |

Every acceptance test 1-13 maps to exactly one task of each kind above. The spec's R13 test 11 is a command; it is covered by T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures:
  - `executeToolCalls` (`pkg/runner/run.go:890`): gains a `hook ToolHook` parameter and an `error` return. Callers: `pkg/runner/run.go:662` only (`rg -n "executeToolCalls\(" --glob '*.go' .` at HEAD `3f7f7d0` returns lines 662 and 890). Caller update assigned to T3.1.
  - `run` (`pkg/runner/run.go:47`) and `Run` (`pkg/runner/run.go:43`): unchanged signatures.
  - `Options` (`pkg/runner/options.go:6`): gains one field. Existing `Options{...}` literals in `cmd/run.go:80` and `pkg/runner/*_test.go` are keyed. Compile check in T1 (`go build ./...`, `go vet ./pkg/runner/`) covers them.
- Extreme acceptance inputs and recorded evidence:
  - Empty action `""` and unknown action `"bogus"` with non-empty `Content`: covered by T2.2 (test 13). Expected: fixed text, `IsError: true`, host content absent. Evidence: spec OD9; not run yet (red).
  - Empty `Content` on deny: covered by T2.1 (default case). Expected: fixed text. Not run yet.
  - Hook error text and panic value, each with a distinctive string (`secret-hook-detail-123`, `secret-panic-456`): covered by T2.3. Expected: absent from results and messages. Not run yet.
  - Empty argument map: not a separate acceptance input; `call.Arguments` for a no-argument call is copied as an empty map by T3.1. Expected: no panic, hook receives an empty map. Not run yet; the copy loop over a nil map is a no-op.
  - Long strings: the deny text is a fixed constant, so the length of the hook error or content does not change the result (test 8 and test 13 cover the content path). No separate probe.
  - Read-only evidence at HEAD: `go test ./pkg/runner` passes (T0); the declared test names do not exist (T0 `go test -v` filter).
- `go install` or `GOPROXY=` commands: none in this checklist. `go-toolchain-check` pre-flight not triggered.
- Checklist audit: `./.agents/scripts/checklist-test-gate-audit` does not exist in this repository and is not on `~/.agents/scripts/`. Not run.

## Blockers

- None open. Resolved by the orchestrator:
  - Test 7 observes ordering by file creation with no new seam (T2.4; spec R13 test 7).
  - A CHANGELOG `[Unreleased]` Added bullet is required by `CONTRIBUTING.md:185` (T7.1; spec R14).
  - The spec-lint `-run` finding is resolved by keeping every `go test` command in this checklist free of `-run` (Section 1, test-selection convention).

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T2.5 → T2.6 → T2.7 (all test edits in `pkg/runner/run_test.go` or `hook_mcp_test.go`; no two tasks edit the same file at once).
- T2.4 follows T2.3.
- Phase 3 is serial: T3.1 → T3.2 → T3.3 → T3.4 → T3.5 (all edits in `pkg/runner/run.go` and `hook.go`). T3.5 touches a different region of `run.go` but stays serial to keep one file's edits ordered.
- Serial after Phase 3: T4 → T5 → T6 → T7 → T7.1 → T8 → T9.
- Parallel: none in this checklist. The file set is small, and every task edits `pkg/runner`.

## Assumptions

1. The test names in T2.1-T2.7 are chosen by this checklist; the spec does not name tests 1-6 and 8-13. Names are unique (T0 verifies absence).
2. `TestToolHookMCP*` helpers live in `pkg/runner/hook_mcp_test.go` (OD4). The helper is a local copy, not an import from `cmd/`.
3. The copy of arguments (OD3) is performed in `callToolHook` (T3.1), before the first hook call, so every hook receives a fresh map.
4. Test 12 cancels the parent context from inside the hook. The hook receives the derived context from `pkg/runner/run.go:332-333`, which is cancelled when the parent is cancelled.
5. Sub-agent existence is not validated at agent load (`internal/agent/agent.go` validates only `max_depth` and `timeout` among the sub-agent fields, lines 155-162), so test 10 needs no child agent file.
6. `golangci-lint` is on `PATH` (`/home/jaron/.cache/pi-go-bin/golangci-lint`), and the Makefile `lint` target runs `golangci-lint run` (`Makefile:9-10`).
7. The spec's `pkg/runner/run.go:945` for the parallel branch is the `} else {` line; the goroutine starts at `:952`.
8. Test 7 uses file creation as the dispatch-completion signal. A builtin `write_file` writes its file on dispatch, so a completed dispatch before a later hook call is visible to the in-hook file check.

## Limitations

- Test 7 ordering is observed at file-creation granularity (T2.4); the structural guarantee is the decision loop completing before the parallel branch (T3.1).
- The MCP tests use streamable-HTTP only (spec Limitations); `sse` is not covered.
- `make check` (which runs `go test ./...`) and `go test -race ./...` are not run by this checklist. The Factory completion gate runs them.
- The CI `apidiff` job is not exercised here; the local `make apidiff` in T7 is the check.
