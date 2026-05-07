# Runner Pre-Built Message History (#82)

## Section 1: Context & Constraints

### Background
Issue #81 extracted the agent execution engine into `pkg/runner` as a public API. `runner.Run(ctx, opts)` is the single entry point, and `runner.Options` is the configuration struct. The only public way to provide user input today is via `Options.Prompt` (inline string) or `Options.Stdin` (reader), which `Run()` converts into a single `provider.Message{Role: "user", Content: userMessage}` internally.

The conversation loop then appends `assistant` and `tool` messages to this slice during execution. There is no way for an external caller to seed the conversation with prior turns, nor to retrieve the updated history after `Run()` returns.

### Existing Architecture Relevant to This Change
- **`pkg/runner/options.go`**: Defines `runner.Options` (public struct, field names). Currently has `Prompt string` and `Stdin io.Reader`.
- **`pkg/runner/run.go`**: `Run()` builds `req.Messages` as a single-message slice, then appends assistant/tool messages during the conversation loop.
- **`pkg/runner/result.go`**: `Result` struct returned by `Run()`. Contains content, tokens, tool call details, etc. No `Messages` field.
- **`internal/provider/provider.go`**: Defines `provider.Message`, `provider.ToolCall`, `provider.ToolResult`. These are `internal/` — external packages cannot import them.
- **Memory system**: `Run()` appends a memory entry (task → result) on successful completion. The "task" stored is the resolved user message (`opts.Prompt` or stdin or default).

### Constraints
- **Backwards compatibility**: Existing CLI users and programmatic callers must be unaffected. `Prompt`/`Stdin` behavior must be preserved when `Messages` is not set.
- **`internal/` boundary**: `pkg/runner` cannot expose `internal/provider` types directly in its public API. The public `Message` type must be self-contained in `pkg/runner`.
- **XDG/memory conventions**: Memory append behavior for the new path must remain consistent — the "task" text stored should be derivable from the initial user turn.
- **No scheduler/daemon**: Axe is an executor. Multi-turn conversation across multiple `Run()` calls is explicitly a caller concern; `pkg/runner` remains stateless between invocations.
- **Conversation loop**: Max 50 turns, hard limit. Pre-seeded messages count toward this history length (the limit applies to total assistant response count, not messages).

### Approach Already Decided
- Add an **opt-in** `Messages []Message` field to `runner.Options`.
- When `Messages` is non-empty, use it verbatim to initialize `req.Messages` and skip `Prompt`/`Stdin` resolution entirely.
- Define a public `Message` type in `pkg/runner` with fields isomorphic to `provider.Message`.
- Return the final message history in `Result.Messages` so callers can persist and resume.

### Open Questions (to be resolved in implementation phase)
1. Should the public `Message` type in `pkg/runner` be a struct (requiring conversion) or a type alias? The `internal/` boundary rules out alias.
2. How should dry-run display the user message when `Messages` is set? (Options: show first user message, show message count, show full history)

---

## Section 2: Requirements

### REQ-1: Public Message Type
`pkg/runner` must define a public `Message` struct with fields matching the semantic roles of `provider.Message`:
- `Role string` — one of `"user"`, `"assistant"`, `"tool"`
- `Content string` — text content of the message
- `ToolCalls []ToolCall` — tool call definitions for assistant turns
- `ToolResults []ToolResult` — tool execution results for tool turns

`pkg/runner` must also define public `ToolCall` and `ToolResult` structs isomorphic to `provider.ToolCall` and `provider.ToolResult`, so that external callers can construct complete message histories without importing `internal/provider`.

### REQ-2: Options.Messages Field
`runner.Options` must gain a new field: `Messages []Message`. When this slice is non-empty:
- `Run()` must initialize `req.Messages` with the provided slice **verbatim** (preserving order and contents).
- `Run()` must **not** read `opts.Prompt`, `opts.Stdin`, or resolve a default user message.
- The conversation loop must append assistant/tool messages to the **end** of this existing history, just as it does today with a single-message seed.

When `Messages` is empty or nil, the existing behavior (resolve Prompt → Stdin → default) must be preserved exactly.

### REQ-3: Result.Messages Field
`runner.Result` must gain a new field: `Messages []Message`. After `Run()` completes (success or error), this field must contain the full message history including:
- Any pre-seeded messages from `Options.Messages`
- Any assistant messages added during the conversation loop
- Any tool result messages added during the conversation loop

For the non-tool single-shot path (no conversation loop), `Result.Messages` must contain the initial user message plus the single assistant response.

### REQ-4: Role Validation
When `Messages` is non-empty, `Run()` must validate each message's `Role` field before building the request. Invalid roles (anything other than `"user"`, `"assistant"`, `"tool"`) must produce a `ConfigError`. The error message must indicate the invalid role value and its position in the slice.

### REQ-5: Message Type Consistency Validation
When `Messages` is non-empty, `Run()` must validate:
- Messages with `Role == "assistant"` may have non-nil `ToolCalls` and must not have `ToolResults`.
- Messages with `Role == "tool"` may have non-nil `ToolResults` and must have empty `Content`.
- Messages with `Role == "user"` must have empty `ToolCalls` and empty `ToolResults`.

Violations must produce a `ConfigError` with a descriptive message including the offending role and index.

### REQ-6: Tool Call ID Validation (within Messages)
When `Messages` contains assistant messages with `ToolCalls`, and subsequent tool messages with `ToolResults`, `Run()` must validate that every `ToolResult.CallID` matches a `ToolCall.ID` from the most recent preceding assistant message. Unmatched CallIDs must produce a `ConfigError`.

### REQ-7: Dry-Run Behavior
When `opts.DryRun` is true and `opts.Messages` is non-empty:
- `DryRunInfo.UserMessage` must be set to `"(pre-built message history)"`.
- `DryRunInfo` must gain a new field `MessageCount int` showing the number of messages provided.
- The dry-run output must not attempt to display individual messages (the existing user message display is for a single string).

### REQ-8: Memory Append Behavior
When `cfg.Memory.Enabled` is true and `opts.Messages` is non-empty:
- The "task" text stored in memory must be the `Content` of the **first** user message in the history (i.e., the earliest message with `Role == "user"`).
- If no user message exists in the provided history, the task text must be `"(pre-built message history with no user message)"`.
- The "result" text remains the final assistant `Content` from `resp.Content`, as it is today.

### REQ-9: Budget and Token Tracking
Pre-seeded messages count as input for the purpose of token budget enforcement. The `budget.Tracker` must be initialized before the first provider call and must account for tokens used across all turns, including the initial request containing pre-seeded messages. The tracker behavior must be unchanged; only the initial message payload differs.

### REQ-10: Tool Registration and Sub-Agent Depth
Pre-seeded messages must not affect tool registration, MCP tool loading, or sub-agent depth calculation. Specifically:
- `call_agent` tool injection based on depth must work exactly as before.
- The `depth` variable starts at 0 regardless of message history length.

### REQ-11: Streaming Compatibility
When `Messages` is non-empty and streaming is enabled, the stream must receive the full pre-seeded message history exactly as it would with a single user message. The `drainEventStream` helper must function unchanged.

### REQ-12: JSON Output Compatibility
When `opts.JSON` is true and `Messages` is non-empty, the JSON envelope must include the new `messages` field alongside all existing fields. The field must be an array of message objects with `role`, `content`, `tool_calls`, and `tool_results` keys. Empty `tool_calls`/`tool_results` must be serialized as empty arrays, not omitted or null.

### REQ-13: Empty Messages Slice
`Messages` being non-nil but empty (`len(Messages) == 0`) must be treated identically to `Messages == nil`: fall back to `Prompt`/`Stdin` behavior.

### REQ-14: Backwards Compatibility
All existing tests for `cmd/run.go`, `pkg/runner/run_test.go`, and CLI golden files must pass without modification. No existing field in `runner.Options` or `runner.Result` may be removed, renamed, or have its type changed.

---

## Parallelizable Work
- Defining the public `Message`, `ToolCall`, and `ToolResult` types in `pkg/runner`.
- Adding `Messages` to `Options` and `Result`.
- Implementing role and consistency validation logic.
- Implementing dry-run info display changes.
- Writing unit tests for validation, conversion, and round-trip behavior.
- The above can all be done in parallel before integrating into the `Run()` orchestration.

## Serial Dependencies
- The conversion from `runner.Message` to `provider.Message` must be defined before it can be used in `Run()`.
- `Run()` integration (message initialization, conversation loop appends, `Result` population) depends on the types being in place.
- Memory append logic depends on the `Run()` integration being complete.
