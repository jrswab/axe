# Milestone 001: Stable Messages and Chat caching

## Context & Constraints

### Scope and current behavior

This is the complete working slice for milestone 001 of [the milestone record](053_corrected_caching_milestones.md), lines 41–43. Existing source is authoritative; this document specifies the intended behavior, not a claim that it is already implemented. Main means an Agent Run entered via the CLI or public runner; Nested means a Run entered through `call_agent`. Each Nested Run resolves its own agent and context, and its Outcome exposes only final text and a closed cause, not its turns, files or token counts (`GLOSSARY.md:8-25`).

The existing `params.cache_enabled` is an optional boolean (`internal/agent/agent.go:76-84`). Main resolves omission to true and respects an explicit value (`pkg/runner/run.go:438-452`); the Nested request literal (`internal/tool/tool.go:247-255`) currently omits `CacheConfig`. The existing request boolean and response counters suffice (`internal/provider/provider.go:74-101`); no new caching abstraction is needed. Nested task and optional context are user-message content, separate from its resolved system prompt (`internal/tool/tool.go:170-185,236-255`). Main accepts prompt/stdin or pre-built message history separately from the system prompt (`pkg/runner/run.go:276-297,430-452`).

Anthropic currently places an ephemeral breakpoint on a nonempty system when enabled, omits empty system, and shares construction between normal and existing stream paths (`internal/provider/anthropic.go:218-265,377-398`). The independently retrieved [Anthropic prompt caching guide](https://platform.claude.com/docs/en/build-with-claude/prompt-caching), as recorded in the [contract research](../../.scratch/issue-83/assets/provider-cache-contracts-research.md) retrieved 2026-11-18 UTC, defines prefix order tools → system → messages: a system breakpoint covers preceding tools. A tools-only request currently has no breakpoint. Eligibility depends on provider/model constraints and cache availability; a valid hint does not guarantee a hit. The schema converters append required names while ranging over maps (`internal/provider/anthropic.go:183-207`, `internal/provider/openai.go:227-255`); this threatens prefix stability but is not an observed miss.

OpenAI Chat currently sends no Axe cache controls and maps reported cached reads, not writes, in normal and existing stream paths (`internal/provider/openai.go:265-305,379-384,436-480,552-586`). The dated [OpenAI caching guide](https://developers.openai.com/api/docs/guides/prompt-caching) describes rendered-prefix matching; the [Chat create reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create.md) and [stream schema](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events.md) document optional controls and write counts, but the research records conflicting lookup descriptions and uncertain model coverage and write accounting. The [closed boundary decision](../../.scratch/issue-83/openai-completion-boundary.md) retains automatic requests and reads only. An Axe opt-out cannot promise to disable provider-managed caching; an interrupted stream may lack final usage.

MiniMax shares the Anthropic adapter (`internal/provider/minimax.go:13-27`); Atlas Cloud, Cheaper Inference, Eden AI, LiteLLM, Runcrate and Standard Compute share OpenAI behavior (`internal/provider/atlascloud.go:5-28`, `internal/provider/cheaperinference.go:5-29`, `internal/provider/edenai.go:14-44`, `internal/provider/litellm.go:8-40`, `internal/provider/runcrate.go:5-29`, `internal/provider/standardcompute.go:5-29`). OpenRouter also uses the OpenAI tool converter (`internal/provider/openrouter.go:149,293`). These are regression surfaces, not newly established provider caching contracts. LiteLLM opts into cost reporting. Existing generic nonstream transient retry and no stream retry are in `internal/provider/retry.go:49-99,121-145`.

### Settled boundaries and policies

- The approved [recovery decision](../../.scratch/issue-83/cache-rejection-recovery.md) adds no cache-disable replay: the cited provider error research did not establish both a stable cache-only discriminator and safe replay, particularly after partial output or an ambiguous transport outcome. Generic 400s, categories or substrings do not establish either condition. Preserve original errors; a cache-specific diagnostic is actionable only when its cause is actually identifiable.
- Bedrock production, schema and Nested AWS setup corrections are milestone 002, including its separate human approval before rewriting contradicted assertions (`docs/plans/053_corrected_caching_milestones.md:18-19,29,44-46`). Milestone 003 owns verbose-without-budget and expanded accounting evidence (`docs/plans/053_corrected_caching_milestones.md:47-49`). Neither is a prerequisite or implementation task here; preserve existing behavior at their boundaries.
- No live probes, credential use, paid calls, new cache-off replay, model-policy tables, TTL or pricing features, Responses migration, new OpenAI explicit controls (`prompt_cache_options`, `prompt_cache_breakpoint`) or OpenAI cache-write mapping. No hit, savings, universal eligibility, service acceptance or provider-managed opt-out guarantee. Optional later probes are not acceptance criteria or authorized here.
- Preserve public Go, CLI, TOML and JSON interfaces and their semantics under `COMPATIBILITY.md`; no new public surface is required by this acceptance. Keep pipeable stdout, diagnostics on stderr, existing budget accounting, zero omission and Nested Outcome opacity. Existing counts are provider reports, not proof of savings or caching state (`pkg/runner/run.go:621-637,676-683,778-803`; `pkg/runner/result.go:120-131`). Follow provider anchor policy in `AGENTS.md` and `TAGS.md` for changed boundaries; preserve unrelated assertions. Assertion rewrites or acceptance changes require separate human approval and are not authorized here.

## Requirements

### R1 — Independent cache choice

For every Main and Nested Run, resolve its own agent definition's `params.cache_enabled`: omitted → true, explicit true → true, explicit false → false. Apply that resolved value to that Run's provider request on every turn; do not inherit the parent's choice. Verify all three settings independently for each entry path, and both contradictory pairs (parent true/Nested false and parent false/Nested true), observing each request. Include omitted on either side so defaulting cannot be confused with inheritance. This choice controls Axe's applicable explicit hints; it does not assert that provider-managed caching is disabled.

### R2 — Anthropic static prefix and dynamic conversation

For both normal and existing streaming Messages requests, verify each row with caching enabled and disabled:

| Static input | Enabled request | Disabled request |
| --- | --- | --- |
| Nonempty system and tools | An ephemeral system breakpoint covers system and preceding tools; no redundant tool breakpoint is needed. | No Axe cache breakpoint. |
| Nonempty system, no tools | An ephemeral system breakpoint covers the system. | No Axe cache breakpoint. |
| Empty system, tools present | A contract-valid ephemeral breakpoint covers the complete ordered tool prefix. | No Axe cache breakpoint. |
| Empty system, no tools | No synthetic system or breakpoint, including on messages. | No synthetic system or breakpoint. |

Preserve system and tool content and caller-supplied tool order. Task, explicit Nested context, user messages, pre-built history, assistant turns, tool calls and results remain in the conversation rather than being moved into static system or tool definitions or designated as the cache breakpoint. Verify repeated turns with unchanged static inputs and changing dynamic content: the static prefix remains identical, while the dynamic content and its ordering are retained. The prefix is eligible for provider consideration only; no hit is asserted.

### R3 — Stable tool schemas and Chat scope

For Anthropic Messages and OpenAI Chat normal and existing stream requests, identical tool definitions with multiple required parameters produce the same serialized static tool definitions on repeated requests regardless of parameter-map iteration or insertion order. The required array contains exactly the required names, once each; optional parameters remain optional, properties retain their types and descriptions, and the caller's tool order is unchanged. Exercise more than one tool and multiple required parameters; verify across repeated turns as well as repeated construction. OpenRouter's shared converter must retain its existing tool and conversation semantics.

OpenAI Chat requests remain automatic for both cache choices: do not send the deferred explicit controls or map OpenAI cache writes. Preserve normal and streaming cached-read mapping, including absent and zero usage; do not invent missing final usage after interruption. Preserve Anthropic normal and stream read/write mapping. No new streaming path is introduced.

### R4 — Errors, retries and compatibility

Preserve original and unrelated provider errors, their existing category/status/message and caller-visible propagation, including pre-stream failure, failure after partial text or tool output, and ambiguous transport outcomes. No new cache-disable replay occurs for these cases. Preserve existing configured transient nonstream retry behavior and existing absence of stream retries; tests distinguish those retries from a new cache fallback. Do not label a generic error cache-specific without an identifiable cause.

Regression evidence covers MiniMax and the listed OpenAI-compatible adapters, including text and tool streams, optional or absent usage, errors, and LiteLLM's optional cost behavior as applicable to their existing paths. Preserve OpenRouter behavior affected by the shared converter. Do not imply these adapters support first-party caching features merely because they share code.

### R5 — Evidence, documentation and completion

Use deterministic behavioral evidence at request and run boundaries, with raw-wire expectations derived independently from the dated official contracts above, not production serialization structs as the oracle. Cover R1–R4, enabled/disabled static cases, multi-required schemas, missing/zero and existing cumulative usage, pre-stream and partial-stream errors, uncertain outcomes and compatible-adapter regressions. Fixtures prove wire intent and local behavior, not live acceptance or hits. Keep unrelated existing assertions; stop for separate approval if an assertion rewrite is needed.

Ship matching durable documentation with the behavior, not in a later milestone: update `README.md` and `docs/src/configuration/agent-config.md` to describe per-agent omission/true/false, independent Nested choice, the Anthropic static-prefix boundary, and OpenAI automatic caching/opt-out limitation; update `docs/src/providers.md` with the applicable Anthropic and OpenAI boundaries and `docs/src/cli/json-output.md` with existing provider-reported cache-read and cache-write fields, their zero omission and own-run cumulative meaning, without claiming Bedrock corrections or milestone 003 behavior. Add an appropriate `CHANGELOG.md` entry for this slice. Do not claim cache hits or savings; distinguish reported reads/writes from savings and avoid promising universal eligibility. Ensure any other durable documentation affected by the implemented slice agrees with it. No project memory corpus exists; update existing relevant records if one exists when implementation occurs, without creating one solely for this spec (`AGENTS.md`).

For implementation completion, run focused touched-package race tests, `go test -race ./...`, `golangci-lint run`, and `make check` or its constituents (`golangci-lint run`, `go test ./...`, `make apidiff`); also run applicable `CONTRIBUTING.md` checks including `.agents/scripts/ci-check integrity`, `vet`, and `build`. Report each applicable gate not run and why. Docker smoke, if applicable, requires a declared host gate or human host step; do not fabricate sandbox availability. These are implementation gates, not requirements to run Go tests for this Markdown-only specification.

### Dependencies and parallel work

R1 enables independent Nested choice in R2. R2 and R3 can be developed in parallel after agreeing on request-boundary evidence; R4 regression evidence and R5 documentation can be developed in parallel with them but must be checked against the integrated slice. All requirements and accompanying documentation must pass together before milestone 001 is complete. No implementation or release is authorized by this specification alone.
