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
- Current phase: `checklist-pending`
- Current task: none
- Task states — checklist checkboxes render from this list:
none
- Milestone checkbox: `unchecked`
- Next safe action: Draft implementation checklist 007_implement.md, then checklist planning review r1
- Last completed phase/task/gate and timestamp: Spec planning review r1 APPROVED, 2026-11-09T22:18:07.031362+00:00

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

## Limitations & blockers

- Limitations: planning review and the checklist are not run in this phase. `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox (environmental, as recorded for 005 and 006). The sub-agent in test 6 uses `Requests[1]` as the child request, which the spec states; the implementer must confirm the ordering against the mock.
- Blockers: none.
- Next action: Draft implementation checklist 007_implement.md, then checklist planning review r1
