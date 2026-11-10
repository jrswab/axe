# Milestone Checkpoint

The durable recovery record and sole home of run state for the current milestone. Another agent must be able to restart or take over from this file alone.

## Identity

- Milestone ID: `010`
- Milestone source: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 010 at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:252-263`)
- Repository root: `/home/jaron/dev/axe/runner-tool-hooks`
- Branch: `runner-tool-hooks`
- Milestone review base: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- Baseline / pre-existing changes: `git status --porcelain` at setup was empty (clean tree at `8a351c8`). Setup evidence is recorded under Gate evidence.

## Artifacts

Absolute paths inside the Repository root (the feature worktree).

- Spec: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md`
- Implementation checklist: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md` (not yet created)
- Checkpoint: `/home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md`

## State

- Lane: `full` — the fix changes the value an external MCP server receives (a JSON string becomes a JSON integer for integers of 1e6 and above) and the replay text sent to the model, across about 17 decode paths in 8 files, with test-harness gaps. The fast-lane conditions "external contracts untouched" and "small bounded behavior" both fail.
- Current phase: `spec`
- Current task: none
- Task states — checklist checkboxes render from this list:
none
- Milestone checkbox: `unchecked`
- Next safe action: Restart validation, then clean-brief planning review round 2 of the approved 010 specification; create the checklist only after its criterion passes
- Last completed phase/task/gate and timestamp: 010 approved scope reconciliation and lint PASS; handoff interruption, 2026-11-10T14:06:07.848572+00:00

## Gate evidence

One entry per gate or cap decision (review, lint, verification, completion), including failures.

Planning-cap evidence stays separate: count failing planning rounds, list unresolved findings, and record post-cap fixes and their `explore` check.

### 010 setup (phase: setup)

- Gate: milestone-entries
- Command(s) and actual result(s): `milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` exit 0, `PASS: docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md: 13 milestone entries`.
- `HEAD` commit SHA: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- `git status --porcelain`: empty before writing files.
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- Untracked file Git objects: none.

### 010 setup — factory-sync

- Gate: factory-sync `origin/master`
- Command(s) and actual result(s):
  - `~/.agents/scripts/factory-sync` (with fetch): exit 1. JSON `"error": "git fetch --quiet origin failed: Host key verification failed. fatal: Could not read from remote repository."` This is an SSH credential limit in this sandbox. Under the addendum it is reported as `sandbox credential limit: orchestrator must use factory-gh`.
  - `~/.agents/scripts/factory-sync --no-fetch`: exit 0. JSON: `base origin/master`, `ahead 136`, `behind 0`, `shared_files []`, `conflicts []`, `synced true`, `head_before 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`. This compares against the locally stored `origin/master` (`538632ea56da1c71f8f319076f221e4298148d3e`), not a fresh fetch.
- Relevant work: `committed` — stash ref `none`

### 010 setup — go-toolchain-check

- Gate: go-toolchain-check
- Command(s) and actual result(s): `~/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks` exit 0. `go 1.26.0; toolchain go1.26.10`. PASS: release go1.26.0 installed or downloadable; PASS: release go1.26.10 installed or downloadable; PASS: repository go version (`go1.26.10`); PASS: module loads read-only (`github.com/jrswab/axe`); PASS: govulncheck `./...` ("No vulnerabilities found."). Final line: `PASS: Go pre-flight checks.`
- `HEAD` commit SHA: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- Relevant work: `committed`

### 010 spec drafting (phase: spec)

- Gate: spec-lint
- Command(s) and actual result(s):
  - First run exit 1, 8 findings: 5 stale-citation (symbol not on the cited line: `gemini.go:530`, `ollama.go:479`, `opencode.go:1114`, `mcpclient.go:126-128`, and the duplicate `ollama.go:479` in the stream line), 2 missing-file (SDK paths cited under the module cache), 1 `-run` listing finding. Fixed by correcting line numbers to the symbol's line, citing the SDK as external module source, and removing the `-run` selector from the acceptance command (the whole-package run covers the three tests).
  - Final run `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md`: exit 0, `PASS: ...010_spec.md: 95 citations, 0/0 gocyclo values checked, 0 findings`.
- Gate: docs-lint
- Command(s) and actual result(s): `~/.agents/scripts/docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec /home/jaron/dev/axe/runner-tool-hooks` exit 0, `PASS: /home/jaron/dev/axe/runner-tool-hooks: 203 Markdown files, 0 findings, 0 outside --since scope`.
- `HEAD` commit SHA: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- `git status --porcelain` (before this checkpoint was written):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
```

- Relevant work: `uncommitted` — stash ref `none`; untracked spec and checkpoint only. No commit made.
- Untracked file Git objects: none (not stored; never applied).
- Next action: Planning review of 010 spec. Blocker B1 (fetch-based factory-sync) is for the orchestrator.

### 010 setup and spec lint PASS

- Gate: 010 setup and spec lint PASS
- Commands/results:
Factory setup: milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md exit 0, 13 entries. Worktree /home/jaron/dev/axe/runner-tool-hooks, branch runner-tool-hooks, milestone review base 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec, clean baseline. Orchestrator factory-sync exit 0: origin/master, head_before 8a351c8, ahead 136, behind 0, shared_files [], conflicts [], synced true. The agent's sandbox fetch failure was environmental and is superseded by this host-side successful sync. go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks exit 0, go1.26.0/go1.26.10 available, module loads, govulncheck clean. spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit 0, 95 citations, 0 findings. docs-lint --since 8a351c8 exit 0: docs-lint 1: checks task-id, stale-milestone, broken-mem-link, stale-citation; 204 Markdown files, 0 findings, 0 outside scope. Full lane: external MCP value types and integer data integrity change.
- HEAD commit SHA: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
178721e298704594bbfc118b718f37c87cb3aa51  "docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md"
bc15ec11c7b40b494f6db3998f7e461777c456b4  "docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md"
```

- Next action: Planning review of 010 spec

### 010 specification review round 1 CHANGES REQUESTED; scope decision pending

- Gate: 010 specification review round 1 CHANGES REQUESTED; scope decision pending
- Commands/results:
Planning review round 1 CHANGES REQUESTED (first failing planning round, 1 of 3). F1 pending: milestone entry says integer-valued JSON numbers, but the draft narrowed coverage to plain digit literals without user approval. Jaron must choose whether JSON numbers 1e6 and 1000000.0 normalize exactly to 1000000 or remain unchanged. Recommendation: include mathematically integral decimal/exponent numbers with bounded exact normalization, without changing fractional values, string inputs, nested collection formatting, or existing overflow failures/fallbacks. F2 fixed: nested collections retain old float64-derived formatting. F3 fixed: overflow error categories/fallbacks preserved, including nested 1e400 and 400-digit literal tests. F4 fixed: returned message history requirements added. F5 fixed: real-provider stream Arguments/history checks assigned to pkg/runner. F6 fixed: active sandbox sync blocker resolved via host factory-sync exit 0 at 8a351c8, origin/master ahead136 behind0 shared_files[] conflicts[]. Listed round-1 nits were fixed, no re-review yet. Current spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit 0, 108 citations, 0 findings. docs-lint --since 8a351c8 exit 0, docs-lint 1, 204 Markdown files, 0 findings. Independent explore fixed-string greps for six quoted replaced passages: zero active or historical hits. HEAD remains 8a351c8; git status shows only the two untracked 010 planning files, no source changes. Recovery-evidence verification agent returned no result; /tmp/052-restart-evidence.txt does not exist, so its toolchain/hash checks are unverified and must not be claimed. 009 current spec/checklist spec-lint rerun exit0 (100/101 citations,0 findings), but historical missing untracked inventories remain a recovery-evidence limitation. Next scope-approved work must repeat the smallest missing recovery checks before implementation.
- HEAD commit SHA: `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`
- Relevant work: `uncommitted` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
ddf688334f77a5379a1f81e9a9ddba2baa5c882c  "docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md"
502c4d8f554219eab15b0558fc06ba8e59e4927e  "docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md"
```

- Next action: Jaron decides whether 010 includes integral decimal/exponent JSON numbers (1e6 and 1000000.0); then reconcile the specification and run planning review round 2

### 010 scope decision approved and resume checks PASS

- Gate: 010 scope decision approved and resume checks PASS
- Commands/results:
Jaron answered "q1 = yes". Approved scope: include mathematically integral decimal/exponent JSON numbers such as 1e6 and 1000000.0, with bounded exact conversion to plain integer digits for absolute values >=1000000. Fractional values, JSON string inputs, nested collection formatting and current float64-overflow failures/fallbacks remain unchanged. The decimal/exponent blocker B2/F1 is resolved; the spec and canonical milestone entry must record this approval before planning review round2. No source/checklist implementation authorized outside milestone010. Resume evidence: milestone-entries exit0 (13 entries), repo-state clean HEADdcba6db branchrunner-tool-hooks; host factory-sync exit0 origin/master ahead137 behind0 shared_files[] conflicts[]. go-toolchain-check root exit0, go1.26.0/toolchain go1.26.10 available, module loads, govulncheck clean, GOTOOLCHAIN unset and existing PATH retained. Old 009 /tmp test logs are absent, and historical missing inventories cannot be reconstructed; fresh replacement test gate at dcba6db poll0 log /tmp/factory-gate-1001/gate-test-dcba6db0d0d1-20261110T135744Z.log and current 009 spec/checklist lint100/101citations,0findings were recorded in 009 checkpoint commit2eb0ed5. 010 latest inventory spec hash502c4d8f554219eab15b0558fc06ba8e59e4927e matches committed draft; four recorded historical inventory objects exist, checkpoint self-writes and review changes accounted separately, no stash apply. Planning failing roundsremain1of3. Next update approved artifacts, lint, round2 planning review.
- HEAD commit SHA: `2eb0ed5950afe862454f739f0e4c8525fa4259a6`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Revise 010 specification and milestone wording for the approved exact integral decimal/exponent behavior; then lint and planning review round 2

### 010 approved scope reconciliation and lint PASS; handoff interruption

- Gate: 010 approved scope reconciliation and lint PASS; handoff interruption
- Commands/results:
Jaron requested handoff before planning review round2. User decision "q1 = yes" is recorded in the specification, canonical milestone entry010/D12/large-integer scope assumption and current checkpoint approval note. Approved top-level mathematical integer JSON numbers with |value|>=1000000 normalize exactly; 1e6 and1000000.0→1000000, -1e6→-1000000; Float64 must not decide integrality or threshold. |value|<1000000, signedzero, fractions, JSONstrings, nested collection formatting and current magnitude-overflow failures/fallbacks remain unchanged. Work/memory bounded by input size plus finite result, not exponent magnitude. Int64 overflow preserves digitstring fallback; numberparams preserve numerical Float64value, not identical Arguments text. Real provider stream→drain Arguments/history, raw decoded→replay and raw decoded→MCPwire tests remain required. Current verification (HEAD2eb0ed5950afe862454f739f0e4c8525fa4259a6, only three Markdown artifacts modified): spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit0,109citations,0findings. docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit0, docs-lint1 checks task-id/stale-milestone/broken-mem-link/stale-citation;204Markdownfiles,0findings,0outside-scope. Independent explore git diff --check exit0, only _010_spec.md/_010_checkpoint.md/_milestones.md;no checkbox/source changes. Fixed-string greps for await B2, B2 determines, Neither alternative is approved, subject to the unresolved decimal/exponent scope decision, pending Jaron: zerohits. Historical B1/B2 pending records preserved as dated history, superseded by host sync and scope approval. Planning failing roundsremain1of3. This edit/lint is not a passing review; round2 not run, no _010_implement.md, no code implemented. Recovery preflight and fresh go test -race ./... at dcba6db PASS already recorded in 009/010 checkpoint; old missing logs/inventories remain historical limitations, not fabricated recovered evidence. External interruption is the terminal condition for this session.
- HEAD commit SHA: `2eb0ed5950afe862454f739f0e4c8525fa4259a6`
- Relevant work: `uncommitted` — stash ref `b74cda2ab48d0a6b61e2300ef679e67323aee506` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Restart validation, then clean-brief planning review round 2 of the approved 010 specification; create the checklist only after its criterion passes

## Limitations & blockers

- Limitations:
  - Sandbox fetch failure remains historical evidence only; authoritative HOST sync supersedes it (B1 resolved below).
  - B2/F1 scope is approved: integral decimal/exponent numeric forms normalize exactly at |value| >=1000000. Finite beyond-int64 values keep exact digit strings on integer MCP ParseInt failure (spec E5).
  - Number-typed parameters keep `float64` precision (spec E6).
  - Bedrock has no stream path (spec E7).
  - `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`.
  - The rejected design note retains its historical stale streaming citation; canonical entry 010 is reconciled with corrected runner anchors. See the spec's Citation corrections.
  - No `go test` run during spec drafting. The spec pass made no code changes.
- Blockers:
  - B1 resolved: authoritative HOST `run_script factory-sync` at HEAD `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec` exited 0, base `origin/master`, ahead 136, behind 0, shared_files [], conflicts [], synced true; recorded in `010 setup and spec lint PASS`.
  - B2 / F1 resolved: Jaron's 2026-11-10 "q1 = yes" approves exact bounded normalization of top-level integral decimal/exponent JSON numbers with |value| >=1000000. Spec and canonical milestone now record the approval; no active approval or origin-sync blocker remains. Planning review round 2 is still required; no checklist or implementation starts in this pass.
- Next action: Restart validation, then clean-brief planning review round 2 of the approved 010 specification; create the checklist only after its criterion passes

## Approval / decision note — 2026-11-10

- Approval source: Jaron answered "q1 = yes". B2/F1 resolved; B1 already resolved by authoritative host sync. The prior pending/error entries below and under Gate evidence remain historical, not active blockers.
- Approved behavior: exact signed top-level numeric JSON integers with |value| >=1000000 retain original plain digit text or exact normalized digits for integral decimal/exponent forms. `1e6`, `1000000.0` → `1000000`; `-1e6` → `-1000000`; `9007199254740993` and equivalent integral spellings never round through Float64. These digits reach Arguments, histories, replay, hook/observer and detail inputs. Replay remains map[string]string with no type inference.
- Unchanged boundaries: smaller magnitudes, zeros including negative zero, fractions (including `1000000.00000000001`), all JSON strings, recursively Float64-derived nested formatting, malformed/trailing/overflow categories and fallbacks. Float64 validates range, not integrality; no +Inf or exponent-sized allocation. Work/memory is input length plus bounded finite integer output (~309 digits); huge negative exponents and zero exponents retain legacy underflow/signed-zero behavior.
- MCP integer parameters preserve digits within int64; finite beyond-int64 integers remain exact digit strings. MCP number parameters preserve the resulting Float64 numerical value, not identical Arguments text, with signed-zero/rounding/>2^53 verification. No production MCP change unless a declared test exposes an in-scope failure. Internal provider helpers may be exported for runner reuse; no pkg/runner exported API/type/identifier or internal/testutil production change.
- Scope remains milestone 010 only. Milestone 012 owns CHANGELOG/README/version disclosure. Phase remains spec; all milestone/task checkboxes and script-managed states are unchanged. First failing planning round remains **1 of 3**; this document reconciliation is not a passing review. Existing F2-F6 fixes and every listed round-1 nit remain required. No implementation/checklist, source changes, shared lint/bookkeeping, commit or network operation performed. Orchestrator runs lint/bookkeeping and planning review next.

## Decisions & review history — round 1

- First failing planning review round: **1 of 3**. This correction pass is not a re-review or a gate result. State fields and task states above are left for factory-checkpoint.
- F1: mechanically removed false milestone attribution and approved/settled literal-only language; decimal/exponent scope remains pending B2.
- F2: mechanically fixed recursive conversion of ALL nested numbers to old float64 values before collection formatting; exact nested formatting/examples remain unchanged.
- F3: mechanically fixed recursive magnitude-overflow validation (`1e400`, 400-digit integers, including nested values); preserve server `ProviderError` versus empty-args fallback, not exact wording; prohibit `+Inf` in `Arguments`.
- F4: mechanically added explicit returned-history requirements/tests for both integers, every real buffered/available stream decode path, and non-empty pre-built histories.
- F5: mechanically located drained stream-argument/history tests in `pkg/runner` using real providers; Gemini/Ollama emitter checks remain useful; text-delta-only checks are not claimed to fail today.
- F6: mechanically resolved active B1 using the authoritative host sync; sandbox failure retained as historical evidence.
- Round-1 nits mechanically fixed: seven provider source files plus new formatter file; error category/fallback wording; trailing argument-string/chunk/drain cases; raw numeric decode before replay; end-to-end runner raw numeric provider response to raw MCP handler bytes; intermediate `ToolInput` changes versus unchanged unaffected drained arguments; milestone 012 disclosure of corrected detail/history/hook/observer digits; CHANGELOG Added citation range corrected to 10-18.
- No code changed; Go build/test/lint are not applicable to this document-only correction. No shared lint/bookkeeping scripts, checklist, implementation, task checkbox updates, commits, or network/repository mutations run. Remaining mechanically unresolved findings: none identified in this correction pass; F1's user decision remains open.
