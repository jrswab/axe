# 042 — Artifact Management: Structured Intermediate File Handling Between Agents

**Issue:** https://github.com/jrswab/axe/issues/22
**Priority:** Medium
**Status:** Spec

---

## Section 1: Context & Constraints

### Associated Issue

> As pipelines grow beyond simple text passing to producing files/reports as outputs, the filesystem-only approach raises infrastructure questions.
>
> **Questions raised on HN:**
> - Where do intermediate artifacts live?
> - How do later agents reference them?
> - How long should they persist?
> - Are they workflow state or temporary outputs?
> - Should there be a shared artifact layer (local or object storage) as pipelines scale?
>
> **Current state:** Users manually save files between steps — works fine for small pipelines.
>
> **Possible directions:**
> - Named artifact output: `axe run agent --artifact-out report.md`
> - Pipeline-scoped temp directory auto-created and passed through chain
> - Artifact TTL / cleanup config
> - v2 consideration — filesystem works for now

### Research Findings

#### Codebase Structure Relevant to This Feature

- **No artifact concept exists today.** Agents produce text on stdout. Sub-agents return only their final text to the parent via `provider.ToolResult`. There is no structured way to pass files between agents in a pipeline.

- **File tools are sandboxed to workdir.** `write_file`, `read_file`, `list_directory`, and `edit_file` all resolve paths relative to `ExecContext.Workdir` and reject paths that escape it via `validatePath()` in `internal/tool/path_validation.go`. The artifact directory is a second, independent sandbox.

- **`ExecContext` in `internal/tool/registry.go`** holds `Workdir`, `Stderr`, `Verbose`, and `AllowedHosts`. This is the struct passed to all tool executors. New artifact-related fields will be added here.

- **`AgentConfig` in `internal/agent/agent.go`** already has nested config blocks following a consistent pattern: `SubAgentsConf SubAgentsConfig`, `Memory MemoryConfig`, `Params ParamsConfig`, `Retry RetryConfig`, `Budget BudgetConfig`. A new `Artifacts ArtifactsConfig` block follows this established pattern.

- **`Validate()` in `internal/agent/agent.go`** checks constraints on all config blocks. Artifact validation follows the same pattern.

- **`Scaffold()` in `internal/agent/agent.go`** generates a commented TOML template for new agents. The artifact block must be added here.

- **`ToolCall.Arguments` is `map[string]string`.** All tool parameters are strings. Boolean parameters like `artifact` must be parsed from string `"true"`/`"false"` (same pattern as `offset` and `limit` in `read_file` which parse integers from strings).

- **`ToolParameter` in `internal/provider/provider.go`** has `Type string`, `Description string`, `Required bool`. The `Type` field is set to `"string"` for all existing parameters. The `artifact` parameter will also be `"string"` type with values `"true"` or `"false"`.

- **XDG package (`internal/xdg/xdg.go`)** has `GetDataDir()` and `GetConfigDir()` but no `GetCacheDir()`. The XDG Base Directory spec defines `$XDG_CACHE_HOME` (default `~/.cache`) for "non-essential cached data." Auto-generated artifact temp directories belong here.

- **CLI flags are defined in `cmd/run.go`** on the `runCmd` cobra command. Resolution order throughout the codebase is: CLI flag > TOML config > default.

- **JSON output envelope** in `cmd/run.go` is a `map[string]interface{}` with conditional fields (e.g., budget fields are only added when budget > 0). Artifact fields follow this same conditional pattern.

- **Sub-agents are opaque to parents.** `ExecuteCallAgent` in `internal/tool/tool.go` spawns sub-agents with their own TOML config, provider, and conversation loop. The parent never sees sub-agent internals. Sub-agents load their own `AgentConfig` independently.

- **Parallel tool execution is the default.** Multiple tool calls run concurrently via goroutines in `executeToolCalls` in `cmd/run.go`. Any shared artifact tracking must be thread-safe.

- **`isWithinDir()` and `validatePath()` in `internal/tool/path_validation.go`** handle path sandboxing. The same validation logic applies to artifact paths, but against the artifact directory instead of workdir.

#### Decisions Already Made

1. **TOML-first with flag override.** The artifact directory is configured in the agent TOML file. A `--artifact-dir` CLI flag overrides the TOML value. This follows the established resolution pattern used by workdir, budget, and all other config.

2. **Explicit enablement required.** An `[artifacts]` table with `enabled = true` must be present in the agent TOML for the artifact system to activate. Without it, no temp directories are created, no env vars are set, and no behavior changes. This is the primary backward compatibility guarantee.

3. **Default to auto-generated temp dir, allow persistent override.** When `enabled = true` and no `dir` is specified, a temp directory is created under `$XDG_CACHE_HOME/axe/artifacts/<run-id>/`. When `dir` is specified, that persistent directory is used instead. The `--artifact-dir` flag overrides both.

4. **Sub-agents opt in explicitly.** A sub-agent only gets artifact directory access if its own TOML config has `[artifacts] enabled = true`. The parent's artifact directory is never automatically inherited. Sub-agents can reference the parent's artifact dir by setting `dir = "${AXE_ARTIFACT_DIR}"` in their own TOML, since the parent sets this env var when artifacts are active.

5. **Extend existing file tools, don't add new ones.** `write_file`, `read_file`, and `list_directory` gain an optional `artifact` parameter. When `artifact` is `"true"`, the tool operates against the artifact directory instead of workdir. This minimizes API surface — LLMs already know these tools.

6. **JSON output includes artifact manifest.** When `--json` is used and the artifact system is active, the output envelope includes an `artifacts` array listing all files written to the artifact directory during the run.

7. **Auto-cleanup only for auto-generated temp dirs.** Explicitly provided directories (via TOML `dir` or `--artifact-dir` flag) are never cleaned up. Auto-generated temp dirs are cleaned up after the run completes unless `--keep-artifacts` is passed.

#### Approaches Ruled Out

- **New `save_artifact` / `load_artifact` tools:** Adds unnecessary API surface. LLMs already understand `write_file` and `read_file`. An `artifact` parameter on existing tools is simpler.
- **Automatic artifact dir inheritance to sub-agents:** Violates the "opaque sub-agent" principle. Parent state should not leak to children implicitly.
- **`os.MkdirTemp` for temp artifacts:** System temp dir is less discoverable and doesn't follow XDG convention. `$XDG_CACHE_HOME` is the correct location per spec.
- **Silent temp dir creation without opt-in:** Would change behavior for existing users who don't configure artifacts. Rejected to preserve backward compatibility.
- **Artifact dir without `enabled = true`:** Setting `dir` without `enabled` is a likely misconfiguration. Treated as a validation error.

#### Constraints and Assumptions

- **Backward compatibility is non-negotiable.** An existing agent TOML with no `[artifacts]` table must behave identically to today. No temp directories created, no env vars set, no extra JSON fields, no changed tool behavior.
- **`ToolCall.Arguments` values are always strings.** The `artifact` parameter is parsed as a string. `"true"` (case-insensitive) activates artifact mode; anything else (including absent) does not.
- **Artifact directory is a second sandbox, independent of workdir.** An agent can have both a workdir and an artifact dir. They may be different directories. Path validation applies independently to each.
- **Thread safety required.** The artifact tracker must be safe for concurrent writes from parallel tool executions.
- **Run ID uniqueness.** Auto-generated temp dir names must be unique across concurrent `axe run` invocations. A timestamp plus random suffix is sufficient.

#### Open Questions Resolved

- **Q: Should `edit_file` also get an `artifact` parameter?** No. `edit_file` is for modifying existing files in the workdir. Artifacts are typically written whole by one agent and read by another. If editing is needed, the agent can `read_file` (artifact), modify, and `write_file` (artifact).
- **Q: Should the `--artifact-dir` flag activate the system even without `enabled = true` in TOML?** Yes. Passing the flag is an explicit user action that clearly indicates intent. The flag overrides the need for `enabled = true`.
- **Q: Should `dir` set without `enabled = true` be silently ignored or an error?** Validation error. It's a likely misconfiguration — the user set a directory but forgot to enable the system.
- **Q: What happens if the artifact dir doesn't exist when a persistent path is specified?** It is created (including parent directories) at the start of the run, same as auto-generated dirs.
- **Q: Should `AXE_ARTIFACT_DIR` be set for `run_command` tool calls?** Yes. When the artifact system is active, `AXE_ARTIFACT_DIR` is set in the environment for shell commands so scripts can read/write artifacts directly.

---

## Section 2: Requirements

### 2.1 Agent TOML Configuration

A new optional `[artifacts]` configuration table must be supported in agent TOML files.

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | `false` | Whether the artifact system is active for this agent. Must be `true` for auto-generated temp dirs to be created. |
| `dir` | string | `""` | Path to a persistent artifact directory. Supports `~` and `$VAR` expansion. When empty and `enabled` is `true`, an auto-generated temp directory is used. |

**Validation rules:**
- `dir` set to a non-empty value while `enabled` is `false` (or absent) is a validation error: `"artifacts.dir is set but artifacts.enabled is false"`.
- `dir` must not contain path traversal sequences (`..`).
- When `dir` is set, it is expanded via the same path expansion used for `workdir` (tilde, environment variables).

**Example — auto-generated temp dir:**
```toml
[artifacts]
enabled = true
```

**Example — persistent directory:**
```toml
[artifacts]
enabled = true
dir = "~/project-artifacts"
```

### 2.2 CLI Flag Overrides

Two new flags must be added to the `axe run` command.

**`--artifact-dir` flag:**
- Type: string
- Default: `""` (empty)
- When set to a non-empty value, it overrides the TOML `artifacts.dir` value AND activates the artifact system regardless of `artifacts.enabled`.
- The directory is treated as persistent (never auto-cleaned).

**`--keep-artifacts` flag:**
- Type: boolean
- Default: `false`
- When `true`, auto-generated temp directories are not cleaned up after the run completes.
- Has no effect when a persistent directory is used (persistent dirs are never cleaned up regardless).

**Resolution order for artifact directory:**
1. `--artifact-dir` flag (if non-empty) → activates system, uses flag value, persistent
2. TOML `artifacts.dir` (if non-empty and `enabled = true`) → uses TOML value, persistent
3. `artifacts.enabled = true` with no `dir` → auto-generate temp dir, cleaned up unless `--keep-artifacts`
4. None of the above → artifact system inactive

### 2.3 Artifact Directory Lifecycle

**Creation:**
- The artifact directory (whether auto-generated or persistent) must be created at the start of the run, before the first LLM call.
- Parent directories are created as needed (`MkdirAll` equivalent).
- Auto-generated temp dirs use the path: `$XDG_CACHE_HOME/axe/artifacts/<run-id>/` where `<run-id>` is a unique identifier (timestamp + random suffix, e.g., `20260321T143022-a1b2c3`).

**Cleanup:**
- Auto-generated temp dirs are removed (recursively) after the run completes, regardless of success or failure.
- If `--keep-artifacts` is passed, auto-generated temp dirs are NOT removed. A message is printed to stderr with the directory path: `"artifacts preserved: <path>"`.
- Persistent directories (from TOML `dir` or `--artifact-dir` flag) are never removed.

**Environment variable:**
- When the artifact system is active, `AXE_ARTIFACT_DIR` is set in the process environment to the resolved artifact directory path. This makes it available to `run_command` tool calls.

### 2.4 XDG Cache Directory

A `GetCacheDir()` function must be added to the `internal/xdg` package.

**Behavior:**
- Returns `$XDG_CACHE_HOME/axe` if `XDG_CACHE_HOME` is set and non-empty.
- Otherwise returns `$HOME/.cache/axe`.
- Does NOT create the directory (consistent with `GetDataDir()` and `GetConfigDir()`).

### 2.5 Artifact Tracker

A thread-safe tracker must record all files written to the artifact directory during a run.

**Tracked data per entry:**
- `path`: relative path within the artifact directory
- `agent`: name of the agent that wrote the file
- `size`: file size in bytes

**Behaviors:**
- The tracker is created once at the start of the run when the artifact system is active.
- The tracker is shared with tool executors via `ExecContext`.
- When the artifact system is inactive, no tracker is created. The tracker field in `ExecContext` is nil.
- All methods must be safe for concurrent use.

### 2.6 Tool Extensions

Three existing tools gain an optional `artifact` parameter.

#### 2.6.1 `write_file`

**New parameter:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `artifact` | string | no | `""` | When `"true"`, write to the artifact directory instead of workdir. |

**Behavior when `artifact` is `"true"`:**
- Path is resolved relative to the artifact directory (not workdir).
- Path validation (traversal, symlink escape) is applied against the artifact directory.
- Parent directories within the artifact directory are created as needed.
- The write is recorded in the artifact tracker (path, agent name, size).
- Success message includes the artifact directory context: `"wrote N bytes to <path> (artifact)"`.

**Behavior when `artifact` is absent, empty, or not `"true"`:**
- Existing behavior unchanged. Path resolves against workdir.

**Error when artifact system is inactive:**
- If `artifact` is `"true"` but no artifact directory is configured (ExecContext has no artifact dir), return an error result: `"artifact directory not configured for this agent"`.

#### 2.6.2 `read_file`

**New parameter:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `artifact` | string | no | `""` | When `"true"`, read from the artifact directory instead of workdir. |

**Behavior when `artifact` is `"true"`:**
- Path is resolved relative to the artifact directory (not workdir).
- Path validation is applied against the artifact directory.
- All existing read_file behavior (offset, limit, binary detection, line numbering) applies unchanged.

**Behavior when `artifact` is absent, empty, or not `"true"`:**
- Existing behavior unchanged.

**Error when artifact system is inactive:**
- Same as write_file: `"artifact directory not configured for this agent"`.

#### 2.6.3 `list_directory`

**New parameter:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `artifact` | string | no | `""` | When `"true"`, list the artifact directory instead of workdir. |

**Behavior when `artifact` is `"true"`:**
- Path is resolved relative to the artifact directory (not workdir).
- Path validation is applied against the artifact directory.
- All existing list_directory behavior (entry formatting, directory suffix) applies unchanged.

**Behavior when `artifact` is absent, empty, or not `"true"`:**
- Existing behavior unchanged.

**Error when artifact system is inactive:**
- Same as write_file: `"artifact directory not configured for this agent"`.

### 2.7 Sub-Agent Artifact Access

Sub-agents do NOT automatically inherit the parent's artifact directory.

**Behaviors:**
- A sub-agent's artifact configuration comes entirely from its own TOML file.
- If the sub-agent's TOML has `[artifacts] enabled = true` with a `dir`, it uses that directory.
- If the sub-agent's TOML has `[artifacts] enabled = true` without a `dir`, it gets its own auto-generated temp dir (independent of the parent's).
- To share the parent's artifact directory, the sub-agent's TOML must explicitly reference it, e.g., `dir = "${AXE_ARTIFACT_DIR}"`. The parent sets this env var when its artifact system is active.
- The sub-agent's artifact tracker is independent of the parent's. Artifact entries from sub-agents are NOT aggregated into the parent's tracker or JSON output.

### 2.8 JSON Output

When `--json` is used and the artifact system is active, the output envelope must include artifact metadata.

**Additional field:**

| Field | Type | Condition | Description |
|-------|------|-----------|-------------|
| `artifacts` | array of objects | Present only when artifact system is active | Files written to the artifact directory during the run. |

**Each object in the `artifacts` array:**

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Relative path within the artifact directory |
| `agent` | string | Name of the agent that wrote the file |
| `size` | integer | File size in bytes |

**When the artifact system is inactive**, the `artifacts` field must be omitted entirely from the JSON output. Existing JSON consumers must see no new fields.

**Example:**
```json
{
  "model": "anthropic/claude-sonnet-4-20250514",
  "content": "Analysis complete.",
  "artifacts": [
    {"path": "report.md", "agent": "analyzer", "size": 2048},
    {"path": "data/summary.csv", "agent": "analyzer", "size": 512}
  ]
}
```

### 2.9 Scaffold Template

The `axe agents init` scaffold template must include a commented-out `[artifacts]` block:

```toml
# [artifacts]
# enabled = false
# dir = ""
```

### 2.10 Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| No `[artifacts]` table in TOML, no `--artifact-dir` flag | Artifact system inactive. Zero behavior change from today. No temp dirs, no env vars, no JSON fields. |
| `[artifacts]` table present but `enabled = false` (or absent), no `dir` | Artifact system inactive. Same as above. |
| `artifacts.dir` set but `enabled` is `false` | Validation error. Exit code 2. Message: `"artifacts.dir is set but artifacts.enabled is false"`. |
| `artifacts.enabled = true`, no `dir`, no flag | Auto-generate temp dir under XDG cache. Clean up after run. |
| `artifacts.enabled = true`, `dir` set | Use persistent dir. No cleanup. |
| `--artifact-dir /tmp/my-artifacts` with no TOML config | Flag activates system. Use `/tmp/my-artifacts`. No cleanup (explicit path). |
| `--artifact-dir /tmp/override` with TOML `dir = "/other"` | Flag wins. Use `/tmp/override`. |
| `--keep-artifacts` with auto-generated temp dir | Temp dir preserved. Path printed to stderr. |
| `--keep-artifacts` with persistent dir | No effect (persistent dirs are never cleaned regardless). |
| Tool call with `artifact: "true"` but system inactive | Error result: `"artifact directory not configured for this agent"`. Not fatal — returned to LLM. |
| Tool call with `artifact: "false"` or absent | Existing behavior. Operates on workdir. |
| Tool call with `artifact: "TRUE"` or `artifact: "True"` | Treated as `"true"` (case-insensitive comparison). |
| Sub-agent without `[artifacts]` in its TOML | Sub-agent has no artifact access, even if parent does. |
| Sub-agent with `dir = "${AXE_ARTIFACT_DIR}"` | Sub-agent shares parent's artifact dir (if parent set the env var). |
| Concurrent `axe run` invocations with auto-generated dirs | Each gets a unique `<run-id>`. No collision. |
| Artifact dir path with `~` or `$VAR` | Expanded via same logic as workdir. |
| Write to artifact dir, then read back in same run | Works. Both tools resolve against the same artifact directory. |
| Agent has tools but `write_file` is not in tools list | Agent cannot write artifacts (tool not available). `artifact` parameter is irrelevant. |
| Memory append after run with artifacts | Memory behavior unchanged. Artifacts do not affect memory. |
| Auto-generated temp dir cleanup fails (e.g., permissions) | Warning printed to stderr. Not a fatal error. Exit code unchanged. |
