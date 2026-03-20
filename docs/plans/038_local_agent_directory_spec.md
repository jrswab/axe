# 038 — Local/Per-Repo Agent Directory

**GitHub Issue:** [#28](https://github.com/jrswab/axe/issues/28)

---

## Section 1: Context & Constraints

### Milestone

Allow axe to load agent configs from a local directory (`./axe/agents/`) in addition to the global XDG config directory (`$XDG_CONFIG_HOME/axe/agents/`). Add an `--agents-dir` flag for explicit override.

### Research Findings

#### Current Agent Loading

`agent.Load(name string)` resolves a single path:

```
$XDG_CONFIG_HOME/axe/agents/{name}.toml
```

`agent.List()` reads all `.toml` files from that same directory.

Both functions are called from 7 sites:

| Call Site | Function | Purpose |
|---|---|---|
| `cmd/run.go:102` | `agent.Load()` | Load agent for `axe run` |
| `cmd/agents.go:27` | `agent.List()` | `axe agents list` |
| `cmd/agents.go:53` | `agent.Load()` | `axe agents show` |
| `cmd/agents.go:121` | manual path build | `axe agents init` |
| `cmd/agents.go:163` | manual path build | `axe agents edit` |
| `cmd/gc.go:67,200` | `agent.Load()`, `agent.List()` | `axe gc` |
| `internal/tool/tool.go:134` | `agent.Load()` | `call_agent` sub-agent delegation |

#### Decisions Made

1. **Auto-discovery enabled.** If `./axe/agents/` exists relative to the working directory, it is automatically consulted before the global directory. No flag required.
2. **`--agents-dir` flag available.** Explicit flag for custom paths. Takes highest precedence.
3. **Local directory structure mirrors global.** `./axe/agents/*.toml` — not `./axe/*.toml`. Leaves room for `./axe/skills/` in the future.
4. **Sub-agents follow the same rules.** `call_agent` uses the parent agent's resolved workdir as the base for `./axe/agents/` discovery.
5. **`gc` follows the same rules.** Both `agent.Load()` and `agent.List()` calls in `gc` use the same resolution order.
6. **`agents list` does not indicate source.** Output remains clean and pipeable — just names (and descriptions). No `(local)` / `(global)` markers.
7. **`agents init` writes locally when `./axe/agents/` exists.** If `--agents-dir` is set, write there. Else if `./axe/agents/` exists in cwd, write there. Else write to global.

#### Approaches Ruled Out

- **Variadic `localDirs ...string` parameter on `Load`/`List`.** While discussed, the actual resolution logic (auto-discover + flag) belongs at the call site, not in the function signature. The function should accept a clear, ordered list of directories to search.
- **Persistent flag on `agentsCmd` only.** The flag is also needed on `run` and `gc`, so it cannot live only on the agents parent command.
- **`./axe/` without `agents/` subdirectory.** Ruled out to mirror global structure and leave room for future local skills.

#### Constraints

- **Resolution order is: `--agents-dir` > `./axe/agents/` (auto-discovered) > `$XDG_CONFIG_HOME/axe/agents/` (global).** First match wins. This follows the existing pattern: flags override TOML overrides defaults.
- **Auto-discovery base directory:**
  - For `axe run`: the resolved working directory (after `--workdir` / TOML `workdir` / cwd resolution).
  - For `call_agent` (sub-agents): the parent agent's resolved working directory.
  - For `axe agents list/show/init/edit`: the process's current working directory.
  - For `axe gc`: the process's current working directory.
- **Non-existent directories are silently skipped.** If `--agents-dir` points to a path that doesn't exist, or `./axe/agents/` doesn't exist, skip it and continue to the next directory in the resolution order. No error, no warning.
- **The `--agents-dir` flag value is an absolute or relative path to a directory containing `.toml` files directly** (not a path that needs `/agents` appended). If the user passes `--agents-dir ./custom`, axe looks for `./custom/{name}.toml`.
- **Backward compatibility.** All existing behavior is preserved when no local directory exists and no flag is passed. The global directory remains the final fallback.

---

## Section 2: Requirements

### Requirement 1: Agent Resolution Order

When loading an agent by name, the system must search directories in this order:

1. The directory specified by `--agents-dir` (if the flag is provided and non-empty)
2. `./axe/agents/` relative to the applicable working directory (auto-discovery)
3. `$XDG_CONFIG_HOME/axe/agents/` (global fallback)

The first directory containing `{name}.toml` wins. If no directory contains the file, return an error: `"agent config not found: {name}"`.

### Requirement 2: Agent Listing Merges All Sources

When listing agents, the system must read `.toml` files from all directories in the resolution order and merge results. If the same agent name appears in multiple directories, the version from the higher-precedence directory wins (earlier in the list). The merged list is returned as a single flat list with no source indicators.

### Requirement 3: Auto-Discovery of `./axe/agents/`

The system must automatically check for a `./axe/agents/` directory relative to the applicable base directory:

- **`axe run`**: base = the resolved working directory (after `--workdir` / TOML `workdir` / cwd fallback)
- **`call_agent` tool**: base = the parent agent's resolved working directory
- **`axe agents list/show/init/edit`**: base = the process's current working directory (`os.Getwd()`)
- **`axe gc`**: base = the process's current working directory

If the directory does not exist, it is silently skipped. No error, no warning, no creation.

### Requirement 4: `--agents-dir` Flag

A `--agents-dir` flag must be available on:

- `axe run`
- `axe agents list`
- `axe agents show`
- `axe agents init`
- `axe agents edit`
- `axe gc`

The flag accepts a path to a directory containing `.toml` agent files. The path may be absolute or relative (resolved from cwd). If the directory does not exist, it is silently skipped (same as auto-discovery).

### Requirement 5: `agents init` Write Location

When creating a new agent with `axe agents init`:

1. If `--agents-dir` is provided → write to that directory
2. Else if `./axe/agents/` exists in cwd → write there
3. Else → write to `$XDG_CONFIG_HOME/axe/agents/` (global)

The target directory must be created (with `0755` permissions) if it does not exist. If the agent file already exists at the resolved path, return an error: `"agent config already exists: {path}"`.

### Requirement 6: `agents edit` Path Resolution

When editing an agent with `axe agents edit`, the system must find the agent file using the same resolution order as `Load` (Requirement 1). The editor opens the file at the path where it was found. If the agent is not found in any directory, return an error.

### Requirement 7: Sub-Agent Delegation

The `call_agent` tool must pass the local directory resolution context to `agent.Load()`. The auto-discovery base is the parent agent's resolved working directory. If the parent agent was invoked with `--agents-dir`, that value must also propagate to sub-agent loading.

### Requirement 8: Backward Compatibility

When no `--agents-dir` flag is provided and no `./axe/agents/` directory exists in the applicable working directory, all behavior must be identical to the current implementation. No new directories are created. No new errors are produced. No output changes.

### Edge Cases

| Scenario | Expected Behavior |
|---|---|
| `--agents-dir` points to non-existent directory | Silently skip, continue to next source |
| `./axe/agents/` does not exist | Silently skip, continue to global |
| Agent exists in both local and global | Local version is used |
| Agent exists only in global | Global version is used |
| Agent exists in `--agents-dir` and `./axe/agents/` | `--agents-dir` version is used |
| `agents list` with agents in both local and global | Merged list, local wins on name collision |
| `agents init` with `--agents-dir` pointing to non-existent dir | Create the directory, then write the file |
| `agents init` when agent exists in local but not global | Error: "agent config already exists: {local_path}" |
| `agents init` when agent exists in global but `./axe/agents/` exists | Write to local (no collision in local dir) |
| `agents edit` when agent exists in both local and global | Edit the local version (higher precedence) |
| `--agents-dir ""` (empty string) | Treated as not provided; skip to auto-discovery |
| `call_agent` with parent workdir `/project` | Auto-discovers `/project/axe/agents/` |
| `axe run --workdir /other` | Auto-discovers `/other/axe/agents/` |
| `./axe/agents/` is a symlink to a directory | Follow the symlink (standard OS behavior) |
| `./axe/agents/` exists but is empty | No agents found there; fall through to global |
| `./axe/agents/` contains invalid TOML files | Skip invalid files (existing behavior preserved) |
