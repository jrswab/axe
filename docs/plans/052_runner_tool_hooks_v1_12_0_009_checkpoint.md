# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `009`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 009 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:238-250`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Milestone review base: `08d0f26c1ca5f076725ba21ce09a83ac89c815b7`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `08d0f26`). Setup evidence is recorded under Gate evidence.

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_009_implement.md`
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_009_checkpoint.md`

## State

- Lane: `full` — the milestone adds exported fields (`Options.APIKey`, `Options.BaseURL`) to `pkg/runner`, the external API (public Go API = external contract, `COMPATIBILITY.md:5-10`). The fast-lane condition "external contracts untouched" fails.
- Current phase: `complete`
- Current task: none
- Task states — checklist checkboxes render from this list:
- T0: `checked`
- T1: `checked`
- T2.1: `checked`
- T2.2: `checked`
- T2.3: `checked`
- T2.4: `checked`
- T2.5: `checked`
- T2.6: `checked`
- T3.1: `checked`
- T3.2: `checked`
- T3.3: `checked`
- T4: `checked`
- T5: `checked`
- T6: `checked`
- T7: `checked`
- T7.1: `checked`
- T8: `checked`
- T8.1: `checked`
- T9: `checked`
- T10: `checked`
- Milestone checkbox: `checked`
- Next safe action: Push branch (factory-gh push --require-open-pr), CI snapshot, then select milestone 010
- Last completed phase/task/gate and timestamp: Review and completion gate PASS (HEAD 70c21a6), 2026-11-10T02:43:49.893292+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures.

Planning-cap evidence stays separate: count failing planning rounds, list unresolved findings, and record post-cap fixes and their `explore` check.

### 009 setup (phase: setup)

- Gate: milestone-entries
- Command(s) and actual result(s): `milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` exit 0, `PASS: ... 13 milestone entries` (orchestrator-supplied; not re-run by the spec agent).
- `HEAD` commit SHA: `08d0f26c1ca5f076725ba21ce09a83ac89c815b7`
- `git status --porcelain`: empty at setup (verified by the spec agent before writing files).
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
none at setup
```

### 009 setup (phase: setup) — factory-sync and toolchain

- Gate: factory-sync `[]`
- Command(s) and actual result(s): exit 0; base `origin/master`, ahead 115, behind 0, shared_files `[]`, conflicts `[]`, synced true, head `08d0f26` (orchestrator-supplied).
- Gate: go-toolchain-check
- Command(s) and actual result(s): `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0, PASS all (go1.26.0, go1.26.10 available, module loads read-only, govulncheck `No vulnerabilities found`). `go version` go1.26.10, GOTOOLCHAIN auto (orchestrator-supplied).
- `HEAD` commit SHA: `08d0f26c1ca5f076725ba21ce09a83ac89c815b7`
- Relevant work: `committed` — stash ref `none`

### 009 spec drafting (phase: spec)

- Gate: spec drafting (no planning review yet)
- Command(s) and actual result(s):
  - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md`, first run exit 1 with 49 findings. Causes were lint heuristics, not bad citations: a symbol-before-citation rule read the word `is` in "`Sym` is `path:line`"; the removal rule read "replaces `apiKey`"; dotted host and model literals were read as file names; and the `-run` prose was checked as a selector outside a fence. Fixes were rewording only, with no change to any citation's target or to any requirement. Later runs fixed the remaining 5 findings, including bare `:N` citations that needed the full `path:line` form.
  - Rerun exit 0: `PASS: .../052_runner_tool_hooks_v1_12_0_009_spec.md: 98 citations, 0/0 gocyclo values checked, 0 findings`.
- `HEAD` commit SHA: `08d0f26c1ca5f076725ba21ce09a83ac89c815b7`
- `git status --porcelain` (before this checkpoint was written):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_009_spec.md
```

- Relevant work: `uncommitted` — stash ref `none`; untracked spec and checkpoint only.
- Untracked file Git objects: none (not stored; never applied).
- Next action: spec planning review round 1.

### Spec planning review r1 APPROVED WITH NITS; nits fixed in c2e20a2

- Gate: Spec planning review r1 APPROVED WITH NITS; nits fixed in c2e20a2
- Commands/results:
spec-lint 009 spec: exit 0, 100 citations, 0 findings. Review r1 APPROVED WITH NITS: N1 bedrock no-region assertions, N2 apidiff wording, N3 base-URL probe sub-run; all fixed in c2e20a2, grep -F 'only additions' 0 hits. Failing planning rounds: 0 of 3.
- HEAD commit SHA: `c2e20a24d377e7f9857a1e8e3670e18aaf76e507`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Write implementation checklist 052_runner_tool_hooks_v1_12_0_009_implement.md

### Checklist planning review r2 APPROVED

- Gate: Checklist planning review r2 APPROVED
- Commands/results:
spec-lint 009 implement: exit 0, 101 citations, 0 findings. Checklist r1 CHANGES REQUESTED (unset-lookup guard must clear ANTHROPIC_API_KEY), fixed 0de5d02; r2 APPROVED. Failing planning rounds: 1 of 3.
- HEAD commit SHA: `0de5d024599be93fda540578efc4d45017019591`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### Post-approval plan fix: host-env independence; spec r2 APPROVED, checklist r3 APPROVED WITH NITS (nit fixed a1fea73)

- Gate: Post-approval plan fix: host-env independence; spec r2 APPROVED, checklist r3 APPROVED WITH NITS (nit fixed a1fea73)
- Commands/results:
T0 first attempt stopped: host ANTHROPIC_API_KEY non-empty, AWS_DEFAULT_REGION=us-east-1, but T0 required both empty. Plans edited in c4632db (names-only env check; red runs clear ANTHROPIC_API_KEY/AXE_ANTHROPIC_BASE_URL; test 5 clears env and guards nil err; T3.3 env-independence run). Re-review: spec r2 APPROVED, checklist r3 APPROVED WITH NITS (T3.3 rg filter dropped ok line), fixed a1fea73. spec-lint both exit 0. Failing planning rounds: 1 of 3.
- HEAD commit SHA: `a1fea73c8f5dd40376ee0a6e0f496ad00d1de7fe`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T0

### T0-T1 verification PASS (commit c4adc1e)

- Gate: T0-T1 verification PASS (commit c4adc1e)
- Commands/results:
T0: ancestry from 08d0f26 exit 0; no non-plan diff; all rg anchors matched (run.go:385,386,389; Run :48; run :65; DryRun :328); baseline go test ./pkg/runner ./internal/agent ok; go-toolchain-check PASS; env names: ANTHROPIC_API_KEY set, AWS_DEFAULT_REGION set, others unset. T1: options.go:69-80 APIKey/BaseURL with docs; go build ./... 0; go vet ./pkg/runner 0; gofmt -l options.go clean; go test -race ./pkg/runner ok; golangci-lint ./pkg/runner 0 issues.
- HEAD commit SHA: `c4adc1e16d7f0c9c72980de73a1dda6029ef7f8c`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T2.1

### T2.1-T2.6 red verification PASS (commit a9e90f1)

- Gate: T2.1-T2.6 red verification PASS (commit a9e90f1)
- Commands/results:
ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner: exit 1 as expected. FAIL: LiveRunUsesOptions (:34 API key not configured), OptionsWinOverEnv (:65 dial localhost:1), ParallelRun (:93), NotPrinted/live (:143). PASS: NotPrinted/live-fail, NotPrinted/dry, UnsetUsesLookup, BedrockIgnoresOptions, BedrockNoRegionIsConfigError. No other test failed; no compile errors, panics, or races. go vet 0; gofmt clean; golangci-lint ./pkg/runner 0 issues.
- HEAD commit SHA: `a9e90f13584cb6302fc617f2fc601029e2be7436`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T3.1

### T3.1-T3.3 green verification PASS (commit 3c6df3d)

- Gate: T3.1-T3.3 green verification PASS (commit 3c6df3d)
- Commands/results:
Commit 3c6df3d: pkg/runner/run.go +6/-0. APIKey override run.go:387-389, BaseURL override run.go:390-392, bedrock block run.go:395-401 unchanged (awk extraction diff vs 08d0f26 empty, exit 0), newProvider call run.go:413. No `if apiKey == ""` guard (rg exit 1). ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -v ./pkg/runner: exit 0, all 7 TestCredentialsOption_ tests and NotPrinted/{live,live-fail,dry} PASS, no FAIL lines. Env-independence run (ANTHROPIC_API_KEY=sk-dummy-not-a-key AXE_ANTHROPIC_BASE_URL=http://localhost:1 AWS_DEFAULT_REGION=us-east-1 AWS_REGION= AXE_BEDROCK_REGION= go test -race -count=1 -v ./pkg/runner): exit 0, all PASS. go test -count=1 ./pkg/runner ./internal/agent ./internal/config: ok x3. git diff --exit-code -- internal cmd: exit 0. go vet ./pkg/runner/: 0. gofmt -l run.go: empty. golangci-lint run ./pkg/runner/...: 0 issues. Per-task gate pkgs=./pkg/runner, go test -race: ok. Verified on HEAD 329b05d with only run.go modified, then committed as 3c6df3d.
- HEAD commit SHA: `3c6df3de11c9c25b6839370252aa2c6ddbb36386`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T4

### T4-T7.1 verification PASS (commit e73cc3e)

- Gate: T4-T7.1 verification PASS (commit e73cc3e)
- Commands/results:
All go test runs prefixed ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL=. T4 (on 8f8b10a): go test -count=1 ./pkg/runner ./internal/agent ./internal/config ok x3; XDG_CACHE_HOME=$(mktemp -d) go test -count=1 ./cmd ok; regression filter (MaxTurnsExceeded, MaxTurnsConfigured, SingleShot_NoTools, ToolHookMCPAllow, SubAgent) all PASS; 7 TestCredentialsOption_ tests + 3 NotPrinted sub-runs PASS; go build ./... 0; go vet ./pkg/runner/ 0; gofmt -l options.go run.go credentials_option_test.go empty; git diff --exit-code -- internal cmd 0. T5: go test -race -v ./pkg/runner exit 0, all credential lines PASS, rg DATA RACE exit 1. T6: golangci-lint run exit 0, 0 issues. T7: make apidiff exit 0, stdout 0 bytes, no axe-apidiff.* temp dirs; stderr only go toolchain switch notice. T7.1: CHANGELOG.md:18 bullet added verbatim, numstat 1 0, rg Options.APIKey one match; committed as e73cc3e.
- HEAD commit SHA: `e73cc3e60d9ac3fcd76449b54e99a8113b8df5f3`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T8

### T8-T10 verification PASS (commit 6299af3)

- Gate: T8-T10 verification PASS (commit 6299af3)
- Commands/results:
T8 (on a9c2740): non-plan diff 08d0f26..HEAD = exactly CHANGELOG.md, pkg/runner/credentials_option_test.go, pkg/runner/options.go, pkg/runner/run.go; git diff --exit-code -- internal cmd go.mod go.sum 0; options.go:73 APIKey, :80 BaseURL, bedrock doc lines :72 :79; CHANGELOG.md:18 one bullet; no removed lines in run.go or options.go; one opts.APIKey and one opts.BaseURL condition. T8.1: checklist citations refreshed, line numbers only (commit ac12b3a). Spec and milestone entry 009 citations refreshed, line numbers only (commit 6299af3); historical quoted citations in spec lines 17-19 left unchanged. T9: spec-lint spec+checklist exit 0 (100/101 citations, 0 findings); docs-lint --since 08d0f26 exit 0 (202 files, 0 findings); checklist-test-gate-audit absent (exit 1), audit NOT run, as the checklist T9 and T0 record. T10: this record.
- HEAD commit SHA: `6299af3789fdf16135bffcf721ccefcad994b161`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Pre-review gates: records-stale-check, docs-lint, factory-gate run on HEAD, then split code review of 08d0f26..HEAD

### Review and completion gate PASS (HEAD 70c21a6)

- Gate: Review and completion gate PASS (HEAD 70c21a6)
- Commands/results:
Pre-review: records-stale-check 08d0f26 exit 0 (no TAGS.md/.mem/changelog.d). docs-lint --since 08d0f26 exit 0 (docs-lint 1, 202 files, 0 findings). factory-gate test at 1995c84: exit 0, log /tmp/factory-gate-1001/gate-test-1995c84829fc-20261110T023208Z.log. Code review round 1 (range 08d0f26..1995c84, split): backend (options.go, run.go, credentials_option_test.go) APPROVED, no findings. Spec/records CHANGES REQUESTED: F1 checkpoint.md:19,273 stale 'not yet created' and 'No implementation or test has been run yet'. F2 checkpoint.md:272 stale claim that the milestone entry was not edited. Fixed in 70c21a6 (grep -F 0 hits). docs-lint --since 08d0f26 exit 0. Round 2 (range 1995c84..70c21a6, Markdown only) APPROVED. Milestone sync: factory-sync origin/master exit 0, behind 0, shared_files [], HEAD 70c21a6. Completion gate: factory-gate test (go test -race ./...) at 70c21a6 exit 0, log /tmp/factory-gate-1001/gate-test-70c21a652f03-20261110T024155Z.log. Lint: golangci-lint run exit 0, 0 issues at 8f8b10a (T6); no Go file changed since (non-plan diff 8f8b10a..70c21a6 is CHANGELOG.md only). make apidiff exit 0 (T7). Spec comparison: backend review confirmed R1-R8 (precedence, empty=unset, not printed, bedrock ignored, additive API). Git status clean, no pre-existing changes. checklist-test-gate-audit absent, not run.
- HEAD commit SHA: `70c21a652f03e9825a15b2d9b3301be64df0d173`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Push branch (factory-gh push --require-open-pr), CI snapshot, then select milestone 010

## Limitations & blockers

- Limitations:
  - Test 4 (parallel) reads the real process environment and the real user `config.toml`, because it must not call `t.Setenv`. Options precedence makes the asserted header deterministic (spec Limitations).
  - Sub-agents do not inherit `Options.APIKey` or `Options.BaseURL` (spec OS8). This is by design, not a gap.
  - Bedrock ignores both fields by construction (spec OS1, R5).
  - `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`.
  - Milestone entry 009's relevant citations were refreshed in `6299af3` (line numbers only). The design note (`docs/plans/runner-tool-hooks-and-options.md:475-521`) is unchanged by this milestone and still cites stale lines. The spec's "Citation corrections" section records the verified lines.
  - Implementation and the seven credential-option tests are complete, with the full-suite gate passing at `1995c84829fcbf6f58814f1115b5fb3ff9b80418`. The spec-lint PASS covers the spec only.
- Blockers: none.
- Next action: Push branch (factory-gh push --require-open-pr), CI snapshot, then select milestone 010
