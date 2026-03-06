# Implementation Checklist: MCP (Model Context Protocol) Tool Support

**Spec:** `docs/plans/027_mcp_tool_support_spec.md`
**GitHub Issue:** https://github.com/jrswab/axe/issues/9
**Tracking:** Check boxes as tasks are completed across sessions.

---

## Phase 1: SDK Dependency & Agent Config (no new packages yet)

These tasks modify existing files only. No MCP logic runs yet.

- [x] **1.1** Add `github.com/modelcontextprotocol/go-sdk` dependency to `go.mod`. Pin to a specific release version. Run `go mod tidy` to update `go.sum`.
  - Files: `go.mod`, `go.sum`
  - Verify: `go build ./...` succeeds.

- [x] **1.2** Add `MCPServerConfig` struct to `internal/agent/agent.go` (after line ~50). Fields: `Name string`, `URL string`, `Transport string`, `Headers map[string]string` with TOML tags per Requirement 1.1.
  - Files: `internal/agent/agent.go`
  - Verify: `go build ./internal/agent/...` succeeds.

- [x] **1.3** Add `MCPServers []MCPServerConfig` field with tag `toml:"mcp_servers"` to `AgentConfig` struct at `internal/agent/agent.go:37-50`.
  - Files: `internal/agent/agent.go`
  - Verify: Existing `TestParseTOML_*` tests still pass. `make test` passes.

- [x] **1.4** Add MCP server validation logic to `Validate()` at `internal/agent/agent.go:55-84`. After existing field checks, iterate `MCPServers` and validate: non-empty `Name`, non-empty `URL`, `Transport` is `"sse"` or `"streamable-http"`, unique server names. Error messages per Requirements 2.2-2.3.
  - Files: `internal/agent/agent.go`
  - Verify: Existing tests pass. New tests (1.5) confirm behavior.

- [x] **1.5** Write agent config tests in `internal/agent/agent_test.go`:
  - `TestValidate_MCPServers_Valid` (Req 7.5)
  - `TestValidate_MCPServers_MissingName` (Req 7.5)
  - `TestValidate_MCPServers_MissingURL` (Req 7.5)
  - `TestValidate_MCPServers_InvalidTransport` (Req 7.5)
  - `TestValidate_MCPServers_ValidTransports` (Req 7.5)
  - `TestValidate_MCPServers_DuplicateNames` (Req 7.5)
  - `TestParseTOML_MCPServers` (Req 7.5)
  - `TestParseTOML_MCPServers_Empty` (Req 7.5)
  - Files: `internal/agent/agent_test.go`
  - Verify: `make test` passes. Red/green TDD: write tests first, confirm they fail, then confirm 1.4 makes them pass.

---

## Phase 2: Environment Variable Interpolation (`internal/envinterp/`)

New package, no integration yet. Fully testable in isolation.

- [x] **2.1** Create `internal/envinterp/envinterp.go`. Export `ExpandHeaders(headers map[string]string) (map[string]string, error)`. Implement regex-based `${VAR}` expansion per Requirements 3.1-3.7: nil input returns nil, empty map returns empty map, new map is returned (input unmodified), `os.Getenv` lookup, empty/unset vars return error.
  - Files: `internal/envinterp/envinterp.go` (NEW)
  - Verify: `go build ./internal/envinterp/...` succeeds.

- [x] **2.2** Write tests in `internal/envinterp/envinterp_test.go`. All 11 tests from Requirement 7.2:
  - `TestExpandHeaders_StaticValues`
  - `TestExpandHeaders_SingleVar`
  - `TestExpandHeaders_MultipleVarsInOneValue`
  - `TestExpandHeaders_MultipleHeaders`
  - `TestExpandHeaders_MissingVar`
  - `TestExpandHeaders_EmptyVar`
  - `TestExpandHeaders_NilMap`
  - `TestExpandHeaders_EmptyMap`
  - `TestExpandHeaders_NoPattern`
  - `TestExpandHeaders_DollarWithoutBrace`
  - `TestExpandHeaders_InputNotModified`
  - Files: `internal/envinterp/envinterp_test.go` (NEW)
  - Verify: `make test` passes. Red/green TDD.

---

## Phase 3: MCP Client (`internal/mcpclient/mcpclient.go`)

New package. Wraps the MCP SDK. Testable with httptest MCP servers.

- [x] **3.1** Create `internal/mcpclient/mcpclient.go`. Define `Client` struct holding server name and `*mcp.ClientSession`. Implement `Name() string` and `Close() error` methods per Requirements 4.2, 4.6, 4.7.
  - Files: `internal/mcpclient/mcpclient.go` (NEW)
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **3.2** Implement `Connect(ctx context.Context, cfg agent.MCPServerConfig) (*Client, error)` per Requirement 4.3. Steps: expand headers via `envinterp.ExpandHeaders`, create custom `RoundTripper` for header injection, create transport (`SSEClientTransport` or `StreamableClientTransport`), create `mcp.Client` with `{Name: "axe", Version: ...}`, call `client.Connect(ctx, transport)`, return wrapped `*Client`.
  - Files: `internal/mcpclient/mcpclient.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **3.3** Implement `(c *Client) ListTools(ctx context.Context) ([]provider.Tool, error)` per Requirement 4.4. Convert `mcp.Tool` to `provider.Tool` with type mapping: simple types (`string`, `integer`, `number`, `boolean`) map directly; complex types map to `"string"` with description prefix; missing type defaults to `"string"`; extract `required` array from top-level schema.
  - Files: `internal/mcpclient/mcpclient.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **3.4** Implement `(c *Client) CallTool(ctx context.Context, call provider.ToolCall) (provider.ToolResult, error)` per Requirement 4.5. Convert `map[string]string` to `map[string]any`, call `session.CallTool`, extract `TextContent` items, concatenate with `"\n"`, handle no-text and error cases.
  - Files: `internal/mcpclient/mcpclient.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **3.5** Write tests in `internal/mcpclient/mcpclient_test.go` using real MCP test servers (SDK server utilities or httptest JSON-RPC). All tests from Requirement 7.3:
  - `TestConnect_StreamableHTTP_Success`
  - `TestConnect_InvalidTransport`
  - `TestConnect_UnreachableServer`
  - `TestConnect_HeaderInjection`
  - `TestConnect_HeaderEnvVarMissing`
  - `TestListTools_ReturnsTools`
  - `TestListTools_EmptyTools`
  - `TestListTools_ComplexSchema`
  - `TestCallTool_Success`
  - `TestCallTool_ToolError`
  - `TestCallTool_NoTextContent`
  - `TestCallTool_MultipleTextContent`
  - `TestClientName`
  - Files: `internal/mcpclient/mcpclient_test.go` (NEW)
  - Verify: `make test` passes. Red/green TDD.

---

## Phase 4: MCP Tool Router (`internal/mcpclient/router.go`)

Same package as client. Testable with real MCP test servers.

- [x] **4.1** Create `internal/mcpclient/router.go`. Define `Router` struct mapping tool names to `*Client`. Implement `NewRouter() *Router` per Requirements 5.1-5.3.
  - Files: `internal/mcpclient/router.go` (NEW)
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **4.2** Implement `(r *Router) Register(client *Client, tools []provider.Tool, builtinNames map[string]bool) ([]provider.Tool, error)` per Requirement 5.4. Skip tools in `builtinNames`, error on MCP-to-MCP collision (with both server names in message), add remaining tools to map, return filtered slice.
  - Files: `internal/mcpclient/router.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **4.3** Implement `(r *Router) Dispatch(ctx, call) (provider.ToolResult, error)` per Requirement 5.5. Lookup client by `call.Name`, error if not found, delegate to `client.CallTool`.
  - Files: `internal/mcpclient/router.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **4.4** Implement `(r *Router) Has(name string) bool` per Requirement 5.6.
  - Files: `internal/mcpclient/router.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **4.5** Implement `(r *Router) Close() error` per Requirement 5.7. Close each unique client exactly once. Return first error.
  - Files: `internal/mcpclient/router.go`
  - Verify: `go build ./internal/mcpclient/...` succeeds.

- [x] **4.6** Write tests in `internal/mcpclient/router_test.go`. All tests from Requirement 7.4:
  - `TestRouter_RegisterAndDispatch`
  - `TestRouter_Has_UnknownTool`
  - `TestRouter_Dispatch_UnknownTool`
  - `TestRouter_Register_SkipsBuiltins`
  - `TestRouter_Register_MCPCollision`
  - `TestRouter_Close_ClosesAllClients`
  - `TestRouter_Close_DeduplicatesClients`
  - Files: `internal/mcpclient/router_test.go` (NEW)
  - Verify: `make test` passes. Red/green TDD.

---

## Phase 5: Integration into `cmd/run.go`

Wire MCP into the main run command. This is the largest integration phase.

- [x] **5.1** Add `MCPRouter` field (type `*mcpclient.Router`, or an interface if needed) to `ExecuteOptions` in `internal/tool/tool.go:26-35` per Requirement 6.3. This field allows `runConversationLoop` and `ExecuteCallAgent` to access the parent's MCP router.
  - Files: `internal/tool/tool.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

- [x] **5.2** Update `runConversationLoop()` in `internal/tool/tool.go:304-372` to check `opts.MCPRouter` before dispatching to `registry.Dispatch()`. If `opts.MCPRouter != nil && opts.MCPRouter.Has(tc.Name)`, dispatch via `opts.MCPRouter.Dispatch(ctx, tc)` per Requirement 6.2.
  - Files: `internal/tool/tool.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

- [x] **5.3** Update `ExecuteCallAgent()` in `internal/tool/tool.go:69-299` to handle sub-agent MCP per Requirement 6.4. When a sub-agent has its own `cfg.MCPServers`, establish separate MCP connections, create its own router, discover tools, append to `req.Tools`, and defer `router.Close()`. The parent's router is NOT passed to sub-agents.
  - Files: `internal/tool/tool.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

- [x] **5.4** Add MCP server handling in `cmd/run.go` after tool registry resolution (~line 257). Per Requirement 6.1: if `cfg.MCPServers` is non-empty, create router, defer close, iterate servers, connect, list tools, build `builtinNames` from `cfg.Tools`, register with collision checks, append filtered tools to `req.Tools`. Map errors to correct `ExitError` codes (2 for config/env errors, 3 for connection/API errors).
  - Files: `cmd/run.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

- [x] **5.5** Update `executeToolCalls()` in `cmd/run.go:540-606` to accept the router as a parameter per Requirement 6.2. Before falling through to `registry.Dispatch()`, check `router != nil && router.Has(tc.Name)` and dispatch via `router.Dispatch(ctx, tc)`.
  - Files: `cmd/run.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

- [x] **5.6** Pass the router through all call sites of `executeToolCalls()` and into `ExecuteOptions` where needed. Ensure the router flows correctly through the conversation loop for the parent agent only.
  - Files: `cmd/run.go`, `internal/tool/tool.go`
  - Verify: `go build ./...` succeeds. Existing tests pass.

---

## Phase 6: Verbose & Dry-Run Output

- [x] **6.1** Add verbose logging for MCP in `cmd/run.go` per Requirement 8.1. Log to stderr with `[mcp]` prefix: before connecting, after tool discovery, on built-in skip. Use the `verbose` flag and `stderr` writer already available.
  - Files: `cmd/run.go`
  - Verify: Manual verification with `--verbose` flag.

- [x] **6.2** Add verbose logging for MCP tool dispatch in `executeToolCalls()` per Requirement 8.1. Log `[mcp] Routing tool %q to server %q\n` before dispatching an MCP tool call.
  - Files: `cmd/run.go`
  - Verify: Manual verification with `--verbose` flag.

- [x] **6.3** Update `printDryRun()` in `cmd/run.go:457-536` to include `--- MCP Servers ---` section per Requirements 7.1-7.4. Place after `--- Tools ---` and before `--- Sub-Agents ---`. Print each server as `<name>: <url> (<transport>)` or `(none)` if empty. No MCP connections during dry-run.
  - Files: `cmd/run.go`
  - Verify: `make test` passes with new dry-run tests (6.4).

- [x] **6.4** Write dry-run tests in `cmd/run_test.go` per Requirement 7.6:
  - `TestRunCmd_DryRun_WithMCPServers` — fixture agent with MCP config, verify output contains `--- MCP Servers ---`, server name, URL, transport.
  - `TestRunCmd_DryRun_NoMCPServers` — existing fixture, verify output contains `--- MCP Servers ---` followed by `(none)`.
  - Create fixture TOML files in `cmd/testdata/agents/` as needed.
  - Files: `cmd/run_test.go`, `cmd/testdata/agents/*.toml` (new fixtures)
  - Verify: `make test` passes. Red/green TDD.

---

## Phase 7: Scaffold Update

- [x] **7.1** Update `Scaffold()` in `internal/agent/agent.go:157-199` to include a commented-out MCP servers example per Requirement 9.1. Place after the `sub_agents_config` section and before the `[memory]` section. Use the exact TOML comment block from the spec.
  - Files: `internal/agent/agent.go`
  - Verify: `make test` passes. Manual verification of `axe new` output.

---

## Phase 8: JSON Output Verification

- [x] **8.1** Verify MCP tool calls appear in `tool_call_details` array in `--json` output per Requirements 10.1-10.2. MCP tool calls use the same structure as built-in tool calls (`name`, `input`, `output`, `is_error`). The `tool_calls` count includes MCP calls. No new fields needed in the JSON envelope — confirm this works via the existing dispatch path.
  - Files: No changes expected (MCP calls go through the same `ToolResult` path). If any changes needed, update `cmd/run.go`.
  - Verify: Integration test or manual verification with `--json` flag against a real/test MCP server.

---

## Phase 9: Final Validation

- [x] **9.1** Run `make test` and confirm all existing tests pass unchanged. All new tests pass. No regressions.
  - Verify: `make test` exits 0.

- [x] **9.2** Confirm no changes to `internal/provider/provider.go` (Constraint 2).

- [x] **9.3** Confirm no changes to `internal/toolname/toolname.go` (Constraint 3).

- [x] **9.4** Confirm only one new external dependency: `github.com/modelcontextprotocol/go-sdk` (Constraint 1). No other new dependencies.

- [x] **9.5** Manual smoke test: create an agent TOML with `[[mcp_servers]]` pointing to a real or local MCP server. Run agent. Verify tools are discovered, tool calls are dispatched, results are returned. Verify `--dry-run`, `--verbose`, and `--json` output.

---

## Dependency Order

```
Phase 1 (config)
  |
Phase 2 (envinterp) ---- independent of Phase 1, but blocks Phase 3
  |
Phase 3 (mcpclient) ---- depends on Phase 1 (MCPServerConfig) and Phase 2 (envinterp)
  |
Phase 4 (router) -------- depends on Phase 3 (Client)
  |
Phase 5 (integration) --- depends on Phase 4 (Router) and Phase 1 (config)
  |
Phase 6 (output) -------- depends on Phase 5 (integration wiring)
  |
Phase 7 (scaffold) ------ independent, can run anytime after Phase 1
  |
Phase 8 (json verify) --- depends on Phase 5
  |
Phase 9 (final) --------- depends on all phases
```

## Key File Reference

| File | Status | Key Lines |
|------|--------|-----------|
| `go.mod` | MODIFY | Add MCP SDK dependency |
| `internal/agent/agent.go` | MODIFY | `AgentConfig` (L37), `Validate()` (L55), `Scaffold()` (L157) |
| `internal/agent/agent_test.go` | MODIFY | Add 8 new test functions |
| `internal/envinterp/envinterp.go` | NEW | `ExpandHeaders()` |
| `internal/envinterp/envinterp_test.go` | NEW | 11 test functions |
| `internal/mcpclient/mcpclient.go` | NEW | `Client`, `Connect`, `ListTools`, `CallTool`, `Name`, `Close` |
| `internal/mcpclient/mcpclient_test.go` | NEW | 13 test functions |
| `internal/mcpclient/router.go` | NEW | `Router`, `NewRouter`, `Register`, `Dispatch`, `Has`, `Close` |
| `internal/mcpclient/router_test.go` | NEW | 7 test functions |
| `internal/tool/tool.go` | MODIFY | `ExecuteOptions` (L26), `runConversationLoop` (L304), `ExecuteCallAgent` (L69) |
| `cmd/run.go` | MODIFY | MCP connect/discover (~L257), `executeToolCalls` (L540), `printDryRun` (L457) |
| `cmd/run_test.go` | MODIFY | 2 new dry-run test functions |
| `cmd/testdata/agents/` | MODIFY | New fixture TOML files with MCP config |
