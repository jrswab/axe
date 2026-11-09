# Providers

| Provider | Provider Name | API Key Env Var | Default Base URL |
|---|---|---|---|
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` | `https://api.anthropic.com` |
| Atlas Cloud | `atlascloud` | `ATLASCLOUD_API_KEY` | `https://api.atlascloud.ai/v1` |
| OpenAI | `openai` | `OPENAI_API_KEY` | `https://api.openai.com` |
| Google Gemini | `google` | `GEMINI_API_KEY` | `https://generativelanguage.googleapis.com` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` | `https://api.minimax.io/anthropic` |
| Ollama | `ollama` | (none required) | `http://localhost:11434` |
| OpenCode | `opencode` | `OPENCODE_API_KEY` | Configurable |
| Standard Compute | `standardcompute` | `STANDARDCOMPUTE_API_KEY` | `https://api.stdcmpt.com/v1` |
| AWS Bedrock | `bedrock` | (uses AWS credentials) | Region-based |

The "Provider Name" column is the value used in the `model` field of your agent TOML — e.g., `model = "google/gemini-2.0-flash"`.

## Configuring Credentials

API keys are resolved in this order for each provider:

1. **Environment variable** — e.g., `GEMINI_API_KEY`
2. **`config.toml`** — under `[providers.<name>]`

### config.toml

You can store credentials and base URL overrides in `$XDG_CONFIG_HOME/axe/config.toml` instead of environment variables:

```toml
[providers.anthropic]
api_key = "sk-ant-..."

[providers.atlascloud]
api_key = "..."

[providers.google]
api_key = "AIza..."

[providers.minimax]
api_key = "..."

[providers.standardcompute]
api_key = "..."

[providers.openai]
api_key = "sk-..."
base_url = "https://your-openai-proxy.example.com"

[providers.bedrock]
region = "us-east-1"
```

Base URLs can also be overridden with `AXE_<PROVIDER>_BASE_URL` environment variables (takes precedence over `config.toml`).

Atlas Cloud uses the OpenAI-compatible Chat Completions API. For example, set an agent model to `atlascloud/qwen/qwen3.8-max`.

Standard Compute uses the OpenAI-compatible Chat Completions API. Set an agent model to `standardcompute/StandardCompute` to route each request to the cheapest enabled model that can handle it, with automatic fallback, or pin a model with any id from `GET https://api.stdcmpt.com/v1/models` (for example `standardcompute/<model-id>`). See https://standardcompute.com.

## Streaming Support

All providers except AWS Bedrock support response streaming via the `--stream` flag or the `stream = true` TOML field. If a provider does not support streaming, the setting is silently ignored and the request falls back to non-streaming.

See [Run Flags — Streaming](cli/run-flags.md#streaming) for usage details and how streaming interacts with `--json`.

## AWS Bedrock

- **Region resolution order:** `AWS_REGION` → `AWS_DEFAULT_REGION` → `AXE_BEDROCK_REGION` → `[providers.bedrock] region` in `config.toml`
- **Credentials:** Uses environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`) or `~/.aws/credentials` file (supports `AWS_PROFILE` and `AWS_SHARED_CREDENTIALS_FILE`)
- **Model IDs:** Use full Bedrock model IDs (e.g., `bedrock/anthropic.claude-3-5-sonnet-20241022-v2:0`)
