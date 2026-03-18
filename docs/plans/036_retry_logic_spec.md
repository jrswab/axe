# Specification: Retry Logic for Intermittent LLM Provider Failures

**Status:** Draft  
**Version:** 1.0  
**Created:** 2026-03-17  
**GitHub Issue:** [#29 — feat: retry logic for intermittent LLM provider failures](https://github.com/jrswab/axe/issues/29)  
**Scope:** Configurable retry with backoff for transient LLM provider errors, implemented as a provider decorator

---

## 1. Context & Constraints

### Associated Milestone

This is a standalone feature. There is no milestone document — the work is scoped by GitHub Issue #29.

Issue #29 requests configurable retry logic when an upstream LLM provider returns intermittent failures mid-pipeline. In multi-agent pipelines, a transient 429/503 from a provider can fail an entire unattended workflow with no human to retry manually.

### Codebase Structure Relevant to This Work

| File | Role | Lines |
|------|------|-------|
| `internal/provider/provider.go` | `Provider` interface, `ProviderError` struct, `ErrorCategory` constants | 103 |
| `internal/provider/openai.go` | OpenAI provider — `Send()` method, HTTP call, error categorization | 361 |
| `internal/provider/anthropic.go` | Anthropic provider — same pattern as OpenAI | 359 |
| `internal/provider/gemini.go` | Gemini provider — same pattern | 412 |
| `internal/provider/ollama.go` | Ollama provider — same pattern | 367 |
| `internal/provider/opencode.go` | OpenCode gateway provider | 731 |
| `internal/provider/registry.go` | `New()` factory function for creating providers | 68 |
| `internal/agent/agent.go` | `AgentConfig` struct, `Validate()`, `Load()`, `Scaffold()` | 246 |
| `internal/config/config.go` | `GlobalConfig` struct — global config.toml | 107 |
| `cmd/run.go` | Agent execution pipeline — two `prov.Send()` call sites (lines 362, 392), `--json` envelope (line 470), `mapProviderError()` | 689 |
| `internal/testutil/mockserver.go` | `MockLLMServer` for testing providers | 220 |

**Current Provider interface:**
```go
type Provider interface {
    Send(ctx context.Context, req *Request) (*Response, error)
}
```

**Current ProviderError structure:**
```go
type ProviderError struct {
    Category ErrorCategory  // auth, rate_limit, timeout, overloaded, bad_request, server
    Status   int            // HTTP status code
    Message  string
    Err      error
}
```

**Current error categories and their HTTP status codes:**

| Category | Status Codes | Providers |
|----------|-------------|-----------|
| `auth` | 401, 403 | All |
| `bad_request` | 400, 404 | All |
| `rate_limit` | 429 | All |
| `overloaded` | 529 | Anthropic, Gemini |
| `server` | 500, 502, 503 | All |
| `timeout` | (context deadline exceeded) | All |

**Current execution flow (cmd/run.go):**
1. `provider.New()` creates a provider instance (line 225)
2. `prov.Send(ctx, req)` is called — single-shot at line 362, or in conversation loop at line 392
3. On error, `mapProviderError(err)` converts to `ExitError` with exit code 3 (retriable categories) or 1 (bad request)
4. No retry — every failure is immediately terminal

**Current `--json` output envelope (cmd/run.go line 470):**
```go
envelope := map[string]interface{}{
    "model":             resp.Model,
    "content":           resp.Content,
    "input_tokens":      totalInputTokens,
    "output_tokens":     totalOutputTokens,
    "stop_reason":       resp.StopReason,
    "duration_ms":       durationMs,
    "tool_calls":        totalToolCalls,
    "tool_call_details": allToolCallDetails,
    "refused":           refusal.Detect(resp.Content),
}
```

**Current AgentConfig struct (internal/agent/agent.go):**
```go
type AgentConfig struct {
    Name          string            `toml:"name"`
    Description   string            `toml:"description"`
    Model         string            `toml:"model"`
    SystemPrompt  string            `toml:"system_prompt"`
    Skill         string            `toml:"skill"`
    Files         []string          `toml:"files"`
    Workdir       string            `toml:"workdir"`
    Tools         []string          `toml:"tools"`
    MCPServers    []MCPServerConfig `toml:"mcp_servers"`
    SubAgents     []string          `toml:"sub_agents"`
    SubAgentsConf SubAgentsConfig   `toml:"sub_agents_config"`
    Memory        MemoryConfig      `toml:"memory"`
    Params        ParamsConfig      `toml:"params"`
}
```

Existing sub-config patterns to follow: `MemoryConfig`, `ParamsConfig`, `SubAgentsConfig` — all use `toml` struct tags and are validated in `Validate()`.

### Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| **Provider decorator pattern** | Retry logic lives in a new `RetryProvider` that wraps any `Provider`. Clean separation of concerns, single responsibility, testable in isolation. Both `Send()` call sites in `cmd/run.go` benefit transparently without code duplication. |
| **Per-agent config only (no global config)** | Follows the existing pattern (`[memory]`, `[params]`). Keeps config close to the agent. Global defaults can be added in a future iteration. |
| **No third-party backoff library** | Exponential/linear/fixed backoff with jitter is ~30 lines of Go. Keeps the zero-runtime, minimal-dependency philosophy (currently 4 deps). |
| **Default: no retries (max_retries = 0)** | Backward compatible. Existing agents behave identically unless they opt in to retry. |

### Approaches Ruled Out

| Approach | Why Ruled Out |
|----------|--------------|
| **Retry at caller level in cmd/run.go** | Would require duplicating retry logic in two call sites (single-shot line 362 and conversation loop line 392). The decorator pattern handles both transparently. |
| **Global-only config** | No per-agent control. Different agents may talk to different providers with different rate limit behaviors. |
| **Third-party backoff library (cenkalti/backoff)** | Adds a dependency for ~30 lines of code. Violates "single binary, zero runtime" spirit. |
| **Retry inside individual provider Send() methods** | Would require modifying 5+ provider files. The decorator wraps any provider uniformly. |

### Constraints

- **No new external dependencies.** All changes use Go stdlib only.
- **Existing tests must continue to pass without modification.** The decorator is opt-in; when `max_retries = 0` (default), the wrapper is a no-op passthrough.
- **Red/green TDD required.** Tests are written first (red), then implementation (green).
- **Stdout must remain clean and pipeable.** Retry log messages go to stderr via `--verbose` only.
- **Context cancellation must be respected.** If the parent context is cancelled during a backoff sleep, the retry must abort immediately — not sleep through the cancellation.
- **The retry decorator must not modify the `Request` or `Response` objects.** It is purely a retry-on-error wrapper.
- **Jitter is always applied for exponential backoff.** This prevents thundering herd when multiple agents retry simultaneously. Jitter is not configurable — it is a correctness concern, not a preference.

### Open Questions Resolved

| # | Question | Answer |
|---|----------|--------|
| 1 | Should retry stats be available in `--json` output? | **Yes.** A `retry_attempts` field will be added to the JSON envelope. This requires the decorator to communicate retry count back to the caller. |
| 2 | Should the retry decorator return the last error or an aggregate? | **Last error.** The final `ProviderError` from the last attempt is returned. The retry count is communicated separately. |
| 3 | Should backoff jitter be configurable? | **No.** Jitter is always applied for exponential backoff (standard practice to prevent thundering herd). For `fixed` and `linear`, no jitter is applied — the delay is deterministic. |
| 4 | What happens if `max_retries` is set but `backoff` is empty? | **Default to `"exponential"`.** Empty string is treated as the default, not an error. |
| 5 | What happens if `initial_delay_ms` is 0 or unset? | **Default to 500ms.** Zero is treated as "use default", not "no delay". |
| 6 | What happens if `max_delay_ms` is 0 or unset? | **Default to 30000ms (30s).** Zero is treated as "use default". |
| 7 | Should the conversation loop retry individual turns or the whole conversation? | **Individual turns.** The decorator wraps `Send()`, so each call to `Send()` within the conversation loop gets its own retry budget. This is correct — a 429 on turn 3 should retry turn 3, not restart from turn 1. |

---

## 2. Requirements

### R1: Retry Configuration

**R1.1 (Config Struct):** The agent configuration must support a `[retry]` section with the following fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_retries` | integer | 0 | Maximum number of retry attempts after the initial request. 0 means no retries. |
| `backoff` | string | `"exponential"` | Backoff strategy: `"exponential"`, `"linear"`, or `"fixed"`. |
| `initial_delay_ms` | integer | 500 | Base delay in milliseconds before the first retry. |
| `max_delay_ms` | integer | 30000 | Maximum delay in milliseconds. Computed delays exceeding this value are capped. |

**R1.2 (Defaults Applied at Runtime):** When the `[retry]` section is absent or fields are zero-valued, runtime defaults must be applied. The TOML parser will produce zero values for omitted fields. The runtime must treat zero values as "use default" for `backoff`, `initial_delay_ms`, and `max_delay_ms`. For `max_retries`, zero means "no retries" (this is the actual default, not a sentinel).

**R1.3 (Backward Compatibility):** When `[retry]` is absent or `max_retries = 0`, agent behavior must be identical to the current behavior — a single attempt with no retry.

### R2: Validation

**R2.1 (max_retries Range):** `max_retries` must be non-negative. Negative values must produce a validation error.

**R2.2 (backoff Values):** `backoff` must be one of: `"exponential"`, `"linear"`, `"fixed"`, or empty string (treated as default `"exponential"`). Any other value must produce a validation error.

**R2.3 (initial_delay_ms Range):** `initial_delay_ms` must be non-negative. Negative values must produce a validation error.

**R2.4 (max_delay_ms Range):** `max_delay_ms` must be non-negative. Negative values must produce a validation error.

**R2.5 (Validation Errors):** Validation errors must follow the existing pattern: return a descriptive `error` from `Validate()` that names the field and the constraint violated. Examples: `"retry.max_retries must be non-negative"`, `"retry.backoff must be one of: exponential, linear, fixed"`.

### R3: Retriable vs Non-Retriable Errors

**R3.1 (Retriable Categories):** The following `ProviderError` categories must be retried:

| Category | Reason |
|----------|--------|
| `rate_limit` | Transient. Provider will accept requests again after cooldown. |
| `server` | Transient server-side failure (5xx). |
| `overloaded` | Transient provider capacity issue (Anthropic 529). |
| `timeout` | Transient network or provider timeout. |

**R3.2 (Non-Retriable Categories):** The following `ProviderError` categories must NOT be retried — the error must be returned immediately:

| Category | Reason |
|----------|--------|
| `auth` | Credentials are wrong. Retrying will not fix them. |
| `bad_request` | Request payload is malformed. Retrying the same payload will produce the same error. |

**R3.3 (Non-ProviderError Errors):** Errors that are not `*ProviderError` (e.g., JSON marshaling failures, unexpected panics) must NOT be retried. Only typed `ProviderError` values are eligible for retry classification.

### R4: Backoff Strategies

**R4.1 (Exponential Backoff):** When `backoff = "exponential"`, the delay before attempt N (0-indexed, where attempt 0 is the first retry) must be: `min(initial_delay_ms * 2^N + jitter, max_delay_ms)`. Jitter must be a random value in the range `[0, initial_delay_ms)` to prevent thundering herd.

**R4.2 (Linear Backoff):** When `backoff = "linear"`, the delay before attempt N must be: `min(initial_delay_ms * (N + 1), max_delay_ms)`. No jitter is applied.

**R4.3 (Fixed Backoff):** When `backoff = "fixed"`, the delay before every attempt must be: `min(initial_delay_ms, max_delay_ms)`. No jitter is applied.

**R4.4 (Delay Capping):** All computed delays must be capped at `max_delay_ms`. The cap is applied after jitter addition (for exponential).

### R5: Retry Behavior

**R5.1 (Retry Loop):** On a retriable error, the system must wait for the computed backoff delay, then re-invoke `Send()` with the same `Request`. This repeats until either: (a) `Send()` succeeds, (b) `max_retries` attempts are exhausted, or (c) the context is cancelled.

**R5.2 (Context Cancellation During Backoff):** If the parent context is cancelled or its deadline expires during a backoff sleep, the retry must abort immediately and return the context error. The system must NOT sleep through a context cancellation.

**R5.3 (Final Error on Exhaustion):** When all retry attempts are exhausted, the error from the last attempt must be returned. The caller receives the same `*ProviderError` it would have received without retry — the retry is transparent except for the delay.

**R5.4 (Request Immutability):** The `Request` object must not be modified between retry attempts. Each retry sends the identical request.

**R5.5 (No Retry Passthrough):** When `max_retries = 0`, the system must call `Send()` exactly once and return the result directly — no retry logic, no additional overhead.

### R6: Observability

**R6.1 (Verbose Logging):** When `--verbose` is enabled and a retry occurs, a message must be written to stderr for each retry attempt. The message must include: (a) the current attempt number and max retries, (b) the error category that triggered the retry, and (c) the backoff delay in milliseconds. Example: `[retry] Attempt 2/3 after rate_limit, waiting 1000ms`.

**R6.2 (JSON Output):** When `--json` is enabled, the output envelope must include a `retry_attempts` field (integer) indicating the total number of retry attempts that occurred across all `Send()` calls during the agent run. If no retries occurred, the value must be `0`.

**R6.3 (No Output Without Verbose):** When `--verbose` is not enabled, retry attempts must produce no output to stdout or stderr. Retries are silent by default — axe output must remain safe to pipe.

### R7: Scaffold and Documentation

**R7.1 (Agent Scaffold):** The `Scaffold()` function in `internal/agent/agent.go` must include a commented-out `[retry]` section in the generated TOML template, consistent with how `[memory]` and `[params]` are scaffolded.

**R7.2 (Config Schema Doc):** The `docs/design/agent-config-schema.md` file must be updated to document the `[retry]` section, its fields, types, defaults, and valid values.

### R8: Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| `max_retries = 0`, provider returns 429 | Error returned immediately. No retry. Identical to current behavior. |
| `max_retries = 3`, provider returns 429 three times then succeeds | Success on 4th attempt (1 initial + 3 retries). |
| `max_retries = 3`, provider returns 429 four times | Error returned after 4th attempt (1 initial + 3 retries exhausted). Last `ProviderError` returned. |
| `max_retries = 3`, provider returns 401 | Error returned immediately. Auth errors are not retriable. No retries attempted. |
| `max_retries = 3`, provider returns 400 | Error returned immediately. Bad request errors are not retriable. |
| `max_retries = 3`, provider returns 503 then 200 | Success on 2nd attempt (1 initial + 1 retry). Only 1 retry consumed. |
| `max_retries = 3`, provider returns 429, then 401 | First attempt: 429 (retriable, retry). Second attempt: 401 (not retriable, return immediately). Only 1 retry consumed. |
| `max_retries = 3`, context cancelled during backoff | Context error returned immediately. Backoff sleep is interrupted. |
| `max_retries = 3`, context deadline exceeded before any retry | The initial `Send()` returns a timeout `ProviderError`. The retry logic checks the context — if already cancelled, returns immediately without retrying. |
| `max_retries = 5` in TOML | Valid. There is no upper bound on `max_retries` (unlike `max_depth` which caps at 5). Users accept the latency cost. |
| `initial_delay_ms = 0` in TOML | Treated as "use default" (500ms). |
| `max_delay_ms = 100`, `initial_delay_ms = 500` | Every delay is capped at 100ms. The cap is always respected. |
| Conversation loop: turn 1 succeeds, turn 2 gets 429 | Turn 2 retries independently with its own retry budget. Turn 1's success is unaffected. |
| Conversation loop: turn 1 retries twice, turn 2 retries once | `retry_attempts` in `--json` output is 3 (cumulative across all turns). |
| Non-ProviderError returned by Send() | Not retried. Returned immediately. |
| `backoff = ""` in TOML | Treated as default (`"exponential"`). |
| `backoff = "EXPONENTIAL"` (wrong case) | Validation error. Values are case-sensitive. |
| Exponential backoff with large attempt number | Delay is capped at `max_delay_ms`. No integer overflow — delay computation must handle large exponents safely. |
