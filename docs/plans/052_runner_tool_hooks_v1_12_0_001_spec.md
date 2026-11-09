# Milestone 001: Publish the compatibility policy and link it

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 001 (Section 2, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:147-152`). Scope and acceptance are taken from that entry as amended by the decisions in this spec.
- Predecessor: milestone 000 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_000_checkpoint.md`, last gate entry "Milestone 000 review gate and completion gate PASS at c1d1121"). Milestone 001 has no code dependency (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:149`, "Depends on: none").
- Baseline: HEAD `defd01d2944d88d97fa2b0ad0efefda1d0b88a3a`, clean tree.
- Milestone 002 depends on this milestone (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:155`). 002 enforces the policy written here.

### Decisions settled by the milestone (record as settled, never re-evaluate)

- **D1. Compatibility policy (Go 1 style, all v1 releases).** `pkg/runner` API: no exported identifier removed or changed; additions allowed; callers use keyed struct literals and do not compare exported structs with `==`. CLI, TOML, `--json`: additive only. Default runtime behavior: unchanged except documented bug fixes (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54`).
- **D2. Behavior-change gate.** New behavior is opt-in through a new TOML key or `Options` field. The zero value keeps today's behavior. No GODEBUG-style switch (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:55`).
- **D3. Enforcement.** A pinned apidiff check of `pkg/runner` against `v1.11.0` runs in `make check` and CI. It runs as `go run golang.org/x/exp/cmd/apidiff@<pinned version>` and is not added to `go.mod` (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:56`). Milestone 001 documents D3 only. The gate itself is milestone 002, and the pin is unverified until 002 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:105`).
- **D4. Policy location.** Root `COMPATIBILITY.md`, linked from `README.md`, `AGENTS.md`, and the `CHANGELOG.md` header (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:57`). Deprecate, never remove, in v1; removal waits for `/v2` (D4 text in the milestone entry, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:147-152`, and D1 at `:54`).

### Pre-policy exceptions (recorded in COMPATIBILITY.md, not reverted)

1. **1.11.0 `temperature` change.** OpenAI and Atlas Cloud send `"temperature": 0` when agent TOML sets `temperature = 0`. Shipped in a minor release with no opt-out. Recorded at `CHANGELOG.md:24` under `## [1.11.0] - 2026-10-21` (`CHANGELOG.md:13`). Not reverted (D2).
2. **Minimum Go raised to 1.26.0 (milestone 000).** `go.mod:3` reads `go 1.26.0`; `go.mod:5` reads `toolchain go1.26.10`. Before milestone 000 the `go` directive was `go 1.25.0`. The reason is the security fix GO-2026-6860, which needs `golang.org/x/net` v0.62.0 (`docs/plans/052_runner_tool_hooks_v1_12_0_000_spec.md:41-50`). Importers on Go 1.25 cannot build `pkg/runner` after this change.

### Current values and drift (verified at HEAD defd01d)

- `go.mod:3` is `go 1.26.0`. `go.mod:5` is `toolchain go1.26.10`.
- The milestones document still says `go 1.25.0` at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:47` and at `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:103`. That text is the historical research baseline (it describes HEAD `ce6c096`, the state before milestone 000). It is not a current value. COMPATIBILITY.md states the current `go.mod` values (1.26.0 and go1.26.10) and names 1.25.0 only as the prior minimum. The milestones document is not edited by this milestone.
- `Makefile:9` is `check: lint test`. It has no apidiff step yet (milestone 002).
- `CHANGELOG.md:6` claims Semantic Versioning. `CHANGELOG.md:1-6` is the header where the policy link goes. The SemVer sentence is not changed by this milestone (milestone scope is a link only, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:150`).
- `COMPATIBILITY.md` does not exist at HEAD. This milestone creates it.
- `AGENTS.md:224` is the `## Custom Instructions` heading. The text under it is preserved verbatim by codebase-summary, so the policy link must not be placed there. `AGENTS.md:222` is the `## Project Docs` paragraph, which is an acceptable location.
- `README.md:38` is the `## Installation` heading. `README.md:13` is the `## Overview` heading. The link goes in prose outside fenced code blocks.

### Constraints

- **Documentation only.** No `.go`, `go.mod`, `go.sum`, `Makefile`, workflow, or `pkg/`/`internal/`/`cmd/` file changes. Compatibility impact is none (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:151`).
- **Frozen reference.** `docs/plans/runner-tool-hooks-and-options.md` is the committed requirements draft. Acceptance requires it not be edited (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:152`).
- **Link targets are repository-relative.** Each link must resolve from the file that contains it.
- **No questions to the user.** Factory override: unresolved questions are recorded as blockers in this section (see "Blockers and open questions" below).

### Rejected approaches (settled)

- Reverting the 1.11.0 `temperature` change (D2).
- Putting apidiff in `go.mod` (D3).
- Rewording `CHANGELOG.md:6` in this milestone. Only the header link is in scope (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:150`).
- Editing `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` to correct the `go 1.25.0` drift. The drift is recorded here; the milestones document is historical.

### Blockers and open questions

- **Blockers: none.** No unresolved question blocks milestone 001. Each candidate question below is settled by the milestone text or by a later milestone:
  - Exact wording of each surface rule: free text in COMPATIBILITY.md, bounded by D1 and D2 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:54-55`).
  - Apidiff pin value and gate behavior: milestone 002 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:105`, `:154-160`). 001 states only that the pin process exists.
  - SemVer sentence in `CHANGELOG.md:6`: not changed by 001 (scope is a link).
- **Known limitation:** the apidiff pin is unverified until milestone 002. COMPATIBILITY.md must not state a concrete pin version in this milestone.

## Section 2: Requirements

Requirements are verified by the checks listed in "Acceptance commands". Slices R1–R2 can be written together; R3–R5 are sections of the same file; R6 depends on R1 existing.

### R1: Policy file exists at the repository root

- `COMPATIBILITY.md` exists at the repository root and is Markdown.
- It opens by stating that it is the compatibility promise for axe v1 releases.
- Verification: `test -f COMPATIBILITY.md`.

### R2: The promise covers all five surfaces

- The file states the D1 promise for each surface: `pkg/runner` exported API; CLI flags and exit codes; agent TOML keys; `--json` output fields; default runtime behavior.
- For the API surface it states the D1 rules: no exported identifier removed or changed; additions allowed; keyed struct literals expected; exported structs not compared with `==`.
- For CLI, TOML, and `--json` it states additive-only.
- For runtime behavior it states unchanged except documented bug fixes.
- Verification: `rg -n -i "api|cli|toml|json|runtime" COMPATIBILITY.md` shows a rule for each surface. A reviewer confirms the five surfaces by heading or table row.

### R3: Opt-in rule for behavior changes

- The file states D2: new behavior is opt-in through a new TOML key or `Options` field; the zero value keeps today's behavior; no GODEBUG-style switch is used in v1.
- Verification: the text states all three points (opt-in, zero value, no switch).

### R4: Enforcement section describes the planned gate without claiming it exists

- The file has a section for enforcement that states D3: an apidiff check of `pkg/runner` against `v1.11.0`, run in `make check` and CI, invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>`, and not added to `go.mod`.
- The section states the gate lands in a later milestone (002). It does not claim the gate is active today. It states that the pin is recorded when the gate lands and is a concrete version, not `latest`.
- Verification: `rg -n "apidiff" COMPATIBILITY.md` finds the section; `rg -n "golang.org/x/exp" go.mod` returns no lines (unchanged; the pattern is not anchored to `require`, because `go.mod` has two `require (` blocks at lines 7 and 14).

### R5: Deprecation rule and pre-policy exceptions

- The file states D4: in v1, deprecate and never remove; removal waits for `/v2`.
- The file lists exactly two pre-policy exceptions, each with its version and source:
  1. The 1.11.0 `temperature = 0` behavior change, citing `CHANGELOG.md:24`. It states the change is not reverted.
  2. The minimum-Go raise from 1.25.0 to 1.26.0 made by milestone 000, citing `go.mod:3`. It states the `toolchain go1.26.10` line (`go.mod:5`) and that importers on Go 1.25 cannot build `pkg/runner` after this change.
- The file states the current values `go 1.26.0` and `go1.26.10` from `go.mod`. It does not present 1.25.0 as current.
- Verification: `rg -n "1\.25" COMPATIBILITY.md` hits only the exception text (the prior minimum); `rg -n "CHANGELOG.md:24|go.mod:3" COMPATIBILITY.md` finds both exceptions.

### R6: Links from the three documents resolve to COMPATIBILITY.md

- `README.md` contains a Markdown link to `COMPATIBILITY.md` in prose outside fenced code blocks.
- `AGENTS.md` contains a Markdown link to `COMPATIBILITY.md` in a section other than `## Custom Instructions`. The link line must be before the `## Custom Instructions` heading (currently `AGENTS.md:224`), i.e. outside that section. The boundary is the heading's line number found at runtime (`rg -n "^## Custom Instructions" AGENTS.md`), not a fixed number.
- `CHANGELOG.md` contains the link within its header, lines 1–6 (`CHANGELOG.md:1-6`).
- Each link destination is `COMPATIBILITY.md` relative to its file's directory (the root files all sit at the root, so the destination is `COMPATIBILITY.md`).
- Verification: `rg -n "COMPATIBILITY\.md" README.md AGENTS.md CHANGELOG.md` lists one hit per file in the stated location; `test -f COMPATIBILITY.md` resolves each.

### R7: Scope boundary

- Only these files change: `COMPATIBILITY.md` (new), `README.md`, `AGENTS.md`, and `CHANGELOG.md`.
- `docs/plans/runner-tool-hooks-and-options.md` is unchanged.
- No code, `go.mod`, `go.sum`, `Makefile`, workflow, `pkg/`, `internal/`, or `cmd/` file changes.
- No other line in README.md, AGENTS.md, or CHANGELOG.md changes beyond the link.
- Verification: the deliverable diff from `defd01d`, excluding the Factory plan records, lists only the four deliverable files. The Factory records are `docs/plans/052_runner_tool_hooks_v1_12_0_001_*` (this spec, checklist, checkpoint) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (checkbox render). Run after commit: `git diff --name-only defd01d -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_001_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'` must list only `COMPATIBILITY.md`, `README.md`, `AGENTS.md`, `CHANGELOG.md`. If checked before commit, also include untracked files from `git status --porcelain` (new `COMPATIBILITY.md`). `git diff --quiet defd01d -- docs/plans/runner-tool-hooks-and-options.md` exits 0 (frozen draft); `git diff defd01d -- README.md AGENTS.md CHANGELOG.md` shows only added link text.

### R8: Documentation lint passes

- `spec-lint` on this spec exits 0.
- `docs-lint --since origin/master` exits 0 after the change.
- Verification: the two commands in "Acceptance commands" exit 0.

## Parallelizable work

- R1 through R5 are one new file and may be drafted in one pass.
- R6 link insertions in README.md, AGENTS.md, and CHANGELOG.md are independent of each other and can be made in any order, but each needs `COMPATIBILITY.md` to exist (R1).
- R7 and R8 are checks run after R1–R6.

## Serial dependencies

- R1 before R6 (links resolve only when the target exists).
- R1–R6 before R7 and R8.

## Acceptance commands

Run in the repository root after the change. Each command is listed with its expected result.

```sh
test -f COMPATIBILITY.md                                              # expect: exit 0
rg -n "COMPATIBILITY\.md" README.md AGENTS.md CHANGELOG.md            # expect: one hit per file, README in prose, CHANGELOG in lines 1-6
rg -n "CHANGELOG.md:24|go.mod:3" COMPATIBILITY.md                     # expect: hits for both pre-policy exceptions
rg -n "apidiff" COMPATIBILITY.md                                      # expect: the D3 enforcement section
git diff --quiet defd01d -- docs/plans/runner-tool-hooks-and-options.md   # expect: exit 0
test "$(rg -n "COMPATIBILITY\.md" AGENTS.md | head -1 | cut -d: -f1)" -lt "$(rg -n "^## Custom Instructions" AGENTS.md | cut -d: -f1)"   # expect: exit 0 (AGENTS link line before the Custom Instructions heading, both found at runtime)
rg -n "golang.org/x/exp" go.mod                                       # expect: no lines
git diff --quiet defd01d -- go.mod go.sum Makefile .github pkg internal cmd   # expect: exit 0
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_001_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since origin/master .                    # expect: exit 0
```

## Failure modes

1. **Link inside `## Custom Instructions`.** codebase-summary preserves that section verbatim, so the link would be lost on regeneration. Mitigation: place the AGENTS.md link above `AGENTS.md:224` (R6).
2. **Policy claims a gate that does not exist yet.** Mitigation: R4 requires the enforcement section to say the gate lands in milestone 002.
3. **Stale `go 1.25.0` text copied from the milestones document.** Mitigation: R5 requires the current `go.mod` values; the 1.25.0 mention is only the prior minimum.
4. **Broken relative link.** Mitigation: R6 verification with `test -f` and `rg`.
5. **Edit to the frozen draft.** Mitigation: R7 `git diff --quiet` check on `docs/plans/runner-tool-hooks-and-options.md`.
