# 044 — Artifact Dir Workdir Fix: Implementation Guide

**Spec:** `docs/plans/044_artifact_dir_workdir_fix_spec.md`
**Issue:** https://github.com/jrswab/axe/issues/56
**Status:** Implement

---

## Section 1: Context Summary

When `[artifacts] dir` in agent TOML is set to a relative path (e.g., `dir = "output"`), axe resolves it against the process CWD rather than the agent's resolved `--workdir`. In read-only CWD environments like AWS Lambda (`/var/task`), this causes a fatal `mkdir` error. The fix is confined to `cmd/run.go` lines 162–221 where the artifact directory resolution lives. The resolved `workdir` variable (line 157) is already available at that point but is never used when resolving artifact paths. The same issue affects the `--artifact-dir` flag, which is used raw without `~`/`$VAR` expansion or workdir joining. No changes to `internal/resolve`, `internal/agent`, or any other package are needed.

---

## Section 2: Implementation Checklist

### Task 1: Fix `--artifact-dir` flag resolution (expansion + workdir join)

**Can be done in parallel with Task 2.**

- [x] `cmd/run.go`: In the `runAgent()` function, replace the `--artifact-dir` flag branch (lines 175–176):

  **Before:**
  ```go
  if flagArtifactDir != "" {
      artifactDir = flagArtifactDir
  }
  ```

  **After:** Pass `flagArtifactDir` through `resolve.Workdir("", flagArtifactDir)` for `~`/`$VAR` expansion, then join with `workdir` if the result is relative:
  ```go
  if flagArtifactDir != "" {
      expanded, expandErr := resolve.Workdir("", flagArtifactDir)
      if expandErr != nil {
          return &ExitError{Code: 2, Err: fmt.Errorf("failed to resolve --artifact-dir: %w", expandErr)}
      }
      if !filepath.IsAbs(expanded) {
          expanded = filepath.Join(workdir, expanded)
      }
      artifactDir = expanded
  }
  ```

### Task 2: Fix TOML `artifacts.dir` resolution (workdir join for relative paths)

**Can be done in parallel with Task 1.**

- [x] `cmd/run.go`: In the `runAgent()` function, update the TOML `cfg.Artifacts.Dir` branch (lines 177–182). After the existing `resolve.Workdir("", cfg.Artifacts.Dir)` call, add a `filepath.IsAbs` check and join with `workdir` if relative:

  **Before:**
  ```go
  } else if cfg.Artifacts.Enabled && cfg.Artifacts.Dir != "" {
      expanded, expandErr := resolve.Workdir("", cfg.Artifacts.Dir)
      if expandErr != nil {
          return &ExitError{Code: 2, Err: fmt.Errorf("failed to resolve artifacts.dir: %w", expandErr)}
      }
      artifactDir = expanded
  }
  ```

  **After:**
  ```go
  } else if cfg.Artifacts.Enabled && cfg.Artifacts.Dir != "" {
      expanded, expandErr := resolve.Workdir("", cfg.Artifacts.Dir)
      if expandErr != nil {
          return &ExitError{Code: 2, Err: fmt.Errorf("failed to resolve artifacts.dir: %w", expandErr)}
      }
      if !filepath.IsAbs(expanded) {
          expanded = filepath.Join(workdir, expanded)
      }
      artifactDir = expanded
  }
  ```

### Task 3: Add tests for relative artifact dir resolution

**Depends on Tasks 1 and 2 being complete.**

- [x] `cmd/run_test.go`: Add a new table-driven test function `TestRun_ArtifactDirWorkdirResolution` near the existing `TestRun_ArtifactEnvVar` (after line 3122). Follow the same pattern: `resetRunCmd(t)`, `startMockAnthropicServer(t)`, `setupRunTestAgent(t, ...)`, set env vars, execute `rootCmd`, assert directory existence and path correctness.

  **Required test cases (5 cases in one table):**

  1. **`relative TOML artifacts.dir resolves against --workdir`**
     - TOML: `[artifacts] enabled = true` with `dir = "output"`
     - Args: `["run", "artifact-workdir-agent", "--workdir", <tmpWorkdir>]`
     - Assert: directory `<tmpWorkdir>/output` exists on disk

  2. **`absolute TOML artifacts.dir unaffected by --workdir`**
     - TOML: `[artifacts] enabled = true` with `dir = "<absoluteTmpDir>"`
     - Args: `["run", "artifact-workdir-agent", "--workdir", <otherTmpDir>]`
     - Assert: directory `<absoluteTmpDir>` exists on disk (NOT `<otherTmpDir>/<absoluteTmpDir>`)

  3. **`relative --artifact-dir flag resolves against --workdir`**
     - TOML: no `[artifacts]` block
     - Args: `["run", "artifact-workdir-agent", "--artifact-dir", "output", "--workdir", <tmpWorkdir>]`
     - Assert: directory `<tmpWorkdir>/output` exists on disk

  4. **`absolute --artifact-dir flag unaffected by --workdir`**
     - TOML: no `[artifacts]` block
     - Args: `["run", "artifact-workdir-agent", "--artifact-dir", <absoluteTmpDir>, "--workdir", <otherTmpDir>]`
     - Assert: directory `<absoluteTmpDir>` exists on disk

  5. **`relative TOML artifacts.dir with no --workdir resolves against CWD`**
     - TOML: `[artifacts] enabled = true` with `dir = "output"`
     - Args: `["run", "artifact-workdir-agent"]` (no `--workdir` flag)
     - Before execute: `os.Chdir(<tmpCwdDir>)` and defer restore
     - Assert: directory `<tmpCwdDir>/output` exists on disk

  **Each test case must:**
  - Use `t.TempDir()` for all temporary directories
  - Use agent name `"artifact-workdir-agent"` with model `"anthropic/claude-sonnet-4-20250514"`
  - Call `resetRunCmd(t)` at the start
  - Set `ANTHROPIC_API_KEY` and `AXE_ANTHROPIC_BASE_URL` env vars
  - Unset `AXE_ARTIFACT_DIR` before execute
  - Assert the expected directory exists via `os.Stat`
  - Clean up `AXE_ARTIFACT_DIR` after execute

### Task 4: Run tests and verify no regressions

**Depends on Task 3.**

- [x] Run `go test ./cmd/... -run TestRun_Artifact -v` to verify all new and existing artifact tests pass.
- [x] Run `go test ./cmd/... -count=1` to verify no regressions in the full `cmd` package test suite.
- [x] Run `go vet ./...` and confirm no warnings.
