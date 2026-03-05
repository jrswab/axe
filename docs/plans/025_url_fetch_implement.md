# Implementation Checklist: `url_fetch` Built-in Tool

**Spec:** `docs/plans/025_url_fetch_spec.md`
**GitHub Issue:** https://github.com/jrswab/axe/issues/7
**Created:** 2026-03-05

---

## Phase 1: Toolname Constant & Validation (no dependencies)

- [x] Add `URLFetch = "url_fetch"` constant to `internal/toolname/toolname.go` const block (Req 1.1)
- [x] Add `URLFetch: true` entry to the `ValidNames()` map in `internal/toolname/toolname.go` (Req 1.2)
- [x] Update `TestValidNames_ReturnsExpectedCount` in `internal/toolname/toolname_test.go`: change expected count from 5 to 6 (Req 5.1)
- [x] Update `TestValidNames_ContainsAllExpectedNames` in `internal/toolname/toolname_test.go`: add `URLFetch` to the `expected` slice (Req 5.2)
- [x] Update `TestConstants_Values` in `internal/toolname/toolname_test.go`: add table entry `{"URLFetch", URLFetch, "url_fetch"}` (Req 5.3)
- [x] Run `make test` — verify `internal/toolname` tests pass and no regressions

## Phase 2: Tool Tests (RED phase — write failing tests first)

- [x] Create `internal/tool/url_fetch_test.go` with `package tool` declaration
- [x] Write `TestURLFetch_Success` — httptest server returns `"hello world"` status 200; verify `IsError` false, `Content` equals `"hello world"`, `CallID` matches (Req 3.12, Spec 7.2)
- [x] Write `TestURLFetch_EmptyURL` — empty `Arguments` map `{}`; verify `IsError` true, `Content` contains `"url is required"` (Req 3.3, Spec 7.2)
- [x] Write `TestURLFetch_MissingURLArgument` — `Arguments: {"url": ""}`; verify `IsError` true, `Content` contains `"url is required"` (Req 3.3, Spec 7.2)
- [x] Write `TestURLFetch_UnsupportedScheme_File` — `"file:///etc/passwd"`; verify `IsError` true, `Content` contains `"unsupported scheme"` and `"file"` (Req 3.5, Spec 7.2)
- [x] Write `TestURLFetch_UnsupportedScheme_FTP` — `"ftp://example.com/file"`; verify `IsError` true, `Content` contains `"unsupported scheme"` and `"ftp"` (Req 3.5, Spec 7.2)
- [x] Write `TestURLFetch_NoScheme` — `"example.com"`; verify `IsError` true, `Content` contains `"unsupported scheme"` (Req 3.5, Spec 7.2)
- [x] Write `TestURLFetch_Non2xxStatus_404` — httptest server returns `"not found"` status 404; verify `IsError` true, `Content` contains `"HTTP 404"` and `"not found"` (Req 3.11, Spec 7.2)
- [x] Write `TestURLFetch_Non2xxStatus_500` — httptest server returns `"internal error"` status 500; verify `IsError` true, `Content` contains `"HTTP 500"` and `"internal error"` (Req 3.11, Spec 7.2)
- [x] Write `TestURLFetch_LargeResponseTruncation` — httptest server returns 20000 bytes of `'A'`; verify `IsError` false, `Content` contains `"[response truncated, exceeded 10000 characters]"`, `len(Content)` > 10000 but < 20000 (Req 3.10, Spec 7.2)
- [x] Write `TestURLFetch_ExactLimitNotTruncated` — httptest server returns exactly 10000 bytes; verify `IsError` false, `Content` does NOT contain `"truncated"`, `len(Content)` equals 10000 (Req 3.10, Spec 7.2)
- [x] Write `TestURLFetch_ContextCancellation` — httptest server blocks with `time.Sleep(10s)`, context timeout 50ms; verify `IsError` true (Req 3.7, Spec 7.2)
- [x] Write `TestURLFetch_ConnectionRefused` — URL to closed port on 127.0.0.1; verify `IsError` true, `Content` non-empty (Req 3.7, Spec 7.2)
- [x] Write `TestURLFetch_CallIDPassthrough` — httptest server returns 200, call.ID `"uf-unique-42"`; verify `CallID` equals `"uf-unique-42"` (Spec 7.2)
- [x] Write `TestURLFetch_EmptyResponseBody` — httptest server returns 200 with empty body; verify `IsError` false, `Content` equals `""` (Req 3.12, Spec 7.2)
- [x] Write `TestURLFetch_Non2xxWithLargeBody` — httptest server returns 500 with > 10000 bytes; verify `IsError` true, `Content` contains `"HTTP 500"` and `"[response truncated, exceeded 10000 characters]"` (Req 3.10, 3.11, Spec 7.2)
- [x] Confirm all 15 tests compile but fail (RED state — `urlFetchEntry` does not exist yet)

## Phase 3: Tool Implementation (GREEN phase)

- [x] Create `internal/tool/url_fetch.go` with `package tool` declaration (Req 2.1)
- [x] Define package-level constant `maxReadBytes = 10000` (Req 3.9, Constraint 7)
- [x] Implement `urlFetchEntry() ToolEntry` returning `ToolEntry{Definition: urlFetchDefinition, Execute: urlFetchExecute}` (Req 2.2)
- [x] Implement `urlFetchDefinition() provider.Tool` with `Name: toolname.URLFetch`, description, and single `url` parameter (Type `"string"`, Required `true`) (Req 2.3, 2.4)
- [x] Implement `urlFetchExecute(ctx, call, ec) provider.ToolResult` with deferred `toolVerboseLog` (Req 3.1, 3.13):
  - [x] Extract `url` from `call.Arguments["url"]`; return error if empty (Req 3.2, 3.3)
  - [x] Parse URL via `net/url.Parse`; return error on failure (Req 3.4)
  - [x] Validate scheme is `"http"` or `"https"`; return `unsupported scheme %q: only http and https are allowed` on failure (Req 3.5)
  - [x] Create request via `http.NewRequestWithContext(ctx, "GET", urlStr, nil)`; return error on failure (Req 3.6)
  - [x] Execute via `http.DefaultClient.Do(req)`; return `err.Error()` on failure (Req 3.7)
  - [x] Defer `resp.Body.Close()` (Req 3.8)
  - [x] Read body via `io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))`; return error on failure (Req 3.9)
  - [x] Truncate if `len(body) > maxReadBytes`: slice to `maxReadBytes`, append `"\n... [response truncated, exceeded 10000 characters]"` (Req 3.10)
  - [x] If status < 200 or >= 300: return `ToolResult{IsError: true}` with `"HTTP %d: %s"` format (Req 3.11)
  - [x] If status 200-299: return `ToolResult{IsError: false}` with body string (Req 3.12)
- [x] Run `make test` — verify all 15 `url_fetch_test.go` tests pass (GREEN state)

## Phase 4: Registry Registration

- [x] Add `r.Register(toolname.URLFetch, urlFetchEntry())` to `RegisterAll()` in `internal/tool/registry.go` (Req 4.1)
- [x] Add `TestRegisterAll_RegistersURLFetch` to `internal/tool/registry_test.go`: verify `r.Has(toolname.URLFetch)` after `RegisterAll` (Spec 7.4)
- [x] Add `TestRegisterAll_ResolvesURLFetch` to `internal/tool/registry_test.go`: verify resolved tool has `Name == toolname.URLFetch` and `url` parameter (Spec 7.4)
- [x] Run `make test` — verify registry tests pass and no regressions

## Phase 5: Agent Scaffold Update

- [x] Update `Scaffold()` in `internal/agent/agent.go`: change tools comment from `# Valid: list_directory, read_file, write_file, edit_file, run_command` to `# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch` (Req 6.1)
- [x] Run `make test` — verify `internal/agent` tests pass and no regressions

## Phase 6: Full Validation

- [x] Run `make test` — all tests pass with zero failures
- [x] Verify `go build ./...` succeeds with no errors
- [x] Verify `go.mod` is unchanged (no new dependencies, Constraint 1)
- [x] Verify `cmd/run.go` is unchanged (Constraint 3)
- [x] Verify `internal/tool/tool.go` is unchanged (Constraint 3)
- [x] Verify `internal/provider/` is unchanged (Constraint 2)
