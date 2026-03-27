# ISS-56 — Fix: Relative artifacts.dir Resolved Against --workdir, Not Process CWD

**Milestone document:** N/A (standalone bug fix)
**Issue:** https://github.com/jrswab/axe/issues/56
**Status:** Spec

---

## Section 1: Context & Constraints

### Codebase Structure Relevant to This Fix

- **`cmd/run.go` — artifact directory resolution (lines 162–221):** The artifact directory is resolved in a three-way priority chain:
  1. `--artifact-dir` flag (line 175–176): used raw, no expansion, no workdir join
  2. TOML `cfg.Artifacts.Dir` (line 177–182): passed through `resolve.Workdir("", cfg.Artifacts.Dir)` for tilde/env expansion, but NOT joined with workdir if relative
  3. Auto-generated XDG cache path (line 183–196): always absolute

- **`cmd/run.go` — workdir resolution (lines 155–160):** `workdir` is resolved via `resolve.Workdir(flagWorkdir, cfg.Workdir)` before the artifact block. The resolved `workdir` variable is available but is never used when resolving `cfg.Artifacts.Dir`.

- **`internal/resolve/resolve.go` — `Workdir` function (lines 45–62):** Performs only `~` and `$VAR` expansion via `ExpandPath`. A relative path like `"output"` passes through unchanged. It does NOT make paths absolute.

- **`internal/resolve/resolve.go` — `ExpandPath` function (lines 16–40):** Handles `~/...` tilde expansion and `os.ExpandEnv` for `$VAR`/`${VAR}`. Does not resolve relative paths against any base directory.

- **`internal/agent/agent.go` — `ArtifactsConfig` struct (lines 52–56):**
  ```go
  type ArtifactsConfig struct {
      Enabled bool   `toml:"enabled"`
      Dir     string `toml:"dir"`
  }
  ```

- **`internal/agent/agent.go` — `Validate()` (lines 173–178):** Two existing validation rules for artifacts:
  1. `artifacts.dir` set but `artifacts.enabled` is false → validation error
  2. `artifacts.dir` contains `..` → validation error

- **`cmd/run_test.go` — existing artifact tests (lines 3025–3083):** `TestRun_ArtifactFlagsExist` and `TestRun_ArtifactEnvVar` with three table-driven cases. No existing test covers relative path resolution against workdir.

- **`os.MkdirAll(artifactDir, 0o755)` at line 200:** This is where the bug manifests. When `artifactDir` is a relative path like `"output"`, `os.MkdirAll` resolves it against the process CWD — not the agent's resolved workdir.

### Decisions Already Made (from spec 042)

1. **Resolution order for artifact directory:** flag > TOML dir > auto-generated. This order is preserved; only the path joining behavior changes.
2. **`--artifact-dir` flag activates the system even without `enabled = true` in TOML.** This behavior is unchanged.
3. **Persistent directories are never cleaned up.** This behavior is unchanged.
4. **`~` and `$VAR` expansion applies to `artifacts.dir`.** This behavior is preserved; the fix adds workdir-relative joining on top of it.

### Approaches Ruled Out

- **Changing `resolve.Workdir` to accept a base directory:** `resolve.Workdir` is a general-purpose function used for the agent's own workdir resolution. Changing its signature would affect all callers. The fix belongs in `cmd/run.go` at the call site, not in `resolve`.
- **Validating that `artifacts.dir` is absolute in `agent.Validate()`:** Would be a breaking change for users who intentionally use relative paths (e.g., `dir = "output"` to mean "output/ inside my workdir"). The correct behavior is to resolve relative paths against workdir, not reject them.
- **Resolving `--artifact-dir` flag against workdir:** The flag is a direct user invocation argument. Users passing `--artifact-dir /tmp/my-dir` expect that exact path. However, for consistency, if the flag value is relative, it should also resolve against workdir (same rule as TOML). This is a secondary fix in the same change.

### Constraints and Assumptions

- **Backward compatibility:** Any user currently passing an absolute path for `artifacts.dir` (TOML or flag) must see zero behavior change. The `filepath.IsAbs` check ensures this.
- **Workdir is always resolved before artifact dir.** The `workdir` variable at line 157 is always set (falls back to `os.Getwd()` if neither flag nor TOML is set). It is always available when the artifact block runs.
- **The fix is purely in `cmd/run.go`.** No changes to `internal/resolve`, `internal/agent`, or any other package are required.
- **Tests are in `cmd/run_test.go`.** New test cases are added to the existing `TestRun_ArtifactEnvVar` table or as a new table-driven test in the same file.

### Open Questions Resolved

- **Q: Should a relative `--artifact-dir` flag also resolve against workdir?** Yes. Consistency demands it. A user passing `--artifact-dir output` with `--workdir /tmp/abc` should get `/tmp/abc/output`, not `/var/task/output` (process CWD).
- **Q: What if workdir itself is relative?** `resolve.Workdir` always returns an absolute path (it falls back to `os.Getwd()` which returns an absolute path). So `filepath.Join(workdir, expanded)` always produces an absolute path.

---

## Section 2: Requirements

### 2.1 Behavior: Relative `artifacts.dir` in TOML

When `cfg.Artifacts.Dir` is set to a relative path (after `~` and `$VAR` expansion), it must be resolved against the agent's resolved `workdir`.

**Inputs:**
- `cfg.Artifacts.Dir`: a non-empty string that, after expansion, is not an absolute path
- `workdir`: the already-resolved working directory (always absolute)

**Output:**
- `artifactDir` is set to `filepath.Join(workdir, expanded)`

**When `cfg.Artifacts.Dir` is already absolute after expansion:**
- `artifactDir` is set to the expanded value unchanged. No workdir join.

**When `cfg.Artifacts.Dir` is empty:**
- This branch is not reached (guarded by `cfg.Artifacts.Dir != ""`). No change.

### 2.2 Behavior: Relative `--artifact-dir` Flag

When the `--artifact-dir` flag value is a relative path (after `~` and `$VAR` expansion), it must be resolved against the agent's resolved `workdir`.

**Inputs:**
- `flagArtifactDir`: a non-empty string that, after expansion, is not an absolute path
- `workdir`: the already-resolved working directory (always absolute)

**Output:**
- `artifactDir` is set to `filepath.Join(workdir, expanded)`

**When `flagArtifactDir` is already absolute after expansion:**
- `artifactDir` is set to the expanded value unchanged. No workdir join.

**When `flagArtifactDir` is empty:**
- This branch is not reached (guarded by `flagArtifactDir != ""`). No change.

**Note:** The `--artifact-dir` flag value must be passed through `resolve.Workdir("", flagArtifactDir)` (or equivalent `ExpandPath`) before the `filepath.IsAbs` check, so that `~` and `$VAR` in the flag value are expanded. Currently the flag value is used raw without expansion — this is a secondary bug fixed in the same change.

### 2.3 Behavior: Auto-Generated Artifact Directory

The auto-generated XDG cache path (`$XDG_CACHE_HOME/axe/artifacts/<run-id>/`) is always absolute. No change to this branch.

### 2.4 Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| `dir = "output"`, `--workdir /tmp/abc` | `artifactDir` = `/tmp/abc/output` |
| `dir = "output"`, no `--workdir`, process CWD = `/app` | `artifactDir` = `/app/output` (workdir falls back to CWD) |
| `dir = "/absolute/path"`, `--workdir /tmp/abc` | `artifactDir` = `/absolute/path` (absolute, no join) |
| `dir = "~/artifacts"`, `--workdir /tmp/abc` | `~` expanded first → `/home/user/artifacts` (absolute after expansion, no join) |
| `dir = "$OUTDIR"` where `OUTDIR=/tmp/out`, `--workdir /tmp/abc` | Expanded to `/tmp/out` (absolute after expansion, no join) |
| `dir = "$OUTDIR"` where `OUTDIR=output`, `--workdir /tmp/abc` | Expanded to `output` (relative after expansion) → joined → `/tmp/abc/output` |
| `--artifact-dir output`, `--workdir /tmp/abc` | `artifactDir` = `/tmp/abc/output` |
| `--artifact-dir /absolute/path`, `--workdir /tmp/abc` | `artifactDir` = `/absolute/path` (absolute, no join) |
| `--artifact-dir ~/artifacts`, `--workdir /tmp/abc` | `~` expanded → absolute → no join |
| No `[artifacts]` config, no flag | Artifact system inactive. Zero behavior change. |
| `artifacts.enabled = true`, no `dir`, no flag | Auto-generated path. No change. |
| `dir` contains `..` | Rejected by existing `Validate()` before reaching resolution. No change needed here. |

### 2.5 Tests

New test cases must be added to `cmd/run_test.go` covering the scenarios in 2.4. The tests must use the existing table-driven pattern in `TestRun_ArtifactEnvVar` or a new parallel table-driven test.

**Required test cases (can be implemented in parallel with each other):**

1. **Relative TOML `artifacts.dir` resolves against `--workdir`:** Agent TOML has `[artifacts] enabled=true dir="output"`, invocation uses `--workdir <tmpdir>`. Assert that the artifact directory created is `<tmpdir>/output`.

2. **Absolute TOML `artifacts.dir` is unaffected by `--workdir`:** Agent TOML has `[artifacts] enabled=true dir="<absolute-tmpdir>"`, invocation uses `--workdir <other-tmpdir>`. Assert that the artifact directory created is `<absolute-tmpdir>` (not joined with workdir).

3. **Relative `--artifact-dir` flag resolves against `--workdir`:** No TOML artifacts config. Invocation uses `--artifact-dir output --workdir <tmpdir>`. Assert that the artifact directory created is `<tmpdir>/output`.

4. **Absolute `--artifact-dir` flag is unaffected by `--workdir`:** No TOML artifacts config. Invocation uses `--artifact-dir <absolute-tmpdir> --workdir <other-tmpdir>`. Assert that the artifact directory created is `<absolute-tmpdir>`.

5. **Relative TOML `artifacts.dir` with no `--workdir` flag resolves against process CWD:** Agent TOML has `[artifacts] enabled=true dir="output"`, no `--workdir` flag. Assert that the artifact directory created is `<process-CWD>/output`.

Each test case must:
- Use `t.TempDir()` for any temporary directories.
- Assert the directory exists on disk after the run.
- Assert `AXE_ARTIFACT_DIR` env var equals the expected resolved path.
- Clean up after itself (temp dirs via `t.TempDir()` are auto-cleaned).

### 2.6 No Changes Required

The following are explicitly out of scope for this fix:

- `internal/resolve/resolve.go` — no changes
- `internal/agent/agent.go` — no changes (existing `..` validation already covers traversal)
- `internal/tool/` — no changes
- Any provider implementation — no changes
- Documentation files — no changes
- The `042_artifact_management_spec.md` spec document — no retroactive changes; this spec supersedes the path resolution behavior described there for relative paths
