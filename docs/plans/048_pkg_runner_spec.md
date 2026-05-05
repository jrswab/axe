# Spec: Extract core execution logic into `pkg/runner`

## Section 1: Context & Constraints

### Milestone: ISS-80 — Public `pkg/runner` library

> **Problem:** Axe currently encapsulates all agent execution logic inside `internal/` packages and the Cobra command layer in `cmd/run.go`. Because these packages are `internal`, they cannot be imported by other modules.
>
> **Goal:** Extract the core agent execution pipeline into a **public** Go package so that external tools can call Axe programmatically while keeping the CLI experience unchanged for existing users.
>
> **Proposed Approach (Option B):** Create a single `pkg/runner` package with a curated public API. All existing `internal/` packages remain internal. The Cobra command in `cmd/run.go` becomes a thin wrapper that translates CLI flags into `runner.Options` and calls `runner.Run()`.
>
> **Constraints:** Zero breaking changes for existing CLI users. Preserve all functionality: streaming, tool execution, MCP, sub-agents, artifacts, memory, budget tracking, etc.

### Research Findings

#### Codebase structure relevant to this milestone

- `cmd/run.go` contains the entire orchestration pipeline (~900 lines). It handles: agent loading, context resolution, provider setup, conversation loop, tool call execution, memory, artifacts, MCP, streaming, JSON output, budget tracking, dry-run, verbose logging, and exit code mapping.
- All supporting packages live in `internal/`: `agent`, `provider`, `tool`, `resolve`, `config`, `memory`, `artifact`, `budget`, `mcpclient`, `refusal`, `xdg`.
- The `provider` package defines the core abstraction (`Provider` interface, `Request`/`Response` types, `ToolCall`/`ToolResult`/`Message` types).
- The `tool` package uses a registry pattern. `RegisterAll()` adds built-in tools. `Registry.Resolve()` maps tool names to LLM tool definitions. `Registry.Dispatch()` executes tools.
- The `mcpclient` package manages external tool servers. `mcpclient.Router` namespaces tools by server name and dispatches calls.
- Conversation loop safety limit: 50 turns (`maxConversationTurns`).
- Sub-agent depth limit: default 3, hard max 5.
- Tool execution supports parallel (default) and sequential modes.
- Provider retry is implemented as a decorator (`provider.RetryProvider`).
- Streaming is opt-in per agent TOML (`stream = true`) or CLI flag (`--stream`). Not all providers support streaming.
- Budget tracking uses `budget.BudgetTracker`. Exceeding budget returns exit code 4.

#### Decisions already made

- **Option B selected** over Option A (library-ify all packages) and Option C (hybrid). Rationale: a single curated public API minimizes maintenance burden while still enabling programmatic use.
- `internal/` packages stay internal. `pkg/runner` is the only public package.
- CLI backward compatibility is a hard constraint — same flags, same behavior, same exit codes.

#### Constraints and assumptions

- The runner must work with both streaming and non-streaming providers.
- Tool calls may require parallel or sequential execution.
- Sub-agent delegation is opaque — parents see only the final string result.
- MCP server connections are per-run and closed afterward.
- Memory entries are loaded into the system prompt and appended only on successful completion.
- Artifact directories may be user-specified (persistent) or auto-generated (temporary, cleaned up unless `--keep-artifacts`).
- The runner must support dry-run mode for programmatic consumers as well as CLI users.

#### Open questions resolved

- **Public interface style:** Single `Run()` function with `Options` and `Result` types. No struct-based runner instance. Keeps API surface minimal.
- **I/O handling:** The public API accepts `io.Writer` for stdout and stderr. The CLI passes `os.Stdout`/`os.Stderr`; library consumers pass their own writers. Content goes to stdout, verbose/debug to stderr.
- **Configuration source:** The runner accepts an agent name (loads TOML from disk) plus override fields. Programmatic config construction is out of scope for this milestone; consumers who need it can write TOML to a temp dir and point `AgentsDir` at it.
- **Dry-run:** Supported in the library API, not just CLI. Returns a `Result` with `DryRun: true` and resolved context populated, but no LLM call.

---

## Section 2: Requirements

### 2.1 Package Boundary

- **`pkg/runner`** is the sole public package introduced by this milestone.
- All existing packages in `internal/` remain unchanged in their visibility.
- `cmd/run.go` is refactored into a thin Cobra wrapper. The wrapper is responsible only for: flag parsing, help text, version info, constructing `runner.Options`, calling `runner.Run()`, mapping errors to exit codes, and formatting dry-run output.

### 2.2 Public API

The public API consists of:

- A `Run` function that accepts a `context.Context` and an `Options` struct, and returns a `*Result` and an `error`.
- An `Options` struct that exposes all configurable inputs needed to execute an agent run.
- A `Result` struct that captures all outputs of a completed run.

#### Options

The `Options` struct must expose the following fields with the same semantics as their CLI flag counterparts:

- **Agent loading:** agent name, optional agent search directories.
- **Overrides:** model, skill path, working directory, inline prompt, timeout, max-tokens (budget), artifact directory, keep-artifacts, stream, dry-run, verbose, JSON mode.
- **I/O:** stdout writer, stderr writer. When nil, defaults to `os.Stdout` and `os.Stderr`.

Override resolution order must match the CLI exactly: flag values (via Options fields) override TOML config; TOML config overrides defaults. A zero value in Options means "not overridden, use agent config or default."

#### Result

The `Result` struct must contain at minimum:

- The LLM's final content string.
- Input and output token counts (cumulative across all turns).
- Stop reason.
- Total number of tool calls executed.
- Per-tool-call details (name, input, output, turn, duration, error status) when requested.
- Run duration in milliseconds.
- Refusal detection result.
- Retry attempt count.
- Budget state (max, used, exceeded).
- Artifact tracking information (path, agent name, size for each artifact created).
- A boolean indicating whether the run was a dry-run.

### 2.3 Execution Pipeline

`Run` must execute the following pipeline, in order:

1. **Load agent configuration** from the agent name and search directories. Fail with a config error if the agent cannot be found or is invalid.
2. **Apply overrides** from Options onto the loaded agent config, preserving resolution order.
3. **Resolve working directory** from flag override → TOML config → current working directory.
4. **Resolve artifact directory** using the same hierarchy as the CLI: flag override → TOML `artifacts.dir` → auto-generation (if `artifacts.enabled`) → inactive. Create the directory if needed. Set `AXE_ARTIFACT_DIR` for the duration of the run. Clean up auto-generated directories on exit unless `keep_artifacts` is true.
5. **Resolve file globs** against the working directory.
6. **Load skill** from the resolved skill path.
7. **Read user message** with this precedence: prompt override → stdin → default message.
8. **Build system prompt** from system prompt + skill + files. Append memory entries if memory is enabled.
9. **Load and validate global config** for API keys, base URLs, and region settings.
10. **Resolve provider** from the model string (`provider/model-name`). Validate that the provider is supported and that required credentials are present.
11. **Create provider instance** and wrap with retry decorator.
12. **Build tool registry**: register all built-in tools, resolve configured tools from agent config, inject `call_agent` if sub-agents are configured and depth < max.
13. **Connect MCP servers** if configured. Namespace tools by server name. Skip any that collide with built-ins. Close connections on exit.
14. **Build provider request** with system prompt, user message, model parameters (temperature, max_tokens, response format), and tools.
15. **Execute**:
    - If no tools and no streaming: single `Send()` call.
    - If streaming is enabled and the provider supports it: use `SendStream()`, write text chunks incrementally to stdout, and reconstruct the response.
    - If tools are present: conversation loop. Maximum 50 turns. On each turn, send request, receive response, check budget, execute tool calls, append assistant + tool messages, repeat until no tool calls remain or budget exceeded or turn limit reached.
16. **Tool execution** must support both parallel and sequential modes based on agent config. Tool call results are truncated to 1024 bytes when appended to the conversation.
17. **Budget tracking**: accumulate input and output tokens after each provider call. If budget is exceeded, stop before the next LLM call. Return budget exceeded state in Result.
18. **Memory**: on successful completion (no provider errors), append an entry with the original user message and final content.
19. **Artifact tracking**: track all files written via `write_file` during the run.
20. **Dry-run**: if enabled, skip all provider calls. Return a Result with `DryRun: true` and resolved context populated.
21. **JSON output**: if enabled, the Result is marshaled and written to stdout. Otherwise, content is written directly.
22. **Verbose logging**: if enabled, write diagnostic info (model, workdir, skill, file count, prompt source, timeout, params, memory state, per-turn summaries, duration, tokens, stop reason) to stderr.

### 2.4 I/O Contract

- **Stdout** receives: LLM content (streaming or batched), dry-run formatted output, JSON envelope.
- **Stderr** receives: verbose diagnostics, warnings (memory load failures, artifact cleanup failures), budget exceeded notices.
- When `JSON` is true and `Stream` is true, text chunks must still be written to stdout incrementally, but the final JSON envelope must not duplicate the content in its raw form. The CLI wrapper handles this by passing a writer; the runner does not need to know about JSON mode during streaming.
- When streaming is enabled but the provider does not support it, streaming must be silently disabled and a non-streaming call made instead.

### 2.5 Error Contract

`Run` must return errors with enough context that the CLI wrapper (and other consumers) can map them to appropriate actions:

- **Config errors** (missing agent, invalid TOML, validation failures, missing API key, missing workdir, MCP connection failures due to config): return a distinct error type or sentinel so the CLI wrapper can exit with code 2.
- **Runtime errors** (provider call failures, tool execution failures, JSON marshal failure, budget exceeded): return a distinct error type or sentinel so the CLI wrapper can exit with code 1 (or 4 for budget exceeded).
- **Provider errors** (auth, rate limit, timeout, server, bad request): must be wrapped or preserved so the caller can inspect the `ErrorCategory` and map to exit code 3.
- **Conversation turn exhaustion** (50 turns with pending tool calls): return as a runtime error.

### 2.6 CLI Wrapper Behavior

The refactored `cmd/run.go` must:

- Parse all existing flags unchanged.
- Construct `runner.Options` from flag values.
- Call `runner.Run()` with `cmd.Context()`.
- Map returned errors to exit codes: 0 (success), 1 (runtime), 2 (config), 3 (provider/auth/rate-limit/timeout/server), 4 (budget exceeded).
- Preserve all existing flag semantics, help text, and default values.
- Preserve dry-run output formatting.
- Preserve JSON envelope output formatting.
- Preserve verbose stderr output.

### 2.7 Backward Compatibility

- No CLI flags may be removed, renamed, or change semantics.
- No changes to TOML config schema.
- No changes to exit codes.
- No changes to stdout/stderr output format in any mode (default, `--json`, `--dry-run`, `--verbose`).
- No changes to environment variable handling (`AXE_ARTIFACT_DIR`, API key env vars).
- Existing tests for `cmd/` must continue to pass without modification (or with only import path updates if the runner package changes).

### 2.8 Edge Cases

The following edge cases must be handled exactly as the current implementation handles them:

- **Empty agent name:** config error.
- **Agent not found in any search directory:** config error with "agent config not found" message.
- **Missing model field in TOML:** validation error (config error).
- **Invalid model format** (missing `/`, empty provider or model): config error.
- **Missing API key for non-Ollama, non-Bedrock provider:** config error with env var hint.
- **Bedrock missing region:** config error.
- **Unsupported provider:** runtime error from `provider.New()`.
- **Timeout of zero or negative:** config error.
- **Sub-agent depth > 5:** validation error (config error).
- **Memory path resolution failure:** warning to stderr, not fatal.
- **Memory entry load/append failure:** warning to stderr, not fatal.
- **Artifact directory creation failure:** runtime error.
- **Auto-generated artifact directory cleanup failure:** warning to stderr.
- **MCP server connection failure:** config error if due to env var or transport; otherwise runtime error.
- **MCP tool listing failure:** runtime error.
- **MCP tool registration failure:** config error.
- **Tool call execution failure:** returned as tool result to LLM, not fatal.
- **Budget exceeded during conversation:** stop before next LLM call, return budget exceeded in Result.
- **Max turns exceeded with pending tool calls:** runtime error.
- **Streaming provider error:** map to appropriate error category.
- **Retry exhaustion:** return the last provider error.
- **Stdin read failure:** runtime error.
- **Empty or whitespace-only prompt flag:** treated as absent.

### 2.9 Behaviors to Test

Critical path tests (highest priority):

1. Single-shot execution with no tools, no streaming — returns correct content and token counts.
2. Conversation loop with one tool call — message history is built correctly, tool result is appended, final response is returned.
3. Parallel vs sequential tool execution — multiple tool calls in one turn execute in the correct mode.
4. Streaming — text is written incrementally to stdout, final Result has correct aggregated content.
5. Budget tracking — cumulative token tracking, exceeded detection, and Result state.
6. Sub-agent delegation — `call_agent` tool is injected, depth limits are enforced, opaqueness is preserved.
7. MCP tool integration — tools are namespaced, conflicts with built-ins are skipped, dispatch routes to correct server.
8. Error mapping — config errors, provider errors, runtime errors, and budget exceeded are all distinguishable by the caller.
9. Dry-run — skips LLM call, returns resolved context in Result.
10. CLI wrapper — all flags map correctly to Options, exit codes are preserved.

Secondary tests (lower priority, may be added incrementally):

11. Memory load and append — entries loaded into system prompt, appended on success only.
12. Artifact tracking — files written via tools are tracked in Result.
13. Retry wrapper — retry attempts are counted and returned in Result.
14. Refusal detection — boolean returned in Result.
15. JSON output mode — envelope structure matches current CLI output.
