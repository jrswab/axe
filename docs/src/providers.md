# Providers

| Provider | Provider Name | API Key Env Var | Default Base URL |
|---|---|---|---|
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` | `https://api.anthropic.com` |
| Atlas Cloud | `atlascloud` | `ATLASCLOUD_API_KEY` | `https://api.atlascloud.ai/v1` |
| Cheaper Inference | `cheaperinference` | `CHEAPER_INFERENCE_API_KEY` | `https://api.cheaperinference.com/v1` |
| LiteLLM (proxy) | `litellm` | `LITELLM_API_KEY` | `http://localhost:4000/v1` |
| OpenAI | `openai` | `OPENAI_API_KEY` | `https://api.openai.com` |
| Google Gemini | `google` | `GEMINI_API_KEY` | `https://generativelanguage.googleapis.com` |
| MiniMax | `minimax` | `MINIMAX_API_KEY` | `https://api.minimax.io/anthropic` |
| Eden AI | `edenai` | `EDENAI_API_KEY` | `https://api.edenai.run/v3` |
| Ollama | `ollama` | (none required) | `http://localhost:11434` |
| OpenCode | `opencode` | `OPENCODE_API_KEY` | Configurable |
| Runcrate | `runcrate` | `RUNCRATE_API_KEY` | `https://api.runcrate.ai/v1` |
| Standard Compute | `standardcompute` | `STANDARDCOMPUTE_API_KEY` | `https://api.stdcmpt.com/v1` |
| AWS Bedrock | `bedrock` | (uses AWS credentials) | Region-based |

The "Provider Name" column is the value used in the `model` field of your agent TOML — e.g., `model = "google/gemini-2.0-flash"`.

## Prompt Caching

Each run resolves `params.cache_enabled` from its own agent definition: omitted or `true` enables applicable Axe hints; `false` disables them. A sub-agent does not inherit its parent's choice.

For Anthropic Messages, an enabled ephemeral breakpoint marks the system prompt and preceding ordered tools. With an empty system and tools present, the breakpoint instead marks the last tool; with neither, Axe adds no breakpoint. Conversation messages remain outside that static prefix. Model and provider constraints and cache availability determine eligibility; a hint does not guarantee a hit or savings. See [Anthropic's prompt caching guide](https://platform.claude.com/docs/en/build-with-claude/prompt-caching).

For Bedrock Converse, enabled hints add separate default checkpoints after the system text and after the ordered tool specifications when each is present; empty static inputs add no checkpoint. Tool schema required names have stable ordering, and changing conversation messages remain outside these static prefixes. AWS model, API, region and routing determine support, checkpoint limits, minimum prefix sizes and TTL; shorter prefixes may not be cached. Cache reads and writes are provider-reported counts, not proof of hits or savings. See the [AWS prompt caching guide](https://docs.aws.amazon.com/bedrock/latest/userguide/prompt-caching.html) for current eligibility.

Anthropic reports cache-read and cache-write tokens when provided in usage. OpenAI Chat caching is provider-managed and automatic; `cache_enabled = false` does not disable it. Axe sends no explicit OpenAI cache request controls and maps reported cache-read tokens only, not cache-write tokens. Compatible adapters share request code but are not guaranteed to support first-party caching behavior.

Cache counts are provider reports for this run only. With `--verbose` and no token budget, a `Cache:` line shows only the nonzero counts, for example `Cache:    120 read, 40 written` or `Cache:    120 read`; with a budget it shows both counts, as before; `--json` reports the same cumulative counts as `cache_read_tokens` and `cache_write_tokens`, omitted when zero. A sub-agent's counts stay inside its own run. Anthropic and Bedrock report reads and writes; OpenAI Chat reports reads only. Counts are not proof of hits or savings.

**Optional probe.** To observe caching yourself, run the same agent twice with an identical system prompt and tools and a different task, using `--verbose` or `--json`, and compare the counts. Each run is a paid provider call made with your own credentials. Results are point-in-time and depend on model, region and provider limits; a zero count does not prove caching is disabled or unsupported. Do not include API keys or other secrets when you record results.

## Configuring Credentials

For providers using ordinary API keys, keys are resolved in this order (Bedrock Runs use AWS credentials instead):

1. **Environment variable** — e.g., `GEMINI_API_KEY`
2. **`config.toml`** — under `[providers.<name>]`

### config.toml

You can store credentials and base URL overrides in `$XDG_CONFIG_HOME/axe/config.toml` instead of environment variables:

```toml
[providers.anthropic]
api_key = "sk-ant-..."

[providers.atlascloud]
api_key = "..."

[providers.cheaperinference]
api_key = "..."

[providers.edenai]
api_key = "..."

[providers.litellm]
api_key = "sk-..."          # the proxy's virtual or master key
base_url = "http://localhost:4000"

[providers.google]
api_key = "AIza..."

[providers.minimax]
api_key = "..."

[providers.standardcompute]
api_key = "..."

[providers.openai]
api_key = "sk-..."
base_url = "https://your-openai-proxy.example.com"

[providers.runcrate]
api_key = "..."

[providers.bedrock]
region = "us-east-1"
```

For providers other than Bedrock, base URLs can also be overridden with `AXE_<PROVIDER>_BASE_URL` environment variables (takes precedence over `config.toml`).

Atlas Cloud uses the OpenAI-compatible Chat Completions API. For example, set an agent model to `atlascloud/qwen/qwen3.8-max`.

Cheaper Inference uses the OpenAI-compatible Chat Completions API. For example, set an agent model to `cheaperinference/gpt-5.4-mini`.

Eden AI is a gateway, so one key reaches models from several upstream vendors. Set an agent model to the plain model name, for example `edenai/mistral-large-4` or `edenai/claude-haiku-5.5`; the current catalog is at https://www.edenai.co/models. Older vendor-prefixed ids such as `edenai/openai/gpt-4o-mini` still work. Override the endpoint with `AXE_EDENAI_BASE_URL` or `[providers.edenai] base_url`. Custom URLs must be absolute, without credentials, a query, or a fragment. HTTPS is required except for `localhost` and loopback/private IP literals; for an HTTP private proxy, use its private IP rather than a DNS name. These explicit HTTP exceptions send the API key in plaintext, so use only trusted local networks. A trailing slash is optional.

Runcrate serves open-weight models through the OpenAI-compatible Chat Completions API. For example, set an agent model to `runcrate/deepseek-ai/DeepSeek-V4.1-Flash`.

Standard Compute uses the OpenAI-compatible Chat Completions API. Set an agent model to `standardcompute/StandardCompute` to route each request to the cheapest enabled model that can handle it, with automatic fallback, or pin a model with any id from `GET https://api.stdcmpt.com/v1/models` (for example `standardcompute/<model-id>`). See https://standardcompute.com.

## LiteLLM

[LiteLLM](https://github.com/BerriAI/litellm) is a self-hosted AI gateway: one OpenAI-compatible endpoint in front of 100+ providers (Anthropic, Bedrock, Vertex AI, Azure OpenAI, Mistral, and more), with virtual keys, budgets and fallbacks. Point axe at the proxy and use the proxy's model names:

- **Model:** `litellm/<model name on the proxy>`, e.g. `litellm/claude-sonnet` or `litellm/gemini-2.5-flash`. Everything after the first `/` is sent as-is, so aliases containing `/` work too.
- **Base URL:** `AXE_LITELLM_BASE_URL` or `[providers.litellm] base_url`. Both `http://host:4000` and `http://host:4000/v1` work; the default is `http://localhost:4000/v1`.
- **Key:** `LITELLM_API_KEY` or `[providers.litellm] api_key`, the proxy's virtual key.
- **Cost:** when available, axe reads `x-litellm-response-cost` into the JSON result's `cost` field. This is the **final response's cost**, not the total across earlier requests in a run with tool calls. A positive header value takes precedence; streaming falls back to numeric `usage.cost` when the header is absent, zero, or invalid. Missing, invalid, negative, and non-finite costs are treated as unavailable; zero cost is omitted from JSON output.

For streaming cost reporting, enable cost metadata in the **LiteLLM proxy's YAML configuration** (not Axe's `config.toml`):

```yaml
litellm_settings:
  include_cost_in_streaming_usage: true
```

This enables `usage.cost` on supported proxy streaming paths. Without cost metadata from the proxy, the run still succeeds but the JSON result omits `cost`.

## Streaming Support

All providers except AWS Bedrock support response streaming via the `--stream` flag or the `stream = true` TOML field. If a provider does not support streaming, the setting is silently ignored and the request falls back to non-streaming.

See [Run Flags — Streaming](cli/run-flags.md#streaming) for usage details and how streaming interacts with `--json`.

## AWS Bedrock

Main and Nested Runs use the same AWS setup; neither uses an ordinary API key or base URL override.

- **Region resolution order:** `AWS_REGION` → `AWS_DEFAULT_REGION` → `AXE_BEDROCK_REGION` → `[providers.bedrock] region` in `config.toml`. A region is required and must be a single lowercase DNS label (1–63 lowercase ASCII letters, digits or hyphens, without a leading or trailing hyphen).
- **Credentials:** A complete `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment pair takes precedence, with optional `AWS_SESSION_TOKEN`. Otherwise, Axe reads `AWS_SHARED_CREDENTIALS_FILE` or `~/.aws/credentials`, selecting `AWS_PROFILE` or `default`; the profile must contain both keys. This is not the full AWS SDK credential chain. Nested setup failures provide guidance without revealing local credential-file paths to the parent.
- **Model IDs:** Use full Bedrock model IDs (e.g., `bedrock/anthropic.claude-3-5-sonnet-20241022-v2:0`).
