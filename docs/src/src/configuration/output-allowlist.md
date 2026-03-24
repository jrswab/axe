# Output Allowlist

Agents that use `url_fetch` or `web_search` can be restricted to specific hostnames:

```toml
allowed_hosts = ["api.example.com", "docs.example.com"]
```

| Behavior | Detail |
|---|---|
| Empty or absent | All public hostnames allowed |
| Non-empty list | Only exact hostname matches permitted (case-insensitive, no wildcard subdomains) |
| Private IPs | Always blocked regardless of allowlist — loopback, link-local, RFC 1918, CGNAT, IPv6 private |
| Redirects | Each redirect destination is re-validated against the allowlist and private IP check |
| Sub-agents | Inherit the parent's `allowed_hosts` unless the sub-agent TOML explicitly sets its own |
