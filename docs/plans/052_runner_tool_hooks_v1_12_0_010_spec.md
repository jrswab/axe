# Milestone 010: Send integer MCP arguments of 1e6 and above as JSON integers

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 010 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:252-263`). Scope and acceptance tests 1-6 are taken from that entry, reconciled with Jaron's 2026-11-10 approval "q1 = yes": include mathematically integral decimal/exponent JSON numbers for absolute values >=1000000, subject now to the dated superseding "Q1 - ye" eligibility decision below. Only eligible plain integer literals retain original digit text; no literal-only restriction applies.
- Design source: `docs/plans/runner-tool-hooks-and-options.md` §A3 (`docs/plans/runner-tool-hooks-and-options.md:269-321`) is rejected for v1.12.0 (see Rejected approaches). The defect itself is milestone decision D12 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:74`) and §1.2 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:45`).
- Predecessors: none technically (`Depends on: none`, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:254`). Milestones 004-009 are complete at the base and are the precedent for test layout and CHANGELOG style. This spec does not depend on their code.
- Baseline: branch `runner-tool-hooks`, milestone review base `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`. Clean tree at spec time.
- Lane: full. The fix changes the value an external MCP server receives (a JSON string becomes a JSON integer) and the replay text sent to the model, across about 17 decode paths in 8 files, and the test harness has gaps. The fast-lane conditions "external contracts untouched" and "small bounded behavior" both fail.

### Citation corrections (verified at `8a351c8`)

The milestone entry and the design note cite lines that no longer match. This spec uses the verified lines. The canonical milestone entry is reconciled in this planning pass; the rejected design note remains untouched.

- The entry's stream flatten site `pkg/runner/run.go:852` is stale. `drainEventStream` is at `pkg/runner/run.go:884`. The flatten is at `pkg/runner/run.go:938`, fed by the raw-argument parse at `pkg/runner/run.go:933-936`. `pkg/runner/run.go:852` is now `if cfg.Memory.Enabled {`.
- The replay citation `internal/provider/openai.go:168-177` is correct for the marshal (`:168-171`, `:177`), but the replay block starts at `internal/provider/openai.go:165` (`for _, tc := range msg.ToolCalls {`). The entry's `anthropic.go:160-165` is verified (`internal/provider/anthropic.go:160-165`).
- The eleven provider flatten sites in the entry resolve exactly: `internal/provider/anthropic.go:331`, `bedrock.go:280`, `gemini.go:351`, `gemini.go:530`, `ollama.go:303`, `ollama.go:479`, `openai.go:342`, `opencode.go:308`, `opencode.go:1045`, `opencode.go:1211`, `openrouter.go:237`. The design note's `openai.go:338-344` covers the same loop (the loop is `openai.go:339-343`).
- The design note's `pkg/runner/run.go:852` (A3 streaming) is the same stale citation as the entry and is corrected above.
- The entry lists neither the stream emitter re-marshal at `internal/provider/gemini.go:534-539` nor the one at `internal/provider/ollama.go:487`. Both feed the flattened strings into the stream `ToolInput` text and are in scope (R4).
- CHANGELOG reading for milestone 012: the `### Fixed` entry for this change belongs to milestone 012 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:276`, "Fixed: large-integer MCP argument coercion (010)"). Milestone 010 does not edit `CHANGELOG.md`. Verified: the Unreleased `### Added` section at `CHANGELOG.md:10-18` is the only Unreleased section at the base.

### Verified defect and mechanism (at `8a351c8`)

- Every provider decodes tool-call arguments with `encoding/json` into `float64` (`map[string]interface{}`), then flattens each value with `fmt.Sprintf("%v", v)`. Go's `%v` on a `float64` switches to exponent form at one million, so `1000000` becomes `1e+06`. The 11 sites are listed in the Citation corrections.
- The MCP client then sees `"1e+06"`. `coerceArg` (`internal/mcpclient/mcpclient.go:124-140`) calls `strconv.ParseInt`, which fails on exponent text, so the server receives a string. Integers below one million are unaffected.
- Research probe, run in this spec pass with Go 1.26.10 (`GOTOOLCHAIN=local`, scratch module outside the repo). Decoding with `json.Decoder.UseNumber` keeps the literal:

| JSON literal | `%v` of `float64` (today) | `%v` of `json.Number` (literal) |
|---|---|---|
| `999999` | `999999` | `999999` |
| `1000000` | `1e+06` | `1000000` |
| `-1000000` | `-1e+06` | `-1000000` |
| `1234567` | `1.234567e+06` | `1234567` |
| `9007199254740993` | `9.007199254740992e+15` | `9007199254740993` |
| `1.5` | `1.5` | `1.5` |
| `1e6` | `1e+06` | `1e6` |
| `1000000.0` | `1e+06` | `1000000.0` |

- `coerceArg` already parses plain digits with `ParseInt` (`internal/mcpclient/mcpclient.go:127`) into an `int64` return value. Existing tests cover `"3"` and `"-7"` (`internal/mcpclient/mcpclient_test.go:329-330`) and `"notanint"` (`:337`).
- Replay: `openai.go:165-177`, `anthropic.go:160-165`, `gemini.go:183-186`, `ollama.go:138-141`, `bedrock.go:213-216`, and `opencode.go:164-166` copy `Arguments` (`map[string]string`) into the request. They send each value as a string. `opencode.go:946` marshals the argument map (`json.Marshal(tc.Arguments)`), and so does `opencode.go:1114`. Because replay reads the stored `Arguments`, replay text changes with the flatten fix and needs no replay code change (R6).
- Streaming has two shapes. Text-delta providers (`openai`, `openrouter`, `anthropic`, `opencode`) emit raw JSON text as `ToolInput`, and `pkg/runner/run.go:933-938` parses it. Gemini and Ollama flatten first, then re-marshal: `gemini.go:530` flatten and `gemini.go:534` marshal, `ollama.go:479` flatten and `ollama.go:487` marshal. The re-marshal makes the digits a quoted string before `run.go` sees them, so the stream emitters must change too (R4).
- No provider has a `SendStream` in `internal/provider/bedrock.go`. Bedrock is buffered only (verified by search: no `SendStream` match in `bedrock.go`).
- `atlascloud.go` embeds `*OpenAI` (`internal/provider/atlascloud.go:7`) and `minimax.go` returns `*Anthropic` (`internal/provider/minimax.go:14`). Both inherit the fix and need no separate change.
- The MCP SDK (`github.com/modelcontextprotocol/go-sdk` v1.4.1, pinned at `go.mod:9`) raw `AddTool` handler receives `req.Params.Arguments` unvalidated and unmodified. The typed `AddTool` path validates and re-marshals the arguments through `applySchema`, which would hide the digits. Tests must use the raw form. Cited as external module source, not a repository path.
- Tool arguments in the response structs are typed `map[string]interface{}`: `anthropic.go:110`, `bedrock.go:92`, `gemini.go:83`, `ollama.go:99`, `opencode.go:123`. No non-test source type-asserts `float64` (verified: `rg '\.\(float64\)' internal/provider pkg/runner cmd --glob '!*_test.go'` has no matches). `UseNumber` also removes the old float64 magnitude-overflow rejection unless range validation is retained recursively, including numbers overwritten by duplicate keys that the same legacy destination would convert, but not unmatched response fields it skips. R2 and R3 preserve that rejection/fallback; no `+Inf` may enter `Arguments`.

### Superseding numeric decision — 2026-11-10

Jaron answered verbatim "Q1 - ye" to "May I retain current formatting for exceptional tokens that violate those compatibility conditions?" This supersedes only the unconditional numeric normalization promise in the earlier "q1 = yes" approval. In this spec, **eligible** means an exact top-level mathematical JSON integer with |value| >=1000000 whose exact plain result fits at most 309 digits excluding sign AND `ParseFloat(normalized digits, 64)` succeeds and equals legacy `ParseFloat(original token, 64)`. Otherwise retain legacy formatting/error. The 309-digit cap is an explicit policy, not a property guaranteed by ParseFloat. Eligibility requires bounded exact coefficient/exponent work without expanding huge exponents; no input-token-length rejection limit, schema-dependent formatting, or chosen implementation structure is introduced.

Observed evidence: `/tmp/052-numeric-boundaries/main.go` and the existing checkpoint gate record the extreme-token discrepancy. Token A = `"1" + 100006 zero digits + "e-100000"` is exactly 1000000 but legacy Float64 is 0, so it retains `"0"`. Token B = `"0." + 9999 zero digits + "1e100000"` is exactly 10^90000 but legacy Float64 is 1, so it retains `"1"`. These are probe observations, not new implementation. Earlier approvals and checkpoint verdicts remain historical; planning failed rounds remain 2/3. This document reconciliation is not a passing gate.

### Settled constraints (scope approved 2026-11-10, eligibility superseded above)

- **OS1. Shared top-level formatting boundary.** One shared formatter serves all 12 flatten sites (11 provider sites and `pkg/runner/run.go:938`). It lives in `internal/provider`, reachable from `pkg/runner`. Eligible top-level numeric JSON tokens retain original plain integer digit text or normalize integral decimal/exponent forms to exact plain digits; exceptional tokens retain legacy formatting/error. Integrality and threshold use the exact token value, never Float64-rounded value. Nested collections retain their complete existing float64-derived formatting.
- **OS2. Unaffected values retain legacy behavior.** All numeric values with |value| <1000000, zero including negative zero, mathematically fractional values, and all JSON string inputs keep today's formatting/coercion. Float64 validation preserves existing range/error/fallback behavior but must not determine exact integrality. For example, `1000000.00000000001` is fractional even if Float64 rounds it to `1000000`, so it follows legacy formatting.
- **OS3. Decode with `UseNumber`, keep error semantics.** Each buffered decode that feeds a flatten uses `json.Decoder` with `UseNumber`. The decode must keep today's error behavior for malformed input and for trailing data after the first JSON value (`json.Unmarshal` rejects trailing data; a bare `Decoder.Decode` does not, so the implementation adds the check).
- **OS4. Stream emitters carry raw values.** `gemini.go` and `ollama.go` stream paths emit the raw decoded argument values (not the flattened strings) as the `ToolInput` JSON text. `pkg/runner/run.go` drain decodes that text with `UseNumber` (OS3).
- **OS5. No type inference on replay.** `Arguments` stays `map[string]string`. Replay keeps sending values as strings, the existing shape, and only the digit text changes (`1e+06` becomes `1000000`). The original JSON type (number or string) is not recoverable from `map[string]string`, so replay does not guess.
- **OS6. `coerceArg` is verified, not modified.** It already parses plain digits as `int64`. It is unchanged (R5); integer coercion uses ParseInt, number coercion uses ParseFloat, and failures retain the original string.
- **OS7. No public API change.** `Options`, `ToolCall`, `Arguments map[string]string`, and every `pkg/runner` exported API/type/identifier are unchanged. Helpers may be exported in `internal/provider` for runner reuse (existing import at `pkg/runner/run.go:25`). No `RawArguments` field is added (see Rejected approaches). `pkg/runner` results keep `Arguments` text for values below one million.
- **OS8. Test harness: no production change.** `internal/testutil` is not edited. `MockToolCall.Input` stays `map[string]string` (`internal/testutil/mockserver.go:44`). Tests that need JSON numbers use raw-body `testutil.MockLLMResponse{StatusCode: 200, Body: ...}` values built in the test file (the response struct is `internal/testutil/mockserver.go:15-19`). The mock MCP server is the raw `AddTool` form (OS, above), which asserts the bytes it receives.
- **OS9. CHANGELOG and docs untouched.** Milestone 010 edits no `CHANGELOG.md`, `README.md`, or version file. Milestone 012's `Fixed` entry must disclose corrected digits on MCP wire and replay, `tool_call_details[].input`, `messages[].tool_calls[].arguments`, `ToolHook` arguments, and `ToolObserver` call arguments. Field names/types remain unchanged.

### Rejected approaches (settled)

- **Typed MCP arguments, A3 (`RawArguments json.RawMessage` on `ToolCall`).** Rejected for v1.12.0 (milestone §1.4, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:86`; decision D12, `:74`). The fix needs no new field because the digit text is preserved inside `Arguments`. The design note's A3 proposal (`docs/plans/runner-tool-hooks-and-options.md:269-321`) is not implemented. Verification: no new `pkg/runner` exported identifier (R9).
- **Infer JSON number type for replay.** Rejected (OS5). It needs type information `map[string]string` cannot carry, and a string-typed digit argument would be replayed as a number.
- **Modify `coerceArg` to accept exponent text.** Rejected. Exponent text is what is being removed at the source; accepting it would mask a regression (OS6).
- **A `testutil` helper that emits JSON numbers.** Rejected (OS8). Raw-body responses cover the need without a production-package change (milestone decision D14 keeps `internal/testutil` out of the nightshift API, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:76`).
- **Decode only the tool-argument subtree.** Rejected. Go cannot apply `UseNumber` to one nested field without a custom unmarshaller for every response struct. Whole-body `UseNumber` requires same-destination legacy error preservation (R2/R3), and recursive conversion of nested numbers back to float64 before collection formatting (R1-R3). The absence of type assertions alone does not establish compatibility.

### Constraints and assumptions

- **Constraint, wire contract.** The MCP server receives JSON numbers for `integer` parameters with exact preserved/normalized digits within int64; eligible exact integers beyond int64 remain exact digit strings because `ParseInt` fails; exceptional tokens keep legacy formatting/error. The server contract for a `string` parameter is unchanged (it still receives a string).
- **Constraint, compatibility.** The fix is a documented bug fix (`COMPATIBILITY.md:29`, "Default runtime behavior is unchanged except for documented bug fixes"). The value a server receives changes only for eligible exact top-level numeric integers, including integral decimal/exponent forms; all unaffected boundaries remain unchanged.
- **Constraint, no public API.** The change is internal to `internal/provider`, `pkg/runner/run.go`, and tests (OS7).
- **Assumption, mock MCP server.** The SDK raw handler receives the client's bytes unchanged (go-sdk v1.4.1, raw `AddTool`). Tests assert those bytes, not a decoded value, because decoding to `float64` would hide the digits.
- **Assumption, decoded `int64` range.** `coerceArg` parses with `strconv.ParseInt(value, 10, 64)` (`internal/mcpclient/mcpclient.go:127`). Values beyond `int64` stay strings (edge E5).
- **Assumption, streaming tests.** Existing stream tests exist for `openai`, `anthropic`, `openrouter`, `opencode`, `ollama`, and `gemini` (for example `internal/provider/gemini_test.go:1036`, `internal/provider/ollama_test.go:1004`). Gemini/Ollama raw emitter checks are mandatory and may extend them, but stored-argument and history stream cases belong in `pkg/runner`, where the unexported drain lives. Bedrock has no stream path.

## Section 2: Requirements

Requirements are grouped as vertical slices. Each slice is independently verifiable. R1-R3 are shared machinery. R4-R7 are the provider and stream slices. R8-R10 are verification and boundaries.

### R1: Shared exact top-level integer formatting

- One formatter in `internal/provider` turns one decoded tool-argument value into the string stored in `Arguments` (OS1).
- Eligible exact signed top-level numeric JSON tokens retain original plain integer digit text or normalize decimal/exponent forms to exact plain digits; all other tokens retain legacy formatting/error. `1e6` and `1000000.0` become `1000000`; `-1e6` becomes `-1000000`; `9007199254740993` and eligible equivalent integral spellings retain exact value, never Float64 rounding. Exact integers beyond int64 retain normalized digits only if eligible.
- All values below the absolute threshold, zeros including negative zero, fractional values, and JSON strings keep legacy formatting. Float64 validates range but does not decide integrality or the threshold. `1000000.00000000001` stays on the legacy path despite Float64 rounding. Before formatting any object or array, recursively convert ALL nested `json.Number` values to their old float64 values; the entire collection text must remain exactly unchanged. Other scalar values keep today's formatting. R2/R3 range validation precedes formatting.
- Work and memory are bounded by input length plus an explicit policy cap of 309 result digits excluding sign, not arbitrary exponent magnitude. Bounded exact coefficient/exponent work must establish eligibility without expanding huge exponents, and ParseFloat equality is an acceptance boundary, not an algorithm prescription. No input-token-length rejection limit is added. Huge positive exponents must not cause exponent-sized allocations; overflow retains existing failures/fallbacks. Huge negative exponents retain old underflow/fraction formatting. Zero with huge exponents is safe and preserves old signed-zero behavior. These are observable correctness/resource bounds, not a normalization algorithm.
- Verification: unit tests in `internal/provider` cover the requirement/edge table below, both signs, exact >2^53 spellings, finite beyond-int64 values, huge positive/negative exponents and zero exponents. Exact collection examples: `{"a":1e6}` → `map[a:1e+06]`, `[1.50]` → `[1.5]`, `{"a":1000000}` → `map[a:1e+06]`.

### R2: Buffered decode uses `UseNumber`

- Each buffered decode that feeds a flatten uses a `UseNumber` decoder: `anthropic.go:305`, `bedrock.go:348`, `gemini.go:322`, `ollama.go:290`, `openai.go:314`, `openrouter.go:209`, `opencode.go:286`, `opencode.go:1015`, `opencode.go:1184`. The argument-string decodes `openai.go:340`, `openrouter.go:235`, `opencode.go:1040`, and `opencode.go:1206` also use `UseNumber`.
- Malformed input, trailing data after the first JSON value, and float64 magnitude overflow retain the same error category or fallback (not identical wording). Preserve errors from every value the legacy destination decoder would convert, including values later overwritten. A same-destination legacy validation pass into a fresh instance before UseNumber, or equivalent validation retaining those errors, is explicitly allowed. Same-destination includes legacy map subtrees if future raw fields use RawMessage. Do not reject all duplicate keys or blindly validate unmatched response fields: legacy decoding skips those fields. `1e400` and a 400-digit integer remain rejected; no `+Inf` enters `Arguments`. Whole response failures keep `ProviderError` / server category; OpenCode separately decoded argument failures keep that category (`opencode.go:1040-1041`, `opencode.go:1206-1207`). OpenAI/OpenRouter separately decoded argument maps retain empty-args fallback (`openai.go:340-343`, `openrouter.go:235-238`).
- Verification: preserve existing error tests (including `TestOpenAI_Send_InvalidToolCallArguments`, `internal/provider/openai_test.go:989`). Add trailing-response and trailing-argument-string cases (OpenAI/OpenRouter empty args, OpenCode server `ProviderError`), and top-level/nested `1e400` and 400-digit integer cases for every affected decode boundary. At every boundary include duplicate overflow `{"n":1e400,"n":1000000}` and replaced-subtree overflow `{"n":{"bad":1e400},"n":1000000}` in the decoded argument destination, successful duplicates `{"n":999999,"n":1000000}` and `{"n":{"ok":1},"n":1000000}`, and ignored unknown response-field overflow controls where the legacy destination skips that field. Preserve each boundary's stated ProviderError versus empty-args fallback.

### R3: Stream decode uses `UseNumber`

- The Gemini stream chunk decode (`gemini.go:500`), the Ollama stream chunk decode (`ollama.go:467`), and the drain raw-argument decode (`pkg/runner/run.go:936`) use `UseNumber` (OS3, OS4).
- Verification: R7/R8 stream tests, plus trailing-data and top-level/nested overflow cases (`1e400`, 400-digit integer). Gemini/Ollama chunk decode failures retain server `ProviderError` (`gemini.go:500-503`, `ollama.go:467-470`); drain argument failures retain empty args (`pkg/runner/run.go:933-945`). Before formatting or emitting, preserve every error from values the legacy destination decoder would convert, even overwritten values; allow a same-destination legacy pass into a fresh instance before UseNumber or equivalent error-preserving validation, including legacy map subtrees if future raw fields use RawMessage. Require at each R3 boundary duplicate overflow `{"n":1e400,"n":1000000}`, replaced-subtree overflow `{"n":{"bad":1e400},"n":1000000}`, successful duplicates `{"n":999999,"n":1000000}` and `{"n":{"ok":1},"n":1000000}`, plus ignored unknown response-field overflow controls wherever legacy decoding skips them. Do not reject all duplicates or blindly validate unmatched fields. Preserve the stated server ProviderError versus drain empty-args fallback; no `+Inf` enters `Arguments`.

### R4: Stream emitters carry raw values

- Gemini stream (`gemini.go:529-539`) and Ollama stream (`ollama.go:478-487`) emit the decoded raw argument values as the `ToolInput` JSON text, not the flattened strings (OS4). Their `ToolInput` text keeps integer digits. Intermediate `ToolInput` types/text change, but drained `Arguments` for unaffected values remain unchanged, including nested collection text.
- Verification: mandatory provider-level Gemini/Ollama tests assert unquoted numeric ToolInput tokens for `1000000` and `9007199254740993`, and native boolean, null, object and array shapes (not flattened strings). These supplement, not replace, R7 real-provider runner drain/history stream tests.

### R5: MCP wire receives digits as JSON integers

- For an `integer`-typed tool parameter with `Arguments` value `1000000`, `-1000000`, `1234567`, or `9007199254740993`, the MCP server receives the same digits as a JSON number. `coerceArg` is unchanged (OS6).
- Verification: test in `internal/mcpclient` using the raw SDK `AddTool` handler, which asserts the received bytes contain `1000000` and no quotes around it. Pattern: `TestCallTool_CoercesArgTypes` (`internal/mcpclient/mcpclient_test.go:388`). Also require an end-to-end `pkg/runner` test: raw numeric provider response → real provider decode → runner dispatch → raw `AddTool` handler bytes, following `pkg/runner/hook_mcp_test.go:17-48`. Hand-built argument strings alone already pass today and cannot demonstrate the fix.

### R6: Replay carries the digits

- Replay (`openai.go:165-177`, `anthropic.go:160-165`, and the other sites in Verified defect) sends the stored digits. No replay code changes (OS5).
- Verification: first decode a raw numeric provider response through the real provider, then send its returned tool call in the follow-up; do not hand-build `Arguments`. The follow-up request body contains `1000000` and does not contain `1e+06`. For OpenAI the value is in the `arguments` JSON string. For Anthropic it is in the `input` object as a string value.

### R7: Provider decode paths keep digits (buffered and stream)

- Every flatten site in Verified defect and `pkg/runner/run.go:938` keeps original plain digit text or exact normalized digits of an eligible top-level numeric integer, for buffered and (where a stream path exists) streaming responses (milestone acceptance 6).
- Covered decode paths: buffered `anthropic`, `bedrock`, `gemini`, `ollama`, `openai`, `openrouter`, `opencode` (Claude, Responses, and Chat routes); stream `anthropic`, `gemini`, `ollama`, `openai`, `openrouter`, `opencode` (Claude, Responses, and Chat routes), and `run.go` drain. `atlascloud` and `minimax` are covered through `openai` and `anthropic`.
- Verification: one new test per listed path, named with the `TestLargeIntegerArgs_` prefix, using raw numeric mock bodies/chunks and real provider decodes for `1000000` and `9007199254740993`. Buffered provider tests assert stored `Arguments`. Stream-path tests live in `pkg/runner` and use real providers' `SendStream` through `drainEventStream` or `Run` with `stream=true`, asserting drained `Arguments`, not just text deltas. Mandatory provider-level Gemini/Ollama ToolInput tests assert unquoted numeric tokens including `9007199254740993` and native boolean/null/collection shapes; text-delta checks alone are not failing regression tests today.
- Returned histories are explicitly required for every buffered and available streaming decode path: use the real-provider runner entry with non-empty pre-built `Options.Messages` (`Run` for other paths; Bedrock may use existing private `run(..., newProvider)` at `pkg/runner/run.go:48-49,65,413` to inject real `provider.NewBedrock` with `WithBedrockBaseURL(httptestURL)` at `internal/provider/bedrock.go:22-24,41-55` and isolated fake credentials, never a capturing-provider substitute) and assert `result.Messages[].ToolCalls[].Arguments` contains both exact integers. History population is conditional at `pkg/runner/run.go:816-817`; use `pkg/runner/run_test.go:579` (`TestRun_PreBuiltMessages_Streaming`) and `pkg/runner/run_test.go:969` (`TestRun_PreBuiltMessages_ToolLoop`) as setup/history assertion patterns, replacing string-only mock tool input with raw numeric response bodies.

### R8: Below one million and non-integer values are unchanged

- Numeric values with |value| <1000000, zeros including negative zero, fractional values, booleans, all JSON strings, `null`, and nested object or array values keep today's text (OS2, edges E1-E4). MCP number-typed parameters preserve the legacy resulting Float64 numerical value, not identical `Arguments` text: normalization is allowed only when the exact result fits at most 309 digits excluding sign and ParseFloat succeeds with equality to the legacy original-token parse; exceptional tokens retain legacy formatting/error. Verify signed zero, rounding boundaries, and values >2^53 using raw provider decode through MCP bytes. Nested values keep exactly the existing Go-syntax text, including float64-derived number formatting (edge E8).
- Verification: existing provider and runner tests pass unchanged (`go test` on `./internal/provider`, `./internal/mcpclient`, `./pkg/runner`), plus the R1 table rows.

### R9: No public API or harness change

- `Arguments map[string]string` is unchanged. No exported identifier is added, removed, or changed in `pkg/runner`. `make apidiff` exits 0 with no output (`Makefile:14-24`, `COMPATIBILITY.md:39-41`).
- `internal/testutil` is unchanged (OS8). Verification: `git diff --name-only 8a351c8 -- internal/testutil pkg/runner/options.go pkg/runner/result.go` prints no paths.

### R10: Scope boundary

- Only the following paths may be edited. `internal/provider` (7 provider source files, `anthropic.go`, `bedrock.go`, `gemini.go`, `ollama.go`, `openai.go`, `opencode.go`, `openrouter.go`, plus the new formatter file), `pkg/runner/run.go`, `internal/mcpclient` tests (only if R5 needs them), and test files. No other file is edited. `CHANGELOG.md`, `README.md`, `AGENTS.md`, and version files are not edited (OS9).
- Verification: `git diff --name-only 8a351c8 -- . ':(exclude)docs/plans/'` lists only those paths.

### Requirement/edge table

| Input / boundary | Required observable behavior | Verification |
|---|---|---|
| Numeric `1e6`, `1000000.0`, `-1e6` | `1000000`, `1000000`, `-1000000` | R1, every R7 buffered/stream/history path, R5/R6 |
| Eligible numeric `9007199254740993`, `9007199254740993.0`, `90071992547409930e-1` | Exact `9007199254740993`, not `9007199254740992`; exceptional equivalent spellings retain legacy formatting/error | R1, R5-R7 |
| Eligible exact integer beyond int64, including integral decimal/exponent spelling | Exact normalized digits in Arguments only if eligible; integer MCP parameter remains a digit string | R1/R5, E5 |
| `999999`, `-999999`, `999999.0`, `0`, `-0`, `-0.0` | Legacy Float64 formatting/coercion, including signed zero | R1/R8 |
| `1.5`, `1000000.00000000001` | Legacy fractional path even when Float64 rounds to an integer | R1/R8 |
| JSON strings `"1e6"`, `"1000000.0"`, `"9007199254740993"` | Never normalize; existing schema coercion only | R1/R5/R8 |
| Nested objects/arrays, recursively | Entire old Float64-derived Go-syntax text unchanged | R1/R8, E8 |
| `1e400`, 400-digit plain integer, nested overflow | Existing error category/fallback; no `+Inf` | Every R2/R3 boundary |
| Huge positive/negative exponent; zero with huge exponent; exceptional tokens A/B above | Bounded coefficient/exponent work, no huge expansion or token-length rejection; at most 309 result digits excluding sign and successful equal ParseFloat required; A retains `0`, B retains `1`; old errors/underflow/signed zero | R1-R3 resource/error cases |
| MCP `number` parameters, signed zero, rounding boundaries, >2^53 | Same Float64 numerical value, not necessarily identical Arguments text | Raw decode → MCP bytes, R5/R8 |

### Acceptance tests

Tests use the `TestLargeIntegerArgs_` prefix unless an existing name is given. Each test asserts stored `Arguments` text or raw wire bytes. No test decodes the wire body to `float64`.

1. Formatter table (R1): `TestLargeIntegerArgs_Formatter`.
2. Buffered digit survival (R7): one case per buffered provider path listed in R7, with a raw-body response containing `"n": 1000000` and `"n": 9007199254740993`.
3. Stream digit survival (R7): one case per stream path listed in R7, with raw SSE or NDJSON chunks containing the same two integers. Separately require provider-level Gemini/Ollama ToolInput assertions for unquoted `1000000` and `9007199254740993` tokens and native boolean/null/object/array shapes; retain real-provider runner drain and returned-history tests.
4. MCP wire (R5): raw SDK handler receives `1000000`, `-1000000`, `1234567`, and `9007199254740993` as unquoted JSON numbers.
5. Replay (R6): decode raw numeric provider responses before follow-up; follow-up bodies contain `1000000`, not `1e+06`, for OpenAI and Anthropic.
6. Unchanged cases (R8): existing tests in `./internal/provider`, `./internal/mcpclient`, and `./pkg/runner` pass unchanged; new cases assert `999999`, `1.5`, `true`, `"text"`, and `nil`.
7. Trailing data (R2/R3): response bodies, separately decoded argument strings, Gemini/Ollama chunks, and runner drain retain their existing error category or empty-args fallback.
8. `Arguments` below one million in a `pkg/runner` result is unchanged (milestone acceptance 4).
9. Returned histories (R7): every buffered and available stream path retains `1000000` and `9007199254740993` in `result.Messages[].ToolCalls[].Arguments`, using real providers and raw numeric mock bodies/chunks; stream assertions run in `pkg/runner`.
10. Overflow (R2/R3): `1e400` and a 400-digit integer, top-level and nested, retain server `ProviderError` or empty-args fallback at each affected boundary. Every boundary also covers both duplicate-overflow and successful-duplicate controls specified in R2/R3, including replaced subtrees, and ignored unknown response-field overflow controls where applicable. Preserve overwritten-value errors without rejecting successful duplicates or ignored fields. No `+Inf` is stored.
11. Nested formatting (R1/R8): assert the three exact collection examples in R1 unchanged.
12. MCP integration (R5): raw numeric provider response through `pkg/runner` reaches raw `AddTool` handler bytes with exact unquoted digits; eligible beyond-int64 values instead reach it as exact digit strings; exceptional values keep legacy formatting/error. Include approved integral decimal/exponent spellings and JSON string controls.
13. Approved forms across all paths (R1/R7): `1e6`, `1000000.0`, and `-1e6` normalize in every buffered/provider-route and real `SendStream` → runner drain path, returned histories, real decode → replay, and raw decode → MCP bytes. Assert exact normalized `Arguments` also in hook/observer calls and detail inputs; field names/types stay unchanged.
14. Exactness and bounds (R1-R3/R8): verify every requirement/edge table row, including fractional rounding traps, equivalent >2^53 integral spellings, signed zero, huge exponent safety, eligible beyond-int64 normalization, and exceptional tokens A/B retaining `0`/`1`. Assert the explicit 309-digit cap and successful ParseFloat equality, without input-length rejection. MCP `number` tests assert the resulting Float64 numerical value, including signed zero and rounding boundaries; they do not require identical text.

## Parallelizable work

- The formatter (R1) and its table test are independent of the decode changes.
- Each provider file is independent once the formatter exists (R2, R4, R7). Stream and buffered changes in one file are one unit.
- `pkg/runner/run.go` drain (R3) is independent of the provider files.
- MCP wire verification (R5) is independent; it reads the existing `coerceArg`.

## Serial dependencies

- R1 before every other requirement (the formatter must exist).
- R2 and R3 before R7 stream and buffered tests (the decoders must keep digits).
- R4 before the Gemini and Ollama stream cases in R7.
- R7 before R8 final verification (the unchanged-case run covers the whole change).

## Blockers

- **B1 resolved.** Authoritative HOST `run_script factory-sync` at HEAD `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec` exited 0: base `origin/master`, ahead 136, behind 0, shared_files [], conflicts [], synced true. The checkpoint's `010 setup and spec lint PASS` records it. The sandbox fetch failure remains historical evidence only.
- **B2 / F1 resolved 2026-11-10.** Jaron answered "q1 = yes", approving integral decimal/exponent numeric inputs with exact bounded normalization at |value| >=1000000 (R1). This approval changes milestone 010's local behavior promise only; no normalization algorithm is prescribed. No active scope or sync blocker remains. The first failing round was historically 1 of 3; the superseding decision above leaves phase `spec` and current failed planning rounds at 2/3, and this reconciliation is not a passing review. No checklist or implementation starts in this pass.

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result. All `go test` commands use the empty API-key prefix, as the host has a real `ANTHROPIC_API_KEY`.

```sh
go build ./...                                                                 # expect: exit 0
go vet ./internal/provider/ ./internal/mcpclient/ ./pkg/runner/                 # expect: exit 0
{ git diff --name-only --diff-filter=ACMR 8a351c8 --; git ls-files --others --exclude-standard; } | grep '\.go$' | sort -u | xargs -r gofmt -l   # expect: no output; changed/new Go files only, not unrelated baseline drift
ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race ./internal/provider ./internal/mcpclient ./pkg/runner   # expect: exit 0
ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race ./internal/mcpclient ./pkg/runner   # expect: exit 0; existing tests TestCoerceArg, TestCallTool_CoercesArgTypes, TestDrainEventStream_ToolCalls included
XDG_CACHE_HOME=$(mktemp -d) ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test ./cmd   # expect: exit 0 (see Limitations)
make apidiff                                                                    # expect: exit 0, no output
git diff --name-only 8a351c8 -- . ':(exclude)docs/plans/'                       # expect: only the paths in R10
~/.agents/scripts/spec-lint --root /home/jaron/dev/axe/runner-tool-hooks docs/plans/052_runner_tool_hooks_v1_12_0_010_spec.md   # expect: exit 0
~/.agents/scripts/docs-lint --since 8a351c8e475ab1bfdc52000f0593f1e6ed7385ec /home/jaron/dev/axe/runner-tool-hooks   # expect: exit 0
```

## Parallelizable and failure modes

1. **Digits lost in a buffered path.** A provider keeps `float64` and the flatten still produces `1e+06`. Mitigation: R2, R7 buffered cases.
2. **Digits lost in a stream path.** Gemini or Ollama re-marshal the flattened string, so the digits become quoted text (`"1000000"` inside the JSON). Mitigation: R4, R7 stream cases.
3. **Below-one-million text changes.** The formatter changes non-literal output. Mitigation: OS2, R1 table, R8.
4. **Trailing data accepted.** `Decoder.Decode` without a trailing-data check accepts a body that `json.Unmarshal` rejected. Mitigation: OS3, acceptance test 7.
5. **Test hides digits.** A test decodes the wire body to `float64` and passes on `1e+06`. Mitigation: raw SDK handler and byte assertions (OS8, R5).
6. **Replay type drift.** Replay sends a number where a string was sent before. Mitigation: OS5 (no type inference), R6.

## Limitations

- Eligible exact integral values beyond int64 retain exact normalized digits in `Arguments` only under the approved 309-digit and ParseFloat-equality conditions; exceptional tokens retain legacy formatting/error, but `integer` MCP coercion still returns digit text as a string when `ParseInt` fails (E5).
- Number-typed parameters still use `float64` (`coerceArg` `ParseFloat`), so a `number` argument of `9007199254740993` still loses precision to `9007199254740992`. This is unchanged from today (edge E6).
- Replay still sends argument values as strings. The model's original JSON type is not recoverable from `Arguments` (OS5).
- Bedrock has no stream path (edge E7). Its digits are covered by the buffered test only.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005-009.
- The rejected design note retains its historical stale streaming citation. The corrected anchors are in Citation corrections; canonical entry 010 is updated in this reconciliation.

## Edge cases

- **E1.** `999999` stays `999999`; `1000000` becomes `1000000` (was `1e+06`); `-1000000` becomes `-1000000`; `1234567` becomes `1234567`; `9007199254740993` becomes `9007199254740993` (table, Verified defect).
- **E2.** Top-level numeric `1e6` → `1000000`; `-1e6` → `-1000000`, exact normalized digits (approved).
- **E3.** Top-level numeric `1000000.0` → `1000000`; eligible equivalent integral decimal/exponent spellings of `9007199254740993` → `9007199254740993`, never rounded.
- **E4.** Literal `1.5`: stored text stays `1.5` (unchanged).
- **E5.** Integer literal beyond `int64` (for example `99999999999999999999999`) for an `integer` parameter: `ParseInt` fails, and the server receives the digit string. Today it receives `1e+23`. The digits change; the type stays string.
- **E6.** Number-typed parameter with `9007199254740993`: `ParseFloat` returns `9007199254740992`. Unchanged from today, because `%v` of `float64` gave the same value.
- **E7.** Bedrock: buffered only. No streaming case.
- **E8.** Nested objects/arrays are string fallbacks (`internal/mcpclient/mcpclient.go:124-140`, `internal/mcpclient/mcpclient.go:172-189`), entirely unchanged: `{"a":1e6}` → `map[a:1e+06]`, `[1.50]` → `[1.5]`, `{"a":1000000}` → `map[a:1e+06]`. Recursively restore all nested numbers to old float64 values before formatting.
- **E9.** `null` argument: stored text `<nil>`, unchanged.
- **E10.** Boolean argument: `true`/`false`, unchanged.
- **E12.** Exceptional tokens A/B retain `0`/`1` as observed above; every other normalization requires the explicit at-most-309-digit result excluding sign and successful ParseFloat equality. No input-token-length rejection limit is added; `1e400` and 400-digit literals retain old errors/fallbacks.
- **E11.** An integer literal with leading zeros is not valid JSON. The decoder rejects it before the formatter runs, as today.
