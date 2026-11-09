# Milestone 005: Give hosts a metadata-only `ToolObserver` for each tool call

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 005 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:189-201`). Scope and acceptance tests 1-7 are taken from that entry as settled by D7 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:69`) and D2 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:55`).
- Predecessors: milestone 004 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:168`, `[x] 004`). The hook it depends on is in place (`pkg/runner/hook.go:14-22`, `pkg/runner/run.go:963-993`).
- Baseline: branch `runner-tool-hooks`, base commit `c2620fbe11fffe5ad5c73e61ba4c0d63f298e6fa`, clean tree at spec time.
- Lane: full. The milestone adds exported identifiers to `pkg/runner`, which is the external Go API contract (D1). It also runs on the parallel tool dispatch path (`pkg/runner/run.go:1016-1049`), so the observer's ordering and concurrency rules must be tested. The fast-lane condition "external contracts untouched" fails.

### Decisions settled (record as settled, never re-evaluate)

- **D7 (milestone).** Ship `ToolObserver` in v1.12.0, metadata only: call, action, `CallID`, `IsError`, content length, duration, turn. No result text. It runs on the run goroutine after a turn's tool calls finish, in model order. No cross-run ordering guarantee. A panic is recovered and reported as one credential-free stderr line. The observer returns nothing. When no `ToolHook` is set, `Action` is `ToolAllow` for every event. No `ToolCallDetail` or `--json` change (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:69`).
- **D7 overrides the draft.** The draft's `ToolEvent.Result ToolResult` (full text) and "do not recover a panic" are replaced (`docs/plans/runner-tool-hooks-and-options.md:221-226`, `:249`). Result text is rejected because nightshift's audit rule forbids raw tool outputs (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:85`).
- **D2 and D1.** The observer is nil by default. The zero value keeps today's behavior. Additions only (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54-55`).
- **D6 (004, in force).** The hook is called on the run goroutine, in model order, before any dispatch of the turn. A hook deny, hook error, hook panic, and unknown action all produce a non-dispatched result (`pkg/runner/run.go:965-989`, `pkg/runner/hook.go:9`). The observer must report the effective action, not the raw hook output (OD2).
- **OD1 (this spec). Observer call point.** The observer is called in `executeToolCalls`' caller, in the turn loop, after `executeToolCalls` returns with no error and before the tool result message is appended (`pkg/runner/run.go:666-710`). It is called once per element of `resp.ToolCalls`, by index, in model order. Rationale: both the sequential and the parallel branch already place results by index (`pkg/runner/run.go:1003`, `:1047`), and the parallel branch joins before it returns (`pkg/runner/run.go:1045-1048`). Calling from the caller therefore satisfies "after the turn's calls finish, in model order" for both branches with no goroutine involvement.
- **OD2 (this spec). Effective `Action`.** `Action` is the action the runner applied:
  - nil `ToolHook`: `ToolAllow` (D7).
  - hook returns `ToolAllow`: `ToolAllow`.
  - hook returns `ToolDeny`: `ToolDeny`.
  - hook returns `ToolStub`: `ToolStub`.
  - hook returns a non-nil error, panics, or returns an unknown action: `ToolDeny`. This follows D6 and OD9 of 004. The raw hook action is not exposed. Evidence: the unknown-action branch falls through the switch with the deny text (`pkg/runner/run.go:974-986`), and the error branch sets `ToolDeny` (`pkg/runner/run.go:970-972`).
- **OD3 (this spec). `ToolEvent` field types.**
  - `Call ToolCall` (`pkg/runner/messages.go:98-102`).
  - `Action ToolAction` (`pkg/runner/hook.go:14`).
  - `CallID string`, equal to `Call.ID`. Evidence: `provider.ToolResult.CallID string` (`internal/provider/provider.go:48-52`).
  - `IsError bool`, the `IsError` of the result the model receives. Evidence: `provider.ToolResult.IsError bool` (`internal/provider/provider.go:48-52`).
  - `ContentLength int`, a byte count. Evidence: `truncateOutput` measures with `len(s)` (`pkg/runner/run.go:1068`), and `ToolCallDetail.Output` is a `string` (`pkg/runner/result.go:9`). `int` matches the Go `len` type.
  - `Duration time.Duration`. Evidence: `toolExecResult.Duration time.Duration` (`pkg/runner/run.go:37-41`).
  - `Turn int`, zero-based (OD4).
- **OD4 (this spec). `Turn` base.** `Turn` is zero-based, equal to the loop variable `turn` in `for turn := 0; turn < maxConversationTurns; turn++` (`pkg/runner/run.go:591`). It matches `ToolCallDetail.Turn` (`pkg/runner/run.go:701`, `pkg/runner/result.go:11`) and the golden JSON, which shows `"turn": 0` for the first turn (`cmd/testdata/golden/json/with_tools.json:19`). The verbose stderr labels use `turn+1` (`pkg/runner/run.go:607-609`); that is display only and is not the API value. A host correlating `ToolEvent.Turn` with `--json` output therefore needs no offset.
- **OD5 (this spec). `ContentLength` is measured before truncation.** It is `len` of the result content after the decision is applied and before `truncateOutput`. `truncateOutput` runs only when building the JSON detail entry (`pkg/runner/run.go:699`), and it changes content only past `maxToolOutputBytes = 1024` (`pkg/runner/run.go:35`, `:1067-1075`). The milestone's "full length" (test 2) therefore means the untruncated byte length. It equals the byte length of the `tool` message content the model receives on the next request (`pkg/runner/run.go:707-711`). The test asserts that equality.
- **OD6 (this spec). `Duration` for denied and stubbed calls is zero.** A call that is not dispatched has no dispatch duration. The hook-decided result is stored as a `toolExecResult` literal that sets no duration (`pkg/runner/run.go:988`). For dispatched calls, `Duration` is the existing dispatch time (`pkg/runner/run.go:1014`, `:1042`). Time spent inside the hook is not included, so `Duration` is unchanged in meaning from `ToolCallDetail.DurationMs` (`pkg/runner/run.go:702`).
- **OD7 (this spec). `Call.Arguments` is a fresh copy.** The observer receives a new map with the same key-value pairs, as the hook does (`pkg/runner/hook.go:71-76`, OD3 of 004). The same map is the assistant message's `ToolCalls` in history (`pkg/runner/run.go:639-644`). A mutation by the observer would otherwise change later requests, so the copy is required.
- **OD8 (this spec). Observer panic.** Each observer call is wrapped in its own `recover`. On a panic the runner writes one fixed line to the run's stderr writer (`pkg/runner/run.go:55-57`, `pkg/runner/options.go:41`): `Warning: tool observer panicked` followed by a newline. The line includes no panic value, no tool name, no argument, and no header text. The remaining events of the turn are still delivered, and the run continues with its normal result. This follows the `Warning: ...` convention (`pkg/runner/run.go:170`, `:224`) but does not interpolate `%v`, because a panic value can carry credential text.
- **OD9 (this spec). Cancellation.** If `executeToolCalls` returns the cancellation error (`pkg/runner/run.go:990-992`), the observer is not called for any call of that turn. No call of the turn finished, and the error is returned from the run unchanged (`pkg/runner/run.go:667-668`). If cancellation happens during dispatch rather than during the decision pass, `executeToolCalls` returns normally (`pkg/runner/run.go:1051`), so the observer is called for every call of that turn, including calls whose dispatch returned an error result.
- **OD10 (this spec). Dry run and budget.** Dry runs return before any tool dispatch (`pkg/runner/run.go:283-334`), so no event is emitted. A budget break before dispatch (`pkg/runner/run.go:652-654`) also emits no event, because `executeToolCalls` never runs for that turn.
- **OD11 (this spec). `ToolObserver` and `sub_agents`.** `ToolObserver` is independent of `ToolHook`. A non-empty `sub_agents` list does not make it a `ConfigError` (D7 says observer behavior is metadata only; the milestone entry names no such rule). `call_agent` calls are still tool calls and produce events. The hook's sub-agent `ConfigError` stays as it is (`pkg/runner/run.go:75-77`).
- **OD12 (this spec). Concurrency.** The observer is called from the run goroutine only, one event at a time. Concurrent `Run` calls that share one observer may call it concurrently. The doc comment must say so, as the hook doc does (`pkg/runner/hook.go:39-41`, OD6 of 004).
- **OD13 (this spec). Test placement and names.** Observer tests go in `pkg/runner/run_test.go`, following the 004 placement (`docs/plans/052_runner_tool_hooks_v1_12_0_004_spec.md`, OD4). The test names are `TestToolObserver*`. No new test file is created. No MCP server is needed, because every milestone test case is a builtin `write_file` or `read_file` call.
- **OD14 (this spec). Test seams.** Tests use `run()` (`pkg/runner/run.go:48`) with `capturingProvider` (`pkg/runner/run_test.go:48-50`) or `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`). Stderr is captured with `newMockStdoutStderr` (`pkg/runner/run_test.go:44-46`) through `Options.Stderr`. No new seam is added (orchestrator decision: prefer observable effects).
- **OD15 (this spec). CHANGELOG.** One `[Unreleased]` `Added` bullet for `Options.ToolObserver` is in scope (`CONTRIBUTING.md:185`, as for 004 R14). It is the only `CHANGELOG.md` change.

### Current values (verified at HEAD `c2620fb`)

- `pkg/runner/hook.go:9` `toolDenyText`. `:11` `errHookPanic`. `:14-22` `ToolAction` and `ToolAllow`, `ToolDeny`, `ToolStub`. `:30-33` `ToolDecision`. `:46` `ToolHook`. `:71-84` `callToolHook`: copies `Arguments` (`:72-76`) and recovers panics (`:78-82`). No `ToolEvent` or `ToolObserver` exists yet.
- `pkg/runner/options.go:6` `type Options struct`. `:29-32` `ToolHook`. `:39-42` I/O fields; `Stderr` at `:41`.
- `pkg/runner/messages.go:98-102`: public `ToolCall` (`ID`, `Name`, `Arguments map[string]string`).
- `internal/provider/provider.go:41-45`: provider-side `ToolCall`. `:48-52`: `ToolResult{CallID, Content string, IsError bool}`.
- `pkg/runner/run.go:37-41`: `toolExecResult{Result provider.ToolResult; Duration time.Duration}`.
- `pkg/runner/run.go:55-57`: `stderr` defaults to `os.Stderr` when `opts.Stderr` is nil.
- `pkg/runner/run.go:75-77`: `ToolHook` with `sub_agents` is `ConfigError`.
- `pkg/runner/run.go:283`: `if opts.DryRun` returns at `:283-334`.
- `pkg/runner/run.go:336`: request timeout context. The observer receives this context.
- `pkg/runner/run.go:591`: turn loop `for turn := 0; turn < maxConversationTurns; turn++`.
- `pkg/runner/run.go:652-654`: budget break before dispatch.
- `pkg/runner/run.go:657`: `totalToolCalls += len(resp.ToolCalls)`.
- `pkg/runner/run.go:666-669`: `executeToolCalls` call site and error return.
- `pkg/runner/run.go:690-704`: JSON `ToolCallDetail` loop. `:699` `truncateOutput(results[i].Result.Content)`, `:701` `Turn: turn`, `:702` `DurationMs`.
- `pkg/runner/run.go:707-711`: tool result message built from `results[i].Result`, in model order.
- `pkg/runner/run.go:925`: `executeToolCalls` returns `([]toolExecResult, error)`.
- `pkg/runner/run.go:963-993`: decision pass. `:966-967` hook call and `ctx.Err()` check. `:970-972` error sets `ToolDeny`. `:973` deny text. `:974-986` action switch; unknown actions fall through with the deny text. `:988` `toolExecResult{Result: r}` with no `Duration`. `:990-992` cancellation `RuntimeError`.
- `pkg/runner/run.go:995-1014`: sequential branch. `:997-998` skips handled calls. `:1002-1003` MCP dispatch. `:1014` `Duration: time.Since(start)`.
- `pkg/runner/run.go:1016-1048`: parallel branch. `:1027` goroutine. `:1042` `Duration: time.Since(start)`. `:1045-1048` join loop places results by index.
- `pkg/runner/run.go:1051`: `return results, nil`.
- `pkg/runner/run.go:1067-1075`: `truncateOutput`; the `len(s)` check at `pkg/runner/run.go:1068` compares against `maxToolOutputBytes` (`:35`).
- `pkg/runner/run.go:524`: `allToolCallDetails` declared. `:754` assigned to `Result.ToolCallDetails`. `:782-784`: `if opts.JSON` and the nil-to-empty conversion; non-JSON runs keep `nil`.
- `pkg/runner/result.go:6-13`: `ToolCallDetail` (`Name`, `Input`, `Output`, `IsError`, `Turn int`, `DurationMs`). `:64`: `ToolCallDetails`.
- `pkg/runner/errors.go:29-31` `RuntimeError`; `:41-43` `Unwrap`.
- `pkg/runner/run_test.go:24` `setupTestEnv`. `:44-46` `newMockStdoutStderr`. `:48-50` `capturingProvider`. `:383` `TestRun_ConversationLoop_OneToolCall`. `:435` `TestRun_ParallelToolExecution`, the model for the parallel observer test.
- `internal/testutil/mockserver.go:50` (`NewMockLLMServer`) and `internal/testutil/mockserver.go:16` (`MockLLMResponse`).
- `cmd/golden_test.go:205-227` `TestGolden`, json mode. `cmd/testdata/golden/json/with_tools.json:19` `"turn": 0`.
- `CHANGELOG.md:8` `[Unreleased]`. `:10` `### Added`. `:13` the 004 `Options.ToolHook` bullet, the format to follow.
- `CONTRIBUTING.md:185`: "Update CHANGELOG.md with your changes".
- `AGENTS.md:165`: tool errors are tool results, not fatal. A deny is different because the tool never runs (004 OD6).

### Rejected approaches (settled)

- Result text or a `Result` field in `ToolEvent` (D7; draft `docs/plans/runner-tool-hooks-and-options.md:221-226`).
- Result truncation in the observer. `truncateOutput` stays limited to the JSON detail entry (`pkg/runner/run.go:699`).
- Recovering panics from the goroutine in the parallel branch. The observer runs on the run goroutine after the join (OD1), so no goroutine recover is needed.
- Adding `call_id` or `action` to `ToolCallDetail` or `--json` (D7).
- A per-hook timeout or a new dependency (004 OD8).
- A new test seam or an axe test helper for nightshift (D14, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:76`).

### Constraints and assumptions

- **Constraint, compatibility (D1, D2).** The added identifiers are `ToolEvent`, `ToolObserver`, and the `Options.ToolObserver` field. Nothing existing changes. The apidiff gate (milestone 002) compares `pkg/runner` against `v1.11.0`; additions do not fail it.
- **Constraint, library scope.** `ToolEvent` uses only public and standard-library types: `ToolCall`, `ToolAction`, `string`, `bool`, `int`, `time.Duration`. It must not reference `internal/` types in its signature.
- **Constraint, struct comparison.** `Options` already holds func fields (`ToolHook`), so no caller can compare it with `==`. `ToolEvent` holds a `ToolCall`, which holds a map, so it is not comparable either. This matches D1's keyed-literal rule.
- **Constraint, the parallel branch.** Goroutines write only to their own slots and to a channel (`pkg/runner/run.go:1027-1043`). The observer is called only after the join (`:1045-1048`), so it never runs concurrently with a dispatch of its own run.
- **Assumption, the `Action` for a hook error.** OD2 uses `ToolDeny`. This is an inference from D6 and OD1 of 004: a hook error is a deny, so the recorded action is deny.
- **Assumption, `Duration` for hook-decided calls.** OD6 is the chosen value. The milestone does not state it. It is recorded here and not blocked, because it changes no existing output and the milestone's acceptance tests do not assert a duration for denied calls.
- **Assumption, `ContentLength` for stubs and denies.** It is the length of the content the model receives, after the decision is applied (OD5). For a deny with empty decision content this is the length of the fixed deny text (`pkg/runner/run.go:973`, `pkg/runner/hook.go:9`).
- **Assumption, observer timeout.** The observer runs under the run context (`pkg/runner/run.go:336`) and has no axe-imposed timeout, as for the hook (004 assumption). A slow observer delays the next request.

## Section 2: Requirements

Requirements R1-R3 are the public API. R4-R8 are the event contents. R9-R12 are ordering, panic, and boundary behavior. R13-R15 are the contract and tests. R16 is the scope boundary.

### R1: Public types and option field

- `pkg/runner` gains, in `pkg/runner/hook.go`:
  - `ToolEvent` with fields `Call ToolCall`, `Action ToolAction`, `CallID string`, `IsError bool`, `ContentLength int`, `Duration time.Duration`, `Turn int`.
  - `ToolObserver` as a function type `func(ctx context.Context, ev ToolEvent)`.
- `Options` (`pkg/runner/options.go:6`) gains one field `ToolObserver ToolObserver`. Its doc comment states that the field is ignored when nil, that it is not called for dry runs, and that it does not change the result (OD10, R13).
- Verification: `go build ./...` exits 0. `rg -n "ToolEvent|ToolObserver" pkg/runner/` shows the type, the function type, and the field.

### R2: One event per tool call, in model order, after the turn's calls finish

- When `ToolObserver` is non-nil, each element of `resp.ToolCalls` produces exactly one event. The events are delivered in model order, on the run goroutine, after `executeToolCalls` returns without error and before the tool result message is appended (OD1).
- This holds for single-call turns, sequential turns (`parallel = false`), and parallel turns. It covers builtin tools, MCP tools, `call_agent`, unknown names, denied calls, and stubbed calls.
- Within a parallel turn, the observer is not called until every dispatch of that turn has returned (`pkg/runner/run.go:1045-1048`).
- Verification: acceptance tests 1 and 2 (R14).

### R3: Effective action

- `Action` is set per OD2:
  - nil hook: `ToolAllow` for every event.
  - hook `ToolAllow`: `ToolAllow`.
  - hook `ToolDeny`: `ToolDeny`.
  - hook `ToolStub`: `ToolStub`.
  - hook error, hook panic, or unknown action: `ToolDeny`.
- The observer never sees the raw hook output, the hook's error text, or a panic value.
- Verification: acceptance test 3 (R14).

### R4: Identity and error fields

- `CallID` equals `Call.ID`.
- `IsError` equals the `IsError` of the result that the model receives for that call, after the decision is applied. A deny is `true`. A stub is `false`. A dispatch error result is `true` (`pkg/runner/run.go:1009`, `:1037`).
- Verification: acceptance tests 3 and 11 (R14).

### R5: Content length is the full byte length

- `ContentLength` is `len` of the result content after the decision is applied and before `truncateOutput` (OD5). It is not truncated at 1024 bytes.
- It equals the byte length of the `tool` message content for that call in the next request (`pkg/runner/run.go:707-711`).
- Verification: acceptance test 2 (R14).

### R6: Duration and turn

- `Duration` is the dispatch duration for a dispatched call, from `toolExecResult.Duration` (`pkg/runner/run.go:1014`, `:1042`). It is zero for a denied, stubbed, or hook-decided call (OD6).
- `Turn` is the zero-based loop index (OD4, `pkg/runner/run.go:591`).
- Verification: acceptance tests 1 and 3 (R14) assert `Turn` for the first turn equals 0 and the stub and deny durations equal zero.

### R7: `Call` is a copy and carries no result text

- `Call.Arguments` is a fresh map with the same key-value pairs as the parsed arguments (OD7). An observer mutation does not change the request history.
- `ToolEvent` has no field that carries result text. It has `ContentLength` only (D7).
- Verification: acceptance test 1 (R14) asserts `Call.Arguments` equal to the parsed arguments and asserts that the next request's messages are unchanged after the observer mutates its copy. The absence of a text field is checked by `rg -n "Content " pkg/runner/hook.go` returning no `ToolEvent` content field.

### R8: Nil observer is byte-identical

- With `ToolObserver` nil, no event is built and no new code runs in the turn loop beyond a nil check. Output, tool results, `Result`, and JSON are unchanged (D2).
- Verification: acceptance test 5 (R14), the `ToolCallDetails` nil assertion in acceptance test 2 (observer set, JSON off), and the existing golden test (acceptance test 9).

### R9: Observer panic is recovered and reported with one fixed line

- Each observer call is wrapped in its own `recover` (OD8). On a panic the runner writes exactly one line to the run's stderr writer: `Warning: tool observer panicked` followed by a newline.
- The line contains no panic value, no tool name, no argument, and no header text.
- Delivery continues: remaining events of the turn are still delivered. The run completes with its normal `Result` and `error` (`nil`). The panic does not change any tool result.
- Verification: acceptance test 4 (R14).

### R10: Observer and the hook are independent

- A non-empty `sub_agents` list with a non-nil observer is not a `ConfigError` (OD11). Only `ToolHook` keeps the sub-agent `ConfigError` (`pkg/runner/run.go:75-77`).
- `call_agent` calls produce events like any other call (OD11).
- Verification: acceptance test 3 covers a hook deny with an observer set. Acceptance test 12 covers `sub_agents` with a nil hook and a non-nil observer, and asserts the run proceeds and the `call_agent` event is delivered. The existing `TestToolHookSubAgentConfigError` (`pkg/runner/run_test.go:1858`) runs with `ToolObserver` unset, so it does not verify this rule (OD11).

### R11: Cancellation, dry run, and budget emit no events

- A cancellation error from the decision pass (`pkg/runner/run.go:990-992`), returned by `executeToolCalls` (`pkg/runner/run.go:925`), emits no event for that turn (OD9). The error is returned from the run (`pkg/runner/run.go:667-668`).
- Cancellation during dispatch emits one event per call of that turn (OD9), with `IsError` `true` for each dispatch that ended with an error result (test 10).
- A dry run emits no event (OD10, `pkg/runner/run.go:283-334`).
- A budget break before dispatch emits no event (OD10, `pkg/runner/run.go:652-654`).
- Verification: acceptance tests 7, 8, and 10 (R14).

### R12: Nil hook gives `ToolAllow` for every event

- With `ToolObserver` set and `ToolHook` nil, every event has `Action` `ToolAllow` (D7).
- Verification: acceptance test 6 (R14).

### R13: Doc comments state the contract

- The doc comment on `ToolObserver` states: it is called on the run goroutine, sequentially, in model order, after the turn's tool calls finish (OD1, R2); concurrent `Run` calls sharing one observer may call it concurrently, so the host must make it safe (OD12); it returns nothing; a panic is recovered and reported with one fixed stderr line (OD8); it receives metadata only and never result text (D7).
- The doc comment on `ToolEvent` states that `Action` is the effective action (OD2), that `Duration` is zero for calls that were not dispatched (OD6), that `Turn` is zero-based (OD4), and that `ContentLength` is the untruncated byte length (OD5).
- The doc comment on the `Options.ToolObserver` field states that it is ignored when nil and is not called for dry runs (R1).
- Verification: `rg -n "run goroutine|fixed|zero-based|untruncated|not called for dry runs|concurrently" pkg/runner/hook.go pkg/runner/options.go` shows each claim.

### R14: Acceptance tests

All tests are in `pkg/runner/run_test.go` (OD13). Test names use the prefix `TestToolObserver`. Each test is a table or subtest where the cases differ only by input. Tests that use `setupTestEnv` (`pkg/runner/run_test.go:24`) cannot use `t.Parallel`.

1. **Order, fields, and arguments.** Four subtests, run in this order. Each uses one turn with three `write_file` calls (`a.txt`, `b.txt`, `c.txt`), as in `TestRun_ParallelToolExecution` (`pkg/runner/run_test.go:435`).
   - 1a, sequential order: `parallel = false`. The observer records each event's `CallID`, `Turn`, `IsError`, `Duration`, and `Call.Arguments`. Assert the events are in model order, every `Turn` is `0`, and every `Action` is `ToolAllow`.
   - 1b, parallel order and post-join: `parallel = true`. Assert the same as 1a. The observer also asserts that all three target files exist when it is called, which shows it runs after the join (R2).
   - 1c, argument equality: the observer records `Call.Arguments`. Assert each equals the parsed arguments of the model's call, with the same key-value pairs.
   - 1d, argument mutation: the observer sets an extra key in each event's `Call.Arguments`. Assert the next request's assistant tool call arguments equal the original parsed arguments, so the mutation changes no history (R7, OD7).
2. **Full content length.** The workdir holds a file of 4096 bytes, and the model calls `read_file` on it. The observer event's `ContentLength` is greater than 1024 and equals the byte length of the `tool` message content in the next request. The test uses `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`) with `AnthropicToolUseResponse` (`:154`) then `AnthropicResponse` (`:141`), and reads the result from `mock.Requests[1].Body` (`:27`, `:34`) via `hookTestToolResult` (`pkg/runner/run_test.go:1465`). `capturingProvider` (`pkg/runner/run_test.go:52-54`) returns text only, so it cannot produce a tool result. The observer is set and `JSON` is off, so the test asserts `Result.ToolCallDetails` is `nil` (`pkg/runner/run.go:782-784`). The event has no content field (R7).
3. **Deny, stub, hook error, hook panic, and bogus action.** Table cases, each with a `write_file` call and a hook: (a) hook `ToolDeny` with empty content, (b) hook `ToolStub` with `Content: "stub text"`, (c) hook returns an error, (d) hook returns `ToolAction("bogus")`, (e) hook panics, (f) hook returns `ToolAllow`. Assert the event `Action` is `ToolDeny`, `ToolStub`, `ToolDeny`, `ToolDeny`, `ToolDeny`, and `ToolAllow` respectively; `IsError` is `true`, `false`, `true`, `true`, `true`, and `false`; `Duration` is zero for (a)-(e); `ContentLength` equals the length of the content the model receives for every case. For (f), the `write_file` call is dispatched and its result content length is read from the `tool` message in the next request (`mock.Requests[1].Body`, as in test 2; `mock` is built as in test 2). The file is not created in cases (a), (c), (d), and (e); it is created in case (f) with the written content. Also assert the hook's error text and panic text are absent from the observer event fields (`Call`, `Action`, `CallID`, `IsError`, `ContentLength`, `Duration`, `Turn`).
4. **Observer panic.** The observer panics on its first event. `Options.Stderr` is a buffer from `newMockStdoutStderr` (`pkg/runner/run_test.go:44-46`). Assert `err` is `nil`, the `Result` is non-nil, the stderr buffer contains exactly one line equal to `Warning: tool observer panicked`, and the stderr buffer contains no argument or panic text. Use a turn with two calls, and assert the observer is called twice (delivery continues, R9).
5. **Nil observer is unchanged.** A run with no observer and no hook produces the same `Result.Content`, `ToolCalls`, and tool result content as before the change. `Result.ToolCallDetails` is `nil` when `JSON` is false (`pkg/runner/run.go:782-784`).
6. **Nil hook gives `ToolAllow`.** An observer is set, `ToolHook` is nil, and the model calls `read_file`. The event `Action` is `ToolAllow` (R12).
7. **Cancellation during decision.** Reuse the scenario of `TestToolHookCancelDuringDecision` (`pkg/runner/run_test.go:2621`) with an observer set. Assert the observer was not called and the run returns the cancellation error (R11, OD9).
8. **Dry run and budget.** Table cases: a dry run with the observer set (no calls, no events, `Run` returns a dry-run result) and a budget that is exceeded before dispatch (no events for that turn). Assert zero events.
9. **Golden JSON unchanged.** Run `go test ./cmd -run TestGolden -count=1` (`cmd/golden_test.go:205`). The golden files are unchanged: `git diff --name-only c2620fb -- cmd/testdata` is empty. The CLI cannot set an observer, so this is a regression check, not a new seam (milestone test 6).
10. **Cancellation during dispatch** (`TestToolObserverDispatchCancel`). The agent has `tools = ["run_command"]`, there is no hook, `Options.Timeout` is not set, and the observer is set. The parent context is cancelable. The model returns two `run_command` calls in one turn (IDs `tool_1` and `tool_2`), with commands `touch m1 && exec sleep 30` and `touch m2 && exec sleep 30`; they are parallel by default. A helper goroutine polls the workdir for `m1` and `m2`, each wait bounded to 10 s. When both markers exist, it calls `cancel()`. If the bound expires, it still calls `cancel()` and sends a failure to a channel that the test reads after `Run` returns. The helper does not call `t.Fatal`. Both markers exist only after both dispatches have started, so the cancel lands during dispatch, not the decision pass (no hook). `run_command` honors cancellation (`internal/tool/run_command.go:83`), and the parallel branch runs both calls concurrently (`pkg/runner/run.go:1022-1044`). Assert no marker-wait failure, the observer is called exactly twice in model order (`tool_1`, then `tool_2`), and each event has `Action` `ToolAllow`, `Turn` `0`, `IsError` `true`, and `CallID` equal to `Call.ID` and to the model's call ID. The `exec` prefix makes `sh` replace itself with `sleep`. Without it, dash keeps `sleep` as a child that holds the output pipe after the kill, and `CombinedOutput` waits for the full 30 s (measured with a standalone probe outside the repository: 30.0 s without `exec`, 0.3 s with it). Do not assert `Duration` or the run's final error, because the next request runs on the cancelled context. The mock provider needs only the tool-use response.
11. **Dispatch error, no hook** (`TestToolObserverDispatchError`). Set the observer, no hook. The model calls an unregistered tool `no_such_tool` (ID `tool_1`). The registry returns `unknown tool "no_such_tool"` (`internal/tool/registry.go:85`), which the runner converts to an error result (`pkg/runner/run.go:1008-1009`). Assert one event with `Action` `ToolAllow`, `IsError` `true`, `CallID` and `Call.ID` both `tool_1`, `Call.Name` `no_such_tool`, and `ContentLength` equal to the length of that error content.
12. **Sub-agent with no hook** (`TestToolObserverSubAgentNoHook`). Set `sub_agents = ["child-agent"]`, no `ToolHook`, and the observer. The parent model calls `call_agent` (ID `tool_1`), and the child agent returns text, as in `TestRun_SubAgent` (`pkg/runner/run_test.go:766`). Assert `err` is `nil`, the `Result` is non-nil, and exactly one event is delivered with `Call.Name` `call_agent`, `CallID` `tool_1`, `Action` `ToolAllow`, and `IsError` `false` (OD11, R10).

### R15: Concurrency and cross-run ordering

- No cross-run ordering is guaranteed (D7). Concurrent runs sharing one observer are documented (R13), not tested.

### R16: Scope boundary

- Changed tracked files for this milestone are: `pkg/runner/hook.go`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`, and one `[Unreleased]` `Added` bullet in `CHANGELOG.md` for `Options.ToolObserver` (OD15). No other file changes.
- `ToolCallDetail` (`pkg/runner/result.go`), `cmd/`, `internal/`, `go.mod`, `go.sum`, `COMPATIBILITY.md`, `README.md`, `AGENTS.md`, `Makefile`, and `cmd/testdata/` are not changed.
- Verification: `git diff --name-only c2620fb -- . ':(exclude)docs/plans/'` lists only the five files above.

## Parallelizable work

- The `ToolEvent` and `ToolObserver` types and the `Options` field (R1) must exist before the event wiring compiles.
- The doc comments (R13) are written with R1.
- The CHANGELOG bullet (R16) is independent of the code.
- Tests 1-12 (R14) share one file and are written after the wiring (R2-R12).

## Serial dependencies

- R1 before R2-R12 (types must exist).
- R2-R12 before R14 (tests exercise the wiring).
- R13 doc comments are written with R1, before the final gate.
- The focused race run (below) after R14.

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                  # expect: exit 0
go vet ./pkg/runner/                                            # expect: exit 0
go test ./pkg/runner/                                           # expect: exit 0
rg -n "ToolEvent|ToolObserver" pkg/runner/hook.go pkg/runner/options.go   # expect: the type, function type, and field
rg -n "tool observer panicked" pkg/runner/                      # expect: the fixed line in run.go and the test assertion in run_test.go
go test ./cmd -run TestGolden -count=1                          # expect: exit 0 (see environmental limitation for ~/.cache/axe)
git diff --name-only c2620fb -- cmd/testdata                    # expect: empty output
git diff --name-only c2620fb -- . ':(exclude)docs/plans/'       # expect: CHANGELOG.md, pkg/runner/hook.go, pkg/runner/options.go, pkg/runner/run.go, pkg/runner/run_test.go
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since c2620fb /home/jaron/dev/axe/runner-tool-hooks                                              # expect: exit 0
```

Focused race check (after R14 tests exist): the race-enabled run of the `TestToolObserver` tests in `pkg/runner` exits 0 with no `DATA RACE` output. It is not in the fenced block because `spec-lint` reports "selects no test" until the tests are written. The full race run over all packages is Factory's completion gate, not this milestone's check.

## Failure modes

1. **Observer called from a goroutine in the parallel branch.** Events would race and lose model order. Mitigation: the caller loop in the turn loop calls the observer after the join (OD1, R2, test 1b).
2. **Result text reaches the observer.** Mitigation: `ToolEvent` has no content field (R7, R13); test 2 asserts only `ContentLength`.
3. **Truncated length reported.** `ContentLength` would be capped at 1024. Mitigation: length is taken before `truncateOutput` (OD5, R5, test 2).
4. **Raw hook output recorded as `Action`.** A hook returning `bogus` would record `bogus`. Mitigation: OD2 and R3, test 3 case (d).
5. **Observer mutates history.** Mitigation: OD7 copy (R7, test 1d).
6. **Panic value leaks to stderr.** Mitigation: fixed line with no interpolation (OD8, R9, test 4).
7. **Cancelled turn reports events for calls that did not run.** Mitigation: OD9 and R11, tests 7 and 10.
8. **Dry run reports events.** Mitigation: the dry-run return (`pkg/runner/run.go:283-334`) precedes all dispatch (OD10, test 8).
9. **Nil observer changes JSON.** Mitigation: no change to `ToolCallDetail` (R8, test 5, test 9). An observer set with JSON off leaves `Result.ToolCallDetails` nil (R8, test 2).

## Limitations

- The observer is verified in-process through `run()` with `capturingProvider` or `testutil.NewMockLLMServer`. The CLI cannot set an observer, so no CLI test of the observer exists.
- Concurrency across `Run` calls that share one observer is documented (R13), not tested.
- The `Duration` value for dispatched calls excludes hook time (OD6); this is verified by test 3 asserting zero only for the hook-decided cases, not by a timing bound.
- The fixed panic line is verified by exact match in test 4; the wording itself is a decision (OD8), not a stable public contract.
