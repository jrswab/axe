# Milestone 008: Override the turn cap per run with `Options.MaxTurns`

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 008 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:228-236`). Scope and acceptance tests 1-3 are taken from that entry, as bounded by the orchestrator decisions OS1-OS12 below.
- Predecessor: none. The entry declares `Depends on: none` (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:230`). Milestone 007 (`Options.Tools`) is complete and is the precedent for the validation and CHANGELOG pattern.
- Baseline: branch `runner-tool-hooks`, review base `7f56dae088de8c2ab4bfd03690375c9e77ec97c7`, PR `#103`. Clean tree at spec time.
- Lane: full. The milestone adds an exported field to `pkg/runner`, the external Go API. Public Go API is an external contract, so the fast-lane condition "external contracts untouched" fails.

### Citation corrections (verified at 7f56dae)

The milestone entry cites lines that do not match current code. This spec uses the verified lines. The milestone document is not edited by this spec.

- The loop bound the entry cites at `pkg/runner/run.go:587` is at `pkg/runner/run.go:630` (`for turn := 0; turn < maxConversationTurns; turn++ {`). Line `:587` is inside the streaming send closure (`pkg/runner/run.go:580-592`), not the loop.
- The default the entry cites at `pkg/runner/run.go:83-86` is at `pkg/runner/run.go:126-128` (`maxConversationTurns := cfg.MaxTurns`, then the `== 0` fallback). The constant `defaultMaxConversationTurns = 50` is at `pkg/runner/run.go:36-37`.
- The turn-limit `RuntimeError` the entry cites at `pkg/runner/run.go:695` is at `pkg/runner/run.go:760` (`agent exceeded maximum conversation turns (%d)`). The guard that selects it is `pkg/runner/run.go:758-761`, which excludes a budget break.
- The budget break the orchestrator decisions cite at `pkg/runner/run.go:634-637` is at `pkg/runner/run.go:632-635` (`if tracker.Exceeded() { budgetExceeded = true; break }`). The guard at `:759` matches the orchestrator's `:759`.
- The negative precedent `internal/agent/agent.go:167-169` holds (`if cfg.MaxTurns < 0 { return &ValidationError{msg: "max_turns must be non-negative"} }`).
- The `Options` struct is at `pkg/runner/options.go:16-67`. The last field, `Stdin`, is at `pkg/runner/options.go:66`. The new field goes after `Tools` (`pkg/runner/options.go:55-61`).
- The dry-run block opens at `pkg/runner/run.go:322`, and its `return &Result{` is at `:347`.
- The `ConfigError` type is `pkg/runner/errors.go:11`. The `IsConfigError` helper is at `pkg/runner/errors.go:57`.
- The existing TOML-driven test the milestone mirrors is `TestRun_MaxTurnsConfigured` at `pkg/runner/run_test.go:1336-1379`. The default-50 test is `TestRun_MaxTurnsExceeded` at `pkg/runner/run_test.go:1292-1334`.
- The CHANGELOG `Added` section is `CHANGELOG.md:10-16`. The 005-007 bullets are at `CHANGELOG.md:13-16`. The `[Unreleased]` heading is at `CHANGELOG.md:8`.

### Decisions settled (record as settled, never re-evaluate)

- **OS1. Validation runs before the dry-run return.** `Options.MaxTurns` is checked before `pkg/runner/run.go:322`, so a negative value is a `ConfigError` in live and dry-run modes. No provider request is sent in either mode. The check goes next to the `Options.Tools` check (`pkg/runner/run.go:106-111`), the same pattern as 007 OS1 (`docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md:23`).
- **OS2. Resolution order.** `Options.MaxTurns > 0` overrides the TOML `max_turns`. `Options.MaxTurns == 0` keeps the TOML value. A TOML value of 0 or unset falls back to `defaultMaxConversationTurns` (50). The existing `== 0` fallback at `pkg/runner/run.go:126-128` stays, applied after the override.
- **OS3. Loop bound and turn-limit text are unchanged.** The statement at `pkg/runner/run.go:630` and the error text at `:760` keep their wording. Only the value of `maxConversationTurns` changes, and the error reports the effective cap. A configured `MaxTurns: 2` therefore reports `exceeded maximum conversation turns (2)`.
- **OS4. Dry run does not display the cap.** There is no `DryRunInfo` field. `cmd/` is not changed. The CLI does not set `Options.MaxTurns`, so CLI output and golden files are unchanged (`entry: CLI unchanged`).
- **OS5. Error wording is not a stable contract.** The negative-value message names `Options.MaxTurns` and the bad value. Its exact wording is a decision of this spec, not a public contract (Limitations).
- **OS6. TOML validation is unchanged.** `agent.Load` still validates `cfg.MaxTurns` (`internal/agent/agent.go:167-169`) even when `Options.MaxTurns` overrides it. A TOML file with a negative `max_turns` still fails at load (007 OS5 analog; Limitations).
- **OS7. Sub-agents do not inherit `Options.MaxTurns`.** A `call_agent` child loads its own TOML (`internal/tool/tool.go:152`) and resolves its own cap from `cfg.MaxTurns` with the 50 fallback (`internal/tool/tool.go:368-372`, loop at `:374`, error at `:452`). The parent's option is not passed to the child. Unlike `Options.ToolHook`, this is not a `ConfigError` when `sub_agents` is set. The field doc comment states it. This matches `AGENTS.md:187` ("Independent context"), as 006 OS4 and 007 OS8 settled for their fields.
- **OS8. A budget break is not a turn-limit error.** A budget break (`pkg/runner/run.go:632-635`) sets `budgetExceeded` and skips the turn-limit error by the guard at `:758-761`. This behavior is unchanged.
- **OS9. CHANGELOG.** One `[Unreleased]` `Added` bullet for `Options.MaxTurns` is in scope, as 005-007 did (`CHANGELOG.md:13-16`). Milestone 012 builds the release section. It is the only `CHANGELOG.md` change.
- **OS10. Test placement.** The tests go in a new file `pkg/runner/max_turns_option_test.go`. They mirror `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336-1379`) and use `setupTestEnv` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr` (`pkg/runner/run_test.go:44`), and `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`). Tests that call `t.Setenv` or `setupTestEnv` cannot use `t.Parallel`.
- **OS11. Sub-agent non-inheritance test is included.** Existing fixtures support it cheaply (the `call_agent` pattern at `pkg/runner/run_test.go:2573-2600`), so it is acceptance test 4 (R11), not a limitation.
- **OS12. Default-50 coverage.** The both-unset default is already covered by `TestRun_MaxTurnsExceeded` (`pkg/runner/run_test.go:1292-1334`), which stays unchanged. The new file does not duplicate its 50-response fixture.

### Rejected approaches (settled)

- Validating `Options.MaxTurns` after the dry-run return. Rejected because a dry run would accept a value a live run rejects (OS1).
- Adding a `DryRunInfo` field for the cap. Rejected because the dry run does not display it (OS4), and `cmd/` and goldens stay unchanged.
- Passing `Options.MaxTurns` to sub-agents (OS7).
- Making `Options.MaxTurns` with `sub_agents` a `ConfigError` (OS7). Rejected because the hook rule does not apply to turn caps.
- Changing `agent.Load` to skip the TOML `max_turns` check when the option is set (OS6).
- Treating a budget break as a turn-limit error (OS8).
- Adding a CLI flag for the cap. Rejected because the CLI cannot set `Options` fields for this (OS4).

### Constraints and assumptions

- **Constraint, compatibility.** The addition is `Options.MaxTurns int` (`pkg/runner/options.go`). The apidiff gate (`Makefile:14`) compares `pkg/runner` against `v1.11.0`, and additions do not fail it.
- **Constraint, behavior change gate.** New behavior is opt-in through the new field. Zero keeps today's behavior (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:232`, "RT unchanged at zero").
- **Constraint, error classification.** A negative value is `ConfigError` (`pkg/runner/errors.go:11`), as the milestone requires.
- **Assumption, interpretation of the loop-bound clause.** The milestone says "the loop bound ... is unchanged". This spec reads that as: the loop statement and error text are unchanged, and the value they use now comes from the effective cap (OS2, OS3). No other reading is recorded.
- **Assumption, test environment.** Tests use `t.Setenv` for `AXE_ANTHROPIC_BASE_URL`, as existing tests do (`pkg/runner/run_test.go:1357`). They cannot use `t.Parallel`.

## Section 2: Requirements

Requirements R1-R2 are the field and validation. R3-R5 are the effective cap and loop. R6-R8 are the dry run, compatibility, and sub-agents. R9-R10 are the budget boundary and documentation. Acceptance tests are in R11.

### R1: Public field

- `pkg/runner` `Options` gains one field `MaxTurns int`. The doc comment states: `> 0` overrides the agent's TOML `max_turns`; `0` keeps it, and TOML 0 or unset means 50; a negative value is a `ConfigError` in live and dry-run modes; sub-agents do not inherit the value (OS7).
- Verification: `go build ./...` exits 0. `rg -n "MaxTurns int" pkg/runner/options.go` shows the field.

### R2: Validation before the dry-run return

- A negative `Options.MaxTurns` is a `ConfigError` (`pkg/runner/errors.go:11`) returned before the dry-run return at `pkg/runner/run.go:322`, in both live and dry-run modes (OS1). It names `Options.MaxTurns` and the value.
- No provider request is sent in either mode.
- Verification: acceptance test 3 (R11).

### R3: Effective cap

- The cap is `Options.MaxTurns` when it is `> 0`, else the TOML `cfg.MaxTurns` when it is `> 0`, else `defaultMaxConversationTurns` (OS2). The computation replaces the one at `pkg/runner/run.go:126-128`.
- Verification: acceptance tests 1 and 2 (R11).

### R4: Loop bound and error text are unchanged

- The loop statement at `pkg/runner/run.go:630` and the `RuntimeError` text at `pkg/runner/run.go:760` keep their wording (OS3). The reported number is the effective cap.
- Verification: acceptance test 1 asserts `exceeded maximum conversation turns (2)` and `RequestCount == 2`.

### R5: TOML-only behavior is unchanged

- With `Options.MaxTurns` zero, the cap is the TOML value, or 50 when unset or zero. The request, error text, and `--json` output are byte-identical to `7f56dae`.
- Verification: `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336`) and `TestRun_MaxTurnsExceeded` (`pkg/runner/run_test.go:1292`) pass unchanged.

### R6: Dry run

- The dry run does not display the cap (OS4). `DryRunInfo` has no new field, `cmd/` is not changed, and golden files are unchanged.
- Verification: `git diff --name-only 7f56dae -- cmd internal` prints no paths.

### R7: CLI, TOML, and `--json` are unchanged

- No CLI flag, TOML key, `--json` field, or golden file changes.
- Verification: `go test ./cmd -count=1` exits 0 with `XDG_CACHE_HOME` set (Limitations).

### R8: TOML validation is unchanged

- `agent.Load` still validates `cfg.MaxTurns` (`internal/agent/agent.go:167-169`) even when `Options.MaxTurns` overrides it (OS6). `internal/agent/agent.go` is not edited.
- Verification: `git diff --name-only 7f56dae -- internal/agent` prints no paths.

### R9: Sub-agents

- A `call_agent` child resolves only its own TOML cap, with the 50 fallback (OS7; `internal/tool/tool.go:368-372`). The parent's `Options.MaxTurns` does not apply to the child. No `ConfigError` is raised for `sub_agents`.
- Verification: acceptance test 4 (R11).

### R10: Budget break and documentation

- A budget break is not a turn-limit error (OS8). The guard at `pkg/runner/run.go:758-761` is unchanged.
- One `[Unreleased]` `Added` bullet in `CHANGELOG.md` describes `Options.MaxTurns`, the negative `ConfigError`, the zero-value rule, and that sub-agents do not inherit the value (OS9).
- Verification: `rg -n "Options.MaxTurns" pkg/runner/options.go CHANGELOG.md` shows the doc comment and the bullet.

### R11: Acceptance tests

Test file: `pkg/runner/max_turns_option_test.go` (new). Tests use `setupTestEnv` and `t.Setenv`, so none can use `t.Parallel` (OS10).

1. **`TestMaxTurnsOption_OverridesTOML`.** The agent TOML has `max_turns = 5` and `tools = ["list_directory"]`. The mock queues two tool-use responses and no more. `Options.MaxTurns` is `2`. Assert `Run` returns a `RuntimeError` (`IsRuntimeError`) whose message contains `exceeded maximum conversation turns (2)`, and `mock.RequestCount() == 2` (R3, R4). A third request would fail the mock test (`internal/testutil/mockserver.go:75-78`).
2. **`TestMaxTurnsOption_ZeroKeepsTOMLAndDefault`.** A table of two cases. Case `toml 2`: TOML `max_turns = 2`, `Options.MaxTurns` `0`, two tool-use responses queued. Assert the same `RuntimeError` text with `(2)` and `RequestCount == 2` (R3, R5). Case `unset`: no `max_turns` in TOML, `Options.MaxTurns` `0`. The mock queues 50 tool-use responses (a response-generating helper is allowed). Assert the same `RuntimeError` text with `(50)` and `RequestCount == 50`. The default error text is asserted by `TestRun_MaxTurnsExceeded` (OS12).
3. **`TestMaxTurnsOption_NegativeConfigError`.** A table of `-1` and `-5`. Each case runs as a live run and as a dry run (`DryRun: true`), against a mock with an empty response queue (`NewMockLLMServer(t, nil)`). Assert `IsConfigError(err)`, that `err.Error()` contains `Options.MaxTurns` and the case's negative value (`-1` or `-5`), and that `mock.RequestCount() == 0` (R2).
4. **`TestMaxTurnsOption_SubAgentNotInherited`.** The parent TOML has `sub_agents = ["child-agent"]`, `tools = ["list_directory"]`, and no `max_turns`. The child TOML has `tools = ["list_directory"]` and no `max_turns`, so the child uses 50. `Options.MaxTurns` is `2`. The mock queues, in order: parent tool-use calling `call_agent` (request 0, `Requests[0]`), child tool-use (request 1), child tool-use (request 2), child tool-use (request 3), child text (request 4), parent text (request 5). Three child tool turns exceed a cap of 2, so an inherited cap would fail the child. Assert `Run` succeeds and `RequestCount == 6`. Assert `mock.Requests[5].Body` contains the child's successful final text and does not contain `sub-agent exceeded maximum conversation turns`; this assertion is mandatory, because `internal/tool/tool.go:335-345` turns a child loop error into an error tool result, so `Run` succeeds even when the cap is inherited. Assert `mock.Requests[0].Body` contains `call_agent` (R9, OS7).

Focused run: after tests 1-4 exist, the focused run selecting the `TestMaxTurnsOption_` family in package `pkg/runner` must exit 0. It is not in the fenced block because spec-lint reports "selects no test" until the tests are written.

## Parallelizable work

- The field and doc comment (R1), the validation (R2), and the CHANGELOG bullet (R10) are independent of each other.
- Tests 1-4 share one new file and are written after the wiring.

## Serial dependencies

- R1 before R2-R5 (the field must exist).
- R2 before R3 (validation runs before the cap is used).
- R3-R5 and R9 before R11 (tests exercise the wiring).

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
go build ./...                                                          # expect: exit 0
go vet ./pkg/runner/                                                     # expect: exit 0
go test ./internal/agent ./pkg/runner -count=1                           # expect: exit 0
XDG_CACHE_HOME=$(mktemp -d) go test ./cmd -count=1                       # expect: exit 0 (see Limitations)
make apidiff                                                             # expect: exit 0, no incompatible-change output
rg -n "MaxTurns int" pkg/runner/options.go                               # expect: the new field
rg -n "Options.MaxTurns" CHANGELOG.md                                    # expect: one bullet under [Unreleased]
git diff --name-only 7f56dae -- . ':(exclude)docs/plans/'                # expect: CHANGELOG.md, pkg/runner/options.go, pkg/runner/run.go, pkg/runner/max_turns_option_test.go
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_008_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since 7f56dae /home/jaron/dev/axe/runner-tool-hooks                                              # expect: exit 0
```

Focused run: see R11.

## Failure modes

1. **Dry run accepts a negative cap that a live run rejects.** Mitigation: validation before the dry-run return (R2, OS1), test 3.
2. **Zero value changes TOML behavior.** Mitigation: R5, `TestRun_MaxTurnsConfigured`, `TestRun_MaxTurnsExceeded`.
3. **Parent cap leaks to a sub-agent.** Mitigation: OS7, R9, test 4.
4. **Budget break reported as a turn-limit error.** Mitigation: OS8, R10, guard `pkg/runner/run.go:758-761` unchanged.
5. **Override ignored, so a host cap is silently replaced by the TOML value.** Mitigation: R3, test 1 (TOML 5, override 2, exactly 2 requests).

## Limitations

- `Options.MaxTurns` does not apply to `call_agent` children. They use their own TOML `max_turns` or 50 (OS7). The field doc comment states this.
- TOML `max_turns` is still validated at load even when `Options.MaxTurns` overrides it. A TOML file with a negative value still fails (OS6).
- The wording of the negative-value message is a decision of this spec, not a stable public contract (OS5).
- The dry run does not show the effective cap (OS4). The CLI cannot set `Options.MaxTurns`, so the CLI regression check is the existing turn tests and goldens (R7), not a new CLI test.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005-007.
- Milestone entry 008 cites stale lines. The corrected lines are in "Citation corrections (verified at `7f56dae`)". The milestone document is not edited by this spec.
