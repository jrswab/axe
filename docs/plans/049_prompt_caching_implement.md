# Prompt Caching Support

## Section 1: Context Summary

Axe’s multi-turn agent runner re-transmits the full system prompt (skill + files + memory), tool definitions, and message history on every API call. No provider currently sends cache hints, so users pay full input-token costs for static content on every turn. This implementation adds provider-side prompt caching: explicit `cache_control` hints for Anthropic/Bedrock, automatic cache-hit reporting for OpenAI, and cumulative cache-token observability in the runner. Gemini context caching is out of scope; Ollama and OpenCode routes need no changes. Backward compatibility is required: existing agent TOML must work unchanged.

---

## Section 2: Implementation Checklist

### Phase 1: Shared Types (blocks all provider work)

- [x] `internal/provider/provider.go`: Add `CacheReadTokens int` and `CacheWriteTokens int` to `Response` struct.
- [x] `internal/provider/provider.go`: Add `CacheConfig` field to `Request` struct (or equivalent boolean flags) so the runner can signal that caching is desired. Keep it provider-agnostic.
- [x] `pkg/runner/run.go`: Update `Result` type to include `CacheReadTokens` and `CacheWriteTokens` (cumulative across turns).
- [x] `pkg/runner/run.go`: Update `Result` JSON marshaling to include new cache fields.

**Test:** `internal/provider/provider_test.go`: Verify `Response` and `Request` struct composition includes new fields.

---

### Phase 2: Anthropic Provider (explicit cache_control)

- [x] `internal/provider/anthropic.go`: Change `anthropicRequest.System` from `string` to `interface{}` (or add a separate content-block system field) so it can emit either a string or an array of content blocks with `cache_control`.
- [x] `internal/provider/anthropic.go`: Add `cache_control` field to `anthropicContentBlock` struct (type `map[string]string` or dedicated struct).
- [x] `internal/provider/anthropic.go`: `Send()` — when `req.CacheConfig` indicates caching, convert system prompt into content block(s) with `cache_control: {type: "ephemeral"}` on the last block instead of sending as plain string.
- [x] `internal/provider/anthropic.go`: ` anthropicResponse.Usage` struct — add `cache_creation_input_tokens` and `cache_read_input_tokens` fields.
- [x] `internal/provider/anthropic.go`: `Send()` — populate `Response.CacheReadTokens` and `Response.CacheWriteTokens` from new usage fields.
- [x] `internal/provider/anthropic.go`: `SendStream()` — same system-prompt caching logic and usage parsing in streaming path.
- [x] `internal/provider/anthropic.go`: Bump API version constant to a version that supports prompt caching, or verify `2023-06-01` works; update `anthropicVersion` if needed.

**Test:** `internal/provider/anthropic_test.go`:
- Test that cached system prompt emits array of content blocks with `cache_control` on the last block.
- Test that non-cached system prompt still emits plain string.
- Test response parsing with cache usage fields.
- Test streaming response parsing with cache usage fields.

---

### Phase 3: OpenAI Provider (automatic cache reporting)

- [x] `internal/provider/openai.go`: `openaiResponse.Usage` struct — add `prompt_tokens_details` nested struct with `cached_tokens int`.
- [x] `internal/provider/openai.go`: `Send()` — map `cached_tokens` to `Response.CacheReadTokens`. No request changes needed.
- [x] `internal/provider/openai.go`: `openaiStreamUsage` struct — add `prompt_tokens_details` for streaming usage chunk.
- [x] `internal/provider/openai.go`: `SendStream()` — map cached tokens from final usage chunk to `StreamEvent`/`Response`.

**Test:** `internal/provider/openai_test.go`:
- Test response parsing when cached_tokens is present.
- Test response parsing when cached_tokens is absent (graceful zero).
- Test streaming final usage chunk with cached_tokens.

---

### Phase 4: Bedrock Provider (cachePoint)

- [x] `internal/provider/bedrock.go`: Add `cachePoint` struct for the Converse API wire format.
- [x] `internal/provider/bedrock.go`: `bedrockSystemBlock` — add optional `cachePoint` field so system prompt blocks can carry cache hints.
- [x] `internal/provider/bedrock.go`: `bedrockToolConfig` or `bedrockToolDef` — add optional `cachePoint` field if Bedrock supports caching tool definitions (Claude via Bedrock). Verify exact API shape.
- [x] `internal/provider/bedrock.go`: `buildBedrockRequest()` — when caching is enabled, add `cachePoint` to the last system block and/or tool config block.
- [x] `internal/provider/bedrock.go`: `bedrockUsage` — add `cacheReadInputTokens` and `cacheWriteInputTokens` fields.
- [x] `internal/provider/bedrock.go`: `parseBedrockResponse()` — map new usage fields to `Response.CacheReadTokens` and `Response.CacheWriteTokens`.

**Test:** `internal/provider/bedrock_test.go`:
- Test request construction includes `cachePoint` in system blocks when caching enabled.
- Test response parsing with cache usage fields.
- Test graceful handling when provider omits cache fields.

---

### Phase 5: Runner Integration

- [x] `pkg/runner/run.go`: In the request builder, set `req.CacheConfig` (or equivalent) to indicate caching should be used. This should happen unconditionally for all providers that support it; unsupported providers will ignore the flag.
- [x] `pkg/runner/run.go`: Add `cacheReadTokens` and `cacheWriteTokens` local variables, accumulate them across conversation turns.
- [x] `pkg/runner/run.go`: In verbose logging, print cache read/write token counts alongside input/output counts (e.g., `Cache: X read, Y written`).
- [x] `pkg/runner/run.go`: Ensure sub-agent runs (`tool.ExecuteCallAgent`) do not inherit or share parent cache state (each builds its own `Request`). R9 is already satisfied by current architecture; verify no new shared state is introduced.

**Test:** `pkg/runner/run_test.go` (or integration tests):
- Verify that runner sets cache config on provider requests.
- Verify cumulative cache token tracking across turns.
- Verify JSON output contains cache token fields.

---

### Phase 6: Observability & Output

- [x] `pkg/runner/run.go`: Update verbose `stderr` output format to include cache token line.
- [x] `pkg/runner/run.go`: Ensure `Result` struct and its JSON serialization include `cache_read_tokens` and `cache_write_tokens`.

**Test:** Update golden tests / JSON output tests if they test exact JSON shape.

---

### Phase 7: MiniMax Provider (inherits Anthropic)

- [x] `internal/provider/minimax.go`: Verify that MiniMax inherits Anthropic caching changes automatically because it reuses the `Anthropic` struct and `anthropicRequest` types. No separate work expected unless MiniMax does not support the Anthropic caching API.

---

## Parallel Workstreams

After **Phase 1** (shared types) is complete, the following can proceed in parallel:

- **Phase 2** (Anthropic) + **Phase 3** (OpenAI) — independent providers.
- **Phase 4** (Bedrock) — independent, though depends on exact Bedrock/Claude cachePoint docs.
- **Phase 5** (Runner) — depends on Phase 1 only; can be merged before provider phases if the runner sets cache flags that providers currently ignore.
- **Phase 6** (Observability) — depends on Phase 1 and Phase 5.

**Recommended order:** Phase 1 → Phase 2 + Phase 3 (in parallel) → Phase 4 → Phase 5 → Phase 6 → Phase 7 (verification only).
