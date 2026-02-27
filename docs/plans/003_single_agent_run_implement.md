# Implementation Checklist: M3 - Single Agent Run

**Based on:** 003_single_agent_run_spec.md
**Status:** In Progress
**Created:** 2026-02-27

---

## Phase 1: Project Setup

- [x] Create branch `feat/003-single-agent-run` off `develop`
- [x] Create `internal/provider/` directory
- [x] Create `internal/resolve/` directory
- [x] Verify `go.mod` has no new dependencies after phase completion

---

## Phase 2: ExitError Type (`cmd/exit.go`) (Spec SS2.6)

- [x] Write `TestExitError_ErrorInterface` -- verify `ExitError` implements `error` and returns wrapped error message
- [x] Write `TestExitError_Unwrap` -- verify `errors.As` works to extract `ExitError` from wrapped errors
- [x] Implement `ExitError` struct with `Code int` and `Err error` fields in `cmd/exit.go`
- [x] Implement `Error() string` method on `ExitError` (delegates to `Err.Error()`)
- [x] Implement `Unwrap() error` method on `ExitError`
- [x] Run tests -- all ExitError tests pass

---

## Phase 3: Update `Execute()` for Differentiated Exit Codes (`cmd/root.go`) (Spec SS2.6)

- [x] Write `TestExecute_ExitError` -- verify `Execute()` extracts exit code from `ExitError` (use `errors.As`)
- [x] Write `TestExecute_DefaultExitCode` -- verify non-`ExitError` errors default to exit code 1
- [x] Update `Execute()` in `cmd/root.go` to check for `ExitError` using `errors.As` and use its `Code` field
- [x] If error is not `ExitError`, default to exit code 1 (preserves existing behavior)
- [x] Run tests -- all root tests pass (existing + new)

---

## Phase 4: Provider Types (`internal/provider/provider.go`) (Spec SS2.2)

- [x] Write `TestProviderError_ErrorInterface` -- verify `ProviderError` implements `error` with format `<category>: <message>`
- [x] Write `TestProviderError_Unwrap` -- verify `errors.As` and `errors.Is` work with wrapped errors
- [x] Define `ErrorCategory` string type and constants: `ErrCategoryAuth`, `ErrCategoryRateLimit`, `ErrCategoryTimeout`, `ErrCategoryOverloaded`, `ErrCategoryBadRequest`, `ErrCategoryServer`
- [x] Define `Message` struct with `Role string` and `Content string` (JSON tags: `role`, `content`)
- [x] Define `Request` struct with `Model`, `System`, `Messages`, `Temperature`, `MaxTokens` fields
- [x] Define `Response` struct with `Content`, `Model`, `InputTokens`, `OutputTokens`, `StopReason` fields
- [x] Define `Provider` interface with `Send(ctx context.Context, req *Request) (*Response, error)` method
- [x] Define `ProviderError` struct with `Category ErrorCategory`, `Status int`, `Message string`, `Err error`
- [x] Implement `Error() string` on `ProviderError` returning `<category>: <message>`
- [x] Implement `Unwrap() error` on `ProviderError`
- [x] Run tests -- all provider type tests pass

---

## Phase 5: Anthropic Provider (`internal/provider/anthropic.go`) (Spec SS2.3)

### 5a: Constructor

- [x] Write `TestNewAnthropic_EmptyAPIKey` -- empty string returns error
- [x] Write `TestNewAnthropic_ValidAPIKey` -- non-empty string returns `*Anthropic` with no error
- [x] Define `AnthropicOption` functional option type
- [x] Implement `WithBaseURL(url string) AnthropicOption`
- [x] Define `Anthropic` struct with `apiKey`, `baseURL`, `client *http.Client` fields
- [x] Define `defaultMaxTokens = 4096` named constant
- [x] Implement `NewAnthropic(apiKey string, opts ...AnthropicOption) (*Anthropic, error)`
  - [x] Return error if `apiKey` is empty
  - [x] Default `baseURL` to `https://api.anthropic.com`
  - [x] Configure `http.Client` with `CheckRedirect` returning `http.ErrUseLastResponse` (no redirects)
  - [x] Apply functional options
- [x] Run tests -- constructor tests pass

### 5b: Send Method - Success Path

- [x] Write `TestAnthropic_Send_Success` -- `httptest.NewServer` returning valid Anthropic JSON; verify all `Response` fields populated
- [x] Write `TestAnthropic_Send_RequestFormat` -- `httptest.NewServer` inspecting request; verify POST method, `/v1/messages` path, required headers (`x-api-key`, `anthropic-version`, `content-type`), correct JSON body
- [x] Write `TestAnthropic_Send_OmitsEmptySystem` -- system field empty; verify `system` key absent from JSON body
- [x] Write `TestAnthropic_Send_OmitsZeroTemperature` -- temperature is 0; verify `temperature` key absent from JSON body
- [x] Write `TestAnthropic_Send_DefaultMaxTokens` -- MaxTokens is 0; verify request body contains `max_tokens: 4096`
- [x] Implement `Send(ctx context.Context, req *Request) (*Response, error)` on `Anthropic`:
  - [x] Build Anthropic request JSON body (omit `system` if empty, omit `temperature` if 0, default `max_tokens` to 4096 if 0)
  - [x] Create `http.NewRequestWithContext` POST to `<baseURL>/v1/messages`
  - [x] Set required headers: `x-api-key`, `anthropic-version: 2023-06-01`, `content-type: application/json`
  - [x] Execute request via `a.client.Do(req)`
  - [x] Parse success response: extract `content[0].text`, `model`, `usage.input_tokens`, `usage.output_tokens`, `stop_reason`
  - [x] Return `*Response` on success
- [x] Run tests -- all success-path tests pass

### 5c: Send Method - Error Handling

- [x] Write `TestAnthropic_Send_EmptyContent` -- server returns valid response with empty `content` array; verify `ProviderError` with `ErrCategoryServer`
- [x] Write `TestAnthropic_Send_AuthError` -- server returns 401; verify `ProviderError` with `ErrCategoryAuth`
- [x] Write `TestAnthropic_Send_RateLimitError` -- server returns 429; verify `ProviderError` with `ErrCategoryRateLimit`
- [x] Write `TestAnthropic_Send_ServerError` -- server returns 500; verify `ProviderError` with `ErrCategoryServer`
- [x] Write `TestAnthropic_Send_OverloadedError` -- server returns 529; verify `ProviderError` with `ErrCategoryOverloaded`
- [x] Write `TestAnthropic_Send_BadRequestError` -- server returns 400; verify `ProviderError` with `ErrCategoryBadRequest`
- [x] Write `TestAnthropic_Send_Timeout` -- server delays longer than context deadline; verify `ProviderError` with `ErrCategoryTimeout`
- [x] Write `TestAnthropic_Send_ErrorResponseParsing` -- server returns 400 with Anthropic error JSON; verify `ProviderError.Message` contains the error message from JSON body
- [x] Implement error handling in `Send`:
  - [x] Check for context cancellation/deadline exceeded; return `ProviderError` with `ErrCategoryTimeout`
  - [x] For non-2xx responses, attempt to parse Anthropic error JSON; fall back to HTTP status text
  - [x] Map HTTP status codes: 401->Auth, 400->BadRequest, 429->RateLimit, 529->Overloaded, 500/502/503->Server
  - [x] Check for empty `content` array; return `ProviderError` with `ErrCategoryServer`
- [x] Run tests -- all error-handling tests pass

---

## Phase 6: Context Resolution - Workdir (`internal/resolve/resolve.go`) (Spec SS2.4, Req 4.1)

- [x] Write `TestWorkdir_FlagOverride` -- flag value non-empty; verify it is returned regardless of TOML value
- [x] Write `TestWorkdir_TOMLFallback` -- flag empty, TOML non-empty; verify TOML value returned
- [x] Write `TestWorkdir_CWDFallback` -- both flag and TOML empty; verify current working directory returned
- [x] Implement `Workdir(flagValue, tomlValue string) string` in `internal/resolve/resolve.go`
  - [x] Return `flagValue` if non-empty
  - [x] Return `tomlValue` if non-empty
  - [x] Return `os.Getwd()` result; on error return `"."`
- [x] Run tests -- all Workdir tests pass

---

## Phase 7: Context Resolution - Files (`internal/resolve/resolve.go`) (Spec SS2.4, Reqs 4.2-4.7)

- [x] Write `TestFiles_EmptyPatterns` -- nil/empty patterns returns empty slice, no error
- [x] Write `TestFiles_SimpleGlob` -- create temp files matching `*.txt`; verify correct files returned with contents
- [x] Write `TestFiles_DoubleStarGlob` -- create nested dirs; use `**/*.go` pattern; verify recursive matching
- [x] Write `TestFiles_InvalidPattern` -- malformed glob returns error
- [x] Write `TestFiles_NoMatches` -- valid pattern matching no files returns empty slice, no error
- [x] Write `TestFiles_Deduplication` -- two patterns matching same file; verify only one entry in result
- [x] Write `TestFiles_SortedOutput` -- multiple matched files; verify alphabetical ordering by path
- [x] Write `TestFiles_SkipsBinaryFiles` -- file with null bytes in first 512 bytes; verify it is skipped
- [x] Write `TestFiles_SymlinkOutsideWorkdir` -- symlink pointing outside workdir; verify it is skipped
- [x] Define `FileContent` struct with `Path string` and `Content string`
- [x] Implement `Files(patterns []string, workdir string) ([]FileContent, error)`:
  - [x] Return empty slice for nil/empty patterns
  - [x] Return error for malformed glob syntax
  - [x] For patterns without `**`, use `filepath.Glob` relative to workdir
  - [x] For patterns with `**`, use `filepath.WalkDir` for recursive matching
  - [x] Read file contents; skip binary files (null byte in first 512 bytes)
  - [x] Skip symlinks that resolve outside workdir
  - [x] Deduplicate by path (first occurrence wins)
  - [x] Sort results by path
- [x] Run tests -- all Files tests pass

---

## Phase 8: Context Resolution - Skill (`internal/resolve/resolve.go`) (Spec SS2.4, Reqs 4.8-4.9)

- [x] Write `TestSkill_EmptyPath` -- empty path returns empty string, no error
- [x] Write `TestSkill_AbsolutePath` -- absolute path to existing file returns contents
- [x] Write `TestSkill_RelativePath` -- relative path resolved against configDir returns contents
- [x] Write `TestSkill_NotFound` -- non-existent path returns error with `skill not found: <path>`
- [x] Implement `Skill(skillPath, configDir string) (string, error)`:
  - [x] If `skillPath` empty, return `""`, `nil`
  - [x] If absolute path, read file directly
  - [x] If relative path, resolve relative to `configDir`
  - [x] Return file contents or appropriate error
- [x] Run tests -- all Skill tests pass

---

## Phase 9: Context Resolution - Stdin (`internal/resolve/resolve.go`) (Spec SS2.4, Reqs 4.10-4.11)

- [ ] Write `TestStdin_NotPiped` -- document that this test may need skipping in some CI environments
- [ ] Implement `Stdin() (string, error)`:
  - [ ] Check `os.Stdin.Stat()` for `ModeCharDevice` NOT set
  - [ ] If pipe, read all stdin and return as string
  - [ ] If terminal, return `""`, `nil`
  - [ ] If read fails, return error
- [ ] Run tests -- Stdin tests pass

---

## Phase 10: Context Resolution - BuildSystemPrompt (`internal/resolve/resolve.go`) (Spec SS2.4, Reqs 4.12-4.15)

- [ ] Write `TestBuildSystemPrompt_AllSections` -- system prompt, skill, and files all present; verify correct assembly with delimiters
- [ ] Write `TestBuildSystemPrompt_SystemPromptOnly` -- only system prompt; verify no extraneous delimiters
- [ ] Write `TestBuildSystemPrompt_AllEmpty` -- all inputs empty; returns empty string
- [ ] Write `TestBuildSystemPrompt_SkillOnly` -- only skill content present; verify section delimiter
- [ ] Write `TestBuildSystemPrompt_FilesOnly` -- only files present; verify section delimiter and file formatting
- [ ] Implement `BuildSystemPrompt(systemPrompt, skillContent string, files []FileContent) string`:
  - [ ] Concatenate non-empty sections with specified delimiters
  - [ ] System prompt included as-is
  - [ ] Skill prefixed with `\n\n---\n\n## Skill\n\n`
  - [ ] Files prefixed with `\n\n---\n\n## Context Files\n\n`, each file as `### <path>` + fenced code block with extension
  - [ ] Return empty string if all sections empty
- [ ] Run tests -- all BuildSystemPrompt tests pass

---

## Phase 11: `axe run` Command (`cmd/run.go`) (Spec SS2.5)

### 11a: Command Registration and Flags

- [ ] Write `TestRun_NoArgs` -- `axe run` with no args; verify usage error
- [ ] Create `cmd/run.go` with `runCmd` cobra command
  - [ ] Set `Use: "run"`, `Short: "Run an agent"`, `Long` description, `Args: cobra.ExactArgs(1)`
  - [ ] Register flags: `--skill`, `--workdir`, `--model`, `--timeout` (default 120), `--dry-run`, `--verbose` (`-v`), `--json`
  - [ ] Register `runCmd` on `rootCmd` in `init()`
- [ ] Update `rootCmd.Example` in `cmd/root.go` to include `axe run pr-reviewer` example
- [ ] Run test -- NoArgs test passes

### 11b: Model Parsing and Provider Validation

- [ ] Write `TestRun_InvalidModelFormat` -- agent with `model = "noprefix"`; verify error about invalid model format
- [ ] Write `TestRun_UnsupportedProvider` -- agent with `model = "openai/gpt-4"`; verify error about unsupported provider
- [ ] Implement model string parsing in `RunE`:
  - [ ] Split on first `/` only
  - [ ] Error if no `/`: `invalid model format "<model>": expected provider/model-name`
  - [ ] Error if empty provider: `invalid model format "<model>": empty provider`
  - [ ] Error if empty model name: `invalid model format "<model>": empty model name`
  - [ ] Error if provider is not `"anthropic"`: `unsupported provider "<provider>": only "anthropic" is supported in this version`
- [ ] Run tests -- model parsing tests pass

### 11c: Config Loading and Overrides

- [ ] Write `TestRun_MissingAgent` -- `axe run nonexistent` with temp XDG dir; verify error and exit code 2
- [ ] Write `TestRun_MissingAPIKey` -- valid agent, unset `ANTHROPIC_API_KEY`; verify error and exit code 3
- [ ] Implement `RunE` steps 1-13:
  - [ ] Load agent config via `agent.Load(args[0])`; wrap error with exit code 2
  - [ ] Apply `--model` override if non-empty
  - [ ] Apply `--skill` override if non-empty
  - [ ] Parse model string (from 11b)
  - [ ] Validate provider is `"anthropic"`
  - [ ] Resolve workdir, files, skill, stdin
  - [ ] Build system prompt
  - [ ] Check `ANTHROPIC_API_KEY`; wrap error with exit code 3
- [ ] Run tests -- config loading tests pass

### 11d: Dry-Run Mode

- [ ] Write `TestRun_DryRun` -- agent with system prompt, skill, files; run with `--dry-run`; verify output contains all resolved context sections
- [ ] Write `TestRun_DryRunNoFiles` -- agent with no files; run with `--dry-run`; verify `(none)` in files section
- [ ] Implement `--dry-run` output (Spec Req 5.8):
  - [ ] Print formatted dry-run output to `cmd.OutOrStdout()`
  - [ ] Return `nil` (exit code 0) without calling LLM
- [ ] Run tests -- dry-run tests pass

### 11e: LLM Call and Default Output

- [ ] Write `TestRun_Success` -- agent + `ANTHROPIC_API_KEY` + `httptest.NewServer` mock; verify response content printed to stdout
- [ ] Write `TestRun_StdinPiped` -- use `cmd.SetIn()` to provide stdin content; verify it is used as user message
- [ ] Write `TestRun_ModelOverride` -- run with `--model anthropic/claude-haiku-3-20240307`; verify overridden model in API request
- [ ] Write `TestRun_SkillOverride` -- run with `--skill <path>`; verify overridden skill content in prompt
- [ ] Write `TestRun_WorkdirOverride` -- run with `--workdir <path>`; verify files resolved from overridden directory
- [ ] Implement `RunE` steps 14-20:
  - [ ] Create provider via `provider.NewAnthropic(apiKey)`
  - [ ] Build user message (stdin content or default string)
  - [ ] Build `provider.Request`
  - [ ] Create `context.WithTimeout` from `--timeout` flag
  - [ ] Call `provider.Send(ctx, req)`
  - [ ] Print `Response.Content` to `cmd.OutOrStdout()`
- [ ] Run tests -- success path tests pass

### 11f: JSON Output Mode

- [ ] Write `TestRun_JSONOutput` -- same as success test with `--json`; verify valid JSON with all expected fields
- [ ] Implement `--json` output (Spec Req 5.9):
  - [ ] Build JSON object with `model`, `content`, `input_tokens`, `output_tokens`, `stop_reason`, `duration_ms`
  - [ ] Print compact JSON to `cmd.OutOrStdout()`
- [ ] Run tests -- JSON output test passes

### 11g: Verbose Output Mode

- [ ] Write `TestRun_VerboseOutput` -- same as success test with `--verbose`; verify debug info on stderr, response on stdout
- [ ] Implement `--verbose` output (Spec Req 5.7):
  - [ ] Print pre-call debug info to `cmd.ErrOrStderr()`: Model, Workdir, Skill, Files count, Stdin, Timeout, Params
  - [ ] Print post-call debug info to `cmd.ErrOrStderr()`: Duration, Tokens, Stop reason
- [ ] Run tests -- verbose output test passes

### 11h: Error Exit Code Mapping

- [ ] Write `TestRun_TimeoutExceeded` -- slow `httptest.NewServer`, run with `--timeout 1`; verify exit code 3
- [ ] Write `TestRun_APIError` -- `httptest.NewServer` returning 500; verify exit code 3
- [ ] Implement exit code mapping for all provider errors (Spec Req 6.6):
  - [ ] `ProviderError` Auth/RateLimit/Timeout/Overloaded/Server -> exit code 3
  - [ ] `ProviderError` BadRequest -> exit code 1
  - [ ] `agent.Load` errors -> exit code 2
  - [ ] `resolve.Skill` errors -> exit code 2
  - [ ] `resolve.Files` errors (invalid pattern) -> exit code 2
  - [ ] Model format / unsupported provider errors -> exit code 1
  - [ ] Missing API key -> exit code 3
- [ ] Run tests -- all exit code tests pass

---

## Phase 12: Full Test Suite

- [ ] Run `go test ./...` -- all tests pass with 0 failures
- [ ] Run `go vet ./...` -- no issues
- [ ] Run `go build` -- binary compiles without errors
- [ ] Verify `go.mod` contains only `spf13/cobra` and `BurntSushi/toml` as direct dependencies
- [ ] Run `go mod tidy` -- no changes

---

## Phase 13: Verification

- [ ] Manual test: `axe run nonexistent` -- error about missing agent config, exit code 2
- [ ] Manual test: Create agent with invalid model format, `axe run <agent>` -- error about model format
- [ ] Manual test: Create agent with `model = "openai/gpt-4"`, `axe run <agent>` -- unsupported provider error
- [ ] Manual test: Create valid agent, unset `ANTHROPIC_API_KEY`, `axe run <agent>` -- missing API key error, exit code 3
- [ ] Manual test: Create valid agent, set `ANTHROPIC_API_KEY`, `axe run <agent> --dry-run` -- shows resolved context
- [ ] Manual test: `echo "hello" | axe run <agent> --dry-run` -- stdin appears in output
- [ ] Manual test: `axe run <agent> --verbose --dry-run` -- verbose info on stderr
- [ ] Manual test: `axe run <agent>` with valid API key -- receives LLM response on stdout
- [ ] Manual test: `axe run <agent> --json` -- JSON output with metadata
- [ ] Manual test: `axe run <agent> --model anthropic/claude-haiku-3-20240307` -- model override works
- [ ] Manual test: `axe run <agent> --timeout 1` with slow response -- timeout error, exit code 3

---

## Definition of Done

- [ ] All checkboxes in Phases 1-13 are completed
- [ ] All acceptance criteria from 003_single_agent_run_spec.md are met
- [ ] Binary builds successfully with `go build`
- [ ] All tests pass with `go test ./...`
- [ ] No new external dependencies added to `go.mod`
- [ ] Ready for M4: Multi-Provider Support
