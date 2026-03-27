# Token Budget

Cap cumulative token usage (input + output, across all turns and sub-agent calls) for a single run.

## Configuration

```toml
[budget]
max_tokens = 50000   # 0 = unlimited (default)
```

## Override via Flag

```bash
axe run my-agent --max-tokens 10000
```

The flag takes precedence over TOML when set to a value greater than zero.

## Behavior

When the budget is exceeded, the current response is returned but no further tool calls execute. The process exits with **exit code 4**. Memory is not appended on a budget-exceeded run.

With `--verbose`, each turn logs cumulative usage to stderr. With `--json`, the output envelope includes `budget_max_tokens`, `budget_used_tokens`, and `budget_exceeded` fields (omitted when unlimited).
