# Implementation Checklist: M2 - Agent Config

**Based on:** 002_agent_config_spec.md
**Status:** Complete
**Created:** 2026-02-27

---

## Phase 1: Project Setup

- [x] Merge M1 branch (`feat/001-skeleton-cli`) to `master`
- [x] Create branch `feat/002-agent-config` off `master`
- [x] Add TOML dependency: `go get github.com/BurntSushi/toml`
- [x] Run `go mod tidy` and verify `go.mod` includes `BurntSushi/toml`
- [x] Create `internal/agent/` directory

---

## Phase 2: Agent Config Structs (Spec §2.2)

- [x] Define `MemoryConfig` struct in `internal/agent/agent.go`:
  - [x] `Enabled bool` with tag `toml:"enabled"`
  - [x] `Path string` with tag `toml:"path"`
- [x] Define `ParamsConfig` struct in `internal/agent/agent.go`:
  - [x] `Temperature float64` with tag `toml:"temperature"`
  - [x] `MaxTokens int` with tag `toml:"max_tokens"`
- [x] Define `AgentConfig` struct in `internal/agent/agent.go`:
  - [x] `Name string` with tag `toml:"name"`
  - [x] `Description string` with tag `toml:"description"`
  - [x] `Model string` with tag `toml:"model"`
  - [x] `SystemPrompt string` with tag `toml:"system_prompt"`
  - [x] `Skill string` with tag `toml:"skill"`
  - [x] `Files []string` with tag `toml:"files"`
  - [x] `Workdir string` with tag `toml:"workdir"`
  - [x] `SubAgents []string` with tag `toml:"sub_agents"`
  - [x] `Memory MemoryConfig` with tag `toml:"memory"`
  - [x] `Params ParamsConfig` with tag `toml:"params"`

---

## Phase 3: Validate Function (Spec §2.5)

Write tests first (red), then implement (green):

- [x] Write `TestValidate_BothFieldsMissing` — empty struct returns error for `name` (fail-fast)
- [x] Write `TestLoad_MissingName` — TOML with `model` only fails with `agent config missing required field: name`
- [x] Write `TestLoad_MissingModel` — TOML with `name` only fails with `agent config missing required field: model`
- [x] Write `TestLoad_EmptyNameWhitespace` — `name = "  "` fails validation
- [x] Write `TestLoad_EmptyModelWhitespace` — `model = "  "` fails validation
- [x] Implement `Validate(cfg *AgentConfig) error` in `internal/agent/agent.go`:
  - [x] Check `Name` is non-empty after `strings.TrimSpace`; return `agent config missing required field: name`
  - [x] Check `Model` is non-empty after `strings.TrimSpace`; return `agent config missing required field: model`
- [x] Run tests — all Validate tests pass

---

## Phase 4: Load Function (Spec §2.3)

Write tests first (red), then implement (green):

- [x] Write `TestLoad_ValidConfig` — full TOML with all fields, verify every struct field populated
- [x] Write `TestLoad_MinimalConfig` — TOML with only `name` and `model`, verify optional fields are zero values
- [x] Write `TestLoad_MissingFile` — non-existent agent returns `agent config not found: <name>`
- [x] Write `TestLoad_MalformedTOML` — invalid TOML returns `failed to parse agent config "<name>": ...`
- [x] Implement `Load(name string) (*AgentConfig, error)` in `internal/agent/agent.go`:
  - [x] Call `xdg.GetConfigDir()` to resolve base config directory
  - [x] Construct path: `<config_dir>/agents/<name>.toml`
  - [x] Check if file exists; if not, return `agent config not found: <name>`
  - [x] Read file from disk; on failure return `failed to read agent config "<name>": <error>`
  - [x] Decode TOML with `toml.Decode`; on failure return `failed to parse agent config "<name>": <error>`
  - [x] Call `Validate` on decoded struct; propagate validation error
  - [x] Return `*AgentConfig` on success
- [x] Run tests — all Load tests pass (including Validate-related Load tests from Phase 3)

---

## Phase 5: List Function (Spec §2.4)

Write tests first (red), then implement (green):

- [x] Write `TestList_EmptyDirectory` — empty `agents/` dir returns empty slice, no error
- [x] Write `TestList_NoDirectory` — no `agents/` subdirectory returns empty slice, no error
- [x] Write `TestList_MultipleAgents` — multiple valid TOML files all returned
- [x] Write `TestList_SkipsInvalidFiles` — one valid + one malformed TOML, only valid returned
- [x] Write `TestList_IgnoresNonTOML` — `.md` file in `agents/` is ignored
- [x] Write `TestList_IgnoresSubdirectories` — subdirectory in `agents/` is ignored
- [x] Implement `List() ([]AgentConfig, error)` in `internal/agent/agent.go`:
  - [x] Call `xdg.GetConfigDir()` to resolve base config directory
  - [x] Read `agents/` subdirectory; if not found (`os.IsNotExist`), return empty slice
  - [x] Iterate directory entries; skip non-files and non-`.toml` entries
  - [x] For each `.toml` file, call `Load` with filename minus extension
  - [x] Skip files where `Load` returns an error; append successful results
  - [x] Return collected slice
- [x] Run tests — all List tests pass

---

## Phase 6: Scaffold Function (Spec §2.6)

Write tests first (red), then implement (green):

- [x] Write `TestScaffold_ContainsName` — `Scaffold("my-agent")` output contains `name = "my-agent"`
- [x] Write `TestScaffold_ContainsModelPlaceholder` — output contains `model = "provider/model-name"`
- [x] Write `TestScaffold_IsValidTOML` — uncommented output parses as valid TOML
- [x] Implement `Scaffold(name string) (string, error)` in `internal/agent/agent.go`:
  - [x] Build template string with `name` interpolated and `model` set to `"provider/model-name"`
  - [x] Include all optional fields as comments
  - [x] Return the template string
- [x] Run tests — all Scaffold tests pass

---

## Phase 7: `axe agents` Parent Command (Spec §2.7)

- [x] Create `cmd/agents.go`
- [x] Define `agentsCmd` cobra command with `Use: "agents"`, `Short: "Manage agent configurations"`
- [x] Register `agentsCmd` on `rootCmd` in `init()`
- [x] Write `TestAgentsCommand_ShowsHelp` — `axe agents` with no subcommand displays help
- [x] Run test — passes

---

## Phase 8: `axe agents list` Command (Spec §2.8)

Write tests first (red), then implement (green):

- [x] Write `TestAgentsList_Empty` — empty agents dir, no output, exit code 0
- [x] Write `TestAgentsList_WithAgents` — output contains agent names
- [x] Write `TestAgentsList_AlphabeticalOrder` — agents `zebra`, `alpha`, `mid` output in alpha order
- [x] Write `TestAgentsList_WithDescription` — output format is `name - description`
- [x] Write `TestAgentsList_WithoutDescription` — output format is `name` only
- [x] Implement `agentsListCmd` in `cmd/agents.go`:
  - [x] Call `agent.List()`
  - [x] Sort results alphabetically by `Name`
  - [x] Print each agent: `<name> - <description>` or `<name>` if no description
  - [x] Output to `cmd.OutOrStdout()`
  - [x] Register on `agentsCmd` in `init()`
- [x] Run tests — all agents list tests pass

---

## Phase 9: `axe agents show <agent>` Command (Spec §2.9)

Write tests first (red), then implement (green):

- [x] Write `TestAgentsShow_ValidAgent` — full agent, verify all non-zero fields in key-value output
- [x] Write `TestAgentsShow_MinimalAgent` — only `Name` and `Model` printed
- [x] Write `TestAgentsShow_MissingAgent` — error output for nonexistent agent
- [x] Write `TestAgentsShow_NoArgs` — usage error with no arguments
- [x] Implement `agentsShowCmd` in `cmd/agents.go`:
  - [x] Use `cobra.ExactArgs(1)` for argument validation
  - [x] Call `agent.Load(args[0])`
  - [x] On error, return error (exit code 2 for config errors)
  - [x] Print key-value pairs with aligned labels; only print non-zero fields
  - [x] For slice fields, join with `, ` separator
  - [x] Output to `cmd.OutOrStdout()`
  - [x] Register on `agentsCmd` in `init()`
- [x] Run tests — all agents show tests pass

---

## Phase 10: `axe agents init <agent>` Command (Spec §2.10)

Write tests first (red), then implement (green):

- [x] Write `TestAgentsInit_CreatesFile` — file created at correct path with scaffold content
- [x] Write `TestAgentsInit_RefusesOverwrite` — error when file already exists
- [x] Write `TestAgentsInit_CreatesAgentsDir` — `agents/` dir created if missing
- [x] Write `TestAgentsInit_OutputIsPath` — stdout output is full file path
- [x] Write `TestAgentsInit_NoArgs` — usage error with no arguments
- [x] Implement `agentsInitCmd` in `cmd/agents.go`:
  - [x] Use `cobra.ExactArgs(1)` for argument validation
  - [x] Resolve path: `<config_dir>/agents/<name>.toml`
  - [x] Check if file exists; if yes, return error `agent config already exists: <path>` (exit code 2)
  - [x] Create `agents/` directory with `os.MkdirAll` and `0755` if it does not exist
  - [x] Call `agent.Scaffold(name)` to get template content
  - [x] Write file with `os.WriteFile` and permissions `0644`
  - [x] Print full file path to `cmd.OutOrStdout()`
  - [x] Register on `agentsCmd` in `init()`
- [x] Run tests — all agents init tests pass

---

## Phase 11: `axe agents edit <agent>` Command (Spec §2.11)

Write tests first (red), then implement (green):

- [x] Write `TestAgentsEdit_MissingEditor` — error when `$EDITOR` is unset
- [x] Write `TestAgentsEdit_MissingAgent` — error when agent file does not exist
- [x] Write `TestAgentsEdit_NoArgs` — usage error with no arguments
- [x] Implement `agentsEditCmd` in `cmd/agents.go`:
  - [x] Use `cobra.ExactArgs(1)` for argument validation
  - [x] Read `EDITOR` env var; if empty, return error `$EDITOR environment variable is not set` (exit code 1)
  - [x] Resolve path: `<config_dir>/agents/<name>.toml`
  - [x] Check if file exists; if not, return error `agent config not found: <name>` (exit code 2)
  - [x] Execute editor via `exec.Command` with stdin/stdout/stderr connected to parent process
  - [x] Propagate editor exit code on failure
  - [x] Register on `agentsCmd` in `init()`
- [x] Run tests — all agents edit tests pass

---

## Phase 12: Full Test Suite

- [x] Run `go test ./...` — all tests pass with 0 failures
- [x] Run `go vet ./...` — no issues
- [x] Run `go build` — binary compiles without errors

---

## Phase 13: Verification

- [x] Manual test: `axe agents list` with no agents — no output, exit 0
- [x] Manual test: `axe agents init test-agent` — creates TOML file, prints path
- [x] Manual test: `axe agents list` — shows `test-agent`
- [x] Manual test: `axe agents show test-agent` — shows Name and Model fields
- [x] Manual test: `axe agents init test-agent` again — error about existing file
- [x] Manual test: Edit the TOML to set a real model, re-run `axe agents show test-agent` — displays updated model
- [x] Manual test: `axe agents edit test-agent` with `$EDITOR` set — opens editor
- [x] Manual test: `axe agents edit test-agent` with `$EDITOR` unset — error message
- [x] Manual test: `axe agents show nonexistent` — error message
- [x] Verify `go.mod` only contains `spf13/cobra` and `BurntSushi/toml` as direct dependencies
- [x] Run `go mod tidy` — no changes

---

## Definition of Done

- [x] All checkboxes in Phases 1-13 are completed
- [x] All acceptance criteria from 002_agent_config_spec.md are met
- [x] Binary builds successfully with `go build`
- [x] All tests pass with `go test ./...`
- [x] Ready for M3: Single Agent Run implementation
