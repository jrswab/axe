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
timeout = 120       # top-level request timeout in seconds (default: 120)
stream = false      # enable streaming responses (default: false)

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
reasoning_effort = "low"           # optional; OpenAI-compatible providers only (see below)
max_tokens = 4096
# response_format = "json_object"   # text, json_object, or json_schema

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

### Parameters

`reasoning_effort` is an optional string. Axe passes it through unchanged without value validation. Empty or omitted values mean the key is not sent. Only OpenAI, Atlas Cloud, Cheaper Inference, Eden AI, LiteLLM, Runcrate, and Standard Compute send it as top-level `reasoning_effort`. Anthropic, MiniMax, Ollama, OpenCode, Gemini, Bedrock, and OpenRouter ignore it. Supported values depend on the model, for example `low`, `none`, `high`, or `max`.

OpenAI, Atlas Cloud, Cheaper Inference, Eden AI, LiteLLM, Runcrate, and Standard Compute send `temperature` when TOML explicitly sets it, including `"temperature": 0`. When TOML omits `temperature`, these providers omit the key. Other providers still omit zero temperatures.

`params.cache_enabled` defaults to `true` when omitted; set it to `true` or `false` explicitly to choose whether Axe uses applicable cache hints. Each run uses its own agent definition, so a sub-agent does not inherit its parent's choice. OpenAI Chat caching is provider-managed and automatic; setting this to `false` does not disable it.

For Anthropic Messages, an enabled hint marks the system prompt and preceding ordered tools as the static prefix. If the system is empty, it marks the last tool instead; if both are empty, Axe adds no breakpoint. Conversation messages, including tasks and sub-agent context, remain outside that breakpoint. Eligibility depends on provider and model constraints and cache availability; a hint does not guarantee a hit or savings.

### Response Format

Constrain LLM output to a specific format via the `[params]` section. Forwarded by OpenAI, Atlas Cloud, Cheaper Inference, Eden AI, LiteLLM, Ollama, OpenRouter, Runcrate, and Standard Compute providers; other providers ignore this setting. For gateways, support depends on the selected model and backend.

```toml
# String shorthand
[params]
response_format = "json_object"

# Table form (required for json_schema)
[params.response_format]
type = "json_schema"
[params.response_format.schema]
name = "my_schema"
type = "object"
```

| Type | Description |
|---|---|
| `text` | Default free-form text output |
| `json_object` | Valid JSON object (provider-enforced at decoding time) |
| `json_schema` | JSON matching the provided schema; requires `[params.response_format.schema]` |
