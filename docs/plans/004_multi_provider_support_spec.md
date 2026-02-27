# Specification: M4 - Multi-Provider Support

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-27
**Scope:** Provider abstraction, OpenAI provider, Ollama provider, global API key configuration

---

## 1. Purpose

Replace the hardcoded Anthropic-only provider selection in `axe run` with a multi-provider architecture. This milestone introduces a provider factory, two new provider implementations (OpenAI, Ollama), and a global configuration file for API key management. After M4, users can run agents against any supported provider by changing the `model` field in their agent TOML (e.g. `"openai/gpt-4o"`, `"ollama/llama3"`, `"anthropic/claude-sonnet-4-20250514"`).

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **API key storage:** Both environment variables and a global config file (`$XDG_CONFIG_HOME/axe/config.toml`). Env vars take precedence over config file values.
2. **Ollama implementation:** Fully separate provider implementation. No shared code with OpenAI, despite API similarities.
3. **Dependencies:** Continue using stdlib-only (`net/http`, `encoding/json`). No LLM SDK dependencies.
4. **Git branching:** Create `develop` branch from `master`. Branch `feat/004-multi-provider` from `develop`.

---

## 3. Requirements

### 3.1 Global Configuration File (`internal/config/`)

**Requirement 1.1:** Create a new package `internal/config/` for global configuration management.

**Requirement 1.2:** Define a `GlobalConfig` struct representing the parsed global config file:

```go
type GlobalConfig struct {
    Providers map[string]ProviderConfig `toml:"providers"`
}
```

**Requirement 1.3:** Define a `ProviderConfig` struct:

| Go Field | TOML Key | Go Type | Description |
|----------|----------|---------|-------------|
| `APIKey` | `api_key` | `string` | Provider API key |
| `BaseURL` | `base_url` | `string` | Custom base URL override |

**Requirement 1.4:** The global config file path is `$XDG_CONFIG_HOME/axe/config.toml`. Use `xdg.GetConfigDir()` to resolve the base directory.

**Requirement 1.5:** Implement a `Load` function:

```go
func Load() (*GlobalConfig, error)
```

Behavior:
1. Resolve the config file path via `xdg.GetConfigDir()` + `/config.toml`
2. If the file does not exist, return a valid `GlobalConfig` with an empty `Providers` map and no error. A missing config file is not an error.
3. If the file exists but is not valid TOML, return an error: `failed to parse config file: <toml_error>`
4. If the file cannot be read (permissions, etc.), return an error: `failed to read config file: <io_error>`
5. If the file exists and parses successfully, return the populated `*GlobalConfig`

**Requirement 1.6:** Implement a `ResolveAPIKey` method on `GlobalConfig`:

```go
func (c *GlobalConfig) ResolveAPIKey(providerName string) string
```

Resolution order (first non-empty value wins):
1. Environment variable using the provider's canonical env var name (see Requirement 1.8)
2. Config file value at `providers.<providerName>.api_key`
3. Return empty string if neither source has a value

**Requirement 1.7:** Implement a `ResolveBaseURL` method on `GlobalConfig`:

```go
func (c *GlobalConfig) ResolveBaseURL(providerName string) string
```

Resolution order (first non-empty value wins):
1. Environment variable `AXE_<PROVIDER_UPPER>_BASE_URL` (e.g. `AXE_ANTHROPIC_BASE_URL`, `AXE_OPENAI_BASE_URL`, `AXE_OLLAMA_BASE_URL`)
2. Config file value at `providers.<providerName>.base_url`
3. Return empty string if neither source has a value (provider will use its built-in default)

**Requirement 1.8:** Canonical environment variable names for API keys:

| Provider Name | API Key Env Var | Base URL Env Var |
|---------------|-----------------|------------------|
| `anthropic` | `ANTHROPIC_API_KEY` | `AXE_ANTHROPIC_BASE_URL` |
| `openai` | `OPENAI_API_KEY` | `AXE_OPENAI_BASE_URL` |
| `ollama` | *(none — Ollama does not require an API key)* | `AXE_OLLAMA_BASE_URL` |

For unknown provider names, the env var convention is `<PROVIDER_UPPER>_API_KEY` and `AXE_<PROVIDER_UPPER>_BASE_URL`, where `<PROVIDER_UPPER>` is the provider name converted to uppercase. This allows future providers without code changes to the config package.

**Requirement 1.9:** The `ResolveAPIKey` and `ResolveBaseURL` methods must handle a nil or empty `Providers` map without panicking.

**Requirement 1.10:** Example `config.toml` structure:

```toml
[providers.anthropic]
api_key = "sk-ant-..."

[providers.openai]
api_key = "sk-..."

[providers.ollama]
base_url = "http://localhost:11434"
```

All fields are optional. A completely empty `config.toml` is valid.

### 3.2 Provider Factory (`internal/provider/`)

**Requirement 2.1:** Implement a provider factory function:

```go
func New(providerName, apiKey, baseURL string) (Provider, error)
```

**Requirement 2.2:** The factory must dispatch to the correct provider constructor based on `providerName`:

| Provider Name | Constructor Called | Notes |
|---------------|-------------------|-------|
| `"anthropic"` | `NewAnthropic(apiKey, opts...)` | `apiKey` is required (non-empty) |
| `"openai"` | `NewOpenAI(apiKey, opts...)` | `apiKey` is required (non-empty) |
| `"ollama"` | `NewOllama(opts...)` | `apiKey` is ignored; Ollama does not require authentication |

**Requirement 2.3:** If `baseURL` is non-empty, pass it as a functional option to the provider constructor (e.g. `WithBaseURL(baseURL)`, `WithOpenAIBaseURL(baseURL)`, `WithOllamaBaseURL(baseURL)`).

**Requirement 2.4:** If `providerName` is not one of the supported values (`"anthropic"`, `"openai"`, `"ollama"`), return an error: `unsupported provider "<name>": supported providers are anthropic, openai, ollama`.

**Requirement 2.5:** If the provider requires an API key and `apiKey` is empty, the provider constructor will return an error. The factory must propagate this error to the caller without wrapping it further.

**Requirement 2.6:** Provider name matching must be case-sensitive. `"Anthropic"` and `"OPENAI"` are not valid provider names. Only lowercase names are accepted.

### 3.3 OpenAI Provider (`internal/provider/`)

**Requirement 3.1:** Implement an `OpenAI` struct that satisfies the `Provider` interface.

**Requirement 3.2:** Constructor function signature:

```go
func NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error)
```

The constructor must return an error if `apiKey` is an empty string: `API key is required`.

**Requirement 3.3:** Define the `OpenAI` struct:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `apiKey` | `string` | *(required)* | OpenAI API key |
| `baseURL` | `string` | `"https://api.openai.com"` | API base URL |
| `client` | `*http.Client` | Default client with no redirects | HTTP client |

**Requirement 3.4:** Functional option for base URL override:

```go
type OpenAIOption func(*OpenAI)
func WithOpenAIBaseURL(url string) OpenAIOption
```

**Requirement 3.5:** The `Send` method must make an HTTP POST request to `<baseURL>/v1/chat/completions`.

**Requirement 3.6:** Required HTTP headers:

| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <apiKey>` |
| `Content-Type` | `application/json` |

**Requirement 3.7:** Request JSON body must conform to the OpenAI Chat Completions API format:

```json
{
  "model": "<model>",
  "messages": [
    {"role": "system", "content": "<system_prompt>"},
    {"role": "user", "content": "<user_message>"}
  ],
  "max_tokens": <max_tokens>
}
```

**Requirement 3.8:** System prompt handling: If `Request.System` is non-empty, prepend a message with `role: "system"` and `content: <system_prompt>` to the messages array, before the user message. If `Request.System` is empty, do not include a system message.

**Requirement 3.9:** The `temperature` field must be omitted from the JSON body when `Request.Temperature` is `0`.

**Requirement 3.10:** When `Request.MaxTokens` is `0`, omit the `max_tokens` field from the JSON body entirely. The OpenAI API does not require `max_tokens` and will use the model's default maximum. This differs from the Anthropic provider which defaults to `4096`.

**Requirement 3.11:** Response parsing: Extract the following from the OpenAI JSON response:

| OpenAI Response Field | Maps To |
|-----------------------|---------|
| `choices[0].message.content` | `Response.Content` |
| `model` | `Response.Model` |
| `usage.prompt_tokens` | `Response.InputTokens` |
| `usage.completion_tokens` | `Response.OutputTokens` |
| `choices[0].finish_reason` | `Response.StopReason` |

**Requirement 3.12:** If the OpenAI response `choices` array is empty, return a `ProviderError` with category `ErrCategoryServer` and message: `response contains no choices`.

**Requirement 3.13:** HTTP error mapping:

| HTTP Status | Error Category | Description |
|-------------|---------------|-------------|
| 401 | `ErrCategoryAuth` | Invalid API key |
| 400 | `ErrCategoryBadRequest` | Invalid request |
| 403 | `ErrCategoryAuth` | Insufficient permissions or billing issue |
| 404 | `ErrCategoryBadRequest` | Model not found |
| 429 | `ErrCategoryRateLimit` | Rate limited |
| 500, 502, 503 | `ErrCategoryServer` | Server error |
| Context deadline exceeded | `ErrCategoryTimeout` | Request timed out |

**Requirement 3.14:** For non-2xx responses, the provider must attempt to parse the OpenAI error response body:

```json
{
  "error": {
    "message": "<human_readable_message>",
    "type": "<error_type>",
    "code": "<error_code>"
  }
}
```

If the body parses successfully, use the `error.message` field in the `ProviderError.Message`. If parsing fails, use the raw HTTP status text.

**Requirement 3.15:** The `Send` method must respect the `context.Context` passed to it. If the context is cancelled or its deadline is exceeded, the HTTP request must be aborted and a `ProviderError` with category `ErrCategoryTimeout` must be returned.

**Requirement 3.16:** The OpenAI provider must not retry failed requests.

**Requirement 3.17:** The HTTP client must not follow redirects. Use a custom `http.Client` with `CheckRedirect` set to return `http.ErrUseLastResponse`.

**Requirement 3.18:** The default base URL must be a named constant, not a magic string.

### 3.4 Ollama Provider (`internal/provider/`)

**Requirement 4.1:** Implement an `Ollama` struct that satisfies the `Provider` interface.

**Requirement 4.2:** Constructor function signature:

```go
func NewOllama(opts ...OllamaOption) (*Ollama, error)
```

The constructor does not accept an API key parameter. Ollama is a local service that does not require authentication. The constructor must always succeed (return a valid `*Ollama` and nil error) given valid options.

**Requirement 4.3:** Define the `Ollama` struct:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `baseURL` | `string` | `"http://localhost:11434"` | Ollama API base URL |
| `client` | `*http.Client` | Default client with no redirects | HTTP client |

**Requirement 4.4:** Functional option for base URL override:

```go
type OllamaOption func(*Ollama)
func WithOllamaBaseURL(url string) OllamaOption
```

**Requirement 4.5:** The `Send` method must make an HTTP POST request to `<baseURL>/api/chat`.

**Requirement 4.6:** Required HTTP headers:

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |

No authentication headers are sent.

**Requirement 4.7:** Request JSON body must conform to the Ollama Chat API format:

```json
{
  "model": "<model>",
  "messages": [
    {"role": "system", "content": "<system_prompt>"},
    {"role": "user", "content": "<user_message>"}
  ],
  "stream": false,
  "options": {
    "temperature": <temperature>,
    "num_predict": <max_tokens>
  }
}
```

**Requirement 4.8:** The `stream` field must always be set to `false`. Streaming is out of scope.

**Requirement 4.9:** System prompt handling: If `Request.System` is non-empty, prepend a message with `role: "system"` and `content: <system_prompt>` to the messages array. If `Request.System` is empty, do not include a system message.

**Requirement 4.10:** The `options.temperature` field must be omitted from the `options` object when `Request.Temperature` is `0`.

**Requirement 4.11:** The `options.num_predict` field must be omitted from the `options` object when `Request.MaxTokens` is `0`.

**Requirement 4.12:** If both `temperature` and `num_predict` would be omitted, omit the entire `options` object from the request body.

**Requirement 4.13:** Response parsing: Extract the following from the Ollama JSON response:

| Ollama Response Field | Maps To |
|-----------------------|---------|
| `message.content` | `Response.Content` |
| `model` | `Response.Model` |
| `prompt_eval_count` | `Response.InputTokens` |
| `eval_count` | `Response.OutputTokens` |
| `done_reason` | `Response.StopReason` |

**Requirement 4.14:** If the Ollama response `message` field is missing or has empty `content`, return a `ProviderError` with category `ErrCategoryServer` and message: `response contains no content`.

**Requirement 4.15:** HTTP error mapping:

| HTTP Status | Error Category | Description |
|-------------|---------------|-------------|
| 400 | `ErrCategoryBadRequest` | Invalid request (bad model name, etc.) |
| 404 | `ErrCategoryBadRequest` | Model not found / not pulled |
| 500 | `ErrCategoryServer` | Server error |
| Context deadline exceeded | `ErrCategoryTimeout` | Request timed out |

**Requirement 4.16:** Connection refused handling: If the HTTP client returns a connection refused error (the Ollama server is not running), return a `ProviderError` with category `ErrCategoryServer` and message: `connection refused: is Ollama running? (expected at <baseURL>)`.

**Requirement 4.17:** For non-2xx responses, attempt to parse the Ollama error response body as JSON (`{"error": "<message>"}`). If parsing succeeds, use the `error` field value in `ProviderError.Message`. If parsing fails, use the raw HTTP status text.

**Requirement 4.18:** The `Send` method must respect the `context.Context` passed to it. If the context is cancelled or its deadline is exceeded, the HTTP request must be aborted and a `ProviderError` with category `ErrCategoryTimeout` must be returned.

**Requirement 4.19:** The Ollama provider must not retry failed requests.

**Requirement 4.20:** The HTTP client must not follow redirects. Use a custom `http.Client` with `CheckRedirect` set to return `http.ErrUseLastResponse`.

**Requirement 4.21:** The default base URL must be a named constant, not a magic string.

### 3.5 Refactor `cmd/run.go`

**Requirement 5.1:** Remove the hardcoded provider check:

```go
// REMOVE this block:
if provName != "anthropic" {
    return &ExitError{Code: 1, Err: fmt.Errorf(
        "unsupported provider %q: only \"anthropic\" is supported in this version", provName)}
}
```

**Requirement 5.2:** Remove the hardcoded `ANTHROPIC_API_KEY` environment variable lookup. Replace it with `GlobalConfig.ResolveAPIKey(providerName)`.

**Requirement 5.3:** Remove the hardcoded `AXE_ANTHROPIC_BASE_URL` environment variable lookup. Replace it with `GlobalConfig.ResolveBaseURL(providerName)`.

**Requirement 5.4:** Remove the hardcoded `provider.NewAnthropic(apiKey, opts...)` construction. Replace it with `provider.New(providerName, apiKey, baseURL)`.

**Requirement 5.5:** The new provider construction flow in `runAgent` must be:

1. Load global config via `config.Load()`
2. Resolve API key via `globalCfg.ResolveAPIKey(providerName)`
3. If the provider requires an API key (not `"ollama"`) and the resolved key is empty, return an `ExitError` with code 3 and message: `API key for provider "<name>" is not configured (set <ENV_VAR> or add to config.toml)`
4. Resolve base URL via `globalCfg.ResolveBaseURL(providerName)`
5. Create provider via `provider.New(providerName, apiKey, baseURL)`
6. If the factory returns an error (unsupported provider, etc.), return an `ExitError` with code 1

**Requirement 5.6:** The Ollama provider must not require an API key. When `providerName` is `"ollama"`, skip the API key check entirely (step 3 above).

**Requirement 5.7:** If `config.Load()` returns an error (malformed `config.toml`), return an `ExitError` with code 2.

**Requirement 5.8:** The `parseModel` function must remain unchanged. It correctly splits `"provider/model-name"` into provider and model parts.

**Requirement 5.9:** All existing flags (`--skill`, `--workdir`, `--model`, `--timeout`, `--dry-run`, `--verbose`, `--json`) must continue to work identically.

**Requirement 5.10:** All existing exit codes must remain unchanged:
- Exit 0: success (including `--dry-run`)
- Exit 1: agent error (invalid model format, unsupported provider, bad request)
- Exit 2: config error (agent not found, invalid TOML, malformed global config)
- Exit 3: API error (auth, rate limit, timeout, server, overloaded, missing API key)

### 3.6 Update `axe config init`

**Requirement 6.1:** The `axe config init` command must scaffold a `config.toml` file at `$XDG_CONFIG_HOME/axe/config.toml` if it does not already exist.

**Requirement 6.2:** The scaffolded `config.toml` must contain only comments (no actual values) to avoid accidental key exposure:

```toml
# Axe global configuration
# API keys and base URL overrides for LLM providers.
# Environment variables take precedence over values set here.
#
# Env var convention:
#   API key:  <PROVIDER_UPPER>_API_KEY  (e.g. ANTHROPIC_API_KEY)
#   Base URL: AXE_<PROVIDER_UPPER>_BASE_URL  (e.g. AXE_ANTHROPIC_BASE_URL)

# [providers.anthropic]
# api_key = ""
# base_url = ""

# [providers.openai]
# api_key = ""
# base_url = ""

# [providers.ollama]
# base_url = "http://localhost:11434"
```

**Requirement 6.3:** If `config.toml` already exists, do not overwrite it. This is idempotent — running `config init` multiple times must not alter an existing `config.toml`.

**Requirement 6.4:** The `config.toml` file must be created with permissions `0600` (owner read/write only) because it may contain API keys. This differs from agent TOML files which use `0644`.

---

## 4. Project Structure

After M4 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── run.go                    # MODIFIED: use provider factory + global config
│   ├── run_test.go               # MODIFIED: update provider tests, add OpenAI/Ollama paths
│   ├── config.go                 # MODIFIED: scaffold config.toml in config init
│   ├── config_test.go            # MODIFIED: test config.toml scaffolding
│   ├── agents.go                 # UNCHANGED
│   ├── agents_test.go            # UNCHANGED
│   ├── exit.go                   # UNCHANGED
│   ├── exit_test.go              # UNCHANGED
│   ├── root.go                   # UNCHANGED
│   ├── root_test.go              # UNCHANGED
│   ├── version.go                # UNCHANGED
│   └── version_test.go           # UNCHANGED
├── internal/
│   ├── config/
│   │   ├── config.go             # NEW: GlobalConfig, Load, ResolveAPIKey, ResolveBaseURL
│   │   └── config_test.go        # NEW: tests for global config
│   ├── provider/
│   │   ├── provider.go           # UNCHANGED
│   │   ├── provider_test.go      # UNCHANGED
│   │   ├── anthropic.go          # UNCHANGED
│   │   ├── anthropic_test.go     # UNCHANGED
│   │   ├── registry.go           # NEW: provider factory function
│   │   ├── registry_test.go      # NEW: factory tests
│   │   ├── openai.go             # NEW: OpenAI provider implementation
│   │   ├── openai_test.go        # NEW: OpenAI tests
│   │   ├── ollama.go             # NEW: Ollama provider implementation
│   │   └── ollama_test.go        # NEW: Ollama tests
│   ├── agent/                    # UNCHANGED
│   │   ├── agent.go
│   │   └── agent_test.go
│   ├── resolve/                  # UNCHANGED
│   │   ├── resolve.go
│   │   └── resolve_test.go
│   └── xdg/                     # UNCHANGED
│       ├── xdg.go
│       └── xdg_test.go
├── go.mod                        # UNCHANGED (no new dependencies)
├── go.sum                        # UNCHANGED
└── ...
```

---

## 5. Edge Cases

### 5.1 Global Config

| Scenario | Behavior |
|----------|----------|
| `config.toml` does not exist | `config.Load()` returns empty config, no error |
| `config.toml` is empty (0 bytes) | Returns empty config, no error |
| `config.toml` has unknown keys | Ignored silently (TOML decoder does not fail on unknown keys) |
| `config.toml` is not valid TOML | Returns error with exit code 2 |
| `config.toml` has no `[providers]` section | Returns config with empty `Providers` map |
| Provider name not in config file | `ResolveAPIKey`/`ResolveBaseURL` fall back to env var, then empty string |
| Env var set to empty string | Treated as unset; falls through to config file |

### 5.2 Provider Factory

| Scenario | Behavior |
|----------|----------|
| Unknown provider name | Error: `unsupported provider "<name>": supported providers are anthropic, openai, ollama` |
| Empty provider name | Error: `unsupported provider "": supported providers are anthropic, openai, ollama` |
| Provider name with mixed case | Error (case-sensitive matching) |
| Empty API key for Anthropic/OpenAI | Error propagated from provider constructor: `API key is required` |
| Empty API key for Ollama | Ignored; `apiKey` parameter not passed to `NewOllama` |
| Empty base URL | Provider uses its built-in default |
| Non-empty base URL | Passed as functional option to provider constructor |

### 5.3 OpenAI Provider

| Scenario | Behavior |
|----------|----------|
| Model name contains `/` (e.g. `ft:gpt-4o:org:custom:id`) | Sent as-is to the API; `parseModel` already splits on first `/` only |
| Response has empty `choices` array | `ProviderError` with `ErrCategoryServer` |
| Response has `choices` with `null` message | `ProviderError` with `ErrCategoryServer` |
| HTTP 403 (billing/permissions) | `ProviderError` with `ErrCategoryAuth` |
| HTTP 404 (model not found) | `ProviderError` with `ErrCategoryBadRequest` |
| Unparseable error body | Falls back to HTTP status text |

### 5.4 Ollama Provider

| Scenario | Behavior |
|----------|----------|
| Ollama server not running | `ProviderError` with `ErrCategoryServer`, message includes `connection refused: is Ollama running?` |
| Model not pulled | HTTP 404 -> `ProviderError` with `ErrCategoryBadRequest` |
| Token counts not returned (zero values) | `Response.InputTokens` and `Response.OutputTokens` are `0` (acceptable; Ollama may not return counts for all models) |
| `done_reason` field missing | `Response.StopReason` is empty string |
| Large response (no streaming) | Entire response buffered in memory; timeout is the user's defense |
| Custom base URL (remote Ollama) | Fully supported via `base_url` config or `AXE_OLLAMA_BASE_URL` |

### 5.5 API Key Resolution

| Scenario | Behavior |
|----------|----------|
| Env var set, config file has value | Env var wins |
| Env var not set, config file has value | Config file value used |
| Neither env var nor config file | Empty string returned; caller decides if this is an error |
| Provider is `ollama` and no API key | No error; Ollama does not require API key |
| Provider is `openai` and no API key from any source | `ExitError` code 3: `API key for provider "openai" is not configured (set OPENAI_API_KEY or add to config.toml)` |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must still contain only `spf13/cobra` and `BurntSushi/toml` as direct dependencies.

**Constraint 2:** No streaming. All providers receive the complete response before returning.

**Constraint 3:** No retry logic. Failed requests fail immediately for all providers.

**Constraint 4:** No caching of responses, configs, or provider instances.

**Constraint 5:** All user-facing output must go through `cmd.OutOrStdout()` (stdout) or `cmd.ErrOrStderr()` (stderr) for testability.

**Constraint 6:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows.

**Constraint 7:** The Anthropic provider implementation (`internal/provider/anthropic.go`) must not be modified. It already satisfies the `Provider` interface and works correctly. The only changes needed are in `cmd/run.go` (caller), the new factory, and the new providers.

**Constraint 8:** The `Provider` interface must not be modified. It remains:

```go
type Provider interface {
    Send(ctx context.Context, req *Request) (*Response, error)
}
```

**Constraint 9:** The `Request`, `Response`, `Message`, `ProviderError`, and `ErrorCategory` types must not be modified. New providers must map their API-specific formats to/from these existing types.

**Constraint 10:** The HTTP client for all providers must not follow redirects. Use a custom `http.Client` with `CheckRedirect` set to return `http.ErrUseLastResponse`.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M1-M3:

- **Package-level tests:** Tests live in the same package (e.g. `package provider`, `package config`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv()` for environment variable control.
- **HTTP tests:** Use `httptest.NewServer` for all HTTP interactions. No real API calls.
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real I/O. HTTP tests must use `httptest.NewServer` to simulate real APIs. Tests that mock the `Provider` interface to verify "mock returns X, response is X" are not acceptable. Each test must fail if the code under test is deleted.
- **Run tests with:** `make test`

### 7.2 `internal/config/` Tests

**Test: `TestLoad_FileNotFound`** — Point XDG to a temp dir with no `config.toml`. Verify returned `GlobalConfig` has empty `Providers` map and error is nil.

**Test: `TestLoad_EmptyFile`** — Create an empty `config.toml`. Verify returns valid empty config, no error.

**Test: `TestLoad_ValidConfig`** — Write a full `config.toml` with providers section. Verify all fields are populated correctly.

**Test: `TestLoad_MalformedTOML`** — Write invalid TOML content. Verify error message matches `failed to parse config file`.

**Test: `TestLoad_PartialConfig`** — Write config with only one provider section. Verify that provider is loaded, others are missing from map.

**Test: `TestResolveAPIKey_EnvVarTakesPrecedence`** — Set `ANTHROPIC_API_KEY` env var AND config file value. Verify env var value is returned.

**Test: `TestResolveAPIKey_FallsBackToConfig`** — Unset env var, set config file value. Verify config value is returned.

**Test: `TestResolveAPIKey_NeitherSet`** — No env var, no config value. Verify empty string returned.

**Test: `TestResolveAPIKey_EmptyEnvVar`** — Set env var to empty string, set config value. Verify config value is returned (empty env var treated as unset).

**Test: `TestResolveAPIKey_NilProvidersMap`** — Call on a `GlobalConfig` with nil `Providers` map. Verify no panic, falls back to env var.

**Test: `TestResolveAPIKey_UnknownProvider`** — Call with provider name `"groq"`. Verify it checks `GROQ_API_KEY` env var and returns empty if not set.

**Test: `TestResolveBaseURL_EnvVarTakesPrecedence`** — Set `AXE_OPENAI_BASE_URL` env var AND config value. Verify env var wins.

**Test: `TestResolveBaseURL_FallsBackToConfig`** — Unset env var, set config value. Verify config value returned.

**Test: `TestResolveBaseURL_NeitherSet`** — Neither source has a value. Verify empty string returned.

### 7.3 `internal/provider/registry.go` Tests

**Test: `TestNew_Anthropic`** — Call `New("anthropic", "test-key", "")`. Verify returned provider is non-nil, no error.

**Test: `TestNew_OpenAI`** — Call `New("openai", "test-key", "")`. Verify returned provider is non-nil, no error.

**Test: `TestNew_Ollama`** — Call `New("ollama", "", "")`. Verify returned provider is non-nil, no error.

**Test: `TestNew_OllamaIgnoresAPIKey`** — Call `New("ollama", "ignored-key", "")`. Verify no error (key is silently ignored).

**Test: `TestNew_WithBaseURL`** — Call `New("openai", "test-key", "http://custom:8080")`. Verify no error. (Base URL correctness is verified in the respective provider tests.)

**Test: `TestNew_UnsupportedProvider`** — Call `New("groq", "key", "")`. Verify error message contains `unsupported provider "groq"` and lists supported providers.

**Test: `TestNew_EmptyProviderName`** — Call `New("", "key", "")`. Verify error message.

**Test: `TestNew_CaseSensitive`** — Call `New("Anthropic", "key", "")`. Verify error (not case-insensitive).

**Test: `TestNew_MissingAPIKeyAnthropic`** — Call `New("anthropic", "", "")`. Verify error about missing API key.

**Test: `TestNew_MissingAPIKeyOpenAI`** — Call `New("openai", "", "")`. Verify error about missing API key.

### 7.4 `internal/provider/openai.go` Tests

**Test: `TestNewOpenAI_EmptyAPIKey`** — Empty string returns error.

**Test: `TestNewOpenAI_ValidAPIKey`** — Non-empty string returns `*OpenAI` with no error.

**Test: `TestOpenAI_Send_Success`** — Use `httptest.NewServer` returning a valid OpenAI chat completion response. Verify `Response` fields are correctly populated (content, model, tokens, stop reason).

**Test: `TestOpenAI_Send_RequestFormat`** — Use `httptest.NewServer` that inspects the request. Verify:
- Method is POST
- URL path is `/v1/chat/completions`
- `Authorization` header is `Bearer <key>`
- `Content-Type` header is `application/json`
- Body JSON contains correct `model`, `messages` array (with system message first, then user message)

**Test: `TestOpenAI_Send_OmitsEmptySystem`** — System field empty -> no system message in the messages array. Only user message present.

**Test: `TestOpenAI_Send_OmitsZeroTemperature`** — Temperature is 0 -> `temperature` key absent from JSON body.

**Test: `TestOpenAI_Send_OmitsZeroMaxTokens`** — MaxTokens is 0 -> `max_tokens` key absent from JSON body.

**Test: `TestOpenAI_Send_IncludesMaxTokens`** — MaxTokens is non-zero -> `max_tokens` key present in JSON body with correct value.

**Test: `TestOpenAI_Send_AuthError`** — Server returns 401. Verify `ProviderError` with `ErrCategoryAuth`.

**Test: `TestOpenAI_Send_ForbiddenError`** — Server returns 403. Verify `ProviderError` with `ErrCategoryAuth`.

**Test: `TestOpenAI_Send_NotFoundError`** — Server returns 404. Verify `ProviderError` with `ErrCategoryBadRequest`.

**Test: `TestOpenAI_Send_RateLimitError`** — Server returns 429. Verify `ProviderError` with `ErrCategoryRateLimit`.

**Test: `TestOpenAI_Send_ServerError`** — Server returns 500. Verify `ProviderError` with `ErrCategoryServer`.

**Test: `TestOpenAI_Send_Timeout`** — Server delays longer than context deadline. Verify `ProviderError` with `ErrCategoryTimeout`.

**Test: `TestOpenAI_Send_EmptyChoices`** — Server returns valid response with empty `choices` array. Verify `ProviderError` with `ErrCategoryServer`, message contains `no choices`.

**Test: `TestOpenAI_Send_ErrorResponseParsing`** — Server returns 400 with OpenAI error JSON. Verify `ProviderError.Message` contains the error message from the JSON body.

**Test: `TestOpenAI_Send_UnparseableErrorBody`** — Server returns 400 with non-JSON body. Verify `ProviderError.Message` falls back to HTTP status text.

### 7.5 `internal/provider/ollama.go` Tests

**Test: `TestNewOllama_Defaults`** — Call `NewOllama()` with no options. Verify no error.

**Test: `TestNewOllama_WithBaseURL`** — Call with `WithOllamaBaseURL`. Verify no error.

**Test: `TestOllama_Send_Success`** — Use `httptest.NewServer` returning a valid Ollama chat response. Verify `Response` fields are correctly populated.

**Test: `TestOllama_Send_RequestFormat`** — Use `httptest.NewServer` that inspects the request. Verify:
- Method is POST
- URL path is `/api/chat`
- `Content-Type` header is `application/json`
- No `Authorization` header present
- Body JSON contains `model`, `messages`, `stream: false`

**Test: `TestOllama_Send_SystemMessage`** — Verify system prompt is prepended as a system message in the messages array.

**Test: `TestOllama_Send_OmitsEmptySystem`** — System field empty -> no system message in the messages array.

**Test: `TestOllama_Send_OmitsZeroTemperature`** — Temperature is 0 -> `temperature` absent from `options` object.

**Test: `TestOllama_Send_OmitsZeroMaxTokens`** — MaxTokens is 0 -> `num_predict` absent from `options` object.

**Test: `TestOllama_Send_OmitsOptionsWhenEmpty`** — Both temperature and max_tokens are 0 -> entire `options` object absent from body.

**Test: `TestOllama_Send_IncludesOptions`** — Both temperature and max_tokens are non-zero -> `options` object present with both fields.

**Test: `TestOllama_Send_BadRequest`** — Server returns 400. Verify `ProviderError` with `ErrCategoryBadRequest`.

**Test: `TestOllama_Send_NotFound`** — Server returns 404. Verify `ProviderError` with `ErrCategoryBadRequest` (model not pulled).

**Test: `TestOllama_Send_ServerError`** — Server returns 500. Verify `ProviderError` with `ErrCategoryServer`.

**Test: `TestOllama_Send_Timeout`** — Server delays longer than context deadline. Verify `ProviderError` with `ErrCategoryTimeout`.

**Test: `TestOllama_Send_ConnectionRefused`** — Use a base URL pointing to a port with nothing listening (e.g. `http://localhost:1`). Verify `ProviderError` with `ErrCategoryServer` and message containing `connection refused`.

**Test: `TestOllama_Send_EmptyContent`** — Server returns response with empty `message.content`. Verify `ProviderError` with `ErrCategoryServer`.

**Test: `TestOllama_Send_ErrorResponseParsing`** — Server returns 400 with Ollama error JSON (`{"error": "..."}`). Verify `ProviderError.Message` contains the parsed error message.

**Test: `TestOllama_Send_ZeroTokenCounts`** — Server returns response with `prompt_eval_count` and `eval_count` as 0 or missing. Verify `Response.InputTokens` and `Response.OutputTokens` are 0 (no error).

### 7.6 `cmd/run_test.go` Updates

**Test: `TestRun_UnsupportedProvider` (MODIFIED)** — Change agent model to `"fakeprovider/some-model"`. Verify error message: `unsupported provider "fakeprovider": supported providers are anthropic, openai, ollama`. The current test uses `"openai/gpt-4"` which will now be a valid provider.

**Test: `TestRun_OpenAIProviderSuccess` (NEW)** — Create agent with `model = "openai/gpt-4o"`, set `OPENAI_API_KEY`, start `httptest.NewServer` as mock OpenAI API, set `AXE_OPENAI_BASE_URL` to test server. Verify response content is printed to stdout.

**Test: `TestRun_OllamaProviderSuccess` (NEW)** — Create agent with `model = "ollama/llama3"`, start `httptest.NewServer` as mock Ollama API, set `AXE_OLLAMA_BASE_URL` to test server. Verify response content is printed to stdout. No API key env var should be set.

**Test: `TestRun_MissingAPIKeyOpenAI` (NEW)** — Create agent with `model = "openai/gpt-4o"`, unset `OPENAI_API_KEY`, no config file. Verify error message about missing API key and exit code 3.

**Test: `TestRun_OllamaNoAPIKeyRequired` (NEW)** — Create agent with `model = "ollama/llama3"`, no `OLLAMA_API_KEY` set. Start mock server. Verify success (no API key error).

**Test: `TestRun_APIKeyFromConfigFile` (NEW)** — Create agent with `model = "anthropic/claude-sonnet-4-20250514"`, unset `ANTHROPIC_API_KEY`, write `config.toml` with `[providers.anthropic] api_key = "from-config"`. Set `AXE_ANTHROPIC_BASE_URL` to test server. Verify the request uses the config file API key (inspect request headers in the httptest server).

**Test: `TestRun_MalformedGlobalConfig` (NEW)** — Create agent, write invalid TOML to `config.toml`. Verify error and exit code 2.

**Test: `TestRun_DryRun_NonAnthropicProvider` (NEW)** — Create agent with `model = "openai/gpt-4o"`, run with `--dry-run`. Verify output shows `Model: openai/gpt-4o` and exits successfully without needing an API key.

### 7.7 `cmd/config_test.go` Updates

**Test: `TestConfigInit_CreatesConfigTOML` (NEW)** — Run `axe config init`, verify `config.toml` is created at the correct path.

**Test: `TestConfigInit_ConfigTOMLPermissions` (NEW)** — Verify the created `config.toml` has `0600` permissions.

**Test: `TestConfigInit_ConfigTOMLContent` (NEW)** — Verify the scaffolded `config.toml` contains commented-out provider sections.

**Test: `TestConfigInit_DoesNotOverwriteConfigTOML` (NEW)** — Create a custom `config.toml`, run `config init`, verify the file is not overwritten.

### 7.8 Running Tests

All tests must pass when run with:

```bash
make test
```

No test may make real HTTP requests to external APIs. All HTTP interactions must use `httptest.NewServer`.

---

## 8. Exit Codes

The full exit code table after M4 (unchanged from M3):

| Code | Meaning | Used By |
|------|---------|---------|
| 0 | Success | All commands on success, `--dry-run` |
| 1 | Agent/general error | Invalid model format, unsupported provider, bad request, `edit` with no `$EDITOR` |
| 2 | Config error | Agent not found, invalid TOML, missing required fields, invalid skill path, invalid glob, malformed `config.toml` |
| 3 | API error | Auth failure, rate limit, timeout, server error, overloaded, missing API key |

---

## 9. Acceptance Criteria

| Criterion | Test |
|-----------|------|
| Provider Factory | `provider.New("anthropic", key, "")` returns a working Anthropic provider |
| Provider Factory | `provider.New("openai", key, "")` returns a working OpenAI provider |
| Provider Factory | `provider.New("ollama", "", "")` returns a working Ollama provider |
| Provider Factory | `provider.New("unknown", key, "")` returns descriptive error listing supported providers |
| OpenAI Provider | `OpenAI.Send` makes correct HTTP requests to `/v1/chat/completions` |
| OpenAI Auth | `Authorization: Bearer <key>` header is sent |
| OpenAI System | System prompt sent as system role message, omitted when empty |
| OpenAI Errors | HTTP 401->auth, 403->auth, 404->bad_request, 429->rate_limit, 500->server |
| OpenAI Timeout | Context cancellation aborts request and returns timeout error |
| Ollama Provider | `Ollama.Send` makes correct HTTP requests to `/api/chat` |
| Ollama No Auth | No authentication headers sent |
| Ollama System | System prompt sent as system role message, omitted when empty |
| Ollama Options | Temperature and max_tokens mapped to `options` object, omitted when zero |
| Ollama Stream | `stream: false` always sent |
| Ollama Conn Refused | Connection refused returns clear error about Ollama not running |
| Ollama Errors | HTTP 400->bad_request, 404->bad_request, 500->server |
| Global Config Load | Missing file returns empty config, no error |
| Global Config Load | Valid TOML returns populated config |
| Global Config Load | Malformed TOML returns error |
| API Key Resolution | Env var takes precedence over config file |
| API Key Resolution | Config file used when env var unset |
| API Key Resolution | Empty string when neither source has value |
| Base URL Resolution | Env var takes precedence over config file |
| `axe run` Anthropic | Still works identically to M3 |
| `axe run` OpenAI | Works with `model = "openai/gpt-4o"` |
| `axe run` Ollama | Works with `model = "ollama/llama3"` without API key |
| `axe run` Unknown | Returns error with supported provider list |
| `axe config init` | Scaffolds `config.toml` with commented-out examples |
| `axe config init` | `config.toml` has `0600` permissions |
| `axe config init` | Idempotent — does not overwrite existing `config.toml` |
| No New Deps | `go.mod` contains only `spf13/cobra` and `BurntSushi/toml` as direct deps |
| All Tests Pass | `make test` passes with 0 failures |

---

## 10. Out of Scope

The following items are explicitly **not** included in M4:

1. Sub-agent invocation or `call_agent` tool injection (M5)
2. Memory read/write operations (M6)
3. Garbage collection (M7)
4. Streaming output (Future)
5. Retry logic or exponential backoff
6. Response caching
7. Token cost estimation or budget tracking
8. Multi-turn conversations (one user message per run)
9. Image or non-text content in messages
10. Tool use / function calling in the LLM request
11. Interactive prompts or TUI elements
12. Rate limit queuing or backpressure
13. Provider-specific model validation (e.g. checking if a model name exists)
14. Provider auto-detection from model name without the `provider/` prefix
15. Encrypted or OS keychain storage for API keys
16. Multiple API keys per provider (e.g. org-level vs personal)
17. Provider health checks or connectivity tests
18. `axe providers list` or similar provider discovery commands
19. OpenAI-compatible third-party providers (e.g. Together, Groq) as named providers — users can point OpenAI provider at these via `base_url` override

---

## 11. References

- Milestone Definition: `docs/plans/000_milestones.md` (M4 section)
- M3 Spec: `docs/plans/003_single_agent_run_spec.md`
- M2 Spec: `docs/plans/002_agent_config_spec.md`
- Agent Config Schema: `docs/design/agent-config-schema.md`
- CLI Structure: `docs/design/cli-structure.md`
- OpenAI Chat Completions API: https://platform.openai.com/docs/api-reference/chat/create
- Ollama API: https://github.com/ollama/ollama/blob/main/docs/api.md
- Anthropic Messages API: https://docs.anthropic.com/en/api/messages
- models.dev: https://models.dev

---

## 12. Notes

- The provider factory uses a simple switch statement, not a plugin registry. Three providers is a small, fixed set. A registration pattern would be over-engineering for this milestone. If a future milestone adds many more providers, the factory can be refactored into a registry.
- The OpenAI provider can be used with OpenAI-compatible third-party services (Together, Groq, Fireworks, etc.) by setting a custom `base_url`. This is intentional and documented in the config.toml comments but not explicitly supported as named providers.
- The Ollama provider uses the native `/api/chat` endpoint, not the OpenAI-compatible `/v1/chat/completions` endpoint that Ollama also exposes. The native endpoint is more stable and returns Ollama-specific fields (like `done_reason` and token counts via `prompt_eval_count`/`eval_count`).
- The `config.toml` uses `0600` permissions because it may contain API keys. This is more restrictive than agent TOML files (`0644`) which contain no secrets.
- The `ResolveAPIKey` method treats an env var set to empty string as "not set" and falls through to the config file. This handles the case where a user has `export ANTHROPIC_API_KEY=` in their shell profile without a value.
- The error message for missing API keys includes both the env var name and a hint to use `config.toml`. This helps users discover the config file feature.
- Connection refused detection for Ollama is important UX. "Connection refused" is a common error when users forget to start `ollama serve`, and the error message should guide them directly.
- The Anthropic provider code is not modified in M4. The only Anthropic-related changes are in `cmd/run.go` (using the factory instead of direct construction) and in test updates. This minimizes risk to a working provider.
