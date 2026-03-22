---

# 042 — Artifact Management: Implementation Guide

## Section 1: Context Summary

**Associated milestone document:** `docs/plans/000_milestones.md`

Axe agents currently produce text on stdout only; there is no structured way to pass files between agents in a pipeline. As pipelines grow to produce reports, data files, and other artifacts, users need a first-class mechanism for intermediate file handling. This implementation adds an opt-in artifact directory system: agents declare `[artifacts] enabled = true` in their TOML (or pass `--artifact-dir` at the CLI), and the three file tools (`write_file`, `read_file`, `list_directory`) gain an optional `artifact` parameter that redirects operations to a second, independent sandbox. Auto-generated temp directories live under `$XDG_CACHE_HOME/axe/artifacts/<run-id>/` and are cleaned up after the run unless `--keep-artifacts` is passed. Sub-agents never inherit the parent's artifact directory implicitly — they opt in via their own TOML, optionally referencing `${AXE_ARTIFACT_DIR}` which the parent sets in the environment. The JSON output envelope gains an `artifacts` array when the system is active. Backward compatibility is non-negotiable: any agent TOML without an `[artifacts]` table must behave identically to today.

---

## Section 2: Implementation Checklist

### XDG Cache Directory

- [x] `internal/xdg/xdg.go`: Add `GetCacheDir()` — returns `$XDG_CACHE_HOME/axe` if set, else `$HOME/.cache/axe`; does NOT create the directory (consistent with `GetDataDir()` and `GetConfigDir()`)
- [x] `internal/xdg/xdg_test.go`: Add table-driven tests for `GetCacheDir()` covering: `XDG_CACHE_HOME` set, `XDG_CACHE_HOME` unset (falls back to `$HOME/.cache/axe`), `XDG_CACHE_HOME` empty string (falls back)

### Artifact Tracker

- [x] `internal/artifact/tracker.go`: Create new package. Define `Entry` struct with fields `Path string`, `Agent string`, `Size int64`. Define `Tracker` struct with a `sync.Mutex`-protected `[]Entry` slice. Implement `NewTracker() *Tracker`, `Record(entry Entry)` (thread-safe append), `Entries() []Entry` (returns a copy of the slice, thread-safe)
- [x] `internal/artifact/tracker_test.go`: Table-driven tests for `Record()` and `Entries()` including concurrent writes (use `sync.WaitGroup` with multiple goroutines calling `Record()` simultaneously, verify no data races with `-race`)

### Agent Config: ArtifactsConfig

- [x] `internal/agent/agent.go`: Add `ArtifactsConfig` struct with fields `Enabled bool \`toml:"enabled"\`` and `Dir string \`toml:"dir"\``. Add `Artifacts ArtifactsConfig \`toml:"artifacts"\`` field to `AgentConfig`
- [x] `internal/agent/agent.go`: `Validate()` — add validation: if `cfg.Artifacts.Dir != ""` and `!cfg.Artifacts.Enabled`, return `&ValidationError{msg: "artifacts.dir is set but artifacts.enabled is false"}`; if `cfg.Artifacts.Dir` contains `..`, return `&ValidationError{msg: "artifacts.dir must not contain path traversal sequences"}`
- [x] `internal/agent/agent.go`: `Scaffold()` — append commented-out `[artifacts]` block after the `[budget]` block: `# [artifacts]\n# enabled = false\n# dir = ""\n`
- [x] `internal/agent/agent_test.go`: Table-driven tests for `Validate()` covering: `dir` set with `enabled = false` → error, `dir` containing `..` → error, `dir` set with `enabled = true` → no error, no `[artifacts]` table → no error

### ExecContext: Artifact Fields

- [x] `internal/tool/registry.go`: Add `ArtifactDir string` and `ArtifactTracker *artifact.Tracker` fields to `ExecContext`. Both are zero-value safe: empty string means artifact system inactive, nil tracker means no tracking.

### CLI Flags

- [x] `cmd/run.go`: `init()` — register `--artifact-dir` flag (string, default `""`, description: `"Override or set the artifact directory (activates artifact system)"`) and `--keep-artifacts` flag (bool, default `false`, description: `"Preserve auto-generated artifact directories after the run"`)

### Artifact Directory Lifecycle (cmd/run.go: runAgent())

- [x] `cmd/run.go`: `runAgent()` — after workdir resolution, read `--artifact-dir` flag and `--keep-artifacts` flag. Resolve effective artifact dir using precedence: (1) `--artifact-dir` flag if non-empty → persistent, (2) `cfg.Artifacts.Dir` if non-empty and `cfg.Artifacts.Enabled` → persistent (expand `~` and `$VAR` via same logic as workdir), (3) `cfg.Artifacts.Enabled` with no dir → auto-generate path `$XDG_CACHE_HOME/axe/artifacts/<run-id>/` where `<run-id>` is `time.Now().Format("20060102T150405") + "-" + <6-char random hex suffix>`, (4) none → artifact system inactive
- [x] `cmd/run.go`: `runAgent()` — when artifact system is active: call `os.MkdirAll(artifactDir, 0o755)` before first LLM call; set `os.Setenv("AXE_ARTIFACT_DIR", artifactDir)`; create `artifact.NewTracker()` and store it
- [x] `cmd/run.go`: `runAgent()` — after run completes (success or failure), if artifact dir was auto-generated: if `--keep-artifacts` is false, call `os.RemoveAll(artifactDir)` (log warning to stderr on error, do not change exit code); if `--keep-artifacts` is true, print `"artifacts preserved: <path>\n"` to stderr
- [x] `cmd/run.go`: `dispatchToolCall()` — pass `ArtifactDir` and `ArtifactTracker` through `tool.ExecContext` when constructing it (currently hardcoded in `registry.Dispatch` call at line 772)

### Tool Extensions: write_file

- [x] `internal/tool/write_file.go`: `writeFileDefinition()` — add `"artifact"` parameter: `Type: "string"`, `Required: false`, `Description: "When \"true\", write to the artifact directory instead of the working directory."`
- [x] `internal/tool/write_file.go`: `writeFileExecute()` — check `strings.EqualFold(call.Arguments["artifact"], "true")`. If true and `ec.ArtifactDir == ""`, return error result `"artifact directory not configured for this agent"`. If true and `ec.ArtifactDir != ""`, resolve path against `ec.ArtifactDir` (same absolute/traversal/symlink checks as workdir path), create parent dirs, write file, call `ec.ArtifactTracker.Record(artifact.Entry{Path: path, Agent: agentName, Size: int64(len(data))})` if tracker is non-nil, return success message `"wrote N bytes to <path> (artifact)"`. If false/absent, existing behavior unchanged.
- [x] `internal/tool/write_file_test.go`: Table-driven tests covering: `artifact: "true"` with valid artifact dir → writes to artifact dir, records in tracker; `artifact: "true"` with no artifact dir → error result; `artifact: "true"` with path traversal → error result; `artifact: "false"` → writes to workdir (existing behavior); `artifact` absent → writes to workdir (existing behavior); `artifact: "TRUE"` → treated as true (case-insensitive)

### Tool Extensions: read_file

- [x] `internal/tool/read_file.go`: `readFileDefinition()` — add `"artifact"` parameter: `Type: "string"`, `Required: false`, `Description: "When \"true\", read from the artifact directory instead of the working directory."`
- [x] `internal/tool/read_file.go`: `readFileExecute()` — check `strings.EqualFold(call.Arguments["artifact"], "true")`. If true and `ec.ArtifactDir == ""`, return error result `"artifact directory not configured for this agent"`. If true and `ec.ArtifactDir != ""`, call `validatePath(ec.ArtifactDir, path)` instead of `validatePath(ec.Workdir, path)`. All existing read behavior (offset, limit, binary detection, line numbering) unchanged.
- [x] `internal/tool/read_file_test.go`: Table-driven tests covering: `artifact: "true"` with valid artifact dir containing a file → reads from artifact dir; `artifact: "true"` with no artifact dir → error result; `artifact: "true"` with path traversal → error result; `artifact: "false"` → reads from workdir; `artifact` absent → reads from workdir

### Tool Extensions: list_directory

- [x] `internal/tool/list_directory.go`: `listDirectoryDefinition()` — add `"artifact"` parameter: `Type: "string"`, `Required: false`, `Description: "When \"true\", list the artifact directory instead of the working directory."`
- [x] `internal/tool/list_directory.go`: `listDirectoryExecute()` — check `strings.EqualFold(call.Arguments["artifact"], "true")`. If true and `ec.ArtifactDir == ""`, return error result `"artifact directory not configured for this agent"`. If true and `ec.ArtifactDir != ""`, call `validatePath(ec.ArtifactDir, path)` instead of `validatePath(ec.Workdir, path)`. All existing list behavior unchanged.
- [x] `internal/tool/list_directory_test.go`: Table-driven tests covering: `artifact: "true"` with valid artifact dir → lists artifact dir contents; `artifact: "true"` with no artifact dir → error result; `artifact: "false"` → lists workdir; `artifact` absent → lists workdir

### JSON Output

- [x] `cmd/run.go`: `runAgent()` — in the JSON envelope construction (around line 558), after the existing fields: if artifact system is active, add `"artifacts": tracker.Entries()` to the envelope (as `[]map[string]interface{}` with keys `"path"`, `"agent"`, `"size"`). If artifact system is inactive, omit the field entirely.
- [x] `cmd/run_test.go` (or golden test): Verify `--json` output includes `"artifacts"` array when artifact system is active with at least one write; verify `"artifacts"` field is absent when artifact system is inactive

### Integration / Edge Cases

- [x] `cmd/run_integration_test.go`: Add integration test: agent with `[artifacts] enabled = true`, uses `write_file` with `artifact: "true"`, then `read_file` with `artifact: "true"` — verify round-trip works and `--json` output contains the artifact entry
- [x] `cmd/run_integration_test.go`: Add integration test: agent with no `[artifacts]` table — verify zero behavior change (no temp dirs created, no env var set, no `"artifacts"` field in JSON output)
- [x] `cmd/run_integration_test.go`: Add integration test: `--artifact-dir /tmp/test-artifacts-<random>` flag with no TOML config — verify system activates, directory is used, no cleanup (persistent)
- [x] `cmd/run_integration_test.go`: Add integration test: `write_file` with `artifact: "true"` when artifact system is inactive — verify error result is returned to LLM (not fatal), run continues

---
