# Specification: `web_search` Built-in Tool (Tavily)

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-05
**GitHub Issue:** https://github.com/jrswab/axe/issues/6
**Scope:** Web search tool using Tavily Search API with JSON request/response, API key from environment, and formatted text output

---

## 1. Purpose

Implement the `web_search` tool — a built-in tool registered in the `Registry` that performs web searches via the Tavily Search API and returns results as formatted text (titles, URLs, snippets). This tool is needed for feature parity when axe replaces the custom container worker in Agent Engine.

This tool follows the same pattern as the M3-M8 tools (`list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`, `url_fetch`):

- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.WebSearch`** — new constant declared in `internal/toolname/toolname.go`
- **`ExecContext`** — reused for `Stderr` and `Verbose` (verbose logging). `Workdir` is not used by this tool.

This tool does NOT use `validatePath` or `isWithinDir`. It operates on the Tavily API, not file paths. There is no filesystem interaction.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Tavily Search API.** The tool sends a POST request to the Tavily Search API endpoint (`/search`). No other Tavily endpoints (extract, map, crawl) are used.

2. **Single parameter: `query`.** The tool has one required parameter named `query` of type `string`. There are no optional parameters. The `query` string is passed directly to the Tavily API's `query` field.

3. **API key from environment variable.** The Tavily API key is read from the `TAVILY_API_KEY` environment variable via `os.Getenv("TAVILY_API_KEY")` at execution time. If the variable is empty or unset, the tool returns an error. The API key is NOT resolved through `config.ResolveAPIKey` — Tavily is not an LLM provider; it is a tool dependency.

4. **Bearer token authentication.** The API key is sent in the `Authorization` header as `Bearer <api_key>`, per Tavily's API documentation. It is also included in the JSON request body as the `api_key` field for backward compatibility with older Tavily API versions.

5. **Base URL with environment override.** The default base URL is `https://api.tavily.com`. This can be overridden via the `AXE_TAVILY_BASE_URL` environment variable (useful for testing or self-hosted instances). The URL check uses `os.Getenv("AXE_TAVILY_BASE_URL")`.

6. **10 results maximum.** The request includes `"max_results": 10` in the JSON body. This is a package-level constant, not configurable by the agent or user.

7. **Basic search depth.** The request uses `"search_depth": "basic"`. Advanced search is not offered.

8. **No images or videos.** The request sets `"include_images": false` and `"include_images": false`. Only text results are returned.

9. **Formatted text output.** Each result is formatted as:
   ```
   Title: <title>
   URL: <url>
   Snippet: <content>
   ```
   Results are separated by a blank line (`\n\n`). If there are zero results, the output is the string `"No results found."`.

10. **Response body read limit.** The response body from Tavily is read with a limit of 102400 bytes (100KB) via `io.LimitReader`. This prevents excessive memory usage from unexpectedly large API responses. If the response exceeds this limit, the raw body is truncated and the tool returns an error indicating the response was too large to parse.

11. **HTTP error handling.** Non-2xx status codes from the Tavily API return `ToolResult{IsError: true}` with content in the format `"Tavily API error (HTTP %d): %s"` where the first value is the status code and the second is the response body (truncated to the first 500 bytes for readability).

12. **JSON parse errors.** If the Tavily API returns a 2xx response that cannot be parsed as JSON, the tool returns `ToolResult{IsError: true}` with a message containing `"failed to parse Tavily response"` and the underlying error.

13. **Context-based timeout.** The tool creates an `http.Request` using `http.NewRequestWithContext(ctx, "POST", url, body)`. The `ctx` parameter from the executor signature is passed directly. The tool inherits whatever timeout the caller provides.

14. **`http.DefaultClient` for execution.** The tool uses `http.DefaultClient.Do(req)` to execute the request. No custom transport.

15. **No retry logic.** A single request is made. If it fails, the error is returned.

16. **`call_agent` remains outside the registry.** Same as all prior tool milestones.

17. **No new external dependencies.** Uses only Go stdlib (`net/http`, `encoding/json`, `io`, `fmt`, `context`, `os`, `bytes`, `strings`).

---

## 3. Requirements

### 3.1 Tool Name Constant

**Requirement 1.1:** Add a new constant `WebSearch = "web_search"` to the `const` block in `internal/toolname/toolname.go`.

**Requirement 1.2:** Add `WebSearch: true` to the map returned by `ValidNames()` in `internal/toolname/toolname.go`.

### 3.2 `web_search` Tool Definition

**Requirement 2.1:** Create a file `internal/tool/web_search.go`.

**Requirement 2.2:** Define an unexported function `webSearchEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 2.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.WebSearch` (i.e., `"web_search"`)
- `Description`: a clear description for the LLM explaining the tool searches the web using Tavily and returns results with titles, URLs, and snippets
- `Parameters`: one parameter:
  - `query`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the search query to perform
    - `Required`: `true`

**Requirement 2.4:** The tool name in the definition must use `toolname.WebSearch`, not a hardcoded string.

**Requirement 2.5:** `ToolCall.Arguments` is `map[string]string`. The `query` parameter is a string used directly — no type conversion is needed.

### 3.3 `web_search` Tool Executor

**Requirement 3.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 3.2:** Extract the `query` argument from `call.Arguments["query"]`.

**Requirement 3.3 (Empty Query Check):** If `query` is an empty string (including when the key is absent from the map), return a `ToolResult` with `CallID: call.ID`, `Content: "query is required"`, `IsError: true`.

**Requirement 3.4 (API Key Check):** Read the API key from `os.Getenv("TAVILY_API_KEY")`. If the value is an empty string, return a `ToolResult` with `CallID: call.ID`, `Content: "TAVILY_API_KEY environment variable is not set"`, `IsError: true`.

**Requirement 3.5 (Base URL Resolution):** Read the base URL from `os.Getenv("AXE_TAVILY_BASE_URL")`. If the value is an empty string, use the default `"https://api.tavily.com"`. The default URL must be a package-level constant named `tavilyDefaultURL`.

**Requirement 3.6 (Build Request Body):** Construct a JSON request body with the following fields:
- `"query"`: the query string from the tool call argument
- `"max_results"`: the integer 10 (package-level constant `tavilyMaxResults`)
- `"search_depth"`: the string `"basic"`
- `"api_key"`: the API key string

The request body must be serialized using `encoding/json.Marshal()`. The struct used for marshaling must be an unexported type defined in the same file. If marshaling fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.7 (Create HTTP Request):** Create an HTTP request using `http.NewRequestWithContext(ctx, "POST", baseURL+"/search", bytes.NewReader(body))`. Set the following headers:
- `Content-Type`: `"application/json"`
- `Authorization`: `"Bearer " + apiKey`

If request creation fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.8 (Execute Request):** Execute the request using `http.DefaultClient.Do(req)`. If execution fails (network error, DNS failure, TLS error, context cancellation), return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.9 (Close Response Body):** Immediately after a successful `Do()` call, defer `resp.Body.Close()`.

**Requirement 3.10 (Read Response Body):** Read the response body using `io.ReadAll(io.LimitReader(resp.Body, tavilyMaxResponseBytes+1))` where `tavilyMaxResponseBytes` is 102400 (100KB, a package-level constant). If reading fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.11 (Response Too Large):** If the length of the read byte slice exceeds `tavilyMaxResponseBytes`, return a `ToolResult` with `CallID: call.ID`, `Content: "Tavily response too large to process"`, `IsError: true`.

**Requirement 3.12 (Non-2xx Status):** If `resp.StatusCode < 200 || resp.StatusCode >= 300`, return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"Tavily API error (HTTP %d): %s"` where the first value is `resp.StatusCode` and the second value is the response body truncated to the first 500 bytes. `IsError: true`.

**Requirement 3.13 (Parse JSON Response):** Unmarshal the response body into a struct containing a `Results` field (slice of result structs). Each result struct must have `Title string`, `URL string`, and `Content string` fields with appropriate `json` tags. The struct types must be unexported. If unmarshaling fails, return a `ToolResult` with `CallID: call.ID`, `Content` containing `"failed to parse Tavily response"` and the error message, `IsError: true`.

**Requirement 3.14 (Format Results):** Iterate over the results slice. For each result, format as:
```
Title: <title>
URL: <url>
Snippet: <content>
```
Separate results with `"\n\n"`. Use `strings.Builder` for concatenation. Return the final string trimmed of trailing whitespace.

**Requirement 3.15 (Empty Results):** If the results slice is empty (length 0) or nil, return a `ToolResult` with `CallID: call.ID`, `Content: "No results found."`, `IsError: false`.

**Requirement 3.16 (Success Case):** Return a `ToolResult` with `CallID: call.ID`, `Content` containing the formatted results string, `IsError: false`.

**Requirement 3.17 (Verbose Logging):** Use `toolVerboseLog(ec, toolname.WebSearch, result, summary)` via a deferred function, consistent with all other tools. The summary must include the query (truncated to 80 characters if longer). The summary must NOT include the API key.

### 3.4 Registration in Registry

**Requirement 4.1:** Add `r.Register(toolname.WebSearch, webSearchEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 4.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

### 3.5 Toolname Updates

**Requirement 5.1:** The `toolname_test.go` test `TestValidNames_ReturnsExpectedCount` currently asserts `len(names) != 6`. After adding `WebSearch`, this test must be updated to assert `len(names) != 7`.

**Requirement 5.2:** The `toolname_test.go` test `TestValidNames_ContainsAllExpectedNames` currently lists 6 tools. After adding `WebSearch`, this test must be updated to include `WebSearch` in the expected list.

**Requirement 5.3:** The `toolname_test.go` test `TestConstants_Values` currently has 7 entries (including `CallAgent`). After adding `WebSearch`, this test must be updated to include a case for `{"WebSearch", WebSearch, "web_search"}`.

### 3.6 Agent Scaffold Update

**Requirement 6.1:** Update the `Scaffold()` function in `internal/agent/agent.go` to include `web_search` in the commented tools list. The comment should read:
```
# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search
```

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   ├── toolname/
│   │   ├── toolname.go              # MODIFIED: add WebSearch constant + ValidNames entry
│   │   └── toolname_test.go         # MODIFIED: update count (6→7), add WebSearch to expected lists
│   └── tool/
│       ├── web_search.go            # NEW: Definition, Execute, request/response types, entry func
│       ├── web_search_test.go       # NEW: tests
│       ├── registry.go              # MODIFIED: add one line to RegisterAll
│       ├── url_fetch.go             # UNCHANGED
│       ├── run_command.go           # UNCHANGED
│       ├── edit_file.go             # UNCHANGED
│       ├── write_file.go            # UNCHANGED
│       ├── read_file.go             # UNCHANGED
│       ├── list_directory.go        # UNCHANGED
│       ├── path_validation.go       # UNCHANGED
│       ├── verbose.go               # UNCHANGED
│       ├── tool.go                  # UNCHANGED
│       └── tool_test.go            # UNCHANGED
├── internal/agent/
│   └── agent.go                     # MODIFIED: update Scaffold() tools comment
├── go.mod                           # UNCHANGED (no new dependencies)
├── go.sum                           # UNCHANGED
└── ...                              # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Query Argument

| Scenario | Input `query` | Behavior |
|----------|--------------|----------|
| Empty query | `""` | Error: `"query is required"` |
| Missing `query` key in Arguments map | Key absent | `call.Arguments["query"]` returns `""` -> Error: `"query is required"` |
| Valid query | `"golang concurrency patterns"` | Normal API call. Returns formatted results. |
| Very long query | 10,000+ character string | Passed to Tavily API. Tavily may reject it; tool reports the server's response. |
| Query with special characters | `"what is 2+2?"` | Passed as-is in JSON body. JSON marshaling handles escaping. |
| Whitespace-only query | `"   "` | Passed to Tavily API. Not special-cased — the tool does not trim whitespace. Tavily's handling determines the outcome. |

### 5.2 API Key

| Scenario | `TAVILY_API_KEY` env var | Behavior |
|----------|------------------------|----------|
| Not set | Absent | Error: `"TAVILY_API_KEY environment variable is not set"` |
| Empty string | `""` | Error: `"TAVILY_API_KEY environment variable is not set"` |
| Valid key | `"tvly-abc123..."` | Normal API call. Key sent in Authorization header and request body. |
| Invalid key | `"invalid-key"` | Tavily returns HTTP 401. Tool reports: `"Tavily API error (HTTP 401): ..."`. |
| Key with whitespace | `" tvly-abc123 "` | Passed as-is. Leading/trailing whitespace is not trimmed. Tavily may reject. |

### 5.3 Base URL

| Scenario | `AXE_TAVILY_BASE_URL` env var | Behavior |
|----------|------------------------------|----------|
| Not set | Absent | Uses default `"https://api.tavily.com"` |
| Empty string | `""` | Uses default `"https://api.tavily.com"` |
| Custom URL | `"http://localhost:8080"` | POST to `http://localhost:8080/search` |
| URL with trailing slash | `"https://api.tavily.com/"` | POST to `"https://api.tavily.com//search"` (double slash). Not corrected — the user must provide a clean URL. |

### 5.4 Tavily API HTTP Status Codes

| Scenario | Status Code | Behavior |
|----------|------------|----------|
| 200 OK | 200 | Parse JSON, format results, `IsError: false`. |
| 400 Bad Request | 400 | `IsError: true`. Content: `"Tavily API error (HTTP 400): <body>"`. |
| 401 Unauthorized | 401 | `IsError: true`. Content: `"Tavily API error (HTTP 401): <body>"`. Invalid or missing API key. |
| 429 Too Many Requests | 429 | `IsError: true`. Content: `"Tavily API error (HTTP 429): <body>"`. Rate limit exceeded. |
| 432 Key/Plan Limit Exceeded | 432 | `IsError: true`. Content: `"Tavily API error (HTTP 432): <body>"`. |
| 433 PayGo Limit Exceeded | 433 | `IsError: true`. Content: `"Tavily API error (HTTP 433): <body>"`. |
| 500 Internal Server Error | 500 | `IsError: true`. Content: `"Tavily API error (HTTP 500): <body>"`. |

### 5.5 Tavily API Response Parsing

| Scenario | Response body | Behavior |
|----------|--------------|----------|
| Normal results | `{"results": [{"title": "T", "url": "U", "content": "C"}]}` | Formatted output with title, URL, snippet. |
| Empty results array | `{"results": []}` | `"No results found."` with `IsError: false`. |
| Null results field | `{"results": null}` | `"No results found."` with `IsError: false`. |
| Missing results field | `{}` | `"No results found."` with `IsError: false` (JSON unmarshal leaves slice as nil). |
| Invalid JSON | `not json` | Error: `"failed to parse Tavily response: ..."`. |
| Extra fields in response | `{"results": [...], "usage": {...}, "request_id": "..."}` | Extra fields are ignored. Only `results` is extracted. |
| Extra fields in result objects | `{"results": [{"title": "T", "url": "U", "content": "C", "score": 0.95}]}` | Extra fields (`score`, `images`, `videos`) are ignored. Only `title`, `url`, `content` are used. |
| Result with empty title | `{"results": [{"title": "", "url": "U", "content": "C"}]}` | Formatted as `Title: \nURL: U\nSnippet: C`. Empty fields are not special-cased. |
| Response body > 100KB | Very large JSON | Error: `"Tavily response too large to process"`. |

### 5.6 Network Errors

| Scenario | Behavior |
|----------|----------|
| DNS resolution failure | `Do()` returns an error. `IsError: true`. Content: error message. |
| Connection refused | `Do()` returns an error. `IsError: true`. Content: error message. |
| Connection timeout | `Do()` returns an error (via context deadline). `IsError: true`. Content: error message. |
| TLS certificate error | `Do()` returns an error. `IsError: true`. Content: error message. |
| Context cancelled before request | `Do()` returns context error immediately. `IsError: true`. Content: error message. |
| Context cancelled during response read | `io.ReadAll` returns a context error. `IsError: true`. Content: error message. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modifications outside the new files are: one line added to `RegisterAll` in `registry.go`, constant and `ValidNames` update in `toolname.go`, test updates in `toolname_test.go`, scaffold comment in `agent.go`, and new registration tests in `registry_test.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** No path validation functions (`validatePath`, `isWithinDir`) are used by this tool. It operates on a web API, not file paths.

**Constraint 6:** `ToolCall.Arguments` is `map[string]string`. The `query` parameter is a string used directly.

**Constraint 7:** The maximum results count is 10. This value is a package-level constant (`tavilyMaxResults`), not configurable.

**Constraint 8:** The response body read limit is 102400 bytes (100KB). This value is a package-level constant (`tavilyMaxResponseBytes`), not configurable.

**Constraint 9:** The error body truncation for non-2xx responses is 500 bytes. This value is a package-level constant (`tavilyErrorBodyMax`), not configurable.

**Constraint 10:** The API key is read from `os.Getenv("TAVILY_API_KEY")` directly. It is NOT resolved through `config.ResolveAPIKey` or stored in `config.toml`. Tavily is not an LLM provider.

**Constraint 11:** No custom User-Agent header. The default Go `http.DefaultClient` User-Agent is used.

**Constraint 12:** No retry logic. A single request is made. If it fails, the error is returned.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in the existing tool tests:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **`net/http/httptest`:** Use `httptest.NewServer` to create real HTTP test servers that mock the Tavily API. Tests must make real HTTP requests against these servers.
- **Environment variable isolation:** Tests that set environment variables (`TAVILY_API_KEY`, `AXE_TAVILY_BASE_URL`) must use `t.Setenv()` so values are automatically restored after the test.
- **Descriptive names:** `TestWebSearch_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call the actual `webSearchExecute` function through the `ToolEntry.Execute` function pointer. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/web_search_test.go` Tests

**Test: `TestWebSearch_Success`** — Start an `httptest.NewServer` that returns a JSON response with 2 results (`{"results": [{"title": "Result 1", "url": "https://example.com/1", "content": "Snippet 1"}, {"title": "Result 2", "url": "https://example.com/2", "content": "Snippet 2"}]}`) with status 200. Set `t.Setenv("TAVILY_API_KEY", "test-key")` and `t.Setenv("AXE_TAVILY_BASE_URL", server.URL)`. Call `Execute` with `Arguments: {"query": "test query"}` and `ExecContext{}`. Verify `IsError` is false. Verify `Content` contains `"Title: Result 1"`. Verify `Content` contains `"URL: https://example.com/1"`. Verify `Content` contains `"Snippet: Snippet 1"`. Verify `Content` contains `"Title: Result 2"`. Verify `CallID` matches the input `call.ID`.

**Test: `TestWebSearch_EmptyQuery`** — Set `t.Setenv("TAVILY_API_KEY", "test-key")`. Call `Execute` with empty `Arguments` map `{}` and `ExecContext{}`. Verify `IsError` is true. Verify `Content` contains `"query is required"`.

**Test: `TestWebSearch_MissingQueryArgument`** — Set `t.Setenv("TAVILY_API_KEY", "test-key")`. Call `Execute` with `Arguments: {"query": ""}` and `ExecContext{}`. Verify `IsError` is true. Verify `Content` contains `"query is required"`.

**Test: `TestWebSearch_MissingAPIKey`** — Ensure `TAVILY_API_KEY` is not set (use `t.Setenv("TAVILY_API_KEY", "")`). Call `Execute` with `Arguments: {"query": "test"}` and `ExecContext{}`. Verify `IsError` is true. Verify `Content` contains `"TAVILY_API_KEY environment variable is not set"`.

**Test: `TestWebSearch_HTTPError_401`** — Start an `httptest.NewServer` that returns status 401 with body `{"detail": {"error": "Invalid API key"}}`. Set `t.Setenv("TAVILY_API_KEY", "bad-key")` and `t.Setenv("AXE_TAVILY_BASE_URL", server.URL)`. Call `Execute` with `Arguments: {"query": "test"}`. Verify `IsError` is true. Verify `Content` contains `"Tavily API error (HTTP 401)"`. Verify `Content` contains `"Invalid API key"`.

**Test: `TestWebSearch_HTTPError_429`** — Start an `httptest.NewServer` that returns status 429 with body `{"detail": {"error": "Rate limit exceeded"}}`. Set env vars. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"Tavily API error (HTTP 429)"`.

**Test: `TestWebSearch_HTTPError_500`** — Start an `httptest.NewServer` that returns status 500 with body `"internal error"`. Set env vars. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"Tavily API error (HTTP 500)"`. Verify `Content` contains `"internal error"`.

**Test: `TestWebSearch_EmptyResults`** — Start an `httptest.NewServer` that returns `{"results": []}` with status 200. Set env vars. Call `Execute`. Verify `IsError` is false. Verify `Content` equals `"No results found."`.

**Test: `TestWebSearch_NullResults`** — Start an `httptest.NewServer` that returns `{"results": null}` with status 200. Set env vars. Call `Execute`. Verify `IsError` is false. Verify `Content` equals `"No results found."`.

**Test: `TestWebSearch_InvalidJSON`** — Start an `httptest.NewServer` that returns `"not json"` with status 200. Set env vars. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"failed to parse Tavily response"`.

**Test: `TestWebSearch_ContextCancellation`** — Start an `httptest.NewServer` with a handler that blocks for 10 seconds. Create a context with a short timeout (50ms). Set env vars. Call `Execute` with the short-lived context. Verify `IsError` is true.

**Test: `TestWebSearch_ConnectionRefused`** — Create a URL pointing to `http://127.0.0.1:<closed-port>` (use a port from a recently-closed listener). Set `t.Setenv("TAVILY_API_KEY", "test-key")` and `t.Setenv("AXE_TAVILY_BASE_URL", "http://127.0.0.1:<port>")`. Call `Execute`. Verify `IsError` is true. Verify `Content` is non-empty.

**Test: `TestWebSearch_CallIDPassthrough`** — Start an `httptest.NewServer` returning valid results. Set env vars. Call `Execute` with `call.ID` set to `"ws-unique-42"`. Verify the returned `ToolResult.CallID` equals `"ws-unique-42"`.

**Test: `TestWebSearch_BaseURLOverride`** — Start an `httptest.NewServer` that records incoming request paths. Set `t.Setenv("AXE_TAVILY_BASE_URL", server.URL)` and `t.Setenv("TAVILY_API_KEY", "test-key")`. Call `Execute`. Verify the server received a request to the `/search` path. Verify the request method is `POST`.

**Test: `TestWebSearch_RequestBody`** — Start an `httptest.NewServer` that captures the request body and verifies it contains the expected fields. Set env vars. Call `Execute` with `Arguments: {"query": "my search"}`. Verify the captured request body contains `"query": "my search"`, `"max_results": 10`, `"search_depth": "basic"`, and `"api_key": "test-key"`.

**Test: `TestWebSearch_AuthorizationHeader`** — Start an `httptest.NewServer` that captures the `Authorization` header. Set env vars with `TAVILY_API_KEY=test-key-123`. Call `Execute`. Verify the captured header equals `"Bearer test-key-123"`.

**Test: `TestWebSearch_VerboseLog`** — Set env vars. Start an `httptest.NewServer` returning valid results. Create an `ExecContext` with `Verbose: true` and `Stderr` set to a `bytes.Buffer`. Call `Execute` with `Arguments: {"query": "my verbose test"}`. Verify the buffer contains `"my verbose test"`. Verify the buffer does NOT contain the API key value.

**Test: `TestWebSearch_VerboseLogQueryTruncation`** — Same as above but with a query longer than 80 characters. Verify the logged summary is truncated (does not contain the full query).

**Test: `TestWebSearch_LargeResponseTruncation`** — Start an `httptest.NewServer` that returns a 200 response with a body larger than 102400 bytes. Set env vars. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"Tavily response too large to process"`.

**Test: `TestWebSearch_ErrorBodyTruncation`** — Start an `httptest.NewServer` that returns status 400 with a body larger than 500 bytes (e.g., 1000 bytes of `'E'`). Set env vars. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"Tavily API error (HTTP 400)"`. Verify the length of the error body portion in `Content` does not exceed 500 bytes.

### 7.3 `internal/toolname/toolname_test.go` Updates

**Update: `TestValidNames_ReturnsExpectedCount`** — Change the expected count from 6 to 7.

**Update: `TestValidNames_ContainsAllExpectedNames`** — Add `WebSearch` to the `expected` slice.

**Update: `TestConstants_Values`** — Add a table entry: `{"WebSearch", WebSearch, "web_search"}`.

### 7.4 `RegisterAll` Tests (additions to `registry_test.go`)

**Test: `TestRegisterAll_RegistersWebSearch`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.WebSearch)` returns true.

**Test: `TestRegisterAll_ResolvesWebSearch`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.WebSearch})`. Verify the returned tool has `Name` equal to `toolname.WebSearch` and has a `query` parameter.

### 7.5 Existing Tests

All existing tests must continue to pass without modification (except the `toolname_test.go` updates in section 7.3):

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go` (existing tests)
- `internal/tool/url_fetch_test.go`
- `internal/tool/run_command_test.go`
- `internal/tool/edit_file_test.go`
- `internal/tool/write_file_test.go`
- `internal/tool/read_file_test.go`
- `internal/tool/list_directory_test.go`
- `internal/tool/path_validation_test.go`
- `internal/tool/verbose_test.go`
- `internal/agent/agent_test.go`
- `cmd/run_test.go`
- `cmd/smoke_test.go`
- `cmd/golden_test.go`
- `cmd/fixture_test.go`
- All other test files in the project.

### 7.6 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/web_search.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `toolname.WebSearch` constant exists with value `"web_search"` | `TestConstants_Values` passes |
| `ValidNames()` includes `web_search` | `TestValidNames_ReturnsExpectedCount` (7) and `TestValidNames_ContainsAllExpectedNames` pass |
| `RegisterAll` registers `web_search` | `TestRegisterAll_RegistersWebSearch` passes |
| Tool definition has correct name | `toolname.WebSearch` constant used, not hardcoded string |
| Tool definition has 1 parameter | `query` (required) |
| Successful search returns formatted results | `TestWebSearch_Success` passes |
| Empty query rejected | `TestWebSearch_EmptyQuery` passes |
| Missing query rejected | `TestWebSearch_MissingQueryArgument` passes |
| Missing API key returns clear error | `TestWebSearch_MissingAPIKey` passes |
| 401 returns descriptive error | `TestWebSearch_HTTPError_401` passes |
| 429 returns descriptive error | `TestWebSearch_HTTPError_429` passes |
| 500 returns descriptive error | `TestWebSearch_HTTPError_500` passes |
| Empty results handled | `TestWebSearch_EmptyResults` passes |
| Null results handled | `TestWebSearch_NullResults` passes |
| Invalid JSON returns parse error | `TestWebSearch_InvalidJSON` passes |
| Context timeout handled | `TestWebSearch_ContextCancellation` passes |
| Connection failure handled | `TestWebSearch_ConnectionRefused` passes |
| CallID propagated | `TestWebSearch_CallIDPassthrough` passes |
| Base URL override works | `TestWebSearch_BaseURLOverride` passes |
| Request body correct | `TestWebSearch_RequestBody` passes |
| Authorization header correct | `TestWebSearch_AuthorizationHeader` passes |
| Verbose log includes query, excludes key | `TestWebSearch_VerboseLog` passes |
| Verbose log truncates long queries | `TestWebSearch_VerboseLogQueryTruncation` passes |
| Large response rejected | `TestWebSearch_LargeResponseTruncation` passes |
| Error body truncated | `TestWebSearch_ErrorBodyTruncation` passes |
| Registry registers web_search | `TestRegisterAll_RegistersWebSearch` passes |
| Registry resolves web_search with correct params | `TestRegisterAll_ResolvesWebSearch` passes |
| Scaffold comment lists web_search | Manual verification of `Scaffold()` output |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Tavily Extract endpoint (`/extract`)
2. Tavily Map endpoint (`/map`)
3. Tavily Crawl endpoint (`/crawl`)
4. Configurable `search_depth` (always `"basic"`)
5. Configurable `max_results` (always 10)
6. Image or video results (`include_images`, `include_videos` always false)
7. Domain filtering (`include_domains`, `exclude_domains`)
8. Pagination / `next_page` handling
9. Response caching
10. Retry logic or exponential backoff
11. API key resolution through `config.toml` or `config.ResolveAPIKey`
12. Storing Tavily config in `config.toml` provider section
13. Custom HTTP transport, TLS configuration, or proxy settings
14. Custom User-Agent header
15. Rate limiting or request throttling
16. Streaming response output
17. Result scoring or filtering by relevance score
18. HTML content extraction or conversion
19. Changes to `--dry-run`, `--json`, or `--verbose` output formatting (existing patterns already handle new tools)
20. Changes to `cmd/run.go` or `internal/tool/tool.go`

---

## 10. References

- GitHub Issue: https://github.com/jrswab/axe/issues/6
- Tavily Search API: `POST https://api.tavily.com/search`
- Tavily API Authentication: Bearer token via `Authorization` header
- url_fetch Tool Spec (pattern reference): `docs/plans/025_url_fetch_spec.md`
- `RegisterAll` function: `internal/tool/registry.go:92`
- `ValidNames` function: `internal/toolname/toolname.go:19`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ToolEntry` type: `internal/tool/registry.go:20`
- `ExecContext` type: `internal/tool/registry.go:13`
- `toolVerboseLog` helper: `internal/tool/verbose.go:11`
- `Scaffold` function: `internal/agent/agent.go:157`
- Agent config `tools` field: `internal/agent/agent.go:45`
- Tool call milestones: `docs/plans/000_tool_call_milestones.md`

---

## 11. Notes

- **No `config.ResolveAPIKey` usage.** Tavily is not an LLM provider — it's a tool dependency. LLM providers (anthropic, openai, ollama) use the `config.ResolveAPIKey` chain (env var > config.toml). Tavily's key is read directly from `TAVILY_API_KEY` because there is no reason to store it in `config.toml` alongside LLM provider keys. If future tools also need API keys, this pattern can be revisited.
- **`api_key` in both header and body.** The Tavily API documentation shows two authentication methods: Bearer token in the `Authorization` header (current docs) and `api_key` field in the JSON body (older docs/quickstart). Including both ensures compatibility across Tavily API versions.
- **100KB response limit.** The Tavily API typically returns small JSON payloads (a few KB for 10 results). The 100KB limit is a safety rail, not a practical constraint. It prevents memory issues if the API behavior changes or returns unexpected data.
- **500-byte error body truncation.** Tavily error responses are typically small JSON objects (`{"detail": {"error": "..."}}`). The 500-byte limit keeps error messages readable while preventing large error pages from flooding the tool result.
- **`io.LimitReader` for memory safety.** Without a read limit, a misbehaving or compromised API could send a multi-gigabyte response, causing OOM. `io.LimitReader` caps memory usage at `tavilyMaxResponseBytes+1` bytes.
- **No `Workdir` usage.** Unlike file tools, `web_search` does not use `ExecContext.Workdir`. The tool makes network API calls, not filesystem operations.
- **The `RegisterAll` pattern continues.** This adds the seventh tool: `list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`, `url_fetch`, `web_search`.
- **Security model.** The `web_search` tool allows the LLM to perform arbitrary web searches. This is by design — if an agent has `tools = ["web_search"]` in its TOML config, the user has explicitly opted in. The API key acts as the access control — no key means no searches. The tool never exposes the API key to the LLM (it's in the HTTP header/body, not in the tool result).
