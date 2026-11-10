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
- Current phase: `implement`
- Current task: 010 interrupted before post-cap checklist fixes
- Task states — checklist checkboxes render from this list:
- T01a: `unchecked`
- T01b: `unchecked`
- T01c: `unchecked`
- T02: `unchecked`
- T03: `unchecked`
- T04: `unchecked`
- T05: `unchecked`
- T06: `unchecked`
- T07: `unchecked`
- T08: `unchecked`
- T09: `unchecked`
- T10: `unchecked`
- T11: `unchecked`
- T12: `unchecked`
- T13: `unchecked`
- Milestone checkbox: `unchecked`
- Next safe action: Restart validation, then exact post-cap F1/F2 checklist fixes and one explore check before execution
- Last completed phase/task/gate and timestamp: 010 user-requested interruption; durable planning lint PASS, post-cap fixes pending, 2026-11-10T16:12:20.731899+00:00

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

### 010 restart validation, sync, Go preflight and planning lint PASS

- Gate: 010 restart validation, sync, Go preflight and planning lint PASS
- Commands/results:
Current invocation resumes interrupted Factory run. Repository /home/jaron/dev/axe/runner-tool-hooks; branch runner-tool-hooks; baseline HEAD c307b92ebdd7d5f4e221396dc48cadc83249e347; baseline git status --porcelain empty. milestone-entries docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md exit 0: 13 entries, 010 first unchecked; no checklist exists. Host factory-sync --base origin/master exit 0: branch runner-tool-hooks, base origin/master, head_before c307b92ebdd7d5f4e221396dc48cadc83249e347, ahead 139, behind 0, shared_files [], conflicts [], synced true; HEAD unchanged. env -u ANTHROPIC_API_KEY -u AXE_ANTHROPIC_BASE_URL /home/jaron/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/runner-tool-hooks exit 0: go1.26.0 and toolchain go1.26.10 available; repository Go go1.26.10; module loads read-only github.com/jrswab/axe; govulncheck ./... No vulnerabilities found; PASS Go pre-flight. GOTOOLCHAIN unset. PATH=/home/jaron/.pi/agent/bin:/home/jaron/.cache/pi-go-bin:/home/jaron/.local/share/../bin:/home/jaron/bin:/home/jaron/.local/bin:/home/jaron/.npm-global/bin:/home/jaron/.opencode/bin:/home/jaron/.local/share/pi-node/current/bin:/home/jaron/android-sdk/cmdline-tools/bin:/home/jaron/go/bin:/usr/local/go/bin:/home/jaron/.cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin. Historical stash b74cda2ab48d0a6b61e2300ef679e67323aee506 inspected with git show/stash show, never applied. Its spec and milestones match c307b92; only checkpoint changed afterward (27 additions/4 deletions), accounted by final gate/state append and c307b92. Four recorded 010 inventory blobs exist. Current git hash-object without -w matches HEAD artifact blobs: checkpoint 839f15230e9fbb5b4c8c3ca755fccd88a353f3ce, spec 56b81d4351390f7416254572897568188acd9460. Earlier omitted inventories and original 009 gate logs remain unavailable historical evidence, not reconstructed. Existing replacement 009 log at dcba6db exists but is not a gate at current HEAD. git diff --check and git diff --exit-code -- go.mod go.sum exit 0, no output. No source changes. spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit 0: 109 citations, 0 findings. docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit 0; header docs-lint 1; 204 Markdown files, 0 findings, 0 outside scope. Planning failed rounds remain 1/3. Jaron-approved scope is recorded in milestone/spec/checkpoint and needs no new decision. Verbatim prior round-1 findings are available in the user-supplied /tmp handoff and will accompany the clean planning-review brief. No one-way door authorized.
- HEAD commit SHA: `c307b92ebdd7d5f4e221396dc48cadc83249e347`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Run clean-brief 010 specification planning review round 2

### 010 specification planning review round 2 partial CHANGES REQUESTED

- Gate: 010 specification planning review round 2 partial CHANGES REQUESTED
- Commands/results:
Review returned Verdict: CHANGES REQUESTED and Review incomplete: docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md — R1 extreme-number boundary analysis and final acceptance reconciliation remain unfinished. This partial result contains a blocker, so planning failing rounds are now 2/3 total (round 1 and round 2; same-round completion retry must not count another round). Incomplete results for the original spec round-2 scope: 1 consecutive. Completed read: full spec/milestone/relevant guidance, seven provider source files, inherited provider paths, runner and MCP seams. Prior F1/F2/F3/F4/F6 verified corrected. Established blocker verbatim: Gemini/Ollama raw-emitter tests are not explicitly required. Spec :104 delegates to R7 stream cases; :120 says provider ToolInput checks remain useful only for Gemini/Ollama emitter regressions; neither makes them mandatory, but milestone :263 requires them. Reviewer fix: require provider-level Gemini and Ollama tests asserting unquoted numeric ToolInput tokens, including 9007199254740993, and native boolean/null/collection shapes. Retain separate real-provider runner drain/history tests. Nit: spec :121 use Run for every buffered path cannot mock Bedrock via public Run; permit run(..., newProvider) injecting real provider.NewBedrock with WithBedrockBaseURL(httptestURL) and fake credentials, not a capturing-provider substitute. Nit: spec :135 Only the files in the Parallelizable work list points to work units; replace with Only the following paths may be edited. Unfinished checks: R1 long-coefficient/exponent-cancellation against bounded Float64 parser; R2/R3 earlier overflow overwritten by duplicate keys; final complete acceptance reconciliation. No tests, lint, builds, network operations or writes ran in review. No checklist or implementation authorized until specification criterion passes. Next review is a same-round retry of unreviewed portions, keeping all established findings.
- HEAD commit SHA: `c307b92ebdd7d5f4e221396dc48cadc83249e347`
- Relevant work: `committed` — stash ref `be25f07336669cd72b572897cc409b355dc3fb29` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Rerun planning lint and complete unreviewed 010 specification analysis at round 2

### 010 planning lint before round 2 remaining review PASS

- Gate: 010 planning lint before round 2 remaining review PASS
- Commands/results:
spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit 0: 109 citations, 0 findings. docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit 0: docs-lint 1; 204 Markdown files, 0 findings, 0 outside scope. Only checkpoint bookkeeping changed since restart. Same round-2 retry covers unfinished extreme-number analysis and acceptance reconciliation, keeps all partial findings; failing rounds remain 2/3.
- HEAD commit SHA: `c307b92ebdd7d5f4e221396dc48cadc83249e347`
- Relevant work: `committed` — stash ref `82d95a0d4239164fbc8b41b95a33f6a43492e19b` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Complete unreviewed 010 specification boundary/acceptance analysis at round 2

### 010 specification round 2 complete CHANGES REQUESTED; numeric blockers confirmed

- Gate: 010 specification round 2 complete CHANGES REQUESTED; numeric blockers confirmed
- Commands/results:
Complete same-round-2 remaining-scope review returned Verdict: CHANGES REQUESTED with no Review incomplete line. Combined round-2 verdict CHANGES REQUESTED, all original scope complete. Planning failing rounds remain 2/3 (rounds 1 and 2, no new round for completion retry). No checklist or implementation exists. Current BLOCKER is a user scope/observable-behavior decision: exact normalization for all approved top-level mathematical JSON integers, legacy Float64 numerical behavior, and bounded output cannot all hold for extreme valid numeric tokens.

Complete review findings (verbatim offending text and fixes):
1. Exact normalization, legacy numeric behavior and the output bound cannot all hold. Locations spec :85-87,93,125,170. Offending text: “normalize decimal/exponent forms to exact plain integer digits”; “the finite Float64 range permits approximately 309 output digits”; “MCP `number`-typed parameters preserve the resulting Float64 numerical value”. Token A = "1" + 100006 zero digits + "e-100000", exact mathematical value 1000000 but legacy Go1.26.10 ParseFloat returns 0 without error. Token B = "0." + 9999 zero digits + "1e100000", exact mathematical value 10^90000 and 90001 output digits, but legacy ParseFloat returns 1 without error. internal/mcpclient/mcpclient.go:131-133 parses the stored text, so normalized A changes a number-typed MCP value. Required resolution verbatim: Obtain an explicit decision from Jaron resolving these incompatible boundary promises, then reconcile R1–R3/R8, milestone 010 and the approval record before creating the checklist. No implementation-only fix satisfies the current wording. This review does not authorize a new token limit, changed acceptance, schema-dependent formatting or public state.
2. Final decoded-graph validation misses overflow overwritten by duplicate keys. Locations spec :93-94,99,166. Offending text: “Validate ALL decoded numbers recursively, including nested values, against the old float64 range.”; “Validate recursively before formatting or emitting”. {"n":1e400,"n":1000000} and {"n":{"bad":1e400},"n":1000000} fail legacy decoding but succeed UseNumber and final-graph validation. Fix verbatim: Require preservation of errors from every value the legacy destination decoder would convert, including values later overwritten. Explicitly allow a same-destination legacy validation pass into a fresh instance before UseNumber, or equivalent validation that retains those errors. Add top-level and replaced-subtree duplicate-overflow cases to every affected R2/R3 boundary, plus successful duplicate-key controls. Do not reject all duplicate keys or blindly validate unknown response fields: legacy decoding skips unmatched fields.
3. Required Gemini/Ollama raw-emitter tests remain optional. Locations spec :104,120 versus milestone :263. Offending text: “Verification: R7 stream cases for Gemini and Ollama.”; “Provider `ToolInput` checks remain useful only for Gemini/Ollama emitter regressions”. Fix verbatim: Explicitly require provider-level Gemini/Ollama tests asserting unquoted numeric ToolInput tokens, including 9007199254740993, and native boolean/null/collection shapes. Retain the separate real-provider runner drain/history tests.
Nit: spec :121 “use `Run`” cannot mock Bedrock's endpoint. Fix verbatim: permit the existing private run(..., newProvider) seam to inject a real provider.NewBedrock with WithBedrockBaseURL(httptestURL) and isolated fake credentials—not a capturing-provider substitute.
Nit: spec :135 “Only the files in the Parallelizable work list” points to work units. Fix verbatim: replace it with “Only the following paths may be edited.”

Independent deterministic probe: /tmp/052-numeric-boundaries/main.go, Go stdlib only, no repository writes. env -u ANTHROPIC_API_KEY -u AXE_ANTHROPIC_BASE_URL go run /tmp/052-numeric-boundaries/main.go exit 0, empty stderr. Actual stdout: A len=100015, ParseFloat/legacy JSON =0 nil error, UseNumber preserves token, Number.Float64=0 nil; ParseFloat("1000000",64)=1000000, equality false. B len=10009, ParseFloat/legacy JSON=1 nil, UseNumber preserves token, Number.Float64=1 nil; exact result length 90001 from exponent arithmetic only, no expansion. Controls 1e6/1000000.0 yield Float64 1000000 and legacy formatting 1e+06; 9007199254740993 yields rounded Float64 9007199254740992 while token remains exact; 1e400 legacy JSON UnmarshalTypeError and Number.Float64 range error. Both duplicate-overflow examples return legacy UnmarshalTypeError for map and actual-equivalent Gemini/Ollama response map destinations, but UseNumber/final-graph validation succeed. RawMessage outer destinations retain bytes but need legacy subtree validation. Successful duplicate controls and ignored unknown-response-field overflow controls pass legacy decoding. Confirmed all three numeric/error candidates. gofmt -l /tmp/052-numeric-boundaries/main.go and go vet /tmp/052-numeric-boundaries/main.go exit 0, empty output. GOTOOLCHAIN unset, existing PATH retained; vet/run GOPROXY=off. go version: go1.26.10 linux/amd64. git diff --check and git diff --exit-code -- go.mod go.sum exit 0 before/after; HEAD c307b92ebdd7d5f4e221396dc48cadc83249e347; only preexisting orchestrator checkpoint bookkeeping dirty. No repository package tests or paid/network provider calls ran. Probe is not implementation/test-suite evidence.

qa-repro dispatch could not execute due to its suite-policy conflict and missing QA skill; no evidence from that call. The code-agent stdlib probe supplied independent hard evidence instead.

Proposed safe decision, NOT yet approved or applied: normalize an otherwise eligible exact integer only if exact output remains bounded and ParseFloat of normalized digits equals the legacy Float64 value; otherwise retain legacy formatting/error. This narrows prior unconditional exact scope for A/B while retaining ordinary approved examples, integer digits beyond int64, and unchanged external APIs/dependencies. Await Jaron's choice before any spec/milestone reconciliation. No active sync/auth blocker. No push, merge or deploy authorized. 010 and 011/012 remain unchecked.
- HEAD commit SHA: `c307b92ebdd7d5f4e221396dc48cadc83249e347`
- Relevant work: `committed` — stash ref `a40b35dc2183e00089171088ddbd57b7ff877916` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Await Jaron decision on bounded compatibility fallback for exceptional numeric tokens

### 010 bounded compatibility fallback approved by Jaron

- Gate: 010 bounded compatibility fallback approved by Jaron
- Commands/results:
Jaron answered verbatim "Q1 - ye" to the orchestrator's question: "May I retain current formatting for exceptional tokens that violate those compatibility conditions?" Approved recommendation: normalize otherwise eligible exact top-level mathematical JSON integers |value| >= 1000000 only when plain output has at most 309 digits excluding sign and ParseFloat(normalized digits,64) succeeds and equals legacy ParseFloat(original token,64). Otherwise preserve legacy formatting/error behavior. This narrows earlier unconditional approval only for exceptional numeric tokens; ordinary 1e6, 1000000.0, -1e6, 9007199254740993 and int64/beyond-int64 exact digits remain included when eligible. Smaller values, signed zero, fractions, strings, nested formatting, replay strings and MCP numerical values remain unchanged. No new token rejection limit, dependencies, public APIs or RawArguments. User decision resolves the round-2 extreme-number scope blocker; specification/milestone wording must now be reconciled before re-review. Duplicate-key error preservation, mandatory Gemini/Ollama raw-emitter assertions and the two specified nits are technical fixes within scope. Planning failed rounds remain 2/3. No checklist exists. Host milestone-entries exit 0: 13 entries. Host factory-sync --base origin/master exit 0: head_before b65d89d6bb32a5b575ded581496708806586a9ac, ahead 140, behind 0, shared_files [], conflicts [], synced true, HEAD unchanged. Read-only restart check confirms clean status, no source/module changes since c307b92, only prior checkpoint commit b65d89d. Successful Go preflight at c307b92 remains applicable to unchanged module/toolchain/source; PATH unchanged and GOTOOLCHAIN unset. No one-way door authorized by this answer. Preserve historical decisions and review findings as history.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `committed` — stash ref `none` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text

```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Reconcile 010 specification and canonical milestone with approved fallback and round-2 findings

### 010 approved round-2 document fixes and planning lint PASS

- Gate: 010 approved round-2 document fixes and planning lint PASS
- Commands/results:
Approved conditional scope is now reconciled in specification and canonical milestone: at most 309 exact output digits excluding sign, successful normalized ParseFloat equal to legacy ParseFloat, otherwise legacy formatting/error. Same-destination overwritten-error and ignored-field controls, mandatory raw-emitter tests, Bedrock real-provider injection and path wording fixes are added. general changed only 010 spec and canonical milestone; no checkbox/source/API/module changes. Independent explore confirms all nine literal grep -F replacements have zero hits, all additive requirements cite valid current code, and spec/milestone scope agree. git diff --check exit 0, no output. spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md exit 0: 111 citations, 0 findings. docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit 0: docs-lint 1; 204 Markdown files, 0 findings, 0 outside scope. F1-F3 and both round-2 nits are fixed within approved scope; re-review pending. Current failing planning rounds remain 2/3, no checklist exists. The earlier undated Approval note and Limitations & blockers paragraphs describing unconditional approval, 1/3 rounds, round-2 pending and document-reconciliation next action are SUPERSEDED HISTORICAL TEXT. They do not define current run state. This gate and State are current: conditional fallback approved, round-2 complete/failing, fixes independently checked, next action clean specification planning review round 3. Preserve old history rather than falsifying old approvals/gates. Historical preflight remains applicable because no source/toolchain/module files changed. No new user decision or one-way door authorized.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `uncommitted` — stash ref `ee6db1766f2b48fee2c014ecbe9bf86ac6efca80` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Run clean-brief 010 specification planning review round 3

### 010 specification planning review round 3 APPROVED

- Gate: 010 specification planning review round 3 APPROVED
- Commands/results:
Specification planning review round 3 returned Verdict: APPROVED with no Review incomplete line and no nits. All five prior findings/fixes were verified: conditional 309-digit/equal-ParseFloat fallback approved and canonical scope reconciled; same-destination errors include overwritten numeric overflow but skip unknown fields; mandatory separate Gemini/Ollama raw native-value assertions; private real Bedrock injection; permitted-path wording. Review scope complete, no writes/tests/network. Spec-lint exit 0 at current artifact (111 citations, 0 findings) and docs-lint 1 exit 0 (204 Markdown files, 0 findings) were supplied. Independent fixed-string greps all zero hits. Planning failing rounds remain 2/3 across spec/checklist; passing round 3 does not increment this count. Approved spec exists and checklist path remains absent; phase checklist-pending with no task states. No source or API implementation yet. Historical limitations/old approval notes are superseded as stated by the previous gate. Next action load implement skill and create only the resolved 010 checklist. No one-way door authorized.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `uncommitted` — stash ref `bbf1019834ba14785c9136168b2be5eabcc4c4a2` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Load implement skill and create 010 implementation checklist from approved specification

### 010 checklist created and planning lint PASS

- Gate: 010 checklist created and planning lint PASS
- Commands/results:
Resolved checklist created at absolute path /home/jaron/dev/axe/runner-tool-hooks/docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md. 15 unique ordered IDs T01a T01b T01c T02 T03 T04 T05 T06 T07 T08 T09 T10 T11 T12 T13; all unchecked. Spec amended only mechanical acceptance command: changed/new-file formatting instead of measured unrelated baseline drift, deterministic -count=1 removed. Requirements unchanged. Phase2 extreme evidence from /tmp/052-numeric-boundaries/main.go expanded stdlib run: gofmt/vet/go run all exit0 with Go1.26.10, GOTOOLCHAIN unset, unchanged PATH, safety env keys cleared; baseline numeric, 309/310-digit, zero/fractions/A/B/trailing/duplicate/unknown-field controls recorded in checklist Section4. Existing API signatures unchanged; all new helper callsites and ToolInput readers inventoried and assigned, no implementation code yet. Initial spec-lint exit1 (unsupported env -u command prefix and ambiguous citation), fixed mechanically. Second spec-lint exit1 (future -run tests select no current tests), no marker exemption supported, now literal verbose package commands require exact new test names and intended RED assertion once tests exist. This is not a test pass. Final spec-lint --root /home/jaron/dev/axe/runner-tool-hooks 010_spec.md 010_implement.md exit0: spec111 citations, checklist16 citations, 0 findings. docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit0, docs-lint1, 205 Markdown files, 0 findings,0 outside. git diff --check exit0. Checklist planning review pending; existing planning failing rounds remain2/3. Historical approval/limitations paragraphs are superseded by current State/gates. No blockers or one-way doors authorized. No task checkbox may check before its gate.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `uncommitted` — stash ref `214df2bf2ebcfb68ae23f4352579132a8fcd90ae` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
41fbeb51955f8914885245180fd17f88887ead6c  "docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md"
```

- Next action: Run clean-brief 010 checklist planning review round 1 against complete approved specification

### 010 checklist review round 1 CHANGES REQUESTED; planning failing-round cap reached

- Gate: 010 checklist review round 1 CHANGES REQUESTED; planning failing-round cap reached
- Commands/results:
Complete checklist planning review round1 returned Verdict: CHANGES REQUESTED, no Review incomplete. Third failed planning round total for milestone010: spec round1, spec round2 (partial+complete same round counted once), checklist round1. Spec round3 APPROVED did not increment. Planning review cap reached at3/3; dispatch no further planning reviews. Preserve complete findings below, apply exactly specified fixes, spec-lint both artifacts must exit0, then one read-only explore check of every fix against cited code/spec before execution. If that check fails/incomplete, stop High stakes. Execution requires per-task behavioral RED first, intended missing-behavior failure, GREEN/full touched-package gate recorded before checkboxes; already-correct acceptance-only tasks are honest unchanged green confirmations as checklist states.
F1 blocking verbatim: Bedrock history setup omits a prerequisite before the injection seam. Location checklist :148. Quoted offending text: “History uses existing private … `run` … with newProvider callback returning REAL `provider.NewBedrock("us-east-1", provider.WithBedrockBaseURL(server.URL))`, fake isolated AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY and cleared session token”. pkg/runner/run.go:393-397 requires region before callback at413. Fix verbatim: Explicitly set t.Setenv("AWS_REGION", "us-east-1") in TestLargeIntegerArgs_Bedrock_BufferedHistory, alongside the already-required isolated credentials/session token. Keep this environment-mutating test nonparallel. No production change is needed.
F2 blocking verbatim: Required acceptance inputs are not assigned to concrete test packs. Locations checklist :19,49,224; source spec :150,168,176. Quoted offending text: “strings (`\"1e6\"`, `\"1000000.0\"`, `\"9007199254740993\"`)”; “Table includes every numeric/collection row in Section 4 below”. Fix verbatim: Add raw {"n":"text"} → stored "text" to the unaffected pack and formatter table. Assign beyond-int64 integral forms such as 99999999999999999999999.0 and 999999999999999999999990e-1 to T01b and T13. Assert the independent expected normalized digit string in Arguments and the same quoted digit string at the raw MCP handler, because ParseInt fails beyond int64.
Reviewer checked15 task IDs,16 decode sites,12 flatten/removal sites,ToolInput reader ordering and existing signatures; no other findings/nits. No tests/lints/builds/network ran in review, git diff --check exit0. No checklist/source implementation done. No new user decision needed; fixes are within already approved scope. Current superseding scope remains bounded<=309/equal legacy Float64 fallback; historical active-looking note is superseded. Next: exact post-cap fixes then lint and single explore check. No one-way door authorized.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `uncommitted` — stash ref `6ae8137e92dcedac15037be3bbd6b6f7ff386f75` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
41fbeb51955f8914885245180fd17f88887ead6c  "docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md"
```

- Next action: Apply F1/F2 checklist fixes, lint both artifacts, then single post-cap explore verification

### 010 user-requested interruption; durable planning lint PASS, post-cap fixes pending

- Gate: 010 user-requested interruption; durable planning lint PASS, post-cap fixes pending
- Commands/results:
Terminal condition: Jaron invoked handoff, an external interruption. Stop Factory work now; do not dispatch pending F1/F2 fixes or implementation during this handoff. Milestone010 remains full lane unchecked phase implement. Spec review round3 APPROVED. Checklist review round1 complete CHANGES REQUESTED. Planning failing-round cap3/3 reached; no further planning review loop. F1/F2 from previous gate remain unapplied: Bedrock history explicit t.Setenv AWS_REGION us-east-1 and nonparallel; literal text string in unaffected/formatter packs and beyond-int64 integral .0/e-1 forms in T01b/T13 with exact Arguments/quoted raw MCP digit assertions. No code or implementation tests exist. All15 checklist IDs unchecked. Next session restart validation then exact F1/F2 document fixes, lint both artifacts, one independent read-only explore verification against code/spec, then exec skill and per-task TDD if every fix verified. Failed/incomplete post-cap check stops High stakes. Approved Q1 - ye bounded compatibility fallback is recorded/canonical: <=309 digits excluding sign and equal successful ParseFloat, else legacy. No open user scope question. Existing old approval/limitations paragraphs are superseded historical text, not current state.
Pre-handoff current artifact checks via host run_script: spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md exit0: spec111 citations, checklist16 citations,0 findings. docs-lint --since8a351c8e475ab1bfdc52000f0593f1e6ed7385ec exit0: docs-lint1,205 Markdown files,0 findings,0 outside. Lint pass is not checklist review approval. HEAD b65d89d6bb32a5b575ded581496708806586a9ac; status before bookkeeping: M010_checkpoint, M010_spec, Mmilestones, ??010_implement. Only four planning artifacts changed; source/module/public API unchanged. Local preapproved commit will preserve this interrupted planning snapshot; next session accounts separately for checkpoint self-write and commit. No push, PR merge, deploy, paid-provider calls or other one-way door authorized by handoff. Full completion tests were not run for010. Historical Go preflight/probes remain observations, not completion evidence.
- HEAD commit SHA: `b65d89d6bb32a5b575ded581496708806586a9ac`
- Relevant work: `uncommitted` — stash ref `91f625d927396eb3e05e76af60f709c2e0c32e98` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md
 M docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md
?? docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text
41fbeb51955f8914885245180fd17f88887ead6c  "docs/plans/052_runner_tool_hooks_v1_12_0_010_implement.md"
```

- Next action: Restart validation, then exact post-cap F1/F2 checklist fixes and one explore check before execution

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
- Next action: Restart validation, then exact post-cap F1/F2 checklist fixes and one explore check before execution

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
