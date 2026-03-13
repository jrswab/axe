# Specification: MiniMax Provider Support

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-13
**Scope:** New MiniMax provider implementation, registry integration, config integration
**Issue:** Adding MiniMax as a supported provider

---

## 1. Context & Constraints

### 1.1 Associated Milestone

From `docs/plans/000_milestones.md`, M4 (Multi-Provider Support):

> Support any provider/model from models.dev.

MiniMax is being added to the existing M4 infrastructure.

### 1.2 Codebase Structure

Same as other providers (see Gemini spec).

### 1.3 Key Decision: Wraps Anthropic Provider

MiniMax provides an Anthropic-compatible API. The implementation wraps the Anthropic provider directly:
- `NewMiniMax` returns `*Anthropic` — not a new type
- Uses `AnthropicOption` functional options (the same as Anthropic)
- All request/response handling is inherited from Anthropic
- The only difference is the default base URL

This is the simplest possible implementation — no wire type duplication.

### 1.4 MiniMax API Details

**Base URL:** `https://api.minimax.io/anthropic`

**Authentication:** `x-api-key: {api_key}` header

**Request format:** Same as Anthropic Messages API

**Response format:** Same as Anthropic Messages API

### 1.5 Constraints

- **`Provider` interface is frozen** - no modifications.
- **No new entries in `go.mod`** - stdlib-only.
- **No retry logic** - same as other providers.
- **No streaming** - same as other providers.
- **HTTP client must not follow redirects.**
- **Provider name is `"minimax"`** - matches models.dev format.
- **API key env var is `MINIMAX_API_KEY`** - standard mapping.
- **All tests use `httptest.NewServer`** - no real API calls.

---

## 2. Requirements

### 2.1 MiniMax Provider

**Requirement 1.1:** A new provider named `"minimax"` must be added that satisfies the existing `Provider` interface.

**Requirement 1.2:** The provider must require a non-empty API key. If the API key is empty, the constructor must return an error: `API key is required`.

**Requirement 1.3:** The provider must have a configurable base URL with a default of `https://api.minimax.chat/v1`. A functional option must allow overriding the base URL.

**Requirement 1.4:** The `Send` method must make an HTTP POST request to `{baseURL}/text/chatcompletion_pro`.

**Requirement 1.5:** The following HTTP headers must be set:
- `x-api-key`: `{apiKey}`
- `content-type`: `application/json`

**Requirement 1.6:** Request body must follow Anthropic-compatible format (model, messages, system, temperature, max_tokens, tools).

**Requirement 1.7:** Response parsing must extract content, model, usage, and stop_reason similar to Anthropic.

**Requirement 1.8:** HTTP error mapping similar to Anthropic:
- 401 → `ErrCategoryAuth`
- 400 → `ErrCategoryBadRequest`
- 429 → `ErrCategoryRateLimit`
- 500, 502, 503 → `ErrCategoryServer`
- 529 → `ErrCategoryOverloaded`

**Requirement 1.9:** Context cancellation must return `ErrCategoryTimeout`.

### 2.2 Registry Integration

**Requirement 2.1:** `"minimax"` must be added to `supportedProviders`.

**Requirement 2.2:** `New` factory must dispatch `"minimax"` to MiniMax constructor.

**Requirement 2.3:** Error message must include `"minimax"` in the list.

### 2.3 Config Integration

**Requirement 3.1:** `knownAPIKeyEnvVars` must include `"minimax": "MINIMAX_API_KEY"`.

**Requirement 3.2:** `ResolveAPIKey` must work correctly for `"minimax"`.

---

## 3. Edge Cases

| Scenario | Behavior |
|----------|----------|
| Empty API key | Constructor returns error |
| Empty messages | Sent as empty array |
| Context cancelled | Returns `ErrCategoryTimeout` |
| Empty content array | Returns `ProviderError` |
| Zero token counts | Returns 0, no error |

---

## 4. Testing Requirements

### 4.1 MiniMax Provider Tests

| Test Name | Description |
|-----------|-------------|
| `TestNewMiniMax_EmptyAPIKey` | Empty API key returns error |
| `TestNewMiniMax_ValidAPIKey` | Valid API key returns provider |
| `TestMiniMax_Send_Success` | Valid response parsing |
| `TestMiniMax_Send_RequestFormat` | POST to correct endpoint |
| `TestMiniMax_Send_SystemInstruction` | System prompt in body |
| `TestMiniMax_Send_AuthError` | 401 → ErrCategoryAuth |
| `TestMiniMax_Send_RateLimitError` | 429 → ErrCategoryRateLimit |
| `TestMiniMax_Send_ServerError` | 500 → ErrCategoryServer |
| `TestMiniMax_Send_Timeout` | Context cancelled → ErrCategoryTimeout |
| `TestMiniMax_Send_EmptyContent` | Empty content → ProviderError |
| `TestMiniMax_Send_ToolCallResponse` | Tool calls parsed correctly |
| `TestMiniMax_Send_ToolDefinitions` | Tools sent correctly |

### 4.2 Registry Tests

| Test Name | Description |
|-----------|-------------|
| `TestNew_MiniMax` | Constructor works |
| `TestNew_MiniMaxWithBaseURL` | Custom base URL works |
| `TestNew_MiniMaxMissingAPIKey` | Missing API key error |
| `TestSupported_MiniMax` | Supported returns true |

### 4.3 Config Tests

| Test Name | Description |
|-----------|-------------|
| `TestAPIKeyEnvVar_MiniMax` | Returns MINIMAX_API_KEY |
| `TestResolveAPIKey_MiniMax` | Env var precedence |

---

## 5. Files Affected

| File | Action | Description |
|------|--------|-------------|
| `internal/provider/minimax.go` | CREATE | MiniMax provider implementation |
| `internal/provider/minimax_test.go` | CREATE | MiniMax provider tests |
| `internal/provider/registry.go` | MODIFY | Add minimax to registry |
| `internal/provider/registry_test.go` | MODIFY | Add MiniMax tests |
| `internal/config/config.go` | MODIFY | Add minimax to knownAPIKeyEnvVars |
| `internal/config/config_test.go` | MODIFY | Add MiniMax config tests |

---

## 6. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `provider.New("minimax", key, "")` works | Registry test |
| `provider.Supported("minimax")` returns true | Registry test |
| POST to `/text/chatcompletion_pro` | Request format test |
| `x-api-key` header sent | Request format test |
| HTTP errors mapped correctly | Error tests |
| Context cancellation handled | Timeout test |
| `APIKeyEnvVar("minimax")` returns `MINIMAX_API_KEY` | Config test |
| All tests pass | `make test` |

---

## 7. Out of Scope

1. Streaming output
2. Retry logic
3. Response caching
4. Multi-modal content
5. Token cost estimation
6. Integration tests against real MiniMax API
