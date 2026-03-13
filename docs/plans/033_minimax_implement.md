# Implementation Guide: MiniMax Provider Support

## 1. Context Summary

**Associated Milestone:** `docs/plans/000_milestones.md` — M4 (Multi-Provider Support)

Axe's multi-provider infrastructure is complete. This implementation adds a MiniMax provider that wraps the existing Anthropic provider with a different base URL. This is extremely simple — MiniMax provides an Anthropic-compatible API, so we reuse all Anthropic logic and just change the endpoint.

The provider name is `"minimax"`, the API key env var is `MINIMAX_API_KEY` (standard mapping).

## 2. Implementation Checklist

### Phase 1: Config Integration

- [x] Add `"minimax": "MINIMAX_API_KEY"` entry to the `knownAPIKeyEnvVars` map in `internal/config/config.go`.
- [x] Add test `TestAPIKeyEnvVar_MiniMax` in `internal/config/config_test.go`: assert `APIKeyEnvVar("minimax")` returns `"MINIMAX_API_KEY"`.
- [x] Add test `TestResolveAPIKey_MiniMax` in `internal/config/config_test.go`: verify env var `MINIMAX_API_KEY` takes precedence over `providers.minimax.api_key` in config; verify config fallback when env var is empty; verify empty string when neither is set.

### Phase 2: MiniMax Provider — Constructor

- [x] Create `internal/provider/minimax.go` that wraps the Anthropic provider: named constant `defaultMiniMaxBaseURL = "https://api.minimax.io/anthropic"`, function `NewMiniMax(apiKey string, opts ...AnthropicOption) (*Anthropic, error)`.
- [x] Reuse the Anthropic provider's struct, wire types, and methods directly — no duplication needed.
- [x] Implement `NewMiniMax(apiKey string, opts ...AnthropicOption) (*Anthropic, error)` in `internal/provider/minimax.go`: return error `"API key is required"` if apiKey is empty; initialize `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse`; apply functional options from Anthropic.
- [x] Add test `TestNewMiniMax_EmptyAPIKey` in `internal/provider/minimax_test.go`: verify empty API key returns error containing `"API key is required"`.
- [x] Add test `TestNewMiniMax_ValidAPIKey` in `internal/provider/minimax_test.go`: verify non-empty API key returns `*Anthropic` with no error.

### Phase 3: MiniMax Provider — Send Method

- [x] Inherited from Anthropic provider — no implementation needed. The `*Anthropic` returned by `NewMiniMax` has the `Send` method already.

### Phase 4: MiniMax Provider — Tests

- [x] Add test `TestMiniMax_Send_Success` in `internal/provider/minimax_test.go`: use mock server with Anthropic-compatible response format; verify `Response.Content`, `Response.Model`, `Response.InputTokens`, `Response.OutputTokens`, `Response.StopReason`.
- [x] Add test `TestMiniMax_Send_RequestFormat` in `internal/provider/minimax_test.go`: inspect captured request method (POST), URL path, headers (`x-api-key`), and body JSON structure.
- [x] Add test `TestMiniMax_Send_SystemInstruction` in `internal/provider/minimax_test.go`: verify system prompt in request body.
- [x] Add test `TestMiniMax_Send_AuthError` in `internal/provider/minimax_test.go`: 401 status → `ProviderError` with `ErrCategoryAuth`.
- [x] Add test `TestMiniMax_Send_RateLimitError` in `internal/provider/minimax_test.go`: 429 status → `ProviderError` with `ErrCategoryRateLimit`.
- [x] Add test `TestMiniMax_Send_ServerError` in `internal/provider/minimax_test.go`: 500 status → `ProviderError` with `ErrCategoryServer`.
- [x] Add test `TestMiniMax_Send_Timeout` in `internal/provider/minimax_test.go`: use cancelled context; verify `ProviderError` with `ErrCategoryTimeout`.
- [x] Add test `TestMiniMax_Send_EmptyContent` in `internal/provider/minimax_test.go`: response with empty content array → `ProviderError` with `ErrCategoryServer`.
- [x] Add test `TestMiniMax_Send_ToolCallResponse` in `internal/provider/minimax_test.go`: response with tool_use blocks; verify `Response.ToolCalls` parsed correctly.
- [x] Add test `TestMiniMax_Send_ToolDefinitions` in `internal/provider/minimax_test.go`: verify tools in request body.

### Phase 5: Registry Integration

- [x] Add `"minimax": true` to the `supportedProviders` map in `internal/provider/registry.go`.
- [x] Add `case "minimax":` to the `New()` switch in `internal/provider/registry.go`: construct `[]AnthropicOption`, append `WithAnthropicBaseURL(baseURL)` when baseURL non-empty, call `NewMiniMax(apiKey, opts...)`.
- [x] Update the default error message in `New()` in `internal/provider/registry.go` to include `minimax` in the list: `"anthropic, openai, ollama, opencode, google, minimax"`.
- [x] Add test `TestNew_MiniMax` in `internal/provider/registry_test.go`: `New("minimax", "test-key", "")` returns non-nil provider, no error.
- [x] Add test `TestNew_MiniMaxWithBaseURL` in `internal/provider/registry_test.go`: `New("minimax", "test-key", "http://custom:8080")` succeeds.
- [x] Add test `TestNew_MiniMaxMissingAPIKey` in `internal/provider/registry_test.go`: `New("minimax", "", "")` returns error containing `"API key is required"`.
- [x] Add test `TestSupported_MiniMax` in `internal/provider/registry_test.go`: `Supported("minimax")` returns `true`.
- [x] Update `TestSupported_KnownProviders` in `internal/provider/registry_test.go`: add `"minimax"` to the provider name list.
- [x] Update `TestNew_UnsupportedProvider` in `internal/provider/registry_test.go`: change assertion to check for `"anthropic, openai, ollama, opencode, google, minimax"`.
- [x] Update `TestNew_UnsupportedProvider_ErrorMessage` in `internal/provider/registry_test.go`: add `"minimax"` to the list of names that must appear in the error message.

### Phase 6: Final Verification

- [x] Run `make test` — all tests pass, no new entries in `go.mod`, no real HTTP requests.

## 3. Design Notes

Since MiniMax uses the Anthropic-compatible API:
- The implementation wraps the Anthropic provider directly
- `NewMiniMax` returns `*Anthropic` — not a new type
- Uses `AnthropicOption` functional options (the same as Anthropic)
- The only difference is the default base URL (`https://api.minimax.io/anthropic` vs Anthropic's default)
- All request/response handling, error mapping, and tests are inherited from Anthropic
