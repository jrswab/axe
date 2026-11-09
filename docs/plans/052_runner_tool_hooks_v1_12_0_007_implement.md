# Implement Milestone 007: Limit built-in tools from code with `Options.Tools`

## Section 1: Context Summary

Milestone 007 of the v1.12.0 runner tool-hooks release lets a host set the built-in tool list per run in code. `Options.Tools []string` replaces the agent's TOML tool list when non-nil (R4, OS3). A nil value keeps today's behavior byte for byte (R7). An empty non-nil value gives no built-in tools, and with no sub-agents and no MCP tools the request has no `tools` key, the same shape as TOML `tools = []` (R5, OS4). Each name is checked with `toolname.ValidNames()` before the dry-run return; an unknown or empty name is a `ConfigError` that names `Options.Tools[i]` (R2, OS1, OS2). The effective list also drives `DryRunInfo.Tools` (R6). MCP shadowing follows the effective list: a built-in removed by the option no longer shadows a same-named MCP tool (R9, OS6). The list does not limit MCP tools or `call_agent` (OS7), and sub-agents do not inherit it (R11, OS8). `agent.Load` still validates TOML tools (R3, OS5). Source: `docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md` (Sections 1 and 2, R1-R12, OS1-OS12, acceptance tests 1-6) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 007 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:217-226`; D1, D2, D9 at `:54`, `:55`, `:71`). Predecessor 006 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203`, `[x] 006`). Baseline is `de43f3c`, an ancestor of HEAD `ba1feb7`; HEAD adds only the spec and checkpoint draft, so no code has changed since the baseline.

Commit is not authorized in this checklist. Do not push. Do not edit records. The spec, the checkpoint, and the milestones document are orchestrator records; this checklist does not change them (T9 is the orchestrator's). Execution starts from a tree where the only modified path is `docs/plans/052_runner_tool_hooks_v1_12_0_007_checkpoint.md` (orchestrator record, not staged, not reverted). Stop and report if any other file is modified or untracked. Tasks are committed one at a time by the orchestrator; T8 reads the committed range `de43f3c..HEAD`, not the working tree.

Scope of changed files (spec R8, R10, OS11): `CHANGELOG.md`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/tools_option_test.go` (new). Nothing else. `internal/agent/`, `internal/tool/`, `internal/toolname/`, `internal/mcpclient/`, `internal/envinterp/`, and `cmd/` are not changed (OS5, OS7, OS8, R8). Milestone 007 changes no file in `internal/agent`, so unlike 006 the derived race package list is `./pkg/runner` only.

Per-task gate for this milestone: every code task changes only files in `./pkg/runner`, so the derived package list is `./pkg/runner`. T7.1 changes only `CHANGELOG.md` and has an empty derived list. Red tasks run `go test -race -v ./pkg/runner` and record the expected FAIL set; the race run is required green from T3.3 onward (see Section 2).

Test-selection convention: new test names (`TestTools_*`) do not exist until their red task. `spec-lint` rejects a `-run` pattern that selects no current test. Red and green steps therefore run the package verbosely (`go test -race -v ./pkg/runner`) and read results with `rg -- '--- (PASS|FAIL): TestTools_'`. `-run` is used only for existing tests, verified in T0.

Sandbox rules (from 005 and 006): `go test ./cmd` needs `XDG_CACHE_HOME=$(mktemp -d)`. Tests that call `t.Setenv` or `setupTestEnv` cannot use `t.Parallel`. `capturingProvider` (`pkg/runner/run_test.go:48`) is text-only; tests that need tool calls or request inspection use `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), whose recorded requests are `mock.Requests[i].Body` (`internal/testutil/mockserver.go:23-28`). The gofmt drift in `pkg/runner/result.go` and `pkg/runner/result_test.go` is pre-existing (`gofmt -l` reports both at HEAD `ba1feb7`) and out of scope; do not reformat those files.

Decisions this checklist records (not open; none changes scope or behavior beyond the spec):
- **Validation message (A1).** The spec fixes the type (`ConfigError`) and that the message names `Options.Tools[i]` and the bad name (OS2). The wording is `unknown tool %q in Options.Tools[%d]`. Wording is not a stable contract (spec Limitations).
- **Effective list copy (A2).** The effective list is `append([]string{}, opts.Tools...)` when `opts.Tools` is non-nil, and `cfg.Tools` otherwise. The copy is the OS9 "separate slice" rule. The nil path uses the same `cfg.Tools` value as HEAD, so behavior is byte-identical (R7).
- **Test 5 TOML fixture (A3).** The spec does not state the TOML tool list for acceptance test 5. This checklist sets `tools = ["read_file"]` in that agent's TOML. Without it, sub-case (b) would not be a shadowing test at HEAD (see Assumptions). The MCP server comes from `Options.MCPServers` (006), not TOML.
- **Test fixtures.** Tests set `Workdir: t.TempDir()` and `Stdin: strings.NewReader("")`, as `runHookTestMCP` does (`pkg/runner/hook_mcp_test.go:50-88`), and write `[providers.anthropic] api_key = "fake-key"` through `setupTestEnv`.

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

### Phase 0: Pre-flight (serial, read-only)

- [ ] **T0: Pre-flight: HEAD ancestry, tree state, baseline tests, declared names absent**
  - Budget: 30 min
  - Owner: code (verification)
  - Files: none (read-only).
  - Commands and expected results (repository root):
    - `git merge-base --is-ancestor de43f3c HEAD; echo "exit=$?"` → `exit=0`. Verified at writing time (HEAD `ba1feb7`).
    - `git diff --name-only de43f3c HEAD -- . ':(exclude)docs/plans/'` → no output. Verified at writing time.
    - `git status --porcelain` → only ` M docs/plans/052_runner_tool_hooks_v1_12_0_007_checkpoint.md` (orchestrator record). Verified at writing time. Any other path: stop and report.
    - `git rev-parse --short HEAD` → `ba1feb7`. Verified at writing time.
    - `rg -n 'TestTools_' --glob '*.go' .` → no output, `exit=1`. Verified at writing time (names absent).
    - `test -e pkg/runner/tools_option_test.go; echo "exit=$?"` → `exit=1`. Verified at writing time.
    - `rg -n "Options.Tools|Tools \[\]string" pkg/runner/options.go; echo "exit=$?"` → `exit=1`. Verified at writing time (field absent).
    - `git ls-files pkg/runner/mcp_servers_test.go` → `pkg/runner/mcp_servers_test.go`. Verified at writing time; `rg -n "func newMCPServersTestServer" pkg/runner/mcp_servers_test.go` → `pkg/runner/mcp_servers_test.go:28`.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/toolname ./internal/mcpclient ./internal/envinterp` → `ok` for all five. Verified at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -list '^(TestRun_NoSubAgents_NoTools|TestRunCmd_DryRun_WithMCPServers)$' ./cmd` → lists both names, `ok`. Verified at writing time.
    - `go test -list '^(TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)$' ./pkg/runner` → lists all three names, `ok`. Verified at writing time.
    - `git diff --exit-code de43f3c -- internal/mcpclient cmd internal/tool go.mod go.sum; echo "exit=$?"` → `exit=0`. Verified at writing time.
    - `gofmt -l pkg/runner internal/agent internal/envinterp` → only `pkg/runner/result.go` and `pkg/runner/result_test.go` (pre-existing). Verified at writing time.
    - `git rev-parse --verify 'v1.11.0^{commit}'` → `20e6ada179b08cef050c4b436a2497182efcf2ee`. Verified at writing time (apidiff pin baseline, `Makefile:15`).
    - `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` → `PASS: Go pre-flight checks.`, `exit=0`. Verified at writing time. Recorded as a precaution: T7 runs `go run` on a pinned module (`Makefile:3`, `Makefile:22-23`).
    - `command -v golangci-lint` → `/home/jaron/.cache/pi-go-bin/golangci-lint`. Verified at writing time.
    - `test -e .agents/scripts/checklist-test-gate-audit; echo "exit=$?"` → `exit=1` (script absent; not run). Verified at writing time.
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: this task changes no files. The derived package list is empty before any edit.

### Phase 1: Enabling surface (serial)

- [ ] **T1: Public API surface and doc contract (R1, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T0.
  - Enables: T2.1-T2.3 (test code must compile against `Options.Tools`) and T3.x (implementation).
  - Files:
    - `pkg/runner/options.go`: add field `Tools []string` to `type Options struct` after the `MCPServers []MCPServer` field (`pkg/runner/options.go:53`), with a doc comment placed above it.
  - Doc comment (R1, R10, OS2, OS6, OS7, OS8): nil uses the agent's TOML tool list; a non-nil list, including an empty one, replaces it; each name is checked against the built-in set, and an unknown or empty name is a ConfigError; the list does not limit MCP tools or `call_agent`; a built-in removed by the list no longer shadows a same-named MCP tool; sub-agents do not inherit the list.
  - Do not add behavior. `run`, `Run`, `agent.Load`, and `toolname` are unchanged in this task.
  - Failing-first: not applicable (declaration only). Verification:
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner` → `ok` (no behavior changed).
    - `rg -n "Tools \[\]string" pkg/runner/options.go` → the new field.
    - `rg -n "replaces|sub-agents|shadow|MCP tools" pkg/runner/options.go` → each doc claim present (R10).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 2: Red tests (serial, test-only; every declared test must fail on an assertion, or be marked guard, before Phase 3)

Red-run rule: a red test is "red" only if it compiles and fails on an assertion. T1 adds the identifier the tests use (`Options.Tools`), so every red test compiles. Guard tests are marked `[guard]` and pass at HEAD.

Shared test conventions (file `pkg/runner/tools_option_test.go`, new, created in T2.1): each test uses `setupTestEnv` (`pkg/runner/run_test.go:24`) and `newMockStdoutStderr` (`pkg/runner/run_test.go:44`); `Workdir: t.TempDir()`, `Stdin: strings.NewReader("")`; `t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())`; mock responses from `testutil.AnthropicToolUseResponse` (`internal/testutil/mockserver.go:154`) and `testutil.AnthropicResponse` (`internal/testutil/mockserver.go:141`). No test uses `t.Parallel`. Definition checks read `mock.Requests[0].Body` (the first request has no tool-use history); `mock.Requests[i].Body` is a string (`internal/testutil/mockserver.go:27`).

- [ ] **T2.1: Red: acceptance tests 1 and 2 (R2, R4, R6, R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T1.
  - File: `pkg/runner/tools_option_test.go` (new; created in this task).
  - Tests:
    - `TestTools_OptionReplacesTOMLList` (test 1). Agent TOML: `tools = ["list_directory", "read_file"]`, no `sub_agents`, no MCP servers. `opts.Tools` is `[]string{"write_file"}`. Mock: one response `AnthropicResponse("Done")`. Assert `Run` returns no error; `mock.Requests[0].Body` contains `"write_file"` and does not contain `"read_file"` or `"list_directory"` (R4); the caller's slice still equals `[]string{"write_file"}` after the run (R11, mutation assertion). Then a dry run with the same `Tools` (`DryRun: true`) asserts `DryRunInfo.Tools` equals `[]string{"write_file"}` (R6).
    - `TestTools_UnknownNameConfigError` (test 2), table with subtests `bogus` (`"bogus"`), `empty` (`""`), `call_agent` (`"call_agent"`). Each subtest runs twice: a live run and a dry run (`DryRun: true`). Each uses a mock with one response, which must record zero requests. Assert `IsConfigError(err)` (`pkg/runner/errors.go:57`), that `err.Error()` contains `Options.Tools[0]`, and that the live run's mock `RequestCount()` is zero (R2).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestTools_OptionReplacesTOMLList` (request body contains `"read_file"`, and the dry run reports the TOML list) and `--- FAIL: TestTools_UnknownNameConfigError` (each subtest, live and dry, has no error). Filter with `rg -- '--- (PASS|FAIL): TestTools_'`. Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above (`go test -race -v ./pkg/runner`); the race run must be green from T3.3.

- [ ] **T2.2: Red: acceptance tests 3, 4 (guard), 5 (R5, R7, R9)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.1.
  - File: `pkg/runner/tools_option_test.go`.
  - Tests:
    - `TestTools_EmptyListNoToolsKey` (test 3). Agent TOML: `tools = ["read_file"]`, no `sub_agents`, no MCP servers. `opts.Tools` is `[]string{}`. Mock: one response `AnthropicResponse("Done")`. Decode `mock.Requests[0].Body` as JSON (`encoding/json`) into `map[string]json.RawMessage` and assert the top-level object has no `tools` key (R5, OS4). Assert `Run` returns no error.
    - `TestTools_NilUsesTOMLList` (test 4, `[guard]`). The same TOML as test 3, `opts.Tools` nil. Assert `mock.Requests[0].Body` contains `"read_file"` (R7). PASS at HEAD.
    - `TestTools_MCPShadowingFollowsEffectiveList` (test 5), with two subtests:
      - `a_builtin_kept` (`[guard]`, PASS at HEAD). Agent TOML: `tools = ["read_file"]`, no `sub_agents`, no TOML MCP servers. `opts.MCPServers` has one server from `newMCPServersTestServer(t, "read_file")` (`pkg/runner/mcp_servers_test.go:28`), with `Name: "code-mcp"`, `URL` from the helper, `Transport: "streamable-http"`. `opts.Tools` is `[]string{"read_file"}`. Assert `strings.Count(mock.Requests[0].Body, `"name":"read_file"`) == 1` and `mock.Requests[0].Body` does not contain `Returns a greeting` (the built-in `read_file` description starts with `Read the contents of a file`, `internal/tool/read_file.go:26`; the MCP tool description is `Returns a greeting`, `pkg/runner/mcp_servers_test.go:35`).
      - `b_builtin_removed` (red). Same TOML, same MCP server exposing `read_file`. `opts.Tools` is `[]string{"list_directory"}`. The mock's first response is `AnthropicToolUseResponse("Call MCP tool", []testutil.MockToolCall{{ID: "tool_1", Name: "read_file", Input: map[string]string{"name": "axe"}}})`, then `AnthropicResponse("Done")`. Assert `strings.Count(mock.Requests[0].Body, `"name":"read_file"`) == 1` and `mock.Requests[0].Body` contains `Returns a greeting`; assert `mock.Requests[1].Body` contains `hello` (the MCP server's text result, `pkg/runner/mcp_servers_test.go:44`).
    - Both subtests assert `Run` returns no error. Use `Options.MCPServers` (006), not TOML, for the server.
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestTools_EmptyListNoToolsKey` (body has `tools` key), `--- PASS: TestTools_NilUsesTOMLList` (guard), `--- PASS: TestTools_MCPShadowingFollowsEffectiveList/a_builtin_kept` (guard), and `--- FAIL: TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed` (the MCP `read_file` is skipped at HEAD because the TOML built-in shadows it, so `Returns a greeting` is absent). Filter with `rg -- '--- (PASS|FAIL): TestTools_'`. Record the failing assertions.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.3: Red: acceptance test 6 (R11, R12)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.2.
  - File: `pkg/runner/tools_option_test.go`.
  - Test: `TestTools_SubAgentNotInherited` (test 6). Parent TOML: `tools = []`, `sub_agents = ["child-agent"]`, no MCP servers. Child TOML at `filepath.Join(tmpDir, "axe", "agents", "child-agent.toml")` (`tmpDir` from `setupTestEnv`, as `pkg/runner/run_test.go:793-800`): `name = "child-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, `tools = ["read_file"]`, no `sub_agents`. `opts.Tools` is `[]string{"write_file"}`. Mock responses in order: `AnthropicToolUseResponse("Delegating", []testutil.MockToolCall{{ID: "tool_1", Name: "call_agent", Input: map[string]string{"agent": "child-agent", "task": "say hello"}}})`, then `AnthropicResponse("hello from child")`, then `AnthropicResponse("done")`. Assert `Run` returns no error; `mock.Requests[0].Body` (parent) contains `"write_file"`; `mock.Requests[1].Body` (child) contains `"read_file"` and does not contain `"write_file"` (R11, OS8). The order is parent (`Requests[0]`), child (`Requests[1]`), parent (`Requests[2]`).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestTools_SubAgentNotInherited` (parent body has no `write_file`: the TOML tools are empty and `Options.Tools` is ignored at HEAD, so the parent request has no built-in tool). Filter with `rg -- '--- (PASS|FAIL): TestTools_SubAgentNotInherited'`. Record the failing assertion.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.4: Red-run check (all acceptance tests)**
  - Budget: 15 min
  - Owner: code (verification)
  - Depends on: T2.3.
  - Files: none.
  - Command: `go test -race -v ./pkg/runner > /tmp/007-t24-red.txt 2>&1; echo "exit=$?"`, then `rg -- '--- (PASS|FAIL): TestTools_' /tmp/007-t24-red.txt`.
  - Expected: `exit=1` (red tests exist). The `rg` output shows `--- FAIL` for `TestTools_OptionReplacesTOMLList`, `TestTools_UnknownNameConfigError` (each subtest), `TestTools_EmptyListNoToolsKey`, `TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed`, and `TestTools_SubAgentNotInherited`. It shows `--- PASS` for guards `TestTools_NilUsesTOMLList` and `TestTools_MCPShadowingFollowsEffectiveList/a_builtin_kept`. Any other FAIL in the package means a red test is wrong; stop and report.
  - Also `rg -- '--- FAIL: Test' /tmp/007-t24-red.txt` must list only the five red tests and their subtests; any existing test in the output is a stop.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is `./pkg/runner` only if the test file is uncommitted, and the red run above is the gate.

### Phase 3: Green implementation (serial)

Order: T3.1 (validation) → T3.2 (effective list, live path) → T3.3 (dry-run report) → T3.4 (shadowing and sub-agent verification) → T3.5 (all green). All green tasks edit `pkg/runner/run.go` only, so they are serial.

Expected FAIL set after each green task (the race run in each task must show only these; any other FAIL is a stop):
- After T3.1: `TestTools_OptionReplacesTOMLList` (request assertion is green but the dry-run assertion is red), `TestTools_EmptyListNoToolsKey`, `TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed`, `TestTools_SubAgentNotInherited`.
- After T3.2: `TestTools_OptionReplacesTOMLList` (dry-run `DryRunInfo.Tools` assertion only).
- After T3.3 and T3.4: none.

- [ ] **T3.1: Green: validate `Options.Tools` before the dry-run return (R2)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.4.
  - File: `pkg/runner/run.go` only.
  - Behavior:
    - Add `"github.com/jrswab/axe/internal/toolname"` to the internal import group (`pkg/runner/run.go:18-24`); gofmt sorts it.
    - Insert after the cross-source duplicate loop, before the line that builds `effectiveMCP` (insert after line 104 of `pkg/runner/run.go`; that build is on line 105):
      ```go
      validToolNames := toolname.ValidNames()
      for i, name := range opts.Tools {
          if !validToolNames[name] {
              return nil, &ConfigError{Msg: fmt.Sprintf("unknown tool %q in Options.Tools[%d]", name, i)}
          }
      }
      ```
    - A nil `opts.Tools` is a no-op in the range loop, so the nil path is unchanged (R7). An empty non-nil value passes the loop (R5).
    - The effective list and dry-run report are T3.2 and T3.3. Do not change them here.
  - Green targets: `TestTools_UnknownNameConfigError` (live and dry, every subtest) PASS.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_UnknownNameConfigError'` → every line `PASS`.
    - `rg -n "unknown tool %q in Options.Tools|toolname.ValidNames" pkg/runner/run.go` → the check and the message.
    - `go vet ./pkg/runner/` → `exit=0`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: as listed above; any other FAIL is a stop.

- [ ] **T3.2: Green: effective built-in list for the live run (R4, R5, R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.1.
  - File: `pkg/runner/run.go` only.
  - Behavior:
    - After the validation loop from T3.1, add:
      ```go
      effectiveTools := cfg.Tools
      if opts.Tools != nil {
          effectiveTools = append([]string{}, opts.Tools...)
      }
      ```
      The copy is the OS9 "separate slice" rule. A nil `opts.Tools` keeps `cfg.Tools` (R7). An empty non-nil value gives an empty slice (R5).
    - Change the TOML tool resolution at `pkg/runner/run.go:445-446`: `if len(cfg.Tools) > 0 {` becomes `if len(effectiveTools) > 0 {`, and `registry.Resolve(cfg.Tools)` becomes `registry.Resolve(effectiveTools)`.
    - `builtinNames` (`pkg/runner/run.go:464-467`) is built from `req.Tools`, so MCP shadowing follows the effective list with no edit (OS6, R9). Verified in T3.4.
    - MCP and `call_agent` injection (`pkg/runner/run.go:455-456`) are unchanged (OS7).
    - The `agent.Load` function and the sub-agent resolution at `internal/tool/tool.go:266-275` are unchanged (OS5, OS8).
  - Green targets: `TestTools_OptionReplacesTOMLList` request assertions and mutation assertion; `TestTools_EmptyListNoToolsKey` (test 3, no `tools` key); `TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed` (MCP `read_file` registered and reachable); `TestTools_SubAgentNotInherited` (parent body contains `write_file`; child unaffected).
  - Expected FAIL after this task: `TestTools_OptionReplacesTOMLList` dry-run assertion only (fixed in T3.3).
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_(EmptyListNoToolsKey|NilUsesTOMLList|MCPShadowingFollowsEffectiveList|SubAgentNotInherited)'` → every line `PASS`.
    - `rg -n "effectiveTools|registry.Resolve\(effectiveTools\)" pkg/runner/run.go` → the copy, the gate, and the resolve call.
    - `rg -n "registry.Resolve\(cfg.Tools\)" pkg/runner/run.go` → no output (`exit=1`).
    - `go vet ./pkg/runner/` → `exit=0`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: `TestTools_OptionReplacesTOMLList` (dry-run assertion) only.

- [ ] **T3.3: Green: dry run reports the effective list (R6, R7)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.2.
  - File: `pkg/runner/run.go` only.
  - Behavior:
    - Change `pkg/runner/run.go:352` `Tools:           cfg.Tools,` to `Tools:           effectiveTools,` inside the `DryRunInfo` literal (OS1). The dry-run block opens at `pkg/runner/run.go:311`, and the return is at `:336`. `effectiveTools` is defined before both (T3.2), so the dry run reports the same list the live run resolves.
    - For a nil option, `effectiveTools` is `cfg.Tools` (same slice), so the dry-run output is byte-identical to HEAD (R7). The CLI prints `DryRunInfo.Tools` (`cmd/run.go:188-190`) and cannot set the option, so CLI output is unchanged (R8).
  - Green targets: `TestTools_OptionReplacesTOMLList` dry-run assertion (`DryRunInfo.Tools` equals `[]string{"write_file"}`); `TestTools_NilUsesTOMLList` (guard) stays PASS.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_(OptionReplacesTOMLList|NilUsesTOMLList)'` → every line `PASS`.
    - `rg -n "Tools:           effectiveTools," pkg/runner/run.go` → the dry-run field (one match).
    - `rg -n "Tools:           cfg.Tools," pkg/runner/run.go` → no output (`exit=1`).
    - `go test -count=1 ./cmd` is run in T4 (`XDG_CACHE_HOME` needed there).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

- [ ] **T3.4: Verify shadowing and sub-agent boundary (R9, R11, R12); no code expected**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.3.
  - Files: none expected. If a test fails here, fix `pkg/runner/run.go` only in the effective-list wiring from T3.2; do not change the test.
  - Behavior to confirm by reading:
    - `builtinNames` is built from `req.Tools` (`pkg/runner/run.go:464-467`), so the effective list drives shadowing (OS6, R9). The router skip is `internal/mcpclient/router.go:26-28`, unchanged.
    - Sub-agent loading reads only the child's own TOML (`internal/tool/tool.go:152`) and resolves its own `cfg.Tools` (`internal/tool/tool.go:266-275`). `Options.Tools` is not passed to `executeToolCalls` (signature at `pkg/runner/run.go:953`, unchanged) (OS8, R11).
    - `Run` writes nothing to `opts.Tools`; the only use is the copy at T3.2 (OS9).
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_'` → every line `PASS` (including guards `NilUsesTOMLList` and `MCPShadowingFollowsEffectiveList/a_builtin_kept`).
    - `git diff --exit-code de43f3c -- internal/tool internal/mcpclient` → `exit=0`.
    - `rg -n "opts.Tools\[.*\] *=|opts.Tools = " pkg/runner/run.go` → no output (`exit=1`): no write to the caller slice.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files unless a fix is needed; if none, the derived package list is empty. If a fix to `pkg/runner/run.go` was needed, the package is `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

- [ ] **T3.5: Green: all acceptance tests 1-6 pass; TOML and nil behavior unchanged (R7, R13)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.1, T3.2, T3.3, T3.4.
  - Files: none expected. Fix any failure in the file that introduced it, not in the test.
  - Green targets: tests 1-6 `PASS`; guards `TestTools_NilUsesTOMLList` and `TestTools_MCPShadowingFollowsEffectiveList/a_builtin_kept` `PASS`.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_'` → every line `PASS`; `rg -- '--- FAIL' ` → no output.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner` → `ok`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner` (derived list is empty if T3.4 made no change and the tree is committed); run `go test -race ./pkg/runner`. Expected FAIL set: none.

### Phase 4: Full package verification (serial, after Phase 3)

- [ ] **T4: Focused and regression verification; TOML, CLI, and sub-agent paths unchanged (R7, R8, R13)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.5.
  - Files: none (verification only).
  - Commands:
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent ./internal/toolname ./internal/mcpclient ./internal/envinterp` → `ok` for all five.
    - `go test -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestTools_'` → every line `PASS`. Record the list.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd` → `ok` (includes `TestRun_NoSubAgents_NoTools`, `cmd/run_test.go:1007`, and `TestRunCmd_DryRun_WithMCPServers`, `cmd/run_test.go:289`).
    - `go test -count=1 -v ./pkg/runner | rg -- '--- (PASS|FAIL): (TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)$'` → every line `PASS` (existing tests, verified in T0).
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `gofmt -l pkg/runner/options.go pkg/runner/run.go pkg/runner/tools_option_test.go` → no output. The drift in `result.go` and `result_test.go` is out of scope and is not listed.
    - `git diff --exit-code de43f3c -- internal/mcpclient cmd internal/tool internal/agent internal/toolname internal/envinterp go.mod go.sum` → `exit=0`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty. T5 runs the race run.

- [ ] **T5: Race run of the `TestTools_` family and the package (R2, R4, R9, R11)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T4.
  - Step 1: `go test -race -v ./pkg/runner > /tmp/007-t5-race.txt 2>&1; echo "exit=$?"`. Expected `exit=0`.
  - Step 2 (only after step 1 prints `exit=0`; inspect the saved file): `rg -- '--- (PASS|FAIL): TestTools_' /tmp/007-t5-race.txt` → every line `PASS`; `rg -- 'DATA RACE' /tmp/007-t5-race.txt` → no matches (`exit=1`).
  - Pass condition: step 1 exit 0, every `TestTools_` line `PASS`, and no `DATA RACE`. The option list is copied (T3.2) and read-only after the copy, so no race site is expected; the MCP helper's recorder is mutex-protected (`pkg/runner/mcp_servers_test.go:22-26`).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T6: `golangci-lint run` (lint gate)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0`, no findings. Run as one call; a timeout is not a pass, so report it.
  - Any finding in `pkg/runner/options.go`, `pkg/runner/run.go`, or `pkg/runner/tools_option_test.go` is a failure of the task that introduced it; fix it in that task's file, then re-run T4 and T6.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R1, D1)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m007-apidiff-out.txt 2>/tmp/axe-m007-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m007-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0` (`Makefile:4`, `:15`). `Options.Tools` is an addition and must not fail it (`Makefile:14-24`). The tag resolves to `20e6ada` (T0).
  - Cleanup: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache. If it fails for network reasons, report it; do not change the pin (`Makefile:3`).
  - No file changes.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.Tools` (R10, OS11; `CONTRIBUTING.md:185`)**
  - Budget: 15 min
  - Owner: code
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]` → `### Added` (`CHANGELOG.md:8-10`), append one bullet after the last existing bullet of that section, which is the 006 `Options.MCPServers` bullet at `CHANGELOG.md:15`; the blank line follows at `:16`. Text: "- `Options.Tools` in `pkg/runner` replaces the agent's TOML built-in tool list for one run when non-nil. Names are checked like TOML tools, and an unknown or empty name is a `ConfigError`. A nil list keeps the TOML list, and an empty list gives no built-in tools. The list does not limit MCP tools or `call_agent`. A built-in removed by the list no longer shadows a same-named MCP tool, which is now registered." Form follows the bullets at `CHANGELOG.md:13-15`.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`; `rg -n "Options.Tools" CHANGELOG.md` → one match; `rg -n "shadow" CHANGELOG.md` → the shadowing sentence in that bullet (R10).
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. No Go file changes; the derived package list is empty.

- [ ] **T8: Scope, API, removed-line, and spec-lint checks against the committed range (R8, R10, R11)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T7.1.
  - Files: none expected. If a citation refresh is needed (below), this checklist only.
  - Step 1 (citation refresh, if needed): run spec-lint (step 3). If it reports a stale citation in **this checklist** because code lines shifted after T3.x, refresh the line numbers only (no text change) and re-run. If it reports a stale citation in the **spec** (`docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md`), stop and report to the orchestrator; the code agent does not edit the spec.
  - Commands and expected results (committed range `de43f3c..HEAD`, tree expected clean):
    - `git status --porcelain` → no output. The orchestrator's T9 records come after this task.
    - `git diff --name-only de43f3c..HEAD -- . ':(exclude)docs/plans/'` → exactly `CHANGELOG.md`, `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/tools_option_test.go`.
    - `git diff --exit-code de43f3c..HEAD -- internal/mcpclient cmd internal/tool internal/agent internal/toolname internal/envinterp go.mod go.sum` → `exit=0`.
    - `rg -n "Tools \[\]string" pkg/runner/options.go` → the field (R1).
    - `rg -n "replaces|sub-agents|shadow|MCP tools" pkg/runner/options.go` → each doc claim present (R10).
    - `rg -n "Options.Tools" CHANGELOG.md` → one bullet (R10).
    - `git diff de43f3c..HEAD -- pkg/runner/run.go | rg '^-' | rg -v '^---'` → exactly three removed lines: `if len(cfg.Tools) > 0 {`, `resolvedTools, resolveErr := registry.Resolve(cfg.Tools)`, and `Tools:           cfg.Tools,`. Any other removed line is a failure; report it.
    - `git diff de43f3c..HEAD -- pkg/runner/options.go | rg '^-' | rg -v '^---'` → no output (additions only).
    - `git diff de43f3c..HEAD -- pkg/runner/run.go | rg -n 'registry.Resolve\(effectiveTools\)|Tools:           effectiveTools,|unknown tool %q in Options.Tools'` → one match each.
  - Step 3 (spec and checklist lint, one call): `~/.agents/scripts/spec-lint --root . docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md docs/plans/052_runner_tool_hooks_v1_12_0_007_implement.md; echo "exit=$?"` → `exit=0`.
  - Step 4: `~/.agents/scripts/docs-lint --since de43f3c .; echo "exit=$?"` → `exit=0`.
  - No file changes unless step 1 applies.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files (unless step 1 applies); the derived package list is empty.

- [ ] **T9: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Owner: orchestrator, after T8.
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_007_checkpoint.md` (phase, task states, gate evidence). The milestone entry `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:217-226` may receive a line-number-only citation refresh. The milestone checkbox `- [ ] 007` is rendered by factory-checkpoint at the Factory completion gate, after review, not in T9.
  - This writer does not touch these files.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 `Options.Tools []string` field | T1 (declare), T8 (rg) |
| R2 validation before the dry-run return | T2.1 (2), T2.4, T3.1, T5 |
| R3 TOML validation unchanged | T4 (`./internal/agent` unchanged, `git diff --exit-code`) |
| R4 effective list replaces the TOML list | T2.1 (1), T3.2 |
| R5 empty non-nil list: no `tools` key | T2.2 (3), T3.2 |
| R6 dry run reports the effective list | T2.1 (1 dry run), T3.3 |
| R7 nil is byte-identical | T2.2 (4, guard), T3.2 (nil path), T3.3, T4 |
| R8 CLI, TOML, `--json` unchanged | T4 (`./cmd`), T8 (`git diff --exit-code` on `cmd`) |
| R9 MCP shadowing follows the effective list | T2.2 (5a guard, 5b red), T3.2, T3.4 |
| R10 documentation | T1 (doc comment), T7.1 (CHANGELOG), T8 |
| R11 caller data and sub-agents | T2.1 (mutation), T2.3 (6), T3.2 (copy), T3.4 |
| R12 sub-agents do not receive the list | T2.3 (6), T3.2, T3.4 |
| R13 acceptance tests 1-6 | Acceptance Test Mapping below |

## Acceptance Test Mapping (one red task and one green task per test)

| Acceptance test | Test function | Red task | Green task | Note |
|---|---|---|---|---|
| 1 non-nil list replaces TOML; request has only those tools; mutation; dry-run `DryRunInfo.Tools` | `TestTools_OptionReplacesTOMLList` | T2.1 | T3.2 (request, mutation), T3.3 (dry run) | Red at T3.1 and T3.2 on the dry-run assertion only. |
| 2 unknown name `ConfigError`: `bogus`, `empty`, `call_agent`; live and dry; zero requests | `TestTools_UnknownNameConfigError` | T2.1 | T3.1 | |
| 3 empty non-nil list: no `tools` key; `Run` succeeds | `TestTools_EmptyListNoToolsKey` | T2.2 | T3.2 | |
| 4 nil with TOML tools: request carries TOML tools | `TestTools_NilUsesTOMLList` | T2.2 (`[guard]`, PASS at HEAD) | none (guard; stays PASS) | Guard, R7. |
| 5a builtin kept: one built-in `read_file`, MCP skipped | `TestTools_MCPShadowingFollowsEffectiveList/a_builtin_kept` | T2.2 (`[guard]`, PASS at HEAD) | none (guard; stays PASS) | Guard. |
| 5b builtin removed: MCP `read_file` registered and called | `TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed` | T2.2 | T3.2 | Red at HEAD (TOML built-in shadows the MCP tool). |
| 6 sub-agent does not inherit the list | `TestTools_SubAgentNotInherited` | T2.3 | T3.2, confirmed T3.4 | |

The spec's focused run is `TestTools_` in `pkg/runner` (T4, T5). `make apidiff` is T7. The race run is T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures: none. The change is one added field on `Options`.
  - `runner.Options` (`pkg/runner/options.go:16`): gains `Tools`. The keyed literal at `cmd/run.go:80` compiles unchanged, and the CLI does not set the field (R8).
  - `toolname.ValidNames()` (`internal/toolname/toolname.go:20`): signature unchanged. Callers: `internal/agent/agent.go:176` (unchanged), `pkg/runner/run.go` (new, T3.1).
  - `registry.Resolve` (`internal/tool/registry.go:60`): signature unchanged. Callers: `pkg/runner/run.go:446` (edited in T3.2 to `effectiveTools`); `internal/tool/tool.go:267` (sub-agent resolution from the child's own TOML, unchanged; OS8).
  - `agent.Load` and `agent.Validate` (`internal/agent/agent.go:148`): unchanged (OS5).
  - The exported entry point (`func Run`, line 47 of `pkg/runner/run.go`) and the internal entry point (`func run`, line 64 of `pkg/runner/run.go`): signatures unchanged.
  - `executeToolCalls` (`pkg/runner/run.go:953`): signature unchanged; the option list is not passed to it (OS8).
- Extreme acceptance inputs and probe evidence (read-only, at writing time):
  - Empty string `""`: `rg -n '""' internal/toolname/toolname.go` → no output (`exit=1`), so `""` is not a key in `ValidNames`. Expected: `ConfigError` (T2.1 subtest `empty`, T3.1). Verified at writing time.
  - `call_agent`: `internal/toolname/toolname.go:7` defines the constant, and `:19` documents that it is excluded; the `ValidNames` map (`:20-30`) does not list it. Expected: `ConfigError` (OS2; T2.1 subtest `call_agent`). Verified at writing time.
  - Case difference (`Read_File`): map lookup is exact (`pkg/runner/run.go` validation, T3.1; `internal/toolname/toolname.go:20-30`). Expected: `ConfigError`. No case folding is added. Not asserted by a test; recorded so the implementer does not add folding.
  - Empty non-nil `[]string{}`: `registry.Resolve` returns an empty slice for zero names (`internal/tool/registry.go:61-63`), and the T3.2 gate `len(effectiveTools) > 0` skips resolution. Expected: no built-in tools, no `tools` key (R5). Covered by T2.2 (3).
  - nil `Options.Tools`: the range loop in T3.1 is a no-op and `effectiveTools` is `cfg.Tools`. Expected: byte-identical to HEAD (R7). Covered by T2.2 (4) and the existing `TestRun_NoSubAgents_NoTools` (`cmd/run_test.go:1007`) and `TestRun_SingleShot_NoTools` (`pkg/runner/run_test.go:249`).
  - Duplicate names (`["read_file", "read_file"]`): accepted (OS10). `registry.Resolve` appends one definition per entry (`internal/tool/registry.go:65-75`). Expected: accepted; the request may carry a duplicate definition, as TOML does today. Not asserted by a test; recorded as a Limitation.
  - Very long name (for example 10,000 characters): `ValidNames` has no length rule, so the name is simply not a key. Expected: `ConfigError` (unknown name). Not probed; the result follows from map membership.
- `go install` or `GOPROXY=` commands: T7 runs `make apidiff`, which runs `go run` on a pinned module (`Makefile:3`, `:22-23`). The `go-toolchain-check` pre-flight is recorded in T0 (`exit=0`).
- `./.agents/scripts/checklist-test-gate-audit`: absent (T0, `exit=1`). Not run.

## Blockers

None. The spec was reviewed and approved at `ba1feb7` (checkpoint). The one citation drift is recorded in Assumptions and does not change behavior. The acceptance-test fixture gap for test 5 is resolved by Assumption 2 and does not change the stated outcomes.

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T3.1 → T3.2 → T3.3 → T3.4 → T3.5 → T4 → T5 → T6 → T7 → T7.1 → T8 → T9. The red tests share one new file, and the green tasks share `pkg/runner/run.go`.
- Parallel: T7.1 (CHANGELOG) touches no Go file and may be drafted during Phase 2 or 3, but it is committed in order after T7.
- Parallel: T1's doc comment and the T7.1 bullet are independent of each other; both are documentation.
- No removal tasks. Nothing is deleted; `cfg.Tools` reads remain (OS5, the TOML path), so no `Depends on reads:` line is needed.

## Assumptions

1. The validation wording (A1) and the effective-list copy (A2) are as stated in Section 1. Neither changes behavior that the spec fixes.
2. Acceptance test 5's agent TOML is `tools = ["read_file"]` (A3). The spec gives the MCP server and the two option values but not the TOML list. Without the TOML built-in, sub-case (b) would not be red at HEAD and would not test shadowing. The spec's R9 ("a built-in that is in the effective list shadows a same-named MCP tool"; "a built-in that is not in the effective list does not") presupposes the TOML built-in. This checklist adopts that reading. If the orchestrator decides otherwise, record it before T2.2.
3. The spec's OS7 and its Section 1 cite line 454 of `pkg/runner/run.go` for the call_agent gate. At HEAD `ba1feb7`, line 454 holds `depth := 0`. The gate is on line 455 and the injection on line 456 (range `pkg/runner/run.go:453-457`). This checklist cites line 455. The spec is not edited (orchestrator record). The drift does not change behavior, so it is reported and not a blocker.
4. The milestone entry (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:217-226`) still cites the stale lines `pkg/runner/run.go:413-419`, `pkg/runner/run.go:423`, and `internal/agent/agent.go:176-180`. The spec's corrected ranges (`pkg/runner/run.go:441-451`, `pkg/runner/run.go:453-457`, `internal/agent/agent.go:176-181`) are used here. The milestone refresh belongs to T9.
5. Red tasks run `go test -race -v ./pkg/runner` and treat the recorded FAIL set as the gate. The race run is required green from T3.3, as in 006.
6. The derived race package list is `./pkg/runner` only. Milestone 007 changes no file in `internal/agent` or `internal/envinterp`.
7. Test fixtures follow `runHookTestMCP` (`Workdir: t.TempDir()`, `Stdin: strings.NewReader("")`). Tests that need tool calls use `testutil.NewMockLLMServer`.
8. Compact JSON: Go `encoding/json` emits `"name":"read_file"` without spaces, so the count in T2.2 (5a, 5b) matches the request body. The implementer confirms this on the first red run.

## Limitations

- `call_agent` in `Options.Tools` is an unknown name and is a `ConfigError`, which matches TOML. A host controls `call_agent` through `sub_agents`.
- TOML `tools` are still validated at load even when `Options.Tools` overrides them (OS5). An unknown TOML tool still fails.
- Duplicate names in `Options.Tools` are accepted, as in TOML (OS10). The request may carry a duplicate definition.
- The wording of the `Options.Tools[i]` message is not a stable public contract (A1).
- The CLI cannot set `Options.Tools`. The CLI regression check is the existing no-tools and TOML tests (R8), not a new CLI test.
- `cmd/` tests need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005 and 006.
- The MCP shadowing change (OS6) is visible only when a host removes a built-in whose name matches an MCP tool. It is recorded in the doc comment and CHANGELOG (R10).
- The full race run over all packages, `make check`, and `checklist-test-gate-audit` (absent here) are not run by this checklist. The Factory completion gate runs the full race run.
