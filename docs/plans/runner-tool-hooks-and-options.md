# 003 axe requirements: changes needed in `github.com/jrswab/axe`

Status: requirements draft for Jaron. No code change is authorized by this document. It changes no file outside its own path.

Scope: the axe changes that milestone 003 needs. Later milestones (004, 008, 009) are covered only where they change the axe design now.

Evidence labels:

- **Existing**: the item exists today. Each claim cites `path:line`.
- **Proposed**: a new API or behavior. Go signatures in this document are proposals. They do not exist in axe.
- **Conflict**: the recon or a source disagrees with the code. Section 6 lists each conflict.

Base versions:

- axe: `/home/jaron/dev/axe/master`, branch `master`, HEAD `20e6ada179b08cef050c4b436a2497182efcf2ee`, tag `v1.11.0`.
- nightshift: `/home/jaron/dev/nightshift/feature-oncall-offload-v1`, branch `feature/oncall-offload-v1`, HEAD `b3d447c995725862c39e09b0b776e66da042b51c`.

---

## 1. Purpose and context

### 1.1 What nightshift does

Nightshift is an on-call offload agent. It reads Sentry and Better Stack alerts. It queues them. For each alert, it selects a playbook and runs an LLM agent through axe. The agent writes a triage note. The agent may also take state actions (assign, resolve, ignore, comment, acknowledge) when a playbook allows them.

Milestone 003 covers the first end-to-end triage run. It uses one alert, read-only tools, and dry-run stubs for every external write. Source: `docs/plans/001_oncall-offload-agent-v1_milestones.md:286-291`.

Milestone 003 acceptance, in short (`docs/plans/001_oncall-offload-agent-v1_milestones.md:288`):

1. The intercept hook rejects any write tool or argument not in the playbook `actions`.
2. The hook rejects note or comment text without the marker.
3. Dry-run emits the note and planned actions to stdout, with no vendor write.
4. The worker runs `generic` when nothing matches.
5. Sentry text sits in a delimited "untrusted alert data" block.
6. The Better Stack `X-MCP-Tools-Only` allowlist is checked server-side with the hook.

### 1.2 Why axe must change

The milestone names the hook at `pkg/runner/run.go:979`, which is `dispatchMCPToolCall`. That function handles MCP calls only. It does not see builtin tools. Source: `pkg/runner/run.go:979-990` and the branch at `pkg/runner/run.go:927-940`.

A hook at that point cannot reject a builtin write tool. It cannot see `call_agent`. It cannot stub a call before dispatch for every tool kind. The hook must move up to the shared dispatch point. Section 3 gives the detail.

Other gaps:

- axe has no public way to pass MCP servers from code. `Options` has no MCP field. Source: `pkg/runner/options.go:6-33`.
- axe has no public way to set the LLM API key or base URL from code. Both come from environment variables or `config.toml`. Source: `internal/config/config.go:81-82`, `:99-100`.
- axe's `DryRun` returns before MCP connect and before provider construction. It cannot run a stubbed triage. Source: `pkg/runner/run.go:279`, `:358`, `:437`.
- Provider errors cannot be classified from outside axe. Milestone 008 needs this. Source: `pkg/runner/errors.go:77`, `:86`.

### 1.3 Scope split

Nightshift builds its own parts: playbook selection, the tool policy, the prompt builder, and the untrusted-alert block. Section 4 lists them. Axe changes only add the seams that nightshift needs.

---

## 2. Summary of required changes

Priority legend:

- **Required for 003**: 003 cannot meet an acceptance item without it.
- **Strongly recommended**: 003 can run without it, through a workaround. The workaround is named.
- **Nice to have**: no milestone needs it now.
- **Required for later milestone**: needed by 004, 008, or 009. Not needed by 003.

| ID | Name | Priority | Blocks which 003 acceptance item |
|----|------|----------|----------------------------------|
| A1 | Pre-dispatch tool-call hook (allow, deny, stub), covering MCP, builtin, and `call_agent` | Required for 003 | Write tool rejection (`:288`); argument rejection (`:288`); dry-run stubs every external write (`:287`, `:288`); marker rejection (`:288`, via the hook's view of arguments) |
| A2 | Post-dispatch observer with full, untruncated results and decision | Strongly recommended for 003 (dry-run output can also use the hook closure). Required for 004 audit. | Dry-run stdout "emits the note and planned actions" (`:288`) can be done in nightshift's own hook closure. The observer is for 004 audit and for the after-state record. |
| A3 | Argument fidelity: raw JSON arguments on `ToolCall` | Conditional. Required only if the note text or status is nested. See Q4. | Marker check (`:288`) and `update_issue` status check (`:288`, `t18:42`) |
| A4 | `Options.MCPServers` (programmatic MCP config, with `${VAR}` header expansion and typed env errors) | Strongly recommended for 003. Workaround: generate an agent TOML into `Options.AgentsDirs`. | MCP servers from code (`:290`); Better Stack header from env (`t18:46`) |
| A5 | `Options.Tools` (programmatic tool list, validated like `agent.go:176-180`) | Nice to have | Only allowlisted tools and `read_file`/`list_directory` (`:287`). Workaround: generate the TOML `tools` list. |
| A6 | `Options.MaxTurns` (per-run turn cap) | Strongly recommended for 003 and required for 008 and 009. Workaround: generate TOML `max_turns`. | Per-run triage `max_turns` 30 (playbook default at `playbook/parse.go:77`; 008 `milestones:319`) |
| A7 | Provider seam: `Options.APIKey` and `Options.BaseURL` | Strongly recommended for 003 tests. Workaround: env vars and `$XDG_CONFIG_HOME` with `t.Setenv`, which serializes tests. | Stub-only tests (`:289`, `:291`) |
| A8 | Typed provider-error classification exported from `pkg/runner` (category, HTTP status, message) | Required for 008. Not needed by 003. | None for 003. Blocks 008 `429/1009` breaker. |
| A9 | Retry rules: `1009` non-retryable, and a per-run retry override | Required for 008. Workaround: `max_retries = 0` in generated TOML, which works today. | None for 003. Blocks 008 `:321`. |

Section 3 gives each change in full.

---

## 3. Changes

### A1. Pre-dispatch tool-call hook

#### Current behavior

- Sequential tool dispatch: `pkg/runner/run.go:927-940`. The branch order is:
  1. If `mcpRouter != nil && mcpRouter.Has(tc.Name)`, call `dispatchMCPToolCall` (`run.go:931-932`).
  2. Else if `tc.Name == tool.CallAgentToolName`, call `tool.ExecuteCallAgent` (`run.go:933-934`).
  3. Else call `registry.Dispatch` (`run.go:936`). A dispatch error becomes an `IsError` result (`run.go:937`).
- Parallel dispatch repeats the same three branches inside goroutines: `pkg/runner/run.go:952-966`.
- The dispatch function is chosen per call: `pkg/runner/run.go:979-990` (`dispatchMCPToolCall`) for MCP calls only.
- Sub-agents: `internal/tool/tool.go:420-421` and `:455-476` (`dispatchToolCall`) repeat the MCP-first logic.
- The shared call point: `pkg/runner/run.go:662`, `executeToolCalls` defined at `pkg/runner/run.go:890`.
- The parallel flag: `pkg/runner/run.go:655-658`.
- Tool errors go back to the model as results, not as fatal errors. Source: `pkg/runner/run.go:937`, and `AGENTS.md:165` ("Tool errors - Returned as tool results to LLM, not fatal").
- Tool results are appended as a `tool` message: `pkg/runner/run.go:685-689`.
- `ToolCall` and `ToolResult` types: `pkg/runner/messages.go:98-110` (public) and `internal/provider/provider.go:41-53` (internal). `ToolCall.Arguments` is `map[string]string` in both.

#### Problem

1. The named seam is MCP-only. Builtin calls (`registry.Dispatch`, `run.go:936`, `:960`) and `call_agent` (`run.go:933`) never reach it.
2. Sub-agents use another dispatch path (`internal/tool/tool.go:455-476`). A hook at the top loop does not cover them.
3. Nightshift cannot stub a call before it runs. Stubbing needs a pre-dispatch decision point.
4. The parallel branch dispatches inside goroutines (`run.go:952`). A hook there runs in an unordered way.

#### Proposed API

Public types and option (new file `pkg/runner/hook.go`, proposed; existing types are at `pkg/runner/messages.go:98-110`):

```go
// ToolAction is the decision a ToolHook returns for one tool call.
type ToolAction int

const (
	ToolAllow ToolAction = iota // Dispatch the call as today.
	ToolDeny                    // Do not dispatch. Return Content to the model as an error result.
	ToolStub                    // Do not dispatch. Return Content to the model as a normal result.
)

// ToolDecision is the result of a ToolHook call.
type ToolDecision struct {
	Action  ToolAction
	Content string // Deny reason, or stub result text. Ignored for ToolAllow.
}

// ToolHook decides how to handle one tool call before dispatch.
// It runs for every call: builtin, MCP, and call_agent.
type ToolHook func(ctx context.Context, call ToolCall) (ToolDecision, error)
```

Option field (proposed, added to `pkg/runner/options.go:6-33`):

```go
// ToolHook, when non-nil, decides every tool call before dispatch.
ToolHook ToolHook
```

Internal wiring (proposed): add `hook ToolHook` to `executeToolCalls` (`pkg/runner/run.go:890`) and `dispatchToolCall` (`internal/tool/tool.go:455`). Add the same field to `tool.ExecuteOptions` (`internal/tool/tool.go:29`). The internal type cannot be public, so it uses a copy of the decision type. Keep the public and internal types in step.

#### Semantics

Order of operations per turn (proposed):

1. Run the hook for every call in `resp.ToolCalls`, in order, on the calling goroutine. Do this before any dispatch in that turn.
2. For each call with `ToolAllow`, dispatch with today's branches (`run.go:931-938`). Use the existing parallel rule (`run.go:927`).
3. For each call with `ToolDeny`, set `ToolResult{CallID, Content, IsError: true}`.
4. For each call with `ToolStub`, set `ToolResult{CallID, Content, IsError: false}`.

Why decide first, then dispatch: hook calls then run in model order, not goroutine order. This keeps the audit trail stable. It also keeps the hook out of the parallel path.

Edge cases:

- **Refusal shape.** A denied call returns an `IsError` result to the model (`run.go:937` pattern). The run continues. The model can adapt. The turn limit still bounds the loop (`run.go:587`, `:695`). A run-aborting refusal is not proposed. See Q2.
- **Hook returns an error.** Treat it as `ToolDeny`. The model sees a fixed text, for example `tool call denied by policy`. Log the error to stderr when `Verbose` is set. This is fail closed. Do not return the raw error text to the model, because it may hold data from the alert.
- **Hook panics.** Recover, then treat as `ToolDeny`. Fail closed.
- **Unknown tool names.** The hook still runs first. Unknown names keep today's result: `unknown tool %q` (`internal/tool/registry.go:84`) or `unknown MCP tool %q` (`internal/mcpclient/router.go:45`).
- **Context.** The hook receives the run context (`run.go:332` timeout context). It must return quickly. Axe adds no per-hook timeout.
- **Concurrency.** The hook runs on one goroutine per turn, so it need not be safe for concurrent calls. The hook is still called from the run goroutine only. Document this.
- **No hook.** When `ToolHook == nil`, behavior matches today exactly. No new call, no new result.
- **Rewrite.** This proposal has no rewrite action. Allow, deny, or stub only. See Q1.
- **Sub-agents.** When `ToolHook` is set and the agent has `sub_agents`, return `ConfigError` before the run starts. Do not run sub-agent calls without the hook. This is fail closed. Propagation into `internal/tool/tool.go:455` is a later step. See Q3.
- **Dry-run.** `Options.DryRun` is unchanged. It still returns at `run.go:279-334`. Stubbing happens through `ToolHook` on a normal run. The worker connects MCP, lists tools, and calls read tools.

#### How nightshift uses it

Proposed nightshift shape (not code, for reference):

```go
hook := func(ctx context.Context, call runner.ToolCall) (runner.ToolDecision, error) {
    if !policy.Allows(call.Name, call.Arguments) { // ToolPolicy derived from playbook actions
        return runner.ToolDecision{Action: runner.ToolDeny, Content: "denied: not in playbook actions"}, nil
    }
    if policy.IsWrite(call.Name) && dryRun {
        rec.PlannedAction(call) // nightshift records the planned action
        return runner.ToolDecision{Action: runner.ToolStub, Content: "dry-run: not sent"}, nil
    }
    return runner.ToolDecision{Action: runner.ToolAllow}, nil
}
```

The marker check (milestone `:288`) is a second check in the same closure. It reads the note text from `call.Arguments`. See A3 for the limit on argument text.

#### Acceptance tests axe should add

Place tests in `pkg/runner/run_test.go` using the `run()` seam and the `capturingProvider` pattern (`pkg/runner/run_test.go:44-59`, `:92-106`). Add an MCP test using the pattern from `cmd/run_mcp_integration_test.go:17-46`.

1. No hook: output and tool results match the current baseline.
2. Hook denies a builtin write tool (for example `write_file`): the tool is not run. The model gets `IsError: true`.
3. Hook denies an MCP tool: the MCP server records zero calls for that tool.
4. Hook stubs an MCP tool: the MCP server records zero calls. The model gets the stub text, not `IsError`.
5. Hook allows a read MCP tool: the call reaches the MCP server.
6. Hook receives `call.Arguments` exactly as the provider parsed them.
7. Parallel turn with three calls: the hook runs three times in model order, before any dispatch.
8. Hook returns an error: the call is denied, and the raw error text does not appear in the tool result.
9. Hook panics: the call is denied, and the run completes.
10. `ToolHook` set and `sub_agents` configured: `Run` returns `ConfigError` before any provider call.
11. `go test -race ./...` passes for the parallel cases.

#### Backward compatibility

- Additive. Existing callers do not set `ToolHook`. The default path is unchanged.
- The public `ToolDecision` and `ToolHook` names are new. They must not collide with existing names in `pkg/runner`.

---

### A2. Post-dispatch observer and full results

#### Current behavior

- Tool details are kept only in JSON mode: `pkg/runner/run.go:664` (`if opts.JSON`), with the append at `:670`.
- The result `Output` is truncated to 1024 bytes: `pkg/runner/run.go:35` (`maxToolOutputBytes = 1024`), `:673` (`truncateOutput(...)`), `:992-1000` (`truncateOutput`).
- `ToolCallDetails` is on `Result` at `pkg/runner/result.go:64`. It is set at `pkg/runner/run.go:728`.
- A normal `Run` (not JSON) returns `ToolCallDetails` as `nil` in practice. Nightshift cannot read the full result after dispatch without JSON mode.

#### Problem

Milestone 004 needs an audit record "before and after each state change" (`milestones:293-295`). The before record comes from A1. The after record needs the real result, not a 1024-byte prefix. Current axe ties the detail record to JSON mode and truncates it.

#### Proposed API

```go
// ToolEvent describes one tool call after its decision and dispatch.
type ToolEvent struct {
	Call     ToolCall
	Action   ToolAction   // The decision from ToolHook. ToolAllow when no hook is set.
	Result   ToolResult   // Full result. Not truncated.
	Duration time.Duration
}

// ToolObserver receives every tool call after decision and dispatch.
type ToolObserver func(ctx context.Context, ev ToolEvent)
```

Option field (proposed):

```go
// ToolObserver, when non-nil, is called once per tool call after it completes.
ToolObserver ToolObserver
```

#### Semantics

- Call the observer on the run goroutine, after the result is ready.
- For the parallel path, call the observer after the goroutines join, in model order. This avoids concurrent calls to the observer.
- Call it for `ToolDeny` and `ToolStub` too. The `Action` field says which.
- `Result.Content` is full text. Truncation applies only to the existing `ToolCallDetails` output.
- The observer must not change the result. Its return is nothing.
- Observer errors do not exist. The observer returns nothing. A panic in the observer is not recovered. Document this. The observer is the caller's code.

#### How nightshift uses it

- For milestone 003: not required. The stub text is also in nightshift's hook closure.
- For milestone 004: nightshift writes the "after" record for each write call from `ToolEvent`.

#### Acceptance tests axe should add

1. Observer called once per call, in model order, for both sequential and parallel turns.
2. A 4 KB tool output reaches the observer in full. `ToolCallDetails` still truncates as today.
3. Observer receives `ToolDeny` and `ToolStub` events with the right `Action`.
4. Observer set and `JSON` false: `ToolCallDetails` is still `nil`, which keeps the current contract.

#### Backward compatibility

Additive. The observer is nil by default. The JSON envelope does not change.

---

### A3. Argument fidelity

#### Current behavior

- Provider parsing flattens arguments to strings: `internal/provider/openai.go:338-344` (`args[k] = fmt.Sprintf("%v", v)`), `internal/provider/anthropic.go:331` (same pattern). Other providers repeat the pattern: `bedrock.go:280`, `gemini.go:351`, `ollama.go:303`, `opencode.go:308`, `openrouter.go:237`.
- A nested object or array becomes a Go-syntax string such as `map[...]` or `[...]`. It is not JSON. Source: `fmt.Sprintf("%v")` at the same lines.
- Streaming repeats the same flattening: `pkg/runner/run.go:852`.
- MCP argument coercion is by schema type only (`internal/mcpclient/mcpclient.go:94-100`, `:124`), and it handles numbers and booleans. It does not handle nested values.

#### Problem

The marker check (`milestones:288`) and the `update_issue` status check (`t18:42`) read `call.Arguments`. If the note text is a nested field, the string is Go syntax. The check can then miss a marker or misread a status. The result would be a false allow or a false deny.

#### Proposed API

Add a raw field to the public `ToolCall`, populated by each provider parser (proposed):

```go
type ToolCall struct {
	ID           string
	Name         string
	Arguments    map[string]string
	RawArguments json.RawMessage // Proposed. The provider's JSON object, unmodified. May be nil.
}
```

Also expose it on `ToolEvent` if A2 is accepted.

#### Semantics

- `RawArguments` holds the provider JSON object text. It is set only when the provider gives JSON. For a provider that does not, it is `nil`.
- `Arguments` keeps its current meaning. Do not change it.
- Nightshift parses `RawArguments` for nested checks. If it is `nil`, the check fails closed (deny).
- Streaming (`run.go:852`) must set `RawArguments` too.

#### How nightshift uses it

Only if the Sentry and Better Stack note schemas nest the text. Check the MCP `tools/list` schemas first. Then decide. See Q4.

#### Acceptance tests axe should add

1. OpenAI parser: nested argument object appears in `RawArguments` as valid JSON, and `Arguments` is unchanged.
2. Anthropic parser: same check.
3. Streaming path: same check.
4. Argument with a nested array: `Arguments` value is the current `%v` string. `RawArguments` is the JSON.

#### Backward compatibility

Additive field. Existing `Arguments` behavior does not change. Each provider's tests need a new assertion.

---

### A4. `Options.MCPServers`

#### Current behavior

- The TOML type is `internal/agent/agent.go:114-119` (`MCPServerConfig`: `Name`, `URL`, `Transport`, `Headers`).
- The agent field is `internal/agent/agent.go:135`.
- Validation: `internal/agent/agent.go:183-206`. It checks name, URL, scheme, host, transport (`sse` or `streamable-http`, at `:201-202`), and unique names. The duplicate-name check uses `seenMCPNames` (`agent.go:183`).
- Connect: `internal/mcpclient/mcpclient.go:25-50`. Headers are expanded at `:26`. The transport switch is at `:33-41`.
- Env expansion: `internal/envinterp/envinterp.go:9` (regex), `:12` (`ExpandHeaders`), `:47` (error `environment variable %q is not set or empty`).
- The runner maps connect errors to `ConfigError` by matching the error text: `pkg/runner/run.go:443`. It matches `"environment variable"` and `"unsupported MCP transport"`. A text change breaks the mapping.
- MCP connect loop: `pkg/runner/run.go:428-441`.
- `Options` has no MCP field: `pkg/runner/options.go:6-33`.
- `DryRunInfo.MCPServers` shows name, URL, and transport only, not headers: `pkg/runner/run.go:296-300`.
- Tool names from MCP are registered raw, without a server prefix. A collision with a builtin name is skipped: `internal/mcpclient/router.go:22-39`. See Section 8, C13.

#### Problem

Milestone 003 requires MCP server config from code (`milestones:290`). The Better Stack token must come from an env var through a header (`t18:46`). Today the only way is to write an agent TOML file into a directory. That works, but it adds file lifecycle, a second validation step, and a fragile env error check.

#### Proposed API

```go
// MCPServer is an MCP server connection supplied from code.
// Its fields match agent.MCPServerConfig (internal/agent/agent.go:114-119).
type MCPServer struct {
	Name      string
	URL       string
	Transport string            // "sse" or "streamable-http"
	Headers   map[string]string // Raw values. ${VAR} is expanded at connect time, as today.
}
```

Option field (proposed):

```go
// MCPServers are added to the MCP servers from the agent TOML.
// Names must be unique across both sources.
MCPServers []MCPServer
```

#### Semantics

- Merge: the runner builds one list from `cfg.MCPServers` and `opts.MCPServers`. A duplicate name returns `ConfigError`. This matches `agent.go:183-188`.
- Validate `opts.MCPServers` with the same rules as `internal/agent/agent.go:184-206`. Run validation before any connect. A bad server returns `ConfigError` with the same messages.
- Expansion: keep `${VAR}` unexpanded in the public struct. Expand in `mcpclient.Connect` (`mcpclient.go:26`) as today.
- Missing env var: return a typed error from `internal/envinterp` (proposed sentinel `ErrUnsetEnv`, wrapped with `%w`). The runner maps it to `ConfigError` with `errors.Is`. This replaces the text match at `run.go:443`. Keep the text match as a fallback for one release.
- Logging: never print header values. The verbose line at `run.go:439` prints the URL only. Keep it that way.
- Dry-run display: `DryRunInfo.MCPServers` (`run.go:296-300`) includes `Options.MCPServers` too. Headers stay hidden.

#### How nightshift uses it

Build one `MCPServer` per vendor. Headers hold `${VAR}` references. Nightshift passes no token value. The env var name comes from the worker config.

#### Acceptance tests axe should add

1. Server from `Options.MCPServers` only: the tool list includes its tools.
2. Server from both sources: both sets of tools appear. Duplicate names return `ConfigError`.
3. Invalid transport in `Options.MCPServers`: `ConfigError` before connect.
4. Header `Bearer ${VAR}` with `VAR` unset: `ConfigError`, found with `errors.Is`, not by text.
5. Header value is expanded at connect. The HTTP test server sees the expanded header.
6. `DryRunInfo.MCPServers` includes the code-supplied server and no header text.
7. Existing TOML-only tests pass unchanged.

#### Backward compatibility

Additive. `MCPServer` is a new type. TOML behavior is unchanged. The text-match fallback keeps existing errors stable.

---

### A5. `Options.Tools` (nice to have)

#### Current behavior

- Agent `tools` are validated against `toolname.ValidNames()`: `internal/agent/agent.go:176-180`.
- Tools resolve at `pkg/runner/run.go:413-419` (`registry.Resolve(cfg.Tools)`).
- `call_agent` is added when `sub_agents` is set: `pkg/runner/run.go:423-425`.

#### Problem

Milestone 003 says the run uses only allowlisted MCP tools and `read_file`/`list_directory` (`milestones:287`). A generated TOML can set `tools = ["read_file", "list_directory"]` today. So this is a convenience, not a blocker.

#### Proposed API

```go
// Tools, when non-nil, replaces the agent TOML tool list. Names are validated
// the same way as agent.go:176-180.
Tools []string
```

#### Semantics

- `nil` means use the agent TOML list. An empty non-nil slice means no builtin tools.
- Validation errors return `ConfigError`, as at `run.go:416`.
- The model sees only the listed tools in `req.Tools` (`run.go:413-419`). The hook (A1) is still the enforcement point.

#### Acceptance tests axe should add

1. `Options.Tools` replaces the TOML list.
2. Unknown name returns `ConfigError`.
3. Empty non-nil slice: the request has no builtin tools.

#### How nightshift uses it

Nightshift sets `Tools: []string{"read_file", "list_directory"}` for triage runs. It sets `nil` for runs that need no change. The MCP servers still come from A4.

#### Backward compatibility

Additive. `nil` keeps today's behavior.

---

### A6. `Options.MaxTurns`

#### Current behavior

- `maxConversationTurns := cfg.MaxTurns`, default `defaultMaxConversationTurns` (`50`) when zero: `pkg/runner/run.go:83-86`, and the constant at `:33`.
- The loop bound is `run.go:587`. The limit error is `RuntimeError` at `run.go:695`.
- Agent TOML field: `internal/agent/agent.go` (`MaxTurns` at `:131`, validated at `:167`).
- `Options` has no turn field: `pkg/runner/options.go:6-33`.
- Playbook `triage.max_turns` defaults to 30 (`playbook/parse.go:77`). Playbook `code_fix.max_turns` is a separate field (`playbook/parse.go:58-62`).

#### Problem

Each playbook sets its own turn cap. Nightshift can write one agent TOML per run with the right `max_turns`. That works today. But a per-run override avoids a file per run.

#### Proposed API

```go
// MaxTurns, when > 0, overrides the agent TOML max_turns. Zero keeps the agent value.
MaxTurns int
```

#### Semantics

- `0` means keep the agent value (keeps the default of 50 when both are zero).
- Negative values return `ConfigError`, as `agent.go:167` does.
- Same limit error at `run.go:695`.

#### Acceptance tests axe should add

1. `MaxTurns: 2` stops the loop after two turns with the existing `RuntimeError`.
2. `MaxTurns: 0` uses the agent value.
3. Negative value returns `ConfigError`.

#### How nightshift uses it

Nightshift sets `MaxTurns` from the matched playbook `triage.max_turns` (`playbook/parse.go:65-69`, default 30 at `:77`). Until A6 lands, it writes the same value into the generated agent TOML.

#### Backward compatibility

Additive. `0` is the zero value, so existing callers see no change.

---

### A7. Provider seam for tests and config

#### Current behavior

- Provider name is parsed from `model` (`pkg/runner/run.go:353-358`).
- API key: `globalCfg.ResolveAPIKey(provName)` (`run.go:336`). Source: `internal/config/config.go:81-82`. The env var is `APIKeyEnvVar` (`config.go:72`, mapping at `:58-67`). `config.toml` is the fallback.
- Base URL: `globalCfg.ResolveBaseURL(provName)` (`run.go:337`). Source: `internal/config/config.go:99-100`. The env var is `AXE_<PROVIDER>_BASE_URL`.
- Config file: `config.Load()` reads `$XDG_CONFIG_HOME/axe/config.toml` (`internal/config/config.go:30-36`). A missing file is tolerated (`config.go:38-41`).
- Config directory: `internal/xdg/xdg.go:30`.
- Provider constructor: `internal/provider/registry.go:24` (`New`). Supported names at `registry.go:6`. There is no `gloo` provider. Gloo is reached through `openai` with a custom base URL (`internal/provider/openai.go:15` comment).
- Anthropic base URL: `internal/provider/anthropic.go:14`, `WithBaseURL` at `:28`.
- In-package tests can inject a fake provider through `run(ctx, opts, newProvider)` (`pkg/runner/run.go:47`, `pkg/runner/run_test.go:92-106`). External callers cannot.
- `internal/testutil/mockserver.go:50` (`NewMockLLMServer`) is internal and cannot be imported by nightshift.

#### Problem

Nightshift tests need a fake LLM. Today they set `AXE_OPENAI_BASE_URL` and `OPENAI_API_KEY`, with `t.Setenv`. `t.Setenv` cannot run in parallel tests. The API key also appears in the process environment. Gloo is also a configuration of the `openai` provider, so nightshift must set both fields.

#### Proposed API

```go
// APIKey, when non-empty, is used instead of the environment and config.toml lookup.
APIKey string

// BaseURL, when non-empty, is used instead of the environment and config.toml lookup.
BaseURL string
```

#### Semantics

- Precedence: `Options` value, then env var, then `config.toml` (the current order at `config.go:82-98` and `:100-115`).
- A missing `config.toml` stays tolerated (`config.go:38-41`). With both fields set, no env var is needed.
- Do not log either value.
- The provider name still comes from `Model` (`run.go:353`).

#### Acceptance tests axe should add

1. `Options.BaseURL` set, no env vars, `Options.APIKey` set: the request reaches the `httptest` server.
2. Both set: env vars are ignored.
3. Both unset: current behavior.
4. A test runs in parallel (`t.Parallel()`) with no `t.Setenv`.

#### How nightshift uses it

Nightshift's 003 stub tests set `BaseURL` to an `httptest` LLM server and `APIKey` to a fake value. The Gloo base URL is an `openai` provider base URL (`internal/provider/openai.go:15`). Nightshift never sets a real key in tests (`milestones:291`).

#### Backward compatibility

Additive. Empty strings keep the current lookup.

---

### A8. Typed provider-error classification

#### Current behavior

- `ProviderError` is internal: `internal/provider/provider.go:113-118` (`Category`, `Status`, `Message`, `Err`).
- Categories: `internal/provider/provider.go:9-23`, including `rate_limit`, `timeout`, `overloaded`, `server`, `auth`, `bad_request`.
- Public functions: `pkg/runner/errors.go:77` (`ProviderCategory(err) (provider.ErrorCategory, bool)`) and `:86` (`AsProviderError(err, target **provider.ProviderError) bool`).
- `AsProviderError` cannot be called from outside axe in practice. Its parameter type names an internal type. Nightshift cannot declare `**provider.ProviderError`.
- `ProviderCategory` can be called from outside. Its result converts with `string(cat)` because the underlying type is `string`. The type cannot be named, but the value can be compared after conversion.
- `Status` is not reachable from outside.
- Fatal provider errors map to `RuntimeError`: `pkg/runner/run.go:1019-1030`.
- The OpenAI error body struct is `internal/provider/openai.go:141-145`. It is not known whether the Gloo `1009` code survives into `Message`. This is open (Q9).

#### Problem

Milestone 008 needs `429` and `1009` distinguished. It needs the HTTP status and a stable category. Outside axe, only `string(category)` is reachable, and `Status` is not reachable at all.

#### Proposed API

```go
// ProviderErrorInfo is a public view of a provider error.
type ProviderErrorInfo struct {
	Category string // Stable value: "auth", "rate_limit", "timeout", "overloaded", "bad_request", "server", and others.
	Status   int    // HTTP status, or 0 when unknown.
	Message  string
}

// ProviderErrorInfoOf returns the provider error view of err, if any.
func ProviderErrorInfoOf(err error) (ProviderErrorInfo, bool)
```

Keep `ProviderCategory` and `AsProviderError` (`errors.go:77`, `:86`). Mark them deprecated in their doc comments. Do not remove them.

#### Semantics

- `Category` is a string, not `provider.ErrorCategory`, so callers need no internal type.
- `Status` is taken from `ProviderError.Status`.
- A new category for Gloo quota (`1009`) is out of scope here. It depends on the Gloo error body (Q9).

#### Acceptance tests axe should add

1. A `429` from the mock server gives `Category: "rate_limit"`, `Status: 429`.
2. A `500` gives `Category: "server"`, `Status: 500`.
3. A non-provider error returns `false`.
4. `errors.As` on the wrapped `RuntimeError` still works.

#### How nightshift uses it

For 008, the worker maps `rate_limit` with `Status` 429 and the Gloo `1009` text to the circuit breaker. It maps `auth` and `bad_request` to a non-retry failure. The `1009` mapping depends on Q9.

#### Backward compatibility

Additive. Old functions stay. Their signatures do not change.

---

### A9. Retry: `1009` and per-run override

#### Current behavior

- Retry wrapper: `internal/provider/retry.go:51-115`. Zero retries passes straight through (`retry.go:52-53`).
- Retriable categories: `internal/provider/retry.go:135-148` (`isRetriable`). It retries `rate_limit`, `server`, `overloaded`, and `timeout` without checking the body or any code. There is no `1009` or `Retry-After` handling. A grep of `internal/` and `pkg/` for `1009` returns no matches.
- Streaming is not retried: `internal/provider/retry.go:123-124`.
- `max_retries = 0` is valid: `internal/agent/agent.go:210-211` allows zero, and `retry.go:52` passes through. A generated TOML can set it today.
- Retry config is per agent: `internal/agent/agent.go:86-95` (`RetryConfig`), `:140` (field).
- Milestone text: `milestones:321` ("Requires an upstream axe fix or `max_retries: 0`"), `milestones:176-179`, `milestones:389`.

#### Problem

Gloo's `429 / 1009` (monthly cap) must not be retried (`milestones:176`, `milestones:227`). Axe retries any `rate_limit` (`retry.go:143-148`). Nightshift can avoid retry with `max_retries = 0`, which works today. The fix is needed only if nightshift wants retries on other errors.

#### Proposed API

Option A (minimal, recommended for 008): none in axe. Nightshift writes `max_retries = 0` into the generated TOML. Document this as the supported path.

Option B (if retries are wanted): add a per-run field and a classifier hook.

```go
// MaxRetries, when non-nil, overrides the agent TOML retry.max_retries.
MaxRetries *int

// RetryClassifier, when non-nil, decides whether an error is retried.
RetryClassifier func(err error) bool
```

Option B depends on A8 for the classifier input.

#### Semantics

- Option A: no code change. Document the TOML setting in the runner docs.
- Option B: `RetryClassifier` replaces `isRetriable` for that run. A `false` result stops the retry at once.

#### Acceptance tests axe should add

Option A: a test that `max_retries = 0` sends one request on a `429`.

#### How nightshift uses it

Option A is the 008 path today. The worker writes `max_retries = 0` into the generated agent TOML (`internal/agent/agent.go:210-211`). Option B is needed only if nightshift wants retries on errors other than `1009`.

Option B: a test that a classifier returning `false` sends one request on a `429`. A test that `MaxRetries: nil` keeps the agent value.

#### Backward compatibility

Option A: none. Option B: additive, and `nil` keeps the current rule.

---

## 4. Out of scope for axe (stays in nightshift)

- **Playbook selection.** `playbook.Select` (`playbook/select.go:15`) and the generic fallback (`playbook/select.go:43-47`). Nightshift owns this. The `generic` fallback is at `playbook/select.go:43-47`.
- **Tool policy.** The policy derives from playbook `actions` (`playbook/parse.go:51-55`). Global ceilings are checked at lint time (`playbook/validate.go:124-125`). Axe does not know playbooks. The policy is a closure passed to A1.
- **Untrusted alert block.** Nightshift wraps the Sentry text. The `playbook.Alert` struct carries no message, frames, or breadcrumbs (`playbook/match.go:11-17`). Nightshift must fetch and wrap that text. The system prompt rule belongs to nightshift.
- **Note marker.** The check is in nightshift's hook closure. The marker text is in the spec: `[oncall-agent] triage v1 · run <run-id> · playbook <name>@<image-tag>` (`.scratch/wayfinder/oncall-offload-v1-t6.md:24`). Axe does not know the marker.
- **Per-server tool filter.** `t18` chose vendor-side limits (Sentry `?skills=`, Better Stack `X-MCP-Tools-Only`). Axe adds no per-server filter. See Q5.
- **Intake, queue, dedupe, Sentry and Better Stack clients.** Nightshift code (`intake/handoff.go:18-20`, `intake/worker.go:23-29`, `:97`).
- **Audit store and cost ledger.** Nightshift code (`t14`, `t15`).
- **Dry-run stdout.** Nightshift prints the note and the planned actions.
- **Changing `Options.DryRun`.** Its early return (`run.go:279-334`) stays as it is.

---

## 5. Release and integration plan

### 5.1 Versioning

- Current: `cmd/root.go:12` `const Version = "1.11.0"`. The version test asserts it at `cmd/version_test.go:19`.
- Changelog: `CHANGELOG.md:8` `## [Unreleased]`, then `## [1.11.0] - 2026-10-21` at `:13`.
- Release trigger: tag push matching `v*` (`.github/workflows/release.yml:5-8`). The release notes come from the matching CHANGELOG section.
- All A1-A9 options are additive. Under semver for a 1.x module, that is a minor bump. Proposed: `v1.12.0`. See Q10.

Steps for axe (Jaron, in `/home/jaron/dev/axe`):

1. Implement A1 (and any other accepted items) with tests.
2. Add a CHANGELOG entry under `## [Unreleased]`.
3. Bump all three version strings to match: `cmd/root.go:12`, `cmd/version_test.go:19`, and `internal/mcpclient/mcpclient.go:43`. Run `go test ./cmd/ -run TestVersion -count=1` to confirm.
4. Run `make check` (`Makefile:10`).
5. Tag and push `v1.12.0`. Jaron pushes. This doc does not authorize a push.

### 5.2 Nightshift consumption

- Nightshift `go.mod` has no axe requirement. Its module is `github.com/jrswab/nightshift`, `go 1.25.0` (`go.mod:1-3`).
- Add the requirement with `go get github.com/jrswab/axe@v1.12.0`. This needs the module in the local module cache. Assumption: the proxy is offline, so the version must already be cached. Verify before the add.
- Do not commit a `replace` directive to a local path. Use the tagged version.
- For local development before the tag, use a temporary `replace` in a scratch `go.work`, not in `go.mod`. Assumption: `go.work` is not committed.

### 5.3 Dependency impact

- axe `go.mod` requires `golang.org/x/sys v0.45.0` (`axe go.mod:20`). Nightshift `go.mod:17` has `golang.org/x/sys v0.23.0` as indirect. Adding axe raises it to `v0.45.0`.
- axe pulls `github.com/modelcontextprotocol/go-sdk` (`internal/mcpclient/mcpclient.go` imports it). This adds a new module to nightshift's graph. Assumption: the module is in the offline cache. Verify with `go mod download -json` before merging.
- axe `go.mod` also lists `BurntSushi/toml` and `spf13/cobra`. `cobra` is used by `cmd/`, which nightshift does not import. Assumption to verify: `go list -deps github.com/jrswab/axe/pkg/runner` shows no `cobra`.
- Go toolchain: axe's CI uses `go-version: '1.24'` (`.github/workflows/go.yml:19`, `:32`, `:47`), while `go.mod` says `go 1.25.0`. Nightshift uses `go 1.25.0`. See Section 8, C14.

### 5.4 Order of work

1. Jaron decides Q1-Q12.
2. Jaron implements A1 and the tests in axe. Then A4 and A7 if accepted for 003.
3. Jaron tags the release.
4. Nightshift adds the dependency offline, then writes the 003 worker against the new API.
5. Nightshift proves the stub path with `httptest` servers and the fake LLM. No live Gloo call (`milestones:291`).

---

## 6. Open questions for Jaron

Each question has a recommendation.

**Q1. Rewrite action in the hook.** Should the hook be able to change arguments before dispatch?
Recommendation: no, for v1. Allow, deny, or stub only. A rewrite makes it unclear what the audit record holds.

**Q2. Denial shape.** Should a denied call return an error result to the model, or abort the run?
Recommendation: return an error result. The model can adapt, and `max_turns` bounds the loop. Abort is harder to test and hides the reason from the model.

**Q3. Sub-agents with a hook.** Should a hook apply to `call_agent` children, or should axe refuse the combination?
Recommendation: refuse with `ConfigError` for 003, since 003 uses no sub-agents. Propagate the hook in a later release.

**Q4. Argument fidelity.** Are the Sentry and Better Stack note text and status top-level string fields?
Recommendation: check the MCP `tools/list` schema first. If top-level, `Arguments` is enough and A3 is not needed. If nested, add A3.

**Q5. MCP tool filter.** Should axe filter MCP tools by name per server, or rely on the vendor limits and the hook?
Recommendation: rely on the vendor limits and the hook (`t18`). Add no per-server filter.

**Q6. Observer or closure.** For 003, should nightshift record dry-run output in its own hook closure, or wait for A2?
Recommendation: closure for 003. Build A2 before 004.

**Q7. Provider seam.** Should A7 (`Options.APIKey`, `Options.BaseURL`) be in the first axe release, or should tests use `t.Setenv`?
Recommendation: include A7. It removes process-wide env changes from nightshift tests.

**Q8. MCP config path.** Should nightshift use `Options.MCPServers` (A4) or a generated TOML in `Options.AgentsDirs`?
Recommendation: A4. The milestone names it (`milestones:290`). The TOML path is a fallback if A4 slips.

**Q9. Gloo quota error shape.** What does a Gloo `429 / 1009` response body look like? Does the code survive into `ProviderError.Message`?
Recommendation: Jaron supplies one sample response from `t16` or Gloo documentation. Do not make a live call from an agent. Until then, A8 classifies only by HTTP status and category.

**Q10. Version number.** Should the axe release be `v1.12.0` (minor, additive) or `v2.0.0`?
Recommendation: `v1.12.0`. All changes are additive.

**Q11. Option names.** Are these names acceptable: `ToolHook`, `ToolObserver`, `MCPServers`, `Tools`, `MaxTurns`, `APIKey`, `BaseURL`?
Recommendation: yes. Keep the names parallel to the existing `Options` fields (`options.go:6-33`).

**Q12. Parallel dispatch.** Should the hook decide every call in a turn before dispatch (Section A1, proposed), or per call inside the goroutine?
Recommendation: decide first. Model order is kept, and the hook needs no locking.

---

## 7. Reference: file index

### 7.1 axe (`/home/jaron/dev/axe/master`)

| Topic | Location |
|-------|----------|
| `Run` entry | `pkg/runner/run.go:43` |
| Internal `run` with provider factory | `pkg/runner/run.go:47` |
| Load global config | `pkg/runner/run.go:95` |
| Turn limit default and read | `pkg/runner/run.go:33`, `:83-86` |
| `maxToolOutputBytes` | `pkg/runner/run.go:35` |
| Max-token override | `pkg/runner/run.go:269-271` |
| Dry-run early return | `pkg/runner/run.go:279-334` |
| Timeout context | `pkg/runner/run.go:332` |
| Resolve API key and base URL | `pkg/runner/run.go:336-337` |
| Create provider | `pkg/runner/run.go:353-358` |
| Retry wrap | `pkg/runner/run.go:364-374` |
| Registry and `RegisterAll` | `pkg/runner/run.go:410-411` |
| Resolve `cfg.Tools` | `pkg/runner/run.go:413-419` |
| `call_agent` injection | `pkg/runner/run.go:423-425` |
| MCP connect loop | `pkg/runner/run.go:428-441` |
| Connect error to `ConfigError` (text match) | `pkg/runner/run.go:443` |
| DryRunInfo MCP list | `pkg/runner/run.go:296-300`, `:307` |
| Turn loop | `pkg/runner/run.go:587` |
| Parallel flag | `pkg/runner/run.go:655-658` |
| `executeToolCalls` call | `pkg/runner/run.go:662` |
| Tool detail (JSON only) | `pkg/runner/run.go:664`, `:670`, `:673` |
| Tool result message | `pkg/runner/run.go:685-689` |
| Refusal flag | `pkg/runner/run.go:730` |
| Turn-limit error | `pkg/runner/run.go:695` |
| `executeToolCalls` definition | `pkg/runner/run.go:890` |
| Sequential branch | `pkg/runner/run.go:927-940` |
| Parallel branch | `pkg/runner/run.go:952-966` |
| `dispatchMCPToolCall` | `pkg/runner/run.go:979-990` |
| `truncateOutput` | `pkg/runner/run.go:992` |
| `mapProviderError` | `pkg/runner/run.go:1019` |
| Argument flattening (streaming) | `pkg/runner/run.go:852` |
| Public `Options` | `pkg/runner/options.go:6-33` |
| Public `ToolCallDetail` | `pkg/runner/result.go:6-12` |
| `ToolCallDetails` on `Result` | `pkg/runner/result.go:64` |
| Public `ToolCall`, `ToolResult` | `pkg/runner/messages.go:98`, `:105` |
| Public error functions | `pkg/runner/errors.go:77`, `:86` |
| Internal `ToolCall`, `ToolResult` | `internal/provider/provider.go:41`, `:48` |
| `ProviderError` | `internal/provider/provider.go:113-118` |
| Error categories | `internal/provider/provider.go:9-23` |
| Provider constructor `New` | `internal/provider/registry.go:24` |
| Supported providers | `internal/provider/registry.go:6` |
| OpenAI argument flattening | `internal/provider/openai.go:338-344` |
| OpenAI error body | `internal/provider/openai.go:141-145` |
| OpenAI and Gloo base URL comment | `internal/provider/openai.go:15` |
| Anthropic argument flattening | `internal/provider/anthropic.go:331` |
| Anthropic default base URL | `internal/provider/anthropic.go:14`, `WithBaseURL` at `:28` |
| Retry pass-through at zero | `internal/provider/retry.go:52-53` |
| Retry `isRetriable` | `internal/provider/retry.go:135-148` |
| Streaming not retried | `internal/provider/retry.go:123-124` |
| Registry dispatch | `internal/tool/registry.go:82-92` |
| Registry `RegisterAll` | `internal/tool/registry.go:97` |
| Sub-agent options | `internal/tool/tool.go:29` |
| Sub-agent MCP-first loop | `internal/tool/tool.go:420-421` |
| Sub-agent `dispatchToolCall` | `internal/tool/tool.go:455-476` |
| MCP TOML struct | `internal/agent/agent.go:114-119` |
| MCP field on agent | `internal/agent/agent.go:135` |
| Tool name validation | `internal/agent/agent.go:176-180` |
| MCP validation | `internal/agent/agent.go:183-206` |
| Transport check | `internal/agent/agent.go:201-202` |
| Max turns validation | `internal/agent/agent.go:167` |
| Retry config and zero check | `internal/agent/agent.go:86-95`, `:210-211` |
| MCP connect and header expansion | `internal/mcpclient/mcpclient.go:25-50`, `:26`, `:33-41` |
| MCP `CallTool` | `internal/mcpclient/mcpclient.go:94-100` |
| MCP `coerceArg` | `internal/mcpclient/mcpclient.go:124` |
| MCP router register and skip | `internal/mcpclient/router.go:22`, `:26-28`, `:34` |
| MCP router dispatch and has | `internal/mcpclient/router.go:42-46`, `:51` |
| Env var expansion | `internal/envinterp/envinterp.go:9`, `:12`, `:47` |
| Global config load | `internal/config/config.go:30-41` |
| API key lookup | `internal/config/config.go:72`, `:81-82` |
| Base URL lookup | `internal/config/config.go:99-100` |
| XDG config directory | `internal/xdg/xdg.go:30` |
| Fake provider and env helper (tests) | `pkg/runner/run_test.go:20-38`, `:44-59`, `:92-106` |
| MCP integration test server (tests) | `cmd/run_mcp_integration_test.go:17-46` |
| Mock LLM server (internal) | `internal/testutil/mockserver.go:50` |
| Version constant and test | `cmd/root.go:12`, `cmd/version_test.go:19` |
| Changelog | `CHANGELOG.md:8`, `:10` |
| Release trigger | `.github/workflows/release.yml:5-8` |
| CI Go version | `.github/workflows/go.yml:19`, `:32`, `:47` |
| Module requirements | `go.mod:3` (`go 1.25.0`), `:20` (`x/sys v0.45.0`) |
| Tool errors policy | `AGENTS.md:165` |
| Namespacing claim (not implemented) | `AGENTS.md:199` (see Section 8, C13) |

### 7.2 nightshift (`/home/jaron/dev/nightshift/feature-oncall-offload-v1`)

| Topic | Location |
|-------|----------|
| 003 milestone entry | `docs/plans/001_oncall-offload-agent-v1_milestones.md:286-291` |
| 004 entry | `docs/plans/001_oncall-offload-agent-v1_milestones.md:293-295` |
| 007 entry (acknowledge gate) | `docs/plans/001_oncall-offload-agent-v1_milestones.md:311-313` |
| 008 entry | `docs/plans/001_oncall-offload-agent-v1_milestones.md:317-321` |
| 009 entry | `docs/plans/001_oncall-offload-agent-v1_milestones.md:323-327` |
| 010 entry | `docs/plans/001_oncall-offload-agent-v1_milestones.md:330-334` |
| 1009 and retry notes | `docs/plans/001_oncall-offload-agent-v1_milestones.md:176-179`, `:227`, `:389` |
| Axe upstream list | `docs/plans/001_oncall-offload-agent-v1_milestones.md:369` |
| Handoff interface | `intake/handoff.go:9-20` (`Accepted` at `:9`, `Handoff` at `:18`) |
| Worker deps | `intake/worker.go:23-29` |
| Worker handoff call | `intake/worker.go:97` |
| Deliberate no-handoff and error | `cmd/nightshift/work.go:24`, `:30` |
| Playbook `Actions` | `playbook/parse.go:51-55` |
| Playbook `CodeFix` | `playbook/parse.go:58-62` |
| Playbook `Triage` and default | `playbook/parse.go:65-69`, `:77` |
| Selection and generic fallback | `playbook/select.go:15`, `:43-47` |
| Actions ceilings | `playbook/validate.go:124-125` |
| `Alert` struct | `playbook/match.go:11-17` |
| `LoadDir` | `playbook/load.go:12` |
| Module | `go.mod:1-3` |

### 7.3 Wayfinder records (`.scratch/wayfinder/`)

| Record | Location |
|--------|----------|
| t18: intercept hook, `${VAR}`, tool limits | `oncall-offload-v1-t18.md:18`, `:19`, `:32`, `:33`, `:40`, `:42`, `:46` |
| t6: marker, note template | `oncall-offload-v1-t6.md:23-24` |
| t7: untrusted block | `oncall-offload-v1-t7.md:25`, `:37` |
| t8: note-first, marker | `oncall-offload-v1-t8.md:22-23` |
| t14: audit record, hook for writes, no event bus | `oncall-offload-v1-t14.md:27`, `:33` |
| t13: axe chosen over pi | `oncall-offload-v1-t13.md:33` |

---

## 8. Conflicts between recon and the code

These conflicts were found while checking the source. Each one is marked by whether it changes behavior.

| # | Source said | Code says | Changes behavior? |
|---|-------------|-----------|-------------------|
| C1 | Recon R2: nightshift "cannot name or compare `provider.ErrorCategory`." | Nightshift can call `ProviderCategory` and convert the result with `string(cat)`. It cannot call `AsProviderError`, because its parameter type is internal (`pkg/runner/errors.go:86`). | Yes, for A8. `Status` is also unreachable. |
| C2 | Recon R9: `Run` needs a config file. | A missing `config.toml` is tolerated (`internal/config/config.go:38-41`). Env vars alone work. | Yes, for test setup. Only the API key and base URL need env vars. |
| C3 | Milestone `:290` and `t18`: "`Options.MCPServers` or equivalent." | The generated-TOML path works today (`Options.AgentsDirs`, `agent.go:135`). | No. It changes the priority of A4 to Strongly recommended. |
| C4 | Milestone `:321`: "upstream axe fix or `max_retries: 0`." | `max_retries = 0` works today (`internal/provider/retry.go:52`, `internal/agent/agent.go:210-211`). | No. It makes A9 Option A viable now. |
| C5 | Recon: the hook seam is `run.go:929`/`:952`. | Sequential branch starts at `run.go:927`, parallel goroutine at `:952`. | No. Line only. |
| C6 | Orchestrator: `Options` at `options.go:6-38`. | The struct ends at `options.go:33`. | No. Line only. |
| C7 | Orchestrator: builtin bypass at `run.go:961-963`. | Builtin dispatch is at `run.go:936` (sequential) and `run.go:960` (parallel). | No. Line only. |
| C8 | Orchestrator: test base URL at `config.go:16,30`. | Base URL lookup is `config.go:99-100`. File load is `config.go:30-36`. | No. Line only. |
| C9 | Recon: the arg coercion is at `mcpclient.go:110-121`. | It is at `mcpclient.go:124` (`coerceArg`), called from `:94-100`. | No. Line only. |
| C10 | Recon: `registry.go:102-104` registers builtins. | `RegisterAll` is at `internal/tool/registry.go:97`. | No. Line only. |
| C11 | Recon: `envinterp.go:25-52`. | Regex at `:9`, `ExpandHeaders` at `:12`, error at `:47`. | No. Line only. |
| C12 | Recon: `1009` absent in `internal/` and `pkg/`. | Confirmed: no matches. | Confirms R8. |
| C13 | Recon R7: AGENTS.md says tools are namespaced. | Confirmed: the claim is at `AGENTS.md:199`. Code registers raw names (`router.go:34`). | Docs only. Do not cite AGENTS.md for MCP behavior. |
| C14 | Recon R7: CI uses Go 1.24 while `go.mod` requires 1.25.0. | Confirmed: `.github/workflows/go.yml:19`, `:32`, `:47` use `'1.24'`. `go.mod:3` says `go 1.25.0`. | No runtime effect. Build depends on toolchain download. |

