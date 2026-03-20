# 038 — Local/Per-Repo Agent Directory: Implementation Guide

**Spec:** `docs/plans/038_local_agent_directory_spec.md`

---

## Section 1: Context Summary

Axe currently loads all agent TOML configs exclusively from `$XDG_CONFIG_HOME/axe/agents/`. Issue #28 adds support for a local per-repo directory (`./axe/agents/`) so that agent configs can live alongside the code they operate on — useful for git hooks, CI pipelines, and team-shared configs checked into version control. The resolution order is: `--agents-dir` flag (highest) → `./axe/agents/` auto-discovered from the applicable working directory → global XDG config (fallback). Non-existent directories are silently skipped. All 7 call sites of `agent.Load()` and `agent.List()` must be updated. The `ExecuteOptions` struct in `internal/tool/tool.go` must carry the agents-dir value so sub-agent delegation respects the same resolution order using the parent's resolved workdir as the auto-discovery base.

---

## Section 2: Implementation Checklist

### Phase 1 — Core: `agent.Load` and `agent.List` accept ordered search dirs

- [x] **1.1** `internal/agent/agent.go`: Extract private helper `loadFromPath(path string) (*AgentConfig, error)` that reads, TOML-decodes, and validates a single file path. Returns the existing error messages unchanged.

- [x] **1.2** `internal/agent/agent.go`: Change `Load(name string)` signature to `Load(name string, searchDirs []string) (*AgentConfig, error)`. The function iterates `searchDirs` in order, calling `loadFromPath` for `filepath.Join(dir, name+".toml")`. Skips any dir where the file does not exist (`os.IsNotExist`). Falls back to the global XDG dir (`xdg.GetConfigDir()` + `/agents`) as the final entry. Returns `"agent config not found: {name}"` only if no dir yields the file.

- [x] **1.3** `internal/agent/agent.go`: Change `List(searchDirs []string) ([]AgentConfig, error)` to read `.toml` files from all dirs in `searchDirs` plus the global XDG dir (appended last). Deduplicates by agent name — first occurrence wins (earlier dir = higher precedence). Invalid files are silently skipped (existing behavior). Returns a merged flat slice.

- [x] **1.4** `internal/agent/agent_test.go`: `TestLoad_LocalDirTakesPrecedence` — agent with same name in local dir and global dir; local version is returned.

- [x] **1.5** `internal/agent/agent_test.go`: `TestLoad_FallsBackToGlobal` — agent not in local dir; global version is returned.

- [x] **1.6** `internal/agent/agent_test.go`: `TestLoad_NonExistentSearchDir_Skipped` — non-existent dir in `searchDirs` is silently skipped; global version returned.

- [x] **1.7** `internal/agent/agent_test.go`: `TestLoad_EmptySearchDirs_UsesGlobal` — `searchDirs = nil` or `[]string{}` falls back to global only.

- [x] **1.8** `internal/agent/agent_test.go`: `TestLoad_MultipleSearchDirs_FirstWins` — two local dirs both contain the agent; first dir's version is returned.

- [x] **1.9** `internal/agent/agent_test.go`: `TestList_MergesLocalAndGlobal` — agents in local dir and global dir are merged into one list.

- [x] **1.10** `internal/agent/agent_test.go`: `TestList_LocalWinsOnNameCollision` — same agent name in local and global; local version appears in result, global version does not.

- [x] **1.11** `internal/agent/agent_test.go`: `TestList_NonExistentSearchDir_Skipped` — non-existent dir in `searchDirs` produces no error and no agents from that dir.

- [x] **1.12** `internal/agent/agent_test.go`: `TestList_EmptySearchDirs_UsesGlobal` — `searchDirs = nil` returns only global agents (backward compat).

- [x] **1.13** Run `go test ./internal/agent/...` — all tests pass.

---

### Phase 2 — Helper: build search dirs from flag + auto-discovery

- [x] **2.1** `internal/agent/agent.go`: Add exported function `BuildSearchDirs(flagDir string, baseDir string) []string`. Builds the ordered search dirs list:
  1. If `flagDir` is non-empty, append it.
  2. Append `filepath.Join(baseDir, "axe", "agents")` (auto-discovery path).
  Returns the slice (may be empty; global is always appended by `Load`/`List` themselves). Does NOT check existence — callers pass whatever they have; `Load`/`List` skip non-existent dirs.

- [x] **2.2** `internal/agent/agent_test.go`: `TestBuildSearchDirs_FlagAndBase` — both flag and base provided; returns `[flagDir, baseDir/axe/agents]`.

- [x] **2.3** `internal/agent/agent_test.go`: `TestBuildSearchDirs_NoFlag` — empty flag; returns `[baseDir/axe/agents]`.

- [x] **2.4** `internal/agent/agent_test.go`: `TestBuildSearchDirs_EmptyFlag_EmptyBase` — both empty; returns `["axe/agents"]` (relative path from empty base is `filepath.Join("", "axe", "agents")`).

- [x] **2.5** Run `go test ./internal/agent/...` — all tests pass.

---

### Phase 3 — `cmd/run.go`: wire `--agents-dir` flag

- [x] **3.1** `cmd/run.go` `init()`: Add flag `runCmd.Flags().String("agents-dir", "", "Additional agents directory to search before global config")`.

- [x] **3.2** `cmd/run.go` `runAgent()`: After resolving `workdir` (step 6, line ~132), read `flagAgentsDir, _ := cmd.Flags().GetString("agents-dir")` and build `searchDirs := agent.BuildSearchDirs(flagAgentsDir, workdir)`. Pass `searchDirs` to `agent.Load(agentName, searchDirs)` at line ~102.

- [x] **3.3** `cmd/run.go` `executeToolCalls()`: Add `AgentsDir string` and `AgentsBase string` to the function signature (or thread via `ExecuteOptions` — see Phase 5). Pass `flagAgentsDir` and `workdir` through so sub-agents can use the same resolution.

- [x] **3.4** `cmd/run.go`: Existing tests in `cmd/run_test.go` or `cmd/run_integration_test.go` must still pass unchanged (backward compat: no `--agents-dir` = same behavior as before).

---

### Phase 4 — `cmd/agents.go`: wire `--agents-dir` flag to all subcommands

- [x] **4.1** `cmd/agents.go` `init()`: Add `agentsCmd.PersistentFlags().String("agents-dir", "", "Additional agents directory to search before global config")`. Persistent so all subcommands inherit it.

- [x] **4.2** `cmd/agents.go` `agentsListCmd`: Read `flagAgentsDir` from the persistent flag. Get cwd via `os.Getwd()`. Build `searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)`. Pass to `agent.List(searchDirs)`.

- [x] **4.3** `cmd/agents.go` `agentsShowCmd`: Same pattern — read flag, get cwd, build `searchDirs`, pass to `agent.Load(args[0], searchDirs)`.

- [x] **4.4** `cmd/agents.go` `agentsInitCmd`: Determine write target:
  1. If `flagAgentsDir` non-empty → use it.
  2. Else if `filepath.Join(cwd, "axe", "agents")` exists → use it.
  3. Else → use `filepath.Join(configDir, "agents")` (global).
  Create the target directory with `os.MkdirAll(targetDir, 0755)`. Check for existing file before writing. Print the final path to stdout.

- [x] **4.5** `cmd/agents.go` `agentsEditCmd`: Read flag, get cwd, build `searchDirs`, find the agent file path by iterating `searchDirs` + global dir (same order as `Load`). Open the found path in `$EDITOR`. Return error if not found.

---

### Phase 5 — `internal/tool/tool.go`: propagate agents-dir to sub-agents

- [x] **5.1** `internal/tool/tool.go` `ExecuteOptions`: Add two fields:
  ```go
  AgentsDir  string // value of --agents-dir flag (may be empty)
  AgentsBase string // parent agent's resolved workdir (for auto-discovery)
  ```

- [x] **5.2** `internal/tool/tool.go` `ExecuteCallAgent()`: After resolving the sub-agent's `workdir` (step 8, line ~146), build `searchDirs := agent.BuildSearchDirs(opts.AgentsDir, opts.AgentsBase)` and pass to `agent.Load(agentName, searchDirs)`.

- [x] **5.3** `internal/tool/tool.go` `runConversationLoop()` (line ~395, `subOpts` construction): Propagate `AgentsDir` and `AgentsBase` from `opts` into `subOpts`. Use the sub-agent's resolved `workdir` as the new `AgentsBase` for the next level.

- [x] **5.4** `cmd/run.go` `executeToolCalls()`: Populate `execOpts.AgentsDir = flagAgentsDir` and `execOpts.AgentsBase = workdir` when constructing `tool.ExecuteOptions` (line ~677).

- [x] **5.5** `internal/tool/tool_test.go`: `TestExecuteCallAgent_LocalDirPropagated` — set `opts.AgentsBase` to a temp dir containing `axe/agents/sub-agent.toml`; verify the sub-agent is loaded from there rather than global. Use the existing mock server infrastructure.

- [x] **5.6** Run `go test ./internal/tool/...` — all tests pass.

---

### Phase 6 — `cmd/gc.go`: wire `--agents-dir` flag

- [x] **6.1** `cmd/gc.go` `init()`: Add `gcCmd.Flags().String("agents-dir", "", "Additional agents directory to search before global config")`.

- [x] **6.2** `cmd/gc.go` `runGC()`: Read `flagAgentsDir`. Get cwd via `os.Getwd()`. Build `searchDirs := agent.BuildSearchDirs(flagAgentsDir, cwd)`. Pass to both `agent.Load()` (in `runSingleAgentGC`) and `agent.List()` (in `runAllAgentsGC`). Thread `searchDirs` through as a parameter to both functions.

- [x] **6.3** `cmd/gc.go` `runSingleAgentGC(cmd, agentName string, searchDirs []string)`: Update signature to accept `searchDirs`. Pass to `agent.Load(agentName, searchDirs)`.

- [x] **6.4** `cmd/gc.go` `runAllAgentsGC(cmd, searchDirs []string)`: Update signature to accept `searchDirs`. Pass to `agent.List(searchDirs)`. Pass `searchDirs` to each `runSingleAgentGC` call.

- [x] **6.5** Run `go test ./cmd/...` — all tests pass.

---

### Phase 7 — Final validation

- [ ] **7.1** Run `go build ./...` — no compilation errors.

- [ ] **7.2** Run `go test ./...` — all tests pass.

- [ ] **7.3** Run `go vet ./...` — no issues.

- [ ] **7.4** Manual smoke: create `./axe/agents/local-test.toml` in a temp dir with a valid minimal config. Run `axe agents list` from that dir — `local-test` appears. Run `axe agents show local-test` — shows config. Run `axe run local-test` (with a mock or dry-run) — loads from local dir.

- [ ] **7.5** Manual smoke: create the same agent name in both `./axe/agents/` and global config with different descriptions. Run `axe agents show <name>` — local version's description is shown.

- [ ] **7.6** Manual smoke: run `axe agents list` from a dir with no `./axe/agents/` — output identical to current behavior.
