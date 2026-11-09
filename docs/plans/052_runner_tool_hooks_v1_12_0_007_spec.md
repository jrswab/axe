# Milestone 007: Limit built-in tools from code with `Options.Tools`

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 007 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:217-226`). Scope and acceptance tests 1-4 are taken from that entry, as bounded by D1, D2, D9, D14, and Q11 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54`, `:55`, `:71`, `:76`, `:128`).
- Predecessor: milestone 006 (`Options.MCPServers`) is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203`, `[x] 006`). The milestone entry declares `Depends on: 006 (MCP tools remain unaffected by this list)`.
- Baseline: branch `runner-tool-hooks`, review base `de43f3c8c0cea487f35283280fd33ea52d087dc6` (`docs/plans/052_runner_tool_hooks_v1_12_0_006_checkpoint.md` records 006 as committed at `7cc7ccf`, and `de43f3c` is the 006 completion commit). Clean tree at spec time.
- Lane: full. The milestone adds an exported field to `pkg/runner` (the external API, D1). It adds a second validation path for a caller-supplied list. It changes which MCP tool names are shadowed (a security-relevant change to MCP shadowing). No new dependency is added. The fast-lane condition "external contracts untouched" fails.

### Citation corrections (verified at `de43f3c`)

The milestone entry cites two lines that do not match current code. This spec uses the verified lines. The milestone document is not edited by this spec.

- The TOML tool consumption the milestone places at `pkg/runner/run.go:413-419` is at `pkg/runner/run.go:452-462`. The gate `if len(cfg.Tools) > 0` is at `:456`, and `registry.Resolve(cfg.Tools)` is at `:457`. Lines `:413-419` are unrelated code in the same function.
- The call_agent reference the milestone places at run.go line 423 is at `pkg/runner/run.go:464-468`. The sub-agent gate is `:466`, and the injection is `:467`.
- The validation rule the milestone places at `internal/agent/agent.go:176-180` is at `internal/agent/agent.go:176-181` (the loop ends at `:181`).
- The dry-run return the milestone does not cite is at `pkg/runner/run.go:347` (`return &Result{`). The dry-run block opens at `pkg/runner/run.go:322`. `DryRunInfo.Tools` is set at `pkg/runner/run.go:363`.
- The MCP shadowing set is built at `pkg/runner/run.go:475-478` from `req.Tools`. The router skip is `internal/mcpclient/router.go:26-28`.

### Decisions settled (record as settled, never re-evaluate)

- **OS1 (this spec). Effective list is computed before the dry-run return.** The effective built-in list is `Options.Tools` when non-nil, else `cfg.Tools`. It is computed and validated before the dry-run return (`pkg/runner/run.go:322`). `DryRunInfo.Tools` reports the effective list (`pkg/runner/run.go:363`), the same pattern as 006 `DryRunInfo.MCPServers` (`pkg/runner/run.go:364`). The CLI cannot set `Options.Tools`, so CLI output and golden files are unchanged (R8).
- **OS2 (this spec). Validation uses the TOML rule.** Each `Options.Tools` name is checked with `toolname.ValidNames()` (`internal/toolname/toolname.go:20-30`), the same rule as `internal/agent/agent.go:176-181`. An unknown or empty name is a `ConfigError` that names `Options.Tools[i]` and the bad name. Wording is not a stable contract (Limitations). `call_agent` is not in `ValidNames` (`internal/toolname/toolname.go:17-19`), so `call_agent` in `Options.Tools` is an unknown name. This is parity with TOML, where `call_agent` is controlled by `sub_agents`.
- **OS3 (this spec). nil is the TOML list.** A nil `Options.Tools` uses `cfg.Tools` with byte-identical behavior (R7).
- **OS4 (this spec). Empty non-nil means no built-in tools.** `Options.Tools` set to an empty non-nil slice gives no built-in tools. When the agent has no `sub_agents` and no MCP tools, the request carries no `tools` key. This is the same shape as TOML `tools = []` (`cmd/run_test.go:1007`, `TestRun_NoSubAgents_NoTools`). The omission is produced by `omitempty` on the Anthropic request field (`internal/provider/anthropic.go:71`), set from `req.Tools` at `internal/provider/anthropic.go:257`.
- **OS5 (this spec). TOML tools are still validated at load.** `agent.Load` validates `cfg.Tools` (`internal/agent/agent.go:176-181`, called from `pkg/runner/run.go:88`) even when `Options.Tools` overrides them. This spec does not change `agent.Load`. A TOML file with an unknown tool still fails, even when a host overrides the list (Limitations).
- **OS6 (this spec). MCP shadowing follows the effective built-in list.** The `builtinNames` set is built from `req.Tools` (`pkg/runner/run.go:475-478`), and the router skips an MCP tool whose name is in that set (`internal/mcpclient/router.go:26-28`). Today a TOML-listed built-in shadows a same-named MCP tool. Under this milestone, a built-in removed by `Options.Tools` no longer shadows the MCP tool, and the MCP tool is registered and reachable. This is documented in the field doc comment and the CHANGELOG bullet (R9, R10).
- **OS7 (this spec). `Options.Tools` does not limit MCP tools or `call_agent`.** MCP tools are registered from the MCP servers, not from the built-in list (`pkg/runner/run.go:492-513`). `call_agent` is injected when `cfg.SubAgents` is non-empty (`pkg/runner/run.go:466`, injection at `:467`). Neither is filtered by `Options.Tools`.
- **OS8 (this spec). Sub-agents do not inherit `Options.Tools`.** A child agent loads its own TOML (`internal/tool/tool.go:152`) and resolves its own `cfg.Tools` (`internal/tool/tool.go:266-275`). The parent's option list is not passed to the child. This matches `AGENTS.md:187` ("Independent context - Sub-agent loads its own TOML ... No context inheritance except the task string"), as 006 OS4 settled for `Options.MCPServers`.
- **OS9 (this spec). The caller's slice is copied and not mutated.** `Run` does not modify `Options.Tools` or retain a reference to it. The effective list is a separate slice.
- **OS10 (this spec). Duplicate names are not rejected.** A name that appears twice in `Options.Tools` is accepted, as TOML `tools = ["read_file", "read_file"]` is accepted today (`internal/tool/registry.go:60`, `Resolve` appends one definition per entry). Parity with TOML. Recorded as a limitation.
- **OS11 (this spec). CHANGELOG.** One `[Unreleased]` `Added` bullet for `Options.Tools` is in scope, as 005 and 006 did (`CONTRIBUTING.md:185`). Milestone 012 builds the release section. It is the only `CHANGELOG.md` change.
- **OS12 (this spec). Test placement.** The tests go in a new file `pkg/runner/tools_option_test.go`. They use `setupTestEnv` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr` (`pkg/runner/run_test.go:44`), and `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`). The MCP shadowing test uses the `newMCPServersTestServer` helper (`pkg/runner/mcp_servers_test.go:22-57`) from 006. Tests that call `t.Setenv` or `setupTestEnv` cannot use `t.Parallel`.

### Rejected approaches (settled)

- Validating `Options.Tools` after the dry-run return. Rejected because a dry run would accept a list a live run rejects (OS1).
- Reporting the TOML list in `DryRunInfo.Tools` when `Options.Tools` is set. Rejected because the dry run must show what a live run uses (OS1).
- Rejecting `call_agent` in `Options.Tools` with a special message. Rejected because parity with TOML is the rule (OS2).
- Passing the option list to sub-agents (OS8).
- Filtering MCP tools or `call_agent` by `Options.Tools` (OS7, D9).
- Keeping a removed built-in as an MCP shadow. Rejected because the shadow set must follow what the request carries (OS6).
- Adding a CLI flag for the tool list. Rejected because the CLI cannot set `Options` fields for tools (OS1, R8).
- Changing `agent.Load` to skip TOML tool validation when the option is set (OS5).

### Constraints and assumptions

- **Constraint, compatibility.** The addition is `Options.Tools []string` (`pkg/runner/options.go`). The apidiff gate (`Makefile:14`) compares `pkg/runner` against `v1.11.0`, and additions do not fail it (D1, D2).
- **Constraint, behavior change gate.** New behavior is opt-in through the new field. The nil value keeps today's behavior (D2, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:55`).
- **Constraint, public type.** `Options.Tools` uses `[]string`. No `internal/` type appears in the signature.
- **Constraint, error classification.** An invalid `Options.Tools` name is `ConfigError` (`pkg/runner/errors.go:11`), as the milestone requires.
- **Assumption, MCP shadowing change.** OS6 changes which MCP tool name wins when a host removes a built-in. This is intended by the milestone's "MCP tools remain unaffected" and by the parity rule for shadowing. The change is only visible when `Options.Tools` removes a built-in that has the same name as an MCP tool. No TOML behavior changes.
- **Assumption, test environment.** Tests use `t.Setenv` for `AXE_ANTHROPIC_BASE_URL` and the Anthropic key, as existing tests do (`cmd/run_test.go:1031-1032`). They cannot use `t.Parallel`.

## Section 2: Requirements

Requirements R1-R3 are the field and validation. R4-R7 are the effective list, dry run, nil and empty behavior. R8-R10 are the compatibility, shadowing, and documentation contract. R11 is the sub-agent and mutation boundary. Acceptance tests are in R12.

### R1: Public field

- `pkg/runner` `Options` gains one field `Tools []string`. The doc comment states: nil uses the agent's TOML tool list; a non-nil list, including an empty one, replaces it; names are validated against the built-in set; the list does not limit MCP tools or `call_agent`; a built-in removed by the list no longer shadows a same-named MCP tool (OS6); sub-agents do not inherit the list (OS8).
- Verification: `go build ./...` exits 0. `rg -n "Tools \[\]string" pkg/runner/options.go` shows the field.

### R2: Validation before the dry-run return

- The names in a non-nil `Options.Tools` are checked against `toolname.ValidNames()` (OS2) after the agent is loaded (`pkg/runner/run.go:88-91`) and before the dry-run return (`pkg/runner/run.go:322`).
- A failure is `ConfigError` (`pkg/runner/errors.go:11`). It names `Options.Tools[i]` and the bad name. The name `""` is reported as unknown.
- Verification: acceptance test 2 (R12) covers live and dry runs.

### R3: TOML validation is unchanged

- `agent.Load` still validates `cfg.Tools` (OS5). An unknown TOML tool fails at load with the existing message (`internal/agent/agent.go:178-180`), even when `Options.Tools` is set.
- Verification: `go test ./internal/agent -count=1` exits 0 with no edits to `internal/agent`.

### R4: Effective list replaces the TOML list

- When `Options.Tools` is non-nil, the request's built-in tools are exactly the resolved entries of `Options.Tools`, in slice order. The TOML list is not resolved (`pkg/runner/run.go:456-461` runs on the effective list only).
- MCP tools and `call_agent` are added as before (OS7).
- Verification: acceptance test 1 (R12).

### R5: Empty non-nil list

- An empty non-nil `Options.Tools` gives no built-in tools. With no `sub_agents` and no MCP tools, the request body has no `tools` key (OS4). The request is sent without a tool loop, as TOML `tools = []` is today (`pkg/runner/run.go:594`).
- Verification: acceptance test 3 (R12).

### R6: Dry run reports the effective list

- `DryRunInfo.Tools` reports the effective list (OS1). For a nil option it equals the TOML list. For a non-nil option it equals `Options.Tools`.
- The CLI prints `DryRunInfo.Tools` (`cmd/run.go:188-190`). Because the CLI cannot set the option, its output is unchanged.
- Verification: acceptance test 1 asserts `DryRunInfo.Tools` in a dry run (R12).

### R7: nil is byte-identical to HEAD

- With `Options.Tools` nil, the request, dry-run output, error text, and tool order are unchanged from `de43f3c` (OS3).
- Verification: acceptance test 4 (R12), and the existing tests `TestRun_NoSubAgents_NoTools` (`cmd/run_test.go:1007`) and `TestRun_SingleShot_NoTools` (`pkg/runner/run_test.go:249`) pass unchanged.

### R8: CLI, TOML, and `--json` are unchanged

- No CLI flag, TOML key, `--json` field, or golden file changes. The CLI cannot set `Options.Tools`.
- Verification: `git diff --name-only de43f3c -- cmd internal/agent/agent.go internal/toolname internal/mcpclient` prints no paths. `go test ./cmd -count=1` exits 0 with `XDG_CACHE_HOME` set (Limitations).

### R9: MCP shadowing follows the effective list

- The shadowing set is built from the effective built-in list (OS6). A built-in that is in the effective list shadows a same-named MCP tool, and the MCP tool is skipped (`internal/mcpclient/router.go:26-28`). A built-in that is not in the effective list does not shadow, and the MCP tool is registered and reachable.
- Verification: acceptance test 5 (R12).

### R10: Documentation

- The `Options.Tools` doc comment states the rules in R1 (OS6 included).
- One `[Unreleased]` `Added` bullet in `CHANGELOG.md` describes `Options.Tools`, including the shadowing change (OS6) and the rule that the list does not limit MCP tools or `call_agent` (OS11).
- Verification: `rg -n "Options.Tools" pkg/runner/options.go CHANGELOG.md` shows the doc comment and the bullet. `rg -n "shadow" CHANGELOG.md` shows the shadowing sentence in the bullet.

### R11: Caller data and sub-agents

- `Run` does not modify `Options.Tools` or retain a reference to its backing array (OS9). The request's tool list is built from the resolved entries.
- A `call_agent` child resolves only its own TOML tool list (OS8). Tools from the parent's `Options.Tools` do not appear in the child's request.
- Verification: acceptance tests 1 (mutation assertion) and 6 (sub-agent) (R12).

### R12: Acceptance tests

Test file: `pkg/runner/tools_option_test.go` (new). Tests share the `mock` and `setupTestEnv` helpers, so none can use `t.Parallel`. Mock responses use `testutil.AnthropicToolUseResponse` and `testutil.AnthropicResponse`, as in `runHookTestMCP` (`pkg/runner/hook_mcp_test.go:50-88`).

1. **`TestTools_OptionReplacesTOMLList`.** The agent TOML has no `sub_agents`, no MCP servers, and `tools = ["list_directory", "read_file"]`. `Options.Tools` is `[]string{"write_file"}`. Assert `Run` succeeds, the request's tool names are exactly `write_file` (the body contains `"write_file"` and does not contain `"read_file"` or `"list_directory"`), and the caller's slice still holds `write_file` alone after the run (R4, R11). Also run a dry run with the same options and assert `DryRunInfo.Tools` equals `[]string{"write_file"}` (R6).
2. **`TestTools_UnknownNameConfigError`.** A table of `"bogus"`, `""`, and `"call_agent"`. Each case runs as a live run and as a dry run. Assert `IsConfigError(err)`, that `err.Error()` contains `Options.Tools[0]`, and that the mock server received zero requests (R2).
3. **`TestTools_EmptyListNoToolsKey`.** The agent TOML has `tools = ["read_file"]` and no `sub_agents` or MCP servers. `Options.Tools` is `[]string{}`. Decode the request body as JSON and assert the top-level object has no `tools` key. Assert `Run` succeeds (R5).
4. **`TestTools_NilUsesTOMLList`.** The same TOML as test 3, with `Options.Tools` nil. Assert the request body contains `"read_file"` (R7).
5. **`TestTools_MCPShadowingFollowsEffectiveList`.** The option server comes from `newMCPServersTestServer(t, "read_file")` (`pkg/runner/mcp_servers_test.go:28`), so its tool is named like the built-in `read_file`. The MCP tool's description is `Returns a greeting`, and the built-in `read_file` description starts with `Read the contents of a file` (`internal/tool/registry.go:97-105` registers the built-ins; the description is in `internal/tool/read_file.go`). Two sub-cases:
   - (a) `Options.Tools` is `[]string{"read_file"}`. The request has exactly one definition named `read_file`, and it is the built-in: the body does not contain `Returns a greeting`.
   - (b) `Options.Tools` is `[]string{"list_directory"}`. The request contains the MCP `read_file` definition (`Returns a greeting`) exactly once. The mock response calls `read_file` with input `{"name": "axe"}`, and the second request body contains `hello`, the MCP server's text result.
   - Both sub-cases run `Run` with no TOML MCP servers (R9).
6. **`TestTools_SubAgentNotInherited`.** The parent TOML has `sub_agents = ["child-agent"]`, no MCP servers, and `tools = []`. `Options.Tools` is `[]string{"write_file"}`. The child TOML has `tools = ["read_file"]` and no `sub_agents`. The parent calls `call_agent` with the child name; the mock returns the child's text. Assert `Run` succeeds, the parent request (`Requests[0]`) contains `"write_file"`, and the child request (`Requests[1]`) contains `"read_file"` and does not contain `"write_file"` (R11). The call order is parent (`Requests[0]`), child (`Requests[1]`), parent (`Requests[2]`); assert by index on the mock's recorded requests.

Focused run: after tests 1-6 exist, the focused run selecting the `TestTools_` family in package `pkg/runner` must exit 0. It is not in the fenced block because spec-lint reports "selects no test" until the tests are written.

## Parallelizable work

- The validator (R2) and the `Options` field and doc comment (R1, R10) are independent of each other.
- The CHANGELOG bullet (R10) is independent of the code.
- Tests 1-6 share one new file and are written after the wiring.

## Serial dependencies

- R1 before R2-R9 (the field must exist).
- R2 before R4-R6 (validation runs before the effective list is used).
- R4-R9 before R12 (tests exercise the wiring).

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                          # expect: exit 0
go vet ./pkg/runner/ ./internal/agent/ ./internal/toolname/           # expect: exit 0
go test ./internal/agent ./internal/toolname ./internal/mcpclient ./pkg/runner -count=1   # expect: exit 0
XDG_CACHE_HOME=$(mktemp -d) go test ./cmd -count=1                     # expect: exit 0 (see Limitations)
make apidiff                                                            # expect: exit 0, no incompatible-change output
rg -n "Tools \[\]string" pkg/runner/options.go                          # expect: the new field
rg -n "Options.Tools" CHANGELOG.md                                      # expect: one bullet under [Unreleased]
git diff --name-only de43f3c -- . ':(exclude)docs/plans/'               # expect: CHANGELOG.md, pkg/runner/options.go, pkg/runner/run.go, pkg/runner/tools_option_test.go
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since de43f3c /home/jaron/dev/axe/runner-tool-hooks                                              # expect: exit 0
```

Focused run: see R12.

## Failure modes

1. **Dry run accepts a list the live run rejects.** Mitigation: validation before the dry-run return (R2, OS1), test 2.
2. **nil option changes TOML behavior.** Mitigation: R7, test 4, the existing no-tools tests.
3. **Empty list sends a `tools` key or drops the tool loop incorrectly.** Mitigation: R5, test 3.
4. **MCP tool silently shadowed after a built-in is removed, or silently registered when it should be shadowed.** Mitigation: OS6, R9, test 5.
5. **Parent list leaks to a sub-agent.** Mitigation: OS8, R11, test 6.
6. **Caller slice aliased or mutated.** Mitigation: OS9, R11, test 1.
7. **A removed built-in still appears in the dry run.** Mitigation: OS1, R6, test 1.

## Limitations

- `call_agent` in `Options.Tools` is an unknown name and is a `ConfigError`. This matches TOML. A host controls `call_agent` through `Options.Tools`' absence and `sub_agents`.
- TOML `tools` are still validated at load even when `Options.Tools` overrides them (OS5). A TOML file with an unknown tool still fails.
- Duplicate names in `Options.Tools` are accepted, as in TOML (OS10). The request may carry a duplicate definition.
- The wording of the `Options.Tools[i]` messages is a decision of this spec, not a stable public contract.
- The CLI cannot set `Options.Tools`. The CLI regression check is the existing no-tools and TOML tests (R8), not a new CLI test.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005 and 006.
- The MCP shadowing change (OS6) is visible only when a host removes a built-in whose name matches an MCP tool. It is recorded in the doc comment and CHANGELOG (R10).
