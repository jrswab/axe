# Specification: M3 - Single Agent Run

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-27
**Scope:** Load agent config, resolve context, call LLM, print result

---

## 1. Purpose

Implement the core execution loop for Axe: the `axe run <agent>` command. This milestone takes an agent TOML config (from M2), resolves all runtime context (working directory, file globs, SKILL.md, stdin), builds a prompt, calls the Anthropic Messages API, and prints the response. CLI flags provide runtime overrides and output control. This is the first milestone where Axe produces useful output from an LLM.

---

## 2. Requirements

### 2.1 New Packages

**Requirement 1.1:** Create package `internal/resolve/` for all context resolution logic (working directory, file globs, skill loading, stdin reading, prompt assembly).

**Requirement 1.2:** Create package `internal/provider/` for LLM provider types, interface, and the Anthropic implementation.

**Requirement 1.3:** No new external dependencies. All HTTP and JSON handling must use Go standard library (`net/http`, `encoding/json`, `io`, `context`).

### 2.2 Provider Types (`internal/provider/`)

**Requirement 2.1:** Define a `Message` struct representing a single message in the conversation:

| Go Field | JSON Key | Go Type | Description |
|----------|----------|---------|-------------|
| `Role` | `role` | `string` | Message role: `"user"` or `"assistant"` |
| `Content` | `content` | `string` | Message text content |

**Requirement 2.2:** Define a `Request` struct representing an LLM completion request:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Model` | `string` | Model identifier (provider-specific, e.g. `claude-sonnet-4-20250514`) |
| `System` | `string` | System prompt text |
| `Messages` | `[]Message` | Conversation messages (at least one user message required) |
| `Temperature` | `float64` | Sampling temperature (0 means omit from request, use provider default) |
| `MaxTokens` | `int` | Maximum output tokens (0 means use a sensible default; see Requirement 3.8) |

**Requirement 2.3:** Define a `Response` struct representing an LLM completion response:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Content` | `string` | The text content of the LLM response |
| `Model` | `string` | The model that actually processed the request (as returned by the API) |
| `InputTokens` | `int` | Number of input tokens consumed |
| `OutputTokens` | `int` | Number of output tokens generated |
| `StopReason` | `string` | Why generation stopped (e.g. `"end_turn"`, `"max_tokens"`, `"stop_sequence"`) |

**Requirement 2.4:** Define a `Provider` interface:

```go
type Provider interface {
    Send(ctx context.Context, req *Request) (*Response, error)
}
```

This interface has a single method. It accepts a `context.Context` for timeout/cancellation and a `*Request`. It returns a `*Response` on success or an error on failure.

**Requirement 2.5:** Define a `ProviderError` struct that wraps provider-specific errors with categorization:

```go
type ProviderError struct {
    Category ErrorCategory
    Status   int
    Message  string
    Err      error
}
```

Where `ErrorCategory` is a string type with the following constants:

| Constant | Value | Description | Maps to Exit Code |
|----------|-------|-------------|-------------------|
| `ErrCategoryAuth` | `"auth"` | Authentication failure (missing/invalid API key) | 3 |
| `ErrCategoryRateLimit` | `"rate_limit"` | Rate limited by provider | 3 |
| `ErrCategoryTimeout` | `"timeout"` | Request timed out | 3 |
| `ErrCategoryOverloaded` | `"overloaded"` | Provider is overloaded (529) | 3 |
| `ErrCategoryBadRequest` | `"bad_request"` | Malformed request (invalid model, etc.) | 1 |
| `ErrCategoryServer` | `"server"` | Provider server error (5xx) | 3 |

**Requirement 2.6:** `ProviderError` must implement the `error` interface. The `Error()` method must return a message in the format: `<category>: <message>`.

**Requirement 2.7:** `ProviderError` must implement `Unwrap() error` to support `errors.Is` and `errors.As` chaining.

### 2.3 Anthropic Provider (`internal/provider/`)

**Requirement 3.1:** Implement an `Anthropic` struct that satisfies the `Provider` interface.

**Requirement 3.2:** Constructor function signature:

```go
func NewAnthropic(apiKey string) (*Anthropic, error)
```

The constructor must return an error if `apiKey` is an empty string.

**Requirement 3.3:** The `Send` method must make an HTTP POST request to `https://api.anthropic.com/v1/messages`.

**Requirement 3.4:** Required HTTP headers:

| Header | Value |
|--------|-------|
| `x-api-key` | The API key passed to the constructor |
| `anthropic-version` | `2023-06-01` |
| `content-type` | `application/json` |

**Requirement 3.5:** Request JSON body must conform to the Anthropic Messages API format:

```json
{
  "model": "<model>",
  "max_tokens": <max_tokens>,
  "messages": [{"role": "<role>", "content": "<content>"}],
  "system": "<system_prompt>"
}
```

**Requirement 3.6:** The `system` field must be omitted from the JSON body when `Request.System` is an empty string.

**Requirement 3.7:** The `temperature` field must be omitted from the JSON body when `Request.Temperature` is `0`.

**Requirement 3.8:** When `Request.MaxTokens` is `0`, the Anthropic provider must default to `4096`. The Anthropic API requires `max_tokens` to be present and greater than zero. The default value of `4096` must be a named constant, not a magic number.

**Requirement 3.9:** Model name extraction: The `Request.Model` field contains the provider-specific model name (e.g. `claude-sonnet-4-20250514`), not the full `provider/model` format. The caller (the run command) is responsible for stripping the provider prefix before constructing the `Request`. The Anthropic provider must send the model name as-is to the API.

**Requirement 3.10:** Response parsing: Extract the following from the Anthropic JSON response:

| Anthropic Response Field | Maps To |
|--------------------------|---------|
| `content[0].text` | `Response.Content` |
| `model` | `Response.Model` |
| `usage.input_tokens` | `Response.InputTokens` |
| `usage.output_tokens` | `Response.OutputTokens` |
| `stop_reason` | `Response.StopReason` |

**Requirement 3.11:** If the Anthropic response `content` array is empty, return a `ProviderError` with category `ErrCategoryServer` and a descriptive message.

**Requirement 3.12:** HTTP error mapping:

| HTTP Status | Error Category | Description |
|-------------|---------------|-------------|
| 401 | `ErrCategoryAuth` | Invalid API key |
| 400 | `ErrCategoryBadRequest` | Invalid request (bad model name, etc.) |
| 429 | `ErrCategoryRateLimit` | Rate limited |
| 529 | `ErrCategoryOverloaded` | API overloaded |
| 500, 502, 503 | `ErrCategoryServer` | Server error |
| Context deadline exceeded | `ErrCategoryTimeout` | Request timed out |

**Requirement 3.13:** For non-2xx responses, the provider must attempt to parse the Anthropic error response body:

```json
{
  "type": "error",
  "error": {
    "type": "<error_type>",
    "message": "<human_readable_message>"
  }
}
```

If the body parses successfully, use the `error.message` field in the `ProviderError.Message`. If parsing fails, use the raw HTTP status text.

**Requirement 3.14:** The `Send` method must respect the `context.Context` passed to it. If the context is cancelled or its deadline is exceeded, the HTTP request must be aborted and a `ProviderError` with category `ErrCategoryTimeout` must be returned.

**Requirement 3.15:** The Anthropic provider must not retry failed requests. Retry logic is out of scope for M3.

### 2.4 Context Resolution (`internal/resolve/`)

**Requirement 4.1:** Implement a function for working directory resolution:

```go
func Workdir(flagValue, tomlValue string) string
```

Resolution order (first non-empty value wins):
1. `flagValue` (from `--workdir` flag)
2. `tomlValue` (from agent TOML `workdir` field)
3. Current working directory (via `os.Getwd()`)

If all three fail (including `os.Getwd()` returning an error), return `"."`.

**Requirement 4.2:** Define a `FileContent` struct:

| Go Field | Go Type | Description |
|----------|---------|-------------|
| `Path` | `string` | Relative path from workdir (as matched by the glob) |
| `Content` | `string` | File contents as a string |

**Requirement 4.3:** Implement a function for file glob resolution:

```go
func Files(patterns []string, workdir string) ([]FileContent, error)
```

**Requirement 4.4:** The `Files` function must:
1. For each pattern in `patterns`, resolve the glob relative to `workdir`
2. Use `filepath.Glob` for single-level globs (e.g. `*.go`)
3. Use `doublestar`-style matching for `**` patterns — however, since we have no external dependencies, implement recursive directory walking with `filepath.WalkDir` when a pattern contains `**`
4. Read the contents of each matched file
5. Return a slice of `FileContent` with paths relative to `workdir`
6. Skip files that cannot be read (permissions errors, binary files, etc.) — log a warning to stderr if `--verbose` is enabled, but do not fail the entire operation
7. Deduplicate files if multiple patterns match the same file (first occurrence wins)
8. Sort results by path for deterministic output

**Requirement 4.5:** If `patterns` is nil or empty, return an empty slice and no error.

**Requirement 4.6:** If a glob pattern is invalid (malformed syntax), return an error. Do not silently skip invalid patterns.

**Requirement 4.7:** The `Files` function must not follow symlinks outside `workdir`. Matched symlinks within `workdir` that point outside `workdir` must be skipped.

**Requirement 4.8:** Implement a function for skill loading:

```go
func Skill(skillPath, configDir string) (string, error)
```

**Requirement 4.9:** The `Skill` function must:
1. If `skillPath` is empty, return an empty string and no error
2. If `skillPath` is an absolute path, read the file directly
3. If `skillPath` is a relative path, resolve it relative to `configDir` (the XDG config directory)
4. Read the file and return its contents as a string
5. If the file does not exist, return an error: `skill not found: <resolved_path>`
6. If the file cannot be read, return an error: `failed to read skill: <error>`

**Requirement 4.10:** Implement a function for stdin detection and reading:

```go
func Stdin() (string, error)
```

**Requirement 4.11:** The `Stdin` function must:
1. Check if stdin is a pipe (not a terminal) using `os.Stdin.Stat()` and checking that `ModeCharDevice` is NOT set in the mode bits
2. If stdin is a pipe, read all content from `os.Stdin` and return it as a string
3. If stdin is a terminal (interactive), return an empty string and no error
4. If reading stdin fails, return an error

**Requirement 4.12:** Implement a function for prompt assembly:

```go
func BuildSystemPrompt(systemPrompt, skillContent string, files []FileContent) string
```

**Requirement 4.13:** The `BuildSystemPrompt` function must assemble a single system prompt string by concatenating non-empty sections in this order with clear delimiters:

1. **System prompt** (from agent TOML `system_prompt` field) — included as-is if non-empty
2. **Skill content** (from SKILL.md) — prefixed with `\n\n---\n\n## Skill\n\n` if non-empty
3. **File contents** — prefixed with `\n\n---\n\n## Context Files\n\n`, each file formatted as:
   ```
   ### <path>
   ```<extension>
   <content>
   ```
   ```

**Requirement 4.14:** If all sections are empty, return an empty string.

**Requirement 4.15:** Stdin content is NOT included in the system prompt. Stdin is sent as the user message content (see Requirement 5.10).

### 2.5 `axe run` Command (`cmd/run.go`)

**Requirement 5.1:** Register a new `run` command on the root command.

**Requirement 5.2:** The `run` command must have:
- `Use`: `"run"`
- `Short`: `"Run an agent"`
- `Long`: A description explaining that this command loads an agent config, resolves context, calls the LLM, and prints the response.
- `Args`: `cobra.ExactArgs(1)` — the agent name is required

**Requirement 5.3:** Register the following flags on the `run` command:

| Flag | Type | Default | Shorthand | Description |
|------|------|---------|-----------|-------------|
| `--skill` | `string` | `""` | none | Override the agent's default skill path |
| `--workdir` | `string` | `""` | none | Override the working directory |
| `--model` | `string` | `""` | none | Override the model (full `provider/model` format) |
| `--timeout` | `int` | `120` | none | Request timeout in seconds |
| `--dry-run` | `bool` | `false` | none | Show resolved context without calling the LLM |
| `--verbose` | `bool` | `false` | `-v` | Print debug info to stderr |
| `--json` | `bool` | `false` | none | Wrap output in JSON with metadata |

**Requirement 5.4:** The `run` command `RunE` function must execute the following steps in order:

1. Load agent config via `agent.Load(args[0])`
2. Apply `--model` flag override if non-empty (replaces `AgentConfig.Model`)
3. Apply `--skill` flag override if non-empty (replaces `AgentConfig.Skill`)
4. Parse the model string to extract provider name and model name (split on first `/`)
5. Validate that the provider name is `"anthropic"` (the only supported provider in M3)
6. Resolve working directory via `resolve.Workdir(flagWorkdir, cfg.Workdir)`
7. Resolve file globs via `resolve.Files(cfg.Files, workdir)`
8. Load skill via `resolve.Skill(skillPath, configDir)`
9. Read stdin via `resolve.Stdin()`
10. Build system prompt via `resolve.BuildSystemPrompt(cfg.SystemPrompt, skillContent, files)`
11. If `--dry-run`: print resolved context and exit (see Requirement 5.8)
12. Resolve API key from environment variable `ANTHROPIC_API_KEY`
13. If the API key is empty, return an error with exit code 3: `ANTHROPIC_API_KEY environment variable is not set`
14. Create the provider via `provider.NewAnthropic(apiKey)`
15. Build the user message: stdin content if present, otherwise `"Execute the task described in your instructions."`
16. Build the `provider.Request` with model name, system prompt, user message, temperature, and max tokens from agent config
17. Create a `context.Context` with timeout from `--timeout` flag
18. Call `provider.Send(ctx, req)`
19. If `--json`: print JSON envelope (see Requirement 5.9)
20. If default mode: print `Response.Content` to stdout
21. If `--verbose`: print debug info to stderr (see Requirement 5.7)

**Requirement 5.5:** Model string parsing:

The model string (from TOML or `--model` flag) must be in the format `provider/model-name`. The `run` command must:
1. Split the string on the first `/` only (model names may contain `/`)
2. Extract the provider name (before the first `/`)
3. Extract the model identifier (after the first `/`)
4. If the string contains no `/`, return an error: `invalid model format "<model>": expected provider/model-name`
5. If the provider name is empty (string starts with `/`), return an error: `invalid model format "<model>": empty provider`
6. If the model identifier is empty (string ends with `/`), return an error: `invalid model format "<model>": empty model name`

**Requirement 5.6:** Unsupported provider:

If the parsed provider name is anything other than `"anthropic"`, return an error: `unsupported provider "<provider>": only "anthropic" is supported in this version`. This error must exit with code 1 (agent error).

**Requirement 5.7:** Verbose output (`--verbose` / `-v`):

When enabled, print the following debug information to stderr before making the LLM call (after context resolution, before the API call):

```
Model:    <provider/model-name>
Workdir:  <resolved_workdir>
Skill:    <skill_path or "(none)">
Files:    <count> file(s)
Stdin:    <"yes" or "no">
Timeout:  <seconds>s
Params:   temperature=<temp>, max_tokens=<max>
```

After the API call completes (success or failure), print:

```
Duration: <milliseconds>ms
Tokens:   <input> input, <output> output
Stop:     <stop_reason>
```

All verbose output must go to stderr. Response content must still go to stdout.

**Requirement 5.8:** Dry-run output (`--dry-run`):

When enabled, print the following to stdout and exit with code 0 without calling the LLM:

```
=== Dry Run ===

Model:    <provider/model-name>
Workdir:  <resolved_workdir>
Timeout:  <seconds>s
Params:   temperature=<temp>, max_tokens=<max>

--- System Prompt ---
<full assembled system prompt>

--- Skill ---
<skill contents or "(none)">

--- Files (<count>) ---
<file1_path>
<file2_path>
...

--- Stdin ---
<stdin content or "(none)">
```

If there are no files, print `(none)` instead of the file list.

**Requirement 5.9:** JSON output (`--json`):

When enabled, print a JSON object to stdout instead of raw response text:

```json
{
  "model": "<model as returned by API>",
  "content": "<response text>",
  "input_tokens": <number>,
  "output_tokens": <number>,
  "stop_reason": "<stop_reason>",
  "duration_ms": <number>
}
```

The JSON must be printed on a single line (compact, not pretty-printed). The `duration_ms` field measures wall-clock time from the start of the HTTP request to receipt of the complete response, in milliseconds.

**Requirement 5.10:** User message construction:

The user message sent to the LLM must be constructed as follows:
- If stdin content is non-empty (after trimming whitespace): use the stdin content as the user message
- If stdin content is empty: use the literal string `"Execute the task described in your instructions."`

Only one user message is sent per request. The user message is always the first (and only) entry in the `Messages` slice.

### 2.6 Exit Code Handling

**Requirement 6.1:** The `run` command must map errors to exit codes as follows:

| Exit Code | Condition |
|-----------|-----------|
| 0 | Success (including `--dry-run`) |
| 1 | Agent error: unsupported provider, invalid model format, bad request to API, agent not producing useful output |
| 2 | Config error: agent TOML not found, invalid TOML, missing required fields, invalid skill path, invalid glob pattern |
| 3 | API error: auth failure, rate limit, timeout, server error, overloaded, missing API key |

**Requirement 6.2:** The current `Execute()` function in `cmd/root.go` exits with code 1 for all errors. This must be updated to support differentiated exit codes.

**Requirement 6.3:** Define an `ExitError` type that wraps an error with an exit code:

```go
type ExitError struct {
    Code int
    Err  error
}
```

**Requirement 6.4:** `ExitError` must implement the `error` interface and `Unwrap() error`.

**Requirement 6.5:** The `Execute()` function must check if the returned error is an `ExitError` (using `errors.As`) and use its `Code` field for the process exit code. If the error is not an `ExitError`, default to exit code 1.

**Requirement 6.6:** The `run` command must wrap all errors with the appropriate exit code before returning:
- `agent.Load` errors → exit code 2
- `resolve.Skill` errors → exit code 2
- `resolve.Files` errors (invalid pattern) → exit code 2
- Model format errors → exit code 1
- Unsupported provider errors → exit code 1
- Missing API key → exit code 3
- `ProviderError` with `ErrCategoryAuth`, `ErrCategoryRateLimit`, `ErrCategoryTimeout`, `ErrCategoryOverloaded`, `ErrCategoryServer` → exit code 3
- `ProviderError` with `ErrCategoryBadRequest` → exit code 1
- All other errors → exit code 1

### 2.7 `rootCmd` Update

**Requirement 7.1:** Update the `rootCmd.Example` field to include a `run` example:

```
  axe run pr-reviewer     Run the pr-reviewer agent
```

This must be added alongside the existing examples.

---

## 3. Project Structure

After M3 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── run.go               # NEW: axe run command with all flags
│   ├── run_test.go           # NEW: tests for run command
│   ├── exit.go               # NEW: ExitError type
│   ├── exit_test.go          # NEW: tests for ExitError
│   ├── root.go               # MODIFIED: Execute() exit code handling, rootCmd.Example update
│   ├── root_test.go          # MODIFIED: add test for differentiated exit codes
│   ├── agents.go             # UNCHANGED
│   ├── agents_test.go        # UNCHANGED
│   ├── config.go             # UNCHANGED
│   ├── config_test.go        # UNCHANGED
│   ├── version.go            # UNCHANGED
│   └── version_test.go       # UNCHANGED
├── internal/
│   ├── provider/
│   │   ├── provider.go       # NEW: Message, Request, Response, Provider interface, ProviderError, ErrorCategory
│   │   ├── provider_test.go  # NEW: tests for types and error behavior
│   │   ├── anthropic.go      # NEW: Anthropic struct implementing Provider
│   │   └── anthropic_test.go # NEW: tests for Anthropic provider (using httptest)
│   ├── resolve/
│   │   ├── resolve.go        # NEW: Workdir, Files, Skill, Stdin, BuildSystemPrompt, FileContent
│   │   └── resolve_test.go   # NEW: tests for all resolution functions
│   ├── agent/                # UNCHANGED
│   │   ├── agent.go
│   │   └── agent_test.go
│   └── xdg/                  # UNCHANGED
│       ├── xdg.go
│       └── xdg_test.go
├── go.mod                    # UNCHANGED (no new dependencies)
├── go.sum                    # UNCHANGED
└── ...
```

---

## 4. Exit Codes

The full exit code table after M3:

| Code | Meaning | Used By |
|------|---------|---------|
| 0 | Success | All commands on success, `--dry-run` |
| 1 | Agent/general error | Bad model format, unsupported provider, bad request, `edit` with no `$EDITOR` |
| 2 | Config error | Agent not found, invalid TOML, missing required fields, invalid skill path, invalid glob pattern |
| 3 | API error | Auth failure, rate limit, timeout, server error, overloaded, missing API key |

---

## 5. Constraints

**Constraint 1:** No new external dependencies. All HTTP, JSON, and file I/O must use Go standard library packages.

**Constraint 2:** Only the Anthropic provider is implemented in M3. Provider selection is hardcoded to check for `"anthropic"`. The `Provider` interface exists to provide a clean seam for M4.

**Constraint 3:** No streaming. The entire response is received before any output is printed.

**Constraint 4:** No retry logic. Failed requests fail immediately.

**Constraint 5:** No caching of responses or context.

**Constraint 6:** All user-facing output must go through `cmd.OutOrStdout()` (stdout) or `cmd.ErrOrStderr()` (stderr) for testability. Verbose output goes to stderr. All other output goes to stdout.

**Constraint 7:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Stdin detection must work correctly on all platforms.

**Constraint 8:** The `**` glob pattern must be supported without adding a dependency. Use `filepath.WalkDir` for recursive matching when a pattern contains `**`.

**Constraint 9:** File contents read by `resolve.Files` are treated as text. Binary files (detected by the presence of null bytes in the first 512 bytes) must be skipped.

**Constraint 10:** The provider HTTP client must not follow redirects. Use a custom `http.Client` with `CheckRedirect` set to return `http.ErrUseLastResponse`.

---

## 6. Testing Requirements

### 6.1 Test Conventions

Tests must follow the patterns established in M1 and M2:

- **Package-level tests:** Tests live in the same package (e.g. `package provider`, `package resolve`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks (no testify, no gomock)
- **Temp directories:** Use `t.TempDir()` for filesystem isolation
- **Env overrides:** Use `t.Setenv()` for environment variable control
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern
- **Descriptive names:** `TestFunctionName_Scenario` with underscores
- **Test real code, not mocks.** Anthropic HTTP tests must use `httptest.NewServer` to simulate the real API, not mock the `Provider` interface. The `run` command tests must exercise the real command wiring through `rootCmd.Execute()`.

### 6.2 `internal/provider/` Tests

**Test: `TestProviderError_ErrorInterface`** — Verify `ProviderError` implements `error` with correct message format.

**Test: `TestProviderError_Unwrap`** — Verify `errors.As` and `errors.Is` work with wrapped errors.

**Test: `TestNewAnthropic_EmptyAPIKey`** — Empty string returns error.

**Test: `TestNewAnthropic_ValidAPIKey`** — Non-empty string returns `*Anthropic` with no error.

**Test: `TestAnthropic_Send_Success`** — Use `httptest.NewServer` returning a valid Anthropic response JSON. Verify `Response` fields are correctly populated (content, model, tokens, stop reason).

**Test: `TestAnthropic_Send_RequestFormat`** — Use `httptest.NewServer` that inspects the request. Verify:
- Method is POST
- URL path is `/v1/messages`
- Headers include `x-api-key`, `anthropic-version`, `content-type`
- Body contains correct JSON fields (model, messages, max_tokens, system)

**Test: `TestAnthropic_Send_OmitsEmptySystem`** — System field empty → `system` key absent from JSON body.

**Test: `TestAnthropic_Send_OmitsZeroTemperature`** — Temperature is 0 → `temperature` key absent from JSON body.

**Test: `TestAnthropic_Send_DefaultMaxTokens`** — MaxTokens is 0 → request body contains `max_tokens: 4096`.

**Test: `TestAnthropic_Send_AuthError`** — Server returns 401, verify `ProviderError` with `ErrCategoryAuth`.

**Test: `TestAnthropic_Send_RateLimitError`** — Server returns 429, verify `ProviderError` with `ErrCategoryRateLimit`.

**Test: `TestAnthropic_Send_ServerError`** — Server returns 500, verify `ProviderError` with `ErrCategoryServer`.

**Test: `TestAnthropic_Send_OverloadedError`** — Server returns 529, verify `ProviderError` with `ErrCategoryOverloaded`.

**Test: `TestAnthropic_Send_BadRequestError`** — Server returns 400, verify `ProviderError` with `ErrCategoryBadRequest`.

**Test: `TestAnthropic_Send_Timeout`** — Server delays longer than context deadline. Verify `ProviderError` with `ErrCategoryTimeout`.

**Test: `TestAnthropic_Send_EmptyContent`** — Server returns valid response with empty `content` array. Verify `ProviderError` with `ErrCategoryServer`.

**Test: `TestAnthropic_Send_ErrorResponseParsing`** — Server returns 400 with Anthropic error JSON. Verify `ProviderError.Message` contains the error message from the JSON body.

### 6.3 `internal/resolve/` Tests

**Test: `TestWorkdir_FlagOverride`** — Flag value is non-empty, verify it is returned regardless of TOML value.

**Test: `TestWorkdir_TOMLFallback`** — Flag is empty, TOML value is non-empty, verify TOML value is returned.

**Test: `TestWorkdir_CWDFallback`** — Both flag and TOML are empty, verify current working directory is returned.

**Test: `TestFiles_EmptyPatterns`** — Nil/empty patterns returns empty slice, no error.

**Test: `TestFiles_SimpleGlob`** — Create temp files matching `*.txt`, verify correct files returned with contents.

**Test: `TestFiles_DoubleStarGlob`** — Create nested directory structure, use `**/*.go` pattern, verify recursive matching.

**Test: `TestFiles_InvalidPattern`** — Malformed glob returns error.

**Test: `TestFiles_NoMatches`** — Valid pattern matching no files returns empty slice, no error.

**Test: `TestFiles_Deduplication`** — Two patterns matching the same file, verify only one entry in result.

**Test: `TestFiles_SortedOutput`** — Multiple matched files, verify alphabetical ordering by path.

**Test: `TestFiles_SkipsBinaryFiles`** — Create a file with null bytes, verify it is skipped.

**Test: `TestFiles_SymlinkOutsideWorkdir`** — Create symlink pointing outside workdir, verify it is skipped.

**Test: `TestSkill_EmptyPath`** — Empty path returns empty string, no error.

**Test: `TestSkill_AbsolutePath`** — Absolute path to existing file returns contents.

**Test: `TestSkill_RelativePath`** — Relative path resolved against configDir returns contents.

**Test: `TestSkill_NotFound`** — Non-existent path returns error with message `skill not found: <path>`.

**Test: `TestStdin_NotPiped`** — When stdin is a terminal (not piped), returns empty string. (This test may need to be skipped in CI environments where stdin behavior differs; document this.)

**Test: `TestBuildSystemPrompt_AllSections`** — System prompt, skill, and files all present. Verify correct assembly with delimiters.

**Test: `TestBuildSystemPrompt_SystemPromptOnly`** — Only system prompt, no skill or files. Verify no extraneous delimiters.

**Test: `TestBuildSystemPrompt_AllEmpty`** — All inputs empty, returns empty string.

**Test: `TestBuildSystemPrompt_SkillOnly`** — Only skill content present, verify section delimiter.

**Test: `TestBuildSystemPrompt_FilesOnly`** — Only files present, verify section delimiter and file formatting.

### 6.4 `cmd/` Tests

**Test: `TestExitError_ErrorInterface`** — Verify `ExitError` implements `error` and returns wrapped error message.

**Test: `TestExitError_Unwrap`** — Verify `errors.As` works to extract `ExitError` from wrapped errors.

**Test: `TestRun_MissingAgent`** — Run `axe run nonexistent` with temp XDG dir. Verify error message and exit code 2.

**Test: `TestRun_InvalidModelFormat`** — Create agent with `model = "noprefix"`. Verify error about invalid model format.

**Test: `TestRun_UnsupportedProvider`** — Create agent with `model = "openai/gpt-4"`. Verify error about unsupported provider.

**Test: `TestRun_MissingAPIKey`** — Create valid agent, unset `ANTHROPIC_API_KEY`. Verify error about missing API key and exit code 3.

**Test: `TestRun_DryRun`** — Create agent with system prompt, skill, files. Run with `--dry-run`. Verify output contains all resolved context sections.

**Test: `TestRun_DryRunNoFiles`** — Create agent with no files. Run with `--dry-run`. Verify `(none)` appears in files section.

**Test: `TestRun_Success`** — Create agent, set `ANTHROPIC_API_KEY`, start `httptest.NewServer` as mock Anthropic API. Inject the test server URL into the provider (the Anthropic struct must accept a base URL override for testing). Verify response content is printed to stdout.

**Test: `TestRun_JSONOutput`** — Same as success test but with `--json` flag. Verify output is valid JSON with all expected fields.

**Test: `TestRun_VerboseOutput`** — Same as success test but with `--verbose` flag. Verify debug info appears on stderr, response on stdout.

**Test: `TestRun_ModelOverride`** — Create agent, run with `--model anthropic/claude-haiku-3-20240307`. Verify the overridden model is used in the API request.

**Test: `TestRun_SkillOverride`** — Create agent, create a separate skill file, run with `--skill <path>`. Verify the overridden skill content appears in the prompt.

**Test: `TestRun_WorkdirOverride`** — Create agent with files, run with `--workdir <path>`. Verify files are resolved from the overridden directory.

**Test: `TestRun_TimeoutExceeded`** — Start a slow `httptest.NewServer`, run with `--timeout 1`. Verify exit code 3 and timeout error.

**Test: `TestRun_APIError`** — Start `httptest.NewServer` returning 500. Verify exit code 3.

**Test: `TestRun_NoArgs`** — Run `axe run` with no args. Verify usage error.

**Test: `TestRun_StdinPiped`** — Use `cmd.SetIn()` to provide stdin content. Verify it is used as the user message.

### 6.5 Anthropic Base URL Override for Testing

**Requirement (Test Support):** The `Anthropic` struct must accept an optional base URL override. This is used by tests to point the provider at an `httptest.NewServer` instead of the real Anthropic API.

Constructor signature:

```go
func NewAnthropic(apiKey string, opts ...AnthropicOption) (*Anthropic, error)
```

Where `AnthropicOption` is a functional option. The only option in M3 is:

```go
func WithBaseURL(url string) AnthropicOption
```

If no `WithBaseURL` option is provided, the default base URL is `https://api.anthropic.com`.

### 6.6 Running Tests

All tests must pass when run with:

```bash
make test
```

If no `Makefile` exists, tests must pass with:

```bash
go test ./...
```

No test may make real HTTP requests to external APIs. All HTTP interactions must use `httptest.NewServer`.

---

## 7. Acceptance Criteria

| Criterion | Test |
|-----------|------|
| Provider Types | `internal/provider/provider.go` defines `Message`, `Request`, `Response`, `Provider`, `ProviderError`, `ErrorCategory` |
| Anthropic Provider | `provider.NewAnthropic(key)` creates a working provider that calls the Messages API |
| Auth Header | HTTP requests include `x-api-key` header with the provided API key |
| Error Mapping | HTTP 401→auth, 429→rate_limit, 500→server, 529→overloaded, 400→bad_request |
| Timeout | Context cancellation aborts the request and returns timeout error |
| Workdir Resolution | Flag → TOML → cwd chain works correctly |
| File Glob Resolution | Simple and `**` globs resolve files correctly from workdir |
| Binary File Skip | Files with null bytes in the first 512 bytes are skipped |
| Skill Loading | Absolute and relative paths resolve correctly |
| Stdin Detection | Piped stdin is read; terminal stdin is ignored |
| Prompt Assembly | System prompt + skill + files assembled with correct delimiters |
| `axe run <agent>` | Loads config, resolves context, calls LLM, prints response |
| `--dry-run` | Shows all resolved context without calling LLM |
| `--verbose` | Debug info to stderr before and after LLM call |
| `--json` | Structured JSON output with token counts and duration |
| `--model` | Overrides agent config model |
| `--skill` | Overrides agent config skill path |
| `--workdir` | Overrides agent config working directory |
| `--timeout` | Limits request duration |
| Model Parsing | `provider/model` format parsed correctly; errors on invalid format |
| Exit Code 2 | Config errors (missing agent, bad TOML, invalid skill, invalid glob) |
| Exit Code 3 | API errors (auth, rate limit, timeout, server, missing key) |
| Exit Code 1 | Agent errors (bad model format, unsupported provider, bad request) |
| No New Deps | `go.mod` contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies |
| All Tests Pass | `go test ./...` passes with 0 failures |

---

## 8. Out of Scope

The following items are explicitly **not** included in M3:

1. OpenAI provider (M4)
2. Ollama / local provider (M4)
3. Provider abstraction routing by name (M4 — M3 hardcodes `"anthropic"` check)
4. API key from config file (M4 — M3 reads from environment variables only)
5. Sub-agent invocation or `call_agent` tool injection (M5)
6. Memory read/write (M6)
7. Garbage collection (M7)
8. Streaming output (Future)
9. Retry logic or exponential backoff
10. Response caching
11. Token cost estimation or budget tracking
12. Multi-turn conversations (one user message per run)
13. Image or non-text content in messages
14. Tool use / function calling in the LLM request
15. Interactive prompts or TUI elements
16. Rate limit queuing or backpressure
17. Config file hot-reloading
18. `axe skills` commands (not part of M3 scope)

---

## 9. References

- Milestone Definition: `docs/plans/000_milestones.md` (M3 section)
- M1 Skeleton Spec: `docs/plans/001_skeleton_spec.md`
- M2 Agent Config Spec: `docs/plans/002_agent_config_spec.md`
- Agent Config Schema: `docs/design/agent-config-schema.md`
- CLI Structure: `docs/design/cli-structure.md`
- Anthropic Messages API: https://docs.anthropic.com/en/api/messages
- XDG Base Directory Specification: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

---

## 10. Notes

- The `Provider` interface is intentionally minimal (one method). M4 will add provider routing and potentially extend the interface, but M3 callers only need `Send`.
- The `Anthropic` struct uses functional options (`WithBaseURL`) specifically to support test injection. The option pattern is chosen over struct fields to keep the public API clean and prevent misconfiguration.
- The `resolve` package name was chosen over `context` to avoid import shadowing with Go's standard `context` package.
- Stdin is sent as the user message, not embedded in the system prompt. This follows the Anthropic best practice of keeping system instructions in the system prompt and user-provided content in user messages.
- The default user message `"Execute the task described in your instructions."` is used when no stdin is piped. This signals to the LLM that it should follow its system prompt/skill instructions without additional user input.
- The `**` glob implementation using `filepath.WalkDir` is a pragmatic choice to avoid adding a dependency like `doublestar`. It handles the common case (`**/*.ext`) but does not need to support every edge case of full doublestar spec. Specifically, patterns like `a/**/b/**/c` must work, but `{a,b}` brace expansion is not required.
- Binary file detection (null byte check in first 512 bytes) is a simple heuristic. It will correctly skip most binary files (images, compiled objects, etc.) but may miss some edge cases. This is acceptable for M3.
- Exit code differentiation requires modifying `Execute()` in `root.go`. This is a breaking change to the error handling pattern — all existing commands that return errors will continue to exit with code 1 (unchanged behavior) unless they explicitly return an `ExitError`.
- The `--json` output is compact (single line) to support piping to tools like `jq`. Pretty-printing could be added as a future enhancement but is not part of M3.
- The `--timeout` default of 120 seconds matches common LLM API timeouts. Long-running agents (e.g. processing large codebases) may need higher values.
