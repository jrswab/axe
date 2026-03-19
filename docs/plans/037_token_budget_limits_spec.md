# 037 — Token Budget Limits Per Agent Run

**Issue:** https://github.com/jrswab/axe/issues/19
**Priority:** High
**Status:** Spec

---

## Section 1: Context & Constraints

### Associated Issue

> Allow users to set a maximum token or cost budget per agent run to prevent runaway costs when agents fan out.
>
> **Motivation:** Raised on HN: unintentionally fanning out multiple agents with smaller context windows can be more expensive than a single large context window. Users need a guardrail.
>
> **Proposed behavior:**
> - `[budget]` config block in agent TOML (e.g. `max_tokens = 10000` or `max_cost_usd = 0.05`)
> - Hard stop when limit is reached; return partial result with error exit code
> - `--max-tokens` CLI flag override

### Research Findings

#### Codebase Structure Relevant to This Feature

- **Token tracking already exists.** `cmd/run.go` maintains `totalInputTokens` and `totalOutputTokens` variables that accumulate `resp.InputTokens` and `resp.OutputTokens` after every `prov.Send()` call in the conversation loop. These are printed in verbose mode and included in JSON output.

- **Sub-agents do NOT report token usage to parents.** `ExecuteCallAgent` in `internal/tool/tool.go` returns a `provider.ToolResult` which contains only `CallID`, `Content`, and `IsError`. There is no mechanism to propagate token counts from sub-agent back to parent. Sub-agents run their own independent conversation loop (`runConversationLoop` in `internal/tool/tool.go`) with their own provider instance.

- **`ExecuteOptions` is the struct passed to sub-agents.** Defined in `internal/tool/tool.go`, it carries `AllowedAgents`, `ParentModel`, `Depth`, `MaxDepth`, `Timeout`, `GlobalConfig`, `MCPRouter`, `Verbose`, `Stderr`. This is the natural place to add a shared budget tracker.

- **Parallel sub-agent execution is the default.** Multiple sub-agents can run concurrently via goroutines in `executeToolCalls` in `cmd/run.go`. Any shared budget mechanism must be thread-safe.

- **The conversation loop has a hard limit of 50 turns** (`maxConversationTurns` constant in `cmd/run.go`). Budget enforcement is a second, orthogonal limit.

- **`AgentConfig` in `internal/agent/agent.go`** already has nested config blocks: `SubAgentsConf SubAgentsConfig`, `Memory MemoryConfig`, `Params ParamsConfig`, `Retry RetryConfig`. A new `Budget BudgetConfig` block follows the established pattern.

- **`Validate()` in `internal/agent/agent.go`** checks constraints on all config blocks. Budget validation follows the same pattern.

- **`Scaffold()` in `internal/agent/agent.go`** generates a commented TOML template for new agents. Budget block must be added here.

- **CLI flags are defined in `cmd/run.go`** on the `runCmd` cobra command. Resolution order is: CLI flag > TOML config > default (0 = unlimited).

- **Exit codes** are mapped via `ExitError` struct in `cmd/exit.go`. Current codes: 0 (success), 1 (runtime error), 2 (config error), 3 (transient provider error).

- **JSON output envelope** in `cmd/run.go` is a `map[string]interface{}` with fields like `input_tokens`, `output_tokens`, `stop_reason`, etc.

- **`provider.Response`** struct in `internal/provider/provider.go` contains `InputTokens int` and `OutputTokens int`.

#### Decisions Already Made

1. **Shared budget across parent + sub-agents.** Sub-agent token usage counts against the parent's total budget. This directly addresses the fan-out cost problem from the issue. A shared, thread-safe budget tracker is passed from parent to all sub-agents.

2. **Token-based limits only (no cost-based).** `max_cost_usd` is deferred. Cost calculation requires per-model pricing data that changes frequently. Token counts are already tracked and sufficient for guardrails. Users can estimate cost from token counts externally.

3. **Partial result on budget exceeded.** When the budget is exhausted, the agent returns whatever content was generated so far (the last successful LLM response), prints a budget-exceeded warning to stderr, and exits with a non-zero exit code. This preserves potentially useful work.

4. **Shared struct via `ExecuteOptions`.** The budget tracker is a pointer to a thread-safe struct added to `ExecuteOptions`. The parent creates it; sub-agents share the same pointer. No context.Context magic.

#### Approaches Ruled Out

- **Cost-based budgets (`max_cost_usd`):** Requires a pricing table or user-provided cost-per-token config. Deferred to a future spec.
- **Per-sub-agent budgets:** Each sub-agent having its own independent budget from its own TOML does not solve the fan-out problem. All agents in a run share the parent's budget.
- **Context.Value for budget propagation:** Harder to discover and test than an explicit struct field.
- **Budget warnings before exhaustion (e.g., "80% used"):** Nice-to-have, not in scope for this spec.
- **Budget for MCP tool calls:** MCP calls do not consume LLM tokens directly. Out of scope.

#### Constraints and Assumptions

- **Backward compatibility:** Budget defaults to 0 (unlimited). Existing agents with no `[budget]` block behave identically to today.
- **Token counting granularity:** Budget tracks the sum of `InputTokens + OutputTokens` from every `provider.Response` across all agents in the run. This is the total tokens billed by the LLM provider.
- **Budget is per-run, not per-agent-definition.** If the same agent TOML is invoked twice (two separate `axe run` invocations), each run gets its own fresh budget.
- **The response that crosses the budget boundary is still returned.** Budget is checked *before* the next LLM call, not mid-call. The LLM call that pushes usage over the limit completes, its tokens are recorded, and then the loop stops. This means actual usage may exceed `max_tokens` by up to one response's worth of tokens.
- **Sub-agents that are already running when the budget is exceeded will complete their current LLM call** before seeing the exceeded state. This is a consequence of checking before each call, not interrupting in-flight requests.

#### Open Questions Resolved

- **Q: Should `params.max_tokens` and `budget.max_tokens` conflict?** No. `params.max_tokens` controls the maximum *response* length sent to the LLM API (per-call). `budget.max_tokens` controls the cumulative *total* tokens (input + output) across the entire run. They are orthogonal.
- **Q: What exit code for budget exceeded?** Exit code 4 (new). Distinct from runtime errors (1), config errors (2), and transient provider errors (3). This allows callers to distinguish budget exhaustion from other failures.

---

## Section 2: Requirements

### 2.1 Agent TOML Configuration

A new optional `[budget]` configuration block must be supported in agent TOML files.

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_tokens` | integer | 0 | Maximum total tokens (input + output) allowed across the entire run, including all sub-agent calls. 0 means unlimited. |

**Validation rules:**
- `max_tokens` must be non-negative. A negative value is a config error.

**Example:**
```toml
[budget]
max_tokens = 10000
```

### 2.2 CLI Flag Override

A `--max-tokens` flag must be added to the `axe run` command.

**Behavior:**
- Type: integer
- Default: 0 (meaning "use TOML value or unlimited")
- When set to a positive value, it overrides the TOML `[budget].max_tokens` value.
- When set to 0 (default), the TOML value is used. If TOML is also 0 or absent, budget is unlimited.

**Resolution order:** `--max-tokens` flag (if > 0) → TOML `[budget].max_tokens` → 0 (unlimited).

### 2.3 Budget Tracker

A shared, thread-safe budget tracker must be used to accumulate token usage across the parent agent and all sub-agents in a single run.

**Behaviors:**
- The tracker is created once at the start of the run by the parent agent.
- The same tracker instance is shared with all sub-agents (passed explicitly, not via context).
- Every successful `Provider.Send()` response adds its `InputTokens + OutputTokens` to the tracker.
- The tracker must be safe for concurrent use (multiple parallel sub-agents may call it simultaneously).
- When `max_tokens` is 0, the tracker never reports exceeded. All operations are no-ops or return "unlimited" equivalents.

### 2.4 Budget Enforcement

Before each LLM call in the conversation loop (both parent and sub-agent loops), the budget must be checked.

**Behavior when budget is exceeded:**
1. The current conversation loop stops immediately (no further LLM calls).
2. The last successful LLM response content is preserved as the partial result.
3. A warning message is printed to stderr: `"budget exceeded: used %d of %d tokens"` (with actual values).
4. The run exits with exit code 4.
5. If the agent is a sub-agent, it returns its partial result to the parent as a normal `ToolResult` (not an error result), but the parent will also see the budget is exceeded on its next check and stop.

**Boundary behavior:**
- The LLM call that causes usage to cross the limit is allowed to complete. Its tokens are recorded. The loop stops *after* that call, before the next one.
- This means actual usage may exceed `max_tokens` by up to one response's worth of tokens. This is expected and acceptable.

**Single-shot mode (no tools):**
- Budget is checked after the single LLM call. If exceeded, the response is still returned but the exit code is 4 and the stderr warning is printed. (In practice, a single call rarely exceeds a budget, but the check must be consistent.)

### 2.5 Sub-Agent Budget Propagation

The shared budget tracker must be passed from parent to sub-agents through the existing sub-agent execution mechanism.

**Behaviors:**
- The parent's budget tracker is included in the options passed to sub-agent execution.
- Sub-agents use the same tracker instance — they do not create their own.
- Sub-agents that have their own `[budget]` block in their TOML are ignored for budget purposes when running as a sub-agent. The parent's budget governs the entire run. (A sub-agent's own budget only applies when that agent is run directly as a top-level agent.)
- If a sub-agent's LLM call causes the shared budget to be exceeded, the sub-agent stops and returns its partial result. The parent, on its next budget check, also sees the budget is exceeded and stops.

### 2.6 Exit Code

A new exit code must be introduced:

| Exit Code | Meaning |
|-----------|---------|
| 4 | Budget exceeded |

This is distinct from all existing exit codes (0 = success, 1 = runtime error, 2 = config error, 3 = transient provider error).

### 2.7 JSON Output

When `--json` is used, the output envelope must include budget metadata when a budget is configured (i.e., `max_tokens > 0`).

**Additional fields in JSON envelope:**

| Field | Type | Condition | Description |
|-------|------|-----------|-------------|
| `budget_max_tokens` | integer | Always present when budget > 0 | The configured max_tokens budget |
| `budget_used_tokens` | integer | Always present when budget > 0 | Total tokens consumed (input + output, all agents) |
| `budget_exceeded` | boolean | Always present when budget > 0 | Whether the budget was exceeded |

When no budget is configured (max_tokens = 0), these fields must be omitted from the JSON output to maintain backward compatibility.

### 2.8 Verbose Output

When `--verbose` is used and a budget is configured, the existing token summary line in stderr must also include budget information.

**Current format:** `"Tokens: %d input, %d output (cumulative)"`
**New format when budget is active:** `"Tokens: %d input, %d output (cumulative, budget: %d/%d)"`

Where the last two numbers are used tokens and max tokens from the budget tracker.

### 2.9 Scaffold Template

The `axe agents init` scaffold template must include a commented-out `[budget]` block showing the available option:

```toml
# [budget]
# max_tokens = 0
```

### 2.10 Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| `max_tokens = 0` (default) | No budget enforcement. Behaves identically to current behavior. |
| `max_tokens` negative in TOML | Config validation error. Exit code 2. |
| `--max-tokens` negative on CLI | Flag parsing error (cobra handles this for uint/int flags). |
| Budget exceeded on first LLM call | The response is returned, exit code 4, stderr warning. |
| Budget exceeded during sub-agent | Sub-agent returns partial result. Parent stops on next check. Exit code 4. |
| Multiple parallel sub-agents exceed budget simultaneously | All concurrent calls that are already in-flight complete. Each adds its tokens. The tracker's thread-safe state reflects the total. All agents stop on their next check. |
| Sub-agent has its own `[budget]` in TOML but is called as sub-agent | The sub-agent's own budget config is ignored. The parent's shared tracker governs. |
| `--max-tokens 5000` with TOML `max_tokens = 10000` | CLI wins. Budget is 5000. |
| `--max-tokens 0` with TOML `max_tokens = 10000` | TOML wins. Budget is 10000. (0 on CLI means "use TOML value".) |
| Agent with no tools, single LLM call uses 500 tokens, budget is 100 | Response is returned (the call completed), exit code 4, stderr warning. Used: 500, max: 100. |
| Memory append after budget exceeded | Memory is NOT appended on budget-exceeded runs (exit code != 0 means the run did not fully succeed). |
