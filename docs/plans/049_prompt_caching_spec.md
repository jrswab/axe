# Prompt Caching Support

## Section 1: Context & Constraints

### What Exists Today

Axe's runner (`pkg/runner/run.go`) constructs a fresh provider request on every turn of a multi-turn conversation. The request payload includes:

1. The full system prompt (agent `system_prompt` + SKILL.md contents + inlined file contents + memory entries).
2. The complete set of tool definitions.
3. The full message history (user message + prior assistant tool calls + tool results).

No provider in the codebase sends explicit cache hints. The Anthropic provider uses API version `2023-06-01` and does not include `cache_control` blocks. The OpenAI provider does not read `usage.prompt_tokens_details.cached_tokens`. The Bedrock provider does not include `cachePoint` blocks. As a result, users pay full input-token costs for the system prompt and tool definitions on every single turn.

### Why This Matters

Multi-turn tool-use is axe's primary execution mode. A typical agent run resends the same large system context and tool schema 3–10 times per run. Provider-side prompt caching reduces input-token costs for repeated prefixes by 50–90% on cache hits.

### Provider Landscape

| Provider | Mechanism | Explicit Control | Notes |
|---|---|---|---|
| Anthropic | `cache_control: {type: "ephemeral"}` on content blocks | Yes | Available on Claude 3.5 Sonnet and newer. Usage returns `cache_creation_input_tokens` and `cache_read_input_tokens`. |
| OpenAI | Automatic prefix caching | No (automatic) | `usage.prompt_tokens_details.cached_tokens` reports cache hits. No request changes needed. |
| Bedrock (Claude) | `cachePoint` blocks in system/messages/tool config | Yes | Only for supported Claude models via Converse API. Usage returns `cacheReadInputTokens` / `cacheWriteInputTokens`. |
| Gemini | Context caching API (`cachedContent`) | Yes (stateful) | Requires creating a cache object via a separate API call and referencing it by name in subsequent requests. Conflicts with axe's stateless per-request model. |
| Ollama | N/A | N/A | Local inference; prompt caching is irrelevant. |
| OpenCode | Pass-through | Partial | Claude and GPT routes may inherit upstream caching behavior. No explicit gateway-level control assumed. |
| MiniMax | Pass-through | Partial | Uses Anthropic-compatible API; may inherit Anthropic changes. |

### Constraints

- **Backward compatibility is non-negotiable.** Existing agent TOML files must behave identically when caching is unsupported or unavailable.
- **No global state.** Axe passes dependencies explicitly. Any cache hint must travel through the existing `provider.Request` abstraction.
- **Sub-agents are opaque.** A parent agent never sees a sub-agent's internal turns. Cache state must not leak across the sub-agent boundary.
- **Stateless request model.** The runner builds a new `Request` each turn. We will not introduce cross-request client-side cache state (e.g., Gemini context caching) in this milestone.
- **Test coverage is required.** Table-driven tests, mock servers, and golden files follow existing patterns.
- **No TOML changes required for the default path.** Cache hints should be applied automatically where supported. Users should not need to touch agent config to benefit.

### Decisions Already Made / Approaches Ruled Out

- **Gemini context caching is out of scope for this milestone.** It requires a stateful two-phase API (create cache → reference cache in request) that does not fit axe's per-turn request builder.
- **Stateless design preserved.** We will not hold a long-lived cache reference ID in the runner. Each request is self-contained with inline cache hints.

---

## Section 2: Requirements

### R1: Cache annotations in provider requests
The provider request abstraction must support attaching cache-control metadata to content blocks. The metadata is opaque to the runner and provider-agnostic at the type level; each provider maps it to its native wire format where applicable.

### R2: Cache metrics in provider responses
The provider response abstraction must expose cache-related token counts (at minimum: tokens read from cache, tokens written to cache) in addition to standard input and output tokens. Numeric zero values mean "not reported by the provider."

### R3: Anthropic provider caching
When calling Anthropic models, the provider must send explicit cache-control hints on the system prompt blocks and tool definition blocks. It must parse cache-related usage fields from the API response.

### R4: OpenAI provider cache reporting
The OpenAI provider must parse `usage.prompt_tokens_details.cached_tokens` (or equivalent) from the API response and surface it as cache read tokens. No request-level changes are required because OpenAI caching is automatic.

### R5: Bedrock provider caching
When calling Claude models through AWS Bedrock, the provider must send `cachePoint` hints on stable content blocks (system prompt, tool config, message prefixes). It must parse cache usage fields from the Converse API response.

### R6: Runner automatically applies cache hints
The agent runner must automatically annotate the system prompt and tool definitions with cache hints on every provider request. This must happen without agent TOML changes and without user configuration.

### R7: Observability
Cache read and write token counts must be:
- Tracked cumulatively across all turns of a conversation.
- Included in the JSON output envelope when `--json` is used.
- Printed in verbose mode alongside input/output token counts.

### R8: Graceful degradation
If a provider or model does not support caching, if the API rejects a cache hint, or if the response omits cache usage fields, the agent run must continue without error. Cache-related behavior must be silent and safe to ignore.

### R9: Sub-agent isolation
Each sub-agent invocation is an independent provider request with its own system prompt and tools. Cache hints must be generated independently per sub-agent. No cache state or cache IDs may be shared between parent and child agents.

---

## Parallel Work

The following workstreams can proceed in parallel once the core cache abstraction (R1, R2) is defined:

1. **Anthropic provider** (R3) + **OpenAI provider** (R4) — implemented by the same owner or in parallel.
2. **Bedrock provider** (R5) — depends on the same abstraction; can proceed concurrently with Anthropic/OpenAI.
3. **Runner integration** (R6) + **Observability** (R7) — depends on the abstraction; can be developed in parallel with provider implementations.
4. **Test coverage** — mock server updates, table-driven unit tests, and integration tests can be written in parallel for each provider.
