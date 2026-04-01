# ISS-59 M4 — Anthropic Provider Streaming

Milestones: [ISS-59_streaming_milestones.md](ISS-59_streaming_milestones.md)

## Section 1: Context & Constraints

### Codebase Structure

- **Anthropic provider** (`internal/provider/anthropic.go`): Implements
  `Provider` with `Send()`. Uses `http.Client` to POST to `/v1/messages`.
  Existing helpers (`convertToAnthropicMessages`, `convertToAnthropicTools`)
  and error handling (`handleErrorResponse`, `mapStatusToCategory`) are
  reusable. The `anthropicRequest` struct defines the request body.

- **M1 types** (`internal/provider/stream.go`): `StreamEvent`,
  `EventStream`, and `StreamProvider` interface are defined.
  `NewEventStream(body, nextFunc)` is the constructor. `EventStream`
  manages close-once semantics and mutex-protected `Next()`.

- **SSE parser** (`internal/provider/sse.go`): `NewSSEParser(io.Reader)`
  returns a parser with `Next() (SSEEvent, error)`. Returns `io.EOF` when
  exhausted. Each `SSEEvent` has `Event` (the SSE `event:` field) and
  `Data` (the `data:` field).

- **OpenAI streaming reference** (`internal/provider/openai.go`):
  `SendStream()` on OpenAI shows the established pattern — build request,
  set streaming fields, return `NewEventStream()` with a closure that maps
  SSE events to `StreamEvent` values. The Anthropic implementation follows
  the same structural pattern.

- **RetryProvider** (`internal/provider/retry.go`): Already delegates
  `SendStream()` to the inner provider without retries. No changes needed.

### Decisions Already Made

- `StreamProvider` is an optional interface — `Anthropic` will implement it
  by adding `SendStream()`.
- `EventStream` with `Next()/Close()` is the streaming primitive.
- The SSE parser handles wire-level parsing; the provider maps parsed
  events to `StreamEvent` values.
- No retry wrapping for streaming.

### Approaches Ruled Out

- **Reusing `Send()` internally**: `SendStream()` must return an
  `EventStream` immediately with the HTTP body open. It cannot read the
  full body.
- **Goroutine + channel bridge**: The `EventStream.Next()` pull model
  matches the SSE parser's pull model. No goroutine needed.

### Constraints — Anthropic Streaming Wire Format

Anthropic streaming uses SSE with typed `event:` fields (unlike OpenAI
which uses only `data:`). The request must include `"stream": true`.

**SSE event types and their `data:` payloads:**

1. **`event: message_start`**
   ```json
   {
     "type": "message_start",
     "message": {
       "id": "msg_abc",
       "type": "message",
       "role": "assistant",
       "content": [],
       "model": "claude-sonnet-4-20250514",
       "stop_reason": null,
       "usage": { "input_tokens": 25, "output_tokens": 1 }
     }
   }
   ```
   Contains `input_tokens` in `message.usage`. `output_tokens` here is
   typically 1 (a running count at message start).

2. **`event: content_block_start`** (text)
   ```json
   {
     "type": "content_block_start",
     "index": 0,
     "content_block": { "type": "text", "text": "" }
   }
   ```

3. **`event: content_block_start`** (tool_use)
   ```json
   {
     "type": "content_block_start",
     "index": 1,
     "content_block": { "type": "tool_use", "id": "toolu_abc", "name": "read_file", "input": {} }
   }
   ```

4. **`event: content_block_delta`** (text_delta)
   ```json
   {
     "type": "content_block_delta",
     "index": 0,
     "delta": { "type": "text_delta", "text": "Hello" }
   }
   ```

5. **`event: content_block_delta`** (input_json_delta)
   ```json
   {
     "type": "content_block_delta",
     "index": 1,
     "delta": { "type": "input_json_delta", "partial_json": "{\"path\":" }
   }
   ```

6. **`event: content_block_stop`**
   ```json
   { "type": "content_block_stop", "index": 0 }
   ```

7. **`event: message_delta`**
   ```json
   {
     "type": "message_delta",
     "delta": { "stop_reason": "end_turn" },
     "usage": { "output_tokens": 15 }
   }
   ```
   Contains `stop_reason` and final `output_tokens`.

8. **`event: message_stop`**
   ```json
   { "type": "message_stop" }
   ```
   Final event. No more SSE events follow.

9. **`event: ping`**
   ```json
   { "type": "ping" }
   ```
   Keepalive. Ignore.

10. **`event: error`**
    ```json
    {
      "type": "error",
      "error": { "type": "overloaded_error", "message": "Overloaded" }
    }
    ```
    Mid-stream error from the API.

**Key differences from OpenAI streaming:**

- Anthropic uses SSE `event:` field to type events; OpenAI uses only `data:`.
- Anthropic has no `[DONE]` sentinel; `message_stop` signals the end.
- Tool call identity (`id`, `name`) arrives in `content_block_start`, not
  in a delta chunk.
- Tool input arrives as `partial_json` fragments in `input_json_delta`
  deltas, not as `arguments` fragments.
- Usage is split: `input_tokens` in `message_start`, `output_tokens` in
  `message_delta`.
- Content blocks have an `index` that identifies which block is being
  streamed (allowing text and tool_use blocks to interleave).

## Section 2: Requirements

### 2.1 `SendStream()` Method on `Anthropic`

`Anthropic` must implement the `StreamProvider` interface by adding a
`SendStream(ctx context.Context, req *Request) (*EventStream, error)` method.

**Request construction:**

- Build the request body identically to `Send()` (same message conversion,
  tool conversion, temperature/max_tokens handling, defaultMaxTokens).
- Set `"stream": true` in the JSON request body.
- Send to the same endpoint: `{baseURL}/v1/messages`.
- Use the same headers: `x-api-key`, `anthropic-version`, `content-type`.

**Response handling:**

- If the HTTP status is not 2xx, read the full body and return a
  `ProviderError` using the existing `handleErrorResponse()` logic. Close
  the body before returning.
- If the HTTP status is 2xx, do NOT read or close the body. Wrap it in
  an `EventStream` using `NewEventStream()` and return immediately.

**Context cancellation:**

- If the context is cancelled before the HTTP request completes, return a
  `ProviderError` with `ErrCategoryTimeout` (same as `Send()`).

### 2.2 SSE-to-StreamEvent Mapping

The `nextFunc` passed to `NewEventStream` must use `SSEParser` to read
SSE events from the response body and map them to `StreamEvent` values.

**Mapping rules:**

| SSE `event:` type | Data payload | StreamEvent produced |
|---|---|---|
| `message_start` | `message.usage.input_tokens` | No event emitted. Store `input_tokens` in closure state. Continue. |
| `content_block_start` with `content_block.type == "text"` | `index`, `content_block` | No event emitted. Store block `index` → type "text" in closure state. Continue. |
| `content_block_start` with `content_block.type == "tool_use"` | `index`, `content_block.id`, `content_block.name` | `StreamEventToolStart` with `ToolCallID` and `ToolName` set. Store block `index` → `{id, name}` in closure state. |
| `content_block_delta` with `delta.type == "text_delta"` | `index`, `delta.text` | `StreamEventText` with `Text` set to `delta.text`. |
| `content_block_delta` with `delta.type == "input_json_delta"` | `index`, `delta.partial_json` | `StreamEventToolDelta` with `ToolCallID` (looked up by `index`) and `ToolInput` set to `partial_json`. |
| `content_block_stop` | `index` | If the block at `index` is a tool_use block, emit `StreamEventToolEnd` with `ToolCallID` looked up by `index`. If the block is text, no event emitted. Continue. |
| `message_delta` | `delta.stop_reason`, `usage.output_tokens` | `StreamEventDone` with `StopReason`, `OutputTokens`, and the stored `InputTokens`. |
| `message_stop` | — | Return `io.EOF`. |
| `ping` | — | No event emitted. Continue. |
| `error` | `error.type`, `error.message` | Return `ProviderError` with category mapped from `error.type` (see 2.4). |

**Closure state the `nextFunc` must maintain:**

- `inputTokens int` — stored from `message_start`
- `blocks map[int]blockInfo` — maps content block `index` to `{type, id, name}` where `type` is `"text"` or `"tool_use"`

### 2.3 Wire Format Types

Add unexported structs for deserializing Anthropic streaming events. These
are separate from the existing `anthropicResponse` (which is for buffered
responses).

**Required types (all unexported):**

- **Envelope struct** for the outer event:
  - `Type` (string) — matches the SSE `event:` type
  - `Message` — present for `message_start` (contains `Usage` with `InputTokens`)
  - `Index` (int) — present for `content_block_start`, `content_block_delta`, `content_block_stop`
  - `ContentBlock` — present for `content_block_start` (contains `Type`, `ID`, `Name`, `Text`)
  - `Delta` — present for `content_block_delta` and `message_delta`
  - `Usage` — present for `message_delta` (contains `OutputTokens`)
  - `Error` — present for `error` events (contains `Type`, `Message`)

- Use `json.RawMessage` or a single struct with all optional fields. The
  implementation should choose whichever approach is simpler. The important
  thing is that all fields listed above are extractable from the JSON payload.

### 2.4 Edge Cases

1. **`ping` events**: Ignore. Do not emit any `StreamEvent`.

2. **Mid-stream `error` event**: The API sends an SSE event with
   `event: error`. Map `error.type` to a `ProviderError` category:
   - `"overloaded_error"` → `ErrCategoryOverloaded`
   - `"rate_limit_error"` → `ErrCategoryRateLimit`
   - `"api_error"` → `ErrCategoryServer`
   - `"authentication_error"` → `ErrCategoryAuth`
   - anything else → `ErrCategoryServer`

3. **Empty `text_delta`**: If `delta.text` is empty string, skip — do
   not emit a `StreamEventText` with empty `Text`.

4. **Empty `partial_json`**: If `delta.partial_json` is empty string,
   skip — do not emit a `StreamEventToolDelta` with empty `ToolInput`.

5. **Multiple content blocks**: The model may produce text blocks and
   tool_use blocks in the same message. Each block has a unique `index`.
   The block map must track all of them independently.

6. **Connection drop mid-stream**: `SSEParser.Next()` returns the
   underlying read error. The `nextFunc` must check `ctx.Err()` first —
   if the context is done, return `ProviderError` with
   `ErrCategoryTimeout`. Otherwise, return `ProviderError` with
   `ErrCategoryServer`.

7. **`content_block_stop` for a text block**: Do not emit
   `StreamEventToolEnd`. Only emit `StreamEventToolEnd` for tool_use
   blocks.

8. **JSON parse failure**: If `json.Unmarshal` fails on any SSE event's
   data payload, return `ProviderError` with `ErrCategoryServer`.

9. **`message_delta` without prior `message_start`**: If `inputTokens`
   was never set (no `message_start` received), emit `StreamEventDone`
   with `InputTokens` = 0. Do not error.

10. **`content_block_delta` for unknown index**: If the `index` is not
    in the block map, skip the event. Do not error.

### 2.5 Existing Code Reuse

The following existing code must be reused without modification:

- `convertToAnthropicMessages()` — for building the messages array
- `convertToAnthropicTools()` — for building the tools array
- `handleErrorResponse()` — for non-2xx responses before streaming begins
- `mapStatusToCategory()` — used by `handleErrorResponse()`
- `NewSSEParser()` — for parsing the SSE wire format
- `NewEventStream()` — for wrapping the response body

The existing `anthropicRequest` struct must be extended to include the
`stream` field (boolean, `json:"stream,omitempty"`).

### Parallelism

All work in this spec is sequential — `SendStream()` depends on the wire
format types (2.3), and the SSE-to-StreamEvent mapping (2.2) is the core
of `SendStream()`. There are no independent sub-tasks that can be done in
parallel.
