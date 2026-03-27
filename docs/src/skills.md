# Skills

Skills are reusable instruction sets that provide an agent with domain-specific knowledge and workflows. They are defined as `SKILL.md` files following the community SKILL.md format.

## Skill Resolution

The `skill` field in an agent TOML is resolved in order:

1. **Absolute path** — used as-is (e.g. `/home/user/skills/SKILL.md`)
2. **Relative to config dir** — e.g. `skills/code-review/SKILL.md` resolves to `$XDG_CONFIG_HOME/axe/skills/code-review/SKILL.md`
3. **Bare name** — e.g. `code-review` resolves to `$XDG_CONFIG_HOME/axe/skills/code-review/SKILL.md`

## Script Paths

Skills often reference helper scripts. Since `run_command` executes in the agent's `workdir` (not the skill directory), **script paths in SKILL.md must be absolute**. Relative paths will fail because the scripts don't exist in the agent's working directory.

```
# Correct — absolute path
/home/user/.config/axe/skills/my-skill/scripts/fetch.sh <args>

# Wrong — relative path won't resolve from the agent's workdir
scripts/fetch.sh <args>
```

## Directory Structure

```
$XDG_CONFIG_HOME/axe/
├── config.toml
├── agents/
│   └── my-agent.toml
└── skills/
    └── my-skill/
        ├── SKILL.md
        └── scripts/
            └── fetch.sh
```
