# 037 — Token Budget Limits: Implementation Guide

**Spec:** `docs/plans/037_token_budget_limits_spec.md`

---

## Section 1: Context Summary

Agents that fan out via sub-agents can incur runaway costs because sub-agent token usage is invisible to the parent. This implementation adds a `[budget]` TOML config block and `--max-tokens` CLI flag that create a shared, thread-safe token budget tracker. The tracker is passed from parent to all sub-agents via `ExecuteOptions`, accumulates `InputTokens + OutputTokens` from every `Provider.Send()` response across the entire run, and hard-stops the conversation loop when the limit is exceeded — returning the partial result with exit code 4. Budget defaults to 0 (unlimited) for full backward compatibility. Cost-based budgets, per-sub-agent budgets, and pre-exhaustion warnings are explicitly out of scope.

---

## Section 2: Implementation Checklist

### Phase 1: Budget Tracker (no dependencies)

- [x] **Create `internal/budget/budget.go`: `BudgetTracker` struct and constructor `New(maxTokens int)`.**
  Define a struct with `sync.Mutex`, `maxTokens int`, and `usedTokens int` fields. `New()` returns a `*BudgetTracker`. When `maxTokens` is 0, the tracker represents unlimited budget.

- [x] **Create `internal/budget/budget.go`: `Add(input, output int)` method.**
  Locks the mutex, adds `input + output` to `usedTokens`. No-op when `maxTokens` is 0.

- [x] **Create `internal/budget/budget.go`: `Exceeded() bool` method.**
  Locks the mutex, returns `maxTokens > 0 && usedTokens >= maxTokens`. Returns `false` when `maxTokens` is 0.

- [x] **Create `internal/budget/budget.go`: `Used() int` method.**
  Locks the mutex, returns `usedTokens`.

- [x] **Create `internal/budget/budget.go`: `Max() int` method.**
  Returns `maxTokens` (no lock needed, immutable after construction).

- [x] **Create `internal/budget/budget_test.go`: Test `New(0)` unlimited — `Exceeded()` returns false after `Add()`.**
  Call `New(0)`, call `Add(5000, 5000)`, assert `Exceeded()` is `false`, assert `Used()` is `10000`, assert `Max()` is `0`.

- [x] **Create `internal/budget/budget_test.go`: Test `New(100)` within budget — `Exceeded()` returns false.**
  Call `New(100)`, call `Add(30, 20)`, assert `Exceeded()` is `false`, assert `Used()` is `50`.

- [x] **Create `internal/budget/budget_test.go`: Test `New(100)` exceeded — `Exceeded()` returns true.**
  Call `New(100)`, call `Add(50, 30)`, call `Add(20, 10)`, assert `Exceeded()` is `true`, assert `Used()` is `110`.

- [x] **Create `internal/budget/budget_test.go`: Test `New(100)` exactly at limit — `Exceeded()` returns true.**
  Call `New(100)`, call `Add(60, 40)`, assert `Exceeded()` is `true`, assert `Used()` is `100`.

- [x] **Create `internal/budget/budget_test.go`: Test concurrent `Add()` calls are safe.**
  Call `New(1000000)`, launch 100 goroutines each calling `Add(100, 100)`, wait for all to finish, assert `Used()` is `20000`.

### Phase 2: Agent Config (depends on Phase 1)

- [x] **Modify `internal/agent/agent.go`: Add `BudgetConfig` struct.**
  Add above `AgentConfig` (after `RetryConfig`, before `MCPServerConfig`): `type BudgetConfig struct { MaxTokens int \`toml:"max_tokens"\` }`.

- [x] **Modify `internal/agent/agent.go`: Add `Budget` field to `AgentConfig` struct.**
  Add `Budget BudgetConfig \`toml:"budget"\`` to the `AgentConfig` struct, after the `Retry RetryConfig` field (line 68).

- [x] **Modify `internal/agent/agent.go`: Add budget validation to `Validate()` function.**
  After the retry validation block (after line 144, before `return nil`), add: `if cfg.Budget.MaxTokens < 0 { return errors.New("budget.max_tokens must be non-negative") }`.

- [x] **Modify `internal/agent/agent.go`: Add `[budget]` block to `Scaffold()` template.**
  Add the following commented block to the template string in `Scaffold()`, after the `[retry]` block (after line 272): `# [budget]\n# max_tokens = 0\n`.

- [x] **Modify `internal/agent/agent_test.go`: Test `Validate()` rejects negative `budget.max_tokens`.**
  Create `AgentConfig` with `Name: "test"`, `Model: "openai/gpt-4o"`, `Budget: BudgetConfig{MaxTokens: -1}`. Assert `Validate()` returns error `"budget.max_tokens must be non-negative"`.

- [x] **Modify `internal/agent/agent_test.go`: Test `Validate()` accepts zero `budget.max_tokens`.**
  Create `AgentConfig` with `Name: "test"`, `Model: "openai/gpt-4o"`, `Budget: BudgetConfig{MaxTokens: 0}`. Assert `Validate()` returns `nil`.

- [x] **Modify `internal/agent/agent_test.go`: Test `Validate()` accepts positive `budget.max_tokens`.**
  Create `AgentConfig` with `Name: "test"`, `Model: "openai/gpt-4o"`, `Budget: BudgetConfig{MaxTokens: 10000}`. Assert `Validate()` returns `nil`.

- [x] **Modify `internal/agent/agent_test.go`: Test TOML parsing of `[budget]` block.**
  Use `tomlDecode` with input containing `[budget]\nmax_tokens = 5000`. Assert `cfg.Budget.MaxTokens` is `5000`.

- [x] **Modify `internal/agent/agent_test.go`: Test TOML parsing with absent `[budget]` block.**
  Use `tomlDecode` with minimal TOML (name + model only). Assert `cfg.Budget.MaxTokens` is `0`.

- [x] **Modify `internal/agent/agent_test.go`: Test `Load()` with `[budget]` block in TOML file.**
  Write a TOML file with `[budget]\nmax_tokens = 8000`, call `Load()`, assert `cfg.Budget.MaxTokens` is `8000`.

- [x] **Modify `internal/agent/agent_test.go`: Test `Scaffold()` includes commented `[budget]` block.**
  Call `Scaffold("test")`, assert output contains `"# [budget]"` and `"# max_tokens = 0"`.

### Phase 3: Sub-Agent Budget Propagation (depends on Phase 1)

- [x] **Modify `internal/tool/tool.go`: Add `BudgetTracker` field to `ExecuteOptions` struct.**
  Add `BudgetTracker *budget.BudgetTracker` to the `ExecuteOptions` struct (line 37), and add `"github.com/jrswab/axe/internal/budget"` to imports.

- [x] **Modify `internal/tool/tool.go`: Add budget check and tracking to `runConversationLoop()`.**
  At the top of the `for` loop body (line 343, before `prov.Send()`), check `if opts.BudgetTracker != nil && opts.BudgetTracker.Exceeded() { return resp, nil }` (where `resp` is the last response, or `nil` on first iteration — use a variable initialized to `nil` before the loop). After each successful `prov.Send()` call, add `if opts.BudgetTracker != nil { opts.BudgetTracker.Add(resp.InputTokens, resp.OutputTokens) }`. After adding tokens, check `if opts.BudgetTracker.Exceeded() && len(resp.ToolCalls) > 0 { return resp, nil }` to stop before executing tools when budget is blown.

- [x] **Modify `internal/tool/tool.go`: Propagate `BudgetTracker` in `runConversationLoop()` sub-agent calls.**
  In the `subOpts` construction inside `runConversationLoop()` (lines 373-383), add `BudgetTracker: opts.BudgetTracker` to the `ExecuteOptions` literal.

### Phase 4: CLI Flag and Main Loop Budget Enforcement (depends on Phases 1-3)

- [x] **Modify `cmd/run.go`: Add `--max-tokens` flag to `runCmd`.**
  In the `init()` function (after line 58), add: `runCmd.Flags().Int("max-tokens", 0, "Maximum total tokens (input+output) for the entire run (0 = unlimited)")`.

- [x] **Modify `cmd/run.go`: Resolve effective budget and create `BudgetTracker` in `runAgent()`.**
  After reading flags (after line 203), read the `--max-tokens` flag value. Apply resolution order: if flag > 0, use flag; else use `cfg.Budget.MaxTokens`. Create `tracker := budget.New(effectiveMaxTokens)`. Add `"github.com/jrswab/axe/internal/budget"` to imports.

- [x] **Modify `cmd/run.go`: Add budget tracking to single-shot mode.**
  After the single-shot `prov.Send()` call (after line 392), add `tracker.Add(resp.InputTokens, resp.OutputTokens)`. After the verbose block (after line 399), check `if tracker.Exceeded()` — if true, print stderr warning `"budget exceeded: used %d of %d tokens"`, output the response content (or JSON), and return `&ExitError{Code: 4, Err: fmt.Errorf("budget exceeded: used %d of %d tokens", tracker.Used(), tracker.Max())}`.

- [x] **Modify `cmd/run.go`: Add budget check and tracking to conversation loop.**
  At the top of the `for` loop body (line 402, before `prov.Send()`), check `if tracker.Exceeded() { break }`. After `totalOutputTokens += resp.OutputTokens` (line 423), add `tracker.Add(resp.InputTokens, resp.OutputTokens)`. After the `if len(resp.ToolCalls) == 0 { break }` block (line 432), add `if tracker.Exceeded() { break }`.

- [x] **Modify `cmd/run.go`: Add budget-exceeded exit after conversation loop.**
  After the existing "exhausted turns" check (line 471-473), add a budget-exceeded check: `if tracker.Exceeded() { _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "budget exceeded: used %d of %d tokens\n", tracker.Used(), tracker.Max()) }`. This sets up the `budgetExceeded` state for output handling. Use a `budgetExceeded` bool variable.

- [x] **Modify `cmd/run.go`: Pass `BudgetTracker` to `executeToolCalls()` via `ExecuteOptions`.**
  In the `execOpts` construction inside `executeToolCalls()` (lines 624-634), add `BudgetTracker: budgetTracker` field. Add a `budgetTracker *budget.BudgetTracker` parameter to the `executeToolCalls()` function signature. Update the call site in the conversation loop (line 443) to pass the tracker.

- [x] **Modify `cmd/run.go`: Add budget fields to JSON output envelope.**
  In the JSON envelope construction (lines 491-502), after the existing fields, conditionally add budget fields: `if tracker.Max() > 0 { envelope["budget_max_tokens"] = tracker.Max(); envelope["budget_used_tokens"] = tracker.Used(); envelope["budget_exceeded"] = tracker.Exceeded() }`.

- [x] **Modify `cmd/run.go`: Update verbose output to include budget info.**
  In the verbose output blocks (line 397 for single-shot, line 478 for conversation loop), when `tracker.Max() > 0`, change the Tokens line to: `"Tokens: %d input, %d output (cumulative, budget: %d/%d)"` with `tracker.Used()` and `tracker.Max()`.

- [x] **Modify `cmd/run.go`: Return exit code 4 when budget exceeded.**
  After the output section (after line 511), before the memory append section, check `if budgetExceeded { return &ExitError{Code: 4, Err: fmt.Errorf("budget exceeded: used %d of %d tokens", tracker.Used(), tracker.Max())} }`. This skips memory append (spec requirement: memory is NOT appended on budget-exceeded runs).

- [x] **Modify `cmd/run.go`: Add budget info to `printDryRun()`.**
  Add the effective `maxTokens` value as a parameter to `printDryRun()`. Display it in the dry-run output as `Budget: %d tokens (0 = unlimited)`.

### Phase 5: Tests for CLI and Conversation Loop (depends on Phase 4)

- [x] **Modify `cmd/run_test.go`: Test `--max-tokens` flag overrides TOML budget.**
  Set up a mock Anthropic server that returns a response with `InputTokens: 300, OutputTokens: 200`. Create an agent TOML with `[budget]\nmax_tokens = 10000`. Run with `--max-tokens 100`. Assert exit code 4 and stderr contains `"budget exceeded"`. Assert stdout still contains the response content (partial result).

- [x] **Modify `cmd/run_test.go`: Test budget=0 (unlimited) does not trigger budget exceeded.**
  Set up a mock server returning tokens. Create agent TOML with no `[budget]` block. Run without `--max-tokens`. Assert exit code 0 (success).

- [x] **Modify `cmd/run_test.go`: Test budget exceeded in conversation loop stops after crossing turn.**
  Set up a mock server that returns tool calls on first response (500 tokens) and text on second (500 tokens). Create agent with `[budget]\nmax_tokens = 600` and a tool. Assert the agent completes the first LLM call, but stops before or after the second depending on when budget is checked. Assert exit code 4.

- [x] **Modify `cmd/run_test.go`: Test JSON output includes budget fields when budget > 0.**
  Run with `--json --max-tokens 50000`. Assert JSON output contains `budget_max_tokens`, `budget_used_tokens`, `budget_exceeded` fields.

- [x] **Modify `cmd/run_test.go`: Test JSON output omits budget fields when budget = 0.**
  Run with `--json` and no budget. Assert JSON output does NOT contain `budget_max_tokens` key.

- [x] **Modify `cmd/run_test.go`: Test `--max-tokens 0` with TOML `max_tokens = 5000` uses TOML value.**
  Create agent with `[budget]\nmax_tokens = 5000`. Run with `--max-tokens 0` (default). Mock server returns 6000 tokens. Assert exit code 4.

- [x] **Modify `cmd/run_test.go`: Test memory is NOT appended when budget is exceeded.**
  Create agent with `[memory]\nenabled = true` and `[budget]\nmax_tokens = 100`. Run agent. Assert memory file does not have a new entry appended.
