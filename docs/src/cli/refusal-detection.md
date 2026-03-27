# Refusal Detection

Axe automatically detects when an LLM declines to complete a task and surfaces this in the `--json` output envelope as `"refused": true`.

## How It Works

Detection is heuristic — axe scans the response content for patterns indicating refusal:

- "I cannot", "I can't"
- "I'm unable to", "I am unable to"
- "I'm not able to", "I am not able to"
- "I must decline"
- "I don't have the ability", "I do not have the ability"
- "as an AI" combined with limiting language

## Usage

Refusal detection is only active when using `--json` output:

```bash
axe run my-agent --json | jq '.refused'
```

## Limitations

This is a heuristic and not exhaustive. A model that declines in an unusual phrasing may not be detected. It is intended as a lightweight signal for scripting and observability — not a guarantee.
