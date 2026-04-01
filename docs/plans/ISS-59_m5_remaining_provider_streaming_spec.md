# ISS-59 M5: Remaining Provider Streaming

**Milestone document:** [docs/plans/ISS-59_streaming_milestones.md](ISS-59_streaming_milestones.md)

## Section 1: Context & Constraints

### Codebase Structure

The streaming infrastructure is fully in place from M1–M4:

- **`internal/provider/stream.go`** — Defines `StreamEvent` (types: `text`, `tool_start`, `tool_delta`, `tool_end`, `done`), `EventStream` (with `Next()/Close()`), `NewEventStream()`, and the `StreamProvider` interface.
- **`internal/provider/sse.go`** — Generic `SSEParser` with `Next()` returning `SSEEvent{Event, Data}`. Handles `data:`, `event:`, comment lines, and blank-line delimiters.
- **`internal/provider/openai.go`** — Reference `SendStream()` implementation for SSE-based OpenAI Chat Completions. Uses `SSEParser`, accumulates tool calls by index, emits `StreamEvent` types, handles `[DONE]` sentinel and usage chunks.
- **`internal/provider/anthropic.go`** — Reference `SendStream()` implementation for Anthropic typed SSE events. Uses `SSEParser`, tracks content blocks by index, maps `message_start/content_block_start/content_block_delta/content_block_stop/message_delta/message_stop` to `StreamEvent` types.
- **`internal/provider/retry.go`** — `RetryProvider.SendStream()` delegates to inner provider without retries. `SupportsStream()` checks if inner provider implements `StreamProvider`.
- **`cmd/run.go`** — Conversation loop calls `SendStream()` when `--stream` is active and the provider implements `StreamProvider`. Prints text events to stdout (non-JSON mode) or buffers them (JSON mode).

### Providers Requiring Work

1. **Ollama** (`internal/provider/ollama.go`) — Currently `Send()` only. Uses `stream: false`. Ollama streaming uses NDJSON (newline-delimited JSON), not SSE. Each line is a complete JSON object. The final object has `done: true`.
2. **Gemini** (`internal/provider/gemini.go`) — Currently `Send()` only. Uses `generateContent` endpoint. Gemini streaming uses `streamGenerateContent?alt=sse` endpoint, which returns SSE-formatted events where each `data:` line contains a JSON object matching the non-streaming response structure (array of candidates).
3. **OpenCode** (`internal/provider/opencode.go`) — Currently `Send()` only, routes by model prefix: `claude-*` → Anthropic Messages format, `gpt-*` → OpenAI Responses format, default → OpenAI Chat Completions format. `SendStream()` must delegate to the appropriate streaming format per route.

### Providers NOT Requiring Work

- **MiniMax** (`internal/provider/minimax.go`) — `NewMiniMax()` returns `*Anthropic`. It inherits `SendStream()` from the Anthropic type. No work needed.
- **Bedrock** — Explicitly deferred to a separate issue per milestones doc (binary event stream framing, not SSE).

### Decisions Already Made

- `--stream` is explicit opt-in (not TTY auto-detected).
- `stream = true` TOML field; flag overrides TOML.
- `RetryProvider` delegates `SendStream()` without retries.
- Sub-agents always use `Send()`, never stream.
- `EventStream.Next()` returns `io.EOF` when the stream is exhausted.
- Error responses (non-2xx HTTP) are read fully and returned as `*ProviderError` before creating the `EventStream`.

### Approaches Ruled Out

- Auto-detecting streaming support from the provider at runtime — the `StreamProvider` interface is used at compile time.
- Adding internal retries for streaming — the `RetryProvider` explicitly skips retries for `SendStream()`.
- Streaming for Bedrock — deferred; binary event stream framing is a separate concern.

---

## Section 2: Requirements

### R1: Ollama Streaming

The Ollama provider must implement the `StreamProvider` interface.

**Request behavior:**
- Set `stream: true` in the JSON request body (same endpoint: `POST /api/chat`).
- All other request fields (model, messages, options, tools) remain unchanged.

**Response format (NDJSON):**
- The response body is newline-delimited JSON. Each line is a complete JSON object.
- Each intermediate object contains a partial `message` field with `content` (text delta) and/or `tool_calls`.
- The final object has `"done": true` and includes `done_reason`, `prompt_eval_count`, and `eval_count`.
- There is no `[DONE]` sentinel or SSE framing. The existing `SSEParser` cannot be reused; the implementation must read lines and parse JSON directly.

**Event mapping:**
- Intermediate objects with `message.content` → `StreamEventText`
- Intermediate objects with `message.tool_calls` → Emit `StreamEventToolStart` on first appearance of a tool call, then `StreamEventToolEnd` (Ollama delivers complete tool calls in a single object, not streamed incrementally; there is no `StreamEventToolDelta`).
- Final object (`done: true`) → `StreamEventDone` with `StopReason` from `done_reason`, `InputTokens` from `prompt_eval_count`, `OutputTokens` from `eval_count`.
- After the final object, `Next()` returns `io.EOF`.

**Edge cases:**
- Empty `message.content` (`""`) in intermediate objects: skip, do not emit a `StreamEventText`.
- Context cancellation during read: return `ProviderError` with `ErrCategoryTimeout`.
- Connection refused: same error handling as the existing `Send()` method.
- Malformed JSON line: return `ProviderError` with `ErrCategoryServer`.
- Multiple tool calls in a single streaming object: emit one `StreamEventToolStart` + `StreamEventToolEnd` pair per tool call, in order.

### R2: Gemini Streaming

The Gemini provider must implement the `StreamProvider` interface.

**Request behavior:**
- Use the `streamGenerateContent` endpoint instead of `generateContent`: `POST /v1beta/models/{model}:streamGenerateContent?alt=sse`
- The request body is identical to the non-streaming request.

**Response format (SSE):**
- Standard SSE framing. Each `data:` line contains a JSON object matching the `geminiResponse` structure (with `candidates` array and `usageMetadata`).
- The existing `SSEParser` can and should be reused.
- There is no `[DONE]` sentinel. The stream ends when the HTTP response body closes (parser returns `io.EOF`).

**Event mapping:**
- `candidates[0].content.parts` with `text` field → `StreamEventText` (one event per text part).
- `candidates[0].content.parts` with `functionCall` field → Emit `StreamEventToolStart` (with generated tool call ID in format `gemini_{index}`) followed immediately by `StreamEventToolEnd`. Gemini delivers complete function calls, not streamed incrementally; there is no `StreamEventToolDelta`.
- `candidates[0].finishReason` present → Record it. When the stream ends or on the final chunk, emit `StreamEventDone` with the finish reason.
- `usageMetadata` present → Capture `promptTokenCount` and `candidatesTokenCount` for the `StreamEventDone` event.
- After SSE parser returns `io.EOF`, emit `StreamEventDone` (if not already emitted), then return `io.EOF`.

**Edge cases:**
- Empty text part (`""`) in a candidate: skip, do not emit a `StreamEventText`.
- No candidates in a chunk: skip the chunk entirely.
- Multiple text parts in a single chunk: emit one `StreamEventText` per non-empty text part.
- Multiple function calls in a single chunk: emit one `StreamEventToolStart` + `StreamEventToolEnd` pair per function call.
- `usageMetadata` may appear in any chunk (often the last); always update the stored values, use the latest when emitting `StreamEventDone`.
- Context cancellation: return `ProviderError` with `ErrCategoryTimeout`.
- Malformed SSE data: return `ProviderError` with `ErrCategoryServer`.
- Tool call ID generation: Use `gemini_{counter}` format, incrementing a counter across the entire stream (consistent with the existing `Send()` method).

### R3: OpenCode Streaming

The OpenCode provider must implement the `StreamProvider` interface.

**Routing behavior:**
- `SendStream()` must dispatch by model prefix, identical to `Send()`:
  - `claude-*` → Anthropic Messages streaming format (SSE with typed events: `message_start`, `content_block_delta`, etc.)
  - `gpt-*` → OpenAI Responses streaming format (SSE with `response.output_item.*` events)
  - All other models → OpenAI Chat Completions streaming format (SSE with `choices[].delta`)

**Implementation approach:**
- Each route builds the appropriate streaming request (same wire types as the existing `Send()` subroutines, with `stream: true` added).
- Each route parses the SSE stream and maps provider-specific events to `StreamEvent` types.
- The Anthropic Messages route uses the same SSE event mapping as the Anthropic provider's `SendStream()`.
- The Chat Completions route uses the same SSE event mapping as the OpenAI provider's `SendStream()`.
- The GPT/Responses route must handle OpenAI Responses API streaming events, which differ from Chat Completions.

**OpenCode GPT/Responses streaming events:**
- The Responses API streams events like `response.output_item.added`, `response.content_part.delta`, `response.function_call_arguments.delta`, `response.output_item.done`, and `response.completed`.
- `response.content_part.delta` with `delta.text` → `StreamEventText`
- `response.output_item.added` with `item.type == "function_call"` → `StreamEventToolStart` (ID from `item.id`, name from `item.name`)
- `response.function_call_arguments.delta` → `StreamEventToolDelta` (ID from `item_id`, input from `delta`)
- `response.output_item.done` with `item.type == "function_call"` → `StreamEventToolEnd`
- `response.completed` → `StreamEventDone` with usage from `response.usage`

**Edge cases:**
- Unknown model prefix: fall through to Chat Completions streaming (same as `Send()`).
- All error handling, timeout detection, and HTTP error responses follow the same patterns as the existing `Send()` subroutines.
- `stream_options: {include_usage: true}` must be set for the Chat Completions route (same as OpenAI provider).

### R4: No Changes to Existing Infrastructure

- No changes to `StreamEvent`, `EventStream`, `StreamProvider`, `SSEParser`, `RetryProvider`, or the conversation loop.
- No changes to the `Provider` interface or `New()` registry function.
- No changes to how the conversation loop detects `StreamProvider` support.

### Parallelism

The following can be implemented and tested independently, in parallel:

- **Ollama `SendStream()`** — standalone, no dependencies on other M5 work.
- **Gemini `SendStream()`** — standalone, no dependencies on other M5 work.
- **OpenCode `SendStream()`** — depends only on understanding the existing Anthropic and OpenAI streaming patterns (already implemented in M2/M4), not on Ollama or Gemini M5 work.

All three providers can be implemented in parallel.
