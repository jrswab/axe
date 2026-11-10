# Milestone 009: Set the provider API key and base URL from code with `Options.APIKey` and `Options.BaseURL`

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 009 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:238-250`). Scope and acceptance tests 1-7 are taken from that entry, as bounded by the orchestrator decisions OS1-OS7 below.
- Design source: `docs/plans/runner-tool-hooks-and-options.md` §A7 (`docs/plans/runner-tool-hooks-and-options.md:475-521`) and milestone decision D11 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:73`).
- Predecessor: none. The entry declares `Depends on: none` (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:240`). Milestone 008 (`Options.MaxTurns`) is complete and is the precedent for the field, doc comment, and CHANGELOG pattern.
- Baseline: branch `runner-tool-hooks`, review base `08d0f26c1ca5f076725ba21ce09a83ac89c815b7`. Clean tree at spec time.
- Lane: full. The milestone adds exported fields to `pkg/runner`, the external Go API. Public Go API is an external contract (`COMPATIBILITY.md:5-10`), so the fast-lane condition "external contracts untouched" fails.

### Citation corrections (verified at `08d0f26`)

The milestone entry and the design note cite lines that do not match current code. This spec uses the verified lines. Neither source document is edited by this spec.

- The apply point the entry cites at `pkg/runner/run.go:336-337` is at `pkg/runner/run.go:385-386` (`apiKey := globalCfg.ResolveAPIKey(provName)`, `baseURL := globalCfg.ResolveBaseURL(provName)`).
- The bedrock block the entry cites at `pkg/runner/run.go:340-343` is at `pkg/runner/run.go:389-395`. The ConfigError is at `:390-392`; `apiKey = region` is at `:393`; `baseURL = ""` is at `:394`.
- The empty-key check is at `pkg/runner/run.go:397-400`. The `newProvider` call is at `pkg/runner/run.go:407`. The `run` seam (`pkg/runner/run.go:49`) (inside `Run`) and `func run` is at `pkg/runner/run.go:65`. The entry's `:47` and its `:358` are stale.
- The dry-run block (`pkg/runner/run.go:328-379`), and its `return &Result{` is at `:353`.
- `Options` (`pkg/runner/options.go:16-73`). The `MaxTurns` field (from 008) is at `pkg/runner/options.go:63-67`. The new fields go after `Tools` (`pkg/runner/options.go:55-61`) and before the `MaxTurns` block, or after it; placement is free as long as the doc comments are complete. The I/O fields begin at `pkg/runner/options.go:69`.
- `ResolveAPIKey` (`internal/config/config.go:81`) and `ResolveBaseURL` (`internal/config/config.go:99`). Both return env value, then `config.toml`, then `""` (`internal/config/config.go:84-94`, `:102-112`).
- `ResolveRegion` (`internal/config/config.go:117-140`). The order is `AWS_REGION`, `AWS_DEFAULT_REGION`, `AXE_BEDROCK_REGION`, then `config.toml` (`internal/config/config.go:119-137`).
- A missing `config.toml` is tolerated: `os.ReadFile` at `internal/config/config.go:38`, `os.IsNotExist` return at `:40-42`.
- `config.Load` is called at `pkg/runner/run.go:144`.
- AWS credentials: `resolveCredentials` (`internal/provider/awscreds.go:19-45`). The entry's `:17-44` is stale.
- The registry bedrock case (`internal/provider/registry.go:71-74`) (`case "bedrock":` at `:71`, `NewBedrock(apiKey)` at `:74`). The entry's `:72-75` is stale. The registry is not edited.
- Sub-agent credentials: `internal/tool/tool.go:224-225`, unchanged.
- `cmd/gc.go` credential block: `cmd/gc.go:130-141` (the bedrock block is `:134-141`). Unchanged.
- The Anthropic provider sends `x-api-key` from its stored key (`internal/provider/anthropic.go:271`) to `a.baseURL+"/v1/messages"` (`internal/provider/anthropic.go:266`). `NewAnthropic` rejects an empty key (`internal/provider/anthropic.go:43-44`).
- `Options.AgentsDirs` is searched before the default directories (`pkg/runner/run.go:85-87`). Agent loading does not read env.
- The verbose pre-call block (`pkg/runner/run.go:529-530`). The retry verbose line (`internal/provider/retry.go:74-80`). Neither prints a key or base URL (verified by search of `pkg/runner/run.go` and `internal/provider/*.go` for `apiKey` and `baseURL` in print calls).
- `ConfigError` (`pkg/runner/errors.go:11`). The comment says the CLI maps it to exit code 2 (`pkg/runner/errors.go:9-10`). `IsConfigError` (`pkg/runner/errors.go:57`). `IsRuntimeError` (`pkg/runner/errors.go:63`).
- Test helpers: `setupTestEnv` (`pkg/runner/run_test.go:24-42`) and calls `t.Setenv("XDG_CONFIG_HOME", ...)` at `:27`. `newMockStdoutStderr` (`pkg/runner/run_test.go:44`). The existing newProvider seam pattern (`pkg/runner/run_test.go:96-101`). `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336-1379`). No `pkg/runner` test calls `t.Parallel` (search of `pkg/runner/*_test.go`).
- `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`). `URL()` at `internal/testutil/mockserver.go:122`. `RequestCount()` at `internal/testutil/mockserver.go:127`. The recorded requests are `Requests` (`:34`) with `Headers` (`:26`) and `Body` (`:27`). `AnthropicResponse` (`internal/testutil/mockserver.go:141`). `AnthropicToolUseResponse` at `internal/testutil/mockserver.go:154`.
- `COMPATIBILITY.md:5-10` (pkg/runner rules), `:31-35` (opt-in, zero value), `:39-41` (apidiff gate). `CHANGELOG.md:8` is `## [Unreleased]`, `:10` is `### Added`, and the 008 bullet (`CHANGELOG.md:17`).
- `Makefile:14-24` is the `apidiff` target, which compares `pkg/runner` against tag `v1.11.0`. The tag exists locally.

### Decisions settled (record as settled, never re-evaluate)

The orchestrator's decisions D1-D7 are recorded here as OS1-OS7. Each is binding for this milestone.

- **OS1. Override placement (orchestrator D1).** The override goes immediately after `pkg/runner/run.go:385-386` and before the bedrock block at `:389`:
  - `if opts.APIKey != "" { apiKey = opts.APIKey }`
  - `if opts.BaseURL != "" { baseURL = opts.BaseURL }`
  - The bedrock block (`pkg/runner/run.go:389-395`) stays byte-for-byte unchanged. Options values are ignored for bedrock by construction, because the bedrock block overwrites `apiKey` and clears `baseURL`.
  - No `if apiKey == ""` guard is added around `apiKey = region`. That guard would turn the key into the region and is forbidden (milestone `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:111`).
- **OS2. Empty means unset (orchestrator D2).** An empty string is the zero value and means "not set". No format or URL validation is added. No new `ConfigError` is raised for either field.
- **OS3. Neither value is printed (orchestrator D3).** Neither value appears in verbose output, stderr, stdout, errors, or dry-run output. There is no `DryRunInfo` field and no `Result` field. `DryRunInfo` and `--json` are unchanged. This follows the 008 OS4 precedent.
- **OS4. Doc comments (orchestrator D4).** The doc comment on each field includes the sentence `Ignored when the provider is bedrock.` and states the precedence `Options > env var > config.toml`.
- **OS5. Test placement (orchestrator D5).** Tests go in a new file `pkg/runner/credentials_option_test.go`. Test 4 uses `t.Parallel`, `Options.AgentsDirs` with a `t.TempDir` agent TOML, `testutil.NewMockLLMServer`, and no `t.Setenv`. Tests 2, 3, 6, and 7 are not parallel and use `t.Setenv` for env sources with an isolated `XDG_CONFIG_HOME`. The `run()` seam is used for tests 6 and 7.
- **OS6. CHANGELOG (orchestrator D6).** `CHANGELOG.md` `[Unreleased]` `### Added` gets one bullet. Sub-agent credentials (`internal/tool/tool.go:224-225`), `cmd/gc.go`, `internal/provider/registry.go`, OpenRouter attribution, CLI flags, TOML keys, and `--json` are out of scope.
- **OS7. Compatibility (orchestrator D7).** The change is additive: two exported `string` fields on `Options`. `make apidiff` must exit 0 with no incompatible-change report.
- **OS8. Only the apply point changes.** `pkg/runner/run.go` changes only at the override lines (OS1). The sub-agent path, the registry, the CLI, and the config package are not edited. Sub-agents do not inherit `Options.APIKey` or `Options.BaseURL`; their credentials still resolve from env and config (`internal/tool/tool.go:224-225`). This is recorded as a limitation, not a requirement change.

### Rejected approaches (settled)

- Guarding `apiKey = region` with `if apiKey == ""` (OS1). Rejected because the key would become the region.
- Validating the key format or the URL (OS2). Rejected by the orchestrator.
- Printing either value in verbose or dry-run output, or adding a `DryRunInfo` or `Result` field (OS3). Rejected by the orchestrator.
- Passing the Options values to sub-agents (OS8). Deferred; milestone D11 says sub-agent plumbing is deferred.
- Making `Options.BaseURL` a Bedrock endpoint override (milestone `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:96`). Out of scope.
- Changing `internal/provider/registry.go` or `cmd/gc.go` (OS6).

### Constraints and assumptions

- **Constraint, compatibility.** Additive exported fields only. Existing keyed literals compile unchanged (`COMPATIBILITY.md:9`). The zero value keeps today's behavior (`COMPATIBILITY.md:33-34`).
- **Constraint, secrets.** `AGENTS.md` says "When in doubt, print nothing" (`AGENTS.md:29`). The key is a credential; it must not reach any output (OS3).
- **Constraint, provider scope.** The override applies to every provider except bedrock, including `openrouter` (the branch at `pkg/runner/run.go:404-405` receives the overridden values).
- **Assumption, test isolation.** Tests 2, 3, 6, and 7 use `t.Setenv` and `setupTestEnv`, so they cannot run in parallel. The parallel test (test 4) does not call `t.Setenv`, so it reads the real process environment and the real user `config.toml` (`pkg/runner/run.go:144`). Options take precedence over both, so the asserted header is deterministic. This is a recorded limitation (Limitations).
- **Assumption, Anthropic as the mock provider.** Tests use `anthropic/claude-sonnet-4-20250514` with `testutil.AnthropicResponse` and `AnthropicToolUseResponse`, as `TestRun_MaxTurnsConfigured` does (`pkg/runner/run_test.go:1351`).

## Section 2: Requirements

Requirements R1-R3 are the fields and their documentation. R4-R5 are the apply point and bedrock boundary. R6 is output safety. R7 is compatibility and CHANGELOG. Acceptance tests are in R8.

### R1: Public fields

- `pkg/runner` `Options` gains two fields, `APIKey string` and `BaseURL string` (OS7).
- Verification: `go build ./...` exits 0. `rg -n "APIKey string|BaseURL string" pkg/runner/options.go` shows both fields.

### R2: Field documentation

- The `APIKey` doc comment states that a non-empty value is used instead of the `ResolveAPIKey` lookup, that the precedence is `Options > env var > config.toml`, that an empty value means unset, that the value is never printed, and includes the sentence `Ignored when the provider is bedrock.` (OS4).
- The `BaseURL` doc comment states the same for `ResolveBaseURL` with the `AXE_<PROVIDER>_BASE_URL` env var, and includes the same bedrock sentence (OS4).
- Verification: `rg -n "Ignored when the provider is bedrock." pkg/runner/options.go` shows two matches.

### R3: Empty means unset

- An empty `APIKey` or `BaseURL` leaves the lookup result unchanged (OS2). No format or URL validation is performed, and no new `ConfigError` is raised for either field.
- Verification: acceptance test 3 (R8).

### R4: Apply point and precedence

- Non-empty `Options.APIKey` overwrites `apiKey` at `pkg/runner/run.go:385` and non-empty `Options.BaseURL` overwrites `baseURL` at `:386` (OS1). Precedence is Options, then env var, then `config.toml`.
- The override applies to the run's own provider only. Sub-agents are unchanged (OS8).
- Verification: acceptance tests 1 and 2 (R8).

### R5: Bedrock boundary

- For a bedrock model, `Options.APIKey` and `Options.BaseURL` are ignored. The bedrock block (`pkg/runner/run.go:389-395`) stays byte-for-byte unchanged. `newProvider` receives the region as its key argument and `""` as base URL (milestone acceptance 6).
- With `Options.APIKey` set and no region source, the run returns a `ConfigError` (`IsConfigError`) and does not call `newProvider` (milestone acceptance 7).
- Verification: `git diff 08d0f26 -- pkg/runner/run.go` shows no removed line, and the bedrock block is unchanged. Acceptance tests 6 and 7 (R8).

### R6: Output safety

- Neither `Options.APIKey` nor `Options.BaseURL` appears in verbose output, stdout, stderr, returned errors, or dry-run output (OS3). `DryRunInfo`, `Result`, and `--json` are unchanged.
- Verification: acceptance test 5 (R8).

### R7: Compatibility and CHANGELOG

- The public API change is additive only. `make apidiff` exits 0 with no incompatible-change report (OS7).
- `CHANGELOG.md` `[Unreleased]` `### Added` gets one bullet (OS6). The bullet states: `Options.APIKey` and `Options.BaseURL` in `pkg/runner` override the provider key and base URL for one run; precedence is Options, then env var, then `config.toml`; an empty value means unset; neither value is printed; both are ignored for `bedrock`.
- CLI, TOML, `--json`, `cmd/`, `internal/`, and golden files are unchanged.
- Verification: `rg -n "Options.APIKey" CHANGELOG.md` shows one bullet under `[Unreleased]`. `git diff --name-only 08d0f26 -- cmd internal` prints no paths.

### R8: Acceptance tests

Test file: `pkg/runner/credentials_option_test.go` (new) (OS5). Tests are named with the prefix `TestCredentialsOption_`.

1. **`TestCredentialsOption_LiveRunUsesOptions`.** Not parallel. Agent TOML at the `setupTestEnv` directory with `model = "anthropic/claude-sonnet-4-20250514"` and no tools. `setupTestEnv` is called with an empty `configTOML`, so no `config.toml` is written. `t.Setenv("ANTHROPIC_API_KEY", "")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "")` clear the env sources. `Options.APIKey` is a distinctive fake key and `Options.BaseURL` is `mock.URL()`. The mock queues one `AnthropicResponse`. Assert `Run` succeeds, `mock.RequestCount() == 1`, and the recorded request header `x-api-key` (`mock.Requests[0].Headers`) equals the Options key (R4).
2. **`TestCredentialsOption_OptionsWinOverEnv`.** Not parallel. `setupTestEnv` with `config.toml` containing `[providers.anthropic] api_key = "config-key"` and `base_url = "http://localhost:1"`. `t.Setenv("ANTHROPIC_API_KEY", "env-key")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "http://localhost:1")` set env sources that differ from Options. `Options.APIKey` and `Options.BaseURL` (`mock.URL()`) are set. The mock queues one `AnthropicResponse`. Assert success, `RequestCount == 1`, and the `x-api-key` header equals the Options key, not `env-key` or `config-key` (R4).
3. **`TestCredentialsOption_UnsetUsesLookup`.** Not parallel. Both Options fields are `""`. `setupTestEnv` with `config.toml` `[providers.anthropic] api_key = "config-key"`. `t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())` supplies the base URL from env. The mock queues one `AnthropicResponse`. Assert success, `RequestCount == 1`, and `x-api-key == "config-key"` (R3, the current lookup still reaches the mock).
4. **`TestCredentialsOption_ParallelRun`.** Parallel (`t.Parallel()`). No `t.Setenv`. The agent TOML is written to a `t.TempDir()` and passed as `Options.AgentsDirs` (`pkg/runner/run.go:85-86`). `Options.APIKey` and `Options.BaseURL = mock.URL()` are set. The mock queues one `AnthropicResponse`. Assert success, `RequestCount == 1`, and `x-api-key` equals the Options key (milestone acceptance 4). Parallel runs in the same package use separate mock servers.
5. **`TestCredentialsOption_NotPrinted`.** Not parallel. Three sub-runs, each with `Verbose: true` and separate `bytes.Buffer` stdout and stderr. Each sub-run first calls `t.Setenv("ANTHROPIC_API_KEY", "")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "")`, so the host environment cannot supply a key or base URL. All use distinctive values: the key `sk-options-secret-probe` and the base URL `http://options-base-url-probe:9`. Sub-run `live` is a successful live run: `Options.APIKey` is the probe key, `Options.BaseURL` is `mock.URL()` with one `AnthropicResponse` queued, and `Verbose: true`. It asserts that the captured `stdout` and `stderr` do not contain the probe key or the mock URL. Sub-run `live-fail` is a failing live run: `Options.APIKey` is the probe key, `Options.BaseURL` is `http://options-base-url-probe:9` (no mock; the request fails at transport), and `Verbose: true`. It asserts that the captured `stdout` and `stderr` contain neither the probe key nor the base URL probe, and, after a `t.Fatal` if `err == nil`, that `err.Error()` does not contain the probe key. It does not assert on `err.Error()` for the base URL, because Go HTTP transport errors already include the request URL (Limitations). Sub-run `dry` sets `Options.APIKey` to the probe key and `Options.BaseURL` to the base URL probe with `DryRun: true` and no mock, and asserts that the captured output does not contain either probe value. Assert neither value appears in captured output in any sub-run (R6).
6. **`TestCredentialsOption_BedrockIgnoresOptions`.** Not parallel. Model `bedrock/claude-test-model`. `Options.APIKey = "fake"`, `Options.BaseURL = "http://localhost:1"`. `t.Setenv("AWS_REGION", "us-west-2")`. `setupTestEnv` with no `config.toml`. The `run()` seam (`pkg/runner/run.go:65`) receives a closure that records `(name, key, baseURL)` and returns a sentinel error, so the run stops before any request. Assert the captured key is `"us-west-2"`, the captured base URL is `""`, and the error is a `RuntimeError` from `failed to create provider` (`pkg/runner/run.go:410`) (R5).
7. **`TestCredentialsOption_BedrockNoRegionIsConfigError`.** Not parallel. Same model. `Options.APIKey = "fake"`. `t.Setenv` sets `AWS_REGION`, `AWS_DEFAULT_REGION`, and `AXE_BEDROCK_REGION` to `""`. `setupTestEnv` with no `config.toml`, so the region has no config source (`internal/config/config.go:117-140`). The `run()` seam fails the test if called. Assert `IsConfigError(err)` and that `err.Error()` contains `region for provider "bedrock" is not configured` (the message at `pkg/runner/run.go:391`, R5). Assert the `newProvider` seam is not called. CLI exit-2 mapping is covered by the existing ConfigError mapping (`pkg/runner/errors.go:9-10`) and is not re-tested here.

Focused run: the `TestCredentialsOption_` test-name family in package `pkg/runner`, with the race detector. It is not in the fenced block because it selects no tests until the file exists.

## Parallelizable work

- The two field declarations and their doc comments (R1, R2) and the CHANGELOG bullet (R7) are independent of each other.
- The apply-point override (R4) is one edit in `pkg/runner/run.go`. It is independent of the CHANGELOG bullet.
- Tests 1-7 share one new file and are written after the wiring.

## Serial dependencies

- R1 before R3-R5 (the fields must exist before the override compiles).
- R4 and R5 before R8 (tests exercise the wiring).
- R6 verification (test 5) after R4.

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                          # expect: exit 0
go vet ./pkg/runner/                                                     # expect: exit 0
gofmt -l pkg/runner/                                                     # expect: no output
go test -race -count=1 ./pkg/runner ./internal/agent ./internal/config  # expect: exit 0
XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd                      # expect: exit 0 (see Limitations)
make apidiff                                                             # expect: exit 0, no incompatible-change output
rg -n "APIKey string|BaseURL string" pkg/runner/options.go              # expect: the two new fields
rg -n "Ignored when the provider is bedrock." pkg/runner/options.go     # expect: two matches
rg -n "Options.APIKey" CHANGELOG.md                                     # expect: one bullet under [Unreleased]
git diff --name-only 08d0f26 -- . ':(exclude)docs/plans/'               # expect: CHANGELOG.md, pkg/runner/options.go, pkg/runner/run.go, pkg/runner/credentials_option_test.go
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since 08d0f26 /home/jaron/dev/axe/runner-tool-hooks                                              # expect: exit 0
```

Focused run: see R8.

## Failure modes

1. **The key becomes the region for bedrock.** Mitigation: OS1 (no `if apiKey == ""` guard), R5, test 6.
2. **An Options value leaks into output.** Mitigation: OS3, R6, test 5.
3. **Options precedence is reversed, so env or config wins.** Mitigation: R4, test 2.
4. **An empty Options value overrides a working lookup.** Mitigation: OS2, R3, test 3.
5. **Parallel test depends on shared state.** Mitigation: test 4 uses no `t.Setenv` and per-test mock and temp directory (R8).
6. **Bedrock with Options and no region reaches the provider.** Mitigation: R5, test 7.

## Limitations

- Sub-agents do not inherit `Options.APIKey` or `Options.BaseURL`. Their credentials still resolve from env and config (`internal/tool/tool.go:224-225`) (OS8).
- Redacting the base URL from returned transport errors is out of scope. Go HTTP transport errors include the request URL (`internal/provider/anthropic.go:275`); test 5 asserts only that `err.Error()` does not contain the API key probe.
- Bedrock ignores both fields by design. Region and AWS credentials are still read from env and config (`internal/config/config.go:117-140`, `internal/provider/awscreds.go:19-45`) (OS1, R5).
- Test 4 reads the real process environment and the real user `config.toml` (`pkg/runner/run.go:144`), because it does not call `t.Setenv`. Options precedence makes the asserted header deterministic.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005-008.
- The exit-code 2 mapping is a CLI concern (`pkg/runner/errors.go:9-10`). Test 7 asserts `IsConfigError`; the CLI mapping is unchanged and is not tested here.
- Milestone entry 009 and the design note cite stale lines. The corrected lines are in "Citation corrections (verified at `08d0f26`)". Neither document is edited by this spec.
