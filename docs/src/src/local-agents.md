# Local Agent Directories

By default, agents are loaded from `$XDG_CONFIG_HOME/axe/agents/`. Axe also supports project-local agent directories for per-repo agent definitions.

## Auto-Discovery

If `<cwd>/axe/agents/` exists, axe searches it before the global config directory. A local agent with the same name as a global agent shadows the global one.

```
my-project/
└── axe/
    └── agents/
        └── my-agent.toml   ← found automatically
```

## Explicit Override

Use `--agents-dir` to point to any directory:

```bash
axe run my-agent --agents-dir ./custom/agents
```

This flag is available on all commands: `run`, `agents list`, `agents show`, `agents init`, `agents edit`, and `gc`.

## Resolution Order

1. `--agents-dir` (if provided)
2. `<cwd>/axe/agents/` (auto-discovered)
3. `$XDG_CONFIG_HOME/axe/agents/` (global fallback)

The first directory containing a matching `<name>.toml` wins.

## Smart Scaffolding

`axe agents init <name>` writes to `<cwd>/axe/agents/` if that directory already exists, otherwise falls back to the global config directory.
