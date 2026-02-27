# Implementation Checklist: M4 - Multi-Provider Support

**Based on:** 004_multi_provider_support_spec.md
**Status:** Not Started
**Created:** 2026-02-27

---

## Phase 1: Project Setup

- [ ] Create branch `feat/004-multi-provider` off `develop`
- [ ] Create `internal/config/` directory
- [ ] Create a `Makefile` with a `test` target that runs `go test ./...`
- [ ] Verify `go.mod` has no new dependencies after phase completion

---

## Phase 2: Global Configuration — Types and Load (`internal/config/config.go`) (Spec §3.1)

### 2a: Load Function

- [ ] Write `TestLoad_FileNotFound` — XDG points to temp dir with no `config.toml`; verify empty `Providers` map, nil error
- [ ] Write `TestLoad_EmptyFile` — empty `config.toml`; verify valid empty config, nil error
- [ ] Write `TestLoad_ValidConfig` — full `config.toml` with providers section; verify all fields populated
- [ ] Write `TestLoad_MalformedTOML` — invalid TOML content; verify error contains `failed to parse config file`
- [ ] Write `TestLoad_PartialConfig` — only one provider section; verify that provider loaded, others missing from map
- [ ] Define `ProviderConfig` struct with `APIKey` and `BaseURL` fields (TOML tags: `api_key`, `base_url`) (Req 1.3)
- [ ] Define `GlobalConfig` struct with `Providers map[string]ProviderConfig` (TOML tag: `providers`) (Req 1.2)
- [ ] Implement `Load() (*GlobalConfig, error)` (Req 1.4, 1.5):
  - [ ] Resolve path via `xdg.GetConfigDir()` + `/config.toml`
  - [ ] If file does not exist, return `GlobalConfig` with empty `Providers` map, nil error
  - [ ] If file cannot be read (permissions), return error: `failed to read config file: <io_error>`
  - [ ] If file is invalid TOML, return error: `failed to parse config file: <toml_error>`
  - [ ] If file parses successfully, return populated `*GlobalConfig`
- [ ] Run tests — all Load tests pass

### 2b: ResolveAPIKey Method

- [ ] Write `TestResolveAPIKey_EnvVarTakesPrecedence` — env var AND config value set; verify env var returned
- [ ] Write `TestResolveAPIKey_FallsBackToConfig` — env var unset, config value set; verify config value returned
- [ ] Write `TestResolveAPIKey_NeitherSet` — no env var, no config value; verify empty string
- [ ] Write `TestResolveAPIKey_EmptyEnvVar` — env var set to empty string, config value set; verify config value returned
- [ ] Write `TestResolveAPIKey_NilProvidersMap` — nil `Providers` map; verify no panic, falls back to env var
- [ ] Write `TestResolveAPIKey_UnknownProvider` — provider `"groq"`; verify checks `GROQ_API_KEY` env var
- [ ] Implement `ResolveAPIKey(providerName string) string` (Req 1.6, 1.8, 1.9):
  - [ ] Look up canonical env var: known providers use table (Req 1.8), unknown use `<PROVIDER_UPPER>_API_KEY`
  - [ ] If env var is non-empty, return it
  - [ ] If `Providers` map is non-nil and has entry for `providerName`, return `APIKey` field
  - [ ] Otherwise return empty string
- [ ] Run tests — all ResolveAPIKey tests pass

### 2c: ResolveBaseURL Method

- [ ] Write `TestResolveBaseURL_EnvVarTakesPrecedence` — `AXE_OPENAI_BASE_URL` env var AND config value; verify env var wins
- [ ] Write `TestResolveBaseURL_FallsBackToConfig` — env var unset, config value set; verify config value returned
- [ ] Write `TestResolveBaseURL_NeitherSet` — neither source has value; verify empty string
- [ ] Implement `ResolveBaseURL(providerName string) string` (Req 1.7, 1.8, 1.9):
  - [ ] Env var is `AXE_<PROVIDER_UPPER>_BASE_URL`
  - [ ] If env var is non-empty, return it
  - [ ] If `Providers` map is non-nil and has entry, return `BaseURL` field
  - [ ] Otherwise return empty string
- [ ] Run tests — all ResolveBaseURL tests pass

---

## Phase 3: Provider Factory (`internal/provider/registry.go`) (Spec §3.2)

- [ ] Write `TestNew_Anthropic` — `New("anthropic", "test-key", "")`; verify non-nil provider, no error
- [ ] Write `TestNew_OpenAI` — `New("openai", "test-key", "")`; verify non-nil provider, no error
- [ ] Write `TestNew_Ollama` — `New("ollama", "", "")`; verify non-nil provider, no error
- [ ] Write `TestNew_OllamaIgnoresAPIKey` — `New("ollama", "ignored-key", "")`; verify no error
- [ ] Write `TestNew_WithBaseURL` — `New("openai", "test-key", "http://custom:8080")`; verify no error
- [ ] Write `TestNew_UnsupportedProvider` — `New("groq", "key", "")`; verify error contains `unsupported provider "groq"` and lists supported providers
- [ ] Write `TestNew_EmptyProviderName` — `New("", "key", "")`; verify error message
- [ ] Write `TestNew_CaseSensitive` — `New("Anthropic", "key", "")`; verify error
- [ ] Write `TestNew_MissingAPIKeyAnthropic` — `New("anthropic", "", "")`; verify error about missing API key
- [ ] Write `TestNew_MissingAPIKeyOpenAI` — `New("openai", "", "")`; verify error about missing API key
- [ ] Implement `New(providerName, apiKey, baseURL string) (Provider, error)` (Req 2.1–2.6):
  - [ ] Switch on `providerName` (case-sensitive)
  - [ ] `"anthropic"`: call `NewAnthropic(apiKey, opts...)` with optional `WithBaseURL`
  - [ ] `"openai"`: call `NewOpenAI(apiKey, opts...)` with optional `WithOpenAIBaseURL`
  - [ ] `"ollama"`: call `NewOllama(opts...)` with optional `WithOllamaBaseURL`; ignore `apiKey`
  - [ ] Default: return error `unsupported provider "<name>": supported providers are anthropic, openai, ollama`
  - [ ] Propagate constructor errors without wrapping
- [ ] Run tests — all factory tests pass (note: `TestNew_OpenAI` and `TestNew_Ollama` will fail until Phases 4 and 5 are complete; write them as red tests first, they turn green as providers are implemented)

---

## Phase 4: OpenAI Provider (`internal/provider/openai.go`) (Spec §3.3)

### 4a: Constructor

- [ ] Write `TestNewOpenAI_EmptyAPIKey` — empty string returns error `API key is required`
- [ ] Write `TestNewOpenAI_ValidAPIKey` — non-empty string returns `*OpenAI` with no error
- [ ] Define `OpenAIOption` functional option type (Req 3.4)
- [ ] Implement `WithOpenAIBaseURL(url string) OpenAIOption` (Req 3.4)
- [ ] Define `defaultOpenAIBaseURL = "https://api.openai.com"` named constant (Req 3.18)
- [ ] Define `OpenAI` struct with `apiKey`, `baseURL`, `client *http.Client` fields (Req 3.3)
- [ ] Implement `NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error)` (Req 3.2):
  - [ ] Return error if `apiKey` is empty
  - [ ] Default `baseURL` to `defaultOpenAIBaseURL`
  - [ ] Configure `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` (Req 3.17)
  - [ ] Apply functional options
- [ ] Run tests — constructor tests pass

### 4b: Send Method — Request Building

- [ ] Write `TestOpenAI_Send_RequestFormat` — inspect request: POST, `/v1/chat/completions`, `Authorization: Bearer <key>`, `Content-Type: application/json`, correct body JSON (Req 3.5, 3.6, 3.7)
- [ ] Write `TestOpenAI_Send_OmitsEmptySystem` — no system message when `System` is empty (Req 3.8)
- [ ] Write `TestOpenAI_Send_OmitsZeroTemperature` — `temperature` absent from body when 0 (Req 3.9)
- [ ] Write `TestOpenAI_Send_OmitsZeroMaxTokens` — `max_tokens` absent from body when 0 (Req 3.10)
- [ ] Write `TestOpenAI_Send_IncludesMaxTokens` — `max_tokens` present when non-zero (Req 3.10)
- [ ] Implement request building in `Send`:
  - [ ] Build messages array: prepend system message if `System` non-empty (Req 3.8)
  - [ ] Construct JSON body with `model`, `messages`; omit `temperature` when 0, omit `max_tokens` when 0 (Req 3.9, 3.10)
  - [ ] Create `http.NewRequestWithContext` POST to `<baseURL>/v1/chat/completions` (Req 3.5)
  - [ ] Set `Authorization: Bearer <apiKey>` and `Content-Type: application/json` headers (Req 3.6)
- [ ] Run tests — request format tests pass

### 4c: Send Method — Success Response Parsing

- [ ] Write `TestOpenAI_Send_Success` — `httptest.NewServer` returning valid OpenAI response; verify all `Response` fields (Req 3.11)
- [ ] Write `TestOpenAI_Send_EmptyChoices` — empty `choices` array; verify `ProviderError` with `ErrCategoryServer`, message contains `no choices` (Req 3.12)
- [ ] Implement response parsing in `Send`:
  - [ ] Define response structs for JSON unmarshalling
  - [ ] Map `choices[0].message.content` -> `Response.Content` (Req 3.11)
  - [ ] Map `model` -> `Response.Model`
  - [ ] Map `usage.prompt_tokens` -> `Response.InputTokens`
  - [ ] Map `usage.completion_tokens` -> `Response.OutputTokens`
  - [ ] Map `choices[0].finish_reason` -> `Response.StopReason`
  - [ ] If `choices` is empty, return `ProviderError` with `ErrCategoryServer` (Req 3.12)
- [ ] Run tests — success path tests pass

### 4d: Send Method — Error Handling

- [ ] Write `TestOpenAI_Send_AuthError` — 401; verify `ProviderError` with `ErrCategoryAuth` (Req 3.13)
- [ ] Write `TestOpenAI_Send_ForbiddenError` — 403; verify `ProviderError` with `ErrCategoryAuth` (Req 3.13)
- [ ] Write `TestOpenAI_Send_NotFoundError` — 404; verify `ProviderError` with `ErrCategoryBadRequest` (Req 3.13)
- [ ] Write `TestOpenAI_Send_RateLimitError` — 429; verify `ProviderError` with `ErrCategoryRateLimit` (Req 3.13)
- [ ] Write `TestOpenAI_Send_ServerError` — 500; verify `ProviderError` with `ErrCategoryServer` (Req 3.13)
- [ ] Write `TestOpenAI_Send_Timeout` — context deadline exceeded; verify `ProviderError` with `ErrCategoryTimeout` (Req 3.15)
- [ ] Write `TestOpenAI_Send_ErrorResponseParsing` — 400 with OpenAI error JSON; verify message from parsed body (Req 3.14)
- [ ] Write `TestOpenAI_Send_UnparseableErrorBody` — 400 with non-JSON body; verify fallback to HTTP status text (Req 3.14)
- [ ] Implement error handling in `Send`:
  - [ ] Map HTTP status codes to `ErrorCategory` (Req 3.13)
  - [ ] Parse OpenAI error response body `{"error":{"message":...}}` for `ProviderError.Message` (Req 3.14)
  - [ ] Fall back to HTTP status text if body is unparseable (Req 3.14)
  - [ ] Detect context cancellation/deadline and return `ErrCategoryTimeout` (Req 3.15)
  - [ ] No retry logic (Req 3.16)
- [ ] Run tests — all OpenAI error tests pass

---

## Phase 5: Ollama Provider (`internal/provider/ollama.go`) (Spec §3.4)

### 5a: Constructor

- [ ] Write `TestNewOllama_Defaults` — no options; verify no error (Req 4.2)
- [ ] Write `TestNewOllama_WithBaseURL` — with `WithOllamaBaseURL`; verify no error (Req 4.4)
- [ ] Define `OllamaOption` functional option type (Req 4.4)
- [ ] Implement `WithOllamaBaseURL(url string) OllamaOption` (Req 4.4)
- [ ] Define `defaultOllamaBaseURL = "http://localhost:11434"` named constant (Req 4.21)
- [ ] Define `Ollama` struct with `baseURL`, `client *http.Client` fields (Req 4.3)
- [ ] Implement `NewOllama(opts ...OllamaOption) (*Ollama, error)` (Req 4.2):
  - [ ] Default `baseURL` to `defaultOllamaBaseURL`
  - [ ] Configure `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` (Req 4.20)
  - [ ] Apply functional options
  - [ ] Always return valid `*Ollama`, nil error
- [ ] Run tests — constructor tests pass

### 5b: Send Method — Request Building

- [ ] Write `TestOllama_Send_RequestFormat` — inspect request: POST, `/api/chat`, `Content-Type: application/json`, no `Authorization` header, body has `stream: false` (Req 4.5, 4.6, 4.7, 4.8)
- [ ] Write `TestOllama_Send_SystemMessage` — system prompt prepended as system message (Req 4.9)
- [ ] Write `TestOllama_Send_OmitsEmptySystem` — no system message when empty (Req 4.9)
- [ ] Write `TestOllama_Send_OmitsZeroTemperature` — `temperature` absent from `options` when 0 (Req 4.10)
- [ ] Write `TestOllama_Send_OmitsZeroMaxTokens` — `num_predict` absent from `options` when 0 (Req 4.11)
- [ ] Write `TestOllama_Send_OmitsOptionsWhenEmpty` — entire `options` object absent when both 0 (Req 4.12)
- [ ] Write `TestOllama_Send_IncludesOptions` — both non-zero; `options` present with both fields (Req 4.10, 4.11)
- [ ] Implement request building in `Send`:
  - [ ] Build messages array: prepend system message if `System` non-empty (Req 4.9)
  - [ ] Construct JSON body with `model`, `messages`, `stream: false` (Req 4.7, 4.8)
  - [ ] Build `options` object: include `temperature` if non-zero, `num_predict` if non-zero; omit entire `options` if both zero (Req 4.10, 4.11, 4.12)
  - [ ] Create `http.NewRequestWithContext` POST to `<baseURL>/api/chat` (Req 4.5)
  - [ ] Set `Content-Type: application/json` header only (Req 4.6)
- [ ] Run tests — request format tests pass

### 5c: Send Method — Success Response Parsing

- [ ] Write `TestOllama_Send_Success` — `httptest.NewServer` returning valid Ollama response; verify all `Response` fields (Req 4.13)
- [ ] Write `TestOllama_Send_EmptyContent` — empty `message.content`; verify `ProviderError` with `ErrCategoryServer` (Req 4.14)
- [ ] Write `TestOllama_Send_ZeroTokenCounts` — zero/missing token counts; verify `Response.InputTokens` and `OutputTokens` are 0, no error (Req 4.13)
- [ ] Implement response parsing in `Send`:
  - [ ] Define response structs for JSON unmarshalling
  - [ ] Map `message.content` -> `Response.Content` (Req 4.13)
  - [ ] Map `model` -> `Response.Model`
  - [ ] Map `prompt_eval_count` -> `Response.InputTokens`
  - [ ] Map `eval_count` -> `Response.OutputTokens`
  - [ ] Map `done_reason` -> `Response.StopReason`
  - [ ] If `message.content` is empty, return `ProviderError` with `ErrCategoryServer`, message `response contains no content` (Req 4.14)
- [ ] Run tests — success path tests pass

### 5d: Send Method — Error Handling

- [ ] Write `TestOllama_Send_BadRequest` — 400; verify `ProviderError` with `ErrCategoryBadRequest` (Req 4.15)
- [ ] Write `TestOllama_Send_NotFound` — 404; verify `ProviderError` with `ErrCategoryBadRequest` (Req 4.15)
- [ ] Write `TestOllama_Send_ServerError` — 500; verify `ProviderError` with `ErrCategoryServer` (Req 4.15)
- [ ] Write `TestOllama_Send_Timeout` — context deadline exceeded; verify `ProviderError` with `ErrCategoryTimeout` (Req 4.18)
- [ ] Write `TestOllama_Send_ConnectionRefused` — base URL on dead port; verify `ProviderError` with `ErrCategoryServer`, message contains `connection refused` (Req 4.16)
- [ ] Write `TestOllama_Send_ErrorResponseParsing` — 400 with Ollama error JSON `{"error":"..."}`; verify parsed message (Req 4.17)
- [ ] Implement error handling in `Send`:
  - [ ] Map HTTP status codes to `ErrorCategory` (Req 4.15)
  - [ ] Parse Ollama error response body `{"error":"..."}` for `ProviderError.Message` (Req 4.17)
  - [ ] Fall back to HTTP status text if body is unparseable (Req 4.17)
  - [ ] Detect connection refused and return `ErrCategoryServer` with descriptive message (Req 4.16)
  - [ ] Detect context cancellation/deadline and return `ErrCategoryTimeout` (Req 4.18)
  - [ ] No retry logic (Req 4.19)
- [ ] Run tests — all Ollama error tests pass

---

## Phase 6: Refactor `cmd/run.go` (Spec §3.5)

- [ ] Write `TestRun_OpenAIProviderSuccess` — agent with `model = "openai/gpt-4o"`, mock OpenAI server; verify response output (Req 5.4)
- [ ] Write `TestRun_OllamaProviderSuccess` — agent with `model = "ollama/llama3"`, mock Ollama server; verify response output (Req 5.4)
- [ ] Write `TestRun_MissingAPIKeyOpenAI` — no `OPENAI_API_KEY`, no config; verify exit code 3 (Req 5.3)
- [ ] Write `TestRun_OllamaNoAPIKeyRequired` — no API key set, mock server; verify success (Req 5.6)
- [ ] Write `TestRun_APIKeyFromConfigFile` — `ANTHROPIC_API_KEY` unset, config file has key; verify request uses config key (Req 5.2)
- [ ] Write `TestRun_MalformedGlobalConfig` — invalid TOML in `config.toml`; verify exit code 2 (Req 5.7)
- [ ] Write `TestRun_DryRun_NonAnthropicProvider` — `model = "openai/gpt-4o"`, `--dry-run`; verify output shows model, exits 0 (Req 5.9)
- [ ] Modify `TestRun_UnsupportedProvider` — change model to `"fakeprovider/some-model"`; verify error lists supported providers
- [ ] Refactor `runAgent` in `cmd/run.go` (Req 5.1–5.6):
  - [ ] Remove hardcoded `if provName != "anthropic"` check (Req 5.1)
  - [ ] Add `config.Load()` call; if error, return `ExitError{Code: 2}` (Req 5.7)
  - [ ] Replace `os.Getenv("ANTHROPIC_API_KEY")` with `globalCfg.ResolveAPIKey(provName)` (Req 5.2)
  - [ ] Add API key check: if provider is not `"ollama"` and key is empty, return `ExitError{Code: 3}` with message including env var name and config.toml hint (Req 5.3, 5.5, 5.6)
  - [ ] Replace `os.Getenv("AXE_ANTHROPIC_BASE_URL")` with `globalCfg.ResolveBaseURL(provName)` (Req 5.3)
  - [ ] Replace `provider.NewAnthropic(apiKey, opts...)` with `provider.New(provName, apiKey, baseURL)` (Req 5.4)
  - [ ] If factory returns error, return `ExitError{Code: 1}` (Req 5.5)
- [ ] Verify `parseModel` function is unchanged (Req 5.8)
- [ ] Verify all existing flags still work (Req 5.9)
- [ ] Run tests — all `cmd/run_test.go` tests pass (existing + new)

---

## Phase 7: Update `axe config init` (`cmd/config.go`) (Spec §3.6)

- [ ] Write `TestConfigInit_CreatesConfigTOML` — run `config init`; verify `config.toml` created at correct path (Req 6.1)
- [ ] Write `TestConfigInit_ConfigTOMLPermissions` — verify `config.toml` has `0600` permissions (Req 6.4)
- [ ] Write `TestConfigInit_ConfigTOMLContent` — verify scaffolded content contains commented-out provider sections (Req 6.2)
- [ ] Write `TestConfigInit_DoesNotOverwriteConfigTOML` — create custom `config.toml`, run `config init`; verify not overwritten (Req 6.3)
- [ ] Add `config.toml` scaffolding to `configInitCmd` in `cmd/config.go`:
  - [ ] Resolve path: `xdg.GetConfigDir()` + `/config.toml`
  - [ ] If file already exists, skip (idempotent) (Req 6.3)
  - [ ] If file does not exist, write commented-out template with `0600` permissions (Req 6.2, 6.4)
- [ ] Run tests — all `cmd/config_test.go` tests pass (existing + new)

---

## Phase 8: Final Verification

- [ ] Run `make test` — all tests pass with 0 failures
- [ ] Verify `go.mod` still contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies (Constraint 1)
- [ ] Verify `internal/provider/anthropic.go` is unmodified (Constraint 7)
- [ ] Verify `Provider` interface is unmodified (Constraint 8)
- [ ] Verify `Request`, `Response`, `Message`, `ProviderError`, `ErrorCategory` types are unmodified (Constraint 9)
- [ ] Verify all exit codes match spec table (Req 5.10)
- [ ] Verify no real HTTP requests in tests — all use `httptest.NewServer`
