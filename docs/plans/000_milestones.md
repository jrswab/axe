# Axe Milestones

## M1: Skeleton

Get the binary building and the CLI framework wired up.

- [X] Init Go module (`github.com/jrswab/axe`)
- [X] CLI framework (cobra or similar)
- [X] `axe version`
- [X] `axe help`
- [X] `axe config path` — print XDG config dir
- [X] `axe config init` — scaffold config directory

## M2: Agent Config

Load and validate agent TOML files.

- [X] TOML parser (BurntSushi/toml or pelletier/go-toml)
- [X] Load agent config from `$XDG_CONFIG_HOME/axe/agents/<name>.toml`
- [X] Validate required fields (name, model)
- [X] `axe agents list`
- [X] `axe agents show <agent>`
- [X] `axe agents init <agent>` — scaffold a new TOML
- [X] `axe agents edit <agent>` — open in $EDITOR

## M3: Single Agent Run

The core loop — load config, build prompt, call LLM, print result.

- [X] Resolve `workdir` (flag → TOML → cwd)
- [X] Resolve file globs from workdir
- [X] Load SKILL.md contents
- [X] Build system prompt + context payload
- [X] Read stdin when piped
- [X] LLM provider integration (start with one: Anthropic or OpenAI)
- [X] Print response to stdout
- [X] `--model` flag override
- [X] `--skill` flag override
- [X] `--workdir` flag override
- [X] `--timeout` flag
- [X] `--dry-run` — show resolved context without calling LLM
- [X] `--verbose` — debug info to stderr
- [X] `--json` — wrapped output with metadata
- [X] Exit codes (0 success, 1 agent error, 2 config error, 3 API error)

## M4: Multi-Provider Support

Support any provider/model from models.dev.

- [X] Provider abstraction interface
- [X] Anthropic provider
- [X] OpenAI provider
- [X] Ollama / local provider
- [X] API key config (env vars, config file, or both)

## M5: Sub-Agents

Parent agents can delegate to child agents.

- [X] Inject `call_agent` tool when `sub_agents` is defined
- [X] Intercept tool call, load sub-agent config
- [X] Pass task + context to sub-agent
- [X] Return result text as tool response to parent
- [X] Depth tracking and limiting (default 3, max 5)
- [X] Parallel execution for concurrent tool calls
- [X] Timeout per sub-agent
- [X] Error handling (fail gracefully, don't crash parent)

## M6: Memory

Append-only run log with context loading.

- [X] Append timestamped entry after each run
- [X] Load last N entries into context on run
- [X] `[memory]` config (enabled, last_n, max_entries)
- [X] Memory file at `$XDG_DATA_HOME/axe/memory/<agent>.md`

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
