# Implementation Checklist: M4 - Multi-Provider Support

**Based on:** 004_multi_provider_support_spec.md
**Status:** Not Started
**Created:** 2026-02-27

---

## Phase 1: Project Setup

- [x] Create branch `feat/004-multi-provider` off `develop`
- [x] Create `internal/config/` directory
- [x] Create a `Makefile` with a `test` target that runs `go test ./...`
- [x] Verify `go.mod` has no new dependencies after phase completion

---

## Phase 2: Global Configuration — Types and Load (`internal/config/config.go`) (Spec §3.1)

### 2a: Load Function

- [x] Write `TestLoad_FileNotFound` — XDG points to temp dir with no `config.toml`; verify empty `Providers` map, nil error
- [x] Write `TestLoad_EmptyFile` — empty `config.toml`; verify valid empty config, nil error
- [x] Write `TestLoad_ValidConfig` — full `config.toml` with providers section; verify all fields populated
- [x] Write `TestLoad_MalformedTOML` — invalid TOML content; verify error contains `failed to parse config file`
- [x] Write `TestLoad_PartialConfig` — only one provider section; verify that provider loaded, others missing from map
- [x] Define `ProviderConfig` struct with `APIKey` and `BaseURL` fields (TOML tags: `api_key`, `base_url`) (Req 1.3)
- [x] Define `GlobalConfig` struct with `Providers map[string]ProviderConfig` (TOML tag: `providers`) (Req 1.2)
- [x] Implement `Load() (*GlobalConfig, error)` (Req 1.4, 1.5):
  - [x] Resolve path via `xdg.GetConfigDir()` + `/config.toml`
  - [x] If file does not exist, return `GlobalConfig` with empty `Providers` map, nil error
  - [x] If file cannot be read (permissions), return error: `failed to read config file: <io_error>`
  - [x] If file is invalid TOML, return error: `failed to parse config file: <toml_error>`
  - [x] If file parses successfully, return populated `*GlobalConfig`
- [x] Run tests — all Load tests pass

### 2b: ResolveAPIKey Method

- [x] Write `TestResolveAPIKey_EnvVarTakesPrecedence` — env var AND config value set; verify env var returned
- [x] Write `TestResolveAPIKey_FallsBackToConfig` — env var unset, config value set; verify config value returned
- [x] Write `TestResolveAPIKey_NeitherSet` — no env var, no config value; verify empty string
- [x] Write `TestResolveAPIKey_EmptyEnvVar` — env var set to empty string, config value set; verify config value returned
- [x] Write `TestResolveAPIKey_NilProvidersMap` — nil `Providers` map; verify no panic, falls back to env var
- [x] Write `TestResolveAPIKey_UnknownProvider` — provider `"groq"`; verify checks `GROQ_API_KEY` env var
- [x] Implement `ResolveAPIKey(providerName string) string` (Req 1.6, 1.8, 1.9):
  - [x] Look up canonical env var: known providers use table (Req 1.8), unknown use `<PROVIDER_UPPER>_API_KEY`
  - [x] If env var is non-empty, return it
  - [x] If `Providers` map is non-nil and has entry for `providerName`, return `APIKey` field
  - [x] Otherwise return empty string
- [x] Run tests — all ResolveAPIKey tests pass

### 2c: ResolveBaseURL Method

- [x] Write `TestResolveBaseURL_EnvVarTakesPrecedence` — `AXE_OPENAI_BASE_URL` env var AND config value; verify env var wins
- [x] Write `TestResolveBaseURL_FallsBackToConfig` — env var unset, config value set; verify config value returned
- [x] Write `TestResolveBaseURL_NeitherSet` — neither source has value; verify empty string
- [x] Implement `ResolveBaseURL(providerName string) string` (Req 1.7, 1.8, 1.9):
  - [x] Env var is `AXE_<PROVIDER_UPPER>_BASE_URL`
  - [x] If env var is non-empty, return it
  - [x] If `Providers` map is non-nil and has entry, return `BaseURL` field
  - [x] Otherwise return empty string
- [x] Run tests — all ResolveBaseURL tests pass

---

## Phase 3: Provider Factory (`internal/provider/registry.go`) (Spec §3.2)

- [x] Write `TestNew_Anthropic` — `New("anthropic", "test-key", "")`; verify non-nil provider, no error
- [x] Write `TestNew_OpenAI` — `New("openai", "test-key", "")`; verify non-nil provider, no error
- [x] Write `TestNew_Ollama` — `New("ollama", "", "")`; verify non-nil provider, no error
- [x] Write `TestNew_OllamaIgnoresAPIKey` — `New("ollama", "ignored-key", "")`; verify no error
- [x] Write `TestNew_WithBaseURL` — `New("openai", "test-key", "http://custom:8080")`; verify no error
- [x] Write `TestNew_UnsupportedProvider` — `New("groq", "key", "")`; verify error contains `unsupported provider "groq"` and lists supported providers
- [x] Write `TestNew_EmptyProviderName` — `New("", "key", "")`; verify error message
- [x] Write `TestNew_CaseSensitive` — `New("Anthropic", "key", "")`; verify error
- [x] Write `TestNew_MissingAPIKeyAnthropic` — `New("anthropic", "", "")`; verify error about missing API key
- [x] Write `TestNew_MissingAPIKeyOpenAI` — `New("openai", "", "")`; verify error about missing API key
- [x] Implement `New(providerName, apiKey, baseURL string) (Provider, error)` (Req 2.1–2.6):
  - [x] Switch on `providerName` (case-sensitive)
  - [x] `"anthropic"`: call `NewAnthropic(apiKey, opts...)` with optional `WithBaseURL`
  - [x] `"openai"`: call `NewOpenAI(apiKey, opts...)` with optional `WithOpenAIBaseURL`
  - [x] `"ollama"`: call `NewOllama(opts...)` with optional `WithOllamaBaseURL`; ignore `apiKey`
  - [x] Default: return error `unsupported provider "<name>": supported providers are anthropic, openai, ollama`
  - [x] Propagate constructor errors without wrapping
- [x] Run tests — all factory tests pass (note: `TestNew_OpenAI` and `TestNew_Ollama` will fail until Phases 4 and 5 are complete; write them as red tests first, they turn green as providers are implemented)

---

## Phase 4: OpenAI Provider (`internal/provider/openai.go`) (Spec §3.3)

### 4a: Constructor

- [x] Write `TestNewOpenAI_EmptyAPIKey` — empty string returns error `API key is required`
- [x] Write `TestNewOpenAI_ValidAPIKey` — non-empty string returns `*OpenAI` with no error
- [x] Define `OpenAIOption` functional option type (Req 3.4)
- [x] Implement `WithOpenAIBaseURL(url string) OpenAIOption` (Req 3.4)
- [x] Define `defaultOpenAIBaseURL = "https://api.openai.com"` named constant (Req 3.18)
- [x] Define `OpenAI` struct with `apiKey`, `baseURL`, `client *http.Client` fields (Req 3.3)
- [x] Implement `NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error)` (Req 3.2):
  - [x] Return error if `apiKey` is empty
  - [x] Default `baseURL` to `defaultOpenAIBaseURL`
  - [x] Configure `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` (Req 3.17)
  - [x] Apply functional options
- [x] Run tests — constructor tests pass

### 4b: Send Method — Request Building

- [x] Write `TestOpenAI_Send_RequestFormat` — inspect request: POST, `/v1/chat/completions`, `Authorization: Bearer <key>`, `Content-Type: application/json`, correct body JSON (Req 3.5, 3.6, 3.7)
- [x] Write `TestOpenAI_Send_OmitsEmptySystem` — no system message when `System` is empty (Req 3.8)
- [x] Write `TestOpenAI_Send_OmitsZeroTemperature` — `temperature` absent from body when 0 (Req 3.9)
- [x] Write `TestOpenAI_Send_OmitsZeroMaxTokens` — `max_tokens` absent from body when 0 (Req 3.10)
- [x] Write `TestOpenAI_Send_IncludesMaxTokens` — `max_tokens` present when non-zero (Req 3.10)
- [x] Implement request building in `Send`:
  - [x] Build messages array: prepend system message if `System` non-empty (Req 3.8)
  - [x] Construct JSON body with `model`, `messages`; omit `temperature` when 0, omit `max_tokens` when 0 (Req 3.9, 3.10)
  - [x] Create `http.NewRequestWithContext` POST to `<baseURL>/v1/chat/completions` (Req 3.5)
  - [x] Set `Authorization: Bearer <apiKey>` and `Content-Type: application/json` headers (Req 3.6)
- [x] Run tests — request format tests pass

### 4c: Send Method — Success Response Parsing

- [x] Write `TestOpenAI_Send_Success` — `httptest.NewServer` returning valid OpenAI response; verify all `Response` fields (Req 3.11)
- [x] Write `TestOpenAI_Send_EmptyChoices` — empty `choices` array; verify `ProviderError` with `ErrCategoryServer`, message contains `no choices` (Req 3.12)
- [x] Implement response parsing in `Send`:
  - [x] Define response structs for JSON unmarshalling
  - [x] Map `choices[0].message.content` -> `Response.Content` (Req 3.11)
  - [x] Map `model` -> `Response.Model`
  - [x] Map `usage.prompt_tokens` -> `Response.InputTokens`
  - [x] Map `usage.completion_tokens` -> `Response.OutputTokens`
  - [x] Map `choices[0].finish_reason` -> `Response.StopReason`
  - [x] If `choices` is empty, return `ProviderError` with `ErrCategoryServer` (Req 3.12)
- [x] Run tests — success path tests pass

### 4d: Send Method — Error Handling

- [x] Write `TestOpenAI_Send_AuthError` — 401; verify `ProviderError` with `ErrCategoryAuth` (Req 3.13)
- [x] Write `TestOpenAI_Send_ForbiddenError` — 403; verify `ProviderError` with `ErrCategoryAuth` (Req 3.13)
- [x] Write `TestOpenAI_Send_NotFoundError` — 404; verify `ProviderError` with `ErrCategoryBadRequest` (Req 3.13)
- [x] Write `TestOpenAI_Send_RateLimitError` — 429; verify `ProviderError` with `ErrCategoryRateLimit` (Req 3.13)
- [x] Write `TestOpenAI_Send_ServerError` — 500; verify `ProviderError` with `ErrCategoryServer` (Req 3.13)
- [x] Write `TestOpenAI_Send_Timeout` — context deadline exceeded; verify `ProviderError` with `ErrCategoryTimeout` (Req 3.15)
- [x] Write `TestOpenAI_Send_ErrorResponseParsing` — 400 with OpenAI error JSON; verify message from parsed body (Req 3.14)
- [x] Write `TestOpenAI_Send_UnparseableErrorBody` — 400 with non-JSON body; verify fallback to HTTP status text (Req 3.14)
- [x] Implement error handling in `Send`:
  - [x] Map HTTP status codes to `ErrorCategory` (Req 3.13)
  - [x] Parse OpenAI error response body `{"error":{"message":...}}` for `ProviderError.Message` (Req 3.14)
  - [x] Fall back to HTTP status text if body is unparseable (Req 3.14)
  - [x] Detect context cancellation/deadline and return `ErrCategoryTimeout` (Req 3.15)
  - [x] No retry logic (Req 3.16)
- [x] Run tests — all OpenAI error tests pass

---

## Phase 5: Ollama Provider (`internal/provider/ollama.go`) (Spec §3.4)

### 5a: Constructor

- [x] Write `TestNewOllama_Defaults` — no options; verify no error (Req 4.2)
- [x] Write `TestNewOllama_WithBaseURL` — with `WithOllamaBaseURL`; verify no error (Req 4.4)
- [x] Define `OllamaOption` functional option type (Req 4.4)
- [x] Implement `WithOllamaBaseURL(url string) OllamaOption` (Req 4.4)
- [x] Define `defaultOllamaBaseURL = "http://localhost:11434"` named constant (Req 4.21)
- [x] Define `Ollama` struct with `baseURL`, `client *http.Client` fields (Req 4.3)
- [x] Implement `NewOllama(opts ...OllamaOption) (*Ollama, error)` (Req 4.2):
  - [x] Default `baseURL` to `defaultOllamaBaseURL`
  - [x] Configure `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` (Req 4.20)
  - [x] Apply functional options
  - [x] Always return valid `*Ollama`, nil error
- [x] Run tests — constructor tests pass

### 5b: Send Method — Request Building

- [x] Write `TestOllama_Send_RequestFormat` — inspect request: POST, `/api/chat`, `Content-Type: application/json`, no `Authorization` header, body has `stream: false` (Req 4.5, 4.6, 4.7, 4.8)
- [x] Write `TestOllama_Send_SystemMessage` — system prompt prepended as system message (Req 4.9)
- [x] Write `TestOllama_Send_OmitsEmptySystem` — no system message when empty (Req 4.9)
- [x] Write `TestOllama_Send_OmitsZeroTemperature` — `temperature` absent from `options` when 0 (Req 4.10)
- [x] Write `TestOllama_Send_OmitsZeroMaxTokens` — `num_predict` absent from `options` when 0 (Req 4.11)
- [x] Write `TestOllama_Send_OmitsOptionsWhenEmpty` — entire `options` object absent when both 0 (Req 4.12)
- [x] Write `TestOllama_Send_IncludesOptions` — both non-zero; `options` present with both fields (Req 4.10, 4.11)
- [x] Implement request building in `Send`:
  - [x] Build messages array: prepend system message if `System` non-empty (Req 4.9)
  - [x] Construct JSON body with `model`, `messages`, `stream: false` (Req 4.7, 4.8)
  - [x] Build `options` object: include `temperature` if non-zero, `num_predict` if non-zero; omit entire `options` if both zero (Req 4.10, 4.11, 4.12)
  - [x] Create `http.NewRequestWithContext` POST to `<baseURL>/api/chat` (Req 4.5)
  - [x] Set `Content-Type: application/json` header only (Req 4.6)
- [x] Run tests — request format tests pass

### 5c: Send Method — Success Response Parsing

- [x] Write `TestOllama_Send_Success` — `httptest.NewServer` returning valid Ollama response; verify all `Response` fields (Req 4.13)
- [x] Write `TestOllama_Send_EmptyContent` — empty `message.content`; verify `ProviderError` with `ErrCategoryServer` (Req 4.14)
- [x] Write `TestOllama_Send_ZeroTokenCounts` — zero/missing token counts; verify `Response.InputTokens` and `OutputTokens` are 0, no error (Req 4.13)
- [x] Implement response parsing in `Send`:
  - [x] Define response structs for JSON unmarshalling
  - [x] Map `message.content` -> `Response.Content` (Req 4.13)
  - [x] Map `model` -> `Response.Model`
  - [x] Map `prompt_eval_count` -> `Response.InputTokens`
  - [x] Map `eval_count` -> `Response.OutputTokens`
  - [x] Map `done_reason` -> `Response.StopReason`
  - [x] If `message.content` is empty, return `ProviderError` with `ErrCategoryServer`, message `response contains no content` (Req 4.14)
- [x] Run tests — success path tests pass

### 5d: Send Method — Error Handling

- [x] Write `TestOllama_Send_BadRequest` — 400; verify `ProviderError` with `ErrCategoryBadRequest` (Req 4.15)
- [x] Write `TestOllama_Send_NotFound` — 404; verify `ProviderError` with `ErrCategoryBadRequest` (Req 4.15)
- [x] Write `TestOllama_Send_ServerError` — 500; verify `ProviderError` with `ErrCategoryServer` (Req 4.15)
- [x] Write `TestOllama_Send_Timeout` — context deadline exceeded; verify `ProviderError` with `ErrCategoryTimeout` (Req 4.18)
- [x] Write `TestOllama_Send_ConnectionRefused` — base URL on dead port; verify `ProviderError` with `ErrCategoryServer`, message contains `connection refused` (Req 4.16)
- [x] Write `TestOllama_Send_ErrorResponseParsing` — 400 with Ollama error JSON `{"error":"..."}`; verify parsed message (Req 4.17)
- [x] Implement error handling in `Send`:
  - [x] Map HTTP status codes to `ErrorCategory` (Req 4.15)
  - [x] Parse Ollama error response body `{"error":"..."}` for `ProviderError.Message` (Req 4.17)
  - [x] Fall back to HTTP status text if body is unparseable (Req 4.17)
  - [x] Detect connection refused and return `ErrCategoryServer` with descriptive message (Req 4.16)
  - [x] Detect context cancellation/deadline and return `ErrCategoryTimeout` (Req 4.18)
  - [x] No retry logic (Req 4.19)
- [x] Run tests — all Ollama error tests pass

---

## Phase 6: Refactor `cmd/run.go` (Spec §3.5)

- [x] Write `TestRun_OpenAIProviderSuccess` — agent with `model = "openai/gpt-4o"`, mock OpenAI server; verify response output (Req 5.4)
- [x] Write `TestRun_OllamaProviderSuccess` — agent with `model = "ollama/llama3"`, mock Ollama server; verify response output (Req 5.4)
- [x] Write `TestRun_MissingAPIKeyOpenAI` — no `OPENAI_API_KEY`, no config; verify exit code 3 (Req 5.3)
- [x] Write `TestRun_OllamaNoAPIKeyRequired` — no API key set, mock server; verify success (Req 5.6)
- [x] Write `TestRun_APIKeyFromConfigFile` — `ANTHROPIC_API_KEY` unset, config file has key; verify request uses config key (Req 5.2)
- [x] Write `TestRun_MalformedGlobalConfig` — invalid TOML in `config.toml`; verify exit code 2 (Req 5.7)
- [x] Write `TestRun_DryRun_NonAnthropicProvider` — `model = "openai/gpt-4o"`, `--dry-run`; verify output shows model, exits 0 (Req 5.9)
- [x] Modify `TestRun_UnsupportedProvider` — change model to `"fakeprovider/some-model"`; verify error lists supported providers
- [x] Refactor `runAgent` in `cmd/run.go` (Req 5.1–5.6):
  - [x] Remove hardcoded `if provName != "anthropic"` check (Req 5.1)
  - [x] Add `config.Load()` call; if error, return `ExitError{Code: 2}` (Req 5.7)
  - [x] Replace `os.Getenv("ANTHROPIC_API_KEY")` with `globalCfg.ResolveAPIKey(provName)` (Req 5.2)
  - [x] Add API key check: if provider is not `"ollama"` and key is empty, return `ExitError{Code: 3}` with message including env var name and config.toml hint (Req 5.3, 5.5, 5.6)
  - [x] Replace `os.Getenv("AXE_ANTHROPIC_BASE_URL")` with `globalCfg.ResolveBaseURL(provName)` (Req 5.3)
  - [x] Replace `provider.NewAnthropic(apiKey, opts...)` with `provider.New(provName, apiKey, baseURL)` (Req 5.4)
  - [x] If factory returns error, return `ExitError{Code: 1}` (Req 5.5)
- [x] Verify `parseModel` function is unchanged (Req 5.8)
- [x] Verify all existing flags still work (Req 5.9)
- [x] Run tests — all `cmd/run_test.go` tests pass (existing + new)

---

## Phase 7: Update `axe config init` (`cmd/config.go`) (Spec §3.6)

- [x] Write `TestConfigInit_CreatesConfigTOML` — run `config init`; verify `config.toml` created at correct path (Req 6.1)
- [x] Write `TestConfigInit_ConfigTOMLPermissions` — verify `config.toml` has `0600` permissions (Req 6.4)
- [x] Write `TestConfigInit_ConfigTOMLContent` — verify scaffolded content contains commented-out provider sections (Req 6.2)
- [x] Write `TestConfigInit_DoesNotOverwriteConfigTOML` — create custom `config.toml`, run `config init`; verify not overwritten (Req 6.3)
- [x] Add `config.toml` scaffolding to `configInitCmd` in `cmd/config.go`:
  - [x] Resolve path: `xdg.GetConfigDir()` + `/config.toml`
  - [x] If file already exists, skip (idempotent) (Req 6.3)
  - [x] If file does not exist, write commented-out template with `0600` permissions (Req 6.2, 6.4)
- [x] Run tests — all `cmd/config_test.go` tests pass (existing + new)

---

## Phase 8: Final Verification

- [x] Run `make test` — all tests pass with 0 failures
- [x] Verify `go.mod` still contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies (Constraint 1)
- [x] Verify `internal/provider/anthropic.go` is unmodified (Constraint 7)
- [x] Verify `Provider` interface is unmodified (Constraint 8)
- [x] Verify `Request`, `Response`, `Message`, `ProviderError`, `ErrorCategory` types are unmodified (Constraint 9)
- [x] Verify all exit codes match spec table (Req 5.10)
- [x] Verify no real HTTP requests in tests — all use `httptest.NewServer`
