# Implement Milestone 009: Set the provider API key and base URL from code with `Options.APIKey` and `Options.BaseURL`

## Section 1: Context Summary

Milestone 009 of the v1.12.0 runner tool-hooks release lets a host set the provider API key and base URL for one run, in code. Two exported `string` fields are added to `Options` (`pkg/runner/options.go`), after `MaxTurns` and before the I/O block. A non-empty `Options.APIKey` overrides the `ResolveAPIKey` lookup, and a non-empty `Options.BaseURL` overrides the `ResolveBaseURL` lookup. The precedence is Options, then env var, then `config.toml` (OS1, R4). The override is two `if` statements inserted after `apiKey := globalCfg.ResolveAPIKey(provName)` and `baseURL := globalCfg.ResolveBaseURL(provName)` (`pkg/runner/run.go:385-386`) and before the bedrock block (`pkg/runner/run.go:389-395`). The bedrock block is not edited. Bedrock therefore ignores both Options values by construction: it overwrites `apiKey` with the region and clears `baseURL` (R5). No `if apiKey == ""` guard is added, because that would turn the key into the region (OS1). An empty string means unset; there is no validation and no new `ConfigError` (OS2, R3). Neither value is printed anywhere: no verbose line, no output, no `DryRunInfo` or `Result` field, no `--json` change (OS3, R6). Sub-agents do not inherit the values; their credentials still come from env and config (OS8). The change is additive, so `make apidiff` must exit 0 (OS7, R7). One `[Unreleased]` `Added` bullet is the only `CHANGELOG.md` change (OS6).

Source: `docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md` (Sections 1 and 2, R1-R8, OS1-OS8). Milestone entry 009 is `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:238-250`. Its line citations are stale (the apply point at lines 336-337, the bedrock block at lines 340-343, the seam at lines 47 and 358, the credentials at lines 17-44, the registry at lines 72-75); this checklist uses the spec's corrected citations, verified against the code at `c2e20a2` (the code is unchanged since `08d0f26`; the commits after it change only `docs/plans/`). Predecessor: none (`Depends on: none`). Milestone 008 (`Options.MaxTurns`) is the precedent for the field, doc comment, and CHANGELOG pattern.

Commit is not authorized in this checklist. Do not push. Do not edit records: the spec, the checkpoint (`docs/plans/052_runner_tool_hooks_v1_12_0_009_checkpoint.md`), and the milestones document are orchestrator records. Tasks are committed one at a time by the orchestrator. In-task checks read the working tree; the scope check in T8 reads the committed range `08d0f26..HEAD`.

Scope of changed files (R1-R8, OS6): `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/credentials_option_test.go` (new), `CHANGELOG.md`. Nothing else. `internal/`, `cmd/`, `go.mod`, `go.sum`, and `internal/testutil/` are not changed (OS8, R6, R7). Every code change is in `./pkg/runner`, so the derived race package list is `./pkg/runner` only.

Sandbox rules: `go test ./cmd` needs `XDG_CACHE_HOME=$(mktemp -d)`. Tests that call `t.Setenv` or `setupTestEnv` cannot use `t.Parallel` (OS5). Test 4 is the only parallel test and calls no `t.Setenv`. Pre-existing gofmt drift in `pkg/runner/result.go` and `pkg/runner/result_test.go` is out of scope; gofmt checks target only touched files.

Decisions this checklist records (not open; none changes scope or behavior beyond the spec):
- **A1. Test classification.** Spec tests 1, 2, 4, and 5 fail at HEAD and are red. Spec tests 3, 6, and 7 describe behavior HEAD already has (unset uses the lookup; bedrock already ignores the Options values; bedrock with no region is already a `ConfigError`), so they are marked `[guard]` and must PASS at HEAD. They are regression guards for the new wiring.
- **A2. Test 5 live sub-run asserts success.** The spec defines the `live` sub-run as a successful live run. The checklist makes that explicit (`require err == nil`). At HEAD this assertion fails, so test 5 is red for the reason the spec describes: the probe key is not sent, and the run fails before output. The leak assertions run after this check.
- **A3. Test-file layout.** The file `pkg/runner/credentials_option_test.go` is created in T2.1 and is extended by T2.2-T2.5. Each red task adds one test; each guard task adds its own tests. Test names use the spec's prefix `TestCredentialsOption_`.
- **A4. Cite, do not guess.** Every line number below was read from the code at `c2e20a2`. Where a spec citation is off by a line, the checklist uses the verified line and records it in Assumptions.

---

## Section 2: Implementation Checklist

### Per-task gate derivation

Every code task states the gate rule verbatim. The derivation is run before each commit from the module root:

> Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it.

```sh
changed=$({ git diff --name-only --relative HEAD; git ls-files --others --exclude-standard; } | grep '\.go$' | xargs -r -n1 dirname | sort -u)
pkgs=$(go list -e -f '{{.Dir}}' ./... | while read -r d; do r=$(realpath --relative-to=. "$d"); printf '%s\n' "$changed" | grep -qxF "$r" && echo "./$r"; done)
if [ -n "$pkgs" ]; then go test -race $pkgs; fi
```

### PASS-line filter convention

`go test -v` appends a duration to each result line, so every PASS-line filter in this checklist ends the test name with `( |$)` and never anchors `$` directly after the name. Filter form: `rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)'`. Sub-run names (`TestCredentialsOption_NotPrinted/live`) match the `[^ ]*` part. New test names do not exist until T2.1, so `-run` is not used for them; red and green steps run the package verbosely.

### Phase 0: Pre-flight (serial, read-only)

- [x] **T0: Pre-flight: HEAD ancestry from the base commit, tree state, baseline tests, declared names absent**
  - Budget: 30 min
  - Owner: code (verification)
  - Files: none (read-only).
  - Commands and expected results (repository root):
    - `git merge-base --is-ancestor 08d0f26c1ca5f076725ba21ce09a83ac89c815b7 HEAD; echo "exit=$?"` → `exit=0`. Ancestry is checked from the base commit, not an exact HEAD. Verified at writing time (HEAD `c2e20a2`).
    - `git diff --name-only 08d0f26c1ca5f076725ba21ce09a83ac89c815b7 HEAD -- . ':(exclude)docs/plans/'` → no output. Verified at writing time; code is unchanged since the base.
    - `git status --porcelain` → any modified or untracked path must be a 009 plan file (`docs/plans/052_runner_tool_hooks_v1_12_0_009_*.md`). Verified at writing time: ` M docs/plans/052_runner_tool_hooks_v1_12_0_009_checkpoint.md`, an orchestrator record that this writer does not touch. Any other path: stop and report.
    - `rg -n 'TestCredentialsOption_' --glob '*.go' .; echo "exit=$?"` → `exit=1`. Verified at writing time (names absent).
    - `test -e pkg/runner/credentials_option_test.go; echo "exit=$?"` → `exit=1`. Verified at writing time (file absent).
    - `rg -n "APIKey string|BaseURL string" pkg/runner/options.go; echo "exit=$?"` → `exit=1`. Verified at writing time (fields absent).
    - `rg -n "Tools \[\]string" pkg/runner/options.go` → `pkg/runner/options.go:61`. Verified at writing time. The 009 fields go after `MaxTurns` (`pkg/runner/options.go:67`).
    - `rg -n "apiKey := globalCfg.ResolveAPIKey|baseURL := globalCfg.ResolveBaseURL|if provName == \"bedrock\"" pkg/runner/run.go` → `pkg/runner/run.go:385`, `pkg/runner/run.go:386`, `pkg/runner/run.go:389`. Verified at writing time.
    - `rg -n "^func Run\(|^func run\(" pkg/runner/run.go` → `pkg/runner/run.go:48`, `pkg/runner/run.go:65`. Verified at writing time.
    - `rg -n "if opts.DryRun \{" pkg/runner/run.go` → `pkg/runner/run.go:328`. Verified at writing time.
    - `rg -n "^type DryRunInfo struct" pkg/runner/result.go` → `pkg/runner/result.go:30`. Verified at writing time.
    - `rg -n "^type ConfigError struct|^func IsConfigError" pkg/runner/errors.go` → `pkg/runner/errors.go:11`, `pkg/runner/errors.go:57`. Verified at writing time.
    - `rg -n "^func setupTestEnv|^func newMockStdoutStderr|^func TestRun_MaxTurnsConfigured" pkg/runner/run_test.go` → `pkg/runner/run_test.go:24`, `pkg/runner/run_test.go:44`, `pkg/runner/run_test.go:1336`. Verified at writing time.
    - `rg -n "^func NewMockLLMServer|^func \(m \*MockLLMServer\) (URL|RequestCount)|^func AnthropicResponse\(|^func AnthropicToolUseResponse\(" internal/testutil/mockserver.go` → `internal/testutil/mockserver.go:50`, `:122`, `:127`, `:141`, `:154`. Verified at writing time.
    - `go test -list '^(TestRun_MaxTurnsExceeded|TestRun_MaxTurnsConfigured|TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)$' ./pkg/runner` → lists all five names, then `ok`. Run in T0, not at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent` → `ok` for both. Verified at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -list '^(TestRun_NoSubAgents_NoTools|TestRunCmd_DryRun_WithMCPServers)$' ./cmd` → lists both names, then `ok`. Verified at writing time.
    - `git diff --exit-code 08d0f26c1ca5f076725ba21ce09a83ac89c815b7 -- internal/agent internal/tool internal/mcpclient cmd internal/testutil go.mod go.sum; echo "exit=$?"` → `exit=0`. Verified at writing time.
    - `gofmt -l pkg/runner internal/agent` → only `pkg/runner/result.go` and `pkg/runner/result_test.go` (pre-existing drift, out of scope). Verified at writing time.
    - `git rev-parse --verify 'v1.11.0^{commit}'` → `20e6ada179b08cef050c4b436a2497182efcf2ee`. Verified at writing time (apidiff baseline, `Makefile:4`, `Makefile:15`).
    - `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` → `PASS: Go pre-flight checks.`, `exit=0`. Verified at writing time (go1.26.10; govulncheck clean). Recorded because T7 runs `go run` on a pinned module (`Makefile:3`, `Makefile:22-23`).
    - `command -v golangci-lint` → `/home/jaron/.cache/pi-go-bin/golangci-lint`. Verified at writing time.
    - `test -e .agents/scripts/checklist-test-gate-audit; echo "exit=$?"` → `exit=1` (script absent; not run). Verified at writing time.
    - `for v in ANTHROPIC_API_KEY AXE_ANTHROPIC_BASE_URL AWS_REGION AWS_DEFAULT_REGION AXE_BEDROCK_REGION; do if [ -z "${!v+x}" ]; then echo "$v unset"; elif [ -z "${!v}" ]; then echo "$v empty"; else echo "$v set"; fi; done` → one line per name, each `unset`, `empty`, or `set`. Values are never printed. Record only these words in T0. Emptiness is not a precondition: tests 1, 2, 3, 5, 6, and 7 shadow or clear their env sources with `t.Setenv`, and a non-empty `Options` value takes precedence over env and `config.toml` (OS1), so any host value gives the same asserted results. Test 4 reads the host environment as-is; its result is independent of it once `Options` is applied (see Limitations).
    - `rg -n "^\[providers\.anthropic\]" ~/.config/axe/config.toml` → no match outside comments. Verified at writing time (the file has only commented example blocks). Record in T0. Test 4 reads the real user `config.toml` (`pkg/runner/run.go:144`); an active Anthropic `api_key` or `base_url` there would not change the asserted header, because Options wins.
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: this task changes no files. The derived package list is empty before any edit.

### Phase 1: Enabling surface (serial)

- [x] **T1: Public fields and doc contract (R1, R2)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T0.
  - Enables: T2.1-T2.5 (test code must compile against `Options.APIKey` and `Options.BaseURL`) and T3.1 (the apply point).
  - Files:
    - `pkg/runner/options.go`: add two fields to `type Options struct` (`pkg/runner/options.go:16`) after the `MaxTurns int` field (`pkg/runner/options.go:67`) and before the `// I/O` comment (`pkg/runner/options.go:69`), each with a doc comment above it:
      - `APIKey string`
      - `BaseURL string`
  - Doc comment content (R2, OS2, OS3, OS4):
    - `APIKey`: a non-empty value is used instead of the `ResolveAPIKey` lookup; precedence is `Options > env var > config.toml`; an empty value means unset; the value is never printed; the sentence `Ignored when the provider is bedrock.` appears in the comment.
    - `BaseURL`: a non-empty value is used instead of the `ResolveBaseURL` lookup (`AXE_<PROVIDER>_BASE_URL` env var, then `config.toml`); precedence is `Options > env var > config.toml`; an empty value means unset; the value is never printed; the sentence `Ignored when the provider is bedrock.` appears in the comment.
    - Both comments state that sub-agents do not inherit the value (OS8).
  - Do not add behavior. `Run`, `run`, the apply point, and the bedrock block are unchanged in this task. The fields are declared but unused until T3.1, which is the expected state for a declaration task.
  - Failing-first: not applicable (declaration only).
  - Verification:
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner` → `ok` (no behavior changed).
    - `rg -n "APIKey string|BaseURL string" pkg/runner/options.go` → two matches (R1).
    - `rg -n "Ignored when the provider is bedrock." pkg/runner/options.go` → two matches (R2).
    - `rg -n "Options > env var > config.toml|never printed|sub-agents do not inherit" pkg/runner/options.go` → each claim present on both fields (R2).
    - `gofmt -l pkg/runner/options.go` → no output.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 2: Red tests (serial, test-only)

Red-run rule: a red test is "red" only if it compiles and fails on an assertion (or on the mock's unexpected-request fatal at `internal/testutil/mockserver.go:77`, which is a failure of the missing behavior). T1 adds the identifiers the tests use, so every test compiles. Guard tests are marked `[guard]` and must pass at HEAD.

Shared test conventions (file `pkg/runner/credentials_option_test.go`, new, created in T2.1): package `runner`; mirror `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336`): `setupTestEnv(t, configTOML, agentName, agentTOML)` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr()` (`pkg/runner/run_test.go:44`), `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), and `t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())` where the spec calls for an env base URL. The agent model is `anthropic/claude-sonnet-4-20250514`, and responses are `testutil.AnthropicResponse` (`internal/testutil/mockserver.go:141`) and `testutil.AnthropicToolUseResponse` (`internal/testutil/mockserver.go:154`). Recorded requests are `mock.Requests[i].Headers` (`internal/testutil/mockserver.go:26`, `:34`) and `.Body` (`:27`). Imports are added as each test needs them (for example `bytes`, `context`, `errors`, `os`, `path/filepath`, `strings`, `testing`, `github.com/jrswab/axe/internal/provider`, `github.com/jrswab/axe/internal/testutil`). Test 4 is the only `t.Parallel` test in the file, and it calls no `t.Setenv`.

- [ ] **T2.1: Red: acceptance test 1, live run uses Options (R4, R8.1)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T1.
  - File: `pkg/runner/credentials_option_test.go` (new; created in this task).
  - Test: `TestCredentialsOption_LiveRunUsesOptions`. Not parallel. `setupTestEnv(t, "", "test-agent", agentTOML)` with `agentTOML` = `name = "test-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, no tools (so `setupTestEnv` writes no `config.toml`). `t.Setenv("ANTHROPIC_API_KEY", "")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "")` clear the env sources. Mock: `testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{testutil.AnthropicResponse("ok")})`. `Options{AgentName: "test-agent", Stdout: stdout, Stderr: stderr, APIKey: "sk-options-key-1", BaseURL: mock.URL()}`. Assert: `err == nil`; `mock.RequestCount() == 1`; `mock.Requests[0].Headers.Get("x-api-key") == "sk-options-key-1"` (R4).
  - Red step: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner` shows `--- FAIL: TestCredentialsOption_LiveRunUsesOptions`. Expected reason at HEAD: the Options fields are not read, so `apiKey` is `""`, and the empty-key check at `pkg/runner/run.go:397-400` returns a `ConfigError` (`API key for provider "anthropic" is not configured`) before any request. The assertion `err == nil` fails. Record the actual failing assertion.
  - Filter: `rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)'`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above; the race run must be green from T3.3.

- [ ] **T2.2: Red: acceptance test 2, Options win over env and config (R4, R8.2)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.1.
  - File: `pkg/runner/credentials_option_test.go`.
  - Test: `TestCredentialsOption_OptionsWinOverEnv`. Not parallel. `setupTestEnv` with `configTOML` = `[providers.anthropic]` / `api_key = "config-key"` / `base_url = "http://localhost:1"`, and the agent TOML from T2.1. `t.Setenv("ANTHROPIC_API_KEY", "env-key")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "http://localhost:1")`. Mock: one `AnthropicResponse`. `Options{AgentName: "test-agent", Stdout, Stderr, APIKey: "sk-options-key-2", BaseURL: mock.URL()}`. Assert: `err == nil`; `mock.RequestCount() == 1`; `x-api-key` equals `"sk-options-key-2"`, not `"env-key"` or `"config-key"` (R4).
  - Red step: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner` shows `--- FAIL: TestCredentialsOption_OptionsWinOverEnv`. Expected reason at HEAD: `apiKey` resolves to `env-key` and `baseURL` to `http://localhost:1`. The request goes to port 1 and fails at transport, so `err == nil` fails. Record the actual failing assertion.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.3: Red: acceptance test 4, parallel run with no `t.Setenv` (R4, R8.4)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.2.
  - File: `pkg/runner/credentials_option_test.go`.
  - Test: `TestCredentialsOption_ParallelRun`. `t.Parallel()`. No `t.Setenv`, and no `setupTestEnv`. The agent TOML (same as T2.1) is written to `t.TempDir()` (`agentDir`) as `test-agent.toml`, and `agentDir` is passed as `Options.AgentsDirs` (`pkg/runner/run.go:85-86`). Mock: its own `testutil.NewMockLLMServer` with one `AnthropicResponse`. `Options{AgentName: "test-agent", AgentsDirs: []string{agentDir}, Stdout, Stderr, APIKey: "sk-options-key-4", BaseURL: mock.URL()}`. Assert: `err == nil`; `mock.RequestCount() == 1`; `x-api-key` equals `"sk-options-key-4"` (R4, milestone acceptance 4). The test reads the real environment and the real user `config.toml`; Options precedence makes the asserted header deterministic (Limitations).
  - Red step: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner` shows `--- FAIL: TestCredentialsOption_ParallelRun`. The prefix clears the two env sources, so HEAD never sends a request to the real API. Expected reason at HEAD: the Options fields are not read, the user `config.toml` has no active key (T0), and the empty-key check at `pkg/runner/run.go:397-400` returns a `ConfigError` before any request, so `err == nil` fails. Record the actual failing assertion.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.4: Red: acceptance test 5, neither value printed (R6, R8.5)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.3.
  - File: `pkg/runner/credentials_option_test.go`.
  - Test: `TestCredentialsOption_NotPrinted`. Not parallel. Probe values: key `sk-options-secret-probe`; base URL `http://options-base-url-probe:9`. Each sub-run is a `t.Run` with `Verbose: true`, separate `bytes.Buffer` stdout and stderr (`newMockStdoutStderr()` at `pkg/runner/run_test.go:44`), and `setupTestEnv` with the agent TOML from T2.1 and no `config.toml`. Each sub-run first calls `t.Setenv("ANTHROPIC_API_KEY", "")` and `t.Setenv("AXE_ANTHROPIC_BASE_URL", "")`, so the host environment cannot supply a key or base URL; the `Options` values are the only source.
    - Sub-run `live`: `Options.APIKey` is the key probe; `Options.BaseURL` is `mock.URL()` with one `AnthropicResponse` queued. Assert `err == nil` (A2). Assert that the captured `stdout` and `stderr` do not contain the key probe or `mock.URL()` (R6).
    - Sub-run `live-fail`: `Options.APIKey` is the key probe; `Options.BaseURL` is the base URL probe (no mock; the request fails at transport). Assert that the captured `stdout` and `stderr` contain neither the key probe nor the base URL probe. Call `t.Fatal` if `err == nil`, before any `err.Error()` call. Assert that `err.Error()` does not contain the key probe. Do not assert on `err.Error()` for the base URL (Limitations).
    - Sub-run `dry`: `Options.APIKey` is the key probe, `Options.BaseURL` is the base URL probe, `DryRun: true`, no mock. Assert that the captured `stdout` and `stderr` contain neither probe value.
  - Red step: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner` shows `--- FAIL: TestCredentialsOption_NotPrinted/live`. Expected reason at HEAD: the `live` sub-run returns the empty-key `ConfigError`, so the `err == nil` assertion fails. The `live-fail` and `dry` sub-runs pass at HEAD trivially (no Options value is read), and they become meaningful after T3.1. Record the actual failing assertion for each sub-run.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.5: Guards: acceptance tests 3, 6, and 7 (R3, R5, R8.3, R8.6, R8.7)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.4.
  - File: `pkg/runner/credentials_option_test.go`.
  - Tests (all `[guard]`, must PASS at HEAD):
    - `TestCredentialsOption_UnsetUsesLookup` (test 3). Not parallel. Both Options fields are `""`. `setupTestEnv` with `configTOML` = `[providers.anthropic]` / `api_key = "config-key"` and the agent TOML from T2.1. `t.Setenv("ANTHROPIC_API_KEY", "")` clears the env key so the config key is the only source (`internal/config/config.go:81-94`: env overrides config.toml). `t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())` supplies the base URL from env. Mock: one `AnthropicResponse`. Assert `err == nil`; `RequestCount() == 1`; `x-api-key == "config-key"` (R3).
    - `TestCredentialsOption_BedrockIgnoresOptions` (test 6). Not parallel. Model `bedrock/claude-test-model`. `Options.APIKey = "fake"`, `Options.BaseURL = "http://localhost:1"`. `t.Setenv("AWS_REGION", "us-west-2")`. `setupTestEnv` with no `config.toml`. The `run()` seam (`pkg/runner/run.go:65`) gets a closure with the signature `func(string, string, string) (provider.Provider, error)` that records `(name, key, baseURL)` and returns a sentinel error, so the run stops before any request. Assert the recorded key is `"us-west-2"`, the recorded base URL is `""`, and the error is a `RuntimeError` whose message contains `failed to create provider` (`pkg/runner/run.go:410`) (R5).
    - `TestCredentialsOption_BedrockNoRegionIsConfigError` (test 7). Not parallel. Same model. `Options.APIKey = "fake"`. `t.Setenv` sets `AWS_REGION`, `AWS_DEFAULT_REGION`, and `AXE_BEDROCK_REGION` to `""`. `setupTestEnv` with no `config.toml`, so the region has no config source (`internal/config/config.go:117-140`). The `run()` seam calls `t.Fatalf` if it is invoked. Assert `IsConfigError(err)` (`pkg/runner/errors.go:57`) and that `err.Error()` contains `region for provider "bedrock" is not configured` (`pkg/runner/run.go:391`) (R5).
  - Expected at HEAD: all three PASS (guards). Record the PASS lines.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race ./pkg/runner`. Expected FAIL set: only the T2.1-T2.4 tests.

- [ ] **T2.6: Red-run check (all acceptance tests)**
  - Budget: 15 min
  - Owner: code (verification)
  - Depends on: T2.5.
  - Files: none.
  - Command: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner > /tmp/009-t26-red.txt 2>&1; echo "exit=$?"`, then `rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)' /tmp/009-t26-red.txt`.
  - Expected: `exit=1` (red tests exist). The `rg` output shows `--- FAIL` for `TestCredentialsOption_LiveRunUsesOptions`, `TestCredentialsOption_OptionsWinOverEnv`, `TestCredentialsOption_ParallelRun`, and `TestCredentialsOption_NotPrinted` (and its `live` sub-run). It shows `--- PASS` for `TestCredentialsOption_UnsetUsesLookup`, `TestCredentialsOption_BedrockIgnoresOptions`, and `TestCredentialsOption_BedrockNoRegionIsConfigError`.
  - `rg -- '--- FAIL: Test' /tmp/009-t26-red.txt` lists only the red tests and their sub-runs. Any existing test in that output is a stop: report it.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the red run above is the gate. The derived package list is `./pkg/runner`, and the file is uncommitted until the orchestrator commits it.

### Phase 3: Green implementation (serial)

Order: T3.1 (apply point) → T3.2 (boundary verification; no code expected) → T3.3 (full green). T3.1 is the only code edit, and it edits `pkg/runner/run.go` only.

Expected FAIL set after each task (any other FAIL is a stop):
- After T3.1: none (all four red tests turn green).

- [ ] **T3.1: Green: apply `Options.APIKey` and `Options.BaseURL` after the lookup, before the bedrock block (R4, R6)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.6.
  - File: `pkg/runner/run.go` only.
  - Behavior: after `baseURL := globalCfg.ResolveBaseURL(provName)` (`pkg/runner/run.go:386`) and before the `// ` comment and `if provName == "bedrock" {` (`pkg/runner/run.go:389`), insert:
    ```go
    if opts.APIKey != "" {
    	apiKey = opts.APIKey
    }
    if opts.BaseURL != "" {
    	baseURL = opts.BaseURL
    }
    ```
    The bedrock block (`pkg/runner/run.go:389-395`) is not edited (OS1). The `if apiKey == ""` guard is not added around `apiKey = region` (OS1). No log line, error text, or `DryRunInfo` field is added (OS3). The sub-agent path, the registry, and `cmd/` are not edited (OS8).
  - Green target: `TestCredentialsOption_LiveRunUsesOptions`, `TestCredentialsOption_OptionsWinOverEnv`, `TestCredentialsOption_ParallelRun`, and `TestCredentialsOption_NotPrinted` (all three sub-runs) PASS.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)'` → every line `PASS`. `go test -race -v ./pkg/runner | rg -- '--- FAIL'` → no output.
    - `rg -n "opts.APIKey != \"\"|opts.BaseURL != \"\"" pkg/runner/run.go` → one match each, located after `baseURL := globalCfg.ResolveBaseURL` and before `if provName == "bedrock"` (compare relative positions with `rg -n` on both anchors, not pre-edit line numbers).
    - `rg -n "if apiKey == \"\"" pkg/runner/run.go; echo "exit=$?"` → `exit=1`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `gofmt -l pkg/runner/run.go` → no output for the touched file (drift in `result.go` and `result_test.go` is out of scope).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

- [ ] **T3.2: Verify the bedrock block, the boundary, and the output paths are unchanged (R5, R6, R7); no code expected**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.1.
  - Files: none expected. If a check fails, fix only the lines added by T3.1 in `pkg/runner/run.go`; do not change tests or other files.
  - Checks (working tree, in-task):
    - The bedrock block is byte-for-byte unchanged against the base. Brace-match the `if provName == "bedrock" {` block in the working tree and in the base (no fixed line window): `awk '/if provName == "bedrock" \{/{f=1;d=0} f{n=gsub(/\{/,"{"); m=gsub(/\}/,"}"); d+=n-m; print; if(d==0) exit}' pkg/runner/run.go > /tmp/009-bedrock-now.txt`, then `git show 08d0f26c1ca5f076725ba21ce09a83ac89c815b7:pkg/runner/run.go | awk '/if provName == "bedrock" \{/{f=1;d=0} f{n=gsub(/\{/,"{"); m=gsub(/\}/,"}"); d+=n-m; print; if(d==0) exit}' > /tmp/009-bedrock-base.txt`, then `diff /tmp/009-bedrock-base.txt /tmp/009-bedrock-now.txt; echo "exit=$?"` → `exit=0`.
    - `git diff 08d0f26c1ca5f076725ba21ce09a83ac89c815b7 -- pkg/runner/run.go | rg '^-' | rg -v '^---'` → no output (no removed line; additions only, including the MaxTurns lines from 008 if present in this range).
    - `rg -n "newProvider\(provName, apiKey, baseURL\)" pkg/runner/run.go` → one match (the bedrock block's key and base URL flow unchanged into `newProvider`).
    - `rg -n "Options.APIKey|opts.APIKey|Options.BaseURL|opts.BaseURL" pkg/runner/run.go` → exactly the two T3.1 lines; `executeToolCalls` (`pkg/runner/run.go:711`) does not receive the values (OS8).
    - `rg -n "APIKey|BaseURL" pkg/runner/result.go` → no match (no `Result` or `DryRunInfo` field; OS3). Also `rg -n "MaxTurns|APIKey|BaseURL" pkg/runner/result.go` → no match for the new fields.
    - `git diff --exit-code -- internal cmd` → `exit=0` (working tree; no edits outside `pkg/runner`).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files unless a fix was needed; if none, the derived package list is empty. If a fix to `pkg/runner/run.go` was needed, the package is `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.3: Green: all acceptance tests 1-7 pass (R3, R4, R5, R6, R8)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.2.
  - Files: none expected. Fix any failure in `pkg/runner/run.go` (the file that introduced it), not in the test.
  - Green targets: all seven `TestCredentialsOption_` tests PASS, including the `live`, `live-fail`, and `dry` sub-runs of `TestCredentialsOption_NotPrinted`.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)'` → every line `PASS`; `go test -race -v ./pkg/runner | rg -- '--- FAIL'` → no output.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/config` → `ok` for all three.
    - Env-independence: `ANTHROPIC_API_KEY=sk-dummy-not-a-key AXE_ANTHROPIC_BASE_URL=http://localhost:1 AWS_DEFAULT_REGION=us-east-1 AWS_REGION= AXE_BEDROCK_REGION= go test -race -count=1 -v ./pkg/runner 2>&1 | rg 'TestCredentialsOption_|^ok |^FAIL'` → package `ok`, and every `--- PASS: TestCredentialsOption_` line present for the seven tests, no `--- FAIL`. Passes with the host env set to a non-empty key and region, as the spec requires.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

### Phase 4: Verification (serial, after Phase 3)

- [ ] **T4: Regression verification; TOML, CLI, `--json`, and sub-agent paths unchanged (R3, R5, R7)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.3.
  - Files: none (verification only).
  - Commands:
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/config` → `ok` for all three.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd` → `ok`. Run with `XDG_CACHE_HOME` set; this includes `TestRun_NoSubAgents_NoTools` and `TestRunCmd_DryRun_WithMCPServers`.
    - `go test -count=1 -v ./pkg/runner | rg -- '--- (PASS|FAIL): (TestRun_MaxTurnsExceeded|TestRun_MaxTurnsConfigured|TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)( |$)'` → every line `PASS` (existing tests, verified in T0).
    - `go test -count=1 -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)'` → every line `PASS`. Record the list.
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `gofmt -l pkg/runner/options.go pkg/runner/run.go pkg/runner/credentials_option_test.go` → no output. Drift in `result.go` and `result_test.go` is out of scope and is not listed.
    - `git diff --exit-code -- internal cmd` → `exit=0` (working tree).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty. T5 runs the race run.

- [ ] **T5: Race run of the `TestCredentialsOption_` family and the package (R3, R4, R5, R6, R8)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T4.
  - Step 1: `go test -race -v ./pkg/runner > /tmp/009-t5-race.txt 2>&1; echo "exit=$?"`. Expected `exit=0`.
  - Step 2 (only after step 1 prints `exit=0`; inspect the saved file): `rg -- '--- (PASS|FAIL): TestCredentialsOption_[^ ]*( |$)' /tmp/009-t5-race.txt` → every line `PASS`; `rg -- 'DATA RACE' /tmp/009-t5-race.txt; echo "exit=$?"` → `exit=1`.
  - Pass condition: step 1 exit 0, every `TestCredentialsOption_` line `PASS`, and no `DATA RACE`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T6: `golangci-lint run` (lint gate)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0`, no findings. A timeout is not a pass; report it.
  - Any finding in `pkg/runner/options.go`, `pkg/runner/run.go`, or `pkg/runner/credentials_option_test.go` is a failure of the task that introduced it. Fix it in that task's file, then re-run T4 and T6.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R7)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m009-apidiff-out.txt 2>/tmp/axe-m009-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m009-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0` (`Makefile:4`, `Makefile:15`). `Options.APIKey` and `Options.BaseURL` are additions and must not fail it (`Makefile:14-24`). The tag resolves to `20e6ada` (T0).
  - Cleanup: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache. If it fails for network reasons, report it; do not change the pin (`Makefile:3`).
  - No file changes.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

### Phase 5: Documentation (CHANGELOG), then final scope

- [ ] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.APIKey` and `Options.BaseURL` (R7, OS6)**
  - Budget: 15 min
  - Owner: code
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]` (`CHANGELOG.md:8`) → `### Added` (`CHANGELOG.md:10`), append one bullet after the last existing bullet of that section, which is the 008 `Options.MaxTurns` bullet at `CHANGELOG.md:17`. The blank line that follows is `CHANGELOG.md:18`. Text (R7): "- `Options.APIKey` and `Options.BaseURL` in `pkg/runner` override the provider key and base URL for one run; precedence is Options, then env var, then `config.toml`; an empty value means unset; neither value is printed; both are ignored for `bedrock`." Form follows the bullets at `CHANGELOG.md:11-17`.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`; `rg -n "Options.APIKey" CHANGELOG.md` → one match (R7); `rg -n "bedrock" CHANGELOG.md` → the bedrock clause in that bullet.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. No Go file changes; the derived package list is empty.

- [ ] **T8: Scope, API, removed-line, and lint checks against the committed range `08d0f26..HEAD` (R1, R2, R6, R7)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T7.1.
  - Files: none expected.
  - Commands and expected results (committed range `08d0f26..HEAD`, after the orchestrator has committed the work):
    - `git status --porcelain` → no output except 009 plan files and orchestrator records. Any other path: report.
    - `git diff --name-only 08d0f26c1ca5f076725ba21ce09a83ac89c815b7..HEAD -- . ':(exclude)docs/plans/'` → exactly `CHANGELOG.md`, `pkg/runner/credentials_option_test.go`, `pkg/runner/options.go`, `pkg/runner/run.go`.
    - `git diff --exit-code 08d0f26c1ca5f076725ba21ce09a83ac89c815b7..HEAD -- internal cmd go.mod go.sum` → `exit=0` (R6, R7).
    - `rg -n "APIKey string|BaseURL string" pkg/runner/options.go` → two fields (R1).
    - `rg -n "Ignored when the provider is bedrock." pkg/runner/options.go` → two matches (R2).
    - `rg -n "Options.APIKey" CHANGELOG.md` → one bullet (R7).
    - `git diff 08d0f26c1ca5f076725ba21ce09a83ac89c815b7..HEAD -- pkg/runner/run.go | rg '^-' | rg -v '^---'` → no output (additions only in `run.go`). Any removed line is a failure; report it.
    - `git diff 08d0f26c1ca5f076725ba21ce09a83ac89c815b7..HEAD -- pkg/runner/options.go | rg '^-' | rg -v '^---'` → no output (additions only).
    - `git diff 08d0f26c1ca5f076725ba21ce09a83ac89c815b7..HEAD -- pkg/runner/run.go | rg -n 'opts.APIKey != ""|opts.BaseURL != ""'` → one match each.
  - No file changes.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T8.1: Line-number refresh of this checklist (checklist citations only)**
  - Budget: 15 min
  - Owner: code
  - Depends on: T8.
  - File: this checklist (`docs/plans/052_runner_tool_hooks_v1_12_0_009_implement.md`) only.
  - Reason: T3.1 adds lines to `pkg/runner/run.go`, which shifts every `run.go` citation after `pkg/runner/run.go:386`. The checklist's own `run.go` citations (the bedrock block, the `newProvider` call, the empty-key check, the `failed to create provider` error, and `DryRunInfo` sites) must match the committed file.
  - Procedure: run `rg -n` for each `run.go` anchor cited in this checklist (`if provName == "bedrock"`, `apiKey = region`, `baseURL = ""`, `if provider.Supported(provName)`, `newProvider(provName, apiKey, baseURL)`, `failed to create provider`, `executeToolCalls(`, `if opts.DryRun {`, `func run(`, `func Run(`). Replace each stale line number in this checklist with the new one. Change line numbers only; do not change text or task content.
  - Also refresh the `options.go` and `CHANGELOG.md` citations that changed after T1 and T7.1, using the same procedure.
  - Verification: each refreshed citation resolves: `rg -n` for each anchor returns the number written in the checklist. Spec citations are not changed here; if a spec citation is stale, stop and report to the orchestrator (the code agent does not edit the spec).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no Go files; the derived package list is empty.

- [ ] **T9: Spec and checklist lint, docs lint, and checklist audit (final gate)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T8.1.
  - Commands:
    - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md docs/plans/052_runner_tool_hooks_v1_12_0_009_implement.md; echo "exit=$?"` → `exit=0`.
    - `~/.agents/scripts/docs-lint --since 08d0f26c1ca5f076725ba21ce09a83ac89c815b7 /home/jaron/dev/axe/runner-tool-hooks; echo "exit=$?"` → `exit=0`.
    - `test -x .agents/scripts/checklist-test-gate-audit && .agents/scripts/checklist-test-gate-audit docs/plans/052_runner_tool_hooks_v1_12_0_009_implement.md; echo "exit=$?"`. The script is absent (T0), so this prints nothing and returns `exit=1`. Report that the audit was not run because the script is absent. Do not treat that as a pass.
  - No file changes unless a lint failure names a line in this checklist; fix only that line and re-run.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no Go files; the derived package list is empty.

- [ ] **T10: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Owner: orchestrator, after T9.
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_009_checkpoint.md` (phase, task states, gate evidence). The milestone entry `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:238-250` may receive a line-number-only citation refresh. The milestone checkbox `- [ ] 009` is rendered by factory-checkpoint at the Factory completion gate, after review, not in T10.
  - This writer does not touch these files.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 two exported `string` fields on `Options` | T1 (declare), T7 (apidiff), T8 (rg) |
| R2 doc comments, including the bedrock sentence | T1 (doc text), T8 (`rg` count of two), T9 (spec lint) |
| R3 empty means unset; no validation | T2.5 (test 3 guard), T4 (regression), T5 |
| R4 apply point and precedence (Options > env > config) | T2.1, T2.2, T2.3 (red), T3.1 (green), T3.3, T5 |
| R5 bedrock boundary (Options ignored; no-region ConfigError) | T2.5 (tests 6, 7 guards), T3.2 (bedrock block byte-compare), T5 |
| R6 output safety (no value printed) | T2.4 (red), T3.1, T3.2 (`result.go` and no removed line), T5 |
| R7 compatibility and CHANGELOG | T4 (`internal`/`cmd` unchanged), T7 (apidiff), T7.1 (CHANGELOG), T8 (scope) |
| R8 acceptance tests 1-7 | T2.1 (1), T2.2 (2), T2.3 (4), T2.4 (5), T2.5 (3, 6, 7), T3.3, T5 |

## Acceptance Test Mapping

| Acceptance test | Function | Class | Red task | Green task |
|---|---|---|---|---|
| 1 Options key and base URL reach the mock, no env | `TestCredentialsOption_LiveRunUsesOptions` | red | T2.1 | T3.1 |
| 2 Options win over env and config | `TestCredentialsOption_OptionsWinOverEnv` | red | T2.2 | T3.1 |
| 3 unset uses the current lookup | `TestCredentialsOption_UnsetUsesLookup` | `[guard]` (PASS at HEAD) | T2.5 | none (stays PASS) |
| 4 parallel run, no `t.Setenv` | `TestCredentialsOption_ParallelRun` | red | T2.3 | T3.1 |
| 5 neither value in verbose, stdout, stderr, errors, or dry run | `TestCredentialsOption_NotPrinted` (`live`, `live-fail`, `dry`) | red (`live`); the other two sub-runs pass trivially at HEAD and are meaningful after T3.1 | T2.4 | T3.1 |
| 6 bedrock ignores Options; `newProvider` gets region and `""` | `TestCredentialsOption_BedrockIgnoresOptions` | `[guard]` (PASS at HEAD) | T2.5 | none (stays PASS) |
| 7 bedrock with no region is `ConfigError`; no provider call | `TestCredentialsOption_BedrockNoRegionIsConfigError` | `[guard]` (PASS at HEAD) | T2.5 | none (stays PASS) |

The focused run is the `TestCredentialsOption_` family in `pkg/runner` (T2.6, T3.3, T5). `make apidiff` is T7. The race run is T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures: none. The change adds two fields to `Options`. No function signature changes.
  - `runner.Options` (`pkg/runner/options.go:16`): gains `APIKey` and `BaseURL`. The only caller outside `pkg/runner` is at `cmd/run.go:80`, a keyed literal (`runner.Options{ AgentName: ..., AgentsDirs: ..., ... }`); it compiles unchanged, and the CLI does not set the new fields (OS4). Probe: `rg -n "runner.Options\{" --glob '*.go' .` → `./cmd/run.go:80` only (verified at writing time). `pkg/runner` tests use keyed literals (`rg -n "Options\{" pkg/runner/*_test.go` with unkeyed-literal filter → no match, verified at writing time).
  - `Run` (`pkg/runner/run.go:48`) and `run` (`pkg/runner/run.go:65`): signatures unchanged. The `run` seam type `func(string, string, string) (provider.Provider, error)` is unchanged; test 6 uses it as is.
  - `resolveCredentials` (`internal/provider/awscreds.go:19`): unchanged (AWS credentials are not in scope).
  - `ResolveAPIKey` (`internal/config/config.go:81`), `ResolveBaseURL` (`internal/config/config.go:99`), `ResolveRegion` (`internal/config/config.go:117`): unchanged.
  - `executeToolCalls` (`pkg/runner/run.go:711` call site): signature unchanged; the Options values are not passed (OS8).
- Extreme acceptance inputs (expected results from the spec; evidence comes from the tests named):
  - Empty `APIKey` and empty `BaseURL` (`""`): unset; the lookup result is used. Covered by T2.5 (test 3). Expected: `x-api-key == "config-key"`, base URL from env.
  - Non-empty key with empty base URL: the key overrides, the base URL comes from the lookup. Not separately tested; T3.1's `if` structure makes the two independent. Expected per OS2 (independent `if` blocks). Recorded as an assumption (A5 below).
  - Whitespace-only key (`"   "`): non-empty, so it is used verbatim; no validation (OS2). No test; the expected behavior is the spec's and is recorded here as an assumption, not probed. A probe would need a live key that rejects it, which the mock does not model.
  - Very long key or base URL (for example 4096 characters): used verbatim; the spec sets no length limit and does not probe. Recorded as an assumption.
  - Bedrock with a non-empty `APIKey` and region present: the region is the key and the base URL is `""` (test 6, expected from R5).
  - Bedrock with a non-empty `APIKey` and no region: `ConfigError` before any provider call (test 7).
  - Dry run with a non-empty `APIKey` and `BaseURL`: no output contains either value (test 5 `dry` sub-run).
- `go install` and `GOPROXY=` commands: T7 runs `make apidiff`, which runs `go run` on a pinned module (`Makefile:3`, `Makefile:22-23`). The `go-toolchain-check` pre-flight is recorded in T0 (`exit=0`).
- `./.agents/scripts/checklist-test-gate-audit`: absent in this repository (T0, `exit=1`). Not run; T9 reports it as not run.

## Blockers

None. The spec is approved, and every citation in the spec and in this checklist matches the code at `c2e20a2` (code unchanged since `08d0f26`; later commits change only `docs/plans/`). Spec-lint passes with exit 0 on the spec (T0 verification, recorded in the writer's report). The checklist citations that change after T3.1 are refreshed in T8.1.

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T2.5 → T2.6 → T3.1 → T3.2 → T3.3 → T4 → T5 → T6 → T7 → T7.1 → T8 → T8.1 → T9 → T10. The red tests share one new file, and the green task edits `pkg/runner/run.go`.
- Parallel: T7.1 (CHANGELOG) touches no Go file and may be drafted during Phase 1 or 2, but it is committed in order after T7.
- Parallel: the T1 doc comments and the T7.1 bullet are independent of each other; both are documentation.
- No removal tasks. Nothing is deleted. No `Depends on reads:` line is needed.

## Assumptions

1. The doc-comment wording (T1) follows OS4 and R2. Exact wording is not a stable contract.
2. Spec tests 3, 6, and 7 are guards (A1). Spec tests 1, 2, 4, and 5 are red at HEAD. The red reason for test 1 is the empty-key `ConfigError` (`pkg/runner/run.go:397-400`). The red reasons are predicted from code reading; T2.1-T2.4 confirm them, and any mismatch is reported there.
3. Test 5's `live` sub-run asserts success (A2). The spec defines it as a successful live run.
4. The derived race package list is `./pkg/runner` only. No file in `internal/` or `cmd/` changes.
5. Non-empty key with empty base URL, and the reverse, are independent: each `if` applies only to its own field (OS2). No test covers the mixed case; it is recorded here, not probed.
6. Test 5 `live-fail` depends on a transport error to `http://options-base-url-probe:9`. Green-state claim (after T3.1): the request fails fast at DNS or connect. At HEAD the run returns the empty-key `ConfigError` before any request (T2.4). The Anthropic provider maps a transport error to `ErrCategoryServer` (`internal/provider/anthropic.go:284-288`). `MaxRetries` defaults to zero: the user `config.toml` has no `[retry]` section, and `internal/agent/agent.go` sets no default (verified at writing time), so there is no retry delay. If DNS resolution is slow on the host, the sub-run is slower but still correct.
7. Test 4 is parallel and reads the real environment and the real user `config.toml` (`pkg/runner/run.go:144`). Green-state claim: with T3.1 applied, Options precedence makes the asserted `x-api-key` header deterministic whatever the host environment holds. The HEAD red run clears `ANTHROPIC_API_KEY` and `AXE_ANTHROPIC_BASE_URL` in its command (T2.3, T2.6), so HEAD sends no request to api.anthropic.com.
8. The spec does not fix a maximum length for the key or base URL. No upper bound is added.

### Spec observations (non-blocking; reported, spec not edited)

- The spec's `Citation corrections` list gives the apply point at `pkg/runner/run.go:385-386`, the bedrock block at `pkg/runner/run.go:389-395`, the empty-key check at `pkg/runner/run.go:397-400`, the provider call at `pkg/runner/run.go:407`, the provider error at `pkg/runner/run.go:410`, the `func run` seam at `pkg/runner/run.go:65`, the dry-run block at `pkg/runner/run.go:328-379`, and the result literal at `pkg/runner/run.go:353`. These match HEAD `c2e20a2`, verified at writing time with `rg -n`.
- The spec cites `pkg/runner/options.go:16-73` for the `Options` type, `pkg/runner/options.go:63-67` for the `MaxTurns` field, and `pkg/runner/options.go:69` for the I/O fields. These match HEAD. The `MaxTurns` block is at `:63-67` and the I/O comment at `:69`.
- The spec cites `internal/tool/tool.go:224-225` for sub-agent credentials. Not re-verified at writing time beyond `internal/tool/tool.go:222-226`, which shows the sub-agent credential path in the same region. Spec-lint passes the spec, and T3.2 and T8 confirm no change to `internal/`.
- The milestone entry and the design note cite stale lines (see Section 1). The spec's corrected lines are used here.

## Limitations

- Sub-agents do not inherit `Options.APIKey` or `Options.BaseURL`. Their credentials still resolve from env and config (`internal/tool/tool.go:224-225`) (OS8).
- Redacting the base URL from returned transport errors is out of scope. Go HTTP transport errors include the request URL (`internal/provider/anthropic.go:275`). Test 5 asserts only that `err.Error()` does not contain the API key probe.
- Bedrock ignores both fields by design. Region and AWS credentials are still read from env and config (`internal/config/config.go:117-140`, `internal/provider/awscreds.go:19`) (OS1, R5).
- Test 4 reads the real process environment and the real user `config.toml` (`pkg/runner/run.go:144`), because it does not call `t.Setenv`. Green-state claim: with T3.1 applied, Options precedence makes the asserted header deterministic. The HEAD red run clears the two env sources in its command (T2.3, T2.6).
- `cmd/` tests need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`.
- The exit-code 2 mapping is a CLI concern (`pkg/runner/errors.go:9-10`). Test 7 asserts `IsConfigError`; the CLI mapping is unchanged and is not tested here.
- Milestone entry 009 and the design note cite stale lines. The corrected lines are in the spec's "Citation corrections (verified at `08d0f26`)". Neither document is edited by this checklist.
- The full race run over all packages, `make check`, and `checklist-test-gate-audit` (absent here) are not run by this checklist. The Factory completion gate runs the full race run.
