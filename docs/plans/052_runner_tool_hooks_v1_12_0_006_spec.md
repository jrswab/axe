# Milestone 006: Pass MCP servers from code with `Options.MCPServers`

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 006 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203-215`). Scope and acceptance tests 1-7 are taken from that entry, as bounded by D8 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:70`) and the compatibility rules D1 and D2.
- Predecessors: milestone 002 (apidiff gate) and milestone 005 (`Options.ToolObserver`) are complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154`, `[x] 002`; `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:189`, `[x] 005`). `Makefile:14` defines the `apidiff` target that `make check` runs (`Makefile:12`).
- Baseline: branch `runner-tool-hooks`, HEAD `a13e3bcf8b914fad1d17692da00e4ff79eca05bd`, clean tree at spec time. Milestones 004 and 005 are committed and reflected in the current code (`pkg/runner/options.go:29-37`, `pkg/runner/run.go:37-41`).
- Lane: full. The milestone adds an exported type and an exported field to `pkg/runner`, the external API (D1). It moves credential-bearing headers (`Headers`) through a new input path, and it changes config validation. The fast-lane conditions "external contracts untouched" and "no security-sensitive path" both fail.

### Citation corrections (verified at HEAD `a13e3bc`)

The milestone entry cites two lines that do not match current code. This spec uses the verified lines. The orchestrator must decide whether the milestone document is corrected; this spec does not edit it.

- The unset-env text match the milestone places at `pkg/runner/run.go:443` is at `pkg/runner/run.go:447` (`strings.Contains(connErr.Error(), "environment variable")`). `:443` is the verbose connect `Fprintf`.
- The dry-run MCP server list the milestone places at `pkg/runner/run.go:296-300` is at `pkg/runner/run.go:300-303`. The dry-run block itself is `pkg/runner/run.go:283-334`.

### Decisions settled (record as settled, never re-evaluate)

- **D8 (milestone).** `Options.MCPServers` is merged with the TOML servers. A duplicate name across both sources is a `ConfigError`. One shared validator runs before `DryRun` returns. The sentinel `ErrUnsetEnv` is internal, wrapped with `%w`, and the runner maps it with `errors.Is`. The text match stays for one release. Dry run shows merged names and URLs, never headers (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:70`).
- **D1 and D2.** Additions only. The field is nil by default, and the zero value keeps today's behavior (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54-55`). `COMPATIBILITY.md:24` says new fields may be added.
- **Q8 (milestone).** Code-supplied servers are the path for nightshift. Generated TOML is the workaround only (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:88`).
- **Q5 (milestone).** No per-server tool filter (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:87`).
- **OS1 (this spec). Merge order: TOML servers first, then `Options.MCPServers` in slice order.** The effective list is `cfg.MCPServers` followed by the option list. Evidence: the connect loop iterates `cfg.MCPServers` in order (`pkg/runner/run.go:441`), and the router registers tools in connect order (`internal/mcpclient/router.go:22-39`). Putting TOML first keeps the existing order unchanged when the option list is empty (R13).
- **OS2 (this spec). Duplicate names are compared exactly.** Names are compared as the raw strings the validator already uses (`internal/agent/agent.go:183-184`, `:204-206`), with no case folding and no trimming for the comparison. Duplicates are rejected both within `Options.MCPServers` and across the two sources. Two distinct names from the two sources both appear (milestone test 2).
- **OS3 (this spec). One validator, in `internal/agent`.** The rules at `internal/agent/agent.go:183-208` move into one exported function in `internal/agent` that both `Validate` (`internal/agent/agent.go:148`) and the runner call. `pkg/runner` already imports `internal/agent` (`pkg/runner/run.go:16`), so no new dependency is added. The TOML error text is unchanged (R13). Evidence for exact text: `internal/agent/agent_test.go:1082`, `:1100`, `:1118`.
- **OS4 (this spec). Sub-agents do not inherit `Options.MCPServers`.** A child agent loads its own TOML (`internal/tool/tool.go:152`) and connects only its own `cfg.MCPServers` (`internal/tool/tool.go:291-292`). `AGENTS.md:187` states "Independent context - Sub-agent loads its own TOML ... No context inheritance except the task string." A parent's Bearer header passed to a child would widen the credential's audience, so inheritance is rejected. This is a security decision settled by that evidence.
- **OS5 (this spec). Error text of `ErrUnsetEnv` is unchanged.** The current message is `environment variable %q is not set or empty` (`internal/envinterp/envinterp.go:47`), and the header wrap is `header %q: %w` (`internal/envinterp/envinterp.go:21`). Existing tests compare the full text (`internal/envinterp/envinterp_test.go:85`, `:98`), so the sentinel must be matchable without changing `Error()`.
- **OS6 (this spec). Dry run does not connect and does not expand headers.** Dry run returns before any MCP connect (`pkg/runner/run.go:283`, `:334`, connect at `:445`). Header env vars are not read on a dry run, today or for TOML servers. This stays true for code-supplied servers (R8).
- **OS7 (this spec). Transport values are the existing two.** Valid transports are `sse` and `streamable-http` (`internal/agent/agent.go:201`), and `mcpclient.Connect` maps only these (`internal/mcpclient/mcpclient.go:34-40`). No transport is added.
- **OS8 (this spec). Headers are never printed.** The verbose connect line prints the name, URL, and transport only (`pkg/runner/run.go:443`). `DryRunInfo.MCPServers` is formatted as `name: url (transport)` (`pkg/runner/run.go:302`). Headers are not added to either.
- **OS9 (this spec). `internal/mcpclient` is unchanged.** `mcpclient.Connect` takes `agent.MCPServerConfig` (`internal/mcpclient/mcpclient.go:25`). The runner converts each `MCPServer` to that type. Header expansion already runs inside `Connect` (`internal/mcpclient/mcpclient.go:25-28`), so one code path expands both sources.
- **OS10 (this spec). CHANGELOG.** One `[Unreleased]` `Added` bullet for `Options.MCPServers` is in scope (`CONTRIBUTING.md:185`, as for 005 OD15). It is the only `CHANGELOG.md` change.

### Rejected approaches (settled)

- Showing headers in `DryRunInfo` or verbose output. Rejected because headers can carry credentials (D8, OS8).
- An exported `ErrUnsetEnv` in `pkg/runner`. Rejected because the milestone makes the sentinel internal (D8).
- Changing the `mcpclient.Connect` signature (OS9).
- Passing the option list to sub-agents (OS4).
- Removing the text match at `pkg/runner/run.go:447` in this milestone (D8, one release).
- A per-server tool filter (Q5) or generated TOML as the primary path (Q8).

### Constraints and assumptions

- **Constraint, compatibility.** The additions are `MCPServer`, `Options.MCPServers`, and the `internal/agent` validator function. The apidiff gate (`Makefile:14`) compares `pkg/runner` against `v1.11.0`, and additions do not fail it.
- **Constraint, public types.** `MCPServer` uses only `string` and `map[string]string`, with no `internal/` type in its signature (D1).
- **Constraint, error classification.** Connect failures from an unset header variable are `ConfigError`. All other connect failures stay `RuntimeError` (`pkg/runner/run.go:476-478`).
- **Constraint, header secrets.** A header value may contain a literal credential. No error, verbose line, or dry-run field may contain a header value or header text other than the header's name (R9, R10).
- **Assumption, test environment.** Acceptance tests for header expansion must set the referenced environment variable. They use `t.Setenv`, as existing tests do (`internal/mcpclient/mcpclient_test.go:59`, `pkg/runner/run_test.go:264`). The milestone's "no `t.Setenv`" describes what a host does, not the test harness. Tests that call `t.Setenv` cannot use `t.Parallel`.
- **Assumption, URL secrets.** A URL that contains a secret in its query string is printed in the dry run and verbose output, as TOML URLs are today. This milestone does not change URL printing (Limitations).

## Section 2: Requirements

Requirements R1-R5 are the public API and merge rules. R6-R9 are validation, expansion, and error behavior. R10-R12 are output, mutation, and sub-agent boundaries. R13-R14 are the compatibility and documentation contract. Acceptance tests are in R15.

### R1: Public type and option field

- `pkg/runner` gains `type MCPServer struct` with exported fields `Name string`, `URL string`, `Transport string`, and `Headers map[string]string`. The fields match `agent.MCPServerConfig` (`internal/agent/agent.go:114-119`). `Headers` uses the same `${VAR}` syntax.
- `Options` gains one field `MCPServers []MCPServer`. Its doc comment states that a nil or empty value adds no servers, that the servers are merged with the agent's TOML servers, and that dry runs do not read header variables (OS6).
- Verification: `go build ./...` exits 0. `rg -n "MCPServer\b|MCPServers" pkg/runner/options.go` shows the type and the field.

### R2: Merged server set

- The effective server set is the agent's TOML servers in file order, followed by `Options.MCPServers` in slice order (OS1).
- The effective set is used for three things: the dry-run list (R10), the connect loop, and tool registration. No other path changes.
- Verification: acceptance test 1 (R15) and test 6 (R15) assert the order.

### R3: Validation before any connect and before dry-run return

- The shared validator (OS3) runs on the effective set after the agent is loaded (`pkg/runner/run.go:71-74`) and before the dry-run return (`pkg/runner/run.go:283`).
- A failure is `ConfigError` (`pkg/runner/errors.go:11`). It happens before the first `mcpclient.Connect` call (`pkg/runner/run.go:445`) and before any server is contacted, including TOML servers that are valid.
- Validator rules are the rules at `internal/agent/agent.go:185-206`: name non-blank after trimming, URL non-blank, URL parses with `url.ParseRequestURI` after trimming, scheme is `http` or `https`, host is non-empty, transport is `sse` or `streamable-http`, and the name is unique (OS2).
- A TOML source that fails today still fails at load (`internal/agent/agent.go:270`) with the same message (R13).
- Verification: acceptance test 3 (R15) covers invalid cases for dry run and live run, and asserts the valid TOML server receives zero requests when an option server is invalid.

### R4: Duplicate names are `ConfigError`

- A name that appears twice in the effective set is a `ConfigError`. This covers a duplicate within `Options.MCPServers`, and a duplicate across the TOML servers and `Options.MCPServers` (OS2).
- Two distinct names, one from each source, both appear in the request (milestone test 2).
- Verification: acceptance test 2 (R15).

### R5: Tools from both sources

- A server that appears only in `Options.MCPServers` contributes its tools to the request, with the same registration rules as a TOML server: built-in names win, and a cross-server collision is an error (`internal/mcpclient/router.go:26-32`).
- Verification: acceptance tests 1 and 2 (R15).

### R6: Transport and URL errors from the option list

- An invalid option entry (bad transport, bad scheme, empty URL, no host, empty name) is a `ConfigError` that names the source and the index, for example `Options.MCPServers[0]`. The message does not include headers (R10).
- The source label for TOML entries stays `mcp_servers[%d]` (R13).
- Verification: acceptance test 3 (R15) and the validator unit test `TestValidateMCPServers_SharedRules` (R15).

### R7: Header expansion from both sources

- `Headers` values are expanded with `${VAR}` exactly as TOML headers are (`internal/envinterp/envinterp.go:12-27`). The expanded value is the value the server receives.
- Expansion for a code-supplied server runs in `mcpclient.Connect` (`internal/mcpclient/mcpclient.go:25-28`), so it runs only when the server is connected, not on dry run (OS6).
- Verification: acceptance test 5 (R15).

### R8: Unset header variable is `ConfigError`, matchable by sentinel

- A header value that references a variable that is unset or empty (`internal/envinterp/envinterp.go:46`) is a `ConfigError` for both sources. The error chain contains the sentinel `ErrUnsetEnv` in `internal/envinterp`, and `errors.Is(err, envinterp.ErrUnsetEnv)` is true for the error returned by `Run`.
- `ErrUnsetEnv` is wrapped with `%w` (`internal/envinterp/envinterp.go:21` already wraps the header key). The `Error()` text is unchanged (OS5).
- The runner keeps the text match at `pkg/runner/run.go:447` for this release. An error that matches either the sentinel or the text is a `ConfigError` (D8).
- No error text contains the variable's value or any part of the header value other than the header key (R10).
- The dry run does not read header variables and does not fail on them (OS6).
- Verification: acceptance test 4 (R15) and `TestExpandHeaders_UnsetEnvSentinel` (R15).

### R9: Connect failures for other causes are unchanged

- Connect failures that are not an unset variable or an unsupported transport stay `RuntimeError` with the existing message `failed to connect MCP server %q` (`pkg/runner/run.go:450`).
- Verification: the existing `TestConnect_UnreachableServer` (`internal/mcpclient/mcpclient_test.go:47`) passes unchanged, and acceptance test 4 covers the `ConfigError` branch.

### R10: No header text in any output

- No dry-run field, verbose line, `Result`, or error from `Run` contains a header value. The only header text that may appear is a header key in the `ErrUnsetEnv` message that the existing expansion already writes (`internal/envinterp/envinterp.go:21`).
- The verbose connect line prints the name, URL, and transport only (OS8, `pkg/runner/run.go:443`).
- Verification: acceptance tests 4 and 7 (R15) assert that a secret literal placed in a header value is absent from every output.

### R11: Caller data is not modified

- `Run` does not modify `Options.MCPServers`, the slice, or any `Headers` map in it. Expansion produces a new map inside `ExpandHeaders` (`internal/envinterp/envinterp.go:28-33`), called from `mcpclient.Connect`.
- Verification: acceptance test 8 (R15).

### R12: Sub-agents do not receive the option list

- A `call_agent` child connects only its own TOML servers (OS4). A server that exists only in `Options.MCPServers` is not connected for the child and its tools do not appear in the child's request.
- Verification: acceptance test 9 (R15).

### R13: TOML and CLI behavior is unchanged

- With `Options.MCPServers` nil or empty, behavior, dry-run output, error text, and tool order are byte-identical to HEAD `a13e3bc`.
- The TOML error messages in `internal/agent/agent.go:185-206` are unchanged, so `TestValidate_MCPServers_*` (`internal/agent/agent_test.go:1055-1258`) pass without edits.
- `cmd/` and `internal/mcpclient/` are not changed (OS9).
- Verification: acceptance test 7 (R15) and the existing tests named there.

### R14: Doc comments and CHANGELOG

- The `MCPServer` type and the `Options.MCPServers` field have doc comments that state: merge order (OS1), duplicate-name rule (OS2), headers are credentials and are never printed (R10), dry run does not read header variables (OS6), and sub-agents do not inherit the list (OS4).
- One `[Unreleased]` `Added` bullet in `CHANGELOG.md` describes `Options.MCPServers` (OS10).
- Verification: `rg -n "never printed|merged|sub-agents|dry run" pkg/runner/options.go` shows each claim. `rg -n "Options.MCPServers" CHANGELOG.md` shows the bullet.

### R15: Acceptance tests

Test placement: pkg/runner tests go in a new file, `pkg/runner/mcp_servers_test.go`. The existing MCP hook helper `newHookTestMCPServer` (`pkg/runner/hook_mcp_test.go:17`) does not capture request headers, so this file adds its own helper that wraps the same `mcp.NewStreamableHTTPHandler` (`pkg/runner/hook_mcp_test.go:44`) and records the `Authorization` header. Tests use `setupTestEnv` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr` (`pkg/runner/run_test.go:44`), and `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), so none of them can use `t.Parallel`. Mock responses use `AnthropicToolUseResponse` then `AnthropicResponse`, as in `runHookTestMCP` (`pkg/runner/hook_mcp_test.go:50-88`).

1. **`TestMCPServers_OptionsOnlyToolsInRequest`.** The agent TOML has `tools = []` and no `mcp_servers`. `Options.MCPServers` has one server whose tool is `mcp_echo`. Assert `Run` succeeds and `mock.Requests[0].Body` contains `mcp_echo` (R2, R5).
2. **`TestMCPServers_MergedDistinctNames`** and **`TestMCPServers_DuplicateNameConfigError`.**
   - 2a (merged): the TOML server `local-mcp` exposes `mcp_echo`, and the option server `code-mcp` exposes `mcp_other`. Assert both tool names appear in `mock.Requests[0].Body`, and the dry-run list order is `local-mcp` then `code-mcp` (R2, R4).
   - 2b (duplicate): a table of two cases. (i) TOML and option both named `local-mcp`. (ii) two option entries with the same name. Assert `IsConfigError(err)` is true for both (R4).
3. **`TestMCPServers_InvalidServerNoConnect`.** A table of invalid option entries: transport `ws`, scheme `ftp`, empty URL, empty name, and a URL with no host. Each case runs as a live run and as a dry run (`Options.DryRun`). Assert `ConfigError` for each. In the live cases, the TOML server is valid, and its counting server records zero requests. The test server for the option entry records zero requests too (R3, R6).
4. **`TestMCPServers_UnsetHeaderEnvConfigError`.** Table: header `Authorization: Bearer ${TOKEN_UNSET}` with the variable unset, and the same with the variable set to empty. Assert `IsConfigError(err)`, `errors.Is(err, envinterp.ErrUnsetEnv)`, and that `err.Error()` does not contain the literal `secret-literal-value`, a secret placed in a second header value of the same server (R8, R10). Also run the same case from a TOML source to show parity.
5. **`TestMCPServers_HeaderExpandedOnWire`.** The test sets `MCP_TEST_TOKEN` with `t.Setenv` to `tok-123`. The option server's `Headers` is `{"Authorization": "Bearer ${MCP_TEST_TOKEN}"}`. The helper server records `Authorization`. Assert the recorded value is `Bearer tok-123` for every request, and that the server's tools appear (R7).
6. **`TestMCPServers_DryRunMergedNoHeaders`.** A dry run with one TOML server and one option server, whose header references an unset variable. Assert no error, `DryRunInfo.MCPServers` equals `["local-mcp: <url> (streamable-http)", "code-mcp: <url> (streamable-http)"]`, and the result contains no header value or header key (R2, R8, R10).
7. **`TestMCPServers_EmptySliceUnchanged`.** A table of `nil` and `[]MCPServer{}`. Each runs the TOML-only scenario from `TestToolHookMCPAllow` (`pkg/runner/hook_mcp_test.go:118`) and asserts the request, dry-run info, and error values match the TOML-only result. Also run the existing tests named here unchanged: `TestValidate_MCPServers_*` (`internal/agent/agent_test.go:1055-1258`), `TestParseTOML_MCPServers` (`internal/agent/agent_test.go:1277`), `TestConnect_HeaderEnvVarMissing` (`internal/mcpclient/mcpclient_test.go:88`), `TestToolHookMCPDeny` (`pkg/runner/hook_mcp_test.go:90`), and `TestRunCmd_DryRun_WithMCPServers` (`cmd/run_test.go:289`) (R13).
8. **`TestMCPServers_RunDoesNotMutateOptions`.** Build the option slice and headers, deep-copy them, run the live scenario, and assert the original slice and maps are equal to the copy (R11).
9. **`TestMCPServers_SubAgentNotInherited`.** The parent TOML has `sub_agents = ["child-agent"]` and no servers. The parent has `Options.MCPServers` with `mcp_parent_only`. The parent calls `call_agent`, and the child TOML has no servers and returns text. Assert `mock.Requests[1].Body` (the child request) does not contain `mcp_parent_only`, and the run succeeds (R12).
10. **`TestMCPServers_VerboseNoHeaders`.** `Verbose` is true. The header value is `Bearer secret-literal-value`. Assert the stderr buffer contains the server URL and does not contain `secret-literal-value` (R10).
11. **`TestExpandHeaders_UnsetEnvSentinel`** (`internal/envinterp/envinterp_test.go`, new). Table: unset and empty. Assert `errors.Is(err, ErrUnsetEnv)` and that `err.Error()` equals the current text (`internal/envinterp/envinterp_test.go:85`, `:98`) exactly (R8, OS5). `TestExpandHeaders_MissingVar` and `TestExpandHeaders_EmptyVar` run unchanged.
12. **`TestValidateMCPServers_SharedRules`** (`internal/agent/agent_test.go`, new). Table over the rules in R3 and R6 for the shared function, with `source` and index labels. Assert each TOML message equals the existing text, and each option message names `Options.MCPServers[i]`, and no message contains a header value (R3, R6, R13).

Focused run: after tests 1-12 exist, a focused run selecting the TestMCPServers, TestExpandHeaders, and TestValidateMCPServers families in the packages pkg/runner, internal/envinterp, and internal/agent must exit 0. It is not in the fenced block because spec-lint reports "selects no test" until the tests are written.

## Parallelizable work

- The `envinterp` sentinel (R8) and the shared validator in `internal/agent` (R3, R6) have no dependency on each other and can be written in parallel.
- The `MCPServer` type and field (R1) and the doc comments (R14) are written together.
- The CHANGELOG bullet (R14) is independent of the code.
- Tests 1-10 share one new file and are written after the wiring.

## Serial dependencies

- R1 before R2-R12 (the type and field must exist).
- The sentinel (R8) and validator (R3, R6) before the runner wiring that calls them (R2, R3, R7, R8).
- R2-R12 before R15 (tests exercise the wiring).

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                     # expect: exit 0
go vet ./pkg/runner/ ./internal/agent/ ./internal/envinterp/      # expect: exit 0
go test ./internal/agent ./internal/envinterp ./internal/mcpclient ./pkg/runner -count=1   # expect: exit 0
go test ./cmd -count=1                                             # expect: exit 0 (see Limitations for ~/.cache/axe)
make apidiff                                                       # expect: exit 0, no incompatible-change output
rg -n "ErrUnsetEnv" internal/envinterp/envinterp.go                # expect: the sentinel declaration and the wrap
rg -n "ErrUnsetEnv|environment variable" pkg/runner/run.go         # expect: the errors.Is branch and the kept text match
git diff --name-only a13e3bc -- . ':(exclude)docs/plans/'          # expect: CHANGELOG.md, internal/agent/agent.go, internal/agent/agent_test.go, internal/envinterp/envinterp.go, internal/envinterp/envinterp_test.go, pkg/runner/mcp_servers_test.go, pkg/runner/options.go, pkg/runner/run.go
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since a13e3bc /home/jaron/dev/axe/runner-tool-hooks                                              # expect: exit 0
```

Focused run: see R15.

## Failure modes

1. **Headers printed in an error or verbose line.** A secret would reach logs. Mitigation: R10, OS8, tests 4 and 10.
2. **Option servers connected before validation.** A bad entry could still send a request. Mitigation: R3 places validation before the first connect, and test 3 asserts zero requests.
3. **Dry run reads header variables.** A host dry run would fail for a missing variable that the live run does not need yet. Mitigation: OS6 and test 6.
4. **Sentinel wrap changes the message.** Existing exact-text tests would fail, and callers that parse text would break. Mitigation: OS5 and test 11.
5. **Parent credentials reach a sub-agent.** Mitigation: OS4 and test 9.
6. **Caller slice or map mutated.** Mitigation: R11 and test 8.
7. **TOML order changes.** Tool registration order could change for TOML-only agents. Mitigation: OS1, R13, test 7.
8. **Cross-source name collision not caught.** Mitigation: R4 and test 2b.

## Limitations

- The milestone's "no `t.Setenv`" is a host property. Acceptance tests set environment variables with `t.Setenv` (Section 1, Assumption, test environment).
- A URL containing a secret in its query string is printed in dry-run and verbose output, as TOML URLs are today. This milestone does not change URL printing.
- The CLI cannot set `Options.MCPServers`. The CLI regression check is `TestRunCmd_DryRun_WithMCPServers` (`cmd/run_test.go:289`) and the existing TOML tests, not a new CLI test.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005.
- The wording of the new `Options.MCPServers[i]` messages is a decision of this spec, not a stable public contract.
