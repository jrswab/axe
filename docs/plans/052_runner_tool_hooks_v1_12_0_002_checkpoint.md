# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `002`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 002 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154-159`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Review base: `423a3994c56c9e8ee055e03072b80f6741589f68`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `423a399`).

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_002_implement.md` (not yet written; planned path)
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_002_checkpoint.md`

## State

- Lane: `full` — the milestone enforces the `pkg/runner` external contract (the apidiff gate) and edits shared CI (`.github/workflows/go.yml`, `Makefile`). The fast-lane condition "external contracts untouched" fails, and the milestone adds a pinned third-party tool to the build path.
- Current phase: `execute`
- Current task: T6
- Task states — checklist checkboxes render from this list:
- T0: `checked`
- T1.1: `checked`
- T1.2: `checked`
- T1.3: `checked`
- T2: `unchecked`
- T3: `checked`
- T4: `checked`
- T5: `checked`
- T6: `unchecked`
- T7: `unchecked`
- T8: `unchecked`
- T9: `unchecked`
- T10: `unchecked`
- T11: `unchecked`
- T12: `unchecked`
- Milestone checkbox: `unchecked`
- Next safe action: Implement T6-T11 behavior probes
- Last completed phase/task/gate and timestamp: T0-T5 verification PASS at 9f1782f (T2 local only; host CI pending), 2026-11-09T16:33:48.209233+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures:

### Pre-flight and sync

- Gate: Go pre-flight
  - Command(s) and actual result(s): `go version` → `go version go1.26.10 linux/amd64`
  - `HEAD` commit SHA: `423a3994c56c9e8ee055e03072b80f6741589f68`
  - `git status --porcelain`: empty
  - Relevant work: `committed` — stash ref `none`
  - Untracked file Git objects: none

- Gate: factory-sync (reported by the orchestrator; not re-run by this phase)
  - Command(s) and actual result(s): `factory-sync` exit 0, ahead 25, behind 0, no conflicts, at `423a399`
  - `HEAD` commit SHA: `423a3994c56c9e8ee055e03072b80f6741589f68`
  - Relevant work: `committed` — stash ref `none`

### apidiff probe (subagent, this session; all writes under `/tmp/apidiff-probe.Cdxk`; repository untouched)

- Gate: apidiff probe (evidence for the spec's D-pin and D-fail decisions)
  - Pin resolution: `go list -m -json golang.org/x/exp@latest` → `v0.0.0-20261106192419-47ce03d25ef0` (Time `2026-11-06T19:24:19Z`), cross-checked against `proxy.golang.org` `@latest`.
  - Base export: `git archive v1.11.0` extracted to `/tmp/apidiff-probe.Cdxk/base`; `go run golang.org/x/exp/cmd/apidiff@v0.0.0-20261106192419-47ce03d25ef0 -w /tmp/apidiff-probe.Cdxk/old.export github.com/jrswab/axe/pkg/runner` → exit 0. The base `go.mod` declares `go 1.25.0`; the toolchain switched to go1.26.10; base `go.mod`/`go.sum` unchanged.
  - Compare from current tree (`go run ... /tmp/apidiff-probe.Cdxk/old.export github.com/jrswab/axe/pkg/runner`): exit 0, empty output. Repo `go.mod`/`go.sum` unchanged.
  - Scratch copy, `func IsRuntimeError` removed: stdout `Incompatible changes:` / `- IsRuntimeError: removed`, exit 0.
  - Same removal with `-incompatible`: stdout `- IsRuntimeError: removed`, exit 0.
  - Scratch copy, `func ProbeNew() {}` added: listed under `Compatible changes:`, exit 0.
  - Source reading (verified by the phase author at the pin; module cache file `golang.org/x/exp@v0.0.0-20261106192419-47ce03d25ef0/cmd/apidiff/main.go`): flags `-w`, `-incompatible`, `-allow-internal`, `-m` are declared at lines 27-30. The `-w` export path exits 0 at line 69. The compare path prints the report at lines 96-103. Only write and load errors call `die`, which exits 1 (lines 333-335). The incompatible-only text writer is `TextIncompatible` in module file `apidiff/report.go` (line 45). apidiff therefore never exits non-zero for an incompatible change.
  - Conclusion: the gate must test stdout, not the exit status (decision D-fail in the spec).

### Citation and action-defaults verification (this phase)

- Gate: spec citation check and action defaults
  - Command(s) and actual result(s): `curl` of `actions/checkout@v4/action.yml` and `actions/setup-go@v5/action.yml` from `raw.githubusercontent.com` (HTTP 200); defaults read with PyYAML. `gh api` could not run (no `gh` credentials in this sandbox), so the raw-file route was used.
  - Spec-lint: not run by this phase (the orchestrator runs it).

### Spec planning review: round 1 CHANGES REQUESTED (F1 blocking, F2/F3 nits); round 2 first attempt returned no Verdict line (not counted; findings F4/F5 applied); round 2 CHANGES REQUESTED (F6); round 3 APPROVED. Failing planning rounds: 2 of 3. spec-lint exit 0 before each round.

- Gate: Spec planning review: round 1 CHANGES REQUESTED (F1 blocking, F2/F3 nits); round 2 first attempt returned no Verdict line (not counted; findings F4/F5 applied); round 2 CHANGES REQUESTED (F6); round 3 APPROVED. Failing planning rounds: 2 of 3. spec-lint exit 0 before each round.
- Commands/results:
spec-lint --root <repo> docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md: PASS, 57 citations, 0 findings (exit 0). docs-lint --since 423a399: exit 0, 182 files, 0 findings.
- HEAD commit SHA: `423a3994c56c9e8ee055e03072b80f6741589f68`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_002_checkpoint.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
b13593ab78e091210f982858aa2c1cb50120f443  "docs/plans/052_runner_tool_hooks_v1_12_0_002_checkpoint.md"
8571faf686227f67271c59e943c637742ee870db  "docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md"
```

- Next action: Write the 002 implementation checklist (docs/plans/052_runner_tool_hooks_v1_12_0_002_implement.md) and run its planning review

### Checklist planning review round 1 CHANGES REQUESTED (F1 blocking: recipe shape/false self-compare; F2 blocking: corrupted-export probe line not joined; F3 nit: shared temp-dir count). Third failing planning round for milestone 002 (spec r1, spec r2, checklist r1): failing-round cap reached. Fixes applied; post-cap explore check CHECK: PASS (F1, F2, F3 FIXED; dynamic make -n parse blocked by read-only guard, to be confirmed in T1.3). Phase 3 proceeds with per-task gates.

- Gate: Checklist planning review round 1 CHANGES REQUESTED (F1 blocking: recipe shape/false self-compare; F2 blocking: corrupted-export probe line not joined; F3 nit: shared temp-dir count). Third failing planning round for milestone 002 (spec r1, spec r2, checklist r1): failing-round cap reached. Fixes applied; post-cap explore check CHECK: PASS (F1, F2, F3 FIXED; dynamic make -n parse blocked by read-only guard, to be confirmed in T1.3). Phase 3 proceeds with per-task gates.
- Commands/results:
spec-lint --root <repo> 002_implement.md and 002_spec.md: both PASS, 0 findings (exit 0). Explore non-blocking notes: mktemp has no explicit status check (spec-conformant); per-probe TMPDIR count is stricter than R7 glob.
- HEAD commit SHA: `3920c71817929a1face2bd1a3672c7b1ab684689`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_002_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
4c7d23a9786556fa5fa52176cb2a386e7be707ed  "docs/plans/052_runner_tool_hooks_v1_12_0_002_implement.md"
```

- Next action: Implement T0

### Post-cap mechanical fix of stale T0 precondition (58eceb6) PASS

- Gate: Post-cap mechanical fix of stale T0 precondition (58eceb6) PASS
- Commands/results:
factory-sync (args []) exit 0: ahead 27, behind 0, shared_files [], conflicts [], synced true at b0eff05. go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks exit 0 (go1.26.10; govulncheck: No vulnerabilities found). Post-cap mechanical fix (planning review cap reached; no new planning review): 002_implement.md T0 precondition now requires HEAD descending from b0eff05 (git merge-base --is-ancestor b0eff05 HEAD), git diff --name-only b0eff05 HEAD -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*' empty, git status --porcelain --untracked-files=no empty; Section 1 baseline :7 now b0eff05. Remaining 3920c71 hits at :317 and :340 are historical citation records. spec-lint --root <repo> 002_implement.md exit 0 (36 citations, 0 findings). Committed 58eceb6.
- HEAD commit SHA: `58eceb6e907c33442d4cd83700ea7c971ee4495a`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### T0-T5 verification PASS at 9f1782f (T2 local only; host CI pending)

- Gate: T0-T5 verification PASS at 9f1782f (T2 local only; host CI pending)
- Commands/results:
code agent at 049d5d8 then committed 9f1782f (Makefile, .github/workflows/go.yml, COMPATIBILITY.md, CHANGELOG.md; 38+/8-). T0 checks exit 0 (v1.11.0 = 20e6ada; pin resolves, GoVersion 1.26.0). make -n apidiff / make -n check exit 0, show -incompatible and (cd "$tmp/base"; make -n apidiff | sh -n exit 0. rg @latest / git worktree / set -e|pipefail in Makefile: no hits. One pin Makefile:3, identical in COMPATIBILITY.md. go.yml: one fetch-depth (:57), one make apidiff (:65); yaml.safe_load and actionlint exit 0; existing jobs unchanged. COMPATIBILITY.md: no planned/not active text; citation CHANGELOG.md:24 -> :25; go.mod:3/:5 correct. make apidiff exit 0, stdout 0 bytes, stderr 204 bytes (toolchain switch notices). git diff 423a399 -- go.mod go.sum empty; git worktree list unchanged; no axe-apidiff.* temp dirs left. Per-task Go gate: no Go package changed, exit 0. T4 literal regex defect fixed post-cap in c597dce (added ^\+([^+]|$) -> 1; removed ^-([^-]|$) -> none); spec-lint exit 0. T2 stays unchecked until host CI runs the apidiff job.
- HEAD commit SHA: `c597dce6b68b23f02b4d7c1f53ca3f87cd8703e2`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T6-T11 behavior probes

## Limitations & blockers

- Limitations:
  - The apidiff pin was verified by a probe in this session; the gate itself is not yet implemented, so `make apidiff` and the CI job have not run.
  - `gh` is not authenticated in this sandbox; action defaults were read from raw GitHub URLs at tags `v4` and `v5`.
  - `AGENTS.md:207` describes `go.yml` as "tests and linting"; this milestone does not edit it (outside milestone 002 scope). Flagged for the orchestrator, not changed.
- Blockers: `none`
- Next action: Implement T6-T11 behavior probes
