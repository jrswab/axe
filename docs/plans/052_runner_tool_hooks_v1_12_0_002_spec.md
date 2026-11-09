# Milestone 002: Enforce the `pkg/runner` compatibility gate with a pinned apidiff

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 002 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154-159`). Scope and acceptance are taken from that entry as amended by the decisions in this spec.
- Predecessor: milestone 001 is complete (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:147`, `[x] 001`). It published `COMPATIBILITY.md`, which this milestone updates (R8).
- Baseline: review base `423a3994c56c9e8ee055e03072b80f6741589f68`, clean tree. Branch `runner-tool-hooks`.
- Lane: full. The milestone enforces the `pkg/runner` external contract and edits shared CI. It adds a pinned third-party tool to the build path. The fast-lane condition "external contracts untouched" fails.

### Decisions settled (record as settled, never re-evaluate)

- **D3. Enforcement (milestone).** A pinned apidiff check of `pkg/runner` against `v1.11.0` runs in `make check` and CI. It runs as `go run golang.org/x/exp/cmd/apidiff@<pinned version>` and is not added to `go.mod` (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:56`). `go.mod` stays free of `golang.org/x/exp` (`go.mod:7-23` lists the current requires; no `x/exp` line).
- **Base version (milestone).** The base is `v1.11.0` and is not moved until a later release (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:157`).
- **D-pin.** Pin `golang.org/x/exp@v0.0.0-20261106192419-47ce03d25ef0`, recorded once in the `Makefile` as a variable. The pin is concrete, not `latest`. Rationale: the latest release available at implementation time (probe below), and a fixed pin keeps the gate reproducible.
- **D-fail.** The gate runs apidiff with `-incompatible`. It fails (non-zero exit) when apidiff's stdout is non-empty and prints that stdout as the report. It passes when stdout is empty. Tool and load errors also fail the gate. Rationale: apidiff exits 0 on an incompatible change (probe below). Exit status alone cannot detect a break.
- **D-base-tree.** The base operand is created under `mktemp -d` by the steps in R3 (`mkdir`, then `git archive -o` to `<tmp>/base.tar`, then `tar -x -f` into `<tmp>/base`, each status checked), with a trap that removes the temp directory on exit. This replaces the milestone's `git worktree add <tmp>/base v1.11.0` (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:157`, step (2)). **This is a deviation from milestone 002 Scope step (2).** Rationale: this checkout is a linked worktree of the shared bare repository `/home/jaron/dev/axe/.bare` (verified by `git rev-parse --git-common-dir`). `git worktree add` registers state in that shared repository and leaves stale registrations on failure. The resulting tree is the same `v1.11.0` tree.
- **D-script.** The multi-step logic lives in a `Makefile` recipe, not in a new script. Rationale: the repository has no `scripts/` or `.agents/scripts/` directory (verified: `ls` at HEAD). The recipe is one shell block of about a dozen lines, and the Makefile is already the single entry point for `check`. A new script would add a versioned file that CI and `make check` would both depend on.
- **D-check.** `check: lint test apidiff` (`Makefile:9` changes from `check: lint test`).
- **D-ci.** A dedicated `apidiff` job is added to `.github/workflows/go.yml`. Its steps are `actions/checkout@v4` with `fetch-depth: 0`, `actions/setup-go@v5` with `go-version-file: go.mod`, then `make apidiff`. The existing lint, test, and build jobs keep their checkout settings (`.github/workflows/go.yml:14`, `:27`, `:42`).
- **D-doc.** `COMPATIBILITY.md` section `## Enforcement (planned)` (`COMPATIBILITY.md:37`) is updated. It states the gate is active, names `make apidiff`, `make check`, and the CI job, and records the concrete pin. "(planned)" and "not active today" are removed (`COMPATIBILITY.md:41`). This update is required so the published policy stays true.
- **D-changelog.** Add one entry under `## [Unreleased]` `### Added` (`CHANGELOG.md:8`, `:10`), after the existing `provider-key-check` entry (`CHANGELOG.md:11`). Evidence: the repository convention for CI changes is a CHANGELOG entry. Commit `d1e8f4b` ("ci: add scheduled provider key check (#98)") added its CI workflow under `### Added` in `CHANGELOG.md`. `CONTRIBUTING.md:185` says "Update CHANGELOG.md with your changes". The entry is a contributor-facing gate, so it is user-visible for contributors.
- **D-factory-gate.** `.agents/factory-gates.json` is not changed. An apidiff gate there would read from origin's default branch, which is out of scope for this milestone. Current content: `test` and `lint` only (`.agents/factory-gates.json:2-3`).
- **D-base-tag.** The base stays `v1.11.0`. CI needs tags, so checkout uses `fetch-depth: 0` (D-ci).

### Probe evidence (this session)

A subagent ran these probes. All writes went under `/tmp/apidiff-probe.Cdxk`. The repository was not modified. Record these as evidence; the implementation must re-verify the pin and the exit behavior.

- Pin: `go list -m -json golang.org/x/exp@latest` returned `v0.0.0-20261106192419-47ce03d25ef0` (Time `2026-11-06T19:24:19Z`). The proxy `@latest` agreed.
- Base export: `git archive v1.11.0` extracted to a temp directory. `go run golang.org/x/exp/cmd/apidiff@v0.0.0-20261106192419-47ce03d25ef0 -w <tmp>/old.export github.com/jrswab/axe/pkg/runner` exited 0. The base `go.mod` declares `go 1.25.0`; the toolchain switched to go1.26.10; base `go.mod` and `go.sum` were unchanged.
- Compare from the current tree: exit 0, empty output. Repository `go.mod` and `go.sum` unchanged.
- Scratch copy with `func IsRuntimeError` removed: stdout `Incompatible changes:` and `- IsRuntimeError: removed`, exit 0. With `-incompatible`: stdout `- IsRuntimeError: removed`, exit 0.
- Scratch copy with `func ProbeNew() {}` added: listed under `Compatible changes:`, exit 0. Additions do not fail the gate.
- Source, apidiff at the pin (module cache `golang.org/x/exp@v0.0.0-20261106192419-47ce03d25ef0/cmd/apidiff/main.go`, cited without line colons because it is outside the repository): flags `-w`, `-incompatible`, `-allow-internal`, `-m` are declared at lines 27-30. The `-w` export path exits 0 at line 69. The compare path prints the report at lines 95-103. Only write and load errors call `die`, which exits 1 (lines 333-335). The report is written to stdout. The incompatible-only text writer is `TextIncompatible` in `apidiff/report.go` (`func (r Report) TextIncompatible` at line 45, no header when `withHeader` is false).

### Current values (verified at HEAD `423a399`)

- `Makefile:1` is `.PHONY: test lint check`. `Makefile:9` is `check: lint test`. The `test` target is `go test ./...` (`Makefile:4`). The `lint` target is `golangci-lint run` (`Makefile:7`). The Makefile has no apidiff target.
- `.github/workflows/go.yml:9` is `jobs:`. Three jobs exist: `lint` (`:11`), `test` (`:24`), `build` (`:39`). Each has `actions/checkout@v4` (`:14`, `:27`, `:42`) and `go-version-file: go.mod` (`:19`, `:32`, `:47`).
- `go.mod:3` is `go 1.26.0`. `go.mod:5` is `toolchain go1.26.10`.
- `COMPATIBILITY.md:37` is `## Enforcement (planned)`. Its bullets are `:39`-`:42`. `COMPATIBILITY.md:41` says "The gate lands in milestone 002. It is not active today." `COMPATIBILITY.md:42` says "The pin is recorded when the gate lands. It is a concrete version, not `latest`."
- `CHANGELOG.md:8` is `## [Unreleased]`. `CHANGELOG.md:10` is `### Added`. `CHANGELOG.md:11` is the `provider-key-check` entry. `CHANGELOG.md:13` is `## [1.11.0] - 2026-10-21`.
- `pkg/runner/errors.go:63` is `func IsRuntimeError(err error) bool {`. It is the identifier used by the scratch-copy removal test (R10).
- `v1.11.0` is an annotated tag. Its tag object is `5e49a3952fac98b188df4e99538de5a9cfbb523c` and it peels to commit `20e6ada179b08cef050c4b436a2497182efcf2ee` (`git rev-parse v1.11.0` and `git rev-parse v1.11.0^{commit}` at `423a399`). The tag exists in the shared repository.

### Action defaults (for the Action inputs table)

Read from the upstream `action.yml` at the pinned major tags (raw GitHub URLs, HTTP 200; `gh api` could not authenticate in this sandbox):

- `https://raw.githubusercontent.com/actions/checkout/v4/action.yml` (checkout `v4`).
- `https://raw.githubusercontent.com/actions/setup-go/v5/action.yml` (setup-go `v5`).

### Constraints

- **Scope.** Changed files are limited to `Makefile`, `.github/workflows/go.yml`, `COMPATIBILITY.md`, and `CHANGELOG.md`, plus Factory records. `go.mod`, `go.sum`, `pkg/`, `internal/`, `cmd/`, and `.agents/factory-gates.json` are not changed.
- **Toolchain.** The gate's `go run` must build with the toolchain CI resolves from `go.mod` (`go 1.26.0`, `toolchain go1.26.10`).
- **Network.** The first `go run` of the apidiff module needs the module proxy. Local `make check` now needs network on a cold cache. This is a limitation, recorded below.
- **Tags.** The gate needs the local tag `v1.11.0`. A clone without tags fails the gate with a message (R3).
- **Gate runs do not modify `go.mod` or `go.sum`.** The export and compare runs leave both files unchanged (probe confirmed).

### Rejected approaches (settled)

- apidiff in `go.mod` (D3).
- `git worktree add` for the base tree (D-base-tree).
- Gating on the apidiff exit status alone (D-fail).
- `@latest` as the pin (D-pin).
- A new gate in `.agents/factory-gates.json` (D-factory-gate).
- Moving the base past `v1.11.0` (milestone).
- Changing `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` beyond the Factory checkbox at completion.

### Blockers and open questions

- **Blockers: none.** Every decision above is settled by the milestone or the orchestrator decisions. The CHANGELOG decision (D-changelog) is recorded with evidence.
- **Conflict for the orchestrator (not blocking this milestone).** Milestone 012 requires the `[1.12.0]` section to list every Added item and "No item not listed here appears" (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:285`). The D-changelog entry sits under `[Unreleased]`. When 012 creates `[1.12.0]`, this entry must either be listed there or be moved out. Milestone 012 will reconcile this. This spec does not change 012.
- **Known limitation:** the pin is verified by the probe, not by a CI run. The implementation checklist must run `make apidiff` before the milestone can close.

## Section 2: Requirements

Requirements are verified by the checks in "Acceptance commands". Requirements R1-R4 are one Makefile change. R5 is the CI job. R6-R9 are documentation and records. R10 is the behavior check.

### R1: Concrete pin recorded once

- The `Makefile` declares one variable holding `golang.org/x/exp/cmd/apidiff@v0.0.0-20261106192419-47ce03d25ef0`, or the module version alone, used by every apidiff invocation.
- The pin is a concrete pseudo-version. The Makefile contains no `@latest` and no second pin.
- Verification: `rg -n "v0\.0\.0-" Makefile` shows one pin line; `rg -n "@latest" Makefile` returns no lines.

### R2: `make apidiff` target and `check` wiring

- `Makefile` gains an `apidiff` target. The name `apidiff` is added to `.PHONY` (`Makefile:1`).
- `check` becomes `check: lint test apidiff` (`Makefile:9`).
- Every recipe line of the `apidiff` target begins with Make's `@` prefix, so Make does not echo the recipe and a successful run prints nothing to stdout from Make. The incompatible-change branch (R4) prints the report and the failure message explicitly.
- Verification: `make -n check` lists the apidiff recipe; `rg -n "^check:" Makefile` shows `apidiff`.

### R3: Base tree from the `v1.11.0` tag without `git worktree`

- The target first resolves the tag with `git rev-parse --verify 'v1.11.0^{commit}' >/dev/null; status=$$?`. If `$$status` is non-zero, the target exits non-zero with a message that names the missing tag and says to fetch tags (for example `git fetch --tags`). The message names the tag and the fix.
- The target creates a temporary directory with `mktemp -d` under the template `axe-apidiff.XXXXXX` (in `${TMPDIR:-/tmp}`).
- The base tree is created in this order, with `tmp` holding the path from `mktemp -d`:
  1. `mkdir "$tmp/base"`.
  2. `git archive -o "$tmp/base.tar" v1.11.0; status=$$?`. If `$$status` is non-zero, the target exits with that status.
  3. `tar -x -f "$tmp/base.tar" -C "$tmp/base"; status=$$?`. If `$$status` is non-zero, the target exits with that status.
  No pipeline is used, so each status is the command's own status.
- The Makefile defines `APIDIFF_PKG ?= github.com/jrswab/axe/pkg/runner`. Every apidiff invocation uses `$(APIDIFF_PKG)` for the package path.
- The target runs the `-w` export in the base directory: `go run <pinned apidiff> -w <tmp>/old.export $(APIDIFF_PKG)`. The export file lives inside the temp directory. The export is a standalone command with explicit status handling: run `go run <pinned apidiff> -w <tmp>/old.export $(APIDIFF_PKG); status=$$?`, then if `$$status` is non-zero, exit with that status before the compare runs. The export uses no pipeline and no `local` or `export` prefix.
- The target does not run `git worktree add`, `git worktree prune`, or any command that writes to the shared repository's worktree registry.
- Verification: `git worktree list` before and after `make apidiff` shows the same entries; `git -C <repo> count-objects` is not used; `rg -n 'git worktree' Makefile` returns no lines.

### R4: Compare with `-incompatible`; fail on any report or error

- The compare runs from the repository root: `go run <pinned apidiff> -incompatible <tmp>/old.export $(APIDIFF_PKG)`.
- Gate result:
  - Compare exits non-zero (tool or load error): the target fails with that exit status.
  - Compare exits 0 and stdout is non-empty: the target prints the stdout report, prints a message that says `pkg/runner` has incompatible changes against `v1.11.0`, and exits non-zero.
  - Compare exits 0 and stdout is empty: the target passes.
- Only stdout is checked for the report. Build output on stderr does not fail the gate on its own. A non-zero exit from `go run` still fails the gate.
- Shell control flow (POSIX `sh`, Make's default `SHELL`; no Bash-only syntax such as `[[`, arrays, `pipefail`, or `$'...'`):
  1. Capture stdout and status in one assignment, then read the status on the next line: `report=$$(go run <pinned apidiff> -incompatible <tmp>/old.export $(APIDIFF_PKG)); status=$$?`.
  2. Branch on `$$status` before interpreting `$$report`. If it is non-zero, exit with that status.
  3. Only on the zero-status branch: if `$$report` is non-empty, print it, print the incompatible-changes message, and exit 1. If it is empty, pass.
  - Forbidden: `local` or `export` on the capture line, any pipeline whose left side is the `go run` command, and relying on `set -e` to catch the status.
- Verification: R10 cases, plus this check, run in a scratch clone (created as in R10). These edits exist only in the scratch clone; the repository's Makefile is not changed.
  - Unloadable package: run `make apidiff APIDIFF_PKG=github.com/jrswab/doesnotexist`. It exits non-zero.
  - Corrupted export: in the scratch clone's Makefile, add the line `printf 'garbage' > "$$tmp/old.export"` immediately after the `-w` export status check and before the compare, where `tmp` is the shell variable holding the `mktemp -d` path. Then run `make apidiff`. It exits non-zero.

### R5: CI `apidiff` job

- `.github/workflows/go.yml` gains one job named `apidiff`. It has `runs-on: ubuntu-latest`.
- Its steps, in order: `actions/checkout@v4` with `fetch-depth: 0`; `actions/setup-go@v5` with `go-version-file: go.mod`; a `run:` step that executes `make apidiff`.
- The job does not change the lint, test, or build jobs. Their checkout lines (`.github/workflows/go.yml:14`, `:27`, `:42`) keep their current settings (no `fetch-depth`).
- Verification: `rg -n "fetch-depth" .github/workflows/go.yml` shows exactly one line, in the `apidiff` job; `rg -n "make apidiff" .github/workflows/go.yml` shows one line.
- Host step: a CI run of the new job passes on the unmodified tree. The orchestrator checks it on the pushed branch.

### R6: `go.mod` and `go.sum` unchanged

- The gate's base and compare runs do not change `go.mod` or `go.sum`.
- The milestone does not change `go.mod` or `go.sum`.
- Verification: `git diff --exit-code 423a399 -- go.mod go.sum` exits 0 after `make apidiff` runs.

### R7: Temporary directory removed on pass and on fail

- A trap removes the temp directory on every exit path, including a failed compare and a failed export.
- Verification: before any run, `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` prints `0`. After a passing run and after a failing run (R10 case 2), the same command prints `0`.

### R8: COMPATIBILITY.md states the gate as active

- Heading `## Enforcement (planned)` (`COMPATIBILITY.md:37`) becomes `## Enforcement`.
- Bullets `COMPATIBILITY.md:39-42` are replaced by text that:
  - states an apidiff check compares `pkg/runner` against the `v1.11.0` tag and runs in `make check` and in CI (the CI job is `apidiff` in `.github/workflows/go.yml`);
  - names the target `make apidiff`;
  - states the check is invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>` and is not added to `go.mod`;
  - records the concrete pin `v0.0.0-20261106192419-47ce03d25ef0`, and says it is recorded in the `Makefile`;
  - states the gate fails on any incompatible report, and on any tool error.
- The text does not contain "planned", "not active", or "lands in milestone".
- Verification: `rg -n -i "planned|not active|lands in" COMPATIBILITY.md` returns no lines; `rg -n "v0\.0\.0-" COMPATIBILITY.md Makefile` shows the same pin in both files.

### R9: CHANGELOG entry under Unreleased

- One bullet under `### Added` in `## [Unreleased]` (`CHANGELOG.md:8`, `:10`), after the existing bullet at `CHANGELOG.md:11`.
- The bullet states that `make check` and CI now run a pinned apidiff check of `pkg/runner` against `v1.11.0`, and that an incompatible change fails the check.
- No other `CHANGELOG.md` line changes.
- Verification: `git diff -U0 423a399 -- CHANGELOG.md` shows one added line and no removed lines.

### R10: Behavior check: pass on the unmodified tree; fail on removal; pass on addition

Run in the repository for case 1. Run in scratch copies for cases 2 and 3. Each scratch copy is made with `git clone --quiet --no-checkout <repository root> <scratch>` followed by `git -C <scratch> checkout --detach 423a3994c56c9e8ee055e03072b80f6741589f68`. The clone carries all tags including `v1.11.0` and is checked out at the baseline commit, not the source repository's default branch. Then the changed working-tree files are copied into the clone. The clone writes nothing to the shared repository's worktree registry. The scratch directory is removed afterward.

The empty-stdout checks (cases 1 and 3) test stdout only. stderr may carry `go` download or toolchain messages and does not fail them. `make apidiff` is run without `-s`; the `@` prefix on the recipe suppresses command echo, so `-s` is not needed.

1. **Unmodified tree.** `make apidiff` exits 0 and prints nothing to stdout. `make check` exits 0 (lint and test as before, plus apidiff).
2. **Removed identifier.** In a scratch copy, delete `func IsRuntimeError` (`pkg/runner/errors.go:63`, the function body and its doc comment at `:62`). `make apidiff` in the scratch copy exits non-zero. Its stdout contains the line `- IsRuntimeError: removed`, and the failure message is printed. The temp directory count is `0` afterward (R7).
3. **Added identifier.** In a scratch copy, add `func ProbeNew() {}` in a new file under `pkg/runner`. `make apidiff` exits 0 and prints nothing to stdout (additions are compatible and not reported by `-incompatible`).
4. **Tool error.** In a scratch clone made as above but with `git clone --no-tags`, the tag `v1.11.0` is absent. `make apidiff` then exits non-zero with the R3 message. The shared repository's tags are not touched.

### R11: Scope boundary

- Changed tracked files, excluding Factory records, are `Makefile`, `.github/workflows/go.yml`, `COMPATIBILITY.md`, and `CHANGELOG.md`.
- Factory records are the checkpoint, the spec, the checklist, and the milestone checkbox (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154`) at completion.
- Verification: `git diff --name-only 423a399 -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'` lists exactly `.github/workflows/go.yml`, `CHANGELOG.md`, `COMPATIBILITY.md`, and `Makefile`.

### Action inputs

This spec adopts two pinned actions: `actions/checkout@v4` and `actions/setup-go@v5` (R5). Each non-null default in the pinned `action.yml` has one row. `filter` and `sparse-checkout` are omitted because their default is null.

Checkout `v4` (`actions/checkout@v4`):

| Input | Default | Overridden | Why acceptable |
|---|---|---|---|
| repository | `${{ github.repository }}` | no | Checks out this repository only. |
| token | `${{ github.token }}` | no | The job reads the repository and fetches tags. No step pushes or writes to the repository. |
| ssh-strict | `true` | no | Applies only when `ssh-key` is set. This job uses HTTPS. |
| ssh-user | `git` | no | Applies only when `ssh-key` is set. This job uses HTTPS. |
| persist-credentials | `true` | no | The token stays in the job's local git config. The job runs no untrusted script and makes no push. |
| clean | `true` | no | Cleans the workspace before fetch. The runner workspace is fresh. |
| sparse-checkout-cone-mode | `true` | no | Applies only when `sparse-checkout` is set. This job does not set it. |
| fetch-depth | `1` | yes: `0` | Needed so tag `v1.11.0` exists (D-ci). Full history and tags for this job only. |
| fetch-tags | `false` | no | Not needed: `fetch-depth: 0` already fetches all tags. |
| show-progress | `true` | no | Log output only. |
| lfs | `false` | no | The repository uses no LFS objects for `pkg/runner`. |
| submodules | `false` | no | The repository has no submodules used by `pkg/runner`. |
| set-safe-directory | `true` | no | Needed for git commands in the workspace (git archive). |
| allow-unsafe-pr-checkout | `false` | no | Applies only to pull-request checkouts from forks through `pull_request_target`. This workflow uses `pull_request`, not `pull_request_target`. |

Setup-go `v5` (`actions/setup-go@v5`):

| Input | Default | Overridden | Why acceptable |
|---|---|---|---|
| check-latest | `false` | no | `go-version-file: go.mod` pins the toolchain from the module. Checking for newer toolchains is not wanted. |
| token | `${{ github.server_url == 'https://github.com' && github.token || '' }}` | no | Used only to download Go distributions from go-versions. Same token and reason as checkout's token. |
| cache | `true` | no | Caches the Go module and build cache keyed on `go.sum`. This job does not change `go.sum`. |

## Parallelizable work

- R1-R4 (`Makefile`), R5 (`.github/workflows/go.yml`), R8 (`COMPATIBILITY.md`), and R9 (`CHANGELOG.md`) are independent edits to four files. They can be made in any order, one edit call per file.
- R10 checks run after R1-R4 exist.

## Serial dependencies

- R1-R4 before R10 (the target must exist to be tested).
- R5 before the CI run in the Host step (the job must exist on the branch).
- R1-R9 before R11 (scope check reads the final diff).

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result.

```sh
make -n check                                                            # expect: lists the apidiff recipe
make apidiff; echo "exit=$?"                                             # expect: exit=0, empty stdout
make check; echo "exit=$?"                                               # expect: exit=0
rg -n "v0\.0\.0-" Makefile COMPATIBILITY.md                              # expect: the pin in both files
rg -n "@latest" Makefile                                                 # expect: no lines
rg -n 'git worktree' Makefile                                            # expect: no lines
rg -n "fetch-depth" .github/workflows/go.yml                             # expect: exactly one line, in the apidiff job
rg -n "make apidiff" .github/workflows/go.yml                            # expect: one line
rg -n -i "planned|not active|lands in" COMPATIBILITY.md                  # expect: no lines
git diff --exit-code 423a399 -- go.mod go.sum                            # expect: exit 0
git diff -U0 423a399 -- CHANGELOG.md | rg -c "^[-+][^-+]"                # expect: 1
ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l                # expect: 0 (before and after each run)
git diff --name-only 423a399 -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'   # expect: .github/workflows/go.yml, CHANGELOG.md, COMPATIBILITY.md, Makefile
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since origin/master .                      # expect: exit 0
```

R10 cases 2-4 run in scratch copies, so they are not in this list. Each is run with its own scratch directory and removed afterward.

## Failure modes

1. **Exit status hides a break.** apidiff exits 0 on an incompatible change (probe). Mitigation: D-fail, the stdout check in R4.
2. **Base tree registered in the shared repository.** `git worktree add` would write to `/home/jaron/dev/axe/.bare`. Mitigation: D-base-tree (R3).
3. **Shallow CI checkout.** Without `fetch-depth: 0` the tag is missing and the gate fails. Mitigation: R3 tag check and R5 `fetch-depth: 0`.
4. **Pin drifts to `latest`.** Mitigation: R1 grep for `@latest`.
5. **Temporary directories left behind on failure.** Mitigation: trap (R3, R7).
6. **Policy claims an inactive gate.** Mitigation: R8 wording check.
7. **Gate runs against a stale base.** The base is `v1.11.0` by name (`git archive v1.11.0`), not `HEAD`. Mitigation: R3 names the tag, not a branch.

## Limitations

- The pin is verified by a probe in this session, not by a CI run. `make apidiff` and the CI job run for the first time at implementation.
- Cold-cache `make check` needs network to fetch apidiff and its dependencies.
- The `gh` CLI was unauthenticated in this sandbox. Action defaults were read from raw GitHub URLs (see "Action defaults").
- Milestone 012 reconciliation of the CHANGELOG entry is recorded above; it is not part of this milestone.
