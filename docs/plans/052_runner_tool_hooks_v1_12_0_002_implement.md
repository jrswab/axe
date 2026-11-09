# Implement Milestone 002: Enforce the `pkg/runner` compatibility gate with a pinned apidiff

## Section 1: Context Summary

Milestone 002 of the v1.12.0 runner tool-hooks release turns the compatibility policy from milestone 001 into an enforced gate. A pinned apidiff check compares `pkg/runner` against the `v1.11.0` tag. It runs as `go run golang.org/x/exp/cmd/apidiff@<pin>` (not added to `go.mod`) through one `make apidiff` target, wired into `make check` (`Makefile:9`) and into a new `apidiff` CI job. The gate treats any stdout report from `apidiff -incompatible` as a failure, because apidiff exits 0 on an incompatible change. The base tree is extracted from `git archive v1.11.0` into a `mktemp -d` directory with a trap, not created with `git worktree add`, because this checkout is a linked worktree of the shared bare repository. The milestone also updates `COMPATIBILITY.md` from "(planned)" to active and adds one CHANGELOG entry under `[Unreleased]`. It changes no Go source, `go.mod`, or `go.sum`. Source: `docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md` (Sections 1 and 2, R1-R11, Acceptance commands).

Baseline: `b0eff05` (the commit that adds this checklist; descendants may change only `docs/plans/052_runner_tool_hooks_v1_12_0_002_*` files). The code baseline is `423a399`; the only changes since it are the checkpoint and spec under `docs/plans/052_runner_tool_hooks_v1_12_0_002_*`. The tree is clean at HEAD.

---

## Section 2: Implementation Checklist

Commit is not authorized in this checklist. Do not push. Do not change files outside the list in each task. Do not edit the spec, the checkpoint, or the milestones document, except the milestone checkbox and checkpoint at completion (orchestrator).

Failing-first convention: each behavior task states the check that must fail before its change where one exists. Scratch-clone steps run only in the scratch directory named in the task; the repository's `Makefile` is changed only by T1.1-T1.3.

Scratch clone setup (used by T6-T9). Run from the repository root:

```sh
S="$(mktemp -d)/clone" && git clone --quiet --no-checkout /home/jaron/dev/axe/runner-tool-hooks "$S" && git -C "$S" checkout --detach 423a3994c56c9e8ee055e03072b80f6741589f68 && git -C "$S" rev-parse --verify 'v1.11.0^{commit}' && mkdir -p "$S/.github/workflows" && cp Makefile "$S/" && cp .github/workflows/go.yml "$S/.github/workflows/" && cp COMPATIBILITY.md CHANGELOG.md "$S/"
```

Expect: the last `rev-parse` prints `20e6ada179b08cef050c4b436a2497182efcf2ee`. Each task that uses a scratch clone removes it afterward with `rm -rf "$(dirname "$S")"`.

### Phase 0: Pre-flight (serial, read-only)

- [ ] **T0: Pre-flight: HEAD, clean tree, pin resolution, temp dirs, worktree snapshot**
  - Budget: 30 min
  - Files: none (read-only).
  - Commands and expected results:
    - `git merge-base --is-ancestor b0eff05 HEAD` → exit 0 (HEAD is `b0eff05` or a descendant).
    - `git diff --name-only b0eff05 HEAD -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*'` → no output (any commits after `b0eff05` touch only `docs/plans/052_runner_tool_hooks_v1_12_0_002_*` files).
    - `git status --porcelain --untracked-files=no` → no output (tracked tree clean).
    - `git rev-parse --verify 'v1.11.0^{commit}'` → `20e6ada179b08cef050c4b436a2497182efcf2ee`.
    - `go list -m -json golang.org/x/exp@v0.0.0-20261106192419-47ce03d25ef0` → JSON whose `Version` is `v0.0.0-20261106192419-47ce03d25ef0`. (Re-verifies the probe pin, spec "Probe evidence": "the implementation must re-verify the pin". Needs network on a cold cache.)
    - `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
    - `git worktree list > /tmp/axe-m002-worktrees-before.txt` (snapshot for R3 verification in T10).
  - Go toolchain pre-flight: not triggered. No checklist command runs `go install` or sets `GOPROXY=`; the apidiff commands use `go run` with the default proxy. Recorded as "skipped, not triggered".

### Phase 1: Edits (parallel groups; T1.1 → T1.2 → T1.3 serial, all edit `Makefile`; T2, T3, T4 are independent of each other)

- [ ] **T1.1: `Makefile` — pinned apidiff module variable, `APIDIFF_PKG`, `.PHONY`, `check` wiring (R1, R2)**
  - Budget: 30 min
  - File: `Makefile`.
  - Changes:
    - `Makefile:1`: `.PHONY: test lint check` becomes `.PHONY: test lint check apidiff`.
    - Add `APIDIFF_MODULE = golang.org/x/exp/cmd/apidiff@v0.0.0-20261106192419-47ce03d25ef0` once (the single pin). Use `APIDIFF_MODULE` in every apidiff invocation. The name is a choice within the spec's R1 allowance.
    - Add `APIDIFF_PKG ?= github.com/jrswab/axe/pkg/runner` (R3).
    - `Makefile:9`: `check: lint test` becomes `check: lint test apidiff`.
    - Do not add `@latest`. Do not add a second pin.
  - Verification (R1, R2):
    - `rg -n "v0\.0\.0-" Makefile` → one line (the pin).
    - `rg -n "@latest" Makefile` → no lines.
    - `rg -n "^check:" Makefile` → line shows `apidiff`.
    - `make -n check` → lists the `apidiff` recipe once T1.2-T1.3 exist. Before T1.2 it may report no rule for `apidiff`; re-run after T1.3.
  - Gate: `git diff --name-only -- '*.go' go.mod go.sum` prints nothing.

- [ ] **T1.2: `Makefile` — `apidiff` target: tag check, temp dir, trap, base tree from `git archive` (R3, R7)**
  - Depends on: T1.1.
  - Budget: 30 min
  - File: `Makefile`, new target `apidiff`.
  - Recipe shape (F1): the `apidiff` target is ONE logical shell command. Its first physical line starts with Make's `@` prefix; every later physical line is a continuation of that logical line (TAB-indented, ending `; \`), and the last line has no `\`. Make strips the `@` once, so no other physical line carries it. This meets spec R2's "every recipe line begins with `@`" by treating the recipe as one logical line. All steps share one shell, so `tmp` and the `EXIT` trap survive across steps. POSIX `sh` only; no `[[`, arrays, `pipefail`, or `$'...'`.
  - Required steps, in order:
    1. `git rev-parse --verify 'v1.11.0^{commit}' >/dev/null; status=$$?`. If `$$status` is non-zero, print a message that names tag `v1.11.0` and says to fetch tags (for example `git fetch --tags`), then exit non-zero.
    2. `tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/axe-apidiff.XXXXXX")`. The variable must be named `tmp`; the R4 corrupted-export probe (T9) inserts a line that uses it.
    3. `trap 'rm -rf "$$tmp"' EXIT` (R7: removes the directory on every exit path).
    4. `mkdir "$$tmp/base"; status=$$?`, exit with `$$status` if non-zero.
    5. `git archive -o "$$tmp/base.tar" v1.11.0; status=$$?`, exit with `$$status` if non-zero.
    6. `tar -x -f "$$tmp/base.tar" -C "$$tmp/base"; status=$$?`, exit with `$$status` if non-zero.
    7. Export, in a subshell so the working directory is restored after it: `(cd "$$tmp/base" && go run $(APIDIFF_MODULE) -w "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?`, exit with `$$status` if non-zero, before the compare. Use no pipeline and no `local` or `export` prefix.
  - Forbidden in this target: `git worktree` in any form (D-base-tree; spec R3).
  - Verification (static, now):
    - `rg -n 'git worktree' Makefile` → no lines.
    - `make -n apidiff` → shows the steps above, with no `git worktree` command.
  - Verification (runtime, after T1.3): covered by T5 (pass) and T6-T9 (failures and temp-dir count). Worktree registry unchanged: `git worktree list | diff - /tmp/axe-m002-worktrees-before.txt` → no output (T10).
  - Gate: `git diff --name-only -- '*.go' go.mod go.sum` prints nothing.

- [ ] **T1.3: `Makefile` — compare with `-incompatible`; stdout gate (R4)**
  - Depends on: T1.2.
  - Budget: 30 min
  - File: `Makefile`, `apidiff` target (continues after step 7 of T1.2).
  - Required steps, in order:
    1. `report=$$(go run $(APIDIFF_MODULE) -incompatible "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?`. The export runs in a subshell (T1.2 step 7), so this compare runs in the make invocation directory (the repository root), not `$$tmp/base`. Capture stdout and status in one assignment, then read the status in the next command of the same logical line. No `local` or `export` on this line. No pipeline whose left side is `go run`. No reliance on `set -e`.
    2. If `$$status` is non-zero, exit with `$$status`. Only stdout is checked for the report; stderr does not fail the gate on its own.
    3. If `$$report` is non-empty: print `$$report`, print a message that `pkg/runner` has incompatible changes against `v1.11.0`, and exit 1.
    4. If `$$report` is empty, the target passes and prints nothing to stdout.
  - Assembled recipe (T1.2 steps 1-7 and T1.3 steps 1-4 as one logical line; this is the whole target body, each continuation line is TAB-indented):
    ```make
    apidiff:
	@git rev-parse --verify 'v1.11.0^{commit}' >/dev/null; status=$$?; \
	if [ $$status -ne 0 ]; then echo "apidiff: tag v1.11.0 not found; run 'git fetch --tags'" >&2; exit $$status; fi; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/axe-apidiff.XXXXXX"); \
	trap 'rm -rf "$$tmp"' EXIT; \
	mkdir "$$tmp/base"; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	git archive -o "$$tmp/base.tar" v1.11.0; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	tar -x -f "$$tmp/base.tar" -C "$$tmp/base"; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	(cd "$$tmp/base" && go run $(APIDIFF_MODULE) -w "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	report=$$(go run $(APIDIFF_MODULE) -incompatible "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	if [ -n "$$report" ]; then printf '%s\n' "$$report"; echo "apidiff: pkg/runner has incompatible changes against v1.11.0" >&2; exit 1; fi
    ```
  - Verification:
    - `make -n apidiff` → shows the compare line and the `-incompatible` flag. The export line is a subshell `(cd ... && go run ... -w ...)`, and no `cd` precedes the compare line.
    - Compare reads the current tree (F1): T6 (removed identifier) must fail. A self-compare (base export compared with the base tree) would pass, so a T6 failure shows the compare reads the repository tree.
    - `rg -n "incompatible" Makefile` → at least one line in the `apidiff` target.
    - `rg -n "set -e|pipefail" Makefile` → no lines.
  - Gate: `git diff --name-only -- '*.go' go.mod go.sum` prints nothing.

- [ ] **T2: `.github/workflows/go.yml` — add `apidiff` job (R5)**
  - Budget: 30 min
  - Depends on: none (independent of T1.x).
  - File: `.github/workflows/go.yml`. Add one job `apidiff` after the existing `build` job (`.github/workflows/go.yml:39`; its last step line is `:50`, `run: go build -v ./...`).
  - Job body:
    - `runs-on: ubuntu-latest`.
    - Step 1: `uses: actions/checkout@v4` with `with: fetch-depth: 0`.
    - Step 2: `uses: actions/setup-go@v5` with `with: go-version-file: go.mod`.
    - Step 3: `run: make apidiff`.
  - Do not change the `lint`, `test`, or `build` jobs. Their checkout steps (`.github/workflows/go.yml:14`, `:27`, `:42`) keep their current settings.
  - Verification:
    - `rg -n "fetch-depth" .github/workflows/go.yml` → exactly one line, inside the `apidiff` job.
    - `rg -n "make apidiff" .github/workflows/go.yml` → one line.
    - Parse check (addendum requirement, not an acceptance check): `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/go.yml'))"` → exit 0.
  - Host step (orchestrator, not the writer): a CI run of the `apidiff` job on the pushed branch passes on the unmodified tree. Needs push approval per AGENTS.md; not done here.

- [ ] **T3: `COMPATIBILITY.md` — activate the enforcement section (R8)**
  - Budget: 30 min
  - Depends on: T1.1 (for the pin cross-check).
  - File: `COMPATIBILITY.md`, section `## Enforcement (planned)` (`COMPATIBILITY.md:37`) and its bullets (`COMPATIBILITY.md:39-42`).
  - Change:
    - Heading becomes `## Enforcement`.
    - Replace bullets `COMPATIBILITY.md:39-42` with text that:
      - states an apidiff check compares `pkg/runner` against the `v1.11.0` tag, and runs in `make check` and in CI (the CI job is `apidiff` in `.github/workflows/go.yml`);
      - names the target `make apidiff`;
      - states the check is invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>` and is not added to `go.mod`;
      - records the concrete pin `v0.0.0-20261106192419-47ce03d25ef0` and says it is recorded in the `Makefile`;
      - states the gate fails on any incompatible report and on any tool error.
    - Remove the words "planned", "not active", and "lands in milestone" (`COMPATIBILITY.md:41` is the current "not active today" line).
  - Verification:
    - `rg -n -i "planned|not active|lands in" COMPATIBILITY.md` → no lines.
    - `rg -n "v0\.0\.0-" COMPATIBILITY.md Makefile` → the same pin in both files (needs T1.1).
  - Gate: `git diff --name-only -- '*.go' go.mod go.sum` prints nothing.

- [ ] **T4: `CHANGELOG.md` — one entry under `[Unreleased]` / `### Added` (R9)**
  - Budget: 30 min
  - File: `CHANGELOG.md`. Insert one bullet after the existing `provider-key-check` bullet at `CHANGELOG.md:11`, before the blank line and `## [1.11.0]` at `CHANGELOG.md:13`. Anchors: `## [Unreleased]` at `:8`, `### Added` at `:10`.
  - Bullet text: states that `make check` and CI now run a pinned apidiff check of `pkg/runner` against `v1.11.0`, and that an incompatible change fails the check.
  - No other line changes.
  - Verification:
    - `git diff -U0 423a399 -- CHANGELOG.md | rg -c '^\+([^+]|$)'` → `1` (added content lines; `+++` header excluded).
    - `git diff -U0 423a399 -- CHANGELOG.md | rg -c '^-([^-]|$)'` → no output, exit 1 (zero removed content lines; `---` header excluded; checked again in T10).
  - Gate: `git diff --name-only -- '*.go' go.mod go.sum` prints nothing.

### Phase 2: Repository verification (serial, after T1.3, T2, T3, T4)

- [ ] **T5: Run `make apidiff` in the repository; wiring and tree checks (R1, R2, R3, R4 pass case, R6, R7; R10 case 1 for apidiff)**
  - Budget: 30 min
  - Depends on: T1.3, T2, T3, T4.
  - Files: none (verification only). This task makes no edits.
  - Note: the first run needs network on a cold module cache. The run is not `make check`; `make check` runs lint and `go test ./...` and is covered by T11 and the orchestrator's factory-gate.
  - Commands and expected results:
    - `make apidiff >/tmp/axe-m002-out.txt 2>/tmp/axe-m002-err.txt; echo "exit=$?"` → `exit=0`.
    - `wc -c </tmp/axe-m002-out.txt` → `0` (stdout empty; stderr may carry go messages and does not fail the check).
    - `make -n check` → lists the `apidiff` recipe (wiring).
    - `ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l` → `0`.
    - `git diff --exit-code 423a399 -- go.mod go.sum; echo "exit=$?"` → `exit=0`.
  - Gate: `git status --porcelain` shows no new or modified file produced by the apidiff run. Only the T1-T4 files and the checklist appear.

### Phase 3: Behavior checks in scratch clones (R10 cases 2-4; R4 failure checks; R3, R7). Each task creates its own scratch clone with the setup block above and removes it afterward. Tasks T6-T9 can run in parallel after T5.

- [ ] **T6: R10 case 2 (removed identifier fails) and R7 temp-dir count**
  - Budget: 30 min
  - Depends on: T5.
  - Scratch: a fresh clone per the setup block.
  - Edit (scratch only): remove `pkg/runner/errors.go:62-66` (doc comment `:62`, `func IsRuntimeError` `:63`, body `:64-65`, closing brace `:66`). Read `"$S/pkg/runner/errors.go"` lines 60-68 first and confirm the range before deleting. Command: `sed -i '62,66d' "$S/pkg/runner/errors.go"`.
  - Commands and expected results:
    - `PT="$(dirname "$S")/probe-tmp"; mkdir -p "$PT" && TMPDIR="$PT" make -C "$S" apidiff >"$S/../out.txt" 2>"$S/../err.txt"; echo "exit=$?"` → non-zero exit. (F3: each probe has its own `TMPDIR`, so parallel probes do not share a temp-dir count.)
    - `rg -n "^- IsRuntimeError: removed$" "$S/../out.txt"` → one line.
    - The failure message from R4 step 3 is printed (stdout or stderr; check `"$S/../out.txt"` and `"$S/../err.txt"`).
    - `ls -A "$PT" | wc -l` → `0` (R7 on fail; the probe's own `probe-tmp` is empty).
  - Cleanup: `rm -rf "$(dirname "$S")"`.
  - Gate: the repository tree is unchanged: `git status --porcelain` lists no `pkg/` file.

- [ ] **T7: R10 case 3 (added identifier passes)**
  - Budget: 30 min
  - Depends on: T5.
  - Scratch: a fresh clone per the setup block.
  - Edit (scratch only): create `"$S/pkg/runner/probe_new.go"` with `package runner` and `func ProbeNew() {}`.
  - Commands and expected results:
    - `PT="$(dirname "$S")/probe-tmp"; mkdir -p "$PT" && TMPDIR="$PT" make -C "$S" apidiff >"$S/../out.txt" 2>"$S/../err.txt"; echo "exit=$?"` → `exit=0`.
    - `wc -c <"$S/../out.txt"` → `0`.
    - `ls -A "$PT" | wc -l` → `0`.
  - Cleanup: `rm -rf "$(dirname "$S")"`.

- [ ] **T8: R10 case 4 (tool error on missing tag) and R3 tag message**
  - Budget: 30 min
  - Depends on: T5.
  - Scratch: a fresh clone per the setup block.
  - Edit (scratch clone only): `git -C "$S" tag -d v1.11.0`.
  - Commands and expected results:
    - `PT="$(dirname "$S")/probe-tmp"; mkdir -p "$PT" && TMPDIR="$PT" make -C "$S" apidiff >"$S/../out.txt" 2>"$S/../err.txt"; echo "exit=$?"` → non-zero exit.
    - The R3 step 1 message names `v1.11.0` and says to fetch tags. Check: `rg -n "v1\.11\.0" "$S/../out.txt" "$S/../err.txt"` → at least one line, and `rg -n -i "fetch" "$S/../out.txt" "$S/../err.txt"` → at least one line.
    - `git tag -l v1.11.0` run in the repository root → prints `v1.11.0` (the shared repository's tags are not touched).
    - `ls -A "$PT" | wc -l` → `0`.
  - Cleanup: `rm -rf "$(dirname "$S")"` (also removes `probe-tmp`).

- [ ] **T9: R4 failure checks: unloadable package and corrupted export**
  - Budget: 30 min
  - Depends on: T5.
  - Scratch: two fresh clones, one per case. Both via the setup block.
  - Case A, unloadable package (scratch clone A):
    - `PT="$(dirname "$S")/probe-tmp"; mkdir -p "$PT" && TMPDIR="$PT" make -C "$S" apidiff APIDIFF_PKG=github.com/jrswab/doesnotexist >"$S/../out.txt" 2>"$S/../err.txt"; echo "exit=$?"` → non-zero exit.
  - Case B, corrupted export (scratch clone B):
    - Edit the scratch `Makefile` only. Insert one new physical line between the T1.2 step 7 export line and the T1.3 step 1 `report=` line. The line's first character is a TAB, followed by `printf 'garbage' > "$$tmp/old.export"; \`. This keeps the single logical `apidiff` command joined (F2). The line uses the variable `tmp` from T1.2 step 2 and ends with `; \` like its neighbours.
    - `make -C "$S" -n apidiff` → must parse without error and print the `printf 'garbage'` command. Only after that, run the probe.
    - `PT="$(dirname "$S")/probe-tmp"; mkdir -p "$PT" && TMPDIR="$PT" make -C "$S" apidiff >"$S/../out.txt" 2>"$S/../err.txt"; echo "exit=$?"` → non-zero exit.
  - For both cases: `ls -A "$PT" | wc -l` → `0`, using each case's own `probe-tmp`.
  - Cleanup: `rm -rf "$(dirname "$S")"` for each clone.

### Phase 4: Scope, acceptance replay, and records (serial, last)

- [ ] **T10: Scope boundary, worktree registry, and no-Go-change checks (R3, R6, R11)**
  - Budget: 30 min
  - Depends on: T1.3, T2, T3, T4, T5, T6-T9.
  - Files: none (verification only).
  - Commands and expected results:
    - `git diff --name-only 423a399 -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'` → exactly `.github/workflows/go.yml`, `CHANGELOG.md`, `COMPATIBILITY.md`, `Makefile`.
    - `git diff --name-only 423a399 -- '*.go'` → no lines.
    - `git diff --exit-code 423a399 -- go.mod go.sum; echo "exit=$?"` → `exit=0`.
    - `git worktree list | diff - /tmp/axe-m002-worktrees-before.txt` → no output.
    - `git diff -U0 423a399 -- CHANGELOG.md | rg -c '^\+([^+]|$)'` → `1`.
    - `git status --porcelain` → only the four files above (tracked), the checklist and checkpoint and spec (untracked or Factory records), and nothing else.

- [ ] **T11: Acceptance replay (all spec "Acceptance commands") and `make check` hand-off**
  - Budget: 30 min
  - Depends on: T10.
  - Files: none (verification only).
  - Run in the repository root, in this order. Each line lists the expected result:
    ```sh
    make -n check                                                            # expect: lists the apidiff recipe
    make apidiff; echo "exit=$?"                                             # expect: exit=0, empty stdout
    make check; echo "exit=$?"                                               # expect: exit=0 (lint, test, apidiff); see note below
    rg -n "v0\.0\.0-" Makefile COMPATIBILITY.md                              # expect: the pin in both files
    rg -n "@latest" Makefile                                                 # expect: no lines
    rg -n 'git worktree' Makefile                                            # expect: no lines
    rg -n "fetch-depth" .github/workflows/go.yml                             # expect: exactly one line, in the apidiff job
    rg -n "make apidiff" .github/workflows/go.yml                            # expect: one line
    rg -n -i "planned|not active|lands in" COMPATIBILITY.md                  # expect: no lines
    git diff --exit-code 423a399 -- go.mod go.sum                            # expect: exit 0
    git diff -U0 423a399 -- CHANGELOG.md | rg -c '^\+([^+]|$)'               # expect: 1
    git diff -U0 423a399 -- CHANGELOG.md | rg -c '^-([^-]|$)'                # expect: no output, exit 1 (zero removed)
    ls -d "${TMPDIR:-/tmp}"/axe-apidiff.* 2>/dev/null | wc -l                # expect: 0 (before and after each run)
    git diff --name-only 423a399 -- . ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_002_*' ':(exclude)docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md'   # expect: .github/workflows/go.yml, CHANGELOG.md, COMPATIBILITY.md, Makefile
    ~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_002_spec.md   # expect: exit 0
    ~/.agents/scripts/docs-lint --since origin/master .                      # expect: exit 0
    ```
  - Note on `make check`: it runs `golangci-lint run` and `go test ./...` (`Makefile:7`, `Makefile:4`), so it may exceed short tool timeouts. The writer task runs `make apidiff` and `make -n check` (T5, T11 lines 1-2). The full `make check` (`exit=0`) is run by the orchestrator's factory-gate with a long timeout. Do not treat a timeout here as a failure of the change; report it.
  - Host step (orchestrator): the `apidiff` CI job passes on the pushed branch (R5 host step).

- [ ] **T12: Records at completion (orchestrator, not the writer)**
  - Budget: 30 min
  - Files: the checkpoint `docs/plans/052_runner_tool_hooks_v1_12_0_002_checkpoint.md` and the milestone checkbox `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:154` (`[ ] 002` becomes `[x] 002`). Both are Factory records excluded from R11's scope check.
  - Do not change the milestone entry beyond the checkbox (spec Rejected approaches).

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 one concrete pin in `Makefile`; no `@latest`; no second pin | T1.1, T3 (pin cross-check), T5, T11 |
| R2 `apidiff` target, `.PHONY`, `check: lint test apidiff`, `@` prefix | T1.1, T1.2, T1.3, T5, T11 |
| R3 tag check, `mktemp -d`, `git archive` + `tar`, status checks, no `git worktree` | T1.2, T8 (tag message), T10 (worktree registry), T11 |
| R4 `-incompatible`; fail on report or error; stdout-only gate | T1.3, T6 (removed identifier), T9 (unloadable package; corrupted export), T5 (pass) |
| R5 CI `apidiff` job with `fetch-depth: 0`; other jobs unchanged | T2, T11 (grep checks), host step (T11 note) |
| R6 `go.mod` and `go.sum` unchanged | T5, T10, T11 |
| R7 temp directory removed on pass and fail | T1.2 (trap), T5, T6, T7, T8, T9, T11 |
| R8 `COMPATIBILITY.md` states the gate as active; no "planned" wording | T3, T11 |
| R9 one CHANGELOG bullet under `[Unreleased]` `### Added` | T4, T10, T11 |
| R10 case 1 unmodified tree passes (`make apidiff`, `make check`) | T5, T11 (`make apidiff` run; `make check` deferred to factory-gate) |
| R10 case 2 removed identifier fails with `- IsRuntimeError: removed` | T6 |
| R10 case 3 added identifier passes | T7 |
| R10 case 4 tool error on missing tag; shared tags untouched | T8 |
| R11 scope boundary: only four tracked files plus Factory records | T10, T11 |

Every R-ID R1-R11 is covered by at least one task. Every acceptance command in the spec's "Acceptance commands" block is in T11 (lines 1-14, in the spec's order). The R4 and R10 scratch checks are in T6-T9.

## Spec acceptance coverage

| Spec acceptance command | Task |
|---|---|
| `make -n check` | T5, T11 |
| `make apidiff; echo exit=$?` | T5, T11 |
| `make check; echo exit=$?` | T11 (orchestrator factory-gate runs it) |
| `rg -n "v0\.0\.0-" Makefile COMPATIBILITY.md` | T1.1, T3, T11 |
| `rg -n "@latest" Makefile` | T1.1, T11 |
| `rg -n 'git worktree' Makefile` | T1.2, T11 |
| `rg -n "fetch-depth" .github/workflows/go.yml` | T2, T11 |
| `rg -n "make apidiff" .github/workflows/go.yml` | T2, T11 |
| `rg -n -i "planned\|not active\|lands in" COMPATIBILITY.md` | T3, T11 |
| `git diff --exit-code 423a399 -- go.mod go.sum` | T5, T10, T11 |
| `git diff -U0 423a399 -- CHANGELOG.md \| rg -c ...` | T4, T10, T11 |
| `ls -d ... \| wc -l` (before and after each run) | T0, T5, T6-T9, T11 |
| `git diff --name-only 423a399 -- . ':(exclude)...'` | T10, T11 |
| `spec-lint` on the spec | T11 |
| `docs-lint --since origin/master .` | T11 |

## Parallel Work and Serial Dependencies

- Serial: T0 → T1.1 → T1.2 → T1.3 (all in `Makefile`).
- Parallel, independent of T1.x: T2 (`.github/workflows/go.yml`), T4 (`CHANGELOG.md`). T3 (`COMPATIBILITY.md`) is independent of T1.x for its own edits; its pin cross-check needs T1.1.
- Serial after T1.3, T2, T3, T4: T5.
- Parallel after T5: T6, T7, T8, T9 (each uses its own scratch clone).
- Serial after T6-T9: T10 → T11.
- T12 is orchestrator work after T11 and the host CI step.

## Phase 2 Gate Record

- Changed signatures: none. This milestone changes no `.go` file, so no exported signature changes.
- Callers of `IsRuntimeError` (for the R10 case 2 removal, which is probe-only and happens only in a scratch clone): `pkg/runner/errors_test.go:97`, `pkg/runner/errors_test.go:103`, `pkg/runner/run_test.go:1324`, `pkg/runner/run_test.go:1366`, `pkg/runner/run_test.go:1418`, `pkg/runner/run_test.go:1453` (from `rg -n IsRuntimeError --glob '*.go'` at HEAD `3920c71`). No caller update task is needed; none of these files is changed.
- Extreme acceptance probes (inputs and expected results):
  - Empty report (no incompatible change): T5 and T7. Expected: exit 0, empty stdout.
  - Removed exported identifier: T6. Expected: non-zero exit, `- IsRuntimeError: removed` on stdout.
  - Added exported identifier: T7. Expected: exit 0, empty stdout.
  - Unloadable package path (`github.com/jrswab/doesnotexist`): T9 case A. Expected: non-zero exit.
  - Corrupted export file (`garbage`): T9 case B. Expected: non-zero exit.
  - Missing tag `v1.11.0`: T8. Expected: non-zero exit with a message naming the tag.
  - Long identifiers or strings: not applicable. The gate's inputs are a package path, a tag name, and an export file; no identifier length is a parameter of this change.
  - Probe evidence recorded by the spec (not re-run by this writer): pin resolution, base export exit 0, compare exit 0 with empty output, removal and addition probes, apidiff source line references. These are re-checked in T0 (pin) and T5-T9 (behavior).
- `go install` or `GOPROXY=` commands: none. `go-toolchain-check` is not run; the pre-flight rule is not triggered.
- Checklist audit: `checklist-test-gate-audit` is not run by this writer. Only the orchestrator's `spec-lint` applies.

## Blockers

- **Blockers: none.** The spec is unambiguous for scope. Every decision (D3, D-pin, D-fail, D-base-tree, D-script, D-check, D-ci, D-doc, D-changelog, D-factory-gate, D-base-tag) is settled in the spec.

## Assumptions

1. The pin variable is named `APIDIFF_MODULE`. R1 allows either the module path with version or the version alone; the checklist uses the full module path.
2. T0 (`go list -m -json ...`) and the scratch-clone precondition (`git rev-parse --verify 'v1.11.0^{commit}'` in the clone) are added checks. They come from the spec's probe evidence ("the implementation must re-verify the pin") and the R3 tag rule. They are preconditions, not new acceptance.
3. The R4 corrupted-export probe requires the recipe to name the mktemp variable `tmp` (T1.2 step 2). Otherwise the spec's scratch edit does not apply.
4. Scratch-clone probes follow the spec's R10 exactly. They copy the four changed files into the clone, which the spec requires.
5. Line citations were checked at HEAD `3920c71`: `Makefile:1,4,7,9`; `.github/workflows/go.yml:9,11,14,19,24,27,32,39,42,47,50`; `COMPATIBILITY.md:37,39-42,41`; `CHANGELOG.md:3,8,10,11,13`; `pkg/runner/errors.go:62,63,66`; `go.mod:3,5`; caller lines listed in the Phase 2 Gate Record.
6. The writer does not run `make check` in full (per the orchestrator's instruction); T11 says so.

## Limitations

- The pin is verified by the spec's probe in this session, not by a CI run. The first `make apidiff` run and the CI `apidiff` job are the first real checks.
- A cold module cache needs network for `go run` of the pinned apidiff (T0, T5, T6-T9, T11).
- `make check` may exceed short tool timeouts (lint and full test run). The orchestrator's factory-gate runs it.
