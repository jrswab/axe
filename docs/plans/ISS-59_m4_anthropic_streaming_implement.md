# ISS-59 M4 — Anthropic Provider Streaming: Implementation Guide

## Section 1: Context Summary

Axe's streaming infrastructure (M1 types, SSE parser, EventStream) and OpenAI streaming (M2) are already in place. This milestone adds `SendStream()` to the Anthropic provider so it implements `StreamProvider`. The Anthropic SSE wire format differs from OpenAI: it uses typed `event:` fields (not `[DONE]`), splits usage across `message_start` and `message_delta`, and delivers tool identity in `content_block_start` with input fragments as `input_json_delta`. The implementation follows the same structural pattern as `OpenAI.SendStream()` — build request, set stream flag, return `NewEventStream()` with a closure that maps SSE events to `StreamEvent` values. All decisions on architecture, retry policy (none), and pull-model design are already settled in the spec.

## Section 2: Implementation Checklist

All tasks are **sequential** — each depends on the prior. No parallelism.

### 2.1 Wire format types

- [x] Add `Stream bool` field to `anthropicRequest` struct (`internal/provider/anthropic.go:73`, field: `Stream bool \`json:"stream,omitempty"\``)
- [x] Add unexported streaming envelope structs in a new file `internal/provider/anthropic_stream_types.go`: `anthropicStreamEvent` (top-level with `Type`, `Message`, `Index`, `ContentBlock`, `Delta`, `Usage`, `Error` fields), plus nested structs for `anthropicStreamMessage`, `anthropicStreamUsage`, `anthropicStreamContentBlock`, `anthropicStreamDelta`, `anthropicStreamError`. Use concrete fields with `json:",omitempty"` — no `json.RawMessage`.
- [x] Test: In `internal/provider/anthropic_stream_types_test.go`, add `TestAnthropicStreamEvent_Unmarshal` — table-driven test that unmarshals each of the 10 SSE event JSON payloads from the spec (message_start, content_block_start text, content_block_start tool_use, text_delta, input_json_delta, content_block_stop, message_delta, message_stop, ping, error) into `anthropicStreamEvent` and asserts the correct fields are populated.

### 2.2 Mid-stream error mapping

- [x] Add unexported function `mapStreamErrorType(errorType string) ErrorCategory` in `internal/provider/anthropic.go` (after `mapStatusToCategory`). Maps: `"overloaded_error"` → `ErrCategoryOverloaded`, `"rate_limit_error"` → `ErrCategoryRateLimit`, `"api_error"` → `ErrCategoryServer`, `"authentication_error"` → `ErrCategoryAuth`, default → `ErrCategoryServer`.
- [x] Test: In `internal/provider/anthropic_stream_types_test.go`, add `TestMapStreamErrorType` — table-driven test covering all five cases above.

### 2.3 SendStream method

- [x] Add `SendStream(ctx context.Context, req *Request) (*EventStream, error)` method on `*Anthropic` in `internal/provider/anthropic.go`. Implementation:
  1. Build `anthropicRequest` identically to `Send()` (reuse `convertToAnthropicMessages`, `convertToAnthropicTools`, `defaultMaxTokens`, temperature handling). Set `Stream: true`.
  2. Marshal, create `http.NewRequestWithContext`, set same headers (`x-api-key`, `anthropic-version`, `content-type`).
  3. `client.Do()` — on error, check `ctx.Err()` for timeout, else return `ErrCategoryServer`.
  4. Non-2xx: read body, close body, return `handleErrorResponse()`.
  5. 2xx: create `NewSSEParser(httpResp.Body)`, build closure state (`inputTokens int`, `blocks map[int]blockInfo` where `blockInfo` is `struct{typ, id, name string}`), return `NewEventStream(httpResp.Body, nextFunc)`.
- [x] The `nextFunc` closure must implement the mapping table from spec §2.2:
  - Loop calling `parser.Next()`. On parser error: check `ctx.Err()` first (→ `ErrCategoryTimeout`), then `io.EOF` passthrough, else `ErrCategoryServer`.
  - `json.Unmarshal` into `anthropicStreamEvent`. On failure: return `ProviderError` with `ErrCategoryServer`.
  - Switch on `event.Type`: `message_start` (store inputTokens, continue), `content_block_start` (store block info, emit `StreamEventToolStart` for tool_use only), `content_block_delta` (emit `StreamEventText` or `StreamEventToolDelta`, skip empties, skip unknown index), `content_block_stop` (emit `StreamEventToolEnd` for tool_use only), `message_delta` (emit `StreamEventDone` with tokens and stop_reason), `message_stop` (return `io.EOF`), `ping` (continue), `error` (return `ProviderError` via `mapStreamErrorType`).

### 2.4 Tests — Request construction

- [x] Test: `TestAnthropic_SendStream_RequestFormat` in `internal/provider/anthropic_test.go` — httptest server captures request body, asserts `"stream": true` is present alongside model, messages, max_tokens, system, temperature, and tools. Same pattern as `TestAnthropic_Send_RequestFormat`.
- [x] Test: `TestAnthropic_SendStream_DefaultMaxTokens` — asserts max_tokens defaults to 4096 when `req.MaxTokens == 0`.
- [x] Test: `TestAnthropic_SendStream_OmitsZeroTemperature` — asserts temperature key absent when 0.

### 2.5 Tests — Error responses (pre-stream)

- [x] Test: `TestAnthropic_SendStream_ErrorResponse` — httptest server returns 401 with Anthropic error JSON. Assert `ProviderError` with `ErrCategoryAuth`.
- [x] Test: `TestAnthropic_SendStream_ContextCancelled` — httptest server sleeps, context with short timeout. Assert `ProviderError` with `ErrCategoryTimeout`.

### 2.6 Tests — Text streaming

- [x] Test: `TestAnthropic_SendStream_TextDeltas` — httptest server writes full SSE sequence: `message_start` → `content_block_start` (text) → two `content_block_delta` (text_delta) → `content_block_stop` → `message_delta` → `message_stop`. Consume via `Next()` loop, assert: two `StreamEventText` events with correct text, one `StreamEventDone` with `InputTokens`, `OutputTokens`, `StopReason`, then `io.EOF`.

### 2.7 Tests — Tool call streaming

- [x] Test: `TestAnthropic_SendStream_ToolCall` — SSE sequence: `message_start` → `content_block_start` (tool_use, id=`toolu_abc`, name=`read_file`) → two `content_block_delta` (input_json_delta with `{"pat` and `h": "x"}`) → `content_block_stop` → `message_delta` (stop_reason=`tool_use`) → `message_stop`. Assert: `StreamEventToolStart` (correct id/name), two `StreamEventToolDelta` (correct id/input fragments), `StreamEventToolEnd` (correct id), `StreamEventDone`, `io.EOF`.

### 2.8 Tests — Mixed text + tool blocks

- [x] Test: `TestAnthropic_SendStream_TextAndToolBlocks` — SSE sequence with text block (index 0) and tool_use block (index 1) interleaved. Assert events arrive in correct order: text events for index 0, tool start/delta/end for index 1, done, EOF.

### 2.9 Tests — Edge cases

- [x] Test: `TestAnthropic_SendStream_PingIgnored` — SSE sequence with `ping` events interspersed between `content_block_delta` events. Assert ping produces no `StreamEvent` — only the text deltas and done appear.
- [x] Test: `TestAnthropic_SendStream_MidStreamError` — SSE sequence: `message_start` → `content_block_start` → `error` event with `overloaded_error`. Assert `Next()` returns `ProviderError` with `ErrCategoryOverloaded`.
- [x] Test: `TestAnthropic_SendStream_EmptyTextDelta` — SSE `content_block_delta` with `delta.text` = `""`. Assert no `StreamEventText` emitted; next real delta is returned instead.
- [x] Test: `TestAnthropic_SendStream_EmptyPartialJSON` — SSE `content_block_delta` with `delta.partial_json` = `""`. Assert no `StreamEventToolDelta` emitted.
- [x] Test: `TestAnthropic_SendStream_MalformedJSON` — SSE event with invalid JSON data. Assert `ProviderError` with `ErrCategoryServer`.
- [x] Test: `TestAnthropic_SendStream_UnknownBlockIndex` — `content_block_delta` referencing an index not in block map. Assert event is skipped, no error.
- [x] Test: `TestAnthropic_SendStream_MessageDeltaWithoutStart` — `message_delta` arrives without prior `message_start`. Assert `StreamEventDone` with `InputTokens` = 0.
