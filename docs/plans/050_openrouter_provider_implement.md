# OpenRouter First-Class Provider Support — Implementation Guide

## Section 1: Context Summary

Axe currently supports OpenRouter only by overriding the `openai` provider's `base_url`, which loses OpenRouter-specific features. This milestone adds a first-class `openrouter` provider that speaks the OpenAI Chat Completions wire format but additionally sends app attribution headers (`HTTP-Referer`, `X-OpenRouter-Title`, `X-OpenRouter-Categories`), parses per-response cost and cache status, and implements the `StreamProvider` interface for streaming support. Model IDs with slashes (e.g. `anthropic/claude-sonnet-4`) work without parser changes because `parseModel` splits on the first `/` only.

---

## Section 2: Implementation Checklist

### Phase 1: Foundation (parallel)

These four tasks are independent and may be done in any order.

- [ ] **`internal/provider/provider.go`**: Add `Cost float64` and `CacheStatus string` fields to the `Response` struct.
  - Insert `Cost float64` after `OutputTokens int`.
  - Insert `CacheStatus string` after `StopReason string`.

- [ ] **`pkg/runner/result.go`**: Add `Cost float64` and `CacheStatus string` fields to the `Result` struct, and surface them in `MarshalJSON()`.
  - Add fields to the struct definition.
  - In `MarshalJSON`, unconditionally include `m["cost"] = r.Cost` and `m["cache_status"] = r.CacheStatus`.

- [ ] **`internal/config/config.go`**: Extend `ProviderConfig` and add OpenRouter-specific resolution methods.
  - Add `Referer string`, `Title string`, `Categories string` to `ProviderConfig`.
  - Add `ResolveReferer(providerName string) string`
    - Env var: `AXE_{UPPER}_REFERER` > config `[providers.{name}].referer` > empty.
  - Add `ResolveTitle(providerName string) string`
    - Env var: `AXE_{UPPER}_TITLE` > config `[providers.{name}].title` > empty.
  - Add `ResolveCategories(providerName string) string`
    - Env var: `AXE_{UPPER}_CATEGORIES` > config `[providers.{name}].categories` > empty.
  - Follow the existing `ResolveBaseURL` pattern for env-var name construction.

- [ ] **`internal/provider/openrouter.go`**: Create the OpenRouter provider.
  - Define `OpenRouter` struct with `apiKey`, `baseURL`, `referer`, `title`, `categories`, `client *http.Client`.
  - Define `OpenRouterOption func(*OpenRouter)`.
  - Implement `WithOpenRouterBaseURL(url string) OpenRouterOption`.
  - Implement `WithReferer(referer string) OpenRouterOption`.
  - Implement `WithTitle(title string) OpenRouterOption`.
  - Implement `WithCategories(categories string) OpenRouterOption`.
  - Implement `NewOpenRouter(apiKey string, opts ...OpenRouterOption) (*OpenRouter, error)`:
    - Return error if `apiKey` is empty.
    - Default `baseURL` to `https://openrouter.ai/api/v1`.
    - Set `client` with `CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }`.
  - Implement `Send(ctx context.Context, req *Request) (*Response, error)`:
    - Construct OpenAI Chat Completions request body using the existing `openaiRequest` / `openaiMessage` / `openaiToolDef` types from `openai.go` (or duplicate the minimal structs if unexported reuse is awkward).
    - POST to `{baseURL}/chat/completions`.
    - Set `Authorization: Bearer {apiKey}`, `Content-Type: application/json`.
    - Conditionally set `HTTP-Referer`, `X-OpenRouter-Title`, `X-OpenRouter-Categories` if non-empty.
    - Parse HTTP response; on 2xx unmarshal JSON into the response struct.
    - Extract `Content`, `Model`, `InputTokens` (`usage.prompt_tokens`), `OutputTokens` (`usage.completion_tokens`), `StopReason` (`choices[0].finish_reason`), `ToolCalls`.
    - Extract `Cost` from `usage.cost` if present, default to `0`.
    - Extract `CacheStatus` from response header `X-OpenRouter-Cache-Status` if present.
    - On non-2xx, return `*ProviderError` with category mapped via the same status-to-category rules as `OpenAI` (`401/403` → auth, `400/404` → bad_request, `429` → rate_limit, `5xx` → server).
    - Context cancellation maps to `ErrCategoryTimeout`.
  - Implement `SendStream(ctx context.Context, req *Request) (*EventStream, error)`:
    - Build streaming request body with `Stream: true`, `StreamOptions: &openaiStreamOptions{IncludeUsage: true}`.
    - Send same headers as non-streaming.
    - Return `*EventStream` via `NewEventStream(httpResp.Body, nextFunc)`.
    - `nextFunc` must use `NewSSEParser(httpResp.Body)` to parse SSE chunks.
    - Emit `StreamEventText` for content deltas.
    - Emit `StreamEventToolStart`, `StreamEventToolDelta`, `StreamEventToolEnd` for tool calls (follow the exact assembly pattern in `openai.go:SendStream`).
    - Final `StreamEventDone` must include `InputTokens`, `OutputTokens`, `StopReason` from the usage chunk.
    - Extract `Cost` from usage chunk if present.
    - Extract `CacheStatus` from HTTP response headers before starting SSE parsing.

---

### Phase 2: Integration (depends on Phase 1)

- [ ] **`internal/provider/registry.go`**: Add `openrouter` to the registry.
  - Add `"openrouter": true` to `supportedProviders`.
  - Add `case "openrouter":` to `New()`:
    - Build `[]OpenRouterOption`, append `WithOpenRouterBaseURL(baseURL)` if `baseURL != ""`.
    - Call `NewOpenRouter(apiKey, opts...)`.
  - Update the unsupported-provider error message to include `openrouter` in the list.

- [ ] **`pkg/runner/run.go`**: Wire OpenRouter-specific config resolution and verbose output.
  - After resolving `apiKey` and `baseURL` (around the existing bedrock special-case block), add an `openrouter` special-case block before the generic `provider.New` call:
    ```go
    if provName == "openrouter" {
        var opts []provider.OpenRouterOption
        if baseURL != "" {
            opts = append(opts, provider.WithOpenRouterBaseURL(baseURL))
        }
        if r := globalCfg.ResolveReferer(provName); r != "" {
            opts = append(opts, provider.WithReferer(r))
        }
        if t := globalCfg.ResolveTitle(provName); t != "" {
            opts = append(opts, provider.WithTitle(t))
        }
        if c := globalCfg.ResolveCategories(provName); c != "" {
            opts = append(opts, provider.WithCategories(c))
        }
        prov, err = provider.NewOpenRouter(apiKey, opts...)
    } else {
        prov, err = provider.New(provName, apiKey, baseURL)
    }
    ```
    - Keep the existing `apiKey == ""` validation block before this; it should already catch missing OpenRouter keys via the generic check since `openrouter` is not in the `ollama`/`bedrock` exemption list.
  - In the per-turn verbose output block (around lines 509–514, where `Tokens: ...` is printed), add conditional lines after the token line:
    - If `resp.Cost > 0`, print `_, _ = fmt.Fprintf(stderr, "Cost:     $%.6f\n", resp.Cost)`.
    - If `resp.CacheStatus != ""`, print `_, _ = fmt.Fprintf(stderr, "Cache:    %s\n", resp.CacheStatus)`.
  - When building the runner `Result` at the end of each turn, assign `result.Cost = resp.Cost` and `result.CacheStatus = resp.CacheStatus`.

---

### Phase 3: Tests (depends on Phase 1 & 2)

- [ ] **`internal/provider/openrouter_test.go`**: Create table-driven tests using `httptest.NewServer`.
  - `TestNewOpenRouter_EmptyAPIKey` — expect error.
  - `TestNewOpenRouter_ValidAPIKey` — expect non-nil provider.
  - `TestOpenRouter_Send_Success` — assert `Content`, `Model`, `InputTokens`, `OutputTokens`, `StopReason`, `Cost` when present, `CacheStatus` when header present.
  - `TestOpenRouter_Send_RequestFormat` — capture request via `httptest` handler; assert method `POST`, path `/chat/completions`, `Authorization` header starts with `Bearer `, `Content-Type` is `application/json`, body contains correct `model` and `messages` shapes.
  - `TestOpenRouter_Send_AttributionHeaders` — provider created with `WithReferer`, `WithTitle`, `WithCategories`; assert those headers appear in the request.
  - `TestOpenRouter_Send_NoAttributionHeadersByDefault` — assert `HTTP-Referer`, `X-OpenRouter-Title`, `X-OpenRouter-Categories` are absent when not configured.
  - `TestOpenRouter_Send_ErrorResponses` — table test for HTTP `400`, `401`, `403`, `429`, `500`; assert correct `ProviderError.Category`.
  - `TestOpenRouter_Send_GracefulDegradation` — response without `usage.cost` and without `X-OpenRouter-Cache-Status` must succeed with `Cost == 0` and `CacheStatus == ""`.
  - `TestOpenRouter_SendStream_TextDeltas` — assert `StreamEventText` events assemble correctly.
  - `TestOpenRouter_SendStream_UsageAndCost` — final `StreamEventDone` includes `InputTokens`, `OutputTokens`, and `Cost` if the usage chunk contains it.
  - `TestOpenRouter_SendStream_ErrorResponse` — non-2xx on initial request returns `ProviderError`.

- [ ] **`internal/provider/registry_test.go`**: Add OpenRouter cases.
  - `TestNew_OpenRouter` — `provider.New("openrouter", "test-key", "")` returns non-nil, no error.
  - `TestNew_OpenRouterWithBaseURL` — `provider.New("openrouter", "test-key", "http://custom:8080")` returns non-nil.
  - `TestNew_OpenRouterMissingAPIKey` — empty key returns error containing "API key is required".
  - `TestSupported_OpenRouter` — `Supported("openrouter")` is `true`.
  - Update `TestNew_UnsupportedProvider_ErrorMessage` to assert `"openrouter"` appears in the error message.

- [ ] **`internal/config/config_test.go`**: Add resolution tests for new fields.
  - `TestResolveReferer` — env var overrides config file value.
  - `TestResolveTitle` — env var overrides config file value.
  - `TestResolveCategories` — env var overrides config file value.
  - `TestResolveReferer_UnknownProvider` — returns empty string when provider not in config and no env var.

- [ ] **`pkg/runner/result_test.go`**: Add `Cost` and `CacheStatus` JSON serialization coverage.
  - `TestResultMarshalJSON_Cost` — `Result{Cost: 0.00014}` serializes `"cost":0.00014`.
  - `TestResultMarshalJSON_CacheStatus` — `Result{CacheStatus:"HIT"}` serializes `"cache_status":"HIT"`.

- [ ] **`pkg/runner/run_test.go`**: Add verbose output test.
  - `TestRun_VerboseOpenRouterCostAndCache` — run with a mock StreamProvider that returns `Cost > 0` and `CacheStatus == "HIT"`; assert stderr contains both `Cost:` and `Cache:` lines.

---

### Parallel Work Summary

| Workstream | Files | Dependencies |
|------------|-------|-------------|
| Core types | `internal/provider/provider.go`, `pkg/runner/result.go` | None |
| Config extension | `internal/config/config.go` | None |
| Provider impl | `internal/provider/openrouter.go` | `provider.go` ( Response struct ) |
| Registry | `internal/provider/registry.go` | `openrouter.go` |
| Runner integration | `pkg/runner/run.go` | All of the above |
| Tests | `*_test.go` files | Corresponding implementation |
