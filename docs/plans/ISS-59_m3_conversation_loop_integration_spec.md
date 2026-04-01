# ISS-59 M3 — Conversation Loop Integration Spec

**Milestone source:** `docs/plans/ISS-59_streaming_milestones.md` § M3

---

## Section 1: Context & Constraints

### Codebase State After M1 + M2

- `internal/provider/stream.go` defines:
  - `StreamEvent` with types: `text`, `tool_start`, `tool_delta`, `tool_end`, `done`.
  - `EventStream` with `Next() (StreamEvent, error)` and `Close() error`.
  - `StreamProvider` interface: `SendStream(ctx, *Request) (*EventStream, error)`.
- `internal/provider/openai.go` implements `SendStream()` on the OpenAI provider. Tool calls arrive as `tool_start` → N × `tool_delta` → `tool_end` events. The `done` event carries `InputTokens`, `OutputTokens`, and `StopReason`.
- `internal/provider/retry.go` `RetryProvider.SendStream()` delegates directly to the inner provider with no retry wrapping. `RetryProvider` already satisfies `StreamProvider`.
- `cmd/run.go` already:
  - Resolves `streamEnabled` from TOML `stream` field and `--stream` flag (flag wins).
  - Assigns `_ = streamEnabled` (unused placeholder).
  - Has a conversation loop (max 50 turns) that calls `prov.Send()`, appends assistant messages with tool calls, executes tools, appends tool results, and repeats.

### Decisions Already Made

- `--stream` is explicit opt-in (not TTY-detected).
- Flag overrides TOML; both resolved before the loop starts.
- Sub-agents always use `Send()`, never stream. The `call_agent` tool invocation path is unaffected.
- `RetryProvider` passes `SendStream()` through without retries.
- Bedrock streaming is deferred to a separate issue.

### Approaches Ruled Out

- Auto-detecting streaming from TTY — rejected in favor of explicit `--stream`.
- Retrying streaming requests — decided against in M1.
- Streaming sub-agent output to the parent — violates opaque sub-agent boundary.

### Constraints

- Axe is a Unix citizen: stdout must remain clean and pipeable. Streaming text tokens go to stdout. Debug/status goes to stderr.
- `--json` output is a single JSON envelope on stdout. When `--stream --json`, the stream keeps the connection alive but all output is buffered and emitted as one JSON object at the end.
- The conversation loop safety limit (50 turns) still applies.
- Budget tracking (`budget.BudgetTracker`) must still work — token counts come from the `done` event instead of `Response` fields.
- Memory append on success must still work — needs a final `Response`-equivalent after the stream completes.

---

## Section 2: Requirements

### R1: Stream-or-Buffer Decision Point

When the conversation loop is about to call the LLM, it must choose between `Send()` and `SendStream()` based on two conditions:

1. `streamEnabled` is true.
2. The provider (after retry wrapping) implements `StreamProvider`.

If both conditions are met, use `SendStream()`. Otherwise, fall back to `Send()` (current behavior). This check happens on every turn of the loop, not just the first.

### R2: Consuming the EventStream

When `SendStream()` returns an `*EventStream`, the loop must drain it by calling `Next()` repeatedly until `io.EOF` or error. During consumption, the loop must:

- **Accumulate text**: Concatenate all `StreamEventText` payloads into a single content string (equivalent to `Response.Content`).
- **Accumulate tool calls**: Track `tool_start`, `tool_delta`, and `tool_end` events to reconstruct complete `ToolCall` structs (ID, Name, Arguments). Tool call arguments arrive as JSON fragment strings across multiple `tool_delta` events — they must be concatenated and then parsed into `map[string]string` once `tool_end` is received.
- **Extract token counts**: The `done` event's `InputTokens` and `OutputTokens` fields provide the token counts for this turn.
- **Extract stop reason**: The `done` event's `StopReason` field provides the stop reason.
- **Close the stream**: Call `Close()` on the `EventStream` after draining, whether the drain succeeded or errored.

After draining, construct a `*provider.Response` from the accumulated data so the rest of the loop (tool execution, message appending, budget tracking, JSON output, memory) works unchanged.

### R3: Printing Text Tokens to Stdout (Non-JSON Mode)

When `--stream` is active and `--json` is NOT active:

- Each `StreamEventText` payload must be written to stdout immediately as it arrives (before the stream is fully drained).
- No trailing newline is added between text events — the LLM's content is printed verbatim as a continuous stream.
- After the stream completes and the conversation loop exits (no more tool calls), do NOT print `resp.Content` again. The text was already printed incrementally.

### R4: Buffered Output in JSON Mode

When `--stream --json` is active:

- Text events are NOT printed to stdout as they arrive.
- All text events are accumulated silently (the stream still keeps the HTTP connection alive).
- After the conversation loop exits, the JSON envelope is emitted exactly as it is today — `resp.Content` contains the full accumulated text.
- `tool_call_details` in the JSON envelope must include streaming tool calls with the same structure as non-streaming.

### R5: Tool Call Handling Unchanged

When the stream's stop reason is `tool_calls` (or the equivalent for the provider):

- The accumulated tool calls are placed on the assistant message exactly as they are in the non-streaming path.
- Tool execution proceeds identically — `executeToolCalls()` is called with the same arguments.
- Tool results are appended to the conversation.
- The loop continues to the next turn (which may again use `SendStream()`).

### R6: Budget Tracking

After draining the stream on each turn:

- Call `tracker.Add(inputTokens, outputTokens)` using the values from the `done` event.
- Accumulate `totalInputTokens` and `totalOutputTokens` the same as the non-streaming path.
- Budget-exceeded checks happen at the same points as today (before LLM call, after LLM call but before tool execution).

### R7: Error Handling

- If `SendStream()` returns an error, handle it identically to a `Send()` error (map via `mapProviderError`, return `ExitError`).
- If `Next()` returns an error other than `io.EOF`, close the stream and treat it as a provider error. Any text accumulated so far is lost — this is a failed turn.
- Context cancellation during stream consumption must be handled: if `ctx.Err() != nil` after a `Next()` error, return a timeout error.

### R8: Single-Shot Path (No Tools)

When the request has no tools and `streamEnabled` is true and the provider supports streaming:

- Use `SendStream()` instead of `Send()`.
- Print text events to stdout (non-JSON) or buffer them (JSON).
- After the stream completes, proceed to output and memory as normal.

### R9: Verbose Output

When `--verbose` is active and streaming:

- Log `[turn N] Streaming request (M messages)` to stderr before calling `SendStream()`.
- Log `[turn N] Stream complete: <stop_reason> (K tool calls)` to stderr after draining.
- Token and duration logging at the end remains unchanged.

### R10: Dry-Run

No changes to dry-run output. `streamEnabled` is already displayed in dry-run mode.

### Edge Cases

1. **Provider does not implement StreamProvider**: Fall back to `Send()` silently. No error, no warning.
2. **Stream returns zero text events and zero tool calls**: Treat as an empty response. `resp.Content` is `""`, `resp.ToolCalls` is nil. The loop exits.
3. **Stream returns text events AND tool calls in the same turn**: Both are accumulated. Text is printed incrementally (non-JSON). Tool calls are executed after the stream completes. This is valid — some providers emit text before tool calls.
4. **done event has zero token counts**: Use zero. Do not error.
5. **done event is missing (stream ends with EOF before done)**: Use zero for token counts and `""` for stop reason. The turn still succeeds.
6. **Multiple tool calls in one streamed turn**: Each gets its own `tool_start` → `tool_delta*` → `tool_end` sequence. All are accumulated and executed (in parallel or sequentially per config).

### Parallelism

The following parts of this spec are independent and can be implemented in parallel:

- **Stream consumption + Response construction** (R2) and **text printing** (R3/R4) are tightly coupled and must be implemented together.
- **R8 (single-shot path)** can be implemented in parallel with the conversation loop changes, as it's a separate code path.
- **R9 (verbose output)** can be layered on after the core loop changes.

---
