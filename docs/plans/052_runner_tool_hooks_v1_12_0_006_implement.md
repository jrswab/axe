# Implement Milestone 006: Pass MCP servers from code with `Options.MCPServers`

## Section 1: Context Summary

Milestone 006 of the v1.12.0 runner tool-hooks release lets a host pass MCP servers in code. `Options.MCPServers []MCPServer` is merged after the agent's TOML servers (TOML first, then the option list in slice order, OS1). One shared validator in `internal/agent` checks both sources before `DryRun` returns (R3, OS3). A duplicate name across or within the sources is a `ConfigError` (R4, OS2). A header `${VAR}` whose variable is unset or empty is a `ConfigError` matchable with `errors.Is(err, envinterp.ErrUnsetEnv)`, while `Error()` text is unchanged (R8, OS5). Headers are never printed: not in dry run, verbose output, errors, or `Result` (R10, OS8). Dry run does not read header variables (OS6). Sub-agents do not inherit the option list (R12, OS4). With the field nil or empty, behavior is byte-identical to HEAD (R13). Source: `docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md` (Sections 1 and 2, R1-R15, OS1-OS10, acceptance tests 1-12) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 006 at docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203-215; D8 at docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:70; D1/D2 at docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54-55). Predecessors 002 (docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154) and 005 (docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:189) are complete. Baseline is `a13e3bc`, an ancestor of HEAD `63c85f6`; the only commit since baseline is the spec/checkpoint draft (`63c85f6`), so no code has changed.

Commit is not authorized in this checklist. Do not push. Do not change files outside the list in each task. Do not edit the spec, the checkpoint, or the milestones document; those are orchestrator records (T9). Execution starts from a tree where the only modified path is `docs/plans/052_runner_tool_hooks_v1_12_0_006_checkpoint.md` (orchestrator record, not staged, not reverted). Stop and report if any non-record file appears.

Scope of changed files (spec R14, OS10, Acceptance commands): `CHANGELOG.md`, `internal/agent/agent.go`, `internal/agent/agent_test.go`, `internal/envinterp/envinterp.go`, `internal/envinterp/envinterp_test.go`, `pkg/runner/mcp_servers_test.go` (new), `pkg/runner/options.go`, `pkg/runner/run.go`. Nothing else. `internal/mcpclient/`, `internal/tool/`, and `cmd/` are not changed (OS9, R13).

Per-task gate for this milestone: every code task changes only files in `./internal/agent`, `./internal/envinterp`, and `./pkg/runner`, so the derived package list is those three directories. The derivation is the snippet in `/home/jaron/.agents/skills/factory/SKILL.md#per-task-go-gate`, reproduced in Section 2 under "Per-task gate derivation". T7.1 changes only `CHANGELOG.md` and has an empty derived package list. Red tasks do not pass the gate by design; their gate is the red run recorded in the task, and the gate runs in the green task that follows.

Test-selection convention: red tests in this checklist do not exist until their red task, and `spec-lint` rejects a `-run` pattern that selects no current test. Red steps therefore run the package verbosely (`go test -race -v ./pkg/runner`) and read per-test results with `rg -- '--- (PASS|FAIL): <Name>'`. Existing tests (`TestValidate_MCPServers_*`, `TestExpandHeaders_*`, `TestConnect_*`, `TestToolHookMCP*`, `TestRunCmd_DryRun_WithMCPServers`) exist already, so their steps may name them with `-run`.

Sandbox rules (from 005): `go test ./cmd/...` needs `XDG_CACHE_HOME=$(mktemp -d)`. The gofmt drift in `pkg/runner/result.go` and `pkg/runner/result_test.go` is pre-existing (`gofmt -l` reports both at HEAD `63c85f6`) and out of scope; do not reformat those files. Tests that call `t.Setenv` cannot use `t.Parallel`. Tests that need tool calls use `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`); `capturingProvider` (`pkg/runner/run_test.go:48`) is text-only.

Test names. The spec fixes the names of tests 1-12 (`pkg/runner/mcp_servers_test.go` for 1-10, `internal/envinterp/envinterp_test.go` for 11, `internal/agent/agent_test.go` for 12). Those names are used verbatim below. Names are unique (T0 verifies absence).

Decisions this checklist records (not open; none changes scope or behavior beyond the spec):
- **Validator name and signature (A1).** The spec requires "one exported function in `internal/agent`" (OS3) without naming it. This checklist names it `ValidateMCPServers(servers []MCPServerConfig, prefix string) error`. `prefix` is `mcp_servers` for TOML (so messages stay byte-identical, R13) and `Options.MCPServers` for the option list (R6).
- **Sentinel mechanism (A2).** `Error()` text must not change (OS5) and the chain must contain `ErrUnsetEnv` (R8). An `fmt.Errorf` with `%w` cannot do both, so the error at `internal/envinterp/envinterp.go:47` becomes an unexported type `unsetEnvError{name}` whose `Error()` returns the existing text and whose `Unwrap()` returns `ErrUnsetEnv`.
- **Cross-source duplicate wording (A3).** The spec requires a `ConfigError` for a name in both sources (R4) but does not fix its text. This checklist uses `MCP server name %q is in both mcp_servers and Options.MCPServers`. Behavior (type and rule) is fixed by the spec; wording is not a stable contract (Limitations).
- **Option wrapper message (A4).** The option-list validation error is wrapped as `ConfigError{Msg: "invalid MCP server configuration", Err: <validator error>}`. Only the type and the `Options.MCPServers[i]` label are fixed (R6).

---

## Section 2: Implementation Checklist

### Per-task gate derivation

Code tasks state the gate rule verbatim. The derivation, copied from the Factory skill, is run before each commit from the module root:

```sh
changed=$({ git diff --name-only --relative HEAD; git ls-files --others --exclude-standard; } | grep '\.go$' | xargs -r -n1 dirname | sort -u)
pkgs=$(go list -e -f '{{.Dir}}' ./... | while read -r d; do r=$(realpath --relative-to=. "$d"); printf '%s\n' "$changed" | grep -qxF "$r" && echo "./$r"; done)
if [ -n "$pkgs" ]; then go test -race $pkgs; fi
```

### Phase 0: Pre-flight (serial, read-only)

- [x] **T0: Pre-flight: HEAD ancestry, tree state, baseline package tests, declared names absent**
  - Budget: 30 min
  - Owner: code (verification)
  - Files: none (read-only).
  - Commands and expected results (repository root):
    - `git merge-base --is-ancestor a13e3bc HEAD; echo "exit=$?"` → `exit=0`.
    - `git diff --name-only a13e3bc HEAD -- . ':(exclude)docs/plans/'` → no output. Verified at writing time (`63c85f6`).
    - `git status --porcelain` → only ` M docs/plans/052_runner_tool_hooks_v1_12_0_006_checkpoint.md` (orchestrator record). Verified at writing time. Any other path: stop and report.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/envinterp ./internal/mcpclient` → `ok` for all four. Verified at writing time.
    - `rg -n 'TestMCPServers_|TestValidateMCPServers_SharedRules|TestExpandHeaders_UnsetEnvSentinel|ErrUnsetEnv|ValidateMCPServers|type MCPServer ' --glob '*.go' .` → no output (`exit=1`). Verified at writing time.
    - `test -e pkg/runner/mcp_servers_test.go; echo "exit=$?"` → `exit=1`. Verified at writing time.
    - `go test -list 'TestConnect_UnreachableServer|TestConnect_HeaderEnvVarMissing|TestToolHookMCPDeny|TestToolHookMCPAllow|TestRun_SubAgent$' ./internal/mcpclient ./pkg/runner` → lists all five names. Verified at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -list '^TestRunCmd_DryRun_WithMCPServers$' ./cmd` → lists the name. Verified at writing time.
    - `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` → `PASS: Go pre-flight checks.`, exit 0. Run at writing time as a precaution; the Makefile `apidiff` target runs `go run` on a pinned module (`Makefile:3`, `:22-23`), so the pre-flight is recorded.
    - `command -v golangci-lint` → prints a path (verified at writing time as `/home/jaron/.cache/pi-go-bin/golangci-lint`).
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: this task changes no files. The derived package list is empty before any edit.

### Phase 1: Enabling surface (serial)

- [x] **T1: Public API surface, sentinel and validator declarations, doc contract (R1, R14)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T0.
  - Enables: T2.1-T2.7 (test code must compile against the identifiers) and T3.x (implementation).
  - Files:
    - `pkg/runner/options.go`: add `type MCPServer struct { Name string; URL string; Transport string; Headers map[string]string }` after the `import "io"` line (`pkg/runner/options.go:3`). Add field `MCPServers []MCPServer` to `type Options struct` (`pkg/runner/options.go:6`) between the `ToolObserver ToolObserver` field (`:37`) and the I/O block (`:39`).
    - `internal/envinterp/envinterp.go`: add `"errors"` to the import block (internal/envinterp/envinterp.go:3-7) and add `var ErrUnsetEnv = errors.New("environment variable is not set or empty")` after the envPattern declaration (internal/envinterp/envinterp.go:9). Declaration only; no behavior changes in this task.
    - `internal/agent/agent.go`: add `func ValidateMCPServers(servers []MCPServerConfig, prefix string) error` after the Validate function (internal/agent/agent.go:148). Body in this task: `return nil`. This is a declaration so that the red test in T2.7 compiles and fails on assertion. T3.0 replaces the body with the moved rules (internal/agent/agent.go:183-208). The body is not final code and is not kept past T3.0.
  - Doc comments (R14, OS1, OS2, OS4, OS6, OS8, D8):
    - On `MCPServer`: fields match `agent.MCPServerConfig` (`internal/agent/agent.go:114`); `Headers` uses `${VAR}` syntax; header values are credentials and are never printed.
    - On `Options.MCPServers`: a nil or empty value adds no servers; the servers are merged after the agent's TOML servers in slice order; a duplicate name across or within the two sources is a `ConfigError`; dry runs do not read header variables; sub-agents do not inherit the list.
  - Do not add any behavior. `Run`, `run`, `Validate` and `Connect` are unchanged in this task.
  - Failing-first: not applicable (declarations only). Verification:
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/ ./internal/agent/ ./internal/envinterp/` → exit 0.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/envinterp ./internal/mcpclient` → `ok` (no behavior changed; R13).
    - `rg -n "type MCPServer struct|MCPServers \[\]MCPServer" pkg/runner/options.go` → both declarations.
    - `rg -n "ErrUnsetEnv = errors.New|func ValidateMCPServers" internal/envinterp/envinterp.go internal/agent/agent.go` → both declarations.
    - `rg -n "never printed|merged|sub-agents|dry runs" pkg/runner/options.go` → each doc claim present (R14).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/agent`, `./internal/envinterp`, `./pkg/runner`; run `go test -race ./internal/agent ./internal/envinterp ./pkg/runner`.

### Phase 2: Red tests (serial, test-only; every declared test must fail on an assertion or be marked as a guard before Phase 3)

Red-run rule: a red test is "red" only if it compiles and fails on an assertion. T1 adds every identifier the tests use (`MCPServer`, `Options.MCPServers`, `envinterp.ErrUnsetEnv`, `agent.ValidateMCPServers`), so every red test compiles.

Shared helper (T2.1, file `pkg/runner/mcp_servers_test.go`, new). `newMCPServersTestServer(t *testing.T, toolName string) (*httptest.Server, *mcpServersRecorder)` builds an `mcp.NewServer` with one tool `toolName` whose handler returns text `hello`, wraps it with `mcp.NewStreamableHTTPHandler` (`pkg/runner/hook_mcp_test.go:44`), and wraps the handler in an `http.HandlerFunc` that records each request's `Authorization` header and counts requests. `mcpServersRecorder` holds a mutex-protected `auth []string` and an `atomic.Int64` request count. The tool-call count is the same counter, so a test can distinguish "no request" from "tool not called". The helper is the one new helper; `newHookTestMCPServer` (`pkg/runner/hook_mcp_test.go:17`) does not record headers and is not changed. Live scenarios use `setupTestEnv` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr` (`pkg/runner/run_test.go:44`), `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), `AnthropicToolUseResponse` (`:154`), `AnthropicResponse` (`:141`), and `hookTestToolResult` (`pkg/runner/run_test.go:1465`) as in `runHookTestMCP` (`pkg/runner/hook_mcp_test.go:50-88`). Agent TOML in these tests sets `tools = []` where no built-in tool is needed.

- [x] **T2.1: Red: acceptance tests 1, 2a, 2b (R2, R4, R5)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T1.
  - File: `pkg/runner/mcp_servers_test.go` (new; created in this task with the helper above).
  - Tests:
    - `TestMCPServers_OptionsOnlyToolsInRequest` (test 1). TOML has `tools = []` and no `mcp_servers`. `Options.MCPServers` has one server (name `code-mcp`, URL from the helper, transport `streamable-http`) whose tool is `mcp_echo`. Assert `Run` returns no error and `mock.Requests[0].Body` contains `mcp_echo`.
    - `TestMCPServers_MergedDistinctNames` (test 2a). TOML server `local-mcp` exposes `mcp_echo`; option server `code-mcp` exposes `mcp_other`. Assert both names appear in `mock.Requests[0].Body`. Assert the dry-run list order is `local-mcp` then `code-mcp` (run a second `Options.DryRun` call and compare `DryRunInfo.MCPServers` with the two strings in order).
    - `TestMCPServers_DuplicateNameConfigError` (test 2b), table with two subtests: `toml_and_option` (TOML and option both named `local-mcp`) and `option_and_option` (two option entries named `local-mcp`). Each subtest runs as a live run. Assert `IsConfigError(err)`.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMCPServers_OptionsOnlyToolsInRequest`, `--- FAIL: TestMCPServers_MergedDistinctNames`, `--- FAIL: TestMCPServers_DuplicateNameConfigError` (each subtest). Filter with `rg -- '--- (PASS|FAIL): TestMCPServers_'`. Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above; the race run is required once T3.5 is green.

- [x] **T2.2: Red: acceptance tests 3 and 4 (R3, R6, R8, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.1.
  - File: `pkg/runner/mcp_servers_test.go`.
  - Tests:
    - `TestMCPServers_InvalidServerNoConnect` (test 3), table over invalid option entries: `transport_ws` (transport `ws`), `scheme_ftp` (URL `ftp://host/mcp`), `empty_url`, `empty_name`, and `no_host` (URL `http:///mcp`). Each case runs twice: a live run and a dry run (`Options.DryRun`). The TOML server in each case is valid and its helper server records zero requests. The option server's helper records zero requests. Assert `IsConfigError(err)` for each run. Assert `recorder.count == 0` on both servers after each live run.
    - `TestMCPServers_UnsetHeaderEnvConfigError` (test 4). Two subtests: `unset` (`Authorization: Bearer ${TOKEN_UNSET_006}` with the variable never set, as Assumption 3 requires; do not call `os.Unsetenv`) and `empty` (variable set to empty with `t.Setenv`). Each option server also has a second header `X-Secret: secret-literal-value`. Assert `IsConfigError(err)`, `errors.Is(err, envinterp.ErrUnsetEnv)`, and that `err.Error()` does not contain `secret-literal-value`. Add a parity subtest `toml_source` where the same header is on a TOML server; assert `IsConfigError(err)` and the same no-literal rule.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMCPServers_InvalidServerNoConnect` (each subtest) and `--- FAIL: TestMCPServers_UnsetHeaderEnvConfigError` (subtests `unset` and `empty`; `toml_source` may pass at HEAD, since the TOML path already maps the text match at `pkg/runner/run.go:447`; the `errors.Is` assertion still fails). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

- [x] **T2.3: Red: acceptance tests 5 and 6 (R2, R7, R8, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.2.
  - File: `pkg/runner/mcp_servers_test.go`.
  - Tests:
    - `TestMCPServers_HeaderExpandedOnWire` (test 5). `t.Setenv("MCP_TEST_TOKEN", "tok-123")`. Option server `Headers` is `{"Authorization": "Bearer ${MCP_TEST_TOKEN}"}`. The helper records `Authorization`. Assert every recorded value equals `Bearer tok-123`, that at least one request was recorded, and that the tool appears in `mock.Requests[0].Body`.
    - `TestMCPServers_DryRunMergedNoHeaders` (test 6). `Options.DryRun` true. One TOML server `local-mcp` and one option server `code-mcp` whose header references an unset variable (`${MCP_UNSET_006}`), plus a literal `secret-literal-value` in a second header. Assert no error; `DryRunInfo.MCPServers` equals `["local-mcp: <url1> (streamable-http)", "code-mcp: <url2> (streamable-http)"]` (URLs from the helper); the result, marshalled with `fmt.Sprintf("%+v", result)`, contains neither `secret-literal-value`, the header key `Authorization`, nor `${MCP_UNSET_006}`. Assert both helper servers recorded zero requests.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMCPServers_HeaderExpandedOnWire` (no request recorded) and `--- FAIL: TestMCPServers_DryRunMergedNoHeaders` (the list has one entry). Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

- [x] **T2.4: Red: acceptance test 8 (red) and guard test 7 (R11, R13)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.3.
  - File: `pkg/runner/mcp_servers_test.go`.
  - Tests:
    - `TestMCPServers_EmptySliceUnchanged` (test 7, guard). Table with two cases: `nil` (`MCPServers: nil`) and `empty` (`MCPServers: []MCPServer{}`). Each runs the TOML-only scenario of `TestToolHookMCPAllow` (`pkg/runner/hook_mcp_test.go:118`): one TOML server, one `mcp_echo` call, text reply. Each live run (unset, nil, empty) uses its own `testutil.NewMockLLMServer` with its own queued responses (`AnthropicToolUseResponse`, then `AnthropicResponse`), its own `AXE_ANTHROPIC_BASE_URL`, and its own helper server. Compare each live run separately: its `mock.Requests[0].Body` and its error value (nil) equal those of the unset live run in the same subtest. Separately, make a dry run (`Options.DryRun`) for each of unset, nil, and empty, and assert the nil and empty `DryRunInfo.MCPServers` equal the unset one (the live `Result` literal at `pkg/runner/run.go:743-750` has no `DryRunInfo`; the dry-run one is at `pkg/runner/run.go:308-311`).
    - `TestMCPServers_RunDoesNotMutateOptions` (test 8, red). Build `opts.MCPServers` with one server (name `code-mcp`, transport `streamable-http`, headers `{"Authorization": "Bearer ${MCP_TEST_TOKEN}"}`) and `t.Setenv("MCP_TEST_TOKEN", "tok-123")`. Deep-copy the slice and every `Headers` map before the run. Run the live scenario (one `mcp_echo` call from the option server). Assert `Run` returns no error, the request contains `mcp_echo` (so the option server was used), and the original slice and maps equal the copy (`reflect.DeepEqual`).
  - Red step: `go test -race -v ./pkg/runner` shows `--- PASS: TestMCPServers_EmptySliceUnchanged` (guard; passes at HEAD) and `--- FAIL: TestMCPServers_RunDoesNotMutateOptions` (no `mcp_echo` in the request). Filter with `rg -- '--- (PASS|FAIL): TestMCPServers_(EmptySliceUnchanged|RunDoesNotMutateOptions)'`. Record the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

- [x] **T2.5: Red: acceptance tests 9 and 10 (R10, R12)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.4.
  - File: `pkg/runner/mcp_servers_test.go`.
  - Tests:
    - `TestMCPServers_SubAgentNotInherited` (test 9, red). Parent TOML has `tools = []`, `sub_agents = ["child-agent"]`, and no servers. The parent has `Options.MCPServers` with one option server exposing `mcp_parent_only`. The model is `AnthropicToolUseResponse` with `call_agent` (ID `tool_1`, input `{"agent": "child-agent", "task": "say hello"}`), then `AnthropicResponse("hello from child")` for the child, then `AnthropicResponse` for the parent. Child TOML is written at `tmpDir/axe/agents/child-agent.toml` as in pkg/runner/run_test.go:793-800 (the TestRun_SubAgent setup). Assert `Run` returns no error. Assert `mock.Requests[0].Body` contains `mcp_parent_only` (the parent connected and registered the option server; this is the assertion that makes the test red before T3.3). Assert `mock.Requests[1].Body` does not contain `mcp_parent_only`. Assert the child request's tool list (also `mock.Requests[1].Body`) has no `mcp_parent_only`.
    - `TestMCPServers_VerboseNoHeaders` (test 10, red). `Verbose` true, `Stderr` the buffer from `newMockStdoutStderr`. Option server `code-mcp` with header `Authorization: Bearer secret-literal-value` (static value). Assert the stderr buffer contains the server URL and does not contain `secret-literal-value`. Assert the request contains `mcp_echo` (so the server was connected and the verbose line was printed).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMCPServers_SubAgentNotInherited` (`mock.Requests[0].Body` lacks `mcp_parent_only` because the option server is not connected at HEAD) and `--- FAIL: TestMCPServers_VerboseNoHeaders` (no verbose line for the option server). Filter with `rg -- '--- (PASS|FAIL): TestMCPServers_(SubAgentNotInherited|VerboseNoHeaders)'`. Record the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

- [x] **T2.6: Red: acceptance test 11 `TestExpandHeaders_UnsetEnvSentinel` (R8, OS5)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.5.
  - File: `internal/envinterp/envinterp_test.go` (add one test at the end of the file; do not edit existing tests).
  - Test: `TestExpandHeaders_UnsetEnvSentinel`. Table with two subtests: `unset` (`Bearer ${UNSET_006_VAR}`, variable not set) and `empty` (`t.Setenv("EMPTY_006_VAR", "")`, value `Bearer ${EMPTY_006_VAR}`). Each calls `ExpandHeaders` with header `Authorization`. Assert `errors.Is(err, ErrUnsetEnv)` and that `err.Error()` equals, exactly, `header "Authorization": environment variable "UNSET_006_VAR" is not set or empty` (and the `EMPTY_006_VAR` form for the empty case; the existing exact-text tests are internal/envinterp/envinterp_test.go:85, internal/envinterp/envinterp_test.go:98). Add the `errors` import to the test file.
  - Red step: `go test -race -v ./internal/envinterp` shows `--- FAIL: TestExpandHeaders_UnsetEnvSentinel` (each subtest; `errors.Is` is false). internal/envinterp/envinterp_test.go:78 (TestExpandHeaders_MissingVar) and internal/envinterp/envinterp_test.go:90 (TestExpandHeaders_EmptyVar) pass. Filter with `rg -- '--- (PASS|FAIL): TestExpandHeaders_'`. Record the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

- [x] **T2.7: Red: acceptance test 12 `TestValidateMCPServers_SharedRules` (R3, R6, R13)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.6.
  - File: `internal/agent/agent_test.go` (add one test at the end of the file; do not edit existing tests).
  - Test: `TestValidateMCPServers_SharedRules`. Table over the rules at `internal/agent/agent.go:185-206`: blank name, blank URL, malformed URL, scheme `ftp`, missing host, transport `ws`, duplicate name. Each case runs once with prefix `mcp_servers` and once with prefix `Options.MCPServers`. Assert an error is returned for each case. For every row except malformed URL, assert the message equals the existing text for the `mcp_servers` prefix (`mcp_servers[0].name is required`, `mcp_servers names must be unique: "local"`, etc., as in `internal/agent/agent_test.go:1082`), and the same text with `Options.MCPServers` in place of `mcp_servers` for the `Options.MCPServers` prefix (e.g. `Options.MCPServers names must be unique: "local"`). The malformed-URL row is checked by containment only for both prefixes, `<prefix>[0].url is invalid`, because its message embeds `url.ParseRequestURI` text (same rule as `TestValidate_MCPServers_MalformedURL`, `internal/agent/agent_test.go:1105-1122`). Assert no message contains the literal `secret-literal-value`, a header value placed in a `Headers` map of the failing entry.
  - Red step: `go test -race -v ./internal/agent` shows `--- FAIL: TestValidateMCPServers_SharedRules` (the T1 stub returns nil, so no error). Existing `TestValidate_MCPServers_*` (`internal/agent/agent_test.go:1055-1258`) pass. Filter with `rg -- '--- (PASS|FAIL): TestValidate'`. Record the run.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. For this red task the gate is the red run recorded above.

Phase 2 red-run check: after T2.1-T2.7, run `go test -race -v ./pkg/runner ./internal/agent ./internal/envinterp` and filter with `rg -- '--- (PASS|FAIL): (TestMCPServers_|TestExpandHeaders_UnsetEnvSentinel|TestValidateMCPServers_SharedRules)'`. Must show `--- FAIL` for tests 1, 2a, 2b, 3, 4, 5, 6, 8, 9, 10, 11, 12. Must show `--- PASS` for guard 7. Record the output.

### Phase 3: Green implementation (serial)

Order: T3.0 (validator, `internal/agent`) and T3.1 (sentinel, `internal/envinterp`) are independent and may run in either order. T3.2 needs T3.0. T3.3 needs T3.1 and T3.2. T3.4 needs T3.3. T3.5 needs all.

- [ ] **T3.0: Green: move the TOML rules into `ValidateMCPServers` (R3, R6, R13)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.7.
  - File: `internal/agent/agent.go` only.
  - Behavior:
    - Replace the body of `ValidateMCPServers` (stub from T1) with the loop currently at `internal/agent/agent.go:183-208`, using `prefix` in place of the literal `mcp_servers`. Message text for `prefix == "mcp_servers"` is byte-identical to today (`mcp_servers[%d].name is required` at `:186`, `mcp_servers[%d].url is required` at `:189`, `:193`, `:196`, `:199`, `:202`, and `mcp_servers names must be unique: %q` at `:205`). The same rules and order apply for any prefix.
    - `Validate` (`internal/agent/agent.go:148`) replaces its inline loop (`:183-208`) with `if err := ValidateMCPServers(cfg.MCPServers, "mcp_servers"); err != nil { return err }`. `Validate`'s signature and its other checks are unchanged.
    - Unchanged callers: Load calls Validate at internal/agent/agent.go:270; cmd/fixture_test.go:35 and cmd/fixture_test.go:189 call agent.Validate.
  - Green targets: T2.7 `TestValidateMCPServers_SharedRules`; existing `TestValidate_MCPServers_*` (`internal/agent/agent_test.go:1055-1258`) and `TestParseTOML_MCPServers` (`:1277`) still pass unchanged.
  - Verification:
    - `go test -v ./internal/agent | rg -- '--- (PASS|FAIL): TestValidate'` → every line `PASS`.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./internal/agent` → `ok`.
    - `rg -n "mcp_servers names must be unique|mcp_servers\[%d\]" internal/agent/agent.go` → the text is in `ValidateMCPServers`, not in `Validate`.
    - `go vet ./internal/agent/` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/agent`; run `go test -race ./internal/agent`.

- [ ] **T3.1: Green: `unsetEnvError` matchable by `ErrUnsetEnv`, text unchanged (R8, OS5)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.7.
  - File: `internal/envinterp/envinterp.go` only.
  - Behavior:
    - Add an unexported type `unsetEnvError struct { name string }` with `Error() string` returning `fmt.Sprintf("environment variable %q is not set or empty", e.name)` (same text as today at `internal/envinterp/envinterp.go:47`) and `Unwrap() error` returning `ErrUnsetEnv`.
    - In expandValue (internal/envinterp/envinterp.go:46-47), replace `expandErr = fmt.Errorf("environment variable %q is not set or empty", name)` with `expandErr = &unsetEnvError{name: name}`.
    - ExpandHeaders at internal/envinterp/envinterp.go:21 is unchanged; its `%w` wrap keeps the header key in the text.
  - Green targets: T2.6 `TestExpandHeaders_UnsetEnvSentinel`; existing `TestExpandHeaders_MissingVar` (`internal/envinterp/envinterp_test.go:78`), `TestExpandHeaders_EmptyVar` (`:90`), and the other `TestExpandHeaders_*` tests pass unchanged.
  - Verification:
    - `go test -v ./internal/envinterp | rg -- '--- (PASS|FAIL): TestExpandHeaders_'` → every line `PASS`.
    - `rg -n "unsetEnvError|ErrUnsetEnv" internal/envinterp/envinterp.go` → the type, the `Unwrap`, the sentinel, and the single construction site.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./internal/envinterp ./internal/mcpclient` → `ok` (`TestConnect_HeaderEnvVarMissing` at `internal/mcpclient/mcpclient_test.go:88` still passes).
    - `go vet ./internal/envinterp/` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/envinterp`; run `go test -race ./internal/envinterp`.

- [ ] **T3.2: Green: merged effective set, validation before dry-run, cross-source duplicates, dry-run list (R2, R3, R4, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.0, T2.1-T2.5 (red).
  - File: `pkg/runner/run.go` only.
  - Behavior:
    - Add a helper `mcpServerConfigs(in []MCPServer) []agent.MCPServerConfig` in `pkg/runner/run.go` that converts each `MCPServer` to `agent.MCPServerConfig` with the same fields and copies `Headers` into a new map (R11 precondition; the copy is required by T3.4).
    - After the tool-hook check (`pkg/runner/run.go:75-77`) and before Step 3 overrides (`:79`), add: (a) `agent.ValidateMCPServers(mcpServerConfigs(opts.MCPServers), "Options.MCPServers")`; on error return `&ConfigError{Msg: "invalid MCP server configuration", Err: err}` (A4); (b) for each option name, if the same name appears in `cfg.MCPServers`, return `&ConfigError{Msg: fmt.Sprintf("MCP server name %q is in both mcp_servers and Options.MCPServers", name)}` (A3, R4). Validation happens before any connect and before the dry-run return (R3).
    - Build `effectiveMCP := append(append([]agent.MCPServerConfig{}, cfg.MCPServers...), mcpServerConfigs(opts.MCPServers)...)` after the checks (OS1). TOML first, option slice order.
    - Dry run (`pkg/runner/run.go:300-303`): build the list from `effectiveMCP`, not `cfg.MCPServers`. Format is unchanged: `fmt.Sprintf("%s: %s (%s)", s.Name, s.URL, s.Transport)`. No header is read or printed (OS6, OS8).
    - Do not change the connect loop (pkg/runner/run.go:441), the len(cfg.MCPServers) guard (pkg/runner/run.go:432), or the sentinel mapping (pkg/runner/run.go:447) in this task; those are T3.3.
  - Green targets: T2.1 `TestMCPServers_OptionsOnlyToolsInRequest`, `TestMCPServers_MergedDistinctNames` (order part via dry run), `TestMCPServers_DuplicateNameConfigError` (both subtests); T2.2 `TestMCPServers_InvalidServerNoConnect` (live and dry, zero requests); T2.3 `TestMCPServers_DryRunMergedNoHeaders`.
  - Note: T2.1 test 1 is green only after T3.3 connects the option server. Test 1 is listed here because its validation and merge parts are in this task; its request assertion turns green in T3.3. Record that split in the T3.2 run.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMCPServers_(DuplicateNameConfigError|InvalidServerNoConnect|DryRunMergedNoHeaders)'` → every line `PASS`.
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMCPServers_(OptionsOnlyToolsInRequest|MergedDistinctNames)'` → recorded; both may still FAIL on the request assertion until T3.3.
    - `rg -n "ValidateMCPServers|both mcp_servers and Options.MCPServers|effectiveMCP" pkg/runner/run.go` → the calls and the merged list.
    - `go vet ./pkg/runner/` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Until T3.3 is green, the only expected failures are these named tests: `TestMCPServers_OptionsOnlyToolsInRequest` and `TestMCPServers_MergedDistinctNames` (request part, noted above), `TestMCPServers_UnsetHeaderEnvConfigError` (`unset` and `empty` subtests), `TestMCPServers_HeaderExpandedOnWire`, `TestMCPServers_VerboseNoHeaders`, `TestMCPServers_RunDoesNotMutateOptions`, and `TestMCPServers_SubAgentNotInherited` (test 9 red until T3.3 connects the option server; its request assertion is `mock.Requests[0].Body` containing `mcp_parent_only`). Record which tests fail and stop if any other test fails.

- [ ] **T3.3: Green: connect loop over the effective set, sentinel mapping, verbose line, header wiring (R5, R7, R8, R9, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.1, T3.2.
  - File: `pkg/runner/run.go` only. Add imports `"errors"` and `"github.com/jrswab/axe/internal/envinterp"` (pkg/runner/run.go:12 imports strings and pkg/runner/run.go:16 imports internal/agent; there is no errors import today).
  - Behavior:
    - `pkg/runner/run.go:432`: change `if len(cfg.MCPServers) > 0` to `if len(effectiveMCP) > 0`.
    - `pkg/runner/run.go:441`: change `for _, serverCfg := range cfg.MCPServers` to `for _, serverCfg := range effectiveMCP`.
    - `pkg/runner/run.go:447`: change the condition to `errors.Is(connErr, envinterp.ErrUnsetEnv) || strings.Contains(connErr.Error(), "environment variable") || strings.Contains(connErr.Error(), "unsupported MCP transport")`. The text match stays for this release (D8, R8).
    - Other connect failures stay `RuntimeError` with the existing message (pkg/runner/run.go:450, R9).
    - The verbose line (pkg/runner/run.go:443) is unchanged; it prints name, URL, and transport from `serverCfg` (OS8, R10).
    - Header expansion runs in `mcpclient.Connect` (`internal/mcpclient/mcpclient.go:25-29`), unchanged, so one path expands both sources (OS9, R7).
  - Green targets: T2.1 `TestMCPServers_OptionsOnlyToolsInRequest` and `TestMCPServers_MergedDistinctNames` (request part); T2.2 `TestMCPServers_UnsetHeaderEnvConfigError` (`unset`, `empty`, `toml_source`); T2.3 `TestMCPServers_HeaderExpandedOnWire`; T2.5 `TestMCPServers_SubAgentNotInherited` (test 9, red-to-green transition is here) and `TestMCPServers_VerboseNoHeaders`.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMCPServers_(OptionsOnlyToolsInRequest|MergedDistinctNames|UnsetHeaderEnvConfigError|HeaderExpandedOnWire|VerboseNoHeaders|SubAgentNotInherited)'` → every line `PASS`.
    - `go test ./pkg/runner` → `ok`.
    - `rg -n "errors.Is\(connErr, envinterp.ErrUnsetEnv\)|environment variable\"\)|range effectiveMCP" pkg/runner/run.go` → the sentinel branch, the kept text match, and the connect loop over the effective set.
    - `go vet ./pkg/runner/` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.4: Green: caller data not mutated; sub-agents do not inherit the list (R11, R12)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.3.
  - File: `pkg/runner/run.go` only if a change is needed. No change is expected: `mcpServerConfigs` (T3.2) copies each `Headers` map, `ExpandHeaders` builds a new map (`internal/envinterp/envinterp.go:17`), and `Run` never writes to `opts.MCPServers`. Sub-agent loading (`internal/tool/tool.go:152`, `:291-292`) reads only the child's own TOML and receives no option list (`pkg/runner/run.go:666` passes no MCP data; `executeToolCalls` signature at `pkg/runner/run.go:925` is unchanged). Confirm by reading. If a change is needed, record why and stop.
  - Green targets: T2.4 `TestMCPServers_RunDoesNotMutateOptions` (red until now) and the guard `TestMCPServers_EmptySliceUnchanged`. T2.5 `TestMCPServers_SubAgentNotInherited` is already green from T3.3; re-checked here.
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMCPServers_(RunDoesNotMutateOptions|EmptySliceUnchanged|SubAgentNotInherited)'` → all `PASS`.
    - `rg -n "MCPServers|mcp" internal/tool/tool.go` → unchanged from HEAD (`git diff --exit-code a13e3bc -- internal/tool` exits 0).
    - `rg -n "opts.MCPServers\[.*\] *=|opts.MCPServers = " pkg/runner/run.go` → no output (no write to the caller slice or map).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.5: Green: all `TestMCPServers_*`, `TestExpandHeaders_UnsetEnvSentinel`, and `TestValidateMCPServers_SharedRules` pass; TOML-only behavior unchanged (R13)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.0, T3.1, T3.2, T3.3, T3.4.
  - Files: none expected. Fix any failure in the file that introduced it, not in the test.
  - Green targets: tests 1-12 all `PASS` (guard 7 `PASS` throughout; test 9 `PASS` from T3.3).
  - Verification:
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMCPServers_'` → every line `PASS`.
    - `go test -v ./internal/envinterp | rg -- '--- (PASS|FAIL): TestExpandHeaders_UnsetEnvSentinel'` → `PASS`.
    - `go test -v ./internal/agent | rg -- '--- (PASS|FAIL): TestValidateMCPServers_SharedRules'` → `PASS`.
    - `rg -n "tool is not supported with sub_agents|tool hook is not supported" pkg/runner/run.go` → the 004 `ConfigError` for `ToolHook` only (`pkg/runner/run.go:75-76`); no MCP check added there.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/agent`, `./internal/envinterp`, `./pkg/runner`; run `go test -race ./internal/agent ./internal/envinterp ./pkg/runner`.

### Phase 4: Full package verification (serial, after Phase 3)

- [ ] **T4: Focused and regression verification; TOML and CLI unchanged (R13)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.5.
  - Files: none (verification only).
  - Commands:
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./internal/agent ./internal/envinterp ./internal/mcpclient ./pkg/runner` → `ok` for all four.
    - `go test -v ./pkg/runner ./internal/agent ./internal/envinterp | rg -- '--- (PASS|FAIL): (TestMCPServers_|TestExpandHeaders_|TestValidate|TestConnect_|TestToolHookMCP)'` → every line `PASS`. Record the list.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd` → `ok` (includes `TestRunCmd_DryRun_WithMCPServers`, `cmd/run_test.go:289`, and `TestParseTOML_MCPServers`-related CLI paths; the cmd tests need a writable cache).
    - `go build ./...` → exit 0.
    - `go vet ./pkg/runner/ ./internal/agent/ ./internal/envinterp/` → exit 0.
    - `git diff --exit-code a13e3bc -- internal/mcpclient cmd internal/tool go.mod go.sum` → exit 0.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/agent`, `./internal/envinterp`, `./pkg/runner`; run `go test -race ./internal/agent ./internal/envinterp ./pkg/runner`.

- [ ] **T5: Race run of the MCP tests, including parallel tool execution (R2, R5, R9)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T4.
  - Step 1 (unfiltered): `go test -race -v ./pkg/runner ./internal/agent ./internal/envinterp > /tmp/t5-race.txt 2>&1; echo "exit=$?"`. Expected: `exit=0`. Save the full output to `/tmp/t5-race.txt`.
  - Step 2 (only after step 1 prints `exit=0`; inspect the saved file): `rg -- '--- (PASS|FAIL): TestMCPServers_' /tmp/t5-race.txt` → every line `PASS`; `rg -- 'DATA RACE' /tmp/t5-race.txt` → no matches (rg exit 1).
  - Pass condition: step 1 exit 0 (its own exit code), every `TestMCPServers_` line in the saved file `PASS`, and no `DATA RACE` in the saved file. The helper server's request counter and `Authorization` recorder are the most likely race sites; they are mutex- or atomic-protected (T2.1).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./internal/agent`, `./internal/envinterp`, `./pkg/runner`; run `go test -race ./internal/agent ./internal/envinterp ./pkg/runner`.

- [ ] **T6: `golangci-lint run` (lint gate)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0`, no findings. Run as one call; a timeout is not a pass, so report it.
  - Any finding in `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/mcp_servers_test.go`, `internal/agent/agent.go`, `internal/agent/agent_test.go`, `internal/envinterp/envinterp.go`, or `internal/envinterp/envinterp_test.go` is a failure of the task that introduced it; fix it in that task's file, then re-run T4 and T6.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; it runs lint on the T1-T5 changes.

- [ ] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R1, D1)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m006-apidiff-out.txt 2>/tmp/axe-m006-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m006-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0` (`Makefile:4`, `:15`). `MCPServer`, `Options.MCPServers` are additions and must not fail it (`Makefile:14-24`). The tag `v1.11.0` resolves at writing time (`20e6ada`).
  - Cleanup: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache. If it fails for network reasons, report it; do not change the pin (`Makefile:3`).
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.MCPServers` (R14, OS10; `CONTRIBUTING.md:185`)**
  - Budget: 15 min
  - Owner: code
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]` → `### Added` (`CHANGELOG.md:8-10`), append one bullet after the last existing bullet of that section, which is the 005 `Options.ToolObserver` bullet at `CHANGELOG.md:14`. Text: "- `Options.MCPServers` in `pkg/runner` adds MCP servers from code, merged after the agent's TOML servers. A duplicate name is a `ConfigError`. Header values are never printed, and a header variable that is unset is a `ConfigError`. A nil or empty list changes nothing." Form follows the 004 and 005 bullets at `CHANGELOG.md:13-14`.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`; `rg -n "Options.MCPServers" CHANGELOG.md` → one match.
  - Per-task gate: no Go file changes; `git diff --name-only -- '*.go'` is unchanged by this task, so no race run is needed.

- [ ] **T8: Scope, API, and removed-line checks (R13, R14, R15)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T7.1.
  - Files: none (verification only).
  - Commands and expected results:
    - `git diff --name-only a13e3bc -- . ':(exclude)docs/plans/'` → exactly `CHANGELOG.md`, `internal/agent/agent.go`, `internal/agent/agent_test.go`, `internal/envinterp/envinterp.go`, `internal/envinterp/envinterp_test.go`, `pkg/runner/mcp_servers_test.go`, `pkg/runner/options.go`, `pkg/runner/run.go`. Untracked new files appear in `git status`; check both views.
    - `git status --porcelain` → those paths (`pkg/runner/mcp_servers_test.go` as `??` until commit, which is not in this checklist) plus orchestrator records under `docs/plans/`; nothing else.
    - `git diff --exit-code a13e3bc -- internal/mcpclient cmd internal/tool go.mod go.sum` → exit 0.
    - `rg -n "type MCPServer struct|MCPServers \[\]MCPServer" pkg/runner/options.go` → the type and field.
    - `rg -n "never printed|merged|sub-agents|dry runs" pkg/runner/options.go` → each claim present (R14).
    - `rg -n "Options.MCPServers" CHANGELOG.md` → one bullet (R14).
    - `git diff a13e3bc -- pkg/runner/run.go | rg -n '^-' | rg -v '^[0-9]+:---'` → removed lines are only: the dry-run loop source (cfg.MCPServers in pkg/runner/run.go:300-303, re-pointed to effectiveMCP), the len(cfg.MCPServers) > 0 guard (pkg/runner/run.go:432), the connect loop range (pkg/runner/run.go:441), the sentinel condition (pkg/runner/run.go:447, extended, text match kept), and the `"strings"`-only import block lines extended with `errors` and `envinterp`. `executeToolCalls` is unchanged. Any other removed line is a failure; report it.
    - `git diff a13e3bc -- internal/agent/agent.go | rg -n '^-' | rg -v '^[0-9]+:---'` → removed lines are only the inline loop `agent.go:183-208` replaced by the call; the `Validate` signature line is unchanged.
    - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_006_implement.md` → exit 0.
    - `~/.agents/scripts/docs-lint --since a13e3bc /home/jaron/dev/axe/runner-tool-hooks` → exit 0.
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T9: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Owner: orchestrator, after T8.
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_006_checkpoint.md` (phase, task states, gate evidence) and the milestone checkbox `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203` (`- [ ] 006` becomes `- [x] 006`). Do not change the milestone entry beyond the checkbox. Record the milestone entry's stale run.go citations (listed in Blockers) in the checkpoint only, if the orchestrator decides; this checklist does not edit the milestone text.
  - Owner: orchestrator. This writer does not touch these files.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 `MCPServer` type and `Options.MCPServers` field | T1 (declare), T7 (apidiff), T8 (rg) |
| R2 merged server set (TOML first, then option slice order) | T2.1, T2.3, T3.2, T3.3 |
| R3 validation before connect and before dry-run return | T2.2, T2.7, T3.0, T3.2 |
| R4 duplicate names are `ConfigError` (within and across sources) | T2.1 (2b), T3.2 |
| R5 tools from both sources, collision rules unchanged | T2.1 (1, 2a), T3.3, T5 |
| R6 transport and URL errors from the option list, `Options.MCPServers[i]` label | T2.2 (3), T2.7 (12), T3.0, T3.2 |
| R7 header expansion from both sources | T2.3 (5), T3.3 |
| R8 unset header variable is `ConfigError`, matchable by sentinel | T2.2 (4), T2.6 (11), T3.1, T3.3 |
| R9 other connect failures unchanged | T3.3 (`RuntimeError` at `pkg/runner/run.go:450`), T4 (`TestConnect_UnreachableServer`, `internal/mcpclient/mcpclient_test.go:47`) |
| R10 no header text in any output | T2.2 (4), T2.3 (6), T2.5 (10), T2.7 (12), T3.3 |
| R11 caller data not modified | T2.4 (8), T3.2 (copy), T3.4 |
| R12 sub-agents do not receive the option list | T2.5 (9, red), T3.3 (green), T3.4 (confirm) |
| R13 TOML and CLI behavior unchanged | T2.4 (7 guard), T3.0, T3.5, T4 (`./cmd`), T8 |
| R14 doc comments and CHANGELOG | T1 (doc comments), T7.1 (CHANGELOG), T8 |
| R15 acceptance tests 1-12 | Acceptance Test Mapping below |

## Acceptance Test Mapping (R15, one task per test)

| Acceptance test | Test function | Red task | Green task | Note |
|---|---|---|---|---|
| 1 server only in option list: tools in request | `TestMCPServers_OptionsOnlyToolsInRequest` | T2.1 | T3.2 (validation), T3.3 (request) | |
| 2a merged distinct names, dry-run order | `TestMCPServers_MergedDistinctNames` | T2.1 | T3.2 (order), T3.3 (request) | |
| 2b duplicate name `ConfigError` (`toml_and_option`, `option_and_option`) | `TestMCPServers_DuplicateNameConfigError` | T2.1 | T3.2 | |
| 3 invalid option entry, live and dry, zero requests | `TestMCPServers_InvalidServerNoConnect` | T2.2 | T3.2 | |
| 4 unset or empty header variable `ConfigError`, sentinel, no literal (`unset`, `empty`, `toml_source`) | `TestMCPServers_UnsetHeaderEnvConfigError` | T2.2 | T3.1, T3.3 | `toml_source` may pass at HEAD; the sentinel assertion is the red part. |
| 5 expanded header on the wire | `TestMCPServers_HeaderExpandedOnWire` | T2.3 | T3.3 | |
| 6 dry run merged, no headers, no reads | `TestMCPServers_DryRunMergedNoHeaders` | T2.3 | T3.2 | |
| 7 TOML-only and empty list unchanged | `TestMCPServers_EmptySliceUnchanged` | T2.4 | none (guard) | PASS at HEAD and after; plus `TestValidate_MCPServers_*`, `TestConnect_HeaderEnvVarMissing`, `TestToolHookMCPDeny`, `TestRunCmd_DryRun_WithMCPServers`. |
| 8 caller slice and maps not mutated | `TestMCPServers_RunDoesNotMutateOptions` | T2.4 | T3.2, T3.4 | |
| 9 sub-agent does not inherit the list | `TestMCPServers_SubAgentNotInherited` | T2.5 | T3.3 | FAIL at T2.5 on `mcp_parent_only` in `mock.Requests[0].Body` (option server not connected); green at T3.3; T3.4 confirms no `internal/tool` change. |
| 10 verbose output has no header value | `TestMCPServers_VerboseNoHeaders` | T2.5 | T3.3 | |
| 11 sentinel matchable, text unchanged | `TestExpandHeaders_UnsetEnvSentinel` | T2.6 | T3.1 | `TestExpandHeaders_MissingVar` and `TestExpandHeaders_EmptyVar` run unchanged. |
| 12 shared validator rules, TOML text unchanged, option label | `TestValidateMCPServers_SharedRules` | T2.7 | T3.0 | |

Every acceptance test 1-12 maps to a red task and a green or guard task. The spec's focused run is T4 and T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures: none. Additions only.
  - `agent.Validate` (`internal/agent/agent.go:148`): signature unchanged. Callers: `internal/agent/agent.go:270` (inside `Load`), `cmd/fixture_test.go:35`, `cmd/fixture_test.go:189`. No caller update needed.
  - `envinterp.ExpandHeaders` (`internal/envinterp/envinterp.go:12`): signature unchanged. Callers: `internal/mcpclient/mcpclient.go:26`, and `internal/envinterp/envinterp_test.go` (lines `:14`, `:32`, `:47`, `:65`, `:81`, `:94`, `:104`, `:115`, `:130`, `:143`, `:156`). No caller update needed.
  - `mcpclient.Connect` (`internal/mcpclient/mcpclient.go:25`): signature unchanged. Callers: pkg/runner/run.go:445, internal/tool/tool.go:292. No caller update needed (OS9).
  - `runner.Options` (`pkg/runner/options.go:6`): gains one field. Keyed literal at `cmd/run.go:80` and keyed literals in `pkg/runner/*_test.go` compile unchanged.
  - `ValidateMCPServers`, `MCPServer`, `ErrUnsetEnv`: new identifiers, no prior callers.
- Extreme acceptance inputs and probe evidence (read-only, at writing time):
  - Blank and whitespace-only name: `strings.TrimSpace(server.Name) == ""` at `internal/agent/agent.go:185`. Expected `ConfigError` for both sources (R3, R6). Covered by the T2.7 `blank name` row and T2.2 `empty_name`.
  - Empty URL and URL with no host: internal/agent/agent.go:188, internal/agent/agent.go:198. Expected `ConfigError` (R6). Covered by T2.2 `empty_url` and `no_host` (`http:///mcp`), and T2.7.
  - Invalid scheme and transport: `internal/agent/agent.go:195`, `:201`. Expected `ConfigError` (R6). Covered by T2.2 `scheme_ftp` and `transport_ws`.
  - Unset and empty header variable: `internal/envinterp/envinterp.go:46` (`v == ""` covers unset and empty). Expected `errors.Is(err, ErrUnsetEnv)` and unchanged text (R8, OS5). Covered by T2.2 `unset` and `empty`, T2.6.
  - Duplicate name, case differs (`Local` vs `local`): not a duplicate under OS2 (exact comparison, `internal/agent/agent.go:204-206`). No test asserts this case; the spec defines it, and the rule is exact string equality in code (no case folding). Recorded here so the implementer does not add folding.
  - Very long names or URLs: the spec defines no length limit and the validator has none (`internal/agent/agent.go:183-208`). No probe; no expected behavior is defined, so none is asserted. Not applicable.
  - Header value containing a literal secret: `secret-literal-value` (T2.2, T2.3, T2.5, T2.7). Expected absent from every error, verbose line, and dry-run field (R10, OS8).
- `go install` or `GOPROXY=` commands: none in this checklist. The `go-toolchain-check` pre-flight is recorded in T0 as a precaution (exit 0).
- `./.agents/scripts/checklist-test-gate-audit`: not present in this repository (`.agents/scripts` does not exist). Not run.

## Blockers

None. Reviewed against the spec; the open points are recorded as decisions A1-A4 in Section 1 and do not change scope or behavior. The spec's line references were re-verified at HEAD 63c85f6. The milestone entry's run.go references are stale: it cites line 443 for the unset-env text match (current line 447) and lines 296-300 for the dry-run list (current lines 300-303 at pkg/runner/run.go:300-303). The spec uses current lines, and the milestone correction is an orchestrator decision in T9.

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T2.5 → T2.6 → T2.7 (test edits; T2.1-T2.5 share `pkg/runner/mcp_servers_test.go`, and T2.6/T2.7 share no file with them).
- Phase 3 order: T3.0 and T3.1 may run in parallel (different packages: `internal/agent/agent.go` and `internal/envinterp/envinterp.go`). T3.2 after T3.0. T3.3 after T3.1 and T3.2. T3.4 after T3.3. T3.5 after all.
- Serial after Phase 3: T4 → T5 → T6 → T7 → T7.1 → T8 → T9.
- Parallel: T7.1 (CHANGELOG) is independent of the code and may be drafted during Phase 2, but it is committed in order after T7. T2.6 and T2.7 touch other packages and may be drafted in parallel with T2.1-T2.5, but are run in the listed order so the red-run record is clean.

## Assumptions

1. Validator name and signature (A1), sentinel mechanism (A2), cross-source duplicate wording (A3), and option-wrapper message (A4) are as stated in Section 1. None changes behavior the spec fixes.
2. `ValidateMCPServers` is declared in T1 with a `return nil` body only so that T2.7 compiles and fails on assertion. T3.0 replaces the body. The stub is never kept.
3. Test 4's `unset` case relies on the variable not being set in the test process. The implementer uses a variable name that is not set anywhere in the repository (`TOKEN_UNSET_006`), and does not call `os.Unsetenv`.
4. Test 5 and test 8 depend on `Authorization` being set on every MCP request by `headerRoundTripper` (`internal/mcpclient/mcpclient.go:31`). This is existing behavior, verified by `TestConnect_HeaderInjection` (`internal/mcpclient/mcpclient_test.go:58`).
5. Test 9's child agent setup follows pkg/runner/run_test.go:793-800 (the TestRun_SubAgent setup). The child has no servers, so only the parent's option list could leak; the test checks the child request body.
6. `golangci-lint` is on `PATH` (verified at writing time) and `make lint` runs `golangci-lint run` (`Makefile:9-10`).

## Limitations

- The milestone's "no `t.Setenv`" is a host property. Acceptance tests set environment variables with `t.Setenv` (spec Section 1, Assumption, test environment).
- A URL containing a secret in its query string is printed in dry-run and verbose output, as TOML URLs are today. This milestone does not change URL printing.
- The CLI cannot set `Options.MCPServers`. The CLI regression check is `TestRunCmd_DryRun_WithMCPServers` (`cmd/run_test.go:289`) and the existing TOML tests, not a new CLI test.
- The wording of the new `Options.MCPServers[i]` and cross-source duplicate messages (A3, A4) is not a stable public contract.
- The full race run over all packages and `make check` are not run by this checklist; the Factory completion gate runs them.
