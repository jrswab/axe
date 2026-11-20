# Milestone Checkpoint

The durable recovery record for milestone 002. Setup evidence below is retained from the current session and milestone 001; it is not a claim that the creation agent reran those checks.

## Identity

- Milestone ID: `002`
- Milestone source: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_milestones.md`
- Repository root: `/home/jaron/dev/axe/iss-83-finish-caching`
- Branch: `iss-83-finish-caching`
- Baseline / pre-existing changes (`git status --porcelain` at creation):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Start / milestone review base: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676` (actual HEAD after Y012 sync).

## Artifacts

Absolute paths inside the repository root:

- Spec: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_002_spec.md` (drafted; absent at creation).
- Implementation checklist: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_002_implement.md` (not yet created; absent at creation).
- Checkpoint: `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_002_checkpoint.md`.

## State

- Lane: `full` — the external Bedrock wire contract and AWS credential/setup boundary exceed fast-lane criteria.
- Current phase: `spec`
- Current task: none
- Task states — checklist checkboxes render from this list:
none
- Milestone checkbox: `unchecked`
- Next safe action: Reconcile current summaries and verify exact three-file commit, then commit and review

The latest decision ledger and per-gate cap snapshots are in Gate evidence. Tasks, when planned, use complete checklist IDs and follow checklist order.
- Last completed phase/task/gate and timestamp: F202 repaired summaries; S202 current summary reconciliation IN FLIGHT, 2026-11-20T20:14:01.828475+00:00

## Gate evidence

### Decision ledger

- Decision Q3: Jaron's exact approval was `Q3 I approve`. It authorizes only replacing the contradicted assertions in `TestBedrock_Send_Cache` in `internal/provider/bedrock_test.go`: the combined system text/cache-point expectation becomes separate text and default cache-point-only union entries, and the top-level `ToolConfig.CachePoint` expectation becomes a separate default cache-point-only entry in `toolConfig.tools` while retaining the tool specification. Preserve ToolConfig existence, cache read/write usage, and all unrelated assertions/tests; independently cover raw-wire `inputSchema.json` and cache-disabled requests. No other assertion or acceptance change is approved. Original source: `/home/jaron/.pi/agent/sessions/--home-jaron-dev-axe-iss-83-finish-caching--/2026-11-19T15-47-50-732Z_01a1f47e-274c-748d-88b3-337965f93f71.jsonl:106`; durable scope and user message also appear in the 001 checkpoint's Q3 and REC009 entries. Action: future implementation only, not performed here; report: recorded in 001; intended gate: later implementation and applicable tests.
- Decision D009: use `GOMODCACHE=/tmp/axe-v006-cache.itoPgN/go-mod` for the existing Go preflight after B008's read-only module-cache failure. Scope: reversible local cache selection for 002 setup; reason: `XDG_CACHE_HOME` does not redirect Go module downloads. Eligibility: existing full check including vulnerability scan retained, no code/assertion/criterion change, no review or cap bypass, no human-only boundary crossed; stricter reviewer instruction: not applicable. B009 subsequently passed; B008 remains failed history. Intended gate: full existing Go preflight; action: completed; report: retained in 001 and this checkpoint. Evidence: REC009c and B009 in 001, B002-001 below.
- Other new 002 procedure exceptions: `none`. The 001 decision ledger D1–D14 and its historical outcomes remain in `/home/jaron/dev/axe/iss-83-finish-caching/docs/plans/053_corrected_caching_001_checkpoint.md`; they are not reset by this checkpoint.

### Cap-state summary

- Cap `002/planning`: lane `full`; spec drafted and lint PASS; no checklist; independent spec review round 1 is next. No 002 review failures or incompletes; no review scope or dispatch established. Evidence: X002, P201 and L201b below.
- Planning: shared failing rounds `0`; incomplete count `0`; unresolved findings `none`; post-cap fixes/check `not applicable`. The spec is drafted and lint passed; no independent 002 review has launched, so these counts do not reset or replace the historical 001 counts.
- Code review `002`: gate/round `none`; original scope, slices, dispatches, blockers and unreviewed files `none` (no review scope established); combined outcome `not applicable`; round-four eligibility/reason and progress `not applicable`; incomplete count `none` until a scope/round exists.
- Tasks, task reviews, fixes and milestone reviews for 002: `0` launched; no task IDs, review rounds or gate scopes established.
- Historical 001 snapshot: shared planning failures `2`, active consecutive incomplete `0`, Q2 replacement consumed; milestone review failures `1`, active incomplete `0`, R005 passed, no open review findings, no round four; T1–T4 checked and 001 complete. Preserve the 001 checkpoint's full evidence, including V006–V008 failures and B008; none is erased or reclassified as a 002 pass.

### Bootstrap evidence B002-001

- Gate: current-session milestone validation, recovery, host preflight, pre-selection sync, Go preflight, and initial 002 checkpoint creation; no review round.
- Current-session validation, before recovery dispatch: `run_script milestone-entries -- docs/plans/053_corrected_caching_milestones.md` exited `0`; stdout `PASS: docs/plans/053_corrected_caching_milestones.md: 3 milestone entries`; stderr empty. `repo-state --commits 8` exited `0`: root and branch matched, HEAD `ad2b3796a0ed831739b8e3c2d6823cb8cbccc7f1`, base `origin/master` ahead `26`, behind `0`, operation `null`, staged `0`, unstaged `1`, untracked `0`, conflicted `0`; sole status ` M docs/plans/053_corrected_caching_001_checkpoint.md`. The milestone document identifies 001 checked, 002 first unchecked, 003 unchecked.
- REC009a/b: read-only recovery reconciled the pending 001 checkpoint and clean index with retained REC007/C001/REC008 evidence, critical stash objects, current inventory hashes, active identities, outcomes and caps. At recovery HEAD `ad2b3796a0ed831739b8e3c2d6823cb8cbccc7f1`, Y011 had passed without HEAD movement; C002 was not launched. REC009 established 002 checkpoint absent and no 002 planning, task, source or review launches. Historical V007/V008 failed outcomes are known, later superseded by passing 001 completion evidence, not unknown-result dispositions.
- B008 historical failure: `go-toolchain-check` run `general-b008-9CmViN` exited `1`; Go version probes could not create `/home/jaron/go/pkg/mod` on the read-only filesystem, and `govulncheck` failed with no go.mod file. The vulnerability scan did not pass. Combined log `/tmp/run-detached-1001/general-b008-9CmViN/output.log`; no 002 work launched. Do not retroactively turn this into a pass.
- E009 actual `factory-preflight --fix`, launch HEAD `ad2b3796a0ed831739b8e3c2d6823cb8cbccc7f1`, exit `0`, stderr empty. Actual JSON:

```json
{"ok":true,"repo":"/home/jaron/dev/axe/iss-83-finish-caching","checks":[{"name":"disk","status":"pass","detail":"38.5 GB free on /home/jaron/dev/axe/iss-83-finish-caching (minimum 15.0 GB)"},{"name":"tmp","status":"pass","detail":"/tmp: 38.5 GB free (ext4)"},{"name":"origin","status":"pass","detail":"git@github.com:jrswab/axe.git"},{"name":"gates","status":"pass","detail":"2 gate commands found"},{"name":"go","status":"pass","detail":"toolchain ok"},{"name":"browsers","status":"pass","detail":"3 browser binaries resolve"},{"name":"sandbox","status":"pass","detail":"bwrap starts and can write to the worktree"}],"fixed":["go build cache: pruned","uv cache: pruned","npm cache: pruned","/tmp: removed 116 stale entries"]}
```

  This is declared host preflight evidence, not a retroactive B008 pass or a vulnerability-scan result.
- REC009c restored the established 001 checked rendering after a bookkeeping call had defaulted it to unchecked; no substantive launch or commit occurred before that repair. The D009 cache-selection decision was recorded before B009.
- Y012 actual `factory-sync --wip` exit `0`, stderr empty, launched clean after the inspected 001 bookkeeping commit `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`. Actual JSON: `{"branch":"iss-83-finish-caching","base":"origin/master","head_before":"1e9e4c3064e3310da64ea62e3efe25aa7ce7b676","ahead":27,"behind":0,"shared_files":[],"conflicts":[],"synced":true}`. Result HEAD remained `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`; no WIP or merge commit and no HEAD change. This is also the pre-selection sync for 002.
- B009 actual launch: `env -u GOTOOLCHAIN XDG_CACHE_HOME=/tmp/axe-v006-cache.itoPgN GOMODCACHE=/tmp/axe-v006-cache.itoPgN/go-mod /home/jaron/.agents/scripts/run-detached start --name general-b009-f71a38 -- /home/jaron/.agents/scripts/go-toolchain-check /home/jaron/dev/axe/iss-83-finish-caching`; final exit `0`, PID `40`, source HEAD `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`, combined log `/tmp/run-detached-1001/general-b009-f71a38/output.log`. Go `1.26.11` downloaded, release and repository version probes passed, read-only module load returned `github.com/jrswab/axe`, `govulncheck ./...` returned `No vulnerabilities found.`, and `PASS: Go pre-flight checks.` Before/after status was only expected 001 checkpoint bookkeeping, identical diff, clean index; no source changes. Separate child stderr is not established.
- B009 used the recorded checkpoint-session PATH (the original B001 preflight PATH was not separately captured):

```text
/home/jaron/.pi/agent/bin:/home/jaron/.cache/pi-go-bin:/home/jaron/.local/share/../bin:/home/jaron/bin:/home/jaron/.local/bin:/home/jaron/.npm-global/bin:/home/jaron/.opencode/bin:/home/jaron/.local/share/pi-node/current/bin:/home/jaron/android-sdk/cmdline-tools/bin:/home/jaron/go/bin:/usr/local/go/bin:/home/jaron/.cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin
```

- C003 intent was recorded in 001 before this dispatch. Creation agent verified target absent, exclusively created this file without replacing an existing target, and verified spec/checklist absent. HEAD at creation `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`; index `270` entries, all stage 0, cached diff empty. Ordinary status is the baseline shown above; this new file is ignored by `.gitignore:59` and does not appear in ordinary status. No 002 context, planning, task, review or source dispatch was launched by creation.
- Relevant work: `uncommitted`; stash ref `none` (no snapshot created in this creation-only action). Untracked Git objects: `none` stored. Ignored files are not represented by ordinary status or the updater's untracked inventory; new ignored artifacts require explicit accounting if subsequently committed. Six ignored research files remain pre-existing local context per the milestone record.
- Next action: Record a 002 context-discovery intent before establishing lane and context.

### C003 PASS; X002 context discovery IN FLIGHT

- Gate: C003 PASS; X002 context discovery IN FLIGHT
- Commands/results:
C003 initial checkpoint creation completed, only new 002 checkpoint written. HEAD 1e9e4c3064e3310da64ea62e3efe25aa7ce7b676 unchanged. Initial ignored file hash 00db252578e37d8867ecfa05f8f83c1f4e26f6ba. Creator verified template, identity and absolute paths, git diff --check exit 0 and docs-lint exit 0 (53 Markdown files, 0 findings). Source tests not applicable. 001 existing bookkeeping preserved. X002 intent IN FLIGHT: read-only current-code/context inspection for 002 and Phase 1 spec instructions, establish full-lane classification from external wire-contract/AWS credential boundary, ground research and list exact affected interfaces/tests/docs. No tests/reviews/gates/sync or file writes authorized for discovery. Counts: planning failures 0, task reviews 0, milestone reviews 0, no incompletes/findings/round4. Full original acceptance and narrow Q3 assertion approval unchanged.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `be0ae0f751ef4b70f58bc6029df9cb43e96f2ad6` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Discover 002 code and phase-1 instructions, then record lane and spec intent

### X002 PASS; P201 spec drafting IN FLIGHT

- Gate: X002 PASS; P201 spec drafting IN FLIGHT
- Commands/results:
X002 completed read-only discovery with path:line evidence. Lane FULL: Bedrock wire contract and AWS setup boundary fail fast-lane criteria. Defects: bedrock.go:112-134 combines union fields, :164-196 misplaces tool checkpoint, :235-258 sends bare inputSchema and map-order required names. Nested internal/tool/tool.go:218-233 uses API key instead of config.ResolveRegion and existing AWS credentials; Main pkg/runner/run.go:385-414 already handles Bedrock. Preserve region precedence AWS_REGION > AWS_DEFAULT_REGION > AXE_BEDROCK_REGION > config and existing explicit environment/shared-file credential resolver only, not SDK chain. No blocker identified. P201 IN FLIGHT original spec gate 002 round not yet reviewed: write 002 spec from spec skill, full milestone research and accepted/rejected decisions, update initial checkpoint Lane placeholder only to full. No source/test/assertion changes authorized. Narrow Q3 approval preserved. Counts unchanged zero. Next gate spec-lint then independent clean-brief planning review.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `a2a9b70402650f6b44b05f8176c0b005650ac099` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Write 002 spec and establish full lane, then lint before review

### P201 draft PASS; L201 spec/docs lint IN FLIGHT

- Gate: P201 draft PASS; L201 spec/docs lint IN FLIGHT
- Commands/results:
P201 wrote 002 spec and set lane full at checkpoint:29, source HEAD 1e9e4c3064e3310da64ea62e3efe25aa7ce7b676. Spec lines 3-23 context/decisions; 27-40 wire; 42-48 AWS parity; 50-54 usage/failures; 56-62 docs/evidence. git diff --check exit 0; no source or assertion edits. L201 intent IN FLIGHT: run spec-lint --root worktree 002 spec and docs-lint --since 1e9e4c3064e3310da64ea62e3efe25aa7ce7b676. Fix mechanical findings only within same scope if needed after outcome and new intent. No planning review launched yet, shared failing count 0, incomplete count 0. Ignored new artifacts remain uncaptured by standard snapshot and must be force-added on approved commit; hashes from checkpoint inventory do not cover ignored files.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `d99c751dc69cced9782ca6a0d1bd1ea2d9f7ce72` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Run L201 spec-lint and docs-lint, then planning review with clean brief

### L201 citation FAIL/docs PASS; F201 citation repair IN FLIGHT

- Gate: L201 citation FAIL/docs PASS; F201 citation repair IN FLIGHT
- Commands/results:
L201 spec-lint exit 1: 002_spec.md:11 stale internal/provider/bedrock.go:305-365, file has 361 lines; 26 citations, 1 finding. docs-lint exit 0 header docs-lint 1, changed since 1e9e4c3064e3310da64ea62e3efe25aa7ce7b676, 54 Markdown files 0 findings. No review launched or failure count consumed. F201 intent IN FLIGHT: fix only stale citation using current Bedrock Send lines, verify fixed-string old text has zero hits; no requirement changes. Then rerun same lint gate. Counts remain zero.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `9c194e8bd2d0faea06fa302611b93ab4e48fa680` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Repair only stale spec citation, then rerun L201

### F201 PASS; L201b lint rerun IN FLIGHT

- Gate: F201 PASS; L201b lint rerun IN FLIGHT
- Commands/results:
F201 corrected only stale spec:11 citation to internal/provider/bedrock.go:305-361, exact Send range. Writer grep -F old internal/provider/bedrock.go:305-365 returned zero matches exit 1. git diff --check exit 0. No other content changed. L201b IN FLIGHT same lint gate: spec-lint and docs-lint commands unchanged. Independent old-text grep will accompany read-only review brief inspection. Review counts remain zero.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `f25d6759972561b7ec8d34dc5a4e8f99ce038120` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Rerun spec/docs lint, then independent planning review round 1

### L201b PASS; S201 artifact staging IN FLIGHT; P202 review intent

- Gate: L201b PASS; S201 artifact staging IN FLIGHT; P202 review intent
- Commands/results:
L201b spec-lint exit 0: 26 citations 0 findings. docs-lint exit 0, docs-lint 1, 54 Markdown files 0 findings, since milestone base 1e9e4c3064e3310da64ea62e3efe25aa7ce7b676. S201 IN FLIGHT: inspect and force-stage only new ignored 002 spec/checkpoint so they become durable; do not alter ignores or unrelated files. After complete index/pending inspection commit exactly 001 checkpoint and 002 spec/checkpoint. P202 planned independent spec review round 1, clean content brief only, no range, test evidence not applicable, after commit. Independent explore must confirm old citation zero matches before review. Shared failure count 0, incompletes 0, no findings. No code/tests changed.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `8f4a66621fe4767ab4bf258620e7d96fcd1a74c6` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Independently verify citation and bookkeeping scope, stage/commit planning artifacts, then P202 spec review

### S201 not staged; F202 stale summary repair IN FLIGHT

- Gate: S201 not staged; F202 stale summary repair IN FLIGHT
- Commands/results:
Independent precommit inspection verified all scope and citation checks, old citation grep zero hits, but found stale initial summary text at checkpoint:50 and :210 claiming context/lane/spec pending. No stage/commit/review launched. F202 IN FLIGHT: update only current summary text to established lane full, spec drafted/lint PASS, no checklist yet, next P202 spec review round 1, zero failing rounds/incompletes; preserve append-only historical initial evidence. Then force-stage only 002 checkpoint/spec, inspect full index/cached diff and pending changes of all 3 approved paths, and report SAFE/UNSAFE for exact commit. No source changes or review count consumed.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `e763bfaf0b0d88708798930193254772106fd682` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Repair stale 002 summaries, stage only approved new artifacts and verify commit scope

### F202 repaired summaries; S202 current summary reconciliation IN FLIGHT

- Gate: F202 repaired summaries; S202 current summary reconciliation IN FLIGHT
- Commands/results:
F202 changed current checkpoint summaries only, staged new 002 checkpoint/spec, checks passed. Agent found remaining adjacent current-summary sentence at :51 '002 artifact absence and no 002 planning launches' outdated after drafting. No review/commit launched. S202 IN FLIGHT: reconcile all current summary sections with established append-only evidence (full lane, spec drafted/lint PASS, no checklist or reviews, shared failing count zero, independent round-1 spec review next). Historical initial statements remain history. Stage exactly 001 checkpoint, 002 checkpoint, 002 spec and inspect entire index/pending changes. Staging 001 checkpoint is expressly approved; an unstaged approved path alone does not make factory-commit unsafe because it stages listed paths. No other files authorized.
- HEAD commit SHA: `1e9e4c3064e3310da64ea62e3efe25aa7ce7b676`
- Relevant work: `uncommitted` — stash ref `32efa79c9fe7c05885f80db050ba37c0608f1849` (tracked files except intent-to-add; never applied)
- `git status --porcelain` (before checkpoint/checkbox writes):

```text
 M docs/plans/053_corrected_caching_001_checkpoint.md
A  docs/plans/053_corrected_caching_002_checkpoint.md
A  docs/plans/053_corrected_caching_002_spec.md
```

- Untracked file Git objects (including intent-to-add; stored with hash-object -w, never applied):

```text

```

- Next action: Reconcile current summaries and verify exact three-file commit, then commit and review

## Limitations & blockers

- Limitations: Lane is `full`; the spec is drafted and lint PASS, and no checklist exists. The next review is independent spec review round 1; 002 has zero review failures and incompletes. Historical preflight and completion results are recorded as actual prior evidence, not rerun here. No source parse/type/lint or package tests apply to this summary repair. B008 remains a failure; E009 and B009 passed separately.
- Blockers: `none` for this authorized creation; Q3 does not authorize any assertion changes outside its stated narrow scope.
- Next action: Reconcile current summaries and verify exact three-file commit, then commit and review
