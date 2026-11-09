# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `006`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 006 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203-215`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Milestone review base: `a13e3bcf8b914fad1d17692da00e4ff79eca05bd`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `a13e3bc`). Setup evidence: milestone-entries PASS (13 entries); factory-sync origin/master behind 0 at `a13e3bc`; Go pre-flight `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0 carried from the earlier session; PR 103 pushed at `a13e3bc`, CI reports no checks yet.

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_006_implement.md` (not yet written)
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_006_checkpoint.md`

## State

- Lane: `full` — the milestone adds an exported type (`MCPServer`) and an exported field (`Options.MCPServers`) to `pkg/runner`, the external API (D1). It changes config validation (a shared validator) and moves credential-bearing headers through a new input path (security-sensitive). The fast-lane conditions "external contracts untouched" and "no security-sensitive path" both fail.
- Current phase: `execute`
- Current task: T4
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
- T3.0: `checked`
- T3.1: `checked`
- T3.2: `checked`
- T3.3: `checked`
- T3.4: `checked`
- T3.5: `checked`
- T4: `unchecked`
- T5: `unchecked`
- T6: `unchecked`
- T7: `unchecked`
- T7.1: `unchecked`
- T8: `unchecked`
- T9: `unchecked`
- Milestone checkbox: `unchecked`
- Next safe action: Implement T4
- Last completed phase/task/gate and timestamp: T3.0-T3.5 PASS (green committed e9f8cf4), 2026-11-09T21:38:17.720238+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures.

### 006 spec drafting (phase: spec)

- Gate: spec drafting (no planning review yet)
- Command(s) and actual result(s):
  - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md`, first run exit 1: 3 stale-citation findings (`Connect` named on lines that do not contain it: `internal/mcpclient/mcpclient.go:26-28`, `internal/envinterp/envinterp.go:17`) and 2 `-run` listing findings (new test names that do not exist yet). Fixed: citations corrected to `mcpclient.go:25-28` and `envinterp.go:12-17`; the focused `-run` run moved to prose; the cmd `-run` line replaced by `go test ./cmd -count=1`. Rerun exit 0: 85 citations, 0 findings.
  - Milestone citation drift (BLOCKED for orchestrator, see Limitations & blockers): `run.go:443` is the verbose connect line; the text match is at `run.go:447`. `run.go:296-300` is `:300-303`. The spec uses the verified lines.
  - `~/.agents/scripts/milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` exit 0: PASS 13 entries.
- `HEAD` commit SHA: `a13e3bcf8b914fad1d17692da00e4ff79eca05bd`
- `git status --porcelain` (before checkpoint write): `?? docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied). Untracked files: spec and checkpoint only.
- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: spec planning review round 1 (the orchestrator dispatches it).

### Spec planning review r1 APPROVED

- Gate: Spec planning review r1 APPROVED
- Commands/results:
spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_006_spec.md -> exit 0, 85 citations, 0 findings. docs-lint --since a13e3bc -> exit 0. Review r1 (spec vs milestones.md:203-215): APPROVED, no findings. Milestone entry cites stale lines (run.go:443 -> current :447; run.go:296-300 -> current :300-303); spec cites verified current lines; milestone doc left unchanged (citation drift only, no scope change).
- HEAD commit SHA: `63c85f6cbf177315349ce39ef43d6686cee8bef0`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Draft implementation checklist docs/plans/052_runner_tool_hooks_v1_12_0_006_implement.md

### Checklist planning review r3 APPROVED

- Gate: Checklist planning review r3 APPROVED
- Commands/results:
Checklist review r1 (b90d95b) CHANGES REQUESTED F1-F4 blocking, fixed 41af024. r2 CHANGES REQUESTED F5 blocking (SubAgentNotInherited guard passed for wrong reason), fixed c604f42. r3 APPROVED, no new findings. Failing planning rounds for 006: 2 of 3 (checklist r1, r2). spec-lint implement.md exit 0 (135 citations, 0 findings); docs-lint --since a13e3bc exit 0. checklist-test-gate-audit not present in repo.
- HEAD commit SHA: `c604f42bc6a593197aa7545290b4a3afbdbf17c1`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### T0-T2.7 PASS (red batch committed aa4ad07)

- Gate: T0-T2.7 PASS (red batch committed aa4ad07)
- Commands/results:
T0: tree clean at 90d803c (orchestrator: checklist's expected ' M checkpoint' line was stale; checkpoint committed at 90d803c), baseline tests ok, toolchain ok. T1: go build/vet ok; go test -race ./internal/agent ./internal/envinterp ./pkg/runner ok. T2.1-T2.7: go test -race -v on the 3 packages exit 1 with exactly 12 prescribed top-level FAILs (OptionsOnlyToolsInRequest, MergedDistinctNames, DuplicateNameConfigError, InvalidServerNoConnect, UnsetHeaderEnvConfigError, HeaderExpandedOnWire, DryRunMergedNoHeaders, RunDoesNotMutateOptions, SubAgentNotInherited, VerboseNoHeaders, TestExpandHeaders_UnsetEnvSentinel, TestValidateMCPServers_SharedRules) each on the missing-behavior assertion; guard EmptySliceUnchanged PASS; 248 PASS; no races/panics. gofmt -l empty; golangci-lint 0 issues.
- HEAD commit SHA: `aa4ad07392547cb448caf3085a21d8f99a3b7a05`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T3.0

### T3.0-T3.5 PASS (green committed e9f8cf4)

- Gate: T3.0-T3.5 PASS (green committed e9f8cf4)
- Commands/results:
T3.0 agent tests ok; T3.1 envinterp+mcpclient ok; T3.2 exactly 7 permitted red functions; T3.3-T3.5 all acceptance tests PASS; derived race gate ./internal/agent ./internal/envinterp ./pkg/runner exit 0; go build/vet ./... exit 0; gofmt -l empty; golangci-lint 0 issues; XDG_CACHE_HOME=tmp go test ./cmd/... ok. 7 mutation checks red then green (validator, cross-source duplicates, sentinel wrap, option connect, dry-run headers, sub-agent non-inheritance, dry-run env reads). options.go unchanged at T3 (already satisfied). Deviation: T3.0 literal-prefix rg exit 1 because prefixes are %s-formatted; corrected search exit 0.
- HEAD commit SHA: `e9f8cf4894151f859a3ccc956190507c9fa1ee34`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T4

## Limitations & blockers

- Limitations: planning review and the checklist are not run in this phase. The milestone's "no `t.Setenv`" describes the host, not the test harness; the spec's tests use `t.Setenv`. `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox (environmental, as recorded for 005). The PR 103 state was not re-verified by this agent: `gh pr view` failed for missing auth (sandbox credential limit; not retried).
- Blockers: BLOCKED (orchestrator decision needed, not a spec gap): milestone entry 006 cites stale lines. `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:203-215` cites the text match at `pkg/runner/run.go:443`, but the match is at `:447` (`:443` is the verbose connect `Fprintf`). It cites the dry-run server list at `pkg/runner/run.go:296-300`, but it is at `:300-303`. The spec uses the verified lines. The milestone document was not edited (out of scope for this phase).
- Next action: Implement T4
