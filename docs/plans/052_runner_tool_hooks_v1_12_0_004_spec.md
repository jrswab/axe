# Milestone 004: Let a host deny or stub any tool call before dispatch with `Options.ToolHook`

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 004 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:168-188`). Scope, semantics, and acceptance tests are taken from that entry as settled by D6 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:59-67`) and the orchestrator decisions below.
- Predecessors: milestone 002 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154`, `[x] 002`), so the apidiff gate runs on this change. Milestone 003 is complete (`[x] 003`, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:161`).
- Baseline: branch `runner-tool-hooks`, base commit `09d100d3bfe8ad219113c8b82840110dd789d81b`, clean tree (`git status --porcelain` empty at `09d100d`).
- Lane: full. The milestone adds exported identifiers to `pkg/runner`, which is an external Go API contract (D1). It also edits the parallel dispatch path in `executeToolCalls`.

### Decisions settled (record as settled, never re-evaluate)

- **D6 (milestone).** Allow, deny, or stub only; no rewrite (Q1). `ToolAction` is `type ToolAction string` with `"allow"`, `"deny"`, `"stub"` (Q11). A denied call is an error result (`IsError: true`) and the run continues (Q2). A stub is a normal result. Neither is dispatched. `Result.ToolCalls` counts denied and stubbed calls. The hook decides every call of a turn, in model order, on the run goroutine, before any dispatch; allowed calls then use the existing sequential or parallel path (Q12). With a nil hook the decision pass is skipped. Hook error or panic is a deny with fixed text; raw error text never reaches the model. Any unrecognized action is a deny. `Content` is ignored for allow. Cancellation during the decision pass dispatches nothing in that turn and returns the context error as a `RuntimeError` whose `Unwrap` chain reaches `ctx.Err()`. `ToolHook` with a non-empty `sub_agents` list is a `ConfigError` (Q3). No `--json` or `Result` field for decisions. The CLI cannot set a hook (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:59-67`).
- **D6 precedence note.** D6 wins over OD1-OD2 wherever they overlap. The orchestrator decisions below add detail only where D6 is silent.
- **D2 and D14.** New behavior is opt-in through `Options`; the zero value keeps today's behavior (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:55`). No axe test helper is added for nightshift (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:76`).
- **D1 (API).** No exported identifier is removed or changed. Additions only (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54`).
- **OD1 (orchestrator).** Fixed deny text for hook error, hook panic, and unknown action: a single unexported constant whose text is `tool call denied by host policy`. Raw hook error or panic text never appears in the tool result.
- **OD2 (orchestrator).** `ToolDeny` result content is `decision.Content` when non-empty, otherwise the OD1 text; `IsError` is true. `ToolStub` result content is `decision.Content` as-is (empty allowed); `IsError` is false. `ToolAllow` ignores `Content`.
- **OD3 (orchestrator).** The hook receives a copy of `call.Arguments` (a fresh map). A hook mutation cannot change the dispatched arguments.
- **OD4 (orchestrator).** Non-MCP hook tests go in `pkg/runner/run_test.go`, as the milestone entry states. This spec picks `run_test.go` and does not create `pkg/runner/hook_test.go`. MCP tests (acceptance tests 3-5) go in a new file `pkg/runner/hook_mcp_test.go`, with a local go-sdk streamable-HTTP `httptest` helper modeled on `cmd/run_mcp_integration_test.go:17-46`. The parallel test is named `TestToolHookParallelTurn`.
- **OD5 (orchestrator).** Cancellation: after each hook call, and before any dispatch, the runner checks `ctx.Err()`. If it is non-nil, the turn dispatches no call and the run returns `&RuntimeError{Msg: "tool hook: run canceled", Err: ctx.Err()}`.
- **OD6 (orchestrator).** Doc comments state: the hook is called sequentially on the run goroutine, in model order, before any dispatch of that turn; concurrent `Run` calls that share one hook may call it concurrently, so the host must make it safe; a deny differs from ordinary tool errors (`AGENTS.md:165`) because the tool never runs.
- **OD7 (orchestrator).** The fail-closed check keys on `len(cfg.SubAgents) > 0` only. `call_agent` is injected only when that condition holds (`pkg/runner/run.go:423-424`), so a sub-agent call cannot be reached without a non-empty `sub_agents` list. This is stated as a boundary, not as a general guarantee about model output (see Requirement R10).
- **OD8 (orchestrator).** No dependency is added. No CLI, TOML, or `--json` change. Nil-hook behavior is byte-identical.
- **OD9 (orchestrator).** An unknown action (including the empty string) always yields the fixed OD1 text `tool call denied by host policy` with `IsError: true`. `decision.Content` is ignored for an unknown action, because the host's intent is unknown (fail closed). This overrides OD2's "`decision.Content` when non-empty" rule for unknown actions only.

### Current values (verified at HEAD `09d100d`)

- `pkg/runner/options.go:6-33`: `type Options struct` and its fields. `Messages` at `:27`; I/O fields at `:30-32`. No `ToolHook` field exists.
- No `pkg/runner/hook.go` exists. The milestone creates it (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:171`).
- `pkg/runner/messages.go:98-101`: `type ToolCall struct` with `ID`, `Name`, `Arguments map[string]string`. The public-to-provider copy is `pkg/runner/messages.go:136-152` (`toProviderToolCalls`), and the reverse copy is at `:166-170`.
- `internal/provider/provider.go:41-45`: the provider-side `ToolCall` type that `executeToolCalls` receives.
- `pkg/runner/run.go:43-45`: `Run` calls `run(ctx, opts, provider.New)`. `:47` is the `run` seam signature.
- `pkg/runner/run.go:70-73`: agent load. A load error returns `ConfigError` at `:72`. This is the insertion point for the sub-agent check ("right after agent load", D6).
- `pkg/runner/run.go:279`: `if opts.DryRun`. The dry-run return is at `:279-330`.
- `pkg/runner/run.go:332-333`: the request timeout context wraps `ctx`. The hook receives this derived context.
- `pkg/runner/run.go:423-425`: `call_agent` is appended to `req.Tools` only when `len(cfg.SubAgents) > 0 && depth < effectiveMaxDepth`.
- `pkg/runner/run.go:519`: `totalToolCalls` is declared. `:653` adds `len(resp.ToolCalls)` before dispatch, so denied and stubbed calls are counted.
- `pkg/runner/run.go:662`: the single call site of `executeToolCalls(ctx, resp.ToolCalls, ...)`.
- `pkg/runner/run.go:664-679`: JSON `ToolCallDetail` entries are built from `results[i]` for every call in `resp.ToolCalls`.
- `pkg/runner/run.go:682-690`: tool results are appended as one `tool` message in model order.
- `pkg/runner/run.go:890`: `func executeToolCalls(...) []toolExecResult`. It returns no error today.
- `pkg/runner/run.go:927-944`: sequential branch (`if len(toolCalls) == 1 || !parallel`). Each call goes MCP first (`:931`), then the `call_agent` name check (`:933`), then the builtin registry (`:936`).
- `pkg/runner/run.go:945-974`: parallel branch. Goroutines are started at `:952`. The receive loop is at `:970-973`, and results are placed by index at `:972`.
- `pkg/runner/run.go:979-990`: `dispatchMCPToolCall`. A dispatch error becomes `ToolResult{IsError: true}` (`:986-988`).
- `pkg/runner/run.go:1019`: `mapProviderError`. Provider cancellation is mapped here (`pkg/runner/run.go:557` for the single-shot path and `:616` for the loop path).
- `pkg/runner/errors.go:23-25`: `ConfigError.Unwrap`. `:41-43`: `RuntimeError.Unwrap`.
- `pkg/runner/errors.go:56-60`: `IsConfigError` uses a direct type assertion (`err.(*ConfigError)`). A wrapped `ConfigError` is not recognized, so the `ConfigError` must be returned unwrapped.
- `internal/tool/registry.go:82-91`: `Registry.Dispatch`. An unknown name returns an error. `write_file` is registered at `internal/tool/registry.go:100`.
- `internal/tool/tool.go:90`: `ExecuteCallAgent`. It rejects names not in the allowed list (`internal/tool/tool.go:116-124`, the loop over `opts.AllowedAgents` and the rejection that follows).
- `internal/mcpclient/router.go:34`: MCP tools are registered under their raw name. `:26-28`: a builtin name wins. `:42-48`: `Dispatch` returns `unknown MCP tool` for an unregistered name.
- `pkg/runner/result.go:63-64`: `ToolCalls` and `ToolCallDetails` fields on `Result`.
- `pkg/runner/run_test.go:20-38`: `setupTestEnv` uses `t.Setenv`, so the tests are not `t.Parallel`. `:44-58`: `capturingProvider` (`Send` and `SendStream`).
- `pkg/runner/run_test.go:379-429`: `TestRun_ConversationLoop_OneToolCall`. It uses `testutil.NewMockLLMServer` and `AXE_ANTHROPIC_BASE_URL`. `:431-476`: `TestRun_ParallelToolExecution`, the model for the parallel test.
- `internal/testutil/mockserver.go:41-45`: `MockToolCall`.
- `cmd/run_mcp_integration_test.go:17-46`: `newRunTestMCPServer`, the go-sdk streamable-HTTP helper that OD4 says to model on.
- `cmd/run_mcp_integration_test.go:65-69`: agent TOML with `[[mcp_servers]]`, `transport = "streamable-http"`. The only MCP integration test is CLI-level. It cannot set `Options.ToolHook`.
- `AGENTS.md:165`: tool errors are tool results, not fatal. A deny in this milestone differs because the tool never runs.
- `go.mod:9`: `github.com/modelcontextprotocol/go-sdk v1.4.1`. No new dependency is needed (OD8).

### Rejected approaches (settled)

- Rewrite action in the hook (Q1).
- Aborting the run on denial (Q2). The run continues and `max_turns` bounds the loop.
- Hook propagation to sub-agents (Q3). Fail closed with `ConfigError`.
- Per-call decision in `ToolCallDetail`, `--json`, or `Result` (D6).
- Hook timeout added by axe (milestone assumption, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:109`). The hook runs under the run context and must return quickly.
- Dry-run hook invocation. `DryRun` returns before dispatch and is unchanged (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:110`).
- A new dependency or a new `hook_test.go` file (OD4, OD8).

### Constraints and assumptions

- **Constraint, compatibility (D1, D2).** The added identifiers are `ToolAction`, `ToolAllow`, `ToolDeny`, `ToolStub`, `ToolDecision`, `ToolHook`, and the `Options.ToolHook` field. Nothing existing changes. The apidiff gate (milestone 002) compares `pkg/runner` against `v1.11.0`. Additions do not fail it (`COMPATIBILITY.md`, `docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md` probe evidence).
- **Constraint, library scope.** `pkg/runner` is importable by other modules. The new public types must not use an `internal/` type in their signatures. `ToolCall` (`pkg/runner/messages.go:98`) is the public type the hook receives.
- **Constraint, error types.** `IsConfigError` is a type assertion (`pkg/runner/errors.go:57-60`). The sub-agent `ConfigError` must be returned unwrapped.
- **Constraint, test seam.** Non-MCP tests use `run()` (`pkg/runner/run.go:47`) with `capturingProvider` or `testutil.NewMockLLMServer`. Tests that call `setupTestEnv` cannot use `t.Parallel`.
- **Assumption, panic recovery.** The hook runs on the run goroutine (D6), so the recover is placed in the decision pass, not in a spawned goroutine.
- **Assumption, stderr.** Hook errors and panics produce no new stderr line. D6 and OD1 do not require one, and no output contract changes. This is a consequence of D2 and the nil-hook rule, not an open question.
- **Assumption, unknown names.** The decision pass sees every call the model emits, including names with no registered tool (for example a call to `call_agent` when `sub_agents` is empty, or a hallucinated name). Those calls go through the same decision pass before the existing dispatch rules apply.
- **Assumption, JSON detail.** With `JSON` set, `ToolCallDetails` already has one entry per model call (`pkg/runner/run.go:664-679`). A denied call's entry carries the deny text as `Output`, so host-supplied deny content can appear in `--json` output. No new field is added.

## Section 2: Requirements

Requirements R1-R3 are the public API and wiring. R4-R9 are the decision semantics. R10-R12 are the boundaries (sub-agents, nil hook, documentation). R13 is the test set. R14 is the scope boundary.

### R1: Public types and option field

- `pkg/runner` gains, in a new file `pkg/runner/hook.go`: `ToolAction` (string type) with the constants `ToolAllow`, `ToolDeny`, `ToolStub`; `ToolDecision` with `Action ToolAction` and `Content string`; and `ToolHook` as a function `func(ctx context.Context, call ToolCall) (ToolDecision, error)`.
- `Options` (`pkg/runner/options.go:6-33`) gains one field `ToolHook ToolHook`. Its doc comment states that the field is ignored when nil and that it is not applied to dry runs (OD6 and D6).
- Verification: `go build ./...` passes. `rg -n "ToolHook" pkg/runner/` shows the type and the field.

### R2: Decision pass before any dispatch, in model order

- When `ToolHook` is non-nil and a turn has tool calls, the hook is called once per call, in model order, on the run goroutine, before any call of that turn is dispatched. This holds for single-call turns, sequential turns (`parallel = false`), and parallel turns.
- The decision pass happens inside `executeToolCalls` (`pkg/runner/run.go:896`), before both the sequential branch (`:927`) and the parallel branch (`:945`). `executeToolCalls` gains the hook as a parameter and an `error` return. Its single call site (`pkg/runner/run.go:662`) returns that error from the run unwrapped (the cancellation `RuntimeError`, OD5). `totalToolCalls` (`:656`) is still counted before the call, so denied and stubbed calls are counted.
- The hook receives the run context (`pkg/runner/run.go:332-333`) and a `ToolCall` whose `Arguments` is a copy (R5).
- Verification: R13 tests 2, 5, and 7.

### R3: Allowed calls use the existing dispatch paths

- A call decided `ToolAllow` is dispatched by the existing code: MCP first (`pkg/runner/run.go:931`), then the `call_agent` name check (`:933`), then the builtin registry (`:936`), sequentially or in parallel as today. Its result is the existing result, unchanged.
- Verification: acceptance test 5 (R13) and test 1 (no hook).

### R4: Deny is an error result and dispatches nothing

- A call decided `ToolDeny`, or a call whose decision is an error (R7), or a call with an unknown action (R8), is not dispatched: no builtin executor runs, no MCP call is sent, and no `call_agent` runs.
- Its result has `CallID` equal to the call ID and `IsError: true`. For `ToolDeny`, `Content` is as in OD2: `decision.Content` when non-empty, else the OD1 text. For an error (R7) or an unknown action (R8), `Content` is always the OD1 text.
- Other calls in the same turn are unaffected. The run continues (D6, Q2).
- Verification: acceptance tests 2, 3, 8, 9 (R13).

### R5: Stub returns a normal result; hook receives a copy of the arguments

- A call decided `ToolStub` is not dispatched. Its result has `CallID`, `Content` set to `decision.Content` exactly (empty is allowed), and `IsError: false` (OD2, D6).
- The hook receives a fresh map with the same key and value pairs as `call.Arguments` as the provider parsed them (OD3). A mutation by the hook does not change the arguments of the dispatched call, and the dispatched call's arguments equal the parsed arguments.
- Verification: acceptance tests 4 and 6 (R13).

### R6: Allow ignores content

- A call decided `ToolAllow` is dispatched. Any `Content` in the decision is ignored (D6). A decision of `ToolAllow` with a non-nil error is treated as an error (R7), not as an allow.
- Verification: acceptance test 5 (R13) with a decision that carries content.

### R7: Hook error or panic is a deny with fixed text

- If the hook returns a non-nil error, the call is denied: `IsError: true` and the OD1 text. The error text does not appear in any result, message, or `ToolCallDetail` (D6, OD1).
- If the hook panics, the panic is recovered in the decision pass. The call is denied as in the error case, and the run continues. The panic value does not appear in any result (D6, OD1).
- The decision pass recovers panics only from the hook call. It does not recover panics elsewhere.
- Verification: acceptance tests 8 and 9 (R13).

### R8: Unknown action is a deny

- A decision whose `Action` is not `ToolAllow`, `ToolDeny`, or `ToolStub` (including the empty string) is denied (D6). Its result has `IsError: true` and `Content` equal to the fixed OD1 text `tool call denied by host policy`, whatever `decision.Content` holds, empty or non-empty (OD9). `decision.Content` is not read for an unknown action.
- Verification: acceptance test 13 (R13) in `pkg/runner/run_test.go`, with a hook that returns `ToolAction("")` and a second case that returns an unknown action with non-empty `Content`. In both cases the tool result content equals the fixed text.

### R9: Cancellation dispatches nothing and returns the context error

- After each hook call in the decision pass, and before any dispatch of the turn, the runner checks `ctx.Err()`. If it is non-nil, no call of that turn is dispatched (neither decided nor undecided calls), no further hook calls are made in that turn, and the run returns `&RuntimeError{Msg: "tool hook: run canceled", Err: ctx.Err()}` (OD5, D6).
- `errors.Is(err, context.Canceled)` holds when the parent context was cancelled (the `Unwrap` chain reaches `ctx.Err()`, `pkg/runner/errors.go:41-43`).
- The error is returned from the turn loop without `mapProviderError`. The result is `nil`.
- Verification: acceptance test 12 (R13).

### R10: Sub-agents with a hook are a ConfigError, and the boundary is stated

- When `ToolHook` is non-nil and `cfg.SubAgents` is non-empty, `Run` returns `*ConfigError` (unwrapped; `IsConfigError` must hold) right after agent load (`pkg/runner/run.go:70-73`), before the `DryRun` return (`pkg/runner/run.go:279`) and before any provider is constructed (`pkg/runner/run.go:358`). No provider call is made (D6, Q3).
- The check keys on `len(cfg.SubAgents) > 0` (OD7). The boundary is: `call_agent` is injected only under that condition (`pkg/runner/run.go:423-424`). Without a non-empty `sub_agents` list, `call_agent` is not in the request. The hook still sees any `call_agent` name the model emits, and R4 applies to it.
- Verification: acceptance test 10 (R13).

### R11: Nil hook behavior is byte-identical

- With `ToolHook` nil, no decision pass runs, the hook is not called, and no new code path changes output, tool results, `Result`, or JSON (D6, OD8).
- Verification: acceptance test 1 (R13). Existing tests pass unchanged (`go test ./pkg/runner`).

### R12: Doc comments state the contract

- The doc comment on `ToolHook` (in `pkg/runner/hook.go`) states OD6: sequential calls on the run goroutine, in model order, before any dispatch of the turn; concurrent `Run` calls sharing one hook may call it concurrently, so the host must make it safe; a deny differs from an ordinary tool error (`AGENTS.md:165`) because the tool never runs.
- The doc comment on `ToolDecision`, or on each action constant, states OD1 (fixed deny text) and OD2 (content rules).
- The doc comment on the `ToolHook` field in `Options` states that it is ignored for dry runs and that a non-empty `sub_agents` list is a `ConfigError`.
- Verification: `rg -n "run goroutine|fixed|tool call denied by host policy|sub_agents" pkg/runner/hook.go pkg/runner/options.go` shows each claim.

### R13: Acceptance tests

Tests 1, 2, 6, 7, 8, 9, 10, 12, and 13 are in `pkg/runner/run_test.go`. Tests 3, 4, and 5 (MCP) are in `pkg/runner/hook_mcp_test.go` (OD4). Test 11 is a command, not a test function. Each test is a table or subtest where the cases differ only by input.

1. **No hook.** A run with no `ToolHook` produces the same `Content`, `ToolCalls`, and tool result content as before the change for a builtin read tool.
2. **Deny a builtin write tool.** A hook denies `write_file`. The file is not created in the workdir. The model receives a `tool` result with `IsError: true` and the OD1 text (empty `Content` in the decision) or the decision `Content` when set. The run continues to the next turn.
3. **Deny an MCP tool.** A hook denies an MCP tool served by a streamable-HTTP test server. The server records zero calls for that tool. The model receives `IsError: true`.
4. **Stub an MCP tool.** A hook stubs an MCP tool with `Content: "stub text"`. The server records zero calls. The model receives `Content` equal to `stub text` and `IsError: false`.
5. **Allow a read MCP tool.** A hook allows an MCP tool that carries a non-empty `Content` in its decision. The server records one call, and the result is the server's result. `Content` in the decision is ignored (R6).
6. **Hook receives arguments as parsed.** A hook records `call.Arguments` for a call with two string arguments and compares them with the arguments the provider parsed (the values the mock LLM sent). A hook that mutates its copy does not change the dispatched call's arguments (R5).
7. **Parallel turn, three calls.** `TestToolHookParallelTurn`: the model returns three parallel `write_file` calls (`a.txt`, `b.txt`, `c.txt`) in one turn under `parallel = true` (model: `TestRun_ParallelToolExecution` at `pkg/runner/run_test.go:431-476`). The hook runs three times in model order, records each call ID in a mutex-protected log, and on each invocation asserts that none of the three target files exists in the workdir yet. All three calls are allowed. After the run all three files exist. This observes "hook runs three times in model order, before any dispatch" through file creation, with no new seam; a dispatch that has started but not yet created its file is not detected. The test runs under `-race`.
8. **Hook returns an error.** The call is denied with the OD1 text. The error's message is absent from the tool result content, the messages sent in the next request, and `ToolCallDetails` output when `JSON` is set (R7).
9. **Hook panics.** The call is denied with the OD1 text. The run completes with a `Result` and no error. The panic value is absent from the tool result.
10. **Sub-agents with a hook.** An agent with a non-empty `sub_agents` list and a non-nil `ToolHook`: `Run` returns a value for which `IsConfigError` is true, and the mock LLM server receives zero requests (R10).
11. **Focused race test.** Run `TestToolHookParallelTurn` with `-race` in `pkg/runner`. The full `go test -race ./...` run is Factory's completion gate, not this milestone's check.
12. **Cancel during decision.** A turn with two calls. The hook for call 1 cancels the parent context and returns a valid decision. Neither call executes (no file written; no MCP call), the hook is not called for call 2, and `Run` returns an error with `IsRuntimeError` true and `errors.Is(err, context.Canceled)` true (R9).
13. **Unknown action (R8 case).** Table cases: a hook returns `ToolAction("")` with empty `Content`; and a hook returns an unknown action (for example `ToolAction("bogus")`) with non-empty `Content`. In each case the call is denied, the tool result has `IsError: true` and `Content` equal to the fixed OD1 text `tool call denied by host policy` (the decision `Content` is ignored, OD9), and the run continues.

Each MCP test (3-5) uses a local helper in `pkg/runner/hook_mcp_test.go` that creates the go-sdk `mcp.NewServer`, registers one tool, and serves it with `mcp.NewStreamableHTTPHandler` under `httptest.NewServer`, closed by `t.Cleanup`. The helper counts calls with an atomic counter. The agent TOML uses `[[mcp_servers]]` with `transport = "streamable-http"` and the server URL, as in `cmd/run_mcp_integration_test.go:65-69`.

### R14: Scope boundary

- Changed tracked files for this milestone are: `pkg/runner/hook.go` (new), `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/run_test.go`, and `pkg/runner/hook_mcp_test.go` (new), plus one `[Unreleased]` `Added` bullet in `CHANGELOG.md` for `Options.ToolHook`. The CHANGELOG entry is required by `CONTRIBUTING.md:185` ("Update CHANGELOG.md with your changes"); it is the only `CHANGELOG.md` change.
- `go.mod`, `go.sum`, `cmd/`, `internal/`, `COMPATIBILITY.md`, `README.md`, `AGENTS.md`, and `Makefile` are not changed (OD8, milestone scope).
- No golden file changes (`cmd/testdata/golden/`). No CLI flag, TOML key, or `--json` field.
- Verification: `git diff --name-only 09d100d` lists only the six files above plus Factory records.

## Parallelizable work

- R1 (`hook.go` types) must exist before R2-R9 compile. Once it exists, the decision pass and the parallel and sequential wiring are one change in `pkg/runner/run.go`.
- R10 (sub-agent check) and R11 (nil-hook behavior) touch different lines of `run.go` and can be made in the same edit pass.
- The MCP test helper (R13, tests 3-5) and the non-MCP tests (R13) are independent files and can be written in parallel.

## Serial dependencies

- R1 before R2-R9 (types must exist).
- R2-R9 before R13 (tests exercise the wiring).
- R12 doc comments are written with R1.
- R1-R14 before the milestone's focused test and gate (R13 test 11).

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                       # expect: exit 0
go vet ./pkg/runner/                                                 # expect: exit 0
go test ./pkg/runner/                                                # expect: exit 0
rg -n "ToolHook" pkg/runner/options.go pkg/runner/hook.go            # expect: the field and the type
rg -n "tool call denied by host policy" pkg/runner/                  # expect: exactly one constant definition
git diff --name-only 09d100d -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_004_*'   # expect: CHANGELOG.md, pkg/runner/hook.go, hook_mcp_test.go, options.go, run.go, run_test.go
git diff --exit-code 09d100d -- go.mod go.sum cmd internal cmd/testdata   # expect: exit 0
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_004_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since 09d100d /home/jaron/dev/axe/runner-tool-hooks                                                   # expect: exit 0
```

The full `go test -race ./...` run is Factory's completion gate, not a check of this spec.

## Failure modes

1. **Hook interleaved with dispatch in parallel turns.** A goroutine dispatches before a later hook call. Mitigation: the decision pass completes for every call before the parallel branch starts (R2, test 7).
2. **Raw error text leaks to the model.** A hook error message reaches the tool result. Mitigation: R7 and test 8 assert absence.
3. **Hook mutates arguments.** A hook changes a map that is later dispatched. Mitigation: OD3 copy (R5, test 6).
4. **Cancelled turn still dispatches.** The parallel branch starts after cancellation. Mitigation: the `ctx.Err()` check happens before any dispatch of the turn (R9).
5. **Sub-agent run bypasses the hook.** A `call_agent` path runs without the hook. Mitigation: R10 rejects a hook with a non-empty `sub_agents` list before any provider call.
6. **Dry run calls the hook.** Mitigation: the dry-run return (`pkg/runner/run.go:279`) is before the decision pass, and the milestone states dry runs do not invoke hooks.
7. **Panic escapes the decision pass.** Mitigation: R7 recovers panics from the hook call only.
8. **Cancellation error dropped at the call site.** `executeToolCalls` returns the cancellation error but the caller ignores it, so the turn continues. Mitigation: the single call site at `pkg/runner/run.go:662` returns the error from the run (R2, R9, test 12).
9. **Unknown action leaks host `Content`.** A hook returns an unknown action with non-empty `Content`, and that content reaches the model. Mitigation: OD9 and R8, with test 13's second case asserting the fixed text.

## Limitations

- The MCP tests (3-5) exercise the streamable-HTTP transport only. The `sse` transport is not covered by this milestone.
- The decision pass is verified in-process through `run()` and `Run()`. The CLI cannot set a hook (D6), so the CLI path is not tested for hooks.
- Concurrency with shared hooks across `Run` calls is documented (OD6), not tested.
- The OD1 fixed text is verified by substring absence, not by an exact-match golden.
