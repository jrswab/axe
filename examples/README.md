# Examples

Ready-to-run agents that demonstrate common axe patterns. Copy any agent's
directory into your axe config and go.

## Setup

1. Find your config directory:

   ```bash
   axe config path
   ```

   If it doesn't exist yet:

   ```bash
   axe config init
   ```

2. Copy the example agents and skills into your config:

   ```bash
   cp examples/code-reviewer/code-reviewer.toml "$(axe config path)/agents/"
   cp examples/commit-msg/commit-msg.toml "$(axe config path)/agents/"
   cp examples/summarizer/summarizer.toml "$(axe config path)/agents/"
   cp -r examples/code-reviewer/skills/code-review "$(axe config path)/skills/"
   cp -r examples/commit-msg/skills/commit-msg "$(axe config path)/skills/"
   cp -r examples/summarizer/skills/summarizer "$(axe config path)/skills/"
   ```

3. Set your API key:

   ```bash
   export ANTHROPIC_API_KEY="your-key-here"
   ```

4. Run:

   ```bash
   axe run code-reviewer
   ```

## Agents

### code-reviewer

Reviews code diffs for bugs, style issues, and improvements.

```bash
git diff | axe run code-reviewer
git diff main..feature | axe run code-reviewer
```

### commit-msg

Generates conventional commit messages from staged changes.

```bash
git diff --cached | axe run commit-msg
```

Use it in a git hook — add to `.git/hooks/prepare-commit-msg`:

```bash
#!/bin/sh
git diff --cached | axe run commit-msg > "$1"
```

### summarizer

Condenses text into key points. Works with any text input.

```bash
cat article.md | axe run summarizer
curl -s https://example.com/page | axe run summarizer
pbpaste | axe run summarizer
```
