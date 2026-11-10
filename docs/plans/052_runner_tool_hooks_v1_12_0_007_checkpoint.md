# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `007`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 007 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:217-226`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Milestone review base: `de43f3c8c0cea487f35283280fd33ea52d087dc6`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `de43f3c`). Setup evidence: factory-sync origin/master ahead 84, behind 0, no conflicts (session start, recorded by the orchestrator); Go pre-flight `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0 carried from the earlier session of this run.

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_007_implement.md` (not yet written)
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_007_checkpoint.md`

## State

- Lane: `full` — the milestone adds an exported field (`Options.Tools`) to `pkg/runner`, the external API (D1). It adds a second validation path for a caller-supplied list. It changes which MCP tool names are shadowed (a security-relevant change to MCP shadowing). No new dependency is added. The fast-lane condition "external contracts untouched" fails.
- Current phase: `complete`
- Current task: none
- Task states — checklist checkboxes render from this list:
- T0: `checked`
- T1: `checked`
- T2.1: `checked`
- T2.2: `checked`
- T2.3: `checked`
- T2.4: `checked`
- T3.1: `checked`
- T3.2: `checked`
- T3.3: `checked`
- T3.4: `checked`
- T3.5: `checked`
- T4: `checked`
- T5: `checked`
- T6: `checked`
- T7: `checked`
- T7.1: `checked`
- T8: `checked`
- T9: `checked`
- Milestone checkbox: `checked`
- Next safe action: Commit completion, sync, push to PR #103, then start milestone 008
- Last completed phase/task/gate and timestamp: Review round 1 APPROVED (backend + spec/records); completion gate PASS at 0e5c5e9, 2026-11-09T23:16:13.666424+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures.

Planning-cap evidence stays separate: count failing planning rounds, list unresolved findings, and record post-cap fixes and their `explore` check.

### 007 setup (phase: setup)

- Gate: setup
- Command(s) and actual result(s): factory-sync (origin/master) ahead 84, behind 0, no conflicts (session start, orchestrator-recorded). `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0 (carried from the earlier session of this run).
- `HEAD` commit SHA: `de43f3c8c0cea487f35283280fd33ea52d087dc6`
- `git status --porcelain`: clean at setup.
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- Next action: write the spec (done below).

### 007 spec drafting (phase: spec)

- Gate: spec drafting (no planning review yet)
- Command(s) and actual result(s):
  - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md`, first run exit 1: 4 findings (1 ambiguous `run.go` path, 2 stale citations for `MCPServers` and `sub_agents` symbols, 1 `-run` listing finding). Fixed: citations corrected to `pkg/runner/run.go:353` (DryRunInfo.MCPServers assignment), `:454` and `:456` (sub-agent gate and injection), the milestone's `run.go:423` reference quoted as text, and the focused `-run` line moved to prose. A later run found one more stale symbol (`sub_agents` on `:454`), fixed by quoting `cfg.SubAgents`. Rerun exit 0: `PASS: ...007_spec.md: 65 citations, 0/0 gocyclo values checked, 0 findings`.
  - `~/.agents/scripts/docs-lint --since de43f3c /home/jaron/dev/axe/runner-tool-hooks` exit 0: `docs-lint 1: checks task-id, stale-milestone, broken-mem-link, stale-citation; changed since de43f3c (merge-base de43f3c8c0ce)`; `PASS: /home/jaron/dev/axe/runner-tool-hooks: 194 Markdown files, 0 findings, 0 outside --since scope`.
  - Citation drift (for the orchestrator; milestone document not edited): milestone 007 cites `pkg/runner/run.go:413-419` for TOML tools (current `:441-451`, gate `:445`); `run.go:423` for call_agent (current `:453-457`); `internal/agent/agent.go:176-180` (current `:176-181`). The spec uses the verified lines.
  - Decisions recorded as settled in the spec: OS1-OS12 (effective list before dry-run return; TOML rule validation; nil = TOML; empty non-nil = no built-in tools; TOML still validated at load; MCP shadowing follows effective list; no limit on MCP or `call_agent`; sub-agents do not inherit; caller slice copied; duplicates accepted; one CHANGELOG bullet; test placement).
  - Requirements: R1 public field; R2 validation before dry-run return; R3 TOML validation unchanged; R4 effective list replaces TOML list; R5 empty non-nil list; R6 dry run reports effective list; R7 nil byte-identical; R8 CLI, TOML, `--json` unchanged; R9 MCP shadowing follows effective list; R10 documentation; R11 caller data and sub-agents; R12 acceptance tests 1-6 (`TestTools_OptionReplacesTOMLList`, `TestTools_UnknownNameConfigError`, `TestTools_EmptyListNoToolsKey`, `TestTools_NilUsesTOMLList`, `TestTools_MCPShadowingFollowsEffectiveList`, `TestTools_SubAgentNotInherited`) in `pkg/runner/tools_option_test.go`.
- `HEAD` commit SHA: `de43f3c8c0cea487f35283280fd33ea52d087dc6`
- `git status --porcelain` (before checkpoint write): `?? docs/plans/052_runner_tool_hooks_v1_12_0_007_spec.md`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied). Untracked files: spec and checkpoint only.
- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Spec planning review round 1 (the orchestrator dispatches it).

### Spec planning review r1 APPROVED

- Gate: Spec planning review r1 APPROVED
- Commands/results:
spec-lint --root . 007_spec.md exit 0 (65 citations, 0 findings); docs-lint 1 --since de43f3c exit 0 (195 files, 0 findings). Spec review round 1 at ba1feb7: APPROVED, no findings. Failing planning rounds: 0 of 3.
- HEAD commit SHA: `ba1feb715548fb1cf495b879c8dfed15c287683c`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Draft implementation checklist 007_implement.md, then checklist planning review r1

### Checklist planning review r1 CHANGES REQUESTED; fix applied

- Gate: Checklist planning review r1 CHANGES REQUESTED; fix applied
- Commands/results:
Checklist drafted ef4c584 (spec-lint exit 0: spec 65, checklist 79 citations; docs-lint --since de43f3c exit 0, 196 files). Spec citation fix 452dd57 (run.go:454 -> :455). Checklist review r1 at 452dd57: CHANGES REQUESTED, 1 blocking finding: implement.md:141 and :343 claimed TestTools_OptionReplacesTOMLList request assertion green after T3.1, but the effective list arrives in T3.2. Fix: both assertions red after T3.1; red at T3.2 on dry-run only. grep -F of both quoted texts 0 hits; spec-lint exit 0. Failing planning rounds: 1 of 3 (spec r1 APPROVED, checklist r1 failed). Open item for implementer: test 5 fixture uses TOML tools = ["read_file"] (checklist Assumption 2).
- HEAD commit SHA: `452dd57216746dd857e57d4f6404bc990a9bdf3e`
- Relevant work: `uncommitted` — stash ref `2764b80c8434a997a5ac8004d21248796561be60` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_007_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Checklist planning review r2 (prior finding verbatim), then --phase execute

### Checklist planning review round 2 APPROVED (no findings); prior round-1 blocker verified resolved

- Gate: Checklist planning review round 2 APPROVED (no findings); prior round-1 blocker verified resolved
- Commands/results:
factory-sync args [] exit 0: base origin/master, ahead 88, behind 0, shared_files [], conflicts [], head 19158c4 (no merge). spec-lint 007 spec+implement exit 0 (65 and 79 citations, 0 findings). milestone-entries exit 0 (13 entries). Review round 2: Verdict APPROVED. Failing planning rounds: 1 of 3.
- HEAD commit SHA: `19158c4b00f35f4d5ceb171d02ca7885f255b524`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### T0+T1 verification PASS at fa21002

- Gate: T0+T1 verification PASS at fa21002
- Commands/results:
T0: stale expectations at implement.md:48-49 (HEAD ba1feb7, modified checkpoint) replaced with ancestry checks (merge-base --is-ancestor ba1feb7 HEAD; no non-docs/plans diff); all T0 checks matched; go-toolchain-check exit 0. T1: Options.Tools declared pkg/runner/options.go:55-62. go build ./... 0; go vet ./pkg/runner 0; golangci-lint run ./pkg/runner 0 issues; per-task gate go test -race ./pkg/runner ok; spec-lint 007 spec+implement PASS (milestones doc has 33 historical stale-citation findings, outside the gate). Commit fa21002.
- HEAD commit SHA: `fa210022b91b1b93ec18c06b0d8147845ac2c840`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T2.1

### T2.1-T2.4 red gate PASS at fa9561f

- Gate: T2.1-T2.4 red gate PASS at fa9561f
- Commands/results:
pkg/runner/tools_option_test.go (new). go test -race -v ./pkg/runner exit 1 with exactly the expected red: TestTools_OptionReplacesTOMLList, TestTools_UnknownNameConfigError (6 subtests), TestTools_EmptyListNoToolsKey, TestTools_MCPShadowingFollowsEffectiveList/b_builtin_removed, TestTools_SubAgentNotInherited; guards TestTools_NilUsesTOMLList and MCP a_builtin_kept PASS; no pre-existing failures, no races. go test -race ./pkg/runner -skip ^TestTools_ ok. go vet 0; golangci-lint 0 issues; gofmt clean; spec-lint 007 exit 0. Assumptions confirmed: Test 5 TOML tools=[read_file]; error wording noncontract (spec:25,183).
- HEAD commit SHA: `fa9561f823ad32839fd3bffae3b7963b33a437d9`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T3.1

### T3.1-T3.5 verification PASS at 20217a0

- Gate: T3.1-T3.5 verification PASS at 20217a0
- Commands/results:
pkg/runner/run.go: toolname import :29, validation before dry-run :106-111, effective list copy :112-115, registry resolve :456-457, DryRunInfo.Tools :363. Staged states matched checklist: T3.1 exit 1 (4 expected red families), T3.2 exit 1 (only dry-run assertion tools_option_test.go:61), T3.3-T3.5 exit 0. All TestTools_ PASS. go test -race ./pkg/runner ok; go test -count=1 ./pkg/runner ok; go build ./... 0; go vet 0; golangci-lint 0 issues; gofmt clean; internal/tool and internal/mcpclient unchanged vs de43f3c; spec-lint 007 spec+implement exit 0 after line-number refresh.
- HEAD commit SHA: `20217a0cac21025833e14788b86179c2a94800d1`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T4

### T4-T7.1 verification PASS at 1c2ffd1

- Gate: T4-T7.1 verification PASS at 1c2ffd1
- Commands/results:
T4: filter anchor at implement.md:250 fixed from `)$` to `)( |$)` (Go appends duration); go test 5 pkgs ok; 17 TestTools_ PASS; go test ./cmd ok; 3 existing tests PASS; go build 0; go vet ./pkg/runner 0; gofmt clean; unchanged-surface diff vs de43f3c exit 0. T5: go test -race -v ./pkg/runner ok, no DATA RACE. T6: golangci-lint run 0 issues. T7: make apidiff exit 0, 0 bytes stdout, no leftover dirs. T7.1: CHANGELOG.md:16 one bullet. go vet ./... 0; spec-lint 007 exit 0. Commit 1c2ffd1.
- HEAD commit SHA: `1c2ffd1dec898ff242b0bc0ec67fe99e31f6279f`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T8

### T8 scope verification PASS at f8bb97d; T9 records written

- Gate: T8 scope verification PASS at f8bb97d; T9 records written
- Commands/results:
T8 (range de43f3c..HEAD, clean tree): non-plan files = CHANGELOG.md, pkg/runner/options.go, pkg/runner/run.go, pkg/runner/tools_option_test.go; internal/*, cmd, go.mod, go.sum unchanged; Options.Tools field options.go:61 with doc lines 52-60; CHANGELOG.md:16 one bullet; run.go removed exactly 3 cfg.Tools lines; options.go additions only; effectiveTools resolve, dry-run field, and error format present; spec-lint 007 exit 0; docs-lint --since de43f3c exit 0 (196 files, 0 findings). T9: orchestrator records in this checkpoint; milestone checkbox renders only at the completion gate.
- HEAD commit SHA: `f8bb97d301f671c8a9d1c62588dcb55efeb93493`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Review gate: records-stale-check de43f3c, docs-lint --since de43f3c, factory-gate test and lint at HEAD, split review de43f3c..HEAD

### Review round 1 APPROVED (backend + spec/records); completion gate PASS at 0e5c5e9

- Gate: Review round 1 APPROVED (backend + spec/records); completion gate PASS at 0e5c5e9
- Commands/results:
Review prep: records-stale-check de43f3c: no removed or renamed names. docs-lint 1 --since de43f3c exit 0 (196 files, 0 findings). Review range de43f3c..HEAD (>5 commits) split: backend (options.go, run.go, tools_option_test.go) Verdict APPROVED; spec/records (CHANGELOG.md, 007 spec/implement/checkpoint) Verdict APPROVED; no nits. Completion: factory-sync args [] exit 0 (ahead 98, behind 0, no merge, HEAD unchanged 0e5c5e9). (1) go test -race ./... gate-test-0e5c5e92d3b3-20261109T231159Z exit 0; lint gate-lint-0e5c5e92d3b3-20261109T231203Z exit 0. (2) explore audit R1-R12, R12.1-R12.6, milestone AC1-4 + compat all MET; minor non-blocking coverage notes: R6 nil dry-run equality and R5 no-tool-loop are implementation-only. (3) git status clean; changed files de43f3c..HEAD = CHANGELOG.md, options.go, run.go, tools_option_test.go, 007 spec/implement/checkpoint, all accounted for; no pre-existing changes. (4) milestone 007 rendered checked.
- HEAD commit SHA: `0e5c5e92d3b3cd9d8d9b77104df779dc39dcbbe6`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Commit completion, sync, push to PR #103, then start milestone 008

## Limitations & blockers

- Limitations: planning review and the checklist are not run in this phase. `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox (environmental, as recorded for 005 and 006). The sub-agent in test 6 uses `Requests[1]` as the child request, which the spec states; the implementer must confirm the ordering against the mock.
- Blockers: none.
- Next action: Commit completion, sync, push to PR #103, then start milestone 008
