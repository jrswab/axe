# Implement Milestone 001: Publish the compatibility policy and link it

## Section 1: Context Summary

Milestone 001 of the v1.12.0 runner tool-hooks release publishes the compatibility promise for axe v1 as a new root file, `COMPATIBILITY.md`, and links it from `README.md`, `AGENTS.md`, and the `CHANGELOG.md` header. The policy is Go 1 style: no exported `pkg/runner` identifier is removed or changed, CLI, TOML, and `--json` changes are additive only, and default runtime behavior changes only for documented bug fixes. New behavior is opt-in through a new TOML key or `Options` field, and the zero value keeps today's behavior. Deprecate, never remove, in v1. The file also describes the planned apidiff gate (milestone 002) without claiming it is active, and it records two pre-policy exceptions: the 1.11.0 `temperature = 0` change (`CHANGELOG.md:24`) and the minimum-Go raise to 1.26.0 with `toolchain go1.26.10` (`go.mod:3`, `go.mod:5`). This milestone changes documentation only. No `.go`, `go.mod`, `go.sum`, `Makefile`, workflow, `pkg/`, `internal/`, or `cmd/` file changes. The source is `docs/plans/052_runner_tool_hooks_v1_12_0_001_spec.md` (Sections 1 and 2) and the milestone entry in `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`.

Baseline: HEAD `23cc89f`. The spec was written against `defd01d`; the spec commits `c99312c` and `23cc89f` are docs-only. Every cited line below was re-checked at `23cc89f` and matches.

---

## Section 2: Implementation Checklist

Commit is not authorized. Do not push. Do not change files outside the list in each task. Do not edit the spec, the checkpoint, the milestones document, or `docs/plans/runner-tool-hooks-and-options.md`.

Failing-first convention: this milestone has no Go code, so each task's "Before" check is the spec's acceptance check run before the change. It must fail (non-zero or no hits). The "After" check must pass.

### Phase 1: Policy file (serial: T1 → T2 → T3, all edit `COMPATIBILITY.md`)

- [ ] **T1: `COMPATIBILITY.md` — create the file, opening statement, and five surface rules (R1, R2)**
  - File: create `COMPATIBILITY.md` at the repository root.
  - Content: a level-1 title, an opening statement that the file is the compatibility promise for axe v1 releases, and one level-2 heading per surface: `pkg/runner` exported API, CLI flags and exit codes, agent TOML keys, `--json` output fields, default runtime behavior.
  - API rules (D1): no exported identifier removed or changed; additions allowed; callers use keyed struct literals; exported structs are not compared with `==`.
  - CLI, TOML, and `--json` rules: additive only.
  - Runtime rule: unchanged except documented bug fixes.
  - Do not state a concrete apidiff pin or a Go version in this task.
  - Before (must fail): `test -f COMPATIBILITY.md` exits 1.
  - After (must pass): `test -f COMPATIBILITY.md` exits 0; `rg -n -i "api|cli|toml|json|runtime" COMPATIBILITY.md` shows a rule for each of the five surfaces; `rg -n "^#" COMPATIBILITY.md` shows a heading for each surface.
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

- [ ] **T2: `COMPATIBILITY.md` — opt-in rule for behavior changes and the planned enforcement section (R3, R4)**
  - Depends on: T1.
  - File: `COMPATIBILITY.md`, append two sections.
  - R3 section: state D2. New behavior is opt-in through a new TOML key or `Options` field. The zero value keeps today's behavior. No GODEBUG-style switch is used in v1.
  - R4 section (enforcement): state D3. An apidiff check of `pkg/runner` against `v1.11.0` runs in `make check` and CI. It is invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>` and is not added to `go.mod`. State that the gate lands in milestone 002 and is not active today. State that the pin is recorded when the gate lands and is a concrete version, not `latest`.
  - Do not write a concrete pin version. Milestone 001 does not verify the pin (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:105`).
  - Before (must fail): `rg -n "apidiff" COMPATIBILITY.md` exits 1 (no file content yet, or no match after T1).
  - After (must pass):
    - `rg -n -i "opt-in" COMPATIBILITY.md`, `rg -n -i "zero value" COMPATIBILITY.md`, and `rg -n "GODEBUG" COMPATIBILITY.md` each print at least one line.
    - `rg -n "apidiff" COMPATIBILITY.md` finds the enforcement section.
    - `rg -n "apidiff@v" COMPATIBILITY.md` prints no output (no concrete pin).
    - `rg -n "golang.org/x/exp/cmd/apidiff@" COMPATIBILITY.md` prints one line.
    - `rg -n "milestone 002" COMPATIBILITY.md` prints one line.
    - `rg -n "golang.org/x/exp" go.mod` exits 1 (no lines; `go.mod` is unchanged).
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

- [ ] **T3: `COMPATIBILITY.md` — deprecation rule and the two pre-policy exceptions (R5)**
  - Depends on: T2.
  - File: `COMPATIBILITY.md`, append one deprecation section and one exceptions section.
  - Deprecation (D4): in v1, deprecate and never remove. Removal waits for `/v2`.
  - Exception 1: the 1.11.0 `temperature = 0` behavior change for OpenAI and Atlas Cloud. Cite `CHANGELOG.md:24`. State that it is not reverted.
  - Exception 2: the minimum-Go raise from 1.25.0 to 1.26.0 made by milestone 000. Cite `go.mod:3`. State the `toolchain go1.26.10` line (`go.mod:5`). State that importers on Go 1.25 cannot build `pkg/runner` after this change.
  - State current values `go 1.26.0` and `go1.26.10`. Present 1.25.0 only as the prior minimum.
  - Pre-check, before editing: `sed -n 24p CHANGELOG.md` contains `temperature`; `sed -n 3p go.mod` prints `go 1.26.0`; `sed -n 5p go.mod` prints `toolchain go1.26.10`. These confirm the cited lines.
  - Before (must fail): `rg -n "CHANGELOG.md:24|go.mod:3" COMPATIBILITY.md` exits 1.
  - After (must pass):
    - `rg -n "CHANGELOG.md:24|go.mod:3" COMPATIBILITY.md` prints hits for both exceptions.
    - `rg -n "1\.25" COMPATIBILITY.md` prints only lines in the exception text (the prior minimum). Read each hit to confirm none presents 1.25 as current.
    - `rg -n "go 1.26.0|go1.26.10" COMPATIBILITY.md` prints at least one line.
    - `rg -n "/v2" COMPATIBILITY.md` prints at least one line.
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

### Phase 2: Links (parallel after T1; each task edits one file, so T4, T5, and T6 can run in any order)

- [ ] **T4: `README.md` — link to `COMPATIBILITY.md` in the Overview prose (R6, README)**
  - Depends on: T1.
  - File: `README.md`, insert one new prose paragraph after `README.md:17` (the "Axe is the executor, not the scheduler" paragraph) and before `## Features` at `README.md:19`. The paragraph reads: "The compatibility promise for v1 releases is in [COMPATIBILITY.md](COMPATIBILITY.md)."
  - Re-read `README.md:13-20` immediately before the edit. Do not change any existing line.
  - Placement is outside any fenced code block: between `## Overview` (`README.md:13`) and `## Features`, and no fence line falls in that range.
  - Before (must fail): `rg -n "COMPATIBILITY\.md" README.md` exits 1.
  - After (must pass): `rg -n "COMPATIBILITY\.md" README.md` prints exactly one line, and that line number is between 13 and 19 after the insert. `git diff -U0 -- README.md` shows only `+` lines.
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

- [ ] **T5: `AGENTS.md` — link to `COMPATIBILITY.md` in the Project Docs section (R6, AGENTS)**
  - Depends on: T1.
  - File: `AGENTS.md`, insert one new paragraph after `AGENTS.md:222` (the "Design docs live in `./docs/`" paragraph) and before `## Custom Instructions` at `AGENTS.md:224`. The paragraph reads: "The compatibility promise for v1 releases is in [COMPATIBILITY.md](COMPATIBILITY.md)."
  - Do not place the link inside `## Custom Instructions`. That section is preserved verbatim by codebase-summary (`AGENTS.md:224`).
  - Re-read `AGENTS.md:218-226` immediately before the edit. Do not change any existing line.
  - Before (must fail): `rg -n "COMPATIBILITY\.md" AGENTS.md` exits 1.
  - After (must pass):
    - `test "$(rg -n "COMPATIBILITY\.md" AGENTS.md | head -1 | cut -d: -f1)" -lt "$(rg -n "^## Custom Instructions" AGENTS.md | cut -d: -f1)"` exits 0.
    - `rg -n "COMPATIBILITY\.md" AGENTS.md` prints exactly one line.
    - `git diff -U0 -- AGENTS.md` shows only `+` lines.
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

- [ ] **T6: `CHANGELOG.md` — link to `COMPATIBILITY.md` in the header, lines 1–6 (R6, CHANGELOG)**
  - Depends on: T1.
  - File: `CHANGELOG.md`, append a sentence to the end of line 3 (`All notable changes to this project will be documented in this file.`). The line then reads: "All notable changes to this project will be documented in this file. The compatibility promise for v1 releases is in [COMPATIBILITY.md](COMPATIBILITY.md)."
  - Append to line 3 and not as a new line. A new line in the header would move the SemVer sentence at `CHANGELOG.md:6` out of lines 1–6. The spec's header range is lines 1–6, and the SemVer sentence is not changed (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:150`).
  - Do not edit the SemVer sentence at `CHANGELOG.md:6`.
  - Re-read `CHANGELOG.md:1-8` immediately before the edit.
  - Before (must fail): `rg -n "COMPATIBILITY\.md" CHANGELOG.md` exits 1.
  - After (must pass):
    - `rg -n "COMPATIBILITY\.md" CHANGELOG.md` prints exactly one line, and that line number is 1–6.
    - `sed -n 6p CHANGELOG.md` still prints the Semantic Versioning sentence.
    - `git diff --numstat -- CHANGELOG.md` after the change shows 1 added line and 1 deleted line (line 3 replaced by its extended text). Confirm the only change to line 3 is the appended sentence with `git diff -U0 -- CHANGELOG.md`.
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

### Phase 3: Scope and lint verification (after T1–T6)

- [ ] **T7: Scope boundary check before commit (R7)**
  - Depends on: T1, T2, T3, T4, T5, T6.
  - No file edits. Verification only.
  - Verify:
    - `git status --porcelain` lists `?? COMPATIBILITY.md`, ` M README.md`, ` M AGENTS.md`, ` M CHANGELOG.md`, and the Factory records (`docs/plans/052_runner_tool_hooks_v1_12_0_001_*` and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`). It lists nothing else.
    - `git diff --numstat defd01d -- README.md AGENTS.md CHANGELOG.md` shows 0 deleted lines in README.md and AGENTS.md. CHANGELOG.md shows 1 deleted line (line 3, extended in T6).
    - `git diff --quiet defd01d -- go.mod go.sum Makefile .github pkg internal cmd` exits 0.
    - `git diff --quiet defd01d -- docs/plans/runner-tool-hooks-and-options.md` exits 0.
    - `rg -n "golang.org/x/exp" go.mod` exits 1 (no lines).
  - After commit (Factory runs this, not this task): `git diff --name-only defd01d -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_001_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'` must list only `COMPATIBILITY.md`, `README.md`, `AGENTS.md`, and `CHANGELOG.md`.

- [ ] **T8: Documentation lint and full acceptance replay (R8)**
  - Depends on: T7.
  - No file edits. Verification only.
  - Verify:
    - `~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_001_spec.md` exits 0.
    - `~/.agents/scripts/docs-lint --since origin/master .` exits 0.
    - Replay the spec's acceptance block in full:
      - `test -f COMPATIBILITY.md` (exit 0)
      - `rg -n "COMPATIBILITY\.md" README.md AGENTS.md CHANGELOG.md` (one hit per file; README in prose; CHANGELOG at lines 1–6)
      - `rg -n "CHANGELOG.md:24|go.mod:3" COMPATIBILITY.md` (both hits)
      - `rg -n "apidiff" COMPATIBILITY.md` (D3 section)
      - `git diff --quiet defd01d -- docs/plans/runner-tool-hooks-and-options.md` (exit 0)
      - the AGENTS ordering `test` command from T5 (exit 0)
      - `rg -n "golang.org/x/exp" go.mod` (no lines)
      - `git diff --quiet defd01d -- go.mod go.sum Makefile .github pkg internal cmd` (exit 0)
  - Per-task gate: `git diff --name-only -- '*.go'` prints nothing.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 `COMPATIBILITY.md` exists at root, Markdown, opens with v1 promise | T1 |
| R2 five surfaces with D1 rules (API, CLI, TOML, `--json`, runtime) | T1 |
| R3 opt-in rule (D2): opt-in, zero value, no GODEBUG switch | T2 |
| R4 enforcement section (D3): apidiff against v1.11.0, `make check` and CI, `go run` form, not in `go.mod`, gate lands in milestone 002 | T2 |
| R5 deprecation (D4) and exactly two pre-policy exceptions, cited at `CHANGELOG.md:24` (temperature) and at go.mod:3 (Go minimum) | T3 |
| R6 links in README.md (prose), AGENTS.md (before `## Custom Instructions`), CHANGELOG.md (lines 1–6) | T4, T5, T6 |
| R7 scope boundary: only four deliverable files; frozen draft unchanged; no code or module changes | T7 (pre-commit), Factory post-commit diff |
| R8 `spec-lint` exit 0 and `docs-lint --since origin/master` exit 0 | T8 |

Every R-ID R1–R8 is covered by at least one task.

## Parallel Work and Serial Dependencies

- Serial: T1 → T2 → T3 (all three edit `COMPATIBILITY.md`).
- Parallel after T1: T4 (`README.md`), T5 (`AGENTS.md`), T6 (`CHANGELOG.md`). Each edits one distinct file.
- Serial after T1–T6: T7 → T8.

## Phase 2 Gate Record

- Changed signatures: none. This milestone changes no `.go` file, so there are no caller lists and no caller-update tasks.
- Extreme acceptance probes: N/A for code. The acceptance inputs are file contents and line anchors. Probes were run at HEAD `23cc89f` before writing this checklist:
  - `test -f COMPATIBILITY.md` exits 1. The file does not exist. This is the failing precondition for T1.
  - `rg -n "golang.org/x/exp" go.mod` exits 1 with no output.
  - `rg -n "^## Custom Instructions" AGENTS.md` prints `224:## Custom Instructions`.
  - `rg -n "^## (Overview|Installation)|Requires Go" README.md` shows `## Overview` at 13, `## Installation` at 38, and `Requires Go 1.26+.` at 40.
  - `sed -n 3p go.mod` is `go 1.26.0`; `sed -n 5p go.mod` is `toolchain go1.26.10`.
  - `CHANGELOG.md:24` is the 1.11.0 temperature entry; `CHANGELOG.md:6` is the SemVer sentence.
- `go install` or `GOPROXY=` commands: none in this checklist. `go-toolchain-check` is not run, and the pre-flight rule is not triggered.
- Checklist audit: `checklist-test-gate-audit` is not on PATH, and this repository has no `.agents/scripts/` directory. The audit is skipped.

## Blockers

- None. The spec's Section 1 records no blocking questions for milestone 001. The known limitation (the apidiff pin is unverified until milestone 002) is handled by T2: COMPATIBILITY.md states no concrete pin.
- Assumption (not a blocker): the CHANGELOG link is appended to line 3 (T6), not added as a new header line. This keeps the SemVer sentence at `CHANGELOG.md:6`, which the spec cites. If the orchestrator prefers a separate line, T6 would move the SemVer sentence out of lines 1–6 and the spec's R6 range would need amending.
