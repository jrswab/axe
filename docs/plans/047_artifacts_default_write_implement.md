# 047 — Artifact Default Write: Implementation Guide

**Spec:** `docs/plans/047_artifacts_default_write_spec.md`
**Parent milestone:** `docs/plans/042_artifact_management_spec.md`

---

## Section 1: Context Summary

When an agent has the artifact system active (`[artifacts] enabled = true` or `--artifact-dir`), the LLM must explicitly pass `artifact = "true"` in every `write_file` call for the file to land in the artifact directory. If omitted, the file goes to workdir — rarely the intended behavior. Users work around this by adding prompt instructions like "always set artifact = true", which is fragile and context-wasteful. This implementation adds a `default_write` boolean to the `[artifacts]` TOML block so agents can declare the default deterministically. The default only applies when the `artifact` argument is absent/empty AND the artifact system is active. Explicit `artifact` arguments always override the default. Backward compatibility is non-negotiable: omitting `default_write` or setting it to `false` produces identical behavior to today.

---

## Section 2: Implementation Checklist

### Group A — Agent Config (no dependencies, can start immediately)

- [x] **A1: Add `DefaultWrite` field to `ArtifactsConfig`**
  `internal/agent/agent.go` — `ArtifactsConfig` struct (line 96)
  Add `DefaultWrite bool \`toml:"default_write"\`` as the third field after `Dir`.

- [x] **A2: Update `Scaffold()` template**
  `internal/agent/agent.go` — `Scaffold()` function (line 449)
  Add `# default_write = false\n` after `# dir = ""\n` inside the `[artifacts]` comment block.

- [x] **A3: Test that `Scaffold()` includes `default_write`**
  `internal/agent/agent_test.go` — `TestScaffold_IncludesArtifactsConfig` (line 1810)
  Add `# default_write = false` to the `checks` slice so the test asserts the new line is present.

- [x] **A4: Test TOML round-trip parsing of `default_write`**
  `internal/agent/agent_test.go` — add new test `TestLoad_ArtifactsDefaultWrite`
  Write a temp TOML file with `[artifacts] enabled = true, default_write = true`, call `loadFromPath()`, assert `cfg.Artifacts.DefaultWrite == true`. Also test the zero-value case (field omitted → `false`).

### Group B — Tool Execution Context (no dependencies, can start immediately, parallel with Group A)

- [x] **B1: Add `DefaultArtifactWrite` to `ExecContext`**
  `internal/tool/registry.go` — `ExecContext` struct (line 14)
  Add `DefaultArtifactWrite bool` as the last field.

- [x] **B2: Add `DefaultArtifactWrite` to `ExecuteOptions`**
  `internal/tool/tool.go` — `ExecuteOptions` struct (line 29)
  Add `DefaultArtifactWrite bool` as the last field.

- [x] **B3: Thread `DefaultArtifactWrite` through sub-agent `dispatchToolCall`**
  `internal/tool/tool.go` — `dispatchToolCall()` function (line 447)
  Add `DefaultArtifactWrite: opts.DefaultArtifactWrite` to the `ExecContext` literal (line 456).

### Group C — Write File Logic (depends on A1 + B1)

- [x] **C1: Implement default_write resolution in `writeFileExecute`**
  `internal/tool/write_file.go` — `writeFileExecute()` function (line 101–104)
  Replace the current single `if`:
  ```go
  if strings.EqualFold(call.Arguments["artifact"], "true") {
      return writeFileArtifact(ctx, call, ec, path)
  }
  ```
  with:
  ```go
  artifactArg := call.Arguments["artifact"]
  useArtifact := strings.EqualFold(artifactArg, "true")
  if !useArtifact && artifactArg == "" && ec.DefaultArtifactWrite {
      useArtifact = true
  }
  if useArtifact {
      return writeFileArtifact(ctx, call, ec, path)
  }
  ```
  This preserves all existing behavior: explicit `"true"` → artifact, explicit non-empty non-`"true"` → workdir. The new path only fires when the arg is absent/empty AND the default is on.

- [x] **C2: Test `DefaultArtifactWrite = true` with absent `artifact` arg writes to artifact dir**
  `internal/tool/write_file_test.go` — add test `TestWriteFile_DefaultArtifactWrite_True`
  Call `writeFileExecute` with `ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir, ArtifactTracker: tracker, DefaultArtifactWrite: true}` and no `artifact` key in arguments. Assert file lands in `artifactDir`, not `tmpdir`. Assert result content contains `"(artifact)"`.

- [x] **C3: Test `DefaultArtifactWrite = true` with explicit `artifact = "false"` writes to workdir**
  `internal/tool/write_file_test.go` — add test `TestWriteFile_DefaultArtifactWrite_ExplicitFalse`
  Same setup as C2 but set `Arguments["artifact"] = "false"`. Assert file lands in `tmpdir`, not `artifactDir`.

- [x] **C4: Test `DefaultArtifactWrite = false` with absent `artifact` arg writes to workdir**
  `internal/tool/write_file_test.go` — add test `TestWriteFile_DefaultArtifactWrite_False`
  Call with `ExecContext{Workdir: tmpdir, ArtifactDir: artifactDir, DefaultArtifactWrite: false}` and no `artifact` key. Assert file lands in `tmpdir` (existing behavior preserved).

- [x] **C5: Test `DefaultArtifactWrite = true` with `artifact = "no"` writes to workdir**
  `internal/tool/write_file_test.go` — add test `TestWriteFile_DefaultArtifactWrite_NonTrueValue`
  Call with `DefaultArtifactWrite: true` and `Arguments["artifact"] = "no"`. Assert file lands in workdir (non-empty, non-"true" value overrides default).

- [x] **C6: Test `DefaultArtifactWrite = true` but no artifact dir returns error**
  `internal/tool/write_file_test.go` — add test `TestWriteFile_DefaultArtifactWrite_NoArtifactDir`
  Call with `ExecContext{Workdir: tmpdir, DefaultArtifactWrite: true}` (no `ArtifactDir` set). Assert `IsError == true` and content contains `"artifact directory not configured"`.

### Group D — Wiring in cmd/run.go (depends on A1 + B1 + B2 + C1)

- [x] **D1: Pass `DefaultArtifactWrite` into `ExecContext` in `dispatchToolCall`**
  `cmd/run.go` — `dispatchToolCall()` function (line 987)
  Add `DefaultArtifactWrite` parameter to the function signature and pass it into the `tool.ExecContext` literal.

- [x] **D2: Pass `DefaultArtifactWrite` into `ExecuteOptions` in `executeToolCalls`**
  `cmd/run.go` — `executeToolCalls()` function (line 908)
  Add `cfg.Artifacts.DefaultWrite` as `DefaultArtifactWrite` in the `tool.ExecuteOptions` literal (line 911).

- [x] **D3: Pass `DefaultArtifactWrite` to all `dispatchToolCall` call sites in `executeToolCalls`**
  `cmd/run.go` — `executeToolCalls()` function (lines 935, 939, 955, 959)
  Add `cfg.Artifacts.DefaultWrite` as the new argument to each `dispatchToolCall()` call.

- [x] **D4: Pass `DefaultArtifactWrite` to `dispatchToolCall` call site in conversation loop**
  `cmd/run.go` — conversation loop (line 682)
  The `executeToolCalls` call at line 682 does not need a signature change — `executeToolCalls` reads from `cfg.Artifacts.DefaultWrite` internally (D2). Verify no other `dispatchToolCall` call sites exist outside `executeToolCalls`.

- [x] **D5: Verify all `dispatchToolCall` callers compile**
  Run `go build ./...` after D1–D4 to confirm all call sites are updated.

### Group E — Integration Tests (depends on Group D)

- [x] **E1: Integration test — agent with `default_write = true`, `write_file` without `artifact` arg**
  `cmd/run_integration_test.go` — add test `TestIntegration_DefaultArtifactWrite`
  Create a test agent TOML with `[artifacts] enabled = true, default_write = true`. Use `--json` flag. The agent's system prompt instructs it to call `write_file` with only `path` and `content` (no `artifact` arg). Assert the file appears in the artifact directory and the JSON output includes the artifact entry.

- [x] **E2: Integration test — `default_write = true` with `enabled = false` but `--artifact-dir` flag**
  `cmd/run_integration_test.go` — add test `TestIntegration_DefaultArtifactWrite_FlagOverride`
  Agent TOML has `[artifacts] enabled = false, default_write = true`. Run with `--artifact-dir <tmpdir>`. The flag activates the artifact system. Assert `write_file` without `artifact` arg writes to the artifact directory.

### Group F — Documentation (can proceed in parallel with Groups D and E)

- [x] **F1: Update `AGENTS.md` with `default_write` field**
  `AGENTS.md` — `[artifacts]` TOML example section
  Add `default_write = true` to the example and a one-line description of the field.

- [x] **F2: Update spec 042 with `default_write` field**
  `docs/plans/042_artifact_management_spec.md` — Section 2.1 (Agent TOML Configuration)
  Add `default_write` to the fields table and update the TOML examples.

### Parallelism Summary

```
Group A ─┐
         ├──→ Group C ──→ Group D ──→ Group E
Group B ─┘                      ──→ Group F (parallel with D+E)
```

- **A** and **B** can proceed in parallel immediately.
- **C** depends on A1 + B1 (new field on the struct it reads from).
- **D** depends on A1 + B1 + B2 + C1 (all structs and logic must exist before wiring).
- **E** depends on D (needs the full pipeline working).
- **F** can proceed in parallel with D and E.
