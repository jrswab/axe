# Milestone 010: Large-integer tool arguments — implementation checklist

## Section 1: Context Summary

Fix the approved MCP large-integer bug at the provider-to-string boundary, not in MCP coercion. Today float64 `%v` turns 1000000 into `1e+06`, which integer coercion leaves as a string. The authoritative `docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md` approves exact top-level mathematical integers, including integral decimal/exponent forms, only for absolute values >=1000000, at most 309 normalized digits excluding sign, and successful `ParseFloat(normalized,64)` equal to legacy `ParseFloat(original,64)`. Otherwise preserve legacy formatting/error. Nested collections retain entire legacy float64-derived text. Keep existing signatures, response map types, `Arguments map[string]string`, replay string types, MCP `coerceArg`, and testutil production unchanged. No dependencies, RawArguments, token rejection limit, changelog/version change, or future-milestone work. Full lane; planning round 3 APPROVED, historical failed planning rounds remain 2/3. Review base is `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`; approval/fix HEAD is `b65d89d6bb32a5b575ded581496708806586a9ac`.

## Section 2: Implementation Checklist

### Rules applying to every task

Use the `code` sub-agent for implementation. Each code call has Budget: 30 min. Work one behavioral RED → minimal GREEN cycle at a time; do not bulk-write imagined tests or refactor unrelated surfaces. Agreed seams are provider `Send`, real `SendStream` → runner drain / `Run`, returned history, follow-up request body, raw MCP handler bytes, and the spec-required formatter table. Fake only HTTP/MCP boundaries using raw-body httptest fixtures. Never substitute a capturing provider for a required real-provider route. No `internal/testutil` modifications.

Every new test name below is **NEW**, not an existing selector proven with `go test -list`. The documented commands run full packages with verbose output; planned names are literal required-output contracts, not executable selectors for tests that do not yet exist. After writing each named NEW test, run its documented RED command before converting its route; verify verbose output shows its exact name and that it failed on missing intended behavior (such as exponent/rounded digits), not compilation failure from a missing helper or an unrelated test failure. RED tests use existing callable interfaces. For confirmation commands, first write both named NEW tests, then require both exact names in verbose output and passing results. Once T02 switches drain, already-correct text-delta routes and history propagation are honest test-only confirmations, not claimed missing-behavior REDs. Error controls also confirm preserved behavior rather than necessarily failing baseline.

For every route apply these three reusable acceptance packs alongside that route’s implementation:

- **Digits pack (R7):** raw numeric `1000000` and `9007199254740993`, approved `1e6`, `1000000.0`, `-1e6`, and >2^53 equivalent `.0` / `90071992547409930e-1`; independent exact expected string literals. Buffered `Send` asserts stored Arguments. Each available real stream path asserts drained Arguments in `pkg/runner`, never only deltas. Each buffered and stream route separately uses real `Run` with nonempty pre-built `Options.Messages`, a tool response then final response, asserting `result.Messages[].ToolCalls[].Arguments` for both integers and approved forms. Bedrock uses the specified private real-provider injection instead. Set JSON mode and assert detail inputs, hook Arguments, and observer Call.Arguments alongside route history cases; field names/types remain unchanged. This verifies existing propagation, with no propagation code edits expected.
- **Decode pack (R2/R3):** at EACH listed changed boundary, malformed input, trailing value and trailing junk; `1e400` and 400-digit integer top-level/nested; duplicate-overflow `{"n":1e400,"n":1000000}` and replaced-subtree-overflow `{"n":{"bad":1e400},"n":1000000}`; successful duplicates `{"n":999999,"n":1000000}` and `{"n":{"ok":1},"n":1000000}`. Unknown response-field scalar/nested overflow controls must remain ignored where the original struct decoder skips them. Preserve destination type errors (`[]`, scalar, string, bool as an argument map), `{}`/`null` map success, and specified server ProviderError versus empty-args fallback. No `+Inf` stored. For string-argument response structs, put argument-overflow cases inside the decoded string destination; separately test outer trailing response and numeric typed response fields where applicable, not an invented outer argument map.
- **Unaffected pack (R8, supporting each slice):** both signs below threshold, `999999.0`, `0`, `-0`, `-0.0`, `1.5`, fractional rounding traps, strings (`"1e6"`, `"1000000.0"`, `"9007199254740993"`), raw `{"n":"text"}` → stored `"text"`, booleans, null, and nested objects/arrays. Require old text, especially `map[a:1e+06]` for either `{a:1e6}` or `{a:1000000}`, and `[1.5]` for `[1.50]`. Use a shared test-only fixture builder only if needed; keep it in scoped test files, no harness production changes.

Per-task gate: run `go test -race` only on the packages this change touched, derived before commit as below. Do not run `go test -race ./...`; the Factory completion gate runs it.

Copy this exact Factory derivation into each task’s execution brief and apply the environment below to the entire shell:

```sh
changed=$({ git diff --name-only --relative HEAD; git ls-files --others --exclude-standard; } | grep '\.go$' | xargs -r -n1 dirname | sort -u)
pkgs=$(go list -e -f '{{.Dir}}' ./... | while read -r d; do r=$(realpath --relative-to=. "$d"); printf '%s\n' "$changed" | grep -qxF "$r" && echo "./$r"; done)
if [ -n "$pkgs" ]; then go test -race $pkgs; fi
```

All tests/gates use `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL=` with the checkpoint’s unchanged PATH. `cmd` suites additionally use `XDG_CACHE_HOME=$(mktemp -d)`. Never use `-count=1` or `-p1` without a demonstrated timing/randomness need. Every touched-package FULL race suite is required; the verbose behavioral checks below do not replace it. Before each commit: derive packages, format only changed/new Go files, require empty changed-file `gofmt -l`, `git diff --check`, `go list -m`, `go build ./...`, `go vet ./...`, `make lint` (configured golangci-lint), and `make apidiff` (Makefile targets). Clear the same env for make invocations too. Flag unavailable tool/dependency/API gates, never silently skip, install tools, or add `GOPROXY=`. No such install/proxy command is needed, so no new toolchain preflight is triggered. Do not clean unrelated baseline formatting. All task verification statements below include this common gate.

### OpenAI tracer slice and bounded shared machinery

- [x] T01a — Prove OpenAI raw numeric failure and preserve decoding semantics.
  Budget: 30 min
  Primary R-IDs: R2. Dependency: none. Enables T01b’s first real `Send` slice.
  Source: new `internal/provider/tool_arguments.go: DecodeJSON[T any](data []byte, dst *T) error`; `internal/provider/openai.go: OpenAI.Send` response decode at line 314 and argument-map decode at line 340.
  Tests: `internal/provider/openai_test.go`, new `internal/provider/tool_arguments_test.go`. NEW `TestLargeIntegerArgs_OpenAI_Buffered` calls existing Send and asserts exact raw-number Arguments; do not invoke a missing helper for the first RED. NEW `TestLargeIntegerArgs_OpenAI_DecodeErrors` applies decode pack at both boundaries. Preserve existing `TestOpenAI_Send_InvalidToolCallArguments` at line 989.
  RED: after writing NEW `TestLargeIntegerArgs_OpenAI_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenAI_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Implement DecodeJSON: first `json.Unmarshal` into fresh zero T of EXACT legacy destination type; return its error. Then number-preserving UseNumber decode with single-value/EOF semantics. Keep all response maps as current map types; no RawMessage schema rewrite, final-graph-only validation, duplicate rejection, or blanket unknown-field validation. Wire OpenAI’s two calls, retaining outer server ProviderError and inner empty-args fallback. Do not commit a failing slice: T01a’s digit RED is the enabling handoff to T01b; its decode semantics controls pass, then complete T01b before the first slice commit/full gates.
  Depends on reads: none (no write removal).

- [x] T01b — Complete OpenAI exact top-level formatting and bounded arithmetic.
  Budget: 30 min
  Primary R-IDs: R1. Dependency: T01a; completes first buffered tracer slice.
  Source: `internal/provider/tool_arguments.go: FormatToolArgument(value any) string`; `internal/provider/openai.go: OpenAI.Send` flatten line 342.
  Tests: `internal/provider/tool_arguments_test.go: NEW TestLargeIntegerArgs_Formatter`; `internal/provider/openai_test.go: NEW TestLargeIntegerArgs_OpenAI_Buffered` from T01a.
  RED: after extending NEW `TestLargeIntegerArgs_OpenAI_Buffered`, repeat T01a’s exact RED command, still asserting through Send. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenAI_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure; add formatter table cycles after the helper exists. Keep input-length-plus-309-digit work/memory bounds. Exact coefficient/exponent analysis, not Float64, decides mathematical integrality/threshold/output digit count. Never allocate exponent-sized values, reject a long valid token, or use rounded Float64 to classify fractional traps. Require legacy parse success and normalized parse equality before choosing exact digits; preserve legacy sign-zero/fallback. Recursively restore ALL nested json.Number values to float64 before entire collection formatting. Table includes every numeric/collection row in Section 4 below, both signs, max finite 309 digits, cap overflow, A/B, and signed/unsigned zero huge exponents. Assign raw `{"n":"text"}` → stored `"text"` to NEW `TestLargeIntegerArgs_Formatter`; assign numeric `99999999999999999999999.0` and `999999999999999999999990e-1` to that formatter table and NEW `TestLargeIntegerArgs_OpenAI_Buffered`, each asserting the independent expected normalized Arguments digit string `"99999999999999999999999"`. Observe GREEN through OpenAI Send; run common full gates before committing T01a+b together.
  Depends on reads: none (stored map shape retained).

- [x] T01c — OpenAI buffered history and real-decode replay.
  Budget: 30 min
  Primary R-IDs: R6, R7, R8. Dependency: T01b.
  Source read-only: `internal/provider/openai.go: convertToOpenAIMessages` lines 149-201, replay lines 165-177; `pkg/runner/run.go: Run/run` history append/return lines 693-698 and 816-817.
  Tests: `internal/provider/openai_test.go: NEW TestLargeIntegerArgs_OpenAI_Replay`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenAI_BufferedHistory`. First decode raw numeric Send response, then follow-up with returned ToolCalls; inspect arguments JSON string as map[string]string, retaining quoted values and exact digits, no exponent text. Apply digits/unchanged packs to buffered returned history through real Run. No replay production changes expected.
  Confirmation command (not an invented RED after T01b): after writing both NEW tests, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider ./pkg/runner`. Require exact names `TestLargeIntegerArgs_OpenAI_Replay` and `TestLargeIntegerArgs_OpenAI_BufferedHistory` in verbose output, both passing, and the package command to pass.
  Depends on reads: none.

### Runner drain switch before native emitter removals

- [x] T02 — Switch drain and prove OpenAI real stream/history.
  Budget: 30 min
  Primary R-IDs: R3, R7, R8. Dependency: T01b.
  Source: `pkg/runner/run.go: drainEventStream` lines 884-976, DecodeJSON call at 936 and FormatToolArgument at 938. Leave API and history propagation unchanged.
  Tests: `pkg/runner/stream_test.go: NEW TestLargeIntegerArgs_DrainErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenAI_StreamHistory`. Real OpenAI SendStream with raw numeric SSE → drain and real Run(stream=true) with nonempty Messages; apply stream/history digits and unchanged packs. The first failing assertion is drained exact integer text, not provider raw deltas. Decode pack at drain, both start-input and concatenated delta input; errors preserve empty args. Existing patterns: `TestDrainEventStream_ToolCalls` (line 72), `TestDrainEventStream_ToolCallsFromStartInput` (line 109), `TestRun_PreBuiltMessages_Streaming` (`pkg/runner/run_test.go:579`), `TestRun_PreBuiltMessages_ToolLoop` (line 969).
  RED: after writing NEW `TestLargeIntegerArgs_OpenAI_StreamHistory`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./pkg/runner`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenAI_StreamHistory` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none; this task switches the readers needed by T09/T11.

### Remaining real-provider slices

- [x] T03 — Anthropic buffered/stream/history and real-decode replay.
  Budget: 30 min
  Primary R-IDs: R2, R6, R7. Dependency: T02.
  Source: `internal/provider/anthropic.go: Anthropic.Send` decode 305/flatten 331; read-only `Anthropic.SendStream` and `convertToAnthropicMessages` replay 160-165. DecodeJSON then FormatToolArgument; outer errors remain server ProviderError. MiniMax inherits this route.
  Tests: `internal/provider/anthropic_test.go: NEW TestLargeIntegerArgs_Anthropic_Buffered`, `NEW TestLargeIntegerArgs_Anthropic_Replay`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Anthropic_BufferedHistory`, `NEW TestLargeIntegerArgs_Anthropic_StreamHistory`. Patterns read: `TestAnthropic_Send_ToolCallResponse` (line 653), `internal/provider/anthropic_stream_test.go: TestAnthropic_SendStream_ToolCall` (line 389). Apply buffered decode pack, digits and unchanged packs for both modes/history. Replay starts from decoded raw response and preserves string-valued input object. Stream is confirmation after T02, not a claimed emitter defect; no text-delta parser conversion needed.
  RED: after writing NEW `TestLargeIntegerArgs_Anthropic_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Anthropic_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T04 — OpenRouter buffered/stream/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T02.
  Source: `internal/provider/openrouter.go: OpenRouter.Send` response decode 209, argument decode 235, flatten 237; read-only SendStream. Wire both DecodeJSON calls and formatter; outer server error, inner empty args. Existing float64 Cost remains typed float64 and keeps overflow semantics.
  Tests: `internal/provider/openrouter_test.go: NEW TestLargeIntegerArgs_OpenRouter_Buffered`, `NEW TestLargeIntegerArgs_OpenRouter_DecodeErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenRouter_BufferedHistory`, `NEW TestLargeIntegerArgs_OpenRouter_StreamHistory`. Read stream pattern `TestOpenRouter_SendStream_ToolCall` (line 531). Apply packs at each buffer boundary and both histories; real stream confirmation after T02.
  RED: after writing NEW `TestLargeIntegerArgs_OpenRouter_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenRouter_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T05 — OpenCode Claude route buffered/stream/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T02.
  Source: `internal/provider/opencode.go: sendClaude` decode 286/flatten 308; existing Send routing via `claude-*`; read-only streamClaude and convertToOCAnthropicMessages. Add helper calls only at the buffered sites, server errors unchanged.
  Tests: `internal/provider/opencode_test.go: NEW TestLargeIntegerArgs_OpenCodeClaude_Buffered`, `NEW TestLargeIntegerArgs_OpenCodeClaude_DecodeErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenCodeClaude_BufferedHistory`, `NEW TestLargeIntegerArgs_OpenCodeClaude_StreamHistory`. Patterns: `TestOpenCode_Send_Claude_ToolUse` (line 991), `TestOpenCode_SendStream_ClaudeRoute` (line 1090). Use tool SSE instead of the latter’s text-only fixture. Apply packs, both returned histories; stream is confirmation after T02.
  RED: after writing NEW `TestLargeIntegerArgs_OpenCodeClaude_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenCodeClaude_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T06 — OpenCode Responses route buffered/stream/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T05 (same source/test files serialize edits).
  Source: `internal/provider/opencode.go: sendGPT` response decode 1015, argument-string decode 1040, flatten 1045; Send uses `gpt-*`; read-only streamGPT / convertToOCResponsesMessages replay 946. Both decode errors remain server ProviderError, NOT OpenAI empty fallback.
  Tests: `internal/provider/opencode_test.go: NEW TestLargeIntegerArgs_OpenCodeResponses_Buffered`, `NEW TestLargeIntegerArgs_OpenCodeResponses_DecodeErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenCodeResponses_BufferedHistory`, `NEW TestLargeIntegerArgs_OpenCodeResponses_StreamHistory`. Pattern `TestOpenCode_SendStream_GPTRoute` (line 1154) already emits argument text; substitute raw numeric argument delta and assert via drain/Run. Apply packs at both buffer boundaries, both histories; stream confirmation after T02.
  RED: after writing NEW `TestLargeIntegerArgs_OpenCodeResponses_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenCodeResponses_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T07 — OpenCode Chat route buffered/stream/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T06.
  Source: `internal/provider/opencode.go: sendChatCompletions` response decode 1184, argument decode 1206, flatten 1211; non-Claude/non-GPT Send routing; read-only streamChatCompletions / convertToOCChatMessages replay 1114. DecodeJSON twice, formatter once, both errors stay server ProviderError.
  Tests: `internal/provider/opencode_test.go: NEW TestLargeIntegerArgs_OpenCodeChat_Buffered`, `NEW TestLargeIntegerArgs_OpenCodeChat_DecodeErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_OpenCodeChat_BufferedHistory`, `NEW TestLargeIntegerArgs_OpenCodeChat_StreamHistory`. Pattern `TestOpenCode_SendStream_ChatCompletionsRoute` (line 1224) is text-only: use numeric tool SSE, not its text assertion as regression. Apply packs at both buffer boundaries and both histories; stream confirmation after T02.
  RED: after writing NEW `TestLargeIntegerArgs_OpenCodeChat_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_OpenCodeChat_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T08 — Gemini buffered/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T01b.
  Source: `internal/provider/gemini.go: Gemini.Send` decode 322/flatten 351. Keep `geminiFunctionCall.Args map[string]interface{}`; DecodeJSON fresh geminiResponse and formatter; preserve server errors.
  Tests: `internal/provider/gemini_test.go: NEW TestLargeIntegerArgs_Gemini_Buffered`, `NEW TestLargeIntegerArgs_Gemini_BufferedErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Gemini_BufferedHistory`. Pattern `TestGemini_Send_ToolCallResponse` (line 482). Apply buffer decode pack/digits/unchanged and returned history with raw-body fixture, never string-only GeminiToolCallResponse helper.
  RED: after writing NEW `TestLargeIntegerArgs_Gemini_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Gemini_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T09 — Gemini native raw emitter and real stream/history.
  Budget: 30 min
  Primary R-IDs: R3, R4, R7. Dependencies: T02, T08.
  Source: `internal/provider/gemini.go: Gemini.SendStream` chunk decode 500 and flatten/remarshal 529-539. DecodeJSON with unchanged map fields; REMOVE flatten-to-string loop, marshal raw FunctionCall.Args directly. This covers the former eleventh-site inventory’s Gemini stream flatten without adding a formatter call to an emitter that must stay raw.
  Tests: `internal/provider/gemini_test.go: NEW TestLargeIntegerArgs_Gemini_NativeToolInput`, `NEW TestLargeIntegerArgs_Gemini_StreamErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Gemini_StreamHistory`. Pattern `TestGemini_SendStream_ToolCalls` (line 1036). Mandatory native test separate from drain checks: raw numeric 1000000 and 9007199254740993 tokens MUST be unquoted; bool/null/object/array MUST keep native JSON shapes. Then real SendStream → drain/Run packs/history; decode pack at chunk preserves server errors, including duplicates before marshal.
  RED: after writing NEW `TestLargeIntegerArgs_Gemini_NativeToolInput`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Gemini_NativeToolInput` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: `pkg/runner/run.go: drainEventStream` ToolStart input read lines 920-921, ToolDelta read 927, accumulated decode/flatten 933-938; switch completed in T02 BEFORE removing emitter writes. Existing stream transport assertions in `internal/provider/stream_test.go: TestEventStream_ToolCallEvents` compare payload transparently and require no type/API change; numeric/native assertions are NEW.

- [ ] T10 — Ollama buffered/history.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T01b.
  Source: `internal/provider/ollama.go: Ollama.Send` decode 290/flatten 303. Keep Arguments map field unchanged, add DecodeJSON and formatter, server errors retained.
  Tests: `internal/provider/ollama_test.go: NEW TestLargeIntegerArgs_Ollama_Buffered`, `NEW TestLargeIntegerArgs_Ollama_BufferedErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Ollama_BufferedHistory`. Pattern `TestOllama_Send_ToolCallResponse` (line 718). Apply packs and buffered returned history via real Run/raw NDJSON-compatible body, no string-only helper.
  RED: after writing NEW `TestLargeIntegerArgs_Ollama_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Ollama_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

- [ ] T11 — Ollama native raw emitter and real stream/history.
  Budget: 30 min
  Primary R-IDs: R3, R4, R7. Dependencies: T02, T10.
  Source: `internal/provider/ollama.go: Ollama.SendStream` chunk decode 467, flatten 478-480, ToolInput marshal 487. DecodeJSON into unchanged ollamaResponse; REMOVE flattened args writes and pass raw tc.Function.Arguments to existing mustMarshal.
  Tests: `internal/provider/ollama_test.go: NEW TestLargeIntegerArgs_Ollama_NativeToolInput`, `NEW TestLargeIntegerArgs_Ollama_StreamErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Ollama_StreamHistory`. Pattern `TestOllama_SendStream_ToolCalls` (line 1004). Separate mandatory native ToolInput assertions identical in coverage to T09, followed by real drain/history packs; chunk decode pack retains server errors. Existing scanner limits remain unchanged; huge-token helper resource cases do not authorize changing transport limits.
  RED: after writing NEW `TestLargeIntegerArgs_Ollama_NativeToolInput`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Ollama_NativeToolInput` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: same runner start/delta/accumulated reads as T09, switched in T02. No other production ToolInput consumer was found; transport tests compare strings transparently.

- [ ] T12 — Bedrock buffered/history only.
  Budget: 30 min
  Primary R-IDs: R2, R7. Dependency: T01b.
  Source: `internal/provider/bedrock.go: Bedrock.Send` decode 348 and `parseBedrockResponse` flatten 280. DecodeJSON then formatter; no existing signature change. No SendStream exists.
  Tests: `internal/provider/bedrock_test.go: NEW TestLargeIntegerArgs_Bedrock_Buffered`, `NEW TestLargeIntegerArgs_Bedrock_DecodeErrors`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_Bedrock_BufferedHistory`. Pattern `TestBedrock_Send_ToolUse` (line 97); replace its string-only Input fixture with raw numeric response. Apply buffer decode/digits/unchanged packs. History uses existing private `pkg/runner/run.go: run` (line 65) with newProvider callback returning REAL `provider.NewBedrock("us-east-1", provider.WithBedrockBaseURL(server.URL))`, explicit `t.Setenv("AWS_REGION", "us-east-1")` in `TestLargeIntegerArgs_Bedrock_BufferedHistory`, fake isolated AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY and cleared session token; never capturing provider or production factory change. Keep this environment-mutating history test nonparallel. Nonempty Options.Messages and final response required.
  RED: after writing NEW `TestLargeIntegerArgs_Bedrock_Buffered`, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/provider`. Verify verbose output shows exact name `TestLargeIntegerArgs_Bedrock_Buffered` and that it failed on missing intended behavior, not compilation or unrelated test failure.
  Depends on reads: none.

### MCP end-to-end acceptance (no coercion production change)

- [ ] T13 — Verify raw numeric provider-to-MCP wire and observable inputs.
  Budget: 30 min
  Primary R-IDs: R5, R8, R9. Dependency: T02; route-wide histories/details remain assigned to their provider tasks.
  Source read-only: `internal/mcpclient/mcpclient.go: coerceArg` lines 124-140, `CallTool`; `pkg/runner/run.go: run` dispatch/observer/details/history lines 716-772 and 816-817. Production behavior is already enabled by T01b/T02; this is acceptance confirmation, not a new missing-behavior RED or coercion edit.
  Tests: `internal/mcpclient/mcpclient_test.go: NEW TestLargeIntegerArgs_MCPWire`; new `pkg/runner/large_integer_args_test.go: NEW TestLargeIntegerArgs_MCPIntegration`. Patterns `TestCoerceArg` (line 323), `TestCallTool_CoercesArgTypes` (line 388), `pkg/runner/hook_mcp_test.go: newHookTestMCPServer` (line 17) and `TestToolHookMCPAllow` (line 118); `pkg/runner/run_test.go: TestToolHookArgumentsAsParsed` (line 1595), `TestToolObserverOrderAndArguments` (line 1889). Use raw SDK server.AddTool handler and capture req.Params.Arguments BYTES before any float64 decode/schema rewrite. A hand-built Arguments client check is honestly a confirmation; regression acceptance must begin raw numeric real-provider Send/Run → dispatch → raw handler. Cover buffered and OpenAI real-stream MCP end-to-end flows, both signs, 1234567, >2^53/equivalents, approved decimal/exponent forms, eligible beyond-int64 digit strings, JSON string/schema controls, nested fallbacks, A/B and number-parameter signed zero/rounding boundary preservation. For number schema compare resulting float64 numerical value and signbit against independently established legacy result, NOT equal Arguments text; wire token assertions must not lose digits via float64 decoding. Capture exact normalized hook/observer/detail inputs in JSON Run mode and preserve existing field types.
  Confirmation: after writing both NEW tests, run `env -u GOTOOLCHAIN ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -v ./internal/mcpclient ./pkg/runner`. Require exact names `TestLargeIntegerArgs_MCPWire` and `TestLargeIntegerArgs_MCPIntegration` in verbose output, both passing, and the package command to pass.
  Assign numeric `99999999999999999999999.0` and `999999999999999999999990e-1` to NEW `TestLargeIntegerArgs_MCPIntegration` in both its buffered and OpenAI real-stream flows: assert independent expected normalized Arguments `"99999999999999999999999"` and raw MCP handler bytes containing `{"n":"99999999999999999999999"}` for an integer parameter. Assign the same quoted raw-handler expectation to NEW `TestLargeIntegerArgs_MCPWire` using that normalized Arguments string. The digit string MUST remain QUOTED because ParseInt fails beyond int64; do not derive expectations from the formatter or a float64 decode.
  Depends on reads: none. coerceArg stays untouched.

### Parallelism and execution order

T01a → T01b completes the first working provider slice and commits together, followed by T01c/T02. After T02, independent source files T03/T04/T08/T10/T12 can be assigned in parallel ONLY with isolated worktrees or non-overlapping test files; the proposed shared runner test file otherwise serializes edits. T05 → T06 → T07 serialize OpenCode. T09 requires T08+T02 and T11 requires T10+T02. T13 follows T02. Budget exhaustion or an out-of-scope prerequisite is a blocker, not permission to expand. No task/milestone boxes are checked in this planning call.

## Section 3: Caller and read-path inventory

Existing symbols need NO signature changes, so no existing caller signature migrations. NEW helpers are the settled names `FormatToolArgument(value any) string` and `DecodeJSON[T any](data []byte, dst *T) error`, in new `internal/provider/tool_arguments.go` with `internal/provider/tool_arguments_test.go`.

Read-only repository-wide searches performed in this document pass:

```sh
rg -n 'FormatToolArgument|DecodeJSON' . --glob '*.go'
rg -n 'fmt.Sprintf\("%v", v\)|json.Unmarshal|ToolInput' internal/provider pkg/runner
rg -n 'ToolInput' --glob '*.go' --glob '!vendor/**' .
rg -n '^func .*' internal/provider/{openai,anthropic,openrouter,opencode,gemini,ollama,bedrock}.go
rg -n '^func Test.*(Tool|Stream|Invalid|PreBuilt|Coerce)' internal/provider/{openai,anthropic,openrouter,opencode,gemini,ollama,bedrock}*test.go pkg/runner/{run,stream,hook_mcp}*test.go internal/mcpclient/mcpclient_test.go
```

Helper collision search returned no Go matches (exit 1). Flatten/decode inventory results and ALL assigned new helper callsites:

| Task | File/function | DecodeJSON sites (current lines) | FormatToolArgument sites / removal |
|---|---|---|---|
| T01a/b | internal/provider/openai.go: OpenAI.Send | 314, 340 | 342 |
| T02 | pkg/runner/run.go: drainEventStream | 936 | 938 |
| T03 | internal/provider/anthropic.go: Anthropic.Send | 305 | 331 |
| T04 | internal/provider/openrouter.go: OpenRouter.Send | 209, 235 | 237 |
| T05 | internal/provider/opencode.go: sendClaude | 286 | 308 |
| T06 | internal/provider/opencode.go: sendGPT | 1015, 1040 | 1045 |
| T07 | internal/provider/opencode.go: sendChatCompletions | 1184, 1206 | 1211 |
| T08 | internal/provider/gemini.go: Gemini.Send | 322 | 351 |
| T09 | internal/provider/gemini.go: Gemini.SendStream | 500 | REMOVE 530; raw marshal 534-539 |
| T10 | internal/provider/ollama.go: Ollama.Send | 290 | 303 |
| T11 | internal/provider/ollama.go: Ollama.SendStream | 467 | REMOVE 479; raw marshal 487 |
| T12 | internal/provider/bedrock.go: Bedrock.Send / parseBedrockResponse | 348 | 280 |

This assigns all 11 existing provider flatten sites plus drain: 9 provider formatter callers, 2 raw-emitter removals, 1 runner formatter caller; all 16 decode calls assigned (13 buffered including string maps, 2 native chunks, 1 drain). Test helper direct calls belong to T01a/b’s new table; route tests primarily invoke Send/Run.

ToolInput search found only one production consumer: `pkg/runner/run.go: drainEventStream` reads start payload at 920-921, deltas at 927, and accumulated text at 933-938. Producers: `openai.go:580`, `openrouter.go:459`, `anthropic.go:529`, `opencode.go:456,617,783` retain raw text; `internal/provider/gemini.go:539` switches after T02. `internal/provider/ollama.go:487` switches after T02. StreamEvent field declaration is `internal/provider/stream.go:22`, unchanged. Test consumers: `internal/provider/openai_test.go:1405,1417`, `anthropic_stream_test.go:431,434,672`, `internal/provider/stream_test.go:141-142`; `pkg/runner/stream_test.go:75-76,79,111,137,308,314-315`, `pkg/runner/large_integer_args_test.go:286` construct payloads. No hidden production read depends on flattened emitter strings.

Replay readers remain unchanged: `openai.go: convertToOpenAIMessages` 165-177 (T01c); `anthropic.go: convertToAnthropicMessages` 160-165 (T03); `gemini.go: convertToGeminiContents` 183-186 (T08/T09 history); `ollama.go: convertToOllamaMessages` 138-141 (T10/T11); `bedrock.go: convertMessages` 213-216 (T12); `opencode.go: convertToOCAnthropicMessages` 164-166 (T05), `convertToOCResponsesMessages` 946 (T06), `convertToOCChatMessages` 1114 (T07). OpenRouter uses convertToOpenAIMessages (T04). AtlasCloud embeds OpenAI (`internal/provider/atlascloud.go:7`); MiniMax returns Anthropic (`internal/provider/minimax.go:14`), so no separate implementation. Production source/test regions cited by tasks were read during checklist research; new files/tests are declared, not falsely inventoried as existing.

## Section 4: Phase-2 extreme evidence

Transcribed **observed code-agent results supplied by the orchestrator**, not probes executed by this document agent. Scratch `/tmp/052-numeric-boundaries/main.go` was inspected read-only in full. Original evidence is in `docs/plans/052_runner_tool_hooks_v1_12_0_010_checkpoint.md:248`; later code-agent run supplied the expanded controls below. Scratch is reproducibility aid, not durable completion gate. Supplied expanded-run commands: `ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= gofmt -l /tmp/052-numeric-boundaries/main.go`, same env `go vet /tmp/052-numeric-boundaries/main.go`, same env `go run /tmp/052-numeric-boundaries/main.go`; all exited 0 on Go1.26.10, GOTOOLCHAIN unset, PATH unchanged. No probe command here sets GOPROXY; historical checkpoint proxy setting is not an execution instruction.

| Actual probe input | Observed legacy result / UseNumber discrepancy | Spec expected / task |
|---|---|---|
| Argument map `{}`, `null` | success, empty/nil map | same; decode pack all boundaries |
| Map `[]`, `0`, `-0`, `-0.0`, true, string | map destination type error | same error/fallback; decode pack |
| Top numeric `0`, `-0`, `-0.0` | `0`, `-0`, `-0` | identical legacy text; T01b/T13 |
| +/-999999, `999999.0`, `1.5` | digits unchanged / `999999` / `1.5` | same; T01b/all route packs |
| +/-1000000, `1e6`, `1000000.0`, `-1e6` | +/-`1e+06` | exact +/-1000000; every route/history, replay, wire |
| `999999.99999999999`, `1000000.00000000001` | Float64 rounds to 1e6, `%v`=`1e+06` | remain legacy fractional, never classify rounded integrality; T01b/T13 |
| `9007199254740993`, `.0`, `90071992547409930e-1` | Float64=9007199254740992; text `9.007199254740992e+15` | exact 9007199254740993; all routes/history/replay/wire |
| 309 digits: `1`+308 zeros | finite/equal ParseFloat, text `1e+308` | exact 309 digits; T01b |
| 309 digits: `17976931348623157`+292 zeros | finite/equal, text `1.7976931348623157e+308` | exact 309 digits; T01b |
| `1e308` | finite, `1e+308` | exact `1`+308 zeros; T01b |
| 310/400-digit literals, `1e309`, `1e400`, `1e1000000000` | legacy JSON error / ParseFloat range error; UseNumber alone succeeds | retain error/fallback, no Inf; T01b and every decode pack |
| `1e-1000000000` | underflows to `0` | legacy fraction `0`, no expansion; T01b |
| A: `1`+100006 zeros+`e-100000` (100015-byte token) | legacy `0`, exact 1000000, normalized ParseFloat equality false | retain `0`, no long-token rejection; T01b/T13 |
| B: `0.`+9999 zeros+`1e100000` (10009 bytes) | legacy `1`, exact 10^90000 (90001 output digits) | retain `1` under cap fallback, no expansion; T01b/T13 |
| `0e1000000000`, `-0e1000000000` | `0`, `-0` | same safe signed zero; T01b/T13 |
| `99999999999999999999999` | legacy `1e+23`, finite/equal candidate | eligible exact 23 digits; integer MCP digit string; T01b/T13 |
| Numeric `99999999999999999999999.0` | Legacy Float64-derived text `1e+23` | independent expected Arguments `"99999999999999999999999"`; integer raw MCP `{"n":"99999999999999999999999"}` (quoted, ParseInt fails); T01b/T13 |
| Numeric `999999999999999999999990e-1` | Legacy Float64-derived text `1e+23` | independent expected Arguments `"99999999999999999999999"`; integer raw MCP `{"n":"99999999999999999999999"}` (quoted, ParseInt fails); T01b/T13 |
| Raw `{"n":"text"}` | legacy stored `"text"` | same stored `"text"`; T01b formatter table/all route unaffected packs |
| Argument JSON followed by `{}` or junk | Unmarshal error; first bare Decode succeeds; second decode EOF check exposes trailing input | complete consumption rejection, same error/fallback; every decode pack |
| Nested `[1.50]`, `{a:1e6}`, `{a:1000000}` | `[1.5]`, `map[a:1e+06]`, `map[a:1e+06]` | restore recursive float64 formatting exactly; T01b/all route packs |
| duplicate scalar/subtree overflow then replacement | legacy map and Gemini/Ollama map-field response errors; UseNumber final graph succeeds | fresh exact legacy destination validation must preserve errors; every decode pack |
| successful scalar/subtree duplicates | legacy succeeds | accept, no blanket duplicate rejection; every decode pack |
| unknown response scalar/subtree overflow | legacy skips field, succeeds | continue ignoring unmatched fields; every applicable decode pack |
| JSON strings `1e6`, `1000000.0`, `9007199254740993`; null, bool | unchanged string text / `<nil>` / `true` | unchanged; T01b/all route packs/T13 |

No behavioral ambiguity remains under the supplied approved decision. If implementation cannot satisfy resource/error compatibility without changing scope/prerequisites, stop and report; do not reinterpret eligibility.

## Section 5: Completion and planning verification

Code execution (future orchestrator/code-agent work): all task common full gates, unchanged `internal/provider`, `internal/mcpclient`, `pkg/runner` suites, `cmd` suite with temporary XDG cache, full module build/vet/configured lint, API check, and R9/R10 scope checks. `make check` runs lint/test/apidiff; the full race completion remains Factory-owned. R9 scope command: `git diff --name-only 8a351c8 -- internal/testutil pkg/runner/options.go pkg/runner/result.go` expects no paths. R10: `git diff --name-only 8a351c8 -- . ':(exclude)docs/plans/'` expects only approved source/test paths. No public identifiers/signatures changed.

After all implementation, commit verified code first (no attribution trailers); obtain a HEAD-matched `factory-gate` completion log including full `go test -race ./...`. Orchestrator runs `docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec` and `records-stale-check` before review. Measure full base..HEAD scope: more than 5 commits OR more than 15 files requires parallel backend/UI/spec-records review with every changed file assigned once (empty subsets omitted), per Factory threshold. Review follows implementation and passing gates, not partial code. Factory alone renders task completion and canonical milestone checkbox; this checklist does not authorize records/checkpoint/future-milestone writes by a code task.

Changed-file formatting check (after formatting only those files):

```sh
{ git diff --name-only --diff-filter=ACMR 8a351c8 --; git ls-files --others --exclude-standard; } | grep '\.go$' | sort -u | xargs -r gofmt -l
```

Expected: no output. The same mechanical clarification is in the approved spec acceptance block at `docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md:204-207`; only formatting scope and deterministic repetition flags changed.

Formatting baseline (measured earlier by code agent, not re-run here): preexisting drift in `internal/provider/anthropic_stream_types.go`, `awscreds.go`, `bedrock.go`, `opencode.go`, `openrouter.go`, `provider_test.go`, `sse_test.go`, `stream.go`, `pkg/runner/result.go`, `result_test.go`. Format full files actually touched by this implementation (including bedrock/opencode/openrouter), but never unrelated files simply to clean baseline. Diff-scoped formatting replaces the approved spec’s broad formatting command; removing deterministic `-count=1` changes only verification mechanics.

This document-only call runs only `git diff --check` and unchecked/unique task-ID inspection. No tests/lints/build/typecheck/network/GitHub/commits or checkpoint writes run here. Orchestrator runs spec-lint on both artifacts and docs-lint through run_script. The existing checkpoint/spec/milestones working-tree edits are preexisting and preserved; only this new checklist and the narrow spec acceptance-command clarification are this call’s repository writes. No current-HEAD code gate is claimed by this plan; historical preflight/probe evidence is not a completion result.

## User-approved toolchain prerequisite exception — 2026-11-11

The user approved a separate prerequisite patch from Go toolchain 1.26.10 to 1.26.11, committed as `70e31fd56c48216a027479a49ebf2cbebe25768c`, followed by pre-flight and compatibility checks before T04. This is outside milestone 010 behavior, not a new task or a waived check. Only that toolchain-line change is excepted from R10; all other scope restrictions and requirements remain in force, and the original commands above remain historical. No push, deploy, or other one-way door is authorized.

The checkpoint's failed pre-flight found GO-2026-6859 in Go 1.26.10. The later `/tmp/052-toolchain-preflight.log` records exit 0 for Go 1.26.11 and govulncheck with no vulnerabilities; `/tmp/052-toolchain-checks.log` records exit 0 for module load, build, vet, lint and apidiff checks. These ran with the pin change uncommitted at parent HEAD `33c3d1928d10b86bc29754b415d4622ede8268e1`, not at a later HEAD; apidiff printed toolchain-selection notices. This passing pin preflight removes the technical blocker but does not satisfy T04, a HEAD-matched review, or completion. It does not replace any later gate.

For the effective R10 scope check before T04 and at completion, run all of the following from the repository root in Bash. The first three checks must exit 0 with no output; the last must exit 0 and list only R10-approved source/test paths. Stop and report any failure or unexpected path. The first check confirms the approved commit is in HEAD's ancestry; the second verifies the entire commit is precisely the one-line toolchain patch, with no other file change; the third requires both module files to match the review base before that patch and the approved commit in the current tree. The last retains the original `8a351c8` base and `docs/plans/` exclusion, excluding only `go.mod` additionally. R9 and every other original restriction and gate remain unchanged.

```bash
git merge-base --is-ancestor 70e31fd56c48216a027479a49ebf2cbebe25768c HEAD

set -o pipefail
git diff --no-ext-diff --no-color --no-renames --no-textconv --full-index --unified=0 70e31fd56c48216a027479a49ebf2cbebe25768c^ 70e31fd56c48216a027479a49ebf2cbebe25768c -- | diff -u <(printf '%s\n' \
  'diff --git a/go.mod b/go.mod' \
  'index 580f71c94b5e83e19844adf193f36bed1329fdd4..0036e19cb2d5df871a96d9d332cd272b19526c3a 100644' \
  '--- a/go.mod' \
  '+++ b/go.mod' \
  '@@ -5 +5 @@ go 1.26.0' \
  '-toolchain go1.26.10' \
  '+toolchain go1.26.11') -

git diff --exit-code 8a351c8 70e31fd56c48216a027479a49ebf2cbebe25768c^ -- go.mod go.sum
git diff --exit-code 70e31fd56c48216a027479a49ebf2cbebe25768c -- go.mod go.sum

git diff --name-only 8a351c8 -- . ':(exclude)docs/plans/' ':(exclude)go.mod'
```

The two module-file comparisons must both exit 0 with no output. Together with the exact patch check, they preserve the `go 1.26.0` language directive, dependencies, and `go.sum`, and forbid any additional module change. The approved pin does not change T04 or any later task behavior.
