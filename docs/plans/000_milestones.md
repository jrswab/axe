# Axe Milestones

## M1: Skeleton

Get the binary building and the CLI framework wired up.

- [ ] Init Go module (`github.com/jrswab/axe`)
- [ ] CLI framework (cobra or similar)
- [ ] `axe version`
- [ ] `axe help`
- [ ] `axe config path` — print XDG config dir
- [ ] `axe config init` — scaffold config directory

## M2: Agent Config

Load and validate agent TOML files.

- [ ] TOML parser (BurntSushi/toml or pelletier/go-toml)
- [ ] Load agent config from `$XDG_CONFIG_HOME/axe/agents/<name>.toml`
- [ ] Validate required fields (name, model)
- [ ] `axe agents list`
- [ ] `axe agents show <agent>`
- [ ] `axe agents init <agent>` — scaffold a new TOML
- [ ] `axe agents edit <agent>` — open in $EDITOR

## M3: Single Agent Run

The core loop — load config, build prompt, call LLM, print result.

- [ ] Resolve `workdir` (flag → TOML → cwd)
- [ ] Resolve file globs from workdir
- [ ] Load SKILL.md contents
- [ ] Build system prompt + context payload
- [ ] Read stdin when piped
- [ ] LLM provider integration (start with one: Anthropic or OpenAI)
- [ ] Print response to stdout
- [ ] `--model` flag override
- [ ] `--skill` flag override
- [ ] `--workdir` flag override
- [ ] `--timeout` flag
- [ ] `--dry-run` — show resolved context without calling LLM
- [ ] `--verbose` — debug info to stderr
- [ ] `--json` — wrapped output with metadata
- [ ] Exit codes (0 success, 1 agent error, 2 config error, 3 API error)

## M4: Multi-Provider Support

Support any provider/model from models.dev.

- [ ] Provider abstraction interface
- [ ] Anthropic provider
- [ ] OpenAI provider
- [ ] Ollama / local provider
- [ ] API key config (env vars, config file, or both)

## M5: Sub-Agents

Parent agents can delegate to child agents.

- [ ] Inject `call_agent` tool when `sub_agents` is defined
- [ ] Intercept tool call, load sub-agent config
- [ ] Pass task + context to sub-agent
- [ ] Return result text as tool response to parent
- [ ] Depth tracking and limiting (default 3, max 5)
- [ ] Parallel execution for concurrent tool calls
- [ ] Timeout per sub-agent
- [ ] Error handling (fail gracefully, don't crash parent)

## M6: Memory

Append-only run log with context loading.

- [ ] Append timestamped entry after each run
- [ ] Load last N entries into context on run
- [ ] `[memory]` config (enabled, last_n, max_entries)
- [ ] Memory file at `$XDG_DATA_HOME/axe/memory/<agent>.md`

## M7: Garbage Collection

Pattern detection and memory trimming.

- [ ] `axe gc <agent>` — analyze + trim
- [ ] `axe gc <agent> --dry-run` — analyze only
- [ ] `axe gc --all` — all agents
- [ ] Pattern detection prompt
- [ ] Trim to last_n entries
- [ ] Suggestions output to stdout

## Future (v2+)

- Built-in file watcher trigger
- Built-in webhook server
- Streaming output
- Structured JSON sub-agent output
- Shared memory between parent/sub-agents
- Token cost tracking
- Plugin system for custom tools
- Community skill registry
