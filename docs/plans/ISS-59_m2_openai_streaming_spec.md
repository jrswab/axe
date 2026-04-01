# ISS-59 M2 — OpenAI Provider Streaming

Milestones: [ISS-59_streaming_milestones.md](ISS-59_streaming_milestones.md)

## Section 1: Context & Constraints

### Codebase Structure

- **OpenAI provider** (`internal/provider/openai.go`): Implements `Provider`
  with `Send()`. Uses `http.Client` to POST to `/chat/completions`. Existing
  request-building helpers (`convertToOpenAIMessages`,
  `convertToOpenAITools`) and error handling (`handleErrorResponse`,
  `mapStatusToCategory`) are reusable.

- **M1 types** (`internal/provider/stream.go`): `StreamEvent`,
  `EventStream`, and `StreamProvider` interface are already defined.
  `NewEventStream(body, nextFunc)` is the constructor. `EventStream` manages
  close-once semantics and mutex-protected `Next()`.

- **SSE parser** (`internal/provider/sse.go`): `NewSSEParser(io.Reader)`
  returns a parser with `Next() (SSEEvent, error)`. Returns `io.EOF` when
  the reader is exhausted. Each `SSEEvent` has `Event` (type) and `Data`
  (payload) fields.

- **RetryProvider** (`internal/provider/retry.go`): Already implements
  `SendStream()` by delegating to the inner provider without retries. No
  changes needed for M2.

### Decisions Already Made

- `StreamProvider` is an optional interface — `OpenAI` will implement it by
  adding `SendStream()`.
- `EventStream` with `Next()/Close()` is the streaming primitive.
- The SSE parser handles wire-level parsing; the provider maps parsed events
  to `StreamEvent` values.
- No retry wrapping for streaming (handled by `RetryProvider` in M1).

### Approaches Ruled Out

- **Reusing `Send()` internally**: `SendStream()` must return an
  `EventStream` immediately with the HTTP body open. It cannot read the
  full body like `Send()` does.
- **Goroutine + channel bridge**: The `EventStream.Next()` pull model
  already matches the SSE parser's pull model. No goroutine needed.
- **Parsing `usage` from `stream_options`**: OpenAI supports
  `stream_options: {include_usage: true}` which sends a final chunk with
  usage data. This is the correct approach — usage is not available in
  individual delta chunks.

### Constraints

- **OpenAI streaming wire format**: When `stream: true` is set in the
  request body, the response is `text/event-stream` (SSE). Each SSE event
  has `data:` containing a JSON chunk. The final event is `data: [DONE]`.

- **Chunk JSON structure** (text delta):
  ```json
  {
    "model": "gpt-4o",
    "choices": [{
      "index": 0,
      "delta": {"content": "Hello"},
      "finish_reason": null
    }]
  }
  ```

- **Chunk JSON structure** (tool call):
  ```json
  {
    "choices": [{
      "delta": {
        "tool_calls": [{
          "index": 0,
          "id": "call_abc",
          "type": "function",
          "function": {"name": "read_file", "arguments": ""}
        }]
      },
      "finish_reason": null
    }]
  }
  ```
  Subsequent chunks for the same tool call omit `id`, `type`, and
  `function.name` — only `index` and `function.arguments` (fragment) are
  present.

- **Chunk JSON structure** (finish):
  ```json
  {
    "choices": [{
      "delta": {},
      "finish_reason": "stop"
    }]
  }
  ```

- **Usage chunk** (when `stream_options.include_usage` is set):
  ```json
  {
    "choices": [],
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 5
    }
  }
  ```
  This arrives as a separate chunk after the `finish_reason` chunk and
  before `[DONE]`.

- **Tool call accumulation**: A single tool call is split across many
  chunks. The first chunk for a tool call (identified by `index`) carries
  `id` and `function.name`. Subsequent chunks carry `function.arguments`
  fragments. The `finish_reason` of `"tool_calls"` signals all tool calls
  are complete.

- **Error responses**: If the API returns an HTTP error status (4xx/5xx),
  the response is NOT SSE — it's a JSON error body, identical to the
  non-streaming case. Mid-stream errors manifest as the connection dropping
  (the SSE parser returns a read error).

## Section 2: Requirements

### 2.1 `SendStream()` Method on `OpenAI`

`OpenAI` must implement the `StreamProvider` interface by adding a
`SendStream(ctx context.Context, req *Request) (*EventStream, error)` method.

**Request construction:**

- Build the request body identically to `Send()` (same message conversion,
  tool conversion, temperature/max_tokens handling).
- Set `"stream": true` in the JSON request body.
- Set `"stream_options": {"include_usage": true}` in the JSON request body
  so that usage data is included in the final chunk.
- Send to the same endpoint: `{baseURL}/chat/completions`.
- Use the same headers: `Authorization: Bearer {apiKey}`,
  `Content-Type: application/json`.

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

| SSE data content | StreamEvent produced |
|---|---|
| `[DONE]` sentinel | Return `io.EOF` (no `StreamEvent` emitted) |
| Chunk with `choices[0].delta.content` (non-empty string) | `StreamEventText` with `Text` set to the content value |
| Chunk with `choices[0].delta.tool_calls[N]` where `id` is present | `StreamEventToolStart` with `ToolCallID` and `ToolName` set |
| Chunk with `choices[0].delta.tool_calls[N]` where `id` is absent and `function.arguments` is present | `StreamEventToolDelta` with `ToolCallID` (looked up by index) and `ToolInput` set to the arguments fragment |
| Chunk with `choices[0].finish_reason == "tool_calls"` | Emit one `StreamEventToolEnd` per accumulated tool call (by index order), then continue to the next SSE event |
| Chunk with `choices[0].finish_reason == "stop"` (or any non-null, non-`"tool_calls"` value) | Continue to the next SSE event (the done event will carry stop reason) |
| Chunk with `usage` present and `choices` empty | `StreamEventDone` with `InputTokens`, `OutputTokens`, and `StopReason` set |
| Chunk with empty `delta` and `finish_reason == null` | Skip (no event emitted), continue to next SSE event |
| JSON parse failure on a chunk | Return `ProviderError` with `ErrCategoryServer` |

**Tool call index tracking:**

- The `nextFunc` closure must maintain a map of `index` → `{id, name}` to
  correlate tool call delta chunks with their starting chunk.
- When a `tool_calls` entry has an `id` field, it is a new tool call —
  store `index → {id, name}` and emit `StreamEventToolStart`.
- When a `tool_calls` entry lacks `id`, look up the stored `id` by `index`
  and emit `StreamEventToolDelta`.

**Stop reason and usage tracking:**

- The `nextFunc` closure must store the `finish_reason` from the finish
  chunk so it can be included in the `StreamEventDone` emitted when the
  usage chunk arrives.
- If the stream ends (`[DONE]`) without a usage chunk (e.g., provider
  doesn't honor `stream_options`), emit `StreamEventDone` with zero
  tokens and the stored `finish_reason`. Do not error.

### 2.3 Wire Format Types

Add unexported structs for deserializing OpenAI streaming chunks. These
are separate from the existing `openaiResponse` (which is for buffered
responses).

**Required fields:**

- `model` (string)
- `choices` (array of objects, each with):
  - `index` (int)
  - `delta` (object with optional `content`, `role`, `tool_calls`)
  - `finish_reason` (nullable string)
- `usage` (optional object with `prompt_tokens`, `completion_tokens`)

The `delta.tool_calls` array entries have:
- `index` (int) — always present
- `id` (string) — present only on the first chunk for a given tool call
- `type` (string) — present only on the first chunk
- `function` (object) — with optional `name` and `arguments`

### 2.4 Edge Cases

1. **Empty `delta.content`**: Some providers send chunks with
   `"content": ""`. Skip these — do not emit a `StreamEventText` with
   empty `Text`.

2. **Multiple tool calls in parallel**: The model may initiate multiple
   tool calls in one response. Each gets a unique `index`. The mapping
   must track all of them independently.

3. **`content` and `tool_calls` in the same response**: The model may
   produce text content before switching to tool calls. Both must be
   emitted as their respective event types.

4. **Connection drop mid-stream**: `SSEParser.Next()` returns the
   underlying read error. The `nextFunc` must wrap this in a
   `ProviderError` with `ErrCategoryServer`.

5. **Context cancelled mid-stream**: The HTTP response body read will
   fail with the context error. The `nextFunc` must check `ctx.Err()`
   and return a `ProviderError` with `ErrCategoryTimeout` if the context
   is done.

6. **No choices in chunk**: Some chunks (like the usage-only chunk) have
   an empty `choices` array. Handle gracefully — do not index into an
   empty slice.

7. **`finish_reason` of `"length"`**: Treat like `"stop"` — store it and
   include in the done event. Do not error.

8. **`stream_options` not supported by endpoint**: Some OpenAI-compatible
   endpoints may ignore `stream_options`. The stream must still terminate
   cleanly — emit `StreamEventDone` with zero tokens on `[DONE]` if no
   usage chunk was received.

### 2.5 Existing Code Reuse

The following existing code must be reused without modification:

- `convertToOpenAIMessages()` — for building the messages array
- `convertToOpenAITools()` — for building the tools array
- `handleErrorResponse()` — for non-2xx responses before streaming begins
- `mapStatusToCategory()` — used by `handleErrorResponse()`
- `NewSSEParser()` — for parsing the SSE wire format
- `NewEventStream()` — for wrapping the response body

The existing `openaiRequest` struct must be extended (or a new streaming
variant created) to include the `stream` and `stream_options` fields.

### Parallelism

All work in this spec is sequential — `SendStream()` depends on the wire
format types (2.3), and the SSE-to-StreamEvent mapping (2.2) is the core
of `SendStream()`. There are no independent sub-tasks that can be done in
parallel.
