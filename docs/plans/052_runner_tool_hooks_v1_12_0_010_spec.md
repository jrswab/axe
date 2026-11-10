# Milestone 010: Send integer MCP arguments of 1e6 and above as JSON integers

## Section 1: Context & Constraints

### Source

- Milestone: `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md`, entry 010 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:252-263`). Scope and acceptance tests 1-6 are taken from that entry, subject to the unresolved decimal/exponent scope decision in Blockers. No literal-only scope choice is approved.
- Design source: `docs/plans/runner-tool-hooks-and-options.md` §A3 (`docs/plans/runner-tool-hooks-and-options.md:269-321`) is rejected for v1.12.0 (see Rejected approaches). The defect itself is milestone decision D12 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:74`) and §1.2 (`docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:45`).
- Predecessors: none technically (`Depends on: none`, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:254`). Milestones 004-009 are complete at the base and are the precedent for test layout and CHANGELOG style. This spec does not depend on their code.
- Baseline: branch `runner-tool-hooks`, milestone review base `8a351c8e475ab1bfdc52000f0593f1e6ed7385ec`. Clean tree at spec time.
- Lane: full. The fix changes the value an external MCP server receives (a JSON string becomes a JSON integer) and the replay text sent to the model, across about 17 decode paths in 8 files, and the test harness has gaps. The fast-lane conditions "external contracts untouched" and "small bounded behavior" both fail.

### Citation corrections (verified at `8a351c8`)

The milestone entry and the design note cite lines that no longer match. This spec uses the verified lines. Neither source document is edited by this spec.

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
- Tool arguments in the response structs are typed `map[string]interface{}`: `anthropic.go:110`, `bedrock.go:92`, `gemini.go:83`, `ollama.go:99`, `opencode.go:123`. No non-test source type-asserts `float64` (verified: `rg '\.\(float64\)' internal/provider pkg/runner cmd --glob '!*_test.go'` has no matches). `UseNumber` also removes the old float64 magnitude-overflow rejection unless range validation is retained recursively, including nested numbers. R2 and R3 preserve that rejection/fallback; no `+Inf` may enter `Arguments`.

### Mechanically settled constraints (scope choice remains blocked)

- **OS1. Shared top-level formatting boundary.** One shared formatter serves all 12 flatten sites (11 provider sites and `pkg/runner/run.go:938`). It lives in `internal/provider`, reachable from `pkg/runner`. Plain integer digit literals must retain exact digits. Whether mathematically integral decimal/exponent forms are also normalized is pending in B2, not settled here. Nested collections retain their complete existing float64-derived formatting.
- **OS2. Below one million is unchanged by construction.** For an integer literal below one million, `%v` of the `float64` and the literal text are the same string (table above). So the formatter changes output only at 1e6 and above. Acceptance tests still assert the unchanged cases.
- **OS3. Decode with `UseNumber`, keep error semantics.** Each buffered decode that feeds a flatten uses `json.Decoder` with `UseNumber`. The decode must keep today's error behavior for malformed input and for trailing data after the first JSON value (`json.Unmarshal` rejects trailing data; a bare `Decoder.Decode` does not, so the implementation adds the check).
- **OS4. Stream emitters carry raw values.** `gemini.go` and `ollama.go` stream paths emit the raw decoded argument values (not the flattened strings) as the `ToolInput` JSON text. `pkg/runner/run.go` drain decodes that text with `UseNumber` (OS3).
- **OS5. No type inference on replay.** `Arguments` stays `map[string]string`. Replay keeps sending values as strings, the existing shape, and only the digit text changes (`1e+06` becomes `1000000`). The original JSON type (number or string) is not recoverable from `map[string]string`, so replay does not guess.
- **OS6. `coerceArg` is verified, not modified.** It already parses plain digits as `int64`. It changes only if a test proves a wire failure (R5).
- **OS7. No public API change.** `Options`, `ToolCall`, `Arguments map[string]string`, and every exported identifier are unchanged. No `RawArguments` field is added (see Rejected approaches). `pkg/runner` results keep `Arguments` text for values below one million.
- **OS8. Test harness: no production change.** `internal/testutil` is not edited. `MockToolCall.Input` stays `map[string]string` (`internal/testutil/mockserver.go:44`). Tests that need JSON numbers use raw-body `testutil.MockLLMResponse{StatusCode: 200, Body: ...}` values built in the test file (the response struct is `internal/testutil/mockserver.go:15-19`). The mock MCP server is the raw `AddTool` form (OS, above), which asserts the bytes it receives.
- **OS9. CHANGELOG and docs untouched.** Milestone 010 edits no `CHANGELOG.md`, `README.md`, or version file. Milestone 012's `Fixed` entry must disclose corrected digits on MCP wire and replay, `tool_call_details[].input`, `messages[].tool_calls[].arguments`, `ToolHook` arguments, and `ToolObserver` call arguments. Field names/types remain unchanged.

### Rejected approaches (settled)

- **Typed MCP arguments, A3 (`RawArguments json.RawMessage` on `ToolCall`).** Rejected for v1.12.0 (milestone §1.4, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:86`; decision D12, `:74`). The fix needs no new field because the digit text is preserved inside `Arguments`. The design note's A3 proposal (`docs/plans/runner-tool-hooks-and-options.md:269-321`) is not implemented. Verification: no new exported identifier (R9).
- **Infer JSON number type for replay.** Rejected (OS5). It needs type information `map[string]string` cannot carry, and a string-typed digit argument would be replayed as a number.
- **Modify `coerceArg` to accept exponent text.** Rejected. Exponent text is what is being removed at the source; accepting it would mask a regression (OS6).
- **A `testutil` helper that emits JSON numbers.** Rejected (OS8). Raw-body responses cover the need without a production-package change (milestone decision D14 keeps `internal/testutil` out of the nightshift API, `docs/plans/052_runner_tool_hooks_v1_12_0_milestones.md:76`).
- **Decode only the tool-argument subtree.** Rejected. Go cannot apply `UseNumber` to one nested field without a custom unmarshaller for every response struct. Whole-body `UseNumber` requires recursive float64 range validation to retain old overflow failures, and recursive conversion of nested numbers back to float64 before collection formatting (R1-R3). The absence of type assertions alone does not establish compatibility.

### Constraints and assumptions

- **Constraint, wire contract.** The MCP server receives JSON numbers for `integer` parameters with digits preserved. The server contract for a `string` parameter is unchanged (it still receives a string).
- **Constraint, compatibility.** The fix is a documented bug fix (`COMPATIBILITY.md:29`, "Default runtime behavior is unchanged except for documented bug fixes"). The value a server receives changes only for integers of 1e6 and above.
- **Constraint, no public API.** The change is internal to `internal/provider`, `pkg/runner/run.go`, and tests (OS7).
- **Assumption, mock MCP server.** The SDK raw handler receives the client's bytes unchanged (go-sdk v1.4.1, raw `AddTool`). Tests assert those bytes, not a decoded value, because decoding to `float64` would hide the digits.
- **Assumption, decoded `int64` range.** `coerceArg` parses with `strconv.ParseInt(value, 10, 64)` (`internal/mcpclient/mcpclient.go:127`). Values beyond `int64` stay strings (edge E5).
- **Assumption, streaming tests.** Existing stream tests exist for `openai`, `anthropic`, `openrouter`, `opencode`, `ollama`, and `gemini` (for example `internal/provider/gemini_test.go:1036`, `internal/provider/ollama_test.go:1004`). Gemini/Ollama emitter checks may extend them, but stored-argument and history stream cases belong in `pkg/runner`, where the unexported drain lives. Bedrock has no stream path.

## Section 2: Requirements

Requirements are grouped as vertical slices. Each slice is independently verifiable. R1-R3 are shared machinery. R4-R7 are the provider and stream slices. R8-R10 are verification and boundaries.

### R1: Shared top-level integer formatter (scope pending B2)

- One formatter in `internal/provider` turns one decoded tool-argument value into the string stored in `Arguments` (OS1).
- Top-level plain integer digit literals retain exact digit text. Decimal/exponent integer-valued inputs await B2; fractional values keep `%v` of their old float64 values. Before formatting any object or array, recursively convert ALL nested `json.Number` values to their old float64 values; the entire collection text must remain exactly unchanged. Other scalar values keep today's formatting. R2/R3 range validation precedes formatting.
- Verification: unit tests in `internal/provider` for `1000000`, `-1000000`, `1234567`, `9007199254740993` returning digits; `999999`, `1.5`, `true`, `"text"`, and `nil` unchanged. B2 determines decimal/exponent expectations. Exact collection examples: `{"a":1e6}` → `map[a:1e+06]`, `[1.50]` → `[1.5]`, `{"a":1000000}` → `map[a:1e+06]`.

### R2: Buffered decode uses `UseNumber`

- Each buffered decode that feeds a flatten uses a `UseNumber` decoder: `anthropic.go:305`, `bedrock.go:348`, `gemini.go:322`, `ollama.go:290`, `openai.go:314`, `openrouter.go:209`, `opencode.go:286`, `opencode.go:1015`, `opencode.go:1184`. The argument-string decodes `openai.go:340`, `openrouter.go:235`, `opencode.go:1040`, and `opencode.go:1206` also use `UseNumber`.
- Malformed input, trailing data after the first JSON value, and float64 magnitude overflow retain the same error category or fallback (not identical wording). Validate ALL decoded numbers recursively, including nested values, against the old float64 range. `1e400` and a 400-digit integer remain rejected; no `+Inf` enters `Arguments`. Whole response failures keep `ProviderError` / server category; OpenCode separately decoded argument failures keep that category (`opencode.go:1040-1041`, `opencode.go:1206-1207`). OpenAI/OpenRouter separately decoded argument maps retain empty-args fallback (`openai.go:340-343`, `openrouter.go:235-238`).
- Verification: preserve existing error tests (including `TestOpenAI_Send_InvalidToolCallArguments`, `internal/provider/openai_test.go:926`). Add trailing-response and trailing-argument-string cases (OpenAI/OpenRouter empty args, OpenCode server `ProviderError`), and top-level/nested `1e400` and 400-digit integer cases for every affected decode boundary.

### R3: Stream decode uses `UseNumber`

- The Gemini stream chunk decode (`gemini.go:500`), the Ollama stream chunk decode (`ollama.go:467`), and the drain raw-argument decode (`pkg/runner/run.go:936`) use `UseNumber` (OS3, OS4).
- Verification: R7/R8 stream tests, plus trailing-data and top-level/nested overflow cases (`1e400`, 400-digit integer). Gemini/Ollama chunk decode failures retain server `ProviderError` (`gemini.go:500-503`, `ollama.go:467-470`); drain argument failures retain empty args (`pkg/runner/run.go:933-945`). Validate recursively before formatting or emitting; no `+Inf` enters `Arguments`.

### R4: Stream emitters carry raw values

- Gemini stream (`gemini.go:529-539`) and Ollama stream (`ollama.go:478-487`) emit the decoded raw argument values as the `ToolInput` JSON text, not the flattened strings (OS4). Their `ToolInput` text keeps integer digits. Intermediate `ToolInput` types/text change, but drained `Arguments` for unaffected values remain unchanged, including nested collection text.
- Verification: R7 stream cases for Gemini and Ollama.

### R5: MCP wire receives digits as JSON integers

- For an `integer`-typed tool parameter with `Arguments` value `1000000`, `-1000000`, `1234567`, or `9007199254740993`, the MCP server receives the same digits as a JSON number. `coerceArg` is unchanged unless a test shows otherwise (OS6).
- Verification: test in `internal/mcpclient` using the raw SDK `AddTool` handler, which asserts the received bytes contain `1000000` and no quotes around it. Pattern: `TestCallTool_CoercesArgTypes` (`internal/mcpclient/mcpclient_test.go:388`). Also require an end-to-end `pkg/runner` test: raw numeric provider response → real provider decode → runner dispatch → raw `AddTool` handler bytes, following `pkg/runner/hook_mcp_test.go:17-48`. Hand-built argument strings alone already pass today and cannot demonstrate the fix.

### R6: Replay carries the digits

- Replay (`openai.go:165-177`, `anthropic.go:160-165`, and the other sites in Verified defect) sends the stored digits. No replay code changes (OS5).
- Verification: first decode a raw numeric provider response through the real provider, then send its returned tool call in the follow-up; do not hand-build `Arguments`. The follow-up request body contains `1000000` and does not contain `1e+06`. For OpenAI the value is in the `arguments` JSON string. For Anthropic it is in the `input` object as a string value.

### R7: Provider decode paths keep digits (buffered and stream)

- Every flatten site in Verified defect and `pkg/runner/run.go:938` keeps the digit text of an integer of 1e6 and above, for buffered and (where a stream path exists) streaming responses (milestone acceptance 6).
- Covered decode paths: buffered `anthropic`, `bedrock`, `gemini`, `ollama`, `openai`, `openrouter`, `opencode` (Claude, Responses, and Chat routes); stream `anthropic`, `gemini`, `ollama`, `openai`, `openrouter`, `opencode` (Claude, Responses, and Chat routes), and `run.go` drain. `atlascloud` and `minimax` are covered through `openai` and `anthropic`.
- Verification: one new test per listed path, named with the `TestLargeIntegerArgs_` prefix, using raw numeric mock bodies/chunks and real provider decodes for `1000000` and `9007199254740993`. Buffered provider tests assert stored `Arguments`. Stream-path tests live in `pkg/runner` and use real providers' `SendStream` through `drainEventStream` or `Run` with `stream=true`, asserting drained `Arguments`, not just text deltas. Provider `ToolInput` checks remain useful only for Gemini/Ollama emitter regressions; text-delta checks alone are not failing regression tests today.
- Returned histories are explicitly required for every buffered and available streaming decode path: use `Run` with non-empty pre-built `Options.Messages` and assert `result.Messages[].ToolCalls[].Arguments` contains both exact integers. History population is conditional at `pkg/runner/run.go:816-817`; use `pkg/runner/run_test.go:579` (`TestRun_PreBuiltMessages_Streaming`) and `pkg/runner/run_test.go:969` (`TestRun_PreBuiltMessages_ToolLoop`) as setup/history assertion patterns, replacing string-only mock tool input with raw numeric response bodies.

### R8: Below one million and non-integer values are unchanged

- Integers below one million, booleans, strings, `null`, `number`-typed values, and nested object or array values keep today's text (OS2, edges E1-E4). Nested values keep exactly the existing Go-syntax text, including float64-derived number formatting (edge E8).
- Verification: existing provider and runner tests pass unchanged (`go test` on `./internal/provider`, `./internal/mcpclient`, `./pkg/runner`), plus the R1 table rows.

### R9: No public API or harness change

- `Arguments map[string]string` is unchanged. No exported identifier is added, removed, or changed in `pkg/runner`. `make apidiff` exits 0 with no output (`Makefile:14-24`, `COMPATIBILITY.md:39-41`).
- `internal/testutil` is unchanged (OS8). Verification: `git diff --name-only 8a351c8 -- internal/testutil pkg/runner/options.go pkg/runner/result.go` prints no paths.

### R10: Scope boundary

- Only the files in the Parallelizable work list are edited: `internal/provider` (7 provider source files, `anthropic.go`, `bedrock.go`, `gemini.go`, `ollama.go`, `openai.go`, `opencode.go`, `openrouter.go`, plus the new formatter file), `pkg/runner/run.go`, `internal/mcpclient` tests (only if R5 needs them), and test files. No other file is edited. `CHANGELOG.md`, `README.md`, `AGENTS.md`, and version files are not edited (OS9).
- Verification: `git diff --name-only 8a351c8 -- . ':(exclude)docs/plans/'` lists only those paths.

### Acceptance tests

Tests use the `TestLargeIntegerArgs_` prefix unless an existing name is given. Each test asserts stored `Arguments` text or raw wire bytes. No test decodes the wire body to `float64`.

1. Formatter table (R1): `TestLargeIntegerArgs_Formatter`.
2. Buffered digit survival (R7): one case per buffered provider path listed in R7, with a raw-body response containing `"n": 1000000` and `"n": 9007199254740993`.
3. Stream digit survival (R7): one case per stream path listed in R7, with raw SSE or NDJSON chunks containing the same two integers.
4. MCP wire (R5): raw SDK handler receives `1000000`, `-1000000`, `1234567`, and `9007199254740993` as unquoted JSON numbers.
5. Replay (R6): decode raw numeric provider responses before follow-up; follow-up bodies contain `1000000`, not `1e+06`, for OpenAI and Anthropic.
6. Unchanged cases (R8): existing tests in `./internal/provider`, `./internal/mcpclient`, and `./pkg/runner` pass unchanged; new cases assert `999999`, `1.5`, `true`, `"text"`, and `nil`.
7. Trailing data (R2/R3): response bodies, separately decoded argument strings, Gemini/Ollama chunks, and runner drain retain their existing error category or empty-args fallback.
8. `Arguments` below one million in a `pkg/runner` result is unchanged (milestone acceptance 4).
9. Returned histories (R7): every buffered and available stream path retains `1000000` and `9007199254740993` in `result.Messages[].ToolCalls[].Arguments`, using real providers and raw numeric mock bodies/chunks; stream assertions run in `pkg/runner`.
10. Overflow (R2/R3): `1e400` and a 400-digit integer, top-level and nested, retain server `ProviderError` or empty-args fallback at each affected boundary. No `+Inf` is stored.
11. Nested formatting (R1/R8): assert the three exact collection examples in R1 unchanged.
12. MCP integration (R5): raw numeric provider response through `pkg/runner` reaches raw `AddTool` handler bytes with exact unquoted digits.

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
- **B2 / F1 pending Jaron's decision.** Include mathematically integral decimal/exponent inputs (`1e6`, `1000000.0`), normalized exactly to `1000000`, or fix only plain digit literals and leave those forms unchanged? Recommendation: inclusion with bounded exact normalization and unchanged treatment of fractional values. Neither alternative is approved; the literal-only proposal is draft only. No normalization algorithm is specified. Milestone 010 remains blocked; no checklist or implementation starts.

## Acceptance commands

Run in the repository root after the change. Each command lists the expected result. All `go test` commands use the empty API-key prefix, as the host has a real `ANTHROPIC_API_KEY`.

```sh
go build ./...                                                                 # expect: exit 0
go vet ./internal/provider/ ./internal/mcpclient/ ./pkg/runner/                 # expect: exit 0
gofmt -l internal/provider/ pkg/runner/ internal/mcpclient/                     # expect: no output
ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -count=1 ./internal/provider ./internal/mcpclient ./pkg/runner   # expect: exit 0
ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -race -count=1 ./internal/mcpclient ./pkg/runner   # expect: exit 0; existing tests TestCoerceArg, TestCallTool_CoercesArgTypes, TestDrainEventStream_ToolCalls included
XDG_CACHE_HOME=$(mktemp -d) ANTHROPIC_API_KEY= AXE_ANTHROPIC_BASE_URL= go test -count=1 ./cmd   # expect: exit 0 (see Limitations)
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

- Decimal/exponent integer-valued input scope is pending B2. The draft literal-only alternative would leave `1000000.0` and `1e6` as `1e+06` strings; this limitation is not approved.
- Number-typed parameters still use `float64` (`coerceArg` `ParseFloat`), so a `number` argument of `9007199254740993` still loses precision to `9007199254740992`. This is unchanged from today (edge E6).
- Replay still sends argument values as strings. The model's original JSON type is not recoverable from `Arguments` (OS5).
- Bedrock has no stream path (edge E7). Its digits are covered by the buffered test only.
- `cmd/` tests may need a writable `XDG_CACHE_HOME` in this sandbox. Run them with `XDG_CACHE_HOME=$(mktemp -d)`, as recorded for 005-009.
- Milestone entry 010 and the design note cite stale lines. The corrected lines are in Citation corrections. Neither document is edited by this spec.

## Edge cases

- **E1.** `999999` stays `999999`; `1000000` becomes `1000000` (was `1e+06`); `-1000000` becomes `-1000000`; `1234567` becomes `1234567`; `9007199254740993` becomes `9007199254740993` (table, Verified defect).
- **E2.** Top-level `1e6`: B2 pending. Draft alternatives are exact `1000000` normalization or unchanged `1e+06` string fallback. Neither is approved.
- **E3.** Top-level `1000000.0`: same pending alternatives as E2.
- **E4.** Literal `1.5`: stored text stays `1.5` (unchanged).
- **E5.** Integer literal beyond `int64` (for example `99999999999999999999999`) for an `integer` parameter: `ParseInt` fails, and the server receives the digit string. Today it receives `1e+23`. The digits change; the type stays string.
- **E6.** Number-typed parameter with `9007199254740993`: `ParseFloat` returns `9007199254740992`. Unchanged from today, because `%v` of `float64` gave the same value.
- **E7.** Bedrock: buffered only. No streaming case.
- **E8.** Nested objects/arrays are string fallbacks (`internal/mcpclient/mcpclient.go:124-140`, `internal/mcpclient/mcpclient.go:172-189`), entirely unchanged: `{"a":1e6}` → `map[a:1e+06]`, `[1.50]` → `[1.5]`, `{"a":1000000}` → `map[a:1e+06]`. Recursively restore all nested numbers to old float64 values before formatting.
- **E9.** `null` argument: stored text `<nil>`, unchanged.
- **E10.** Boolean argument: `true`/`false`, unchanged.
- **E11.** An integer literal with leading zeros is not valid JSON. The decoder rejects it before the formatter runs, as today.
