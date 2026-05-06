# OpenRouter First-Class Provider Support

## Section 1: Context & Constraints

### Codebase Structure and Patterns

Axe's provider system lives in `internal/provider/`:

- `provider.go` — Defines the `Provider` interface (`Send(ctx, *Request) (*Response, error)`), `Request`, `Response`, `Message`, `Tool`, `ToolCall`, `ToolResult`, and `ProviderError` with `ErrorCategory`.
- `registry.go` — `supportedProviders` map and `New(providerName, apiKey, baseURL)` constructor dispatch. Adding a provider requires updating both.
- `stream.go` — `StreamProvider` interface extends `Provider` with `SendStream(ctx, *Request) (*EventStream, error)`. All existing first-class providers implement `StreamProvider`.
- `openai.go` — OpenAI Chat Completions implementation. OpenRouter uses the exact same wire format for requests and responses, with extra fields.
- `config/config.go` — `ProviderConfig` struct holds `APIKey`, `BaseURL`, `Region`. `ResolveAPIKey`, `ResolveBaseURL`, `ResolveRegion` use env-var-first resolution order.

Provider creation in `pkg/runner/run.go`:
1. `parseModel(cfg.Model)` splits on the **first `/` only** into `(providerName, modelName)`.
2. `globalCfg.ResolveAPIKey(provName)` and `globalCfg.ResolveBaseURL(provName)` fetch credentials.
3. `provider.New(provName, apiKey, baseURL)` dispatches to the constructor.
4. Provider is wrapped with `provider.NewRetry(...)`.

The `Response` struct currently contains: `Content`, `Model`, `InputTokens`, `OutputTokens`, `StopReason`, `ToolCalls`.

The `Result` struct (runner output, JSON-serialized) contains: `Content`, `Model`, `InputTokens`, `OutputTokens`, `StopReason`, `ToolCalls`, `ToolCallDetails`, `DurationMs`, `Refused`, `RetryAttempts`, `Budget`, `Artifacts`.

### Decisions Already Made

- OpenRouter will use the **OpenAI Chat Completions wire format** for both requests and responses. No multi-route complexity (unlike OpenCode). This is explicitly chosen for simplicity.
- Model IDs like `openrouter/anthropic/claude-sonnet-4` work today because `parseModel` splits on the first `/` only, yielding provider `openrouter` and model `anthropic/claude-sonnet-4`. No parser changes are needed.
- App attribution headers (`HTTP-Referer`, `X-OpenRouter-Title`, `X-OpenRouter-Categories`) and provider routing preferences are configurable. The initial implementation will support global config-level attribution headers; agent-level `[openrouter]` routing config is deferred.
- Cost parsing and cache-status reporting are **new surface area** in the `Response`/`Result` structs. They must be optional fields that degrade gracefully when absent.

### Approaches Ruled Out

- **Reusing the `openai` provider with base_url override**: Rejected because it loses OpenRouter-specific features (cost tracking, cache status, attribution headers, normalized finish reasons).
- **Custom wire format**: Rejected. OpenRouter speaks standard OpenAI Chat Completions; no bespoke request/response types are needed beyond extending the OpenAI-shaped structs with OpenRouter-specific fields.
- **Agent-level provider routing config in TOML**: Deferred. The `[openrouter]` table with `provider_order`, `allow_fallbacks`, etc. is not in scope for the initial implementation.

### Constraints and Assumptions

- **Zero new external dependencies.** All work must use the Go standard library.
- **Env var resolution order**: For OpenRouter-specific fields, env vars override config file values, following the existing pattern (`AXE_OPENROUTER_*`).
- **Streaming required**: All first-class providers implement `StreamProvider`. OpenRouter must too.
- **Graceful degradation**: If `usage.cost`, `usage.cost_details`, `native_finish_reason`, or `X-OpenRouter-Cache-Status` are absent from the response, the provider must not error.
- **Config schema extension**: `ProviderConfig` may need new fields (or a different mechanism) for `Referer`, `Title`, `Categories`. This must not break existing providers.
- **Tests must not use mocks when avoidable** (per AGENTS.md). Use `httptest` servers, as all existing provider tests do.
- **Budget tracker integration**: The `budget` package currently tracks tokens. Cost data from OpenRouter is exposed via `Result` for user visibility but does not replace token budgeting in this milestone.

### Open Questions Resolved During Research

- **Q: Does `parseModel` handle `openrouter/anthropic/claude-sonnet-4`?**  
  A: Yes. `strings.Index(model, "/")` splits on the first `/` only, so `modelName` becomes `anthropic/claude-sonnet-4`.
- **Q: Does the existing `ProviderConfig` support arbitrary extra fields?**  
  A: No. `ProviderConfig` only has `APIKey`, `BaseURL`, `Region`. OpenRouter-specific fields need new struct members or a generic extension mechanism.
- **Q: Is streaming mandatory?**  
  A: Yes. The `StreamProvider` interface is the de facto standard for all providers. `runner.Run` supports `--stream` and expects providers to implement it.
- **Q: How are test fixtures created?**  
  A: Provider tests use `httptest.NewServer` and table-driven tests. See `openai_test.go`, `anthropic_test.go` for patterns.

---

## Section 2: Requirements

### R1: Provider Registration

- `provider.Supported("openrouter")` MUST return `true`.
- `provider.New("openrouter", apiKey, baseURL)` MUST return a working `StreamProvider`.
- `provider.New("openrouter", "", "")` MUST return an error when the API key is empty.
- The unsupported-provider error message MUST include `openrouter` in the list of supported providers.

### R2: Model ID Handling

- When `cfg.Model` is `openrouter/anthropic/claude-sonnet-4`, the provider MUST receive model name `anthropic/claude-sonnet-4`.
- When `cfg.Model` is `openrouter/openai/gpt-5`, the provider MUST receive model name `openai/gpt-5`.
- Any model name containing `/` after the provider prefix MUST be passed through verbatim.

### R3: Request Construction

- The provider MUST POST to `{baseURL}/chat/completions` with `Content-Type: application/json` and `Authorization: Bearer {apiKey}`.
- The request body MUST be valid OpenAI Chat Completions format (`model`, `messages`, optional `temperature`, `max_completion_tokens`, `tools`, `response_format`, `stream`, `stream_options`).
- When configured, the provider MUST send the following optional headers:
  - `HTTP-Referer`
  - `X-OpenRouter-Title`
  - `X-OpenRouter-Categories`
- Default `baseURL` MUST be `https://openrouter.ai/api/v1`.
- The `client` MUST follow the same redirect policy as existing providers (`CheckRedirect` returns `http.ErrUseLastResponse`).

### R4: Config and Environment Variable Resolution

- OpenRouter API key resolution order: `OPENROUTER_API_KEY` env var > `[providers.openrouter].api_key` in config.toml > empty.
- OpenRouter base URL resolution order: `AXE_OPENROUTER_BASE_URL` env var > `[providers.openrouter].base_url` in config.toml > default.
- OpenRouter app attribution resolution order (for each field): `AXE_OPENROUTER_*` env var > `[providers.openrouter].*` in config.toml > omitted.
- The `config.toml` schema MUST support under `[providers.openrouter]`:
  - `api_key` (string)
  - `base_url` (string)
  - `referer` (string)
  - `title` (string)
  - `categories` (string)
- Env var names:
  - `OPENROUTER_API_KEY`
  - `AXE_OPENROUTER_BASE_URL`
  - `AXE_OPENROUTER_REFERER`
  - `AXE_OPENROUTER_TITLE`
  - `AXE_OPENROUTER_CATEGORIES`

### R5: Response Parsing (Non-Streaming)

- On HTTP 2xx, the provider MUST parse the standard OpenAI Chat Completions response shape.
- `Response.Content` MUST contain the assistant message text (or empty string if null).
- `Response.Model` MUST contain the model field from the response.
- `Response.InputTokens` MUST contain `usage.prompt_tokens`.
- `Response.OutputTokens` MUST contain `usage.completion_tokens`.
- `Response.StopReason` MUST contain `choices[0].finish_reason`.
- `Response.ToolCalls` MUST be populated from `choices[0].message.tool_calls`.
- If the response contains `usage.cost`, the provider MUST capture it. If absent, cost MUST default to `0`.
- If the response contains `native_finish_reason`, it MUST be captured alongside the normalized `finish_reason`. If absent, it MUST default to the normalized value.

### R6: Response Parsing (Streaming)

- The provider MUST implement `SendStream` returning an `*EventStream`.
- SSE parsing MUST use the existing `SSEParser` infrastructure.
- Stream events MUST emit `StreamEventText`, `StreamEventToolStart`, `StreamEventToolDelta`, `StreamEventToolEnd`, and `StreamEventDone` correctly.
- The final `StreamEventDone` MUST include `InputTokens` and `OutputTokens` from the usage chunk if present.
- If a usage chunk contains `cost`, it MUST be captured. If absent, cost MUST default to `0`.

### R7: Cache Status

- The provider MUST read the `X-OpenRouter-Cache-Status` response header (`HIT` or `MISS`).
- Cache status MUST be reported in verbose mode (via stderr) when `--verbose` is enabled.
- If the header is absent, verbose output MUST omit cache status (no "MISS assumed" or similar fallback message).

### R8: Cost and Cache in Result / Verbose Output

- The `Response` struct MUST gain a `Cost` field (float64) populated from `usage.cost`.
- The `Result` struct MUST gain a `Cost` field (float64) surfaced in JSON output when `--json` is used.
- When `--json` is used and cost is `0`, the `cost` field MAY be omitted or set to `0` (implementation detail), but it MUST NOT be omitted when cost is non-zero.
- When `--verbose` is used, the runner MUST print cost and cache status to stderr after each LLM turn.
- When `--verbose` is used with non-OpenRouter providers, cost and cache status MUST NOT appear (no extraneous output).

### R9: Error Handling

- HTTP 4xx/5xx responses MUST be mapped to `ProviderError` with appropriate `ErrorCategory`:
  - `401`, `403` → `ErrCategoryAuth`
  - `400`, `404` → `ErrCategoryBadRequest`
  - `429` → `ErrCategoryRateLimit`
  - `500`, `502`, `503` → `ErrCategoryServer`
- Error response bodies MUST be parsed for a human-readable message if available.
- Context cancellation/timeouts MUST map to `ErrCategoryTimeout`.

### R10: Testing Requirements

- Constructor tests: empty API key returns error; valid API key returns non-nil provider.
- Non-streaming success test: assert Content, Model, InputTokens, OutputTokens, StopReason, Cost when present.
- Non-streaming request format test: assert method, path, Authorization header, Content-Type header, presence of optional attribution headers when configured, and correct JSON body shape.
- Non-streaming error test: assert `ProviderError` category for each HTTP status class.
- Streaming success test: assert text deltas, tool call assembly, and final usage/cost.
- Streaming error test: assert error propagation.
- Graceful degradation test: response without `usage.cost` or `X-OpenRouter-Cache-Status` must succeed with cost=0 and no cache output.
- Config resolution test: assert env vars override config file values for all OpenRouter-specific fields.
- Registry test: `Supported("openrouter")` returns `true`; `New("openrouter", ...)` dispatches correctly.

### Parallel Work

The following tasks are independent and may be done in parallel:

1. **Provider implementation** (`internal/provider/openrouter.go`): constructor, `Send`, `SendStream`, response parsing, header injection.
2. **Config extension** (`internal/config/config.go`): add OpenRouter-specific fields to `ProviderConfig` and env var resolution helpers.
3. **Registry update** (`internal/provider/registry.go`): add `openrouter` to `supportedProviders` and `New` dispatch.
4. **Response/Result struct extension** (`internal/provider/provider.go`, `pkg/runner/result.go`): add `Cost` field and JSON serialization.
5. **Runner integration** (`pkg/runner/run.go`): wire up OpenRouter-specific config resolution, pass attribution headers to constructor, print cost/cache in verbose mode.
6. **Tests** for each of the above.

---
