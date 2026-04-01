# ISS-59 M3 — Conversation Loop Integration: Implementation Guide

**Spec:** `docs/plans/ISS-59_m3_conversation_loop_integration_spec.md`

---

## Section 1: Context Summary

Axe already has `StreamProvider`, `EventStream`, and a working OpenAI `SendStream()` from M1/M2. The `--stream` flag and TOML `stream` field are resolved into `streamEnabled` in `cmd/run.go: runAgent()` but currently discarded (`_ = streamEnabled`). This milestone wires streaming into the conversation loop so that when streaming is active and the provider supports it, `SendStream()` is called instead of `Send()`, text tokens are printed incrementally to stdout (non-JSON) or buffered (JSON), tool calls are reconstructed from stream events, and a `*provider.Response` is built from the accumulated stream so all downstream code (tool execution, budget tracking, JSON envelope, memory) works unchanged.

---

## Section 2: Implementation Checklist

### Stream Drain Function (R2, R3, R4, R7)

These tasks are tightly coupled — the drain function, text printing, and Response construction must be built together.

- [x] **Create `drainEventStream()` in `cmd/run.go`** — Add a function with signature `func drainEventStream(stream *provider.EventStream, w io.Writer) (*provider.Response, error)`. `w` is the writer for incremental text output (`cmd.OutOrStdout()` in non-JSON mode, `nil` in JSON mode). The function:
  - Calls `stream.Next()` in a loop until `io.EOF` or error.
  - On `StreamEventText`: appends to a content string builder; if `w != nil`, writes the text payload to `w` immediately.
  - On `StreamEventToolStart`: starts tracking a new tool call (ID, Name) keyed by `ToolCallID`.
  - On `StreamEventToolDelta`: appends `ToolInput` to the tracked tool call's argument buffer (keyed by `ToolCallID`).
  - On `StreamEventToolEnd`: parses the accumulated argument JSON string into `map[string]string`, constructs a `provider.ToolCall`, appends to the result list.
  - On `StreamEventDone`: captures `InputTokens`, `OutputTokens`, `StopReason`.
  - On `io.EOF`: stops the loop.
  - On any other error: returns the error (caller handles).
  - Defers `stream.Close()` at the top.
  - Returns a `*provider.Response` with `Content`, `ToolCalls`, `InputTokens`, `OutputTokens`, `StopReason` populated from accumulated data.
  - Edge: if no `done` event arrived before EOF, tokens default to 0 and stop reason to `""`.

- [x] **Test `drainEventStream()` — text-only stream** (`cmd/run_stream_test.go`) — Create an `EventStream` from a sequence of text events + done event. Assert returned `Response.Content` matches concatenated text. Assert `Response.InputTokens` and `Response.OutputTokens` match done event values.

- [x] **Test `drainEventStream()` — tool calls** (`cmd/run_stream_test.go`) — Stream with `tool_start` → multiple `tool_delta` → `tool_end` for two tool calls, then done. Assert `Response.ToolCalls` has correct IDs, names, and parsed arguments.

- [x] **Test `drainEventStream()` — text + tool calls in same turn** (`cmd/run_stream_test.go`) — Stream with text events followed by tool call events. Assert both `Content` and `ToolCalls` are populated.

- [x] **Test `drainEventStream()` — incremental write to writer** (`cmd/run_stream_test.go`) — Pass a `bytes.Buffer` as `w`. Assert each text event's payload was written to the buffer in order. Assert the buffer's content matches `Response.Content`.

- [x] **Test `drainEventStream()` — nil writer (JSON mode)** (`cmd/run_stream_test.go`) — Pass `nil` as `w`. Assert no panic and `Response.Content` is still accumulated.

- [x] **Test `drainEventStream()` — mid-stream error** (`cmd/run_stream_test.go`) — Stream returns an error after two text events. Assert the function returns the error, not partial content.

- [x] **Test `drainEventStream()` — empty stream (only done)** (`cmd/run_stream_test.go`) — Stream emits only a done event. Assert `Response.Content` is `""` and `Response.ToolCalls` is nil.

- [x] **Test `drainEventStream()` — EOF without done event** (`cmd/run_stream_test.go`) — Stream emits text then EOF (no done). Assert tokens are 0 and stop reason is `""` but content is accumulated.

### Conversation Loop Integration (R1, R5, R6, R9)

Depends on `drainEventStream()` being complete.

- [x] **Wire streaming into the conversation loop** in `cmd/run.go: runAgent()` — Inside the `for turn := 0; ...` loop, replace the `prov.Send(ctx, req)` call with a branch: if `streamEnabled` and `prov` satisfies `provider.StreamProvider` (type-assert), call `sp.SendStream(ctx, req)`, then `drainEventStream(stream, textWriter)` where `textWriter` is `cmd.OutOrStdout()` when `!jsonOutput`, or `nil` when `jsonOutput`. Assign the returned `*provider.Response` to `resp`. On `SendStream()` error, handle identically to `Send()` error. On `drainEventStream()` error, map via `mapProviderError()`. Remove `_ = streamEnabled`.

- [x] **Add a `streamedText` flag** in `cmd/run.go: runAgent()` — Track whether any text was printed incrementally during streaming. After the loop exits, in the non-JSON output section, skip `fmt.Fprint(cmd.OutOrStdout(), resp.Content)` when `streamedText` is true.

- [x] **Verbose logging for streaming turns** in `cmd/run.go: runAgent()` — Before `SendStream()`, log `[turn N] Streaming request (M messages)` to stderr. After `drainEventStream()` returns, log `[turn N] Stream complete: <stop_reason> (K tool calls)` to stderr. Gate both behind `verbose`.

### Single-Shot Path (R8)

Can be implemented in parallel with the conversation loop changes — it's the `len(req.Tools) == 0` branch.

- [x] **Wire streaming into the single-shot path** in `cmd/run.go: runAgent()` — In the `if len(req.Tools) == 0` block, add the same `streamEnabled` + `StreamProvider` type-assert branch. Call `SendStream()` → `drainEventStream()`. Set `streamedText` if non-JSON. Assign returned `Response` to `resp`. Error handling identical to buffered path.

### Integration Tests

Depends on all above tasks.

- [x] **Integration test: streaming end-to-end with tools** (`cmd/run_integration_test.go`) — Use `testutil.NewMockLLMServer()` configured to return SSE streaming responses (text events → tool_calls finish → done; then after tool results, text events → stop → done). Run the compiled binary with `--stream`. Assert stdout contains the streamed text. Assert the tool was executed (check via tool side-effects or mock).

- [x] **Integration test: `--stream --json` buffers output** (`cmd/run_integration_test.go`) — Same mock server, run with `--stream --json`. Assert stdout is a single valid JSON object. Assert `content` field contains the full text. Assert no incremental text appeared before the JSON envelope.

- [x] **Integration test: streaming with non-StreamProvider falls back** (`cmd/run_integration_test.go`) — Use a provider that does not implement `StreamProvider`. Run with `--stream`. Assert it completes successfully using buffered `Send()`.

---

### Parallelism

The following can be implemented concurrently:

- **Group A**: `drainEventStream()` + all its unit tests
- **Group B**: Single-shot streaming path (R8) — independent code branch, but requires `drainEventStream()` signature to be finalized

Once Group A is complete:

- **Group C**: Conversation loop wiring + `streamedText` flag + verbose logging
- **Group D**: Integration tests (requires Groups A + C)
