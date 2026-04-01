# ISS-59 M5: Remaining Provider Streaming — Implementation Guide

## Section 1: Context Summary

Streaming infrastructure (core types, SSE parser, conversation loop integration) and reference implementations (OpenAI Chat Completions, Anthropic Messages) are complete from M1–M4. Three providers remain: Ollama (NDJSON-based streaming), Gemini (SSE via `streamGenerateContent`), and OpenCode (routing proxy that delegates to three different streaming wire formats). Each provider already has a working `Send()` method; the task is to add `SendStream()` implementing the `StreamProvider` interface. No changes to existing infrastructure are needed. All three providers can be implemented and tested in parallel.

## Section 2: Implementation Checklist

### Group A: Ollama Streaming (independent — can run in parallel with B and C)

- [x] Add `SendStream()` method to `internal/provider/ollama.go: Ollama.SendStream()` — Build the same `ollamaRequest` as `Send()` but set `Stream: true`. Make HTTP request, check for non-2xx (read body fully, return `*ProviderError`), then return `NewEventStream()` with an NDJSON-parsing `nextFunc`. The `nextFunc` reads lines with `bufio.Scanner`, parses each as JSON into `ollamaResponse`. Map: non-empty `message.content` → `StreamEventText`; each `message.tool_calls[i]` → `StreamEventToolStart` + `StreamEventToolEnd` pair (Ollama delivers complete tool calls, no deltas); `done: true` → `StreamEventDone` with `StopReason` from `done_reason`, `InputTokens` from `prompt_eval_count`, `OutputTokens` from `eval_count`; after done object return `io.EOF`. Skip empty content strings. On context cancellation return `ProviderError{ErrCategoryTimeout}`. On malformed JSON return `ProviderError{ErrCategoryServer}`. On connection refused, same handling as existing `Send()`.

- [x] Add an `ollamaStreamResponse` struct in `internal/provider/ollama.go` if needed, or reuse `ollamaResponse` with a `Done bool` field — The existing `ollamaResponse` lacks a `Done` field. Add `Done bool \`json:"done"\`` to `ollamaResponse` (it is already not used in `Send()` since `done` is always `true` in non-streaming mode, so adding it is safe).

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_RequestFormat` — Verify request body has `"stream":true`, same endpoint `/api/chat`, correct model/messages/tools. Use `httptest.NewServer` returning NDJSON lines. Drain stream.

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_TextDeltas` — Server returns 3 intermediate NDJSON objects with `message.content` fragments, then a final `done:true` object. Assert: 3 `StreamEventText` events with correct text, 1 `StreamEventDone` with token counts and stop reason, then `io.EOF`.

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_ToolCalls` — Server returns an intermediate object with `message.tool_calls` containing 2 tool calls. Assert: 2 `StreamEventToolStart`+`StreamEventToolEnd` pairs (no deltas), then `StreamEventDone`, then `io.EOF`.

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_EmptyContent` — Server returns intermediate objects with `message.content: ""` interspersed with real content. Assert: empty content objects are skipped.

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_ErrorResponse` — Server returns 500. Assert: `*ProviderError` with `ErrCategoryServer` returned from `SendStream()` (pre-stream error).

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_MalformedJSON` — Server returns a line that is not valid JSON. Assert: `*ProviderError` with `ErrCategoryServer`.

- [x] Test: `internal/provider/ollama_test.go: TestOllama_SendStream_ContextCancelled` — Use `context.WithTimeout` with a slow server. Assert: `*ProviderError` with `ErrCategoryTimeout`.

### Group B: Gemini Streaming (independent — can run in parallel with A and C)

- [x] Add `SendStream()` method to `internal/provider/gemini.go: Gemini.SendStream()` — Build the same `geminiRequest` as `Send()`. Change endpoint to `streamGenerateContent?alt=sse`. Make HTTP request, check for non-2xx (read body fully, return `*ProviderError`). Return `NewEventStream()` using `SSEParser` on the response body. The `nextFunc` calls `parser.Next()`, parses `sseEvent.Data` as `geminiResponse`. Map: `candidates[0].content.parts` with non-empty `text` → `StreamEventText` (one per text part); parts with `functionCall` → `StreamEventToolStart` (ID: `gemini_{counter}`) + `StreamEventToolEnd`; capture `finishReason` and `usageMetadata` as they appear (always update stored values). When parser returns `io.EOF`, emit `StreamEventDone` (if not already emitted) with latest `finishReason`, `promptTokenCount`, `candidatesTokenCount`, then return `io.EOF`. Skip chunks with no candidates. Skip empty text parts. On context cancellation return `ProviderError{ErrCategoryTimeout}`. On malformed SSE data return `ProviderError{ErrCategoryServer}`.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_RequestFormat` — Verify endpoint is `streamGenerateContent?alt=sse`, request body unchanged, API key header present. Use `httptest.NewServer` returning SSE-formatted responses. Drain stream.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_TextDeltas` — Server returns 3 SSE data lines each containing a `geminiResponse` with text parts. Assert: 3 `StreamEventText` events, 1 `StreamEventDone` with token counts and finish reason, then `io.EOF`.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_ToolCalls` — Server returns SSE chunks with `functionCall` parts. Assert: `StreamEventToolStart`+`StreamEventToolEnd` pairs with `gemini_N` IDs. No `StreamEventToolDelta`.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_MixedTextAndToolCalls` — Server returns a chunk with both text and functionCall parts. Assert: text events and tool start/end events in order.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_EmptyTextPart` — Server returns chunks with empty text parts interspersed. Assert: empty parts skipped.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_NoCandidates` — Server returns a chunk with empty candidates array. Assert: chunk skipped, stream continues.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_UsageInMiddleChunk` — Server returns `usageMetadata` in a middle chunk and again in the final chunk. Assert: `StreamEventDone` uses the latest values.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_ErrorResponse` — Server returns 401. Assert: `*ProviderError` with `ErrCategoryAuth`.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_MalformedSSE` — Server returns non-JSON in `data:` line. Assert: `*ProviderError` with `ErrCategoryServer`.

- [x] Test: `internal/provider/gemini_test.go: TestGemini_SendStream_ContextCancelled` — Use `context.WithTimeout` with a slow server. Assert: `*ProviderError` with `ErrCategoryTimeout`.

### Group C: OpenCode Streaming (independent — can run in parallel with A and B)

- [x] Add `SendStream()` method to `internal/provider/opencode.go: OpenCode.SendStream()` — Dispatch by model prefix identical to `Send()`: `claude-*` → `streamClaude()`, `gpt-*` → `streamGPT()`, default → `streamChatCompletions()`.

- [x] Add `internal/provider/opencode.go: OpenCode.streamClaude()` — Build `ocAnthropicRequest` same as `sendClaude()` but add `Stream: true` field (add `Stream bool \`json:"stream,omitempty"\`` to `ocAnthropicRequest`). Set headers (Authorization Bearer, anthropic-version, Content-Type). Check non-2xx → `mapOCAnthropicHTTPError`. Return `NewEventStream()` using `SSEParser`. The `nextFunc` uses the same event mapping as `Anthropic.SendStream()`: parse SSE data as an Anthropic stream event struct, map `message_start/content_block_start/content_block_delta/content_block_stop/message_delta/message_stop` to `StreamEvent` types. Reuse or duplicate the Anthropic stream event types locally (they are unexported).

- [x] Add `internal/provider/opencode.go: OpenCode.streamGPT()` — Build `ocResponsesRequest` same as `sendGPT()` but add `Stream: true` field (add `Stream bool \`json:"stream,omitempty"\`` to `ocResponsesRequest`). Set headers. Check non-2xx → `mapOCOpenAIHTTPError`. Return `NewEventStream()` using `SSEParser`. Define wire types for Responses API streaming events. The `nextFunc` maps: `response.content_part.delta` with `delta.text` → `StreamEventText`; `response.output_item.added` with `item.type=="function_call"` → `StreamEventToolStart`; `response.function_call_arguments.delta` → `StreamEventToolDelta`; `response.output_item.done` with `item.type=="function_call"` → `StreamEventToolEnd`; `response.completed` → `StreamEventDone` with usage; `[DONE]` → `io.EOF`.

- [x] Add `internal/provider/opencode.go: OpenCode.streamChatCompletions()` — Build `ocChatRequest` same as `sendChatCompletions()` but add `Stream: true` and `StreamOptions` fields (add fields to `ocChatRequest`). Set `stream_options: {include_usage: true}`. Set headers. Check non-2xx → `mapOCOpenAIHTTPError`. Return `NewEventStream()` using `SSEParser`. The `nextFunc` uses the same event mapping as `OpenAI.SendStream()`: parse SSE data as OpenAI chat completion chunks, handle `[DONE]`, map deltas to `StreamEvent` types.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_ClaudeRoute` — Server verifies request to `/v1/messages` with `stream:true`, returns Anthropic-style SSE events. Assert: text events, done event with usage, `io.EOF`.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_GPTRoute` — Server verifies request to `/v1/responses` with `stream:true`, returns Responses API SSE events. Assert: text events, tool start/delta/end events, done event, `io.EOF`.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_ChatCompletionsRoute` — Server verifies request to `/v1/chat/completions` with `stream:true` and `stream_options.include_usage:true`, returns Chat Completions SSE events. Assert: text events, done event with usage, `io.EOF`.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_UnknownModelFallsThrough` — Model without known prefix (e.g., `mistral-7b`). Assert: request goes to `/v1/chat/completions`.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_ErrorResponse` — Server returns 401 for each route. Assert: `*ProviderError` with `ErrCategoryAuth`.

- [x] Test: `internal/provider/opencode_test.go: TestOpenCode_SendStream_ContextCancelled` — Use `context.WithTimeout`. Assert: `*ProviderError` with `ErrCategoryTimeout`.
