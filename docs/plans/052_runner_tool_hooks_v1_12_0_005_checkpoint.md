# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `005`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 005 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:189-201`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Base commit: `c2620fbe11fffe5ad5c73e61ba4c0d63f298e6fa`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `c2620fb`). Go pre-flight this session: `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0 (go1.26.0/go1.26.10, govulncheck no vulnerabilities). Known out-of-scope: gofmt drift in `pkg/runner/result.go` and `pkg/runner/result_test.go`.

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_005_implement.md` (not yet created)
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md`

## State

- Lane: `full` — the milestone adds exported identifiers to `pkg/runner`, the external Go API contract (D1), and runs on the parallel tool dispatch path in `executeToolCalls` (`pkg/runner/run.go:896`, parallel branch `:985-1018`). The fast-lane condition "external contracts untouched" fails.
- Current phase: `implement`
- Current task: none
- Task states — checklist checkboxes render from this list:
- T0: `unchecked`
- T1: `unchecked`
- T2.1: `unchecked`
- T2.2: `unchecked`
- T2.3: `unchecked`
- T2.4: `unchecked`
- T2.5: `unchecked`
- T2.6: `unchecked`
- T2.7: `unchecked`
- T2.8: `unchecked`
- T3.0: `unchecked`
- T3.1: `unchecked`
- T3.2: `unchecked`
- T3.3: `unchecked`
- T3.4: `unchecked`
- T3.5: `unchecked`
- T4: `unchecked`
- T5: `unchecked`
- T6: `unchecked`
- T7: `unchecked`
- T7.1: `unchecked`
- T8: `unchecked`
- T9: `unchecked`
- Milestone checkbox: `unchecked`
- Review: not started
- Blockers: none
- Relevant work: `uncommitted` — the two new spec and checkpoint files are untracked; no code, milestone doc, or 004 artifact is changed; nothing committed.
- Next safe action: Apply post-cap F1 fix to 005 checklist and spec, then explore check
- Last completed phase/task/gate and timestamp: 005 checklist planning review round 1 CHANGES REQUESTED; FAILING-ROUND CAP REACHED (3/3: spec r1, spec r2, checklist r1), 2026-11-09T19:39:50.612283+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures. The orchestrator records gate evidence with factory-checkpoint; this phase does not add gate entries.

### 005 spec drafting (phase: spec)

- Gate: spec drafting (no planning review yet)
- Command(s) and actual result(s): evidence read at HEAD `c2620fb` (`pkg/runner/hook.go`, `options.go`, `run.go`, `result.go`, `messages.go`, `internal/provider/provider.go`, `run_test.go`, `cmd/golden_test.go`, `cmd/testdata/golden/json/with_tools.json`, `CHANGELOG.md`, `CONTRIBUTING.md:185`). OD1-OD15 recorded as settled in the spec. No BLOCKED items. Lint: `spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md` first run exit 1 (6 stale-citation findings: symbol not on the cited line; 1 `-run` pattern selecting no test). Fixed by correcting citations and moving the focused race check to prose. Rerun exit 0: 160 citations, 0 findings. `docs-lint --since c2620fb /home/jaron/dev/axe/runner-tool-hooks` exit 0: 189 Markdown files, 0 findings.
- `HEAD` commit SHA: `c2620fbe11fffe5ad5c73e61ba4c0d63f298e6fa`
- `git status --porcelain` (after both lints): `?? docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md` and `?? docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md`. Nothing else.
- Relevant work: `uncommitted` — stash ref `none`

### 005 spec planning review round 1 pre-lint PASS

- Gate: 005 spec planning review round 1 pre-lint PASS
- Commands/results:
milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md: exit 0 (13 entries). factory-sync: exit 0 ahead 60 behind 0 synced. spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md: exit 0, 160 citations, 2 -run patterns, 0 findings.
- HEAD commit SHA: `2a40fb7030978a9e282ca20ee2d32af0d3474328`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Planning review of 005 spec, round 1

### 005 spec planning review round 1 CHANGES REQUESTED (failing round 1/3); fixes committed cefc765; spec-lint PASS

- Gate: 005 spec planning review round 1 CHANGES REQUESTED (failing round 1/3); fixes committed cefc765; spec-lint PASS
- Commands/results:
Round 1 review: CHANGES REQUESTED, 5 blocking: F1 no dispatch-cancel test; F2 sub-agent observer test missing; F3 hook-panic and dispatch-error cases missing; F4 test 1 cases inconsistent ('fourth case'); F5 wrong cross-refs (test 6 vs 9, test 1 vs 3). Fixed in cefc765 (tests 10, 11, 12 added; 3(e); 1a-1d). spec-lint exit 0, 166 citations, 0 findings.
- HEAD commit SHA: `cefc765961e0dabb878a3c5e4d630c6e3d7f4f77`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Planning review of 005 spec, round 2

### 005 spec planning review round 2 CHANGES REQUESTED (failing round 2/3); fixes committed 75f95e0; spec-lint PASS

- Gate: 005 spec planning review round 2 CHANGES REQUESTED (failing round 2/3); fixes committed 75f95e0; spec-lint PASS
- Commands/results:
Round 2 review: F2-F5 resolved; F1 open (test 10 timing-dependent); new F6 (no hook ToolAllow case), F7 (observer-set JSON-off ToolCallDetails nil untested). Fixed in 75f95e0: test 10 marker-synchronized cancel with exec sleep (probe: plain sleep orphan holds pipe 30s, exec form 302ms); test 3(f); test 2 nil assertion. spec-lint exit 0, 166 citations, 0 findings.
- HEAD commit SHA: `75f95e0f66f4c6918906e19c8d69dcb3aafd357b`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Planning review of 005 spec, round 3

### 005 spec planning review round 3 APPROVED (failing rounds so far 2/3)

- Gate: 005 spec planning review round 3 APPROVED (failing rounds so far 2/3)
- Commands/results:
Round 3 review at 75f95e0: APPROVED, F1/F6/F7 resolved, no new findings. spec-lint exit 0, 166 citations.
- HEAD commit SHA: `75f95e0f66f4c6918906e19c8d69dcb3aafd357b`
- Relevant work: `committed` — stash ref `aa69ac6d8a6b67c8a4e6b46890e274710d91ff36` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Write 005 implementation checklist (implement skill), spec-lint, planning review

### 005 checklist drafted 3919eb2; spec-lint PASS

- Gate: 005 checklist drafted 3919eb2; spec-lint PASS
- Commands/results:
spec-lint --root <repo> docs/plans/052_runner_tool_hooks_v1_12_0_005_implement.md: exit 0, 81 citations, 0 findings. Orchestrator decision: Action carried via unexported toolExecResult.Action (option a).
- HEAD commit SHA: `3919eb2a4f43cedb18dd21655cfb115660a13dd9`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Planning review of 005 checklist, round 1 (failing rounds so far 2/3)

### 005 checklist planning review round 1 CHANGES REQUESTED; FAILING-ROUND CAP REACHED (3/3: spec r1, spec r2, checklist r1)

- Gate: 005 checklist planning review round 1 CHANGES REQUESTED; FAILING-ROUND CAP REACHED (3/3: spec r1, spec r2, checklist r1)
- Commands/results:
Unresolved at cap: F1 (blocking) implement.md:86 and :100 (and spec test 2 / 3(f)) prescribe capturingProvider.requests[1], but capturingProvider.Send (pkg/runner/run_test.go:50-52) returns text only, so no tool runs and requests[1] does not exist (run.go:645-648). Reviewer fix: use testutil.NewMockLLMServer with AnthropicToolUseResponse then AnthropicResponse in T2.2 and T2.3(f), decode tool result from mock.Requests[1].Body; do not change capturingProvider. Plan: apply fix to checklist and spec, one explore check, then Phase 3 with per-task TDD gates; no further planning review.
- HEAD commit SHA: `3919eb2a4f43cedb18dd21655cfb115660a13dd9`
- Relevant work: `committed` — stash ref `bc22ca41f4f0f363c9fb2d19d9f6cab321f1ada8` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Apply post-cap F1 fix to 005 checklist and spec, then explore check

## Limitations & blockers

- Limitations: planning review and the checklist are not run in this phase. The milestone's test 6 (`--json` golden unchanged) is a regression check through `cmd/golden_test.go`, which may need a writable `~/.cache/axe` in this sandbox (environmental, as recorded for 004).
- Blockers: `none`
- Next action: Apply post-cap F1 fix to 005 checklist and spec, then explore check
