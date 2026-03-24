# Introduction

![axe banner](../banner.png)

Most AI tooling assumes you want a chatbot. A long-running session with a massive context window doing everything at once. But that's not how good software works. Good software is small, focused, and composable.

Axe treats LLM agents the same way Unix treats programs. Each agent does one thing well. You define it in a TOML file, give it a focused skill, and run it from the command line. Pipe data in, get results out. Chain agents together. Trigger them from cron, git hooks, or CI. Whatever you already use. No daemon, no GUI, no framework to buy into. Just a binary and your configs.

## Overview

Axe orchestrates LLM-powered agents defined via TOML configuration files. Each agent has its own system prompt, model selection, skill files, context files, working directory, persistent memory, and the ability to delegate to sub-agents.

Axe is the executor, not the scheduler. It is designed to be composed with standard Unix tools — cron, git hooks, pipes, file watchers — rather than reinventing scheduling or workflow orchestration.

## Features

- **Multi-provider support** — Anthropic, OpenAI, Ollama (local models), OpenCode, and AWS Bedrock
- **TOML-based agent configuration** — declarative, version-controllable agent definitions
- **Sub-agent delegation** — agents can call other agents via LLM tool use, with depth limiting and parallel execution
- **Persistent memory** — timestamped markdown logs that carry context across runs
- **Memory garbage collection** — LLM-assisted pattern analysis and trimming
- **Skill system** — reusable instruction sets that can be shared across agents
- **Stdin piping** — pipe any output directly into an agent (`git diff | axe run reviewer`)
- **Local agent directories** — auto-discovers agents from `<cwd>/axe/agents/` before the global config, or use `--agents-dir` to point anywhere
- **Dry-run mode** — inspect resolved context without calling the LLM
- **JSON output** — structured output with metadata for scripting
- **Built-in tools** — file operations (read, write, edit, list) sandboxed to working directory; shell command execution; URL fetching; web search
- **Output allowlist** — restrict `url_fetch` and `web_search` to specific hostnames; private/reserved IPs are always blocked (SSRF protection)
- **Token budgets** — cap cumulative token usage per agent run via `[budget]` config or `--max-tokens` flag
- **MCP tool support** — connect to external MCP servers for additional tools via SSE or streamable-HTTP transport
- **Configurable retry** — exponential, linear, or fixed backoff for transient provider errors (429, 5xx, timeouts)
- **Minimal dependencies** — four direct dependencies (cobra, toml, mcp-go-sdk, x/net); all LLM calls use the standard library
