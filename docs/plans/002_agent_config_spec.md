# Specification: M2 - Agent Config

**Status:** Draft
**Version:** 1.0
**Created:** 2026-02-27
**Scope:** Load, validate, and manage agent TOML configuration files

---

## 1. Purpose

Implement agent configuration management for Axe. This milestone introduces TOML-based agent configuration files, a library for parsing and validating them, and CLI commands for listing, inspecting, creating, and editing agent configurations. This builds directly on the M1 skeleton (XDG directory structure, Cobra CLI framework) and provides the foundation for M3 (Single Agent Run).

---

## 2. Requirements

### 2.1 TOML Dependency

**Requirement 1.1:** Add `github.com/BurntSushi/toml` as a project dependency for TOML parsing.

**Requirement 1.2:** No other new external dependencies are permitted for this milestone.

### 2.2 Agent Config Struct

**Requirement 2.1:** Define an `AgentConfig` struct in a new package `internal/agent/` that represents a parsed agent TOML file. The struct must contain the following fields with their corresponding TOML keys:

| Go Field | TOML Key | Go Type | Required | Description |
|----------|----------|---------|----------|-------------|
| `Name` | `name` | `string` | yes | Agent identifier; must match the TOML filename (without `.toml` extension) |
| `Description` | `description` | `string` | no | Human-readable description of the agent |
| `Model` | `model` | `string` | yes | Provider/model string per models.dev (e.g. `anthropic/claude-sonnet-4-20250514`) |
| `SystemPrompt` | `system_prompt` | `string` | no | Agent persona/instructions |
| `Skill` | `skill` | `string` | no | Relative path to a SKILL.md file |
| `Files` | `files` | `[]string` | no | Glob patterns for context files |
| `Workdir` | `workdir` | `string` | no | Working directory for glob resolution |
| `SubAgents` | `sub_agents` | `[]string` | no | Names of agents this agent can invoke |
| `Memory` | `[memory]` | `MemoryConfig` | no | Memory sub-configuration (see Requirement 2.2) |
| `Params` | `[params]` | `ParamsConfig` | no | Model parameter overrides (see Requirement 2.3) |

**Requirement 2.2:** Define a `MemoryConfig` struct with the following fields:

| Go Field | TOML Key | Go Type | Default | Description |
|----------|----------|---------|---------|-------------|
| `Enabled` | `enabled` | `bool` | `false` | Enable persistent memory |
| `Path` | `path` | `string` | `""` | Custom memory directory (empty = default location) |

**Requirement 2.3:** Define a `ParamsConfig` struct with the following fields:

| Go Field | TOML Key | Go Type | Default | Description |
|----------|----------|---------|---------|-------------|
| `Temperature` | `temperature` | `float64` | `0` | Model temperature (0 means use provider default) |
| `MaxTokens` | `max_tokens` | `int` | `0` | Max output tokens (0 means use provider default) |

**Requirement 2.4:** All struct fields must have TOML struct tags matching the TOML key names exactly (e.g. `` toml:"system_prompt" ``).

**Requirement 2.5:** Optional fields that are absent from the TOML file must resolve to their Go zero values (`""` for strings, `nil` for slices, `false` for bools, `0` for numbers).

### 2.3 Loading Agent Config

**Requirement 3.1:** Implement a function with the signature:
```go
func Load(name string) (*AgentConfig, error)
```

**Requirement 3.2:** The `Load` function must:
1. Call `xdg.GetConfigDir()` to resolve the base config directory
2. Construct the file path as `<config_dir>/agents/<name>.toml`
3. Read the file from disk
4. Decode the file contents using `BurntSushi/toml`
5. Call `Validate` on the decoded struct (see Requirement 4.x)
6. Return the populated `*AgentConfig` on success

**Requirement 3.3:** The `name` parameter is the agent name without the `.toml` extension. Example: `Load("pr-reviewer")` reads `agents/pr-reviewer.toml`.

**Requirement 3.4:** Error conditions and their messages:

| Condition | Error Message Pattern |
|-----------|-----------------------|
| File does not exist | `agent config not found: <name>` |
| File is not valid TOML | `failed to parse agent config "<name>": <toml_error>` |
| XDG resolution fails | Propagate the error from `xdg.GetConfigDir()` |
| File read fails (permissions, etc.) | `failed to read agent config "<name>": <io_error>` |

**Requirement 3.5:** The `Load` function must not create any files or directories. It is strictly a read operation.

### 2.4 Listing Agent Configs

**Requirement 3.6:** Implement a function with the signature:
```go
func List() ([]AgentConfig, error)
```

**Requirement 3.7:** The `List` function must:
1. Call `xdg.GetConfigDir()` to resolve the base config directory
2. Read the `agents/` subdirectory
3. For each file with a `.toml` extension, call `Load` with the filename (minus extension)
4. Return a slice of all successfully loaded and validated `AgentConfig` structs
5. Skip files that fail to parse or validate; do not abort the entire list operation due to one bad file

**Requirement 3.8:** If the `agents/` directory does not exist, return an empty slice and no error.

**Requirement 3.9:** If the `agents/` directory exists but is empty, return an empty slice and no error.

**Requirement 3.10:** Non-`.toml` files in the `agents/` directory must be silently ignored.

**Requirement 3.11:** Subdirectories within `agents/` must be silently ignored.

### 2.5 Validating Agent Config

**Requirement 4.1:** Implement a function with the signature:
```go
func Validate(cfg *AgentConfig) error
```

**Requirement 4.2:** Validation must check the following required fields:

| Field | Validation Rule | Error Message |
|-------|----------------|---------------|
| `Name` | Must be non-empty after trimming whitespace | `agent config missing required field: name` |
| `Model` | Must be non-empty after trimming whitespace | `agent config missing required field: model` |

**Requirement 4.3:** If both required fields are missing, return an error for `name` only (check `name` first, fail fast).

**Requirement 4.4:** Validation must NOT check:
- Whether the model string is a valid provider/model combination (deferred to M3/M4)
- Whether referenced sub_agents exist as TOML files
- Whether file globs are valid patterns
- Whether the skill path points to an existing file
- Whether the workdir exists

These are runtime concerns, not config validation concerns.

### 2.6 Scaffold Template

**Requirement 5.1:** Implement a function with the signature:
```go
func Scaffold(name string) (string, error)
```

**Requirement 5.2:** The `Scaffold` function must return a string containing a valid TOML template with the following content:

```toml
name = "<name>"
description = ""

# Full provider/model per models.dev
model = "provider/model-name"

# Agent persona (optional)
# system_prompt = ""

# Default skill (optional, can be overridden with --skill flag)
# skill = ""

# Context files - glob patterns resolved from workdir or cwd (optional)
# files = []

# Working directory (optional)
# workdir = ""

# Sub-agents this agent can invoke (optional)
# sub_agents = []

# [memory]
# enabled = false
# path = ""

# [params]
# temperature = 0.3
# max_tokens = 4096
```

**Requirement 5.3:** The `<name>` placeholder in the template must be replaced with the `name` argument passed to `Scaffold`.

**Requirement 5.4:** The `model` field must be set to the literal string `"provider/model-name"` as a placeholder that the user must edit.

**Requirement 5.5:** Optional fields must be present as comments (prefixed with `#`) to serve as documentation for the user.

**Requirement 5.6:** The returned string must be valid TOML if all comment lines are removed and `model` is replaced with a real value.

### 2.7 `axe agents` Parent Command

**Requirement 6.1:** Register a new parent command `agents` on the root command.

**Requirement 6.2:** The `agents` command must have:
- `Use`: `"agents"`
- `Short`: `"Manage agent configurations"`
- `Long`: A description explaining that subcommands manage agent TOML files

**Requirement 6.3:** Running `axe agents` with no subcommand must display help text (Cobra default behavior).

### 2.8 `axe agents list`

**Requirement 7.1:** Register a `list` subcommand on the `agents` command.

**Requirement 7.2:** The command takes no arguments.

**Requirement 7.3:** The command must call `agent.List()` and print each agent on its own line.

**Requirement 7.4:** Output format for each agent:
- If the agent has a non-empty `Description`: `<name> - <description>`
- If the agent has an empty `Description`: `<name>`

**Requirement 7.5:** Agents must be listed in alphabetical order by name.

**Requirement 7.6:** If no agents exist, print nothing to stdout and exit with code 0.

**Requirement 7.7:** Output must go to `cmd.OutOrStdout()` for testability.

**Requirement 7.8:** Exit code must be 0 on success.

### 2.9 `axe agents show <agent>`

**Requirement 8.1:** Register a `show` subcommand on the `agents` command.

**Requirement 8.2:** The command requires exactly 1 positional argument: the agent name.

**Requirement 8.3:** If no argument is provided, Cobra must display a usage error (use `cobra.ExactArgs(1)`).

**Requirement 8.4:** The command must call `agent.Load(name)` and print the agent's configuration as key-value pairs.

**Requirement 8.5:** Output format — print each field on its own line as `Key:  <value>` with consistent label alignment. Only print fields that have non-zero values. The display order and labels are:

| Label | Field | Display Condition |
|-------|-------|-------------------|
| `Name` | `Name` | Always (required) |
| `Description` | `Description` | Non-empty string |
| `Model` | `Model` | Always (required) |
| `System Prompt` | `SystemPrompt` | Non-empty string |
| `Skill` | `Skill` | Non-empty string |
| `Files` | `Files` | Non-nil, length > 0 |
| `Workdir` | `Workdir` | Non-empty string |
| `Sub-Agents` | `SubAgents` | Non-nil, length > 0 |
| `Memory Enabled` | `Memory.Enabled` | `true` |
| `Memory Path` | `Memory.Path` | Non-empty string |
| `Temperature` | `Params.Temperature` | Non-zero |
| `Max Tokens` | `Params.MaxTokens` | Non-zero |

**Requirement 8.6:** For slice fields (`Files`, `SubAgents`), print comma-separated values on the same line. Example: `Files:  src/**/*.go, CONTRIBUTING.md`

**Requirement 8.7:** If the agent does not exist, print an error message to stderr and exit with code 2 (config error).

**Requirement 8.8:** If the agent's TOML is invalid, print an error message to stderr and exit with code 2.

**Requirement 8.9:** Output must go to `cmd.OutOrStdout()` for testability.

### 2.10 `axe agents init <agent>`

**Requirement 9.1:** Register an `init` subcommand on the `agents` command.

**Requirement 9.2:** The command requires exactly 1 positional argument: the agent name (use `cobra.ExactArgs(1)`).

**Requirement 9.3:** The command must:
1. Resolve the target path: `<config_dir>/agents/<name>.toml`
2. Check if the file already exists
3. If it exists, return an error (see Requirement 9.5)
4. If it does not exist, call `agent.Scaffold(name)` and write the result to the target path
5. Print the full path to the created file on success

**Requirement 9.4:** The created file must have permissions `0644`.

**Requirement 9.5:** If the file already exists, print an error to stderr with the message: `agent config already exists: <path>` and exit with code 2.

**Requirement 9.6:** If the `agents/` directory does not exist, create it with `os.MkdirAll` and permissions `0755` before writing the file.

**Requirement 9.7:** Output (on success) must be the full absolute path to the created TOML file, followed by a newline.

**Requirement 9.8:** Output must go to `cmd.OutOrStdout()` for testability.

### 2.11 `axe agents edit <agent>`

**Requirement 10.1:** Register an `edit` subcommand on the `agents` command.

**Requirement 10.2:** The command requires exactly 1 positional argument: the agent name (use `cobra.ExactArgs(1)`).

**Requirement 10.3:** The command must:
1. Read the `EDITOR` environment variable
2. If `EDITOR` is empty or unset, return an error (see Requirement 10.5)
3. Resolve the target path: `<config_dir>/agents/<name>.toml`
4. Check if the file exists
5. If the file does not exist, return an error (see Requirement 10.6)
6. Execute the editor: `$EDITOR <path>`

**Requirement 10.4:** The editor must be executed using `os/exec`, replacing the current process with `syscall.Exec` or running as a child process via `exec.Command`. The command must connect stdin, stdout, and stderr to the parent process's respective file descriptors so the editor is interactive.

**Requirement 10.5:** If `EDITOR` is not set, print to stderr: `$EDITOR environment variable is not set` and exit with code 1.

**Requirement 10.6:** If the agent TOML file does not exist, print to stderr: `agent config not found: <name>` and exit with code 2.

**Requirement 10.7:** If the editor process exits with a non-zero code, propagate that exit code.

---

## 3. Project Structure

After M2 completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── agents.go              # NEW: agents parent + list, show, init, edit subcommands
│   ├── agents_test.go         # NEW: tests for all agents subcommands
│   ├── config.go              # UNCHANGED
│   ├── config_test.go         # UNCHANGED
│   ├── root.go                # UNCHANGED
│   ├── root_test.go           # UNCHANGED
│   ├── version.go             # UNCHANGED
│   └── version_test.go        # UNCHANGED
├── internal/
│   ├── agent/
│   │   ├── agent.go           # NEW: AgentConfig struct, Load, List, Validate, Scaffold
│   │   └── agent_test.go      # NEW: tests for Load, List, Validate, Scaffold
│   └── xdg/
│       ├── xdg.go             # UNCHANGED
│       └── xdg_test.go        # UNCHANGED
├── go.mod                     # MODIFIED: add BurntSushi/toml
├── go.sum                     # MODIFIED
└── ...                        # all other files UNCHANGED
```

---

## 4. Exit Codes

All `agents` subcommands must follow the exit code conventions established in the CLI structure design:

| Code | Meaning | Used By |
|------|---------|---------|
| 0 | Success | All commands on success |
| 1 | General error | `edit` when `$EDITOR` is unset, editor process failure |
| 2 | Config error | `show`/`init`/`edit` when agent not found, invalid TOML, file already exists |

---

## 5. Constraints

**Constraint 1:** The only new external dependency is `github.com/BurntSushi/toml`. All other functionality must use the Go standard library.

**Constraint 2:** No validation of runtime values (model existence, sub-agent existence, file glob validity, workdir existence). Validation is limited to required field presence.

**Constraint 3:** The `List` function must be resilient. A single malformed TOML file must not prevent other agents from being listed.

**Constraint 4:** All output intended for the user must go through `cmd.OutOrStdout()` (for stdout) or `cmd.ErrOrStderr()` (for errors) to maintain testability.

**Constraint 5:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. The `edit` command's use of `syscall.Exec` may require platform-specific handling or use of `exec.Command` as a portable alternative.

**Constraint 6:** No interactive prompts. The `init` command writes a template with placeholders; it does not ask for user input.

**Constraint 7:** All functions in `internal/agent/` must accept or resolve paths via `xdg.GetConfigDir()`. No hardcoded paths.

---

## 6. Testing Requirements

### 6.1 Test Conventions

Tests must follow the patterns established in M1:

- **Package-level tests:** Tests live in the same package (e.g. `package agent`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks (no testify, no gomock)
- **Temp directories:** Use `t.TempDir()` for filesystem isolation
- **Env overrides:** Use `t.Setenv("XDG_CONFIG_HOME", tmpDir)` for XDG path control
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern
- **Descriptive names:** `TestFunctionName_Scenario` with underscores (e.g. `TestLoad_ValidConfig`, `TestLoad_MissingFile`)
- **Test real code, not mocks.** Tests must call the actual `Load`, `List`, `Validate`, and `Scaffold` functions with real TOML files on disk. Command tests must exercise the real command wiring through `rootCmd.Execute()`.

### 6.2 `internal/agent/` Tests

**Test: `TestLoad_ValidConfig`** — Write a valid TOML with all fields to a temp dir, call `Load`, verify every struct field is populated correctly.

**Test: `TestLoad_MinimalConfig`** — Write a TOML with only `name` and `model`, call `Load`, verify required fields populated and optional fields are zero values.

**Test: `TestLoad_MissingFile`** — Call `Load` for a non-existent agent name, verify error message matches `agent config not found: <name>`.

**Test: `TestLoad_MalformedTOML`** — Write invalid TOML content, call `Load`, verify error message matches `failed to parse agent config`.

**Test: `TestLoad_MissingName`** — Write TOML with `model` but no `name`, call `Load`, verify validation error for missing `name`.

**Test: `TestLoad_MissingModel`** — Write TOML with `name` but no `model`, call `Load`, verify validation error for missing `model`.

**Test: `TestLoad_EmptyNameWhitespace`** — Write TOML with `name = "  "`, call `Load`, verify validation error for missing `name` (whitespace-only is treated as empty).

**Test: `TestLoad_EmptyModelWhitespace`** — Write TOML with `name = "test"` and `model = "  "`, call `Load`, verify validation error for missing `model`.

**Test: `TestValidate_BothFieldsMissing`** — Pass an empty `AgentConfig` to `Validate`, verify the error is for `name` (fail-fast order).

**Test: `TestList_EmptyDirectory`** — Create an empty `agents/` dir, call `List`, verify empty slice returned with no error.

**Test: `TestList_NoDirectory`** — Point XDG to a dir without an `agents/` subdirectory, call `List`, verify empty slice with no error.

**Test: `TestList_MultipleAgents`** — Write multiple valid TOML files, call `List`, verify all agents returned.

**Test: `TestList_SkipsInvalidFiles`** — Write one valid and one malformed TOML, call `List`, verify only the valid agent is returned.

**Test: `TestList_IgnoresNonTOML`** — Place a `.md` file in `agents/`, call `List`, verify it is ignored.

**Test: `TestList_IgnoresSubdirectories`** — Place a subdirectory in `agents/`, call `List`, verify it is ignored.

**Test: `TestScaffold_ContainsName`** — Call `Scaffold("my-agent")`, verify the output contains `name = "my-agent"`.

**Test: `TestScaffold_ContainsModelPlaceholder`** — Call `Scaffold("test")`, verify output contains `model = "provider/model-name"`.

**Test: `TestScaffold_IsValidTOML`** — Call `Scaffold("test")`, uncomment all lines and set model to a real value, verify the result parses without error.

### 6.3 `cmd/` Tests

**Test: `TestAgentsCommand_ShowsHelp`** — Run `axe agents` with no subcommand, verify help text is displayed.

**Test: `TestAgentsList_Empty`** — Set up empty agents dir, run `axe agents list`, verify no output and exit code 0.

**Test: `TestAgentsList_WithAgents`** — Write agent TOML files to temp dir, run `axe agents list`, verify output contains agent names.

**Test: `TestAgentsList_AlphabeticalOrder`** — Write agents `zebra.toml`, `alpha.toml`, `mid.toml`, verify output order is `alpha`, `mid`, `zebra`.

**Test: `TestAgentsList_WithDescription`** — Write agent with description, verify output format is `name - description`.

**Test: `TestAgentsList_WithoutDescription`** — Write agent without description, verify output format is `name` only.

**Test: `TestAgentsShow_ValidAgent`** — Write a full agent TOML, run `axe agents show <name>`, verify key-value output for all non-zero fields.

**Test: `TestAgentsShow_MinimalAgent`** — Write a minimal agent TOML (name + model), run show, verify only Name and Model are printed.

**Test: `TestAgentsShow_MissingAgent`** — Run `axe agents show nonexistent`, verify error output.

**Test: `TestAgentsShow_NoArgs`** — Run `axe agents show` with no args, verify usage error.

**Test: `TestAgentsInit_CreatesFile`** — Run `axe agents init my-agent`, verify file created at correct path with correct content.

**Test: `TestAgentsInit_RefusesOverwrite`** — Create agent file, run init again, verify error about existing file.

**Test: `TestAgentsInit_CreatesAgentsDir`** — Point XDG to a dir without `agents/` subdirectory, run init, verify directory created and file written.

**Test: `TestAgentsInit_OutputIsPath`** — Run init, verify stdout output is the full file path.

**Test: `TestAgentsInit_NoArgs`** — Run `axe agents init` with no args, verify usage error.

**Test: `TestAgentsEdit_MissingEditor`** — Unset `EDITOR`, run `axe agents edit test`, verify error about `$EDITOR`.

**Test: `TestAgentsEdit_MissingAgent`** — Set `EDITOR` but don't create agent file, run `axe agents edit test`, verify error about missing agent.

**Test: `TestAgentsEdit_NoArgs`** — Run `axe agents edit` with no args, verify usage error.

Note: Testing the actual editor launch (exec) is not required since it involves process replacement. Tests must verify the precondition checks (EDITOR set, file exists) and argument construction.

### 6.4 Running Tests

All tests must pass when run with:
```bash
make test
```

If no `Makefile` exists yet, tests must pass with:
```bash
go test ./...
```

---

## 7. Acceptance Criteria

| Criterion | Test |
|-----------|------|
| TOML Dependency | `go.mod` includes `github.com/BurntSushi/toml` |
| Agent Struct | `internal/agent/agent.go` defines `AgentConfig`, `MemoryConfig`, `ParamsConfig` with correct TOML tags |
| Load Valid | `agent.Load("name")` successfully parses a valid TOML file into `AgentConfig` |
| Load Missing | `agent.Load("nonexistent")` returns descriptive error |
| Validate Name | TOML without `name` fails validation with specific error |
| Validate Model | TOML without `model` fails validation with specific error |
| List Agents | `agent.List()` returns all valid agents from the agents directory |
| List Resilient | `agent.List()` skips malformed files without aborting |
| Scaffold | `agent.Scaffold("name")` returns valid TOML template with name pre-filled |
| agents list | `axe agents list` prints agents in alphabetical order, one per line |
| agents show | `axe agents show <name>` prints key-value config details |
| agents init | `axe agents init <name>` creates TOML template at correct path |
| agents init no-overwrite | `axe agents init <name>` refuses to overwrite existing file |
| agents edit prereqs | `axe agents edit <name>` checks for `$EDITOR` and file existence |
| Exit codes | Config errors exit with code 2; general errors with code 1 |
| All tests pass | `go test ./...` passes with 0 failures |

---

## 8. Out of Scope

The following items are explicitly **not** included in M2:

1. LLM provider integration or API calls (M3)
2. File glob resolution or context building (M3)
3. SKILL.md content loading (M3)
4. Model string validation against provider APIs (M4)
5. Sub-agent invocation or delegation (M5)
6. Memory read/write operations (M6)
7. Garbage collection (M7)
8. Validation of `workdir` path existence
9. Validation of `files` glob pattern syntax
10. Validation of `sub_agents` references to existing agents
11. Validation of `skill` path existence
12. Any `axe run` functionality
13. Interactive prompts or TUI elements
14. Configuration file watching or hot-reloading

---

## 9. References

- Milestone Definition: `docs/plans/000_milestones.md` (M2 section)
- M1 Skeleton Spec: `docs/plans/001_skeleton_spec.md`
- Agent Config Schema: `docs/design/agent-config-schema.md`
- CLI Structure: `docs/design/cli-structure.md`
- BurntSushi/toml: https://github.com/BurntSushi/toml
- XDG Base Directory Specification: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

---

## 10. Notes

- The `agents/` directory is already created by `axe config init` (M1). The `agents init` command in M2 must also handle the case where `agents/` does not yet exist (user runs `agents init` before `config init`).
- The `Scaffold` function returns a string rather than writing to disk. This separates content generation from I/O, making it easier to test and reuse.
- The `List` function's resilience design (skip invalid files) is intentional. In a directory of 10 agents, one typo in a TOML file should not prevent the user from seeing the other 9.
- The `edit` command is the only command in M2 that interacts with external processes. Platform-specific behavior (e.g. `syscall.Exec` vs `exec.Command`) should be documented in code comments.
- Exit code 2 (config error) is used consistently for all agent-not-found and invalid-config scenarios, matching the exit code table from the CLI structure design doc.
- The `model` field uses the `provider/model-name` format from models.dev. Validation of this format is deferred to M3/M4 when provider integration is implemented.
