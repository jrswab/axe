# ISS-59: Response Streaming — Milestones

## Problem

When Axe calls an LLM provider behind nginx with a 60-second idle timeout,
long-running requests get killed before the response completes. Streaming
keeps data flowing so the connection stays alive. Additionally, users want
to see tokens as they arrive in terminal mode.

## Behavioral Matrix

| Flags              | Provider → Axe              | Axe → stdout                    |
| ------------------ | --------------------------- | ------------------------------- |
| (default)          | Buffered (current behavior) | Buffered                        |
| `--stream`         | SSE streaming               | Print tokens as they arrive     |
| `--stream --json`  | SSE streaming (keeps alive) | Buffer, emit JSON envelope last |

## Milestones

### M1 — Core Types, SSE Parser, Config Plumbing ✅

Add `StreamEvent`, `EventStream`, and `StreamProvider` interface to the
provider package. Implement a generic SSE line parser. Add `--stream` CLI
flag and `stream` TOML field. No provider implementations or loop changes.

### M2 — OpenAI Provider Streaming ✅

Implement `SendStream()` on the OpenAI provider. This is the primary use
case (internal AI service speaks OpenAI-compatible SSE).

### M3 — Conversation Loop Integration ✅

Wire streaming into `cmd/run.go`: call `SendStream()` when `--stream` is
active and the provider implements `StreamProvider`. Print text events to
stdout (no `--json`) or buffer them (`--json`). Tool call accumulation and
the rest of the loop stay unchanged. Sub-agents never stream.

### M4 — Anthropic Provider Streaming ✅

Implement `SendStream()` on the Anthropic provider. Anthropic SSE uses
typed events (`message_start`, `content_block_delta`, etc.) that map to
the same `StreamEvent` types.

### M5 — Remaining Provider Streaming ✅

Implement `SendStream()` on Ollama (NDJSON), Gemini (SSE), MiniMax
(Anthropic-compatible SSE), and OpenCode (delegates to underlying route).
Bedrock is deferred to a separate issue (binary event stream framing).

## Decisions

- **Flag**: `--stream` (explicit opt-in, not auto-detected from TTY)
- **TOML**: `stream = true` at agent top level; flag overrides TOML
- **Streaming primitive**: `EventStream` struct with `Next()/Close()`
- **Retry**: `RetryProvider` delegates `SendStream()` to inner provider without retries
- **Sub-agents**: Always use `Send()`, never stream
- **Bedrock**: Deferred to separate issue (binary event stream, not SSE)
