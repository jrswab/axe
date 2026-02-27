# Axe Agent Config Schema

Agent configs live at `$XDG_CONFIG_HOME/axe/agents/<name>.toml`.

## Example

```toml
name = "pr-reviewer"
description = "Reviews pull requests for style and correctness"

# Full provider/model per models.dev
model = "anthropic/claude-sonnet-4-20250514"

# Agent persona — reusable across skills
system_prompt = """
You are a senior code reviewer. Be concise, actionable,
and cite line numbers.
"""

# Default skill (can be overridden with --skill flag)
skill = "skills/code-review/SKILL.md"

# Context files (globs resolved from workdir or cwd)
files = [
  "CONTRIBUTING.md",
  "src/**/*.go",
]

# Working directory (required for cron/automation, optional for interactive use)
# Resolution order: --workdir flag → TOML workdir → cwd
workdir = "/home/user/projects/myapp"

# Sub-agents this agent can invoke
sub_agents = ["test-runner", "lint-checker"]

[memory]
enabled = false
path = ""  # defaults to $XDG_DATA_HOME/axe/memory/<agent-name>/

[params]
temperature = 0.3
max_tokens = 4096
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Agent identifier |
| `description` | string | no | Human-readable description |
| `model` | string | yes | Provider/model string per models.dev |
| `system_prompt` | string | no | Agent persona/instructions |
| `skill` | string | no | Path to SKILL.md (default, overridable via `--skill`) |
| `files` | string[] | no | Glob patterns for context files |
| `workdir` | string | no | Working directory for glob resolution |
| `sub_agents` | string[] | no | Names of agents this agent can invoke |
| `memory.enabled` | bool | no | Enable persistent memory (default: false) |
| `memory.path` | string | no | Custom memory directory |
| `params.temperature` | float | no | Model temperature |
| `params.max_tokens` | int | no | Max output tokens |

## Stdin

Piped input is always accepted as additional context when present. No config needed.

```bash
# Pipe git diff as context
git diff --cached | axe run pr-reviewer

# Pipe logs
cat error.log | axe run log-analyzer

# Pipe webhook body
echo "$WEBHOOK_BODY" | axe run webhook-processor
```

## Triggers (v1)

Axe is the executor, not the scheduler. Use existing tools for triggers:

| Trigger | How |
|---------|-----|
| Manual | `axe run <agent>` |
| Cron | System cron/systemd timer calls `axe run` |
| Git hooks | `.git/hooks/*` calls `axe run` |
| File watch | `entr`/`watchman`/`fswatch` pipes to `axe run` |
| Webhook | User's HTTP server calls `axe run` |
| Pipe | stdin piped as additional context |

## Runtime Overrides

CLI flags override TOML values:

- `--skill <path>` — override default skill
- `--workdir <path>` — override working directory
- `--model <provider/model>` — override model
