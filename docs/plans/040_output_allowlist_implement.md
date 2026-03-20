# 040 — Output Allowlist Implementation Guide

Spec: `docs/plans/040_output_allowlist_spec.md`

---

## Section 1: Context Summary

The `url_fetch` built-in tool lets the LLM make HTTP GET requests to arbitrary URLs, creating a prompt injection → SSRF attack surface. There is currently no mechanism to restrict which hosts the LLM may contact. This implementation adds a per-agent `allowed_hosts` TOML field that gates `url_fetch` requests by hostname, plus unconditional private IP blocking that prevents `url_fetch` from ever contacting loopback, RFC 1918, link-local, or CGNAT addresses — regardless of allowlist configuration. Sub-agents inherit the parent's allowlist as a fallback unless they define their own. All host validation logic lives in a new `internal/hostcheck/` package.

---

## Section 2: Implementation Checklist

### Phase 1 — Foundation (parallelizable: tasks 1–3 have no dependencies on each other)

- [x] **Task 1a: Create `internal/hostcheck/hostcheck.go`**
  - New package `hostcheck` with three exported functions:
    - `IsAllowed(hostname string, allowlist []string) bool` — returns `true` if `allowlist` is empty (allow-all mode) or if `hostname` matches any entry (case-insensitive). Matching is exact — no wildcards, no subdomain matching.
    - `IsPrivateIP(ip net.IP) bool` — returns `true` if `ip` falls within any of: `127.0.0.0/8`, `::1/128`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16`, `fe80::/10`, `0.0.0.0/8`, `100.64.0.0/10`. Define the CIDR ranges as package-level `net.IPNet` values parsed once at init.
    - `CheckHost(ctx context.Context, hostname string, allowlist []string, resolver Resolver) error` — the combined check used by `url_fetch`. Steps: (1) if `allowlist` is non-empty and `hostname` is not in it, return an error naming the blocked host. (2) If `hostname` is a raw IP literal, parse it and check `IsPrivateIP`; return an error naming the blocked IP if private. (3) Otherwise, resolve `hostname` via `resolver.LookupIPAddr(ctx, hostname)`. (4) If any resolved IP is private, return an error naming the blocked IP. (5) Return `nil` (allowed). The `Resolver` is an interface: `type Resolver interface { LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) }` — this allows tests to inject a fake resolver without real DNS.
  - Error messages must be actionable (spec R8): `"host %q is not in allowed_hosts"` for allowlist violations, `"host %q resolves to private address %s"` for private IP violations.

- [x] **Task 1b: Create `internal/hostcheck/hostcheck_test.go`**
  - Tests for `IsAllowed`:
    - Empty allowlist returns `true` for any hostname.
    - Non-empty allowlist: exact match returns `true`, non-match returns `false`.
    - Case-insensitive: `"API.Example.COM"` matches allowlist entry `"api.example.com"`.
    - Subdomain does not match: `"sub.api.example.com"` does not match `"api.example.com"`.
    - Parent domain does not match: `"example.com"` does not match `"api.example.com"`.
  - Tests for `IsPrivateIP`:
    - Each blocked range has at least one positive test case: `127.0.0.1`, `::1`, `10.0.0.1`, `172.16.0.1`, `192.168.1.1`, `169.254.1.1`, `fe80::1`, `0.0.0.1`, `100.64.0.1`.
    - Public IPs return `false`: `8.8.8.8`, `2001:4860:4860::8888`.
    - Boundary cases: `172.15.255.255` (not private), `172.16.0.0` (private), `172.31.255.255` (private), `172.32.0.0` (not private).
  - Tests for `CheckHost`:
    - Use a fake `Resolver` that returns controlled IPs.
    - Allowlist non-empty, hostname not in list → error.
    - Allowlist non-empty, hostname in list, resolves to public IP → nil.
    - Allowlist non-empty, hostname in list, resolves to private IP → error (spec E2).
    - Allowlist empty, resolves to public IP → nil.
    - Allowlist empty, resolves to private IP → error.
    - Raw IP literal in hostname (e.g. `"192.168.1.1"`) → private IP error, no DNS lookup.
    - Raw public IP literal (e.g. `"8.8.8.8"`) with empty allowlist → nil.
    - Raw public IP literal with non-empty allowlist → error (IPs don't match hostname entries, spec E3).
    - Hostname resolves to mixed public + private IPs → error (spec E1: any private = blocked).
    - DNS resolution error → return the DNS error (not silently allow).

- [x] **Task 2: Add `AllowedHosts` to `AgentConfig` — `internal/agent/agent.go: AgentConfig` struct (line 68)**
  - Add field `AllowedHosts []string \`toml:"allowed_hosts"\`` to the `AgentConfig` struct. Place it after the `Tools` field (line 76) to group network-related config together.
  - No changes to `Validate()` — spec R12 says entries are not validated at load time.

- [x] **Task 2b: Test TOML decode of `AllowedHosts` — `internal/agent/agent_test.go`**
  - Add a test that decodes a TOML string containing `allowed_hosts = ["api.example.com", "docs.example.com"]` and asserts `cfg.AllowedHosts` equals `[]string{"api.example.com", "docs.example.com"}`.
  - Add a test that decodes a TOML string with no `allowed_hosts` field and asserts `cfg.AllowedHosts` is `nil`.
  - Add a test that decodes `allowed_hosts = []` and asserts `cfg.AllowedHosts` is empty (length 0).
  - Follow the existing pattern: raw TOML string → `tomlDecode(input, &cfg)` → assert fields.

- [x] **Task 3: Add `AllowedHosts` to `ExecContext` — `internal/tool/registry.go: ExecContext` struct (line 13)**
  - Add field `AllowedHosts []string` to the `ExecContext` struct.

### Phase 2 — Wiring (tasks 4, 5, 6 depend on Phase 1; tasks 5 and 6 are parallelizable with each other)

- [x] **Task 4: Gate `url_fetch` — `internal/tool/url_fetch.go: urlFetchExecute()` (line 97)**
  - After the existing scheme check (line 127) and before the HTTP request is built (line 132), add a call to `hostcheck.CheckHost(ctx, parsedURL.Hostname(), ec.AllowedHosts, net.DefaultResolver)`.
  - If `CheckHost` returns an error, return `provider.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}` immediately. Do not make the HTTP request.
  - Replace `http.DefaultClient.Do(req)` with a request that pins to a resolved IP to prevent DNS rebinding between the check and the connection. Do this by creating a custom `http.Transport` with a `DialContext` that connects to a known-safe resolved IP (the first public IP returned by `CheckHost`). To support this, change `CheckHost` to return the safe IP alongside the nil error: `CheckHost(...) (net.IP, error)` — returns the first non-private resolved IP on success, or `nil, error` on failure. When the hostname is a raw public IP literal, return that IP directly.
  - For redirect safety (spec E9): set `CheckRedirect` on the per-request `http.Client` to validate each redirect URL against the same allowlist and private IP check before following it. If a redirect target fails the check, return `http.ErrUseLastResponse` to stop following.

- [x] **Task 4b: Tests for `url_fetch` allowlist enforcement — `internal/tool/url_fetch_test.go`**
  - Follow the existing test pattern: `httptest.NewServer` + direct `urlFetchEntry().Execute()` calls.
  - Test: allowlist configured, URL hostname matches → request succeeds (200 from test server).
  - Test: allowlist configured, URL hostname does not match → `ToolResult.IsError == true`, content mentions "not in allowed_hosts".
  - Test: no allowlist (empty), public test server → request succeeds.
  - Test: private IP blocking — URL with `127.0.0.1` → blocked regardless of allowlist. (Use a raw IP URL; no real connection attempted.)
  - Test: redirect from allowed host to disallowed host → blocked. (Use `httptest.NewServer` that returns a 301 to a different test server whose hostname is not in the allowlist.)
  - Note: testing DNS resolution behavior requires the fake `Resolver` from `hostcheck`. Inject it by making the resolver a field on `ExecContext` or by adding a package-level `var` (like `urlFetchTimeout`) that tests can override. Choose whichever pattern is already established — the existing `urlFetchTimeout` var override pattern (line 21) is the precedent.

- [x] **Task 5: Thread `AllowedHosts` into `ExecContext` at top level — `cmd/run.go: dispatchToolCall()` (line 745)**
  - In `cmd/run.go: dispatchToolCall()` (line 759), add `AllowedHosts` to the `tool.ExecContext` literal. The value comes from the `cfg.AllowedHosts` field. This requires adding `allowedHosts []string` as a parameter to `dispatchToolCall()`.
  - Update the `dispatchToolCall()` call site in `executeToolCalls()` (line 687) to pass `cfg.AllowedHosts` through.
  - Update the `executeToolCalls()` signature and its call site at line 488 to accept and forward `cfg.AllowedHosts`.

- [x] **Task 6a: Add `AllowedHosts` to `ExecuteOptions` — `internal/tool/tool.go: ExecuteOptions` struct (line 28)**
  - Add field `AllowedHosts []string` to `ExecuteOptions`.

- [x] **Task 6b: Propagate `AllowedHosts` in `ExecuteCallAgent()` — `internal/tool/tool.go: ExecuteCallAgent()` (line 75)**
  - After loading the sub-agent's TOML config (`cfg` at line 137), compute the effective allowlist:
    ```
    effectiveAllowedHosts = cfg.AllowedHosts          // sub-agent's own list
    if len(effectiveAllowedHosts) == 0 {
        effectiveAllowedHosts = opts.AllowedHosts      // inherit parent's list
    }
    ```
  - Pass `effectiveAllowedHosts` into the `ExecContext` built at line 438 (inside `dispatchToolCall` in tool.go) — this requires threading it through `runConversationLoop` and into `dispatchToolCall`.
  - Pass `effectiveAllowedHosts` into the recursive `ExecuteOptions` built at lines 398–411 (inside `runConversationLoop`) so sub-sub-agents inherit correctly.

- [x] **Task 6c: Thread `AllowedHosts` through `runConversationLoop()` — `internal/tool/tool.go: runConversationLoop()` (line 347)**
  - Add `allowedHosts []string` parameter to `runConversationLoop()` signature.
  - In `dispatchToolCall()` (line 429), pass `allowedHosts` into the `ExecContext` at line 438.
  - In the recursive `subOpts` (lines 398–411), set `AllowedHosts: allowedHosts`.
  - Update the call site of `runConversationLoop` inside `ExecuteCallAgent()` (line 306) to pass `effectiveAllowedHosts`.

- [x] **Task 6d: Set `AllowedHosts` on `ExecuteOptions` in `cmd/run.go` — `cmd/run.go: executeToolCalls()` (line 690)**
  - In the `tool.ExecuteOptions` literal at lines 690–703, add `AllowedHosts: cfg.AllowedHosts`.

### Phase 3 — Integration (depends on all of Phase 2)

- [x] **Task 7: Integration tests — `cmd/`**
  - Add integration tests that compile the binary and run end-to-end, following the pattern in `cmd/run_integration_test.go`.
  - Test cases:
    - Agent with `allowed_hosts = ["httpbin.org"]` and `url_fetch` tool: LLM asked to fetch an allowed URL → succeeds.
    - Agent with `allowed_hosts = ["httpbin.org"]` and `url_fetch` tool: LLM asked to fetch a disallowed URL → tool returns error.
    - Agent with no `allowed_hosts`: LLM asked to fetch a public URL → succeeds (backwards compatible).
    - Private IP blocking: agent asked to fetch `http://127.0.0.1` → blocked regardless of allowlist.
    - Sub-agent inheritance: parent has `allowed_hosts`, sub-agent has none → sub-agent inherits parent's list.
    - Sub-agent override: parent has `allowed_hosts`, sub-agent defines its own → sub-agent uses its own list.
  - Note: integration tests that involve actual LLM calls may need to use the mock server from `internal/testutil/mockserver.go`. Design the test so the LLM response includes a `url_fetch` tool call to a `httptest.NewServer`, and verify the allowlist/block behavior on the tool execution side.

### Parallel Work Map

```
Phase 1 (all parallel):
  Task 1a + 1b ─┐
  Task 2  + 2b ─┤
  Task 3  ──────┘
                 │
Phase 2:         ▼
  Task 4 + 4b ──── (depends on 1a, 1b, 3)
  Task 5 ───────── (depends on 2, 3)      ← parallel with Task 6
  Task 6a–6d ───── (depends on 2, 3)      ← parallel with Task 5
                 │
Phase 3:         ▼
  Task 7 ───────── (depends on 4, 5, 6)
```
