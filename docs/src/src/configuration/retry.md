# Retry

Agents can retry on transient LLM provider errors — rate limits (429), server errors (5xx), and timeouts. Retry is opt-in and disabled by default.

| Field | Default | Description |
|---|---|---|
| `max_retries` | 0 | Number of retry attempts after the initial request. 0 disables retry. |
| `backoff` | `"exponential"` | Strategy: `"exponential"` (with jitter), `"linear"`, or `"fixed"` |
| `initial_delay_ms` | 500 | Base delay in milliseconds before the first retry |
| `max_delay_ms` | 30000 | Maximum delay cap in milliseconds |

Only transient errors are retried. Authentication errors (401/403) and bad requests (400) are never retried.

When `--verbose` is enabled, each retry attempt is logged to stderr. The `--json` envelope includes a `retry_attempts` field for observability.

```toml
[retry]
max_retries = 3
backoff = "exponential"
initial_delay_ms = 500
max_delay_ms = 30000
```
