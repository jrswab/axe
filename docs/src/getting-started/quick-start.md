# Quick Start

## 1. Initialize Configuration

```bash
axe config init
```

Creates `$XDG_CONFIG_HOME/axe/` with a sample skill and default `config.toml`.

## 2. Scaffold an Agent

```bash
axe agents init my-agent
```

## 3. Edit the Agent

```bash
axe agents edit my-agent
```

## 4. Run the Agent

```bash
axe run my-agent
```

## Piping Input

Axe accepts stdin, so you can pipe output from other tools directly into an agent:

```bash
git diff --cached | axe run pr-reviewer
cat error.log | axe run log-analyzer
```

## Examples

The [`examples/`](https://github.com/jrswab/axe/tree/main/examples) directory contains ready-to-run agents you can copy and use immediately — a code reviewer, commit message generator, and text summarizer, each with a focused SKILL.md.

```bash
# Copy an example agent into your config
cp examples/code-reviewer/code-reviewer.toml "$(axe config path)/agents/"
cp -r examples/code-reviewer/skills/ "$(axe config path)/skills/"

# Set your API key and run
export ANTHROPIC_API_KEY="your-key-here"
git diff | axe run code-reviewer
```
