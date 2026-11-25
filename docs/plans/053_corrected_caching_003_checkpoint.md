# Milestone Checkpoint

## Identity

- Milestone ID: `003`
- Milestone source: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_milestones.md`
- Repository root: `/home/jaron/dev/axe/iss-83-finish-caching`
- Branch: `iss-83-finish-caching`
- Baseline / pre-existing changes at administrative setup: ` M docs/plans/053_corrected_caching_002_checkpoint.md`

## Artifacts

- Spec: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_003_spec.md` (approved at spec review round 2)
- Implementation checklist: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_003_implement.md` (drafted; spec-lint PASS)
- Checkpoint: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_003_checkpoint.md`
- Parking: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_parked.md`

## State

- Lane: `full` — the change alters default `--verbose` stderr content, and COMPATIBILITY.md's opt-in rule makes the external-contract fast-lane condition doubtful, so full (when in doubt, full).
- Current phase: `execute`
- Current task: T2
- Task states — checklist checkboxes render from this list:
- T1: `checked`
- T2: `unchecked`
- Milestone checkbox: `unchecked`
- Next safe action: T2
- Last completed phase/task/gate and timestamp: T1 reviews passed; nits batched, 2026-11-25T18:39:04.389040+00:00

## Gate evidence

### Decision ledger

- 003 inherits no procedure exceptions; the narrow Q3 assertion approval is retained in the 002 checkpoint and grants no action in this administrative task.

### Cap-state summary

- Cap `003/planning`: failing rounds 0, incompletes 0 (before the first spec review).
- The 002 planning history is recorded in `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_002_checkpoint.md`; shared failing planning rounds `2` and historical consecutive same-scope/round incompletes `2` belong to 002, not 003. P206 approved the checklist and resolved P205's F5 and F6; A004 used all four calls and A005 used both calls, with no retries. A006 resolved the T1 scope question. T1 is checked: Design risk APPROVED in round 2 at `67fef1d41d9ac87ebddafe10f4db5943ad9c8ce2`, and Standards and Spec APPROVED WITH NITS in round 3 at `7b50c9651bde3395bb786aff3cb10b7247351380`; no T1 reviews remain pending. T2 and T3 remain unchecked. 003 remains blocked only on 002 completion.

### Administrative initialization

- No 003 spec, checklist, task, or review has ever been launched. No 003 implementation range exists.
- At administrative creation, HEAD `f8f35709bf0baf257d3f8ba30d01f9048a1a19fc` was the creation source only, not an implementation review base.
- Retained prerequisite evidence, not rerun here: 001 complete; B008 failed, B009 passed, E009 preflight passed, and Y012 sync passed at `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676` without a HEAD change. See the 002 checkpoint bootstrap evidence.
- At initialization, administrative documents were uncommitted; no stash or untracked-file objects were created. No source or test edits were made during initialization.
- No external probes were authorized or run.

### Dependency parked after A004 P205 result

- Gate: Dependency parked after A004 P205 result
- Commands/results:
003 still needs 002. P205 002 checklist round 2 failed with two blocking findings after the only authorized four-call batch; no retries. See 002 checkpoint and parking record. 003 stays unchecked; no phase, spec, checklist, test or implementation launched. Continue only after the dependency completes under separately resolved review direction.
- HEAD commit SHA: `9bbbb36747c199ab6f95aa5c20e5f04d982a3b6c`
- Relevant work: `committed` — stash ref `b158227de1936e10f9699017fbaf7b2552d22bbb` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_002_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Keep 003 parked until 002 passes its completion gate.

### dependency parked

- Gate: dependency parked
- Commands/results:
003 parked: blocked by 002. 002 has unchecked T2/T3 after three complete round-1 task review failures and a security publication hold; see local 002 checkpoint and parking file. No 003 spec, checklist, task, review, implementation range or cap consumption. Draft PR #108 remains at d18abf7a2bd50cd9f99112243482ba1b10974d4c; no publication in this restart.
- HEAD commit SHA: `9fe4129510d4078e5aabfcd6f79bb68aed7cd05f`
- Relevant work: `committed` — stash ref `85b494b0e8c254c9e5f38956a2f01a75201884a9` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_002_checkpoint.md
 M docs/plans/053_corrected_caching_parked.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Wait for 002 security and scope decisions and actual milestone completion before starting 003.

### 003 setup and spec draft

- Gate: 003 setup and spec draft
- Commands/results:
Outer loop after 002 completion at 93627c1de9a4bbd55715e0651aa3f3aa5121f339 (milestones.md 002 [x]). Validated 002 checkpoint: phase complete, T1-T3 checked, milestone checked, matches milestones.md and implement.md checkboxes. Milestone sync: last run at 608e0730f988f750e1b3e7e9f5fe8474c4c0f323 (exit 0, behind 0); HEAD since moved only by the 002 completion bookkeeping commit; sync rerun before 003's completion gate. 003 needs 001 and 002, both checked; 003 is the only unchecked milestone. Lane full: verbose stderr default content changes; COMPATIBILITY.md 'Behavior changes are opt-in' makes the external-contract fast-lane condition doubtful. Context grounded: run.go:629-637 and :779-787 print Cache only with positive budget; stream drain run.go:884-970; Nested prints no token/cache counts (tool.go:340-376); no test asserts verbose Cache line. Spec drafted at docs/plans/053_corrected_caching_003_spec.md; spec-lint --root . exit 0, 9 citations, 0 findings. Decided for you (UX gap filled from the milestone's 'without inventing zero-count or status UX'): budget-free line shows only nonzero counts using the existing 'Cache:' label, e.g. 'Cache:    120 read'; budget path unchanged. IN FLIGHT: spec planning review round 1.
- HEAD commit SHA: `93627c1de9a4bbd55715e0651aa3f3aa5121f339`
- Relevant work: `uncommitted` — stash ref `f0a3bb18b204116804abece672bfe89c4d8b4502` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Dispatch 003 spec planning review round 1.

### 003 spec review round 1 outcome and round 2 intent

- Gate: 003 spec review round 1 outcome and round 2 intent
- Commands/results:
Spec review round 1: CHANGES REQUESTED. F1 blocker: no gate-results requirement (milestone acceptance 'applicable gate results or explicit reasons for omissions'; Verification policy gates). Nits: F2 OpenAI stream/tool-loop rows missing; F3 usage-absent path unspecified (normal and stream, final usage event missing); F4 Verbose:false test needs nonzero fixtures, stream-error test needs Verbose:true and no Tokens:/Cache:; F5 docs-lint invocation and mdbook build docs/ unspecified, compatibility rationale must cite 'Default runtime behavior' and 'Behavior changes are opt-in'. Optional: probe results must not include secrets. Fixes: all applied (new R4 gate results; table rows; R2 test strengthening; R3 docs-lint invocation and mdbook; compatibility bullet; probe secrets). spec-lint --root . exit 0, 10 citations, 0 findings. mdbook not installed locally (which mdbook: not found). Cap 003/planning: failing rounds 1, incompletes 0. IN FLIGHT: spec review round 2.
- HEAD commit SHA: `93627c1de9a4bbd55715e0651aa3f3aa5121f339`
- Relevant work: `uncommitted` — stash ref `49af328ef65df194fe3d532227c89986faabc0af` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Dispatch 003 spec planning review round 2.

### 003 spec approved

- Gate: 003 spec approved
- Commands/results:
Spec review round 2: APPROVED WITH NITS; F1-F5 RESOLVED. Nit N1 verbatim-in-substance: positive-budget test must use a nonzero cache count and assert budget used equals input+output only (budget: used/max and --json budget_used_tokens). Applied N1. spec-lint --root . exit 0. Spec approved. Cap 003/planning: failing rounds 1, incompletes 0.
- HEAD commit SHA: `93627c1de9a4bbd55715e0651aa3f3aa5121f339`
- Relevant work: `uncommitted` — stash ref `ead6bb82e94888397e93dcc7ca0754fb2b1235ab` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Write the 003 implementation checklist and run its planning review.

### 003 checklist draft and review intent

- Gate: 003 checklist draft and review intent
- Commands/results:
Checklist drafted at docs/plans/053_corrected_caching_003_implement.md (T1 standard R1/R2 in pkg/runner; T2 Markdown R3). First spec-lint run exit 1: '-run ^TestRun_VerboseCacheCounts$ selects no test in current files' (new test not yet written); reworded to construct the focused command at RED dispatch, as in the 002 checklist. Rerun spec-lint --root . exit 0, 1 -run pattern, 0 findings. Repository checklist-test-gate-audit script absent (.agents/scripts has only check-dry-skill, ci-check, release-check). IN FLIGHT: checklist planning review round 1 (shared 003 planning failing rounds 1 so far).
- HEAD commit SHA: `93627c1de9a4bbd55715e0651aa3f3aa5121f339`
- Relevant work: `uncommitted` — stash ref `31dc3a9722fd686dd567a6138f317c8f55f42cce` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Dispatch 003 checklist planning review round 1.

### 003 checklist approved

- Gate: 003 checklist approved
- Commands/results:
Checklist planning review round 1: APPROVED WITH NITS. Nits: F1 invalid sed probe syntax (use sed -n '629,637p'); F2 state sequential test, no t.Parallel, Bedrock row owns its parity fixture; F3 stream-error row must use Stream:true JSON:false with message_start, text_delta, event: error and assert partial stdout; F4 say stream rows set Options.Stream and tool rows reuse list_directory and TestRun_CacheUsageBoundaries shapes; F5 name docs-lint base 93627c1de9a4bbd55715e0651aa3f3aa5121f339 and resolve T2 'mechanical' label vs planned reviews. All applied; T2 relabelled standard (stricter: Standards and Spec reviews kept). spec-lint first rerun flagged stale-citation TestRun_CacheUsageBoundaries not in run_test.go:263-309; corrected citation to 250-309; spec-lint exit 0. Planning gate passed. Final 003/planning cap: failing rounds 1 (spec r1), incompletes 0.
- HEAD commit SHA: `93627c1de9a4bbd55715e0651aa3f3aa5121f339`
- Relevant work: `uncommitted` — stash ref `2b5579f0e00fbefd135dc9a7adf489d83e354796` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Implement T1

### T1 RED, GREEN and per-task gate

- Gate: T1 RED, GREEN and per-task gate
- Commands/results:
Fixed point 3552e6e85b8ec99c9881bcb6eea71785166938ff. RED: new pkg/runner/verbose_cache_test.go TestRun_VerboseCacheCounts (blob f9160777fc04ba57a8cc0a29f3ce4a13f34d5833). Focused command: go test -count=1 ./pkg/runner -run '^TestRun_VerboseCacheCounts$' -> FAIL; exactly 9 rows fail, each 'want consecutive "Tokens:... Cache:... Stop:"' with stderr showing Tokens then Stop and no Cache line: anthropic_single_normal, anthropic_single_stream, anthropic_loop_normal, anthropic_loop_stream, openai_single_normal_reads_only, openai_single_stream_reads_only, openai_loop_reads_only, anthropic_write_only, bedrock_single_read_and_write. Passing pre-fix as planned: zero_normal, zero_stream, absent_normal, absent_stream_without_usage_chunk, budget_line_unchanged_with_zero, not_verbose, stream_error_after_partial_output, nested_counts_stay_opaque. go vet ok. FIX: pkg/runner/run.go writeReportedCacheCounts (nonzero-only 'Cache:    N read[, M written]') called in both budget-free verbose branches; budget branches unchanged. GREEN: same focused command ok; test blob unchanged f9160777fc04ba57a8cc0a29f3ce4a13f34d5833; unchanged go test -count=1 ./pkg/runner -run '^(TestRun_CacheUsageBoundaries|TestResultMarshalJSON_CacheTokens(OmittedWhenZero)?)$' ok. Per-task gate: go test -race ./pkg/runner ok; gofmt clean; golangci-lint ./pkg/runner/... 0 issues.
- HEAD commit SHA: `3552e6e85b8ec99c9881bcb6eea71785166938ff`
- Relevant work: `uncommitted` — stash ref `44929bca43b2cae9db34fe544dea91a4845d9228` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M pkg/runner/run.go
?? pkg/runner/verbose_cache_test.go
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
f9160777fc04ba57a8cc0a29f3ce4a13f34d5833  "pkg/runner/verbose_cache_test.go"
```

- Next action: Commit T1, run whole-suite gate, dispatch Standards and Spec reviews.

### T1 gate and review intent

- Gate: T1 gate and review intent
- Commands/results:
T1 commit d2a5066b8e60f5ddbdb4033ad0f97d118a3e656c. factory-gate test gate-test-d2a5066b8e60-20261125T183508Z: see poll above in this entry's command output; log /tmp/factory-gate-1001/gate-test-d2a5066b8e60-20261125T183508Z.log. IN FLIGHT: T1 round 1 Standards and Spec reviews (standard risk), range 3552e6e85b8ec99c9881bcb6eea71785166938ff..d2a5066b8e60f5ddbdb4033ad0f97d118a3e656c. Caps T1: Standards 0, Spec 0 failing; 0 incompletes.
- HEAD commit SHA: `d2a5066b8e60f5ddbdb4033ad0f97d118a3e656c`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Record T1 Standards and Spec review verdicts.

### T1 reviews passed; nits batched

- Gate: T1 reviews passed; nits batched
- Commands/results:
T1 round 1 at 3552e6e85b8ec99c9881bcb6eea71785166938ff..d2a5066b8e60f5ddbdb4033ad0f97d118a3e656c: Standards APPROVED WITH NITS; Spec APPROVED WITH NITS (reviewer reproduced RED by reverting run.go: exactly the 9 recorded rows failed). Nits: Standards N1 negated flags noJSON/quiet; N2 indented SSE literals stripped at runtime; N3 opaque vc prefix; N4 one helper carries three modes (optional); N5 duplicated JSON decode and env setup; N6 Cache: label literal in three places (acceptable, budget branches must stay byte-identical). Spec S1 error row does not assert nil Result; S2 no tool-loop positive-budget row. Batched: N1 (jsonOff/verboseOff), N2 (unindented literals, no ReplaceAll), N3 (verboseCache prefix), N5 (verboseCacheServer, verboseCacheDecode), S1 (result nil on error), S2 (budget_loop_line_unchanged row). Not adopted: N4 (reviewer: readable at current size), N6 (would change byte-identical budget lines). Verification: gofmt, go vet ok; go test -count=1 -v ./pkg/runner -run '^TestRun_VerboseCacheCounts$' 18 PASS; go test -race ./pkg/runner ok; golangci-lint ./pkg/runner/... 0 issues. Final T1 caps: Standards 0, Spec 0 failing; 0 incompletes. T1 checked.
- HEAD commit SHA: `d2a5066b8e60f5ddbdb4033ad0f97d118a3e656c`
- Relevant work: `uncommitted` — stash ref `b10397c2fa334088768975e92ce4c118e95c8c8b` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_003_checkpoint.md
 M pkg/runner/verbose_cache_test.go
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: T2

## Limitations & blockers

- Limitations: no 003 planning, implementation, review, or verification launched.
- Blockers: milestone 002 remains unchecked; its security and scope decisions were made and T1–T3 are checked, so 003 waits only on 002's final milestone review and completion gate.
- Next action: T2
