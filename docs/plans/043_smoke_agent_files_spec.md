# 043 — Smoke Test Agent Files

**Milestone document:** `docs/plans/ISS-40_smoke_test_agents_milestones.md`
**Issue:** https://github.com/jrswab/axe/issues/40
**Status:** Spec

---

## Section 1: Context & Constraints

### What This Milestone Covers

Create all static files that the smoke-test CI jobs (milestones 044–048) will consume:
- Agent TOML files in `.github/smoke-agents/`
- A skill file at `.github/smoke-agents/skills/smoke-skill/SKILL.md`
- A context text file at `.github/smoke-agents/smoke-context.txt`

No workflow YAML is created here. No Go code is changed. This milestone is purely file creation.

### Agent Loading & Search Paths

`axe run` resolves agents via `agent.BuildSearchDirs(flagAgentsDir, cwd)` then `agent.Load(agentName, searchDirs)`. The search order is:
1. `--agents-dir` flag (if provided)
2. Current working directory
3. XDG config dir (`$XDG_CONFIG_HOME/axe/agents/`)

In CI, the workflow will invoke `./axe run --agents-dir .github/smoke-agents <agent-name>`. The agent name must match the TOML `name` field exactly (case-sensitive).

### Skill Path Resolution — Critical Constraint

`resolve.Skill(skillPath, configDir)` resolves relative skill paths against `configDir`, which is **always** `$XDG_CONFIG_HOME/axe/` — not the `--agents-dir` directory and not the TOML file's location.

Resolution chain for a relative `skill` value:
1. `configDir + "/" + skillPath` — if that is a regular file, use it
2. `configDir + "/" + skillPath + "/SKILL.md"` — if that path is a directory
3. `configDir + "/skills/" + skillPath + "/SKILL.md"` — if `skillPath` has no path separators (bare name)

**Consequence:** A relative skill path in a smoke agent TOML will NOT find `.github/smoke-agents/skills/smoke-skill/SKILL.md` because `configDir` points to the XDG config dir, not the repo root.

**Required approach:** The `skill` field in `cfg-skill.toml` and `dry-skill.toml` must use an absolute path. In CI, the absolute path is constructed from `$GITHUB_WORKSPACE`, e.g.:
```
skill = "/home/runner/work/axe/axe/.github/smoke-agents/skills/smoke-skill/SKILL.md"
```
However, hardcoding a CI-specific absolute path in a committed TOML file is fragile. The correct approach is to use the `--skill` flag override at invocation time in the workflow, rather than embedding the path in the TOML. The `cfg-skill.toml` and `dry-skill.toml` files therefore do NOT set a `skill` field — the workflow step passes `--skill .github/smoke-agents/skills/smoke-skill/SKILL.md` as a flag. The `--skill` flag accepts a path that is resolved the same way as the TOML field, but the workflow can construct the correct absolute path at runtime.

**Note for milestone 044 and 047:** The workflow steps for `cfg-skill` and `dry-skill` agents must pass `--skill $(pwd)/.github/smoke-agents/skills/smoke-skill/SKILL.md` (absolute path via `$(pwd)`) to ensure correct resolution regardless of XDG config dir location.

### Files Config Resolution

The `files` field in agent TOML is resolved relative to the agent's `workdir` (default: current working directory). In CI, the workflow runs from the repo root, so `files = ["smoke-context.txt"]` will NOT find `.github/smoke-agents/smoke-context.txt`.

**Required approach:** The `cfg-files.toml` and `dry-files.toml` agents must set `files = [".github/smoke-agents/smoke-context.txt"]` so the path resolves correctly from the repo root (the CI working directory).

### Memory Path Resolution

The `[memory]` block's `path` field, if unset, defaults to `$XDG_DATA_HOME/axe/memory/<agent-name>.md`. In CI, the XDG data dir is the runner's default. The memory agents do not set a custom `path` — the default is acceptable because each CI job runs in a fresh environment.

### Sub-Agent Name Resolution

The `sub_agents` list contains agent names (not file paths). The name must match the `name` field in the child agent's TOML exactly. The child agent is found via the same `--agents-dir` search path as the parent. Both parent and child TOMLs must be in `.github/smoke-agents/`.

### Existing Fixture Agents (Do Not Modify)

The existing fixture agents in `cmd/testdata/agents/` are used by Go-level tests (`cmd/smoke_test.go`, `cmd/golden_test.go`, `cmd/run_integration_test.go`). These must not be touched. The smoke agents in `.github/smoke-agents/` are entirely separate.

### Model Choices (Already Decided)

| Use case | Model string | Rationale |
|----------|-------------|-----------|
| Anthropic provider test | `anthropic/claude-3-5-haiku-20241022` | Cheapest Anthropic model with tool calling |
| OpenAI provider test | `openai/gpt-4o-mini` | Cheapest OpenAI model with tool calling |
| All other live tests | `opencode/minimax-m2.5` | Cheapest via OpenCode Zen; routes to chat completions |
| All dry-run tests | `openai/gpt-4o` | Model string irrelevant for dry-run; matches existing fixture pattern |

### Approaches Ruled Out

- **Relative skill paths in TOML:** Will not resolve correctly because `configDir` is always XDG, not the repo. Ruled out.
- **Copying skill file to XDG config dir in CI:** Adds CI setup complexity. The `--skill` flag override is simpler.
- **Reusing `cmd/testdata/agents/` fixtures:** Those use expensive models and are wired to Go test infrastructure. Separate files are required.
- **Single "do-everything" agent:** Each agent must test exactly one concern. Mixing config sections in one agent makes failures ambiguous.

---

## Section 2: Requirements

### 2.1 Directory Structure

The following directory structure must be created under `.github/smoke-agents/`:

```
.github/smoke-agents/
├── skills/
│   └── smoke-skill/
│       └── SKILL.md
├── smoke-context.txt
├── provider-anthropic.toml
├── provider-openai.toml
├── provider-opencode.toml
├── tool-list-dir.toml
├── tool-read-file.toml
├── tool-write-file.toml
├── tool-edit-file.toml
├── tool-run-cmd.toml
├── tool-url-fetch.toml
├── tool-web-search.toml
├── cfg-basic.toml
├── cfg-skill.toml
├── cfg-memory.toml
├── cfg-files.toml
├── cfg-subagents.toml
├── cfg-sub-child.toml
├── cfg-params.toml
├── pipe-basic.toml
├── dry-basic.toml
├── dry-tools.toml
├── dry-skill.toml
├── dry-memory.toml
└── dry-files.toml
```

### 2.2 Supporting Files

#### 2.2.1 `.github/smoke-agents/skills/smoke-skill/SKILL.md`

A minimal skill file. Its content must instruct the LLM to include the exact phrase `smoke-skill-loaded` somewhere in its response. This phrase is what the CI workflow asserts on.

Requirements:
- Must be valid Markdown.
- Must contain a clear instruction to include `smoke-skill-loaded` in the response.
- Must be short — no more than 10 lines.

#### 2.2.2 `.github/smoke-agents/smoke-context.txt`

A small plain-text file used to verify that the `files` config section injects file content into the LLM context.

Requirements:
- Must contain the exact phrase `smoke-context-loaded` on a line by itself.
- Must be short — no more than 5 lines.
- Must be plain text (no Markdown, no special formatting).

### 2.3 Provider Test Agents

These agents verify that `axe` can connect to and receive a valid response from each supported provider. They have no tools, no skill, no memory, no files. They are the simplest possible agents.

All three can be created in parallel.

#### 2.3.1 `provider-anthropic.toml`

| Field | Value |
|-------|-------|
| `name` | `provider-anthropic` |
| `model` | `anthropic/claude-3-5-haiku-20241022` |
| `description` | `Smoke test: Anthropic provider connectivity` |

No other fields.

#### 2.3.2 `provider-openai.toml`

| Field | Value |
|-------|-------|
| `name` | `provider-openai` |
| `model` | `openai/gpt-4o-mini` |
| `description` | `Smoke test: OpenAI provider connectivity` |

No other fields.

#### 2.3.3 `provider-opencode.toml`

| Field | Value |
|-------|-------|
| `name` | `provider-opencode` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: OpenCode provider connectivity` |

No other fields.

### 2.4 Tool Test Agents

These agents verify that each built-in tool executes correctly. All use `opencode/minimax-m2.5`. Each agent enables exactly one tool. All seven can be created in parallel.

#### 2.4.1 `tool-list-dir.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-list-dir` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: list_directory tool` |
| `tools` | `["list_directory"]` |

No other fields.

#### 2.4.2 `tool-read-file.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-read-file` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: read_file tool` |
| `tools` | `["read_file"]` |

No other fields.

#### 2.4.3 `tool-write-file.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-write-file` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: write_file tool` |
| `tools` | `["write_file"]` |

No other fields.

#### 2.4.4 `tool-edit-file.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-edit-file` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: edit_file tool` |
| `tools` | `["edit_file", "read_file"]` |

Note: `read_file` is included alongside `edit_file` so the LLM can verify the edit was applied. This is the only tool agent with two tools.

#### 2.4.5 `tool-run-cmd.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-run-cmd` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: run_command tool` |
| `tools` | `["run_command"]` |

No other fields.

#### 2.4.6 `tool-url-fetch.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-url-fetch` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: url_fetch tool` |
| `tools` | `["url_fetch"]` |

No other fields.

#### 2.4.7 `tool-web-search.toml`

| Field | Value |
|-------|-------|
| `name` | `tool-web-search` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: web_search tool` |
| `tools` | `["web_search"]` |

No other fields.

### 2.5 Config Section Test Agents

These agents verify that each TOML configuration section is parsed and applied correctly. All use `opencode/minimax-m2.5` unless noted. All seven can be created in parallel.

#### 2.5.1 `cfg-basic.toml`

Baseline — verifies that a minimal agent config (name + model only) produces a valid response.

| Field | Value |
|-------|-------|
| `name` | `cfg-basic` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: basic config (name + model only)` |

No other fields.

#### 2.5.2 `cfg-skill.toml`

Verifies that the `skill` config section is applied. The `skill` field is intentionally omitted from this TOML — the CI workflow passes `--skill $(pwd)/.github/smoke-agents/skills/smoke-skill/SKILL.md` at invocation time to avoid hardcoding an absolute path.

| Field | Value |
|-------|-------|
| `name` | `cfg-skill` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: skill config section` |

No `skill` field. No other fields.

#### 2.5.3 `cfg-memory.toml`

Verifies that the `[memory]` config section is parsed and that memory entries persist across runs.

| Field | Value |
|-------|-------|
| `name` | `cfg-memory` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: memory config section` |
| `[memory].enabled` | `true` |
| `[memory].last_n` | `5` |
| `[memory].max_entries` | `50` |

No `path` override — uses the default XDG data dir path.

#### 2.5.4 `cfg-files.toml`

Verifies that the `files` config section injects file content into the LLM context.

| Field | Value |
|-------|-------|
| `name` | `cfg-files` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: files config section` |
| `files` | `[".github/smoke-agents/smoke-context.txt"]` |

The path `.github/smoke-agents/smoke-context.txt` is relative to the agent's workdir, which defaults to the current working directory (the repo root in CI).

#### 2.5.5 `cfg-subagents.toml`

Verifies that the `sub_agents` config section and `[sub_agents_config]` block are parsed and that the parent can delegate to a child agent.

| Field | Value |
|-------|-------|
| `name` | `cfg-subagents` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: sub_agents config section` |
| `sub_agents` | `["cfg-sub-child"]` |
| `[sub_agents_config].max_depth` | `2` |
| `[sub_agents_config].parallel` | `true` |
| `[sub_agents_config].timeout` | `60` |

#### 2.5.6 `cfg-sub-child.toml`

The child agent delegated to by `cfg-subagents`. Must be a simple responder — no tools, no skill, no memory.

| Field | Value |
|-------|-------|
| `name` | `cfg-sub-child` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: sub-agent child` |
| `system_prompt` | `"You are a simple responder. When called, reply with only: sub-agent-ok"` |

#### 2.5.7 `cfg-params.toml`

Verifies that the `[params]` config block is parsed and applied without crashing.

| Field | Value |
|-------|-------|
| `name` | `cfg-params` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: params config section` |
| `[params].temperature` | `0.5` |
| `[params].max_tokens` | `100` |

### 2.6 Piped Input Test Agent

#### 2.6.1 `pipe-basic.toml`

A minimal agent used for all stdin piping tests. No tools, no skill, no memory.

| Field | Value |
|-------|-------|
| `name` | `pipe-basic` |
| `model` | `opencode/minimax-m2.5` |
| `description` | `Smoke test: piped stdin input` |

No other fields.

### 2.7 Dry-Run Test Agents

These agents are used with `--dry-run` only. No API calls are made. The model string is irrelevant to correctness but must be a valid `provider/model` format. All use `openai/gpt-4o` to match the existing fixture pattern. All five can be created in parallel.

#### 2.7.1 `dry-basic.toml`

Baseline dry-run. Verifies that `=== Dry Run ===` and `--- System Prompt ---` appear in output.

| Field | Value |
|-------|-------|
| `name` | `dry-basic` |
| `model` | `openai/gpt-4o` |
| `description` | `Smoke test: dry-run baseline` |

No other fields.

#### 2.7.2 `dry-tools.toml`

Verifies that the `--- Tools ---` section in dry-run output is populated (not `(none)`).

| Field | Value |
|-------|-------|
| `name` | `dry-tools` |
| `model` | `openai/gpt-4o` |
| `description` | `Smoke test: dry-run with tools` |
| `tools` | `["read_file", "list_directory"]` |

#### 2.7.3 `dry-skill.toml`

Verifies that the `--- Skill ---` section in dry-run output is not `(none)`. The `skill` field is intentionally omitted — the CI workflow passes `--skill $(pwd)/.github/smoke-agents/skills/smoke-skill/SKILL.md` at invocation time (same reason as `cfg-skill.toml`).

| Field | Value |
|-------|-------|
| `name` | `dry-skill` |
| `model` | `openai/gpt-4o` |
| `description` | `Smoke test: dry-run with skill` |

No `skill` field. No other fields.

#### 2.7.4 `dry-memory.toml`

Verifies that the `--- Memory ---` section appears in dry-run output when memory is enabled.

| Field | Value |
|-------|-------|
| `name` | `dry-memory` |
| `model` | `openai/gpt-4o` |
| `description` | `Smoke test: dry-run with memory` |
| `[memory].enabled` | `true` |
| `[memory].last_n` | `5` |
| `[memory].max_entries` | `50` |

#### 2.7.5 `dry-files.toml`

Verifies that the `--- Files ---` section in dry-run output is not `(none)`.

| Field | Value |
|-------|-------|
| `name` | `dry-files` |
| `model` | `openai/gpt-4o` |
| `description` | `Smoke test: dry-run with files` |
| `files` | `[".github/smoke-agents/smoke-context.txt"]` |

### 2.8 Agent Name Constraints

- Every `name` field must be unique across all files in `.github/smoke-agents/`.
- Every `name` field must use only lowercase letters, digits, and hyphens (no underscores, no spaces). This matches the existing agent naming convention in the codebase.
- The filename (without `.toml`) does not need to match the `name` field, but it should for clarity. All files in this milestone follow that convention.

### 2.9 TOML Format Constraints

- All TOML files must be valid TOML (parseable by the `github.com/BurntSushi/toml` library used by the codebase).
- String values must be quoted.
- Array values use TOML array syntax: `["item1", "item2"]`.
- Inline tables are not used — nested config uses TOML table headers: `[memory]`, `[params]`, `[sub_agents_config]`.
- No fields beyond those specified in each agent's requirements. Extra fields are not needed and add noise.

### 2.10 Edge Cases & Explicit Behaviors

| Scenario | Required Behavior |
|----------|------------------|
| `cfg-skill.toml` has no `skill` field | The CI workflow supplies `--skill` at invocation time. The TOML itself must not set `skill`. |
| `dry-skill.toml` has no `skill` field | Same as above. |
| `cfg-sub-child.toml` `system_prompt` contains the phrase `sub-agent-ok` | This is the marker the CI workflow asserts on in the parent's stdout. The child's `system_prompt` must instruct it to include this exact phrase. |
| `cfg-files.toml` `files` path is relative | Resolved relative to workdir (repo root in CI). Path must start from repo root: `.github/smoke-agents/smoke-context.txt`. |
| `dry-files.toml` `files` path is relative | Same as above. |
| `cfg-memory.toml` has no `path` override | Uses default XDG data dir. Acceptable — CI runner has a fresh environment per job. |
| `dry-memory.toml` has no `path` override | Same as above. |
| `tool-edit-file.toml` includes `read_file` | Required so the LLM can verify its edit. This is the only tool agent with two tools. |
| `cfg-subagents.toml` `sub_agents` list | Must contain exactly `["cfg-sub-child"]` — the name must match the `name` field in `cfg-sub-child.toml` exactly. |
| Any agent TOML with an unknown field | `agent.Validate()` will return an error. Do not add fields not defined in `AgentConfig`. |
| `smoke-context.txt` content | Must contain `smoke-context-loaded` as a standalone phrase. The CI workflow asserts this phrase appears in the LLM's response. |
| `SKILL.md` content | Must instruct the LLM to include `smoke-skill-loaded` in its response. The CI workflow asserts this phrase appears in stdout. |
