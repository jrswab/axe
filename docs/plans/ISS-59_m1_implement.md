# ISS-59 M1 — Implementation Guide: Core Types, SSE Parser, Config Plumbing

Spec: [ISS-59_m1_core_types_sse_parser_spec.md](ISS-59_m1_core_types_sse_parser_spec.md)

## Section 1: Context Summary

Axe agents calling LLM providers through nginx proxies with 60-second idle timeouts get killed before long responses complete. Streaming keeps bytes flowing to prevent idle disconnects. M1 lays the foundation: event types, an SSE wire-format parser, a `StreamProvider` interface, `RetryProvider` passthrough, and the `--stream` / TOML `stream` config plumbing. No provider implementations or conversation-loop changes — those are M2–M5. All decisions (explicit `--stream` opt-in, `EventStream` with `Next()/Close()`, no retry wrapping for streams, sub-agents never stream, Bedrock deferred) are settled in the spec and must not be re-evaluated.

## Section 2: Implementation Checklist

### Stream Types (internal/provider/stream.go — new file)

These three tasks have no dependencies on each other or on other groups and can be implemented together.

- [x] Define exported string constants `StreamEventText`, `StreamEventToolStart`, `StreamEventToolDelta`, `StreamEventToolEnd`, `StreamEventDone` in `internal/provider/stream.go`
- [x] Define `StreamEvent` struct with fields: `Type string`, `Text string`, `ToolCallID string`, `ToolName string`, `ToolInput string`, `InputTokens int`, `OutputTokens int`, `StopReason string` in `internal/provider/stream.go`
- [x] Define `EventStream` struct with an `io.Closer` field (HTTP response body), a `nextFunc func() (StreamEvent, error)` field, and a `closed bool` field. Implement `Next() (StreamEvent, error)` (returns `io.EOF` after close or after done event), `Close() error` (idempotent, closes underlying body) in `internal/provider/stream.go`
- [x] Define `StreamProvider` interface embedding `Provider` with method `SendStream(ctx context.Context, req *Request) (*EventStream, error)` in `internal/provider/stream.go`
- [x] **Test:** `internal/provider/stream_test.go` — Table-driven tests for `EventStream`: `Next()` returns events in order then `io.EOF`; `Close()` is idempotent (double-close returns nil); `Next()` after `Close()` returns `io.EOF`. Use a real `EventStream` with a pipe-backed `io.ReadCloser` and a `nextFunc` that reads from it — no mocks.

### SSE Parser (internal/provider/sse.go — new file)

No dependency on stream types. Can be implemented in parallel with stream types and config plumbing.

- [x] Define `SSEEvent` struct with `Event string` and `Data string` fields in `internal/provider/sse.go`
- [x] Define `SSEParser` struct wrapping a `bufio.Scanner` (line-by-line) in `internal/provider/sse.go`
- [x] Implement `NewSSEParser(r io.Reader) *SSEParser` in `internal/provider/sse.go`
- [x] Implement `SSEParser.Next() (SSEEvent, error)` in `internal/provider/sse.go` — reads lines until a blank line (event boundary). Handles: `data:` lines (concatenate with `\n`), `event:` lines, comment lines (`:` prefix — skip), optional space after colon, `id:` and `retry:` ignored, `[DONE]` passed through as data. Returns `io.EOF` when reader is exhausted.
- [x] **Test:** `internal/provider/sse_test.go` — Table-driven tests covering: single data event, multi-line data (joined with `\n`), event type field, comment lines skipped, `[DONE]` sentinel passed through, space-after-colon stripping (`data:hello` vs `data: hello`), empty events between blank lines, `id:`/`retry:` fields ignored, `io.EOF` after last event. Feed real `strings.Reader` content into `NewSSEParser`.

### RetryProvider StreamProvider Delegation (internal/provider/retry.go)

Depends on `StreamProvider` interface from stream types.

- [x] Add method `SendStream(ctx context.Context, req *Request) (*EventStream, error)` on `*RetryProvider` in `internal/provider/retry.go`: type-assert `r.inner` to `StreamProvider`; if yes, delegate directly (no retry wrapping); if no, return a `ProviderError` with `ErrCategoryBadRequest` and message indicating the inner provider does not support streaming.
- [x] **Test:** `internal/provider/retry_test.go` — Two cases: (1) inner provider implements `StreamProvider` → `RetryProvider` delegates `SendStream()` and returns the stream; (2) inner provider does NOT implement `StreamProvider` → `SendStream()` returns error. Use minimal concrete test types (a struct implementing `Provider` only, and one implementing both `Provider` + `StreamProvider`) — not mocks.

### TOML Config (internal/agent/agent.go)

No dependency on any of the above. Can be implemented in parallel.

- [x] Add `Stream bool` field with TOML tag `toml:"stream"` to `AgentConfig` struct in `internal/agent/agent.go`
- [x] **Test:** `internal/agent/agent_test.go` — Verify `stream = true` in TOML round-trips through `Load()` correctly; verify default is `false` when field is omitted. Use a real temp TOML file, not mocks.

### CLI Flag and Dry-Run (cmd/run.go)

No dependency on any of the above. Can be implemented in parallel.

- [x] Register `--stream` boolean flag (default `false`) in `cmd/run.go: init()`
- [x] Read the flag in `cmd/run.go: runAgent()` and resolve against `cfg.Stream` (flag overrides TOML). Store as a local `streamEnabled` variable. No behavior change yet — just resolve and pass to dry-run.
- [x] Add `Stream: yes/no` line to `cmd/run.go: printDryRun()` output showing the resolved value
- [x] **Test:** `cmd/run_test.go` or `cmd/golden_test.go` — Verify `--dry-run --stream` shows `Stream: yes` in output; verify `--dry-run` without `--stream` shows `Stream: no`. Use the golden-file pattern if one already exists for dry-run, otherwise a standard command execution test.

### Scaffold Update (internal/agent/agent.go)

- [x] Add commented-out `# stream = false` line to `Scaffold()` template in `internal/agent/agent.go: Scaffold()`, placed near other top-level fields (after `timeout`)

## Parallelism

The following groups have **zero dependencies** between them and should be implemented concurrently:

| Group | Tasks |
|-------|-------|
| **A** | Stream types + tests (`stream.go`, `stream_test.go`) |
| **B** | SSE parser + tests (`sse.go`, `sse_test.go`) |
| **C** | TOML `Stream` field + test |
| **D** | CLI `--stream` flag + dry-run + test |

**Sequential after A completes:**
- RetryProvider `SendStream` delegation + tests (depends on `StreamProvider` interface from Group A)

**Sequential after C completes:**
- Scaffold update (trivial, touches same file)
