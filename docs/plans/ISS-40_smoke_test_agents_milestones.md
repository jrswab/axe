# Milestone Plan: Smoke Testing Axe with Axe Agents in CI

**GitHub Issue:** [#40 — Set up Axe Agents for Smoke Testing Axe](https://github.com/jrswab/axe/issues/40)
**Date:** 2026-03-25

---

## Section 1: Research Findings

### Codebase Structure & Relevant Files

#### Existing CI Workflows (`.github/workflows/`)

| File | Trigger | What it does |
|------|---------|-------------|
| `go.yml` | Push to `master`, PRs | Lint (golangci-lint), test (`go test -race -v ./...`), build. **No secrets configured** — cannot run live provider tests. |
| `release.yml` | Tag push (`v*`) | GoReleaser multi-platform build + GitHub Release + Docker image to GHCR. No test stage. |
| `docs.yml` | Manual dispatch | Builds mdBook docs, rsyncs to remote host. Irrelevant to smoke testing. |

Key observation: `go.yml` runs all Go tests including the existing `cmd/smoke_test.go` tests, but those tests are designed to work without API keys (they use `--dry-run`, mock servers, or strip API keys). The new smoke-test workflow is a **separate workflow** that exercises real provider calls.

#### Existing Smoke Test Infrastructure (`cmd/smoke_test.go`)

The file provides reusable patterns for subprocess-based testing:

| Function | Purpose |
|----------|---------|
| `setupSmokeEnv(t)` | Creates isolated XDG dir tree in `t.TempDir()`, returns `configDir`, `dataDir`, and env map. Does NOT call `t.Setenv` — env is for child process only. |
| `stripAPIKeys(env)` | Blanks `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OLLAMA_API_KEY` in the env map. |
| `runAxe(t, env, stdinData, args...)` | Builds binary once via `testutil.BuildBinary(t)`, runs as subprocess, returns `(stdout, stderr, exitCode)`. |
| `replaceOrAppendEnv` | Sets/replaces an env var in the `os.Environ()` slice. |
| `extractSection(stdout, header)` | Extracts content between a `---` header and the next `---` delimiter. |

Existing smoke tests (all pass without API keys):
1. `TestSmoke_Version` — `axe version` exits 0
2. `TestSmoke_ConfigPath` — `axe config path` outputs XDG config dir
3. `TestSmoke_ConfigInit` — creates files, idempotent, no overwrite
4. `TestSmoke_RunNonexistentAgent` — exit 2, agent name in stderr
5. `TestSmoke_RunDryRun` — `--dry-run` with basic fixture agent
6. `TestSmoke_BadModelFormat` — `--model no-slash-here` → exit 1
7. `TestSmoke_MissingAPIKey` — exit 2, "API key" in stderr
8. `TestSmoke_PipedStdinInDryRun` — piped stdin appears in dry-run output

These are Go-level subprocess tests. Issue #40 asks for a **GitHub Actions workflow** with shell-level tests — a different layer.

#### `--dry-run` Flag (Fully Implemented)

- **Flag:** `cmd/run.go` line 67 — `runCmd.Flags().Bool("dry-run", false, "Show resolved context without calling the LLM")`
- **Flow:** After resolving all context (agent config, model, workdir, files, skill, stdin, system prompt, memory), if `dryRun == true`, calls `printDryRun(...)` and returns immediately without any API call.
- **Output format** (`printDryRun`, lines 684–774):
  ```
  === Dry Run ===

  Model:    <provider>/<model>
  Workdir:  <path>
  Timeout:  <n>s
  Params:   temperature=<n>, max_tokens=<n>
  Budget:   <n> tokens (0 = unlimited)

  --- System Prompt ---
  <content>

  --- Skill ---
  <content or "(none)">

  --- Files (<n>) ---
  <paths or "(none)">

  --- User Message ---
  <content or "(default)">

  --- Memory ---        (only if cfg.Memory.Enabled)
  <content or "(none)">

  --- Tools ---
  <comma-joined list or "(none)">

  --- MCP Servers ---
  <lines or "(none)">

  --- Sub-Agents ---
  <comma-joined list with config, or "(none)">
  ```

This makes `--dry-run` perfect for the free CI job — validates the entire resolution pipeline without touching any API.

#### Agent Loading & Search Paths

`cmd/run.go` lines 124–130: `agent.BuildSearchDirs(flagAgentsDir, cwd)` then `agent.Load(agentName, searchDirs)`.

Search order:
1. `--agents-dir` flag (if provided)
2. Current working directory
3. XDG config dir (`$XDG_CONFIG_HOME/axe/agents/`)

For CI, we use `--agents-dir .github/smoke-agents` to load smoke-test agents without polluting XDG.

#### Registered Tools (`internal/tool/registry.go`)

`RegisterAll` registers these 7 built-in tools:

| Constant | Tool name |
|----------|-----------|
| `toolname.ListDirectory` | `list_directory` |
| `toolname.ReadFile` | `read_file` |
| `toolname.WriteFile` | `write_file` |
| `toolname.EditFile` | `edit_file` |
| `toolname.RunCommand` | `run_command` |
| `toolname.URLFetch` | `url_fetch` |
| `toolname.WebSearch` | `web_search` |

Additionally, `call_agent` is injected separately in `cmd/run.go` line 386 when `sub_agents` is configured. It is not part of the registry.

#### OpenCode Provider Routing (`internal/provider/opencode.go`)

The OpenCode provider routes based on the bare model name (after the `opencode/` prefix is stripped by the provider dispatcher):

| Model name prefix | Endpoint | Wire format |
|-------------------|----------|-------------|
| `claude-` | `POST {baseURL}/v1/messages` | Anthropic Messages API |
| `gpt-` | `POST {baseURL}/v1/responses` | OpenAI Responses API |
| anything else | `POST {baseURL}/v1/chat/completions` | OpenAI Chat Completions |

Default base URL: `https://opencode.ai/zen`

For `opencode/minimax-m2.5`, the model name `minimax-m2.5` doesn't start with `claude-` or `gpt-`, so it routes to the chat completions endpoint. This is the correct and cheapest path.

#### Existing Fixture Agents (`cmd/testdata/agents/`)

| File | Model | Config sections |
|------|-------|----------------|
| `basic.toml` | `openai/gpt-4o` | name, model only |
| `with_files.toml` | `openai/gpt-4o` | files glob patterns |
| `with_memory.toml` | `openai/gpt-4o` | `[memory]` block |
| `with_skill.toml` | `openai/gpt-4o` | skill path |
| `with_subagents.toml` | `anthropic/claude-sonnet-4-20250514` | sub_agents + `[sub_agents_config]` |
| `with_tools.toml` | `openai/gpt-4o` | tools list |

These are for Go-level tests. The smoke-test agents in `.github/smoke-agents/` are separate and purpose-built for CI.

#### Example Agents (`examples/`)

Three examples exist: `code-reviewer`, `commit-msg`, `summarizer`. All use `anthropic/claude-sonnet-4-20250514` with relative skill paths. These demonstrate the TOML + SKILL.md pattern but are too expensive for CI smoke tests.

#### User Message Precedence (`cmd/run.go` lines 303–309)

1. `-p` / `--prompt` flag (non-empty, non-whitespace)
2. Piped stdin (non-empty, non-whitespace)
3. Built-in default: `"Execute the task described in your instructions."`

### Key Decisions Made

| Decision | Rationale |
|----------|-----------|
| **Separate workflow file (`smoke-test.yml`)** | Issue explicitly states this is separate from `check.yml` (now `go.yml`). Different purpose: integration-level smoke testing vs unit tests. Different secret requirements. |
| **Anthropic model: `claude-3-5-haiku-20241022`** | Cheapest Anthropic model that supports tool calling. Newer and more accurate than `claude-haiku-3-20240307`. |
| **OpenAI model: `gpt-4o-mini`** | Cheapest OpenAI model with tool calling support. |
| **OpenCode model: `minimax-m2.5`** | Cheapest option via OpenCode Zen gateway. Routes to chat completions endpoint (not claude/gpt paths). Supports tool calling. |
| **OpenCode `minimax-m2.5` for all tool tests** | Single provider minimizes cost. Tool tests need tool-calling capability, and minimax-m2.5 supports it at the lowest cost. |
| **No Bedrock tests** | Deferred — cost needs evaluation. Will be tracked as a separate GitHub issue. |
| **No Ollama tests** | Requires local LM inference, not suitable for CI runners. |
| **`--agents-dir .github/smoke-agents`** | Loads smoke agents without XDG setup. Clean separation from fixture agents used by Go tests. |
| **Dry-run agents use `openai/gpt-4o`** | Model string doesn't matter for dry-run (no API call made). Using `openai/gpt-4o` matches existing fixture patterns. |
| **`TAVILY_API_KEY` as a secret** | Required for `web_search` tool test. User confirmed they will add all secrets to GitHub. |

### Approaches Considered & Rejected

| Approach | Why Rejected |
|----------|-------------|
| **Extend existing `cmd/smoke_test.go` with live provider tests** | Issue explicitly asks for a GitHub Actions workflow, not Go tests. The workflow provides better CI visibility (separate jobs per category, clear pass/fail in Actions UI). Go-level smoke tests remain for offline/dry-run validation. |
| **Single monolithic CI job** | Issue specifies individual jobs per test category so failures are isolated and easy to find without digging through logs. |
| **Use expensive models (claude-sonnet, gpt-4o)** | Cost. Smoke tests run on every push/PR. Cheapest models that support tool calling are sufficient. |
| **Reuse `cmd/testdata/agents/` fixtures** | Those are designed for Go test infrastructure (`testutil.SeedFixtureAgents`). Smoke-test agents need different models (cheap), different prompts (minimal), and live in a different directory (`.github/smoke-agents/`). |
| **Build tag gating (`//go:build live`)** | The existing `000_i9n_milestones.md` Phase 6 mentions this approach for Go-level live tests. Issue #40 is a different approach — shell-level workflow tests. Both can coexist. |

### Constraints & Assumptions

- **Secrets must be configured in GitHub:** `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENCODE_API_KEY`, `TAVILY_API_KEY`. User confirmed they will add these.
- **Workflow triggers match `go.yml`:** Push to `master` and PRs. This ensures smoke tests run on the same events as lint/test/build.
- **Each job builds `axe` from source:** `go build -o axe .` before running tests. No pre-built binary caching (keeps it simple, build is fast).
- **Stdout is the primary assertion target:** Axe's Unix philosophy means stdout is clean and pipeable. Tests assert on stdout content and exit codes.
- **LLM responses are non-deterministic:** Assertions must be loose — check for presence of key phrases (case-insensitive), not exact string matches. For tool tests, verify the tool was used (file exists, output contains expected data) rather than matching LLM prose.
- **Memory test requires two sequential runs:** First run creates a memory entry, second run's dry-run output shows the memory section is populated. This tests the full memory lifecycle.
- **Sub-agent test requires two agent files:** A parent agent with `sub_agents = ["cfg-sub-child"]` and a child agent. Both live in `.github/smoke-agents/`.
- **Skill test requires a SKILL.md file:** A minimal skill file in `.github/smoke-agents/skills/smoke-skill/SKILL.md` that instructs the LLM to include a marker phrase.
- **Files test requires a context file:** A small `.github/smoke-agents/smoke-context.txt` file whose content the LLM should reference.
- **`web_search` tool test depends on Tavily API availability:** If Tavily is down, this test will fail. This is acceptable — it's a smoke test, not a unit test.
- **Tool tests are inherently flaky due to LLM non-determinism:** The LLM might not use the tool as expected, or might produce unexpected output. Assertions should be as loose as possible while still validating the tool was exercised.

### Open Questions & Answers

| # | Question | Answer |
|---|----------|--------|
| 1 | Which Anthropic model for provider tests? | `claude-3-5-haiku-20241022` — cheapest with tool calling support. |
| 2 | Which OpenAI model for provider tests? | `gpt-4o-mini` — cheapest with tool calling support. |
| 3 | Which model for tool tests? | `opencode/minimax-m2.5` via OpenCode Zen — cheapest option with tool calling. |
| 4 | Include Bedrock? | No — deferred to a separate GitHub issue. Cost needs evaluation first. |
| 5 | Include Ollama? | No — requires local inference, not suitable for CI. |
| 6 | OpenCode model string format? | `opencode/minimax-m2.5`. The `opencode/` prefix selects the provider; `minimax-m2.5` is the bare model name routed to chat completions endpoint. |
| 7 | `web_search` needs `TAVILY_API_KEY`? | Yes. User confirmed they will add all required secrets to GitHub. |
| 8 | Should dry-run tests need API keys? | No. `--dry-run` resolves all context without calling any API. These tests are free. |
| 9 | How to load smoke agents in CI? | `./axe run --agents-dir .github/smoke-agents <agent-name>`. The `--agents-dir` flag adds the directory to the search path. |
| 10 | Should smoke tests block PRs? | Yes — same trigger as `go.yml` (push to master + PRs). Failures indicate real regressions. |

### Files to Create

| Path | Purpose |
|------|---------|
| `.github/workflows/smoke-test.yml` | New CI workflow with 5 jobs |
| `.github/smoke-agents/*.toml` | ~20+ minimal agent TOML files (one per test scenario) |
| `.github/smoke-agents/skills/smoke-skill/SKILL.md` | Minimal skill for the skill config test |
| `.github/smoke-agents/smoke-context.txt` | Small text file for the files config test |

### Smoke Agent Inventory

#### Provider Tests
| Agent file | Model | Purpose |
|------------|-------|---------|
| `provider-anthropic.toml` | `anthropic/claude-3-5-haiku-20241022` | Verify Anthropic provider connectivity |
| `provider-openai.toml` | `openai/gpt-4o-mini` | Verify OpenAI provider connectivity |
| `provider-opencode.toml` | `opencode/minimax-m2.5` | Verify OpenCode provider connectivity |

#### Tool Tests (all use `opencode/minimax-m2.5`)
| Agent file | Tool enabled | Verification approach |
|------------|-------------|----------------------|
| `tool-list-dir.toml` | `list_directory` | Prompt to list cwd; assert non-empty stdout, exit 0 |
| `tool-read-file.toml` | `read_file` | Create known file, prompt to read it; assert content in stdout |
| `tool-write-file.toml` | `write_file` | Prompt to write specific file; assert file exists after run |
| `tool-edit-file.toml` | `edit_file` | Create file, prompt to edit it; assert edit applied |
| `tool-run-cmd.toml` | `run_command` | Prompt to run `echo hello-smoke`; assert `hello-smoke` in stdout |
| `tool-url-fetch.toml` | `url_fetch` | Prompt to fetch `https://example.com`; assert `Example Domain` in stdout |
| `tool-web-search.toml` | `web_search` | Prompt to search; assert exit 0, non-empty stdout |

#### Config Section Tests (all use `opencode/minimax-m2.5` unless noted)
| Agent file | Config section tested | Verification approach |
|------------|----------------------|----------------------|
| `cfg-basic.toml` | name, model only | Run with prompt; assert non-empty response, exit 0 |
| `cfg-skill.toml` | `skill` path | Skill instructs marker phrase; assert phrase in stdout |
| `cfg-memory.toml` | `[memory]` block | Run twice; second run's dry-run shows memory populated |
| `cfg-files.toml` | `files` list | File content injected; assert LLM references it |
| `cfg-subagents.toml` | `sub_agents` + config | Parent delegates to child; assert combined result |
| `cfg-sub-child.toml` | (child for above) | Simple responder agent |
| `cfg-params.toml` | `[params]` block | Set temperature/max_tokens; assert no crash, valid response |

#### Piped Input Tests
| Agent file | Model | Purpose |
|------------|-------|---------|
| `pipe-basic.toml` | `opencode/minimax-m2.5` | Baseline agent for stdin piping tests |

#### Dry-Run Tests (no API keys needed)
| Agent file | Model | Config sections tested |
|------------|-------|-----------------------|
| `dry-basic.toml` | `openai/gpt-4o` | Baseline dry-run |
| `dry-tools.toml` | `openai/gpt-4o` | `--- Tools ---` section populated |
| `dry-skill.toml` | `openai/gpt-4o` | `--- Skill ---` section not `(none)` |
| `dry-memory.toml` | `openai/gpt-4o` | `--- Memory ---` section present |
| `dry-files.toml` | `openai/gpt-4o` | `--- Files ---` section not `(none)` |

---

## Section 2: Milestones

Status key: `[ ]` not started · `[-]` in progress · `[x]` done

- [x] 043: Create smoke-test agent TOML files, skill file, and context file in `.github/smoke-agents/`
- [ ] 044: Create `dry-run` CI job — validate `--dry-run` output for each agent type (no API keys required)
- [ ] 045: Create `providers` CI job — verify Anthropic, OpenAI, and OpenCode connectivity with minimal prompts
- [x] 046: Create `tools` CI job — exercise all 7 built-in tools via OpenCode/minimax-m2.5
- [ ] 047: Create `config-sections` CI job — verify TOML config parsing for basic, skill, memory, files, sub_agents, and params
- [ ] 048: Create `piped-input` CI job — verify stdin piping works across multiple scenarios
- [ ] 049: Assemble all jobs into `.github/workflows/smoke-test.yml` with proper secret injection and triggers
