# Implement Milestone 000: Clear govulncheck findings (toolchain and x/net)

## Section 1: Context Summary

Milestone 000 of the v1.12.0 runner tool-hooks release clears the 37 govulncheck findings that `~/.agents/scripts/go-toolchain-check ./` reports at baseline: 36 in the Go standard library at go1.25.0 and 6 in `golang.org/x/net@v0.55.0`. The fix pins `toolchain go1.26.10`, raises the `go` directive to `go 1.26.0` (x/net v0.62.0 requires it), and upgrades `golang.org/x/net` to v0.62.0, the first fixed version for GO-2026-6860. Because the `go` directive rises, the CI setup-go steps, the Dockerfile base image, and the Go-version mentions in the docs move with it. The change is pre-policy and recorded in `COMPATIBILITY.md` by milestone 001. No `.go` file changes and no public API changes. Source: `docs/plans/052_runner_tool_hooks_v1_12_0_000_spec.md` (Sections 1 and 2) and `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md` (entry 000).

---

## Section 2: Implementation Checklist

Commit is not authorized. Do not push. Do not change files outside the list in each task.

### Phase 1: Module files (serial: T1 → T2 → T3)

- [x] **T1: `go.mod` — pin the `go` directive and toolchain (R1)**
  - File: `go.mod:3` change `go 1.25.0` to `go 1.26.0`. Add the line `toolchain go1.26.10` directly after it.
  - Do not edit `go.sum` or the `require` block in this task.
  - Verify:
    - `go mod edit -json | rg '"Go"|"Toolchain"'` prints `"Go": "1.26.0"` and `"Toolchain": "go1.26.10"`.
  - Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it. This milestone changes no .go files, so the derived package list is expected to be empty. Derivation: `git diff --name-only -- '*.go'` must print nothing before commit; if it prints a file, derive its package with `go list -f '{{.ImportPath}}' ./path/to/dir` and run `go test -race` on that package only.

- [x] **T2: `go.mod` and `go.sum` — upgrade `golang.org/x/net` to v0.62.0 (R2)**
  - Depends on: T1.
  - Command (run in repository root): `GOTOOLCHAIN=go1.26.10 go get golang.org/x/net@v0.62.0`, then `GOTOOLCHAIN=go1.26.10 go mod tidy`.
  - Expected transitive change: `golang.org/x/sys` v0.45.0 → v0.49.0 (indirect). Accept any other bump tidy requires. Do not pin indirect modules by hand.
  - Files touched: `go.mod:9` (x/net line), `go.sum` (the `golang.org/x/net v0.55.0` lines at `go.sum:26-27` are replaced by v0.62.0 lines), and whatever indirect lines tidy changes.
  - Verify:
    - `go list -m golang.org/x/net` prints `golang.org/x/net v0.62.0`.
    - `rg -n 'golang.org/x/net v0.55.0' go.mod go.sum` prints no output.
    - `GOTOOLCHAIN=go1.26.10 go mod verify` exits 0.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T3: Govulncheck is clean (R4)**
  - Depends on: T1, T2.
  - No file edits. Verification only.
  - Verify: `~/.agents/scripts/go-toolchain-check ./` exits 0 and reports no findings. Record the output and exit status in the checkpoint notes. If it exits 3 or lists any GO id, stop and report the GO id to the orchestrator. Do not add a suppression.
  - Per-task gate: as in T1 (expected derived package list is empty).

### Phase 2: CI workflows (parallel with Phase 1 after T1 is saved; each task is one file)

These tasks change only YAML `go-version` lines. They are independent of Phase 1 at the file level. Run them after T1 so the directive they read is the new one.

- [x] **T4: `.github/workflows/go.yml` — read Go version from `go.mod` (R3, part 1)**
  - Replace `go-version: '1.24'` with `go-version-file: go.mod` at the three setup-go steps: `.github/workflows/go.yml:19`, `:32`, `:47`.
  - Verify:
    - `rg -n "go-version" .github/workflows/go.yml` shows only `go-version-file: go.mod` at those three lines.
    - `ruby -ryaml -e 'YAML.load_file(".github/workflows/go.yml"); puts "ok"'` prints `ok`. If ruby is absent, use `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/go.yml')); print('ok')"`.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T5: `.github/workflows/smoke-test.yml` — read Go version from `go.mod` (R3, part 2)**
  - Replace `go-version: '1.24'` with `go-version-file: go.mod` at the five setup-go steps: `.github/workflows/smoke-test.yml:19`, `:70`, `:105`, `:164`, `:212`.
  - Verify:
    - `rg -n "go-version" .github/workflows/smoke-test.yml` shows only `go-version-file: go.mod` at those five lines.
    - YAML parse check as in T4 for `.github/workflows/smoke-test.yml`.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T6: `.github/workflows/release.yml` — replace `stable` with `go.mod` (R3, part 3)**
  - Replace `go-version: stable` with `go-version-file: go.mod` at `.github/workflows/release.yml:24`.
  - Verify:
    - `rg -n "go-version" .github/workflows/release.yml` shows only `go-version-file: go.mod`.
    - YAML parse check as in T4 for `.github/workflows/release.yml`.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T7: `.github/workflows/` — no Go-version pin remains (R3 acceptance)**
  - Depends on: T4, T5, T6.
  - No file edits. Verification only.
  - Verify: `rg -n "1\.24" .github/workflows/` prints no output. `rg -n "go-version:" .github/workflows/` prints no output.
  - Per-task gate: as in T1 (expected derived package list is empty).

### Phase 3: Dockerfile (parallel with Phase 2)

- [x] **T8: `Dockerfile` — base image to Go 1.26 (R6, edit only)**
  - Change `Dockerfile:2` from `FROM golang:1.25-alpine AS build` to `FROM golang:1.26-alpine AS build`. Lines 7 (`go mod download`) and 12 (`go build`) are unchanged.
  - Verify:
    - `rg -n "golang:" Dockerfile` prints `2:FROM golang:1.26-alpine AS build`.
    - `rg -n "go mod download|go build" Dockerfile` still shows lines 7 and 12.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T9: Docker host build (R6, host step; not run in the sandbox)**
  - Depends on: T1, T2, T8.
  - Owner: Jaron runs `docker build .` on a host with Docker. The orchestrator verifies the result. Not delegated to a subagent. The sandbox masks the Docker socket.
  - Expected: exit 0, and the build reports go1.26.10 (through the base image or the `GOTOOLCHAIN=auto` download). If the download fails, the build fails with a toolchain error; report it, do not add `GOTOOLCHAIN=local`.
  - Verify: the orchestrator records the exit status and the reported Go version.

### Phase 4: Documentation (parallel with Phases 2 and 3)

- [x] **T10: `AGENTS.md` and `README.md` — Go-version mentions (R7, part 1)**
  - `AGENTS.md:215`: `golang:1.25-alpine` → `golang:1.26-alpine`. No other `AGENTS.md` line changes.
  - `README.md:40`: `Requires Go 1.25+.` → `Requires Go 1.26+.`
  - `README.md:50`: change `1.25` to `1.26` in that sentence only.
  - Verify: `rg -n "1\.25" AGENTS.md README.md` prints no output. `rg -n "1\.26" AGENTS.md README.md` shows lines 215 (AGENTS.md), 40 and 50 (README.md).
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T11: `CONTRIBUTING.md` and `docs/src/getting-started/installation.md` — Go-version mentions (R7, part 2)**
  - `CONTRIBUTING.md:19`: `- Go 1.25.0 or later` → `- Go 1.26.0 or later`. No other line changes.
  - `docs/src/getting-started/installation.md:3`: `Requires **Go 1.25+**.` → `Requires **Go 1.26+**.`
  - `docs/src/getting-started/installation.md:32`: change `older than 1.25` to `older than 1.26` in that sentence only.
  - Verify: `rg -n "1\.25" CONTRIBUTING.md docs/src/getting-started/installation.md` prints no output.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T12: Repository-wide Go-version sweep (R7 acceptance)**
  - Depends on: T10, T11, T8, T4, T5, T6.
  - No file edits. Verification only.
  - Verify:
    - `rg -n "1\.25" AGENTS.md README.md CONTRIBUTING.md Dockerfile docs/src/getting-started/installation.md` prints no output.
    - `rg -n '1\.25' --glob '!docs/plans/**' --glob '!CHANGELOG.md' --glob '!go.sum' .` prints only `internal/hostcheck/hostcheck_test.go:119` (`172.31.255.255`, an IP address, not a Go version).
  - Per-task gate: as in T1 (expected derived package list is empty).

### Phase 5: Build, lint, and no-API-change checks (after Phases 1–4)

- [x] **T13: Build and lint (R5, part 1)**
  - Depends on: T1, T2, T3.
  - Verify:
    - `GOTOOLCHAIN=go1.26.10 go build ./...` exits 0.
    - `golangci-lint run` reports `0 issues.`
  - Per-task gate: as in T1 (expected derived package list is empty).

- [x] **T14: No public API or behavior change (R8)**
  - Depends on: T1–T12.
  - No file edits. Verification only.
  - Verify: `git diff --stat -- pkg/ internal/ cmd/` prints no output. The only module changes are the `go` directive, toolchain, `golang.org/x/net` v0.62.0, and indirect changes produced by `go get`/`go mod tidy`.
  - Per-task gate: as in T1 (expected derived package list is empty).

- [ ] **T15: CI verification after push (R3 acceptance, R5 CI; not run in the sandbox)**
  - Depends on: T1–T14, and the branch pushed. Pushing requires Jaron's approval under the standing rules; this milestone does not authorize it.
  - Verify: the setup-go jobs in `go.yml` and `smoke-test.yml` resolve Go 1.26.x and pass, and the lint action passes. Record the run URL and result. The CI run is the verification for `actions/setup-go` and `golangci-lint-action@v7` behavior, which the spec did not check in the sandbox.

### Completion gate (Factory, not per task)

- Gate step (Factory, R5 test acceptance): run `XDG_CACHE_HOME=$(mktemp -d) GOTOOLCHAIN=go1.26.10 go test -race ./...` from the repository root, so the cache is writable. Pass condition: exit 0 and every package reports `ok` with no `FAIL` lines. Record the exact command and its result (exit code and per-package status) in the completion report. The `cmd` tests `TestRun_AutoGeneratedArtifactsCleanedUp` and `TestRun_KeepArtifactsPreservesDirectory` fail with `read-only file system` under `~/.cache/axe/artifacts` when the cache is unwritable; that failure is not an accepted result. If any package fails, the gate fails and the milestone is not complete.

---

## Requirement-to-Task Mapping

| Requirement | Tasks |
|---|---|
| R1 `go.mod` go 1.26.0 + toolchain go1.26.10 | T1 |
| R2 x/net v0.62.0 (go.mod, go.sum, tidy) | T2 |
| R3 setup-go `go-version-file: go.mod` (go.yml, smoke-test.yml, release.yml) | T4, T5, T6, T7, T15 |
| R4 govulncheck clean | T3 |
| R5 build, test, lint | T13, Completion gate (test), T15 (CI) |
| R6 Dockerfile base image and host build | T8, T9 |
| R7 documentation Go-version mentions | T10, T11, T12 |
| R8 no public API or behavior change | T14 |

## Parallel Work and Serial Dependencies

- Serial: T1 → T2 → T3. Govulncheck (T3) and build/lint (T13) run only after the final `go.sum`.
- Parallel after T1: T4, T5, T6 (distinct workflow files), T8 (Dockerfile), and T10, T11 (distinct doc files). Each task is one file or one file group, so no two parallel tasks edit the same file.
- T7, T12, T14 are checks that run after their listed dependencies.
- T9 (host Docker build) waits for T1, T2, T8. T15 (CI) waits for the push.

## Phase 2 Gate Record

- Changed signatures: none. This milestone changes no `.go` file, so there are no caller lists to build and no caller-update tasks.
- Extreme acceptance probes: N/A. The acceptance inputs are version strings and file contents, checked by the exact commands in T1–T12. No code path takes an input that needs boundary probes.
- `go install` or `GOPROXY=` commands: none in this checklist. The `GOTOOLCHAIN=go1.26.10` prefix on `go get`, `go mod tidy`, `go mod verify`, and `go build` selects the toolchain and does not set `GOPROXY`. `go-toolchain-check` is run as a verification command, not `go install`. The pre-flight rule is therefore not triggered.
- Baseline evidence: `go-toolchain-check ./` exits 1 at the spec's citation HEAD with 37 findings (`docs/plans/052_runner_tool_hooks_v1_12_0_000_checkpoint.md:55`). The current HEAD is `543bf32`, which differs from the spec's `be94237`; every cited line in this checklist (`go.mod:3`, `go.mod:9`, `go.sum:26-27`, `Dockerfile:2`, the workflow lines, `AGENTS.md:215`, `README.md:40,50`, `CONTRIBUTING.md:19`, `docs/src/getting-started/installation.md:3,32`) was re-checked at HEAD `543bf32` and matches.

## Blockers

- None. The spec resolved B1–B4 (`docs/plans/052_runner_tool_hooks_v1_12_0_000_spec.md`, Decisions). The CI and host Docker steps are verified outside the sandbox and are listed as tasks T9 and T15, not as blockers.
