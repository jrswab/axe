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
- Current phase: `execute`
- Current task: T9
- Task states — checklist checkboxes render from this list:
- T0: `checked`
- T1: `checked`
- T2.1: `checked`
- T2.2: `checked`
- T2.3: `checked`
- T2.4: `checked`
- T2.5: `checked`
- T2.6: `checked`
- T2.7: `checked`
- T2.8: `checked`
- T3.0: `checked`
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
- T9: `unchecked`
- Milestone checkbox: `unchecked`
- Review: not started
- Blockers: none
- Relevant work: `uncommitted` — the two new spec and checkpoint files are untracked; no code, milestone doc, or 004 artifact is changed; nothing committed.
- Next safe action: T9 records: commit records, then review gates (records-stale-check c2620fb, factory-gate test+lint, review split per scope)
- Last completed phase/task/gate and timestamp: T4-T8 PASS (CHANGELOG a85cf51; citation refresh uncommitted at this write), 2026-11-09T20:20:41.593819+00:00

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

### 005 post-cap F1 fix committed 716e580; explore check PASS; spec-lint PASS both

- Gate: 005 post-cap F1 fix committed 716e580; explore check PASS; spec-lint PASS both
- Commands/results:
Post-cap fix: T2.2/T2.3(f) and spec tests 2/3(f) use NewMockLLMServer + hookTestToolResult (run_test.go:1463). explore post-cap check: PASS (checks 1-5), notes: implement.md:86 still mentions ToolResults[0].Content wording (non-defect); copy Stdin strings.NewReader("") from template run_test.go:1521. spec-lint implement exit 0 (91 citations), spec exit 0 (172). Phase 3 runs with per-task TDD gates per planning-review cap.
- HEAD commit SHA: `716e5800f4db6938516c451bac88930c0614b40b`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### T0-T2.8 PASS (red tests committed a27cdf6)

- Gate: T0-T2.8 PASS (red tests committed a27cdf6)
- Commands/results:
T0: literal rg prerequisite matched unrelated pendingToolEvents (internal/provider/openrouter.go:361-456, unchanged since c2620fb); orchestrator authorized exact-identifier rg '\b(ToolObserver|ToolEvent)\b' -> exit 1 no output. T1: hook.go:53 ToolEvent, :69 ToolObserver, options.go:37 Options.ToolObserver. T2.1-T2.8: 11 tests run_test.go:1889-2620; red: 1a/1b/1c, FullContentLength, ActionDecisions a-f, PanicLine, NilHookAllow, DispatchCancel (0.03s), DispatchError, SubAgentNoHook fail 'observer events = 0'; expected-green: 1d, NilUnchanged, CancelDuringDecision, DryRunAndBudget. go test -race -skip TestToolObserver ./pkg/runner exit 0; go vet, go build ./..., golangci-lint ./pkg/runner 0 issues; gofmt touched files clean (pre-existing result.go drift).
- HEAD commit SHA: `a27cdf67af8332ec29e64005f2a29917aad85cf2`
- Relevant work: `committed` — stash ref `1a60b15e84699e43d620a49e4fd0f0ce35a6efd0` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T3.0-T3.5 (green)

### T3.0-T3.5 PASS (green committed 883402a)

- Gate: T3.0-T3.5 PASS (green committed 883402a)
- Commands/results:
run.go only (+34/-3): toolExecResult.Action :37; decision actions :985-988; dispatch ToolAllow :1014, :1042; event delivery :671-688; callToolObserver :822-830. Mutations (drop seq/par Action, remove recover, alias args, truncated length, unknown-action normalization) each red then green. All TestToolObserver* PASS. go test -race ./pkg/runner exit 0; -count=3 DispatchCancel exit 0; vet, build, golangci-lint 0 issues; gofmt clean.
- HEAD commit SHA: `883402acb8a48c77d69a386b06c21787fa99c2cc`
- Relevant work: `committed` — stash ref `dc2b93a18560ba8859338b3cc2c3afc3bc924ead` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T4-T8

### T4-T8 PASS (CHANGELOG a85cf51; citation refresh uncommitted at this write)

- Gate: T4-T8 PASS (CHANGELOG a85cf51; citation refresh uncommitted at this write)
- Commands/results:
T4: go test -v ./pkg/runner all 23 observer lines PASS; go build, go vet clean; XDG_CACHE_HOME=$(mktemp -d) go test ./cmd -count=1 ok 11.391s; cmd/testdata unchanged. T5: go test -race -v ./pkg/runner ok, no DATA RACE. T6: golangci-lint run 0 issues. T7: make apidiff exit 0, empty output. T7.1: CHANGELOG.md:14 Options.ToolObserver bullet (a85cf51). T8: five-file diff vs c2620fb exact; go.mod/go.sum/cmd/internal unchanged; run.go removed lines = 3 (expected 3); initial spec-lint FAIL (stale citations from line shifts) fixed by line-number-only refresh (187 tokens); after refresh spec-lint spec exit 0 (172 citations), implement exit 0 (91); docs-lint --since c2620fb exit 0, 190 files, 0 findings. Checklist T0 rg changed to \b form; T8 note on per-task commits.
- HEAD commit SHA: `a85cf51c2351ba393022e9c0d5c27bbb138b0e23`
- Relevant work: `uncommitted` — stash ref `d8a7cd436463556855cd7da527b23395d7a01b08` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_implement.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_005_spec.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: T9 records: commit records, then review gates (records-stale-check c2620fb, factory-gate test+lint, review split per scope)

## Limitations & blockers

- Limitations: planning review and the checklist are not run in this phase. The milestone's test 6 (`--json` golden unchanged) is a regression check through `cmd/golden_test.go`, which may need a writable `~/.cache/axe` in this sandbox (environmental, as recorded for 004).
- Blockers: `none`
- Next action: T9 records: commit records, then review gates (records-stale-check c2620fb, factory-gate test+lint, review split per scope)
