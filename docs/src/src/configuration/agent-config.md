# Agent Configuration

Agents are defined as TOML files in `$XDG_CONFIG_HOME/axe/agents/`.

All fields except `name` and `model` are optional.

```toml
name = "pr-reviewer"
description = "Reviews pull requests for issues and improvements"
model = "anthropic/claude-sonnet-4-20250514"
system_prompt = "You are a senior code reviewer. Be concise and actionable."
skill = "skills/code-review/SKILL.md"
files = ["src/**/*.go", "CONTRIBUTING.md"]
workdir = "/home/user/projects/myapp"
tools = ["read_file", "list_directory", "run_command"]
sub_agents = ["test-runner", "lint-checker"]
allowed_hosts = ["api.example.com", "docs.example.com"]

[sub_agents_config]
max_depth = 3       # maximum nesting depth (hard max: 5)
parallel = true     # run sub-agents concurrently
timeout = 120       # per sub-agent timeout in seconds

[memory]
enabled = true
last_n = 10         # load last N entries into context
max_entries = 100   # warn when exceeded

[[mcp_servers]]
name = "my-tools"
url = "https://my-mcp-server.example.com/sse"
transport = "sse"
headers = { Authorization = "Bearer ${MY_TOKEN}" }

[params]
temperature = 0.3
max_tokens = 4096

[budget]
max_tokens = 50000    # 0 = unlimited (default)

[retry]
max_retries = 3
backoff = "exponential"
initial_delay_ms = 500
max_delay_ms = 30000

[[mcp_servers]]
name = "filesystem"
transport = "stdio"
command = "/usr/local/bin/mcp-server-filesystem"
args = ["--root", "/home/user/projects"]
```
