# Run Flags

| Flag | Default | Description |
|---|---|---|
| `--model <provider/model>` | from TOML | Override the model (e.g. `anthropic/claude-sonnet-4-20250514`) |
| `--skill <path>` | from TOML | Override the skill file path |
| `--workdir <path>` | from TOML or cwd | Override the working directory |
| `--timeout <seconds>` | 120 | Request timeout |
| `--max-tokens <int>` | 0 (unlimited) | Cap cumulative token usage for the run (exit code 4 if exceeded) |
| `--dry-run` | false | Show resolved context without calling the LLM |
| `--verbose` / `-v` | false | Print debug info (model, timing, tokens, retries) to stderr |
| `--json` | false | Wrap output in a JSON envelope with metadata |
| `-p` / `--prompt <string>` | (none) | Inline prompt used as the user message; takes precedence over stdin |
| `--agents-dir <path>` | (auto-discover) | Override agent search directory |

## User Message Precedence

The user message sent to the LLM is resolved in this order:

1. **`-p` / `--prompt` flag** — If provided with a non-empty, non-whitespace value, it is used as the user message.
2. **Piped stdin** — If `-p` is absent or empty/whitespace-only, piped stdin is used.
3. **Built-in default** — If neither `-p` nor stdin provides content, the default message `"Execute the task described in your instructions."` is used.

When `-p` is provided alongside piped stdin, the piped stdin is silently ignored. An empty or whitespace-only `-p` value is treated as absent and falls through to stdin, then the default.

**Example:**
```bash
axe run my-agent -p "Summarize the README"
```
