# ISS-59 M1 — Core Types, SSE Parser, Config Plumbing

Milestones: [ISS-59_streaming_milestones.md](ISS-59_streaming_milestones.md)

## Section 1: Context & Constraints

### Codebase Structure

- **Provider interface** (`internal/provider/provider.go`): All 7 providers
  implement `Provider` with a single `Send(ctx, *Request) (*Response, error)`
  method. The new streaming interface must coexist without breaking existing
  providers.

- **RetryProvider** (`internal/provider/retry.go`): Wraps `Provider.Send()`.
  Must also implement `StreamProvider` by delegating to the inner provider
  without retry logic. This aligns with the project rule: "Do not add internal
  LLM retries for provider errors."

- **Agent TOML** (`internal/agent/agent.go`): Agent config is loaded via
  `agent.Load()` and validated via `agent.Validate()`. New fields must be
  added to the `AgentConfig` struct and validated.

- **CLI flags** (`cmd/run.go`): Flags are registered in `init()` and read
  in `runAgent()`. Resolution order: flags > TOML > defaults.

- **Go version**: 1.25 (per `go.mod`). `iter.Seq2` is available but the
  `EventStream` struct with `Next()/Close()` was chosen for explicit
  resource cleanup (HTTP response body) without relying on iterator
  semantics.

### Decisions Already Made

- `--stream` flag name (explicit opt-in, no TTY auto-detection).
- `stream = true/false` in agent TOML at top level.
- `EventStream` with `Next()/Close()` pattern (like `sql.Rows`).
- `StreamProvider` is an optional interface — providers that don't support
  streaming simply don't implement it.
- Sub-agents never stream (only top-level agent).
- `--stream --json` streams from provider to Axe but buffers output.
- Bedrock streaming deferred to separate issue.

### Approaches Ruled Out

- **Changing `Send()` to internally stream**: Invisible to callers, breaks
  the explicit opt-in principle. The streaming transport should be a
  conscious choice, not hidden behavior.
- **`iter.Seq2` return type**: Requires careful cleanup semantics when the
  caller breaks out of iteration. `Next()/Close()` with `defer` is safer
  and more idiomatic for resource-holding types.
- **Channel-based streaming**: Requires goroutine management and has leak
  risks if the receiver stops consuming.
- **TTY auto-detection for streaming default**: Users pipe Axe output in
  CI/lambda/scripts. Auto-detecting would change behavior unexpectedly.

### Constraints

- SSE parser must handle: multi-line `data:` fields, empty events,
  `event:` type fields, `[DONE]` sentinel (OpenAI), comment lines
  (lines starting with `:`).
- SSE parser is a shared utility — Anthropic, OpenAI, Gemini, and MiniMax
  all use SSE (with different event payloads). The parser handles the
  wire format; each provider maps parsed events to `StreamEvent`.
- `EventStream.Close()` must close the underlying HTTP response body.
  Callers must always call `Close()` (via `defer`).
- `StreamEvent.Type` values must cover: text deltas, tool call lifecycle
  (start/delta/end), and a terminal "done" event with usage + stop reason.

## Section 2: Requirements

### 2.1 StreamEvent Type

A struct representing a single event from a streaming LLM response.

**Fields:**

| Field          | Type   | Populated When           | Description                                  |
| -------------- | ------ | ------------------------ | -------------------------------------------- |
| `Type`         | string | Always                   | One of the event type constants below        |
| `Text`         | string | `Type == "text"`         | Text content delta                           |
| `ToolCallID`   | string | `Type` is tool-related   | Provider-assigned tool call ID               |
| `ToolName`     | string | `Type == "tool_start"`   | Name of the tool being called                |
| `ToolInput`    | string | `Type == "tool_delta"`   | JSON fragment of tool input                  |
| `InputTokens`  | int    | `Type == "done"`         | Total input tokens for the request           |
| `OutputTokens` | int    | `Type == "done"`         | Total output tokens for the request          |
| `StopReason`   | string | `Type == "done"`         | Why the model stopped (e.g. "end_turn")      |

**Event type constants** (exported string constants):

| Constant             | Value          | Meaning                                     |
| -------------------- | -------------- | ------------------------------------------- |
| `StreamEventText`    | `"text"`       | Text content delta                          |
| `StreamEventToolStart` | `"tool_start"` | A new tool call began (ID + name)          |
| `StreamEventToolDelta` | `"tool_delta"` | Tool input JSON fragment                   |
| `StreamEventToolEnd` | `"tool_end"`   | Tool call input is complete                 |
| `StreamEventDone`    | `"done"`       | Stream finished; usage + stop reason set    |

### 2.2 EventStream Type

A struct that wraps a streaming HTTP response and yields `StreamEvent`
values one at a time.

**Methods:**

- `Next() (StreamEvent, error)` — Returns the next event. Returns
  `io.EOF` when the stream is complete (after the "done" event has been
  returned). Returns a `ProviderError` on mid-stream failures.
- `Close() error` — Closes the underlying HTTP response body. Must be
  safe to call multiple times. Must be called by the caller (via `defer`)
  regardless of whether `Next()` returned `io.EOF` or an error.

**Edge cases:**

- If the HTTP connection drops mid-stream, `Next()` returns a
  `ProviderError` with `ErrCategoryServer`.
- If context is cancelled, `Next()` returns a `ProviderError` with
  `ErrCategoryTimeout`.
- `Close()` after `Close()` is a no-op (no error).
- `Next()` after `Close()` returns `io.EOF`.

### 2.3 StreamProvider Interface

```
StreamProvider interface {
    Provider
    SendStream(ctx context.Context, req *Request) (*EventStream, error)
}
```

- Embeds `Provider` so every `StreamProvider` is also a `Provider`.
- `SendStream()` returns an error for connection/setup failures (before
  any events). Mid-stream errors come through `EventStream.Next()`.
- Providers that don't support streaming simply don't implement this
  interface. The conversation loop (M3) will type-assert to check.

### 2.4 RetryProvider StreamProvider Support

`RetryProvider` must implement `StreamProvider` if its inner provider does:

- Type-assert the inner provider to `StreamProvider`.
- If the inner provider is a `StreamProvider`, delegate `SendStream()`
  directly — no retry wrapping.
- If the inner provider is not a `StreamProvider`, `RetryProvider` does
  not implement `StreamProvider` (the type assertion in the conversation
  loop will fail gracefully and fall back to `Send()`).

### 2.5 SSE Parser

A shared utility for parsing Server-Sent Events from an `io.Reader`.

**Input**: An `io.Reader` (the HTTP response body).

**Output**: Parsed SSE events, each containing:
- `Event` (string) — the `event:` field value, empty if not present
- `Data` (string) — the concatenated `data:` field values (joined by `\n`
  for multi-line data)

**Behavior:**

- Events are delimited by blank lines (`\n\n`).
- Lines starting with `:` are comments — skip them.
- `event: <value>` sets the event type for the current event.
- `data: <value>` appends to the data buffer for the current event.
  Multiple `data:` lines within one event are joined with `\n`.
- `id:` and `retry:` fields are ignored (not needed for LLM streaming).
- A `data:` line with value `[DONE]` must be passed through as-is (the
  provider-specific code decides how to handle it).
- The parser reads until the reader returns `io.EOF` or an error.
- The space after the colon in field names is optional per the SSE spec
  (i.e. `data:hello` and `data: hello` are equivalent — strip one
  leading space from the value if present).

**Interface**: A struct with a `Next() (SSEEvent, error)` method that
returns `io.EOF` when the reader is exhausted. This mirrors the
`EventStream` pattern and allows the caller to pull events in a loop.

### 2.6 TOML Config: `stream` Field

Add a `stream` boolean field to `AgentConfig`:

- **Default**: `false` (current buffered behavior).
- **Validation**: None required (boolean with a sensible default).
- **Behavior**: When `true`, the conversation loop (M3) will attempt to
  use `SendStream()` if the provider supports it.

### 2.7 CLI Flag: `--stream`

Add a `--stream` boolean flag to the `run` command:

- **Default**: `false`.
- **Resolution order**: `--stream` flag overrides TOML `stream` field.
- **Mutual exclusivity**: `--stream` and `--json` are NOT mutually
  exclusive. When both are set, streaming is used on the wire but output
  is buffered into the JSON envelope.
- **Dry-run**: When `--dry-run` is set, the resolved `stream` value
  should be displayed in the dry-run output.

### Parallelism

The following can be implemented in parallel (no dependencies between them):

- **2.1 + 2.2 + 2.3** (StreamEvent, EventStream, StreamProvider) — these
  are type definitions in `provider.go`
- **2.5** (SSE parser) — standalone package, no dependency on provider types
- **2.4** (RetryProvider) depends on 2.3 (StreamProvider interface)
- **2.6 + 2.7** (TOML + CLI flag) can be done in parallel with all of the
  above
