# 040 -- Output Allowlist Spec

# Milestone Document
docs/plans/000_milestones.md

GitHub Issue: https://github.com/jrswab/axe/issues/24

---

## Section 1: Context & Constraints

### Problem

The `url_fetch` built-in tool allows the LLM to make outgoing HTTP GET requests to arbitrary URLs. This creates a prompt injection → SSRF (Server-Side Request Forgery) attack surface: a malicious document or web page can instruct the LLM to fetch internal resources, cloud metadata endpoints, or other unintended targets.

There is currently no mechanism to restrict which hosts the LLM may contact via `url_fetch`. Operators have no way to express intent like "this agent should only be allowed to fetch from api.example.com" — or to harden an agent against fetching private network addresses even if no explicit allowlist is configured.

### Codebase Structure Relevant to This Milestone

**Agent configuration** (`internal/agent/agent.go`):

The `AgentConfig` struct is decoded directly from TOML. It holds all per-agent settings. The `Validate()` function performs sequential field validation after load. This is the source of truth for an agent's configuration at runtime.

**Tool execution context** (`internal/tool/registry.go`):

The `ExecContext` struct is the cross-cutting data bag passed to every tool executor at call time. It currently carries workdir, stderr writer, and verbosity. It is the correct mechanism for threading per-agent runtime configuration into tools without coupling tool implementations to the agent loader.

**`url_fetch` tool** (`internal/tool/url_fetch.go`):

Makes outgoing HTTP GET requests on behalf of the LLM. It already enforces a scheme check (only `http`/`https` allowed). It uses `http.DefaultClient` for the actual request. There is no host validation today. This is the only tool that must be gated by this feature.

**Top-level run** (`cmd/run.go`):

Builds the `ExecContext` for the root agent's tool dispatch. This is where agent-level configuration is threaded into the tool execution layer.

**Sub-agent delegation** (`internal/tool/tool.go`):

The `ExecuteOptions` struct is the options bag passed from a parent agent to a sub-agent invocation. The `ExecuteCallAgent` function loads a sub-agent's TOML, constructs its execution context, and runs it. The `runConversationLoop` function also builds an `ExecContext` for sub-agent tool dispatch.

**New package**: `internal/hostcheck/`

All host validation logic — allowlist matching, private IP blocking, and DNS resolution — must live in a single dedicated package. This keeps `url_fetch.go` clean, makes the logic independently testable, and provides a single authoritative place to audit security behavior.

### Decisions Already Made

**1. Default: allow-all when no `allowed_hosts` is configured.**

An empty `allowed_hosts` field means no restriction is applied. This preserves backwards compatibility for all existing agents that do not define the field. Operators must explicitly opt in to restriction.

**2. Only `url_fetch` is gated.**

`web_search` connects to a fixed Tavily API base URL that is user- or env-controlled, not LLM-controlled. MCP HTTP connections are user-configured in TOML at load time. LLM provider connections are also user-configured. None of these are driven by LLM output at request time, so they are not part of the prompt-injection attack surface and are out of scope.

**3. Private IP blocking is unconditional and separate from the allowlist.**

Regardless of whether `allowed_hosts` is configured, `url_fetch` must never contact private, loopback, or link-local addresses. This is a hardening baseline, not an allowlist feature. The two concerns (allowlist filtering and private IP blocking) are independent: a host can pass the allowlist check but still be blocked if it resolves to a private IP.

The following IP ranges are always blocked:

| Range | Classification |
|---|---|
| `127.0.0.0/8` | IPv4 loopback |
| `::1/128` | IPv6 loopback |
| `10.0.0.0/8` | RFC 1918 private |
| `172.16.0.0/12` | RFC 1918 private |
| `192.168.0.0/16` | RFC 1918 private |
| `169.254.0.0/16` | IPv4 link-local / AWS IMDS |
| `fe80::/10` | IPv6 link-local |
| `0.0.0.0/8` | This network |
| `100.64.0.0/10` | CGNAT / shared address space |

**4. Private IP check must happen after DNS resolution.**

Checking only the hostname is insufficient — a DNS rebinding attack could resolve an allowed hostname to a private IP. The hostname must be resolved to its IP address(es), and each resolved IP must be checked against the blocked ranges before any connection is made.

**5. Allowlist matching is hostname/domain-only in v1.**

CIDR notation for allowlist entries (e.g., `10.0.1.0/24`) is deferred to a future milestone. v1 allowlist entries are strings matched against the request hostname only.

**6. Sub-agent propagation uses inheritance with explicit override.**

Each agent uses its own `allowed_hosts` from its own TOML. If a sub-agent's TOML does NOT define `allowed_hosts`, it inherits the parent's list as a fallback. If a sub-agent's TOML DOES define its own `allowed_hosts`, that list is used — the user explicitly configured that agent's permissions. This rule applies recursively through the sub-agent tree.

This behavior means:
- A sub-agent can be more restrictive than its parent (define a subset of hosts).
- A sub-agent can be less restrictive than its parent (define a wider or different set of hosts), because it is an explicit operator decision in the sub-agent's own TOML.
- A sub-agent with no opinion inherits the parent's restrictions.

### Approaches Ruled Out

**Union (permissive merge)**: Combining parent and sub-agent lists would allow a sub-agent to widen the parent's restrictions. This is a security regression — a compromised or misconfigured sub-agent TOML could silently expand access beyond what the parent operator intended.

**Parent always overrides sub-agent**: Ignoring the sub-agent's `allowed_hosts` when the parent has one silently discards intentional operator configuration. If an operator explicitly wrote `allowed_hosts` into a sub-agent's TOML, that decision must be respected.

**Gating `web_search`**: The Tavily API base URL is configured by the user or environment, not by the LLM at request time. It is not an LLM-controlled attack surface and is out of scope.

**Gating MCP server connections**: MCP server URLs are defined in agent TOML by the operator. They are user-configured at load time, not LLM-controlled. Out of scope.

**Gating LLM provider connections**: Provider base URLs are user- or env-configured. Not LLM-controlled. Out of scope.

### Constraints and Assumptions

- The feature must be backwards compatible. Agents that do not define `allowed_hosts` must behave exactly as they do today.
- All validation logic for host access is centralized in `internal/hostcheck/` and not duplicated elsewhere.
- Private IP blocking applies universally — it cannot be disabled by any configuration.
- Sub-agent propagation is recursive. The same inheritance-with-explicit-override rule applies at every depth level.
- Allowlist matching in v1 is exact hostname matching only. No wildcard subdomain support, no CIDR, no regex.
- The private IP check is performed on all resolved IP addresses for a hostname. If any resolved address is in a blocked range, the request is rejected.
- DNS resolution for the purpose of private IP checking is done by the host validation layer, not by the HTTP client. The connection must be made to a known-safe resolved address.

---

## Section 2: Requirements

### R1: New `allowed_hosts` field in agent configuration

The agent configuration must accept an optional `allowed_hosts` field containing a list of hostname strings. When the field is absent or empty, no host restriction is applied to `url_fetch` requests (allow-all).

### R2: Host validation is a distinct, independently testable unit

All logic for determining whether a URL is allowed to be fetched — allowlist matching and private IP blocking — must be encapsulated in a single package. This package must be testable in isolation from the HTTP stack and from the tool registry.

### R3: Private IP ranges are always blocked

`url_fetch` must never successfully complete a request to a URL that resolves to any address in the private, loopback, link-local, CGNAT, or this-network IP ranges listed in Section 1. This applies whether or not `allowed_hosts` is configured. It is not configurable.

### R4: Private IP check is performed after DNS resolution

The hostname in the request URL must be resolved to its IP address(es) before any connection is made. Each resolved IP address must be checked against the blocked ranges. If any resolved address falls within a blocked range, the request must be rejected. This prevents DNS rebinding attacks.

### R5: Allowlist check behavior when `allowed_hosts` is configured

When `allowed_hosts` is non-empty, `url_fetch` must reject any request whose URL hostname does not exactly match one of the entries in `allowed_hosts`. The allowlist check occurs before the private IP check; both must pass for a request to proceed.

### R6: Allowlist check behavior when `allowed_hosts` is empty

When `allowed_hosts` is empty (the default), no allowlist filtering is applied. Only the unconditional private IP check (R3) is enforced.

### R7: Hostname matching is exact in v1

An allowlist entry of `api.example.com` permits requests to `api.example.com` and to nothing else. It does not permit `sub.api.example.com`, `example.com`, or `www.example.com`. Matching is case-insensitive (hostnames are not case-sensitive by specification), but no wildcard or prefix/suffix logic is applied.

### R8: Rejection produces a clear, actionable error message

When a `url_fetch` request is blocked — whether by the private IP check or the allowlist check — the error returned to the LLM must indicate which rule was violated and which host/IP triggered it. The LLM must have enough information to understand that the request was intentionally blocked, not that a network error occurred.

### R9: `allowed_hosts` is threaded through the execution context

The effective allowlist for an agent must be available to `url_fetch` at call time via the same execution context mechanism used by all other per-agent tool configuration. No global state. No package-level variables.

### R10: Sub-agent inheritance rule

When a sub-agent is invoked:

- If the sub-agent's TOML defines a non-empty `allowed_hosts`, that list is used as the sub-agent's effective allowlist.
- If the sub-agent's TOML does NOT define `allowed_hosts` (empty), the parent agent's effective allowlist is inherited and used as the sub-agent's effective allowlist.
- If both are empty, the sub-agent operates with no host restriction (allow-all), subject only to R3.

This rule is applied recursively at every level of the sub-agent tree.

### R11: Sub-agent propagation for recursive depth

The effective allowlist used by a sub-agent must also be propagated as the "parent allowlist" to any sub-sub-agents that the sub-agent may invoke. This ensures the inheritance rule applies uniformly at all depths without requiring special handling at each level.

### R12: No validation of `allowed_hosts` entries at load time

Entries in `allowed_hosts` are not validated for syntax, resolvability, or reachability when the agent TOML is loaded. Any string is accepted. Invalid entries (e.g., a malformed hostname) will simply never match any request hostname and are silently ineffective. This avoids failing agent startup due to an unreachable host.

### R13: `web_search`, MCP connections, and provider connections are unaffected

This feature must not alter the behavior of `web_search`, MCP server connections, or LLM provider connections in any way. These are explicitly out of scope.

---

## Edge Cases

### E1: Hostname resolves to both public and private IPs

If DNS returns multiple addresses for a hostname (e.g., one public and one private), the request must be blocked if any resolved address is in a blocked range. A single private address in the response set is sufficient cause for rejection.

### E2: `allowed_hosts` contains an entry that resolves to a private IP

An operator could misconfigure `allowed_hosts` with a hostname that itself resolves to a private address (e.g., `internal.corp.example.com`). The private IP check (R3) applies unconditionally and takes effect after the allowlist check passes. The request is blocked even though the hostname is on the allowlist. This is the correct and intended behavior — the private IP check is a safety rail, not an allowlist bypass.

### E3: URL uses an IP address directly instead of a hostname

If the LLM provides a URL with a raw IP address (e.g., `http://192.168.1.1/data`), the private IP check applies to that IP directly (no DNS resolution needed). If the IP is in a blocked range, the request is rejected. If `allowed_hosts` is configured, a raw IP address in the URL will not match any hostname entry and the request is also rejected by the allowlist check.

### E4: URL uses `localhost` or similar aliases

`localhost` resolves to `127.0.0.1` (or `::1`), both of which are in blocked ranges. The private IP check catches this after resolution. No special casing for `localhost`, `0.0.0.0`, or other hostname aliases is required — DNS resolution handles them uniformly.

### E5: Sub-agent defines an empty list explicitly vs. not defining the field at all

An omitted `allowed_hosts` field (nil at load time) causes the sub-agent to inherit the parent's list. An explicit `allowed_hosts = []` (non-nil, zero length) clears the parent's restrictions — the sub-agent operates with no host restriction, subject only to R3 (unconditional private IP blocking). This distinction allows operators to explicitly opt a sub-agent out of inherited restrictions when needed.

### E6: Root agent has no `allowed_hosts`; sub-agent has no `allowed_hosts`

Both inherit "allow-all." The only enforcement is the unconditional private IP block (R3). This is the backwards-compatible default.

### E7: Root agent has `allowed_hosts`; sub-agent is invoked with `call_agent` but no tool calls

The sub-agent runs, produces text output, and returns. No `url_fetch` calls are made. The allowlist is threaded through but never evaluated. No behavioral difference from today.

### E8: Allowlist entry with a port

A `url_fetch` URL may include a port (e.g., `https://api.example.com:8443/endpoint`). The allowlist entry is matched against the hostname only (`api.example.com`), not the host+port combination. An entry of `api.example.com` permits requests to that host on any port, provided the scheme check and private IP check also pass.

### E9: Redirect following

If the HTTP client follows a redirect (e.g., HTTP 301 from `https://allowed.example.com` to `https://other.example.com`), the redirected URL must also pass both the allowlist check and the private IP check before the redirected request is made. A redirect to a non-allowlisted host must be blocked.

---

## Parallel Work Structure

The following tasks have no dependencies on each other and may be implemented concurrently:

- **Task 1**: Create `internal/hostcheck/` package with full test coverage (TDD). This package owns allowlist matching, private IP range definitions, DNS resolution, and the combined check used by `url_fetch`.
- **Task 2**: Add `AllowedHosts []string` to `AgentConfig` with TOML tag and associated tests.
- **Task 3**: Add `AllowedHosts []string` to `ExecContext`.

The following tasks have dependencies:

- **Task 4** (depends on Tasks 1 and 3): Gate `url_fetch` using `hostcheck`, performing DNS-resolved private IP checks and allowlist checks before making the HTTP request. Includes tests for blocked and allowed cases.
- **Task 5** (depends on Tasks 2 and 3): Thread `cfg.AllowedHosts` from the loaded agent config into `ExecContext` in the root agent run path.
- **Task 6** (depends on Tasks 2 and 3): Add `AllowedHosts []string` to `ExecuteOptions`. Implement sub-agent propagation logic in `ExecuteCallAgent` and `runConversationLoop`.
- **Task 7** (depends on Tasks 4, 5, and 6): Integration test covering: allowlist enforcement end-to-end, private IP blocking, sub-agent inheritance, and sub-agent explicit override.

---

## Post-Implementation Refinements

The following changes were made after the initial implementation to improve testability and code quality. These do not alter any production behavior or security properties.

### R14: Sub-agent allowlist inheritance is an exported, testable function

The nil-vs-empty inheritance logic for `allowed_hosts` propagation (described in R10 and R11) is encapsulated in the exported function `EffectiveAllowedHosts(subAgent, parent []string) []string` in `internal/tool/tool.go`. `ExecuteCallAgent` calls this function rather than implementing the logic inline. This ensures the inheritance rule is tested directly against the production code path, not a duplicated copy.

### R15: `url_fetch` host validation dependencies are injected, not global

The `url_fetch` tool executor uses a `urlFetcher` struct (unexported, in `internal/tool/url_fetch.go`) that holds its DNS resolver, host-check function, and HTTP timeout as instance fields. The production constructor `newURLFetcher()` wires the same defaults previously held by package-level variables (`net.DefaultResolver`, `hostcheck.CheckHost`, `15s`). Tests construct per-test instances with injected fakes, eliminating mutable package-level state and enabling safe parallel test execution.

### Affected files

| File | Change |
|---|---|
| `internal/tool/tool.go` | Extracted `EffectiveAllowedHosts()` function; `ExecuteCallAgent` calls it |
| `internal/tool/tool_test.go` | `TestEffectiveAllowedHosts` calls production function; uses `reflect.DeepEqual` for nil-aware slice comparison |
| `internal/tool/url_fetch.go` | Replaced 3 mutable `var` declarations with `urlFetcher` struct and `newURLFetcher()` constructor |
| `internal/tool/url_fetch_test.go` | Removed `skipHostCheck()` global-mutation helper; all tests use per-instance `urlFetcher` |
