# Specification: `url_fetch` Built-in Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-05
**GitHub Issue:** https://github.com/jrswab/axe/issues/7
**Scope:** HTTP GET tool with response truncation, scheme validation, and graceful error handling

---

## 1. Purpose

Implement the `url_fetch` tool — a built-in tool registered in the `Registry` that performs an HTTP GET request and returns the response body as text. This tool is needed for feature parity when axe replaces the custom container worker in Agent Engine.

This tool follows the same pattern as the M3-M7 tools (`list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`):

- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.URLFetch`** — new constant declared in `internal/toolname/toolname.go`
- **`ExecContext`** — reused for `Stderr` and `Verbose` (verbose logging). `Workdir` is not used by this tool.

This tool does NOT use `validatePath` or `isWithinDir`. It operates on URLs, not file paths. There is no filesystem interaction.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **HTTP GET only.** The tool performs a single HTTP GET request. No POST, PUT, DELETE, or other methods. No request body. No custom headers.

2. **Scheme validation.** Only `http://` and `https://` schemes are accepted. Any other scheme (e.g., `file://`, `ftp://`, `data:`) is rejected before any network request is made. This prevents the LLM from reading local files or using unexpected protocols.

3. **URL parsing via `net/url.Parse`.** The `url` argument is parsed using Go's `net/url.Parse()`. If parsing fails, the tool returns an error. The parsed URL's `Scheme` field is checked against `"http"` and `"https"`.

4. **Context-based timeout.** The tool creates an `http.Request` using `http.NewRequestWithContext(ctx, "GET", url, nil)`. The `ctx` parameter from the executor signature is passed directly. This means the tool inherits whatever timeout the caller provides (from the agent's `--timeout` flag). The tool does not impose its own timeout.

5. **`http.DefaultClient` for execution.** The tool uses `http.DefaultClient.Do(req)` to execute the request. No custom transport, no TLS configuration, no cookie jar. `http.DefaultClient` follows redirects (up to 10 by default per Go stdlib).

6. **Response body read limit via `io.LimitReader`.** The response body is read using `io.LimitReader(resp.Body, maxReadBytes+1)` where `maxReadBytes` is 10000. Reading one byte beyond the limit allows detecting whether truncation occurred. If exactly `maxReadBytes+1` bytes are read, the response is truncated to `maxReadBytes` bytes and a truncation notice is appended.

7. **Truncation notice.** When the response exceeds 10000 bytes, the content is truncated to 10000 bytes and the string `"\n... [response truncated, exceeded 10000 characters]"` is appended. Truncation is byte-level, not rune-aware.

8. **Non-2xx status codes return an error.** If the HTTP response status code is outside the 200-299 range, the tool returns `ToolResult{IsError: true}` with content in the format `"HTTP %d: %s"` where the first value is the status code and the second value is the response body (truncated if needed). The response body is still read and included because it often contains useful error information.

9. **Success result.** For 2xx responses, the tool returns `ToolResult{CallID: call.ID, Content: string(body), IsError: false}` where `body` is the response body after truncation if applicable.

10. **`resp.Body.Close()` via `defer`.** The response body is closed via `defer resp.Body.Close()` immediately after a successful `http.DefaultClient.Do()` call.

11. **Network and DNS errors.** If `http.DefaultClient.Do()` returns an error (DNS failure, connection refused, TLS error, context cancellation), the tool returns `ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}`.

12. **Single parameter: `url`.** The tool has one required parameter named `url` of type `string`. There are no optional parameters. An empty or missing `url` is rejected before any parsing or network request.

13. **`call_agent` remains outside the registry.** Same as all prior tool milestones.

14. **No new external dependencies.** Uses only Go stdlib (`net/http`, `net/url`, `io`, `fmt`, `context`).

---

## 3. Requirements

### 3.1 Tool Name Constant

**Requirement 1.1:** Add a new constant `URLFetch = "url_fetch"` to the `const` block in `internal/toolname/toolname.go`.

**Requirement 1.2:** Add `URLFetch: true` to the map returned by `ValidNames()` in `internal/toolname/toolname.go`.

### 3.2 `url_fetch` Tool Definition

**Requirement 2.1:** Create a file `internal/tool/url_fetch.go`.

**Requirement 2.2:** Define an unexported function `urlFetchEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 2.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.URLFetch` (i.e., `"url_fetch"`)
- `Description`: a clear description for the LLM explaining the tool fetches content from a URL via HTTP GET and returns the response body as text
- `Parameters`: one parameter:
  - `url`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the URL to fetch
    - `Required`: `true`

**Requirement 2.4:** The tool name in the definition must use `toolname.URLFetch`, not a hardcoded string.

**Requirement 2.5:** `ToolCall.Arguments` is `map[string]string`. The `url` parameter is a string used directly — no type conversion is needed.

### 3.3 `url_fetch` Tool Executor

**Requirement 3.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 3.2:** Extract the `url` argument from `call.Arguments["url"]`.

**Requirement 3.3 (Empty URL Check):** If `url` is an empty string (including when the key is absent from the map), return a `ToolResult` with `CallID: call.ID`, `Content: "url is required"`, `IsError: true`.

**Requirement 3.4 (URL Parsing):** Parse the URL using `net/url.Parse(urlStr)`. If parsing fails, return a `ToolResult` with `CallID: call.ID`, `Content` containing the parse error message, `IsError: true`.

**Requirement 3.5 (Scheme Validation):** After successful parsing, check that the URL's `Scheme` field is either `"http"` or `"https"`. If not, return a `ToolResult` with `CallID: call.ID`, `Content: "unsupported scheme %q: only http and https are allowed"` (where `%q` is the actual scheme), `IsError: true`. If the scheme is empty (URL has no scheme), the error message must reflect this (the empty string formatted with `%q` produces `""`).

**Requirement 3.6 (Create HTTP Request):** Create an HTTP request using `http.NewRequestWithContext(ctx, "GET", urlStr, nil)`. If request creation fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.7 (Execute Request):** Execute the request using `http.DefaultClient.Do(req)`. If execution fails (network error, DNS failure, TLS error, context cancellation), return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.8 (Close Response Body):** Immediately after a successful `Do()` call, defer `resp.Body.Close()`.

**Requirement 3.9 (Read Response Body):** Read the response body using `io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))` where `maxReadBytes` is 10000 (a package-level constant). If reading fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 3.10 (Truncation):** If the length of the read byte slice exceeds `maxReadBytes` (i.e., `len(body) > maxReadBytes`), truncate the slice to `maxReadBytes` bytes and convert to string. Append `"\n... [response truncated, exceeded 10000 characters]"` to the string.

**Requirement 3.11 (Non-2xx Status):** If `resp.StatusCode < 200 || resp.StatusCode >= 300`, return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"HTTP %d: %s"` (where the first value is `resp.StatusCode` and the second value is the body string after truncation if applicable), `IsError: true`.

**Requirement 3.12 (Success Case):** If the status code is in the 200-299 range, return a `ToolResult` with `CallID: call.ID`, `Content: bodyStr` (after truncation if applicable), `IsError: false`.

**Requirement 3.13 (Verbose Logging):** Use `toolVerboseLog(ec, toolname.URLFetch, result, summary)` via a deferred function, consistent with all other tools. The summary should include the URL (or a truncated version for long URLs) and the HTTP status code when available.

### 3.4 Registration in Registry

**Requirement 4.1:** Add `r.Register(toolname.URLFetch, urlFetchEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 4.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

### 3.5 Toolname Updates

**Requirement 5.1:** The `toolname_test.go` test `TestValidNames_ReturnsExpectedCount` currently asserts `len(names) != 5`. After adding `URLFetch`, this test must be updated to assert `len(names) != 6`.

**Requirement 5.2:** The `toolname_test.go` test `TestValidNames_ContainsAllExpectedNames` currently lists 5 tools. After adding `URLFetch`, this test must be updated to include `URLFetch` in the expected list.

**Requirement 5.3:** The `toolname_test.go` test `TestConstants_Values` currently has 6 entries (including `CallAgent`). After adding `URLFetch`, this test must be updated to include a case for `{"URLFetch", URLFetch, "url_fetch"}`.

### 3.6 Agent Scaffold Update

**Requirement 6.1:** Update the `Scaffold()` function in `internal/agent/agent.go` to include `url_fetch` in the commented tools list. The comment should read:
```
# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch
```

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   ├── toolname/
│   │   ├── toolname.go              # MODIFIED: add URLFetch constant + ValidNames entry
│   │   └── toolname_test.go         # MODIFIED: update count (5→6), add URLFetch to expected lists
│   └── tool/
│       ├── url_fetch.go             # NEW: Definition, Execute, helper entry func
│       ├── url_fetch_test.go        # NEW: tests
│       ├── registry.go              # MODIFIED: add one line to RegisterAll
│       ├── registry_test.go         # MODIFIED: add url_fetch registration tests
│       ├── run_command.go           # UNCHANGED
│       ├── edit_file.go             # UNCHANGED
│       ├── write_file.go            # UNCHANGED
│       ├── read_file.go             # UNCHANGED
│       ├── list_directory.go        # UNCHANGED
│       ├── path_validation.go       # UNCHANGED
│       ├── verbose.go               # UNCHANGED
│       ├── tool.go                  # UNCHANGED
│       └── tool_test.go             # UNCHANGED
├── internal/agent/
│   └── agent.go                     # MODIFIED: update Scaffold() tools comment
├── go.mod                           # UNCHANGED (no new dependencies)
├── go.sum                           # UNCHANGED
└── ...                              # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 URL Argument

| Scenario | Input `url` | Behavior |
|----------|------------|----------|
| Empty URL | `""` | Error: `"url is required"` |
| Missing `url` key in Arguments map | Key absent | `call.Arguments["url"]` returns `""` -> Error: `"url is required"` |
| Valid HTTPS URL | `"https://example.com"` | Normal HTTP GET. Returns response body. |
| Valid HTTP URL | `"http://example.com"` | Normal HTTP GET. Returns response body. |
| URL without scheme | `"example.com"` | `net/url.Parse` succeeds but `Scheme` is empty. Error: `unsupported scheme "": only http and https are allowed`. |
| File scheme | `"file:///etc/passwd"` | Error: `unsupported scheme "file": only http and https are allowed`. |
| FTP scheme | `"ftp://files.example.com/data"` | Error: `unsupported scheme "ftp": only http and https are allowed`. |
| Data URI | `"data:text/plain;base64,SGVsbG8="` | Error: `unsupported scheme "data": only http and https are allowed`. |
| Malformed URL | `"://bad"` | `net/url.Parse` returns an error. Error includes parse error message. |
| URL with port | `"http://localhost:8080/path"` | Normal HTTP GET. Port is part of the URL. |
| URL with query string | `"https://api.example.com/data?key=val"` | Normal HTTP GET. Query string is preserved. |
| URL with fragment | `"https://example.com/page#section"` | Normal HTTP GET. Fragment is not sent to server (per HTTP spec). |
| URL with auth | `"https://user:pass@example.com"` | Normal HTTP GET. Go's http client includes auth in the request. |
| Very long URL | 10,000+ character URL | Passed to http client. Server may reject with 414. Tool reports the server's response. |

### 5.2 HTTP Status Codes

| Scenario | Status Code | Behavior |
|----------|------------|----------|
| 200 OK | 200 | `IsError: false`. Content is response body. |
| 201 Created | 201 | `IsError: false`. Content is response body. (Any 2xx is success.) |
| 204 No Content | 204 | `IsError: false`. Content is empty string (no body). |
| 301 Moved Permanently | 301 | `http.DefaultClient` follows the redirect (up to 10 hops). Final response determines success/error. |
| 302 Found | 302 | Same as 301 — redirect is followed automatically. |
| 400 Bad Request | 400 | `IsError: true`. Content: `"HTTP 400: <body>"`. |
| 401 Unauthorized | 401 | `IsError: true`. Content: `"HTTP 401: <body>"`. |
| 403 Forbidden | 403 | `IsError: true`. Content: `"HTTP 403: <body>"`. |
| 404 Not Found | 404 | `IsError: true`. Content: `"HTTP 404: <body>"`. |
| 429 Too Many Requests | 429 | `IsError: true`. Content: `"HTTP 429: <body>"`. |
| 500 Internal Server Error | 500 | `IsError: true`. Content: `"HTTP 500: <body>"`. |
| 502 Bad Gateway | 502 | `IsError: true`. Content: `"HTTP 502: <body>"`. |
| 503 Service Unavailable | 503 | `IsError: true`. Content: `"HTTP 503: <body>"`. |

### 5.3 Response Body Truncation

| Scenario | Response size | Behavior |
|----------|--------------|----------|
| Response < 10000 bytes | < 10000 bytes | No truncation. Content is full body. |
| Response = 10000 bytes exactly | 10000 bytes | No truncation. The threshold is "exceeds", not "equals". (`io.LimitReader` reads 10001 max; if read returns exactly 10000 bytes, no truncation.) |
| Response > 10000 bytes | > 10000 bytes | Truncated to 10000 bytes. `"\n... [response truncated, exceeded 10000 characters]"` appended. |
| Large response on success (2xx) | > 10000 bytes, status 200 | Content: first 10000 bytes + truncation notice. `IsError: false`. |
| Large response on error (non-2xx) | > 10000 bytes, status 500 | Content: `"HTTP 500: "` + first 10000 bytes + truncation notice. `IsError: true`. |
| Truncation at multi-byte boundary | Truncated mid-UTF-8 sequence | Truncation occurs at the byte level. The resulting string may contain an incomplete UTF-8 sequence. No rune-aware truncation is performed. |
| Empty response body | 0 bytes | Content is empty string (or `"HTTP <code>: "` for non-2xx). |
| Binary response | Binary content | Content contains raw bytes as string. No encoding detection or conversion. |

### 5.4 Network Errors

| Scenario | Behavior |
|----------|----------|
| DNS resolution failure | `Do()` returns an error. `IsError: true`. Content: error message (e.g., `"dial tcp: lookup nonexistent.invalid: no such host"`). |
| Connection refused | `Do()` returns an error. `IsError: true`. Content: error message. |
| Connection timeout | `Do()` returns an error (via context deadline). `IsError: true`. Content: error message. |
| TLS certificate error | `Do()` returns an error. `IsError: true`. Content: error message (e.g., `"tls: certificate is not trusted"`). |
| Context cancelled before request | `Do()` returns context error immediately. `IsError: true`. Content: `"context canceled"` or similar. |
| Context cancelled during response read | `io.ReadAll` returns a context error. `IsError: true`. Content: error message. |
| Server closes connection mid-response | `io.ReadAll` returns an error (e.g., `"unexpected EOF"`). `IsError: true`. Content: error message. |
| Redirect loop (> 10 hops) | `Do()` returns an error. `IsError: true`. Content: error message about redirect loop. |

### 5.5 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `url` argument present | Normal operation. |
| `url` argument missing from `call.Arguments` | `call.Arguments["url"]` returns `""` -> Error: `"url is required"`. |
| `url` argument is empty string | Error: `"url is required"`. |
| Whitespace-only URL | Passed to `net/url.Parse`. Parsing may succeed or fail depending on content. If scheme validation fails, appropriate error. Not special-cased — the tool does not trim whitespace. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modifications outside the new files are: one line added to `RegisterAll` in `registry.go`, constant and `ValidNames` update in `toolname.go`, test updates in `toolname_test.go`, scaffold comment in `agent.go`, and new registration tests in `registry_test.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** No path validation functions (`validatePath`, `isWithinDir`) are used by this tool. It operates on URLs, not file paths.

**Constraint 6:** `ToolCall.Arguments` is `map[string]string`. The `url` parameter is a string used directly.

**Constraint 7:** The read limit is 10000 bytes. This value is a package-level constant (`maxReadBytes`), not configurable.

**Constraint 8:** No response transformation. The response body is returned as-is (after truncation if applicable). No HTML-to-text conversion, no JSON pretty-printing, no encoding detection.

**Constraint 9:** No custom User-Agent header. The default Go `http.DefaultClient` User-Agent is used.

**Constraint 10:** No retry logic. A single request is made. If it fails, the error is returned.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in the existing tool tests:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **`net/http/httptest`:** Use `httptest.NewServer` to create real HTTP test servers. Tests must make real HTTP requests against these servers.
- **Descriptive names:** `TestURLFetch_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real HTTP servers. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/url_fetch_test.go` Tests

**Test: `TestURLFetch_Success`** — Start an `httptest.NewServer` that returns `"hello world"` with status 200. Call `Execute` with `Arguments: {"url": server.URL}` and `ExecContext{}`. Verify `IsError` is false. Verify `Content` equals `"hello world"`. Verify `CallID` matches the input `call.ID`.

**Test: `TestURLFetch_EmptyURL`** — Call `Execute` with empty `Arguments` map `{}` and `ExecContext{}`. Verify `IsError` is true. Verify `Content` contains `"url is required"`.

**Test: `TestURLFetch_MissingURLArgument`** — Call `Execute` with `Arguments: {"url": ""}` and `ExecContext{}`. Verify `IsError` is true. Verify `Content` contains `"url is required"`.

**Test: `TestURLFetch_UnsupportedScheme_File`** — Call `Execute` with `Arguments: {"url": "file:///etc/passwd"}`. Verify `IsError` is true. Verify `Content` contains `"unsupported scheme"`. Verify `Content` contains `"file"`.

**Test: `TestURLFetch_UnsupportedScheme_FTP`** — Call `Execute` with `Arguments: {"url": "ftp://example.com/file"}`. Verify `IsError` is true. Verify `Content` contains `"unsupported scheme"`. Verify `Content` contains `"ftp"`.

**Test: `TestURLFetch_NoScheme`** — Call `Execute` with `Arguments: {"url": "example.com"}`. Verify `IsError` is true. Verify `Content` contains `"unsupported scheme"`.

**Test: `TestURLFetch_Non2xxStatus_404`** — Start an `httptest.NewServer` that returns `"not found"` with status 404. Call `Execute` with the server URL. Verify `IsError` is true. Verify `Content` contains `"HTTP 404"`. Verify `Content` contains `"not found"`.

**Test: `TestURLFetch_Non2xxStatus_500`** — Start an `httptest.NewServer` that returns `"internal error"` with status 500. Call `Execute` with the server URL. Verify `IsError` is true. Verify `Content` contains `"HTTP 500"`. Verify `Content` contains `"internal error"`.

**Test: `TestURLFetch_LargeResponseTruncation`** — Start an `httptest.NewServer` that returns a response body larger than 10000 bytes (e.g., 20000 bytes of `'A'`). Call `Execute` with the server URL. Verify `IsError` is false. Verify `Content` contains `"[response truncated, exceeded 10000 characters]"`. Verify `len(Content)` is greater than 10000 (includes truncation notice) but significantly less than 20000 (proving truncation).

**Test: `TestURLFetch_ExactLimitNotTruncated`** — Start an `httptest.NewServer` that returns a response body of exactly 10000 bytes. Call `Execute` with the server URL. Verify `IsError` is false. Verify `Content` does NOT contain `"truncated"`. Verify `len(Content)` equals 10000.

**Test: `TestURLFetch_ContextCancellation`** — Start an `httptest.NewServer` with a handler that blocks (e.g., `time.Sleep(10 * time.Second)`). Create a context with a short timeout (e.g., `context.WithTimeout(context.Background(), 50*time.Millisecond)`). Call `Execute` with the server URL. Verify `IsError` is true.

**Test: `TestURLFetch_ConnectionRefused`** — Create a URL pointing to `http://127.0.0.1:<closed-port>` (use a port from a recently-closed listener). Call `Execute`. Verify `IsError` is true. Verify `Content` is non-empty (contains an error message).

**Test: `TestURLFetch_CallIDPassthrough`** — Start an `httptest.NewServer` that returns 200. Call `Execute` with `call.ID` set to `"uf-unique-42"`. Verify the returned `ToolResult.CallID` equals `"uf-unique-42"`.

**Test: `TestURLFetch_EmptyResponseBody`** — Start an `httptest.NewServer` that returns status 200 with an empty body. Call `Execute`. Verify `IsError` is false. Verify `Content` equals `""`.

**Test: `TestURLFetch_Non2xxWithLargeBody`** — Start an `httptest.NewServer` that returns status 500 with a body larger than 10000 bytes. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"HTTP 500"`. Verify `Content` contains `"[response truncated, exceeded 10000 characters]"`.

### 7.3 `internal/toolname/toolname_test.go` Updates

**Update: `TestValidNames_ReturnsExpectedCount`** — Change the expected count from 5 to 6.

**Update: `TestValidNames_ContainsAllExpectedNames`** — Add `URLFetch` to the `expected` slice.

**Update: `TestConstants_Values`** — Add a table entry: `{"URLFetch", URLFetch, "url_fetch"}`.

### 7.4 `RegisterAll` Tests (additions to `registry_test.go`)

**Test: `TestRegisterAll_RegistersURLFetch`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.URLFetch)` returns true.

**Test: `TestRegisterAll_ResolvesURLFetch`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.URLFetch})`. Verify the returned tool has `Name` equal to `toolname.URLFetch` and has a `url` parameter.

### 7.5 Existing Tests

All existing tests must continue to pass without modification (except the `toolname_test.go` updates in section 7.3):

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go` (existing tests)
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
| `internal/tool/url_fetch.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `toolname.URLFetch` constant exists with value `"url_fetch"` | `TestConstants_Values` passes |
| `ValidNames()` includes `url_fetch` | `TestValidNames_ReturnsExpectedCount` (6) and `TestValidNames_ContainsAllExpectedNames` pass |
| `RegisterAll` registers `url_fetch` | `TestRegisterAll_RegistersURLFetch` passes |
| Tool definition has correct name | `toolname.URLFetch` constant used, not hardcoded string |
| Tool definition has 1 parameter | `url` (required) |
| Successful GET returns body | `TestURLFetch_Success` passes |
| Empty URL rejected | `TestURLFetch_EmptyURL` passes |
| Missing URL rejected | `TestURLFetch_MissingURLArgument` passes |
| File scheme rejected | `TestURLFetch_UnsupportedScheme_File` passes |
| FTP scheme rejected | `TestURLFetch_UnsupportedScheme_FTP` passes |
| URL without scheme rejected | `TestURLFetch_NoScheme` passes |
| 404 returns error with status | `TestURLFetch_Non2xxStatus_404` passes |
| 500 returns error with status | `TestURLFetch_Non2xxStatus_500` passes |
| Large response truncated | `TestURLFetch_LargeResponseTruncation` passes |
| Exact limit not truncated | `TestURLFetch_ExactLimitNotTruncated` passes |
| Context timeout handled | `TestURLFetch_ContextCancellation` passes |
| Connection failure handled | `TestURLFetch_ConnectionRefused` passes |
| CallID propagated | `TestURLFetch_CallIDPassthrough` passes |
| Empty body returns empty string | `TestURLFetch_EmptyResponseBody` passes |
| Non-2xx with large body truncated | `TestURLFetch_Non2xxWithLargeBody` passes |
| Registry registers url_fetch | `TestRegisterAll_RegistersURLFetch` passes |
| Registry resolves url_fetch with correct params | `TestRegisterAll_ResolvesURLFetch` passes |
| Scaffold comment lists url_fetch | Manual verification of `Scaffold()` output |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. HTTP methods other than GET (POST, PUT, DELETE, etc.)
2. Custom request headers (User-Agent, Authorization, Accept, etc.)
3. Request body / payload
4. Response format conversion (HTML-to-text, JSON formatting, Markdown conversion)
5. Response encoding detection or conversion (charset handling)
6. Custom TLS configuration or certificate pinning
7. Retry logic or exponential backoff
8. Rate limiting or request throttling
9. Caching of responses
10. Cookie handling beyond `http.DefaultClient` defaults
11. Proxy configuration beyond `http.DefaultClient` defaults (which respects `HTTP_PROXY` env)
12. Custom timeout per tool invocation (inherits caller's context)
13. Configurable truncation limit (hardcoded at 10000 bytes)
14. Streaming response output
15. Following `meta` refresh redirects (only HTTP 3xx redirects are followed)
16. Robot.txt compliance
17. Changes to `--dry-run`, `--json`, or `--verbose` output formatting (existing patterns already handle new tools)
18. Changes to `cmd/run.go` or `internal/tool/tool.go`

---

## 10. References

- GitHub Issue: https://github.com/jrswab/axe/issues/7
- M7 Tool Spec (pattern reference): `docs/plans/018_run_command_spec.md`
- `RegisterAll` function: `internal/tool/registry.go:92`
- `ValidNames` function: `internal/toolname/toolname.go:18`
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

- **No path validation needed.** Unlike file tools, `url_fetch` operates on URLs, not file paths. The scheme validation (`http`/`https` only) is the security boundary, preventing `file://` access to the local filesystem.
- **`io.LimitReader` for memory safety.** Without a read limit, the LLM could instruct the tool to fetch a multi-gigabyte resource, causing OOM. `io.LimitReader` caps memory usage at `maxReadBytes+1` bytes regardless of response size. The extra byte is used to detect truncation without requiring a second read or `Content-Length` header inspection.
- **`http.DefaultClient` follows redirects.** Go's default client follows up to 10 redirects. This is desirable behavior — the LLM doesn't need to handle redirects manually. If a redirect loop occurs, `Do()` returns an error which the tool reports.
- **Byte-level truncation, not rune-aware.** Consistent with `run_command` (100KB byte truncation). For web content (predominantly ASCII/UTF-8), byte-level truncation rarely breaks in practice. The truncation notice clearly indicates truncation occurred.
- **Non-2xx body is included in error.** Many APIs return useful error information in the response body (JSON error objects, HTML error pages). Including the body (truncated) in the error result gives the LLM actionable information.
- **No `Workdir` usage.** Unlike file tools, `url_fetch` does not use `ExecContext.Workdir`. The tool operates on network URLs, not filesystem paths. `Workdir` is irrelevant.
- **The `RegisterAll` pattern continues.** This adds the sixth tool: `list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`, `url_fetch`.
- **Security model.** The `url_fetch` tool allows the LLM to make arbitrary HTTP GET requests to any `http://` or `https://` URL. This is by design — if an agent has `tools = ["url_fetch"]` in its TOML config, the user has explicitly opted in. Scheme validation prevents `file://` abuse. Network-level restrictions (firewalls, egress policies) are outside axe's scope.
