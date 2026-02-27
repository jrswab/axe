# Axe CLI Structure

## Usage

```
axe <command> [args] [flags]
```

## Running Agents

```bash
# Run an agent
axe run pr-reviewer
axe run pr-reviewer --skill path/to/SKILL.md
axe run pr-reviewer --workdir /path/to/repo
axe run pr-reviewer --model anthropic/claude-haiku-4-20250414

# Pipe context via stdin
git diff --cached | axe run pr-reviewer
cat error.log | axe run log-analyzer
echo "$WEBHOOK_BODY" | axe run webhook-processor
```

### Run Flags

| Flag | Description |
|------|-------------|
| `--skill <path>` | Override default skill |
| `--workdir <path>` | Override working directory |
| `--model <provider/model>` | Override model |
| `--verbose` / `-v` | Show sub-agent calls, token usage, timing |
| `--dry-run` | Show resolved context without calling the LLM |
| `--timeout <seconds>` | Override timeout |
| `--json` | Wrap output with metadata (tokens, model, duration, sub-agent calls) |

### Output

- Default: LLM response printed to stdout (clean, pipeable)
- `--json`: Structured output with metadata
- `--verbose`: Debug info to stderr, response to stdout

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Agent error (LLM returned error, agent not found, etc.) |
| 2 | Config error (bad TOML, missing required fields) |
| 3 | API error (provider unreachable, auth failure, timeout) |

## Built-in Commands

### agents

```bash
axe agents list              # List all configured agents
axe agents show <agent>      # Show agent config details
axe agents init <agent>      # Scaffold a new agent TOML
axe agents edit <agent>      # Open agent TOML in $EDITOR
```

### skills

```bash
axe skills list              # List available skills
axe skills show <skill>      # Print a SKILL.md
```

### config

```bash
axe config path              # Print config directory path
axe config init              # Scaffold $XDG_CONFIG_HOME/axe/
```

### gc

```bash
axe gc <agent>               # Analyze patterns + trim memory
axe gc <agent> --dry-run     # Analyze only, don't trim
axe gc --all                 # Run GC on all agents
```

### meta

```bash
axe version                  # Print version
axe help [command]           # Show help
```

## Dry Run

`--dry-run` shows everything that would be sent to the LLM:

- Resolved system prompt
- Skill contents
- Resolved file list and contents
- Stdin input (if piped)
- Model and params
- Available sub-agents / injected tools

Useful for debugging context, estimating token cost, and verifying glob resolution.
