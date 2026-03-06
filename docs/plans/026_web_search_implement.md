# Implementation: `web_search` Built-in Tool (Tavily)

**Spec:** `docs/plans/026_web_search_spec.md`
**GitHub Issue:** https://github.com/jrswab/axe/issues/6
**Created:** 2026-03-05

---

## Phase 1: Tool Name Constant & Validation (no dependencies)

- [x] Add `WebSearch = "web_search"` constant to the `const` block in `internal/toolname/toolname.go` *(Req 1.1)*
- [x] Add `WebSearch: true` entry to the map returned by `ValidNames()` in `internal/toolname/toolname.go` *(Req 1.2)*
- [x] Update `TestValidNames_ReturnsExpectedCount` in `internal/toolname/toolname_test.go`: change expected count from 6 to 7 *(Req 5.1)*
- [x] Update `TestValidNames_ContainsAllExpectedNames` in `internal/toolname/toolname_test.go`: add `WebSearch` to the `expected` slice *(Req 5.2)*
- [x] Update `TestConstants_Values` in `internal/toolname/toolname_test.go`: add table entry `{"WebSearch", WebSearch, "web_search"}` *(Req 5.3)*
- [x] Verify: `make test` passes for `internal/toolname/` package

---

## Phase 2: Web Search Tool — Types & Entry Function (depends on Phase 1)

- [x] Create file `internal/tool/web_search.go` *(Req 2.1)*
- [x] Define unexported request struct with JSON tags for: `query`, `max_results`, `search_depth`, `api_key` *(Req 3.6)*
- [x] Define unexported response struct with `Results` field (slice of result structs) *(Req 3.13)*
- [x] Define unexported result struct with `Title`, `URL`, `Content` fields and JSON tags *(Req 3.13)*
- [x] Define package-level constants: `tavilyDefaultURL`, `tavilyMaxResults`, `tavilyMaxResponseBytes`, `tavilyErrorBodyMax` *(Req 3.5, 3.6, 3.10, 3.11, 3.12)*
- [x] Define unexported function `webSearchEntry() ToolEntry` that returns a `ToolEntry` with non-nil `Definition` and `Execute` *(Req 2.2)*
- [x] `Definition` function returns tool with `Name: toolname.WebSearch` (not hardcoded) *(Req 2.3, 2.4)*
- [x] `Definition` includes description explaining web search via Tavily returning titles, URLs, snippets *(Req 2.3)*
- [x] `Definition` has one parameter: `query` (type `"string"`, required `true`, with description) *(Req 2.3, 2.5)*
- [x] Verify: `go build ./internal/tool/` compiles

---

## Phase 3: Web Search Tool — Executor Logic (depends on Phase 2)

- [x] `Execute` function signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult` *(Req 3.1)*
- [x] Extract `query` from `call.Arguments["query"]` *(Req 3.2)*
- [x] Return error `ToolResult` if `query` is empty string: `"query is required"` *(Req 3.3)*
- [x] Read API key via `os.Getenv("TAVILY_API_KEY")`; return error if empty: `"TAVILY_API_KEY environment variable is not set"` *(Req 3.4)*
- [x] Read base URL via `os.Getenv("AXE_TAVILY_BASE_URL")`; fall back to `tavilyDefaultURL` if empty *(Req 3.5)*
- [x] Marshal request body using `json.Marshal` with the unexported request struct *(Req 3.6)*
- [x] Create HTTP request with `http.NewRequestWithContext(ctx, "POST", baseURL+"/search", bytes.NewReader(body))` *(Req 3.7)*
- [x] Set headers: `Content-Type: application/json`, `Authorization: Bearer <apiKey>` *(Req 3.7)*
- [x] Execute request with `http.DefaultClient.Do(req)` *(Req 3.8)*
- [x] Defer `resp.Body.Close()` immediately after successful `Do()` *(Req 3.9)*
- [x] Read response body with `io.ReadAll(io.LimitReader(resp.Body, tavilyMaxResponseBytes+1))` *(Req 3.10)*
- [x] If read bytes length > `tavilyMaxResponseBytes`, return error: `"Tavily response too large to process"` *(Req 3.11)*
- [x] If status code is non-2xx, return error: `fmt.Sprintf("Tavily API error (HTTP %d): %s", ...)` with body truncated to 500 bytes *(Req 3.12)*
- [x] Unmarshal response body into response struct; return error with `"failed to parse Tavily response"` on failure *(Req 3.13)*
- [x] If results slice is empty/nil, return success with `"No results found."` *(Req 3.15)*
- [x] Format results using `strings.Builder`: `Title: <title>\nURL: <url>\nSnippet: <content>` separated by `\n\n` *(Req 3.14)*
- [x] Return success `ToolResult` with formatted content *(Req 3.16)*
- [x] All error/success `ToolResult`s set `CallID: call.ID` *(Req 3.3–3.16)*
- [x] Add deferred `toolVerboseLog(ec, toolname.WebSearch, result, summary)` call *(Req 3.17)*
- [x] Summary includes query truncated to 80 chars if longer *(Req 3.17)*
- [x] Summary must NOT include API key *(Req 3.17)*
- [x] Verify: `go build ./internal/tool/` compiles

---

## Phase 4: Registration (depends on Phase 3)

- [x] Add `r.Register(toolname.WebSearch, webSearchEntry())` to `RegisterAll` in `internal/tool/registry.go` *(Req 4.1)*
- [x] Verify: no other changes to `registry.go` *(Req 4.2)*

---

## Phase 5: Scaffold Update (no dependency on Phase 2–4)

- [x] Update `Scaffold()` in `internal/agent/agent.go` tools comment to: `# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search` *(Req 6.1)*

---

## Phase 6: Tests — Toolname (depends on Phase 1)

Write tests RED first, then verify GREEN after Phase 1 code is in place.

- [x] `TestValidNames_ReturnsExpectedCount` — asserts count is 7
- [x] `TestValidNames_ContainsAllExpectedNames` — includes `WebSearch`
- [x] `TestConstants_Values` — includes `{"WebSearch", WebSearch, "web_search"}`
- [x] Verify: `make test` passes for `internal/toolname/`

---

## Phase 7: Tests — Registry (depends on Phase 4)

- [x] `TestRegisterAll_RegistersWebSearch` — `r.Has(toolname.WebSearch)` returns true *(Spec 7.4)*
- [x] `TestRegisterAll_ResolvesWebSearch` — resolved tool has `Name == toolname.WebSearch` with `query` parameter *(Spec 7.4)*
- [x] Verify: `make test` passes for `internal/tool/` registry tests

---

## Phase 8: Tests — Web Search Core (depends on Phase 3)

Write each test RED, then verify GREEN. Use `httptest.NewServer`, `t.Setenv`, real `Execute` calls.

### Input Validation
- [x] `TestWebSearch_EmptyQuery` — empty Arguments map → `"query is required"`, `IsError: true` *(Spec 7.2)*
- [x] `TestWebSearch_MissingQueryArgument` — `{"query": ""}` → `"query is required"`, `IsError: true` *(Spec 7.2)*
- [x] `TestWebSearch_MissingAPIKey` — `TAVILY_API_KEY=""` → `"TAVILY_API_KEY environment variable is not set"`, `IsError: true` *(Spec 7.2)*

### Success Path
- [x] `TestWebSearch_Success` — 2 results, verify formatted output, `IsError: false`, `CallID` match *(Spec 7.2)*
- [x] `TestWebSearch_CallIDPassthrough` — `call.ID = "ws-unique-42"` → `ToolResult.CallID == "ws-unique-42"` *(Spec 7.2)*

### Empty/Null Results
- [x] `TestWebSearch_EmptyResults` — `{"results": []}` → `"No results found."`, `IsError: false` *(Spec 7.2)*
- [x] `TestWebSearch_NullResults` — `{"results": null}` → `"No results found."`, `IsError: false` *(Spec 7.2)*

### HTTP Errors
- [x] `TestWebSearch_HTTPError_401` — status 401 → `"Tavily API error (HTTP 401)"`, contains `"Invalid API key"` *(Spec 7.2)*
- [x] `TestWebSearch_HTTPError_429` — status 429 → `"Tavily API error (HTTP 429)"` *(Spec 7.2)*
- [x] `TestWebSearch_HTTPError_500` — status 500 → `"Tavily API error (HTTP 500)"`, contains `"internal error"` *(Spec 7.2)*

### Parse Errors
- [x] `TestWebSearch_InvalidJSON` — `"not json"` with 200 → `"failed to parse Tavily response"` *(Spec 7.2)*

### Network Errors
- [x] `TestWebSearch_ContextCancellation` — handler blocks 10s, context timeout 50ms → `IsError: true` *(Spec 7.2)*
- [x] `TestWebSearch_ConnectionRefused` — closed port → `IsError: true`, non-empty content *(Spec 7.2)*

### Request Verification
- [x] `TestWebSearch_BaseURLOverride` — verify server receives POST to `/search` *(Spec 7.2)*
- [x] `TestWebSearch_RequestBody` — verify body contains `query`, `max_results: 10`, `search_depth: "basic"`, `api_key` *(Spec 7.2)*
- [x] `TestWebSearch_AuthorizationHeader` — verify header `"Bearer test-key-123"` *(Spec 7.2)*

### Verbose Logging
- [x] `TestWebSearch_VerboseLog` — `Verbose: true`, buffer contains query, does NOT contain API key *(Spec 7.2)*
- [x] `TestWebSearch_VerboseLogQueryTruncation` — query >80 chars, logged summary is truncated *(Spec 7.2)*

### Safety Limits
- [x] `TestWebSearch_LargeResponseTruncation` — body >102400 bytes → `"Tavily response too large to process"` *(Spec 7.2)*
- [x] `TestWebSearch_ErrorBodyTruncation` — status 400, body >500 bytes → error body portion ≤500 bytes *(Spec 7.2)*

---

## Phase 9: Final Verification

- [x] `make test` exits 0 — all existing tests pass unmodified (except toolname_test.go updates)
- [x] `go build ./internal/tool/` succeeds
- [x] `go build ./...` succeeds
- [x] No new external dependencies in `go.mod`
- [x] No changes to `cmd/run.go` or `internal/tool/tool.go`
- [x] No changes to `internal/provider/` packages
- [x] `call_agent` is NOT registered in the registry
- [x] Scaffold comment includes `web_search`

---

## Constraints Checklist

- [x] No new external dependencies (`go.mod` unchanged) *(Constraint 1)*
- [x] No changes to `internal/provider/` *(Constraint 2)*
- [x] Only modified files outside new ones: `registry.go` (1 line), `toolname.go`, `toolname_test.go`, `agent.go`, `registry_test.go` *(Constraint 3)*
- [x] `call_agent` stays outside registry *(Constraint 4)*
- [x] No path validation used *(Constraint 5)*
- [x] `query` param is string from `map[string]string` *(Constraint 6)*
- [x] `tavilyMaxResults = 10` is a constant *(Constraint 7)*
- [x] `tavilyMaxResponseBytes = 102400` is a constant *(Constraint 8)*
- [x] `tavilyErrorBodyMax = 500` is a constant *(Constraint 9)*
- [x] API key via `os.Getenv`, not `config.ResolveAPIKey` *(Constraint 10)*
- [x] No custom User-Agent *(Constraint 11)*
- [x] No retry logic *(Constraint 12)*
