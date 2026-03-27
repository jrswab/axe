# Running Agents

Mount your config directory and pass API keys as environment variables:

```bash
docker run --rm \
  -v ./my-config:/home/axe/.config/axe \
  -e ANTHROPIC_API_KEY \
  axe run my-agent
```

Pipe stdin with the `-i` flag:

```bash
git diff | docker run --rm -i \
  -v ./my-config:/home/axe/.config/axe \
  -e ANTHROPIC_API_KEY \
  axe run pr-reviewer
```

> Without a config volume mounted, axe exits with code 2 (config error) because no agent TOML files exist.

## Running a Single Agent

If you only need to run one agent with one skill, mount just those files to their expected XDG paths inside the container. No `config.toml` is needed when API keys are passed via environment variables.

```bash
docker run --rm -i \
  -e ANTHROPIC_API_KEY \
  -v ./agents/reviewer.toml:/home/axe/.config/axe/agents/reviewer.toml:ro \
  -v ./skills/code-review/:/home/axe/.config/axe/skills/code-review/:ro \
  axe run reviewer
```

The agent's `skill` field resolves automatically against the XDG config path inside the container, so no `--skill` flag is needed.

## Overriding the Skill

To use a different skill than the one declared in the agent's TOML, use `--skill` and mount only the replacement skill:

```bash
docker run --rm -i \
  -e ANTHROPIC_API_KEY \
  -v ./agents/reviewer.toml:/home/axe/.config/axe/agents/reviewer.toml:ro \
  -v ./alt-review.md:/home/axe/alt-review.md:ro \
  axe run reviewer --skill /home/axe/alt-review.md
```

## Sub-agents

If the agent declares `sub_agents`, all referenced agent TOMLs and their skills must also be mounted.

## Persistent Memory

Agent memory persists across runs when you mount a data volume:

```bash
docker run --rm \
  -v ./my-config:/home/axe/.config/axe \
  -v axe-data:/home/axe/.local/share/axe \
  -e ANTHROPIC_API_KEY \
  axe run my-agent
```

## Volume Mounts

| Container Path | Purpose | Default Access |
|---|---|---|
| `/home/axe/.config/axe/` | Agent TOML files, skills, `config.toml` | Read-write |
| `/home/axe/.local/share/axe/` | Persistent memory files | Read-write |

Config is read-write because `axe config init` and `axe agents init` write into it. Mount as `:ro` if you only run agents.

## Environment Variables

| Variable | Required | Purpose |
|---|---|---|
| `ANTHROPIC_API_KEY` | If using Anthropic | API authentication |
| `OPENAI_API_KEY` | If using OpenAI | API authentication |
| `AXE_OLLAMA_BASE_URL` | If using Ollama | Ollama endpoint (default in compose: `http://ollama:11434`) |
| `AXE_ANTHROPIC_BASE_URL` | No | Override Anthropic API endpoint |
| `AXE_OPENAI_BASE_URL` | No | Override OpenAI API endpoint |
| `AXE_OPENCODE_BASE_URL` | No | Override OpenCode API endpoint |
| `TAVILY_API_KEY` | If using web_search | Tavily web search API key |
| `AXE_WEB_SEARCH_BASE_URL` | No | Override web search endpoint |
