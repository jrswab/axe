# 047 — Artifact Default Write: Configurable `artifact = "true"` Default for `write_file`

**Milestone document:** `docs/plans/042_artifact_management_spec.md`

**Priority:** Medium
**Status:** Spec

---

## Section 1: Context & Constraints

### Associated Feature

When an agent has the artifact system active (`[artifacts] enabled = true` or `--artifact-dir` flag), the LLM must explicitly pass `artifact = "true"` in every `write_file` tool call for the file to land in the artifact directory. If the LLM omits the parameter, the file is written to the workdir instead — which is rarely the intended behavior for agents configured with artifacts.

Users currently work around this by adding instructions like "When using write_file, always set artifact = 'true'" to the system prompt. This is fragile: it depends on the LLM following instructions rather than on deterministic tool behavior.

### Codebase Structure Relevant to This Feature

- **`ArtifactsConfig` in `internal/agent/agent.go`** currently has two fields: `Enabled bool` and `Dir string`. A new `DefaultWrite bool` field follows the established pattern of nested config blocks.

- **`ExecContext` in `internal/tool/registry.go`** holds `Workdir`, `Stderr`, `Verbose`, `AllowedHosts`, `ArtifactDir`, and `ArtifactTracker`. This struct is passed to all tool executors. A new `DefaultArtifactWrite bool` field will carry the agent's default setting.

- **`ExecuteOptions` in `internal/tool/tool.go`** holds configuration for sub-agent execution, including `ArtifactDir` and `ArtifactTracker`. A new `DefaultArtifactWrite bool` field will carry the setting through to sub-agent tool dispatch.

- **`writeFileExecute` in `internal/tool/write_file.go`** currently checks `call.Arguments["artifact"]` with `strings.EqualFold`. The artifact decision logic is a single `if` statement at the top of the function. The default_write logic extends this exact check.

- **`dispatchToolCall` in `cmd/run.go`** constructs `tool.ExecContext` with `ArtifactDir` and `ArtifactTracker`. It must also pass `DefaultArtifactWrite`.

- **`executeToolCalls` in `cmd/run.go`** constructs `tool.ExecuteOptions` for sub-agent delegation. It must also pass `DefaultArtifactWrite`.

- **`dispatchToolCall` in `internal/tool/tool.go`** (sub-agent conversation loop) constructs `ExecContext` for tool dispatch within sub-agents. It must also pass `DefaultArtifactWrite`.

- **`Scaffold()` in `internal/agent/agent.go`** generates a commented TOML template for new agents. The `[artifacts]` block must include the new field.

- **`Validate()` in `internal/agent/agent.go`** checks constraints on all config blocks. No new validation rules are needed for `default_write` — it is a simple boolean with sensible fallback behavior.

### Decisions Already Made

1. **Configurable per agent, not global.** The `default_write` setting lives in the agent's TOML `[artifacts]` block. This preserves backward compatibility — existing agents without the setting continue to behave identically.

2. **`default_write` only takes effect when the artifact system is active.** If `[artifacts] enabled = false` and no `--artifact-dir` flag is provided, `default_write = true` is silently ignored. The user must still opt into the artifact system explicitly. This keeps concerns separate: `enabled` controls whether the artifact system exists; `default_write` controls the default tool behavior when it does exist.

3. **Explicit `artifact` arguments always override the default.** If the LLM passes `artifact = "false"`, the file is written to the workdir regardless of `default_write`. Only an absent/empty `artifact` argument triggers the default.

4. **`default_write = false` is the implicit default.** When the field is omitted from TOML, behavior is identical to today. No backward compatibility break.

### Approaches Ruled Out

- **Unconditional default to `artifact = "true"`:** Would break backward compatibility for all existing agents that use `write_file` without the `artifact` parameter. Rejected.
- **System prompt injection:** Adding "always use artifact = true" to the system prompt automatically. This is fragile (LLMs may not follow it) and pollutes the context window. Deterministic tool behavior is superior.
- **Separate tool (e.g., `write_artifact`):** Adds unnecessary API surface. The `artifact` parameter already exists on `write_file`. The feature is about changing its default, not adding a new tool.
- **`default_write = true` implying `enabled = true`:** Would couple two concerns. The user might want to set `default_write = true` as a template value without enabling artifacts in every environment. Keeping them separate gives the user control.

### Constraints and Assumptions

- **Backward compatibility is non-negotiable.** Any agent TOML without `default_write` (or with `default_write = false`) must behave identically to today.
- **`ToolCall.Arguments` values are always strings.** The `artifact` parameter is a string. The absence of the key (or an empty string value) is the trigger for the default — not a boolean zero value.
- **Sub-agents load their own TOML.** A sub-agent's `default_write` comes from its own TOML, not the parent's. This follows the existing "sub-agents are opaque" principle.
- **The `--artifact-dir` flag activates the artifact system.** When `--artifact-dir` is passed, the artifact system is active even without `[artifacts] enabled = true` in TOML. In this case, the TOML `default_write` value still applies (it's read from the agent config, not from the flag).

---

## Section 2: Requirements

### 2.1 Agent TOML Configuration

A new optional field must be added to the `[artifacts]` configuration table.

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default_write` | boolean | `false` | When `true` and the artifact system is active, `write_file` tool calls without an explicit `artifact` argument default to writing to the artifact directory. |

**No new validation rules.** The field is a simple boolean. The following scenarios are valid:
- `default_write = true` with `enabled = true` — active default.
- `default_write = true` with `enabled = false` — silently ignored (artifact system inactive).
- `default_write = true` with no `[artifacts]` table — silently ignored (artifact system inactive).
- `default_write = false` or omitted — existing behavior (backward compatible).

**Example:**
```toml
[artifacts]
enabled = true
dir = "output"
default_write = true
```

### 2.2 Tool Behavior Change: `write_file`

The `write_file` tool must apply the `default_write` setting when the `artifact` argument is absent or empty.

**Artifact resolution logic:**

1. If the `artifact` argument is explicitly `"true"` (case-insensitive) → write to artifact directory.
2. If the `artifact` argument is explicitly set to any non-empty, non-`"true"` value (e.g., `"false"`, `"no"`) → write to workdir.
3. If the `artifact` argument is absent or empty **and** `DefaultArtifactWrite` is `true` **and** the artifact system is active → write to artifact directory.
4. If the `artifact` argument is absent or empty **and** `DefaultArtifactWrite` is `false` (or the artifact system is inactive) → write to workdir.

This preserves the existing behavior for all cases while adding the new default path.

### 2.3 Configuration Threading

The `default_write` value from the agent TOML must be propagated to all tool execution contexts.

**Propagation path:**

1. `cfg.Artifacts.DefaultWrite` is read from the loaded agent config.
2. It is passed into `tool.ExecContext.DefaultArtifactWrite` for direct tool dispatch in `cmd/run.go`.
3. It is passed into `tool.ExecuteOptions.DefaultArtifactWrite` for sub-agent delegation in `cmd/run.go`.
4. Inside `internal/tool/tool.go`, the `dispatchToolCall` function must pass `opts.DefaultArtifactWrite` into `ExecContext.DefaultArtifactWrite` for sub-agent tool dispatch.

**Sub-agent behavior:** A sub-agent loads its own TOML config. Its own `default_write` value applies to its own tool calls. The parent's `default_write` is never inherited.

### 2.4 Scaffold Template

The `axe agents init` scaffold template must include the new field in the commented-out `[artifacts]` block:

```toml
# [artifacts]
# enabled = false
# dir = ""
# default_write = false
```

### 2.5 Edge Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| Agent TOML has no `[artifacts]` table, no `--artifact-dir` flag | Artifact system inactive. `default_write` is irrelevant. Zero behavior change. |
| `[artifacts] enabled = false`, `default_write = true`, no flag | Artifact system inactive. `default_write` silently ignored. `write_file` without `artifact` arg writes to workdir. |
| `[artifacts] enabled = true`, `default_write = true`, `write_file` called without `artifact` arg | File written to artifact directory. |
| `[artifacts] enabled = true`, `default_write = true`, `write_file` called with `artifact = "false"` | File written to workdir. Explicit argument overrides default. |
| `[artifacts] enabled = true`, `default_write = true`, `write_file` called with `artifact = "true"` | File written to artifact directory. Same as today. |
| `[artifacts] enabled = true`, `default_write = false` (or omitted), `write_file` called without `artifact` arg | File written to workdir. Same as today. |
| `--artifact-dir /tmp/out` with no TOML `[artifacts]` table, `default_write` not in TOML | Artifact system active (flag). `default_write` defaults to `false`. `write_file` without `artifact` arg writes to workdir. |
| `--artifact-dir /tmp/out` with TOML `default_write = true` but `enabled = false` | Artifact system active (flag). `default_write = true` from TOML applies. `write_file` without `artifact` arg writes to artifact directory. |
| Sub-agent with `default_write = true` in its own TOML | Sub-agent's `write_file` defaults to artifact mode. Independent of parent. |
| Sub-agent with no `[artifacts]` in its TOML, parent has `default_write = true` | Sub-agent has no artifact access and no `default_write`. Parent setting does not propagate. |
| `default_write = true`, artifact system active, `write_file` called with `artifact = "TRUE"` | File written to artifact directory. Case-insensitive match preserved. |
| `default_write = true`, artifact system active, `write_file` called with `artifact = "no"` | File written to workdir. Non-empty, non-"true" value overrides default. |

### 2.6 Parallel Work

The following work items can proceed in parallel after this spec is approved:

1. **Agent config changes** (`internal/agent/agent.go`, `agent_test.go`, scaffold template) — adding `DefaultWrite` field and updating scaffold.
2. **Tool execution context changes** (`internal/tool/registry.go`, `internal/tool/tool.go`) — adding `DefaultArtifactWrite` to `ExecContext` and `ExecuteOptions`.
3. **Write file logic change** (`internal/tool/write_file.go`, `write_file_test.go`) — depends on items 1 and 2 being complete.

Items 1 and 2 are independent of each other. Item 3 depends on both. After all three are complete, the integration work can proceed:

4. **Wiring in cmd/run.go** — depends on items 1, 2, and 3.
5. **Integration tests** (`cmd/run_integration_test.go`) — depends on item 4.
6. **Documentation updates** (`AGENTS.md`, `docs/plans/042_artifact_management_spec.md`) — can proceed in parallel with items 4 and 5.
