# JSON Output

The `--json` flag wraps the agent's output in a JSON envelope with metadata for scripting and observability.

```bash
axe run my-agent --json
```

## Envelope Fields

| Field | Type | Description |
|---|---|---|
| `model` | string | The model that produced the response |
| `content` | string | The agent's text response |
| `input_tokens` | int | Total input tokens across all turns |
| `output_tokens` | int | Total output tokens across all turns |
| `stop_reason` | string | Why the LLM stopped (e.g., `end_turn`, `tool_use`) |
| `duration_ms` | int | Wall-clock time for the entire run in milliseconds |
| `tool_calls` | int | Total number of tool calls made |
| `tool_call_details` | array | Per-call details (see below) |
| `refused` | bool | Whether refusal was detected in the response |
| `retry_attempts` | int | Number of retries that occurred |
| `budget_max_tokens` | int | Token budget cap (only present when a budget is set) |
| `budget_used_tokens` | int | Cumulative tokens used (only present when a budget is set) |
| `budget_exceeded` | bool | Whether the budget was exceeded (only present when a budget is set) |

## tool_call_details

Each entry in the `tool_call_details` array represents one tool invocation:

| Field | Type | Description |
|---|---|---|
| `name` | string | Tool name (e.g., `read_file`, `run_command`) |
| `input` | object | Arguments passed to the tool |
| `output` | string | Tool output, truncated to 1024 bytes |
| `is_error` | bool | Whether the tool call resulted in an error |
| `turn` | int | Conversation turn in which the tool was called (0-indexed) |
| `duration_ms` | int | Wall-clock time for this tool call in milliseconds |

## Example

```json
{
  "model": "claude-sonnet-4-20250514",
  "content": "Here is the summary...",
  "input_tokens": 1024,
  "output_tokens": 312,
  "stop_reason": "end_turn",
  "duration_ms": 4200,
  "tool_calls": 2,
  "tool_call_details": [
    {
      "name": "read_file",
      "input": { "path": "main.go" },
      "output": "package main\n...",
      "is_error": false,
      "turn": 0,
      "duration_ms": 12
    },
    {
      "name": "run_command",
      "input": { "command": "go test ./..." },
      "output": "ok  \tgithub.com/jrswab/axe\t0.42s",
      "is_error": false,
      "turn": 0,
      "duration_ms": 1580
    }
  ],
  "refused": false,
  "retry_attempts": 0
}
```

## Streaming Compatibility

When `--stream` and `--json` are both enabled, streaming is used internally but the output is still delivered as a single JSON envelope. No incremental text is printed to stdout — the JSON remains safe for machine parsing. See [Run Flags — Streaming](run-flags.md#streaming) for details.

## Refusal Detection

The `refused` field is set to `true` when axe detects that the LLM declined to complete the task. Detection is heuristic — it scans the response for phrases like "I cannot", "I'm unable to", "I must decline", etc. See the [Refusal Detection](refusal-detection.md) page for details.
