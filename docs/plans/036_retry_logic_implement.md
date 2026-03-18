# Implementation Guide: Retry Logic for Intermittent LLM Provider Failures

**Spec:** `docs/plans/036_retry_logic_spec.md`  
**GitHub Issue:** [#29](https://github.com/jrswab/axe/issues/29)  
**Created:** 2026-03-17

---

## 1. Context Summary

**Associated spec:** `docs/plans/036_retry_logic_spec.md` (standalone — no milestone document).

Unattended multi-agent pipelines fail entirely when a provider returns a transient 429/503. There is currently no retry — every `prov.Send()` failure is immediately terminal. This implementation adds a provider decorator (`RetryProvider`) that wraps any `Provider` with configurable retry and backoff, a `[retry]` config section in agent TOML, and a `retry_attempts` field in `--json` output. The decorator is opt-in (default `max_retries = 0`), uses no third-party dependencies, and respects context cancellation during backoff sleeps. All decisions — decorator pattern, per-agent config only, stdlib-only backoff, jitter always on for exponential — are final per the spec.

---

## 2. Implementation Checklist

### Phase 1: Agent Config — `RetryConfig` struct and validation

- [x] **1a. Add `RetryConfig` struct** — `internal/agent/agent.go`: Add a new `RetryConfig` struct with fields `MaxRetries int \`toml:"max_retries"\``, `Backoff string \`toml:"backoff"\``, `InitialDelayMs int \`toml:"initial_delay_ms"\``, `MaxDelayMs int \`toml:"max_delay_ms"\``. Add a `Retry RetryConfig \`toml:"retry"\`` field to `AgentConfig` (after the `Params` field, following the existing sub-config pattern of `MemoryConfig`, `ParamsConfig`). Spec refs: R1.1.

- [x] **1b. Add retry validation to `Validate()`** — `internal/agent/agent.go: Validate()`: Append validation checks after the existing MCP server validation block (after line 119). Check: `retry.max_retries` must be >= 0; `retry.backoff` must be one of `"exponential"`, `"linear"`, `"fixed"`, or `""` (empty); `retry.initial_delay_ms` must be >= 0; `retry.max_delay_ms` must be >= 0. Error messages must follow the existing pattern: `"retry.max_retries must be non-negative"`, `"retry.backoff must be one of: exponential, linear, fixed"`, etc. Spec refs: R2.1–R2.5.

- [x] **1c. Test `RetryConfig` validation** — `internal/agent/agent_test.go`: Add table-driven tests following the existing pattern (individual `TestValidate_*` functions). Test cases: (1) valid retry config with all fields set passes validation, (2) `max_retries = -1` returns error, (3) `backoff = "invalid"` returns error, (4) `backoff = "EXPONENTIAL"` (wrong case) returns error, (5) `backoff = ""` passes (treated as default), (6) `initial_delay_ms = -1` returns error, (7) `max_delay_ms = -1` returns error, (8) zero-valued retry config (all defaults) passes, (9) each valid backoff value (`"exponential"`, `"linear"`, `"fixed"`) passes. Spec refs: R2.1–R2.5, R8 edge cases.

- [x] **1d. Test TOML parsing of `[retry]` section** — `internal/agent/agent_test.go`: Add a test that decodes a TOML string containing a `[retry]` section and verifies all four fields are correctly populated in `AgentConfig.Retry`. Add a second test that decodes TOML without a `[retry]` section and verifies all fields are zero-valued. Spec refs: R1.1, R1.2.

### Phase 2: Provider Retry Decorator

- [x] **2a. Create `RetryConfig` and `RetryProvider` types** — `internal/provider/retry.go` (new file): Define a `RetryConfig` struct with fields: `MaxRetries int`, `Backoff string`, `InitialDelayMs int`, `MaxDelayMs int`, `Verbose bool`, `Stderr io.Writer`. Define a `RetryProvider` struct with fields: `inner Provider`, `cfg RetryConfig`, `attempts int` (cumulative retry count across all `Send()` calls). Expose `Attempts() int` method to read the cumulative count. Spec refs: R1.1, R6.1, R6.2.

- [x] **2b. Implement `NewRetry()` constructor** — `internal/provider/retry.go: NewRetry()`: Signature: `func NewRetry(p Provider, cfg RetryConfig) *RetryProvider`. Apply runtime defaults: if `cfg.Backoff` is `""`, set to `"exponential"`; if `cfg.InitialDelayMs` is 0, set to 500; if `cfg.MaxDelayMs` is 0, set to 30000. If `cfg.MaxRetries <= 0`, still return a `RetryProvider` (the `Send()` method handles the passthrough — R5.5). Spec refs: R1.2, R5.5.

- [x] **2c. Implement `isRetriable()` helper** — `internal/provider/retry.go: isRetriable()`: Unexported function. Accepts `error`, returns `bool`. Uses `errors.As` to extract `*ProviderError`. Returns `true` for categories: `rate_limit`, `server`, `overloaded`, `timeout`. Returns `false` for `auth`, `bad_request`, and any non-`ProviderError` error. Spec refs: R3.1–R3.3.

- [x] **2d. Implement `computeDelay()` helper** — `internal/provider/retry.go: computeDelay()`: Unexported function. Signature: `func computeDelay(attempt int, cfg RetryConfig) time.Duration`. For `"exponential"`: `min(initial_delay_ms * 2^attempt + rand.IntN(initial_delay_ms), max_delay_ms)`. Use bit shift for `2^attempt`; guard against overflow by capping the shift (if `attempt >= 63` or the intermediate result overflows, use `max_delay_ms` directly). For `"linear"`: `min(initial_delay_ms * (attempt + 1), max_delay_ms)`. For `"fixed"`: `min(initial_delay_ms, max_delay_ms)`. Return as `time.Duration` (milliseconds converted). Spec refs: R4.1–R4.4, R8 overflow edge case.

- [x] **2e. Implement `Send()` method** — `internal/provider/retry.go: (*RetryProvider).Send()`: If `cfg.MaxRetries == 0`, delegate directly to `inner.Send()` and return (R5.5 passthrough). Otherwise: call `inner.Send()`. On success, return. On error: if not retriable, return immediately. If retriable and retries remain: compute delay, log to stderr if verbose (R6.1 format: `[retry] Attempt %d/%d after %s, waiting %dms`), select on `time.After(delay)` vs `ctx.Done()` (R5.2 — context cancellation aborts immediately), then retry. Increment `r.attempts` for each retry attempted. On exhaustion, return the last error (R5.3). Never modify the `*Request` (R5.4). Spec refs: R5.1–R5.5, R6.1, R6.3.

- [x] **2f. Test `isRetriable()`** — `internal/provider/retry_test.go` (new file): Table-driven test covering all six `ErrorCategory` values plus a non-`ProviderError` error and a nil error. Verify: `rate_limit`, `server`, `overloaded`, `timeout` → true; `auth`, `bad_request` → false; plain `error` → false; nil → false. Spec refs: R3.1–R3.3.

- [x] **2g. Test `computeDelay()`** — `internal/provider/retry_test.go: TestComputeDelay`: Test each backoff strategy. For `"fixed"`: verify delay equals `min(initial_delay_ms, max_delay_ms)` regardless of attempt number. For `"linear"`: verify delay equals `min(initial_delay_ms * (attempt+1), max_delay_ms)` for attempts 0, 1, 2 and a capped case. For `"exponential"`: verify delay is within `[initial_delay_ms * 2^attempt, initial_delay_ms * 2^attempt + initial_delay_ms)` and capped at `max_delay_ms`. Test overflow safety: attempt=100 with `initial_delay_ms=500`, `max_delay_ms=30000` must return `max_delay_ms`, not panic. Test `max_delay_ms < initial_delay_ms`: delay is always capped at `max_delay_ms`. Spec refs: R4.1–R4.4, R8.

- [x] **2h. Test `Send()` — success on first try** — `internal/provider/retry_test.go`: Create a mock provider (unexported struct in test file implementing `Provider`) that returns a `*Response` on the first call. Wrap with `NewRetry(mock, RetryConfig{MaxRetries: 3, ...})`. Call `Send()`. Verify: response returned, `Attempts()` is 0, mock was called exactly once. Spec ref: R5.1.

- [x] **2i. Test `Send()` — success after retries** — `internal/provider/retry_test.go`: Mock provider returns `ProviderError{Category: rate_limit}` on calls 1–2, then succeeds on call 3. `MaxRetries: 3`. Verify: success returned, `Attempts()` is 2, mock called 3 times. Use small `InitialDelayMs` (1ms) to keep test fast. Spec refs: R5.1, R8 row 2.

- [x] **2j. Test `Send()` — exhaustion** — `internal/provider/retry_test.go`: Mock provider always returns `ProviderError{Category: server}`. `MaxRetries: 2`. Verify: error returned is the last `ProviderError`, `Attempts()` is 2, mock called 3 times (1 initial + 2 retries). Spec refs: R5.3, R8 row 3.

- [x] **2k. Test `Send()` — non-retriable error** — `internal/provider/retry_test.go`: Mock provider returns `ProviderError{Category: auth}`. `MaxRetries: 3`. Verify: error returned immediately, `Attempts()` is 0, mock called exactly once. Spec refs: R3.2, R8 row 4.

- [x] **2l. Test `Send()` — non-ProviderError** — `internal/provider/retry_test.go`: Mock provider returns `errors.New("unexpected")`. `MaxRetries: 3`. Verify: error returned immediately, `Attempts()` is 0, mock called exactly once. Spec ref: R3.3.

- [x] **2m. Test `Send()` — retriable then non-retriable** — `internal/provider/retry_test.go`: Mock returns `ProviderError{Category: rate_limit}` on call 1, then `ProviderError{Category: auth}` on call 2. `MaxRetries: 3`. Verify: auth error returned, `Attempts()` is 1, mock called twice. Spec ref: R8 row 7.

- [x] **2n. Test `Send()` — context cancellation during backoff** — `internal/provider/retry_test.go`: Mock always returns `ProviderError{Category: rate_limit}`. `MaxRetries: 5`, `InitialDelayMs: 5000` (long delay). Create context with 50ms timeout. Call `Send()`. Verify: returns within ~100ms (not 5s), error is context-related, mock called at most twice (initial + possibly one retry before cancel). Spec refs: R5.2, R8 row 8.

- [x] **2o. Test `Send()` — zero max_retries passthrough** — `internal/provider/retry_test.go`: Mock returns `ProviderError{Category: rate_limit}`. `MaxRetries: 0`. Verify: error returned immediately, `Attempts()` is 0, mock called exactly once. Spec refs: R5.5, R8 row 1.

- [x] **2p. Test `Send()` — verbose logging** — `internal/provider/retry_test.go`: Mock returns `ProviderError{Category: rate_limit}` once then succeeds. `Verbose: true`, `Stderr: &bytes.Buffer{}`. Verify: buffer contains `[retry] Attempt 1/` and the error category. Spec ref: R6.1.

- [x] **2q. Test `Send()` — no output when not verbose** — `internal/provider/retry_test.go`: Same as 2p but `Verbose: false`. Verify: stderr buffer is empty. Spec ref: R6.3.

### Phase 3: Integration — Wire decorator into `cmd/run.go`

- [x] **3a. Wire `RetryProvider` after `provider.New()`** — `cmd/run.go: runAgent()`: After `prov, err := provider.New(...)` (line 225), wrap the provider: construct a `provider.RetryConfig` from `cfg.Retry` (mapping `agent.RetryConfig` fields to `provider.RetryConfig` fields, plus `Verbose` from the verbose flag and `Stderr` from `cmd.ErrOrStderr()`), then call `retryProv := provider.NewRetry(prov, retryCfg)` and reassign `prov = retryProv` (since `RetryProvider` satisfies `Provider`). Keep a reference to `retryProv` for reading `Attempts()` later. Spec refs: R1.2, R5.5.

- [x] **3b. Add `retry_attempts` to `--json` envelope** — `cmd/run.go: runAgent()`: In the JSON output block (around line 470), add `"retry_attempts": retryProv.Attempts()` to the envelope map. When retry is not configured (max_retries=0), this will be 0. Spec ref: R6.2.

- [x] **3c. Test `--json` includes `retry_attempts`** — `cmd/run_test.go`: Add an integration test using the existing mock server pattern. Configure an agent with `[retry] max_retries = 0`. Run with `--json`. Verify the JSON output contains `"retry_attempts":0`. Spec ref: R6.2.

- [x] **3d. Test retry integration with mock server** — `cmd/run_test.go`: Add an integration test where the mock server returns HTTP 429 on the first request and 200 on the second. Configure agent with `[retry] max_retries = 2, initial_delay_ms = 1`. Run the agent. Verify: success, and with `--json`, `retry_attempts` is 1. Spec refs: R5.1, R6.2, R8 row 6.

### Phase 4: Scaffold and Documentation

- [x] **4a. Add `[retry]` to scaffold template** — `internal/agent/agent.go: Scaffold()`: Add a commented-out `[retry]` section after the `[params]` section in the template string. Format: `# [retry]\n# max_retries = 0\n# backoff = "exponential"\n# initial_delay_ms = 500\n# max_delay_ms = 30000\n`. Spec ref: R7.1.

- [x] **4b. Test scaffold includes retry section** — `internal/agent/agent_test.go`: Add `TestScaffold_IncludesRetryConfig` that calls `Scaffold("test")` and verifies the output contains `# [retry]`, `# max_retries`, `# backoff`, `# initial_delay_ms`, and `# max_delay_ms`. Follow the existing `TestScaffold_Includes*` pattern. Spec ref: R7.1.

- [x] **4c. Update config schema doc** — `docs/design/agent-config-schema.md`: Add a `[retry]` section to the example TOML and add rows to the Fields table: `retry.max_retries` (int, no, default 0), `retry.backoff` (string, no, default "exponential"), `retry.initial_delay_ms` (int, no, default 500), `retry.max_delay_ms` (int, no, default 30000). Spec ref: R7.2.

### Phase 5: Final Verification

- [x] **5a. Run full test suite** — Verify all existing tests pass: `go test ./...`. No existing test should break. The decorator defaults to 0 retries (passthrough), so all current behavior is preserved.

- [x] **5b. Run `go vet ./...`** — Verify no vet warnings introduced.