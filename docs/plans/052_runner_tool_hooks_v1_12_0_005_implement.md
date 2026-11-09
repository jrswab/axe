# Implement Milestone 005: Give hosts a metadata-only `ToolObserver` for each tool call

## Section 1: Context Summary

Milestone 005 of the v1.12.0 runner tool-hooks release adds `ToolObserver`, a host callback that receives one metadata-only `ToolEvent` per tool call after a turn's calls finish. The event carries the call, the effective action the runner applied, `CallID`, `IsError`, the untruncated content length, the dispatch duration, and the zero-based turn. It never carries result text. The observer runs on the run goroutine, in model order, from the turn loop after `executeToolCalls` returns (OD1), so the parallel branch needs no goroutine handling. A nil observer keeps today's behavior byte for byte. A panic in the observer is recovered and reported as one fixed stderr line with no panic value. Source: `docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md` (Sections 1 and 2, R1-R16, OD1-OD15, acceptance tests 1a-1d, 2-12) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 005, D7, D2, D1). Predecessor 004 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:168`, `[x] 004`); the hook it depends on is in `pkg/runner/hook.go:71-84` and the decision pass (`pkg/runner/run.go:962-993`) inside `executeToolCalls`. Baseline is `c2620fb`, which is an ancestor of HEAD `75f95e0`. The spec's line references were re-verified at HEAD.

Commit is not authorized in this checklist. Do not push. Do not change files outside the list in each task. Do not edit the spec, the checkpoint, or the milestones document; those are orchestrator records. Execution starts from a clean tree apart from orchestrator records; at writing time `docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md` is modified and uncommitted. Do not stage or revert it; stop and report if any non-record file appears.

**Decided (orchestrator, B1 option (a)):** the effective `Action` (OD2) reaches the turn loop through a new unexported `Action ToolAction` field on `toolExecResult` (`pkg/runner/run.go:37-41`). `executeToolCalls` keeps its signature. No blocker remains. See T3.0.

Scope of changed files (spec R16): `pkg/runner/hook.go`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`, and one `[Unreleased]` `Added` bullet in `CHANGELOG.md` (OD15, `CONTRIBUTING.md:185`). Nothing else.

Per-task gate for this milestone: every code task changes only files in `./pkg/runner`, so the derived package list is `./pkg/runner` (derived with the snippet in `/home/jaron/.agents/skills/factory/SKILL.md#per-task-go-gate`). T7.1 changes only `CHANGELOG.md` and has an empty derived package list. Red tasks do not pass the gate by design; their gate is the red run recorded in the task, and the gate runs in the green task that follows.

Test-selection convention: the red tests in this checklist do not exist until their red task, and `spec-lint` rejects a `-run` pattern that selects no current test. Red steps therefore run the package verbosely (`go test -race -v ./pkg/runner`) and read per-test results with `rg -- '--- (PASS|FAIL): <Name>'`. Guard tests and existing tests exist already, so their steps may name the exact test with a `-run` flag once it exists; this checklist does not give such a literal command for a test that is not yet written.

Test names. The spec fixes `TestToolObserver*` as the prefix (OD13) and names only tests 10-12. This checklist names tests 1-9 as follows: 1a-1d are subtests of `TestToolObserverOrderAndArguments`; 2 is `TestToolObserverFullContentLength`; 3 is `TestToolObserverActionDecisions`; 4 is `TestToolObserverPanicLine`; 5 is `TestToolObserverNilUnchanged`; 6 is `TestToolObserverNilHookAllow`; 7 is `TestToolObserverCancelDuringDecision`; 8 is `TestToolObserverDryRunAndBudget`; 9 is the existing `TestGolden` in `cmd/` (no new name). Names are unique (T0 verifies absence).

---

## Section 2: Implementation Checklist

### Phase 0: Pre-flight (serial, read-only)

- [x] **T0: Pre-flight: HEAD ancestry, tree state, baseline package tests, declared names absent**
  - Budget: 30 min
  - Files: none (read-only).
  - Commands and expected results (repository root):
    - `git merge-base --is-ancestor c2620fb HEAD; echo "exit=$?"` → `exit=0`.
    - `git diff --name-only c2620fb HEAD -- . ':(exclude)docs/plans/'` → no output (no non-record file changed since the spec baseline). Verified at writing time.
    - `git status --porcelain` → only orchestrator records under `docs/plans/` (at writing time: ` M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md`). Any other path: stop and report.
    - `go test ./pkg/runner` → `ok` (baseline). Verified at writing time: `ok github.com/jrswab/axe/pkg/runner`.
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver'` → no output; no `TestToolObserver*` test exists yet.
    - `rg -n '\b(ToolObserver|ToolEvent)\b' pkg/ cmd/ internal/` → no output. Verified at writing time. Note: this is the form the run used; the unbounded `ToolObserver|ToolEvent` form also matched the unrelated `pendingToolEvents` in `internal/provider/openrouter.go`.
    - `go test -list 'TestToolHook|TestRun_ParallelToolExecution|TestRun_SubAgent$' ./pkg/runner` → lists the existing 004 tests and the two cited existing tests. Verified at writing time: `TestRun_ParallelToolExecution`, `TestRun_SubAgent`, and the `TestToolHook*` tests are listed.
    - `go test -list '^TestGolden$' ./cmd` → `TestGolden`. Verified at writing time (`ok`). See T4 for the cache note.
  - Go toolchain pre-flight: not triggered. No checklist command runs `go install` or sets `GOPROXY=`. Recorded as "skipped, not triggered".
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: this task changes no files. The derived package list is empty before any edit.

### Phase 1: Enabling surface (serial)

- [x] **T1: Public API surface and doc contract (R1, R13)**
  - Budget: 30 min
  - Depends on: T0.
  - Enables: T2.1-T2.8 (test code must compile against the identifiers) and T3.x (implementation).
  - Files:
    - `pkg/runner/hook.go`: add `ToolEvent` and `ToolObserver` after the existing `ToolHook` type (`pkg/runner/hook.go:46`). Add the `time` import (for `time.Duration`) next to the existing `context` and `errors` imports.
      - `type ToolEvent struct { Call ToolCall; Action ToolAction; CallID string; IsError bool; ContentLength int; Duration time.Duration; Turn int }`. Field types per OD3; `Call` is the public `ToolCall` type (`pkg/runner/messages.go:98-102`) (the struct starts at `:98`).
      - `type ToolObserver func(ctx context.Context, ev ToolEvent)`.
    - `pkg/runner/options.go`: add field `ToolObserver ToolObserver` to `type Options struct` (`pkg/runner/options.go:6`), after the `ToolHook ToolHook` field (`:29-32`) and before the I/O block (`:39`).
  - Doc comments (R13, OD2, OD4, OD5, OD6, OD8, OD12, D7):
    - On `ToolObserver`: called on the run goroutine, sequentially, in model order, after the turn's tool calls finish; concurrent `Run` calls sharing one observer may call it concurrently, so the host must make it safe; it returns nothing; a panic is recovered and reported as the fixed line `Warning: tool observer panicked`; it receives metadata only and never result text.
    - On `ToolEvent`: `Action` is the effective action the runner applied, not the raw hook output; `Duration` is zero for calls that were not dispatched; `Turn` is zero-based; `ContentLength` is the byte length before truncation.
    - On the `Options.ToolObserver` field: ignored when nil; not called for dry runs (`DryRun`); does not change the result.
  - Do not add any behavior. `executeToolCalls`, `run`, and `Run` are unchanged in this task.
  - Failing-first: not applicable (type declarations only). Verification instead:
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/` → exit 0.
    - `go test ./pkg/runner` → `ok` (no behavior changed; R8).
    - `rg -n "ToolEvent|ToolObserver" pkg/runner/hook.go pkg/runner/options.go` → the struct, the function type, and the field.
    - `rg -n "run goroutine|zero-based|untruncated|not called for dry runs|concurrently|fixed" pkg/runner/hook.go pkg/runner/options.go` → each doc claim present.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 2: Red tests (serial, test-only; every declared test must fail or be marked as a guard before Phase 3)

Test file rule: every test goes in `pkg/runner/run_test.go` (OD13); no new test file is created. Tests use `setupTestEnv` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr` (`:44-46`), `capturingProvider` (`:48-50`) or `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`). Tests that use `setupTestEnv` cannot use `t.Parallel`. Each test sets `Options.Workdir` to `t.TempDir()` when it writes files. Model turns use `testutil.AnthropicToolUseResponse` (`internal/testutil/mockserver.go:154`), with `testutil.AnthropicToolUseResponseWithTokens` (`:168`) where a budget is needed. The mock `Requests` slice (`internal/testutil/mockserver.go:34`) holds the request bodies.

Red-run rule: a red test is "red" only if it compiles and fails on an assertion. Because T1 adds the identifiers first, the red tests compile; they fail because no event is delivered.

- [x] **T2.1: Red: acceptance tests 1a-1d, `TestToolObserverOrderAndArguments` (R2, R3, R6, R7)**
  - Budget: 30 min
  - Depends on: T1.
  - File: `pkg/runner/run_test.go`. One parent test `TestToolObserverOrderAndArguments` with four subtests, run in this order. Each subtest uses one turn with three `write_file` calls (`a.txt`, `b.txt`, `c.txt`, content `x`, IDs `tool_1`, `tool_2`, `tool_3`), then a text response, modeled on `TestRun_ParallelToolExecution` (`pkg/runner/run_test.go:435`). Agent `tools = ["write_file"]`.
    - 1a sequential: `[sub_agents_config] parallel = false` (as `pkg/runner/run_test.go:501-502`). The observer appends `CallID`, `Turn`, `IsError`, `Duration`, `Call.Arguments`, `Action` to a slice. Assert events are in model order (`tool_1`, `tool_2`, `tool_3`), every `Turn` is `0`, every `Action` is `ToolAllow`, and `len(events) == 3`.
    - 1b parallel: default parallel. Assert the same as 1a. The observer also asserts, with `t.Errorf`, that all three target files exist when it is called (proves the call is after the join, R2).
    - 1c arguments: the observer records `Call.Arguments`. Assert each equals the parsed arguments of the model's call (`path` and `content` keys with the same values).
    - 1d mutation: the observer sets an extra key (`"injected"`) in each event's `Call.Arguments`. Assert the second request's assistant `tool_use` input for each call equals the original parsed arguments (no `injected` key). The second request body is `mock.Requests[1].Body`.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverOrderAndArguments` and its subtests (the slice is empty, so 1a-1c fail on length; 1b file-existence checks never run). Record the failing assertions. The subtest 1d assertion on the request body is evaluated against the unmodified history, so it may pass; record its result either way.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above; the race run is required once T3.5 is green.

- [x] **T2.2: Red: acceptance test 2 `TestToolObserverFullContentLength` and guard test 5 `TestToolObserverNilUnchanged` (R5, R8)**
  - Budget: 30 min
  - Depends on: T2.1.
  - File: `pkg/runner/run_test.go`.
    - `TestToolObserverFullContentLength` (test 2, red). Workdir holds `big.txt` of 4096 bytes (`strings.Repeat("a", 4096)`). Model calls `read_file` with `path` = `big.txt` (`tools = ["read_file"]`), then text. Observer set, `JSON` off. Assert: one event; `ContentLength > 1024`; `ContentLength` equals `len(content)` of the `tool` message `ToolResults[0].Content` in the second request. Use `mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicToolUseResponse("", []testutil.MockToolCall{{ID: "tool_1", Name: "read_file", Input: map[string]string{"path": "big.txt"}}}), testutil.AnthropicResponse("Done")})` (`internal/testutil/mockserver.go:50`, `:154`, `:141`), set `AXE_ANTHROPIC_BASE_URL` to `mock.URL()` (as `pkg/runner/run_test.go:264`), and decode with `hookTestToolResult` (`pkg/runner/run_test.go:1465`), which reads `mock.Requests[1].Body` (`internal/testutil/mockserver.go:27`, `:34`). Do not use `capturingProvider`: its `Send` returns text only (`pkg/runner/run_test.go:52-54`), so no tool runs and `requests[1]` never exists (`pkg/runner/run.go:646-649`); the event has no content field (R7, checked by `rg`); `result.ToolCallDetails` is `nil` (`pkg/runner/run.go:782-784`).
    - `TestToolObserverNilUnchanged` (test 5, guard). No observer, no hook. One `list_directory` call on `.` in a workdir with `a.txt` and an empty directory `sub/`, then text. Assert `Result.Content`, `Result.ToolCalls == 1`, the second request's tool result content is exactly `a.txt\nsub/` with `IsError: false`, and `Result.ToolCallDetails` is `nil`. This test passes at HEAD and after T3.x.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverFullContentLength` (no event, so the `ContentLength` assertion fails). Guard: `TestToolObserverNilUnchanged` (defined in this task) is run with `go test -race -v ./pkg/runner` and filtered with `rg -- '--- (PASS|FAIL): TestToolObserverNilUnchanged'`. It passes now and must still pass after T3.5.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.3: Red: acceptance test 3 `TestToolObserverActionDecisions` (deny, stub, error, panic, bogus, allow) (R3, R4, R6, R7)**
  - Budget: 30 min
  - Depends on: T2.2.
  - File: `pkg/runner/run_test.go`. Table test `TestToolObserverActionDecisions`, one subtest per case. Each case: one `write_file` call (`path` `out.txt`, content `hi`), then text, served by `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`) with `AnthropicToolUseResponse` then `AnthropicResponse` (see T2.2; `capturingProvider` cannot run a tool); `tools = ["write_file"]`; observer set; hook set per case.
    - (a) hook `ToolDecision{Action: ToolDeny}`, empty content. Expect `Action` `ToolDeny`, `IsError` `true`, `Duration` `0`, `ContentLength` = `len("tool call denied by host policy")`.
    - (b) hook `ToolDecision{Action: ToolStub, Content: "stub text"}`. Expect `Action` `ToolStub`, `IsError` `false`, `Duration` `0`, `ContentLength` = `len("stub text")`.
    - (c) hook returns `ToolDecision{}` and `errors.New("secret-hook-detail-123")`. Expect `Action` `ToolDeny`, `IsError` `true`, `Duration` `0`, `ContentLength` = `len("tool call denied by host policy")`.
    - (d) hook returns `ToolDecision{Action: ToolAction("bogus"), Content: "host text"}`. Expect `Action` `ToolDeny`, `IsError` `true`, `Duration` `0`, `ContentLength` = `len("tool call denied by host policy")`.
    - (e) hook panics with `panic("secret-panic-456")`. Expect `Action` `ToolDeny`, `IsError` `true`, `Duration` `0`, `ContentLength` = `len("tool call denied by host policy")`.
    - (f) hook returns `ToolDecision{Action: ToolAllow}`. Expect `Action` `ToolAllow`, `IsError` `false`, `ContentLength` = length of the `tool` message content in the second request (the real write result), decoded with `hookTestToolResult` (`pkg/runner/run_test.go:1465`) from `mock.Requests[1].Body`, and `Duration` is non-negative and the file `out.txt` exists with content `hi`.
    - For every case: `Turn` `0`; `CallID` and `Call.ID` `tool_1`; the strings `secret-hook-detail-123`, `secret-panic-456`, and `host text` are absent from every event field (`Call`, `Action`, `CallID`, `IsError`, `ContentLength`, `Duration`, `Turn`) and from the observer's recorded fields; `Run` returns no error. The file `out.txt` is absent in cases (a)-(e).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverActionDecisions` in every subtest (no event). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.4: Red: acceptance tests 4 `TestToolObserverPanicLine` and 6 `TestToolObserverNilHookAllow` (R9, R12)**
  - Budget: 30 min
  - Depends on: T2.3.
  - File: `pkg/runner/run_test.go`.
    - `TestToolObserverPanicLine` (test 4, red). One turn with two `list_directory` calls (`tool_1`, `tool_2`, path `.`), then text. The observer panics with `panic("secret-observer-789")` on its first event and counts calls. `Options.Stderr` is the buffer from `newMockStdoutStderr`. Assert: `err` is `nil`; `Result` is non-nil; the observer was called twice (delivery continues, R9); the stderr buffer, split on `\n`, contains exactly one line equal to `Warning: tool observer panicked` (plus the trailing newline); the stderr buffer does not contain `secret-observer-789`, `tool_1`, `list_directory`, or the path. The tool results are unchanged (`IsError` `false` for both).
    - `TestToolObserverNilHookAllow` (test 6, red). Observer set, `ToolHook` nil, one `read_file` call on a small file, then text. Assert the one event has `Action` `ToolAllow` (R12) and `IsError` `false`.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverPanicLine` and `--- FAIL: TestToolObserverNilHookAllow` (no observer call, no warning line, no event). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.5: Red guards: acceptance tests 7 `TestToolObserverCancelDuringDecision` and 8 `TestToolObserverDryRunAndBudget` (R11)**
  - Budget: 30 min
  - Depends on: T2.4.
  - File: `pkg/runner/run_test.go`.
    - `TestToolObserverCancelDuringDecision` (test 7, guard). Copy the scenario of `TestToolHookCancelDuringDecision` (`pkg/runner/run_test.go:2621-2676`) with an observer set in addition to the hook. The hook cancels on `tool_1` and allows; `tool_2` fails the test if reached. Assert: `err` is a `RuntimeError` (`IsRuntimeError`), `errors.Is(err, context.Canceled)`, `Result` is nil, and the observer was called zero times.
    - `TestToolObserverDryRunAndBudget` (test 8, guard). Table with two cases, each with an observer set, zero events asserted:
      - `dry run`: `Options.DryRun` true, the mock is not called; `Run` returns a non-nil result with the dry-run output (`pkg/runner/run.go:283-334`); assert zero events and `mock.RequestCount() == 0`.
      - `budget exceeded before dispatch`: agent `[budget] max_tokens = 10`; model response `testutil.AnthropicToolUseResponseWithTokens("Over", []testutil.MockToolCall{{ID: "tool_1", Name: "list_directory", Input: {"path": "."}}}, 8, 7)` (input plus output exceed 10); the budget check at `pkg/runner/run.go:652-654` breaks before dispatch. Assert zero events, `Result.Budget.Exceeded` is true, and the tool was not run.
  - Red step: these are guards, not red tests. Their observer-side assertions pass at HEAD because no event is emitted. Run `go test -race -v ./pkg/runner` and filter with `rg -- '--- (PASS|FAIL): TestToolObserver(CancelDuringDecision|DryRunAndBudget)'` → `PASS` now. They must still pass after T3.x; a wrong call site (e.g. calling the observer before the decision pass or on dry-run) makes them fail. Record the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.6: Red: acceptance test 10 `TestToolObserverDispatchCancel` (R2, R11)**
  - Budget: 30 min
  - Depends on: T2.5.
  - File: `pkg/runner/run_test.go`. Agent `tools = ["run_command"]`, no hook, `Options.Timeout` not set, observer set. Parent context is cancelable (`context.WithCancel`). The model returns two `run_command` calls in one turn (IDs `tool_1` and `tool_2`) with commands `touch m1 && exec sleep 30` and `touch m2 && exec sleep 30`. They are parallel by default. A helper goroutine polls the workdir for `m1` and `m2`, each wait bounded to 10 s. When both exist, it calls `cancel()`. If the bound expires, it still calls `cancel()` and sends a failure to a buffered channel that the test reads after `Run` returns. The helper never calls `t.Fatal`.
    - Assert: no marker-wait failure on the channel; the observer is called exactly twice, in model order (`tool_1`, then `tool_2`); each event has `Action` `ToolAllow`, `Turn` `0`, `IsError` `true`, and `CallID` equal to `Call.ID` and to the model's call ID.
    - Do not assert `Duration` or the run's final error. The `exec` prefix makes `sh` replace itself with `sleep`, so the kill returns promptly (spec test 10 measurement: 30.0 s without `exec`, 0.3 s with it).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverDispatchCancel` (zero events, so the call-count assertion fails). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.7: Red: acceptance test 11 `TestToolObserverDispatchError` (R4, R2)**
  - Budget: 30 min
  - Depends on: T2.6.
  - File: `pkg/runner/run_test.go`. No hook, observer set, `tools = ["write_file"]` (the agent's registry must not contain `no_such_tool`). The model calls `no_such_tool` (ID `tool_1`, input `{}`), then text. The registry returns `unknown tool "no_such_tool"` (`internal/tool/registry.go:85`), which the runner converts to an error result (`pkg/runner/run.go:1008-1009` sequential, `:1036-1037` parallel).
    - Assert one event: `Action` `ToolAllow`, `IsError` `true`, `CallID` and `Call.ID` both `tool_1`, `Call.Name` `no_such_tool`, `ContentLength` equal to the length of the error content in the second request's tool result, `Turn` `0`.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverDispatchError` (no event). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T2.8: Red: acceptance test 12 `TestToolObserverSubAgentNoHook` (R10)**
  - Budget: 30 min
  - Depends on: T2.7.
  - Model on the test `TestRun_SubAgent`: its setup is at `pkg/runner/run_test.go:766`, and the child-agent file is written from `pkg/runner/run_test.go:793`. Parent agent `sub_agents = ["child-agent"]`, no `ToolHook`, observer set. The parent model calls `call_agent` (ID `tool_1`, input `{"agent": "child-agent", "task": "..."}` as in `pkg/runner/run_test.go:772`), the child agent returns `hello from child`, the parent returns final text. Child agent file written to `tmpDir/axe/agents/child-agent.toml` (`pkg/runner/run_test.go:793-802`). The model for the whole test is `TestRun_SubAgent` (`pkg/runner/run_test.go:766-802`).
    - Assert: `err` is `nil`; `Result` is non-nil; exactly one event with `Call.Name` `call_agent`, `CallID` `tool_1`, `Action` `ToolAllow`, `IsError` `false`, `Turn` `0`. This proves `sub_agents` with a nil hook is not a `ConfigError` (OD11, R10), and that the existing `TestToolHookSubAgentConfigError` (`pkg/runner/run_test.go:1858`) does not cover this rule because it sets no observer.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestToolObserverSubAgentNoHook` (no event). Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

Phase 2 red-run check: after T2.1-T2.8, run `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver'`. `TestToolObserverOrderAndArguments`, `TestToolObserverFullContentLength`, `TestToolObserverActionDecisions`, `TestToolObserverPanicLine`, `TestToolObserverNilHookAllow`, `TestToolObserverDispatchCancel`, `TestToolObserverDispatchError`, and `TestToolObserverSubAgentNoHook` must show `--- FAIL`. The guards `TestToolObserverNilUnchanged`, `TestToolObserverCancelDuringDecision`, and `TestToolObserverDryRunAndBudget` must show `--- PASS`. Record the output.

### Phase 3: Green implementation (serial, all in `pkg/runner`)

- [x] **T3.0: Carry the effective `Action` from the decision pass to the turn loop (R3, OD2)**
  - Budget: 30 min
  - Depends on: T1.
  - Status: Ready. The channel is settled (see Decision).
  - The gap: the spec (OD1, OD2) fixes what the observer reports but not how the effective action reaches the turn loop. `executeToolCalls` (`pkg/runner/run.go:925`) returns `([]toolExecResult, error)`, and `toolExecResult` (`pkg/runner/run.go:37-41`) held only `Result` and `Duration`. The decision pass (`pkg/runner/run.go:963-989`) computes the action but nothing returned it. The call site is `pkg/runner/run.go:666-668`, and `rg -n "executeToolCalls\(" --glob '*.go' .` at HEAD `75f95e0` returns lines 665 and 896 (the only caller is `pkg/runner/run.go:666`).
  - Decision (orchestrator, option (a)): add an unexported field `Action ToolAction` to `toolExecResult` (`pkg/runner/run.go:37-41`). `executeToolCalls` keeps its signature. Rejected: a second return value (changes the signature and the caller for no gain), and a parallel slice built in `run` (duplicates the decision logic).
  - Behavior:
    - Decision pass (`pkg/runner/run.go:963-989`): record the effective action for every call it decides, as `results[i] = toolExecResult{Result: r, Action: <effective>}`. Hook error or panic gives `ToolDeny` (`decision = ToolDecision{Action: ToolDeny}`). `ToolDeny` gives `ToolDeny`. `ToolStub` gives `ToolStub`. An unknown action, including `""`, gives `ToolDeny`: the switch has no default, so add `Action: ToolDeny` for it explicitly. `ToolAllow` is not stored; the dispatch sets it.
    - Dispatch branches overwrite whole entries, so both must set `Action: ToolAllow`: the sequential `results[i] = toolExecResult{Result: r, Duration: time.Since(start)}` (`pkg/runner/run.go:1014`) and the parallel `toolExecResult{Result: res, Duration: time.Since(start)}` inside the goroutine (`pkg/runner/run.go:1042`). A nil hook reaches dispatch with no decision pass, so every call gets `ToolAllow`.
  - Proof: tests 3 (`TestToolObserverActionDecisions`) and 6 (`TestToolObserverNilHookAllow`) turn green in T3.2.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T3.1: Green: observer call site, nil check, and event delivery in model order (R2, R5, R6, R7, R8)**
  - Budget: 30 min
  - Depends on: T2.1-T2.8 (red), T1, T3.0.
  - File: `pkg/runner/run.go`, the turn loop immediately after the `executeToolCalls` error check (`pkg/runner/run.go:666-668`) and before the JSON detail loop (`:690`) and tool result message (`:707-711`).
  - Behavior:
    - If `opts.ToolObserver == nil`, no event is built; the only added code is the nil check (R8).
    - Otherwise, for each index `i` in `resp.ToolCalls` in model order, build one `ToolEvent`:
      - `Call`: a fresh `ToolCall{ID, Name, Arguments}` whose `Arguments` is a new map with the same key-value pairs (OD7, R7). Build it per event, so an observer mutation never reaches `resp.ToolCalls` or the history at `pkg/runner/run.go:640-644`.
      - `CallID`: `resp.ToolCalls[i].ID`.
      - `IsError`: `results[i].Result.IsError`.
      - `ContentLength`: `len(results[i].Result.Content)` taken before `truncateOutput` (OD5, R5). `truncateOutput` (`pkg/runner/run.go:1067-1075`) is not called here.
      - `Duration`: `results[i].Duration` (zero for non-dispatched calls, OD6).
      - `Turn`: `turn` (zero-based, OD4).
      - `Action`: `results[i].Action`, set by T3.0.
    - Call the observer with `ctx` (the request context, `pkg/runner/run.go:336`) and the event.
    - No change to `totalToolCalls` (`pkg/runner/run.go:657`) or to the JSON detail or tool message code.
  - Caller list (Phase 2 gate): `executeToolCalls` has one caller (`pkg/runner/run.go:666`); its signature is unchanged in this task. No test calls it directly.
  - Green targets: T2.2 test 2 (`TestToolObserverFullContentLength`), T2.1 subtests 1a-1d (`TestToolObserverOrderAndArguments`) except the `Action` assertions, and T2.7 (`TestToolObserverDispatchError`) except `Action`.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver'` → the targets above pass or fail only on the `Action` assertion (record the list).
    - `go test ./pkg/runner` → `ok` for the pre-existing tests and the guards.
    - `go vet ./pkg/runner/` → exit 0.
    - `rg -n "executeToolCalls\(" --glob '*.go' .` → the definition and the one call site, unchanged.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T3.2: Green: effective `Action` on every event (R3, R12)**
  - Budget: 30 min
  - Depends on: T3.0 (decision recorded and implemented), T3.1.
  - File: `pkg/runner/run.go` only. The `Action` values are set by T3.0; this task verifies them.
  - Behavior (OD2): hook nil gives `ToolAllow`; hook `ToolAllow` gives `ToolAllow`; hook `ToolDeny` gives `ToolDeny`; hook `ToolStub` gives `ToolStub`; hook error, panic, or unknown action (including `""`) gives `ToolDeny`. The raw hook action is never exposed (R3).
  - Green targets: T2.3 `TestToolObserverActionDecisions` (all six subtests), T2.4 `TestToolObserverNilHookAllow`, and the `Action` assertions in T2.1 and T2.6 (`TestToolObserverDispatchCancel` and `TestToolObserverSubAgentNoHook`).
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver(ActionDecisions|NilHookAllow)'` → PASS.
    - `go test ./pkg/runner` → `ok`.
    - `rg -n "bogus|secret-hook-detail-123|secret-panic-456" pkg/runner/run.go pkg/runner/hook.go` → no output (the observer path never copies raw hook data; test strings live only in `run_test.go`).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T3.3: Green: observer panic recovered with one fixed stderr line (R9)**
  - Budget: 30 min
  - Depends on: T3.1.
  - Files: `pkg/runner/run.go` (the observer call from T3.1). Add an unexported helper `callToolObserver(ctx context.Context, observer ToolObserver, ev ToolEvent, stderr io.Writer)` in `pkg/runner/hook.go` next to `callToolHook`, or in `run.go`; pick one and keep it in `pkg/runner`.
  - Behavior (OD8): each observer call is wrapped in its own `defer` with `recover`. On a panic, write exactly `Warning: tool observer panicked\n` to `stderr` (the run's stderr writer, `pkg/runner/run.go:55-57`). The line has no panic value, no tool name, no argument, and no header text; do not use `%v`. Remaining events of the turn still go to the observer. The run continues with its normal result; `err` is nil and tool results are unchanged.
  - Green targets: T2.4 `TestToolObserverPanicLine`.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserverPanicLine'` → PASS.
    - `rg -n "tool observer panicked" pkg/runner/` → the fixed line in `run.go` or `hook.go`, and the assertion in `run_test.go`.
    - `rg -n 'tool observer panicked.*%' pkg/runner/` → no output (no interpolation).
    - `go test ./pkg/runner` → `ok`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T3.4: Green: cancellation and dry run emit no events (R11)**
  - Budget: 30 min
  - Depends on: T3.1.
  - File: `pkg/runner/run.go` only if a change is needed. No change is expected: the decision-pass the `ctx.Err()` check in the decision pass returns before dispatch (`pkg/runner/run.go:990-992`) the call site, so the observer call is never reached; the dry-run return (`pkg/runner/run.go:283-334`) and the budget break (`pkg/runner/run.go:652-654`) precede dispatch. Confirm this by reading the code; if a change is needed, record why and stop.
  - Green targets: none new; this task confirms the guards T2.5 (`TestToolObserverCancelDuringDecision`, `TestToolObserverDryRunAndBudget`) still pass and that T2.6 (`TestToolObserverDispatchCancel`) shows one event per call during dispatch cancellation.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver(CancelDuringDecision|DryRunAndBudget|DispatchCancel)'` → all PASS.
    - `rg -n "tool hook: run canceled" pkg/runner/run.go` → one line, unchanged from 004 (`pkg/runner/run.go:991`).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T3.5: Green: all `TestToolObserver*` tests pass; nil observer unchanged (R8, R10)**
  - Budget: 30 min
  - Depends on: T3.2, T3.3, T3.4.
  - Files: none expected. Fix any failure in the file that introduced it (`pkg/runner/run.go`, `pkg/runner/hook.go`), not in the test.
  - Green targets: T2.1-T2.8 all PASS; T2.2 guard `TestToolObserverNilUnchanged` PASS; T2.8 `TestToolObserverSubAgentNoHook` PASS (proves no `ConfigError` for `sub_agents` with observer only, OD11).
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver'` → every line `PASS`.
    - `go test ./pkg/runner` → `ok`.
    - `rg -n "tool is not supported with sub_agents|tool hook is not supported" pkg/runner/run.go` → the 004 `ConfigError` only for `ToolHook` (`pkg/runner/run.go:75-76`), with no `ToolObserver` check added.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 4: Full package verification (serial, after Phase 3)

- [x] **T4: Acceptance tests 1a-1d, 2-8, 10-12 green in one run; test 9 golden check (R8, R13)**
  - Budget: 30 min
  - Depends on: T3.5.
  - Files: none (verification only).
  - Commands:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestToolObserver'` → every line `PASS`. Record the list.
    - `go test ./pkg/runner` → `ok`.
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/` → exit 0.
    - Test 9 (golden JSON, R8): run the existing golden test from the module root with a temporary cache directory: `XDG_CACHE_HOME=$(mktemp -d) go test ./cmd -count=1`, expected `ok`. The temporary `XDG_CACHE_HOME` is required because `~/.cache/axe` is read-only in this sandbox; without it the command can fail for environmental reasons, not because of this change. `TestGolden` is at `cmd/golden_test.go:205`.
    - `git diff --name-only c2620fb -- cmd/testdata` → empty output (golden files unchanged).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [x] **T5: Race run of the observer tests, including the parallel turn (R2, R13)**
  - Budget: 30 min
  - Depends on: T4.
  - Command: `go test -race -v ./pkg/runner`, filtered with `rg -- '--- (PASS|FAIL): TestToolObserver|DATA RACE'`. Expected: exit 0; `--- PASS` for every `TestToolObserver*` test; no `DATA RACE` lines in the output.
  - Pass condition: exit 0 and no `DATA RACE`. The parallel-order check (T2.1 subtest 1b) and dispatch cancel (T2.6) are the cases most likely to race.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 5: Lint, API gate, CHANGELOG, scope, and records checks (serial, last)

- [x] **T6: `golangci-lint run` (lint gate)**
  - Budget: 30 min
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0`, no findings. Run as one call; a timeout is not a pass, so report it.
  - Any finding in `pkg/runner/hook.go`, `pkg/runner/options.go`, or the changed regions of `pkg/runner/run.go` and `pkg/runner/run_test.go` is a failure of the task that introduced it; fix it in that task's file, then re-run T4 and T6.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; it runs lint on the T1-T5 changes.

- [x] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R1, D1)**
  - Budget: 30 min
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m005-apidiff-out.txt 2>/tmp/axe-m005-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m005-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0`; `ToolEvent`, `ToolObserver`, and `Options.ToolObserver` are additions and must not fail it (`Makefile:14-24`).
  - Cleanup: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache. If it fails for network reasons, report it; do not change the pin.
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [x] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.ToolObserver` (R16, OD15; `CONTRIBUTING.md:185`)**
  - Budget: 15 min
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]` → `### Added` (`CHANGELOG.md:8-10`), append one bullet after the last existing bullet of that section (after the 004 `Options.ToolHook` bullet at `CHANGELOG.md:13`): "- `Options.ToolObserver` in `pkg/runner` receives one metadata-only `ToolEvent` per tool call after each turn's calls finish: the call, the effective action, the call ID, whether it failed, the result length, the dispatch duration, and the turn. It never includes result text. A nil observer changes nothing." Form follows the 004 bullet and the 002 bullet style.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`.
  - Per-task gate: no Go file changes; `git diff --name-only -- '*.go'` is unchanged by this task, so no race run is needed.

- [x] **T8: Scope, module, API, and removed-line checks (R16, D1, R13)**
  - Budget: 30 min
  - Depends on: T7.1.
  - Files: none (verification only).
  - Commands and expected results:
    - `git diff --name-only c2620fb -- . ':(exclude)docs/plans/'` → exactly `CHANGELOG.md`, `pkg/runner/hook.go`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`.
    - `git status --porcelain` → those five paths plus orchestrator records under `docs/plans/`; nothing else. Untracked files appear here; check both views. Note: this run committed per task (a27cdf6, 883402a, a85cf51), so the five-file check was done with `git diff --name-only c2620fb -- . ':(exclude)docs/plans/'`, which returned exactly the five expected files.
    - `git diff --exit-code c2620fb -- go.mod go.sum cmd internal cmd/testdata` → exit 0.
    - `rg -n "ToolObserver|ToolEvent" pkg/runner/hook.go pkg/runner/options.go` → the type, function type, and field.
    - `rg -n "Content " pkg/runner/hook.go` → no `ToolEvent` content field (R7).
    - `rg -n "run goroutine|zero-based|untruncated|not called for dry runs|concurrently|fixed" pkg/runner/hook.go pkg/runner/options.go` → each claim present (R13).
    - `git diff c2620fb -- pkg/runner/run.go | rg -n '^-' | rg -v '^[0-9]+:---'` → review: removed lines are only those the spec permits (the decision-pass assignment `results[i] = toolExecResult{Result: r}` at HEAD `pkg/runner/run.go:988`; the two dispatch assignments (HEAD `pkg/runner/run.go:1014`; HEAD `pkg/runner/run.go:1042`), each re-added with `Action`; the `executeToolCalls` signature is unchanged). Any other removed line is a failure; report it.
    - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_005_implement.md` → exit 0.
    - `~/.agents/scripts/docs-lint --since c2620fb /home/jaron/dev/axe/runner-tool-hooks` → exit 0.
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T9: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md` and the milestone checkbox `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:189` (`[ ] 005` becomes `[x] 005`). Do not change the milestone entry beyond the checkbox.
  - Owner: orchestrator after T8.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 public types and `Options.ToolObserver` field | T1 |
| R2 one event per call, model order, after the turn's calls finish | T2.1, T2.6, T3.1, T5 |
| R3 effective action (OD2) | T3.0, T2.3, T3.2 |
| R4 identity and error fields | T2.3, T2.7, T3.1 |
| R5 untruncated content length (OD5) | T2.2, T3.1 |
| R6 duration and turn (OD4, OD6) | T2.1, T2.3, T3.1 |
| R7 `Call` copy and no result text (OD7) | T2.1 (1c, 1d), T2.2, T3.1 |
| R8 nil observer is byte-identical | T2.2 (guard test 5), T2.1 (1d), T3.1, T4 (golden) |
| R9 panic recovered, fixed line | T2.4, T3.3 |
| R10 observer and hook are independent (OD11) | T2.8, T3.5 |
| R11 cancellation, dry run, budget emit no events | T2.5 (guards 7, 8), T2.6 (test 10), T3.4 |
| R12 nil hook gives `ToolAllow` | T2.4 (test 6), T3.2 |
| R13 doc comments state the contract | T1, T8 (rg checks) |
| R14 acceptance tests | Acceptance Test Mapping table below |
| R15 concurrency and cross-run ordering (documented, not tested) | T1 (doc comment), T8 |
| R16 scope boundary and CHANGELOG bullet (OD15) | T7.1, T8 |

## Acceptance Test Mapping (R14, one task per test)

| Acceptance test | Test function | Red task | Green task | Note |
|---|---|---|---|---|
| 1a sequential order and fields | `TestToolObserverOrderAndArguments/1a` | T2.1 | T3.1, T3.2 | |
| 1b parallel order, post-join | `TestToolObserverOrderAndArguments/1b` | T2.1 | T3.1, T3.2 | Race run in T5. |
| 1c argument equality | `TestToolObserverOrderAndArguments/1c` | T2.1 | T3.1 | |
| 1d argument mutation | `TestToolObserverOrderAndArguments/1d` | T2.1 | T3.1 | Result may pass before T3.1; recorded in T2.1. |
| 2 full content length, JSON off | `TestToolObserverFullContentLength` | T2.2 | T3.1 | |
| 3 deny, stub, error, panic, bogus, allow | `TestToolObserverActionDecisions` (a-f) | T2.3 | T3.2 | Depends on T3.0. |
| 4 observer panic | `TestToolObserverPanicLine` | T2.4 | T3.3 | |
| 5 nil observer unchanged | `TestToolObserverNilUnchanged` | T2.2 | none (guard) | Passes before and after. |
| 6 nil hook gives `ToolAllow` | `TestToolObserverNilHookAllow` | T2.4 | T3.2 | Depends on T3.0. |
| 7 cancellation during decision | `TestToolObserverCancelDuringDecision` | T2.5 | none (guard) | Must stay PASS after T3.4. |
| 8 dry run and budget | `TestToolObserverDryRunAndBudget` | T2.5 | none (guard) | Must stay PASS after T3.4. |
| 9 golden JSON unchanged | existing `TestGolden` (`cmd/golden_test.go:205`) | none (regression) | T4 | Run with `XDG_CACHE_HOME` set to a temp directory. |
| 10 cancellation during dispatch | `TestToolObserverDispatchCancel` | T2.6 | T3.4 | |
| 11 dispatch error, no hook | `TestToolObserverDispatchError` | T2.7 | T3.1, T3.2 | Depends on T3.0 for `Action`. |
| 12 sub-agent with no hook | `TestToolObserverSubAgentNoHook` | T2.8 | T3.5 | |

Every acceptance test 1a-1d, 2-12 maps to a red task and a green or guard task. Test 9 is a regression check in T4. The spec's focused race check is T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures:
  - `executeToolCalls` (`pkg/runner/run.go:925`): signature unchanged in T3.0 and T3.1 (option (a)). Caller: `pkg/runner/run.go:666` only (`rg -n "executeToolCalls\(" --glob '*.go' .` at HEAD `75f95e0` returns lines 665 and 896; verified at writing time). No caller change.
  - `run` (`pkg/runner/run.go:48`) and `Run` (`pkg/runner/run.go:44`): unchanged signatures.
  - `Options` (`pkg/runner/options.go:6`): gains one field. Existing `Options{...}` literals are keyed (`cmd/run.go:80`, `pkg/runner/*_test.go`). Compile check in T1 covers them.
  - `ToolEvent` and `ToolObserver`: new identifiers, no callers.
- Extreme acceptance inputs and probe evidence:
  - Empty or zero: a non-dispatched call has `Duration` zero (T2.3 cases a-e; OD6). A nil observer and nil hook (T2.2 test 5). Expected per spec OD6 and R8.
  - Very long content: 4096-byte `read_file` result (T2.2 test 2). Expected `ContentLength` is the full length, not 1024 (OD5). Read-only evidence at writing time: `truncateOutput` caps at `maxToolOutputBytes = 1024` (`pkg/runner/run.go:35`, `:1067-1075`), so the test distinguishes the two.
  - Secret-looking strings: `secret-hook-detail-123`, `secret-panic-456`, `secret-observer-789`, and `host text` (T2.3, T2.4). Expected absent from every observer field and from stderr (OD8).
  - Empty and unknown action (`""`, `"bogus"`): T2.3 case (d); `""` is covered by the 004 test `TestToolHookUnknownAction`. Expected `ToolDeny` (OD2).
  - Dispatch error text from the registry: T2.7. Expected `IsError` `true`, `ContentLength` equal to the error text length.
- Read-only probes run at writing time (no edits): `go test ./pkg/runner` → `ok`; `go test -list` for the existing and cited tests lists them; `rg -n "ToolObserver|ToolEvent" pkg/ cmd/ internal/` → no output; `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md` → `PASS ... 0 findings`, exit 0.
- `go install` or `GOPROXY=` commands: none in this checklist. `go-toolchain-check` pre-flight not triggered.
- `./.agents/scripts/checklist-test-gate-audit`: not present in this repository. Not run.

## Blockers

- **B1 (resolved by orchestrator):** the effective `Action` reaches the turn loop through `toolExecResult.Action` (option (a) in T3.0). Options (b) and (c) were rejected. No open blocker.
- Resolved at writing time, no action needed: the spec's line references were checked against HEAD and match; the spec passes `spec-lint` with 0 findings.

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T2.5 → T2.6 → T2.7 → T2.8 (all test edits in `pkg/runner/run_test.go`; no two tasks edit the same file at once).
- Phase 3 is serial: T3.0 → T3.1 → T3.2 → T3.3 → T3.4 → T3.5, all in `pkg/runner/run.go` and `pkg/runner/hook.go`.
- Serial after Phase 3: T4 → T5 → T6 → T7 → T7.1 → T8 → T9.
- Parallel: T7.1 (CHANGELOG) is independent of the code and may be drafted during Phase 2, but it is committed in order after T7. No other parallel work: every task edits `pkg/runner`.

## Assumptions

1. The test names for tests 1-9 are chosen by this checklist (see Section 1); the spec names only tests 10-12. Names are unique (T0 checks absence).
2. The observer's `Call.Arguments` copy is built per event in the turn loop (T3.1), so the copy is independent of the hook's copy in `callToolHook` (`pkg/runner/hook.go:71-76`).
3. Test 10 uses file creation as the dispatch-start signal (markers `m1`, `m2`), so both calls have started before cancel. This is the spec's design; no new test seam is added.
4. Test 8's budget case relies on `tracker.Add` before the check at `pkg/runner/run.go:652-654` (verified: `tracker.Add` at `:627`, check at `:652`).
5. `call_agent` in test 12 needs no child agent beyond child-agent setup at `pkg/runner/run_test.go:793-802`, as in the `TestRun_SubAgent` model.
6. `golangci-lint` is on `PATH` and `make lint` runs `golangci-lint run` (`Makefile:9-10`).

## Limitations

- The observer is verified in-process through `run()` with `capturingProvider` or `testutil.NewMockLLMServer`. The CLI cannot set an observer, so no CLI test exists.
- Concurrency across `Run` calls that share one observer is documented (R13), not tested (R15).
- `Duration` for dispatched calls excludes hook time (OD6); no timing bound is asserted.
- The fixed panic line is verified by exact match (T2.4); the wording is a decision (OD8), not a stable public contract.
- The full race run over all packages and `make check` are not run by this checklist; the Factory completion gate runs them.
