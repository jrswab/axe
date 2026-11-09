# Implement Milestone 008: Override the turn cap per run with `Options.MaxTurns`

## Section 1: Context Summary

Milestone 008 of the v1.12.0 runner tool-hooks release lets a host set the turn cap per run in code. `Options.MaxTurns int` is added to the `Options` struct (`pkg/runner/options.go:16`); the new field goes after `Tools` (`pkg/runner/options.go:61`). A value `> 0` overrides the agent's TOML `max_turns`; `0` keeps it; a TOML value of `0` or unset falls back to `defaultMaxConversationTurns` (50, `pkg/runner/run.go:36-37`) (R3, OS2). A negative value is a `ConfigError` checked before the dry-run return, so live and dry-run modes both reject it and send no provider request (R2, OS1). The loop statement (`pkg/runner/run.go:630`) and the turn-limit error text (`pkg/runner/run.go:760`) keep their wording; only the effective cap changes, so a configured `MaxTurns: 2` reports `exceeded maximum conversation turns (2)` (R4, OS3). A budget break still skips the turn-limit error (guard `pkg/runner/run.go:759-761`, OS8). The dry run does not display the cap, and `cmd/` is unchanged (R6, OS4). `agent.Load` still validates TOML `max_turns` (`internal/agent/agent.go:167-169`, OS6). Sub-agents do not inherit the option: a `call_agent` child resolves its own TOML cap with the 50 fallback (`internal/tool/tool.go:368`, loop `:374`, error `:452`) (R9, OS7). One `[Unreleased]` `Added` bullet is the only `CHANGELOG.md` change (R10, OS9).

Source: `docs/plans/052_runner_tool_hooks_v1_12_0_008_spec.md` (Sections 1 and 2, R1-R11, OS1-OS12). Milestone entry 008 is `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:228-236`; its line citations are stale, and this checklist uses the spec's corrected citations, re-verified at HEAD `bb9f625` (code unchanged since `7f56dae`). Predecessor: none (`Depends on: none`). Milestone 007 (`Options.Tools`) is the precedent for the validation and CHANGELOG pattern.

Commit is not authorized in this checklist. Do not push. Do not edit records: the spec, the checkpoint (`docs/plans/052_runner_tool_hooks_v1_12_0_008_checkpoint.md`), and the milestones document are orchestrator records. Tasks are committed one at a time by the orchestrator. In-task checks read the working tree; the scope check in T8 reads the committed range `7f56dae..HEAD`.

Scope of changed files (R1, R10, OS9): `pkg/runner/options.go`, `pkg/runner/run.go`, `pkg/runner/max_turns_option_test.go` (new), `CHANGELOG.md`. Nothing else. `internal/agent/`, `internal/tool/`, `internal/mcpclient/`, `cmd/`, and `internal/testutil/` are not changed (R6, R7, R8, OS4, OS6). Every code change is in `./pkg/runner`, so the derived race package list is `./pkg/runner` only.

Sandbox rules: `go test ./cmd` needs `XDG_CACHE_HOME=$(mktemp -d)` (noted where cmd tests are run). Tests that call `t.Setenv` or `setupTestEnv` cannot use `t.Parallel` (OS10). Pre-existing gofmt drift in `pkg/runner/result.go` and `pkg/runner/result_test.go` is out of scope; gofmt checks target only touched files.

Dispatch groups (orchestrator decision 5): Group A = T0-T1 (preflight and API scaffolding). Group B = T2.1-T2.4 (red tests). Group C = T3.1-T3.4 (implementation; T3.4 is verification only). Group D = T4-T7 (verification and lint). Group E = T7.1 (CHANGELOG), then T8 (final scope), then T9 (orchestrator records).

Decisions this checklist records (not open; none changes scope or behavior beyond the spec):
- **A1. Validation message.** The spec fixes the type (`ConfigError`) and that the message names `Options.MaxTurns` and the bad value (OS5). The wording is `Options.MaxTurns must be non-negative, got %d`, mirroring the TOML wording at `internal/agent/agent.go:168` (`max_turns must be non-negative`). Wording is not a stable contract (spec Limitations).
- **A2. Cap computation.** The override is applied after `maxConversationTurns := cfg.MaxTurns` (`pkg/runner/run.go:126`) and before the existing `== 0` fallback (`pkg/runner/run.go:127-129`). A negative option cannot reach that code, because the validation in T3.1 returns first.
- **A3. Test classification.** Only spec tests 1 and 3 fail at HEAD. Spec tests 2 and 4 describe behavior that HEAD already has (zero and unset keep the TOML value or 50; sub-agents already use their own cap), so they are marked `[guard]` and must PASS at HEAD. They are regression guards for the new wiring.

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

`go test -v` appends a duration to each result line, so every PASS-line filter in this checklist ends the test name with `( |$)` and never anchors `$` directly after the name. Filter form: `rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)'`. New test names do not exist until T2.1, so `-run` is not used for them; red and green steps run the package verbosely.

### Phase 0: Pre-flight (serial, read-only)

- [ ] **T0: Pre-flight: HEAD ancestry, tree state, baseline tests, declared names absent**
  - Budget: 30 min
  - Owner: code (verification)
  - Files: none (read-only).
  - Commands and expected results (repository root):
    - `git merge-base --is-ancestor 7f56dae HEAD; echo "exit=$?"` → `exit=0`. Verified at writing time (HEAD `bb9f625`).
    - `git diff --name-only 7f56dae HEAD -- . ':(exclude)docs/plans/'` → no output. Verified at writing time; code is unchanged since `7f56dae`.
    - `git status --porcelain` → any modified or untracked path must be an 008 plan file (`docs/plans/052_runner_tool_hooks_v1_12_0_008_*.md`). Any other path: stop and report.
    - `rg -n 'TestMaxTurnsOption_' --glob '*.go' .; echo "exit=$?"` → `exit=1`. Verified at writing time (names absent).
    - `test -e pkg/runner/max_turns_option_test.go; echo "exit=$?"` → `exit=1`. Verified at writing time (file absent).
    - `rg -n "MaxTurns" pkg/runner/options.go; echo "exit=$?"` → `exit=1`. Verified at writing time (field absent).
    - `rg -n "Tools \[\]string" pkg/runner/options.go` → `pkg/runner/options.go:61` (007 field present; the 008 field goes after it).
    - `rg -n "maxConversationTurns := cfg.MaxTurns" pkg/runner/run.go` → `pkg/runner/run.go:126`.
    - `rg -n "for turn := 0; turn < maxConversationTurns; turn\+\+" pkg/runner/run.go` → `pkg/runner/run.go:630`.
    - `rg -n 'exceeded maximum conversation turns' pkg/runner/run.go` → `pkg/runner/run.go:760`.
    - `go test -list '^(TestRun_MaxTurnsExceeded|TestRun_MaxTurnsConfigured|TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)$' ./pkg/runner` → lists all five names, `ok`. Verified at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent` → `ok` for both. Verified at writing time.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -list '^(TestRun_NoSubAgents_NoTools|TestRunCmd_DryRun_WithMCPServers)$' ./cmd` → lists both names, `ok`. Verified at writing time.
    - `git diff --exit-code 7f56dae -- internal/agent internal/tool internal/mcpclient cmd internal/testutil go.mod go.sum; echo "exit=$?"` → `exit=0`. Verified at writing time.
    - `gofmt -l pkg/runner internal/agent` → only `pkg/runner/result.go` and `pkg/runner/result_test.go` (pre-existing drift, out of scope). Verified at writing time.
    - `git rev-parse --verify 'v1.11.0^{commit}'` → `20e6ada179b08cef050c4b436a2497182efcf2ee`. Verified at writing time (apidiff baseline, `Makefile:15`).
    - `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` → `PASS: Go pre-flight checks.`, `exit=0`. Verified at writing time (go1.26.10; govulncheck clean). Recorded because T7 runs `go run` on a pinned module (`Makefile:3`, `Makefile:22-23`).
    - `command -v golangci-lint` → `/home/jaron/.cache/pi-go-bin/golangci-lint`. Verified at writing time.
    - `test -e .agents/scripts/checklist-test-gate-audit; echo "exit=$?"` → `exit=1` (script absent; not run).
  - Verification: all expected results match. If any differs, stop and report.
  - Per-task gate: this task changes no files. The derived package list is empty before any edit.

### Phase 1: Enabling surface (serial)

- [ ] **T1: Public field and doc contract (R1, R10)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T0.
  - Enables: T2.1-T2.3 (test code must compile against `Options.MaxTurns`) and T3.x (implementation).
  - Files:
    - `pkg/runner/options.go`: add field `MaxTurns int` to `type Options struct` (`pkg/runner/options.go:16`) after the `Tools []string` field (`pkg/runner/options.go:61`), with a doc comment above it.
  - Doc comment (R1, R10, OS2, OS4, OS7): `> 0` overrides the agent's TOML `max_turns`; `0` keeps the TOML value, and a TOML value of 0 or unset means 50; a negative value is a ConfigError in live and dry-run modes; sub-agents do not inherit the value.
  - Do not add behavior. `run`, `Run`, `agent.Load`, and the cap computation are unchanged in this task. The field is declared but unused until T3.2, which is the expected state for a declaration task.
  - Failing-first: not applicable (declaration only).
  - Verification:
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner` → `ok` (no behavior changed).
    - `rg -n "MaxTurns int" pkg/runner/options.go` → one match (R1).
    - `rg -n "overrides|sub-agents do not inherit|ConfigError" pkg/runner/options.go` → each doc claim present (R10).
    - `gofmt -l pkg/runner/options.go` → no output.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

### Phase 2: Red tests (serial, test-only)

Red-run rule: a red test is "red" only if it compiles and fails on an assertion (or on the mock's unexpected-request fatal, which is a failure of the missing behavior). T1 adds the identifier the tests use (`Options.MaxTurns`), so every test compiles. Guard tests are marked `[guard]` and must pass at HEAD.

Shared test conventions (file `pkg/runner/max_turns_option_test.go`, new, created in T2.1): package `runner`; mirror `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336`): `setupTestEnv(t, configTOML, agentName, agentTOML)` (`pkg/runner/run_test.go:24`), `newMockStdoutStderr()` (`pkg/runner/run_test.go:44`), `testutil.NewMockLLMServer` (`internal/testutil/mockserver.go:50`), `t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())` (`pkg/runner/run_test.go:1357`). TOML config uses `[providers.anthropic] api_key = "fake-key"`. Mock responses use `testutil.AnthropicResponse` (`internal/testutil/mockserver.go:141`) and `testutil.AnthropicToolUseResponse` (`internal/testutil/mockserver.go:154`). Requests are read as `mock.Requests[i].Body` (`internal/testutil/mockserver.go:27`, `:34`). No test uses `t.Parallel`. Imports are added as each test needs them (for example `context`, `fmt`, `os`, `path/filepath`, `strings`, `testing`, and `github.com/jrswab/axe/internal/testutil`).

- [ ] **T2.1: Red: acceptance test 1 (R3, R4, R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T1.
  - File: `pkg/runner/max_turns_option_test.go` (new; created in this task).
  - Test: `TestMaxTurnsOption_OverridesTOML`. Agent TOML: `name = "test-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, `max_turns = 5`, `tools = ["list_directory"]`. Mock queues exactly two responses, each `AnthropicToolUseResponse("Tool call", []testutil.MockToolCall{{ID: "tool_N", Name: "list_directory", Input: map[string]string{"path": "."}}})`. `Options{AgentName: "test-agent", Stdout: stdout, Stderr: stderr, MaxTurns: 2}`. Assert `Run` returns an error for which `IsRuntimeError(err)` is true and whose `err.Error()` contains `exceeded maximum conversation turns (2)`; assert `mock.RequestCount() == 2` (R3, R4).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMaxTurnsOption_OverridesTOML`. Expected reason at HEAD: the option is ignored, so the loop runs to the TOML cap of 5; the third request reaches the mock's unexpected-request fatal (`internal/testutil/mockserver.go:77`), and the RuntimeError assertion fails. Record the actual failing assertion.
  - Filter: `rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)'`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above; the race run must be green from T3.2.

- [ ] **T2.2: Red: acceptance test 3 (R2, R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.1.
  - File: `pkg/runner/max_turns_option_test.go`.
  - Test: `TestMaxTurnsOption_NegativeConfigError`, table over `value` in `{-1, -5}` and `dryRun` in `{false, true}` (four subtests; subtest name `fmt.Sprintf("value_%d_dry_run_%t", value, dryRun)`). Each subtest: agent TOML `name = "test-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, no `max_turns`; mock `testutil.NewMockLLMServer(t, nil)` (empty queue, so any request is fatal); `Options{AgentName: "test-agent", Stdout, Stderr, MaxTurns: value, DryRun: dryRun}`. Assert `IsConfigError(err)` (`pkg/runner/errors.go:57`); assert `err.Error()` contains `Options.MaxTurns` and `fmt.Sprintf("%d", value)`; assert `mock.RequestCount() == 0` (R2, OS1).
  - Red step: `go test -race -v ./pkg/runner` shows `--- FAIL: TestMaxTurnsOption_NegativeConfigError` for each subtest. Expected reason at HEAD: no validation exists, so the live subtests reach the empty-queue mock (fatal) or return a non-ConfigError, and the dry-run subtests return success with no error. Record the actual failing assertion for each subtest kind.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`. For this red task the gate is the red run above.

- [ ] **T2.3: Guards: acceptance tests 2 and 4 (R3, R5, R9, R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.2.
  - File: `pkg/runner/max_turns_option_test.go`.
  - Tests (both `[guard]`, must PASS at HEAD):
    - `TestMaxTurnsOption_ZeroKeepsTOMLAndDefault` (test 2, table with two subtests). Subtest `toml_2`: agent TOML `max_turns = 2`, `tools = ["list_directory"]`; `Options.MaxTurns: 0`; two tool-use responses queued (as in T2.1). Assert `IsRuntimeError` and `exceeded maximum conversation turns (2)`, and `RequestCount() == 2`. Subtest `unset`: agent TOML with no `max_turns`, `tools = ["list_directory"]`; `Options.MaxTurns: 0`; fifty tool-use responses queued, built with the same loop as `TestRun_MaxTurnsExceeded` (`pkg/runner/run_test.go:1292`); assert `exceeded maximum conversation turns (50)` and `RequestCount() == 50`. The default error text is also asserted by `TestRun_MaxTurnsExceeded` (OS12).
    - `TestMaxTurnsOption_SubAgentNotInherited` (test 4). Parent TOML: `name = "parent-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, `sub_agents = ["child-agent"]`, `tools = ["list_directory"]`, no `max_turns`. Child TOML at `filepath.Join(tmpDir, "axe", "agents", "child-agent.toml")` (`tmpDir` from `setupTestEnv`; pattern at `pkg/runner/run_test.go:793-800`): `name = "child-agent"`, `model = "anthropic/claude-sonnet-4-20250514"`, `tools = ["list_directory"]`, no `max_turns`, no `sub_agents`. Mock queue in order: request 0 parent `AnthropicToolUseResponse("Delegating", []testutil.MockToolCall{{ID: "tool_1", Name: "call_agent", Input: map[string]string{"agent": "child-agent", "task": "say hello"}}})` (pattern at `pkg/runner/run_test.go:2576`); requests 1, 2, 3 child `AnthropicToolUseResponse` with `list_directory` (`Input: map[string]string{"path": "."}`); request 4 child `AnthropicResponse("child done")`; request 5 parent `AnthropicResponse("parent done")`. `Options{AgentName: "parent-agent", Stdout, Stderr, MaxTurns: 2}`. Assert `Run` returns no error; `RequestCount() == 6`; `mock.Requests[5].Body` contains `child done` and does not contain `sub-agent exceeded maximum conversation turns`; `mock.Requests[0].Body` contains `call_agent` (R9, OS7). The child-error assertion is required, because `internal/tool/tool.go:333` turns a child loop error into an error tool result, so `Run` can succeed even when the cap is inherited.
  - Expected at HEAD: both PASS (guards). Record the PASS lines.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: only the T2.1 and T2.2 tests.

- [ ] **T2.4: Red-run check (all acceptance tests)**
  - Budget: 15 min
  - Owner: code (verification)
  - Depends on: T2.3.
  - Files: none.
  - Command: `go test -race -v ./pkg/runner > /tmp/008-t24-red.txt 2>&1; echo "exit=$?"`, then `rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)' /tmp/008-t24-red.txt`.
  - Expected: `exit=1` (red tests exist). The `rg` output shows `--- FAIL` for `TestMaxTurnsOption_OverridesTOML` and for each `TestMaxTurnsOption_NegativeConfigError` subtest; it shows `--- PASS` for `TestMaxTurnsOption_ZeroKeepsTOMLAndDefault` (both subtests) and `TestMaxTurnsOption_SubAgentNotInherited`.
  - Also `rg -- '--- FAIL: Test' /tmp/008-t24-red.txt` lists only the red tests and their subtests. Any existing test in that output is a stop: report it.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the red run above is the gate. The derived package list is `./pkg/runner`, and the file is uncommitted until the orchestrator commits it.

### Phase 3: Green implementation (serial)

Order: T3.1 (validation) → T3.2 (effective cap) → T3.3 (dry run, sub-agent, and budget verification; no code expected) → T3.4 (full package green). Both code tasks edit `pkg/runner/run.go` only, so they are serial.

Expected FAIL set after each task (any other FAIL is a stop):
- After T3.1: `TestMaxTurnsOption_OverridesTOML` only (the cap is not yet applied).
- After T3.2: none.

- [ ] **T3.1: Green: validate `Options.MaxTurns` before the dry-run return (R2)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T2.4.
  - File: `pkg/runner/run.go` only.
  - Behavior: insert after the `Options.Tools` validation loop, which closes at `pkg/runner/run.go:111`, and before `effectiveTools := cfg.Tools` (`pkg/runner/run.go:112`):
    ```go
    if opts.MaxTurns < 0 {
    	return nil, &ConfigError{Msg: fmt.Sprintf("Options.MaxTurns must be non-negative, got %d", opts.MaxTurns)}
    }
    ```
    The check is before the dry-run block (`pkg/runner/run.go:322`), so live and dry-run modes both reject a negative value before any provider request (OS1). A zero or positive value passes (R5).
  - Green target: `TestMaxTurnsOption_NegativeConfigError` (all four subtests) PASS.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMaxTurnsOption_NegativeConfigError[^ ]*( |$)'` → every line `PASS`.
    - `rg -n "Options.MaxTurns must be non-negative" pkg/runner/run.go` → one match.
    - `go vet ./pkg/runner/` → `exit=0`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: `TestMaxTurnsOption_OverridesTOML` only.

- [ ] **T3.2: Green: effective cap for the loop (R3, R4)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.1.
  - File: `pkg/runner/run.go` only.
  - Behavior: at `pkg/runner/run.go:126-128`, the current block is `maxConversationTurns := cfg.MaxTurns` followed by `if maxConversationTurns == 0 {` and `maxConversationTurns = defaultMaxConversationTurns`. Insert between the first line and the `== 0` check:
    ```go
    if opts.MaxTurns > 0 {
    	maxConversationTurns = opts.MaxTurns
    }
    ```
    The existing `== 0` fallback stays, applied after the override (OS2). The loop statement (`pkg/runner/run.go:630`) and the error text (`pkg/runner/run.go:760`) are not edited (OS3). The budget guard (`pkg/runner/run.go:759`) is not edited (OS8).
  - Green target: `TestMaxTurnsOption_OverridesTOML` PASS (cap 2 from TOML 5, `RequestCount == 2`).
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMaxTurnsOption_OverridesTOML[^ ]*( |$)'` → `PASS`.
    - `rg -n "opts.MaxTurns > 0" pkg/runner/run.go` → one match inside the cap block.
    - `rg -n "maxConversationTurns := cfg.MaxTurns" pkg/runner/run.go` → one match (the TOML source is kept).
    - `go vet ./pkg/runner/` → `exit=0`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

- [ ] **T3.3: Verify dry run, sub-agents, budget guard, and TOML validation unchanged (R6, R8, R9, R10); no code expected**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.2.
  - Files: none expected. If a check fails, fix only `pkg/runner/run.go` in the lines changed by T3.1 or T3.2; do not change tests or other files.
  - Checks (working tree, in-task):
    - `rg -n "if opts.DryRun \{" pkg/runner/run.go` → `pkg/runner/run.go:322`; the validation (T3.1) appears above that line (R2, OS1). Confirm the validation line number is smaller than 322.
    - `rg -n "DryRunInfo\{" -A 12 pkg/runner/run.go` shows no `MaxTurns` field (OS4).
    - `rg -n "if !budgetExceeded && resp != nil" pkg/runner/run.go` → one match, unchanged (OS8).
    - `rg -n "MaxTurns" internal/tool/tool.go` → `internal/tool/tool.go:368` only (the child cap from its own TOML; OS7). No `opts.MaxTurns` use in `internal/tool/`.
    - `git diff --exit-code -- internal cmd` → `exit=0` (working tree; no edits outside `pkg/runner`).
    - `rg -n "opts.MaxTurns" pkg/runner/run.go` → exactly the validation (T3.1) and the cap (T3.2). Sub-agent calls (`executeToolCalls`, `pkg/runner/run.go:964`) do not receive `opts.MaxTurns` (OS7).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files unless a fix was needed; if none, the derived package list is empty. If a fix to `pkg/runner/run.go` was needed, the package is `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T3.4: Green: all acceptance tests 1-4 pass (R11)**
  - Budget: 30 min
  - Owner: code
  - Depends on: T3.3.
  - Files: none expected. Fix any failure in `pkg/runner/run.go` (the file that introduced it), not in the test.
  - Green targets: `TestMaxTurnsOption_OverridesTOML`, `TestMaxTurnsOption_NegativeConfigError`, `TestMaxTurnsOption_ZeroKeepsTOMLAndDefault`, and `TestMaxTurnsOption_SubAgentNotInherited` all PASS.
  - Verification:
    - `go test -race -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)'` → every line `PASS`; `go test -race -v ./pkg/runner | rg -- '--- FAIL'` → no output.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent` → `ok` for both.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`. Expected FAIL set: none.

### Phase 4: Verification (serial, after Phase 3)

- [ ] **T4: Regression verification; TOML, CLI, `--json`, and sub-agent paths unchanged (R5, R7, R8)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T3.4.
  - Files: none (verification only).
  - Commands:
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./pkg/runner ./internal/agent` → `ok` for both.
    - `XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd` → `ok`. Run with `XDG_CACHE_HOME` set; this includes `TestRun_NoSubAgents_NoTools` (`cmd/run_test.go:1007`) and `TestRunCmd_DryRun_WithMCPServers` (`cmd/run_test.go:289`).
    - `go test -count=1 -v ./pkg/runner | rg -- '--- (PASS|FAIL): (TestRun_MaxTurnsExceeded|TestRun_MaxTurnsConfigured|TestRun_SingleShot_NoTools|TestToolHookMCPAllow|TestRun_SubAgent)( |$)'` → every line `PASS` (existing tests, verified in T0).
    - `go test -count=1 -v ./pkg/runner | rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)'` → every line `PASS`. Record the list.
    - `go build ./...` → `exit=0`.
    - `go vet ./pkg/runner/` → `exit=0`.
    - `gofmt -l pkg/runner/options.go pkg/runner/run.go pkg/runner/max_turns_option_test.go` → no output. Drift in `result.go` and `result_test.go` is out of scope and is not listed.
    - `git diff --exit-code -- internal cmd` → `exit=0` (working tree).
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty. T5 runs the race run.

- [ ] **T5: Race run of the `TestMaxTurnsOption_` family and the package (R2, R3, R9, R11)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T4.
  - Step 1: `go test -race -v ./pkg/runner > /tmp/008-t5-race.txt 2>&1; echo "exit=$?"`. Expected `exit=0`.
  - Step 2 (only after step 1 prints `exit=0`; inspect the saved file): `rg -- '--- (PASS|FAIL): TestMaxTurnsOption_[^ ]*( |$)' /tmp/008-t5-race.txt` → every line `PASS`; `rg -- 'DATA RACE' /tmp/008-t5-race.txt` → no matches (`exit=1`).
  - Pass condition: step 1 exit 0, every `TestMaxTurnsOption_` line `PASS`, and no `DATA RACE`.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. Package for this task: `./pkg/runner`; run `go test -race ./pkg/runner`.

- [ ] **T6: `golangci-lint run` (lint gate)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T5.
  - Command (repository root): `golangci-lint run; echo "exit=$?"`. Expected: `exit=0`, no findings. A timeout is not a pass; report it.
  - Any finding in `pkg/runner/options.go`, `pkg/runner/run.go`, or `pkg/runner/max_turns_option_test.go` is a failure of the task that introduced it. Fix it in that task's file, then re-run T4 and T6.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

- [ ] **T7: `make apidiff` (milestone 002 gate; additions only expected) (R1)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T6.
  - Command: `make apidiff >/tmp/axe-m008-apidiff-out.txt 2>/tmp/axe-m008-apidiff-err.txt; echo "exit=$?"`. Expected: `exit=0`; `wc -c </tmp/axe-m008-apidiff-out.txt` → `0`. The gate compares `pkg/runner` with `v1.11.0` (`Makefile:4`, `Makefile:15`). `Options.MaxTurns` is an addition and must not fail it (`Makefile:14-24`). The tag resolves to `20e6ada` (T0).
  - Cleanup: `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
  - Needs network on a cold module cache. If it fails for network reasons, report it; do not change the pin (`Makefile:3`).
  - No file changes.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files; the derived package list is empty.

### Phase 5: Documentation (CHANGELOG), then final scope

- [ ] **T7.1: CHANGELOG `[Unreleased]` Added bullet for `Options.MaxTurns` (R10, OS9; `CONTRIBUTING.md:185`)**
  - Budget: 15 min
  - Owner: code
  - Depends on: T7.
  - File: `CHANGELOG.md` only. Under `## [Unreleased]` (`CHANGELOG.md:8`) → `### Added` (`CHANGELOG.md:10`), append one bullet after the last existing bullet of that section, which is the 007 `Options.Tools` bullet at `CHANGELOG.md:16`. The blank line that follows is `CHANGELOG.md:17`. Text: "- `Options.MaxTurns` in `pkg/runner` overrides the agent's TOML `max_turns` for one run when greater than 0. Zero keeps the TOML value, and a TOML value of 0 or unset means 50. A negative value is a `ConfigError`. Sub-agents do not inherit the value." Form follows the bullets at `CHANGELOG.md:13-16`.
  - Verification: `git diff --numstat -- CHANGELOG.md` → `1	0	CHANGELOG.md`; `git diff -- CHANGELOG.md` shows one added line inside `[Unreleased]` `### Added`; `rg -n "Options.MaxTurns" CHANGELOG.md` → one match (R10); `rg -n "inherit" CHANGELOG.md` → the sub-agent sentence in that bullet.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. No Go file changes; the derived package list is empty.

- [ ] **T8: Scope, API, removed-line, and lint checks against the committed range `7f56dae..HEAD` (R6, R7, R8, R10)**
  - Budget: 30 min
  - Owner: code (verification)
  - Depends on: T7.1.
  - Files: none expected. If a citation refresh is needed (step 1), this checklist only.
  - Step 1 (citation refresh, if needed): run spec-lint (step 3). If it reports a stale citation in **this checklist** because code lines shifted after T3.x, refresh the line numbers only (no text change) and re-run. If it reports a stale citation in the **spec** (`docs/plans/052_runner_tool_hooks_v1_12_0_008_spec.md`), stop and report to the orchestrator; the code agent does not edit the spec.
  - Commands and expected results (committed range `7f56dae..HEAD`, after the orchestrator has committed the work):
    - `git status --porcelain` → no output except 008 plan files and orchestrator records. Any other path: report.
    - `git diff --name-only 7f56dae..HEAD -- . ':(exclude)docs/plans/'` → exactly `CHANGELOG.md`, `pkg/runner/max_turns_option_test.go`, `pkg/runner/options.go`, `pkg/runner/run.go`.
    - `git diff --exit-code 7f56dae..HEAD -- internal cmd go.mod go.sum` → `exit=0` (R6, R7, R8).
    - `rg -n "MaxTurns int" pkg/runner/options.go` → the field (R1).
    - `rg -n "overrides|sub-agents do not inherit" pkg/runner/options.go` → each doc claim present (R10).
    - `rg -n "Options.MaxTurns" CHANGELOG.md` → one bullet (R10).
    - `git diff 7f56dae..HEAD -- pkg/runner/run.go | rg '^-' | rg -v '^---'` → no output (additions only in `run.go`; the cap line at `pkg/runner/run.go:126` is kept, so no removed line is expected). Any removed line is a failure; report it.
    - `git diff 7f56dae..HEAD -- pkg/runner/options.go | rg '^-' | rg -v '^---'` → no output (additions only).
    - `git diff 7f56dae..HEAD -- pkg/runner/run.go | rg -n 'Options.MaxTurns must be non-negative|opts.MaxTurns > 0'` → one match each.
  - Step 3 (spec and checklist lint, one call): `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_008_spec.md docs/plans/052_runner_tool_hooks_v1_12_0_008_implement.md; echo "exit=$?"` → `exit=0`.
  - Step 4: `~/.agents/scripts/docs-lint --since 7f56dae /home/jaron/dev/axe/runner-tool-hooks; echo "exit=$?"` → `exit=0`.
  - No file changes unless step 1 applies.
  - Per-task gate: Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This task changes no files (unless step 1 applies); the derived package list is empty.

- [ ] **T9: Records (orchestrator, not the writer)**
  - Budget: 30 min
  - Owner: orchestrator, after T8.
  - Files: `docs/plans/052_runner_tool_hooks_v1_12_0_008_checkpoint.md` (phase, task states, gate evidence). The milestone entry `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:228-236` may receive a line-number-only citation refresh. The milestone checkbox `- [ ] 008` is rendered by factory-checkpoint at the Factory completion gate, after review, not in T9.
  - This writer does not touch these files.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 `Options.MaxTurns int` field | T1 (declare), T7 (apidiff), T8 (rg) |
| R2 validation before the dry-run return | T2.2 (red), T3.1 (green), T3.3 (order check), T5 |
| R3 effective cap | T2.1 (red), T2.3 (guard, zero/unset), T3.2 (green), T5 |
| R4 loop bound and error text unchanged | T2.1 (`exceeded maximum conversation turns (2)`, `RequestCount == 2`), T3.2, T3.3 |
| R5 TOML-only behavior unchanged | T2.3 (guard, zero/unset), T3.2, T4 (`TestRun_MaxTurnsExceeded`, `TestRun_MaxTurnsConfigured`) |
| R6 dry run does not display the cap; `cmd/` unchanged | T3.3 (`DryRunInfo` check), T4, T8 (`git diff --exit-code` on `cmd`) |
| R7 CLI, TOML, `--json` unchanged | T4 (`./cmd` with `XDG_CACHE_HOME`), T8 |
| R8 TOML validation unchanged | T3.3 (`internal/` unchanged), T4, T8 (`git diff --exit-code` on `internal`) |
| R9 sub-agents do not inherit the cap | T2.3 (test 4 guard), T3.3 (`executeToolCalls` not given the option), T3.4 |
| R10 documentation | T1 (doc comment), T7.1 (CHANGELOG), T8 |
| R11 acceptance tests 1-4 | T2.1 (1), T2.2 (3), T2.3 (2 and 4), T3.4, T5 |

## Acceptance Test Mapping

| Acceptance test | Function | Class | Red task | Green task |
|---|---|---|---|---|
| 1 `MaxTurns: 2` overrides TOML 5; two requests; `exceeded maximum conversation turns (2)` | `TestMaxTurnsOption_OverridesTOML` | red | T2.1 | T3.2 |
| 2 zero keeps TOML (2) and unset gives 50 | `TestMaxTurnsOption_ZeroKeepsTOMLAndDefault` | `[guard]` (PASS at HEAD) | T2.3 | none (stays PASS) |
| 3 negative value is `ConfigError` in live and dry run; zero requests | `TestMaxTurnsOption_NegativeConfigError` | red | T2.2 | T3.1 |
| 4 sub-agent does not inherit the cap | `TestMaxTurnsOption_SubAgentNotInherited` | `[guard]` (PASS at HEAD) | T2.3 | none (stays PASS) |

The focused run is the `TestMaxTurnsOption_` family in `pkg/runner` (T4, T5). `make apidiff` is T7. The race run is T5.

## Caller Coverage and Extreme Input Probes (Phase 2 Gate)

- Changed signatures: none. The change adds one field to `Options`.
  - `runner.Options` (`pkg/runner/options.go:16`): gains `MaxTurns`. The keyed literal at `cmd/run.go:80` compiles unchanged, and the CLI does not set the field (R6, OS4). `rg -n "runner.Options\{" --glob '*.go' .` returns only `cmd/run.go:80` outside `pkg/runner` tests; the `pkg/runner` tests use keyed literals.
  - `agent.Load` (`internal/agent/agent.go`): unchanged (OS6). `cmd/agents.go:97-98` prints the TOML cap; unchanged.
  - `Run` (`pkg/runner/run.go:48`) and `run` (`pkg/runner/run.go:65`): signatures unchanged.
  - `executeToolCalls` (`pkg/runner/run.go:964`): signature unchanged; the option is not passed to it (OS7).
  - `runConversationLoop` (`internal/tool/tool.go:367`): cap at `internal/tool/tool.go:368`, loop at `:374`, unchanged; the child reads its own `cfg.MaxTurns` (OS7).
- Extreme acceptance inputs (expected results; evidence comes from the tests named):
  - `-1`: `ConfigError` in live and dry run, zero requests. Covered by T2.2 (`value_-1_dry_run_false`, `value_-1_dry_run_true`) and T3.1.
  - `-5`: same. Covered by T2.2 (`value_-5_dry_run_false`, `value_-5_dry_run_true`).
  - `0`: keeps the TOML value, or 50 when unset (R5). Covered by T2.3 (`toml_2`, `unset`).
  - `1` through the 50 default: `1` through `50` override or keep as stated in R3. `1` is not a test value; the code path is the same as `2` in T2.1.
  - Very large positive value (for example `math.MaxInt`): accepted by the `> 0` rule; the loop bound is that value, and the run ends at the first text response, the budget break, or a provider error. The spec sets no upper bound; this is recorded as an assumption, not probed.
  - Dry run with a valid value (for example `2`): the validation passes, and the dry run returns before the loop. Covered by the dry-run half of T2.2 only for negatives; a positive dry run is covered by the existing dry-run tests (unchanged).
- `go install` and `GOPROXY=` commands: T7 runs `make apidiff`, which runs `go run` on a pinned module (`Makefile:3`, `Makefile:22-23`). The `go-toolchain-check` pre-flight is recorded in T0 (`exit=0`).
- `./.agents/scripts/checklist-test-gate-audit`: absent in this repository (T0, `exit=1`). Not run.

## Blockers

None. The spec is approved, and every citation in the spec and in this checklist matches HEAD `bb9f625`. Spec-lint passes with exit 0 on the spec. See "Spec observations" in the Assumptions for a non-blocking citation drift.

## Parallel Work and Serial Dependencies

- Serial: T0 → T1 → T2.1 → T2.2 → T2.3 → T2.4 → T3.1 → T3.2 → T3.3 → T3.4 → T4 → T5 → T6 → T7 → T7.1 → T8 → T9. The red tests share one new file, and the green tasks share `pkg/runner/run.go`.
- Parallel: T7.1 (CHANGELOG) touches no Go file and may be drafted during Phase 2 or 3, but it is committed in order after T7.
- Parallel: the T1 doc comment and the T7.1 bullet are independent of each other; both are documentation.
- No removal tasks. Nothing is deleted. The `maxConversationTurns := cfg.MaxTurns` line is kept (T3.2 adds lines around it), so no `Depends on reads:` line is needed.

## Assumptions

1. The validation wording (A1) and the cap placement (A2) are as stated in Section 1. Neither changes behavior the spec fixes.
2. Spec tests 2 and 4 are guards (A3). Only tests 1 and 3 are red at HEAD. The red reason for test 1 is the missing override: at HEAD the loop runs to the TOML cap of 5, and the third request reaches the mock's unexpected-request fatal (`internal/testutil/mockserver.go:77`).
3. The red-test failure messages above are predicted from code reading and the mock's behavior. They are confirmed in T2.1-T2.4; any mismatch is reported there.
4. The derived race package list is `./pkg/runner` only. No file in `internal/` or `cmd/` changes.
5. Test fixtures mirror `TestRun_MaxTurnsConfigured` (`pkg/runner/run_test.go:1336`); `Workdir` is not set, as there.
6. Compact JSON: Go `encoding/json` emits no spaces, so the request-body assertions in T2.3 (test 4) match the `call_agent` and `child done` strings as written.
7. The spec does not fix an upper bound on `Options.MaxTurns`. The `> 0` rule (R3) applies to all positive values, so no upper bound is added.

### Spec observations (non-blocking; reported, spec not edited)

- The spec's OS7 and test-4 text cite `internal/tool/tool.go:335-345` for the child loop error turning into an error tool result. At HEAD the error return is `internal/tool/tool.go:333` (`return errorResult(call.ID, agentName, err.Error(), opts)`). The claim is true; the cited range is about two lines off. The checklist cites `:333`. The spec passes spec-lint (67 citations, 0 findings).
- The spec and milestone cite lines that the spec corrects (for example `pkg/runner/run.go:630`, `pkg/runner/run.go:760`). Those corrections match HEAD and are used here.

## Limitations

- `Options.MaxTurns` does not apply to `call_agent` children. They use their own TOML `max_turns` or 50 (OS7). The field doc comment states this.
- TOML `max_turns` is still validated at load even when `Options.MaxTurns` overrides it. A TOML file with a negative value still fails (OS6).
- The wording of the negative-value message is not a stable public contract (A1, OS5).
- The dry run does not show the effective cap (OS4). The CLI cannot set `Options.MaxTurns`, so the CLI regression check is the existing turn tests and goldens (R7), not a new CLI test.
- `cmd/` tests need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005-007.
- Milestone entry 008 cites stale lines. The corrected lines are in the spec's "Citation corrections (verified at `7f56dae`)". The milestone document is not edited by this checklist (T9 is the orchestrator's).
- The full race run over all packages, `make check`, and `checklist-test-gate-audit` (absent here) are not run by this checklist. The Factory completion gate runs the full race run.
